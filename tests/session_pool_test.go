// Blackbox tests for the throwaway-session lifecycle (package tests),
// driving internal/zbridge through its exported API only — the same layout
// the DeepseekFreeAPI reference uses (reference/tests/session_pool_test.go):
//
//   - pool warmup / acquire / release / refill discipline
//   - acquire timeout + shutdown semantics (leftover cleanup, no refill)
//   - create-retry backoff path
//   - DeleteZAIChat against a mock chat.z.ai (DELETE /api/v1/chats/{id}:
//     "true" on success, {"detail":"We could not find ..."} when already
//     gone — which must count as a successful idempotent delete)
//   - the sync-mode glue (Acquire/ReleaseStatelessSession with no pool
//     attached) end to end through the GC
//   - the full HTTP loop: handler draws a pooled session, deletes it after
//     the response, refills the batch

package tests

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "sync"
    "testing"
    "time"

    "zai-api/internal/zbridge"
)

// ── stub backend ────────────────────────────────────────────────────────────

// stubBackend is a scriptable zbridge.SessionBackend for pool tests.
type stubBackend struct {
    mu          sync.Mutex
    nextID      int
    created     []string
    deleted     []string
    failCreates int  // that many upcoming CreateChatSession calls fail first
    blockCreate bool // park creates until their context expires
}

func (s *stubBackend) CreateChatSession(ctx context.Context) (string, error) {
    s.mu.Lock()
    if s.blockCreate {
        s.mu.Unlock()
        <-ctx.Done()
        return "", ctx.Err()
    }
    if s.failCreates > 0 {
        s.failCreates--
        s.mu.Unlock()
        return "", errors.New("stub create failure")
    }
    s.nextID++
    id := fmt.Sprintf("stub-session-%d", s.nextID)
    s.created = append(s.created, id)
    s.mu.Unlock()
    return id, nil
}

func (s *stubBackend) DeleteChatSession(ctx context.Context, ids ...string) error {
    s.mu.Lock()
    s.deleted = append(s.deleted, ids...)
    s.mu.Unlock()
    return nil
}

func (s *stubBackend) snapshot() (created, deleted []string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    created = append(created, s.created...)
    deleted = append(deleted, s.deleted...)
    return
}

// waitFor polls cond until it holds or the timeout elapses.
func waitFor(t *testing.T, what string, timeout time.Duration, cond func() bool) {
    t.Helper()
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        if cond() {
            return
        }
        time.Sleep(5 * time.Millisecond)
    }
    t.Fatalf("timed out waiting for %s", what)
}

// ── pool semantics ──────────────────────────────────────────────────────────

func TestSessionPoolWarmupAcquireRelease(t *testing.T) {
    b := &stubBackend{}
    p := zbridge.NewSessionPool(b, 3)
    p.Start()

    waitFor(t, "warmup to stock 3 sessions", 3*time.Second, func() bool {
        return p.Ready() == 3
    })

    // The whole batch can be checked out; every ID is unique.
    seen := map[string]bool{}
    var first string
    for i := 0; i < 3; i++ {
        id, err := p.Acquire(context.Background(), time.Second)
        if err != nil {
            t.Fatalf("Acquire %d failed: %v", i, err)
        }
        if seen[id] {
            t.Fatalf("Acquire handed out duplicate session %q", id)
        }
        seen[id] = true
        if i == 0 {
            first = id
        }
    }
    if p.Ready() != 0 {
        t.Fatalf("ready = %d after draining the batch, want 0", p.Ready())
    }

    // Releasing a consumed session deletes it upstream and refills the batch.
    p.Release(first)
    waitFor(t, "used session deleted + batch refilled", 3*time.Second, func() bool {
        _, deleted := b.snapshot()
        return len(deleted) == 1 && deleted[0] == first && p.Ready() == 1
    })

    p.Shutdown()
}

func TestSessionPoolAcquireTimeout(t *testing.T) {
    b := &stubBackend{}
    p := zbridge.NewSessionPool(b, 1) // never Started: batch stays empty

    start := time.Now()
    _, err := p.Acquire(context.Background(), 50*time.Millisecond)
    if !errors.Is(err, zbridge.ErrPoolTimeout) {
        t.Fatalf("Acquire on empty pool: err = %v, want ErrPoolTimeout", err)
    }
    if elapsed := time.Since(start); elapsed > 2*time.Second {
        t.Fatalf("Acquire took %s, want ~50ms", elapsed)
    }

    // A canceled request context wins over the wait window.
    ctx, cancel := context.WithCancel(context.Background())
    cancel()
    if _, err := p.Acquire(ctx, time.Minute); !errors.Is(err, context.Canceled) {
        t.Fatalf("Acquire with canceled ctx: err = %v, want context.Canceled", err)
    }
}

