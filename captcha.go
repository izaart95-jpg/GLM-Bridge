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
    "regexp"
    "runtime/debug"
    "strconv"
    "strings"
    "sync"
    "sync/atomic"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/mxschmitt/playwright-go"
    _ "modernc.org/sqlite" // pure-Go SQLite, no CGO needed
)

// ---------- Configuration ----------
const (
    MaxTokens                = 1500
    UnsafeMaxTokens          = 1500
    DefaultTokens            = 850
    DefaultBatch             = 5
    MaxBatch                 = 9
    UnsafeMaxBatch           = 25
    SendWaitMs               = 10000
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
    blockTrackersFlag = flag.Bool("block-trackers", false, "enable URL allowlist filter to block trackers (off by default)")
    noTUIFlag         = flag.Bool("no-tui", false, "disable TUI, use plain text output")
)

// ---------- init: tune GC for throughput ----------
// Default Go GC runs at 100% (doubles heap before collecting).
// Bumping to 200% lets the heap grow 3× before collecting, cutting GC
// pauses by ~half in allocation-heavy workloads like token collection.
func init() {
    debug.SetGCPercent(200)
}

// ---------- TUI (Bubble Tea) ----------
var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// logCapture stores log lines in a ring buffer for the TUI to display.
type logCapture struct {
    mu     sync.Mutex
    lines  []string
    maxLen int
}

func (lc *logCapture) addLine(line string) {
    lc.mu.Lock()
    lc.lines = append(lc.lines, line)
    if len(lc.lines) > lc.maxLen {
        lc.lines = lc.lines[len(lc.lines)-lc.maxLen:]
    }
    lc.mu.Unlock()
}

func (lc *logCapture) Lines() []string {
    lc.mu.Lock()
    defer lc.mu.Unlock()
    out := make([]string, len(lc.lines))
    copy(out, lc.lines)
    return out
}

// Global TUI state — lock-free atomics for reads from the TUI goroutine.
var (
    tuiLogCapture      = &logCapture{maxLen: 1000}
    tuiStatus          atomic.Value // string
    tuiBatchesDone     atomic.Int64
    tuiTokensCollected atomic.Int64
    tuiTotalBatches    atomic.Int64
    tuiTokensPerBatch  atomic.Int64
    tuiWorkers         atomic.Int64
    tuiParallel        atomic.Bool
    tuiStartTime       atomic.Value // time.Time
    tuiDone            atomic.Bool
    tuiErr             atomic.Value // error
)

// init: initialise TUI default state
func init() {
    tuiStatus.Store("Initializing...")
    tuiStartTime.Store(time.Now())
}

func tuiSetStatus(s string) { tuiStatus.Store(s) }

// Bubble Tea messages
type tickMsg time.Time
type doneMsg struct{ err error }

func tuiTick() tea.Cmd {
    return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// tuiModel is the Bubble Tea model for the token collector TUI.
type tuiModel struct {
    width     int
    height    int
    logOffset int // lines scrolled up from bottom
    done      bool
    err       error
}

func (m tuiModel) Init() tea.Cmd { return tuiTick() }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "up", "k":
            m.logOffset++
        case "down", "j":
            if m.logOffset > 0 {
                m.logOffset--
            }
        case "g":
            m.logOffset = len(tuiLogCapture.Lines())
        case "G":
            m.logOffset = 0
        }
    case tickMsg:
        if tuiDone.Load() {
            m.done = true
            if v := tuiErr.Load(); v != nil {
                m.err = v.(error)
            }
            return m, tea.Quit
        }
        return m, tuiTick()
    case doneMsg:
        m.done = true
        m.err = msg.err
        return m, tea.Quit
    }
    return m, nil
}

