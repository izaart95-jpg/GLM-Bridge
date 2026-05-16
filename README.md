# GLM Bridge — Z.AI Proxy API

An OpenAI- and Anthropic-compatible API proxy for [chat.z.ai](https://chat.z.ai), letting you use Z.AI's models through standard API clients like Claude Code, Roo Code, Kilo Code, and any OpenAI-compatible tool.

---

## ⚠️ Important: Model Access & Authentication

Z.AI enforces model access based on account tier. **Guest sessions are restricted to `glm-4.7` and below.** Attempting to use higher-tier models (`glm-5`, `GLM-5-Turbo`, etc.) without a valid logged-in token will fail silently or return degraded responses.

### Unlocking Higher-Tier Models

To use `glm-5`, `GLM-5-Turbo`, or any advanced model, you must authenticate with your Z.AI account token.

**Step 1 — Log in to Z.AI**

Open [https://chat.z.ai](https://chat.z.ai) in your browser and sign in to your account.

**Step 2 — Retrieve your token**

Open browser DevTools (`F12`) and run this in the **Console** tab:

```javascript
localStorage.getItem('token')
```

Alternatively, find it under **Application → Local Storage → `https://chat.z.ai`** (key: `token`). The token is also stored in cookies — both locations contain the same value.

**Step 3 — Set the environment variable**

```bash
# Windows CMD
set ZAI_TOKEN=<your_token_here>

# Windows PowerShell
$env:ZAI_TOKEN = "<your_token_here>"

# Linux / macOS
export ZAI_TOKEN="<your_token_here>"
```

**Step 4 — Start the server**

```bash
# npm install
node main.js
```

When `ZAI_TOKEN` is set, the server skips guest initialization entirely and uses your account token directly, granting access to all models available on your plan.

> **Security note:** Treat this token like a password. Do not commit it to version control. Use a `.env` file (excluded via `.gitignore`) or your system's secret manager.

---

## ⚠️ Known Limitations

- **Tool / function calling** is not natively supported by Z.AI. The bridge parses XML-format tool calls from model output and converts them to structured blocks, but results may vary for complex agentic workflows.
- **HTTP 429 errors** indicate your IP has been rate-limited or blocked by Z.AI for excessive API usage. To avoid this, keep prompts under 60,000 characters and avoid sending rapid bursts of requests.

---

## Modes

| Mode | File | Description | Status |
|------|------|-------------|--------|
| ⚡ Direct HTTP | `main.js` | Calls Z.AI's REST API directly via HMAC-signed requests. No browser required. | ✅ Recommended |
| 🌐 Browser Automation | `browser.js` | Connects to a live browser tab via WebSocket injection. Requires an open browser session. | ⚠️ Deprecated |

Use `main.js`. It is faster, more stable, and requires no browser. `browser.js` is deprecated and will not receive further updates.

---

## Quick Start

### 1. Install

```bash
git clone https://github.com/izaart95-jpg/GLM-Bridge.git
cd GLM-Bridge
npm install
```

### 2. Configure (optional but recommended)

Set `ZAI_TOKEN` to unlock higher-tier models (see above). Without it, only `glm-4.7` and below are accessible.

### 3. Start

```bash
node main.js
```

Server starts at `http://localhost:3001`. Open that URL in a browser for the status dashboard.

---

## Claude Code Integration

The server exposes a native Anthropic-compatible `/v1/messages` endpoint. Point Claude Code directly at it — no LiteLLM required.

**Windows PowerShell**

```powershell
$env:ANTHROPIC_BASE_URL  = "http://localhost:3001"
$env:ANTHROPIC_AUTH_TOKEN = "Waguri"
$env:ANTHROPIC_API_KEY   = ""
claude
```

**Windows CMD**

```cmd
set ANTHROPIC_BASE_URL=http://localhost:3001
set ANTHROPIC_AUTH_TOKEN=Waguri
set ANTHROPIC_API_KEY=
claude
```

**Persistent — `~/.claude/settings.json`**

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://localhost:3001",
    "ANTHROPIC_AUTH_TOKEN": "Waguri",
    "ANTHROPIC_API_KEY": ""
  }
}
```

### Model Mapping

Claude model names are automatically mapped to their Z.AI equivalents:

| Claude Model | Z.AI Model | Requires Login |
|---|---|---|
| `claude-opus-*` | `GLM-5-Turbo` | ✅ Yes |
| `claude-sonnet-*` | `glm-5` | ✅ Yes |
| `claude-haiku-*` | `glm-5` | ✅ Yes |

All Claude model aliases resolve to `glm-5` or `GLM-5-Turbo`, both of which require an authenticated token. Set `ZAI_TOKEN` before starting the server or Claude Code requests will fail.

---

## Configuration

All settings are in `config.js` and can be overridden with environment variables.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3001` | Server port |
| `HOST` | `0.0.0.0` | Bind address |
| `AUTH_TOKEN` | `Waguri` | Token required by API clients to authenticate with this bridge |
| `ZAI_TOKEN` | *(unset)* | Your Z.AI account JWT. Required for `glm-5` and higher. |
| `PARSE_TOOL` | `false` | Set to `true` to parse XML/JSON tool calls into structured blocks |
| `LOG_LEVEL` | `info` | Logging verbosity: `debug`, `info`, `warn`, `error` |
| `TIMEOUT` | `300000` | Request timeout in milliseconds |

