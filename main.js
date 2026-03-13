"use strict";

const express = require("express");
const http = require("http");
const crypto = require("crypto");
const https = require("https");
const config = require("./config");

const app = express();
const server = http.createServer(app);

// ============== Z.AI DIRECT CONFIG ==============

const BASE_URL = "https://chat.z.ai";
const SALT_KEY = "key-@@@@)))()((9))-xxxx&&&%%%%%";
const DEFAULT_FE_VERSION = "prod-fe-1.0.185";

// ============== SESSION STATE ==============

const session = {
  token: "",
  userId: "",
  userName: "Guest",
  chatId: crypto.randomUUID(),
  messages: [],
  saltKey: SALT_KEY,
  feVersion: DEFAULT_FE_VERSION,
  features: {
    webSearch: false,
    autoWebSearch: false,
    thinking: false,
    imageGen: false,
    previewMode: false,
  },
  initialized: false,
  initializing: false,
};

// Streaming buffers for tool call detection (requestId -> accumulated content)
const streamingBuffers = new Map();

// Core instructions to inject at the start of every conversation (for Roo/Kilo Code compatibility)
const CORE_INSTRUCTIONS = `CRITICAL INSTRUCTIONS (ALWAYS FOLLOW):
1. When using tools, ALWAYS output tool calls in XML format like: <tool_call><function=name><parameter=key>value</parameter></function></tool_call>
   NEVER use JSON or markdown code blocks for tool calls.
2. Follow every instruction in the prompt deeply and thoroughly. Execute tasks completely. 
3. Also One  rule  only applies if my prompt includes this string  "You are Roo" if it does then  when changing modes with function switch_mode funtion toolcall dont ever forget parameter mode_slug or else it will likely fail and heres the tool usage for  making todo list use it when doing complex task etc
 <update_todo_list>
<todos>
- [ ] Create a .test file with content "im good"
- [ ] Read C:\\key.txt and display its contents
</todos>
</update_todo_list> this is only a example of tool usage   if my Prompt doesnt includes You are Roo then ignore This rule 
4. When using attempt_completion, ALWAYS use <parameter=result> - NEVER use <parameter=message> or <parameter=summary>. The parameter MUST be named "result".`;

// ============== MIDDLEWARE ==============

app.use((req, res, next) => {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS");
  res.header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-Id, X-Fresh-Session");
  if (req.method === "OPTIONS") return res.sendStatus(200);
  next();
});

app.use(express.json({ limit: "50mb" }));

function authMiddleware(req, res, next) {
  if (!config.auth.enabled) return next();
  const authHeader = req.headers.authorization;
  const token = authHeader?.replace(/^Bearer\s+/i, "");
  if (token !== config.auth.token) {
    return res.status(401).json({
      error: { message: "Invalid or missing authentication token", type: "authentication_error", code: "invalid_api_key" }
    });
  }
  next();
}

// ============== UTILITY FUNCTIONS ==============

function generateId() {
  return crypto.randomBytes(16).toString("hex");
}

function estimateTokens(text) {
  if (!text) return 0;
  return Math.ceil(text.length / 4);
}

function getMessageContent(content) {
  if (!content) return "";
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content
      .filter(part => part.type === "text" || typeof part === "string")
      .map(part => (typeof part === "string" ? part : part.text || ""))
      .join("\n");
  }
  return String(content);
}

function messagesToPrompt(messages, includeToolInstructions = true) {
  if (!Array.isArray(messages)) return messages;

  let systemMsg = null;
  const conversation = [];

  for (const msg of messages) {
    if (msg.role === "system") {
      systemMsg = getMessageContent(msg.content);
    } else {
      conversation.push(msg);
    }
  }

  let prompt = "";
  if (includeToolInstructions) prompt += `${CORE_INSTRUCTIONS}\n\n`;
  if (systemMsg) prompt += `System: ${systemMsg}\n\n`;

  for (const msg of conversation) {
    const role = msg.role || "user";
    const content = getMessageContent(msg.content);
    if (role === "user") prompt += `User: ${content}\n\n`;
    else if (role === "assistant") prompt += `Assistant: ${content}\n\n`;
    else if (role === "tool") prompt += `Tool Result: ${content}\n\n`;
  }

  return prompt.trim();
}

