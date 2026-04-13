# Z.AI Proxy API

> OpenAI & Anthropic-compatible API proxy for [chat.z.ai](https://chat.z.ai) — available in two modes.

> ⚠️ **Warning**: If you are getting error 405, it means Z.ai has blocked your IP Address for abusing the web UI models through unofficial API usage.  
> **Best practice**: Be chill, stop sending requests, wait, and do not send prompts longer than 60,000 characters.

## Overview

| Mode | File | Description | Status |
|------|------|-------------|--------|
| ⚡ **Direct HTTP** | `main.js` | Calls Z.AI's REST API directly using HMAC signatures. No browser required. | ✅ Recommended |
| 🌐 **Browser Automation** | `browser.js` | Connects to a live browser tab via WebSocket injection. Requires browser open. | ⚠️ Deprecated |

> **Recommendation:** Use `main.js`. It's faster, more stable, and requires no browser setup.  
> `browser.js` is deprecated and will not receive further updates. Switch to it only as a fallback if you encounter issues with `main.js`.

---

## What's New

- **GLM 5.1** model added
- **Anthropic API** (`/v1/messages`) support added — native SSE streaming.
- **Claude Code** integration added (no LiteLLM required)
- **Tool call parse toggle** — choose between parsed `tool_use` blocks or raw passthrough
- **Core instructions toggle** — optionally inject Roo/Cline XML tool format hints into every prompt
- **Debug mode added** - log exact request body sent to z.ai through proxy

---

## Features

- **OpenAI-Compatible API** — Drop-in replacement for the OpenAI API
- **Anthropic-Compatible API** — Native `/v1/messages` endpoint for Claude Code and Anthropic SDK
- **Streaming Support** — Real-time SSE streaming responses (both formats)
- **Tool Call Parsing** — Full support for Roo Code / Kilo Code XML tool format
- **Session Management** — Fresh session support via `X-Fresh-Session` header
- **Feature Toggles** — Web search, deep thinking, image generation, preview mode
- **Auto Session Recovery** — Re-authenticates automatically on token expiry *(Direct mode)*

---

## Quick Start

### 1. Clone and Install

```bash
git clone https://github.com/izaart95-jpg/GLM-Bridge.git
cd GLM-Bridge
npm install
```

---

### Mode A — Direct HTTP (Recommended)

No browser needed. Authenticates as a guest and calls Z.AI's internal API directly.

```bash
node main.js
```

Server starts on `http://localhost:3001`.

---

### Mode B — Browser Automation (Legacy)

**Requires** a browser tab open at `https://chat.z.ai`.

```bash
node browser.js
```

Then open the browser console on `https://chat.z.ai` and run:

```javascript
const script = document.createElement('script');
script.src = 'http://localhost:3001/inject.js';
document.head.appendChild(script);
```

> ⚠️ **Important:** Keep the browser tab open and in the foreground while using this mode.

---

## Claude Code Integration (No LiteLLM Required)

The server exposes a native Anthropic-compatible `/v1/messages` endpoint. Point Claude Code directly at it.

### Windows PowerShell

```powershell
$env:ANTHROPIC_BASE_URL = "http://localhost:3001"
$env:ANTHROPIC_AUTH_TOKEN = "Waguri"
$env:ANTHROPIC_API_KEY = ""
claude
```

### Windows CMD

```cmd
set ANTHROPIC_BASE_URL=http://localhost:3001
set ANTHROPIC_AUTH_TOKEN=Waguri
set ANTHROPIC_API_KEY=
claude
```

### Permanent — `~/.claude/settings.json`

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://localhost:3001",
    "ANTHROPIC_AUTH_TOKEN": "Waguri",
    "ANTHROPIC_API_KEY": ""
  }
}
```

Claude model names are mapped to Z.AI models automatically:

| Claude Model | Z.AI Model Used |
|---|---|
| `claude-opus-*` | `GLM-5-Turbo` |
| `claude-sonnet-*` | `glm-5` |
| `claude-haiku-*` | `glm-5` |

---

## API Reference

### Anthropic-Compatible Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/messages` | `POST` | Native Anthropic Messages API — streaming SSE + `tool_use` blocks |
| `/v1/models` | `GET` | List models (returns Anthropic-style model IDs) |

#### Example — Non-Streaming

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

#### Example — Streaming

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

#### Supported Request Fields

