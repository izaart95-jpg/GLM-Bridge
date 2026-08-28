// Code moved from the original main.go monolith during the internal/ restructure.
// See README "Project Structure". Part of the Z.AI bridge core (package zbridge).

package zbridge

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"
)

// fetchModelsFromZAI retrieves models from Z.AI /api/models,
// keeping only glm-4.7 and newer (the API returns newest-first).
func fetchModelsFromZAI() []ModelInfo {
    modelsCacheMu.Lock()
    defer modelsCacheMu.Unlock()

    if len(modelsCache) > 0 && time.Since(modelsCacheTime) < modelsCacheTTL {
        return modelsCache
    }

    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, "GET", BASE_URL+"/api/models", nil)
    if err != nil {
        logError("fetchModels request: " + err.Error())
        if len(modelsCache) > 0 {
            return modelsCache
        }
        return fallbackModels
    }
    req.Header.Set("Accept", "application/json")
    req.Header.Set("authorization", "Bearer "+session.Token)
    req.Header.Set("User-Agent", zaiUserAgent)
    resp, err := zaiHTTPClient.Do(req)
    if err != nil {
        logError("fetchModels do: " + err.Error())
        if len(modelsCache) > 0 {
            return modelsCache
        }
        return fallbackModels
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        logError(fmt.Sprintf("fetchModels status: %d", resp.StatusCode))
        if len(modelsCache) > 0 {
            return modelsCache
        }
        return fallbackModels
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        logError("fetchModels read: " + err.Error())
        if len(modelsCache) > 0 {
            return modelsCache
        }
        return fallbackModels
    }

    var apiResp struct {
        Data []struct {
            ID      string `json:"id"`
            Name    string `json:"name"`
            Created int64  `json:"created"`
            Info    struct {
                Name string `json:"name"`
                Meta struct {
                    Description  string                 `json:"description"`
                    Capabilities map[string]interface{} `json:"capabilities"`
                } `json:"meta"`
            } `json:"info"`
        } `json:"data"`
    }

    if err := json.Unmarshal(body, &apiResp); err != nil {
        logError("fetchModels parse: " + err.Error())
        if len(modelsCache) > 0 {
            return modelsCache
        }
        return fallbackModels
    }

    var filtered []ModelInfo
    for _, m := range apiResp.Data {
        filtered = append(filtered, ModelInfo{
            ID:           m.ID,
            Name:         m.Name,
            Description:  m.Info.Meta.Description,
            Capabilities: m.Info.Meta.Capabilities,
            Created:      m.Created,
        })
        if m.ID == "glm-4.7" {
            break
        }
    }
    
    if len(filtered) > 0 {
        modelsCache = filtered
        modelsCacheTime = time.Now()
        logInfo(fmt.Sprintf("Fetched %d models from Z.AI", len(filtered)))
    }

    if len(modelsCache) > 0 {
        return modelsCache
    }
    return fallbackModels
}

// getFeaturesForModel maps a model's capabilities to a Features struct.
// enable_thinking defaults to true; web_search/auto_web_search default to false.
func getFeaturesForModel(modelID string) Features {
    f := Features{Thinking: true} // enable_thinking enabled by default
    for _, m := range fetchModelsFromZAI() {
        if strings.EqualFold(m.ID, modelID) {
            if v, ok := m.Capabilities["enable_thinking"].(bool); ok {
                f.Thinking = v
            }
            if v, ok := m.Capabilities["preview_mode"].(bool); ok {
                f.PreviewMode = v
            }
            break
        }
    }
    return f
}

// getModelCapabilities returns the raw capabilities map for a model.
func getModelCapabilities(modelID string) map[string]interface{} {
    for _, m := range fetchModelsFromZAI() {
        if strings.EqualFold(m.ID, modelID) {
            return m.Capabilities
        }
    }
    return nil
}

// modelSupportsReasoningEffort returns true only when the model's capabilities
// JSON explicitly contains "reasoning_effort": true.
// Models with "reasoning_effort": false or without the field are NOT supported.
func modelSupportsReasoningEffort(modelID string) bool {
    if modelID == "" {
        return false
    }
    caps := getModelCapabilities(modelID)
    if caps == nil {
        return false
    }
    v, ok := caps["reasoning_effort"].(bool)
    return ok && v
}

