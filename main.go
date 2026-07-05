// main.go
// Merged: Z.AI Direct Bridge + In-Memory Captcha Verification
//
// Combines:
//   1. Aliyun captcha verification parameter generator (previously FIFO-based server)
//   2. Z.AI Direct Bridge HTTP server
//
// The captcha_verify_param is now computed in-memory via direct function calls,
// eliminating FIFO/named pipe overhead for maximum speed.
//
// compile using: go build -trimpath -ldflags="-s -w" -gcflags="all=-l=4" -o zai-bridge .

package main

import (
    "bufio"
    "bytes"
    "compress/zlib"
    "context"
    "crypto/hmac"
    "crypto/rand"
    "crypto/sha1"
    "crypto/sha256"
    "database/sql"
    "encoding/base64"
    "encoding/hex"
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "io"
    "log"
    "net/http"
    "net/url"
    "os"
    "regexp"
    "sort"
    "strconv"
    "strings"
    "sync"
    "sync/atomic"
    "time"

    _ "modernc.org/sqlite"
)

// ============================================================================
// CONFIGURATION
// ============================================================================

const (
    // Aliyun captcha credentials
    accessKey       = "LTAI5tSEBwYMwVKAQGpxmvTd"
    secretKey       = "YSKfst7GaVkXwZYvVihJsKF9r89koz"
    sceneID         = "didk33e0"
    pipeName        = "captcha_pipe" // kept for reference; no longer used
    maxTokenRetries = 2

    // Z.AI direct config
    BASE_URL           = "https://chat.z.ai"
    SALT_KEY           = "key-@@@@)))()((9))-xxxx&&&%%%%%"
    DEFAULT_FE_VERSION = "prod-fe-1.0.185"
)

// ---------- Config struct (Z.AI) ----------

type Config struct {
    Server struct {
        Port int
        Host string
    }
    Auth struct {
        Enabled bool
        Token   string
    }
    Timeouts struct {
        Default int
    }
    ZaiToken string
    Logging  struct {
        Level  string
        Format string
    }
    KnownModels []string
}

func loadConfig() *Config {
    c := &Config{}
    c.Server.Port = 3001
    c.Server.Host = "0.0.0.0"
    c.Auth.Enabled = true
    c.Auth.Token = "Waguri"
    c.Timeouts.Default = 300000
    c.ZaiToken = ""
    c.Logging.Level = "debug"
    c.Logging.Format = "text"
    c.KnownModels = []string{"GLM-5.1", "GLM-5"}

    if p := os.Getenv("PORT"); p != "" {
        if n, err := strconv.Atoi(p); err == nil {
            c.Server.Port = n
        }
    }
    if h := os.Getenv("HOST"); h != "" {
        c.Server.Host = h
    }
    if t := os.Getenv("AUTH_TOKEN"); t != "" {
        c.Auth.Token = t
    }
    if t := os.Getenv("TIMEOUT"); t != "" {
        if n, err := strconv.Atoi(t); err == nil {
            c.Timeouts.Default = n
        }
    }
    if t := os.Getenv("ZAI_TOKEN"); t != "" {
        c.ZaiToken = t
    }
    if l := os.Getenv("LOG_LEVEL"); l != "" {
        c.Logging.Level = l
    }
    if f := os.Getenv("LOG_FORMAT"); f != "" {
        c.Logging.Format = f
    }
    return c
}

var config = loadConfig()

// ============================================================================
// TYPE DEFINITIONS
// ============================================================================

// ---------- Z.AI types ----------

type Features struct {
    WebSearch      bool `json:"webSearch"`
    AutoWebSearch  bool `json:"autoWebSearch"`
    Thinking       bool `json:"thinking"`
    ImageGen       bool `json:"imageGen"`
    PreviewMode    bool `json:"previewMode"`
    PersistHistory bool `json:"persistHistory"`
}

type Message struct {
    Role    string          `json:"role"`
    Content json.RawMessage `json:"content"`
}

type SessionState struct {
    mu           sync.Mutex
    Token        string
    UserID       string
    UserName     string
    ChatID       string
    Messages     []Message
    SaltKey      string
    FeVersion    string
    Features     Features
    Initialized  bool
    Initializing bool
}

type ClientSession struct {
    ChatID   string
    Messages []Message
    LastUsed time.Time
}


type ZAIResult struct {
    Chunk    string
    FullText string
    Err      error
}


type SendOptions struct {
    Model             string
    WebSearch         *bool
    Thinking          *bool
    ImageGen          *bool
    PreviewMode       *bool
    ChatID            string
    Messages          []Message
    ClientMessagesRaw json.RawMessage
}

type ResponseResult struct {
    Content      string
    Text         string
    Prompt       string
    FinishReason string
}

// ---------- Captcha JSON struct types ----------

type InitCaptchaResponse struct {
    CertifyID string `json:"CertifyId"`
}

type CVP struct {
    CertifyID   string `json:"certifyId"`
    Data        string `json:"data"`
    DeviceToken string `json:"deviceToken"`
    SceneID     string `json:"sceneId"`
}

type VerifyCaptchaResponse struct {
    Success bool `json:"Success"`
    Result  struct {
        VerifyResult  bool   `json:"VerifyResult"`
        SecurityToken string `json:"securityToken"`
        CertifyID     string `json:"certifyId"`
    } `json:"Result"`
}

type FinalPayload struct {
    CertifyID     string `json:"certifyId"`
    IsSign        bool   `json:"isSign"`
    SceneID       string `json:"sceneId"`
    SecurityToken string `json:"securityToken"`
}

type TrackList struct {
    FI        string `json:"fi"`
    KS        string `json:"ks"`
    MC        string `json:"mc"`
    MP        string `json:"mp"`
    MU        string `json:"mu"`
    StartTime int64  `json:"startTime"`
    TC        string `json:"tc"`
    TE        string `json:"te"`
    TMV       string `json:"tmv"`
}

type Track struct {
    TrackList      TrackList `json:"TrackList"`
    TrackStartTime int64     `json:"TrackStartTime"`
    VerifyTime     int64     `json:"VerifyTime"`
    Arg            string    `json:"arg"`
}

// ============================================================================
// GLOBAL STATE
// ============================================================================

// ---------- Captcha globals ----------

var (
    dbPath   string
    verbose  bool
    gRunning atomic.Bool
    dbMu     sync.Mutex
    logMu    sync.Mutex
    globalDB *sql.DB
)

// ---------- Z.AI globals ----------

var session = &SessionState{
    ChatID:    randomUUID(),
    UserName:  "Guest",
    SaltKey:   SALT_KEY,
    FeVersion: DEFAULT_FE_VERSION,
}

var (
    sessions   = make(map[string]*ClientSession)
    sessionsMu sync.Mutex
)

const SESSION_TTL = 30 * 60 * 1000 * time.Millisecond

var knownModels = []string{
    "glm-4.7", "glm-5", "GLM-5-Turbo", "GLM-5v-Turbo", "GLM-5.1",
}

var feVersionRe = regexp.MustCompile(`prod-fe-\d+\.\d+\.\d+`)

// ============================================================================
// INITIALIZATION
// ============================================================================

func init() {
    // Initialise URL safe-character table for custom URL encoder
    for i := 0; i < 256; i++ {
        c := byte(i)
        if (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
            c == '-' || c == '_' || c == '.' || c == '~' {
            baseSafeTable[i] = true
        }
    }

    // Session cleanup ticker
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        for range ticker.C {
            now := time.Now()
            sessionsMu.Lock()
            for id, s := range sessions {
                if now.Sub(s.LastUsed) > SESSION_TTL {
                    delete(sessions, id)
                }
            }
            sessionsMu.Unlock()
        }
    }()
}