func (m tuiModel) View() string {
    if m.height < 10 || m.width < 40 {
        return "Terminal too small (min 40x10). Resize or press q to quit."
    }

    // Color styles
    stTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
    stLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
    stAcc   := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
    stWarn  := lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
    stDim   := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
    stBar   := lipgloss.NewStyle().Foreground(lipgloss.Color("99"))

    // Gather state from atomics (lock-free)
    status := "Initializing..."
    if v := tuiStatus.Load(); v != nil {
        status = v.(string)
    }
    bd  := tuiBatchesDone.Load()
    tb  := tuiTotalBatches.Load()
    tc  := tuiTokensCollected.Load()
    tpb := tuiTokensPerBatch.Load()
    wk  := tuiWorkers.Load()

    var st time.Time
    if v := tuiStartTime.Load(); v != nil {
        st = v.(time.Time)
    }
    elapsed := time.Since(st).Round(time.Second)

    // --- Header / status block ---
    var hdr strings.Builder
    hdr.WriteString(stTitle.Render("🔑 Token Collector"))
    hdr.WriteByte('\n')

    if m.done {
        if m.err != nil {
            hdr.WriteString(stLabel.Render("Status: ") + stWarn.Render(fmt.Sprintf("ERROR: %v", m.err)))
        } else {
            hdr.WriteString(stLabel.Render("Status: ") + stAcc.Render("✅ COMPLETE"))
        }
    } else {
        sp := spinnerChars[int(time.Now().UnixMilli()/200)%len(spinnerChars)]
        hdr.WriteString(stLabel.Render("Status: ") + fmt.Sprintf("%s %s", sp, stAcc.Render(status)))
    }
    hdr.WriteByte('\n')

    // Progress bar
    if tb > 0 {
        pct := float64(bd) / float64(tb)
        bw := 20
        f := int(pct * float64(bw))
        if f > bw {
            f = bw
        }
        bar := stBar.Render(strings.Repeat("█", f)) + stDim.Render(strings.Repeat("░", bw-f))
        hdr.WriteString(fmt.Sprintf("%s %s  %s\n",
            stLabel.Render("Progress:"),
            bar,
            stAcc.Render(fmt.Sprintf("%d/%d (%.0f%%)", bd, tb, pct*100))))
    }

    // Stats line
    target := tb * tpb
    stats := fmt.Sprintf("%s %s / %s",
        stLabel.Render("Tokens:"),
        stAcc.Render(fmt.Sprintf("%d", tc)),
        stDim.Render(fmt.Sprintf("%d", target)))
    if wk > 1 {
        stats += fmt.Sprintf("  %s %s", stLabel.Render("Workers:"), stAcc.Render(fmt.Sprintf("%d", wk)))
    }
    stats += fmt.Sprintf("  %s %s", stLabel.Render("Elapsed:"), stAcc.Render(elapsed.String()))
    hdr.WriteString(stats)
    hdr.WriteByte('\n')

    headerStr := hdr.String()
    headerLines := strings.Count(headerStr, "\n")

    // --- Log pane ---
    logH := m.height - headerLines - 4 // -4: sep + log header + sep + footer
    if logH < 2 {
        logH = 2
    }

    logs := tuiLogCapture.Lines()
    start := len(logs) - logH - m.logOffset
    if start < 0 {
        start = 0
    }
    end := start + logH
    if end > len(logs) {
        end = len(logs)
    }

    truncSt := lipgloss.NewStyle().MaxWidth(m.width)
    var logLines []string
    if len(logs) == 0 {
        logLines = []string{stDim.Render("(waiting for output...)")}
    } else if start < end {
        for _, l := range logs[start:end] {
            logLines = append(logLines, truncSt.Render(l))
        }
    } else {
        logLines = []string{stDim.Render("(no more logs)")}
    }
    logStr := strings.Join(logLines, "\n")

    // Separator and footer
    sep := stDim.Render(strings.Repeat("─", m.width))
    footer := stDim.Render(" ↑/↓ scroll  •  q quit")

    return headerStr + sep + "\n" + stLabel.Render("📋 Logs") + "\n" + logStr + "\n" + sep + "\n" + footer
}

// ---------- End TUI ----------

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
    // The page is reused across batches; route handlers (if any) were installed
    // once at page creation in newWorkerPage. Each call here force-reloads
    // the page by re-navigating to URL.
    if _, err := page.Goto(URL, playwright.PageGotoOptions{
        WaitUntil: playwright.WaitUntilStateDomcontentloaded,
        Timeout:   playwright.Float(60000),
    }); err != nil {
        return nil, fmt.Errorf("goto: %w", err)
    }

    // Wait for both elements concurrently
    tuiSetStatus("Locating UI elements...")
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
    tuiSetStatus(fmt.Sprintf("Collecting %d tokens...", total))
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

// ---------- Create a worker page with optional route allowlist ----------
// Route handlers persist across reloads on the same page, so the allowlist is
// installed exactly once at page creation rather than per batch.
func newWorkerPage(browser playwright.Browser) (playwright.Page, error) {
    page, err := browser.NewPage()
    if err != nil {
        return nil, err
    }
    if *blockTrackersFlag {
        if err := page.Route("**/*", func(route playwright.Route) {
            if urlAllowed(route.Request().URL()) {
                route.Continue()
            } else {
                route.Abort()
            }
        }); err != nil {
            _ = page.Close()
            return nil, fmt.Errorf("route setup: %w", err)
        }
    }
    return page, nil
}