// isValidReasoningEffort validates the accepted reasoning_effort values.
// Accepted: "high", "max". Any other value is rejected.
func isValidReasoningEffort(value string) bool {
    switch value {
    case "high", "max":
        return true
    default:
        return false
    }
}

// modelSupportsVision returns true only when the model's capabilities JSON
// explicitly contains "vision": true. Models without the field (or with it
// set to false) are treated as text-only.
func modelSupportsVision(modelID string) bool {
    if modelID == "" {
        return false
    }
    caps := getModelCapabilities(modelID)
    if caps == nil {
        return false
    }
    v, ok := caps["vision"].(bool)
    return ok && v
}

// architectureFor builds the OpenAI-style architecture object for a model.
// Vision-capable models accept text+image input; everything is text-output.
func architectureFor(caps map[string]interface{}) map[string]interface{} {
    vision := false
    if caps != nil {
        if v, ok := caps["vision"].(bool); ok {
            vision = v
        }
    }
    inputModalities := []string{"text"}
    modality := "text->text"
    if vision {
        inputModalities = []string{"text", "image"}
        modality = "text+image->text"
    }
    return map[string]interface{}{
        "modality":           modality,
        "input_modalities":   inputModalities,
        "output_modalities":  []string{"text"},
    }
}

func modelsHandler(w http.ResponseWriter, r *http.Request) {
    now := time.Now().Unix()
    models := fetchModelsFromZAI()
    data := make([]map[string]interface{}, 0, len(models))
    for _, m := range models {
        created := m.Created
        if created == 0 {
            created = now
        }
        entry := map[string]interface{}{
            "id":           m.ID,
            "object":       "model",
            "created":      created,
            "owned_by":     "z-ai",
            "display_name": m.Name,
            "description":  m.Description,
            "architecture": architectureFor(m.Capabilities),
        }
        data = append(data, entry)
    }
    writeJSON(w, 200, map[string]interface{}{
        "object": "list",
        "data":   data,
    })
}

func modelsHandler2(w http.ResponseWriter, r *http.Request) {
    models := fetchModelsFromZAI()
    ids := make([]string, 0, len(models))
    for _, m := range models {
        ids = append(ids, m.ID)
    }
    currentModel := "glm-5.2"
    if len(ids) > 0 {
        currentModel = ids[0]
    }
    writeJSON(w, 200, map[string]interface{}{
        "models":       ids,
        "currentModel": currentModel,
    })
}

// ============================================================================
// AGENT MODE (LEGACY SHIM) — Tools & Role Translation for Z.AI Compatibility
// ============================================================================
//
// NOTE: This is the LEGACY agent-mode shim, kept for backward compatibility
// (select it with --agent-mode-variant=legacy / AGENT_MODE_VARIANT=legacy).
// The default MODERN shim — XML-sectioned prompt, history summarization,
// tolerant marker/fence/payload parsing — lives in agent.go and is ported
// from the DeepseekFreeAPI reference implementation.
//
// Z.AI's unofficial /api/v2/chat/completions endpoint only accepts messages
// with role="user". System, assistant, and tool roles cause INTERNAL_ERROR.
// OpenAI-style tools/function_calls are also rejected.
//
// The legacy agent mode performs three transformations when active:
//
//   1. Mandatory System Prefix: A user message is prepended explaining the
//      prompt architecture (roles, tools) so the model can interpret the
//      rewritten conversation correctly.
//
//   2. Role Replacement: Every non-user message is rewritten as a user
//      message with a [ROLE: <original_role>] tag prepended to its content.
//      e.g. system message "Do X" becomes user message "[ROLE: system] Do X".
//
//   3. Tool Translation & Simulation: OpenAI tools JSON is rendered into a
//      user message with a strict contract: the model MUST emit any tool
//      invocation as a fenced JSON block of the form
//
//          <<<TOOL_CALL>>>
//          {"name":"<tool_name>","arguments":{...}}
//          <<<END_TOOL_CALL>>>
