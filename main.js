"use strict";

const express = require("express");
const http = require("http");
const crypto = require("crypto");
const config = require("./config");
const os = require("os");
const fs = require("fs");
const fsp = fs.promises;
const path = require("path");
const { execSync } = require("child_process");

const app = express();
const server = http.createServer(app);

// ============== Z.AI DIRECT CONFIG ==============

const BASE_URL = "https://chat.z.ai";
const SALT_KEY = "key-@@@@)))()((9))-xxxx&&&%%%%%";
const DEFAULT_FE_VERSION = "prod-fe-1.0.185";

// ============== SESSION STATE ==============

const session = {
  token: "",
  userId: "",
  userName: "Guest",
  chatId: crypto.randomUUID(),
  messages: [],
  saltKey: SALT_KEY,
  feVersion: DEFAULT_FE_VERSION,
  features: {
    webSearch: false,
    autoWebSearch: false,
    thinking: false,
    imageGen: false,
    previewMode: false,
    persistHistory: false,
  },
  initialized: false,
  initializing: false,
};

// ============== PER-SESSION CONVERSATION STATE ==============

const sessions = new Map(); // sessionId -> { chatId, messages, lastUsed }
const SESSION_TTL = 30 * 60 * 1000; // 30 minutes

function getOrCreateSession(req) {
  const sessionId = req.headers["x-session-id"] || "default";
  const fresh = req.headers["x-fresh-session"] === "true";

  if (fresh || !sessions.has(sessionId)) {
    sessions.set(sessionId, {
      chatId: crypto.randomUUID(),
      messages: [],
      lastUsed: Date.now(),
    });
  }

  const s = sessions.get(sessionId);
  s.lastUsed = Date.now();
  return s;
}

// Periodically clean up stale sessions
setInterval(() => {
  const now = Date.now();
  for (const [id, s] of sessions) {
    if (now - s.lastUsed > SESSION_TTL) sessions.delete(id);
  }
}, 5 * 60 * 1000); // check every 5 minutes

// ============== MIDDLEWARE ==============

app.use((req, res, next) => {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS");
  res.header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-Id, X-Fresh-Session");
  if (req.method === "OPTIONS") return res.sendStatus(200);
  next();
});

app.use(express.json({ limit: "50mb" }));

function authMiddleware(req, res, next) {
  if (!config.auth.enabled) return next();
  const authHeader = req.headers.authorization;
  const provided = authHeader?.replace(/^Bearer\s+/i, "");
  if (provided !== config.auth.token) {
    return res.status(401).json({
      type: "error",
      error: { type: "authentication_error", message: "Invalid or missing authentication token" }
    });
  }
  next();
}

// ============== UTILITY FUNCTIONS ==============

function generateId() {
  return crypto.randomBytes(16).toString("hex");
}

function estimateTokens(text) {
  if (!text) return 0;
  return Math.ceil(text.length / 4);
}

function getMessageContent(content) {
  if (!content) return "";
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content
      .filter(part => part.type === "text" || typeof part === "string")
      .map(part => (typeof part === "string" ? part : (part.text || "")))
      .join("\n");
  }
  return String(content);
}

// Flatten OpenAI messages array → prompt string
// Used ONLY for signature_prompt computation (HMAC signing).
// The actual messages array is forwarded as-is to Z.AI.
function messagesToPrompt(messages) {
  if (!Array.isArray(messages)) return String(messages);

  let prompt = "";
  for (const msg of messages) {
    const content = getMessageContent(msg.content);
    prompt += `${content}\n\n`;
  }

  return prompt.trim();
}
// ============== CAPTCHA NAMED-PIPE (cross-platform) ==============

const _isWin = process.platform === "win32";
const _tmpDir = process.env.TEMPDIR || os.tmpdir();
const CAPTCHA_REQ_PIPE = _isWin
  ? "\\\\.\\pipe\\captcha_pipe.req"
  : path.join(_tmpDir, "captcha_pipe.req");
