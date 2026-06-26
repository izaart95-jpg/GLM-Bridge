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
function findTokenInEnv() {
  // 1. Process environment
  if (process.env.TOKEN) return process.env.TOKEN.trim();

  // 2. .env file in cwd
  const envPath = path.join(process.cwd(), '.env');
  if (fs.existsSync(envPath)) {
    const raw = fs.readFileSync(envPath, 'utf8');
    for (const line of raw.split('\n')) {
      const match = line.match(/^\s*TOKEN\s*=\s*"?([^"\s]+)"?\s*$/);
      if (match) return match[1].trim();
    }
  }

  return null;
}

// ─── Readline helpers ─────────────────────────────────────────────────────────
function createRL() {
  return readline.createInterface({
    input:  process.stdin,
    output: process.stdout,
    terminal: true,
  });
}

function ask(rl, question) {
  return new Promise(resolve => rl.question(question, resolve));
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
  let token = findTokenInEnv();

  const rl = createRL();

  // Graceful Ctrl+C
  rl.on('SIGINT', () => {
    console.log('\n' + c(A.yellow, '  Bye! ✌') + '\n');
    rl.close();
    process.exit(0);
  });

  if (token) {
    log('success', `Token found in environment ${dim('(TOKEN=...)')}`);
  } else {
    log('warn', 'No TOKEN found in env or .env file.');
    console.log(
      dim('  ') +
      'Go to ' + c(A.blue, 'https://image.z.ai') +
      ', open DevTools → Application → Cookies, copy the ' +
      b('session') + ' cookie value.\n'
    );
    token = (await ask(rl, c(A.cyan, '  Enter your session token: '))).trim();
    if (!token) {
      log('error', 'No token provided. Exiting.');
      rl.close();
      process.exit(1);
    }
  }

  const cookieHeader = `session=${token}`;

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
    const raw = (await ask(rl, c(A.cyan + A.bold, '  › '))).trim();

    if (!raw) continue;
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
    const spinner = makeSpinner('AI is thinking…');

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

    const { status, json } = result;

    // 5. Handle response
    if (status === 401 || (json && json.code === 401)) {
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
    const dl = (await ask(rl, c(A.yellow, '  Download image? (yes/no): '))).trim().toLowerCase();

    if (dl === 'yes' || dl === 'y') {
      const defaultName = `image_${Date.now()}.png`;
      const customName  = (
        await ask(rl, c(A.yellow, `  Save as [${defaultName}]: `))
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

main().catch(err => {
  console.error(c(A.red, '\n  Fatal error: ') + err.message);
  process.exit(1);
});
