// Code moved from the original main.go monolith during the internal/ restructure.
// See README "Project Structure". Part of the Z.AI bridge core (package zbridge).

package zbridge

import (
    "database/sql"
    "encoding/json"
    "regexp"
    "sync"
    "sync/atomic"
    "time"
)

// ============================================================================
// TYPE DEFINITIONS
// ============================================================================

// ---------- Z.AI types ----------

type Features struct {
    WebSearch     bool `json:"webSearch"`
    AutoWebSearch bool `json:"autoWebSearch"`
    Thinking      bool `json:"thinking"`
    ImageGen      bool `json:"imageGen"`
    PreviewMode   bool `json:"previewMode"`
}

type Message struct {
    Role    string          `json:"role"`
    Content json.RawMessage `json:"content"`
}

type SessionState struct {
    mu           sync.Mutex
    Token        string
    UserID       string
    UserName     string
    ChatID       string
    Messages     []Message
    SaltKey      string
    FeVersion    string
    Features     Features
    Initialized  bool
    Initializing bool
}

type ZAIResult struct {
    Chunk     string
    FullText  string
    Reasoning string
    Err       error
}

type SendOptions struct {
    Model             string
    WebSearch         *bool
    Thinking          *bool
    ImageGen          *bool
    PreviewMode       *bool
    ChatID            string
    Messages          []Message
    ClientMessagesRaw json.RawMessage
    ReasoningEffort   string // "high" or "max"; only forwarded if model supports it
    // Files carries already-uploaded Z.AI file entries (see vision.go) to
    // attach to the upstream completion request as the top-level "files"
    // array. Empty for text-only requests (field then stays out of the body).
    Files []map[string]interface{}
}

type ResponseResult struct {
    Content      string
    Text         string
    Prompt       string
    FinishReason string
    Reasoning    string
}

// ---------- Captcha JSON struct types ----------

type InitCaptchaResponse struct {
    CertifyID string `json:"CertifyId"`
}

type CVP struct {
    CertifyID   string `json:"certifyId"`
    Data        string `json:"data"`
    DeviceToken string `json:"deviceToken"`
    SceneID     string `json:"sceneId"`
}

type VerifyCaptchaResponse struct {
    Success bool `json:"Success"`
    Result  struct {
        VerifyResult  bool   `json:"VerifyResult"`
        SecurityToken string `json:"securityToken"`
        CertifyID     string `json:"certifyId"`
    } `json:"Result"`
}

type FinalPayload struct {
    CertifyID     string `json:"certifyId"`
    IsSign        bool   `json:"isSign"`
    SceneID       string `json:"sceneId"`
    SecurityToken string `json:"securityToken"`
}

type TrackList struct {
    FI        string `json:"fi"`
    KS        string `json:"ks"`
    MC        string `json:"mc"`
    MP        string `json:"mp"`
    MU        string `json:"mu"`
    StartTime int64  `json:"startTime"`
    TC        string `json:"tc"`
    TE        string `json:"te"`
    TMV       string `json:"tmv"`
}

type Track struct {
    TrackList      TrackList `json:"TrackList"`
    TrackStartTime int64     `json:"TrackStartTime"`
    VerifyTime     int64     `json:"VerifyTime"`
    Arg            string    `json:"arg"`
}

// ============================================================================
// GLOBAL STATE
// ============================================================================

// ---------- Captcha globals ----------

var (
    dbPath   string
    verbose  bool
    gRunning atomic.Bool
    dbMu     sync.Mutex
    logMu    sync.Mutex
    globalDB *sql.DB
)

// ---------- Z.AI globals ----------

var session = &SessionState{
    ChatID:    randomUUID(),
    UserName:  "Guest",
    SaltKey:   SALT_KEY,
    FeVersion: DEFAULT_FE_VERSION,
    Features:  Features{Thinking: true}, // enable_thinking on by default
}

type ModelInfo struct {
    ID           string
    Name         string
    Description  string
    Capabilities map[string]interface{}
    Created      int64 // upstream "created" unix seconds (0 = unknown)
}

var (
    modelsCache     []ModelInfo
    modelsCacheTime time.Time
    modelsCacheMu   sync.Mutex
)

const modelsCacheTTL = 5 * time.Minute

// Fallback if Z.AI API is unreachable and cache is empty
var fallbackModels = []ModelInfo{
    {ID: "glm-5.2", Name: "GLM-5.2", Description: "Flagship model, excels at coding and long-horizon tasks"},
    {ID: "GLM-5.1", Name: "GLM-5.1", Description: "Previous flagship model"},
    {ID: "GLM-5-Turbo", Name: "GLM-5-Turbo", Description: "New model for chat, coding, and agentic task"},
    {ID: "GLM-5v-Turbo", Name: "GLM-5V-Turbo", Description: "Vision model with evolved intelligence",
        Capabilities: map[string]interface{}{"vision": true}},
    {ID: "glm-4.7", Name: "GLM-4.7", Description: "Classic high-performance model"},
}

var feVersionRe = regexp.MustCompile(`prod-fe-\d+\.\d+\.\d+`)

// ---------- Per-model feature state (dynamic, model-aware) ----------

// ModelFeatureState tracks per-model feature configuration.
// IncludeAll: when true, ALL server capabilities are sent to /completions.
// Overrides: user-supplied per-model feature overrides (snake_case keys).
type ModelFeatureState struct {
    IncludeAll bool
    Overrides  map[string]interface{}
}

var (
    modelFeatureStates   = make(map[string]*ModelFeatureState)
    modelFeatureStatesMu sync.Mutex
)

// normalizeFeatureKey converts a camelCase key to snake_case.
// No alias mapping — users must use the real server capability key name.
// Special handling for reasoning/thinking -> enable_thinking is done in featuresHandler.
