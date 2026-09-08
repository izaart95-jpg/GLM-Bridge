// Blackbox tests for the two operational endpoints added to the bridge:
//
//   GET  /health — now carries a real-time tokenCount from the active
//                  tokens.sqlite (TTL-cached, -1 when unavailable).
//   POST /sqlite — hot-swaps the active database without a restart:
//                  validates the candidate (exists / readable / SQLite /
//                  `tokens` schema), swaps atomically, throttles rather
//                  than denies concurrent traffic, and lets in-flight
//                  work finish on the old handle.
//
// Driven through the real HTTP surface (zbridge.NewHandler) exactly like
// the other blackbox tests in this package.

package tests

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "testing"

    "zai-api/internal/zbridge"
)

// makeSQLite creates a temp SQLite database with the collector's schema and
// n token rows, returning its path. Seeded in one transaction — per-row
// autocommit each pay an fsync.
func makeSQLite(t *testing.T, n int) string {
    t.Helper()
    path := filepath.Join(t.TempDir(), "tokens.sqlite")
    db, err := sql.Open("sqlite", path)
    if err != nil {
        t.Fatalf("open: %v", err)
    }
    defer db.Close()
    if _, err := db.Exec(`CREATE TABLE tokens (
        id    INTEGER PRIMARY KEY AUTOINCREMENT,
        token TEXT    NOT NULL,
        batch INTEGER NOT NULL
    )`); err != nil {
        t.Fatalf("schema: %v", err)
    }
    tx, err := db.Begin()
    if err != nil {
        t.Fatalf("begin: %v", err)
    }
    defer tx.Rollback()
    for i := 0; i < n; i++ {
        if _, err := tx.Exec("INSERT INTO tokens (token, batch) VALUES (?, ?)",
            fmt.Sprintf("tok-%d", i), 1); err != nil {
            t.Fatalf("seed: %v", err)
        }
    }
    if err := tx.Commit(); err != nil {
        t.Fatalf("commit: %v", err)
    }
    return path
}

// attach points the bridge at a freshly made database and detaches it on
// test cleanup.
func attach(t *testing.T, n int) string {
    t.Helper()
    path := makeSQLite(t, n)
    db, err := sql.Open("sqlite", path)
    if err != nil {
        t.Fatal(err)
    }
    zbridge.AttachDatabase(db, path)
    t.Cleanup(zbridge.DetachDatabase)
    return path
}

// healthySession marks the Z.AI session initialised so /health returns 200
// while we exercise the tokenCount / swap surface (the session itself is
// not under test here). Restored on cleanup.
func healthySession(t *testing.T) {
    t.Helper()
    restore := zbridge.OverrideSessionState("test-token", "test-user", true)
    t.Cleanup(restore)
}

func getHealth(t *testing.T) (int, map[string]interface{}) {
    t.Helper()
    req := httptest.NewRequest("GET", "/health", nil)
    rec := httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)
    var body map[string]interface{}
    if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
        t.Fatalf("health body not JSON: %v (%s)", err, rec.Body.String())
    }
    return rec.Code, body
}

func TestHealthReportsRealTimeTokenCount(t *testing.T) {
    attach(t, 12)
    healthySession(t)

    code, body := getHealth(t)
    if code != http.StatusOK {
        t.Fatalf("/health status = %d, want 200", code)
    }
    got, ok := body["tokenCount"]
    if !ok {
        t.Fatalf("/health missing tokenCount field: %v", body)
    }
    if got.(float64) != 12 {
        t.Fatalf("tokenCount = %v, want 12", got)
    }
    if body["mode"] != "direct" {
        t.Fatalf("mode = %v, want direct", body["mode"])
    }
}

func TestHealthTokenCountMinusOneWithoutDB(t *testing.T) {
    zbridge.DetachDatabase()
    t.Cleanup(zbridge.DetachDatabase)

    _, body := getHealth(t)
    got, ok := body["tokenCount"]
    if !ok {
        t.Fatalf("/health missing tokenCount field: %v", body)
    }
    if got.(float64) != -1 {
        t.Fatalf("tokenCount = %v, want -1 (no database attached)", got)
    }
}

func TestHealthTokenCountFollowsSwap(t *testing.T) {
    first := attach(t, 3)
    healthySession(t)
    if got := zbridge.TokenCount(); got != 3 {
        t.Fatalf("TokenCount on %s = %d, want 3", first, got)
    }

    // Swap to a DB with a different count; health must reflect it
    // immediately (swap invalidates the TTL cache).
    second := makeSQLite(t, 77)
    if err := zbridge.SwapDatabase(second); err != nil {
        t.Fatalf("SwapDatabase: %v", err)
    }
    if got := zbridge.ActiveDBPath(); got != second {
        t.Fatalf("ActiveDBPath = %s, want %s", got, second)
    }
    _, body := getHealth(t)
    if got := body["tokenCount"].(float64); got != 77 {
        t.Fatalf("post-swap tokenCount = %v, want 77", got)
    }
}

