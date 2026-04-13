const crypto = require('crypto');
const https = require('https');
const readline = require('readline');

const BASE_HOST = 'chat.z.ai';
const SALT_KEY = 'key-@@@@)))()((9))-xxxx&&&%%%%%';
const MODEL = 'GLM-5-Turbo';

// ─── helpers ────────────────────────────────────────────────────────────────

function request(options, body) {
  return new Promise((resolve, reject) => {
    const req = https.request(options, (res) => {
      let data = '';
      res.on('data', (c) => (data += c));
      res.on('end', () => {
        try { resolve({ status: res.statusCode, headers: res.headers, body: JSON.parse(data) }); }
        catch { resolve({ status: res.statusCode, headers: res.headers, body: data }); }
      });
    });
    req.on('error', reject);
    if (body) req.write(body);
    req.end();
  });
}

function generateSignature(prompt, userId) {
  const timestamp = String(Date.now());
  const requestId = crypto.randomUUID();
  const bucket = Math.floor(Number(timestamp) / 300000);

  const wKey = crypto.createHmac('sha256', SALT_KEY).update(String(bucket)).digest('hex');
  const payload = { requestId, timestamp, user_id: userId };
  const sorted = Object.entries(payload)
    .sort((a, b) => a[0].localeCompare(b[0]))
    .map(([k, v]) => `${k},${v}`)
    .join(',');

  const promptB64 = Buffer.from(prompt.trim()).toString('base64');
  const dataToSign = `${sorted}|${promptB64}|${timestamp}`;
  const signature = crypto.createHmac('sha256', wKey).update(dataToSign).digest('hex');

  return { signature, timestamp, requestId };
}

function decodeJWT(token) {
  try {
    const payload = token.split('.')[1];
    const padded = payload + '='.repeat((4 - (payload.length % 4)) % 4);
    return JSON.parse(Buffer.from(padded, 'base64').toString('utf8'));
  } catch {
    return null;
  }
}

// ─── API calls ──────────────────────────────────────────────────────────────

async function workspaceUp(token, chatId) {
  const bodyStr = JSON.stringify({ chatId, flags: ['general_agent'] });

  const options = {
    hostname: BASE_HOST,
    path: '/api/v1/web-dev/workspaces/up',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'accept': 'text/event-stream',
      authorization: `Bearer ${token}`,
      'Content-Length': Buffer.byteLength(bodyStr),
    },
  };

  console.log('🔧 Initializing workspace...');

  return new Promise((resolve, reject) => {
    const req = https.request(options, (res) => {
      if (res.statusCode !== 200) {
        let errBody = '';
        res.on('data', (c) => (errBody += c));
        res.on('end', () => reject(new Error(`Workspace init failed (${res.statusCode}): ${errBody}`)));
        return;
      }

      let buffer = '';

      res.on('data', (chunk) => {
        buffer += chunk.toString();
        const lines = buffer.split('\n');
        buffer = lines.pop();

        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed.startsWith('data: ')) continue;
          const data = trimmed.slice(6).trim();
          if (!data) continue;

          try {
            const json = JSON.parse(data);
            if (json.status === 'checking') {
              console.log(`   ⏳ ${json.message}`);
            } else if (json.status === 'limit_check_passed') {
              console.log(`   ✅ ${json.message}`);
            } else if (json.status === 'initializing') {
              console.log(`   🔨 ${json.message}`);
            } else if (json.status === 'success') {
              console.log(`   🚀 ${json.message}`);
              resolve(json.data);
            } else if (json.status === 'error') {
              reject(new Error(`Workspace error: ${json.message}`));
            }
          } catch {}
        }
      });

      res.on('end', () => {
        // If stream ended without success, reject
        reject(new Error('Workspace stream ended without success'));
      });
    });

    req.on('error', reject);
    req.write(bodyStr);
    req.end();
  });
}

