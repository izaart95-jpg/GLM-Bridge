// Code moved from the original main.go monolith during the internal/ restructure.
// See README "Project Structure". Part of the Z.AI bridge core (package zbridge).

package zbridge

import (
    "encoding/json"
    "net/http"
    "strings"
)

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
// MIDDLEWARE
// ============================================================================

func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Include-All-Features, x-api-key, anthropic-version")
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
    // Anthropic clients send the API key via x-api-key header
    if provided == "" {
        provided = r.Header.Get("x-api-key")
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
    http.Redirect(w, r, "/health", http.StatusFound)
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

    writeJSON(w, 200, map[string]interface{}{
        "connected":   session.Initialized,
        "userName":    session.UserName,
        "userId":      userIDPreview,
        "feVersion":   session.FeVersion,
        "features":    session.Features,
        "mode":        "direct",
        "sessionPool": sessionPoolStatus(),
    })
}

