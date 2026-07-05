// main.go
//
// To build:
//   go mod init token-collector
//   go get github.com/playwright-community/playwright-go
//   go get modernc.org/sqlite
//   go build -o token-collector .
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
    "strconv"
    "strings"
    "sync"
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
)

// ---------- Flags ----------
var (
    unsafeFlag = flag.Bool("unsafe", false, "increase token limit to 1500 and batch limit to 25")
    tokensFlag = flag.Int("tokens", 0, "tokens per batch (0 = prompt)")
    batchFlag  = flag.Int("batch", 0, "number of batches (0 = prompt)")
    headedFlag = flag.Bool("headed", false, "show browser window for debugging")
)

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


// ---------- Collect tokens on a single page ----------
func collectTokensOnPage(page playwright.Page, total int) ([]string, error) {
    // Use 'domcontentloaded' for fast navigation (not slow 'networkidle')
    // FIX: page.Goto returns (Response, error)
    if _, err := page.Goto(URL, playwright.PageGotoOptions{
        WaitUntil: playwright.WaitUntilStateDomcontentloaded,
        Timeout:   playwright.Float(60000),
    }); err != nil {
        return nil, fmt.Errorf("goto: %w", err)
    }

    // Wait for both elements concurrently (Go equivalent of Promise.all)
    fmt.Println("  Locating UI elements in parallel...")
    var (
        err1, err2 error
        wg         sync.WaitGroup
    )
    wg.Add(2)
    go func() {
        defer wg.Done()
        // FIX: Locator.WaitFor returns only error
        err1 = page.Locator("#model-selector-glm-4_7-button").WaitFor(
            playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)},
        )
    }()
    go func() {
        defer wg.Done()
        // FIX: Locator.WaitFor returns only error
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

    // Fill textarea
    // FIX: Get locator directly instead of from WaitFor
    textarea := page.Locator("#chat-input")
    if err := textarea.Fill("__"); err != nil {
        return nil, fmt.Errorf("fill textarea: %w", err)
    }
    fmt.Println(`✅ Textarea filled with "__"`)

    // Click send button
    // FIX: Get locator directly, then call WaitFor
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

    // Wait for token endpoint to initialize
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
                // Yield to browser event loop every 50 tokens
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
// Each retry opens a fresh page (open new page, close old page pattern).
func runBatch(browser playwright.Browser, total, batchNum int) ([]string, error) {
    var lastErr error
    for attempt := 1; attempt <= MaxRetries; attempt++ {
        fmt.Printf("\n🔄 [Batch %d] Attempt %d of %d\n", batchNum, attempt, MaxRetries)

        // Open new page
        page, err := browser.NewPage()
        if err != nil {
            lastErr = err
            continue
        }

        // Farm tokens on this page
        tokens, err := collectTokensOnPage(page, total)

        // Close old page (always, success or fail)
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

// ---------- Merge tokens into SQLite database ----------
func mergeIntoDB(dbPath string, batchNum int, tokens []string) error {
    fmt.Println("🗄️  Merging tokens into SQLite database...")

    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return err
    }
    defer db.Close()

    if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS tokens (
        id    INTEGER PRIMARY KEY,
        token TEXT,
        batch INTEGER
    )`); err != nil {
        return err
    }

    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Get next available ID
    var nextID int
    if err := tx.QueryRow(`SELECT COALESCE(MAX(id), -1) + 1 FROM tokens`).Scan(&nextID); err != nil {
        return err
    }

    stmt, err := tx.Prepare(`INSERT INTO tokens (id, token, batch) VALUES (?, ?, ?)`)
    if err != nil {
        return err
    }
    defer stmt.Close()

    for i, t := range tokens {
        if _, err := stmt.Exec(nextID+i, t, batchNum); err != nil {
            return err
        }
    }

    return tx.Commit()
}

// ---------- Core run logic ----------
func run(tokenCount, batchCount int, headed bool) error {
    // Install Playwright browsers (best-effort — no-op if already installed)
    fmt.Println("⏳ Ensuring Playwright Chromium browser is installed...")
    if err := playwright.Install(&playwright.RunOptions{
        Browsers: []string{"chromium"},
    }); err != nil {
        fmt.Fprintf(os.Stderr, "⚠️  playwright install: %v (continuing anyway)\n", err)
    }

    // Launch browser
    pw, err := playwright.Run()
    if err != nil {
        return fmt.Errorf("playwright run: %w", err)
    }
    defer pw.Stop()

    browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
        Headless: playwright.Bool(!headed),
    })
    if err != nil {
        return fmt.Errorf("browser launch: %w", err)
    }
    defer browser.Close()

    // Database path — start fresh
    dbPath := filepath.Join(".", "tokens.sqlite")
    _ = os.Remove(dbPath)

    // ---------- Batch loop ----------
    // For each batch:
    //   1. Open new page
    //   2. Farm tokens
    //   3. Close old page
    //   4. Merge into database
    //   5. Continue to next batch
    totalCollected := 0
    for b := 1; b <= batchCount; b++ {
        fmt.Printf("\n══════════════════════════════════════════\n")
        fmt.Printf("  BATCH %d of %d\n", b, batchCount)
        fmt.Printf("══════════════════════════════════════════\n")

        tokens, err := runBatch(browser, tokenCount, b)
        if err != nil {
            return err
        }

        if err := mergeIntoDB(dbPath, b, tokens); err != nil {
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

    fmt.Printf("\n🎯 Plan: %d tokens × %d batches = %d total tokens\n",
        tokenCount, batchCount, tokenCount*batchCount)

    // ---------- Run ----------
    if err := run(tokenCount, batchCount, *headedFlag); err != nil {
        fmt.Fprintf(os.Stderr, "\n🚫 Fatal error: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("\n🎉 Script finished successfully.")
}
