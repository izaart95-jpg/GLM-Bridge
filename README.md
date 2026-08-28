# GLM Bridge — Z.AI Proxy API

An OpenAI- **and Anthropic-compatible** API proxy for [chat.z.ai](https://chat.z.ai). Drop it in front of any OpenAI/Anthropic-compatible tool and use Z.AI's GLM models — pure HTTP, no browser automation at runtime.

**Hosted instance:** `https://glm.cognix.sryze.cc/v1` — auth `Waguri` (Bearer or `x-api-key`), runs agent mode, supports `glm-5.2` and `glm-5.3-flash`. OpenAI endpoint recommended; Anthropic works but is primitive.

---

## Features

- **Dual protocol** — OpenAI `/v1/chat/completions` + Anthropic `/v1/messages` on one server; streaming (SSE, 5 s keep-alive pings) and non-streaming
- **Pure HTTP** — no Playwright/Selenium/browser at runtime; HTTP/2 + pooled connections; pure-Go SQLite (`modernc.org/sqlite`, no CGO)
- **Vision (image input)** — images on both endpoints (OpenAI `image_url` parts, Anthropic `image` blocks; URL or base64). Each is downloaded/decoded, uploaded to Z.AI's file endpoint, and attached exactly like the official web client. **10 images / 50 MB each** per request. Requires `ZAI_TOKEN` (guest sessions get `401`)
- **Throwaway sessions (context-rot guard)** — every request's chat is deleted on Z.AI (`DELETE /api/v1/chats/{chat_id}`) the moment its response completes, so no history outlives a request and the account never accumulates dead sessions
- **Async session pool (default)** — a standing batch of pre-made sessions (`SESSION_POOL_SIZE`, default 5) is kept ready; requests grab one instantly and each consumed session is deleted + replaced, so the batch refills itself. `--sync-mode` restores the legacy per-request flow
- **Agent mode** — translates OpenAI tools/roles into the user-only prompt Z.AI's endpoint accepts, then rewrites the model's `<<<TOOL_CALL>>>` blocks back into native `tool_calls` (OpenAI) / `tool_use` (Anthropic). Modern XML-sectioned shim (default) + legacy `[ROLE: ...]` shim (opt-in)
- **Per-model feature resolution** — resolved from Z.AI server capabilities with stored per-model overrides; `image_generation` is **always forced `false`**; `reasoning_effort` (`high`/`max`) forwarded only when the model supports it (forces `enable_thinking=true`)
- **Live model list** — full catalog from Z.AI `/api/models` (cached 5 min, static fallback) with OpenAI-style `architecture`/modality on `/v1/models`
- **Token pool** — device tokens harvested via `cmd/token-collector` into `tokens.sqlite`, consumed FIFO and deleted after use (up to 5 retries per captcha)
- **Background captcha cache** — with agent mode on, captcha params are pre-generated asynchronously (2 cached, 75 s TTL, auto-pauses after 3 min idle)
- **Graceful shutdown** — CTRL+C/SIGTERM drains in-flight requests (10 s), then deletes every remaining pooled session on Z.AI before exiting (second CTRL+C force-exits)

---

## Supported Models

Models are fetched live from Z.AI's `/api/models` (cached 5 min). The server keeps models from the newest down to `glm-4.7` (inclusive). The fallback list (used if Z.AI is unreachable) is:

| Model ID | Notes |
|---|---|
| `glm-5.3-flash` | Lightweight flagship, premium quality, instant response |
| `glm-5.3` | Flagship, excels at coding and long-horizon tasks |
| `glm-5.2` | Previous flagship |
| `GLM-5.1` | Older flagship |
| `GLM-5-Turbo` | Chat, coding, and agentic tasks |
| `GLM-5v-Turbo` | Vision model |
| `glm-4.7` | Classic high-performance model |

Each `/v1/models` entry carries an OpenAI-style `architecture` object derived from server capabilities (`info.meta.capabilities`); vision models (`"vision": true`) advertise image input:

```json
{
  "id": "GLM-5v-Turbo",
  "object": "model",
  "created": 1774521032,
  "owned_by": "z-ai",
  "display_name": "GLM-5V-Turbo",
  "description": "Vision model with evolved intelligence",
  "architecture": {
    "modality": "text+image->text",
    "input_modalities": ["text", "image"],
    "output_modalities": ["text"]
  }
}
```

Text-only models report `"modality": "text->text"` / `"input_modalities": ["text"]`; `output_modalities` is always `["text"]`.

> **Notes:**
> - No `model` in the request body → defaults to `glm-5`.
> - Guest session (no `ZAI_TOKEN`) typically only allows `glm-5.3-flash` and `glm-4.7` — use `glm-4.7` for fast tokenless testing.
> - `/models` (plural) returns `{ models: [...], currentModel: "glm-5.2" }` for clients expecting that shape.

---

## Getting `ZAI_TOKEN` (optional, but recommended)

`ZAI_TOKEN` is a Z.AI JWT. Setting it skips guest initialization and unlocks all models (and vision uploads).

1. Log in at **https://chat.z.ai**, open DevTools (`F12`) → **Application → Local Storage → https://chat.z.ai** → copy the value of key **`token`** (or run `localStorage.getItem('token')` in the Console).
2. Export it before starting the server:

   ```bash
   export ZAI_TOKEN="paste-the-copied-jwt-here"        # Linux / macOS
   $env:ZAI_TOKEN="paste-the-copied-jwt-here"          # Windows PowerShell
   ```

---

## Getting Started

```bash
# 1. Clone
git clone https://github.com/izaart95-jpg/GLM-Free-API/ zai-api && cd zai-api

# 2. Initialize the Go module (repo ships without go.mod by design)
go mod init zai-api && go mod tidy

# 3. Optional: playwright deps (only for the token collector, when deps are missing)
npx playwright install-deps

# 4. Generate the token database
go run ./cmd/token-collector

# 5. Start the server
go run .
```

For better performance build first: `go build -o zai-api -trimpath -gcflags="all=-l=4" -ldflags="-s -w" . && ./zai-api` (same pattern for `./cmd/token-collector`).

On startup a banner shows the health URL, endpoints, and auth token. The Z.AI session initializes asynchronously — if guest init fails, the first chat request retries it.

---

## Configuration

### CLI Flags

| Flag | Default | Description |
|---|---|---|
| `--db-path` | `tokens.sqlite` | Path to the SQLite token database |
| `--verbose` | `false` | Verbose captcha/debug logging (captcha-subsystem `logInfo`/`logError` are silent otherwise) |
| `--agent-mode` | `false` | Enable agent mode (tools/role translation; starts the background captcha cache) |
| `--agent-mode-variant` | `modern` | Agent shim: `modern` (recommended) or `legacy` |
| `--sync-mode` | `false` | Legacy synchronous flow: fresh chat per request instead of the pre-warmed pool (still GC'd) |

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `3001` | HTTP port |
| `HOST` | `0.0.0.0` | Bind address |
| `AUTH_TOKEN` | `Waguri` | Bearer / `x-api-key` token for client auth |
| `TIMEOUT` | `300000` | Request timeout (ms) |
| `ZAI_TOKEN` | *(empty)* | Hardcoded Z.AI JWT — skips guest initialization |
| `AGENT_MODE` | `false` | Enable agent mode (`1`/`true`/`yes`/`on`/`modern` → modern shim, `legacy` → legacy shim) |
| `AGENT_MODE_VARIANT` | `modern` | Shim variant override (`modern`/`legacy`; takes precedence over `AGENT_MODE`'s implicit variant) |
| `LOG_LEVEL` | `debug` | `debug` dumps every Z.AI request/response, SSE lines, and headers |
| `LOG_FORMAT` | `text` | Log format |
| `STREAM_HOLDBACK` | `24` | Runes held back at a live stream's tail to absorb Z.AI `edit_content` backtracks before they reach the client (`0` disables; issue #23) |
| `SYNC_MODE` | `false` | Legacy synchronous session flow (no pre-warmed pool) |
| `SESSION_POOL_SIZE` | `5` | Standing batch of ready chat sessions |
| `SESSION_ACQUIRE_TIMEOUT` | `10` | Seconds to wait for a pooled session before creating one directly (`0` = wait forever) |

---

## Session Lifecycle — Throwaway Sessions & the Session Pool

OpenAI-compatible clients are **stateless** — they re-send the whole conversation every request, and the bridge forwards it inside a `chat_id` that materializes server-side. Leaving those chats behind would (1) fill the account with dead sessions and (2) cause **context rot** (server-side history stacking on re-sent history). So every request is **throwaway** (design ported from [DeepseekFreeAPI](https://github.com/izaart95-jpg/DeepseekFreeAPI)):

- Each request draws a session, streams its response, and **once fully written (or definitively failed)** the chat is deleted via `DELETE /api/v1/chats/{chat_id}`. Deletion is idempotent — Z.AI's `{"detail":"We could not find what you're looking for :/"}` counts as success.
- Clients never see the `chat_id`; their state lives entirely in their own `messages` array, so deletion is invisible.

**Async mode (default):** at startup the bridge pre-makes `SESSION_POOL_SIZE` (5) sessions. Requests acquire one instantly; if a burst exhausts the batch, waiters give up after `SESSION_ACQUIRE_TIMEOUT` (10 s) and create a session directly. Consumed sessions are deleted + replaced only after the response is fully processed. Z.AI chat IDs are client-generated UUIDs (a chat only materializes on first completion), so warmup is instant/local and unconsumed sessions never touch the account.

**Sync mode (legacy):** `--sync-mode` / `SYNC_MODE=true` — each request creates its own session, completes, then GCs it; no pre-warming.

**Graceful shutdown:** CTRL+C/SIGTERM stops accepting connections, drains in-flight responses (10 s), prints `clearing all remaining sessions...`, deletes every still-pooled session on Z.AI, then exits. A second CTRL+C force-exits.

**Observability:** `GET /status` reports pool state: `{ "mode": "async", "throwaway": true, "gc_enabled": true, "size": 5, "ready": 5 }`.

---

## API Reference

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

Anthropic clients authenticate via `x-api-key` (same value as `AUTH_TOKEN`); `anthropic-version` is allowed through CORS.

#### `/v1/chat/completions` body

| Field | Type | Default | Notes |
|---|---|---|---|
| `model` | string | `glm-5` | Any ID from `/v1/models` |
| `messages` | array | *(required)* | OpenAI-style; `content` may be a string or parts with `text`/`image_url` |
| `stream` | bool | `true` | SSE stream when true |
| `reasoning` | bool | *(per-model)* | Enables `enable_thinking` |
| `thinking` | object | *(per-model)* | `{"type":"enabled"}` / `{"type":"disabled"}` → `enable_thinking` |
| `reasoning_effort` | string | *(empty)* | `"high"`/`"max"` — forwarded only if the model supports it; forces `enable_thinking=true` |
| `tools` | array | *(empty)* | OpenAI-style tools (requires agent mode) |
| `webSearch` / `search` | bool | *(per-model)* | Toggle `auto_web_search` + `web_search` |

#### Image input (vision)

Both endpoints accept images. The bridge extracts every image, uploads it to Z.AI (`POST /api/v1/files/`), and attaches the returned file objects as a top-level `files` array — the same payload the official web client sends. Message text is forwarded as usual; the pixels ride in `files`.

- **OpenAI** — `image_url` parts, remote URL or base64 data URL:
  ```json
  {"role":"user","content":[
    {"type":"text","text":"What is in this image?"},
    {"type":"image_url","image_url":{"url":"https://example.com/photo.jpg"}},
    {"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw..."}}
  ]}
  ```
- **Anthropic** — `image` blocks, `base64` or `url` source:
  ```json
  {"role":"user","content":[
    {"type":"text","text":"What is in this image?"},
    {"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBORw..."}},
    {"type":"image","source":{"type":"url","url":"https://example.com/photo.jpg"}}
  ]}
  ```

**Limits:** max **10 images / 50 MB each** per request — exceeding either returns `400` before any upload. Images are held in memory only (no temp files). Use a vision-capable model (`architecture.input_modalities`); images on a non-vision model log a warning and forward anyway.

> ⚠️ **Requires a real account (`ZAI_TOKEN`).** The `/api/v1/files/` upload endpoint only accepts a logged-in token. On a guest session, model listing and text completions still work, but uploads are rejected with `401` and the request fails with `400 {"message":"image processing failed: file upload unauthorized (401)"}`. Images are uploaded per request (never cached/deduplicated) and are not deleted by the bridge afterwards.

#### `/v1/messages` body

Standard Anthropic fields: `model`, `messages`, `system`, `max_tokens`, `temperature`, `top_p`, `stop_sequences`, `stream`, `thinking`, `tools`, `reasoning_effort`. `tool_use`/`tool_result` blocks translate to/from OpenAI format internally (tool calls require agent mode). Image blocks convert to OpenAI `image_url` parts and use the shared vision pipeline.

#### Request headers

| Header | Purpose |
|---|---|
| `Authorization: Bearer <AUTH_TOKEN>` | OpenAI-style auth |
| `x-api-key: <AUTH_TOKEN>` | Anthropic-style auth |
| `Include-All-Features: true` | (Only `POST /features`) send all server capabilities to `/completions` |

### Management

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET`  | `/` | ❌ | Redirects to `/health` |
| `GET`  | `/health` | ❌ | `200` if initialised, else `503` |
| `GET`  | `/status` | ❌ | Live session status (connected, userName, userId, feVersion, features, mode, sessionPool) |
| `GET`  | `/admin/health` | ❌ | Same as `/health` |
| `GET`  | `/admin/stats` | ❌ | Mode, totalClients, totalRequests |
| `GET`  | `/admin/clients` | ❌ | Client list |
| `POST` | `/features` | ✅ | Per-model feature overrides (below) |
| `GET`  | `/features` | ✅ | Inspect resolved features / stored states |
| `POST` | `/stop` | ✅ | Acknowledged no-op |

---

## `/features` — Per-Model Feature Configuration

Features resolve **per model**: start from the model's server capabilities → if `IncludeAll` is set, include all capabilities except `reasoning_effort` (a support flag, not a feature) → apply stored per-model overrides → `enable_thinking` defaults `true` unless overridden. `think` is never included (only `enable_thinking` reaches the request), `reasoning_effort` is never stored (per-request only), and **`image_generation` is always forced `false`**.

**`GET /features`** — without query returns all per-model states; with `?model=<id>` returns the resolved `features`, stored `includeAll`/`overrides`, and raw `capabilities`:

```bash
curl -H "Authorization: Bearer Waguri" http://localhost:3001/features
curl -H "Authorization: Bearer Waguri" "http://localhost:3001/features?model=glm-4.7"
```

**`POST /features`** — body must contain `model`; any other key is a feature override (camelCase auto-converts to snake_case):

```bash
curl -X POST http://localhost:3001/features \
  -H "Content-Type: application/json" -H "Authorization: Bearer Waguri" \
  -d '{"model":"glm-4.7","enable_thinking":false}'
```

| Special key | Behaviour |
|---|---|
| `reasoning` (bool) | stored as `enable_thinking` |
| `thinking` (bool or `{"type":"enabled"}`) | stored as `enable_thinking` |
| `image_generation` | ignored — always `false` |
| `think` | ignored — use `enable_thinking`/`reasoning`/`thinking` |
| `reasoning_effort` | not stored — per-request only |

To enable **all** server capabilities for a model (testing), add the header `Include-All-Features: true` to the POST.

---

## Agent Mode

Z.AI's unofficial `/api/v2/chat/completions` only accepts `role="user"` messages — system/assistant/tool roles and OpenAI `tools`/`tool_calls` all cause `INTERNAL_ERROR`. With agent mode on (`--agent-mode` / `AGENT_MODE=true`), the bridge folds roles & tools into a user-only prompt and converts the model's `<<<TOOL_CALL>>>` output back into native `tool_calls` deltas (OpenAI) / `tool_use` blocks (Anthropic) with `finish_reason="tool_calls"` / `stop_reason="tool_use"`. **Agent mode is required for tool-calling to work** — without it, `tools` in the body are ignored. The hosted instance already runs it.

**Modern shim (default, recommended)** — folds the whole conversation + tool contract into **one XML-sectioned user message**: `<system>` (output contract), `<tools>` (definitions), `<history_summary>` (older tool exchanges summarized — anti context-rot), `<recent>` (last 6 exchanges verbatim in `<tool_exchange>` groups), `<current_task>`, `<output_rules>`. Tolerant parsing: markers matched with 2–4 angle brackets, ` ```json ` fences stripped, flat/alternate payloads (`tool`, `tool_name`, `function`, `parameters`, `args`, …) normalized, and a hold-back window keeps markers split across SSE chunks from leaking.

**Legacy shim (opt-in)** — `--agent-mode-variant=legacy` / `AGENT_MODE_VARIANT=legacy`: prepends a system-prefix user message, rewrites every non-user message as `[ROLE: <role>] ...`, and renders tools into a strict `<<<TOOL_CALL>>> {"name":...,"arguments":{...}} <<<END_TOOL_CALL>>>` contract.

Enabling agent mode also starts the background captcha cache (2 pre-generated params, 75 s TTL, pauses after 3 min idle).

---

## Examples

> Self-hosted examples use `glm-4.7` so they work **without** `ZAI_TOKEN`. For the hosted instance use base URL `https://glm.cognix.sryze.cc/v1`, model `glm-5.2` or `glm-5.3-flash`, same auth.

```bash
# OpenAI — non-streaming
curl -X POST http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" -H "Authorization: Bearer Waguri" \
  -d '{"model":"glm-4.7","stream":false,"messages":[{"role":"user","content":"Hello, who are you?"}]}'

# OpenAI — streaming with deep thinking
curl -N -X POST http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" -H "Authorization: Bearer Waguri" \
  -d '{"model":"glm-4.7","stream":true,"reasoning":true,"messages":[{"role":"user","content":"Summarize today'\''s top AI news."}]}'

# Anthropic — streaming with thinking (primitive on hosted; OpenAI recommended)
curl -N -X POST http://localhost:3001/v1/messages \
  -H "Content-Type: application/json" -H "x-api-key: Waguri" -H "anthropic-version: 2023-06-01" \
  -d '{"model":"glm-4.7","max_tokens":4096,"stream":true,"thinking":{"type":"enabled"},"messages":[{"role":"user","content":"Explain quantum entanglement."}]}'
```

```python
# Python — OpenAI SDK (self-hosted shown; hosted: base_url="https://glm.cognix.sryze.cc/v1", model="glm-5.3-flash" or "glm-5.2")
from openai import OpenAI
client = OpenAI(base_url="http://localhost:3001/v1", api_key="Waguri")
resp = client.chat.completions.create(model="glm-4.7", messages=[{"role": "user", "content": "Hello!"}])
print(resp.choices[0].message.content)

# Python — Anthropic SDK
from anthropic import Anthropic
client = Anthropic(base_url="http://localhost:3001", api_key="Waguri")
resp = client.messages.create(model="glm-4.7", max_tokens=1024, messages=[{"role": "user", "content": "Hello!"}])
print(resp.content[0].text)
```

---

## How It Works

1. **Session init** — startup calls Z.AI `/api/v1/auths/guest` (+ `/api/v1/auths/`) for a JWT, or uses `ZAI_TOKEN`; the frontend version (`prod-fe-x.y.z`) is scraped from the homepage.
2. **Captcha** — per request, an Aliyun `captcha_verify_param` is generated in-memory: `InitCaptchaV3` → `certifyId`; `arg` via RC4-like permutation cipher (KSA + PRGA over a 64-byte state); `Track` JSON + custom `ali_hash` (16-byte-state hash), zlib-compress, base64-encode, then a second RC4-like `encrypt` pass (different key); `VerifyCaptchaV3` with a pooled device token → `securityToken`; final `{certifyId, isSign, sceneId, securityToken}` base64-encoded. Tokens come FIFO from `tokens.sqlite` and are deleted after use (up to 5 retries). Hard 90 s timeout → `500` on failure.
3. **Signature** — HMAC-SHA256 over `(sortedPayload | promptBase64 | timestamp)` with a salted bucket key from `SALT_KEY` and `timestamp / 300000`.
4. **Streaming** — POST `/api/v2/chat/completions` with `stream:true`; parses SSE (`edit_content` + `edit_index`, `delta_content`, `content`, or OpenAI-style deltas) and re-emits as OpenAI/Anthropic SSE. Semantics mirror the official `prod-fe` bundle: `edit_content` replaces from `edit_index` (a **UTF-16 code-unit offset**, missing = 0 = full replace), `content` = full replace, `delta_content` = append. Deltas are cut on rune boundaries with a `STREAM_HOLDBACK` tail so backtracks never surface as garble (issue #23). Inline errors (HTTP 200 + `data.error`) are surfaced; on `401` the session re-initialises and retries once.
5. **Session GC** — after the response is fully written/failed, the throwaway chat is deleted in the background and (async mode) the pool restocks. Shutdown clears all pooled sessions. See [Session Lifecycle](#session-lifecycle--throwaway-sessions--the-session-pool).

---

## Token Collection (`cmd/token-collector`)

Seeds `tokens.sqlite` with device tokens harvested from `chat.z.ai` via Playwright, with a Bubble Tea TUI (progress bar, live logs, spinner).

```bash
go build -ldflags="-s -w" -trimpath -o token-collector ./cmd/token-collector   # portable
# modern CPUs, fully static: CGO_ENABLED=0 GOAMD64=v3 go build -ldflags="-s -w" -trimpath -o token-collector ./cmd/token-collector
./token-collector                        # interactive TUI prompts
./token-collector --tokens 750 --batch 3 # specific configuration
./token-collector --parallel 3 --headed  # parallel workers, visible browser
```

| Flag | Default | Description |
|---|---|---|
| `--unsafe` | `false` | Raise limits: tokens 1500, batches 25, parallel 5 |
| `--tokens` | `0` | Tokens per batch (0 = prompt; default 850, max 1500) |
| `--batch` | `0` | Batches (0 = prompt; default 5, max 9 / 25 unsafe) |
| `--parallel` | `0` | Parallel pages on one browser (0 = prompt; max 3 / 5 unsafe) |
| `--headed` | `false` | Show browser window |
| `--block-trackers` | `false` | URL allowlist blocking tracker/analytics requests |
| `--no-tui` | `false` | Plain text output |

Implementation: pure-Go SQLite (WAL, 64 MB page cache, 256 MB mmap); one page per worker kept open across batches; parallel batches on a single browser; `GOGC=200`; lock-free atomics for abort/total; TUI ring buffer (1000 lines, scroll ↑/↓ g/G); allowlist permits only `chat.z.ai`, z-cdn assets, Aliyun captcha scripts, and `cloudauth-device`.

---

## Project Structure

Bridge core in `internal/zbridge` (mirrors the DeepseekFreeAPI layout); `main.go` is a thin entry point; blackbox tests in `tests/`, whitebox tests with their package.

```
zai-api/
├── main.go                        # Thin entry point -> zbridge.Run()
├── internal/zbridge/              # Bridge core (package zbridge)
│   ├── run.go                     # Run(), NewHandler(), graceful shutdown
│   ├── config.go                  # Config + env/flag loading
│   ├── types.go                   # Shared types + global state
│   ├── features.go                # Per-model feature resolution
│   ├── util.go                    # Logging, HTTP clients, cookie jar, helpers
│   ├── captcha.go                 # Aliyun captcha machinery + cache
│   ├── zai.go                     # Signature, session init, streaming, SSE parse
│   ├── format.go                  # OpenAI response/error formatting
│   ├── anthropic.go               # Anthropic endpoint + translation
│   ├── middleware.go              # CORS, auth, JSON, misc handlers
│   ├── models.go                  # Live model list + capabilities + architecture
│   ├── vision.go                  # Image input: extract, download, upload, files payload
│   ├── handlers.go                # /v1/chat/completions + admin handlers
│   ├── agent.go                   # Modern agent shim (XML-sectioned prompt)
│   ├── agent_legacy.go            # Legacy [ROLE: ...] shim
│   ├── session_pool.go            # Throwaway sessions + async pool + GC
│   ├── testhooks.go               # Exported seams for tests/
│   ├── agent_test.go              # Whitebox: modern agent shim
│   ├── sse_garble_test.go         # Whitebox: SSE parser (issue #23)
│   └── vision_test.go             # Whitebox: url->file-id rewrite
├── cmd/token-collector/           # Standalone binary: seeds tokens.sqlite (TUI)
├── tests/                         # Blackbox tests (package tests)
│   ├── session_pool_test.go       # Pool mechanics, chat-delete client, GC wiring
│   ├── integration_test.go        # E2E HTTP garble regression (issue #23)
│   └── vision_test.go             # Vision e2e (upload + files), /v1/models, limits
└── README.md
```

Run the suite: `go test ./...`

---

## Notes

- The default auth token (`Waguri`) is a placeholder — set `AUTH_TOKEN` in production.
- Device tokens are **consumed and deleted** after use — re-run the token collector to replenish (each captcha tries up to 5 tokens).
- `--verbose` only controls the captcha subsystem's logging; standard bridge/SSE logging is gated by `LOG_LEVEL=debug`.
- Each request gets a fresh `chat_id` — no server-side conversation persistence; the chat is deleted right after the response completes.

---

## License

Provided under MIT License. Use responsibly and in accordance with Z.AI's terms of service.
