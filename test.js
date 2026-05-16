"use strict";

const express = require("express");
const http = require("http");
const crypto = require("crypto");
const config = require("./config");

const app = express();
const server = http.createServer(app);

// ============== Z.AI DIRECT CONFIG ==============

const BASE_URL = "https://chat.z.ai";
const INCLUDE_CORE_INSTRUCTIONS = false;
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
    persistHistory: false,
  },
  initialized: false,
  initializing: false,
};

// ============== PER-SESSION CONVERSATION STATE ==============

const sessions = new Map(); // sessionId -> { chatId, messages, lastUsed }
const SESSION_TTL = 30 * 60 * 1000; // 30 minutes

function getOrCreateSession(req) {
  const sessionId = req.headers["x-session-id"] || "default";
  const fresh = req.headers["x-fresh-session"] === "true";

  if (fresh || !sessions.has(sessionId)) {
    sessions.set(sessionId, {
      chatId: crypto.randomUUID(),
      messages: [],
      lastUsed: Date.now(),
    });
  }

  const s = sessions.get(sessionId);
  s.lastUsed = Date.now();
  return s;
}

// Periodically clean up stale sessions
setInterval(() => {
  const now = Date.now();
  for (const [id, s] of sessions) {
    if (now - s.lastUsed > SESSION_TTL) sessions.delete(id);
  }
}, 5 * 60 * 1000);

// ============== MIDDLEWARE ==============

app.use((req, res, next) => {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS");
  res.header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-Id, X-Fresh-Session, anthropic-version, anthropic-beta");
  if (req.method === "OPTIONS") return res.sendStatus(200);
  next();
});

app.use(express.json({ limit: "50mb" }));

function authMiddleware(req, res, next) {
  if (!config.auth.enabled) return next();
  const authHeader = req.headers.authorization;
  const token = authHeader?.replace(/^Bearer\s+/i, "").replace(/^x-api-key\s+/i, "");
  const apiKey = req.headers["x-api-key"];
  const provided = token || apiKey;
  if (provided !== config.auth.token) {
    return res.status(401).json({
      type: "error",
      error: { type: "authentication_error", message: "Invalid or missing authentication token" }
    });
  }
  next();
}

const CORE_INSTRUCTIONS = `CRITICAL INSTRUCTIONS (ALWAYS FOLLOW):
1. When using tools, ALWAYS output tool calls in XML format like: <tool_call<function=name><parameter=key>value</parameter></function></tool_call
   NEVER use JSON or markdown code blocks for tool calls.
2. Follow every instruction in the prompt deeply and thoroughly. Execute tasks completely.
3. Also One rule only applies if my prompt includes this string "You are Roo" if it does then when changing modes with function switch_mode function toolcall dont ever forget parameter mode_slug or else it will likely fail and heres the tool usage for making todo list use it when doing complex task etc
 <update_todo_list>
<todos>
- [ ] Create a .test file with content "im good"
- [ ] Read C:\\key.txt and display its contents
</todos>
</update_todo_list> this is only a example of tool usage if my Prompt doesnt includes You are Roo then ignore This rule
4. When using attempt_completion, ALWAYS use <parameter=result> - NEVER use <parameter=message> or <parameter=summary>. The parameter MUST be named "result".`;

// ============================================================
// ── TOOL COMPATIBILITY LAYER ─────────────────────────────────
// Z.AI does not support the tools/tool_choice fields natively.
// We convert them into a prompt-engineering system message that
// instructs the model to emit a parseable XML tool-call block
// when it wants to invoke a function.  The response is then
// intercepted and re-serialised into proper tool_use blocks
// before being returned to the caller.
// ============================================================

/**
 * Convert an Anthropic-format tool definition array → OAI-format tool array.
 * Anthropic uses  { name, description, input_schema }
 * OAI uses        { type:"function", function:{ name, description, parameters } }
 */
function anthropicToolsToOAI(tools) {
  if (!Array.isArray(tools)) return [];
  return tools.map(t => ({
    type: "function",
    function: {
      name: t.name,
      description: t.description || "",
      parameters: t.input_schema || { type: "object", properties: {} },
    },
  }));
}

/**
 * Build the tool-shim system message that is prepended to the message array
 * sent to Z.AI.  The model is told to respond with a specific XML envelope
 * whenever it decides to call a tool so we can parse it back out.
 *
 * Format the model must produce:
 *
 *   <tool_invoke>
 *   <tool_name>get_weather</tool_name>
 *   <tool_input>{"location":"Beijing","unit":"celsius"}</tool_input>
 *   </tool_invoke>
 *
 * Nothing else should appear on that turn when a tool is being called.
 */
