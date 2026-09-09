// Hot-swappable SQLite device-token database holder.
//
// The bridge consumes tokens (rows of the `tokens` table in tokens.sqlite)
// on the captcha path. Historically the database was opened once at startup
// (globalDB) and never changed — the only way to point the server at a
// freshly harvested file was to restart it, dropping every in-flight
// request.
//
// This file replaces that global with a small holder that can atomically
// swap the live database under load:
//
//   - mu (RWMutex): readers take the FULL write lock just long enough to
//     capture the active handle AND register it as "in flight" — the
//     registration is a map write, so a read lock would let two readers
//     write the map at once ("fatal error: concurrent map writes", the
//     agent-mode startup crash in issue #40) — then they run their query
//     outside the lock. swapDB takes the same write lock, installs the
//     new handle, then WAITS for the captured in-flight count on the
//     retired handle to fall to zero before closing it. In-flight
//     queries are never interrupted, never denied, and never see a
//     closed handle.
//   - swapGate: a buffered(1) semaphore serialising swaps themselves, so
//     two concurrent POST /sqlite requests cannot interleave.
//   - token-count cache: /health reads the count through a TTL cache
//     (default 5 s, TOKEN_COUNT_TTL_MS) so a health probe never turns
//     into a per-request DB query.
//
// Requests arriving DURING a swap are throttled — they block for the
// few microseconds the write lock is held, then see the new database —
// exactly the "throttle, don't deny, keep active requests" contract the
// swap workflow requires.
//
// Schema expectation (mirrors cmd/token-collector):
//
//	CREATE TABLE tokens (
//	    id    INTEGER PRIMARY KEY AUTOINCREMENT,
//	    token TEXT    NOT NULL,
//	    batch INTEGER NOT NULL
//	);

package zbridge

import (
    "database/sql"
    "errors"
    "fmt"
    "os"
    "strconv"
    "sync"
    "time"

    _ "modernc.org/sqlite" // pure-Go SQLite driver registration (no CGO)
)

// ============================================================================
// CONFIG
// ============================================================================

// tokenCountCacheTTL bounds how long a cached token count is trusted on
// /health. Default 5 s; overridable via TOKEN_COUNT_TTL_MS (parsed once at
// package init; values <= 0 keep the default).
var tokenCountCacheTTL = defaultTokenCountTTL

const defaultTokenCountTTL = 5 * time.Second

func init() {
    if raw := os.Getenv("TOKEN_COUNT_TTL_MS"); raw != "" {
        if ms, err := strconv.Atoi(raw); err == nil && ms > 0 {
            tokenCountCacheTTL = time.Duration(ms) * time.Millisecond
        }
    }
}

// ============================================================================
// DB HOLDER
// ============================================================================

// dbHandle couples a live sql.DB with the path it was opened from. The path
// is kept so health/swap logging can name files and so per-handle in-flight
// tracking can key off the struct.
type dbHandle struct {
    db   *sql.DB
    path string
}

// dbState is the single source of truth for the active database.
type dbState struct {
    // mu guards `active` and the per-handle in-flight counters. Readers
    // (withTokenDB, tokenCount) take the full write lock only while
    // capturing the handle + bumping its in-flight counter — the bump is
    // a map write, so it cannot share a read lock with another reader
    // (issue #40) — and the query itself runs lock-free. swapDB holds the
    // write lock for the pointer swap, then drains the retired handle
    // outside mu.
    mu     sync.RWMutex
    active dbHandle

    // inflight counts in-flight queries per live handle. EVERY access —
    // read or write — happens under the full write lock: acquires are map
    // writes, and a write lock is required to serialize them (issue #40).
    inflight map[*sql.DB]int
    // drained is signalled when a handle's inflight count hits 0 so
    // drainAndClose can wake up and retire the handle.
    drained *sync.Cond

    swapGate chan struct{} // buffered(1): only one swap at a time

    countMu   sync.Mutex
    count     int64  // last cached token count (-1 = unknown/unavailable)
    countAt   time.Time
    countPath string // which db file the cached count belongs to

    swaps uint64 // completed swaps (observability)
}

// globalDBState is the process-wide active database.
var globalDBState = &dbState{
    swapGate: make(chan struct{}, 1),
    count:    -1,
}

// ErrDBUnavailable reports that no usable database is attached (startup
// failed, or every swap attempt so far has been rejected).
var ErrDBUnavailable = errors.New("no active sqlite database")

func init() {
    globalDBState.inflight = make(map[*sql.DB]int)
    globalDBState.drained = sync.NewCond(&globalDBState.mu)
}

// ============================================================================
// OPEN / VALIDATE
// ============================================================================

// openTokenDB opens path as a SQLite database with the same single-connection
// settings the bridge has always used (SQLite serialises writes internally;
// one connection avoids "database is locked" and pool overhead).
func openTokenDB(path string) (*sql.DB, error) {
    db, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, err
    }
    db.SetMaxOpenConns(1)
    db.SetMaxIdleConns(1)
    db.SetConnMaxLifetime(0)
    // Force the connection open so a bad file fails here, not on first query.
    if err := db.Ping(); err != nil {
        db.Close()
        return nil, err
    }
    return db, nil
}

