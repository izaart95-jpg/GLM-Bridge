// Vision (image input) support for the Z.AI bridge.
//
// Both the OpenAI (/v1/chat/completions) and Anthropic (/v1/messages) surfaces
// accept image inputs, but Z.AI's /api/v2/chat/completions endpoint does not
// take inline image payloads — images must first be uploaded to Z.AI's file
// endpoint and then referenced via a top-level "files" array on the completion
// request (this mirrors what the official chat.z.ai web client does).
//
// Pipeline (processVisionMessages):
//
//  1. Scan the OpenAI-formatted messages for image_url parts (the Anthropic
//     handler converts its own image blocks into this shape first).
//  2. Resolve each image to raw bytes: http(s) URLs are downloaded, data: URLs
//     are base64-decoded. Both paths enforce maxImageSize.
//  3. Upload each image to POST {BASE_URL}/api/v1/files/ as multipart/form-data
//     (field name "file"), authenticated with the active Z.AI token.
//  4. Build one "files" entry per upload in the exact shape the web client uses.
//  5. Rewrite the messages to strip the image parts (keeping the text), so the
//     prompt/upstream messages stay text-only while the files carry the pixels.
//
// Limits: maxImagesPerRequest images per request, each at most maxImageSize.
// Text-only requests are returned byte-for-byte unchanged (no "files" field is
// ever added to the upstream body in that case).
//
// Auth: the file-upload endpoint requires a LOGGED-IN account token. Guest
// sessions (no ZAI_TOKEN) are rejected with 401 on upload, so vision input is
// effectively a ZAI_TOKEN-only feature; model listing and text completions are
// unaffected.

package zbridge

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "mime/multipart"
    "net/http"
    "net/textproto"
    "net/url"
    "path"
    "strings"
    "sync"
    "time"
)

// ============================================================================
// LIMITS & HTTP CLIENT
// ============================================================================

const (
    // maxImagesPerRequest bounds how many images one completion request may
    // carry (Z.AI web client caps attachments; this keeps uploads bounded).
    maxImagesPerRequest = 10
    // maxImageSize bounds a single image (bytes).
    maxImageSize = int64(50 << 20) // 50 MB
    // imageDownloadTimeout bounds one external image download.
    imageDownloadTimeout = 60 * time.Second
    // imageUploadTimeout bounds one upload to Z.AI /api/v1/files/.
    imageUploadTimeout = 120 * time.Second
    // imageUploadWorkers bounds concurrent resolve+upload goroutines.
    imageUploadWorkers = 4
)

// imageHTTPClient fetches external images. Unlike zaiHTTPClient (uTLS Chrome
// fingerprint for the Z.AI WAF) this is a plain client — arbitrary third-party
// image hosts do not need fingerprint spoofing, but we do honour the standard
// proxy environment variables. Per-request context bounds each download.
var imageHTTPClient = &http.Client{
    Transport: &http.Transport{
        Proxy:               http.ProxyFromEnvironment,
        MaxIdleConns:        50,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
    },
}

// resolvedImage is one image reduced to raw bytes plus the metadata needed to
// upload it as a believable file.
type resolvedImage struct {
    data        []byte
    filename    string
    contentType string
}

// ============================================================================
// ENTRY POINT
// ============================================================================

