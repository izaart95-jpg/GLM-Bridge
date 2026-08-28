// Code moved from the original main.go monolith during the internal/ restructure.
// See README "Project Structure". Part of the Z.AI bridge core (package zbridge).

package zbridge

import (
    "encoding/hex"
    "net/url"
    "sort"
    "bufio"
    "bytes"
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "log"
    "net/http"
    "strconv"
    "strings"
    "time"
    "unicode/utf16"
    "unicode/utf8"
)

// ============================================================================
// Z.AI SIGNATURE GENERATION
// ============================================================================

func generateZaSignature(prompt, token, userID string) (signature, timestamp, urlParams string) {
    tsMs := time.Now().UnixMilli()
    timestamp = strconv.FormatInt(tsMs, 10)
    requestId := randomUUID()
    bucket := tsMs / 300000

    mac := hmac.New(sha256.New, []byte(session.SaltKey))
    mac.Write([]byte(strconv.FormatInt(bucket, 10)))
    wKey := hex.EncodeToString(mac.Sum(nil))

    type kv struct{ k, v string }
    payloadDict := []kv{
        {"requestId", requestId},
        {"timestamp", timestamp},
        {"user_id", userID},
    }
    sort.Slice(payloadDict, func(i, j int) bool {
        return payloadDict[i].k < payloadDict[j].k
    })
    var parts []string
    for _, p := range payloadDict {
        parts = append(parts, p.k+","+p.v)
    }
    sortedPayload := strings.Join(parts, ",")

    promptB64 := base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(prompt)))
    dataToSign := sortedPayload + "|" + promptB64 + "|" + timestamp

    mac2 := hmac.New(sha256.New, []byte(wKey))
    mac2.Write([]byte(dataToSign))
    signature = hex.EncodeToString(mac2.Sum(nil))

    params := url.Values{}
    params.Set("timestamp", timestamp)
    params.Set("requestId", requestId)
    params.Set("user_id", userID)
    params.Set("version", "0.0.1")
    params.Set("platform", "web")
    params.Set("token", token)
    params.Set("user_agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0")
    params.Set("language", "en-US")
    params.Set("screen_resolution", "1920x1080")
    params.Set("viewport_size", "1920x1080")
    params.Set("timezone", "Europe/Paris")
    params.Set("timezone_offset", "-60")
    params.Set("signature_timestamp", timestamp)
    urlParams = params.Encode()

    return
}

// ============================================================================
// JWT DECODE
// ============================================================================

func decodeJWT(token string) (id, name string) {
    parts := strings.Split(token, ".")
    if len(parts) < 2 {
        return "", ""
    }
    decoded, err := base64Decode(parts[1])
    if err != nil {
        return "", ""
    }
    var data map[string]interface{}
    if err := json.Unmarshal(decoded, &data); err != nil {
        return "", ""
    }
    id, _ = data["id"].(string)
    email, _ := data["email"].(string)
    name = "Guest"
    if email != "" {
        name = strings.Split(email, "@")[0]
    }
    return id, name
}

// ============================================================================
// Z.AI SESSION INITIALIZATION
// ============================================================================

func scrapeConfig() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    req, err := http.NewRequestWithContext(ctx, "GET", BASE_URL, nil)
    if err != nil {
        log.Printf("[Config] Scrape error: %s, using default feVersion", err.Error())
        return
    }
    resp, err := zaiHTTPClient.Do(req)
    if err != nil {
        log.Printf("[Config] Scrape error: %s, using default feVersion", err.Error())
        return
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    if match := feVersionRe.FindString(string(body)); match != "" {
        session.mu.Lock()
        session.FeVersion = match
        session.mu.Unlock()
        log.Printf("[Config] fe_version: %s", match)
    }
}

