# GLM Bridge — Z.AI Proxy API

An OpenAI- **and Anthropic-compatible** API proxy for [chat.z.ai](https://chat.z.ai). Drop it in front of any OpenAI- or Anthropic-compatible tool and start using Z.AI's GLM models without browser automation or complex setup at runtime.

---

## Features

- **Dual protocol** — OpenAI `/v1/chat/completions` + Anthropic `/v1/messages` on the same server
- **Pure HTTP** — No Playwright, no Selenium, no browser overhead at runtime
- **Background captcha cache** — When agent mode is enabled, captcha params are pre-generated asynchronously (2 cached, 75 s TTL, auto-pauses after 3 min idle)
- **Streaming + non-streaming** — Full SSE support with keep-alive pings every 5 s
- **Agent mode** — Translates OpenAI tools / function-calling into a text contract that Z.AI can understand, then intercepts `<<<TOOL_CALL>>>` blocks from the model's output and rewrites them back into native `tool_calls` deltas
- **Per-model feature resolution** — Features resolved per-model from Z.AI server capabilities, with user overrides stored per-model. `image_generation` is **always forced to `false`**.
- **`reasoning_effort` support** — `high` / `max` values forwarded only when the model's capabilities explicitly allow it; `enable_thinking` is force-enabled when active
- **Token pool** — Device tokens harvested via `captcha.go` and stored in `tokens.sqlite`. Consumed FIFO and removed after use (up to 5 retries per captcha computation)
- **Live model list** — Models fetched from Z.AI `/api/models` (cached 5 min, falls back to a static list)
- **Pure-Go SQLite** — Uses `modernc.org/sqlite` — no CGO required
- **HTTP/2 + pooled connections** — Optimised transport for both Aliyun and Z.AI endpoints

---

## Supported Models

Models are fetched live from Z.AI's `/api/models` (cached 5 min). The server keeps models from the newest down to `glm-4.7` (inclusive). The fallback list (used if Z.AI is unreachable) is:

| Model ID | Notes |
|---|---|
| `glm-5.2` | Flagship model, excels at coding and long-horizon tasks |
| `GLM-5.1` | Previous flagship model |
| `GLM-5-Turbo` | New model for chat, coding, and agentic tasks |
| `GLM-5v-Turbo` | Vision model with evolved intelligence |
| `glm-4.7` | Classic high-performance model |

> **Note:**
> - If you don't pass `model` in `/v1/chat/completions` or `/v1/messages`, the server defaults to `glm-5`.
> - Z.AI's guest session (no `ZAI_TOKEN`) typically only allows `glm-4.7`. Use `glm-4.7` for tokenless testing.
> - `/models` (plural) returns `{ models: [...], currentModel: "glm-5.2" }` for clients that expect that shape.

---

## Getting `ZAI_TOKEN` (optional, but recommended)

`ZAI_TOKEN` is a Z.AI JWT. Setting it skips guest initialization and unlocks all models.

1. Go to **https://chat.z.ai** and log in.
2. Open browser **DevTools** (`F12` or `Ctrl+Shift+I`).
3. Navigate to **Application → Local Storage → https://chat.z.ai**.
4. Find the key named **`token`** and copy its value.
5. Export it before starting the server:

   ```bash
   # Linux / macOS
   export ZAI_TOKEN="paste-the-copied-jwt-here"

   # Windows PowerShell
   $env:ZAI_TOKEN="paste-the-copied-jwt-here"
   ```

   Or, in the DevTools **Console** tab, run:

   ```js
   localStorage.getItem('token')
   ```

   and copy the printed string.

---

## Getting Started

```bash
# 1. Clone the repo
git clone https://github.com/izaart95-jpg/GLM-Free-API/ zai-api
cd zai-api

# 2. Initialize the Go module
go mod init zai-api
go mod tidy

# 3. Generate the token database
go run captcha.go
# Recommended: build first for better performance and faster startup:
#   go build -o token-collector -trimpath -gcflags="all=-l=4" -ldflags="-s -w" captcha.go && ./token-collector

# 4. Start the server
go run main.go
# Recommended: build first for better performance and faster startup:
#   go build -o zai-api -trimpath -gcflags="all=-l=4" -ldflags="-s -w" main.go && ./zai-api
```

On startup, you'll see a banner with your health URL, API endpoints, and auth token. The Z.AI session is initialised asynchronously — if guest init fails, the first chat request will retry it.

---

## Configuration

### CLI Flags

| Flag | Default | Description |
|---|---|---|
| `--db-path` | `tokens.sqlite` | Path to the SQLite token database |
| `--verbose` | `false` | Enable verbose captcha/debug logging (`logError` / `logInfo` are silent unless this is set) |
| `--agent-mode` | `false` | Enable agent mode (translate tools & roles for Z.AI compatibility; also starts the background captcha cache) |

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `3001` | HTTP server port |
| `HOST` | `0.0.0.0` | Bind address |
| `AUTH_TOKEN` | `Waguri` | Bearer / `x-api-key` token for client authentication |
| `TIMEOUT` | `300000` | Request timeout in milliseconds |
| `ZAI_TOKEN` | *(empty)* | Hardcoded Z.AI JWT — skips guest initialization |
| `AGENT_MODE` | `false` | Enable agent mode (`1`/`true`/`yes`/`on` to enable) |
| `LOG_LEVEL` | `debug` | Log level (`debug` dumps every Z.AI request/response, SSE lines, and headers) |
| `LOG_FORMAT` | `text` | Log format |

---

## API Reference

> **Reminder:** On the hosted instance (`api.lelouch.indevs.in`), the **OpenAI endpoint is recommended**. The Anthropic endpoint works but is considered primitive.

### OpenAI-Compatible

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/v1/chat/completions` | ✅ | Chat completions (streaming + non-streaming) |
| `GET`  | `/v1/models` | ✅ | OpenAI-style model list |
| `GET`  | `/models` | ✅ | Compact `{ models, currentModel }` shape |

### Anthropic-Compatible

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/v1/messages` | ✅ | Anthropic Messages API (streaming + non-streaming, tool use, thinking) |

Anthropic clients authenticate via the `x-api-key` header (same value as `AUTH_TOKEN`). The `anthropic-version` header is allowed through CORS.

#### `/v1/chat/completions` body

| Field | Type | Default | Notes |
|---|---|---|---|
| `model` | string | `glm-5` | Any model ID from `/v1/models` |
| `messages` | array | *(required)* | OpenAI-style message array |
| `stream` | bool | `true` | SSE stream when true |
| `reasoning` | bool | *(per-model)* | Enables `enable_thinking` for this request |
| `thinking` | object | *(per-model)* | `{"type":"enabled"}` or `{"type":"disabled"}` → `enable_thinking` |
| `reasoning_effort` | string | *(empty)* | `"high"` or `"max"` — only forwarded if the model supports it; forces `enable_thinking=true` |
| `tools` | array | *(empty)* | OpenAI-style tools (requires agent mode) |
| `webSearch` / `search` | bool | *(per-model)* | Toggle `auto_web_search` + `web_search` |

#### `/v1/messages` body

Standard Anthropic Messages API fields: `model`, `messages`, `system`, `max_tokens`, `temperature`, `top_p`, `stop_sequences`, `stream`, `thinking` (`{"type":"enabled"}`), `tools`, `reasoning_effort`.

Tool-use content blocks (`tool_use` / `tool_result`) are translated to/from OpenAI format internally. Requires agent mode for tool calls to function.

#### Request headers

| Header | Purpose |
|---|---|
| `Authorization: Bearer <AUTH_TOKEN>` | OpenAI-style auth |
| `x-api-key: <AUTH_TOKEN>` | Anthropic-style auth |
| `Include-All-Features: true` | (Only for `POST /features`) Send all server capabilities to `/completions` |

### Management

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET`  | `/` | ❌ | Redirects to `/health` |
| `GET`  | `/health` | ❌ | Health check (`200` if initialised, else `503`) |
| `GET`  | `/status` | ❌ | Live session status (JSON: connected, userName, userId, feVersion, features, mode) |
| `GET`  | `/admin/health` | ❌ | Same as `/health` |
| `GET`  | `/admin/stats` | ❌ | Mode, totalClients, totalRequests |
| `GET`  | `/admin/clients` | ❌ | Client list (`[]` or one idle entry if initialised) |
| `POST` | `/features` | ✅ | Per-model feature overrides (see below) |
| `GET`  | `/features` | ✅ | Inspect resolved features / stored states |
| `POST` | `/stop` | ✅ | Acknowledged no-op (returns `{ success: true }`) |
| `GET`  | `/inject.js` | ❌ | Returns `{"message":"Direct mode"}` |

---

## `/features` — Per-Model Feature Configuration

Features are resolved **per model** (not globally). The resolution logic is:

1. Start from the model's server capabilities (`getModelCapabilities`).
2. If `IncludeAll` is set for that model → include **all** capabilities except `reasoning_effort` (which is a support flag, not a feature value). Otherwise include nothing by default.
3. Apply stored user overrides (per-model).
4. `enable_thinking` defaults to `true` unless explicitly overridden.
5. `think` is **never** included — only `enable_thinking` reaches the request.
6. **`image_generation` is always forced to `false`** (overrides are ignored).
7. `reasoning_effort` is **never stored** — it is a per-request parameter validated against model capabilities.

### `GET /features`

- Without query: returns all per-model states:

  ```bash
  curl -X GET http://localhost:3001/features \
    -H "Authorization: Bearer Waguri"
  ```
  ```json
  {
    "states": {
      "glm-4.7": {
        "includeAll": false,
        "overrides": {
          "preview_mode": true,
          "enable_thinking": true
        }
      }
    }
  }
  ```

- With `?model=glm-4.7`: returns the resolved feature map, the stored `includeAll` flag, stored `overrides`, and the model's raw `capabilities`:

  ```bash
  curl -X GET "http://localhost:3001/features?model=glm-4.7" \
    -H "Authorization: Bearer Waguri"
  ```
  ```json
  {
    "model": "glm-4.7",
    "features": {
      "enable_thinking": true,
      "preview_mode": false,
      "image_generation": false
    },
    "includeAll": false,
    "overrides": {
      "enable_thinking": true
    },
    "capabilities": {
      "enable_thinking": true,
      "preview_mode": false,
      "image_generation": true
    }
  }
  ```

### `POST /features`

Body **must** contain `model`. Any other key is treated as a feature override (camelCase keys are converted to snake_case automatically).

**Special key handling:**

| Key | Behaviour |
|---|---|
| `reasoning` (bool) | → stored as `enable_thinking` |
| `thinking` (bool or `{"type":"enabled"}`) | → stored as `enable_thinking` |
| `image_generation` | Ignored — always `false` |
| `think` | Ignored — use `enable_thinking`, `reasoning`, or `thinking` |
| `reasoning_effort` | Not stored — per-request only |

**Toggle thinking for `glm-4.7`:**

```bash
curl -X POST http://localhost:3001/features \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -d '{"model":"glm-4.7","enable_thinking":false}'
```

Response:

```json
{
  "success": true,
  "model": "glm-4.7",
  "includeAll": false,
  "overrides": {
    "enable_thinking": true
  },
  "features": {
    "enable_thinking": true,
    "preview_mode": false,
    "image_generation": false
  }
}
```

To enable **all** server capabilities for a model (e.g. for testing), send the header:

```bash
curl -X POST http://localhost:3001/features \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -H "Include-All-Features: true" \
  -d '{"model":"glm-4.7"}'
```

> `image_generation` is **always** `false` — sending it in the body has no effect.

---

## Agent Mode

> The hosted instance at `api.lelouch.indevs.in` is **already running in agent mode** — you can use tools/function-calling without any extra configuration.

Z.AI's unofficial `/api/v2/chat/completions` endpoint only accepts messages with `role="user"`. System, assistant, and tool roles cause `INTERNAL_ERROR`. OpenAI-style `tools` / `tool_calls` are also rejected.

When `AGENT_MODE` is enabled (via `--agent-mode` flag or `AGENT_MODE=true` env), the server performs three transformations:

1. **Mandatory system prefix** — A user message is prepended explaining the prompt architecture (roles, tools, execution contract) so the model can interpret the rewritten conversation.

2. **Role replacement** — Every non-user message is rewritten as a user message with a `[ROLE: <original_role>]` tag. e.g. a system message `"Do X"` becomes `"[ROLE: system] Do X"`.

3. **Tool translation & simulation** — OpenAI tools JSON is rendered into a user message with a strict contract: the model must emit any tool invocation as:

   ```
   <<<TOOL_CALL>>>
   {"name":"<tool_name>","arguments":{...}}
   <<<END_TOOL_CALL>>>
   ```

   The stream interceptor detects this token sequence in the assistant's output, parses the JSON, and rewrites the chunk into an OpenAI-style `tool_calls` delta (or Anthropic `tool_use` content block) with `finish_reason="tool_calls"` / `stop_reason="tool_use"`.

Enabling agent mode also starts the **background captcha cache**, which pre-generates captcha verify params asynchronously (2 cached, 75 s TTL, auto-pauses after 3 minutes of inactivity) to reduce per-request latency.

---

## Examples

> The examples below use `localhost` for self-hosting. For the hosted instance, swap the base URL to `https://api.lelouch.indevs.in/v1` and use `glm-5.2` — auth stays `Waguri`.

All self-hosted examples use `glm-4.7` so they work **without** `ZAI_TOKEN`.

**Basic non-streaming request (OpenAI)**

```bash
curl -X POST http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -d '{
    "model": "glm-4.7",
    "stream": false,
    "messages": [{"role": "user", "content": "Hello, who are you?"}]
  }'
```

**Streaming (SSE)**

```bash
curl -N -X POST http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -d '{
    "model": "glm-4.7",
    "stream": true,
    "messages": [{"role": "user", "content": "Write a haiku about Go."}]
  }'
```

**Deep thinking**

```bash
curl -N -X POST http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -d '{
    "model": "glm-4.7",
    "stream": true,
    "reasoning": true,
    "messages": [{"role": "user", "content": "Summarize today'\''s top AI news."}]
  }'
```

**Anthropic Messages API** *(primitive on the hosted instance — OpenAI recommended)*

```bash
curl -X POST http://localhost:3001/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: Waguri" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "glm-4.7",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello, who are you?"}]
  }'
```

**Anthropic streaming with thinking**

```bash
curl -N -X POST http://localhost:3001/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: Waguri" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "glm-4.7",
    "max_tokens": 4096,
    "stream": true,
    "thinking": {"type": "enabled"},
    "messages": [{"role": "user", "content": "Explain quantum entanglement."}]
  }'
```

**Python (OpenAI SDK) — using the hosted instance**

```python
from openai import OpenAI

client = OpenAI(
    base_url="https://api.lelouch.indevs.in/v1",
    api_key="Waguri",
)

resp = client.chat.completions.create(
    model="glm-5.2",
    messages=[{"role": "user", "content": "Hello!"}],
)
print(resp.choices[0].message.content)
```

**Python (OpenAI SDK) — self-hosted**

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:3001/v1",
    api_key="Waguri",
)

