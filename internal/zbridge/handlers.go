// Code moved from the original main.go monolith during the internal/ restructure.
// See README "Project Structure". Part of the Z.AI bridge core (package zbridge).

package zbridge

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "log"
    "net/http"
    "strings"
    "sync"
    "time"
)

func chatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var body struct {
        Model           string          `json:"model"`
        Messages        json.RawMessage `json:"messages"`
        Stream          *bool           `json:"stream"`
        Reasoning       *bool           `json:"reasoning"`
        Thinking        json.RawMessage `json:"thinking"`
        WebSearch       *bool           `json:"webSearch"`
        Search          *bool           `json:"search"`
        Tools           json.RawMessage `json:"tools"`
        ToolChoice      json.RawMessage `json:"tool_choice"`
        ReasoningEffort string          `json:"reasoning_effort"`
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

    // ── Vision: extract image_url parts, upload them to Z.AI, strip them ──
    // from the messages. cleanedMessages is byte-identical to body.Messages
    // when the request carries no images (the common case).
    cleanedMessages, files, vErr := processVisionMessages(r.Context(), body.Messages)
    if vErr != nil {
        writeJSON(w, 400, formatOpenAIError(vErr.Error(), "invalid_request_error", nil))
        return
    }
    if len(files) > 0 {
        if !modelSupportsVision(model) {
            log.Printf("[Vision] %d image(s) attached but model %q does not advertise vision support; forwarding anyway", len(files), model)
        }
        // Re-parse the cleaned (text-only) messages for prompt building.
        var localMsgs []Message
        if err := json.Unmarshal(cleanedMessages, &localMsgs); err == nil {
            messages = localMsgs
        }
    }

    stream := true
    if body.Stream != nil {
        stream = *body.Stream
    }

    // Stateless request: run it on a THROWAWAY chat session (ported from the
    // DeepseekFreeAPI reference). Async mode takes a pre-made session from
    // the standing pool batch; sync mode mints one on the spot. Either way
    // the chat is deleted on Z.AI once the response is fully processed
    // (deferred below), so no server-side history survives the request —
    // this is what keeps the account from accumulating dead sessions and
    // stops stale server-side context from rotting later requests.
    chatID, pooled, err := AcquireStatelessSession(r.Context())
    if err != nil {
        if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
            return // client gone; nothing to answer
        }
        writeJSON(w, 503, formatOpenAIError(err.Error(), "server_error", "shutting_down"))
        return
    }
    defer ReleaseStatelessSession(chatID, pooled)
    requestId := generateID()

    // ── Agent mode: transform tools & roles for Z.AI compatibility ──
    // Modern shim (default): one XML-sectioned prompt in a single user message.
    // Legacy shim: [ROLE: ...] rewritten user messages + tool contract message.
    // Operates on the cleaned (image-stripped) messages.
    var transformedMessages json.RawMessage = cleanedMessages
    if config.AgentMode {
        if tm, err := agentTransformMessages(cleanedMessages, body.Tools); err == nil {
            transformedMessages = tm
            // Re-parse so local `messages` reflects the rewritten content
            var localMsgs []Message
            if err := json.Unmarshal(tm, &localMsgs); err == nil {
                messages = localMsgs
            }
        } else {
            logError("agent transform failed: " + err.Error())
        }
    }

    prompt := messagesToPrompt(messages)

    // Features are now resolved per-model inside sendToZAI.
    // Per-request overrides are only set if explicitly provided in the body.
    opts := SendOptions{
        Model:             model,
        ChatID:            chatID,
        ClientMessagesRaw: transformedMessages,
        ReasoningEffort:   body.ReasoningEffort,
        Files:             files,
    }

    // Parse thinking configuration:
    //   reasoning: true/false  ->  enable_thinking
    //   "thinking": {"type":"enabled"|"disabled"}  ->  enable_thinking
    if body.Reasoning != nil {
        opts.Thinking = body.Reasoning
    } else if len(body.Thinking) > 0 {
        var thinkCfg struct {
            Type string `json:"type"`
        }
        if err := json.Unmarshal(body.Thinking, &thinkCfg); err == nil {
            enabled := thinkCfg.Type == "enabled"
            opts.Thinking = &enabled
        }
    }

    if body.WebSearch != nil {
        opts.WebSearch = body.WebSearch
    } else if body.Search != nil {
        opts.WebSearch = body.Search
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
        fullReasoning := ""

        var interceptor agentInterceptor
        if config.AgentMode {
            interceptor = newAgentInterceptor()
        }
        toolCallEmitted := false

        emitToolCallDelta := func(tc map[string]interface{}) {
            chunk := map[string]interface{}{
                "id":      "chatcmpl-" + requestId,
                "object":  "chat.completion.chunk",
                "created": time.Now().Unix(),
                "model":   model,
                "choices": []map[string]interface{}{
                    {
                        "index":         0,
                        "delta":         map[string]interface{}{"tool_calls": []map[string]interface{}{tc}},
                        "finish_reason": nil,
                    },
                },
            }
            writeSSE(toJSON(chunk))
        }

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
            writeSSE(toJSON(formatOpenAIError(err.Error(), "api_error", statusFromError(err.Error()))))
            writeSSE("[DONE]")
            errored = true
        } else {
            for result := range ch {
                if result.Err != nil {
                    log.Printf("[Stream] Error: %s", result.Err.Error())
                    writeSSE(toJSON(formatOpenAIError(result.Err.Error(), "api_error", statusFromError(result.Err.Error()))))
                    writeSSE("[DONE]")
                    errored = true
                    break
                }
                
                if result.Reasoning != "" {
                    fullReasoning += result.Reasoning
                    rChunk := map[string]interface{}{
                        "id":      "chatcmpl-" + requestId,
                        "object":  "chat.completion.chunk",
                        "created": time.Now().Unix(),
                        "model":   model,
                        "choices": []map[string]interface{}{
                            {
                                "index":         0,
                                "delta":         map[string]interface{}{"reasoning_content": result.Reasoning},
                                "finish_reason": nil,
                            },
                        },
                    }
                    writeSSE(toJSON(rChunk))
                    continue
                }
                if result.FullText != "" && !strings.HasPrefix(result.FullText, fullContent) {
                    // A deep edit_content rewrite rewound text that was
                    // already forwarded: the agent interceptor's view of
                    // the stream is stale, reset it (issue #23).
                    if interceptor != nil {
                        interceptor = newAgentInterceptor()
                    }
                }
                if result.FullText != "" {
                    fullContent = result.FullText
                } else {
                    fullContent += result.Chunk
                }

                // The parser emits the exact rune-safe delta to forward.
                delta := result.Chunk
                if delta == "" {
                    continue
                }

                if interceptor != nil {
                    contentDelta, toolCalls := interceptor.feed(delta)
                    if contentDelta != "" {
                        c := formatOpenAIResponse(ResponseResult{Content: contentDelta}, model, requestId, true)
                        writeSSE(toJSON(c))
                    }
                    for _, tc := range toolCalls {
                        emitToolCallDelta(tc)
                        toolCallEmitted = true
                    }
                } else {
                    c := formatOpenAIResponse(ResponseResult{Content: delta}, model, requestId, true)
                    writeSSE(toJSON(c))
                }
            }
        }

        if !errored {
            if interceptor != nil {
                // Drain the interceptor tail: trailing text plus any
                // tool call whose block only completed at end of stream
                // (the modern shim holds back a window while streaming).
                rem, tailCalls := interceptor.finish()
                if rem != "" && !toolCallEmitted {
                    c := formatOpenAIResponse(ResponseResult{Content: rem}, model, requestId, true)
                    writeSSE(toJSON(c))
                }
                for _, tc := range tailCalls {
                    emitToolCallDelta(tc)
                    toolCallEmitted = true
                }

                // Safety net: fallback tool call extraction at stream end
                if !toolCallEmitted {
                    fallbackCalls := agentExtractToolCalls(fullContent)
                    if len(fallbackCalls) > 0 {
                        for _, tc := range fallbackCalls {
                            emitToolCallDelta(tc)
                        }
                        toolCallEmitted = true
                    }
                }

                if toolCallEmitted {
                    finalChunk := map[string]interface{}{
                        "id":      "chatcmpl-" + requestId,
                        "object":  "chat.completion.chunk",
                        "created": time.Now().Unix(),
                        "model":   model,
                        "choices": []map[string]interface{}{
                            {
                                "index":         0,
                                "delta":         map[string]interface{}{},
                                "finish_reason": "tool_calls",
                            },
                        },
                    }
                    writeSSE(toJSON(finalChunk))
                } else {
                    finalChunk := formatOpenAIResponse(ResponseResult{Content: "", FinishReason: "stop"}, model, requestId, true)
                    writeSSE(toJSON(finalChunk))
                }
            } else {
                finalChunk := formatOpenAIResponse(ResponseResult{Content: "", FinishReason: "stop"}, model, requestId, true)
                writeSSE(toJSON(finalChunk))
            }
            writeSSE("[DONE]")
        }

        close(keepAliveStop)
        wg.Wait()

    } else {
        ch, err := sendToZAI(prompt, opts)
        if err != nil {
            log.Printf("[API] Error: %s", err.Error())
            writeJSON(w, statusFromError(err.Error()), formatOpenAIError(err.Error(), "api_error", nil))
            return
        }

        fullContent := ""
        fullReasoning := ""
        for result := range ch {
            if result.Err != nil {
                log.Printf("[API] Error: %s", result.Err.Error())
                writeJSON(w, statusFromError(result.Err.Error()), formatOpenAIError(result.Err.Error(), "api_error", nil))
                return
            }
            if result.Reasoning != "" {
                fullReasoning += result.Reasoning
                continue
            }
            if result.FullText != "" {
                fullContent = result.FullText
            } else {
                fullContent += result.Chunk
            }
        }

        // Agent-mode: parse out tool-call blocks for non-stream response
        if config.AgentMode {
            toolCalls := agentExtractToolCalls(fullContent)
            if len(toolCalls) > 0 {
                stripped := agentStripToolCalls(fullContent)
                writeJSON(w, 200, map[string]interface{}{
                    "id":      "chatcmpl-" + requestId,
                    "object":  "chat.completion",
                    "created": time.Now().Unix(),
                    "model":   model,
                    "choices": []map[string]interface{}{
                        {
                            "index": 0,
                            "message": func() map[string]interface{} {
                                m := map[string]interface{}{
                                    "role":       "assistant",
                                    "content":    stripped,
                                    "tool_calls": toolCalls,
                                }
                                if fullReasoning != "" {
                                    m["reasoning_content"] = fullReasoning
                                }
                                return m
                            }(),
                            "finish_reason": "tool_calls",
                        },
                    },
                    "usage": map[string]interface{}{
                        "prompt_tokens":     estimateTokens(prompt),
                        "completion_tokens": estimateTokens(fullContent),
                        "total_tokens":      estimateTokens(prompt) + estimateTokens(fullContent),
                    },
                })
                return
            }
        }

        writeJSON(w, 200, formatOpenAIResponse(ResponseResult{Content: fullContent, Reasoning: fullReasoning}, model, requestId, false))
    }
}
func featuresHandler(w http.ResponseWriter, r *http.Request) {
    // ── GET: return resolved features for a model ──
    if r.Method == "GET" {
        model := r.URL.Query().Get("model")
        if model != "" {
            resolved := resolveFeaturesForModel(model)
            state := getModelFeatureState(model)
            caps := getModelCapabilities(model)
            writeJSON(w, 200, map[string]interface{}{
                "model":       model,
                "features":    resolved,
                "includeAll":  state.IncludeAll,
                "overrides":   state.Overrides,
                "capabilities": caps,
            })
            return
        }
        // No model specified — return all per-model states
        modelFeatureStatesMu.Lock()
        states := make(map[string]interface{})
        for k, v := range modelFeatureStates {
            states[k] = map[string]interface{}{
                "includeAll": v.IncludeAll,
                "overrides":  v.Overrides,
            }
        }
        modelFeatureStatesMu.Unlock()
        writeJSON(w, 200, map[string]interface{}{
            "states": states,
        })
        return
    }

    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // ── POST: update per-model feature state ──

    bodyBytes, err := io.ReadAll(r.Body)
    if err != nil {
        writeJSON(w, 400, map[string]interface{}{"error": "Failed to read body"})
        return
    }

    // Parse as raw map to capture arbitrary capability keys
    var body map[string]interface{}
    if err := json.Unmarshal(bodyBytes, &body); err != nil {
        writeJSON(w, 400, map[string]interface{}{"error": "Invalid JSON"})
        return
    }

    model, _ := body["model"].(string)
    if model == "" {
        writeJSON(w, 400, map[string]interface{}{"error": "model is required"})
        return
    }

    // Check Include-All-Features header
    includeAllHeader := strings.EqualFold(r.Header.Get("Include-All-Features"), "true")

    modelFeatureStatesMu.Lock()
    state, ok := modelFeatureStates[model]
    if !ok {
        state = &ModelFeatureState{
            IncludeAll: false,
            Overrides:  make(map[string]interface{}),
        }
        modelFeatureStates[model] = state
    }

    // Set IncludeAll flag if header is present
    if includeAllHeader {
        state.IncludeAll = true
    }

    // Process user overrides — any key except "model" is treated as a feature override.
    // Special handling: reasoning/thinking -> enable_thinking
    for k, v := range body {
        if k == "model" {
            continue
        }

        // reasoning: true/false -> enable_thinking
        if k == "reasoning" {
            if b, ok := v.(bool); ok {
                state.Overrides["enable_thinking"] = b
            }
            continue
        }

        // "thinking": {"type":"enabled"|"disabled"} or thinking: true/false -> enable_thinking
        if k == "thinking" {
            if b, ok := v.(bool); ok {
                state.Overrides["enable_thinking"] = b
                continue
            }
            if m, ok := v.(map[string]interface{}); ok {
                if t, ok := m["type"].(string); ok {
                    state.Overrides["enable_thinking"] = (t == "enabled")
                }
                continue
            }
            continue
        }

        // All other keys: convert camelCase to snake_case (no alias mapping)
        snakeKey := normalizeFeatureKey(k)
        // image_generation overrides are ignored — always forced false
        if snakeKey == "image_generation" {
            continue
        }
        // 'think' is not accepted — use enable_thinking, reasoning, or thinking
        if snakeKey == "think" {
            continue
        }
        // reasoning_effort is a per-request parameter validated against model
        // capabilities; it is NOT stored as a persistent override.
        if snakeKey == "reasoning_effort" {
            continue
        }
        state.Overrides[snakeKey] = v
    }

    // Resolve final features for response
    caps := getModelCapabilities(model)
    resolved := resolveFeaturesWithState(caps, state)
    includeAll := state.IncludeAll
    overrides := make(map[string]interface{})
    for k, v := range state.Overrides {
        overrides[k] = v
    }
    modelFeatureStatesMu.Unlock()

    // Update session.Features for backward compat (dashboard display)
    session.mu.Lock()
    if v, ok := resolved["auto_web_search"].(bool); ok {
        session.Features.WebSearch = v
        session.Features.AutoWebSearch = v
    }
    if v, ok := resolved["enable_thinking"].(bool); ok {
        session.Features.Thinking = v
    }
    if v, ok := resolved["preview_mode"].(bool); ok {
        session.Features.PreviewMode = v
    }
    session.Features.ImageGen = false
    session.mu.Unlock()

    log.Printf("[Features] model=%s includeAll=%v overrides=%+v resolved=%+v",
        model, includeAll, overrides, resolved)

    writeJSON(w, 200, map[string]interface{}{
        "success":    true,
        "model":      model,
        "includeAll": includeAll,
        "overrides":  overrides,
        "features":   resolved,
    })
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
    session.mu.Lock()
    initialized := session.Initialized
    session.mu.Unlock()

    totalClients := 0
    if initialized {
        totalClients = 1
    }

    writeJSON(w, 200, map[string]interface{}{
        "mode":         "direct",
        "totalClients": totalClients,
        "stats": map[string]interface{}{
            "totalRequests": 0,
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