func initializeSession() error {
    session.mu.Lock()
    if session.Initializing {
        session.mu.Unlock()
        for {
            time.Sleep(100 * time.Millisecond)
            session.mu.Lock()
            if !session.Initializing {
                session.mu.Unlock()
                return nil
            }
            session.mu.Unlock()
        }
    }
    session.Initializing = true
    session.mu.Unlock()

    defer func() {
        session.mu.Lock()
        session.Initializing = false
        session.mu.Unlock()
    }()

    if config.ZaiToken != "" {
        log.Println("[Session] Using hardcoded ZAI_TOKEN, skipping guest init.")
        session.Token = config.ZaiToken
        id, name := decodeJWT(session.Token)
        session.UserID = id
        if name != "" {
            session.UserName = name
        }
        if session.UserID == "" {
            session.UserName = "User"
        }
        uidPreview := session.UserID
        if len(uidPreview) > 8 {
            uidPreview = uidPreview[:8]
        }
        log.Printf("[Session] Token user: %s... (%s)", uidPreview, session.UserName)
        session.Initialized = true
        return nil
    }

    log.Println("[Session] Initializing Z.AI session...")

    scrapeConfig()

    headers := map[string]string{
        "Origin":       BASE_URL,
        "Referer":      BASE_URL + "/",
        "User-Agent":       zaiUserAgent,
        "Content-Type": "application/json",
    }

    // Initial guest POST (fire-and-forget)
    ctx1, cancel1 := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel1()
    req1, _ := http.NewRequestWithContext(ctx1, "POST", BASE_URL+"/api/v1/auths/guest", strings.NewReader("{}"))
    for k, v := range headers {
        req1.Header.Set(k, v)
    }
    zaiHTTPClient.Do(req1)

    // GET /api/v1/auths/
    ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel2()
    req2, _ := http.NewRequestWithContext(ctx2, "GET", BASE_URL+"/api/v1/auths/", nil)
    for k, v := range headers {
        req2.Header.Set(k, v)
    }
    resp, err := zaiHTTPClient.Do(req2)
    if err != nil {
        log.Printf("[Session] Initialization error: %s", err.Error())
        session.Initialized = false
        return err
    }

    if resp.StatusCode != 200 {
        resp.Body.Close()
        err := fmt.Errorf("Auth failed: %d", resp.StatusCode)
        log.Printf("[Session] Initialization error: %s", err.Error())
        session.Initialized = false
        return err
    }

    var authData struct {
        Token string `json:"token"`
    }
    body, _ := io.ReadAll(resp.Body)
    resp.Body.Close()
    json.Unmarshal(body, &authData)
    session.Token = authData.Token

    if session.Token == "" {
        ctx3, cancel3 := context.WithTimeout(context.Background(), 15*time.Second)
        defer cancel3()
        req3, _ := http.NewRequestWithContext(ctx3, "POST", BASE_URL+"/api/v1/auths/guest", strings.NewReader("{}"))
        for k, v := range headers {
            req3.Header.Set(k, v)
        }
        guestResp, err := zaiHTTPClient.Do(req3)
        if err == nil {
            var gd struct {
                Token string `json:"token"`
            }
            gb, _ := io.ReadAll(guestResp.Body)
            guestResp.Body.Close()
            json.Unmarshal(gb, &gd)
            session.Token = gd.Token
        }
    }

    if session.Token != "" {
        id, name := decodeJWT(session.Token)
        session.UserID = id
        if name != "" {
            session.UserName = name
        }
        uidPreview := session.UserID
        if len(uidPreview) > 8 {
            uidPreview = uidPreview[:8]
        }
        log.Printf("[Session] Connected. UserID: %s... (%s)", uidPreview, session.UserName)
        session.Initialized = true
        return nil
    }

    session.Initialized = false
    return errors.New("No token received from Z.AI")
}

// ============================================================================
// Z.AI COMMUNICATION
// ============================================================================

