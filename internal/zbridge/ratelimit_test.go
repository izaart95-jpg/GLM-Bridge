package zbridge

import (
    "context"
    "net/http"
    "net/http/httptest"
    "sync"
    "testing"
    "time"
)

// recordingTransport notes when each request reached the underlying transport.
type recordingTransport struct {
    mu    sync.Mutex
    times []time.Time
}

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    r.mu.Lock()
    r.times = append(r.times, time.Now())
    r.mu.Unlock()
    return &http.Response{
        StatusCode: 200,
        Body:       http.NoBody,
        Request:    req,
    }, nil
}

func (r *recordingTransport) gaps() []time.Duration {
    r.mu.Lock()
    defer r.mu.Unlock()
    var out []time.Duration
    for i := 1; i < len(r.times); i++ {
        out = append(out, r.times[i].Sub(r.times[i-1]))
    }
    return out
}

func TestPacedTransportSpacesSequentialRequests(t *testing.T) {
    rec := &recordingTransport{}
    const gap = 40 * time.Millisecond
    rt := pacedWithGap(rec, gap)

    for i := 0; i < 4; i++ {
        req := httptest.NewRequest("GET", "https://example.invalid/", nil)
        if _, err := rt.RoundTrip(req); err != nil {
            t.Fatalf("request %d: %v", i, err)
        }
    }
    for i, got := range rec.gaps() {
        // Timers fire no earlier than requested; allow slack for scheduling.
        if got < gap-5*time.Millisecond {
            t.Fatalf("gap %d too small: %v, want >= %v", i, got, gap)
        }
    }
}

// A burst is the shape that trips the WAF, so concurrent callers must be
// spread out too, not merely delayed by the same amount.
func TestPacedTransportSpreadsBurst(t *testing.T) {
    rec := &recordingTransport{}
    const gap = 25 * time.Millisecond
    const n = 6
    rt := pacedWithGap(rec, gap)

    var wg sync.WaitGroup
    start := time.Now()
    for i := 0; i < n; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            req := httptest.NewRequest("GET", "https://example.invalid/", nil)
            if _, err := rt.RoundTrip(req); err != nil {
                t.Errorf("burst request: %v", err)
            }
        }()
    }
    wg.Wait()

    if got := len(rec.times); got != n {
        t.Fatalf("want %d requests through, got %d", n, got)
    }
    // n requests at one per gap cannot finish faster than (n-1) gaps.
    if elapsed := time.Since(start); elapsed < time.Duration(n-1)*gap-10*time.Millisecond {
        t.Fatalf("burst was not paced: %d requests in %v", n, elapsed)
    }
}

func TestPacedTransportHonoursContextCancellation(t *testing.T) {
    rec := &recordingTransport{}
    rt := pacedWithGap(rec, time.Second)

    // First request claims the slot immediately, the second must wait a second.
    if _, err := rt.RoundTrip(httptest.NewRequest("GET", "https://example.invalid/", nil)); err != nil {
        t.Fatalf("first request: %v", err)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
    defer cancel()
    req := httptest.NewRequest("GET", "https://example.invalid/", nil).WithContext(ctx)
    if _, err := rt.RoundTrip(req); err == nil {
        t.Fatal("want the wait to abort with the request context, got nil error")
    }
    if got := len(rec.times); got != 1 {
        t.Fatalf("cancelled request still reached upstream: %d calls", got)
    }
}

// A zero interval must hand the request straight to the base transport.
func TestPacedTransportDisabled(t *testing.T) {
    rec := &recordingTransport{}
    rt := pacedWithGap(rec, 0)
    start := time.Now()
    for i := 0; i < 3; i++ {
        if _, err := rt.RoundTrip(httptest.NewRequest("GET", "https://example.invalid/", nil)); err != nil {
            t.Fatalf("request %d: %v", i, err)
        }
    }
    if elapsed := time.Since(start); elapsed > 20*time.Millisecond {
        t.Fatalf("pacing still applied with a zero interval: %v", elapsed)
    }
}

// The interval is read on first use, so a value set after construction still
// takes effect — that is what lets TestMain switch pacing off for the mock.
func TestPacedTransportReadsIntervalLazily(t *testing.T) {
    rec := &recordingTransport{}
    rt := newPacedTransport(rec)
    t.Setenv("UPSTREAM_MIN_INTERVAL_MS", "0")
    start := time.Now()
    for i := 0; i < 3; i++ {
        if _, err := rt.RoundTrip(httptest.NewRequest("GET", "https://example.invalid/", nil)); err != nil {
            t.Fatalf("request %d: %v", i, err)
        }
    }
    if elapsed := time.Since(start); elapsed > 20*time.Millisecond {
        t.Fatalf("environment set after construction was ignored: %v", elapsed)
    }
}

func TestUpstreamMinIntervalFromEnv(t *testing.T) {
    cases := map[string]time.Duration{
        "":       defaultUpstreamMinIntervalMS * time.Millisecond,
        "0":      0,
        "750":    750 * time.Millisecond,
        "-5":     defaultUpstreamMinIntervalMS * time.Millisecond,
        "nonsen": defaultUpstreamMinIntervalMS * time.Millisecond,
    }
    for raw, want := range cases {
        // An empty value reads back as "" from os.Getenv, same as unset.
        t.Setenv("UPSTREAM_MIN_INTERVAL_MS", raw)
        if got := upstreamMinInterval(); got != want {
            t.Fatalf("UPSTREAM_MIN_INTERVAL_MS=%q: got %v, want %v", raw, got, want)
        }
    }
}

// pacedWithGap builds a transport with a fixed interval, bypassing the
// environment lookup.
func pacedWithGap(base http.RoundTripper, gap time.Duration) http.RoundTripper {
    return &pacedTransport{base: base, gapFor: func() time.Duration { return gap }}
}