const CAPTCHA_RESP_PIPE = _isWin
  ? "\\\\.\\pipe\\captcha_pipe.resp"
  : path.join(_tmpDir, "captcha_pipe.resp");

// Ensure FIFOs exist on Linux
if (!_isWin) {
  for (const p of [CAPTCHA_REQ_PIPE, CAPTCHA_RESP_PIPE]) {
    if (!fs.existsSync(p)) {
      try { execSync(`mkfifo -m 666 "${p}"`); } catch (_) {}
    }
  }
}

// Captcha Gen
// ============== CAPTCHA NAMED-PIPE (cross-platform) ==============

const _isWin = process.platform === "win32";
const _tmpDir = process.env.TEMPDIR || os.tmpdir();
const CAPTCHA_REQ_PIPE = _isWin
  ? "\\\\.\\pipe\\captcha_pipe.req"
  : path.join(_tmpDir, "captcha_pipe.req");
const CAPTCHA_RESP_PIPE = _isWin
  ? "\\\\.\\pipe\\captcha_pipe.resp"
  : path.join(_tmpDir, "captcha_pipe.resp");

// Ensure FIFOs exist on Linux
if (!_isWin) {
  for (const p of [CAPTCHA_REQ_PIPE, CAPTCHA_RESP_PIPE]) {
    if (!fs.existsSync(p)) {
      try { execSync(`mkfifo -m 666 "${p}"`); } catch (_) {}
    }
  }
}

function getCaptchaVerifyParam() {
  return new Promise((resolve, reject) => {
    let data = "";
    
    // 1. Set up reader for the response pipe first
    const respStream = fs.createReadStream(CAPTCHA_RESP_PIPE, { encoding: "utf8" });
    
    respStream.on("data", (chunk) => { data += chunk; });
    respStream.on("end", () => {
      const result = data.trim();
      if (!result) return reject(new Error("Captcha pipe returned empty response"));
      resolve(result);
    });
    respStream.on("error", reject);

    // 2. Write trigger to req pipe
    const reqStream = fs.createWriteStream(CAPTCHA_REQ_PIPE);
    reqStream.on("error", reject);
    reqStream.on("open", () => {
      reqStream.write("1", () => reqStream.end());
    });
  });
}
// ============================================================
// ── FORMAT HELPERS ──────────────────────────────────────────
// ============================================================

// ── OpenAI /v1/chat/completions format ──

function formatOpenAIResponse(result, model, requestId, stream = false) {
  const timestamp = Math.floor(Date.now() / 1000);
  const rawContent = result.content || result.text || "";

  if (stream) {
    if (result.finish_reason !== "stop") {
      return {
        id: `chatcmpl-${requestId}`,
        object: "chat.completion.chunk",
        created: timestamp,
        model: model || "glm-5",
        choices: [{ index: 0, delta: { content: rawContent }, finish_reason: null }]
      };
    }

    return {
      id: `chatcmpl-${requestId}`,
      object: "chat.completion.chunk",
      created: timestamp,
      model: model || "glm-5",
      choices: [{ index: 0, delta: { content: rawContent }, finish_reason: "stop" }]
    };
  }

  return {
    id: `chatcmpl-${requestId}`,
    object: "chat.completion",
    created: timestamp,
    model: model || "glm-5",
    choices: [{
      index: 0,
      message: {
        role: "assistant",
        content: rawContent
      },
      finish_reason: "stop"
    }],
    usage: {
      prompt_tokens: estimateTokens(result.prompt || ""),
      completion_tokens: estimateTokens(rawContent),
      total_tokens: estimateTokens(result.prompt || "") + estimateTokens(rawContent)
    }
  };
}

function formatOpenAIError(message, type = "api_error", code = null) {
  return { error: { message, type, code, param: null } };
}

// ============== Z.AI DIRECT HTTP FUNCTIONS ==============

async function scrapeConfig() {
  try {
    const res = await fetch(BASE_URL, { signal: AbortSignal.timeout(10000) });
    const text = await res.text();
    const match = text.match(/prod-fe-\d+\.\d+\.\d+/);
    if (match) {
      session.feVersion = match[0];
      console.log(`[Config] fe_version: ${session.feVersion}`);
    }
  } catch (e) {
    console.warn(`[Config] Scrape error: ${e.message}, using default feVersion`);
  }
}

