// captcha.go
//
// Optimized build commands (pick one):
//
//   Linux/macOS (modern CPU, fully static, stripped):
//     CGO_ENABLED=0 GOAMD64=v3 go build -ldflags="-s -w" -trimpath -o token-collector captcha.go
//
//   Windows PowerShell:
//     $env:CGO_ENABLED=0; $env:GOAMD64="v3"; go build -ldflags="-s -w" -trimpath -o token-collector.exe captcha.go
//
//   Portable fallback (any CPU / any OS):
//     go build -ldflags="-s -w" -trimpath -o token-collector captcha.go
//
// Usage:
//   ./token-collector                  # interactive prompts
//   ./token-collector --unsafe         # 1500 tokens, 25 batches max
//   ./token-collector --tokens 750 --batch 3
//   ./token-collector --headed         # visible browser for debugging

package main

import (
    "bufio"
    "database/sql"
    "flag"
    "fmt"
    "os"
    "path/filepath"
    "runtime/debug"
    "strconv"
    "strings"
    "sync"
    "sync/atomic"
    "time"

    "github.com/mxschmitt/playwright-go"
    _ "modernc.org/sqlite" // pure-Go SQLite, no CGO needed
)

// ---------- Configuration ----------
const (
    MaxTokens                = 1250
    UnsafeMaxTokens          = 1500
    DefaultTokens            = 750
    DefaultBatch             = 3
    MaxBatch                 = 9
    UnsafeMaxBatch           = 25
    SendWaitMs               = 7000
    MaxRetries               = 3
    TokenCollectionTimeoutMs = 90000
    URL                      = "https://chat.z.ai"

    // Parallel workers = parallel PAGES on a single browser (not parallel browsers)
    MaxParallel       = 3
    UnsafeMaxParallel = 5
)

// ---------- Flags ----------
var (
    unsafeFlag   = flag.Bool("unsafe", false, "increase token limit to 1500 and batch limit to 25")
    tokensFlag   = flag.Int("tokens", 0, "tokens per batch (0 = prompt)")
    batchFlag    = flag.Int("batch", 0, "number of batches (0 = prompt)")
    headedFlag   = flag.Bool("headed", false, "show browser window for debugging")
    parallelFlag = flag.Int("parallel", 0, "parallel workers (pages) on a single browser; 0 = prompt y/N")
)

// ---------- init: tune GC for throughput ----------
// Default Go GC runs at 100% (doubles heap before collecting).
// Bumping to 200% lets the heap grow 3× before collecting, cutting GC
// pauses by ~half in allocation-heavy workloads like token collection.
func init() {
    debug.SetGCPercent(200)
}

// ---------- Fast sleep ----------
func sleep(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }

// ---------- Prompt user for integer ----------
func promptInt(reader *bufio.Reader, prompt string, def, max int) int {
    fmt.Print(prompt)
    line, err := reader.ReadString('\n')
    if err != nil {
        return def
    }
    line = strings.TrimSpace(line)
    if line == "" {
        return def
    }
    n, err := strconv.Atoi(line)
    if err != nil || n <= 0 {
        fmt.Printf("⚠️  Invalid input, using default %d.\n", def)
        return def
    }
    if n > max {
        fmt.Printf("⚠️  Capping to max %d.\n", max)
        return max
    }
    return n
}

// ---------- Prompt user for y/N ----------
func promptBool(reader *bufio.Reader, prompt string, def bool) bool {
    fmt.Print(prompt)
    line, err := reader.ReadString('\n')
    if err != nil {
        return def
    }
    line = strings.TrimSpace(strings.ToLower(line))
    if line == "" {
        return def
    }
    return line == "y" || line == "yes"
}

// ---------- Persistent SQLite store with tuned PRAGMAs ----------
// Opening/closing the DB per batch (as the original did) forces a full
// fsync and reparse of the schema every time. Keeping one connection
// open with WAL mode + large cache eliminates that overhead entirely.
type tokenStore struct {
    db   *sql.DB
    stmt *sql.Stmt
    mu   sync.Mutex // serialise writes (SQLite is single-writer)
}

