// Exported seams for the blackbox integration tests in tests/ (and for
// operational scripting). They expose just enough of the bridge's live state
// to point it at a mock upstream and bypass the captcha machinery — the same
// role the exported ProxyServer/SessionPool API plays in the DeepseekFreeAPI
// reference this layout follows.

package zbridge

import "time"

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
