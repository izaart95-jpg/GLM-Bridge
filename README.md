# GLM Bridge — Z.AI Proxy API

An OpenAI-compatible API proxy for [chat.z.ai](https://chat.z.ai). Drop it in front of any OpenAI-compatible tool and start using Z.AI's GLM models without browser automation or complex setup.

---

## Features

- **OpenAI-compatible** — Works as a drop-in replacement for `/v1/chat/completions` and `/v1/models`
- **Pure HTTP** — No Playwright, no Selenium, no browser overhead
- **In-process captcha** — Aliyun CaptchaV3 verification handled entirely in-memory
- **Streaming + non-streaming** — Full SSE support with keep-alive ticks
- **Session management** — Per-client conversation threads via `X-Session-Id`, with 30-minute TTL
- **Feature toggles** — Web search, deep thinking, image generation, preview mode, and history persistence
- **Token pool** — Device tokens stored in `tokens.sqlite`, consumed FIFO and removed after use
- **Live dashboard** — Status, features, and curl examples at `/`
- **Pure-Go SQLite** — Uses `modernc.org/sqlite` — no CGO required

---

## Supported Models

| Model ID | Notes |
|---|---|
| `glm-4.7` | Available without `ZAI_TOKEN` |
| `glm-5` | Default |
| `GLM-5-Turbo` | New model for chat, coding, and agentic task |
| `GLM-5v-Turbo` | Vision variant |
| `GLM-5.1` | Previous flagship model |

> **Note:** If `ZAI_TOKEN` is not set, only `glm-4.7` is available.

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
go run init.go
# Recommended: build first for better performance and faster startup:
#   go build -o token-collector -ldflags="-s -w" init.go && ./token-collector

# 4. Start the server
go run main.go
# Recommended: build first for better performance and faster startup:
#   go build -o zai-api -ldflags="-s -w" main.go && ./zai-api
```

On startup, you'll see a banner with your dashboard URL and auth token.

---

## Configuration

### CLI Flags

| Flag | Default | Description |
|---|---|---|
| `--db-path` | `tokens.sqlite` | Path to the SQLite token database |
| `--verbose` | `false` | Enable verbose captcha/debug logging |

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `3001` | HTTP server port |
| `HOST` | `0.0.0.0` | Bind address |
| `AUTH_TOKEN` | `Waguri` | Bearer token for client authentication |
| `TIMEOUT` | `300000` | Request timeout in milliseconds |
| `ZAI_TOKEN` | *(empty)* | Hardcoded Z.AI JWT — skips guest initialization |
| `LOG_LEVEL` | `debug` | Log level (`debug` dumps Z.AI requests/responses) |
| `LOG_FORMAT` | `text` | Log format |

---

## API Reference

### OpenAI-Compatible

| Method | Path | Description |
|---|---|---|
| `POST` | `/v1/chat/completions` | Chat completions (streaming + non-streaming) |
| `GET` | `/v1/models` | List available models |

### Management

| Method | Path | Description |
|---|---|---|
| `POST` | `/features` | Toggle `webSearch`, `thinking`, `imageGen`, etc. |
| `POST` | `/admin/session/clear` | Clear all conversation histories |
| `GET` | `/status` | Live session and feature status (JSON) |
| `GET` | `/admin/health` | Health check (`200` / `503`) |
| `GET` | `/` | HTML dashboard |

---

## Examples

**Basic non-streaming request**

```bash
curl -X POST http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -d '{
    "model": "glm-5",
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
    "model": "glm-5",
    "stream": true,
    "messages": [{"role": "user", "content": "Write a haiku about Go."}]
  }'
```

**Web search + deep thinking**

```bash
curl -X POST http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -d '{
    "model": "GLM-5.1",
    "stream": true,
    "webSearch": true,
    "deepThink": true,
    "messages": [{"role": "user", "content": "Summarize today'\''s top AI news."}]
  }'
```

**Toggle global features**

```bash
curl -X POST http://localhost:3001/features \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer Waguri" \
  -d '{"thinking": true, "webSearch": true, "imageGen": false}'
```

**Python (OpenAI SDK)**

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:3001/v1",
    api_key="Waguri",
)

resp = client.chat.completions.create(
    model="glm-5",
    messages=[{"role": "user", "content": "Hello!"}],
)
print(resp.choices[0].message.content)
```

---

## Session Persistence

Pass `X-Session-Id` to pin a conversation thread across requests. Use `X-Fresh-Session: true` to start a new one. Sessions expire after 30 minutes of inactivity.

```bash
curl -X POST http://localhost:3001/v1/chat/completions \
  -H "Authorization: Bearer Waguri" \
  -H "X-Session-Id: my-thread-1" \
  -H "Content-Type: application/json" \
  -d '{"model":"glm-5","messages":[{"role":"user","content":"My name is Alice."}]}'
```

---

## How It Works

1. **Guest token** — On startup, the server calls Z.AI's `/api/v1/auths/guest` for a session JWT, or uses `ZAI_TOKEN` if provided.
2. **Captcha** — For each request, an Aliyun `captcha_verify_param` is generated in-memory:
   - `InitCaptchaV3` → obtain `certifyId`
   - Generate `arg` via RC4-like permutation cipher
   - Compute `ali_hash`, zlib-compress, base64-encode, then `encrypt`
   - `VerifyCaptchaV3` with a pooled device token → receive `securityToken`
   - Base64-encode the final payload
3. **Signature** — HMAC-SHA256 over `(sortedPayload | promptBase64 | timestamp)` with a salted bucket key.
4. **Streaming** — POST to `/api/v2/chat/completions` with `stream: true`, parse SSE chunks (`edit_content`, `delta_content`, `content`), and forward as OpenAI-formatted SSE.

---

## Project Structure

```
zai-api/
├── main.go          # HTTP server, captcha generation, Z.AI bridge, OpenAI shim
├── init.go          # Seeds tokens.sqlite with device tokens
├── tokens.sqlite    # Generated token pool (consumed at runtime)
├── go.mod
└── README.md
```

---

## Notes

- Device tokens are **consumed and deleted** after use. Re-run `init.go` to replenish the pool.
- The default auth token (`Waguri`) is a placeholder — set `AUTH_TOKEN` in production.
- `ZAI_TOKEN` bypasses guest initialization entirely; without it, only `glm-4.7` is accessible.
- `LOG_LEVEL=debug` dumps every Z.AI request and response — useful for troubleshooting.

---

## License

Provided as-is for educational and interoperability purposes. Use responsibly and in accordance with Z.AI's terms of service.