func sendToZAI(prompt string, opts SendOptions) (<-chan ZAIResult, error) {
    session.mu.Lock()
    defaultChatID := session.ChatID
    defaultMessages := session.Messages
    initialized := session.Initialized
    session.mu.Unlock()

    model := opts.Model
    if model == "" {
        model = "glm-5"
    }

    // Resolve features dynamically from per-model state
    // (server defaults + stored user overrides)
    featuresMap := resolveFeaturesForModel(model)

    // Apply per-request overrides (highest precedence)
    if opts.WebSearch != nil {
        if *opts.WebSearch {
            featuresMap["auto_web_search"] = true
            featuresMap["web_search"] = true
        } else {
            delete(featuresMap, "auto_web_search")
            delete(featuresMap, "web_search")
        }
    }
    if opts.Thinking != nil {
        featuresMap["enable_thinking"] = *opts.Thinking
    }
    if opts.ImageGen != nil {
        featuresMap["image_generation"] = *opts.ImageGen
    }
    if opts.PreviewMode != nil {
        featuresMap["preview_mode"] = *opts.PreviewMode
    }

    // ── reasoning_effort handling ──
    // Defensive: always strip any stale reasoning_effort first so unsupported
    // models never receive a placeholder value (would cause malfunction).
    delete(featuresMap, "reasoning_effort")

    if opts.ReasoningEffort != "" {
        if modelSupportsReasoningEffort(model) {
            if isValidReasoningEffort(opts.ReasoningEffort) {
                // Forward reasoning_effort INSIDE the features payload
                featuresMap["reasoning_effort"] = opts.ReasoningEffort
                // When reasoning_effort is active, enable_thinking MUST be true
                // and any user modification on enable_thinking is ignored.
                featuresMap["enable_thinking"] = true
                logInfo(fmt.Sprintf(
                    "[reasoning_effort] model=%s effort=%s enabled (enable_thinking forced true)",
                    model, opts.ReasoningEffort))
            } else {
                logError(fmt.Sprintf(
                    "[reasoning_effort] invalid value '%s' for model=%s (accepted: high, max); ignored",
                    opts.ReasoningEffort, model))
            }
        } else {
            logInfo(fmt.Sprintf(
                "[reasoning_effort] model=%s does not support reasoning_effort; parameter ignored",
                model))
        }
    }

    // Remove 'think' entirely; ALWAYS force image_generation to false
    delete(featuresMap, "think")
    featuresMap["image_generation"] = false

    chatID := opts.ChatID
    if chatID == "" {
        chatID = defaultChatID
    }
    messages := opts.Messages
    if messages == nil {
        messages = defaultMessages
    }

    if !initialized {
        if err := initializeSession(); err != nil {
            return nil, err
        }
    }

    resolvedOpts := struct {
        Model, ChatID     string
        FeaturesMap       map[string]interface{}
        Messages          []Message
        ClientMessagesRaw json.RawMessage
        Files             []map[string]interface{}
    }{
        Model:             model,
        ChatID:            chatID,
        FeaturesMap:       featuresMap,
        Messages:          messages,
        ClientMessagesRaw: opts.ClientMessagesRaw,
        Files:             opts.Files,
    }

    ch := make(chan ZAIResult, 100)
    go func() {
        defer close(ch)
        err := sendToZAIStream(prompt, resolvedOpts, ch)
        if err != nil {
            ch <- ZAIResult{Err: err}
        }
    }()
    return ch, nil
}