func TestSessionPoolShutdownClearsLeftovers(t *testing.T) {
    b := &stubBackend{}
    p := zbridge.NewSessionPool(b, 3)
    p.Start()

    waitFor(t, "warmup to stock 3 sessions", 3*time.Second, func() bool {
        return p.Ready() == 3
    })

    p.Shutdown()

    _, deleted := b.snapshot()
    if len(deleted) != 3 {
        t.Fatalf("shutdown deleted %d sessions (%v), want all 3", len(deleted), deleted)
    }
    if p.Ready() != 0 {
        t.Fatalf("ready = %d after shutdown, want 0", p.Ready())
    }

    // Acquire after shutdown is refused, and a second Shutdown is a no-op.
    if _, err := p.Acquire(context.Background(), 50*time.Millisecond); !errors.Is(err, zbridge.ErrPoolClosing) {
        t.Fatalf("Acquire after shutdown: err = %v, want ErrPoolClosing", err)
    }
    p.Shutdown()
    _, deleted = b.snapshot()
    if len(deleted) != 3 {
        t.Fatalf("second shutdown deleted more sessions: %v", deleted)
    }
}

func TestSessionPoolReleaseAfterShutdownNoRefill(t *testing.T) {
    b := &stubBackend{}
    p := zbridge.NewSessionPool(b, 1)
    p.Start()

    waitFor(t, "warmup to stock 1 session", 3*time.Second, func() bool {
        return p.Ready() == 1
    })

    id, err := p.Acquire(context.Background(), time.Second)
    if err != nil {
        t.Fatalf("Acquire failed: %v", err)
    }

    p.Shutdown() // nothing left in stock

    // The checked-out session is still retired by its request's Release,
    // but the batch must NOT be rebuilt during shutdown.
    p.Release(id)
    waitFor(t, "checked-out session deleted", 3*time.Second, func() bool {
        _, deleted := b.snapshot()
        return len(deleted) == 1 && deleted[0] == id
    })
    time.Sleep(50 * time.Millisecond) // give a wrongful refill a chance to show
    created, _ := b.snapshot()
    if len(created) != 1 {
        t.Fatalf("pool refilled during shutdown: created = %v", created)
    }
    if p.Ready() != 0 {
        t.Fatalf("ready = %d after shutdown release, want 0", p.Ready())
    }
}

func TestSessionPoolCreateRetry(t *testing.T) {
    b := &stubBackend{failCreates: 1} // first create fails, retry succeeds
    p := zbridge.NewSessionPool(b, 1)
    p.Start()

    // fillSlot backs off 1s after the failure, then stocks the retry.
    waitFor(t, "session stocked after create retry", 5*time.Second, func() bool {
        return p.Ready() == 1
    })
    created, _ := b.snapshot()
    if len(created) != 1 {
        t.Fatalf("created = %v, want exactly one successful session", created)
    }
    p.Shutdown()
}

// ── Z.AI chat delete client ─────────────────────────────────────────────────

// mockZAI points the bridge at a mock upstream with a pre-authenticated
// session and restores everything on cleanup.
func mockZAI(t *testing.T, baseURL string) {
    t.Helper()
    oldBase := zbridge.BASE_URL
    zbridge.BASE_URL = baseURL
    t.Cleanup(func() { zbridge.BASE_URL = oldBase })
    t.Cleanup(zbridge.OverrideSessionState("test-token", "test-user", true))
}

