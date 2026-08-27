// Code moved from the original main.go monolith during the internal/ restructure.
// See README "Project Structure". Part of the Z.AI bridge core (package zbridge).

package zbridge

import (
    "strings"
)

func normalizeFeatureKey(k string) string {
    var sb strings.Builder
    for i, r := range k {
        if i > 0 && r >= 'A' && r <= 'Z' {
            sb.WriteByte('_')
        }
        if r >= 'A' && r <= 'Z' {
            sb.WriteRune(r + 32)
        } else {
            sb.WriteRune(r)
        }
    }
    return sb.String()
}

// getModelFeatureState returns the per-model state, creating it if necessary.
func getModelFeatureState(modelID string) *ModelFeatureState {
    modelFeatureStatesMu.Lock()
    defer modelFeatureStatesMu.Unlock()
    if s, ok := modelFeatureStates[modelID]; ok {
        return s
    }
    s := &ModelFeatureState{
        IncludeAll: false,
        Overrides:  make(map[string]interface{}),
    }
    modelFeatureStates[modelID] = s
    return s
}

// resolveFeaturesForModel computes the final feature map for /completions.
//
// Rules:
//   - By default, auto_web_search and web_search are NOT included.
//   - enable_thinking defaults to true for all models.
//   - 'think' is never included — only enable_thinking reaches the request.
//   - IncludeAll: include ALL server capabilities.
//   - User overrides always take precedence.
//   - image_generation is ALWAYS forced to false.
func resolveFeaturesForModel(modelID string) map[string]interface{} {
    caps := getModelCapabilities(modelID)
    modelFeatureStatesMu.Lock()
    state, ok := modelFeatureStates[modelID]
    modelFeatureStatesMu.Unlock()
    if !ok {
        state = &ModelFeatureState{
            IncludeAll: false,
            Overrides:  make(map[string]interface{}),
        }
    }
    return resolveFeaturesWithState(caps, state)
}

// resolveFeaturesWithState does the actual resolution given caps + state.
//
// Rules:
//   - By default, auto_web_search and web_search are NOT included.
//   - enable_thinking defaults to true for all models.
//   - 'think' is never included — only enable_thinking reaches the request.
//   - User overrides always take precedence.
//   - image_generation is ALWAYS forced to false.
func resolveFeaturesWithState(caps map[string]interface{}, state *ModelFeatureState) map[string]interface{} {
    result := make(map[string]interface{})

    if state.IncludeAll {
        // Include ALL server capabilities except reasoning_effort
        // (reasoning_effort in capabilities is a boolean support flag,
        //  not an actual feature value — handled separately per-request)
        for k, v := range caps {
            if k == "reasoning_effort" {
                continue
            }
            result[k] = v
        }
    }
    // By default: no auto_web_search, no web_search, no think.
    // enable_thinking defaults to true (set below).

    // Apply user overrides (per-model stored overrides take precedence)
    for k, v := range state.Overrides {
        result[k] = v
    }

    // Defensive: reasoning_effort is never a stored feature — strip any stale value.
    // It is a per-request parameter validated against model capabilities in sendToZAI.
    delete(result, "reasoning_effort")

    // enable_thinking defaults to true unless explicitly overridden
    if _, ok := result["enable_thinking"]; !ok {
        result["enable_thinking"] = true
    }

    // Remove 'think' entirely — only enable_thinking reaches the request
    delete(result, "think")

    // ALWAYS exclude image_generation
    result["image_generation"] = false

    return result
}