function generateZaSignature(prompt, token, userId) {
  const timestamp = String(Date.now());
  const requestId = crypto.randomUUID();
  const bucket = Math.floor(Number(timestamp) / 300000);

  const wKey = crypto
    .createHmac("sha256", session.saltKey)
    .update(String(bucket))
    .digest("hex");

  const payloadDict = { requestId, timestamp, user_id: userId };
  const sortedItems = Object.entries(payloadDict).sort((a, b) => a[0].localeCompare(b[0]));
  const sortedPayload = sortedItems.map(([k, v]) => `${k},${v}`).join(",");

  const promptB64 = Buffer.from(prompt.trim()).toString("base64");
  const dataToSign = `${sortedPayload}|${promptB64}|${timestamp}`;

  const signature = crypto
    .createHmac("sha256", wKey)
    .update(dataToSign)
    .digest("hex");

  const params = new URLSearchParams({
    timestamp, requestId, user_id: userId,
    version: "0.0.1", platform: "web", token,
    user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0",
    language: "en-US", screen_resolution: "1920x1080",
    viewport_size: "1920x1080", timezone: "Europe/Paris",
    timezone_offset: "-60", signature_timestamp: timestamp
  });

  return { signature, timestamp, urlParams: params.toString() };
}

async function initializeSession() {
  if (session.initializing) {
    await new Promise(resolve => {
      const check = setInterval(() => {
        if (!session.initializing) { clearInterval(check); resolve(); }
      }, 100);
    });
    return;
  }

  session.initializing = true;

  // ── Fast path: use hardcoded ZAI_TOKEN, skip guest flow ──
  if (config.zaiToken) {
    console.log("[Session] Using hardcoded ZAI_TOKEN, skipping guest init.");
    session.token = config.zaiToken;
    try {
      const parts = session.token.split(".");
      const padded = parts[1] + "==";
      const payload = JSON.parse(Buffer.from(padded, "base64").toString("utf8"));
      session.userId = payload.id || "";
      session.userName = (payload.email || "User").split("@")[0];
      console.log(`[Session] Token user: ${session.userId.substring(0, 8)}... (${session.userName})`);
    } catch (e) {
      console.warn("[Session] Token decode failed, continuing with raw token.");
      session.userId = "";
      session.userName = "User";
    }
    session.initialized = true;
    session.initializing = false;
    return;
  }

  console.log("[Session] Initializing Z.AI session...");

  try {
    await scrapeConfig();

    const headers = {
      "Origin": BASE_URL,
      "Referer": `${BASE_URL}/`,
      "Content-Type": "application/json"
    };

    await fetch(`${BASE_URL}/api/v1/auths/guest`, {
      method: "POST", headers, body: "{}", signal: AbortSignal.timeout(15000)
    });

    const authRes = await fetch(`${BASE_URL}/api/v1/auths/`, {
      headers, signal: AbortSignal.timeout(15000)
    });

    if (!authRes.ok) throw new Error(`Auth failed: ${authRes.status}`);

    const authData = await authRes.json();
    session.token = authData.token || "";

    if (!session.token) {
      const guestRes = await fetch(`${BASE_URL}/api/v1/auths/guest`, {
        method: "POST", headers, body: "{}", signal: AbortSignal.timeout(15000)
      });
      if (guestRes.ok) {
        const gd = await guestRes.json();
        session.token = gd.token || "";
      }
    }

    if (session.token) {
      try {
        const parts = session.token.split(".");
        const padded = parts[1] + "==";
        const payload = JSON.parse(Buffer.from(padded, "base64").toString("utf8"));
        session.userId = payload.id || "";
        session.userName = (payload.email || "Guest").split("@")[0];
        console.log(`[Session] Connected. UserID: ${session.userId.substring(0, 8)}... (${session.userName})`);
      } catch (e) {
        console.warn("[Session] Token decode failed, but continuing.");
      }
      session.initialized = true;
    } else {
      throw new Error("No token received from Z.AI");
    }
  } catch (e) {
    console.error("[Session] Initialization error:", e.message);
    session.initialized = false;
    throw e;
  } finally {
    session.initializing = false;
  }
}