func TestDeleteZAIChat(t *testing.T) {
    type seen struct {
        method string
        path   string
        auth   string
    }
    var mu sync.Mutex
    var requests []seen

    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        mu.Lock()
        requests = append(requests, seen{r.Method, r.URL.Path, r.Header.Get("authorization")})
        mu.Unlock()
        switch {
        case strings.HasPrefix(r.URL.Path, "/api/v1/chats/ok-"):
            w.WriteHeader(200)
            w.Write([]byte("true"))
        case strings.HasPrefix(r.URL.Path, "/api/v1/chats/gone-"):
            // The live reply for a chat that no longer exists.
            w.WriteHeader(404)
            w.Write([]byte(`{"detail":"We could not find what you're looking for :/"}`))
        default:
            w.WriteHeader(500)
            w.Write([]byte(`{"detail":"boom"}`))
        }
    }))
    defer upstream.Close()
    mockZAI(t, upstream.URL)

    // Success: DELETE with the bearer token, "true" reply.
    if err := zbridge.DeleteZAIChat(context.Background(), "ok-123"); err != nil {
        t.Fatalf("DeleteZAIChat(ok) failed: %v", err)
    }
    // Already gone: idempotent success (the hint's second DELETE reply).
    if err := zbridge.DeleteZAIChat(context.Background(), "gone-456"); err != nil {
        t.Fatalf("DeleteZAIChat(gone) must be an idempotent success, got: %v", err)
    }
    // Server error surfaces as a failure.
    if err := zbridge.DeleteZAIChat(context.Background(), "bad-789"); err == nil {
        t.Fatal("DeleteZAIChat(bad) succeeded, want error")
    }
    // Empty ID is a no-op.
    if err := zbridge.DeleteZAIChat(context.Background(), ""); err != nil {
        t.Fatalf("DeleteZAIChat(\"\") failed: %v", err)
    }

    mu.Lock()
    defer mu.Unlock()
    if len(requests) != 3 {
        t.Fatalf("upstream saw %d requests, want 3 (empty ID must not hit the wire)", len(requests))
    }
    want := []seen{
        {"DELETE", "/api/v1/chats/ok-123", "Bearer test-token"},
        {"DELETE", "/api/v1/chats/gone-456", "Bearer test-token"},
        {"DELETE", "/api/v1/chats/bad-789", "Bearer test-token"},
    }
    for i, w := range want {
        if requests[i] != w {
            t.Errorf("request %d = %+v, want %+v", i, requests[i], w)
        }
    }
}

// ── bridge glue (sync-mode path) ────────────────────────────────────────────

func TestSyncModeAcquireReleaseDeletesUpstream(t *testing.T) {
    // No pool attached -> legacy per-request flow, still garbage-collected.
    restore := zbridge.AttachSessionPool(nil, 10*time.Second)
    defer restore()

    var mu sync.Mutex
    deletedPaths := []string{}
    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/v1/chats/") {
            mu.Lock()
            deletedPaths = append(deletedPaths, r.URL.Path)
            mu.Unlock()
            w.WriteHeader(200)
            w.Write([]byte("true"))
            return
        }
        http.NotFound(w, r)
    }))
    defer upstream.Close()
    mockZAI(t, upstream.URL)

    chatID, pooled, err := zbridge.AcquireStatelessSession(context.Background())
    if err != nil {
        t.Fatalf("AcquireStatelessSession failed: %v", err)
    }
    if pooled {
        t.Fatal("sync mode must not hand out pool-owned sessions")
    }
    if chatID == "" {
        t.Fatal("sync mode must mint a non-empty chat ID")
    }

    zbridge.ReleaseStatelessSession(chatID, pooled)

    // GC is fire-and-forget: poll for the DELETE to land upstream.
    waitFor(t, "GC to delete the used chat upstream", 5*time.Second, func() bool {
        mu.Lock()
        defer mu.Unlock()
        return len(deletedPaths) == 1 && deletedPaths[0] == "/api/v1/chats/"+chatID
    })
}

func TestAcquireStatelessSessionPoolBusyFallback(t *testing.T) {
    // A starved pool (batch exhausted, no refill coming) must fall back to
    // creating a session on demand after poolWait instead of stalling.
    b := &stubBackend{blockCreate: true} // warmup/refill never produce anything
    restore := zbridge.AttachSessionPool(zbridge.NewSessionPool(b, 1), 50*time.Millisecond)
    defer restore()

    start := time.Now()
    chatID, pooled, err := zbridge.AcquireStatelessSession(context.Background())
    if err != nil {
        t.Fatalf("AcquireStatelessSession failed: %v", err)
    }
    if pooled {
        t.Fatal("on-demand fallback session must not be pool-owned")
    }
    if chatID == "" {
        t.Fatal("on-demand fallback must mint a non-empty chat ID")
    }
    if elapsed := time.Since(start); elapsed > 3*time.Second {
        t.Fatalf("fallback took %s, want ~50ms poolWait", elapsed)
    }
}