// validateTokenDB checks that path is a usable SQLite file carrying the
// expected `tokens` schema. It opens its own short-lived connection so the
// check can never disturb (or be disturbed by) queries on the active one.
func validateTokenDB(path string) error {
    if path == "" {
        return errors.New("db_path is empty")
    }
    fi, err := os.Stat(path)
    if err != nil {
        return fmt.Errorf("database file not accessible: %w", err)
    }
    if fi.IsDir() {
        return fmt.Errorf("db_path is a directory, not a file: %s", path)
    }
    if fi.Mode()&0400 == 0 {
        return fmt.Errorf("database file is not readable: %s", path)
    }

    db, err := openTokenDB(path)
    if err != nil {
        return fmt.Errorf("not a valid SQLite database: %w", err)
    }
    defer db.Close()

    // A missing tokens table is the classic "wrong file" failure — validate
    // the schema explicitly rather than letting the first captcha request
    // die far away from the swap that caused it.
    var name string
    err = db.QueryRow(
        "SELECT name FROM sqlite_master WHERE type='table' AND name='tokens';",
    ).Scan(&name)
    if errors.Is(err, sql.ErrNoRows) {
        return errors.New("schema mismatch: table 'tokens' not found")
    }
    if err != nil {
        return fmt.Errorf("schema check failed: %w", err)
    }

    // Prove the column shape too — a `tokens` table without a `token`
    // column would pass every check above and still break lookups.
    var count int
    err = db.QueryRow(
        "SELECT COUNT(*) FROM pragma_table_info('tokens') WHERE name = 'token';",
    ).Scan(&count)
    if err != nil {
        return fmt.Errorf("column check failed: %w", err)
    }
    if count == 0 {
        return errors.New("schema mismatch: table 'tokens' has no 'token' column")
    }
    return nil
}

// ============================================================================
// ATTACH / CAPTURE / RELEASE
// ============================================================================

// attachDB installs db as the active database without validation. It is the
// primitive both startup (initDB) and tests build on.
func attachDB(db *sql.DB, path string) {
    s := globalDBState
    s.mu.Lock()
    old := s.active
    s.active = dbHandle{db: db, path: path}
    s.mu.Unlock()
    invalidateTokenCount()
    // Startup attach replaces nothing in a healthy process; if it does
    // (tests), drain politely rather than closing under a live reader.
    if old.db != nil && old.db != db {
        go drainAndClose(s, old.db)
    }
}

// acquireDB captures the active handle and registers the caller as in
// flight on it. The returned release function MUST be called when the
// query is done.
//
// The registration (s.inflight[h.db]++) is a MAP WRITE, so it must hold
// the full write lock — under a read lock two concurrent readers (the
// parallel captcha generators agent mode spawns at startup) both write
// the map and the process dies with "fatal error: concurrent map writes"
// (issue #40). The critical section is a pointer read + map bump, so the
// write lock costs nothing; the query itself still runs lock-free.
func (s *dbState) acquireDB() (dbHandle, func(), error) {
    s.mu.Lock()
    h := s.active
    if h.db == nil {
        s.mu.Unlock()
        return dbHandle{}, nil, ErrDBUnavailable
    }
    s.inflight[h.db]++
    s.mu.Unlock()

    release := func() {
        s.mu.Lock()
        if n, ok := s.inflight[h.db]; ok {
            if n <= 1 {
                delete(s.inflight, h.db)
            } else {
                s.inflight[h.db] = n - 1
            }
            s.drained.Broadcast()
        }
        s.mu.Unlock()
    }
    return h, release, nil
}

// drainAndClose waits until no query is in flight on db, then closes it.
// Called by swapDB (synchronously) and attachDB (background) so a retired
// handle is never closed while a reader still holds it.
func drainAndClose(s *dbState, db *sql.DB) {
    s.mu.Lock()
    for s.inflight[db] > 0 {
        s.drained.Wait()
    }
    s.mu.Unlock()
    db.Close()
}

// withTokenDB runs fn against the currently active database. The handle is
// captured and registered as in flight under the write lock (capture and
// registration are one atomic step, so a concurrent swap can never retire a
// handle between them), then fn runs without any lock held — so a swap
// cannot pull the database out from under it, and the query completes
// against the file that was active when it started. This is the throttle
// point required by the swap notes: during a swap, callers briefly wait on
// the mutex instead of failing.
func withTokenDB(fn func(h dbHandle) error) error {
    h, release, err := globalDBState.acquireDB()
    if err != nil {
        return err
    }
    defer release()
    return fn(h)
}

// ============================================================================
// TOKEN COUNT (TTL-cached, /health)
// ============================================================================

