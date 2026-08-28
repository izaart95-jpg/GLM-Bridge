// Blackbox tests for vision (image input) support and the enriched /v1/models
// endpoint. Only the Z.AI upstream is mocked (httptest); BASE_URL points at the
// mock. Captcha is bypassed via the agent-mode cache (pre-seeded) for the tests
// that reach sendToZAI; the limit tests fail fast in processVisionMessages
// before any captcha/session work, so they need no captcha seeding.

package tests

import (
    "bytes"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/http/httptest"
    "strings"
    "sync"
    "testing"
    "time"

    "zai-api/internal/zbridge"
)

// tinyPNG is a valid 1x1 transparent PNG.
const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="

func tinyPNGBytes(t *testing.T) []byte {
    t.Helper()
    b, err := base64.StdEncoding.DecodeString(tinyPNGBase64)
    if err != nil {
        t.Fatalf("decode tiny png: %v", err)
    }
    return b
}

// modelsPayload is a mock /api/models response with a vision model, a text-only
// model, and a model older than the glm-4.7 cutoff.
const modelsPayload = `{
  "data": [
    {
      "id": "glm-5v-turbo",
      "name": "GLM-5V-Turbo",
      "created": 1774521032,
      "info": {
        "name": "GLM-5V-Turbo",
        "params": {"max_tokens": 128000},
        "meta": {
          "description": "Vision model with evolved intelligence",
          "capabilities": {"vision": true, "think": true}
        }
      }
    },
    {
      "id": "glm-4.7",
      "name": "GLM-4.7",
      "created": 1700000000,
      "info": {
        "name": "GLM-4.7",
        "params": {},
        "meta": {
          "description": "Classic high-performance model",
          "capabilities": {"vision": false}
        }
      }
    },
    {
      "id": "glm-4.6-legacy",
      "name": "GLM-4.6",
      "created": 1690000000,
      "info": {
        "name": "GLM-4.6",
        "params": {},
        "meta": {
          "description": "Older model that used to be cut off",
          "capabilities": {}
        }
      }
    }
  ]
}`

// ─────────────────────────────────────────────────────────────────────────────
// /v1/models — glm-4.7-and-newer list + architecture object
// ─────────────────────────────────────────────────────────────────────────────

func TestModelsCappedWithArchitecture(t *testing.T) {
    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/api/models" {
            w.Header().Set("Content-Type", "application/json")
            io.WriteString(w, modelsPayload)
            return
        }
        http.NotFound(w, r)
    }))
    defer upstream.Close()

    oldBase := zbridge.BASE_URL
    zbridge.BASE_URL = upstream.URL
    defer func() { zbridge.BASE_URL = oldBase }()
    defer zbridge.OverrideSessionState("test-token", "test-user", true)()
    zbridge.FlushModelsCache()

    cfg := zbridge.GetConfig()
    req := httptest.NewRequest("GET", "/v1/models", nil)
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec := httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)

    if rec.Code != 200 {
        t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
    }

    var resp struct {
        Object string `json:"object"`
        Data   []struct {
            ID          string `json:"id"`
            Object      string `json:"object"`
            Created     int64  `json:"created"`
            OwnedBy     string `json:"owned_by"`
            DisplayName string `json:"display_name"`
            Description string `json:"description"`
            Architecture struct {
                Modality          string   `json:"modality"`
                InputModalities   []string `json:"input_modalities"`
                OutputModalities  []string `json:"output_modalities"`
            } `json:"architecture"`
        } `json:"data"`
    }
    if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
        t.Fatalf("parse response: %v (body=%s)", err, rec.Body.String())
    }
    if resp.Object != "list" {
        t.Errorf("object = %q, want list", resp.Object)
    }

    byID := make(map[string]int)
    for i, m := range resp.Data {
        byID[m.ID] = i
    }

    // The list keeps every model from the newest down to glm-4.7 (inclusive)
    // and cuts off anything older (see README "Models").
    if len(resp.Data) != 2 {
        t.Fatalf("got %d models, want 2 (newest..glm-4.7 inclusive): %s", len(resp.Data), rec.Body.String())
    }
    for _, id := range []string{"glm-5v-turbo", "glm-4.7"} {
        if _, ok := byID[id]; !ok {
            t.Errorf("model %q missing from /v1/models", id)
        }
    }
    if _, ok := byID["glm-4.6-legacy"]; ok {
        t.Errorf("model older than glm-4.7 must be cut off from /v1/models")
    }

    // Vision model: image input advertised.
    v := resp.Data[byID["glm-5v-turbo"]]
    if v.Architecture.Modality != "text+image->text" {
        t.Errorf("vision modality = %q, want text+image->text", v.Architecture.Modality)
    }
    if !equalStrSlice(v.Architecture.InputModalities, []string{"text", "image"}) {
        t.Errorf("vision input_modalities = %v, want [text image]", v.Architecture.InputModalities)
    }
    if !equalStrSlice(v.Architecture.OutputModalities, []string{"text"}) {
        t.Errorf("vision output_modalities = %v, want [text]", v.Architecture.OutputModalities)
    }
    if v.Created != 1774521032 {
        t.Errorf("vision created = %d, want upstream 1774521032", v.Created)
    }
    if v.Description != "Vision model with evolved intelligence" {
        t.Errorf("vision description = %q", v.Description)
    }

    // Text-only model: no image input.
    tx := resp.Data[byID["glm-4.7"]]
    if tx.Architecture.Modality != "text->text" {
        t.Errorf("text modality = %q, want text->text", tx.Architecture.Modality)
    }
    if !equalStrSlice(tx.Architecture.InputModalities, []string{"text"}) {
        t.Errorf("text input_modalities = %v, want [text]", tx.Architecture.InputModalities)
    }
    if !equalStrSlice(tx.Architecture.OutputModalities, []string{"text"}) {
        t.Errorf("text output_modalities = %v, want [text]", tx.Architecture.OutputModalities)
    }
}