// ============================================================================
// LOGGING — silent unless --verbose
// ============================================================================

func logError(msg string) {
    if !verbose {
        return
    }
    ts := time.Now().UTC().Format("2006-01-02T15:04:05Z")
    logMu.Lock()
    fmt.Fprintf(os.Stderr, "[%s] ERROR: %s\n", ts, msg)
    logMu.Unlock()
}

func logInfo(msg string) {
    if !verbose {
        return
    }
    ts := time.Now().UTC().Format("2006-01-02T15:04:05Z")
    logMu.Lock()
    fmt.Fprintf(os.Stderr, "[%s] INFO: %s\n", ts, msg)
    logMu.Unlock()
}

// ============================================================================
// BUFFER POOLS — eliminate GC pressure on hot paths
// ============================================================================

var bufPool = sync.Pool{
    New: func() interface{} { return bytes.NewBuffer(make([]byte, 0, 4096)) },
}

var zlibWriterPool = sync.Pool{
    New: func() interface{} {
        w, _ := zlib.NewWriterLevel(io.Discard, zlib.DefaultCompression)
        return w
    },
}

// ============================================================================
// HTTP CLIENTS — pooled connections, HTTP/2, keep-alive
// ============================================================================

// Optimised client for Aliyun captcha API calls
var aliyunHTTPClient = &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:          100,
        MaxIdleConnsPerHost:   20,
        MaxConnsPerHost:       20,
        IdleConnTimeout:       90 * time.Second,
        TLSHandshakeTimeout:   10 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,
        ResponseHeaderTimeout: 15 * time.Second,
        ForceAttemptHTTP2:     true,
    },
    Timeout: 30 * time.Second,
}

// Optimised client for Z.AI API calls (no global timeout — streaming)
var zaiHTTPClient = &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:          100,
        MaxIdleConnsPerHost:   20,
        MaxConnsPerHost:       20,
        IdleConnTimeout:       90 * time.Second,
        TLSHandshakeTimeout:   10 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,
        ForceAttemptHTTP2:     true,
    },
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

func randomUUID() string {
    b := make([]byte, 16)
    rand.Read(b)
    b[6] = (b[6] & 0x0f) | 0x40
    b[8] = (b[8] & 0x3f) | 0x80
    return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func generateID() string {
    b := make([]byte, 16)
    rand.Read(b)
    return hex.EncodeToString(b)
}

// ---------- UUID v4 — manual hex encoding, no fmt.Sprintf ----------

func generateUUID() string {
    var b [16]byte
    rand.Read(b[:])
    b[6] = (b[6] & 0x0F) | 0x40
    b[8] = (b[8] & 0x3F) | 0x80

    var dst [36]byte
    j := 0
    for i := 0; i < 16; i++ {
        if i == 4 || i == 6 || i == 8 || i == 10 {
            dst[j] = '-'
            j++
        }
        dst[j] = hexLower[b[i]>>4]
        dst[j+1] = hexLower[b[i]&0xF]
        j += 2
    }
    return string(dst[:])
}

// ---------- Timestamp helpers ----------

func getTimestampUTC() string {
    return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func currentTimeMillis() int64 {
    return time.Now().UnixMilli()
}

// ---------- Token estimation ----------

func estimateTokens(text string) int {
    if text == "" {
        return 0
    }
    return (len(text) + 3) / 4
}

// ---------- Message helpers ----------

func getMessageContent(content json.RawMessage) string {
    if len(content) == 0 {
        return ""
    }
    var s string
    if err := json.Unmarshal(content, &s); err == nil {
        return s
    }
    var arr []interface{}
    if err := json.Unmarshal(content, &arr); err == nil {
        var texts []string
        for _, item := range arr {
            switch v := item.(type) {
            case string:
                texts = append(texts, v)
            case map[string]interface{}:
                t, _ := v["type"].(string)
                if t == "text" {
                    if txt, ok := v["text"].(string); ok {
                        texts = append(texts, txt)
                    }
                }
            }
        }
        return strings.Join(texts, "\n")
    }
    return string(content)
}

func messagesToPrompt(messages []Message) string {
    var sb strings.Builder
    for _, msg := range messages {
        content := getMessageContent(msg.Content)
        sb.WriteString(content)
        sb.WriteString("\n\n")
    }
    return strings.TrimSpace(sb.String())
}

func boolPtr(b bool) *bool { return &b }

func getOrCreateSession(r *http.Request) *ClientSession {
    sessionId := r.Header.Get("X-Session-Id")
    if sessionId == "" {
        sessionId = "default"
    }
    fresh := r.Header.Get("X-Fresh-Session") == "true"

    sessionsMu.Lock()
    defer sessionsMu.Unlock()

    if fresh {
        s := &ClientSession{ChatID: randomUUID(), LastUsed: time.Now()}
        sessions[sessionId] = s
        return s
    }
    if s, ok := sessions[sessionId]; ok {
        s.LastUsed = time.Now()
        return s
    }
    s := &ClientSession{ChatID: randomUUID(), LastUsed: time.Now()}
    sessions[sessionId] = s
    return s
}

// ============================================================================
// URL ENCODING — custom lookup table, zero allocations for safe chars
// ============================================================================

const hexUpper = "0123456789ABCDEF"
const hexLower = "0123456789abcdef"

var baseSafeTable [256]bool

func urlEncode(s string, safe string) string {
    var safeTable [256]bool
    safeTable = baseSafeTable
    for i := 0; i < len(safe); i++ {
        safeTable[safe[i]] = true
    }

    var b strings.Builder
    b.Grow(len(s)*3 + 16)
    for i := 0; i < len(s); i++ {
        c := s[i]
        if safeTable[c] {
            b.WriteByte(c)
        } else {
            b.WriteByte('%')
            b.WriteByte(hexUpper[c>>4])
            b.WriteByte(hexUpper[c&0x0F])
        }
    }
    return b.String()
}

func fromHex(c byte) byte {
    switch {
    case c >= '0' && c <= '9':
        return c - '0'
    case c >= 'A' && c <= 'F':
        return c - 'A' + 10
    case c >= 'a' && c <= 'f':
        return c - 'a' + 10
    default:
        return 0
    }
}

// ============================================================================
// CRYPTO HELPERS
// ============================================================================

func base64Encode(data []byte) string {
    return base64.StdEncoding.EncodeToString(data)
}

func hmacSHA1(key, msg []byte) []byte {
    h := hmac.New(sha1.New, key)
    h.Write(msg)
    return h.Sum(nil)
}

func base64Decode(s string) ([]byte, error) {
    if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
        return b, nil
    }
    if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
        return b, nil
    }
    if b, err := base64.URLEncoding.DecodeString(s); err == nil {
        return b, nil
    }
    if b, err := base64.StdEncoding.DecodeString(s); err == nil {
        return b, nil
    }
    if b, err := base64.StdEncoding.DecodeString(s + "=="); err == nil {
        return b, nil
    }
    if b, err := base64.URLEncoding.DecodeString(s + "=="); err == nil {
        return b, nil
    }
    return nil, errors.New("base64 decode failed")
}

// ============================================================================
// JSON MARSHALING — disables HTML escaping, uses pooled buffer
// ============================================================================

func jsonMarshal(v interface{}) ([]byte, error) {
    buf := bufPool.Get().(*bytes.Buffer)
    buf.Reset()
    enc := json.NewEncoder(buf)
    enc.SetEscapeHTML(false)
    if err := enc.Encode(v); err != nil {
        bufPool.Put(buf)
        return nil, err
    }
    raw := buf.Bytes()
    result := make([]byte, len(raw)-1)
    copy(result, raw)
    bufPool.Put(buf)
    return result, nil
}

