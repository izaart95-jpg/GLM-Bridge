// Whitebox tests for the vision pipeline's message rewriting (package zbridge,
// so the unexported processVisionMessages is reachable). The upload endpoint is
// mocked via BASE_URL; no captcha/session pool is involved.

package zbridge

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

// TestProcessVisionMessagesReplacesURLWithFileID verifies the core vision
// linkage: every image_url part stays in the message, but its url is replaced
// with the uploaded file's id (the exact reference form the chat.z.ai web
// client sends). Text parts and message structure are preserved.
func TestProcessVisionMessagesReplacesURLWithFileID(t *testing.T) {
    const mockFileID = "test-file-id-123"

    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/api/v1/files/" && r.Method == "POST" {
            if err := r.ParseMultipartForm(32 << 20); err != nil {
                http.Error(w, err.Error(), 400)
                return
            }
            resp := map[string]interface{}{
                "id":       mockFileID,
                "user_id":  "test-user",
                "hash":     nil,
                "filename": "image.png",
                "data":     map[string]interface{}{},
                "meta": map[string]interface{}{
                    "name":         "image.png",
                    "content_type": "image/png",
                    "size":         70,
                    "cdn_url":      "https://cdn.example/" + mockFileID,
                    "data":         map[string]interface{}{},
                    "oss_endpoint": "bj",
                },
                "created_at": 1787884926,
                "updated_at": 1787884926,
            }
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(resp)
            return
        }
        http.NotFound(w, r)
    }))
    defer upstream.Close()

    oldBase := BASE_URL
    BASE_URL = upstream.URL
    defer func() { BASE_URL = oldBase }()
    defer OverrideSessionState("test-token", "test-user", true)()

    raw := json.RawMessage(`[
      {"role":"system","content":"be brief"},
      {"role":"user","content":[
        {"type":"text","text":"what is this"},
        {"type":"image_url","image_url":{"url":"data:image/png;base64,` + tinyPngB64 + `"}}
      ]}
    ]`)

    rewritten, files, err := processVisionMessages(context.Background(), raw)
    if err != nil {
        t.Fatalf("processVisionMessages error: %v", err)
    }
    if len(files) != 1 {
        t.Fatalf("files len = %d, want 1", len(files))
    }
    if id, _ := files[0]["id"].(string); id != mockFileID {
        t.Errorf("files[0].id = %q, want %q", id, mockFileID)
    }

    // Parse the rewritten messages and check the image_url part now points at
    // the file id while the text part and system message are untouched.
    var msgs []struct {
        Role    string `json:"role"`
        Content json.RawMessage `json:"content"`
    }
    if err := json.Unmarshal(rewritten, &msgs); err != nil {
        t.Fatalf("parse rewritten messages: %v", err)
    }
    if len(msgs) != 2 {
        t.Fatalf("rewritten messages len = %d, want 2", len(msgs))
    }
    if msgs[0].Role != "system" || string(msgs[0].Content) != `"be brief"` {
        t.Errorf("system message altered: %s", string(msgs[0].Content))
    }

    var parts []struct {
        Type     string `json:"type"`
        Text     string `json:"text"`
        ImageURL *struct {
            URL string `json:"url"`
        } `json:"image_url"`
    }
    if err := json.Unmarshal(msgs[1].Content, &parts); err != nil {
        t.Fatalf("parse rewritten user content: %v", err)
    }
    if len(parts) != 2 {
        t.Fatalf("user content parts = %d, want 2 (image part must be kept)", len(parts))
    }
    if parts[0].Type != "text" || parts[0].Text != "what is this" {
        t.Errorf("text part altered: %+v", parts[0])
    }
    if parts[1].Type != "image_url" || parts[1].ImageURL == nil {
        t.Fatalf("image_url part missing: %+v", parts[1])
    }
    if parts[1].ImageURL.URL != mockFileID {
        t.Errorf("image_url.url = %q, want file id %q", parts[1].ImageURL.URL, mockFileID)
    }
    if strings.Contains(string(rewritten), "data:image/png;base64,") {
        t.Errorf("original data: URL still present in rewritten messages")
    }
}

// TestProcessVisionMessagesNoImages verifies text-only requests pass through
// byte-for-byte with no files.
func TestProcessVisionMessagesNoImages(t *testing.T) {
    raw := json.RawMessage(`[{"role":"user","content":"just text"}]`)
    rewritten, files, err := processVisionMessages(context.Background(), raw)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if files != nil {
        t.Errorf("files = %v, want nil for text-only input", files)
    }
    if string(rewritten) != string(raw) {
        t.Errorf("text-only messages altered:\n got %s\nwant %s", rewritten, raw)
    }
}

// tinyPngB64 is a valid 1x1 transparent PNG (base64).
const tinyPngB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="