function parseToolCalls(content) {
  const toolCalls = [];

  content = content.replace(/<tool_call>([a-zA-Z_][a-zA-Z0-9_]*)>/gi, "<$1>");

  const markdownJsonPattern = /```(?:json)?\s*\n?\s*(\{[\s\S]*?\})\s*\n?```/gi;
  let match;

  while ((match = markdownJsonPattern.exec(content)) !== null) {
    try {
      const jsonData = JSON.parse(match[1]);
      if (jsonData.tool_calls && Array.isArray(jsonData.tool_calls)) {
        for (const tc of jsonData.tool_calls) {
          toolCalls.push({
            id: tc.id || `call_${generateId().substring(0, 24)}`,
            type: "function",
            function: {
              name: tc.function?.name || tc.name,
              arguments: typeof tc.function?.arguments === "string"
                ? tc.function.arguments
                : JSON.stringify(tc.function?.arguments || tc.arguments || tc.parameters || {})
            }
          });
        }
      } else if (jsonData.name || jsonData.function) {
        toolCalls.push({
          id: `call_${generateId().substring(0, 24)}`,
          type: "function",
          function: {
            name: jsonData.name || jsonData.function,
            arguments: typeof jsonData.arguments === "string"
              ? jsonData.arguments
              : JSON.stringify(jsonData.arguments || jsonData.parameters || {})
          }
        });
      }
    } catch (e) {}
  }

  const xmlPattern = /<tool_call>\s*<function=([^>]+)>([\s\S]*?)<\/function>\s*<\/tool_call>/gi;
  while ((match = xmlPattern.exec(content)) !== null) {
    const funcName = match[1].trim();
    const paramsBlock = match[2];
    const params = {};
    const paramPattern = /<parameter=([^>]+)>\s*([\s\S]*?)\s*<\/parameter>/gi;
    let paramMatch;
    while ((paramMatch = paramPattern.exec(paramsBlock)) !== null) {
      let paramValue = paramMatch[2].trim();
      try { paramValue = JSON.parse(paramValue); } catch (e) {}
      params[paramMatch[1].trim()] = paramValue;
    }
    toolCalls.push({
      id: `call_${generateId().substring(0, 24)}`,
      type: "function",
      function: { name: funcName, arguments: JSON.stringify(params) }
    });
  }

  const jsonPattern = /<tool_call>\s*(\{[\s\S]*?\})\s*<\/tool_call>/gi;
  while ((match = jsonPattern.exec(content)) !== null) {
    try {
      const toolData = JSON.parse(match[1]);
      if (toolData.name || toolData.function) {
        toolCalls.push({
          id: `call_${generateId().substring(0, 24)}`,
          type: "function",
          function: {
            name: toolData.name || toolData.function,
            arguments: typeof toolData.arguments === "string"
              ? toolData.arguments
              : JSON.stringify(toolData.arguments || toolData.parameters || {})
          }
        });
      }
    } catch (e) {}
  }

  const rooClineTools = [
    "write_file","read_file","apply_diff","execute_command","list_files","search_files",
    "ask_followup_question","attempt_completion","browser_action","update_todo_list",
    "switch_mode","new_task","fetch_instructions","delete_file","read_multiple_files",
    "write_multiple_files","search_and_replace","write_to_file","read_from_file",
    "list_directory","execute_shell","run_command","create_file","edit_file",
    "replace_in_file","insert_code","delete_code","move_file","copy_file","rename_file",
    "search_code","find_files","grep_search","ask_question","complete_task","finish_task",
    "submit_result","write","read","edit","bash","glob","grep","task","webfetch",
    "todowrite","todoread","skill","Write","Read","Edit","Bash","Glob","Grep","Task",
    "WebFetch","TodoWrite","TodoRead","Skill","AskUserQuestion"
  ];

  for (const toolName of rooClineTools) {
    const toolPattern = new RegExp("<" + toolName + "(?:\\s[^>]*)?>([\\s\\S]*?)</" + toolName + ">", "gi");
    while ((match = toolPattern.exec(content)) !== null) {
      const innerContent = match[1];
      const params = {};
      const paramPattern = /<([a-z_]+)>([\s\S]*?)<\/\1>/gi;
      let paramMatch;
      while ((paramMatch = paramPattern.exec(innerContent)) !== null) {
        params[paramMatch[1]] = paramMatch[2];
      }

      if (toolName === "attempt_completion" && !params.result) {
        const textContent = innerContent.replace(/<[^>]*>/g, "").trim();
        params.result = textContent || "Task completed successfully.";
      }

      const toolNameLower = toolName.toLowerCase();
      for (const t of ["write","read","edit"]) {
        if (toolNameLower === t) {
          if (params.filePath && !params.file_path) { params.file_path = params.filePath; delete params.filePath; }
          if (params.path && !params.file_path) { params.file_path = params.path; delete params.path; }
          if (params.file && !params.file_path) { params.file_path = params.file; delete params.file; }
        }
      }
      if (toolNameLower === "bash" && !params.description && params.command) params.description = "Execute command";
      if (toolNameLower === "todowrite" && params.todos && typeof params.todos === "string") {
        try { params.todos = JSON.parse(params.todos); } catch (e) {}
      }

      if (Object.keys(params).length > 0 || ["list_files"].includes(toolName)) {
        toolCalls.push({
          id: `call_${generateId().substring(0, 24)}`,
          type: "function",
          function: { name: toolName, arguments: JSON.stringify(params) }
        });
      }
    }
  }

  const unclosedFuncPattern = /<function=([a-z_]+)>([\s\S]*?)(?=<function=|$)/gi;
  while ((match = unclosedFuncPattern.exec(content)) !== null) {
    const funcName = match[1].trim();
    const paramsBlock = match[2];
    const params = {};
    const unclosedParamPattern = /<parameter=([a-z_]+)>([\s\S]*?)(?=<parameter=|<function=|$)/gi;
    let paramMatch;
    while ((paramMatch = unclosedParamPattern.exec(paramsBlock)) !== null) {
      params[paramMatch[1].trim()] = paramMatch[2].trim();
    }
    if (funcName === "attempt_completion" && !params.result) {
      params.result = params.summary || params.message || "Task completed successfully.";
      delete params.summary; delete params.message;
    }
    if (Object.keys(params).length > 0) {
      toolCalls.push({
        id: `call_${generateId().substring(0, 24)}`,
        type: "function",
        function: { name: funcName, arguments: JSON.stringify(params) }
      });
    }
  }

  return toolCalls;
}