function buildToolShimMessage(oaiTools, toolChoice) {
  const toolDefs = oaiTools.map(t => {
    const fn = t.function;
    return [
      `### Tool: ${fn.name}`,
      `Description: ${fn.description || "(no description)"}`,
      `Parameters (JSON Schema):`,
      JSON.stringify(fn.parameters, null, 2),
    ].join("\n");
  }).join("\n\n");

  // Resolve tool_choice to a hint string.
  // Handles all four formats:
  //   OAI string:   "auto" | "required" | "none"
  //   OAI object:   { type:"function", function:{ name:"..." } }
  //   Anthropic:    { type:"auto" } | { type:"any" } | { type:"none" } | { type:"tool", name:"..." }
  let choiceHint = "";
  if (toolChoice) {
    // Normalise to a simple string key for easier comparison
    let tcType = "";
    let tcName = "";

    if (typeof toolChoice === "string") {
      tcType = toolChoice.toLowerCase();  // "auto" | "required" | "none"
    } else if (typeof toolChoice === "object") {
      tcType = (toolChoice.type || "").toLowerCase();
      // OAI forced: { type:"function", function:{ name } }
      // Anthropic forced: { type:"tool", name }
      tcName = toolChoice.function?.name || toolChoice.name || "";
    }

    if (tcType === "none") {
      choiceHint = "\nDo NOT call any tools. Respond in plain text only.";
    } else if (tcType === "required" || tcType === "any") {
      // OAI "required" == Anthropic "any" — model MUST call a tool
      choiceHint = "\nYou MUST call one of the tools above. Do not respond in plain text — only output the <tool_invoke> XML block.";
    } else if ((tcType === "function" || tcType === "tool") && tcName) {
      // Forced specific tool
      choiceHint = `\nYou MUST call the tool named "${tcName}". Do not respond in plain text — only output the <tool_invoke> XML block for that tool.`;
    }
    // "auto" / "" → no hint, model decides
  }

  const shimText = `\
You have access to the following tools. When you need to call a tool, respond ONLY with the XML block below and nothing else on that turn. Do not include any explanation, preamble, or follow-up text alongside the XML — the caller will send you the result and you can then continue.

TOOL CALL FORMAT (use exactly this XML, no markdown fences):
<tool_invoke>
<tool_name>TOOL_NAME_HERE</tool_name>
<tool_input>{"param1":"value1","param2":"value2"}</tool_input>
</tool_invoke>

Rules:
- tool_input MUST be valid JSON.
- Only call one tool per response.
- If no tool is needed, respond normally in plain text — do NOT emit the XML block.
- Never invent tool names. Only use tools listed below.${choiceHint}

AVAILABLE TOOLS:
${toolDefs}`;

  return { role: "system", content: shimText };
}

/**
 * Detect whether a completed response contains a tool invocation shim block.
 */
function hasToolInvokeBlock(content) {
  return /<tool_invoke>[\s\S]*?<\/tool_invoke>/i.test(content);
}

/**
 * Parse <tool_invoke> blocks out of a response string.
 * Returns an array of { name, input } objects (input is a parsed JS object).
 */
function parseToolInvokeBlocks(content) {
  const results = [];
  const re = /<tool_invoke>([\s\S]*?)<\/tool_invoke>/gi;
  let m;
  while ((m = re.exec(content)) !== null) {
    const inner = m[1];
    const nameMatch  = /<tool_name>\s*([\s\S]*?)\s*<\/tool_name>/i.exec(inner);
    const inputMatch = /<tool_input>([\s\S]*?)<\/tool_input>/i.exec(inner);
    if (!nameMatch) continue;
    const name = nameMatch[1].trim();
    let input = {};
    if (inputMatch) {
      try { input = JSON.parse(inputMatch[1].trim()); }
      catch (e) { input = { raw: inputMatch[1].trim() }; }
    }
    results.push({ name, input });
  }
  return results;
}

/**
 * Strip <tool_invoke> blocks from a response string (leaves surrounding text).
 */
function removeToolInvokeBlocks(content) {
  return content.replace(/<tool_invoke>[\s\S]*?<\/tool_invoke>/gi, "").trim();
}

/**
 * Convert parsed tool-invoke blocks → Anthropic tool_use content blocks.
 */
function toolInvokesToAnthropicBlocks(invocations) {
  return invocations.map(inv => ({
    type: "tool_use",
    id: `toolu_${generateId().substring(0, 24)}`,
    name: inv.name,
    input: inv.input,
  }));
}

/**
 * Convert parsed tool-invoke blocks → OAI tool_calls array.
 */
function toolInvokesToOAIToolCalls(invocations) {
  return invocations.map(inv => ({
    id: `call_${generateId().substring(0, 24)}`,
    type: "function",
    function: {
      name: inv.name,
      arguments: JSON.stringify(inv.input),
    },
  }));
}

/**
 * Inject the tool shim into an OAI-format messages array.
 * The shim is inserted as a system message at position 0
 * (or merged with an existing system message at position 0).
 */
function injectToolShimIntoMessages(messages, oaiTools, toolChoice) {
  if (!oaiTools || oaiTools.length === 0) return messages;
  const shimMsg = buildToolShimMessage(oaiTools, toolChoice);
  const result = [...messages];
  if (result.length > 0 && result[0].role === "system") {
    // Merge with existing system message
    result[0] = {
      ...result[0],
      content: shimMsg.content + "\n\n---\n\n" + result[0].content,
    };
  } else {
    result.unshift(shimMsg);
  }
  return result;
}

// ============================================================
// ── END TOOL COMPATIBILITY LAYER ─────────────────────────────
// ============================================================

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
      .filter(part => part.type === "text" || part.type === "tool_result" || typeof part === "string")
      .map(part => {
        if (typeof part === "string") return part;
        if (part.type === "tool_result") {
          const inner = Array.isArray(part.content)
            ? part.content.map(c => c.text || "").join("\n")
            : (part.content || "");
          return `Tool Result (${part.tool_use_id}): ${inner}`;
        }
        return part.text || "";
      })
      .join("\n");
  }
  return String(content);
}

