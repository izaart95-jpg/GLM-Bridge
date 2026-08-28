#!/usr/bin/env node

/**
 * z.ai Image Generation TUI
 * Node.js built-in modules only — no external dependencies.
 */

'use strict';

const readline = require('readline');
const fs       = require('fs');
const path     = require('path');
const https    = require('https');
const http     = require('http');

// ─── ANSI helpers ────────────────────────────────────────────────────────────
const A = {
  reset:   '\x1b[0m',
  bold:    '\x1b[1m',
  dim:     '\x1b[2m',
  cyan:    '\x1b[36m',
  green:   '\x1b[32m',
  yellow:  '\x1b[33m',
  red:     '\x1b[31m',
  magenta: '\x1b[35m',
  blue:    '\x1b[34m',
  white:   '\x1b[97m',
  bgBlue:  '\x1b[44m',
  bgBlack: '\x1b[40m',
};

const c  = (color, str)  => `${color}${str}${A.reset}`;
const b  = (str)         => c(A.bold,    str);
const dim= (str)         => c(A.dim,     str);

// ─── Banner ───────────────────────────────────────────────────────────────────
function printBanner() {
  const line = '─'.repeat(52);
  console.log('');
  console.log(c(A.cyan, `  ╔${line}╗`));
  console.log(c(A.cyan, `  ║`) + c(A.bold + A.white, '      ⚡  z.ai  Image  Generator  TUI  ⚡      ') + c(A.cyan, '║'));
  console.log(c(A.cyan, `  ╚${line}╝`));
  console.log('');
}

// ─── ENV / .env token lookup ──────────────────────────────────────────────────
function findEnvValue(name) {
  // 1. Process environment
  if (process.env[name]) return process.env[name].trim();

  // 2. .env file in cwd
  const envPath = path.join(process.cwd(), '.env');
  if (fs.existsSync(envPath)) {
    const raw = fs.readFileSync(envPath, 'utf8');
    const re = new RegExp(`^\\s*${name}\\s*=\\s*"?([^"\\s]+)"?\\s*$`);
    for (const line of raw.split('\n')) {
      const match = line.match(re);
      if (match) return match[1].trim();
    }
  }

  return null;
}

// Direct image.z.ai session cookie (used as-is when present).
function findTokenInEnv() {
  return findEnvValue('TOKEN');
}

// chat.z.ai bearer token → used for automatic OAuth session bootstrap.
function findZaiTokenInEnv() {
  return findEnvValue('ZAI_TOKEN');
}

// ─── Readline helpers ─────────────────────────────────────────────────────────
function createRL() {
  return readline.createInterface({
    input:  process.stdin,
    output: process.stdout,
    terminal: true,
  });
}

// Input manager wrapping readline.
//
// Plain rl.question() breaks in headless/piped use: lines that arrive while no
// question is pending (during the startup OAuth exchange or an API call) are
// dropped, and stdin EOF closes the interface so the next question throws
// "readline was closed". Here stray lines are queued for the next question and
// EOF simply answers pending questions with '' instead of crashing.
function createInput() {
  const rl = createRL();
  const queue = [];
  let pendingResolve = null;
  let closed = false;

  // Registered before any rl.question() listener, so it sees every line
  // first; when a question is pending, rl.question() consumes the line itself.
  rl.on('line', line => {
    if (!pendingResolve) queue.push(line);
  });
  rl.on('close', () => {
    closed = true;
    if (pendingResolve) {
      const resolve = pendingResolve;
      pendingResolve = null;
      resolve('');
    }
  });

  return {
    rl,
    closed: () => closed,
    ask(question) {
      if (queue.length) return Promise.resolve(queue.shift());
      if (closed) return Promise.resolve('');
      return new Promise(resolve => {
        pendingResolve = resolve;
        rl.question(question, answer => { pendingResolve = null; resolve(answer); });
      });
    },
  };
}