func TestSQLiteSwapEndpoint(t *testing.T) {
    attach(t, 5)
    cfg := zbridge.GetConfig()
    healthySession(t)

    second := makeSQLite(t, 25)

    // Wrong method (auth still required — the middleware runs first).
    req := httptest.NewRequest("GET", "/sqlite", nil)
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec := httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)
    if rec.Code != http.StatusMethodNotAllowed {
        t.Fatalf("GET /sqlite status = %d, want 405 (body=%s)", rec.Code, rec.Body.String())
    }

    // Missing db_path.
    req = httptest.NewRequest("POST", "/sqlite", strings.NewReader(`{}`))
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec = httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)
    if rec.Code != http.StatusBadRequest {
        t.Fatalf("empty-body status = %d, want 400 (%s)", rec.Code, rec.Body.String())
    }

    // Invalid JSON.
    req = httptest.NewRequest("POST", "/sqlite", strings.NewReader(`not json`))
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec = httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)
    if rec.Code != http.StatusBadRequest {
        t.Fatalf("bad-json status = %d, want 400", rec.Code)
    }

    // Nonexistent file.
    req = httptest.NewRequest("POST", "/sqlite",
        strings.NewReader(`{"db_path":"/nonexistent/path/tokens.sqlite"}`))
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec = httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)
    if rec.Code != http.StatusBadRequest {
        t.Fatalf("missing-file status = %d, want 400", rec.Code)
    }

    // Valid swap through the full HTTP surface.
    req = httptest.NewRequest("POST", "/sqlite",
        strings.NewReader(fmt.Sprintf(`{"db_path":%q}`, second)))
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec = httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)
    if rec.Code != http.StatusOK {
        t.Fatalf("valid swap status = %d, body=%s", rec.Code, rec.Body.String())
    }
    var resp struct {
        Success    bool   `json:"success"`
        DBPath     string `json:"db_path"`
        TokenCount int64  `json:"token_count"`
    }
    if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
        t.Fatal(err)
    }
    if !resp.Success || resp.DBPath != second {
        t.Fatalf("swap response = %+v, want success+path %s", resp, second)
    }
    if resp.TokenCount != 25 {
        t.Fatalf("swap token_count = %d, want 25", resp.TokenCount)
    }
    if got := zbridge.ActiveDBPath(); got != second {
        t.Fatalf("ActiveDBPath = %s, want %s", got, second)
    }
}

func TestSQLiteSwapRejectsNonSQLiteFile(t *testing.T) {
    attach(t, 1)
    cfg := zbridge.GetConfig()
    original := zbridge.ActiveDBPath()

    junk := filepath.Join(t.TempDir(), "junk.sqlite")
    if err := os.WriteFile(junk, []byte("this is not a database"), 0o644); err != nil {
        t.Fatal(err)
    }
    req := httptest.NewRequest("POST", "/sqlite",
        strings.NewReader(fmt.Sprintf(`{"db_path":%q}`, junk)))
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec := httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)
    if rec.Code != http.StatusBadRequest {
        t.Fatalf("non-sqlite swap status = %d, want 400 (%s)", rec.Code, rec.Body.String())
    }
    // Rejected swap must leave the active DB untouched.
    if got := zbridge.ActiveDBPath(); got != original {
        t.Fatalf("rejected swap changed active DB: %s != %s", got, original)
    }
}

func TestSQLiteSwapRequiresAuth(t *testing.T) {
    cfg := zbridge.GetConfig()
    if !cfg.Auth.Enabled {
        t.Skip("auth disabled")
    }
    attach(t, 1)
    path := zbridge.ActiveDBPath()

    req := httptest.NewRequest("POST", "/sqlite",
        strings.NewReader(fmt.Sprintf(`{"db_path":%q}`, path)))
    // No Authorization header.
    rec := httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)
    if rec.Code != http.StatusUnauthorized {
        t.Fatalf("unauthenticated swap status = %d, want 401", rec.Code)
    }
}

// TestSQLiteSwapThrottlesNotDenies proves the workflow contract: while a
// swap runs, health probes are THROTTLED (they wait), never denied — and
// they observe either the old or the new database, never a torn state.
func TestSQLiteSwapThrottlesNotDenies(t *testing.T) {
    first := attach(t, 100)
    second := makeSQLite(t, 200)
    healthySession(t)

    var wg sync.WaitGroup
    stop := make(chan struct{})
    failures := make(chan string, 64)

    // Health probes hammering /health continuously across the swap.
    for i := 0; i < 8; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                select {
                case <-stop:
                    return
                default:
                }
                req := httptest.NewRequest("GET", "/health", nil)
                rec := httptest.NewRecorder()
                zbridge.NewHandler().ServeHTTP(rec, req)
                if rec.Code != http.StatusOK {
                    select {
                    case failures <- fmt.Sprintf("probe denied mid-swap: %d %s", rec.Code, rec.Body.String()):
                    default:
                    }
                    return
                }
                var body struct {
                    TokenCount int64 `json:"tokenCount"`
                }
                if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
                    select {
                    case failures <- "probe body: " + err.Error():
                    default:
                    }
                    return
                }
                // Throttled means: value is either the old DB's count or the
                // new one's — anything else (errors, -1) is a torn state.
                if body.TokenCount != 100 && body.TokenCount != 200 {
                    select {
                    case failures <- fmt.Sprintf("torn probe value: %d", body.TokenCount):
                    default:
                    }
                    return
                }
            }
        }()
    }

    if err := zbridge.SwapDatabase(second); err != nil {
        t.Errorf("swap: %v", err)
    }

    close(stop)
    wg.Wait()
    close(failures)
    for f := range failures {
        t.Error(f)
    }

    if got := zbridge.ActiveDBPath(); got != second || got == first {
        t.Fatalf("post-swap active = %s, want %s", got, second)
    }
}