// processVisionMessages scans an OpenAI-formatted messages array for image_url
// parts, uploads every image to Z.AI, and returns:
//
//   - rewritten: the messages where every image_url part's "url" has been
//     REPLACED with the uploaded file's id (this is how the official web client
//     references attachments — the part stays, its url becomes the file UUID).
//     Equal to the input bytes when no images are present.
//   - files: one "files" entry per uploaded image (nil when none).
//
// The returned error is non-nil when the request is invalid (too many images)
// or any download/upload failed; callers should surface it as a 400.
func processVisionMessages(ctx context.Context, raw json.RawMessage) (json.RawMessage, []map[string]interface{}, error) {
    var messages []map[string]json.RawMessage
    if err := json.Unmarshal(raw, &messages); err != nil {
        // Not a JSON array of objects — nothing to extract; pass through.
        return raw, nil, nil
    }

    type foundImage struct {
        msgIndex  int
        partIndex int
        url       string
    }
    var found []foundImage
    imageMsgIdx := make(map[int]bool)

    // ── Pass 1: locate every image_url part ──
    for i, msg := range messages {
        contentRaw, ok := msg["content"]
        if !ok || len(contentRaw) == 0 {
            continue
        }
        var parts []map[string]interface{}
        if err := json.Unmarshal(contentRaw, &parts); err != nil {
            continue // string (or non-array) content carries no images
        }
        for p, part := range parts {
            if u := extractImageURL(part); u != "" {
                found = append(found, foundImage{msgIndex: i, partIndex: p, url: u})
                imageMsgIdx[i] = true
            }
        }
    }

    if len(found) == 0 {
        return raw, nil, nil
    }
    if len(found) > maxImagesPerRequest {
        return nil, nil, fmt.Errorf(
            "too many images: %d provided, max %d per request", len(found), maxImagesPerRequest)
    }

    // One ref_user_msg_id per image-bearing message (the web client ties each
    // attachment to the user message it was sent with).
    refIDs := make(map[int]string)
    for _, f := range found {
        if _, ok := refIDs[f.msgIndex]; !ok {
            refIDs[f.msgIndex] = randomUUID()
        }
    }

    // ── Pass 2: resolve + upload with bounded concurrency (order-preserving) ──
    entries := make([]map[string]interface{}, len(found))
    errs := make([]error, len(found))
    sem := make(chan struct{}, imageUploadWorkers)
    var wg sync.WaitGroup
    for idx, f := range found {
        wg.Add(1)
        go func(idx int, f foundImage) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()

            img, err := resolveImage(ctx, f.url)
            if err != nil {
                errs[idx] = err
                return
            }
            fileObj, err := uploadImageToZAI(ctx, img)
            if err != nil {
                errs[idx] = err
                return
            }
            entries[idx] = buildZAIFileEntry(fileObj, img, refIDs[f.msgIndex])
        }(idx, f)
    }
    wg.Wait()

    var errMsgs []string
    for _, e := range errs {
        if e != nil {
            errMsgs = append(errMsgs, e.Error())
        }
    }
    if len(errMsgs) > 0 {
        return nil, nil, fmt.Errorf("image processing failed: %s", strings.Join(errMsgs, "; "))
    }

    // ── Pass 3: point every image_url part at its uploaded file id ──
    // Group file ids by message so each content array is rewritten once.
    fileIDByMsg := make(map[int]map[int]string) // msgIndex -> partIndex -> file id
    for idx, f := range found {
        if fileIDByMsg[f.msgIndex] == nil {
            fileIDByMsg[f.msgIndex] = make(map[int]string)
        }
        id, _ := entries[idx]["id"].(string)
        fileIDByMsg[f.msgIndex][f.partIndex] = id
    }

    for i := range messages {
        if !imageMsgIdx[i] {
            continue
        }
        msg := messages[i]
        contentRaw, ok := msg["content"]
        if !ok {
            continue
        }
        var parts []map[string]interface{}
        if err := json.Unmarshal(contentRaw, &parts); err != nil {
            continue
        }
        for p, part := range parts {
            fileID, ok := fileIDByMsg[i][p]
            if !ok || fileID == "" {
                continue
            }
            if iu, ok := part["image_url"].(map[string]interface{}); ok {
                iu["url"] = fileID // the web client's reference form
            }
        }
        enc, err := json.Marshal(parts)
        if err != nil {
            return nil, nil, fmt.Errorf("re-encode message content: %s", err.Error())
        }
        msg["content"] = enc
    }

    rewritten, err := json.Marshal(messages)
    if err != nil {
        return nil, nil, fmt.Errorf("re-encode messages: %s", err.Error())
    }
    return rewritten, entries, nil
}

// extractImageParts returns every image_url content part found in an
// OpenAI-formatted messages array, in message order. After
// processVisionMessages has run, each returned part's "url" is the uploaded
// Z.AI file id — the reference the model needs to actually receive the
// pixels. Used to re-attach image references after transformations that
// flatten message content to plain text (see agentTransformMessages).
func extractImageParts(raw json.RawMessage) []map[string]interface{} {
    var messages []map[string]json.RawMessage
    if err := json.Unmarshal(raw, &messages); err != nil {
        return nil
    }
    var parts []map[string]interface{}
    for _, msg := range messages {
        contentRaw, ok := msg["content"]
        if !ok || len(contentRaw) == 0 {
            continue
        }
        var msgParts []map[string]interface{}
        if err := json.Unmarshal(contentRaw, &msgParts); err != nil {
            continue // string (or non-array) content carries no images
        }
        for _, p := range msgParts {
            if extractImageURL(p) != "" {
                parts = append(parts, p)
            }
        }
    }
    return parts
}