async function* sendToZAI(prompt, options = {}) {
  const {
    model = "glm-5",
    webSearch = session.features.webSearch,
    thinking = session.features.thinking,
    imageGen = session.features.imageGen,
    previewMode = session.features.previewMode,
    chatId = session.chatId,
    messages = session.messages,
    clientMessages = null,  // Structured messages from client — forwarded as-is
  } = options;

  if (!session.initialized) await initializeSession();

  const { signature, urlParams } = generateZaSignature(prompt, session.token, session.userId);
  const url = `${BASE_URL}/api/v2/chat/completions`;

  const headers = {
    "Origin": BASE_URL,
    "Referer": `${BASE_URL}/`,
    "Authorization": `Bearer ${session.token}`,
    "X-Signature": signature,
    "X-FE-Version": session.feVersion,
    "Content-Type": "application/json"
  };

  // ── Forward structured messages, NOT flattened prompt ──
  const forwardedMessages = clientMessages
    ? clientMessages
    : [...messages, { role: "user", content: prompt }];

  const requestBody = {
    model,
    chat_id: chatId,
    messages: forwardedMessages,
    signature_prompt: prompt,
    stream: true,
    features: {
      image_generation: imageGen,
      web_search: webSearch,
      auto_web_search: webSearch,
      preview_mode: previewMode,
      flags: [],
      enable_thinking: thinking
    }
  };

  requestBody.captcha_verify_param = await getCaptchaVerifyParam();
  const body = JSON.stringify(requestBody);

  if (config.logging.level === "debug") {
    console.log("[DEBUG] Z.AI url", url);
    console.log("[DEBUG] Z.AI request body:", body);
    console.log("[DEBUG] Z.AI request headers", JSON.stringify(headers, null, 2));
  }

  let res;
  try {
    res = await fetch(url, { method: "POST", headers, body, signal: AbortSignal.timeout(90000) });
  } catch (e) {
    throw new Error(`Z.AI connection error: ${e.message}`);
  }

  // ── ADDED: Response status and headers logging ──
  if (config.logging.level === "debug") {
    console.log("[DEBUG] Z.AI response status:", res.status, res.statusText);
    console.log("[DEBUG] Z.AI response headers:", JSON.stringify(Object.fromEntries(res.headers.entries()), null, 2));
  }

  if (res.status === 401) {
    session.initialized = false;
    await initializeSession();
    yield* sendToZAI(prompt, options);
    return;
  }

  if (!res.ok) {
    const errText = await res.text().catch(() => "");
    // ── ADDED: Error body logging ──
    if (config.logging.level === "debug") {
      console.error("[DEBUG] Z.AI error body:", errText);
    }
    throw new Error(`Z.AI error ${res.status}: ${errText}`);
  }

  const decoder = new TextDecoder();
  let buffer = "";

  for await (const chunk of res.body) {
    buffer += decoder.decode(chunk, { stream: true });
    const lines = buffer.split("\n");
    buffer = lines.pop();

    // ── ADDED: Raw SSE line dump ──
    if (config.logging.level === "debug") {
      for (const line of lines) {
        if (line.trim()) console.log("[DEBUG] Z.AI SSE line:", line.trim());
      }
    }

    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed || !trimmed.startsWith("data: ")) continue;
      const dataStr = trimmed.slice(6);
      if (dataStr === "[DONE]") return;
      try {
        const json = JSON.parse(dataStr);
        let chunk = "";
        if (json.data?.delta_content !== undefined) chunk = json.data.delta_content;
        else if (json.choices?.[0]?.delta?.content !== undefined) chunk = json.choices[0].delta.content;
        if (chunk) yield chunk;
      } catch (e) {
        // ── ADDED: Parse failure logging ──
        if (config.logging.level === "debug") {
          console.warn("[DEBUG] Z.AI failed to parse SSE:", dataStr);
        }
      }
    }
  }

  if (buffer.trim().startsWith("data: ")) {
    const dataStr = buffer.trim().slice(6);
    if (dataStr !== "[DONE]") {
      try {
        const json = JSON.parse(dataStr);
        let chunk = "";
        if (json.data?.delta_content !== undefined) chunk = json.data.delta_content;
        else if (json.choices?.[0]?.delta?.content !== undefined) chunk = json.choices[0].delta.content;
        if (chunk) yield chunk;
      } catch (e) {
        // ── ADDED: Parse failure logging for buffer ──
        if (config.logging.level === "debug") {
          console.warn("[DEBUG] Z.AI failed to parse final SSE buffer:", dataStr);
        }
      }
    }
  }
}