resp = client.chat.completions.create(
    model="glm-4.7",
    messages=[{"role": "user", "content": "Hello!"}],
)
print(resp.choices[0].message.content)
```

**Python (Anthropic SDK)** *(primitive — OpenAI recommended)*

```python
from anthropic import Anthropic

client = Anthropic(
    base_url="http://localhost:3001",
    api_key="Waguri",
)

resp = client.messages.create(
    model="glm-4.7",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello!"}],
)
print(resp.content[0].text)
```

---

## How It Works

1. **Guest token** — On startup, the server calls Z.AI's `/api/v1/auths/guest` (and `/api/v1/auths/`) for a session JWT, or uses `ZAI_TOKEN` if provided. The frontend version (`prod-fe-x.y.z`) is scraped from the Z.AI homepage.

2. **Captcha** — For each request, an Aliyun `captcha_verify_param` is generated **in-memory** (no FIFO, no named pipe):
   - `InitCaptchaV3` → obtain `certifyId`
   - Generate `arg` via RC4-like permutation cipher (KSA + PRGA over a 64-byte state)
   - Build a `Track` JSON, compute `ali_hash` (custom 16-byte-state hash), zlib-compress, base64-encode, then `encrypt` (second RC4-like pass with a different key)
   - `VerifyCaptchaV3` with a pooled device token → receive `securityToken`
   - Base64-encode the final `{ certifyId, isSign, sceneId, securityToken }` payload
   - Tokens are consumed FIFO from `tokens.sqlite` and deleted after use (up to **5 retries** per captcha computation)
   - When agent mode is on, a background cache pre-generates params (2 cached, 75 s TTL) to reduce latency

3. **Signature** — HMAC-SHA256 over `(sortedPayload | promptBase64 | timestamp)` with a salted bucket key derived from `SALT_KEY` and `timestamp / 300000`.

4. **Streaming** — POST to `/api/v2/chat/completions` with `stream: true`, parse SSE chunks (`edit_content` with `edit_index`, `delta_content`, `content`, or OpenAI-style `choices[0].delta.content`), and forward as OpenAI-formatted SSE or Anthropic SSE events. Inline errors (HTTP 200 with `data.error`) are detected and surfaced. On `401`, the session is re-initialised and the request retried once.

---

## Token Collection (`captcha.go`)

The `captcha.go` script seeds `tokens.sqlite` with device tokens harvested from `chat.z.ai` using Playwright. It features a full TUI (Bubble Tea) with progress bar, live logs, and spinner.

### Build & Run

```bash
# Portable fallback (any CPU / any OS)
go build -ldflags="-s -w" -trimpath -o token-collector captcha.go
./token-collector

