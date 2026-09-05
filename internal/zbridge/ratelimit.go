package zbridge

import (
    "net/http"
    "os"
    "strconv"
    "sync"
    "time"
)

// Upstream pacing.
//
// One proxied chat request costs several upstream calls — a captcha challenge,
// the completion itself, and the DELETE that retires the throwaway chat — and
// the session pool refills in the background on top of that. Bursts therefore
// hit chat.z.ai far harder than the request count suggests, and Aliyun's WAF
// answers a burst by blocking the source IP: every later request comes back as
// the 405 block page, for hours, no matter which account or session is used.
//
// Pacing every upstream call through one shared gate keeps a burst from ever
// forming. It is deliberately a floor on the gap between requests rather than
// a token bucket: a bucket lets a burst through as long as the average holds,
// which is exactly the shape that trips the WAF.
//
// UPSTREAM_MIN_INTERVAL_MS tunes it; 0 disables pacing entirely.
const defaultUpstreamMinIntervalMS = 200

func upstreamMinInterval() time.Duration {
    raw := os.Getenv("UPSTREAM_MIN_INTERVAL_MS")
    if raw == "" {
        return defaultUpstreamMinIntervalMS * time.Millisecond
    }
    ms, err := strconv.Atoi(raw)
    if err != nil || ms < 0 {
        return defaultUpstreamMinIntervalMS * time.Millisecond
    }
    return time.Duration(ms) * time.Millisecond
}

// pacedTransport spaces out the requests handed to base. Callers block in
// RoundTrip until their slot is due, so pacing applies no matter which code
// path issues the call.
type pacedTransport struct {
    base http.RoundTripper

    // The interval is resolved on first use, not at package init: these
    // transports are package-level variables, so reading the environment in
    // the constructor would freeze the value before main (or TestMain) has
    // had a chance to set it.
    gapOnce sync.Once
    gap     time.Duration
    gapFor  func() time.Duration

    mu   sync.Mutex
    next time.Time // earliest instant the next request may start
}

// newPacedTransport wraps base with the interval from the environment.
func newPacedTransport(base http.RoundTripper) http.RoundTripper {
    return &pacedTransport{base: base, gapFor: upstreamMinInterval}
}

func (t *pacedTransport) minGap() time.Duration {
    t.gapOnce.Do(func() {
        gapFor := t.gapFor
        if gapFor == nil {
            gapFor = upstreamMinInterval
        }
        t.gap = gapFor()
    })
    return t.gap
}

func (t *pacedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    minGap := t.minGap()
    if minGap <= 0 {
        return t.base.RoundTrip(req) // pacing disabled
    }

    // Claim a slot: concurrent callers queue up rather than all waiting for
    // the same instant and then firing together.
    t.mu.Lock()
    now := time.Now()
    slot := t.next
    if slot.Before(now) {
        slot = now
    }
    t.next = slot.Add(minGap)
    t.mu.Unlock()

    if wait := time.Until(slot); wait > 0 {
        timer := time.NewTimer(wait)
        defer timer.Stop()
        select {
        case <-timer.C:
        case <-req.Context().Done():
            return nil, req.Context().Err()
        }
    }
    return t.base.RoundTrip(req)
}