// ============================================================================
// DATABASE — SQLite, pure-Go driver (no CGO)
// ============================================================================

func initDB() error {
    var err error
    globalDB, err = sql.Open("sqlite", dbPath)
    if err != nil {
        return err
    }
    globalDB.SetMaxOpenConns(1)
    globalDB.SetMaxIdleConns(1)
    return nil
}

func getNextToken() (string, bool) {
    dbMu.Lock()
    defer dbMu.Unlock()

    if _, err := os.Stat(dbPath); err != nil {
        logError("Database file not found: " + dbPath)
        return "", false
    }

    var token string
    err := globalDB.QueryRow("SELECT token FROM tokens ORDER BY id LIMIT 1;").Scan(&token)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            logError("No device tokens available in table 'tokens'")
        } else {
            logError("Failed to query token: " + err.Error())
        }
        return "", false
    }
    return token, true
}

func removeToken(token string) {
    dbMu.Lock()
    defer dbMu.Unlock()

    _, err := globalDB.Exec("DELETE FROM tokens WHERE token = ?;", token)
    if err != nil {
        logError("Failed to delete consumed token: " + err.Error())
    }
}

// ============================================================================
// ALIYUN SIGNATURE
// ============================================================================

func generateSignature(params map[string]string, secKey string) string {
    keys := make([]string, 0, len(params)+1)
    for k := range params {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    var canonical strings.Builder
    canonical.Grow(512)
    for i, k := range keys {
        if i > 0 {
            canonical.WriteByte('&')
        }
        canonical.WriteString(urlEncode(k, ""))
        canonical.WriteByte('=')
        canonical.WriteString(urlEncode(params[k], ""))
    }

    stringToSign := "POST&" + urlEncode("/", "") + "&" + urlEncode(canonical.String(), "")
    signingKey := secKey + "&"
    return base64Encode(hmacSHA1([]byte(signingKey), []byte(stringToSign)))
}

func buildQueryString(params map[string]string) string {
    keys := make([]string, 0, len(params))
    for k := range params {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    var b strings.Builder
    b.Grow(512)
    for i, k := range keys {
        if i > 0 {
            b.WriteByte('&')
        }
        b.WriteString(urlEncode(k, ""))
        b.WriteByte('=')
        b.WriteString(urlEncode(params[k], ""))
    }
    return b.String()
}

// ============================================================================
// HTTP POST — pooled buffer for response, connection reuse
// ============================================================================

func httpPost(targetURL, body string, extraHeaders map[string]string) (string, error) {
    req, err := http.NewRequest("POST", targetURL, strings.NewReader(body))
    if err != nil {
        return "", err
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
    req.ContentLength = int64(len(body))
    for k, v := range extraHeaders {
        req.Header.Set(k, v)
    }

    resp, err := aliyunHTTPClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    buf := bufPool.Get().(*bytes.Buffer)
    buf.Reset()
    if _, err := io.Copy(buf, resp.Body); err != nil {
        bufPool.Put(buf)
        return "", err
    }
    result := buf.String()
    bufPool.Put(buf)
    return result, nil
}

// ============================================================================
// CAPTCHA GENERATION — PART 1: InitCaptchaV3
// ============================================================================

func initCaptcha() (string, error) {
    params := map[string]string{
        "AccessKeyId":      accessKey,
        "Action":           "InitCaptchaV3",
        "Format":           "JSON",
        "Language":         "en",
        "Mode":             "popup",
        "SceneId":          sceneID,
        "SignatureMethod":  "HMAC-SHA1",
        "SignatureNonce":   generateUUID(),
        "SignatureVersion": "1.0",
        "Timestamp":        getTimestampUTC(),
        "UpLang":           "true",
        "Version":          "2023-03-05",
    }
    params["Signature"] = generateSignature(params, secretKey)

    body := buildQueryString(params)
    resp, err := httpPost(
        "https://no8xfe.captcha-open-southeast.aliyuncs.com/", body, nil)
    if err != nil {
        return "", err
    }

    var result InitCaptchaResponse
    if err := json.Unmarshal([]byte(resp), &result); err != nil {
        return "", fmt.Errorf("parse InitCaptchaV3 response: %w", err)
    }
    return result.CertifyID, nil
}

// ============================================================================
// CAPTCHA GENERATION — PART 2: Generate arg (RC4-like stream cipher)
// ============================================================================

var argPermTable = [64]int{
    32, 50, 10, 51, 6, 44, 37, 16, 46, 11, 62, 19, 43, 25, 23, 30,
    60, 33, 53, 34, 7, 26, 12, 48, 5, 2, 20, 4, 61, 13, 47, 49,
    18, 29, 27, 22, 1, 17, 39, 56, 41, 38, 55, 31, 15, 58, 52, 40,
    8, 57, 45, 35, 59, 36, 42, 54, 63, 3, 24, 28, 14, 9, 0, 21,
}

const argConstant = "4xrihv8zb8tf1mfj"

func generateArg(certifyID string) string {
    encoded := urlEncode(certifyID, "")

    // URL-decode (identity for already-decoded strings, kept for faithfulness)
    o := make([]byte, 0, len(encoded))
    for i := 0; i < len(encoded); {
        if encoded[i] == '%' && i+2 < len(encoded) {
            o = append(o, fromHex(encoded[i+1])<<4|fromHex(encoded[i+2]))
            i += 3
        } else {
            o = append(o, encoded[i])
            i++
        }
    }

    // KSA
    r := argPermTable
    n := argConstant
    rlen := 64

    i, j := 0, 0
    for i < rlen {
        j = (((i + j + r[i] + r[j]) >> 1) + int(n[i%len(n)])) & (rlen - 1)
        if i != j {
            r[i], r[j] = r[j], r[i]
        }
        i++
    }

    // PRGA
    t := make([]byte, 0, len(o))
    e, a := 0, 0
    for idx := 0; idx < len(o); idx++ {
        a = ((e ^ a) + (r[e] ^ r[a])) & (rlen - 1)
        if e != a {
            r[e], r[a] = r[a], r[e]
        }
        m := int(o[idx])
        m = m + e + r[e] - a - r[a]
        m = m ^ (r[e] + r[a])
        m = m ^ r[(r[e]+r[a])&(rlen-1)]
        m = m & 255
        t = append(t, byte(m))
        e = (e + 1) & (rlen - 1)
    }
    return base64Encode(t)
}

// ============================================================================
// CAPTCHA GENERATION — PART 4: ali_hash (custom hash with 16-byte state)
// ============================================================================

func aliHash(inputStr, saltStr string) string {
    o := inputStr
    r := saltStr
    aLen := len(o)
    m := len(r)

    var e [16]int
    for i := 0; i < 16; i++ {
        e[i] = (i << 4) + (i % 16)
    }
    f := 16

    i, j := 0, 0
    for i < f {
        j = (((i + j + e[i] + e[j]) >> 1) + int(r[i%m])) & (f - 1)
        e[i], e[j] = e[j], e[i]
        i++
    }

    idx, p, q := 0, 0, 0
    for idx < aLen {
        q = ((p ^ q) + (e[p] ^ e[q])) & (f - 1)
        e[p], e[q] = e[q], e[p]
        C := int(o[idx])
        C = (C + p + q) ^ e[p] ^ e[q]
        C = C & 255
        e[p] = C
        p = (p + 1) & (f - 1)
        idx++
    }

    for step := 0; step < 2*f; step++ {
        pos := step % f
        if pos != 0 {
            e[pos] ^= e[pos-1]
        } else {
            e[0] ^= e[f-1]
        }
    }

    var result [32]byte
    for i, b := range e {
        result[i*2] = hexLower[(b>>4)&0xF]
        result[i*2+1] = hexLower[b&0xF]
    }
    return string(result[:])
}

// ============================================================================
// CAPTCHA GENERATION — PART 7: encrypt (same RC4-like cipher, different key)
// ============================================================================

const encryptKey = "3e627e1b4c63f913"

func encrypt(plaintext []byte) string {
    o := plaintext
    n := encryptKey
    r := argPermTable
    rlen := 64

    oKsa, tKsa := 0, 0
    for oKsa < rlen {
        tKsa = (((oKsa + tKsa + r[oKsa] + r[tKsa]) >> 1) + int(n[oKsa%len(n)])) & (rlen - 1)
        if oKsa != tKsa {
            r[oKsa], r[tKsa] = r[tKsa], r[oKsa]
        }
        oKsa++
    }

    t := make([]byte, 0, len(o))
    e, a := 0, 0
    for nPrga := 0; nPrga < len(o); nPrga++ {
        a = ((e ^ a) + (r[e] ^ r[a])) & (rlen - 1)
        if e != a {
            r[e], r[a] = r[a], r[e]
        }
        m := int(o[nPrga])
        m = m + e + r[e] - a - r[a]
        m = m ^ (r[e] + r[a])
        m = m ^ r[(r[e]+r[a])&(rlen-1)]
        m = m & 255
        t = append(t, byte(m))
        e = (e + 1) & (rlen - 1)
    }
    return base64Encode(t)
}

// ============================================================================
// ZLIB COMPRESS — pooled writer, pooled output buffer
// ============================================================================

func zlibCompress(data []byte) []byte {
    buf := bufPool.Get().(*bytes.Buffer)
    buf.Reset()
    buf.Grow(len(data) + len(data)/2 + 128)

    w := zlibWriterPool.Get().(*zlib.Writer)
    w.Reset(buf)
    w.Write(data)
    w.Close()
    zlibWriterPool.Put(w)

    result := make([]byte, buf.Len())
    copy(result, buf.Bytes())
    bufPool.Put(buf)
    return result
}

// ============================================================================
// CAPTCHA GENERATION — PART 8: VerifyCaptchaV3
// ============================================================================

func verifyCaptcha(certifyID, dataValue, deviceToken string) (string, error) {
    cvpJSON, err := jsonMarshal(CVP{
        CertifyID:   certifyID,
        Data:        dataValue,
        DeviceToken: deviceToken,
        SceneID:     sceneID,
    })
    if err != nil {
        return "", err
    }

    params := map[string]string{
        "AccessKeyId":        accessKey,
        "Action":             "VerifyCaptchaV3",
        "Format":             "JSON",
        "SignatureMethod":    "HMAC-SHA1",
        "SignatureVersion":   "1.0",
        "Timestamp":          getTimestampUTC(),
        "Version":            "2023-03-05",
        "SceneId":            sceneID,
        "CertifyId":          certifyID,
        "CaptchaVerifyParam": string(cvpJSON),
        "SignatureNonce":     generateUUID(),
    }
    params["Signature"] = generateSignature(params, secretKey)

    body := buildQueryString(params)
    resp, err := httpPost(
        "https://no8xfe-verify.captcha-open-southeast.aliyuncs.com/",
        body, map[string]string{"Referer": ""})
    if err != nil {
        return "", err
    }

    var respJSON VerifyCaptchaResponse
    if err := json.Unmarshal([]byte(resp), &respJSON); err != nil {
        return "", fmt.Errorf("parse VerifyCaptchaV3 response: %w", err)
    }

    if respJSON.Success && respJSON.Result.VerifyResult {
        st := respJSON.Result.SecurityToken
        ci := respJSON.Result.CertifyID
        if st != "" && ci != "" {
            fpJSON, err := jsonMarshal(FinalPayload{
                CertifyID:     ci,
                IsSign:        true,
                SceneID:       sceneID,
                SecurityToken: st,
            })
            if err != nil {
                return "", err
            }
            return base64Encode(fpJSON), nil
        }
        logError("VerifyCaptchaV3 succeeded but securityToken/certifyId empty for deviceToken=" + deviceToken)
    } else if respJSON.Success {
        logError("deviceToken failed verification (VerifyResult=false): " + deviceToken)
    } else {
        logError("VerifyCaptchaV3 request unsuccessful for deviceToken=" + deviceToken + " response=" + resp)
    }
    return "", nil
}

// ============================================================================
// COMPUTE FINAL PAYLOAD — tries tokens until success or exhausted
// ============================================================================

func computeFinalPayload() string {
    for attempt := 0; attempt < maxTokenRetries; attempt++ {
        deviceToken, ok := getNextToken()
        if !ok {
            logError(fmt.Sprintf("No device tokens remaining (attempt %d/%d)",
                attempt+1, maxTokenRetries))
            return ""
        }
        logInfo(fmt.Sprintf("Attempt %d/%d using deviceToken=%s",
            attempt+1, maxTokenRetries, deviceToken))

        payload, err := tryCompute(deviceToken)
        if err != nil {
            logError(fmt.Sprintf("Attempt %d failed for deviceToken=%s: %v",
                attempt+1, deviceToken, err))
            continue
        }
        if payload != "" {
            return payload
        }
        logError("deviceToken=" + deviceToken + " produced empty payload, retrying")
    }
    logError(fmt.Sprintf("All %d token retries exhausted", maxTokenRetries))
    return ""
}

func tryCompute(deviceToken string) (string, error) {
    certifyID, err := initCaptcha()
    if err != nil {
        removeToken(deviceToken)
        return "", fmt.Errorf("initCaptcha: %w", err)
    }

    argValue := generateArg(certifyID)
    ct := currentTimeMillis()

    track := Track{
        TrackList: TrackList{
            StartTime: ct,
        },
        TrackStartTime: ct,
        VerifyTime:     ct + 300,
        Arg:            argValue,
    }
    jsonBytes, err := jsonMarshal(track)
    if err != nil {
        removeToken(deviceToken)
        return "", err
    }

    h := aliHash(string(jsonBytes), "0000")
    combined := h + string(jsonBytes)
    compressed := zlibCompress([]byte(combined))
    fb64 := base64Encode(compressed)
    finalVal := encrypt([]byte(fb64))

    // Always remove token after use — prevents conflicts
    removeToken(deviceToken)

    payload, err := verifyCaptcha(certifyID, finalVal, deviceToken)
    if err != nil {
        return "", fmt.Errorf("verifyCaptcha: %w", err)
    }
    return payload, nil
}

// ============================================================================
// CAPTCHA VERIFICATION PARAM — IN-MEMORY (no FIFO / named pipe)
// ============================================================================

func getCaptchaVerifyParam() (string, error) {
    startedAt := time.Now()
    log.Printf("[Captcha] → computing CaptchaVerifyParam (in-memory IPC)")

    type result struct {
        val string
        err error
    }
    ch := make(chan result, 1)

    go func() {
        payload := computeFinalPayload()
        if payload == "" {
            ch <- result{"", errors.New("captcha generation returned empty payload")}
            return
        }
        ch <- result{payload, nil}
    }()

    select {
    case r := <-ch:
        elapsed := time.Since(startedAt).Seconds()
        if r.err != nil {
            log.Printf("[Captcha] ✗ error: %s", r.err.Error())
            return "", r.err
        }
        if r.val == "" {
            log.Printf("[Captcha] ✗ empty response after %.1fs", elapsed)
            return "", errors.New("captcha generation returned empty response")
        }
        log.Printf("[Captcha] ✓ got %db in %.1fs", len(r.val), elapsed)
        return r.val, nil
    case <-time.After(90 * time.Second):
        elapsed := time.Since(startedAt).Seconds()
        log.Printf("[Captcha] ✗ timeout after %.1fs", elapsed)
        return "", errors.New("captcha generation timeout after 90s")
    }
}

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
    features := session.Features
    defaultChatID := session.ChatID
    defaultMessages := session.Messages
    initialized := session.Initialized
    session.mu.Unlock()

    model := opts.Model
    if model == "" {
        model = "glm-5"
    }
    webSearch := features.WebSearch
    if opts.WebSearch != nil {
        webSearch = *opts.WebSearch
    }
    thinking := features.Thinking
    if opts.Thinking != nil {
        thinking = *opts.Thinking
    }
    imageGen := features.ImageGen
    if opts.ImageGen != nil {
        imageGen = *opts.ImageGen
    }
    previewMode := features.PreviewMode
    if opts.PreviewMode != nil {
        previewMode = *opts.PreviewMode
    }
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
        Model, ChatID         string
        WebSearch, Thinking   bool
        ImageGen, PreviewMode bool
        Messages              []Message
        ClientMessagesRaw     json.RawMessage
    }{
        Model: model, ChatID: chatID,
        WebSearch: webSearch, Thinking: thinking,
        ImageGen: imageGen, PreviewMode: previewMode,
        Messages:          messages,
        ClientMessagesRaw: opts.ClientMessagesRaw,
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
    Model, ChatID         string
    WebSearch, Thinking   bool
    ImageGen, PreviewMode bool
    Messages              []Message
    ClientMessagesRaw     json.RawMessage
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

        requestBody := map[string]interface{}{
            "model":                opts.Model,
            "chat_id":              opts.ChatID,
            "messages":             messagesField,
            "signature_prompt":     prompt,
            "stream":               true,
            "captcha_verify_param": captchaParam,
            "features": map[string]interface{}{
                "image_generation": opts.ImageGen,
                "web_search":       opts.WebSearch,
                "auto_web_search":  opts.WebSearch,
                "preview_mode":     opts.PreviewMode,
                "flags":            []interface{}{},
                "enable_thinking":  opts.Thinking,
            },
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

        ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
        req, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(bodyBytes))
        if err != nil {
            cancel()
            return fmt.Errorf("Z.AI connection error: %s", err.Error())
        }
        req.Header.Set("authorization", "Bearer "+token)
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

func streamSSEResponse(body io.Reader, ch chan<- ZAIResult) error {
    scanner := bufio.NewScanner(body)
    scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

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
            return nil
        }

        var j map[string]interface{}
        if err := json.Unmarshal([]byte(dataStr), &j); err != nil {
            if config.Logging.Level == "debug" {
                log.Println("[DEBUG] Z.AI failed to parse SSE:", dataStr)
            }
            continue
        }

        
        if data, ok := j["data"].(map[string]interface{}); ok {
            if phase, ok := data["phase"].(string); ok && phase == "done" {
                return nil
            }
        }

        chunk := ""
        if data, ok := j["data"].(map[string]interface{}); ok {
            if ec, ok := data["edit_content"].(string); ok && ec != "" {
                chunk = ec
            } else if dc, ok := data["delta_content"].(string); ok && dc != "" {
                chunk = dc
            } else if tc, ok := data["content"].(string); ok && tc != "" {
                chunk = tc
            }
        }
        if chunk == "" {
            if choices, ok := j["choices"].([]interface{}); ok && len(choices) > 0 {
                if choice, ok := choices[0].(map[string]interface{}); ok {
                    if delta, ok := choice["delta"].(map[string]interface{}); ok {
                        if c, ok := delta["content"].(string); ok {
                            chunk = c
                        }
                    }
                }
            }
        }
        if chunk != "" {
            ch <- ZAIResult{Chunk: chunk, FullText: chunk}
        }

    }

    return scanner.Err()
}

// ============================================================================
// FORMAT HELPERS
// ============================================================================

func formatOpenAIResponse(result ResponseResult, model, requestId string, stream bool) interface{} {
    timestamp := time.Now().Unix()
    rawContent := result.Content
    if rawContent == "" {
        rawContent = result.Text
    }

    if stream {
        if result.FinishReason != "stop" {
            return map[string]interface{}{
                "id":      "chatcmpl-" + requestId,
                "object":  "chat.completion.chunk",
                "created": timestamp,
                "model":   model,
                "choices": []map[string]interface{}{
                    {
                        "index":         0,
                        "delta":         map[string]interface{}{"content": rawContent},
                        "finish_reason": nil,
                    },
                },
            }
        }
        return map[string]interface{}{
            "id":      "chatcmpl-" + requestId,
            "object":  "chat.completion.chunk",
            "created": timestamp,
            "model":   model,
            "choices": []map[string]interface{}{
                {
                    "index":         0,
                    "delta":         map[string]interface{}{"content": rawContent},
                    "finish_reason": "stop",
                },
            },
        }
    }

    promptTokens := estimateTokens(result.Prompt)
    completionTokens := estimateTokens(rawContent)

    return map[string]interface{}{
        "id":      "chatcmpl-" + requestId,
        "object":  "chat.completion",
        "created": timestamp,
        "model":   model,
        "choices": []map[string]interface{}{
            {
                "index": 0,
                "message": map[string]interface{}{
                    "role":    "assistant",
                    "content": rawContent,
                },
                "finish_reason": "stop",
            },
        },
        "usage": map[string]interface{}{
            "prompt_tokens":     promptTokens,
            "completion_tokens": completionTokens,
            "total_tokens":      promptTokens + completionTokens,
        },
    }
}

func formatOpenAIError(message, errType string, code interface{}) interface{} {
    return map[string]interface{}{
        "error": map[string]interface{}{
            "message": message,
            "type":    errType,
            "code":    code,
            "param":   nil,
        },
    }
}

func toJSON(v interface{}) string {
    b, _ := json.Marshal(v)
    return string(b)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}

// ============================================================================
// DASHBOARD HTML
// ============================================================================

func getDashboardHTML(host string) string {
    html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Z.AI Direct Bridge</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      background: linear-gradient(135deg, #1e3a5f 0%, #0d1b2a 50%, #1b263b 100%);
      min-height: 100vh; color: #e0e0e0; padding: 20px;
    }
    .container { max-width: 1200px; margin: 0 auto; }
    .header {
      text-align: center; padding: 40px 20px;
      background: rgba(255,255,255,0.05); border-radius: 16px;
      margin-bottom: 30px; border: 1px solid rgba(255,255,255,0.1);
    }
    .header h1 {
      font-size: 2.5rem;
      background: linear-gradient(135deg, #3b82f6, #1d4ed8, #60a5fa);
      -webkit-background-clip: text; -webkit-text-fill-color: transparent;
      margin-bottom: 10px;
    }
    .header p { color: #888; font-size: 1.1rem; }
    .badges { display: flex; gap: 8px; justify-content: center; margin-top: 12px; flex-wrap: wrap; }
    .badge {
      display: inline-block; padding: 4px 12px; border-radius: 12px;
      font-size: 0.8rem; font-weight: 700;
    }
    .badge-green { background: #22c55e; color: #000; }
    .badge-blue  { background: #3b82f6; color: #fff; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
    .card {
      background: rgba(255,255,255,0.05); border-radius: 12px;
      padding: 24px; border: 1px solid rgba(255,255,255,0.1);
    }
    .card h2 { color: #60a5fa; margin-bottom: 16px; font-size: 1.2rem; }
    .stat-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
    .stat { background: rgba(0,0,0,0.2); padding: 12px; border-radius: 8px; }
    .stat .label { color: #888; font-size: 0.85rem; }
    .stat .value { color: #60a5fa; font-weight: 600; font-size: 1.5rem; margin-top: 4px; }
    .code-block {
      background: #0d1117; border-radius: 8px; padding: 16px; overflow-x: auto;
      font-family: 'Monaco', 'Menlo', monospace; font-size: 0.85rem;
      border: 1px solid #30363d; margin: 12px 0;
    }
    .code-block code { color: #c9d1d9; white-space: pre-wrap; }
    .endpoint { background: rgba(0,0,0,0.2); padding: 12px; border-radius: 8px; margin-bottom: 8px; }
    .method {
      display: inline-block; padding: 4px 8px; border-radius: 4px;
      font-size: 0.75rem; font-weight: 600; margin-right: 8px;
    }
    .method.get { background: #22c55e; color: #000; }
    .method.post { background: #3b82f6; color: #fff; }
    .path { font-family: monospace; color: #e0e0e0; }
    .desc { color: #888; font-size: 0.85rem; margin-top: 4px; }
    .section-label {
      font-size: 0.75rem; font-weight: 700; text-transform: uppercase;
      letter-spacing: 0.1em; color: #a855f7; margin: 16px 0 8px;
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>Z.AI Direct Bridge</h1>
      <p>HTTP-only mode — No browser required</p>
      <div class="badges">
        <span class="badge badge-green">⚡ Direct Mode</span>
        <span class="badge badge-blue">OpenAI Compatible</span>
      </div>
    </div>

    <div class="grid">
      <div class="card">
        <h2>Session Status</h2>
        <div class="stat-grid">
          <div class="stat">
            <div class="label">Connection</div>
            <div class="value" id="sessionStatus">...</div>
          </div>
          <div class="stat">
            <div class="label">User</div>
            <div class="value" id="sessionUser" style="font-size:1rem">...</div>
          </div>
          <div class="stat">
            <div class="label">Messages</div>
            <div class="value" id="msgCount">0</div>
          </div>
          <div class="stat">
            <div class="label">FE Version</div>
            <div class="value" id="feVersion" style="font-size:0.85rem">...</div>
          </div>
        </div>
      </div>

      <div class="card">
        <h2>Features</h2>
        <div class="stat-grid">
          <div class="stat"><div class="label">Web Search</div><div class="value" id="featSearch">-</div></div>
          <div class="stat"><div class="label">Thinking</div><div class="value" id="featThink">-</div></div>
          <div class="stat"><div class="label">Image Gen</div><div class="value" id="featImage">-</div></div>
          <div class="stat"><div class="label">Preview</div><div class="value" id="featPreview">-</div></div>
        </div>
      </div>

      <div class="card" style="grid-column: span 2;">
        <h2>API Endpoints</h2>

        <div class="section-label">OpenAI-Compatible</div>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/v1/chat/completions</span>
          <div class="desc">OpenAI-compatible chat endpoint. Supports streaming.</div>
        </div>
        <div class="endpoint">
          <span class="method get">GET</span>
          <span class="path">/v1/models</span>
          <div class="desc">Model list</div>
        </div>

        <div class="section-label">Management</div>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/features</span>
          <div class="desc">Toggle webSearch, thinking, imageGen, previewMode, persistHistory</div>
        </div>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/admin/session/clear</span>
          <div class="desc">Clear conversation history</div>
        </div>
      </div>

      <div class="card" style="grid-column: span 2;">
        <h2>Test the OpenAI endpoint</h2>
        <div class="code-block">
          <code># Non-streaming
curl -X POST http://__HOST__/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer __TOKEN__" \
  -d '{"model":"glm-5","messages":[{"role":"user","content":"Hello!"}],"stream":false}'

# Streaming
curl -X POST http://__HOST__/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer __TOKEN__" \
  -d '{"model":"glm-5","stream":true,"messages":[{"role":"user","content":"Say hi"}]}'</code>
        </div>
      </div>
    </div>
  </div>

  <script>
    async function updateStatus() {
      try {
        const res = await fetch('/status');
        const d = await res.json();
        document.getElementById('sessionStatus').textContent = d.connected ? '✓ OK' : '✗ Off';
        document.getElementById('sessionUser').textContent = d.userName || '-';
        document.getElementById('msgCount').textContent = d.messageCount || 0;
        document.getElementById('feVersion').textContent = d.feVersion || '-';
        document.getElementById('featSearch').textContent = d.features?.webSearch ? 'ON' : 'OFF';
        document.getElementById('featThink').textContent = d.features?.thinking ? 'ON' : 'OFF';
        document.getElementById('featImage').textContent = d.features?.imageGen ? 'ON' : 'OFF';
        document.getElementById('featPreview').textContent = d.features?.previewMode ? 'ON' : 'OFF';
      } catch(e) { console.error(e); }
    }
    updateStatus();
    setInterval(updateStatus, 3000);
  </script>
</body>
</html>`

    html = strings.ReplaceAll(html, "__HOST__", host)
    html = strings.ReplaceAll(html, "__TOKEN__", config.Auth.Token)
    return html
}

// ============================================================================
// MIDDLEWARE
// ============================================================================

func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-Id, X-Fresh-Session")
        if r.Method == "OPTIONS" {
            w.WriteHeader(200)
            return
        }
        next.ServeHTTP(w, r)
    })
}

func checkAuth(r *http.Request) bool {
    if !config.Auth.Enabled {
        return true
    }
    authHeader := r.Header.Get("Authorization")
    provided := authHeader
    if len(authHeader) >= 7 && strings.EqualFold(authHeader[:7], "Bearer ") {
        provided = authHeader[7:]
    }
    return provided == config.Auth.Token
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !config.Auth.Enabled {
            next(w, r)
            return
        }
        if !checkAuth(r) {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(401)
            json.NewEncoder(w).Encode(map[string]interface{}{
                "type": "error",
                "error": map[string]interface{}{
                    "type":    "authentication_error",
                    "message": "Invalid or missing authentication token",
                },
            })
            return
        }
        next(w, r)
    }
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    host := r.Host
    if host == "" {
        host = fmt.Sprintf("localhost:%d", config.Server.Port)
    }
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(getDashboardHTML(host)))
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
    session.mu.Lock()
    defer session.mu.Unlock()

    var userIDPreview interface{}
    if session.UserID != "" {
        uid := session.UserID
        if len(uid) > 8 {
            uid = uid[:8]
        }
        userIDPreview = uid + "..."
    }

    sessionsMu.Lock()
    activeSessions := len(sessions)
    sessionsMu.Unlock()

    writeJSON(w, 200, map[string]interface{}{
        "connected":      session.Initialized,
        "userName":       session.UserName,
        "userId":         userIDPreview,
        "feVersion":      session.FeVersion,
        "activeSessions": activeSessions,
        "features":       session.Features,
        "mode":           "direct",
    })
}

func modelsHandler(w http.ResponseWriter, r *http.Request) {
    now := time.Now().Unix()
    data := make([]map[string]interface{}, 0, len(knownModels))
    for _, m := range knownModels {
        data = append(data, map[string]interface{}{
            "id":           m,
            "object":       "model",
            "created":      now,
            "owned_by":     "z-ai",
            "display_name": m,
        })
    }
    writeJSON(w, 200, map[string]interface{}{
        "object": "list",
        "data":   data,
    })
}

func modelsHandler2(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, 200, map[string]interface{}{
        "models":       knownModels,
        "currentModel": "glm-5",
    })
}

func chatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var body struct {
        Model     string          `json:"model"`
        Messages  json.RawMessage `json:"messages"`
        Stream    *bool           `json:"stream"`
        DeepThink *bool           `json:"deepThink"`
        Search    *bool           `json:"search"`
        WebSearch *bool           `json:"webSearch"`
    }
    bodyBytes, err := io.ReadAll(r.Body)
    if err != nil {
        writeJSON(w, 400, formatOpenAIError("Failed to read body", "invalid_request_error", nil))
        return
    }
    if err := json.Unmarshal(bodyBytes, &body); err != nil {
        writeJSON(w, 400, formatOpenAIError("Invalid JSON", "invalid_request_error", nil))
        return
    }

    model := body.Model
    if model == "" {
        model = "glm-5"
    }

    var messages []Message
    if err := json.Unmarshal(body.Messages, &messages); err != nil || len(messages) == 0 {
        writeJSON(w, 400, formatOpenAIError("messages is required and must be an array", "invalid_request_error", nil))
        return
    }

    stream := true
    if body.Stream != nil {
        stream = *body.Stream
    }

    reqSession := getOrCreateSession(r)
    requestId := generateID()
    prompt := messagesToPrompt(messages)

    session.mu.Lock()
    features := session.Features
    session.mu.Unlock()

    webSearch := features.WebSearch
    if body.WebSearch != nil {
        webSearch = *body.WebSearch
    } else if body.Search != nil {
        webSearch = *body.Search
    }

    thinking := features.Thinking
    if body.DeepThink != nil {
        thinking = *body.DeepThink
    }

    opts := SendOptions{
        Model:             model,
        WebSearch:         boolPtr(webSearch),
        Thinking:          boolPtr(thinking),
        ImageGen:          boolPtr(features.ImageGen),
        PreviewMode:       boolPtr(features.PreviewMode),
        ChatID:            reqSession.ChatID,
        Messages:          reqSession.Messages,
        ClientMessagesRaw: body.Messages,
    }

    if stream {
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")
        w.Header().Set("X-Accel-Buffering", "no")

        flusher, _ := w.(http.Flusher)
        var writeMu sync.Mutex

        writeSSE := func(data string) {
            writeMu.Lock()
            defer writeMu.Unlock()
            fmt.Fprintf(w, "data: %s\n\n", data)
            if flusher != nil {
                flusher.Flush()
            }
        }

        initChunk := formatOpenAIResponse(ResponseResult{Content: ""}, model, requestId, true)
        writeSSE(toJSON(initChunk))

        fullContent := ""
        sentContent := ""

        keepAliveStop := make(chan struct{})
        var wg sync.WaitGroup
        wg.Add(1)
        go func() {
            defer wg.Done()
            ticker := time.NewTicker(5 * time.Second)
            defer ticker.Stop()
            for {
                select {
                case <-ticker.C:
                    ka := formatOpenAIResponse(ResponseResult{Content: ""}, model, requestId, true)
                    writeSSE(toJSON(ka))
                case <-keepAliveStop:
                    return
                }
            }
        }()

        errored := false
        ch, err := sendToZAI(prompt, opts)
        if err != nil {
            log.Printf("[Stream] Error: %s", err.Error())
            writeSSE(toJSON(map[string]interface{}{"error": map[string]string{"message": err.Error()}}))
            writeSSE("[DONE]")
            errored = true
        } else {
            for result := range ch {
                if result.Err != nil {
                    log.Printf("[Stream] Error: %s", result.Err.Error())
                    writeSSE(toJSON(map[string]interface{}{"error": map[string]string{"message": result.Err.Error()}}))
                    writeSSE("[DONE]")
                    errored = true
                    break
                }
                
                if result.FullText != "" {
                    fullContent = result.FullText
                } else {
                    fullContent += result.Chunk
                }
                if len(fullContent) > len(sentContent) {
                    delta := fullContent[len(sentContent):]
                    sentContent = fullContent

                    c := formatOpenAIResponse(ResponseResult{Content: delta}, model, requestId, true)
                    writeSSE(toJSON(c))
                }
            }
        }

        if !errored {
            finalChunk := formatOpenAIResponse(ResponseResult{Content: "", FinishReason: "stop"}, model, requestId, true)
            writeSSE(toJSON(finalChunk))
            writeSSE("[DONE]")
        }

        close(keepAliveStop)
        wg.Wait()

        session.mu.Lock()
        if session.Features.PersistHistory {
            promptJSON, _ := json.Marshal(prompt)
            reqSession.Messages = append(reqSession.Messages, Message{Role: "user", Content: json.RawMessage(promptJSON)})
            if fullContent != "" {
                contentJSON, _ := json.Marshal(fullContent)
                reqSession.Messages = append(reqSession.Messages, Message{Role: "assistant", Content: json.RawMessage(contentJSON)})
            }
        }
        session.mu.Unlock()
    } else {
        ch, err := sendToZAI(prompt, opts)
        if err != nil {
            log.Printf("[API] Error: %s", err.Error())
            statusCode := 500
            if strings.Contains(err.Error(), "401") {
                statusCode = 401
            }
            writeJSON(w, statusCode, formatOpenAIError(err.Error(), "api_error", nil))
            return
        }

        fullContent := ""
        for result := range ch {
            if result.Err != nil {
                log.Printf("[API] Error: %s", result.Err.Error())
                statusCode := 500
                if strings.Contains(result.Err.Error(), "401") {
                    statusCode = 401
                }
                
                writeJSON(w, statusCode, formatOpenAIError(result.Err.Error(), "api_error", nil))
                return
            }
            if result.FullText != "" {
                fullContent = result.FullText
            } else {
                fullContent += result.Chunk
            }
        }


        session.mu.Lock()
        if session.Features.PersistHistory {
            promptJSON, _ := json.Marshal(prompt)
            reqSession.Messages = append(reqSession.Messages, Message{Role: "user", Content: json.RawMessage(promptJSON)})
            if fullContent != "" {
                contentJSON, _ := json.Marshal(fullContent)
                reqSession.Messages = append(reqSession.Messages, Message{Role: "assistant", Content: json.RawMessage(contentJSON)})
            }
        }
        session.mu.Unlock()

        writeJSON(w, 200, formatOpenAIResponse(ResponseResult{Content: fullContent}, model, requestId, false))
    }
}

func promptHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var body struct {
        Prompt    string `json:"prompt"`
        Search    *bool  `json:"search"`
        DeepThink *bool  `json:"deepThink"`
        WebSearch *bool  `json:"webSearch"`
    }
    bodyBytes, _ := io.ReadAll(r.Body)
    json.Unmarshal(bodyBytes, &body)

    if body.Prompt == "" {
        writeJSON(w, 400, map[string]interface{}{"error": "Prompt is required"})
        return
    }

    reqSession := getOrCreateSession(r)

    session.mu.Lock()
    features := session.Features
    session.mu.Unlock()

    webSearch := features.WebSearch
    if body.WebSearch != nil {
        webSearch = *body.WebSearch
    } else if body.Search != nil {
        webSearch = *body.Search
    }
    thinking := features.Thinking
    if body.DeepThink != nil {
        thinking = *body.DeepThink
    }

    opts := SendOptions{
        WebSearch: boolPtr(webSearch),
        Thinking:  boolPtr(thinking),
        ChatID:    reqSession.ChatID,
        Messages:  reqSession.Messages,
    }

    ch, err := sendToZAI(body.Prompt, opts)
    if err != nil {
        log.Printf("[Prompt] Error: %s", err.Error())
        writeJSON(w, 500, map[string]interface{}{"success": false, "error": err.Error()})
        return
    }

    fullContent := ""
    for result := range ch {
        if result.Err != nil {
            log.Printf("[Prompt] Error: %s", result.Err.Error())
            
            writeJSON(w, 500, map[string]interface{}{"success": false, "error": result.Err.Error()})
            return
        }
        if result.FullText != "" {
            fullContent = result.FullText
        } else {
            fullContent += result.Chunk
        }
    }


    session.mu.Lock()
    if session.Features.PersistHistory {
        promptJSON, _ := json.Marshal(body.Prompt)
        reqSession.Messages = append(reqSession.Messages, Message{Role: "user", Content: json.RawMessage(promptJSON)})
        if fullContent != "" {
            contentJSON, _ := json.Marshal(fullContent)
            reqSession.Messages = append(reqSession.Messages, Message{Role: "assistant", Content: json.RawMessage(contentJSON)})
        }
    }
    session.mu.Unlock()

    writeJSON(w, 200, map[string]interface{}{"success": true, "response": fullContent})
}

func featuresHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var body struct {
        WebSearch      *bool `json:"webSearch"`
        Thinking       *bool `json:"thinking"`
        ImageGen       *bool `json:"imageGen"`
        PreviewMode    *bool `json:"previewMode"`
        PersistHistory *bool `json:"persistHistory"`
    }
    bodyBytes, _ := io.ReadAll(r.Body)
    json.Unmarshal(bodyBytes, &body)

    session.mu.Lock()
    if body.WebSearch != nil {
        session.Features.WebSearch = *body.WebSearch
        session.Features.AutoWebSearch = *body.WebSearch
    }
    if body.Thinking != nil {
        session.Features.Thinking = *body.Thinking
    }
    if body.ImageGen != nil {
        session.Features.ImageGen = *body.ImageGen
    }
    if body.PreviewMode != nil {
        session.Features.PreviewMode = *body.PreviewMode
    }
    if body.PersistHistory != nil {
        session.Features.PersistHistory = *body.PersistHistory
    }
    features := session.Features
    session.mu.Unlock()

    log.Printf("[Features] Updated: %+v", features)
    writeJSON(w, 200, map[string]interface{}{
        "success":  true,
        "features": features,
    })
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
    sessionsMu.Lock()
    totalMessages := 0
    for _, s := range sessions {
        totalMessages += len(s.Messages)
    }
    activeSessions := len(sessions)
    sessionsMu.Unlock()

    session.mu.Lock()
    initialized := session.Initialized
    session.mu.Unlock()

    totalClients := 0
    if initialized {
        totalClients = 1
    }

    writeJSON(w, 200, map[string]interface{}{
        "mode":           "direct",
        "totalClients":   totalClients,
        "activeSessions": activeSessions,
        "stats": map[string]interface{}{
            "totalRequests": totalMessages / 2,
        },
    })
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    session.mu.Lock()
    healthy := session.Initialized
    session.mu.Unlock()

    status := 200
    if !healthy {
        status = 503
    }
    writeJSON(w, status, map[string]interface{}{"healthy": healthy, "mode": "direct"})
}

func clientsHandler(w http.ResponseWriter, r *http.Request) {
    session.mu.Lock()
    initialized := session.Initialized
    session.mu.Unlock()

    var clients []map[string]interface{}
    if initialized {
        clients = []map[string]interface{}{
            {"id": "session", "status": "idle"},
        }
    } else {
        clients = []map[string]interface{}{}
    }
    writeJSON(w, 200, map[string]interface{}{"clients": clients})
}

func clearSessionHandler(w http.ResponseWriter, r *http.Request) {
    sessionsMu.Lock()
    sessions = make(map[string]*ClientSession)
    sessionsMu.Unlock()
    log.Println("[Session] All session histories cleared.")
    writeJSON(w, 200, map[string]interface{}{
        "success":        true,
        "message":        "All session histories cleared",
        "activeSessions": 0,
    })
}

func clearClientHandler(w http.ResponseWriter, r *http.Request) {
    if !checkAuth(r) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(401)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "type": "error",
            "error": map[string]interface{}{
                "type":    "authentication_error",
                "message": "Invalid or missing authentication token",
            },
        })
        return
    }
    if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/clear") {
        sessionsMu.Lock()
        sessions = make(map[string]*ClientSession)
        sessionsMu.Unlock()
        writeJSON(w, 200, map[string]interface{}{"success": true, "message": "History cleared"})
        return
    }
    http.NotFound(w, r)
}

func injectHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{"message":"Direct mode"}`))
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, 200, map[string]interface{}{
        "success": true,
        "message": "Stop acknowledged",
    })
}

// ============================================================================
// MAIN
// ============================================================================

func main() {
    flag.StringVar(&dbPath, "db-path", "tokens.sqlite", "Path to SQLite database")
    flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
    flag.Parse()

    logInfo("Starting with db-path='" + dbPath + "' verbose=true")

    if err := initDB(); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
        os.Exit(1)
    }
    defer globalDB.Close()

    gRunning.Store(true)

    // HTTP server setup
    mux := http.NewServeMux()

    mux.HandleFunc("/", dashboardHandler)
    mux.HandleFunc("/status", statusHandler)
    mux.HandleFunc("/v1/models", authMiddleware(modelsHandler))
    mux.HandleFunc("/models", authMiddleware(modelsHandler2))
    mux.HandleFunc("/v1/chat/completions", authMiddleware(chatCompletionsHandler))
    mux.HandleFunc("/prompt", authMiddleware(promptHandler))
    mux.HandleFunc("/features", authMiddleware(featuresHandler))
    mux.HandleFunc("/admin/stats", statsHandler)
    mux.HandleFunc("/admin/health", healthHandler)
    mux.HandleFunc("/admin/clients", clientsHandler)
    mux.HandleFunc("/admin/session/clear", authMiddleware(clearSessionHandler))
    mux.HandleFunc("/admin/clients/", clearClientHandler)
    mux.HandleFunc("/inject.js", injectHandler)
    mux.HandleFunc("/stop", authMiddleware(stopHandler))

    handler := corsMiddleware(mux)

    addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)

    tokenPadded := fmt.Sprintf("%-44s", config.Auth.Token)
    fmt.Printf(`
╔═══════════════════════════════════════════════════════════════╗
║           Z.AI Direct Bridge Server Started                   ║
╠═══════════════════════════════════════════════════════════════╣
║  Mode:          DIRECT HTTP (no browser needed)               ║
║  Captcha IPC:   IN-MEMORY (no FIFO / named pipe)             ║
║  Dashboard:     http://localhost:%d                      ║
╠═══════════════════════════════════════════════════════════════╣
║  OpenAI API:    http://localhost:%d/v1/chat/completions  ║
╠═══════════════════════════════════════════════════════════════╣
║  Auth Token:    %s║
╚═══════════════════════════════════════════════════════════════╝
`, config.Server.Port, config.Server.Port, tokenPadded)

    go func() {
        if err := initializeSession(); err != nil {
            log.Println("[Startup] Session init deferred — will retry on first request.")
        }
    }()

    srv := &http.Server{
        Addr:    addr,
        Handler: handler,
    }
    if err := srv.ListenAndServe(); err != nil {
        log.Fatal(err)
    }
}