// ─── Command parser ───────────────────────────────────────────────────────────
const VALID_RATIOS = new Set(['9:16','9:21','21:9','16:9','3:4','1:1','4:3']);

function parseInput(raw, state) {
  let text = raw;

  // /ratio X:Y
  text = text.replace(/\/ratio\s+(\S+)/gi, (_, val) => {
    if (VALID_RATIOS.has(val)) {
      state.ratio = val;
      log('info', `Ratio set to ${b(val)}`);
    } else {
      log('warn', `Unknown ratio "${val}", keeping ${b(state.ratio)}`);
    }
    return '';
  });

  // /resolution low|high
  text = text.replace(/\/resolution\s+(low|high)/gi, (_, val) => {
    state.resolution = val.toLowerCase() === 'high' ? '2K' : '1K';
    log('info', `Resolution set to ${b(state.resolution)}`);
    return '';
  });

  // /watermark true|false
  text = text.replace(/\/watermark\s+(true|false)/gi, (_, val) => {
    // /watermark true  → user wants watermark REMOVED → rm_label_watermark = false
    // /watermark false → user wants watermark KEPT   → rm_label_watermark = true
    state.rm_label_watermark = val.toLowerCase() !== 'true';
    log('info', `Watermark ${val.toLowerCase() === 'true' ? c(A.green,'removed') : c(A.yellow,'kept')}`);
    return '';
  });

  return text.replace(/\s+/g, ' ').trim();
}

// ─── Logging ──────────────────────────────────────────────────────────────────
function log(type, msg) {
  const prefix = {
    info:    c(A.cyan,    '  ℹ'),
    warn:    c(A.yellow,  '  ⚠'),
    error:   c(A.red,     '  ✖'),
    success: c(A.green,   '  ✔'),
    think:   c(A.magenta, '  ◌'),
    url:     c(A.blue,    '  🔗'),
  }[type] || '  ·';
  console.log(`${prefix}  ${msg}`);
}

// ─── Spinner (pure ANSI, no deps) ─────────────────────────────────────────────
function makeSpinner(msg) {
  const frames = ['⣾','⣽','⣻','⢿','⡿','⣟','⣯','⣷'];
  let i = 0;
  const interval = setInterval(() => {
    process.stdout.write(`\r${c(A.magenta, frames[i++ % frames.length])}  ${dim(msg)}   `);
  }, 80);
  return {
    stop(clearLine = true) {
      clearInterval(interval);
      if (clearLine) process.stdout.write('\r\x1b[2K');
    }
  };
}

// ─── HTTPS request helper (returns parsed JSON) ───────────────────────────────
function postJSON(url, body, cookieHeader) {
  return new Promise((resolve, reject) => {
    const payload = JSON.stringify(body);
    const parsed  = new URL(url);
    const opts = {
      hostname: parsed.hostname,
      path:     parsed.pathname + parsed.search,
      method:   'POST',
      headers: {
        'Content-Type':   'application/json',
        'Content-Length': Buffer.byteLength(payload),
        'Cookie':         cookieHeader,
        'User-Agent':     'zai-tui/1.0',
      },
      timeout: 120_000,   // 2 min — API can be slow
    };

    const lib = parsed.protocol === 'https:' ? https : http;
    const req = lib.request(opts, res => {
      let data = '';
      res.on('data', chunk => (data += chunk));
      res.on('end', () => {
        try {
          resolve({ status: res.statusCode, json: JSON.parse(data) });
        } catch (e) {
          reject(new Error(`Non-JSON response (${res.statusCode}): ${data.slice(0,200)}`));
        }
      });
    });

    req.on('timeout', () => { req.destroy(); reject(new Error('Request timed out (120s)')); });
    req.on('error',   reject);
    req.write(payload);
    req.end();
  });
}