func equalStrSlice(a, b []string) bool {
    if len(a) != len(b) {
        return false
    }
    for i := range a {
        if a[i] != b[i] {
            return false
        }
    }
    return true
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared mock upstream for the vision end-to-end tests
// ─────────────────────────────────────────────────────────────────────────────

type uploadRecord struct {
    ID          string
    Filename    string
    ContentType string
    Bytes       []byte
}

type visionMock struct {
    mu             sync.Mutex
    uploads        []uploadRecord
    uploadCounter  int
    completionsBody []byte
    downloadBytes  []byte
    downloadCT     string
}

func (vm *visionMock) handler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        switch {
        case r.URL.Path == "/api/models":
            w.Header().Set("Content-Type", "application/json")
            io.WriteString(w, modelsPayload)

        case r.URL.Path == "/img.png":
            w.Header().Set("Content-Type", vm.downloadCT)
            w.Write(vm.downloadBytes)

        case r.URL.Path == "/api/v1/files/" && r.Method == "POST":
            vm.handleUpload(w, r)

        case r.URL.Path == "/api/v2/chat/completions" && r.Method == "POST":
            body, _ := io.ReadAll(r.Body)
            vm.mu.Lock()
            vm.completionsBody = body
            vm.mu.Unlock()
            w.Header().Set("Content-Type", "text/event-stream")
            flusher, _ := w.(http.Flusher)
            for _, e := range []string{
                `{"data":{"delta_content":"looks good"}}`,
                `{"data":{"phase":"done"}}`,
            } {
                fmt.Fprintf(w, "data: %s\n\n", e)
                if flusher != nil {
                    flusher.Flush()
                }
            }
            fmt.Fprintf(w, "data: [DONE]\n\n")
            if flusher != nil {
                flusher.Flush()
            }

        case strings.HasPrefix(r.URL.Path, "/api/v1/chats/") && r.Method == "DELETE":
            io.WriteString(w, "true")

        default:
            http.NotFound(w, r)
        }
    }
}