func openTokenStore(dbPath string) (*tokenStore, error) {
    dsn := "file:" + filepath.ToSlash(dbPath) +
        "?_pragma=busy_timeout(10000)" +
        "&_pragma=journal_mode(WAL)" +
        "&_pragma=synchronous(NORMAL)" +
        "&_pragma=cache_size(-65536)" + // 64 MB page cache
        "&_pragma=temp_store(MEMORY)" +
        "&_pragma=mmap_size(268435456)" + // 256 MB mmap
        "&_pragma=wal_autocheckpoint(1000)"

    db, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, err
    }
    // SQLite serialises writes internally — one connection avoids
    // "database is locked" errors and avoids pool overhead.
    db.SetMaxOpenConns(1)
    db.SetMaxIdleConns(1)
    db.SetConnMaxLifetime(0)

    // Force the connection open so PRAGMAs take effect immediately.
    if err := db.Ping(); err != nil {
        db.Close()
        return nil, err
    }

    if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS tokens (
        id    INTEGER PRIMARY KEY AUTOINCREMENT,
        token TEXT    NOT NULL,
        batch INTEGER NOT NULL
    )`); err != nil {
        db.Close()
        return nil, err
    }
    // Index for fast batch lookups
    if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_tokens_batch ON tokens(batch)`); err != nil {
        db.Close()
        return nil, err
    }

    stmt, err := db.Prepare(`INSERT INTO tokens (token, batch) VALUES (?, ?)`)
    if err != nil {
        db.Close()
        return nil, err
    }
    return &tokenStore{db: db, stmt: stmt}, nil
}

func (ts *tokenStore) merge(batchNum int, tokens []string) error {
    ts.mu.Lock()
    defer ts.mu.Unlock()

    tx, err := ts.db.Begin()
    if err != nil {
        return err
    }
    // Rollback is a no-op after Commit, so this is safe.
    defer tx.Rollback()

    // Bind the prepared statement to this transaction.
    txStmt := tx.Stmt(ts.stmt)
    defer txStmt.Close()

    for _, tok := range tokens {
        if _, err := txStmt.Exec(tok, batchNum); err != nil {
            return err
        }
    }
    return tx.Commit()
}

func (ts *tokenStore) close() {
    ts.stmt.Close()
    ts.db.Close()
}