function removeToolCallsFromContent(content) {
  let cleaned = content;
  cleaned = cleaned.replace(/<tool_call>([a-zA-Z_][a-zA-Z0-9_]*)>[\s\S]*?<\/\1>/gi, "");
  cleaned = cleaned.replace(/<tool_call>([a-zA-Z_][a-zA-Z0-9_]*)>[\s\S]*?(?=<tool_call>|$)/gi, "");
  cleaned = cleaned.replace(/<tool_call>[\s\S]*?<\/tool_call>/gi, "");

  const rooClineTools = [
    "write_file","read_file","apply_diff","execute_command","list_files","search_files",
    "ask_followup_question","attempt_completion","browser_action","update_todo_list",
    "switch_mode","new_task","fetch_instructions","delete_file","read_multiple_files",
    "write_multiple_files","search_and_replace","write_to_file","read_from_file",
    "list_directory","execute_shell","run_command","create_file","edit_file",
    "replace_in_file","insert_code","delete_code","move_file","copy_file","rename_file",
    "search_code","find_files","grep_search","ask_question","complete_task","finish_task",
    "submit_result","write","read","edit","bash","glob","grep","task","webfetch",
    "todowrite","todoread","skill","Write","Read","Edit","Bash","Glob","Grep","Task",
    "WebFetch","TodoWrite","TodoRead","Skill","AskUserQuestion"
  ];

  for (const toolName of rooClineTools) {
    const pattern = new RegExp("<" + toolName + "(?:\\s[^>]*)?>[\\s\\S]*?</" + toolName + ">", "gi");
    cleaned = cleaned.replace(pattern, "");
  }

  cleaned = cleaned.replace(/<function=[a-z_]+>[\s\S]*$/gi, "");
  cleaned = cleaned.replace(/```(?:json)?\s*\n?\s*\{[\s\S]*?"(?:name|tool_calls)"[\s\S]*?\}\s*\n?```/gi, "");
  cleaned = cleaned.replace(/\n{3,}/g, "\n\n").trim();
  return cleaned;
}