func (vm *visionMock) handleUpload(w http.ResponseWriter, r *http.Request) {
    if err := r.ParseMultipartForm(64 << 20); err != nil {
        http.Error(w, "bad multipart: "+err.Error(), 400)
        return
    }
    f, fh, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "missing file field: "+err.Error(), 400)
        return
    }
    data, _ := io.ReadAll(f)
    f.Close()
    ct := fh.Header.Get("Content-Type")

    vm.mu.Lock()
    vm.uploadCounter++
    id := fmt.Sprintf("file-id-%d", vm.uploadCounter)
    vm.uploads = append(vm.uploads, uploadRecord{
        ID:          id,
        Filename:    fh.Filename,
        ContentType: ct,
        Bytes:       data,
    })
    vm.mu.Unlock()

    resp := map[string]interface{}{
        "id":       id,
        "user_id":  "test-user",
        "hash":     nil,
        "filename": fh.Filename,
        "data":     map[string]interface{}{},
        "meta": map[string]interface{}{
            "name":         fh.Filename,
            "content_type": ct,
            "size":         len(data),
            "cdn_url":      "https://cdn.example/" + id,
            "data":         map[string]interface{}{},
            "oss_endpoint": "bj",
        },
        "created_at": time.Now().Unix(),
        "updated_at": time.Now().Unix(),
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func (vm *visionMock) getUploads() []uploadRecord {
    vm.mu.Lock()
    defer vm.mu.Unlock()
    out := make([]uploadRecord, len(vm.uploads))
    copy(out, vm.uploads)
    return out
}

func (vm *visionMock) getCompletionsBody() []byte {
    vm.mu.Lock()
    defer vm.mu.Unlock()
    out := make([]byte, len(vm.completionsBody))
    copy(out, vm.completionsBody)
    return out
}

// setupVisionE2E points the bridge at a fresh mock upstream, seeds session +
// captcha, and returns the mock plus a cleanup func.
func setupVisionE2E(t *testing.T, downloadBytes []byte, downloadCT string) (*visionMock, func()) {
    t.Helper()
    vm := &visionMock{downloadBytes: downloadBytes, downloadCT: downloadCT}
    upstream := httptest.NewServer(vm.handler())

    oldBase := zbridge.BASE_URL
    zbridge.BASE_URL = upstream.URL

    restoreSession := zbridge.OverrideSessionState("test-token", "test-user", true)
    cfg := zbridge.GetConfig()
    oldAgentMode := cfg.AgentMode
    cfg.AgentMode = true
    zbridge.SeedCaptchaParam("test-captcha-param")
    zbridge.FlushModelsCache()

    cleanup := func() {
        // Let the fire-and-forget session GC hit the mock before teardown.
        time.Sleep(50 * time.Millisecond)
        cfg.AgentMode = oldAgentMode
        restoreSession()
        upstream.Close()
        zbridge.BASE_URL = oldBase
    }
    return vm, cleanup
}

// ─────────────────────────────────────────────────────────────────────────────
// OpenAI /v1/chat/completions — image_url parts (data: URL + http URL)
// ─────────────────────────────────────────────────────────────────────────────

func TestOpenAIVisionEndToEnd(t *testing.T) {
    pngBytes := tinyPNGBytes(t)
    downloadPayload := []byte("FAKE-IMAGE-BYTES-FOR-DOWNLOAD-TEST")
    vm, cleanup := setupVisionE2E(t, downloadPayload, "image/png")
    defer cleanup()

    cfg := zbridge.GetConfig()
    body := map[string]interface{}{
        "model":  "glm-5v-turbo",
        "stream": false,
        "messages": []map[string]interface{}{
            {
                "role": "user",
                "content": []map[string]interface{}{
                    {"type": "text", "text": "describe these"},
                    {"type": "image_url", "image_url": map[string]interface{}{
                        "url": "data:image/png;base64," + tinyPNGBase64,
                    }},
                    {"type": "image_url", "image_url": map[string]interface{}{
                        "url": zbridge.BASE_URL + "/img.png",
                    }},
                },
            },
        },
    }
    bodyJSON, _ := json.Marshal(body)
    req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyJSON))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec := httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)

    if rec.Code != 200 {
        t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
    }

    // Both images were uploaded to Z.AI with the exact bytes.
    uploads := vm.getUploads()
    if len(uploads) != 2 {
        t.Fatalf("got %d uploads, want 2", len(uploads))
    }
    gotPayloads := map[string]bool{}
    byID := map[string]uploadRecord{}
    for _, u := range uploads {
        byID[u.ID] = u
        gotPayloads[string(u.Bytes)] = true
        if u.ContentType != "image/png" {
            t.Errorf("upload %s content-type = %q, want image/png", u.ID, u.ContentType)
        }
    }
    if !gotPayloads[string(pngBytes)] {
        t.Errorf("data: URL image bytes were not uploaded correctly")
    }
    if !gotPayloads[string(downloadPayload)] {
        t.Errorf("downloaded image bytes were not uploaded correctly")
    }

    // The upstream completion body carries a well-formed files array and none
    // of the ORIGINAL image payloads (data: URL / download URL) leak through —
    // the message references the image purely by uploaded file id.
    compBody := vm.getCompletionsBody()
    if len(compBody) == 0 {
        t.Fatalf("upstream completions request was not captured")
    }
    if bytes.Contains(compBody, []byte("base64,"+tinyPNGBase64)) {
        t.Errorf("original data: URL payload leaked into the upstream request body")
    }
    if bytes.Contains(compBody, []byte("/img.png")) {
        t.Errorf("original download URL leaked into the upstream request body")
    }
    var comp struct {
        Files    []map[string]interface{} `json:"files"`
        Messages []struct {
            Role    string          `json:"role"`
            Content json.RawMessage `json:"content"`
        } `json:"messages"`
    }
    if err := json.Unmarshal(compBody, &comp); err != nil {
        t.Fatalf("parse upstream body: %v", err)
    }
    if len(comp.Files) != 2 {
        t.Fatalf("upstream files len = %d, want 2 (body=%s)", len(comp.Files), string(compBody))
    }

    // Regression (vision under agent mode): the upstream messages must still
    // reference every uploaded image by file id. The agent shims flatten
    // message content to plain text; if the image_url parts are dropped the
    // model never receives the pixels and answers "I don't see any image"
    // even though the files array is attached.
    refs := upstreamImageRefs(t, comp.Messages)
    if len(refs) != 2 {
        t.Fatalf("upstream messages carry %d image refs, want 2 (body=%s)",
            len(refs), string(compBody))
    }
    for _, id := range refs {
        if _, ok := byID[id]; !ok {
            t.Errorf("message image ref %q does not match any uploaded file id", id)
        }
    }

    var refIDs, itemIDs []string
    for _, fe := range comp.Files {
        id, _ := fe["id"].(string)
        if id == "" {
            t.Errorf("files entry missing id: %v", fe)
            continue
        }
        if u, ok := fe["url"].(string); !ok || u != "/api/v1/files/"+id {
            t.Errorf("files url = %v, want /api/v1/files/%s", fe["url"], id)
        }
        if ty, _ := fe["type"].(string); ty != "image" {
            t.Errorf("files type = %v, want image", fe["type"])
        }
        if st, _ := fe["status"].(string); st != "uploaded" {
            t.Errorf("files status = %v, want uploaded", fe["status"])
        }
        if md, _ := fe["media"].(string); md != "image" {
            t.Errorf("files media = %v, want image", fe["media"])
        }
        if name, _ := fe["name"].(string); name == "" {
            t.Errorf("files name empty")
        }
        if sz, ok := fe["size"].(float64); !ok || sz <= 0 {
            t.Errorf("files size = %v, want > 0", fe["size"])
        }
        if _, ok := fe["file"].(map[string]interface{}); !ok {
            t.Errorf("files entry missing embedded file object")
        }
        if _, ok := byID[id]; !ok {
            t.Errorf("files id %q does not match any uploaded file", id)
        }
        if ref, ok := fe["ref_user_msg_id"].(string); ok {
            refIDs = append(refIDs, ref)
        } else {
            t.Errorf("files entry missing ref_user_msg_id")
        }
        if item, ok := fe["itemId"].(string); ok {
            itemIDs = append(itemIDs, item)
        } else {
            t.Errorf("files entry missing itemId")
        }
    }
    // Same source message -> same ref_user_msg_id; distinct itemId per file.
    if len(refIDs) == 2 && refIDs[0] != refIDs[1] {
        t.Errorf("ref_user_msg_id differs within one message: %v", refIDs)
    }
    if len(itemIDs) == 2 && itemIDs[0] == itemIDs[1] {
        t.Errorf("itemId should be unique per file, got %v", itemIDs)
    }

    // The client got the model's answer.
    if !strings.Contains(rec.Body.String(), "looks good") {
        t.Errorf("client response missing content: %s", rec.Body.String())
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Anthropic /v1/messages — base64 image block
// ─────────────────────────────────────────────────────────────────────────────

func TestAnthropicVisionEndToEnd(t *testing.T) {
    pngBytes := tinyPNGBytes(t)
    vm, cleanup := setupVisionE2E(t, []byte("unused"), "image/png")
    defer cleanup()

    cfg := zbridge.GetConfig()
    body := map[string]interface{}{
        "model":      "glm-5v-turbo",
        "stream":     false,
        "max_tokens": 100,
        "messages": []map[string]interface{}{
            {
                "role": "user",
                "content": []map[string]interface{}{
                    {"type": "text", "text": "what is in this image"},
                    {"type": "image", "source": map[string]interface{}{
                        "type":       "base64",
                        "media_type": "image/png",
                        "data":       tinyPNGBase64,
                    }},
                },
            },
        },
    }
    bodyJSON, _ := json.Marshal(body)
    req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(bodyJSON))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-api-key", cfg.Auth.Token)
    req.Header.Set("anthropic-version", "2023-06-01")
    rec := httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)

    if rec.Code != 200 {
        t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
    }

    uploads := vm.getUploads()
    if len(uploads) != 1 {
        t.Fatalf("got %d uploads, want 1", len(uploads))
    }
    if !bytes.Equal(uploads[0].Bytes, pngBytes) {
        t.Errorf("uploaded bytes mismatch for Anthropic base64 image")
    }
    if uploads[0].ContentType != "image/png" {
        t.Errorf("upload content-type = %q, want image/png", uploads[0].ContentType)
    }

    compBody := vm.getCompletionsBody()
    if bytes.Contains(compBody, []byte("base64,"+tinyPNGBase64)) {
        t.Errorf("original base64 payload leaked into the upstream request body")
    }
    var comp struct {
        Files []map[string]interface{} `json:"files"`
    }
    if err := json.Unmarshal(compBody, &comp); err != nil {
        t.Fatalf("parse upstream body: %v", err)
    }
    if len(comp.Files) != 1 {
        t.Fatalf("upstream files len = %d, want 1 (body=%s)", len(comp.Files), string(compBody))
    }
    if ty, _ := comp.Files[0]["type"].(string); ty != "image" {
        t.Errorf("files type = %v, want image", comp.Files[0]["type"])
    }
    if id, _ := comp.Files[0]["id"].(string); id == "" {
        t.Errorf("files entry missing id")
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Agent mode — image references must survive the prompt-shim rewrite
// ─────────────────────────────────────────────────────────────────────────────

// upstreamImageRefs extracts every image_url part url from an upstream
// messages array (content may be a string or a typed-parts array).
func upstreamImageRefs(t *testing.T, messages []struct {
    Role    string          `json:"role"`
    Content json.RawMessage `json:"content"`
}) []string {
    t.Helper()
    var refs []string
    for _, m := range messages {
        var parts []map[string]interface{}
        if err := json.Unmarshal(m.Content, &parts); err != nil {
            continue // string content carries no image refs
        }
        for _, p := range parts {
            if ty, _ := p["type"].(string); ty != "image_url" {
                continue
            }
            if iu, ok := p["image_url"].(map[string]interface{}); ok {
                if u, _ := iu["url"].(string); u != "" {
                    refs = append(refs, u)
                }
            }
        }
    }
    return refs
}

// TestAgentModeVisionImageRefsPreserved asserts that BOTH agent shim variants
// keep the uploaded image references in the upstream messages, and that
// text-only agent requests keep their plain-string body shape (no files, no
// parts array). This is the exact regression behind the "I don't see any
// image" reports on agent-mode deployments.
func TestAgentModeVisionImageRefsPreserved(t *testing.T) {
    for _, variant := range []string{"modern", "legacy"} {
        t.Run(variant, func(t *testing.T) {
            vm, cleanup := setupVisionE2E(t, []byte("unused"), "image/png")
            defer cleanup()

            cfg := zbridge.GetConfig()
            oldVariant := cfg.AgentModeVariant
            cfg.AgentModeVariant = variant
            defer func() { cfg.AgentModeVariant = oldVariant }()

            body := map[string]interface{}{
                "model":  "glm-5v-turbo",
                "stream": false,
                "messages": []map[string]interface{}{
                    {"role": "user", "content": []map[string]interface{}{
                        {"type": "text", "text": "what is in this image"},
                        {"type": "image_url", "image_url": map[string]interface{}{
                            "url": "data:image/png;base64," + tinyPNGBase64,
                        }},
                    }},
                },
            }
            bodyJSON, _ := json.Marshal(body)
            req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyJSON))
            req.Header.Set("Content-Type", "application/json")
            req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
            rec := httptest.NewRecorder()
            zbridge.NewHandler().ServeHTTP(rec, req)

            if rec.Code != 200 {
                t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
            }

            uploads := vm.getUploads()
            if len(uploads) != 1 {
                t.Fatalf("got %d uploads, want 1", len(uploads))
            }

            compBody := vm.getCompletionsBody()
            var comp struct {
                Files    []map[string]interface{} `json:"files"`
                Messages []struct {
                    Role    string          `json:"role"`
                    Content json.RawMessage `json:"content"`
                } `json:"messages"`
            }
            if err := json.Unmarshal(compBody, &comp); err != nil {
                t.Fatalf("parse upstream body: %v", err)
            }
            if len(comp.Files) != 1 {
                t.Fatalf("upstream files len = %d, want 1", len(comp.Files))
            }
            refs := upstreamImageRefs(t, comp.Messages)
            if len(refs) != 1 {
                t.Fatalf("upstream messages carry %d image refs, want 1 (body=%s)",
                    len(refs), string(compBody))
            }
            if refs[0] != uploads[0].ID {
                t.Errorf("image ref = %q, want uploaded file id %q", refs[0], uploads[0].ID)
            }
        })
    }
}