| Field | Type | Description |
|-------|------|-------------|
| `model` | string | Any Claude model name (mapped to GLM internally) |
| `messages` | array | Anthropic messages format — `user` / `assistant` turns |
| `system` | string \| array | System prompt (string or content block array) |
| `stream` | boolean | Enable SSE streaming |
| `max_tokens` | number | Accepted, not forwarded |
| `tools` | array | Accepted — tool definitions are injected into the prompt |
| `tool_choice` | object | Accepted, ignored |
| `temperature` | number | Accepted, ignored |

#### Response Format

Non-streaming returns a standard Anthropic message object with `content` blocks of type `text` and (when detected) `tool_use`. The `stop_reason` is `"tool_use"` when tool calls are present, otherwise `"end_turn"`.

---

### OpenAI-Compatible Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/models` | `GET` | List available models |
| `/v1/chat/completions` | `POST` | Chat completion (streaming / non-streaming) |

### Legacy Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/prompt` | `POST` | Simple prompt endpoint |
| `/models` | `GET` | List models (legacy format) |
| `/features` | `POST` | Toggle `webSearch`, `thinking`, `imageGen`, `previewMode` |

### Admin Endpoints

| Endpoint | Method | Description | Mode |
|----------|--------|-------------|------|
| `/status` | `GET` | Session / pool status | Both |
| `/admin/health` | `GET` | Health check | Both |
| `/admin/stats` | `GET` | Statistics | Both |
| `/admin/clients` | `GET` | List clients / session info | Both |
| `/admin/session/clear` | `POST` | Clear conversation history and generate new `chatId` | Direct |
| `/admin/clients/:id/clear` | `POST` | Clear client chat history | Both |
| `/stop` | `POST` | Stop current generation | Both |
| `/inject.js` | `GET` | Browser injection script | Browser |

---

## Configuration

Configure via environment variables or `config.js`.

### Server & Auth

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3001` | Server port |
| `HOST` | `0.0.0.0` | Server host |
| `AUTH_TOKEN` | `Waguri` | API authentication token |
| `TIMEOUT` | `120000` | Default request timeout (ms) |

### Behavior Toggles (in `main.js`)

These are constants at the top of `main.js` you can flip before starting the server:

```js
// ============== Z.AI DIRECT CONFIG ==============

const PARSE_TOOL_CALLS     = true;   // true  → parse XML/JSON tool calls into tool_use blocks
                                      // false → pass raw model output through unchanged

const INCLUDE_CORE_INSTRUCTIONS = false; // true  → prepend Roo/Cline XML tool format hints to every prompt
                                          // false → send prompts as-is (default)
```

#### `PARSE_TOOL_CALLS`

| Value | Behavior |
|-------|----------|
| `true` (default) | XML / JSON tool call syntax in the model's response is detected, parsed, and returned as proper `tool_use` content blocks (Anthropic format) or `tool_calls` (OpenAI format). Raw tool syntax is stripped from the `text` block. |
| `false` | The model's raw output is returned as-is inside a single `text` block. Useful if you want to handle tool call parsing yourself, or if the model's native output format is preferred. |

#### `INCLUDE_CORE_INSTRUCTIONS`

| Value | Behavior |
|-------|----------|
| `false` (default) | Prompts are forwarded to Z.AI without modification. |
| `true` | A block of critical instructions is prepended to every prompt, telling the model to emit tool calls in the XML format expected by Roo Code / Kilo Code. Enable this if tool calls are not being emitted correctly. |

---

### Timeout Handling

If you experience timeout issues, increase these values:

```bash
export TIMEOUT=300000                  # 5 minutes
export STREAMING_CHUNK_TIMEOUT=120000  # 2 minutes
```

---

## Usage Examples

### Basic Chat Completion (OpenAI)

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -d '{
    "model": "glm-4.7",
    "messages": [{"role": "user", "content": "What is 2+2?"}]
  }'
```

### Streaming Response (OpenAI)

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -d '{
    "model": "glm-4.7",
    "stream": true,
    "messages": [{"role": "user", "content": "Write a haiku"}]
  }'
```

### With Web Search Enabled

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -d '{
    "model": "glm-4.7",
    "webSearch": true,
    "messages": [{"role": "user", "content": "What is the latest news?"}]
  }'
```

### Fresh Session (Clear History)

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -H "X-Fresh-Session: true" \
  -d '{
    "model": "glm-4.7",
    "messages": [{"role": "user", "content": "Start fresh"}]
  }'
