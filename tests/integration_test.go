// Blackbox end-to-end regression test for issue #23 through the REAL HTTP
// stack, now driven via the exported zbridge.NewHandler() surface:
//
//   NewHandler -> authMiddleware -> chatCompletionsHandler -> sendToZAI ->
//   sendToZAIStream -> streamSSEResponse -> OpenAI SSE chunks on the wire.
//
// Only the Z.AI upstream is mocked (httptest); captcha is bypassed via the
// agent-mode cache (pre-seeded), and BASE_URL points at the mock. The mock
// stream contains every pathological shape from the issue report: multibyte
// Chinese text, an edit_content tail revision, and a <details> reasoning tag
// streamed character by character.

package tests

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
    "unicode/utf8"

    "zai-api/internal/zbridge"
)

func TestHTTPEndToEndGarbleFix(t *testing.T) {
    longPrefix := strings.Repeat("长", 30)

    // ── Mock Z.AI upstream ────────────────────────────────────────────────
    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/api/v2/chat/completions" {
            http.NotFound(w, r)
            return
        }
        w.Header().Set("Content-Type", "text/event-stream")
        flusher, _ := w.(http.Flusher)
        events := []string{
            fmt.Sprintf(`{"data":{"delta_content":"%s"}}`, longPrefix),
            `{"data":{"delta_content":"<det"}}`,
            `{"data":{"delta_content":"ails>让我想想。</details>"}}`,
            `{"data":{"delta_content":"答案是："}}`,
            `{"data":{"delta_content":"42。"}}`,
            // Tail revision: replace "42。". edit_index counts the RAW
            // accumulated text (frontend: lt.content.substring(0, i)),
            // which INCLUDES the <details> block:
            //   30 (长×30) + 9 ("<details>") + 5 ("让我想想。")
            //   + 10 ("</details>") + 4 ("答案是：") = 58
            `{"data":{"edit_content":"四十二！","edit_index":58}}`,
            `{"data":{"delta_content":" 🙂"}}`,
            `{"data":{"phase":"done"}}`,
        }
        for _, e := range events {
            fmt.Fprintf(w, "data: %s\n\n", e)
            if flusher != nil {
                flusher.Flush()
            }
        }
        fmt.Fprintf(w, "data: [DONE]\n\n")
        if flusher != nil {
            flusher.Flush()
        }
    }))
    defer upstream.Close()

    // ── Point the bridge at the mock ──────────────────────────────────────
    oldBase := zbridge.BASE_URL
    zbridge.BASE_URL = upstream.URL
    defer func() { zbridge.BASE_URL = oldBase }()

    // ── Pre-seed session (skip real guest auth) ───────────────────────────
    defer zbridge.OverrideSessionState("test-token", "test-user", true)()

    // ── Bypass captcha via the agent-mode cache ───────────────────────────
    cfg := zbridge.GetConfig()
    oldAgentMode, oldHoldback := cfg.AgentMode, cfg.StreamHoldback
    cfg.AgentMode = true
    cfg.StreamHoldback = 24
    defer func() { cfg.AgentMode, cfg.StreamHoldback = oldAgentMode, oldHoldback }()
    zbridge.SeedCaptchaParam("test-captcha-param")

    // ── Drive the real handler through the full HTTP surface ─────────────
    body := `{"model":"glm-4.7","stream":true,"messages":[{"role":"user","content":"hi"}]}`
    req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec := httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)

    if rec.Code != 200 {
        t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
    }

    // ── Parse the OpenAI SSE the client received over the wire ────────────
    clientText := ""
    reasoning := ""
    contentChunks := 0
    for _, line := range strings.Split(rec.Body.String(), "\n") {
        line = strings.TrimSpace(line)
        if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
            continue
        }
        var chunk struct {
            Choices []struct {
                Delta struct {
                    Content          string `json:"content"`
                    ReasoningContent string `json:"reasoning_content"`
                }
            }
            Error *struct {
                Message string `json:"message"`
            }
        }
        if err := json.Unmarshal([]byte(line[6:]), &chunk); err != nil {
            continue
        }
        if chunk.Error != nil {
            t.Fatalf("client received error chunk: %s", chunk.Error.Message)
        }
        for _, ch := range chunk.Choices {
            if ch.Delta.Content != "" {
                if !utf8.ValidString(ch.Delta.Content) {
                    t.Errorf("client received INVALID UTF-8 content delta (renders as garble): %q", ch.Delta.Content)
                }
                clientText += ch.Delta.Content
                contentChunks++
            }
            if ch.Delta.ReasoningContent != "" {
                if !utf8.ValidString(ch.Delta.ReasoningContent) {
                    t.Errorf("client received INVALID UTF-8 reasoning delta: %q", ch.Delta.ReasoningContent)
                }
                reasoning += ch.Delta.ReasoningContent
            }
        }
    }

    if strings.Contains(clientText, "<det") || strings.Contains(clientText, "<details") {
        t.Errorf("<details> tag fragment leaked into client output: %q", clientText)
    }
    if contentChunks < 2 {
        t.Errorf("expected the response to stream in multiple chunks, got %d", contentChunks)
    }
    want := longPrefix + "答案是：四十二！ 🙂"
    if clientText != want {
        t.Errorf("client text = %q, want %q", clientText, want)
    }
    if reasoning != "让我想想。" {
        t.Errorf("reasoning = %q, want %q", reasoning, "让我想想。")
    }

    // Keep the test from racing the fire-and-forget session GC against the
    // deferred BASE_URL restore.
    time.Sleep(50 * time.Millisecond)
}