// ---------- Collect tokens on a single page ----------
func collectTokensOnPage(page playwright.Page, total int) ([]string, error) {
    if _, err := page.Goto(URL, playwright.PageGotoOptions{
        WaitUntil: playwright.WaitUntilStateDomcontentloaded,
        Timeout:   playwright.Float(60000),
    }); err != nil {
        return nil, fmt.Errorf("goto: %w", err)
    }

    // Wait for both elements concurrently
    fmt.Println("  Locating UI elements in parallel...")
    var (
        err1, err2 error
        wg         sync.WaitGroup
    )
    wg.Add(2)
    go func() {
        defer wg.Done()
        err1 = page.Locator("#model-selector-glm-4_7-button").WaitFor(
            playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)},
        )
    }()
    go func() {
        defer wg.Done()
        err2 = page.Locator("#chat-input").WaitFor(
            playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)},
        )
    }()
    wg.Wait()

    if err1 != nil {
        return nil, fmt.Errorf("model button not found: %w", err1)
    }
    if err2 != nil {
        return nil, fmt.Errorf("textarea not found: %w", err2)
    }
    fmt.Println("✅ Model button & textarea found")

    textarea := page.Locator("#chat-input")
    if err := textarea.Fill("__"); err != nil {
        return nil, fmt.Errorf("fill textarea: %w", err)
    }
    fmt.Println(`✅ Textarea filled with "__"`)

    sendBtn := page.Locator("#send-message-button")
    if err := sendBtn.WaitFor(
        playwright.LocatorWaitForOptions{Timeout: playwright.Float(5000)},
    ); err != nil {
        return nil, fmt.Errorf("send button not found: %w", err)
    }
    if err := sendBtn.Click(); err != nil {
        return nil, fmt.Errorf("click send: %w", err)
    }
    fmt.Println("✅ Send clicked")

    fmt.Printf("⏳ Waiting %dms for token endpoint to initialize...\n", SendWaitMs)
    sleep(SendWaitMs)

    // ---------- Fast token collection with timeout ----------
    fmt.Println("🚀 Collecting tokens...")
    t0 := time.Now()

    type evalResult struct {
        val interface{}
        err error
    }
    resultCh := make(chan evalResult, 1)

    go func() {
        val, err := page.Evaluate(`async (args) => {
            const total = args.total;
            const out = new Array(total);
            for (let i = 0; i < total; i++) {
                const tok = window.z_um.getToken();
                out[i] = (tok && typeof tok.then === 'function') ? await tok : tok;
                if (i % 50 === 0) {
                    await new Promise(r => setTimeout(r, 0));
                }
            }
            return out;
        }`, map[string]interface{}{"total": total})
        resultCh <- evalResult{val, err}
    }()

    select {
    case res := <-resultCh:
        if res.err != nil {
            return nil, fmt.Errorf("evaluate: %w", res.err)
        }
        arr, ok := res.val.([]interface{})
        if !ok {
            return nil, fmt.Errorf("unexpected evaluate result type: %T", res.val)
        }
        // Pre-allocate with exact capacity — avoids slice growth reallocations.
        tokens := make([]string, 0, len(arr))
        for _, v := range arr {
            if s, ok := v.(string); ok {
                tokens = append(tokens, s)
            } else if v != nil {
                tokens = append(tokens, fmt.Sprintf("%v", v))
            }
        }
        elapsed := time.Since(t0).Seconds()
        fmt.Printf("✅ Collected %d tokens in %.2fs\n", len(tokens), elapsed)
        return tokens, nil

    case <-time.After(TokenCollectionTimeoutMs * time.Millisecond):
        return nil, fmt.Errorf("⏱️ token collection timed out after %ds", TokenCollectionTimeoutMs/1000)
    }
}

// ---------- Run a single batch with retries ----------
func runBatch(browser playwright.Browser, total, batchNum int) ([]string, error) {
    var lastErr error
    for attempt := 1; attempt <= MaxRetries; attempt++ {
        fmt.Printf("\n🔄 [Batch %d] Attempt %d of %d\n", batchNum, attempt, MaxRetries)

        page, err := browser.NewPage()
        if err != nil {
            lastErr = err
            continue
        }

        tokens, err := collectTokensOnPage(page, total)

        if cerr := page.Close(); cerr != nil {
            fmt.Printf("⚠️  page close error: %v\n", cerr)
        }

        if err != nil {
            lastErr = err
            fmt.Printf("❌ Attempt %d failed: %v\n", attempt, err)
            if attempt == MaxRetries {
                fmt.Fprintln(os.Stderr, "🚫 All retries exhausted.")
                break
            }
            fmt.Println("♻️  Retrying with a fresh page load...")
            continue
        }
        return tokens, nil
    }
    return nil, fmt.Errorf("batch %d failed: %w", batchNum, lastErr)
}

// ---------- Run batches in PARALLEL using N pages on a single browser ----------
// Uses lock-free atomics for the abort flag and running total — eliminates
// two mutex lock/unlock pairs per batch that were pure overhead.
func runParallel(browser playwright.Browser, tokenCount, batchCount, workers int, ts *tokenStore, dbPath string) (int, error) {
    var (
        aborted  atomic.Bool
        totalCol atomic.Int64
        wg       sync.WaitGroup
        once     sync.Once
        firstErr error
    )

    batchCh := make(chan int, batchCount)
    for b := 1; b <= batchCount; b++ {
        batchCh <- b
    }
    close(batchCh)

    for w := 1; w <= workers; w++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            for batchNum := range batchCh {
                // Lock-free check — no mutex contention.
                if aborted.Load() {
                    return
                }

                fmt.Printf("\n👷 [Worker %d] starting batch %d\n", workerID, batchNum)

                tokens, err := runBatch(browser, tokenCount, batchNum)
                if err != nil {
                    once.Do(func() {
                        firstErr = err
                        aborted.Store(true)
                    })
                    return
                }

                dbErr := ts.merge(batchNum, tokens)
                if dbErr != nil {
                    once.Do(func() {
                        firstErr = fmt.Errorf("database merge: %w", dbErr)
                        aborted.Store(true)
                    })
                    return
                }

                // Lock-free atomic add — no mutex.
                cur := totalCol.Add(int64(len(tokens)))

                fmt.Printf("✅ [Worker %d] batch %d done — %d tokens (running total: %d)\n",
                    workerID, batchNum, len(tokens), cur)
            }
        }(w)
    }
    wg.Wait()
    return int(totalCol.Load()), firstErr
}