function hasIncompleteToolCall(content) {
  const patterns = [
    /<tool_call>(?![\s\S]*<\/tool_call>)/i,
    /<function=[^>]+>(?![\s\S]*<\/function>)/i,
    /<write_file>(?![\s\S]*<\/write_file>)/i,
    /<write_to_file>(?![\s\S]*<\/write_to_file>)/i,
    /<read_file>(?![\s\S]*<\/read_file>)/i,
    /<read_from_file>(?![\s\S]*<\/read_from_file>)/i,
    /<apply_diff>(?![\s\S]*<\/apply_diff>)/i,
    /<execute_command>(?![\s\S]*<\/execute_command>)/i,
    /<run_command>(?![\s\S]*<\/run_command>)/i,
    /<attempt_completion>(?![\s\S]*<\/attempt_completion>)/i,
    /<complete_task>(?![\s\S]*<\/complete_task>)/i,
    /<edit_file>(?![\s\S]*<\/edit_file>)/i,
    /<replace_in_file>(?![\s\S]*<\/replace_in_file>)/i,
    /```(?:json)?\s*\n?\s*\{[^}]*$/i,
    /<write>(?![\s\S]*<\/write>)/i,
    /<read>(?![\s\S]*<\/read>)/i,
    /<edit>(?![\s\S]*<\/edit>)/i,
    /<bash>(?![\s\S]*<\/bash>)/i,
    /<glob>(?![\s\S]*<\/glob>)/i,
    /<grep>(?![\s\S]*<\/grep>)/i,
    /<task>(?![\s\S]*<\/task>)/i,
    /<Write>(?![\s\S]*<\/Write>)/,
    /<Read>(?![\s\S]*<\/Read>)/,
    /<Edit>(?![\s\S]*<\/Edit>)/,
    /<Bash>(?![\s\S]*<\/Bash>)/,
    /<Glob>(?![\s\S]*<\/Glob>)/,
    /<Grep>(?![\s\S]*<\/Grep>)/,
    /<Task>(?![\s\S]*<\/Task>)/,
    /<TodoWrite>(?![\s\S]*<\/TodoWrite>)/,
    /<AskUserQuestion>(?![\s\S]*<\/AskUserQuestion>)/,
    /<tool_call>[A-Za-z]+>(?![\s\S]*<\/[A-Za-z]+>)/,
  ];
  for (const p of patterns) if (p.test(content)) return true;
  return false;
}

function formatOpenAIResponse(result, model, requestId, stream = false, fullContent = null) {
  const timestamp = Math.floor(Date.now() / 1000);
  const rawContent = result.content || result.text || "";

  if (stream) {
    if (result.finish_reason !== "stop") {
      return {
        id: `chatcmpl-${requestId}`,
        object: "chat.completion.chunk",
        created: timestamp,
        model: model || "glm-4.7",
        choices: [{ index: 0, delta: { content: rawContent }, finish_reason: null }]
      };
    }

    const contentToCheck = fullContent || rawContent;
    const toolCalls = parseToolCalls(contentToCheck);

    if (toolCalls.length > 0) {
      return {
        id: `chatcmpl-${requestId}`,
        object: "chat.completion.chunk",
        created: timestamp,
        model: model || "glm-4.7",
        choices: [{
          index: 0,
          delta: {
            tool_calls: toolCalls.map((tc, idx) => ({
              index: idx, id: tc.id, type: "function",
              function: { name: tc.function.name, arguments: tc.function.arguments }
            }))
          },
          finish_reason: "tool_calls"
        }]
      };
    }

    return {
      id: `chatcmpl-${requestId}`,
      object: "chat.completion.chunk",
      created: timestamp,
      model: model || "glm-4.7",
      choices: [{ index: 0, delta: { content: rawContent }, finish_reason: "stop" }]
    };
  }

  const toolCalls = parseToolCalls(rawContent);
  const cleanContent = toolCalls.length > 0 ? removeToolCallsFromContent(rawContent) : rawContent;

  return {
    id: `chatcmpl-${requestId}`,
    object: "chat.completion",
    created: timestamp,
    model: model || "glm-4.7",
    choices: [{
      index: 0,
      message: {
        role: "assistant",
        content: toolCalls.length > 0 ? (cleanContent || null) : cleanContent,
        ...(toolCalls.length > 0 && { tool_calls: toolCalls })
      },
      finish_reason: toolCalls.length > 0 ? "tool_calls" : "stop"
    }],
    usage: {
      prompt_tokens: estimateTokens(result.prompt || ""),
      completion_tokens: estimateTokens(rawContent),
      total_tokens: estimateTokens(result.prompt || "") + estimateTokens(rawContent)
    }
  };
}

function formatOpenAIError(message, type = "api_error", code = null) {
  return { error: { message, type, code, param: null } };
}

// ============== Z.AI DIRECT HTTP FUNCTIONS ==============

async function scrapeConfig() {
  try {
    const res = await fetch(BASE_URL, { signal: AbortSignal.timeout(10000) });
    const text = await res.text();
    const match = text.match(/prod-fe-\d+\.\d+\.\d+/);
    if (match) {
      session.feVersion = match[0];
      console.log(`[Config] fe_version: ${session.feVersion}`);
    }
  } catch (e) {
    console.warn(`[Config] Scrape error: ${e.message}, using default feVersion`);
  }
}

function generateZaSignature(prompt, token, userId) {
  const timestamp = String(Date.now());
  const requestId = crypto.randomUUID();
  const bucket = Math.floor(Number(timestamp) / 300000);

  const wKey = crypto
    .createHmac("sha256", session.saltKey)
    .update(String(bucket))
    .digest("hex");

  const payloadDict = { requestId, timestamp, user_id: userId };
  const sortedItems = Object.entries(payloadDict).sort((a, b) => a[0].localeCompare(b[0]));
  const sortedPayload = sortedItems.map(([k, v]) => `${k},${v}`).join(",");

  const promptB64 = Buffer.from(prompt.trim()).toString("base64");
  const dataToSign = `${sortedPayload}|${promptB64}|${timestamp}`;

  const signature = crypto
    .createHmac("sha256", wKey)
    .update(dataToSign)
    .digest("hex");

  const params = new URLSearchParams({
    timestamp, requestId, user_id: userId,
    version: "0.0.1", platform: "web", token,
    user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0",
    language: "en-US", screen_resolution: "1920x1080",
    viewport_size: "1920x1080", timezone: "Europe/Paris",
    timezone_offset: "-60", signature_timestamp: timestamp
  });

  return { signature, timestamp, urlParams: params.toString() };
}

async function initializeSession() {
  if (session.initializing) {
    // Wait for ongoing init
    await new Promise(resolve => {
      const check = setInterval(() => {
        if (!session.initializing) { clearInterval(check); resolve(); }
      }, 100);
    });
    return;
  }

  session.initializing = true;
  console.log("[Session] Initializing Z.AI session...");

  try {
    await scrapeConfig();

    const headers = {
      "Origin": BASE_URL,
      "Referer": `${BASE_URL}/`,
      "Content-Type": "application/json"
    };

    // Guest auth
    await fetch(`${BASE_URL}/api/v1/auths/guest`, {
      method: "POST", headers, body: "{}", signal: AbortSignal.timeout(15000)
    });

    // Get token
    const authRes = await fetch(`${BASE_URL}/api/v1/auths/`, {
      headers, signal: AbortSignal.timeout(15000)
    });

    if (!authRes.ok) throw new Error(`Auth failed: ${authRes.status}`);

    const authData = await authRes.json();
    session.token = authData.token || "";

    if (!session.token) {
      const guestRes = await fetch(`${BASE_URL}/api/v1/auths/guest`, {
        method: "POST", headers, body: "{}", signal: AbortSignal.timeout(15000)
      });
      if (guestRes.ok) {
        const gd = await guestRes.json();
        session.token = gd.token || "";
      }
    }

    if (session.token) {
      try {
        const parts = session.token.split(".");
        const padded = parts[1] + "==";
        const payload = JSON.parse(Buffer.from(padded, "base64").toString("utf8"));
        session.userId = payload.id || "";
        session.userName = (payload.email || "Guest").split("@")[0];
        console.log(`[Session] Connected. UserID: ${session.userId.substring(0, 8)}... (${session.userName})`);
      } catch (e) {
        console.warn("[Session] Token decode failed, but continuing.");
      }
      session.initialized = true;
    } else {
      throw new Error("No token received from Z.AI");
    }
  } catch (e) {
    console.error("[Session] Initialization error:", e.message);
    session.initialized = false;
    throw e;
  } finally {
    session.initializing = false;
  }
}

function getContextVars() {
  const now = new Date();
  const pad = n => String(n).padStart(2, "0");
  const date = `${now.getFullYear()}-${pad(now.getMonth()+1)}-${pad(now.getDate())}`;
  const time = `${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`;
  const days = ["Sunday","Monday","Tuesday","Wednesday","Thursday","Friday","Saturday"];
  return {
    "{{USER_NAME}}": session.userName,
    "{{USER_LOCATION}}": "Unknown",
    "{{CURRENT_DATETIME}}": `${date} ${time}`,
    "{{CURRENT_DATE}}": date,
    "{{CURRENT_TIME}}": time,
    "{{CURRENT_WEEKDAY}}": days[now.getDay()],
    "{{CURRENT_TIMEZONE}}": "Europe/Paris",
    "{{USER_LANGUAGE}}": "en-US"
  };
}

/**
 * Send prompt to Z.AI, returns an async generator yielding text chunks.
 * Handles 401 by re-initializing session once.
 */
async function* sendToZAI(prompt, options = {}) {
  const {
    model = "glm-4.7",
    webSearch = session.features.webSearch,
    thinking = session.features.thinking,
    imageGen = session.features.imageGen,
    previewMode = session.features.previewMode,
    chatId = session.chatId,
    messages = session.messages,
    freshSession = false,
  } = options;

  if (!session.initialized) await initializeSession();

  const { signature, urlParams } = generateZaSignature(prompt, session.token, session.userId);
  const url = `${BASE_URL}/api/v2/chat/completions?${urlParams}`;

  const headers = {
    "Origin": BASE_URL,
    "Referer": `${BASE_URL}/`,
    "Authorization": `Bearer ${session.token}`,
    "X-Signature": signature,
    "X-FE-Version": session.feVersion,
    "Content-Type": "application/json"
  };

  const msgList = [...messages];
  msgList.push({ role: "user", content: prompt });

  const body = JSON.stringify({
    model,
    chat_id: chatId,
    messages: msgList,
    signature_prompt: prompt,
    stream: true,
    params: {},
    extra: {},
    features: {
      image_generation: imageGen,
      web_search: webSearch,
      auto_web_search: webSearch,
      preview_mode: previewMode,
      flags: [],
      enable_thinking: thinking
    },
    variables: getContextVars(),
    background_tasks: { title_generation: true, tags_generation: true }
  });

  let res;
  try {
    res = await fetch(url, { method: "POST", headers, body, signal: AbortSignal.timeout(90000) });
  } catch (e) {
    throw new Error(`Z.AI connection error: ${e.message}`);
  }

  if (res.status === 401) {
    // Re-init session and retry once
    session.initialized = false;
    await initializeSession();
    yield* sendToZAI(prompt, options);
    return;
  }

  if (!res.ok) {
    const errText = await res.text().catch(() => "");
    throw new Error(`Z.AI error ${res.status}: ${errText}`);
  }

  // SSE streaming parse
  const decoder = new TextDecoder();
  let buffer = "";

  for await (const chunk of res.body) {
    buffer += decoder.decode(chunk, { stream: true });
    const lines = buffer.split("\n");
    buffer = lines.pop(); // keep incomplete line

    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed || !trimmed.startsWith("data: ")) continue;
      const dataStr = trimmed.slice(6);
      if (dataStr === "[DONE]") return;
      try {
        const json = JSON.parse(dataStr);
        let chunk = "";
        if (json.data?.delta_content !== undefined) chunk = json.data.delta_content;
        else if (json.choices?.[0]?.delta?.content !== undefined) chunk = json.choices[0].delta.content;
        if (chunk) yield chunk;
      } catch (e) {}
    }
  }

  // Flush remaining buffer
  if (buffer.trim().startsWith("data: ")) {
    const dataStr = buffer.trim().slice(6);
    if (dataStr !== "[DONE]") {
      try {
        const json = JSON.parse(dataStr);
        let chunk = "";
        if (json.data?.delta_content !== undefined) chunk = json.data.delta_content;
        else if (json.choices?.[0]?.delta?.content !== undefined) chunk = json.choices[0].delta.content;
        if (chunk) yield chunk;
      } catch (e) {}
    }
  }
}

// ============== DASHBOARD HTML ==============

const getDashboardHTML = (host) => `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Z.AI Direct Bridge</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      background: linear-gradient(135deg, #1e3a5f 0%, #0d1b2a 50%, #1b263b 100%);
      min-height: 100vh; color: #e0e0e0; padding: 20px;
    }
    .container { max-width: 1200px; margin: 0 auto; }
    .header {
      text-align: center; padding: 40px 20px;
      background: rgba(255,255,255,0.05); border-radius: 16px;
      margin-bottom: 30px; border: 1px solid rgba(255,255,255,0.1);
    }
    .header h1 {
      font-size: 2.5rem;
      background: linear-gradient(135deg, #3b82f6, #1d4ed8, #60a5fa);
      -webkit-background-clip: text; -webkit-text-fill-color: transparent;
      margin-bottom: 10px;
    }
    .header p { color: #888; font-size: 1.1rem; }
    .badge {
      display: inline-block; background: #22c55e; color: #000;
      padding: 4px 12px; border-radius: 12px; font-size: 0.8rem; font-weight: 700;
      margin-top: 8px;
    }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
    .card {
      background: rgba(255,255,255,0.05); border-radius: 12px;
      padding: 24px; border: 1px solid rgba(255,255,255,0.1);
    }
    .card h2 { color: #60a5fa; margin-bottom: 16px; font-size: 1.2rem; }
    .stat-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
    .stat { background: rgba(0,0,0,0.2); padding: 12px; border-radius: 8px; }
    .stat .label { color: #888; font-size: 0.85rem; }
    .stat .value { color: #60a5fa; font-weight: 600; font-size: 1.5rem; margin-top: 4px; }
    .status-dot { width: 12px; height: 12px; border-radius: 50%; display: inline-block; margin-right: 8px; }
    .status-dot.ok { background: #22c55e; }
    .status-dot.err { background: #ef4444; }
    .code-block {
      background: #0d1117; border-radius: 8px; padding: 16px; overflow-x: auto;
      font-family: 'Monaco', 'Menlo', monospace; font-size: 0.85rem;
      border: 1px solid #30363d; margin: 12px 0;
    }
    .code-block code { color: #c9d1d9; white-space: pre-wrap; }
    .endpoint { background: rgba(0,0,0,0.2); padding: 12px; border-radius: 8px; margin-bottom: 8px; }
    .method {
      display: inline-block; padding: 4px 8px; border-radius: 4px;
      font-size: 0.75rem; font-weight: 600; margin-right: 8px;
    }
    .method.get { background: #22c55e; color: #000; }
    .method.post { background: #3b82f6; color: #fff; }
    .path { font-family: monospace; color: #e0e0e0; }
    .desc { color: #888; font-size: 0.85rem; margin-top: 4px; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>Z.AI Direct Bridge</h1>
      <p>HTTP-only mode — No browser required</p>
      <span class="badge">⚡ Direct Mode</span>
    </div>

    <div class="grid">
      <div class="card">
        <h2>Session Status</h2>
        <div class="stat-grid">
          <div class="stat">
            <div class="label">Connection</div>
            <div class="value" id="sessionStatus">...</div>
          </div>
          <div class="stat">
            <div class="label">User</div>
            <div class="value" id="sessionUser" style="font-size:1rem">...</div>
          </div>
          <div class="stat">
            <div class="label">Messages</div>
            <div class="value" id="msgCount">0</div>
          </div>
          <div class="stat">
            <div class="label">FE Version</div>
            <div class="value" id="feVersion" style="font-size:0.85rem">...</div>
          </div>
        </div>
      </div>

      <div class="card">
        <h2>Features</h2>
        <div class="stat-grid">
          <div class="stat">
            <div class="label">Web Search</div>
            <div class="value" id="featSearch">-</div>
          </div>
          <div class="stat">
            <div class="label">Thinking</div>
            <div class="value" id="featThink">-</div>
          </div>
          <div class="stat">
            <div class="label">Image Gen</div>
            <div class="value" id="featImage">-</div>
          </div>
          <div class="stat">
            <div class="label">Preview</div>
            <div class="value" id="featPreview">-</div>
          </div>
        </div>
      </div>

      <div class="card" style="grid-column: span 2;">
        <h2>API Endpoints</h2>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/v1/chat/completions</span>
          <div class="desc">OpenAI-compatible chat endpoint. Supports streaming with stream: true</div>
        </div>
        <div class="endpoint">
          <span class="method get">GET</span>
          <span class="path">/v1/models</span>
          <div class="desc">List available models (OpenAI format)</div>
        </div>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/prompt</span>
          <div class="desc">Legacy prompt endpoint</div>
        </div>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/features</span>
          <div class="desc">Update session features (webSearch, thinking, imageGen, previewMode)</div>
        </div>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/admin/session/clear</span>
          <div class="desc">Clear conversation history and start new chat</div>
        </div>
      </div>

      <div class="card" style="grid-column: span 2;">
        <h2>Example Usage</h2>
        <div class="code-block">
          <code># Basic chat (non-streaming)
curl -X POST http://${host}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${config.auth.token}" \\
  -d '{"model":"glm-4.7","messages":[{"role":"user","content":"Hello!"}],"stream":false}'

# Streaming
curl -X POST http://${host}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${config.auth.token}" \\
  -d '{"model":"glm-4.7","messages":[{"role":"user","content":"Tell me a story"}],"stream":true}'

# Fresh session (resets context)
curl -X POST http://${host}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${config.auth.token}" \\
  -H "X-Fresh-Session: true" \\
  -d '{"model":"z1","messages":[{"role":"user","content":"New topic"}]}'

# Enable web search
curl -X POST http://${host}/features \\
  -H "Authorization: Bearer ${config.auth.token}" \\
  -H "Content-Type: application/json" \\
  -d '{"webSearch":true}'</code>
        </div>
      </div>
    </div>
  </div>

  <script>
    async function updateStatus() {
      try {
        const res = await fetch('/status');
        const d = await res.json();
        document.getElementById('sessionStatus').textContent = d.connected ? '✓ OK' : '✗ Off';
        document.getElementById('sessionUser').textContent = d.userName || '-';
        document.getElementById('msgCount').textContent = d.messageCount;
        document.getElementById('feVersion').textContent = d.feVersion || '-';
        document.getElementById('featSearch').textContent = d.features?.webSearch ? 'ON' : 'OFF';
        document.getElementById('featThink').textContent = d.features?.thinking ? 'ON' : 'OFF';
        document.getElementById('featImage').textContent = d.features?.imageGen ? 'ON' : 'OFF';
        document.getElementById('featPreview').textContent = d.features?.previewMode ? 'ON' : 'OFF';
      } catch(e) { console.error(e); }
    }
    updateStatus();
    setInterval(updateStatus, 3000);
  </script>
</body>
</html>`;

// ============== ROUTES ==============

app.get("/", (req, res) => {
  const host = req.headers.host || `localhost:${config.server.port}`;
  res.send(getDashboardHTML(host));
});

app.get("/status", (req, res) => {
  res.json({
    connected: session.initialized,
    userName: session.userName,
    userId: session.userId ? session.userId.substring(0, 8) + "..." : null,
    feVersion: session.feVersion,
    chatId: session.chatId,
    messageCount: session.messages.length,
    features: session.features,
    mode: "direct"
  });
});

// ============== OPENAI-COMPATIBLE API ==============

const knownModels = ["glm-4.7", "glm-5", "z1", "z1-mini"];

app.get("/v1/models", authMiddleware, (req, res) => {
  res.json({
    object: "list",
    data: knownModels.map(m => ({
      id: m, object: "model", created: Math.floor(Date.now() / 1000),
      owned_by: "z-ai", permission: [], root: m, parent: null
    }))
  });
});

app.get("/models", authMiddleware, (req, res) => {
  res.json({ models: knownModels, currentModel: "glm-4.7" });
});

app.post("/v1/chat/completions", authMiddleware, async (req, res) => {
  const { model = "glm-4.7", messages, stream = true, deepThink, search, webSearch } = req.body;

  if (!messages || !Array.isArray(messages)) {
    return res.status(400).json(formatOpenAIError("messages is required and must be an array", "invalid_request_error"));
  }

  const freshSession = req.headers["x-fresh-session"] === "true";
  const requestId = generateId();

  // Extract last user prompt and keep the rest in session messages
  const prompt = messagesToPrompt(messages);

  // Handle fresh session
  if (freshSession) {
    session.messages = [];
    session.chatId = crypto.randomUUID();
    console.log(`[Session] Fresh session started. New chatId: ${session.chatId}`);
  }

  const opts = {
    model,
    webSearch: webSearch ?? search ?? session.features.webSearch,
    thinking: deepThink ?? session.features.thinking,
    imageGen: session.features.imageGen,
    previewMode: session.features.previewMode,
    chatId: session.chatId,
    messages: session.messages,
  };

  if (stream) {
    res.setHeader("Content-Type", "text/event-stream");
    res.setHeader("Cache-Control", "no-cache");
    res.setHeader("Connection", "keep-alive");
    res.setHeader("X-Accel-Buffering", "no");
    res.flushHeaders();

    // Initial empty chunk
    const initChunk = formatOpenAIResponse({ content: "", finish_reason: null }, model, requestId, true);
    res.write(`data: ${JSON.stringify(initChunk)}\n\n`);

    let fullContent = "";
    let sentContent = "";

    const keepAlive = setInterval(() => {
      try {
        const ka = formatOpenAIResponse({ content: "", finish_reason: null }, model, requestId, true);
        res.write(`data: ${JSON.stringify(ka)}\n\n`);
      } catch (e) { clearInterval(keepAlive); }
    }, 5000);

    try {
      for await (const chunk of sendToZAI(prompt, opts)) {
        fullContent += chunk;

        if (hasIncompleteToolCall(fullContent)) continue; // buffer

        const delta = fullContent.substring(sentContent.length);
        if (delta) {
          sentContent = fullContent;
          const c = formatOpenAIResponse({ content: delta, finish_reason: null }, model, requestId, true);
          res.write(`data: ${JSON.stringify(c)}\n\n`);
        }
      }

      // Send any remaining buffered content
      const remaining = fullContent.substring(sentContent.length);
      if (remaining) {
        const c = formatOpenAIResponse({ content: remaining, finish_reason: null }, model, requestId, true);
        res.write(`data: ${JSON.stringify(c)}\n\n`);
      }

      // Final stop chunk (checks full content for tool calls)
      const finalChunk = formatOpenAIResponse({ content: "", finish_reason: "stop" }, model, requestId, true, fullContent);
      res.write(`data: ${JSON.stringify(finalChunk)}\n\n`);
      res.write("data: [DONE]\n\n");

      // Update session history
      session.messages.push({ role: "user", content: prompt });
      if (fullContent) session.messages.push({ role: "assistant", content: fullContent });

    } catch (e) {
      console.error("[Stream] Error:", e.message);
      res.write(`data: ${JSON.stringify({ error: { message: e.message } })}\n\n`);
      res.write("data: [DONE]\n\n");
    } finally {
      clearInterval(keepAlive);
      res.end();
    }

  } else {
    // Non-streaming
    try {
      let fullContent = "";
      for await (const chunk of sendToZAI(prompt, opts)) {
        fullContent += chunk;
      }

      // Update session history
      session.messages.push({ role: "user", content: prompt });
      if (fullContent) session.messages.push({ role: "assistant", content: fullContent });

      res.json(formatOpenAIResponse({ content: fullContent }, model, requestId));
    } catch (e) {
      console.error("[API] Error:", e.message);
      const statusCode = e.message.includes("401") ? 401 : 500;
      res.status(statusCode).json(formatOpenAIError(e.message));
    }
  }
});

// ============== LEGACY API ==============

app.post("/prompt", authMiddleware, async (req, res) => {
  const { prompt, search, deepThink, webSearch } = req.body;
  if (!prompt) return res.status(400).json({ error: "Prompt is required" });

  const freshSession = req.headers["x-fresh-session"] === "true";
  if (freshSession) { session.messages = []; session.chatId = crypto.randomUUID(); }

  try {
    let fullContent = "";
    for await (const chunk of sendToZAI(prompt, {
      webSearch: webSearch ?? search ?? session.features.webSearch,
      thinking: deepThink ?? session.features.thinking,
    })) {
      fullContent += chunk;
    }
    session.messages.push({ role: "user", content: prompt });
    if (fullContent) session.messages.push({ role: "assistant", content: fullContent });

    res.json({ success: true, response: fullContent });
  } catch (e) {
    console.error("[Prompt] Error:", e.message);
    res.status(500).json({ success: false, error: e.message });
  }
});

// Features toggle
app.post("/features", authMiddleware, (req, res) => {
  const { webSearch, thinking, imageGen, previewMode } = req.body;
  if (webSearch !== undefined) { session.features.webSearch = !!webSearch; session.features.autoWebSearch = !!webSearch; }
  if (thinking !== undefined) session.features.thinking = !!thinking;
  if (imageGen !== undefined) session.features.imageGen = !!imageGen;
  if (previewMode !== undefined) session.features.previewMode = !!previewMode;
  console.log("[Features] Updated:", session.features);
  res.json({ success: true, features: session.features });
});

// ============== ADMIN ENDPOINTS ==============

app.get("/admin/stats", (req, res) => {
  res.json({
    mode: "direct",
    totalClients: session.initialized ? 1 : 0,
    idleClients: session.initialized ? 1 : 0,
    busyClients: 0,
    queueLength: 0,
    clients: session.initialized ? [{
      id: session.userId?.substring(0, 8) || "session",
      status: "idle",
      currentModel: "glm-4.7",
      requestCount: session.messages.length / 2
    }] : [],
    stats: {
      totalRequests: Math.floor(session.messages.length / 2),
      successfulRequests: Math.floor(session.messages.length / 2),
      rateLimitHits: 0,
      averageLatency: 0
    }
  });
});

app.get("/admin/health", (req, res) => {
  const healthy = session.initialized;
  res.status(healthy ? 200 : 503).json({
    healthy, mode: "direct",
    session: { initialized: session.initialized, messageCount: session.messages.length }
  });
});

app.get("/admin/clients", (req, res) => {
  res.json({
    clients: session.initialized ? [{
      id: session.userId?.substring(0, 8) || "session",
      status: "idle",
      userName: session.userName,
      messageCount: session.messages.length,
      chatId: session.chatId,
      features: session.features
    }] : []
  });
});

// Clear session history (replaces /admin/clients/:id/clear)
app.post("/admin/session/clear", authMiddleware, (req, res) => {
  session.messages = [];
  session.chatId = crypto.randomUUID();
  console.log("[Session] History cleared. New chatId:", session.chatId);
  res.json({ success: true, message: "Session history cleared", chatId: session.chatId });
});

// Compat route for old /admin/clients/:id/clear
app.post("/admin/clients/:id/clear", authMiddleware, (req, res) => {
  session.messages = [];
  session.chatId = crypto.randomUUID();
  res.json({ success: true, message: "History cleared" });
});

// No-op inject.js (direct mode doesn't need browser injection)
app.get("/inject.js", (req, res) => {
  res.type("application/json").send(JSON.stringify({ message: "Direct mode - no injection needed" }));
});

app.post("/stop", authMiddleware, (req, res) => {
  // In direct mode, no streaming abort support yet; just ack
  res.json({ success: true, message: "Stop acknowledged (direct mode)" });
});

// ============== START SERVER ==============

server.listen(config.server.port, config.server.host, async () => {
  console.log(`
╔═══════════════════════════════════════════════════════════════╗
║           Z.AI Direct Bridge Server Started                   ║
╠═══════════════════════════════════════════════════════════════╣
║  Mode:          DIRECT HTTP (no browser needed)               ║
║  Dashboard:     http://localhost:${config.server.port}                      ║
║  OpenAI API:    http://localhost:${config.server.port}/v1/chat/completions  ║
╠═══════════════════════════════════════════════════════════════╣
║  Auth Token:    ${config.auth.token}                            ║
╠═══════════════════════════════════════════════════════════════╣
║  Features:                                                    ║
║  ✓ Direct HTTPS to chat.z.ai (HMAC-signed)                   ║
║  ✓ Auto guest session init + JWT decode                       ║
║  ✓ OpenAI-compatible /v1/chat/completions                     ║
║  ✓ SSE Streaming support                                      ║
║  ✓ Tool call parsing (XML, JSON, Roo/Cline, OpenCode)         ║
║  ✓ Fresh session support (X-Fresh-Session header)             ║
║  ✓ Feature toggles (webSearch, thinking, imageGen, preview)   ║
║  ✓ Auto session re-init on 401                                ║
║  ✓ Conversation history tracking                              ║
╚═══════════════════════════════════════════════════════════════╝
`);

  // Initialize session at startup
  try {
    await initializeSession();
  } catch (e) {
    console.warn("[Startup] Session init deferred — will retry on first request.");
  }
});
