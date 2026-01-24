# Z.AI Proxy API

OpenAI-compatible API proxy for Z.AI (chat.z.ai) with browser automation, client pooling, and Roo/Kilo Code tool support.

## Features

- **OpenAI-Compatible API** - Drop-in replacement for OpenAI API
- **Streaming Support** - Real-time SSE streaming responses
- **Client Pool** - Multiple browser clients with LRU/round-robin/random rotation
- **Tool Call Parsing** - Full support for Roo Code/Kilo Code XML tool format
- **Rate Limit Handling** - Automatic cooldown and recovery
- **Session Management** - Fresh session support with `X-Fresh-Session` header
- **Search & DeepThink** - Toggle Z.AI search and deep thinking features

## Quick Start

### 1. Install Dependencies

```bash
npm install
```

### 2. Start the Server

```bash
npm start
```

Server runs on `http://localhost:3001` by default.

### 3. Connect Browser Client

Open browser console on `https://chat.z.ai` and run:

```javascript
const script = document.createElement('script');
script.src = 'http://localhost:3001/inject.js';
document.head.appendChild(script);
```

### 4. Make API Requests

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ZaiProxy2024" \
  -d '{"model":"z1","messages":[{"role":"user","content":"Hello!"}]}'
```

## API Endpoints

### OpenAI-Compatible

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/models` | GET | List available models |
| `/v1/chat/completions` | POST | Chat completion (streaming/non-streaming) |

### Legacy

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/prompt` | POST | Simple prompt endpoint with search/deepThink options |
| `/models` | GET | List models (legacy format) |
| `/features` | POST | Set search/deepThink features |

### Admin

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/status` | GET | Server and pool status |
| `/admin/clients` | GET | List connected clients |
| `/admin/stats` | GET | Pool statistics |
| `/admin/queue` | GET | Request queue status |
| `/admin/health` | GET | Health check |
| `/admin/clients/:id/clear` | POST | Clear client chat history |
| `/admin/clients/:id` | DELETE | Disconnect client |
| `/stop` | POST | Stop current generation |
| `/inject.js` | GET | Browser injection script |

## Configuration

Environment variables or `config.js`:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3001` | Server port |
| `HOST` | `0.0.0.0` | Server host |
| `AUTH_TOKEN` | `ZaiProxy2024` | API authentication token |
| `ROTATION_STRATEGY` | `lru` | Client rotation: `lru`, `round-robin`, `random` |
| `RATE_LIMIT_COOLDOWN` | `300000` | Rate limit cooldown (ms) |
| `TIMEOUT` | `120000` | Default request timeout (ms) |
| `QUEUE_MAX_SIZE` | `100` | Max queued requests (0=unlimited) |
| `QUEUE_MAX_WAIT` | `60000` | Max queue wait time (ms) |

## Usage Examples

### Basic Chat Completion

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ZaiProxy2024" \
  -d '{
    "model": "z1",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "What is 2+2?"}
    ]
  }'
```

### Streaming Response

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ZaiProxy2024" \
  -d '{
    "model": "z1",
    "stream": true,
    "messages": [{"role": "user", "content": "Write a haiku"}]
  }'
```

### With Search Enabled

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ZaiProxy2024" \
  -d '{
    "model": "z1",
    "search": true,
    "messages": [{"role": "user", "content": "What is the latest news?"}]
  }'
```

### Fresh Session (Clear History)

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ZaiProxy2024" \
  -H "X-Fresh-Session: true" \
  -d '{
    "model": "z1",
    "messages": [{"role": "user", "content": "Start fresh conversation"}]
  }'
```

### Legacy Prompt Endpoint

```bash
curl http://localhost:3001/prompt \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ZaiProxy2024" \
  -d '{
    "prompt": "Hello, how are you?",
    "search": true,
    "deepThink": false
  }'
```

## Tool Call Support

Parses Roo Code/Kilo Code XML tool calls automatically.

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

**Roo/Cline Style**
```xml
<write_to_file>
<path>test.txt</path>
<content>Hello World</content>
</write_to_file>
```

### Supported Tools

- `write_file`, `write_to_file`, `create_file`
- `read_file`, `read_from_file`, `read_multiple_files`
- `edit_file`, `replace_in_file`, `apply_diff`
- `delete_file`, `move_file`, `copy_file`, `rename_file`
- `list_files`, `list_directory`, `find_files`
- `search_files`, `search_code`, `grep_search`
- `execute_command`, `run_command`, `execute_shell`
- `attempt_completion`, `complete_task`, `finish_task`
- `ask_followup_question`, `ask_question`
- `browser_action`, `update_todo_list`
- `switch_mode`, `new_task`, `fetch_instructions`

## Roo Code / Kilo Code Integration

In settings, set:
- **API Base URL**: `http://localhost:3001/v1`
- **API Key**: `ZaiProxy2024`
- **Model**: `z1` or `z1-mini`

## Models

| Model | Description |
|-------|-------------|
| `z1` | Z.AI main model |
| `z1-mini` | Z.AI mini model |

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  API Client │────>│  Z.AI Proxy  │<───>│  Browser    │
│             │     │   Server     │ WS  │  (chat.z.ai)│
└─────────────┘     └──────────────┘     └─────────────┘
```