function anthropicMessagesToPrompt(messages, systemPrompt, includeToolInstructions = true) {
  let prompt = "";

  if (includeToolInstructions && INCLUDE_CORE_INSTRUCTIONS) {
    prompt += `${CORE_INSTRUCTIONS}\n\n`;
  }

  if (systemPrompt) {
    const sysText = typeof systemPrompt === "string"
      ? systemPrompt
      : Array.isArray(systemPrompt)
        ? systemPrompt.map(b => b.text || "").join("\n")
        : String(systemPrompt);
    prompt += `${sysText}\n\n`;
  }

  for (const msg of messages) {
    const content = msg.content;

    if (Array.isArray(content)) {
      const textParts = [];
      for (const part of content) {
        if (part.type === "text") {
          textParts.push(part.text);
        } else if (part.type === "tool_result") {
          const inner = Array.isArray(part.content)
            ? part.content.map(c => c.text || "").join("\n")
            : (part.content || "");
          textParts.push(`[Tool Result for ${part.tool_use_id}]: ${inner}`);
        } else if (part.type === "tool_use") {
          textParts.push(`[Tool Call: ${part.name}(${JSON.stringify(part.input)})]`);
        } else if (part.text) {
          textParts.push(part.text);
        }
      }
      prompt += `${textParts.join("\n")}\n\n`;
    } else {
      prompt += `${getMessageContent(content)}\n\n`;
    }
  }

  return prompt.trim();
}

function anthropicToOpenAIMessages(messages, system) {
  const result = [];

  if (system) {
    const sysText = typeof system === "string"
      ? system
      : Array.isArray(system)
        ? system.map(b => b.text || "").join("\n")
        : String(system);
    result.push({ role: "system", content: sysText });
  }

  for (const msg of messages) {
    const { role, content } = msg;

    if (typeof content === "string") {
      result.push({ role, content });
      continue;
    }

    if (!Array.isArray(content)) {
      result.push({ role, content: String(content ?? "") });
      continue;
    }

    const textParts = [];
    const imageParts = [];
    const toolCalls = [];
    const toolResultBlocks = [];

    for (const block of content) {
      if (block.type === "text") {
        textParts.push(block.text);
      } else if (block.type === "tool_use") {
        toolCalls.push({
          id: block.id || `call_${generateId().substring(0, 24)}`,
          type: "function",
          function: {
            name: block.name,
            arguments: typeof block.input === "string"
              ? block.input
              : JSON.stringify(block.input)
          }
        });
      } else if (block.type === "tool_result") {
        const resultContent = typeof block.content === "string"
          ? block.content
          : Array.isArray(block.content)
            ? block.content.map(c => {
                if (typeof c === "string") return c;
                if (c.type === "text") return c.text || "";
                return JSON.stringify(c);
              }).join("\n")
            : String(block.content ?? "");

        toolResultBlocks.push({
          role: "tool",
          tool_call_id: block.tool_use_id,
          content: resultContent,
        });
      } else if (block.type === "thinking") {
        textParts.push(`[Thinking: ${block.thinking || ""}]`);
      } else if (block.type === "redacted_thinking") {
        textParts.push("[Redacted Thinking]");
      } else if (block.type === "image") {
        if (block.source?.type === "base64") {
          const dataUrl = `data:${block.source.media_type};base64,${block.source.data}`;
          imageParts.push({ type: "image_url", image_url: { url: dataUrl } });
        } else if (block.source?.type === "url") {
          imageParts.push({ type: "image_url", image_url: { url: block.source.url } });
        } else {
          textParts.push(`[Image: ${block.source?.type || "unknown"}]`);
        }
      } else if (block.type === "document") {
        if (block.title) textParts.push(`[Document: ${block.title}]`);
        else textParts.push(`[Document: ${block.source?.media_type || "unknown"}]`);
        if (block.context) textParts.push(block.context);
      } else if (block.text) {
        textParts.push(block.text);
      }
    }

    const msgObj = { role };

    if (imageParts.length > 0) {
      const contentArr = [];
      if (textParts.length > 0) {
        contentArr.push({ type: "text", text: textParts.join("\n") });
      }
      contentArr.push(...imageParts);
      msgObj.content = contentArr;
    } else if (textParts.length > 0) {
      msgObj.content = textParts.join("\n");
    } else if (toolCalls.length > 0) {
      msgObj.content = null;
    } else {
      msgObj.content = "";
    }

    if (toolCalls.length > 0) {
      msgObj.tool_calls = toolCalls;
    }

    result.push(msgObj);

    for (const tr of toolResultBlocks) {
      result.push(tr);
    }
  }

  return result;
}

function messagesToPrompt(messages, includeToolInstructions = true) {
  if (!Array.isArray(messages)) return String(messages);

  let prompt = "";
  if (includeToolInstructions && INCLUDE_CORE_INSTRUCTIONS) prompt += `${CORE_INSTRUCTIONS}\n\n`;

  for (const msg of messages) {
    const content = getMessageContent(msg.content);
    prompt += `${content}\n\n`;
  }

  return prompt.trim();
}

// ── Tool call parsers (unchanged) ──

