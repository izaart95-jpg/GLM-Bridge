// Exported seams for the blackbox integration tests in tests/ (and for
// operational scripting). They expose just enough of the bridge's live state
// to point it at a mock upstream and bypass the captcha machinery — the same
// role the exported ProxyServer/SessionPool API plays in the DeepseekFreeAPI
// reference this layout follows.

package zbridge

import (
    "database/sql"
    "time"
)

// GetConfig returns the live bridge configuration (env/flag parsed).
// Callers may read it or toggle fields (e.g. AgentMode) and should restore
// what they changed.
func GetConfig() *Config { return config }

// OverrideSessionState swaps the Z.AI session identity (token / user / init
// flag) used by the bridge and returns a function that restores the previous
// state. Integration tests use it to skip guest auth against a mock
// upstream.
func OverrideSessionState(token, userID string, initialized bool) func() {
    session.mu.Lock()
    oldToken, oldUser, oldInit := session.Token, session.UserID, session.Initialized
    session.Token, session.UserID, session.Initialized = token, userID, initialized
    session.mu.Unlock()
    return func() {
        session.mu.Lock()
        session.Token, session.UserID, session.Initialized = oldToken, oldUser, oldInit
        session.mu.Unlock()
    }
}

// SeedCaptchaParam pushes a ready-made captcha_verify_param into the
// agent-mode cache so requests can bypass the Aliyun captcha machinery
// (integration tests only — the live cache is fed by captchaCache.Run).
func SeedCaptchaParam(value string) {
    captchaCache.mu.Lock()
    captchaCache.params = append(captchaCache.params, cachedCaptcha{
        value:       value,
        generatedAt: time.Now(),
    })
    captchaCache.mu.Unlock()
}

// FlushModelsCache clears the cached model list so the next /v1/models (or
// capability lookup) re-fetches from the current BASE_URL. Tests use it to
// repopulate the cache from their mock upstream after pointing BASE_URL at it.
func FlushModelsCache() {
    modelsCacheMu.Lock()
    modelsCache = nil
    modelsCacheTime = time.Time{}
    modelsCacheMu.Unlock()
}

// ActiveDBPath returns the file path of the currently attached SQLite
// database, or "" when none is attached. Tests use it to assert swap effects.
func ActiveDBPath() string {
    globalDBState.mu.RLock()
    h := globalDBState.active
    globalDBState.mu.RUnlock()
    if h.db == nil {
        return ""
    }
    return h.path
}

// AttachDatabase installs an already-open database (opened by the caller,
// e.g. a test fixture) as the active one. Operational callers should prefer
// SwapDatabase, which validates the file first.
func AttachDatabase(db *sql.DB, path string) { attachDB(db, path) }

// DetachDatabase removes the active database (closing it after any in-flight
// query drains), leaving the bridge in the "no database" state.
func DetachDatabase() { closeDB() }

// SwapDatabase exposes the hot-swap primitive (POST /sqlite's core) for
// direct use by tests and operational scripting.
func SwapDatabase(newPath string) error { return swapDB(newPath) }

// TokenCount exposes the TTL-cached token count (the /health value) for
// tests and operational scripting. -1 means "unavailable".
func TokenCount() int64 { return globalDBState.tokenCount() }
