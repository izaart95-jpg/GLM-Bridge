# ⚡ z.ai Image Generator TUI

A fully interactive terminal UI for generating AI images via [image.z.ai](https://image.z.ai),
built with **zero external dependencies** — only Node.js built-in modules.

---

## Requirements

| Requirement | Details |
|---|---|
| **Node.js** | v14 or higher (uses `URL`, `async/await`, optional chaining) |
| **OS** | macOS, Linux, Windows (with a terminal that supports ANSI colors) |
| **Network** | Outbound HTTPS access to `image.z.ai` (and to `chat.z.ai` when using `ZAI_TOKEN` auto-auth) |
| **Account** | Either a session cookie from [image.z.ai](https://image.z.ai) (`TOKEN`) **or** a chat.z.ai bearer token (`ZAI_TOKEN`) for automatic OAuth |

---

## Installation

No `npm install` needed. Just download the script and run it.

```bash
# Clone or download
curl -O https://raw.githubusercontent.com/izaart95-jpg/GLM-Free-API/refs/heads/main/image-gen/image-gen.js

# Make it executable (macOS / Linux)
chmod +x image-gen.js
```

---

## Getting Your Session Token

### Recommended — `ZAI_TOKEN` automatic OAuth (no browser DevTools needed)

The recommended way to authenticate — especially for **headless workflows**
(CI, cron, containers, SSH) — is to set **`ZAI_TOKEN`**, your chat.z.ai bearer
token, in the environment or in a `.env` file. The script then automatically
performs the chat.z.ai OAuth flow to obtain the image.z.ai session token:

1. `GET  image.z.ai/api/v1/z-image/auth/` → fetch the authorization URL
2. `POST chat.z.ai/api/oauth/authorize` (with `Bearer <ZAI_TOKEN>`) → approve, receive an authorization code
3. `POST image.z.ai/api/v1/z-image/auth` → exchange the code for the session token

No browser, no DevTools, no manual cookie copying. If the session token expires
while you are using the script, it is refreshed automatically the same way (see
*Method 3* below). When both `TOKEN` and `ZAI_TOKEN` are set, `TOKEN` always wins.

### Manual alternative — copy the `session` cookie

1. Open [https://image.z.ai](https://image.z.ai) in your browser and log in.
2. Open **DevTools** (`F12` or `Cmd+Option+I`).
3. Go to **Application** → **Cookies** → `https://image.z.ai`.
4. Find the cookie named **`session`** and copy its value.

> ⚠️ Tokens expire. If you get a `401 Invalid access token` error, repeat the steps above to get a fresh cookie — or switch to `ZAI_TOKEN`, which refreshes automatically.

---

## Starting the Script

### Method 1 — Inline environment variable

```bash
TOKEN="eyJhbGci..." node image-gen.js
```

### Method 2 — `.env` file (auto-detected on startup)

Create a `.env` file in the same directory as the script:

```dotenv
TOKEN="eyJhbGci..."
```

Then just run:

```bash
node image-gen.js
```

### Method 3 — Automatic OAuth via `ZAI_TOKEN` (headless)

Instead of copying the `session` cookie by hand, provide a **chat.z.ai bearer
token** as `ZAI_TOKEN` (inline or in `.env`). On startup the script runs the
OAuth exchange automatically and derives the image.z.ai session token for you:

```bash
ZAI_TOKEN="eyJhbGci..." node image-gen.js
```

```dotenv
ZAI_TOKEN="eyJhbGci..."
```

Notes:
- Resolution order is **`TOKEN` → `ZAI_TOKEN` → interactive prompt**. A direct
  `TOKEN` always takes precedence when both are present.
- If the session expires mid-session (a `401` during generation), the script
  automatically re-runs the OAuth flow once to refresh the session token and
  retries the request.
- On OAuth failure: in an interactive terminal it falls back to the manual
  token prompt; in headless mode (no TTY) it prints a clear error and exits
  with code `1`.

### Method 4 — Interactive prompt (fallback)

If no token is found in the environment or `.env`, the script will ask for it:

```
  ⚠  No TOKEN found in env or .env file.
  Enter your session token: █
```

Paste the session cookie value and press Enter.

---

## The Chat Prompt

Once the token is accepted, you land at the interactive prompt:

```
  ✔  Ready!  Type a prompt to generate an image.
  Type "exit" or press Ctrl+C to quit.

  Settings → ratio:9:16  res:1K  watermark:kept
  Commands → /ratio 16:9  /resolution high  /watermark true|false

  › █
```

Type any image description (your **prompt**) here, optionally mixed with **command modifiers**.

---

## Command Modifiers

Modifiers are special slash-commands you embed directly inside your prompt. They are parsed
and stripped before the text is sent to the API. They can appear **anywhere** in the input —
beginning, middle, or end.

---

### `/ratio <value>`

Sets the **aspect ratio** of the generated image.

**Syntax:**
```
/ratio <W:H>
```

**Supported values:**

| Value | Orientation | Typical use |
|-------|-------------|-------------|
| `9:16` | Portrait (tall) | Phone wallpaper, social stories — **default** |
| `9:21` | Ultra-portrait | Tall cinematic |
| `21:9` | Ultra-landscape | Wide cinematic banner |
| `16:9` | Landscape | Desktop wallpaper, YouTube thumbnail |
| `3:4` | Portrait | Print, photo |
| `4:3` | Landscape | Classic photo, presentation |
| `1:1` | Square | Instagram, avatar |

**Examples:**
```
› a sunset over the ocean /ratio 16:9
› /ratio 1:1 a minimalist logo for a coffee brand
› dark forest path /ratio 9:21 /resolution high
```

> If you supply an unrecognised value (e.g. `/ratio 5:3`), the script warns you and keeps the
> previously active ratio.

---

### `/resolution <low|high>`

Sets the **output resolution** of the image.

**Syntax:**
```
/resolution low
/resolution high
```

| Argument | API value | Pixel class | Notes |
|----------|-----------|-------------|-------|
| `low` | `1K` | ~1K px on the long edge | Faster, smaller file — **default** |
| `high` | `2K` | ~2K px on the long edge | Slower, larger file |

**Examples:**
```
› a portrait of a samurai /resolution high
› quick concept sketch /resolution low
```

---

### `/watermark <true|false>`

Controls whether the z.ai watermark is **removed** from the generated image.

**Syntax:**
```
/watermark true
/watermark false
```

| Argument | Behaviour | API flag (`rm_label_watermark`) |
|----------|-----------|--------------------------------|
| `true` | Watermark **removed** from image | `false` |
| `false` | Watermark **kept** on image | `true` — **default** |

> Think of `/watermark true` as "yes, remove the watermark."

**Examples:**
```
› product photo on white background /watermark true
› casual doodle /watermark false
```

---

## Combining Multiple Modifiers

All three modifiers can be used in a single input, in any order:

```
› a futuristic city at night, neon lights /ratio 16:9 /resolution high /watermark true
```

```
› /watermark true /resolution high cinematic portrait of an astronaut /ratio 9:16
```

The cleaned prompt sent to the API will be:
```
a futuristic city at night, neon lights
```

---

## Session Persistence

Settings you apply with modifiers **persist for the rest of the session**. You do not need to
repeat them with every prompt.

```
  › /ratio 16:9 /resolution high a red panda in a bamboo forest

  ℹ  Ratio set to 16:9
  ℹ  Resolution set to 2K

  ◌  AI is thinking…

  ✔  Image generated!
  🔗  https://z-ai-audio.oss-cn-hongkong...

  Settings → ratio:16:9  res:2K  watermark:kept    ← carried forward

  › another image prompt here                       ← uses 16:9 + 2K automatically
```

---

## After Generation — Download Flow

When an image is successfully generated, the script shows the URL and asks:

```
  Download image? (yes/no):
```

| Input | Result |
|-------|--------|
| `yes` or `y` | Prompts for a filename, then downloads the image |
| `no` or `n` or Enter | Skips the download, returns to the prompt |

### Saving the file

```
  Save as [image_1717000000000.png]:
```

- Press **Enter** to accept the auto-generated timestamped name (`image_<unix-ms>.png`).
- Or type a custom filename: `my_image.png`, `output/wallpaper.png`, etc.

The file is saved in the **current working directory** (or the path you specified).

---

## Exiting the Script

| Method | Result |
|--------|--------|
| Type `exit` at the prompt | Clean exit |
| Press `Ctrl+C` | Clean exit with goodbye message |

---

## Error Reference

| Error | Cause | Resolution |
|-------|-------|------------|
| `Token expired or invalid` | Session cookie has expired or is wrong | Re-copy the `session` cookie from DevTools (or set `ZAI_TOKEN` to auto-refresh) |
| `OAuth authentication failed: ...` | `ZAI_TOKEN` expired/invalid or the OAuth exchange failed at startup | Refresh your chat.z.ai bearer token; on a TTY the script falls back to the manual prompt |
| `ZAI_TOKEN was rejected by chat.z.ai: ...` | chat.z.ai refused the bearer token (expired/invalid) during OAuth | Get a fresh chat.z.ai token and update `ZAI_TOKEN` |
| `Headless mode: cannot fall back to manual token entry` | OAuth failed and stdin is not a terminal, so no interactive fallback is possible | Fix `ZAI_TOKEN` (or provide `TOKEN`), or run the script in an interactive terminal |
| `OAuth refresh failed: ...` / `Could not refresh session token` / `Token still rejected after OAuth refresh` | The session expired mid-run and the automatic OAuth refresh failed or was still rejected | Check that `ZAI_TOKEN` is still valid and that `chat.z.ai` is reachable, then restart |
| `No prompt text after parsing commands` | Input contained only modifiers with no descriptive text | Add actual image description text |
| `Unknown ratio "X"` | Unsupported ratio value used | Use one of the 7 supported values |
| `Request timed out (120s)` | API took longer than 2 minutes | Retry — the API can be slow under load |
| `Network error: ...` | No internet / DNS failure / proxy block | Check network connection |
| `Download failed: HTTP XXX` | CDN issue with the image URL | The URL is still printed; you can download it manually |

---

## Full Worked Examples

### Basic prompt (all defaults)
```
  › a golden retriever puppy playing in autumn leaves
```
Generates a **9:16**, **1K**, **watermarked** image.

---

### Portrait wallpaper, max quality, no watermark
```
  › ethereal goddess standing in a moonlit forest /ratio 9:16 /resolution high /watermark true
```

---

### Desktop wallpaper
```
  › vast alien landscape with two moons at dusk /ratio 16:9 /resolution high /watermark true
```

---

### Square avatar / icon
```
  › minimal geometric fox logo on black background /ratio 1:1 /resolution high /watermark true
```

---

### Quick concept sketch (fast, low quality)
```
  › rough layout idea for a mobile app home screen /resolution low
```

---

### Changing settings mid-session

```
  › a quiet Japanese garden /ratio 3:4        ← sets ratio for this and all future prompts
  › koi fish in the garden pond               ← still uses 3:4
  › /ratio 1:1 same pond, square crop         ← overrides to 1:1 for this and forward
```

---

## How It Works (Technical Summary)

```
startup
  └─ scan process.env.TOKEN  /  .env TOKEN="..."
  └─ if TOKEN found → use it as the session cookie
  └─ else scan process.env.ZAI_TOKEN  /  .env ZAI_TOKEN="..."
       └─ if ZAI_TOKEN found → run OAuth (no interaction):
            GET  image.z.ai/api/v1/z-image/auth/        → authorization URL
            POST chat.z.ai/api/oauth/authorize          → approve (Bearer ZAI_TOKEN)
            POST image.z.ai/api/v1/z-image/auth         → exchange code → auth_token
            use session=<auth_token>
       └─ on OAuth failure → TTY: manual prompt · headless: exit 1
  └─ else → prompt user interactively

chat loop
  └─ read user input
  └─ parse /ratio, /resolution, /watermark → update session state, strip from text
  └─ POST https://image.z.ai/api/proxy/images/generate
       Cookie: session=<TOKEN>
       { prompt, ratio, resolution, rm_label_watermark }
  └─ spinner while waiting (up to 120s)
  └─ on 200 → print URL → offer download
  └─ on 401 → if ZAI_TOKEN: re-run OAuth once, refresh session, retry request
              else print token help → exit
  └─ on other error → print error → continue loop
```

All networking uses Node.js `https`/`http` built-ins.
All terminal interaction uses Node.js `readline` built-in.
No npm packages. No `node_modules`. No installation step.

---

## License

do whatever you want with it.