// ============== DASHBOARD HTML ==============

const getDashboardHTML = (host) => `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Z.AI Direct Bridge</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      background: linear-gradient(135deg, #1e3a5f 0%, #0d1b2a 50%, #1b263b 100%);
      min-height: 100vh; color: #e0e0e0; padding: 20px;
    }
    .container { max-width: 1200px; margin: 0 auto; }
    .header {
      text-align: center; padding: 40px 20px;
      background: rgba(255,255,255,0.05); border-radius: 16px;
      margin-bottom: 30px; border: 1px solid rgba(255,255,255,0.1);
    }
    .header h1 {
      font-size: 2.5rem;
      background: linear-gradient(135deg, #3b82f6, #1d4ed8, #60a5fa);
      -webkit-background-clip: text; -webkit-text-fill-color: transparent;
      margin-bottom: 10px;
    }
    .header p { color: #888; font-size: 1.1rem; }
    .badges { display: flex; gap: 8px; justify-content: center; margin-top: 12px; flex-wrap: wrap; }
    .badge {
      display: inline-block; padding: 4px 12px; border-radius: 12px;
      font-size: 0.8rem; font-weight: 700;
    }
    .badge-green { background: #22c55e; color: #000; }
    .badge-blue  { background: #3b82f6; color: #fff; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
    .card {
      background: rgba(255,255,255,0.05); border-radius: 12px;
      padding: 24px; border: 1px solid rgba(255,255,255,0.1);
    }
    .card h2 { color: #60a5fa; margin-bottom: 16px; font-size: 1.2rem; }
    .stat-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
    .stat { background: rgba(0,0,0,0.2); padding: 12px; border-radius: 8px; }
    .stat .label { color: #888; font-size: 0.85rem; }
    .stat .value { color: #60a5fa; font-weight: 600; font-size: 1.5rem; margin-top: 4px; }
    .code-block {
      background: #0d1117; border-radius: 8px; padding: 16px; overflow-x: auto;
      font-family: 'Monaco', 'Menlo', monospace; font-size: 0.85rem;
      border: 1px solid #30363d; margin: 12px 0;
    }
    .code-block code { color: #c9d1d9; white-space: pre-wrap; }
    .endpoint { background: rgba(0,0,0,0.2); padding: 12px; border-radius: 8px; margin-bottom: 8px; }
    .method {
      display: inline-block; padding: 4px 8px; border-radius: 4px;
      font-size: 0.75rem; font-weight: 600; margin-right: 8px;
    }
    .method.get { background: #22c55e; color: #000; }
    .method.post { background: #3b82f6; color: #fff; }
    .path { font-family: monospace; color: #e0e0e0; }
    .desc { color: #888; font-size: 0.85rem; margin-top: 4px; }
    .section-label {
      font-size: 0.75rem; font-weight: 700; text-transform: uppercase;
      letter-spacing: 0.1em; color: #a855f7; margin: 16px 0 8px;
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>Z.AI Direct Bridge</h1>
      <p>HTTP-only mode — No browser required</p>
      <div class="badges">
        <span class="badge badge-green">⚡ Direct Mode</span>
        <span class="badge badge-blue">OpenAI Compatible</span>
      </div>
    </div>

    <div class="grid">
      <div class="card">
        <h2>Session Status</h2>
        <div class="stat-grid">
          <div class="stat">
            <div class="label">Connection</div>
            <div class="value" id="sessionStatus">...</div>
          </div>
          <div class="stat">
            <div class="label">User</div>
            <div class="value" id="sessionUser" style="font-size:1rem">...</div>
          </div>
          <div class="stat">
            <div class="label">Messages</div>
            <div class="value" id="msgCount">0</div>
          </div>
          <div class="stat">
            <div class="label">FE Version</div>
            <div class="value" id="feVersion" style="font-size:0.85rem">...</div>
          </div>
        </div>
      </div>

      <div class="card">
        <h2>Features</h2>
        <div class="stat-grid">
          <div class="stat"><div class="label">Web Search</div><div class="value" id="featSearch">-</div></div>
          <div class="stat"><div class="label">Thinking</div><div class="value" id="featThink">-</div></div>
          <div class="stat"><div class="label">Image Gen</div><div class="value" id="featImage">-</div></div>
          <div class="stat"><div class="label">Preview</div><div class="value" id="featPreview">-</div></div>
        </div>
      </div>

      <div class="card" style="grid-column: span 2;">
        <h2>API Endpoints</h2>

        <div class="section-label">OpenAI-Compatible</div>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/v1/chat/completions</span>
          <div class="desc">OpenAI-compatible chat endpoint. Supports streaming.</div>
        </div>
        <div class="endpoint">
          <span class="method get">GET</span>
          <span class="path">/v1/models</span>
          <div class="desc">Model list</div>
        </div>

        <div class="section-label">Management</div>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/features</span>
          <div class="desc">Toggle webSearch, thinking, imageGen, previewMode, persistHistory</div>
        </div>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/admin/session/clear</span>
          <div class="desc">Clear conversation history</div>
        </div>
      </div>

      <div class="card" style="grid-column: span 2;">
        <h2>Test the OpenAI endpoint</h2>
        <div class="code-block">
          <code># Non-streaming
curl -X POST http://${host}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${config.auth.token}" \\
  -d '{"model":"glm-5","messages":[{"role":"user","content":"Hello!"}],"stream":false}'

# Streaming
curl -X POST http://${host}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${config.auth.token}" \\
  -d '{"model":"glm-5","stream":true,"messages":[{"role":"user","content":"Say hi"}]}'</code>
        </div>
      </div>
    </div>
  </div>

  <script>
    async function updateStatus() {
      try {
        const res = await fetch('/status');
        const d = await res.json();
        document.getElementById('sessionStatus').textContent = d.connected ? '✓ OK' : '✗ Off';
        document.getElementById('sessionUser').textContent = d.userName || '-';
        document.getElementById('msgCount').textContent = d.messageCount;
        document.getElementById('feVersion').textContent = d.feVersion || '-';
        document.getElementById('featSearch').textContent = d.features?.webSearch ? 'ON' : 'OFF';
        document.getElementById('featThink').textContent = d.features?.thinking ? 'ON' : 'OFF';
        document.getElementById('featImage').textContent = d.features?.imageGen ? 'ON' : 'OFF';
        document.getElementById('featPreview').textContent = d.features?.previewMode ? 'ON' : 'OFF';
      } catch(e) { console.error(e); }
    }
    updateStatus();
    setInterval(updateStatus, 3000);
  </script>
</body>
</html>`;