// ---------- Browser launch args for maximum throughput ----------
// Disables background throttling, unnecessary services, and automation
// detection — keeps the renderer hot and avoids IPC storms.
var chromiumPerfArgs = []string{
    "--disable-blink-features=AutomationControlled",
    "--disable-background-timer-throttling",
    "--disable-renderer-backgrounding",
    "--disable-backgrounding-occluded-windows",
    "--disable-ipc-flooding-protection",
    "--disable-background-networking",
    "--disable-default-apps",
    "--disable-extensions",
    "--disable-sync",
    "--disable-translate",
    "--disable-component-update",
    "--disable-client-side-phishing-detection",
    "--disable-hang-monitor",
    "--disable-popup-blocking",
    "--disable-prompt-on-repost",
    "--disable-domain-reliability",
    "--disable-features=Translate,MediaRouter,OptimizationHints",
    "--no-first-run",
    "--no-default-browser-check",
    "--metrics-recording-only",
    "--safebrowsing-disable-auto-update",
    "--password-store=basic",
    "--use-mock-keychain",
}

// ---------- Core run logic ----------
func run(tokenCount, batchCount, parallelWorkers int, headed bool) error {
    // Install Playwright browsers (best-effort)
    fmt.Println("⏳ Ensuring Playwright Chromium browser is installed...")
    if err := playwright.Install(&playwright.RunOptions{
        Browsers: []string{"chromium"},
    }); err != nil {
        fmt.Fprintf(os.Stderr, "⚠️  playwright install: %v (continuing anyway)\n", err)
    }

    pw, err := playwright.Run()
    if err != nil {
        return fmt.Errorf("playwright run: %w", err)
    }
    defer pw.Stop()

    browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
        Headless: playwright.Bool(!headed),
        Args:     chromiumPerfArgs,
    })
    if err != nil {
        return fmt.Errorf("browser launch: %w", err)
    }
    defer browser.Close()

    // Database path — start fresh. Also nuke WAL/SHM sidecar files.
    dbPath := filepath.Join(".", "tokens.sqlite")
    _ = os.Remove(dbPath)
    _ = os.Remove(dbPath + "-wal")
    _ = os.Remove(dbPath + "-shm")

    // Open DB once and keep it — avoids per-batch open/close/fsync.
    ts, err := openTokenStore(dbPath)
    if err != nil {
        return fmt.Errorf("open db: %w", err)
    }
    defer ts.close()

    // ---------- Parallel path ----------
    if parallelWorkers > 1 && batchCount > 1 {
        fmt.Printf("\n🚀 PARALLEL mode: %d worker page(s) on a single browser\n", parallelWorkers)
        totalCollected, err := runParallel(browser, tokenCount, batchCount, parallelWorkers, ts, dbPath)
        if err != nil {
            return err
        }

        fmt.Printf("\n══════════════════════════════════════════\n")
        fmt.Printf("  ✅ ALL BATCHES COMPLETE (parallel: %d workers)\n", parallelWorkers)
        fmt.Printf("  📦 %d batches × %d tokens = %d total collected\n",
            batchCount, tokenCount, totalCollected)
        if info, err := os.Stat(dbPath); err == nil {
            fmt.Printf("  💾 %s (%.1f KB)\n", dbPath, float64(info.Size())/1024.0)
        }
        fmt.Printf("══════════════════════════════════════════\n")

        return nil
    }

    // ---------- Sequential batch loop ----------
    totalCollected := 0
    for b := 1; b <= batchCount; b++ {
        fmt.Printf("\n══════════════════════════════════════════\n")
        fmt.Printf("  BATCH %d of %d\n", b, batchCount)
        fmt.Printf("══════════════════════════════════════════\n")

        tokens, err := runBatch(browser, tokenCount, b)
        if err != nil {
            return err
        }

        if err := ts.merge(b, tokens); err != nil {
            return fmt.Errorf("database merge: %w", err)
        }

        totalCollected += len(tokens)

        if info, err := os.Stat(dbPath); err == nil {
            fmt.Printf("💾 Database: %s (%.1f KB) — %d tokens total across %d batch(es)\n",
                dbPath, float64(info.Size())/1024.0, totalCollected, b)
        }
    }

    // ---------- Final summary ----------
    fmt.Printf("\n══════════════════════════════════════════\n")
    fmt.Printf("  ✅ ALL BATCHES COMPLETE\n")
    fmt.Printf("  📦 %d batches × %d tokens = %d total collected\n", batchCount, tokenCount, totalCollected)
    if info, err := os.Stat(dbPath); err == nil {
        fmt.Printf("  💾 %s (%.1f KB)\n", dbPath, float64(info.Size())/1024.0)
    }
    fmt.Printf("══════════════════════════════════════════\n")

    return nil
}