async function createNewChat(token, firstMessage) {
  const userId = decodeJWT(token)?.id || '';
  const msgId = crypto.randomUUID();
  const ts = Math.floor(Date.now() / 1000);
  const tsMs = Date.now();

  const chatBody = {
    chat: {
      id: '',
      title: 'New Chat',
      models: [MODEL],
      params: {},
      history: {
        messages: {
          [msgId]: {
            id: msgId,
            parentId: null,
            childrenIds: [],
            role: 'user',
            content: firstMessage,
            timestamp: ts,
            models: [MODEL],
          },
        },
        currentId: msgId,
      },
      tags: [],
      flags: ['general_agent'],
      features: [
        { type: 'web_search', server: 'web_search_h', status: 'hidden' },
        { type: 'tool_selector', server: 'tool_selector_h', status: 'hidden' },
        { type: 'hidden-thinking', server: 'hidden-thinking', status: 'hidden' },
      ],
      mcp_servers: [],
      enable_thinking: true,
      auto_web_search: false,
      message_version: 1,
      extra: {},
      timestamp: tsMs,
      type: 'general_agent',
    },
  };

  const bodyStr = JSON.stringify(chatBody);

  const options = {
    hostname: BASE_HOST,
    path: '/api/v1/chats/new',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      authorization: `Bearer ${token}`,
      'Content-Length': Buffer.byteLength(bodyStr),
    },
  };

  console.log('🔍 Creating new chat session...');
  const res = await request(options, bodyStr);

  if (res.status !== 200) {
    throw new Error(`Failed to create chat (${res.status}): ${JSON.stringify(res.body)}`);
  }

  const chat = res.body;
  const chatId = chat.id || chat.chat?.id;
  const userMessageId = Object.keys(chat.chat?.history?.messages || {})[0];
  console.log(`✅ Chat created! ID: ${chatId}`);
  console.log(`   Title: ${chat.title || 'New Chat'}`);
  console.log(`   User Message ID: ${userMessageId}`);
  console.log();

  return { chatId, userId, userMessageId };
}

const DEBUG = false;