// ============== ROUTES ==============

app.get("/", (req, res) => {
  const host = req.headers.host || `localhost:${config.server.port}`;
  res.send(getDashboardHTML(host));
});

app.get("/status", (req, res) => {
  res.json({
    connected: session.initialized,
    userName: session.userName,
    userId: session.userId ? session.userId.substring(0, 8) + "..." : null,
    feVersion: session.feVersion,
    activeSessions: sessions.size,
    features: session.features,
    mode: "direct"
  });
});

// ============================================================
// ── OPENAI-COMPATIBLE /v1/chat/completions ──────────────────
// ============================================================

const knownModels = [
  "glm-4.7", "glm-5", "GLM-5-Turbo", "GLM-5v-Turbo", "GLM-5.1",
];

app.get("/v1/models", authMiddleware, (req, res) => {
  res.json({
    object: "list",
    data: knownModels.map(m => ({
      id: m,
      object: "model",
      created: Math.floor(Date.now() / 1000),
      owned_by: "z-ai",
      display_name: m,
    }))
  });
});

app.get("/models", authMiddleware, (req, res) => {
  res.json({ models: knownModels, currentModel: "glm-5" });
});

app.post("/v1/chat/completions", authMiddleware, async (req, res) => {
  const { model = "glm-5", messages, stream = true, deepThink, search, webSearch } = req.body;

  if (!messages || !Array.isArray(messages)) {
    return res.status(400).json(formatOpenAIError("messages is required and must be an array", "invalid_request_error"));
  }

  const reqSession = getOrCreateSession(req);
  const requestId = generateId();

  // ── Forward structured messages, flatten ONLY for signature ──
  const prompt = messagesToPrompt(messages);

  const opts = {
    model,
    webSearch: webSearch ?? search ?? session.features.webSearch,
    thinking: deepThink ?? session.features.thinking,
    imageGen: session.features.imageGen,
    previewMode: session.features.previewMode,
    chatId: reqSession.chatId,
    messages: reqSession.messages,
    clientMessages: messages,  // ← structured messages forwarded as-is
  };

  if (stream) {
    res.setHeader("Content-Type", "text/event-stream");
    res.setHeader("Cache-Control", "no-cache");
    res.setHeader("Connection", "keep-alive");
    res.setHeader("X-Accel-Buffering", "no");
    res.flushHeaders();

    const initChunk = formatOpenAIResponse({ content: "", finish_reason: null }, model, requestId, true);
    res.write(`data: ${JSON.stringify(initChunk)}\n\n`);

    let fullContent = "";
    let sentContent = "";

    const keepAlive = setInterval(() => {
      try {
        const ka = formatOpenAIResponse({ content: "", finish_reason: null }, model, requestId, true);
        res.write(`data: ${JSON.stringify(ka)}\n\n`);
      } catch (e) { clearInterval(keepAlive); }
    }, 5000);

    try {
      for await (const chunk of sendToZAI(prompt, opts)) {
        fullContent += chunk;
        const delta = fullContent.substring(sentContent.length);
        if (delta) {
          sentContent = fullContent;
          const c = formatOpenAIResponse({ content: delta, finish_reason: null }, model, requestId, true);
          res.write(`data: ${JSON.stringify(c)}\n\n`);
        }
      }

      const finalChunk = formatOpenAIResponse({ content: "", finish_reason: "stop" }, model, requestId, true);
      res.write(`data: ${JSON.stringify(finalChunk)}\n\n`);
      res.write("data: [DONE]\n\n");

      // ── History persistence (toggle-gated) ──
      if (session.features.persistHistory) {
        reqSession.messages.push({ role: "user", content: prompt });
        if (fullContent) reqSession.messages.push({ role: "assistant", content: fullContent });
      }

    } catch (e) {
      console.error("[Stream] Error:", e.message);
      res.write(`data: ${JSON.stringify({ error: { message: e.message } })}\n\n`);
      res.write("data: [DONE]\n\n");
    } finally {
      clearInterval(keepAlive);
      res.end();
    }

  } else {
    try {
      let fullContent = "";
      for await (const chunk of sendToZAI(prompt, opts)) {
        fullContent += chunk;
      }

      // ── History persistence (toggle-gated) ──
      if (session.features.persistHistory) {
        reqSession.messages.push({ role: "user", content: prompt });
        if (fullContent) reqSession.messages.push({ role: "assistant", content: fullContent });
      }

      res.json(formatOpenAIResponse({ content: fullContent }, model, requestId));
    } catch (e) {
      console.error("[API] Error:", e.message);
      const statusCode = e.message.includes("401") ? 401 : 500;
      res.status(statusCode).json(formatOpenAIError(e.message));
    }
  }
});