// ---------- Main ----------
func main() {
    flag.Parse()

    // Apply --unsafe limits
    maxTokens := MaxTokens
    maxBatch := MaxBatch
    if *unsafeFlag {
        maxTokens = UnsafeMaxTokens
        maxBatch = UnsafeMaxBatch
        fmt.Println("⚠️  --unsafe mode enabled: token limit=1500, batch limit=25")
    }

    reader := bufio.NewReader(os.Stdin)

    // ---------- Prompt for token count ----------
    tokenCount := *tokensFlag
    if tokenCount <= 0 {
        tokenCount = promptInt(reader,
            fmt.Sprintf("How many tokens to collect per batch? [default: %d, max: %d] ", DefaultTokens, maxTokens),
            DefaultTokens, maxTokens)
    } else if tokenCount > maxTokens {
        fmt.Printf("⚠️  Capping tokens to max %d.\n", maxTokens)
        tokenCount = maxTokens
    }

    // ---------- Prompt for batch count ----------
    batchCount := *batchFlag
    if batchCount <= 0 {
        batchCount = promptInt(reader,
            fmt.Sprintf("How many batches? [default: %d, max: %d] ", DefaultBatch, maxBatch),
            DefaultBatch, maxBatch)
    } else if batchCount > maxBatch {
        fmt.Printf("⚠️  Capping batch to max %d.\n", maxBatch)
        batchCount = maxBatch
    }

    // ---------- Prompt for parallel workers ----------
    maxParallel := MaxParallel
    if *unsafeFlag {
        maxParallel = UnsafeMaxParallel
    }

    parallelWorkers := *parallelFlag
    if parallelWorkers == 0 {
        if promptBool(reader, "Enable parallel workers (parallel pages on one browser)? [y/N] ", false) {
            parallelWorkers = promptInt(reader,
                fmt.Sprintf("How many parallel workers? [default: %d, max: %d] ", maxParallel, maxParallel),
                maxParallel, maxParallel)
        }
    } else if parallelWorkers < 0 {
        parallelWorkers = 0
    } else if parallelWorkers > maxParallel {
        fmt.Printf("⚠️  Capping parallel workers to max %d.\n", maxParallel)
        parallelWorkers = maxParallel
    }

    fmt.Printf("\n🎯 Plan: %d tokens × %d batches = %d total tokens",
        tokenCount, batchCount, tokenCount*batchCount)
    if parallelWorkers > 1 {
        fmt.Printf("  (parallel: %d workers)", parallelWorkers)
    }
    fmt.Println()

    // ---------- Run ----------
    if err := run(tokenCount, batchCount, parallelWorkers, *headedFlag); err != nil {
        fmt.Fprintf(os.Stderr, "\n🚫 Fatal error: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("\n🎉 Script finished successfully.")
}