```

### Toggle Features (Direct Mode)

```bash
curl -X POST http://localhost:3001/features \
  -H "Authorization: Bearer Waguri" \
  -H "Content-Type: application/json" \
  -d '{"webSearch": true, "thinking": true}'
```

### Clear Session History (Direct Mode)

```bash
curl -X POST http://localhost:3001/admin/session/clear \
  -H "Authorization: Bearer Waguri"
```

### Legacy Prompt Endpoint

```bash
curl http://localhost:3001/prompt \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -d '{
    "prompt": "Hello, how are you?",
    "search": true,
    "deepThink": false
  }'
```

---

## Tool Call Support

Roo Code / Kilo Code XML tool calls are parsed automatically when `PARSE_TOOL_CALLS = true` (default). Works in both OpenAI and Anthropic endpoint modes.

### Supported Formats

**XML Format (Recommended)**

```xml
<tool_call>
<function=write_to_file>
<parameter=path>test.txt</parameter>
<parameter=content>Hello World</parameter>
</function>
</tool_call>
```

**Roo / Cline Style**

```xml
<write_to_file>
<path>test.txt</path>
<content>Hello World</content>
</write_to_file>
```

### Supported Tools

| Category | Tools |
|----------|-------|
| **File Write** | `write_file`, `write_to_file`, `create_file` |
| **File Read** | `read_file`, `read_from_file`, `read_multiple_files` |
| **File Edit** | `edit_file`, `replace_in_file`, `apply_diff` |
| **File Management** | `delete_file`, `move_file`, `copy_file`, `rename_file` |
| **Directory** | `list_files`, `list_directory`, `find_files` |
| **Search** | `search_files`, `search_code`, `grep_search` |
| **Shell** | `execute_command`, `run_command`, `execute_shell` |
| **Task Flow** | `attempt_completion`, `complete_task`, `finish_task` |
| **Interaction** | `ask_followup_question`, `ask_question` |
| **Misc** | `browser_action`, `update_todo_list`, `switch_mode`, `new_task`, `fetch_instructions` |
| **OpenCode** | `write`, `read`, `edit`, `bash`, `glob`, `grep`, `task`, `webfetch`, `todowrite`, `todoread` |

---

## Roo Code / Kilo Code Integration

In your Roo Code or Kilo Code settings, configure:

| Setting | Value |
|---------|-------|
| **API Base URL** | `http://localhost:3001/v1` |
| **API Key** | `Waguri` |
| **Model** | `glm-4.7`, `glm-5`, or `GLM-5-Turbo` |

---

## Models

| Model | Description |
|-------|-------------|
| `glm-5` | Default model (Direct mode) |
| `GLM-5-Turbo` | GLM 5 Turbo for complex tasks |
| `GLM-5v-Turbo` | GLM 5V Turbo (vision) |
| `glm-4.7` | GLM 4.7 — good for fast tasks |
| `claude-sonnet-4-6` | Alias → `glm-5` (for Claude Code) |
| `claude-opus-4-6` | Alias → `GLM-5-Turbo` (for Claude Code) |
| `claude-haiku-4-5-*` | Alias → `glm-5` (for Claude Code) |

---

## Architecture

### Direct HTTP Mode (`main.js`)

```
┌─────────────────┐     ┌──────────────────────┐     ┌─────────────────┐
│  API Client     │────▶│  main.js             │────▶│  chat.z.ai      │
│  (Claude Code,  │     │  HMAC-signed HTTP    │     │  REST API       │
│   Roo, curl)    │     │  OpenAI + Anthropic  │     │                 │
└─────────────────┘     └──────────────────────┘     └─────────────────┘
```

### Browser Automation Mode (`browser.js`)

```
┌─────────────┐     ┌──────────────────────┐  WS  ┌─────────────────┐
│  API Client │────▶│  browser.js          │◀────▶│  Browser Tab    │
│  (Roo/curl) │     │  WebSocket pool      │      │  (chat.z.ai)    │
└─────────────┘     └──────────────────────┘      └─────────────────┘
```

---

## File Reference

| File | Description |
|------|-------------|
| `main.js` | Direct HTTP server — no browser required |
| `browser.js` | Browser automation server *(deprecated)* |
| `config.js` | Shared configuration |
| `src/pool.js` | Browser client pool *(Browser mode only)* |
| `src/injection.js` | Browser injection script *(Browser mode only)* |