// ---------- Run a single batch with retries ----------
// Reuses the given page across batches; collectTokensOnPage force-reloads it
// on every call (and on every retry) by re-navigating to URL.
func runBatch(page playwright.Page, total, batchNum int) ([]string, error) {
    var lastErr error
    for attempt := 1; attempt <= MaxRetries; attempt++ {
        tuiSetStatus(fmt.Sprintf("Batch %d — attempt %d/%d", batchNum, attempt, MaxRetries))
        fmt.Printf("\n🔄 [Batch %d] Attempt %d of %d\n", batchNum, attempt, MaxRetries)

        tokens, err := collectTokensOnPage(page, total)

        if err != nil {
            lastErr = err
            fmt.Printf("❌ Attempt %d failed: %v\n", attempt, err)
            if attempt == MaxRetries {
                fmt.Fprintln(os.Stderr, "🚫 All retries exhausted.")
                break
            }
            fmt.Println("♻️  Retrying with a forced page reload...")
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
            // Each worker keeps ONE page open for all its batches; every batch
            // force-reloads the page instead of opening a new one and closing
            // the old one.
            page, perr := newWorkerPage(browser)
            if perr != nil {
                once.Do(func() {
                    firstErr = fmt.Errorf("worker %d page: %w", workerID, perr)
                    aborted.Store(true)
                })
                return
            }
            defer page.Close()
            for batchNum := range batchCh {
                // Lock-free check — no mutex contention.
                if aborted.Load() {
                    return
                }

                fmt.Printf("\n👷 [Worker %d] starting batch %d\n", workerID, batchNum)

                tokens, err := runBatch(page, tokenCount, batchNum)
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

                tuiBatchesDone.Add(1)
                tuiTokensCollected.Add(int64(len(tokens)))
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

// ---------- Network allowlist (surgical URL filter) ----------
// Pre-compiled regex patterns for wildcard rules only.
// Simple prefix/exact rules use strings.HasPrefix / == (no regex overhead).
var (
    // https://z-cdn.chatglm.cn/z-ai/frontend/prod-fe-*/assets/index-*.js
    reZCDN = regexp.MustCompile(`^https://z-cdn\.chatglm\.cn/z-ai/frontend/prod-fe-[^/]+/assets/index-[^/]+\.js$`)
    // https://cloudauth-device-dualstack.*aliyuncs.com/
    reCloudAuth = regexp.MustCompile(`^https://cloudauth-device-dualstack\.[^/]*aliyuncs\.com/`)
    // https://g.alicdn.com/captcha-frontend/FeiLin/*/feilin*.*.js
    reFeiLin = regexp.MustCompile(`^https://g\.alicdn\.com/captcha-frontend/FeiLin/[^/]+/feilin[^/]*\.[^/]*\.js$`)
)

// urlAllowed checks a URL against the allowlist.
// Fast path: prefix checks via strings.HasPrefix (~5 ns each).
// Slow path: regex only for wildcard patterns (3 of 5 rules).
// Switch short-circuits on first match — most requests are decided
// in O(prefix_length) without ever touching the regex engine.
func urlAllowed(u string) bool {
    switch {
    // 1. Entire chat.z.ai domain — also allow wss:// for WebSocket upgrades
    case strings.HasPrefix(u, "https://chat.z.ai/"), strings.HasPrefix(u, "wss://chat.z.ai/"):
        return true
    // 2. z-cdn build assets (prefix filter → regex confirm)
    case strings.HasPrefix(u, "https://z-cdn.chatglm.cn/z-ai/frontend/prod-fe-"):
        return reZCDN.MatchString(u)
    // 3. Exact Aliyun captcha script (string equality, no regex)
    case u == "https://o.alicdn.com/captcha-frontend/aliyunCaptcha/AliyunCaptcha.js":
        return true
    // 4. cloudauth-device-dualstack.*aliyuncs.com (prefix filter → regex confirm)
    case strings.HasPrefix(u, "https://cloudauth-device-dualstack."):
        return reCloudAuth.MatchString(u)
    // 5. FeiLin captcha assets (prefix filter → regex confirm)
    case strings.HasPrefix(u, "https://g.alicdn.com/captcha-frontend/FeiLin/"):
        return reFeiLin.MatchString(u)
    }
    return false
}

// ---------- Core run logic ----------
func run(tokenCount, batchCount, parallelWorkers int, headed bool) error {
    // Install Playwright browsers (best-effort)
    tuiSetStatus("Installing Playwright...")
    fmt.Println("⏳ Ensuring Playwright Chromium browser is installed...")
    if err := playwright.Install(&playwright.RunOptions{
        Browsers: []string{"chromium"},
    }); err != nil {
        fmt.Fprintf(os.Stderr, "⚠️  playwright install: %v (continuing anyway)\n", err)
    }

    tuiSetStatus("Launching browser...")
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
        tuiSetStatus(fmt.Sprintf("Parallel: %d workers", parallelWorkers))
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
    tuiSetStatus("Starting sequential batches...")
    // Keep ONE page open across all batches; each batch force-reloads it
    // instead of opening a new page and closing the old one.
    page, err := newWorkerPage(browser)
    if err != nil {
        return fmt.Errorf("page create: %w", err)
    }
    defer page.Close()

    totalCollected := 0
    for b := 1; b <= batchCount; b++ {
        fmt.Printf("\n══════════════════════════════════════════\n")
        fmt.Printf("  BATCH %d of %d\n", b, batchCount)
        fmt.Printf("══════════════════════════════════════════\n")

        tokens, err := runBatch(page, tokenCount, b)
        if err != nil {
            return err
        }

        if err := ts.merge(b, tokens); err != nil {
            return fmt.Errorf("database merge: %w", err)
        }

        totalCollected += len(tokens)
        tuiBatchesDone.Add(1)
        tuiTokensCollected.Store(int64(totalCollected))

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

    // ---------- TUI setup (before plan so plan shows in TUI logs) ----------
    useTUI := !*noTUIFlag
    var origStdout, origStderr *os.File
    var pipeWriter *os.File

    if useTUI {
        origStdout = os.Stdout
        origStderr = os.Stderr

        r, w, perr := os.Pipe()
        if perr != nil {
            fmt.Fprintf(os.Stderr, "pipe error: %v\n", perr)
            os.Exit(1)
        }
        pipeWriter = w
        os.Stdout = w
        os.Stderr = w

        // Goroutine: read piped output → logCapture ring buffer.
        // Non-blocking; uses 1MB scanner buffer for long lines.
        go func() {
            scanner := bufio.NewScanner(r)
            scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
            for scanner.Scan() {
                tuiLogCapture.addLine(scanner.Text())
            }
        }()

        tuiTotalBatches.Store(int64(batchCount))
        tuiTokensPerBatch.Store(int64(tokenCount))
        wk := parallelWorkers
        if wk < 1 {
            wk = 1
        }
        tuiWorkers.Store(int64(wk))
        tuiParallel.Store(parallelWorkers > 1)
        tuiStartTime.Store(time.Now())
        tuiStatus.Store("Starting...")
    }

    // ---------- Plan ----------
    fmt.Printf("\n🎯 Plan: %d tokens × %d batches = %d total tokens",
        tokenCount, batchCount, tokenCount*batchCount)
    if parallelWorkers > 1 {
        fmt.Printf("  (parallel: %d workers)", parallelWorkers)
    }
    fmt.Println()

    // ---------- Run ----------
    if !useTUI {
        // Plain text mode — no TUI, original behaviour
        if err := run(tokenCount, batchCount, parallelWorkers, *headedFlag); err != nil {
            fmt.Fprintf(os.Stderr, "\n🚫 Fatal error: %v\n", err)
            os.Exit(1)
        }
        fmt.Println("\n🎉 Script finished successfully.")
        return
    }

    // ---------- TUI mode ----------
    // tea.WithOutput(origStdout) sends TUI rendering to the real terminal
    // while fmt.Println goes to the pipe → logCapture.
    p := tea.NewProgram(tuiModel{},
        tea.WithAltScreen(),
        tea.WithOutput(origStdout),
    )

    go func() {
        err := run(tokenCount, batchCount, parallelWorkers, *headedFlag)
        if err == nil {
            tuiSetStatus("Complete!")
        }
        tuiDone.Store(true)
        if err != nil {
            tuiErr.Store(err)
        }
        pipeWriter.Close() // EOF → scanner goroutine exits
        p.Send(doneMsg{err: err})
    }()

    if _, err := p.Run(); err != nil {
        os.Stdout = origStdout
        os.Stderr = origStderr
        fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
        os.Exit(1)
    }

    os.Stdout = origStdout
    os.Stderr = origStderr

    if v := tuiErr.Load(); v != nil {
        fmt.Fprintf(os.Stderr, "\n🚫 Fatal error: %v\n", v.(error))
        os.Exit(1)
    }

    if !tuiDone.Load() {
        fmt.Println("\n⏹️  Interrupted by user.")
        os.Exit(0)
    }

    fmt.Println("\n🎉 Script finished successfully.")
}