// tokenCount returns the number of rows in the active `tokens` table, served
// from the TTL cache when fresh. It returns -1 when the database cannot be
// queried (inaccessible file, schema gone, no handle attached) — the health
// endpoint's documented signal for "unknown".
func (s *dbState) tokenCount() int64 {
    s.mu.RLock()
    h := s.active
    s.mu.RUnlock()

    s.countMu.Lock()
    fresh := h.db != nil &&
        !s.countAt.IsZero() &&
        time.Since(s.countAt) < tokenCountCacheTTL &&
        s.countPath == h.path &&
        s.count >= 0
    s.countMu.Unlock()

    if fresh {
        s.countMu.Lock()
        c := s.count
        s.countMu.Unlock()
        return c
    }

    if h.db == nil {
        return -1
    }

    var n int64
    h, err := withHandleForCount(s, &n)
    if err != nil {
        return -1
    }

    s.countMu.Lock()
    s.count = n
    s.countAt = time.Now()
    s.countPath = h.path
    s.countMu.Unlock()
    return n
}

// withHandleForCount captures the active handle, registers it in flight,
// and runs the COUNT query on it. Capture + registration are one atomic
// step under the write lock — a concurrent swap may retire (but never
// close) the handle this count runs on, and the handle actually queried
// is returned so the caller caches the count under the right path. Same
// map-write-under-write-lock rule as acquireDB (issue #40).
func withHandleForCount(s *dbState, out *int64) (dbHandle, error) {
    s.mu.Lock()
    h := s.active
    if h.db == nil {
        s.mu.Unlock()
        return dbHandle{}, ErrDBUnavailable
    }
    s.inflight[h.db]++
    s.mu.Unlock()

    defer func() {
        s.mu.Lock()
        if n, ok := s.inflight[h.db]; ok {
            if n <= 1 {
                delete(s.inflight, h.db)
            } else {
                s.inflight[h.db] = n - 1
            }
            s.drained.Broadcast()
        }
        s.mu.Unlock()
    }()

    return h, h.db.QueryRow("SELECT COUNT(*) FROM tokens;").Scan(out)
}

// invalidateTokenCount drops the cached count so the next reader queries the
// (possibly new) database instead of reporting a stale number.
func invalidateTokenCount() {
    globalDBState.countMu.Lock()
    globalDBState.count = -1
    globalDBState.countAt = time.Time{}
    globalDBState.countPath = ""
    globalDBState.countMu.Unlock()
}

// ============================================================================
// HOT SWAP
// ============================================================================

// swapDB atomically replaces the active database with the one at newPath.
//
// Graceful by construction:
//
//  1. The file is validated first (exists, readable, SQLite, `tokens`
//     schema) using a private connection — a rejected candidate never
//     touches the live database.
//  2. The new sql.DB is opened and pinged BEFORE anything live is touched,
//     so the window in which the server could be left without a database
//     is exactly one pointer swap.
//  3. In-flight queries are not interrupted: they hold their own captured
//     handle and complete against the old file. swapDB waits for their
//     in-flight count to drain, then closes the retired handle — a query
//     never observes a closed DB.
//  4. Queries that arrive DURING the swap are throttled (they block for
//     the microseconds the write lock is held) and then see the new
//     database. Nothing is denied or failed mid-swap.
func swapDB(newPath string) error {
    // Validate before touching anything live.
    if err := validateTokenDB(newPath); err != nil {
        return err
    }

    // Open + ping the candidate so the live swap itself is instant.
    newDB, err := openTokenDB(newPath)
    if err != nil {
        return fmt.Errorf("failed to open new database: %w", err)
    }

    s := globalDBState

    // Serialise swaps: one at a time; concurrent POSTs get an explicit,
    // retryable error rather than an interleaved half-swap.
    select {
    case s.swapGate <- struct{}{}:
    default:
        newDB.Close()
        return errors.New("a database swap is already in progress — retry shortly")
    }
    defer func() { <-s.swapGate }()

    // The swap proper: one write-locked pointer exchange.
    s.mu.Lock()
    old := s.active
    s.active = dbHandle{db: newDB, path: newPath}
    s.swaps++
    s.mu.Unlock()
    invalidateTokenCount()

    // Graceful drain: retire the old handle only after every in-flight
    // query using it has finished. New queries go to the new handle, so
    // this cannot deadlock — the count is strictly decreasing.
    drainAndClose(s, old.db)

    logInfo(fmt.Sprintf("[DB Swap] active database is now %s (previous: %s)", newPath, old.path))
    return nil
}

// ============================================================================
// BRIDGE-FACING ACCESSORS (startup / captcha path)
// ============================================================================

// initDB opens dbPath and attaches it as the active database. Kept under its
// historical name so run.go's startup flow is unchanged.
func initDB() error {
    if _, err := os.Stat(dbPath); err != nil {
        return err
    }
    db, err := openTokenDB(dbPath)
    if err != nil {
        return err
    }
    attachDB(db, dbPath)
    return nil
}

// closeDB closes the active database if one is attached (shutdown path).
// Drains in-flight queries first so a graceful shutdown also retires the
// handle politely.
func closeDB() {
    s := globalDBState
    s.mu.Lock()
    h := s.active
    s.active = dbHandle{}
    s.mu.Unlock()
    invalidateTokenCount()
    if h.db != nil {
        drainAndClose(s, h.db)
    }
}
