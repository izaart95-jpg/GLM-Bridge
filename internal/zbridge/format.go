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