// ============== LEGACY + ADMIN ROUTES ==============

app.post("/prompt", authMiddleware, async (req, res) => {
  const { prompt, search, deepThink, webSearch } = req.body;
  if (!prompt) return res.status(400).json({ error: "Prompt is required" });
  const reqSession = getOrCreateSession(req);
  try {
    let fullContent = "";
    for await (const chunk of sendToZAI(prompt, {
      webSearch: webSearch ?? search ?? session.features.webSearch,
      thinking: deepThink ?? session.features.thinking,
      chatId: reqSession.chatId,
      messages: reqSession.messages,
    })) { fullContent += chunk; }

    if (session.features.persistHistory) {
      reqSession.messages.push({ role: "user", content: prompt });
      if (fullContent) reqSession.messages.push({ role: "assistant", content: fullContent });
    }

    res.json({ success: true, response: fullContent });
  } catch (e) {
    console.error("[Prompt] Error:", e.message);
    res.status(500).json({ success: false, error: e.message });
  }
});

app.post("/features", authMiddleware, (req, res) => {
  const { webSearch, thinking, imageGen, previewMode, persistHistory } = req.body;
  if (webSearch     !== undefined) { session.features.webSearch = !!webSearch; session.features.autoWebSearch = !!webSearch; }
  if (thinking      !== undefined) session.features.thinking     = !!thinking;
  if (imageGen      !== undefined) session.features.imageGen     = !!imageGen;
  if (previewMode   !== undefined) session.features.previewMode  = !!previewMode;
  if (persistHistory !== undefined) session.features.persistHistory = !!persistHistory;
  console.log("[Features] Updated:", session.features);
  res.json({ success: true, features: session.features });
});