// attachImageParts appends image_url parts to the LAST user message of a
// transformed messages array, converting string content to the typed-parts
// form when needed. Returns the re-encoded messages, or an error when the
// input is not a messages array (in which case the caller keeps the original).
func attachImageParts(transformed json.RawMessage, imageParts []map[string]interface{}) ([]byte, error) {
    if len(imageParts) == 0 {
        return transformed, nil
    }
    var messages []map[string]json.RawMessage
    if err := json.Unmarshal(transformed, &messages); err != nil || len(messages) == 0 {
        return nil, fmt.Errorf("attach image parts: not a messages array")
    }

    // Locate the last user message (fallback: the last message overall).
    target := -1
    for i := len(messages) - 1; i >= 0; i-- {
        var role string
        if raw, ok := messages[i]["role"]; ok {
            _ = json.Unmarshal(raw, &role)
        }
        if role == "user" {
            target = i
            break
        }
    }
    if target < 0 {
        target = len(messages) - 1
    }

    msg := messages[target]
    contentRaw := msg["content"]

    var newParts []map[string]interface{}
    trimmed := bytes.TrimSpace(contentRaw)
    if len(trimmed) > 0 && trimmed[0] == '[' {
        // Already a parts array — append to it.
        if err := json.Unmarshal(trimmed, &newParts); err != nil {
            return nil, fmt.Errorf("attach image parts: parse content array: %s", err.Error())
        }
    } else {
        // String (or other scalar) content — promote to a text part first.
        var s string
        if err := json.Unmarshal(trimmed, &s); err == nil && s != "" {
            newParts = []map[string]interface{}{{"type": "text", "text": s}}
        }
    }
    newParts = append(newParts, imageParts...)

    enc, err := json.Marshal(newParts)
    if err != nil {
        return nil, fmt.Errorf("attach image parts: encode content: %s", err.Error())
    }
    msg["content"] = enc

    out, err := json.Marshal(messages)
    if err != nil {
        return nil, fmt.Errorf("attach image parts: encode messages: %s", err.Error())
    }
    return out, nil
}

// ============================================================================
// IMAGE DETECTION & RESOLUTION
// ============================================================================

// extractImageURL returns the image URL from an OpenAI image_url content part,
// or "" if the part is not an image. Accepts the standard shape
// {"type":"image_url","image_url":{"url":"..."}}.
func extractImageURL(part map[string]interface{}) string {
    if part == nil {
        return ""
    }
    if t, _ := part["type"].(string); t != "image_url" {
        return ""
    }
    iu, ok := part["image_url"].(map[string]interface{})
    if !ok {
        return ""
    }
    u, _ := iu["url"].(string)
    return strings.TrimSpace(u)
}

// resolveImage reduces one image reference (http(s) URL or data: URL) to raw
// bytes plus filename/content-type metadata, enforcing maxImageSize.
func resolveImage(ctx context.Context, rawURL string) (*resolvedImage, error) {
    rawURL = strings.TrimSpace(rawURL)
    if rawURL == "" {
        return nil, fmt.Errorf("empty image URL")
    }
    if strings.HasPrefix(rawURL, "data:") {
        return resolveDataURL(rawURL)
    }
    return downloadImage(ctx, rawURL)
}

// resolveDataURL decodes a base64 data: URL (data:<mime>;base64,<payload>).
func resolveDataURL(rawURL string) (*resolvedImage, error) {
    comma := strings.Index(rawURL, ",")
    if comma < 0 {
        return nil, fmt.Errorf("malformed data URL (missing ',')")
    }
    const prefix = "data:"
    if comma < len(prefix) {
        return nil, fmt.Errorf("malformed data URL")
    }
    header := rawURL[len(prefix):comma]
    payload := rawURL[comma+1:]

    segs := strings.Split(header, ";")
    mime := strings.TrimSpace(segs[0])
    base64Flag := false
    for _, s := range segs[1:] {
        if strings.EqualFold(strings.TrimSpace(s), "base64") {
            base64Flag = true
        }
    }
    if !base64Flag {
        return nil, fmt.Errorf("unsupported data URL encoding (only ;base64 is supported)")
    }

    data, err := base64.StdEncoding.DecodeString(payload)
    if err != nil {
        // Fall back to the lenient decoder (handles missing padding / URL-safe).
        data, err = base64Decode(payload)
        if err != nil {
            return nil, fmt.Errorf("invalid base64 payload in data URL: %s", err.Error())
        }
    }
    if int64(len(data)) > maxImageSize {
        return nil, fmt.Errorf("image exceeds max size of %d bytes", maxImageSize)
    }
    if mime == "" {
        mime = http.DetectContentType(data)
    }
    return &resolvedImage{
        data:        data,
        filename:    "image" + extFromMime(mime),
        contentType: mime,
    }, nil
}