function parseToolCalls(content) {
  const toolCalls = [];
  content = content.replace(/<tool_call<([a-zA-Z_][a-zA-Z0-9_]*)>/gi, "<$1>");
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

  const xmlPattern = /<tool_call\s*<function=([^>]+)>([\s\S]*?)<\/function>\s*<\/tool_call>/gi;
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

  const jsonPattern = /<tool_call\s*(\{[\s\S]*?\})\s*<\/tool_call>/gi;
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
  cleaned = cleaned.replace(/<tool_call<([a-zA-Z_][a-zA-Z0-9_]*)>[\s\S]*?<\/\1>/gi, "");
  cleaned = cleaned.replace(/<tool_call<([a-zA-Z_][a-zA-Z0-9_]*)>[\s\S]*?(?=<tool_call|$)/gi, "");
  cleaned = cleaned.replace(/<tool_call[\s\S]*?<\/tool_call>/gi, "");

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
    /<tool_call(?![\s\S]*<\/tool_call>)/i,
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
    /<tool_call<[A-Za-z]+>(?![\s\S]*<\/[A-Za-z]+>)/,
    // Also buffer incomplete tool_invoke blocks (from our shim)
    /<tool_invoke>(?![\s\S]*<\/tool_invoke>)/i,
    /<tool_name>(?![\s\S]*<\/tool_name>)/i,
    /<tool_input>(?![\s\S]*<\/tool_input>)/i,
  ];
  for (const p of patterns) if (p.test(content)) return true;
  return false;
}

// ============================================================
// ── FORMAT HELPERS ──────────────────────────────────────────
// ============================================================

function toolCallsToAnthropicBlocks(toolCalls) {
  return toolCalls.map(tc => ({
    type: "tool_use",
    id: tc.id || `toolu_${generateId().substring(0, 24)}`,
    name: tc.function.name,
    input: (() => {
      try { return JSON.parse(tc.function.arguments); }
      catch (e) { return { raw: tc.function.arguments }; }
    })()
  }));
}

function formatAnthropicResponse(fullContent, model, requestId, shimToolInvocations = []) {
  const msgId = `msg_${requestId}`;

  // Prefer shim-parsed tool invocations if present, else fall back to legacy parser
  let toolBlocks = [];
  let cleanText = fullContent;

  if (shimToolInvocations.length > 0) {
    toolBlocks = toolInvokesToAnthropicBlocks(shimToolInvocations);
    cleanText = removeToolInvokeBlocks(fullContent);
  } else if (config.parseTool) {
    const toolCalls = parseToolCalls(fullContent);
    if (toolCalls.length > 0) {
      toolBlocks = toolCallsToAnthropicBlocks(toolCalls);
      cleanText = removeToolCallsFromContent(fullContent);
    }
  }

  const contentBlocks = [];
  if (cleanText && cleanText.trim()) {
    contentBlocks.push({ type: "text", text: cleanText });
  }
  contentBlocks.push(...toolBlocks);
  if (contentBlocks.length === 0) {
    contentBlocks.push({ type: "text", text: "" });
  }

  return {
    id: msgId,
    type: "message",
    role: "assistant",
    model: model || "claude-sonnet-4-6",
    content: contentBlocks,
    stop_reason: toolBlocks.length > 0 ? "tool_use" : "end_turn",
    stop_sequence: null,
    usage: {
      input_tokens: estimateTokens(fullContent),
      output_tokens: estimateTokens(fullContent),
    }
  };
}

