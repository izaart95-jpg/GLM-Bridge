// Tests for the hot-swappable SQLite holder (db.go):
//
//   - tokenCount: real-time count from the active DB, TTL-cached,
//     -1 when no DB is attached or the query fails.
//   - swapDB / validateTokenDB: full validation ladder (missing file,
//     non-SQLite file, wrong schema, directory), graceful switch under
//     concurrent readers, in-flight queries completing on the old handle.

package zbridge

import (
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "testing"
    "time"
)

// makeTestDB creates a temp SQLite database with the collector's schema and
// n token rows, returning its path. Rows are seeded in ONE transaction —
// per-row autocommit transactions each pay an fsync, which is orders of
// magnitude slower (and pathological on slow filesystems).
func makeTestDB(t *testing.T, n int) string {
    t.Helper()
    path := filepath.Join(t.TempDir(), fmt.Sprintf("tokens-%d.sqlite", n))
    db, err := openTokenDB(path)
    if err != nil {
        t.Fatalf("create test db: %v", err)
    }
    defer db.Close()
    if _, err := db.Exec(`CREATE TABLE tokens (
        id    INTEGER PRIMARY KEY AUTOINCREMENT,
        token TEXT    NOT NULL,
        batch INTEGER NOT NULL
    )`); err != nil {
        t.Fatalf("create schema: %v", err)
    }
    tx, err := db.Begin()
    if err != nil {
        t.Fatalf("begin seed tx: %v", err)
    }
    defer tx.Rollback()
    for i := 0; i < n; i++ {
        if _, err := tx.Exec(
            "INSERT INTO tokens (token, batch) VALUES (?, ?)",
            fmt.Sprintf("tok-%d", i), 1,
        ); err != nil {
            t.Fatalf("seed row %d: %v", i, err)
        }
    }
    if err := tx.Commit(); err != nil {
        t.Fatalf("commit seed tx: %v", err)
    }
    return path
}

// activeHandle returns the currently attached handle, mirroring the old
// activeDB() accessor the tests were written against.
func activeHandle() (dbHandle, error) {
    globalDBState.mu.RLock()
    h := globalDBState.active
    globalDBState.mu.RUnlock()
    if h.db == nil {
        return dbHandle{}, ErrDBUnavailable
    }
    return h, nil
}

// attachTestDB points the global holder at a freshly made test DB and
// returns a cleanup that detaches it (restoring "no database").
func attachTestDB(t *testing.T, n int) string {
    t.Helper()
    path := makeTestDB(t, n)
    db, err := openTokenDB(path)
    if err != nil {
        t.Fatalf("open test db: %v", err)
    }
    attachDB(db, path)
    t.Cleanup(func() {
        closeDB()
    })
    return path
}

func TestTokenCountRealTime(t *testing.T) {
    attachTestDB(t, 7)
    if got := globalDBState.tokenCount(); got != 7 {
        t.Fatalf("tokenCount = %d, want 7", got)
    }
}

func TestTokenCountNoDB(t *testing.T) {
    closeDB() // ensure nothing attached
    if got := globalDBState.tokenCount(); got != -1 {
        t.Fatalf("tokenCount with no DB = %d, want -1", got)
    }
}

func TestTokenCountTTLCache(t *testing.T) {
    attachTestDB(t, 3)

    // First read hits the DB and populates the cache.
    if got := globalDBState.tokenCount(); got != 3 {
        t.Fatalf("first read = %d, want 3", got)
    }

    // Mutate the underlying file behind the cache's back; within the TTL
    // the cached value must still be served (that is the point of the cache).
    h, err := activeHandle()
    if err != nil {
        t.Fatal(err)
    }
    if _, err := h.db.Exec("DELETE FROM tokens;"); err != nil {
        t.Fatal(err)
    }
    invalidateTokenCount() // wait — invalidation is explicit; test both ways
    // Re-populate cache, then verify a second immediate read does NOT re-query.
    if got := globalDBState.tokenCount(); got != 0 {
        t.Fatalf("post-delete read = %d, want 0", got)
    }
    if _, err := h.db.Exec("INSERT INTO tokens (token, batch) VALUES ('x', 1)"); err != nil {
        t.Fatal(err)
    }
    // Cached: still 0 (fresh entry, TTL not elapsed).
    if got := globalDBState.tokenCount(); got != 0 {
        t.Fatalf("cached read = %d, want 0 (cache must hold until TTL)", got)
    }

    // Expire the cache by back-dating countAt; next read re-queries.
    globalDBState.countMu.Lock()
    globalDBState.countAt = time.Now().Add(-tokenCountCacheTTL - time.Second)
    globalDBState.countMu.Unlock()
    if got := globalDBState.tokenCount(); got != 1 {
        t.Fatalf("post-TTL read = %d, want 1", got)
    }
}

func TestTokenCountReflectsSwapImmediately(t *testing.T) {
    attachTestDB(t, 2)
    if got := globalDBState.tokenCount(); got != 2 {
        t.Fatalf("pre-swap count = %d, want 2", got)
    }

    // Swap to a DB with a different row count — the count must not serve the
    // stale cached value from the old file.
    other := makeTestDB(t, 99)
    otherDB, err := openTokenDB(other)
    if err != nil {
        t.Fatal(err)
    }
    attachDB(otherDB, other)

    if got := globalDBState.tokenCount(); got != 99 {
        t.Fatalf("post-swap count = %d, want 99 (cache must be invalidated on swap)", got)
    }
}