async function streamChat(token, chatId, userId, userMessageId, prompt) {
  const { signature, timestamp, requestId } = generateSignature(prompt, userId);

  const now = new Date();
  const pad = (n) => String(n).padStart(2, '0');
  const currentDateTime = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`;
  const currentDate = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
  const currentTime = `${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`;
  const weekdays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
  const weekday = weekdays[now.getDay()];
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Calcutta';

  const queryParams = new URLSearchParams({
    timestamp,
    requestId,
    user_id: userId,
    version: '0.0.1',
    platform: 'web',
    token,
    user_agent: 'Mozilla/5.0',
    language: 'en-US',
    timezone: tz,
    screen_resolution: '1366x768',
    current_url: `https://chat.z.ai/c/${chatId}`,
    signature_timestamp: timestamp,
  });

  const postData = JSON.stringify({
    stream: true,
    model: MODEL,
    messages: [{ role: 'user', content: prompt }],
    signature_prompt: prompt,
    params: {},
    extra: {},
    features: {
      image_generation: false,
      web_search: false,
      auto_web_search: false,
      preview_mode: true,
      flags: ['general_agent'],
      vlm_tools_enable: false,
      vlm_web_search_enable: false,
      vlm_website_mode: false,
      enable_thinking: true,
    },
    variables: {
      '{{USER_NAME}}': 'Guest',
      '{{USER_LOCATION}}': 'Unknown',
      '{{CURRENT_DATETIME}}': currentDateTime,
      '{{CURRENT_DATE}}': currentDate,
      '{{CURRENT_TIME}}': currentTime,
      '{{CURRENT_WEEKDAY}}': weekday,
      '{{CURRENT_TIMEZONE}}': tz,
      '{{USER_LANGUAGE}}': 'en-US',
    },
    chat_id: chatId,
    id: requestId,
    current_user_message_id: crypto.randomUUID(),
    current_user_message_parent_id: null,
    background_tasks: { title_generation: true, tags_generation: true },
  });

  const options = {
    hostname: BASE_HOST,
    path: `/api/v2/chat/completions?${queryParams}`,
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      authorization: `Bearer ${token}`,
      'x-signature': signature,
      'x-fe-version': 'prod-fe-1.1.7',
      'Content-Length': Buffer.byteLength(postData),
    },
  };

  return new Promise((resolve, reject) => {
    const req = https.request(options, (res) => {
      if (res.statusCode !== 200) {
        let errBody = '';
        res.on('data', (c) => (errBody += c));
        res.on('end', () => reject(new Error(`HTTP ${res.statusCode}: ${errBody}`)));
        return;
      }

      let fullResponse = '';
      let buffer = '';
      let thinkingContent = '';
      let lastEvent = '';
      let errorMsg = '';

      process.stdout.write('\n🤖 AI: ');

      res.on('data', (chunk) => {
        buffer += chunk.toString();
        const lines = buffer.split('\n');
        buffer = lines.pop();

        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed) continue;

          // Track SSE event type
          if (trimmed.startsWith('event:')) {
            lastEvent = trimmed.slice(6).trim();
            if (DEBUG) process.stderr.write(`[EVENT] ${lastEvent}\n`);
            continue;
          }

          if (!trimmed.startsWith('data:')) continue;
          const data = trimmed.slice(5).trim();
          if (!data || data === '[DONE]') continue;

          if (DEBUG) process.stderr.write(`[DATA] ${data.substring(0, 300)}\n`);

          try {
            const json = JSON.parse(data);

            // Handle error events
            if (json.type === 'error' && json.data?.error) {
              const err = json.data.error;
              errorMsg += (err.code || '') + ': ' + (err.message || JSON.stringify(err));
              continue;
            }

            // Extract the payload - z.ai wraps in json.data, OpenAI in json.choices
            const d = json.data || json.choices?.[0]?.delta || json;

            // Capture errors inside completion payloads too
            if (d.error) {
              const err = d.error;
              errorMsg += (err.code || '') + ': ' + (err.message || err.detail || JSON.stringify(err));
            }

            // Handle thinking/reasoning content (hidden-thinking)
            if (d.thinking_content) {
              thinkingContent += d.thinking_content;
              if (DEBUG) process.stderr.write('[THINK] ');
            }

            // Primary: delta_content (z.ai streaming format)
            if (d.delta_content) {
              fullResponse += d.delta_content;
              process.stdout.write(d.delta_content);
            }
            // content field (non-streaming completion or final chunk)
            else if (typeof d.content === 'string' && d.content) {
              fullResponse += d.content;
              process.stdout.write(d.content);
            }
            // OpenAI streaming delta
            else if (d.delta?.content) {
              fullResponse += d.delta.content;
              process.stdout.write(d.delta.content);
            }
            // message content (non-streaming fallback)
            else if (d.message?.content) {
              fullResponse += d.message.content;
              process.stdout.write(d.message.content);
            }
            // text field
            else if (typeof d.text === 'string' && d.text) {
              fullResponse += d.text;
              process.stdout.write(d.text);
            }

            // edit_content: full replacement
            if (d.edit_content) {
              fullResponse = d.edit_content;
            }
          } catch (e) {
            if (DEBUG) process.stderr.write(`[ERR] ${e.message} raw=${data.substring(0, 100)}\n`);
          }
        }
      });

      res.on('end', () => {
        if (DEBUG && thinkingContent) {
          process.stderr.write(`\n[THINKING] ${thinkingContent}\n`);
        }
        if (errorMsg) {
          process.stdout.write(`\n\n⚠️  Server Error: ${errorMsg}`);
        }
        process.stdout.write('\n\n');
        resolve({ response: fullResponse.trim(), thinking: thinkingContent.trim(), error: errorMsg || null });
      });
    });

    req.on('error', reject);
    req.write(postData);
    req.end();
  });
}