# Or, for modern CPUs (fully static, stripped)
CGO_ENABLED=0 GOAMD64=v3 go build -ldflags="-s -w" -trimpath -o token-collector captcha.go
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `--unsafe` | `false` | Increase token limit to 1500, batch limit to 25, parallel limit to 5 |
| `--tokens` | `0` | Tokens per batch (0 = prompt; default 850, max 1500) |
| `--batch` | `0` | Number of batches (0 = prompt; default 5, max 9 / 25 with `--unsafe`) |
| `--parallel` | `0` | Parallel workers (pages) on a single browser; 0 = prompt y/N (max 3 / 5 with `--unsafe`) |
| `--headed` | `false` | Show browser window for debugging |
| `--block-trackers` | `false` | Enable URL allowlist filter to block tracker/analytics requests |
| `--no-tui` | `false` | Disable TUI, use plain text output |

### Usage Examples

```bash
# Interactive prompts (TUI)
./token-collector

# High volume
./token-collector --unsafe

# Specific configuration
./token-collector --tokens 750 --batch 3

# Parallel collection with a visible browser
./token-collector --parallel 3 --headed

# Plain text output (no TUI)
./token-collector --no-tui

# Block tracker domains during collection
./token-collector --block-trackers
```

### Implementation Details

- Uses pure-Go SQLite (`modernc.org/sqlite`) — no CGO needed.
- Opens a single DB connection with WAL mode + 64 MB page cache + 256 MB mmap.
- Keeps one page open per worker across all batches (force-reloads instead of open/close).
- Runs batches in parallel using multiple pages on a single browser instance.
- GC tuned to 200% to reduce pause times in allocation-heavy workloads.
- Uses lock-free atomics for the abort flag and running total.
- TUI captures stdout/stderr into a ring buffer (1000 lines) with scroll support (↑/↓, g/G).
- Optional URL allowlist blocks tracker/analytics domains — only allows `chat.z.ai`, z-cdn build assets, Aliyun captcha scripts, and `cloudauth-device` endpoints.