// downloadImage fetches an http(s) image, capping the read at maxImageSize.
func downloadImage(ctx context.Context, rawURL string) (*resolvedImage, error) {
    u, err := url.Parse(rawURL)
    if err != nil {
        return nil, fmt.Errorf("invalid image URL %q: %s", rawURL, err.Error())
    }
    if u.Scheme != "http" && u.Scheme != "https" {
        return nil, fmt.Errorf("unsupported image URL scheme %q", u.Scheme)
    }

    dctx, cancel := context.WithTimeout(ctx, imageDownloadTimeout)
    defer cancel()
    req, err := http.NewRequestWithContext(dctx, "GET", rawURL, nil)
    if err != nil {
        return nil, fmt.Errorf("image download request: %s", err.Error())
    }
    req.Header.Set("User-Agent", zaiUserAgent)
    req.Header.Set("Accept", "image/*,*/*;q=0.8")

    resp, err := imageHTTPClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("image download failed: %s", err.Error())
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("image download returned status %d for %s", resp.StatusCode, rawURL)
    }

    data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize+1))
    if err != nil {
        return nil, fmt.Errorf("image download read: %s", err.Error())
    }
    if int64(len(data)) > maxImageSize {
        return nil, fmt.Errorf("image exceeds max size of %d bytes", maxImageSize)
    }

    // Content type: prefer a real detected image type, else the header.
    ct := http.DetectContentType(data)
    if !strings.HasPrefix(ct, "image/") {
        if hct := strings.TrimSpace(strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0]); strings.HasPrefix(hct, "image/") {
            ct = hct
        }
    }

    filename := filenameFromURL(u)
    if filename == "" {
        filename = "image" + extFromMime(ct)
    }
    return &resolvedImage{data: data, filename: filename, contentType: ct}, nil
}

// ============================================================================
// UPLOAD TO Z.AI
// ============================================================================

// uploadImageToZAI POSTs one image to {BASE_URL}/api/v1/files/ as
// multipart/form-data (field "file") and returns the parsed file object. It
// re-initialises the Z.AI session once on a 401, mirroring DeleteZAIChat.
//
// NOTE: the /api/v1/files/ endpoint only accepts a LOGGED-IN account token.
// Guest sessions (no ZAI_TOKEN) are rejected with 401 even though the same
// guest token works for /api/models and chat completions. In that case the
// retry re-initialises a fresh guest session, is rejected again, and the
// caller surfaces "file upload unauthorized (401)" as a 400. Vision therefore
// requires ZAI_TOKEN to be set.
func uploadImageToZAI(ctx context.Context, img *resolvedImage) (map[string]interface{}, error) {
    var lastErr error
    for attempt := 0; attempt < 2; attempt++ {
        session.mu.Lock()
        token := session.Token
        initialized := session.Initialized
        session.mu.Unlock()

        if token == "" || !initialized {
            if err := initializeSession(); err != nil {
                return nil, fmt.Errorf("session init for file upload: %s", err.Error())
            }
            continue // re-read the fresh token
        }

        var body bytes.Buffer
        mw := multipart.NewWriter(&body)
        h := make(textproto.MIMEHeader)
        h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, img.filename))
        if img.contentType != "" {
            h.Set("Content-Type", img.contentType)
        }
        part, err := mw.CreatePart(h)
        if err != nil {
            return nil, fmt.Errorf("file upload: build multipart: %s", err.Error())
        }
        if _, err := part.Write(img.data); err != nil {
            return nil, fmt.Errorf("file upload: write part: %s", err.Error())
        }
        if err := mw.Close(); err != nil {
            return nil, fmt.Errorf("file upload: close multipart: %s", err.Error())
        }

        uctx, cancel := context.WithTimeout(ctx, imageUploadTimeout)
        req, err := http.NewRequestWithContext(uctx, "POST", BASE_URL+"/api/v1/files/", &body)
        if err != nil {
            cancel()
            return nil, fmt.Errorf("file upload: build request: %s", err.Error())
        }
        req.Header.Set("authorization", "Bearer "+token)
        req.Header.Set("User-Agent", zaiUserAgent)
        req.Header.Set("Accept", "application/json")
        req.Header.Set("Content-Type", mw.FormDataContentType())

        if config.Logging.Level == "debug" {
            log.Printf("[DEBUG] Z.AI file upload: POST %s filename=%s size=%d ct=%s",
                BASE_URL+"/api/v1/files/", img.filename, len(img.data), img.contentType)
        }

        resp, err := zaiHTTPClient.Do(req)
        if err != nil {
            cancel()
            return nil, fmt.Errorf("file upload connection error: %s", err.Error())
        }
        respBody, _ := io.ReadAll(resp.Body)
        resp.Body.Close()
        cancel()

        if config.Logging.Level == "debug" {
            log.Printf("[DEBUG] Z.AI file upload response: %d %s",
                resp.StatusCode, truncateString(string(respBody), 400))
        }

        switch {
        case resp.StatusCode == 401:
            session.mu.Lock()
            session.Initialized = false
            session.mu.Unlock()
            lastErr = fmt.Errorf("file upload unauthorized (401)")
            continue
        case resp.StatusCode >= 200 && resp.StatusCode < 300:
            var fileObj map[string]interface{}
            if err := json.Unmarshal(respBody, &fileObj); err != nil {
                return nil, fmt.Errorf("file upload: invalid JSON response: %s", err.Error())
            }
            if id, _ := fileObj["id"].(string); id == "" {
                return nil, fmt.Errorf("file upload: response missing file id: %s",
                    truncateString(string(respBody), 300))
            }
            return fileObj, nil
        default:
            return nil, fmt.Errorf("file upload failed: %d: %s",
                resp.StatusCode, truncateString(string(respBody), 300))
        }
    }
    if lastErr == nil {
        lastErr = fmt.Errorf("file upload: max retries exceeded")
    }
    return nil, lastErr
}

