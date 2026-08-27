// Code moved from the original main.go monolith during the internal/ restructure.
// See README "Project Structure". Part of the Z.AI bridge core (package zbridge).

package zbridge

import (
    "time"
)

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
                "message": func() map[string]interface{} {
                    m := map[string]interface{}{
                        "role":    "assistant",
                        "content": rawContent,
                    }
                    if result.Reasoning != "" {
                        m["reasoning_content"] = result.Reasoning
                    }
                    return m
                }(),
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

// ============================================================================
// ANTHROPIC /v1/messages PROTOCOL SUPPORT  [ANTHROPIC_PROTOCOL_PATCHED]
// ============================================================================
//
// Exposes the Anthropic Messages API at /v1/messages. Feeds directly on the
// ZAIResult stream produced by sendToZAI — the same stream powering the
// OpenAI handler — and converts each chunk to Anthropic SSE events on the
// fly with zero intermediate allocations beyond the event JSON itself.
//