// ─── File commands ──────────────────────────────────────────────────────────

async function filesList(token, chatId) {
  const bodyStr = JSON.stringify({ chatId });

  const options = {
    hostname: BASE_HOST,
    path: '/api/v1/web-dev/workspaces/files/ls-tree',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
      'Content-Length': Buffer.byteLength(bodyStr),
    },
  };

  console.log('\n📂 Fetching file list...');
  const res = await request(options, bodyStr);

  if (res.status !== 200) {
    throw new Error(`Failed to list files (${res.status}): ${JSON.stringify(res.body)}`);
  }

  const files = res.body;
  if (Array.isArray(files)) {
    if (files.length === 0) {
      console.log('   📭 No files found.');
    } else {
      console.log(`   📁 ${files.length} file(s):\n`);
      // Group by directory for readability
      const dirs = {};
      for (const f of files) {
        const parts = f.split('/');
        const dir = parts.length > 1 ? parts.slice(0, -1).join('/') : '.';
        if (!dirs[dir]) dirs[dir] = [];
        dirs[dir].push(parts[parts.length - 1]);
      }
      for (const [dir, fileNames] of Object.entries(dirs).sort()) {
        console.log(`   📂 ${dir}/`);
        for (const name of fileNames.sort()) {
          console.log(`      📄 ${name}`);
        }
      }
    }
  } else {
    console.log('   ', JSON.stringify(files, null, 2));
  }
  console.log();
}

async function filesView(token, chatId, filepath) {
  const bodyStr = JSON.stringify({ chatId, rev: 'HEAD', filepath });

  const options = {
    hostname: BASE_HOST,
    path: '/api/v1/web-dev/workspaces/files/content',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
      'Content-Length': Buffer.byteLength(bodyStr),
    },
  };

  console.log(`\n📄 Viewing: ${filepath}`);
  const res = await request(options, bodyStr);

  if (res.status !== 200) {
    throw new Error(`Failed to view file (${res.status}): ${JSON.stringify(res.body)}`);
  }

  console.log(`\n${'─'.repeat(60)}`);
  console.log(res.body);
  console.log(`${'─'.repeat(60)}\n`);
}