app.get("/admin/stats", (req, res) => {
  let totalMessages = 0;
  for (const s of sessions.values()) totalMessages += s.messages.length;
  res.json({
    mode: "direct",
    totalClients: session.initialized ? 1 : 0,
    activeSessions: sessions.size,
    stats: { totalRequests: Math.floor(totalMessages / 2) }
  });
});

app.get("/admin/health", (req, res) => {
  const healthy = session.initialized;
  res.status(healthy ? 200 : 503).json({ healthy, mode: "direct" });
});

app.get("/admin/clients", (req, res) => {
  res.json({ clients: session.initialized ? [{ id: "session", status: "idle" }] : [] });
});

app.post("/admin/session/clear", authMiddleware, (req, res) => {
  sessions.clear();
  console.log("[Session] All session histories cleared.");
  res.json({ success: true, message: "All session histories cleared", activeSessions: 0 });
});

app.post("/admin/clients/:id/clear", authMiddleware, (req, res) => {
  sessions.clear();
  res.json({ success: true, message: "History cleared" });
});

app.get("/inject.js", (req, res) => {
  res.type("application/json").send(JSON.stringify({ message: "Direct mode" }));
});

app.post("/stop", authMiddleware, (req, res) => {
  res.json({ success: true, message: "Stop acknowledged" });
});

// ============== START SERVER ==============

server.listen(config.server.port, config.server.host, async () => {
  console.log(`
╔═══════════════════════════════════════════════════════════════╗
║           Z.AI Direct Bridge Server Started                   ║
╠═══════════════════════════════════════════════════════════════╣
║  Mode:          DIRECT HTTP (no browser needed)               ║
║  Dashboard:     http://localhost:${config.server.port}                      ║
╠═══════════════════════════════════════════════════════════════╣
║  OpenAI API:    http://localhost:${config.server.port}/v1/chat/completions  ║
╠═══════════════════════════════════════════════════════════════╣
║  Auth Token:    ${config.auth.token.padEnd(44)}║
╚═══════════════════════════════════════════════════════════════╝
`);

  try {
    await initializeSession();
  } catch (e) {
    console.warn("[Startup] Session init deferred — will retry on first request.");
  }
});