### Behavior Toggles (top of `main.js`)

```js
const INCLUDE_CORE_INSTRUCTIONS = false;  // Prepend Roo/Cline XML format hints to every prompt
```

| Toggle | Default | Effect |
|--------|---------|--------|
| `PARSE_TOOL` (env) | `false` | Parse tool calls from model output into `tool_use` / `tool_calls` blocks |
| `INCLUDE_CORE_INSTRUCTIONS` | `false` | Inject XML formatting hints for Roo/Cline tool call format into every prompt |

---

## Available Models

| Model | Description | Guest Access |
|-------|-------------|:---:|
| `glm-4.7` | Fast, lightweight — suited for simple tasks | ✅ |
| `glm-5` | Default capable model | ❌ Login required |
| `GLM-5-Turbo` | Best for complex and long-context tasks | ❌ Login required |
| `GLM-5v-Turbo` | Vision-capable variant | ❌ Login required |
| `GLM-5.1` | Latest generation | ❌ Login required |
| `claude-sonnet-*` | Alias → `glm-5` | ❌ Login required |
| `claude-opus-*` | Alias → `GLM-5-Turbo` | ❌ Login required |
| `claude-haiku-*` | Alias → `glm-5` | ❌ Login required |

---

## API Reference

### Anthropic-Compatible

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/messages` | `POST` | Native Anthropic Messages API — streaming SSE and `tool_use` blocks |
| `/v1/models` | `GET` | Model list (Anthropic-style IDs) |

**Non-streaming**

```bash
curl -X POST http://localhost:3001/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: Waguri" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-6",
    "max_tokens": 100,
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**Streaming**

```bash
curl -X POST http://localhost:3001/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: Waguri" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-6",
    "max_tokens": 500,
    "stream": true,
    "messages": [{"role": "user", "content": "Say hi"}]
  }'
```

**Supported request fields**

| Field | Type | Notes |
|-------|------|-------|
| `model` | string | Any Claude model name — mapped to GLM internally |
| `messages` | array | Standard Anthropic format (`user` / `assistant` turns) |
| `system` | string \| array | System prompt — string or content block array |
| `stream` | boolean | Enable SSE streaming |
| `max_tokens` | number | Accepted, not forwarded |
| `tools` | array | Tool definitions injected into the prompt |
| `tool_choice` | object | Accepted, ignored |
| `temperature` | number | Accepted, ignored |

---

### OpenAI-Compatible

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | `POST` | Chat completions — streaming and non-streaming |
| `/v1/models` | `GET` | List available models |

**Basic request**

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -d '{
    "model": "glm-5",
    "messages": [{"role": "user", "content": "What is 2+2?"}]
  }'
```

**With web search**

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -d '{
    "model": "glm-5",
    "webSearch": true,
    "messages": [{"role": "user", "content": "Latest AI news"}]
  }'
```