// ─── Generic HTTP request helper (GET / POST, raw text response) ──────────────
function httpRequest(url, { method = 'GET', headers = {}, body = null } = {}) {
  return new Promise((resolve, reject) => {
    const parsed = new URL(url);
    const opts = {
      hostname: parsed.hostname,
      path:     parsed.pathname + parsed.search,
      method,
      headers: { 'User-Agent': 'zai-tui/1.0', ...headers },
      timeout: 30_000,
    };
    if (body != null) opts.headers['Content-Length'] = Buffer.byteLength(body);

    const lib = parsed.protocol === 'https:' ? https : http;
    const req = lib.request(opts, res => {
      let data = '';
      res.on('data', chunk => (data += chunk));
      res.on('end', () => resolve({ status: res.statusCode, text: data }));
    });

    req.on('timeout', () => { req.destroy(); reject(new Error('Request timed out (30s)')); });
    req.on('error',   reject);
    if (body != null) req.write(body);
    req.end();
  });
}

// True when an API response means "unauthorized" (HTTP 401 or in-band code 401).
const isUnauthorized = result =>
  result.status === 401 || (result.json && result.json.code === 401);

// ─── Automatic OAuth: chat.z.ai bearer → image.z.ai session token ─────────────
// Port of the verified reference flow: fetch the authorization URL from
// image.z.ai, approve it on chat.z.ai using the ZAI_TOKEN bearer, then exchange
// the returned authorization code for an image.z.ai session token.
// Returns { token, name, userId }; throws Error with a clear message on failure.
async function authenticateViaOAuth(zaiToken) {
  // Step 1: get the authorization URL
  const authUrlRes = await httpRequest('https://image.z.ai/api/v1/z-image/auth/', {
    headers: { 'Content-Type': 'application/json' },
  });
  if (authUrlRes.status !== 200) {
    throw new Error(`Could not get authorization URL (HTTP ${authUrlRes.status})`);
  }

  let authUrl = null;
  try {
    const parsed = JSON.parse(authUrlRes.text);
    authUrl = (parsed && (parsed.url || (parsed.data && parsed.data.url))) || null;
  } catch (e) { /* fall back to regex extraction below */ }
  if (!authUrl) {
    const match = authUrlRes.text.match(/"url":"([^"]*)"/);
    if (match) authUrl = match[1].replace(/\\u0026/g, '&');
  }
  if (!authUrl) {
    throw new Error(`No authorization URL in response: ${authUrlRes.text.slice(0, 200)}`);
  }

  // Step 2: extract client_id / redirect_uri / state.
  // client_id and state are kept raw (still percent-encoded) and only
  // redirect_uri is decoded — matching the verified reference implementation.
  const pick = param => {
    const m = authUrl.match(new RegExp(`${param}=([^&]*)`));
    return m ? m[1] : null;
  };
  const clientId    = pick('client_id');
  const state       = pick('state');
  let   redirectUri = pick('redirect_uri');
  if (redirectUri) {
    try { redirectUri = decodeURIComponent(redirectUri); } catch (e) { /* keep raw */ }
  }
  if (!clientId || !state || !redirectUri) {
    throw new Error(`Authorization URL is missing required parameters: ${authUrl}`);
  }

  // Step 3: approve the authorization on chat.z.ai
  const formBody = new URLSearchParams({
    client_id:     clientId,
    redirect_uri:  redirectUri,
    state:         state,
    response_type: 'code',
    action:        'approve',
  }).toString();
  const approveRes = await httpRequest('https://chat.z.ai/api/oauth/authorize', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${zaiToken}`,
      'Content-Type':  'application/x-www-form-urlencoded',
    },
    body: formBody,
  });
  if (approveRes.status === 401 || approveRes.status === 403) {
    throw new Error(`ZAI_TOKEN was rejected by chat.z.ai (HTTP ${approveRes.status}) — it may be expired or invalid`);
  }
  if (approveRes.status !== 200) {
    throw new Error(`Authorization failed (HTTP ${approveRes.status}): ${approveRes.text.slice(0, 200)}`);
  }

  // Step 4: extract the authorization code from the redirect URL
  let approveJson = null;
  try { approveJson = JSON.parse(approveRes.text); } catch (e) { /* handled below */ }
  const redirectUrl = (approveJson && approveJson.redirect_url) || null;
  if (!redirectUrl) {
    // chat.z.ai answers HTTP 200 with an in-band error when the bearer is rejected
    if (approveJson && (approveJson.error || approveJson.message)) {
      throw new Error(
        `ZAI_TOKEN was rejected by chat.z.ai: ` +
        `${approveJson.error || 'error'}${approveJson.message ? ` — ${approveJson.message}` : ''} ` +
        `(token may be expired or invalid)`
      );
    }
    throw new Error(`No redirect_url in authorization response: ${approveRes.text.slice(0, 200)}`);
  }
  let code = null;
  try {
    code = new URL(redirectUrl).searchParams.get('code');
  } catch (e) { /* fall back to regex below */ }
  if (!code) {
    const m = redirectUrl.match(/code=([^&]+)/);
    if (m) code = m[1];
  }
  if (!code) {
    throw new Error(`No authorization code in redirect URL: ${redirectUrl}`);
  }

  // Step 5: exchange the code for an image.z.ai session token
  const tokenRes = await httpRequest('https://image.z.ai/api/v1/z-image/auth', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code }),
  });
  if (tokenRes.status !== 200) {
    throw new Error(`Token exchange failed (HTTP ${tokenRes.status}): ${tokenRes.text.slice(0, 200)}`);
  }
  let tokenJson = null;
  try { tokenJson = JSON.parse(tokenRes.text); } catch (e) { /* handled below */ }
  const authToken = tokenJson && tokenJson.data && tokenJson.data.auth_token;
  if (!authToken || (tokenJson.code !== undefined && tokenJson.code !== 200)) {
    throw new Error(`Token exchange unsuccessful: ${tokenRes.text.slice(0, 200)}`);
  }
  return {
    token:  authToken,
    name:   (tokenJson.data && tokenJson.data.name)    || null,
    userId: (tokenJson.data && tokenJson.data.user_id) || null,
  };
}

// ─── Image downloader ─────────────────────────────────────────────────────────
function downloadImage(imageUrl, destPath) {
  return new Promise((resolve, reject) => {
    const follow = (url, redirects = 0) => {
      if (redirects > 5) return reject(new Error('Too many redirects'));
      const parsed = new URL(url);
      const lib    = parsed.protocol === 'https:' ? https : http;
      lib.get(url, res => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return follow(res.headers.location, redirects + 1);
        }
        if (res.statusCode !== 200) {
          return reject(new Error(`Download failed: HTTP ${res.statusCode}`));
        }
        const file = fs.createWriteStream(destPath);
        res.pipe(file);
        file.on('finish', () => file.close(resolve));
        file.on('error',  err => { fs.unlink(destPath, () => {}); reject(err); });
      }).on('error', reject);
    };
    follow(imageUrl);
  });
}

// ─── Status bar (current session defaults) ───────────────────────────────────
function printStatus(state) {
  console.log('');
  console.log(
    dim('  Settings → ') +
    c(A.cyan,   `ratio:${b(state.ratio)}`) + dim('  ') +
    c(A.cyan,   `res:${b(state.resolution)}`) + dim('  ') +
    c(A.cyan,   `watermark:${b(state.rm_label_watermark ? 'kept' : 'removed')}`)
  );
  console.log(
    dim('  Commands → ') +
    dim('/ratio 16:9  /resolution high  /watermark true|false')
  );
  console.log('');
}

// ─── Main ─────────────────────────────────────────────────────────────────────
async function main() {
  printBanner();

  // 1. Token acquisition
  //    Resolution order:
  //      a) TOKEN      — direct image.z.ai session cookie, used as-is (as today)
  //      b) ZAI_TOKEN  — chat.z.ai bearer → automatic OAuth for a session token
  //      c) otherwise  — interactive manual prompt (as today)
  let token = findTokenInEnv();
  const zaiToken = findZaiTokenInEnv();

  const input = createInput();
  const rl = input.rl;

  // Graceful Ctrl+C
  rl.on('SIGINT', () => {
    console.log('\n' + c(A.yellow, '  Bye! ✌') + '\n');
    rl.close();
    process.exit(0);
  });

  if (token) {
    log('success', `Token found in environment ${dim('(TOKEN=...)')}`);
  } else {
    if (zaiToken) {
      // Automatic OAuth bootstrap — headless-safe, no interaction required.
      log('info', 'Authenticating via chat.z.ai OAuth…');
      try {
        const auth = await authenticateViaOAuth(zaiToken);
        token = auth.token;
        log('success', `OAuth authentication successful${auth.name ? ` ${dim(`(user: ${auth.name})`)}` : ''}`);
      } catch (err) {
        log('error', `OAuth authentication failed: ${err.message}`);
        if (!process.stdin.isTTY) {
          log('error', 'Headless mode: cannot fall back to manual token entry. Exiting.');
          rl.close();
          process.exit(1);
        }
        log('warn', 'Falling back to manual session token entry.');
      }
    }

    if (!token) {
      if (!process.stdin.isTTY) {
        log('error', 'No token available (set TOKEN or ZAI_TOKEN) and stdin is not a terminal — cannot prompt. Exiting.');
        rl.close();
        process.exit(1);
      }
      log('warn', 'No TOKEN found in env or .env file.');
      console.log(
        dim('  ') +
        'Go to ' + c(A.blue, 'https://image.z.ai') +
        ', open DevTools → Application → Cookies, copy the ' +
        b('session') + ' cookie value.\n'
      );
      token = (await input.ask(c(A.cyan, '  Enter your session token: '))).trim();
      if (!token) {
        log('error', 'No token provided. Exiting.');
        rl.close();
        process.exit(1);
      }
    }
  }

  let cookieHeader = `session=${token}`;

  // 2. Session state (mutable, per-prompt overrideable)
  const state = {
    ratio:              '9:16',
    resolution:         '1K',
    rm_label_watermark: true,   // true = watermark kept (default)
  };

  console.log('');
  console.log(c(A.green, '  ✔  Ready!') + '  Type a prompt to generate an image.');
  console.log(dim('  Type "exit" or press Ctrl+C to quit.\n'));
  printStatus(state);

  // 3. Chat loop
  // eslint-disable-next-line no-constant-condition
  while (true) {
    const raw = (await input.ask(c(A.cyan + A.bold, '  › '))).trim();

    if (!raw) {
      // stdin exhausted (piped/headless input fully consumed) → clean exit.
      if (input.closed()) {
        console.log('\n' + dim('  Input closed — exiting.') + '\n');
        rl.close();
        process.exit(0);
      }
      continue;
    }
    if (raw.toLowerCase() === 'exit') {
      console.log('\n' + c(A.yellow, '  Bye! ✌') + '\n');
      rl.close();
      process.exit(0);
    }

    // Parse commands & clean prompt
    const prompt = parseInput(raw, state);

    if (!prompt) {
      log('warn', 'No prompt text after parsing commands. Try again.');
      continue;
    }

    // 4. Call API
    const reqBody = {
      prompt,
      ratio:              state.ratio,
      resolution:         state.resolution,
      rm_label_watermark: state.rm_label_watermark,
    };

    console.log('');
    let spinner = makeSpinner('AI is thinking…');

    let result;
    try {
      result = await postJSON(
        'https://image.z.ai/api/proxy/images/generate',
        reqBody,
        cookieHeader
      );
    } catch (err) {
      spinner.stop();
      log('error', `Network error: ${err.message}`);
      continue;
    }

    spinner.stop();

    // 5. Handle response
    if (isUnauthorized(result)) {
      if (!zaiToken) {
        // No ZAI_TOKEN → identical behavior to before: explain and exit.
        log('error', b('Token expired or invalid.'));
        console.log(
          dim('\n  ') +
          'Go to ' + c(A.blue, 'https://image.z.ai') +
          ', open DevTools → Application → Cookies,\n' +
          dim('  ') + 'copy the ' + b('session') + ' cookie value and restart the script.'
        );
        console.log('');
        rl.close();
        process.exit(1);
      }

      // ZAI_TOKEN available → refresh the session token via OAuth, retry once.
      log('warn', 'Session token expired — refreshing via chat.z.ai OAuth…');
      let refreshed = null;
      try {
        refreshed = await authenticateViaOAuth(zaiToken);
      } catch (err) {
        log('error', `OAuth refresh failed: ${err.message}`);
      }
      if (!refreshed) {
        log('error', b('Could not refresh session token. Exiting.'));
        console.log('');
        rl.close();
        process.exit(1);
      }
      token = refreshed.token;
      cookieHeader = `session=${token}`;
      log('success', `Session refreshed${refreshed.name ? ` ${dim(`(user: ${refreshed.name})`)}` : ''} — retrying request…`);

      console.log('');
      spinner = makeSpinner('AI is thinking…');
      try {
        result = await postJSON(
          'https://image.z.ai/api/proxy/images/generate',
          reqBody,
          cookieHeader
        );
      } catch (err) {
        spinner.stop();
        log('error', `Network error: ${err.message}`);
        continue;
      }
      spinner.stop();

      if (isUnauthorized(result)) {
        log('error', b('Token still rejected after OAuth refresh. Exiting.'));
        console.log('');
        rl.close();
        process.exit(1);
      }
    }

    const { status, json } = result;

    if (status !== 200 || !json || json.code !== 200) {
      const msg = (json && json.message) || `HTTP ${status}`;
      log('error', `API error: ${msg}`);
      continue;
    }

    // Success
    const imageUrl = json?.data?.image?.image_url;
    if (!imageUrl) {
      log('error', 'API returned success but no image_url found.');
      console.log(dim('  Raw: ') + JSON.stringify(json).slice(0, 300));
      continue;
    }

    log('success', 'Image generated!');
    log('url', c(A.blue, imageUrl));

    const imgMeta = json.data.image;
    if (imgMeta) {
      console.log(
        dim(`  Size: ${imgMeta.size || '?'}  |  `) +
        dim(`Ratio: ${imgMeta.ratio || '?'}  |  `) +
        dim(`Res: ${imgMeta.resolution || '?'}`)
      );
    }
    console.log('');

    // 6. Download?
    const dl = (await input.ask(c(A.yellow, '  Download image? (yes/no): '))).trim().toLowerCase();

    if (dl === 'yes' || dl === 'y') {
      const defaultName = `image_${Date.now()}.png`;
      const customName  = (
        await input.ask(c(A.yellow, `  Save as [${defaultName}]: `))
      ).trim();
      const filename = customName || defaultName;
      const destPath = path.resolve(process.cwd(), filename);

      const dlSpinner = makeSpinner(`Downloading → ${filename}`);
      try {
        await downloadImage(imageUrl, destPath);
        dlSpinner.stop();
        log('success', `Saved to ${b(destPath)}`);
      } catch (err) {
        dlSpinner.stop();
        log('error', `Download failed: ${err.message}`);
      }
    } else {
      log('info', 'Skipped download.');
    }

    console.log('');
    printStatus(state);
  }
}

// ─── Entrypoint ───────────────────────────────────────────────────────────────
if (require.main === module) {
  main().catch(err => {
    console.error(c(A.red, '\n  Fatal error: ') + err.message);
    process.exit(1);
  });
}

// Exported for non-interactive testing (requiring this module does not start the TUI).
module.exports = { findTokenInEnv, findZaiTokenInEnv, authenticateViaOAuth, httpRequest };