func sendToZAIStream(prompt string, opts struct {
    Model, ChatID     string
    FeaturesMap       map[string]interface{}
    Messages          []Message
    ClientMessagesRaw json.RawMessage
    Files             []map[string]interface{}
}, ch chan<- ZAIResult) error {

    for attempt := 0; attempt < 2; attempt++ {
        session.mu.Lock()
        token := session.Token
        userID := session.UserID
        feVersion := session.FeVersion
        session.mu.Unlock()

        signature, _, _ := generateZaSignature(prompt, token, userID)
        urlStr := BASE_URL + "/api/v2/chat/completions"

        var messagesField interface{}
        if len(opts.ClientMessagesRaw) > 0 {
            messagesField = json.RawMessage(opts.ClientMessagesRaw)
        } else {
            forwarded := make([]Message, 0, len(opts.Messages)+1)
            forwarded = append(forwarded, opts.Messages...)
            promptJSON, _ := json.Marshal(prompt)
            forwarded = append(forwarded, Message{Role: "user", Content: json.RawMessage(promptJSON)})
            messagesField = forwarded
        }

        captchaParam, err := getCaptchaVerifyParam()
        if err != nil {
            return err
        }

        // Build features payload from dynamically resolved per-model features.
        // reasoning_effort is only present in opts.FeaturesMap if the model supports
        // it AND a valid value was provided — no placeholder is added for
        // unsupported models (would cause malfunction).
        featuresPayload := make(map[string]interface{})
        for k, v := range opts.FeaturesMap {
            featuresPayload[k] = v
        }
        // Remove 'think' entirely — only enable_thinking reaches the request
        delete(featuresPayload, "think")
        featuresPayload["flags"] = []interface{}{}
        // image_generation is ALWAYS false
        featuresPayload["image_generation"] = false

        requestBody := map[string]interface{}{
            "model":                opts.Model,
            "chat_id":              opts.ChatID,
            "messages":             messagesField,
            "signature_prompt":     prompt,
            "stream":               true,
            "captcha_verify_param": captchaParam,
            "features":             featuresPayload,
        }
        // Attach uploaded files (images) only when present — text-only
        // requests keep the exact body shape they always had.
        if len(opts.Files) > 0 {
            requestBody["files"] = opts.Files
        }

        bodyBytes, _ := json.Marshal(requestBody)

        if config.Logging.Level == "debug" {
            log.Println("[DEBUG] Z.AI url", urlStr)
            log.Println("[DEBUG] Z.AI request body:", string(bodyBytes))
            hdrMap := map[string]string{
                "authorization": "Bearer " + token,
                "content-type":  "application/json",
                "x-fe-Version":  feVersion,
                "x-region":      "overseas",
                "x-signature":   signature,
            }
            hdrJSON, _ := json.MarshalIndent(hdrMap, "", "  ")
            log.Println("[DEBUG] Z.AI request headers", string(hdrJSON))
        }

        timeout := time.Duration(config.Timeouts.Default) * time.Millisecond * 2
        ctx, cancel := context.WithTimeout(context.Background(), timeout)
        req, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(bodyBytes))
        if err != nil {
            cancel()
            return fmt.Errorf("Z.AI connection error: %s", err.Error())
        }
        req.Header.Set("authorization", "Bearer "+token)
        req.Header.Set("User-Agent", zaiUserAgent)
        req.Header.Set("content-type", "application/json")
        req.Header.Set("x-fe-Version", feVersion)
        req.Header.Set("x-region", "overseas")
        req.Header.Set("x-signature", signature)

        resp, err := zaiHTTPClient.Do(req)
        if err != nil {
            cancel()
            return fmt.Errorf("Z.AI connection error: %s", err.Error())
        }

        if config.Logging.Level == "debug" {
            log.Printf("[DEBUG] Z.AI response status: %d %s", resp.StatusCode, resp.Status)
            hdrs := map[string]string{}
            for k, v := range resp.Header {
                hdrs[k] = strings.Join(v, ", ")
            }
            hdrJSON, _ := json.MarshalIndent(hdrs, "", "  ")
            log.Println("[DEBUG] Z.AI response headers:", string(hdrJSON))
        }

        if resp.StatusCode == 401 {
            resp.Body.Close()
            cancel()
            session.mu.Lock()
            session.Initialized = false
            session.mu.Unlock()
            if err := initializeSession(); err != nil {
                return err
            }
            continue
        }

        if resp.StatusCode < 200 || resp.StatusCode >= 300 {
            errBody, _ := io.ReadAll(resp.Body)
            resp.Body.Close()
            cancel()
            if config.Logging.Level == "debug" {
                log.Println("[DEBUG] Z.AI error body:", string(errBody))
            }
            return fmt.Errorf("Z.AI error %d: %s", resp.StatusCode, string(errBody))
        }

        err = streamSSEResponse(resp.Body, ch)
        resp.Body.Close()
        cancel()
        return err
    }
    return errors.New("Max retries exceeded")
}

// extractZAIError inspects a parsed Z.AI SSE payload for an embedded error
// (Z.AI sometimes returns HTTP 200 with the error inside the JSON body).
// Returns the human-readable detail string, or "" if no error is present.
func extractZAIError(j map[string]interface{}) string {
    if data, ok := j["data"].(map[string]interface{}); ok {
        // data.error
        if errObj, ok := data["error"].(map[string]interface{}); ok {
            detail, _ := errObj["detail"].(string)
            if detail == "" {
                if s, ok := errObj["message"].(string); ok {
                    detail = s
                }
            }
            if detail != "" {
                if code, ok := errObj["code"]; ok && code != nil {
                    return fmt.Sprintf("%s (code: %v)", detail, code)
                }
                return detail
            }
        }
        // data.data.error (nested variant observed in production)
        if nested, ok := data["data"].(map[string]interface{}); ok {
            if errObj, ok := nested["error"].(map[string]interface{}); ok {
                detail, _ := errObj["detail"].(string)
                if detail == "" {
                    if s, ok := errObj["message"].(string); ok {
                        detail = s
                    }
                }
                if detail != "" {
                    if code, ok := errObj["code"]; ok && code != nil {
                        return fmt.Sprintf("%s (code: %v)", detail, code)
                    }
                    return detail
                }
            }
        }
    }
    // Top-level error (non-Z.AI shape, just in case)
    if errObj, ok := j["error"].(map[string]interface{}); ok {
        detail, _ := errObj["detail"].(string)
        if detail == "" {
            if s, ok := errObj["message"].(string); ok {
                detail = s
            }
        }
        if detail != "" {
            return detail
        }
    }
    return ""
}