func TestAcquireStatelessSessionFromPool(t *testing.T) {
    b := &stubBackend{}
    pool := zbridge.NewSessionPool(b, 2)
    restore := zbridge.AttachSessionPool(pool, time.Second)
    defer func() {
        pool.Shutdown()
        restore()
    }()
    pool.Start()

    waitFor(t, "warmup to stock 2 sessions", 3*time.Second, func() bool {
        return pool.Ready() == 2
    })

    chatID, pooled, err := zbridge.AcquireStatelessSession(context.Background())
    if err != nil {
        t.Fatalf("AcquireStatelessSession failed: %v", err)
    }
    if !pooled {
        t.Fatal("async mode must hand out pool-owned sessions")
    }

    // Release retires it through the pool: deleted upstream + refilled.
    // Batch was 2 with one checked out, so after release+refill it is full.
    zbridge.ReleaseStatelessSession(chatID, pooled)
    waitFor(t, "pool to retire + refill the session", 3*time.Second, func() bool {
        _, deleted := b.snapshot()
        return len(deleted) == 1 && deleted[0] == chatID && pool.Ready() == 2
    })
}

// ── end-to-end: handler draws a pooled session and deletes it after use ────

// TestHTTPChatCompletionsUsesPooledSessionThenDeletes drives the REAL HTTP
// surface (zbridge.NewHandler: routes + auth + CORS) with the async pool
// attached and a mock Z.AI upstream, proving the full throwaway-session
// loop:
//
//   warm pool -> handler acquires a pooled chat_id -> completion references
//   it upstream -> response fully written -> deferred release deletes the
//   chat on the mock (DELETE /api/v1/chats/{id}) -> pool refills the batch.
func TestHTTPChatCompletionsUsesPooledSessionThenDeletes(t *testing.T) {
    var mu sync.Mutex
    completionChatIDs := []string{}
    deletedChatIDs := []string{}

    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch {
        case r.URL.Path == "/api/v2/chat/completions":
            var reqBody struct {
                ChatID string `json:"chat_id"`
            }
            json.NewDecoder(r.Body).Decode(&reqBody)
            mu.Lock()
            completionChatIDs = append(completionChatIDs, reqBody.ChatID)
            mu.Unlock()
            w.Header().Set("Content-Type", "text/event-stream")
            flusher, _ := w.(http.Flusher)
            fmt.Fprintf(w, "data: {\"data\":{\"delta_content\":\"hello\"}}\n\n")
            fmt.Fprintf(w, "data: {\"data\":{\"phase\":\"done\"}}\n\n")
            fmt.Fprintf(w, "data: [DONE]\n\n")
            if flusher != nil {
                flusher.Flush()
            }
        case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/v1/chats/"):
            mu.Lock()
            deletedChatIDs = append(deletedChatIDs, strings.TrimPrefix(r.URL.Path, "/api/v1/chats/"))
            mu.Unlock()
            w.WriteHeader(200)
            w.Write([]byte("true"))
        default:
            http.NotFound(w, r)
        }
    }))
    defer upstream.Close()
    mockZAI(t, upstream.URL)

    // Bypass captcha via the agent-mode cache.
    cfg := zbridge.GetConfig()
    oldAgentMode := cfg.AgentMode
    cfg.AgentMode = true
    defer func() { cfg.AgentMode = oldAgentMode }()
    zbridge.SeedCaptchaParam("test-captcha-param")

    // Attach a real pool backed by the production Z.AI backend.
    pool := zbridge.NewSessionPool(zbridge.NewZAIChatBackend(), 2)
    restore := zbridge.AttachSessionPool(pool, time.Second)
    defer func() {
        pool.Shutdown()
        restore()
    }()
    pool.Start()
    waitFor(t, "warmup to stock 2 sessions", 3*time.Second, func() bool {
        return pool.Ready() == 2
    })

    body := `{"model":"glm-4.7","stream":false,"messages":[{"role":"user","content":"hi"}]}`
    req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec := httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)

    if rec.Code != 200 {
        t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
    }
    if !strings.Contains(rec.Body.String(), "hello") {
        t.Fatalf("response missing upstream content: %s", rec.Body.String())
    }

    mu.Lock()
    if len(completionChatIDs) != 1 {
        t.Fatalf("upstream saw %d completions, want 1", len(completionChatIDs))
    }
    usedID := completionChatIDs[0]
    mu.Unlock()
    if usedID == "" {
        t.Fatal("completion went upstream without a chat_id")
    }

    // After the response is fully written, the handler's deferred release
    // must delete the used chat upstream and refill the batch to full.
    waitFor(t, "used chat deleted upstream + pool refilled", 5*time.Second, func() bool {
        mu.Lock()
        defer mu.Unlock()
        return len(deletedChatIDs) == 1 && deletedChatIDs[0] == usedID && pool.Ready() == 2
    })
}