// TestAgentModeTextOnlyBodyUnchanged asserts a text-only agent-mode request
// still sends a plain-string message content and no "files" field — the
// image re-attach path must not alter the common case.
func TestAgentModeTextOnlyBodyUnchanged(t *testing.T) {
    vm, cleanup := setupVisionE2E(t, []byte("unused"), "image/png")
    defer cleanup()

    cfg := zbridge.GetConfig()
    body := map[string]interface{}{
        "model":    "glm-5v-turbo",
        "stream":   false,
        "messages": []map[string]interface{}{{"role": "user", "content": "just text"}},
    }
    bodyJSON, _ := json.Marshal(body)
    req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyJSON))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec := httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)

    if rec.Code != 200 {
        t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
    }
    if n := len(vm.getUploads()); n != 0 {
        t.Fatalf("text-only request uploaded %d files", n)
    }

    compBody := vm.getCompletionsBody()
    var comp struct {
        Files    json.RawMessage `json:"files"`
        Messages []struct {
            Role    string          `json:"role"`
            Content json.RawMessage `json:"content"`
        } `json:"messages"`
    }
    if err := json.Unmarshal(compBody, &comp); err != nil {
        t.Fatalf("parse upstream body: %v", err)
    }
    if len(comp.Files) != 0 {
        t.Errorf("text-only request carries files field: %s", string(comp.Files))
    }
    if len(comp.Messages) == 0 {
        t.Fatalf("upstream messages empty")
    }
    var s string
    if err := json.Unmarshal(comp.Messages[len(comp.Messages)-1].Content, &s); err != nil {
        t.Errorf("text-only content is not a plain string: %s",
            string(comp.Messages[len(comp.Messages)-1].Content))
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Limits — max file count and max file size
// ─────────────────────────────────────────────────────────────────────────────

// TestVisionTooManyImages asserts >10 images are rejected before any upload.
func TestVisionTooManyImages(t *testing.T) {
    vm := &visionMock{downloadBytes: []byte("x"), downloadCT: "image/png"}
    upstream := httptest.NewServer(vm.handler())
    defer upstream.Close()

    oldBase := zbridge.BASE_URL
    zbridge.BASE_URL = upstream.URL
    defer func() { zbridge.BASE_URL = oldBase }()
    defer zbridge.OverrideSessionState("test-token", "test-user", true)()

    cfg := zbridge.GetConfig()

    var parts []map[string]interface{}
    parts = append(parts, map[string]interface{}{"type": "text", "text": "too many"})
    for i := 0; i < 11; i++ {
        parts = append(parts, map[string]interface{}{
            "type":      "image_url",
            "image_url": map[string]interface{}{"url": "data:image/png;base64," + tinyPNGBase64},
        })
    }
    body := map[string]interface{}{
        "model":    "glm-5v-turbo",
        "stream":   false,
        "messages": []map[string]interface{}{{"role": "user", "content": parts}},
    }
    bodyJSON, _ := json.Marshal(body)
    req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyJSON))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec := httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)

    if rec.Code != 400 {
        t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
    }
    if !strings.Contains(rec.Body.String(), "too many images") {
        t.Errorf("error should mention too many images: %s", rec.Body.String())
    }
    if n := len(vm.getUploads()); n != 0 {
        t.Errorf("expected 0 uploads after count rejection, got %d", n)
    }
}