// statusFromError maps a Z.AI/bridge error string to an HTTP status code.
func statusFromError(errMsg string) int {
    switch {
    case strings.Contains(errMsg, "401"):
        return 401
    case strings.Contains(errMsg, "403"):
        return 403
    case strings.Contains(errMsg, "429"):
        return 429
    case strings.Contains(errMsg, "400"):
        return 400
    default:
        return 500
    }
}

// utf16IndexToByteIndex converts a UTF-16 code-unit offset — the indexing
// semantics of JavaScript strings, used by the official Z.AI web frontend
// (`content.substring(0, edit_index)`, see the prod-fe bundle) — into a
// byte offset within s. It clamps at the end of s and never returns an
// offset inside a multi-byte rune: an offset landing between the two
// units of a surrogate pair is clamped to the start of that rune.
func utf16IndexToByteIndex(s string, utf16Idx int) int {
    if utf16Idx <= 0 {
        return 0
    }
    byteIdx, units := 0, 0
    for byteIdx < len(s) {
        if units == utf16Idx {
            return byteIdx
        }
        r, size := utf8.DecodeRuneInString(s[byteIdx:])
        ru := utf16.RuneLen(r)
        if ru < 0 {
            ru = 1 // invalid byte: the JS frontend also sees one unit here
        }
        if units+ru > utf16Idx {
            return byteIdx // inside a surrogate pair — clamp to rune start
        }
        units += ru
        byteIdx += size
    }
    return len(s)
}

// commonPrefixLen returns the byte length of the longest common prefix of
// a and b. The result is always on a rune boundary, so slicing either
// string at that offset cannot produce invalid UTF-8.
func commonPrefixLen(a, b string) int {
    i := 0
    for i < len(a) && i < len(b) {
        ra, sa := utf8.DecodeRuneInString(a[i:])
        rb, _ := utf8.DecodeRuneInString(b[i:])
        if ra != rb {
            break
        }
        i += sa
    }
    return i
}

// holdBackTail trims up to n runes from the end of s (rune-safe).
func holdBackTail(s string, n int) string {
    if n <= 0 || s == "" {
        return s
    }
    i, count := len(s), 0
    for i > 0 && count < n {
        _, size := utf8.DecodeLastRuneInString(s[:i])
        i -= size
        count++
    }
    return s[:i]
}

// holdBackPartialDetailsTag trims a trailing fragment that could still be
// the beginning of a <details> tag whose completion has not arrived yet,
// so a tag streamed character by character never leaks to the client.
// A COMPLETE "</details>" literal is kept (legitimate text); a complete
// "<details" is held (waiting for its ">" to decide whether it is a tag).
func holdBackPartialDetailsTag(s string) string {
    i := strings.LastIndex(s, "<")
    if i < 0 {
        return s
    }
    suffix := s[i:]
    if len(suffix) <= len("<details") && strings.HasPrefix("<details", suffix) {
        return s[:i]
    }
    if len(suffix) < len("</details>") && strings.HasPrefix("</details>", suffix) {
        return s[:i]
    }
    return s
}

// holdBackPartialQuoteMarker trims a trailing ">" that forms the entire
// last line of s. Lines inside a <details> reasoning body are markdown-
// quoted ("> ..."), and while the body streams in one character at a
// time a new line's quote marker is transiently present as a bare ">"
// — which stripDetailsTags cannot strip yet (TrimPrefix needs the
// space) but strips one character later when the "> " completes.
// Forwarding that transient ">" makes the stripped-reasoning snapshot
// sequence non-monotonic, diverging the reasoning emitter: every later
// snapshot re-emits everything after the stale ">" — the growing-prefix
// reasoning_content duplication seen by clients. Holding the marker
// back keeps the sequence monotonic; the final flush releases it if
// the text really ends there.
func holdBackPartialQuoteMarker(s string) string {
    if !strings.HasSuffix(s, ">") {
        return s
    }
    body := s[:len(s)-1]
    if body == "" || strings.HasSuffix(body, "\n") {
        return body
    }
    return s
}