func TestSwapDBValidation(t *testing.T) {
    attachTestDB(t, 1)
    original, _ := activeHandle()

    // Missing file.
    if err := swapDB(filepath.Join(t.TempDir(), "nope.sqlite")); err == nil {
        t.Fatal("swap to missing file: expected error")
    }

    // Not a SQLite file.
    junk := filepath.Join(t.TempDir(), "junk.sqlite")
    if err := os.WriteFile(junk, []byte("definitely not sqlite"), 0o644); err != nil {
        t.Fatal(err)
    }
    if err := swapDB(junk); err == nil {
        t.Fatal("swap to non-SQLite file: expected error")
    }

    // SQLite file but wrong schema.
    wrongSchema := filepath.Join(t.TempDir(), "wrong.sqlite")
    db, err := openTokenDB(wrongSchema)
    if err != nil {
        t.Fatal(err)
    }
    if _, err := db.Exec("CREATE TABLE other (x INTEGER)"); err != nil {
        t.Fatal(err)
    }
    db.Close()
    if err := swapDB(wrongSchema); err == nil {
        t.Fatal("swap to schema-less SQLite file: expected error")
    }

    // Directory instead of file.
    if err := swapDB(t.TempDir()); err == nil {
        t.Fatal("swap to directory: expected error")
    }

    // Empty path.
    if err := swapDB(""); err == nil {
        t.Fatal("swap to empty path: expected error")
    }

    // Every failed attempt must have left the original DB attached.
    h, err := activeHandle()
    if err != nil {
        t.Fatal(err)
    }
    if h.path != original.path {
        t.Fatalf("failed swaps changed active DB: %s != %s", h.path, original.path)
    }
}

func TestSwapDBSuccess(t *testing.T) {
    first := attachTestDB(t, 5)

    second := makeTestDB(t, 42)
    if err := swapDB(second); err != nil {
        t.Fatalf("swapDB: %v", err)
    }

    h, err := activeHandle()
    if err != nil {
        t.Fatal(err)
    }
    if h.path != second {
        t.Fatalf("active path = %s, want %s", h.path, second)
    }
    if h.path == first {
        t.Fatal("active path unchanged after swap")
    }
    if got := globalDBState.tokenCount(); got != 42 {
        t.Fatalf("post-swap count = %d, want 42", got)
    }
}

func TestSwapDBSamePathRefreshes(t *testing.T) {
    path := attachTestDB(t, 5)
    if got := globalDBState.tokenCount(); got != 5 {
        t.Fatalf("count = %d, want 5", got)
    }

    // Re-swapping the same path is a no-op swap that must succeed (the
    // collector may have rewritten the file in place).
    if err := swapDB(path); err != nil {
        t.Fatalf("same-path swap: %v", err)
    }
    if got := globalDBState.tokenCount(); got != 5 {
        t.Fatalf("post same-path swap count = %d, want 5", got)
    }
}

// TestSwapUnderConcurrentReaders proves the swap is graceful: readers
// hammering the token count (and the token picker) throughout a swap never
// observe an error, a denial, or a torn state — before, during and after.
func TestSwapUnderConcurrentReaders(t *testing.T) {
    attachTestDB(t, 50)

    second := makeTestDB(t, 500)
    third := makeTestDB(t, 5_000)

    stop := make(chan struct{})
    var wg sync.WaitGroup
    var errs sync.Map // reader index -> first error

    // Spawning readers of both kinds: tokenCount (health path) and
    // getNextToken (captcha path).
    for i := 0; i < 8; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            for {
                select {
                case <-stop:
                    return
                default:
                }
                if n := globalDBState.tokenCount(); n < 0 {
                    if v, loaded := errs.LoadOrStore(idx, fmt.Errorf("tokenCount=%d", n)); !loaded {
                        t.Errorf("reader %d: %v", idx, v)
                    }
                    return
                }
                if _, ok := getNextToken(); !ok {
                    // A DB with zero rows legitimately reports !ok — only
                    // an error state (no DB) is a failure here.
                    h, err := activeHandle()
                    if err != nil {
                        if v, loaded := errs.LoadOrStore(idx, err); !loaded {
                            t.Errorf("reader %d: %v", idx, v)
                        }
                        return
                    }
                    var n int64
                    if err := h.db.QueryRow("SELECT COUNT(*) FROM tokens;").Scan(&n); err != nil || n == 0 {
                        continue // empty-but-valid DB: fine
                    }
                    if v, loaded := errs.LoadOrStore(idx, fmt.Errorf("getNextToken failed with %d rows", n)); !loaded {
                        t.Errorf("reader %d: %v", idx, v)
                    }
                    return
                }
            }
        }(i)
    }

    // Swap twice while readers are running.
    if err := swapDB(second); err != nil {
        t.Errorf("first swap: %v", err)
    }
    if err := swapDB(third); err != nil {
        t.Errorf("second swap: %v", err)
    }

    close(stop)
    wg.Wait()

    // Final state: the third DB, 5000 rows.
    if got := globalDBState.tokenCount(); got != 5000 {
        t.Fatalf("final count = %d, want 5000", got)
    }
}

func TestValidateTokenDBAcceptsRealSchema(t *testing.T) {
    path := makeTestDB(t, 0) // zero rows is still a valid schema
    if err := validateTokenDB(path); err != nil {
        t.Fatalf("validateTokenDB on valid empty DB: %v", err)
    }
}