// TestVisionOversizedDownload asserts a URL image larger than 50MB is rejected.
func TestVisionOversizedDownload(t *testing.T) {
    // Stream maxImageSize+1 bytes without allocating a huge slice up front.
    oversized := int64(50<<20) + 1
    vm := &visionMock{downloadCT: "image/png"}
    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/big.png" {
            w.Header().Set("Content-Type", "image/png")
            // Write a valid-looking header then pad to oversized.
            chunk := bytes.Repeat([]byte{0x00}, 1<<20)
            var written int64
            for written < oversized {
                n := int64(len(chunk))
                if written+n > oversized {
                    n = oversized - written
                }
                w.Write(chunk[:n])
                written += n
            }
            return
        }
        vm.handler()(w, r)
    }))
    defer upstream.Close()

    oldBase := zbridge.BASE_URL
    zbridge.BASE_URL = upstream.URL
    defer func() { zbridge.BASE_URL = oldBase }()
    defer zbridge.OverrideSessionState("test-token", "test-user", true)()

    cfg := zbridge.GetConfig()
    body := map[string]interface{}{
        "model":  "glm-5v-turbo",
        "stream": false,
        "messages": []map[string]interface{}{
            {"role": "user", "content": []map[string]interface{}{
                {"type": "text", "text": "big"},
                {"type": "image_url", "image_url": map[string]interface{}{
                    "url": zbridge.BASE_URL + "/big.png",
                }},
            }},
        },
    }
    bodyJSON, _ := json.Marshal(body)
    req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyJSON))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec := httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)

    if rec.Code != 400 {
        t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
    }
    if !strings.Contains(rec.Body.String(), "max size") {
        t.Errorf("error should mention max size: %s", rec.Body.String())
    }
    if n := len(vm.getUploads()); n != 0 {
        t.Errorf("expected 0 uploads after size rejection, got %d", n)
    }
}

