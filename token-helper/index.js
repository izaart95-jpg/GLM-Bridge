import { chromium } from 'playwright';
import fs from 'fs';
import path from 'path';
import readline from 'readline/promises';
import { stdin as input, stdout as output } from 'process';
import initSqlJs from 'sql.js';

// ---------- Configuration ----------
const MAX_TOKENS = 1250;
const DEFAULT_TOKENS = 750;
const SEND_WAIT_MS = 7000;       // wait after clicking send
const MAX_RETRIES = 3;
const TOKEN_COLLECTION_TIMEOUT_MS = 90000; // 90 seconds
const URL = 'https://chat.z.ai';

// ---------- Prompt user for token count ----------
async function promptTokenCount() {
  const rl = readline.createInterface({ input, output });
  try {
    const answer = await rl.question(
      `How many tokens to collect? [default: ${DEFAULT_TOKENS}, max: ${MAX_TOKENS}] `
    );
    const trimmed = answer.trim();
    let n = trimmed === '' ? DEFAULT_TOKENS : parseInt(trimmed, 10);
    if (!Number.isFinite(n) || n <= 0) n = DEFAULT_TOKENS;
    if (n > MAX_TOKENS) {
      console.log(`⚠️  Capping to max ${MAX_TOKENS}.`);
      n = MAX_TOKENS;
    }
    return n;
  } finally {
    rl.close();
  }
}

// ---------- Fast sleep ----------
const sleep = (ms) => new Promise(r => setTimeout(r, ms));

(async () => {
  let browser;
  try {
    const total = await promptTokenCount();
    console.log(`\n🎯 Collecting ${total} tokens`);

    // Pre-fetch WASM binary directly to avoid Node.js fs/path issues
    const sqlPromise = (async () => {
      console.log('⏳ Fetching sql-wasm binary...');
      const wasmUrl = 'https://cdnjs.cloudflare.com/ajax/libs/sql.js/1.10.3/sql-wasm.wasm';
      const response = await fetch(wasmUrl);
      if (!response.ok) throw new Error(`Failed to fetch wasm: ${response.statusText}`);
      const wasmBinary = await response.arrayBuffer();
      return initSqlJs({ wasmBinary });
    })();

    browser = await chromium.launch({ headless: true });
    const page = await browser.newPage();

    let success = false;

    for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
      console.log(`\n🔄 Attempt ${attempt} of ${MAX_RETRIES}`);

      try {
        // Use 'domcontentloaded' + element waits instead of slow 'networkidle'
        await page.goto(URL, { waitUntil: 'domcontentloaded', timeout: 60000 });

        // Use Promise.all to wait for both elements concurrently
        console.log('  Locating UI elements in parallel...');
        const [modelButton, textarea] = await Promise.all([
          page.waitForSelector('#model-selector-glm-4_7-button', { timeout: 15000 }),
          page.waitForSelector('#chat-input', { timeout: 15000 }),
        ]);
        console.log('✅ Model button & textarea found');

        await textarea.fill('__');
        console.log('✅ Textarea filled with "__"');

        const sendButton = await page.waitForSelector('#send-message-button', { timeout: 5000 });
        await sendButton.click();
        console.log('✅ Send clicked');

        console.log(`⏳ Waiting ${SEND_WAIT_MS}ms for token endpoint to initialize...`);
        await sleep(SEND_WAIT_MS);

        // ---------- Fast token collection with timeout ----------
        console.log('🚀 Collecting tokens...');
        const t0 = Date.now();

        // Create a timeout promise that rejects after TOKEN_COLLECTION_TIMEOUT_MS
        const timeoutPromise = new Promise((_, reject) => {
          setTimeout(() => {
            reject(new Error(`⏱️ Token collection timed out after ${TOKEN_COLLECTION_TIMEOUT_MS / 1000}s`));
          }, TOKEN_COLLECTION_TIMEOUT_MS);
        });

        const tokens = await Promise.race([
          page.evaluate(async ({ total }) => {
            const out = new Array(total);
            for (let i = 0; i < total; i++) {
              const tok = window.z_um.getToken();
              out[i] = (tok && typeof tok.then === 'function') ? await tok : tok;
              
              // Yield to the browser event loop every 50 tokens
              if (i % 50 === 0) {
                await new Promise(r => setTimeout(r, 0));
              }
            }
            return out;
          }, { total }),
          timeoutPromise,
        ]);

        const elapsed = ((Date.now() - t0) / 1000).toFixed(2);
        console.log(`✅ Collected ${tokens.length} tokens in ${elapsed}s`);

        // ---------- Build SQLite in Node ----------
        console.log('🗄️  Building SQLite database in Node...');
        const SQL = await sqlPromise;
        const db = new SQL.Database();
        db.run('CREATE TABLE tokens (id INTEGER PRIMARY KEY, token TEXT);');
        db.run('BEGIN TRANSACTION;');
        const stmt = db.prepare('INSERT INTO tokens (id, token) VALUES (?, ?);');
        for (let i = 0; i < tokens.length; i++) {
          stmt.run([i, tokens[i]]);
        }
        stmt.free();
        db.run('COMMIT;');

        const data = db.export();
        db.close();

        const filePath = path.join(process.cwd(), 'tokens.sqlite');
        fs.writeFileSync(filePath, Buffer.from(data));
        console.log(`✅ Saved: ${filePath} (${(data.length / 1024).toFixed(1)} KB)`);

        success = true;
        break;
      } catch (error) {
        console.log(`❌ Attempt ${attempt} failed: ${error.message}`);
        if (attempt === MAX_RETRIES) {
          console.error('🚫 All retries exhausted.');
          throw error;
        }
        console.log('♻️  Retrying with a fresh page load...');
      }
    }

    if (!success) throw new Error('Failed after maximum retries.');

    await browser.close();
    console.log('\n🎉 Script finished successfully.');
    // Force exit to prevent lingering WASM/Playwright background processes
    process.exit(0);
    
  } catch (error) {
    console.error(`\n🚫 Fatal error: ${error.message}`);
    if (browser) {
      try {
        await browser.close();
      } catch (e) {}
    }
    // Exit with error code
    process.exit(1);
  }
})();