// ============================================================================
// FILES PAYLOAD
// ============================================================================

// buildZAIFileEntry shapes one uploaded file into the exact "files" array entry
// the chat.z.ai web client sends alongside a completion request.
func buildZAIFileEntry(fileObj map[string]interface{}, img *resolvedImage, refUserMsgID string) map[string]interface{} {
    id, _ := fileObj["id"].(string)

    name := img.filename
    if fn, ok := fileObj["filename"].(string); ok && fn != "" {
        name = fn
    }

    size := int64(len(img.data))
    if meta, ok := fileObj["meta"].(map[string]interface{}); ok {
        if s, ok := meta["size"].(float64); ok && s > 0 {
            size = int64(s)
        }
    }

    return map[string]interface{}{
        "type":            "image",
        "file":            fileObj,
        "id":              id,
        "url":             "/api/v1/files/" + id,
        "name":            name,
        "status":          "uploaded",
        "size":            size,
        "error":           "",
        "itemId":          randomUUID(),
        "media":           "image",
        "uploadedAt":      time.Now().UnixMilli(),
        "ref_user_msg_id": refUserMsgID,
    }
}

// ============================================================================
// SMALL HELPERS
// ============================================================================

// extFromMime maps a MIME type to a file extension (with leading dot).
func extFromMime(mime string) string {
    switch strings.ToLower(strings.TrimSpace(mime)) {
    case "image/png":
        return ".png"
    case "image/jpeg", "image/jpg":
        return ".jpg"
    case "image/gif":
        return ".gif"
    case "image/webp":
        return ".webp"
    case "image/bmp":
        return ".bmp"
    case "image/svg+xml":
        return ".svg"
    case "image/tiff":
        return ".tiff"
    case "image/avif":
        return ".avif"
    default:
        return ".bin"
    }
}

// filenameFromURL derives a sanitized filename from a URL path, or "" if none.
func filenameFromURL(u *url.URL) string {
    base := strings.TrimSpace(path.Base(u.Path))
    if base == "" || base == "." || base == "/" {
        return ""
    }
    var sb strings.Builder
    for _, r := range base {
        switch {
        case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
            r == '-' || r == '_' || r == '.':
            sb.WriteRune(r)
        default:
            sb.WriteRune('_')
        }
    }
    out := sb.String()
    if len(out) > 100 {
        out = out[:100]
    }
    return out
}

// truncateString clips s to at most n runes, appending an ellipsis when cut.
func truncateString(s string, n int) string {
    if len(s) <= n {
        return s
    }
    return s[:n] + "..."
}