**Fresh session** (clears context for this request)

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -H "X-Fresh-Session: true" \
  -d '{"model": "glm-5", "messages": [{"role": "user", "content": "Start fresh"}]}'
```

---

### Management & Admin

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/features` | `POST` | Toggle `webSearch`, `thinking`, `imageGen`, `previewMode`, `persistHistory` |
| `/status` | `GET` | Session status and feature flags |
| `/admin/health` | `GET` | Health check (`200` healthy, `503` not ready) |
| `/admin/stats` | `GET` | Usage statistics |
| `/admin/session/clear` | `POST` | Clear all conversation history |
| `/prompt` | `POST` | Legacy single-prompt endpoint |

**Toggle features**

```bash
curl -X POST http://localhost:3001/features \
  -H "Authorization: Bearer Waguri" \
  -H "Content-Type: application/json" \
  -d '{"webSearch": true, "thinking": true}'
```

**Clear session history**

```bash
curl -X POST http://localhost:3001/admin/session/clear \
  -H "Authorization: Bearer Waguri"
```

---

## Tool Call Support

When `PARSE_TOOL=true`, the bridge detects and parses XML tool calls emitted by the model and converts them to structured `tool_use` (Anthropic) or `tool_calls` (OpenAI) blocks. Raw tool syntax is stripped from the text block.

**Supported formats**

Generic XML:

```xml
<tool_call>
<function=write_to_file>
<parameter=path>output.txt</parameter>
<parameter=content>Hello World</parameter>
</function>
</tool_call>
```

Roo / Cline style:

```xml
<write_to_file>
<path>output.txt</path>
<content>Hello World</content>
</write_to_file>
```

**Supported tool categories**

| Category | Tools |
|----------|-------|
| File write | `write_file`, `write_to_file`, `create_file` |
| File read | `read_file`, `read_from_file`, `read_multiple_files` |
| File edit | `edit_file`, `replace_in_file`, `apply_diff` |
| File management | `delete_file`, `move_file`, `copy_file`, `rename_file` |
| Directory | `list_files`, `list_directory`, `find_files` |
| Search | `search_files`, `search_code`, `grep_search` |
| Shell | `execute_command`, `run_command`, `execute_shell` |
| Task flow | `attempt_completion`, `complete_task`, `finish_task` |
| Interaction | `ask_followup_question`, `ask_question` |
| OpenCode | `write`, `read`, `edit`, `bash`, `glob`, `grep`, `task`, `webfetch`, `todowrite`, `todoread` |

---

## Roo Code / Kilo Code Setup

| Setting | Value |
|---------|-------|
| API Base URL | `http://localhost:3001/v1` |
| API Key | `Waguri` |
| Model | `glm-5` or `GLM-5-Turbo` (requires `ZAI_TOKEN`) |

---

## Architecture

**Direct HTTP mode (`main.js`)**

```
┌──────────────────┐     ┌────────────────────────┐     ┌────────────────┐
│  API Client      │────▶│  main.js               │────▶│  chat.z.ai     │
│  Claude Code /   │     │  HMAC-signed HTTP       │     │  REST API      │
│  Roo / curl      │◀────│  OpenAI + Anthropic     │◀────│                │
└──────────────────┘     └────────────────────────┘     └────────────────┘
```

**Browser Automation mode (`browser.js`) — deprecated**

```
┌──────────────┐     ┌────────────────────────┐  WS  ┌──────────────────┐
│  API Client  │────▶│  browser.js            │◀────▶│  Browser Tab     │
│              │◀────│  WebSocket pool         │      │  (chat.z.ai)     │
└──────────────┘     └────────────────────────┘      └──────────────────┘
```

---

## File Reference

| File | Description |
|------|-------------|
| `main.js` | Direct HTTP server — primary entrypoint |
| `browser.js` | Browser automation server *(deprecated)* |
| `config.js` | Shared configuration and environment variable mappings |
| `src/pool.js` | Browser client pool *(Browser mode only)* |
| `src/injection.js` | Browser-side injection script *(Browser mode only)* |