// sseEmitter forwards snapshots of a growing (and occasionally rewritten)
// text to an append-only consumer as rune-safe deltas. It tracks exactly
// what the consumer has received so far and never emits a slice that
// starts inside a multi-byte rune, so the consumer can never receive
// invalid UTF-8 (which its JSON renderer would show as U+FFFD
// replacement garble — the symptom reported in issue #23).
type sseEmitter struct {
    clientView string // exactly what the consumer has received so far
}

// delta returns the text to append to the consumer so it converges on
// target as closely as possible, and updates the tracked view:
//   - target extends the view   -> the new suffix (normal growth)
//   - target is a prefix of the view (a deep edit truncated the text)
//     -> "" — nothing can be taken back; the view is kept as-is so the
//     following growth is not re-sent from a rewound base
//   - target rewrote part of the view -> everything after the longest
//     common prefix; the stale fragment in between stays on the consumer
//     (unavoidable for append-only SSE, but it remains valid UTF-8).
//     The tracked state then re-syncs to target so later growth emits
//     only the genuinely new suffix; keeping the stale fragment in the
//     tracked view would make every later snapshot diverge at the same
//     point and re-emit everything after it on every call (cascading
//     growing-prefix duplication).
func (e *sseEmitter) delta(target string) string {
    if target == e.clientView {
        return ""
    }
    if strings.HasPrefix(target, e.clientView) {
        delta := target[len(e.clientView):]
        e.clientView = target
        return delta
    }
    cp := commonPrefixLen(e.clientView, target)
    if cp == len(target) {
        return "" // consumer already has everything target contains
    }
    delta := target[cp:]
    e.clientView = target
    return delta
}

// splitDetails extracts every complete <details ...>...</details> block
// from raw: the block bodies (concatenated) become reasoning, everything
// else becomes content. A trailing opener whose '>' has not arrived yet is
// held pending (neither reasoning nor content) until more data arrives.
func splitDetails(raw string) (reasoning, content string) {
    var rb, cb strings.Builder
    rest := raw
    for {
        idx := strings.Index(rest, "<details")
        if idx < 0 {
            cb.WriteString(rest)
            break
        }
        cb.WriteString(rest[:idx])
        tagEnd := strings.Index(rest[idx:], ">")
        if tagEnd < 0 {
            break // incomplete opener at the tail — hold pending
        }
        afterTag := rest[idx+tagEnd+1:]
        closeIdx := strings.Index(afterTag, "</details>")
        if closeIdx < 0 {
            rb.WriteString(afterTag) // reasoning still streaming
            break
        }
        rb.WriteString(afterTag[:closeIdx])
        rest = afterTag[closeIdx+len("</details>"):]
    }
    return rb.String(), cb.String()
}