async function filesSearch(token, versionId, prefix) {
  const options = {
    hostname: BASE_HOST,
    path: `/api/v1/storage/versions/${versionId}/files?prefix=${encodeURIComponent(prefix)}`,
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${token}`,
    },
  };

  console.log(`\n🔍 Searching files with prefix: "${prefix}"...`);
  const res = await request(options, null);

  if (res.status !== 200) {
    throw new Error(`Failed to search files (${res.status}): ${JSON.stringify(res.body)}`);
  }

  const result = res.body;
  if (result.success && result.data && result.data.files) {
    const files = result.data.files;
    if (files.length === 0) {
      console.log('   📭 No files found matching that prefix.');
    } else {
      console.log(`   📁 ${files.length} file(s) found (total: ${result.data.total}):\n`);
      for (const f of files) {
        const size = f.size ? ` (${f.size} bytes)` : '';
        const modified = f.last_modified ? ` - ${f.last_modified}` : '';
        console.log(`   📄 ${f.key}${size}${modified}`);
      }
    }
  } else {
    console.log('   ', JSON.stringify(result, null, 2));
  }
  console.log();
}

// ─── main ───────────────────────────────────────────────────────────────────

async function main() {
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });

  const ask = (q) => new Promise((r) => rl.question(q, r));

  console.log('╔═══════════════════════════════════════════════════════╗');
  console.log('║          Z.AI CLI Chat Client                        ║');
  console.log('║          Type "exit" or "quit" to stop               ║');
  console.log('║          Type "new" to start a new chat              ║');
  console.log('║          /files list   - List all workspace files    ║');
  console.log('║          /files view <path> - View file contents     ║');
  console.log('║          /files search <prefix> - Search files       ║');
  console.log('║          /preview - Show app preview URL             ║');
  console.log('║          /setversion <id> - Set storage version ID   ║');
  console.log('║          /help - Show all commands                   ║');
  console.log('╚═══════════════════════════════════════════════════════╝');
  console.log();

  const token = await ask('🔑 Enter your Z.AI token: ').then((s) => s.trim());
  if (!token) {
    console.log('❌ No token provided. Exiting.');
    rl.close();
    return;
  }

  console.log('⚠️  Note: You must be logged in with a real account (NOT a guest) for full functionality.');
  console.log('   Guest accounts have limited access to workspaces, files, and app previews.');
  console.log('   Make sure your token belongs to a registered, logged-in user.');
  console.log();

  const decoded = decodeJWT(token);
  if (decoded) {
    console.log(`   👤 User: ${decoded.email || decoded.id || 'unknown'}`);
    // Warn if it looks like a guest
    const email = decoded.email || '';
    const name = decoded.name || '';
    if (email.toLowerCase().includes('guest') || name.toLowerCase().includes('guest') || !decoded.id) {
      console.log('   ⚠️  This appears to be a guest account. Some features may not work.');
      console.log('   Please log in with a real account for full access.');
    }
  }

  console.log();

  // First message to create the session
  const firstPrompt = await ask('💬 Enter your first message: ').then((s) => s.trim());
  if (!firstPrompt) {
    console.log('❌ No message provided. Exiting.');
    rl.close();
    return;
  }

  let chatId, userId;
  let workspaceData = null;
  let versionId = null;

  try {
    const session = await createNewChat(token, firstPrompt);
    chatId = session.chatId;
    userId = session.userId;
  } catch (err) {
    console.error(`❌ Failed to create chat: ${err.message}`);
    rl.close();
    return;
  }

  // Show app preview URL
  console.log(`🌐 App Preview: https://preview-chat-${chatId}.space.z.ai`);
  console.log('   (If no app is built yet, it will show the z.ai logo)');
  console.log();

  // Initialize workspace
  try {
    workspaceData = await workspaceUp(token, chatId);
    // Try to extract storage version ID from workspace data
    if (workspaceData) {
      versionId = workspaceData.version_id || workspaceData.versionId ||
                  workspaceData.storage_id || workspaceData.storageId ||
                  workspaceData.storage_version_id || workspaceData.id || null;
      if (versionId) {
        console.log(`   📦 Storage version ID: ${versionId}`);
      } else {
        console.log('   ℹ️  Storage version ID not found in workspace data.');
        console.log('      Use /setversion <id> to set it manually for /files search.');
      }
    }
  } catch (err) {
    console.error(`❌ Workspace init failed: ${err.message}`);
    console.log('   Continuing anyway (may get errors)...');
  }

  // Send first message
  try {
    await streamChat(token, chatId, userId, null, firstPrompt);
  } catch (err) {
    console.error(`❌ Chat error: ${err.message}`);
  }

  // Chat loop
  const loop = async () => {
    const prompt = await ask('\n💬 You: ').then((s) => s.trim());
    if (!prompt || prompt === 'exit' || prompt === 'quit') {
      console.log('\n👋 Bye!');
      rl.close();
      process.exit(0);
    }

    if (prompt === 'new') {
      try {
        const newPrompt = await ask('💬 Enter first message for new chat: ').then((s) => s.trim());
        if (!newPrompt) { loop(); return; }
        const session = await createNewChat(token, newPrompt);
        chatId = session.chatId;
        userId = session.userId;
        console.log(`\n🌐 App Preview: https://preview-chat-${chatId}.space.z.ai`);
        console.log('   (If no app is built yet, it will show the z.ai logo)\n');
        workspaceData = await workspaceUp(token, chatId);
        if (workspaceData) {
          versionId = workspaceData.version_id || workspaceData.versionId ||
                      workspaceData.storage_id || workspaceData.storageId ||
                      workspaceData.storage_version_id || workspaceData.id || null;
          if (versionId) {
            console.log(`   📦 Storage version ID: ${versionId}`);
          } else {
            console.log('   ℹ️  Storage version ID not found in workspace data.');
            console.log('      Use /setversion <id> to set it manually for /files search.');
          }
        }
        await streamChat(token, chatId, userId, null, newPrompt);
      } catch (err) {
        console.error(`❌ Error: ${err.message}`);
      }
      loop();
      return;
    }

    // ─── /help command ──────────────────────────────────────────
    if (prompt === '/help') {
      console.log('\n📖 Available commands:');
      console.log('   exit, quit          - Exit the CLI');
      console.log('   new                 - Start a new chat session');
      console.log('   /files list         - List all workspace files');
      console.log('   /files view <path>  - View contents of a file');
      console.log('   /files search <pfx> - Search files by prefix');
      console.log('   /preview            - Show app preview URL');
      console.log('   /setversion <id>    - Set storage version ID for /files search');
      console.log('   /help               - Show this help message');
      console.log();
      loop();
      return;
    }

    // ─── /files commands ────────────────────────────────────────
    if (prompt === '/files list' || prompt === '/files ls') {
      try {
        await filesList(token, chatId);
      } catch (err) {
        console.error(`❌ ${err.message}`);
      }
      loop();
      return;
    }

    if (prompt.startsWith('/files view ')) {
      const filepath = prompt.slice('/files view '.length).trim();
      if (!filepath) {
        console.log('❌ Usage: /files view <filepath>');
        console.log('   Example: /files view download/test.txt');
      } else {
        try {
          await filesView(token, chatId, filepath);
        } catch (err) {
          console.error(`❌ ${err.message}`);
        }
      }
      loop();
      return;
    }

    if (prompt.startsWith('/files search ')) {
      const prefix = prompt.slice('/files search '.length).trim();
      if (!prefix) {
        console.log('❌ Usage: /files search <prefix>');
        console.log('   Example: /files search download');
      } else if (!versionId) {
        console.log('❌ No storage version ID available for /files search.');
        console.log('   Use /setversion <id> to set it manually.');
        console.log('   (The version ID is typically returned when the workspace is initialized.)');
      } else {
        try {
          await filesSearch(token, versionId, prefix);
        } catch (err) {
          console.error(`❌ ${err.message}`);
        }
      }
      loop();
      return;
    }

    if (prompt === '/files') {
      console.log('\n📂 File commands:');
      console.log('   /files list         - List all workspace files');
      console.log('   /files view <path>  - View contents of a file');
      console.log('   /files search <pfx> - Search files by prefix');
      console.log();
      loop();
      return;
    }

    // ─── /preview command ───────────────────────────────────────
    if (prompt === '/preview') {
      console.log(`\n🌐 App Preview: https://preview-chat-${chatId}.space.z.ai`);
      console.log('   (If no app is built yet, it will show the z.ai logo)');
      console.log();
      loop();
      return;
    }

    // ─── /setversion command ────────────────────────────────────
    if (prompt.startsWith('/setversion ')) {
      versionId = prompt.slice('/setversion '.length).trim();
      if (!versionId) {
        console.log('❌ Usage: /setversion <storage-version-id>');
      } else {
        console.log(`✅ Storage version ID set to: ${versionId}`);
      }
      console.log();
      loop();
      return;
    }

    // ─── normal chat ────────────────────────────────────────────
    try {
      await streamChat(token, chatId, userId, null, prompt);
    } catch (err) {
      console.error(`❌ Chat error: ${err.message}`);
      if (err.message.includes('401') || err.message.includes('token')) {
        console.log('💡 Token may be expired. Restart and enter a new token.');
        rl.close();
        process.exit(1);
      }
    }

    loop();
  };

  loop();
}

main().catch((err) => {
  console.error('Fatal error:', err);
  process.exit(1);
});