// TestVisionOversizedBase64 asserts a data: URL image larger than 50MB is
// rejected. This materializes a >50MB payload, so it is heavier than the rest.
func TestVisionOversizedBase64(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping oversized base64 test in -short mode")
    }
    vm := &visionMock{downloadBytes: []byte("x"), downloadCT: "image/png"}
    upstream := httptest.NewServer(vm.handler())
    defer upstream.Close()

    oldBase := zbridge.BASE_URL
    zbridge.BASE_URL = upstream.URL
    defer func() { zbridge.BASE_URL = oldBase }()
    defer zbridge.OverrideSessionState("test-token", "test-user", true)()

    cfg := zbridge.GetConfig()

    // Build a payload that decodes to maxImageSize+1 bytes.
    raw := bytes.Repeat([]byte{0x41}, (50<<20)+1)
    bigB64 := base64.StdEncoding.EncodeToString(raw)

    body := map[string]interface{}{
        "model":  "glm-5v-turbo",
        "stream": false,
        "messages": []map[string]interface{}{
            {"role": "user", "content": []map[string]interface{}{
                {"type": "text", "text": "big"},
                {"type": "image_url", "image_url": map[string]interface{}{
                    "url": "data:image/png;base64," + bigB64,
                }},
            }},
        },
    }
    bodyJSON, _ := json.Marshal(body)
    req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyJSON))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
    rec := httptest.NewRecorder()
    zbridge.NewHandler().ServeHTTP(rec, req)

    if rec.Code != 400 {
        t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
    }
    if !strings.Contains(rec.Body.String(), "max size") {
        t.Errorf("error should mention max size: %s", rec.Body.String())
    }
    if n := len(vm.getUploads()); n != 0 {
        t.Errorf("expected 0 uploads after size rejection, got %d", n)
    }
}