func streamSSEResponse(body io.Reader, ch chan<- ZAIResult) error {
    scanner := bufio.NewScanner(body)
    scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

    // ── Accumulated state across SSE lines ──
    var fullText strings.Builder      // raw upstream content, verbatim
    contentEmitter := &sseEmitter{}   // tracks what the client has received
    reasoningEmitter := &sseEmitter{} // same for the reasoning channel

    // stripDetailsTags removes <details ...> and </details> wrappers
    // and leading "> " markdown-quote prefixes from each line.
    stripDetailsTags := func(s string) string {
        if idx := strings.Index(s, "<details"); idx >= 0 {
            if end := strings.Index(s[idx:], ">"); end >= 0 {
                s = s[:idx] + s[idx+end+1:]
            }
        }
        s = strings.ReplaceAll(s, "</details>", "")
        lines := strings.Split(s, "\n")
        for i, l := range lines {
            lines[i] = strings.TrimPrefix(l, "> ")
        }
        return strings.TrimSpace(strings.Join(lines, "\n"))
    }

    // emitContent forwards the part of `target` the client has not seen
    // yet. All slicing is rune-safe, so clients can never receive invalid
    // UTF-8 (which their JSON renderer would show as U+FFFD replacement
    // garble — the symptom reported in issue #23). FullText carries the
    // authoritative upstream snapshot for non-stream consumers and for
    // deep-edit re-sync detection in the handlers.
    emitContent := func(target string) {
        if delta := contentEmitter.delta(target); delta != "" {
            ch <- ZAIResult{Chunk: delta, FullText: target}
        }
    }

    flush := func(final bool) {
        raw := fullText.String()

        // Split <details ...> ... </details> into reasoning vs content
        reasoning, content := splitDetails(raw)
        if reasoning != "" {
            reasoning = stripDetailsTags(reasoning)
        }

        // Emit reasoning delta (prefix-aware, rune-safe). Reasoning rides
        // the same edit-based stream as content, so while the stream is
        // live it gets the same protection: hold back a small tail so
        // trailing edit_content backtracks are absorbed invisibly, hold
        // back a partial </details> close tag streamed character by
        // character (splitDetails folds it into the reasoning body until
        // it completes, so forwarding it would leak the fragment and
        // then rewind the snapshot), and hold back a partially-streamed
        // "> " quote marker that a later character would strip again
        // (non-monotonic snapshots diverge the emitter and duplicate
        // everything after them). The final flush releases everything.
        if !final {
            reasoning = holdBackTail(reasoning, config.StreamHoldback)
            reasoning = holdBackPartialDetailsTag(reasoning)
            reasoning = holdBackPartialQuoteMarker(reasoning)
        }
        if delta := reasoningEmitter.delta(reasoning); delta != "" {
            ch <- ZAIResult{Reasoning: delta}
        }

        // While the stream is live, keep a small tail pending so ordinary
        // trailing edit_content backtracks are absorbed invisibly, and
        // never forward a fragment that could still grow into a <details>
        // tag. The final flush releases everything.
        target := content
        if !final {
            target = holdBackTail(target, config.StreamHoldback)
            target = holdBackPartialDetailsTag(target)
        }
        emitContent(target)
    }

    for scanner.Scan() {
        line := scanner.Text()
        trimmed := strings.TrimSpace(line)

        if config.Logging.Level == "debug" && trimmed != "" {
            log.Println("[DEBUG] Z.AI SSE line:", trimmed)
        }

        if !strings.HasPrefix(trimmed, "data: ") {
            continue
        }
        dataStr := trimmed[6:]
        if dataStr == "[DONE]" {
            flush(true)
            return nil
        }

        var j map[string]interface{}
        if err := json.Unmarshal([]byte(dataStr), &j); err != nil {
            if config.Logging.Level == "debug" {
                log.Println("[DEBUG] Z.AI failed to parse SSE:", dataStr)
            }
            continue
        }

        // ── Detect inline errors (HTTP 200 with error in body) ──
        if errDetail := extractZAIError(j); errDetail != "" {
            if config.Logging.Level == "debug" {
                log.Println("[DEBUG] Z.AI inline SSE error:", errDetail)
            }
            return fmt.Errorf("Z.AI error: %s", errDetail)
        }

        if data, ok := j["data"].(map[string]interface{}); ok {
            if phase, ok := data["phase"].(string); ok && phase == "done" {
                flush(true)
                return nil
            }
        }

        // ── Content accumulation ──
        // Semantics mirror the official Z.AI web frontend (prod-fe bundle):
        //   edit_content:  content = content.substring(0, edit_index) + edit_content
        //                  where edit_index is a UTF-16 code-unit offset
        //                  (JavaScript string indexing); a missing
        //                  edit_index defaults to 0 (full replacement)
        //   content:       full replacement of the accumulated text
        //   delta_content: plain append
        if data, ok := j["data"].(map[string]interface{}); ok {
            if ec, ok := data["edit_content"].(string); ok && ec != "" {
                editIndex := 0
                if ei, ok := data["edit_index"].(float64); ok {
                    editIndex = int(ei)
                }
                current := fullText.String()
                byteIdx := utf16IndexToByteIndex(current, editIndex)
                fullText.Reset()
                fullText.WriteString(current[:byteIdx] + ec)
            } else if tc, ok := data["content"].(string); ok && tc != "" {
                fullText.Reset()
                fullText.WriteString(tc)
            } else if dc, ok := data["delta_content"].(string); ok && dc != "" {
                fullText.WriteString(dc)
            }
        }

        flush(false)
    }

    flush(true)
    return scanner.Err()
}