---

## Project Structure

```
zai-api/
├── main.go          # HTTP server, captcha generation, Z.AI bridge, OpenAI + Anthropic shims, agent mode
├── captcha.go       # Seeds tokens.sqlite with device tokens (TUI + parallel collection)
├── tokens.sqlite    # Generated token pool (consumed at runtime)
├── go.mod
└── README.md
```

---

## Notes

- **Hosted instance:** `https://api.lelouch.indevs.in/v1` — Bearer / `x-api-key`: `Waguri` — supports `glm-5.2` — running in agent mode. OpenAI API recommended; Anthropic API is primitive.
- Device tokens are **consumed and deleted** after use. Re-run `captcha.go` to replenish the pool. Each captcha computation tries up to **5 tokens**.
- The default auth token (`Waguri`) is a placeholder — set `AUTH_TOKEN` in production.
- `ZAI_TOKEN` bypasses guest initialization entirely. Without it, Z.AI's guest session typically only permits `glm-4.7`.
- `LOG_LEVEL=debug` dumps every Z.AI request body, response status/headers, and SSE lines — useful for troubleshooting.
- `image_generation` is **always `false`** and cannot be enabled via `/features` or per-request overrides.
- The captcha step has a hard 90-second timeout; if it fails, the request returns `500`.
- `--verbose` controls only the captcha subsystem's `logInfo` / `logError` output. Standard `log.*` calls (Z.AI bridge, SSE debug) are gated by `LOG_LEVEL=debug`.
- Each request gets a fresh `chat_id` — there is no server-side conversation persistence across requests.
- Agent mode is **required** for tool-calling / function-calling to work. Without it, `tools` in the request body are ignored.
- `reasoning_effort` (`"high"` / `"max"`) is only forwarded to Z.AI when the model's capabilities explicitly include `"reasoning_effort": true`. When active, `enable_thinking` is forced to `true`.

---

## License

Provided as-is for educational and interoperability purposes. Use responsibly and in accordance with Z.AI's terms of service.