function sseEvent(event, data) {
  return `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
}

function formatOpenAIResponse(result, model, requestId, stream = false, fullContent = null, shimToolInvocations = []) {
  const timestamp = Math.floor(Date.now() / 1000);
  const rawContent = result.content || result.text || "";

  if (stream) {
    if (result.finish_reason !== "stop") {
      return {
        id: `chatcmpl-${requestId}`,
        object: "chat.completion.chunk",
        created: timestamp,
        model: model || "glm-5",
        choices: [{ index: 0, delta: { content: rawContent }, finish_reason: null }]
      };
    }

    const contentToCheck = fullContent || rawContent;

    // Prefer shim invocations, then legacy parser
    let toolCalls = [];
    if (shimToolInvocations && shimToolInvocations.length > 0) {
      toolCalls = toolInvokesToOAIToolCalls(shimToolInvocations);
    } else if (config.parseTool) {
      toolCalls = parseToolCalls(contentToCheck);
    }

    if (toolCalls.length > 0) {
      return {
        id: `chatcmpl-${requestId}`,
        object: "chat.completion.chunk",
        created: timestamp,
        model: model || "glm-5",
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
      model: model || "glm-5",
      choices: [{ index: 0, delta: { content: rawContent }, finish_reason: "stop" }]
    };
  }

  // Non-streaming
  let toolCalls = [];
  let cleanContent = rawContent;

  if (shimToolInvocations && shimToolInvocations.length > 0) {
    toolCalls = toolInvokesToOAIToolCalls(shimToolInvocations);
    cleanContent = removeToolInvokeBlocks(rawContent);
  } else if (config.parseTool) {
    toolCalls = parseToolCalls(rawContent);
    cleanContent = toolCalls.length > 0 ? removeToolCallsFromContent(rawContent) : rawContent;
  }

  return {
    id: `chatcmpl-${requestId}`,
    object: "chat.completion",
    created: timestamp,
    model: model || "glm-5",
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

function formatAnthropicError(message, type = "api_error") {
  return { type: "error", error: { type, message } };
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

    await fetch(`${BASE_URL}/api/v1/auths/guest`, {
      method: "POST", headers, body: "{}", signal: AbortSignal.timeout(15000)
    });

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

async function* sendToZAI(prompt, options = {}) {
  const {
    model = "glm-5",
    webSearch = session.features.webSearch,
    thinking = session.features.thinking,
    imageGen = session.features.imageGen,
    previewMode = session.features.previewMode,
    chatId = session.chatId,
    messages = session.messages,
    clientMessages = null,
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

  const forwardedMessages = clientMessages
    ? clientMessages
    : [...messages, { role: "user", content: prompt }];

  const requestBody = {
    model,
    chat_id: chatId,
    messages: forwardedMessages,
    signature_prompt: prompt,
    stream: true,
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
  };

  const body = JSON.stringify(requestBody);

  if (config.logging.level === "debug") {
    console.log("[DEBUG] Z.AI request body:", body);
  }

  let res;
  try {
    res = await fetch(url, { method: "POST", headers, body, signal: AbortSignal.timeout(90000) });
  } catch (e) {
    throw new Error(`Z.AI connection error: ${e.message}`);
  }

  if (res.status === 401) {
    session.initialized = false;
    await initializeSession();
    yield* sendToZAI(prompt, options);
    return;
  }

  if (!res.ok) {
    const errText = await res.text().catch(() => "");
    throw new Error(`Z.AI error ${res.status}: ${errText}`);
  }

  const decoder = new TextDecoder();
  let buffer = "";

  for await (const chunk of res.body) {
    buffer += decoder.decode(chunk, { stream: true });
    const lines = buffer.split("\n");
    buffer = lines.pop();

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
    .badges { display: flex; gap: 8px; justify-content: center; margin-top: 12px; flex-wrap: wrap; }
    .badge {
      display: inline-block; padding: 4px 12px; border-radius: 12px;
      font-size: 0.8rem; font-weight: 700;
    }
    .badge-green { background: #22c55e; color: #000; }
    .badge-blue  { background: #3b82f6; color: #fff; }
    .badge-purple{ background: #a855f7; color: #fff; }
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
    .section-label {
      font-size: 0.75rem; font-weight: 700; text-transform: uppercase;
      letter-spacing: 0.1em; color: #a855f7; margin: 16px 0 8px;
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>Z.AI Direct Bridge</h1>
      <p>HTTP-only mode — No browser required</p>
      <div class="badges">
        <span class="badge badge-green">⚡ Direct Mode</span>
        <span class="badge badge-blue">OpenAI Compatible</span>
        <span class="badge badge-purple">Anthropic Compatible</span>
      </div>
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
          <div class="stat"><div class="label">Web Search</div><div class="value" id="featSearch">-</div></div>
          <div class="stat"><div class="label">Thinking</div><div class="value" id="featThink">-</div></div>
          <div class="stat"><div class="label">Image Gen</div><div class="value" id="featImage">-</div></div>
          <div class="stat"><div class="label">Preview</div><div class="value" id="featPreview">-</div></div>
        </div>
      </div>

      <div class="card" style="grid-column: span 2;">
        <h2>API Endpoints</h2>

        <div class="section-label">Anthropic-Compatible (for Claude Code)</div>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/v1/messages</span>
          <div class="desc">Native Anthropic Messages API — streaming SSE + tool_use blocks. Use ANTHROPIC_BASE_URL=http://${host}</div>
        </div>
        <div class="endpoint">
          <span class="method get">GET</span>
          <span class="path">/v1/models</span>
          <div class="desc">Model list (returns Anthropic-style model IDs)</div>
        </div>

        <div class="section-label">OpenAI-Compatible</div>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/v1/chat/completions</span>
          <div class="desc">OpenAI-compatible chat endpoint. Supports streaming.</div>
        </div>

        <div class="section-label">Management</div>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/features</span>
          <div class="desc">Toggle webSearch, thinking, imageGen, previewMode, persistHistory</div>
        </div>
        <div class="endpoint">
          <span class="method post">POST</span>
          <span class="path">/admin/session/clear</span>
          <div class="desc">Clear conversation history</div>
        </div>
      </div>

      <div class="card" style="grid-column: span 2;">
        <h2>Claude Code Setup (no LiteLLM needed)</h2>
        <div class="code-block">
          <code># Windows PowerShell
$env:ANTHROPIC_BASE_URL="http://localhost:${config.server.port}"
$env:ANTHROPIC_AUTH_TOKEN="${config.auth.token}"
$env:ANTHROPIC_API_KEY=""
claude

# Windows CMD
set ANTHROPIC_BASE_URL=http://localhost:${config.server.port}
set ANTHROPIC_AUTH_TOKEN=${config.auth.token}
set ANTHROPIC_API_KEY=""
claude

# Permanent — add to ~/.claude/settings.json:
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://localhost:${config.server.port}",
    "ANTHROPIC_AUTH_TOKEN": "${config.auth.token}",
    "ANTHROPIC_API_KEY": ""
  }
}</code>
        </div>

        <h2 style="margin-top:20px">Test the Anthropic endpoint</h2>
        <div class="code-block">
          <code># Non-streaming
curl -X POST http://${host}/v1/messages \\
  -H "Content-Type: application/json" \\
  -H "x-api-key: ${config.auth.token}" \\
  -H "anthropic-version: 2023-06-01" \\
  -d '{"model":"claude-sonnet-4-6","max_tokens":100,"messages":[{"role":"user","content":"Hello!"}]}'

# Streaming
curl -X POST http://${host}/v1/messages \\
  -H "Content-Type: application/json" \\
  -H "x-api-key: ${config.auth.token}" \\
  -H "anthropic-version: 2023-06-01" \\
  -d '{"model":"claude-sonnet-4-6","max_tokens":500,"stream":true,"messages":[{"role":"user","content":"Say hi"}]}'</code>
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
    activeSessions: sessions.size,
    features: session.features,
    parseTool: config.parseTool,
    mode: "direct"
  });
});

// ============================================================
// ── ANTHROPIC-COMPATIBLE /v1/messages ───────────────────────
// ============================================================

app.post("/v1/messages", authMiddleware, async (req, res) => {
  const {
    model = "claude-sonnet-4-6",
    messages,
    system,
    stream = false,
    tools: anthropicTools,       // Anthropic tool definitions (may be present)
    tool_choice: toolChoice,     // Anthropic tool_choice (may be present)
  } = req.body;

  if (!messages || !Array.isArray(messages)) {
    return res.status(400).json(formatAnthropicError("messages is required and must be an array", "invalid_request_error"));
  }

  const reqSession = getOrCreateSession(req);
  const requestId = generateId();

  // ── Tool shim: convert Anthropic tools → OAI, then inject as system message ──
  const oaiTools = anthropicTools ? anthropicToolsToOAI(anthropicTools) : [];
  const hasTools = oaiTools.length > 0;

  // Build OAI messages (lossless conversion)
  let openaiMessages = anthropicToOpenAIMessages(messages, system);

  // Inject tool shim system message if tools are present
  if (hasTools) {
    openaiMessages = injectToolShimIntoMessages(openaiMessages, oaiTools, toolChoice);
  }

  // Flatten to prompt string for HMAC signing only
  const prompt = anthropicMessagesToPrompt(messages, system);
  const inputTokens = estimateTokens(prompt);

  const zaiModel = (() => {
    const m = (model || "").toLowerCase();
    if (m.includes("opus"))   return "GLM-5-Turbo";
    if (m.includes("haiku"))  return "glm-5";
    return "glm-5";
  })();

  const opts = {
    model: zaiModel,
    webSearch: session.features.webSearch,
    thinking: session.features.thinking,
    imageGen: session.features.imageGen,
    previewMode: session.features.previewMode,
    chatId: reqSession.chatId,
    messages: reqSession.messages,
    clientMessages: openaiMessages,
  };

  // ── STREAMING ──
  if (stream) {
    res.setHeader("Content-Type", "text/event-stream");
    res.setHeader("Cache-Control", "no-cache");
    res.setHeader("Connection", "keep-alive");
    res.setHeader("X-Accel-Buffering", "no");
    res.flushHeaders();

    const msgId = `msg_${requestId}`;

    res.write(sseEvent("message_start", {
      type: "message_start",
      message: {
        id: msgId,
        type: "message",
        role: "assistant",
        model: model,
        content: [],
        stop_reason: null,
        stop_sequence: null,
        usage: { input_tokens: inputTokens, output_tokens: 0 }
      }
    }));

    const keepAlive = setInterval(() => {
      try { res.write(": ping\n\n"); } catch (e) { clearInterval(keepAlive); }
    }, 5000);

    let fullContent = "";
    let sentContent = "";
    let textBlockOpen = false;
    let textBlockIndex = 0;

    try {
      for await (const chunk of sendToZAI(prompt, opts)) {
        fullContent += chunk;

        // Buffer while any tool call (shim or legacy) is still being assembled
        if (hasTools && hasIncompleteToolCall(fullContent)) continue;
        if (config.parseTool && hasIncompleteToolCall(fullContent)) continue;

        const delta = fullContent.substring(sentContent.length);
        if (delta) {
          if (!textBlockOpen) {
            res.write(sseEvent("content_block_start", {
              type: "content_block_start",
              index: textBlockIndex,
              content_block: { type: "text", text: "" }
            }));
            textBlockOpen = true;
          }
          res.write(sseEvent("content_block_delta", {
            type: "content_block_delta",
            index: textBlockIndex,
            delta: { type: "text_delta", text: delta }
          }));
          sentContent = fullContent;
        }
      }

      // Flush remaining
      const remaining = fullContent.substring(sentContent.length);
      if (remaining) {
        if (!textBlockOpen) {
          res.write(sseEvent("content_block_start", {
            type: "content_block_start",
            index: textBlockIndex,
            content_block: { type: "text", text: "" }
          }));
          textBlockOpen = true;
        }
        res.write(sseEvent("content_block_delta", {
          type: "content_block_delta",
          index: textBlockIndex,
          delta: { type: "text_delta", text: remaining }
        }));
      }

      // Close text block
      if (textBlockOpen) {
        res.write(sseEvent("content_block_stop", {
          type: "content_block_stop",
          index: textBlockIndex
        }));
        textBlockOpen = false;
      }

      // ── Resolve tool invocations ──
      // Prefer shim-format blocks; fall back to legacy parseToolCalls
      let shimInvocations = [];
      let legacyToolCalls = [];

      if (hasTools && hasToolInvokeBlock(fullContent)) {
        shimInvocations = parseToolInvokeBlocks(fullContent);
      } else if (config.parseTool) {
        legacyToolCalls = parseToolCalls(fullContent);
      }

      const anthropicToolBlocks = shimInvocations.length > 0
        ? toolInvokesToAnthropicBlocks(shimInvocations)
        : toolCallsToAnthropicBlocks(legacyToolCalls);

      let blockIdx = textBlockIndex + 1;
      for (const tc of anthropicToolBlocks) {
        const inputJson = JSON.stringify(tc.input);
        res.write(sseEvent("content_block_start", {
          type: "content_block_start",
          index: blockIdx,
          content_block: { type: "tool_use", id: tc.id, name: tc.name, input: {} }
        }));
        res.write(sseEvent("content_block_delta", {
          type: "content_block_delta",
          index: blockIdx,
          delta: { type: "input_json_delta", partial_json: inputJson }
        }));
        res.write(sseEvent("content_block_stop", {
          type: "content_block_stop",
          index: blockIdx
        }));
        blockIdx++;
      }

      const outputTokens = estimateTokens(fullContent);
      const stopReason = anthropicToolBlocks.length > 0 ? "tool_use" : "end_turn";

      res.write(sseEvent("message_delta", {
        type: "message_delta",
        delta: { stop_reason: stopReason, stop_sequence: null },
        usage: { output_tokens: outputTokens }
      }));

      res.write(sseEvent("message_stop", { type: "message_stop" }));
      res.write(`data: [DONE]\n\n`);

      if (session.features.persistHistory) {
        reqSession.messages.push({ role: "user", content: prompt });
        if (fullContent) reqSession.messages.push({ role: "assistant", content: fullContent });
      }

    } catch (e) {
      console.error("[Anthropic Stream] Error:", e.message);
      res.write(sseEvent("error", { type: "error", error: { type: "api_error", message: e.message } }));
      res.write(`data: [DONE]\n\n`);
    } finally {
      clearInterval(keepAlive);
      res.end();
    }

  // ── NON-STREAMING ──
  } else {
    try {
      let fullContent = "";
      for await (const chunk of sendToZAI(prompt, opts)) {
        fullContent += chunk;
      }

      if (session.features.persistHistory) {
        reqSession.messages.push({ role: "user", content: prompt });
        if (fullContent) reqSession.messages.push({ role: "assistant", content: fullContent });
      }

      // Resolve tool invocations
      const shimInvocations = (hasTools && hasToolInvokeBlock(fullContent))
        ? parseToolInvokeBlocks(fullContent)
        : [];

      res.json(formatAnthropicResponse(fullContent, model, requestId, shimInvocations));
    } catch (e) {
      console.error("[Anthropic API] Error:", e.message);
      const statusCode = e.message.includes("401") ? 401 : 500;
      res.status(statusCode).json(formatAnthropicError(e.message));
    }
  }
});

// ============================================================
// ── OPENAI-COMPATIBLE /v1/chat/completions ──────────────────
// ============================================================

const knownModels = [
  "glm-4.7", "glm-5", "GLM-5-Turbo", "GLM-5v-Turbo", "GLM-5.1",
  "claude-sonnet-4-6", "claude-opus-4-6", "claude-haiku-4-5-20251001",
];

app.get("/v1/models", authMiddleware, (req, res) => {
  res.json({
    object: "list",
    data: knownModels.map(m => ({
      id: m,
      object: "model",
      created: Math.floor(Date.now() / 1000),
      owned_by: m.startsWith("claude") ? "anthropic" : "z-ai",
      display_name: m,
    }))
  });
});

app.get("/models", authMiddleware, (req, res) => {
  res.json({ models: knownModels, currentModel: "glm-5" });
});

app.post("/v1/chat/completions", authMiddleware, async (req, res) => {
  const {
    model = "glm-5",
    messages,
    stream = true,
    deepThink,
    search,
    webSearch,
    tools: oaiToolsRaw,      // OAI tool definitions (may be present)
    tool_choice: toolChoice, // OAI tool_choice (may be present)
  } = req.body;

  if (!messages || !Array.isArray(messages)) {
    return res.status(400).json(formatOpenAIError("messages is required and must be an array", "invalid_request_error"));
  }

  const reqSession = getOrCreateSession(req);
  const requestId = generateId();

  // ── Tool shim: inject into messages if tools are provided ──
  const oaiTools = Array.isArray(oaiToolsRaw) ? oaiToolsRaw : [];
  const hasTools = oaiTools.length > 0;

  // Inject shim into messages if tools present; otherwise forward as-is
  const clientMessages = hasTools
    ? injectToolShimIntoMessages(messages, oaiTools, toolChoice)
    : messages;

  const prompt = messagesToPrompt(messages);

  const opts = {
    model,
    webSearch: webSearch ?? search ?? session.features.webSearch,
    thinking: deepThink ?? session.features.thinking,
    imageGen: session.features.imageGen,
    previewMode: session.features.previewMode,
    chatId: reqSession.chatId,
    messages: reqSession.messages,
    clientMessages,
  };

  if (stream) {
    res.setHeader("Content-Type", "text/event-stream");
    res.setHeader("Cache-Control", "no-cache");
    res.setHeader("Connection", "keep-alive");
    res.setHeader("X-Accel-Buffering", "no");
    res.flushHeaders();

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
        // Buffer while any tool call block is still being assembled
        if ((hasTools || config.parseTool) && hasIncompleteToolCall(fullContent)) continue;
        const delta = fullContent.substring(sentContent.length);
        if (delta) {
          sentContent = fullContent;
          const c = formatOpenAIResponse({ content: delta, finish_reason: null }, model, requestId, true);
          res.write(`data: ${JSON.stringify(c)}\n\n`);
        }
      }

      const remaining = fullContent.substring(sentContent.length);
      if (remaining) {
        const c = formatOpenAIResponse({ content: remaining, finish_reason: null }, model, requestId, true);
        res.write(`data: ${JSON.stringify(c)}\n\n`);
      }

      // Resolve tool invocations for final chunk
      const shimInvocations = (hasTools && hasToolInvokeBlock(fullContent))
        ? parseToolInvokeBlocks(fullContent)
        : [];

      const finalChunk = formatOpenAIResponse(
        { content: "", finish_reason: "stop" },
        model, requestId, true, fullContent, shimInvocations
      );
      res.write(`data: ${JSON.stringify(finalChunk)}\n\n`);
      res.write("data: [DONE]\n\n");

      if (session.features.persistHistory) {
        reqSession.messages.push({ role: "user", content: prompt });
        if (fullContent) reqSession.messages.push({ role: "assistant", content: fullContent });
      }

    } catch (e) {
      console.error("[Stream] Error:", e.message);
      res.write(`data: ${JSON.stringify({ error: { message: e.message } })}\n\n`);
      res.write("data: [DONE]\n\n");
    } finally {
      clearInterval(keepAlive);
      res.end();
    }

  } else {
    try {
      let fullContent = "";
      for await (const chunk of sendToZAI(prompt, opts)) {
        fullContent += chunk;
      }

      if (session.features.persistHistory) {
        reqSession.messages.push({ role: "user", content: prompt });
        if (fullContent) reqSession.messages.push({ role: "assistant", content: fullContent });
      }

      const shimInvocations = (hasTools && hasToolInvokeBlock(fullContent))
        ? parseToolInvokeBlocks(fullContent)
        : [];

      res.json(formatOpenAIResponse({ content: fullContent }, model, requestId, false, null, shimInvocations));
    } catch (e) {
      console.error("[API] Error:", e.message);
      const statusCode = e.message.includes("401") ? 401 : 500;
      res.status(statusCode).json(formatOpenAIError(e.message));
    }
  }
});

// ============== LEGACY + ADMIN ROUTES ==============

app.post("/prompt", authMiddleware, async (req, res) => {
  const { prompt, search, deepThink, webSearch } = req.body;
  if (!prompt) return res.status(400).json({ error: "Prompt is required" });
  const reqSession = getOrCreateSession(req);
  try {
    let fullContent = "";
    for await (const chunk of sendToZAI(prompt, {
      webSearch: webSearch ?? search ?? session.features.webSearch,
      thinking: deepThink ?? session.features.thinking,
      chatId: reqSession.chatId,
      messages: reqSession.messages,
    })) { fullContent += chunk; }

    if (session.features.persistHistory) {
      reqSession.messages.push({ role: "user", content: prompt });
      if (fullContent) reqSession.messages.push({ role: "assistant", content: fullContent });
    }

    res.json({ success: true, response: fullContent });
  } catch (e) {
    console.error("[Prompt] Error:", e.message);
    res.status(500).json({ success: false, error: e.message });
  }
});

app.post("/features", authMiddleware, (req, res) => {
  const { webSearch, thinking, imageGen, previewMode, persistHistory } = req.body;
  if (webSearch     !== undefined) { session.features.webSearch = !!webSearch; session.features.autoWebSearch = !!webSearch; }
  if (thinking      !== undefined) session.features.thinking     = !!thinking;
  if (imageGen      !== undefined) session.features.imageGen     = !!imageGen;
  if (previewMode   !== undefined) session.features.previewMode  = !!previewMode;
  if (persistHistory !== undefined) session.features.persistHistory = !!persistHistory;
  console.log("[Features] Updated:", session.features);
  res.json({ success: true, features: session.features });
});

app.get("/admin/stats", (req, res) => {
  let totalMessages = 0;
  for (const s of sessions.values()) totalMessages += s.messages.length;
  res.json({
    mode: "direct",
    totalClients: session.initialized ? 1 : 0,
    activeSessions: sessions.size,
    stats: { totalRequests: Math.floor(totalMessages / 2) }
  });
});

app.get("/admin/health", (req, res) => {
  const healthy = session.initialized;
  res.status(healthy ? 200 : 503).json({ healthy, mode: "direct" });
});

app.get("/admin/clients", (req, res) => {
  res.json({ clients: session.initialized ? [{ id: "session", status: "idle" }] : [] });
});

app.post("/admin/session/clear", authMiddleware, (req, res) => {
  sessions.clear();
  console.log("[Session] All session histories cleared.");
  res.json({ success: true, message: "All session histories cleared", activeSessions: 0 });
});

app.post("/admin/clients/:id/clear", authMiddleware, (req, res) => {
  sessions.clear();
  res.json({ success: true, message: "History cleared" });
});

app.get("/inject.js", (req, res) => {
  res.type("application/json").send(JSON.stringify({ message: "Direct mode" }));
});

app.post("/stop", authMiddleware, (req, res) => {
  res.json({ success: true, message: "Stop acknowledged" });
});

// ============== START SERVER ==============

server.listen(config.server.port, config.server.host, async () => {
  console.log(`
╔═══════════════════════════════════════════════════════════════╗
║           Z.AI Direct Bridge Server Started                   ║
╠═══════════════════════════════════════════════════════════════╣
║  Mode:          DIRECT HTTP (no browser needed)               ║
║  Dashboard:     http://localhost:${config.server.port}                      ║
╠═══════════════════════════════════════════════════════════════╣
║  Anthropic API: http://localhost:${config.server.port}/v1/messages          ║
║  OpenAI API:    http://localhost:${config.server.port}/v1/chat/completions  ║
╠═══════════════════════════════════════════════════════════════╣
║  Auth Token:    ${config.auth.token.padEnd(44)}║
╠═══════════════════════════════════════════════════════════════╣
║  Claude Code (no LiteLLM needed):                             ║
║  set ANTHROPIC_BASE_URL=http://localhost:${config.server.port}              ║
║  set ANTHROPIC_AUTH_TOKEN=${config.auth.token.padEnd(39)}║
║  set ANTHROPIC_API_KEY=""    ║ 
║ claude                                                                   ║
╚═══════════════════════════════════════════════════════════════╝
`);

  try {
    await initializeSession();
  } catch (e) {
    console.warn("[Startup] Session init deferred — will retry on first request.");
  }
});
