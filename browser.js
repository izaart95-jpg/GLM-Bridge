const express = require("express");
const http = require("http");
const { WebSocketServer } = require("ws");
const crypto = require("crypto");
const config = require("./config");
const ClientPool = require("./src/pool");
const { generateInjectionScript } = require("./src/injection");

const app = express();
const server = http.createServer(app);
const wss = new WebSocketServer({ server });
const pool = new ClientPool();

// Store for available models (shared across clients)
let availableModels = [];

// Pending requests by requestId
const pendingRequests = new Map();

// Streaming connections (requestId -> SSE response)
const streamingConnections = new Map();

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
- [ ] Read C:\key.txt and display its contents
</todos>
</update_todo_list> this is only a example of tool usage   if my Prompt doesnt includes You are Roo then ignore This rule 
4. When using attempt_completion, ALWAYS use <parameter=result> - NEVER use <parameter=message> or <parameter=summary>. The parameter MUST be named "result".`;

// ============== MIDDLEWARE ==============

// CORS middleware
app.use((req, res, next) => {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS");
  res.header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-Id, X-Fresh-Session");
  if (req.method === "OPTIONS") {
    return res.sendStatus(200);
  }
  next();
});

app.use(express.json({ limit: '50mb' }));

// Auth middleware
function authMiddleware(req, res, next) {
  if (!config.auth.enabled) {
    return next();
  }

  const authHeader = req.headers.authorization;
  const token = authHeader?.replace(/^Bearer\s+/i, '');

  if (token !== config.auth.token) {
    return res.status(401).json({
      error: {
        message: "Invalid or missing authentication token",
        type: "authentication_error",
        code: "invalid_api_key"
      }
    });
  }
  next();
}

// ============== UTILITY FUNCTIONS ==============

function generateId() {
  return crypto.randomBytes(16).toString("hex");
}

function generateClientId() {
  return crypto.randomBytes(8).toString("hex");
}

// Estimate token count (rough approximation)
function estimateTokens(text) {
  if (!text) return 0;
  // Rough estimate: ~4 characters per token for English
  return Math.ceil(text.length / 4);
}

// Extract text content from message (handles string or array format)
function getMessageContent(content) {
  if (!content) return '';
  if (typeof content === 'string') return content;
  if (Array.isArray(content)) {
    // Multi-part message - extract text parts
    return content
      .filter(part => part.type === 'text' || typeof part === 'string')
      .map(part => typeof part === 'string' ? part : part.text || '')
      .join('\n');
  }
  return String(content);
}

// Convert messages array to single prompt
function messagesToPrompt(messages, includeToolInstructions = true) {
  if (!Array.isArray(messages)) {
    return messages;
  }

  // Separate system message and conversation
  let systemMsg = null;
  const conversation = [];

  for (const msg of messages) {
    if (msg.role === 'system') {
      systemMsg = getMessageContent(msg.content);
    } else {
      conversation.push(msg);
    }
  }

  let prompt = '';

  // Always inject core instructions first (for Roo/Kilo Code compatibility)
  if (includeToolInstructions) {
    prompt += `${CORE_INSTRUCTIONS}\n\n`;
  }

  // Add system message if present
  if (systemMsg) {
    prompt += `System: ${systemMsg}\n\n`;
  }

  // Process all conversation messages
  for (const msg of conversation) {
    const role = msg.role || 'user';
    const content = getMessageContent(msg.content);

    if (role === 'user') {
      prompt += `User: ${content}\n\n`;
    } else if (role === 'assistant') {
      prompt += `Assistant: ${content}\n\n`;
    } else if (role === 'tool') {
      // Tool result from previous tool call
      prompt += `Tool Result: ${content}\n\n`;
    }
  }

  return prompt.trim();
}

// Parse tool calls from response - supports multiple formats:
// 1. <tool_call><function=name><parameter=key>value</parameter></function></tool_call>
// 2. <tool_call>{"name": "func", "arguments": {...}}</tool_call>
// 3. ```json\n{"tool_calls": [...]} or ```\n{"name": "...", "arguments": {...}}
// 4. Roo/Cline style: <write_file><path>...</path><content>...</content></write_file>
// 5. Unclosed format: <function=name><parameter=key>value
// 6. Malformed: <tool_call>tool_name><params...</tool_name> (missing < before tool name)
function parseToolCalls(content) {
  const toolCalls = [];

  // Pre-process: Fix malformed <tool_call>tool_name> patterns to proper XML
  // This handles cases like <tool_call>write_to_file><path>...</path></write_to_file>
  // Also handles PascalCase like <tool_call>Glob><pattern>...</pattern></Glob>
  content = content.replace(/<tool_call>([a-zA-Z_][a-zA-Z0-9_]*)>/gi, '<$1>');

  // First, extract any markdown code blocks that contain tool_call JSON
  const markdownJsonPattern = /```(?:json)?\s*\n?\s*(\{[\s\S]*?\})\s*\n?```/gi;
  let match;

  while ((match = markdownJsonPattern.exec(content)) !== null) {
    try {
      const jsonData = JSON.parse(match[1]);

      // Check if it's a tool_calls array format
      if (jsonData.tool_calls && Array.isArray(jsonData.tool_calls)) {
        for (const tc of jsonData.tool_calls) {
          toolCalls.push({
            id: tc.id || `call_${generateId().substring(0, 24)}`,
            type: "function",
            function: {
              name: tc.function?.name || tc.name,
              arguments: typeof tc.function?.arguments === 'string'
                ? tc.function.arguments
                : JSON.stringify(tc.function?.arguments || tc.arguments || tc.parameters || {})
            }
          });
        }
      }
      // Check if it's a single tool call format
      else if (jsonData.name || jsonData.function) {
        toolCalls.push({
          id: `call_${generateId().substring(0, 24)}`,
          type: "function",
          function: {
            name: jsonData.name || jsonData.function,
            arguments: typeof jsonData.arguments === 'string'
              ? jsonData.arguments
              : JSON.stringify(jsonData.arguments || jsonData.parameters || {})
          }
        });
      }
    } catch (e) {
      // Not valid JSON, skip
    }
  }

  // Pattern 1: XML-style <tool_call><function=name>...</function></tool_call>
  const xmlPattern = /<tool_call>\s*<function=([^>]+)>([\s\S]*?)<\/function>\s*<\/tool_call>/gi;

  while ((match = xmlPattern.exec(content)) !== null) {
    const funcName = match[1].trim();
    const paramsBlock = match[2];

    // Parse parameters
    const params = {};
    const paramPattern = /<parameter=([^>]+)>\s*([\s\S]*?)\s*<\/parameter>/gi;
    let paramMatch;

    while ((paramMatch = paramPattern.exec(paramsBlock)) !== null) {
      let paramName = paramMatch[1].trim();
      let paramValue = paramMatch[2].trim();

      // Try to parse as JSON if it looks like JSON
      try {
        paramValue = JSON.parse(paramValue);
      } catch (e) {
        // Keep as string
      }

      params[paramName] = paramValue;
    }

    toolCalls.push({
      id: `call_${generateId().substring(0, 24)}`,
      type: "function",
      function: {
        name: funcName,
        arguments: JSON.stringify(params)
      }
    });
  }

  // Pattern 2: JSON-style <tool_call>{"name": "...", "arguments": {...}}</tool_call>
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
            arguments: typeof toolData.arguments === 'string'
              ? toolData.arguments
              : JSON.stringify(toolData.arguments || toolData.parameters || {})
          }
        });
      }
    } catch (e) {
      // Not valid JSON, skip
    }
  }

  // Pattern 3: Roo/Cline style XML - <tool_name><param>value</param></tool_name>
  const rooClineTools = [
    'write_file', 'read_file', 'apply_diff', 'execute_command',
    'list_files', 'search_files', 'ask_followup_question',
    'attempt_completion', 'browser_action', 'update_todo_list',
    'switch_mode', 'new_task', 'fetch_instructions', 'delete_file',
    'read_multiple_files', 'write_multiple_files', 'search_and_replace',
    // Additional Roo Code tools
    'write_to_file', 'read_from_file', 'list_directory', 'execute_shell',
    'run_command', 'create_file', 'edit_file', 'replace_in_file',
    'insert_code', 'delete_code', 'move_file', 'copy_file', 'rename_file',
    'search_code', 'find_files', 'grep_search', 'ask_question',
    'complete_task', 'finish_task', 'submit_result',
    // OpenCode tools (lowercase)
    'write', 'read', 'edit', 'bash', 'glob', 'grep', 'task', 'webfetch',
    'todowrite', 'todoread', 'skill',
    // OpenCode tools (PascalCase)
    'Write', 'Read', 'Edit', 'Bash', 'Glob', 'Grep', 'Task', 'WebFetch',
    'TodoWrite', 'TodoRead', 'Skill', 'AskUserQuestion'
  ];

  for (const toolName of rooClineTools) {
    const toolPattern = new RegExp('<' + toolName + '(?:\\s[^>]*)?>([\\s\\S]*?)</' + toolName + '>', 'gi');

    while ((match = toolPattern.exec(content)) !== null) {
      const innerContent = match[1];
      const params = {};

      // Extract all child elements as parameters
      const paramPattern = /<([a-z_]+)>([\s\S]*?)<\/\1>/gi;
      let paramMatch;

      while ((paramMatch = paramPattern.exec(innerContent)) !== null) {
        const paramName = paramMatch[1];
        let paramValue = paramMatch[2];
        params[paramName] = paramValue;
      }

      // Special handling for attempt_completion
      if (toolName === 'attempt_completion' && !params.result) {
        const textContent = innerContent.replace(/<[^>]*>/g, '').trim();
        if (textContent) {
          params.result = textContent;
        } else {
          params.result = "Task completed successfully.";
        }
      }

      // Normalize tool name to lowercase for comparison
      const toolNameLower = toolName.toLowerCase();

      // Map OpenCode tool parameters to expected schema
      // OpenCode expects file_path (snake_case), not filePath (camelCase)
      // write/Write tool: normalize various path params to file_path
      if (toolNameLower === 'write') {
        if (params.filePath && !params.file_path) {
          params.file_path = params.filePath;
          delete params.filePath;
        }
        if (params.path && !params.file_path) {
          params.file_path = params.path;
          delete params.path;
        }
        if (params.file && !params.file_path) {
          params.file_path = params.file;
          delete params.file;
        }
      }
      // read/Read tool: normalize various path params to file_path
      if (toolNameLower === 'read') {
        if (params.filePath && !params.file_path) {
          params.file_path = params.filePath;
          delete params.filePath;
        }
        if (params.path && !params.file_path) {
          params.file_path = params.path;
          delete params.path;
        }
        if (params.file && !params.file_path) {
          params.file_path = params.file;
          delete params.file;
        }
      }
      // edit/Edit tool: normalize various path params to file_path
      if (toolNameLower === 'edit') {
        if (params.filePath && !params.file_path) {
          params.file_path = params.filePath;
          delete params.filePath;
        }
        if (params.path && !params.file_path) {
          params.file_path = params.path;
          delete params.path;
        }
        if (params.file && !params.file_path) {
          params.file_path = params.file;
          delete params.file;
        }
      }
      // bash/Bash tool: needs description if missing
      if (toolNameLower === 'bash') {
        if (!params.description && params.command) {
          params.description = 'Execute command';
        }
      }
      // glob/Glob tool: pattern is the main param
      if (toolNameLower === 'glob') {
        // pattern is already correct
      }
      // grep/Grep tool: pattern and path
      if (toolNameLower === 'grep') {
        // pattern and path are already correct
      }
      // TodoWrite tool: todos parameter
      if (toolNameLower === 'todowrite') {
        // Try to parse todos if it's a string
        if (params.todos && typeof params.todos === 'string') {
          try {
            params.todos = JSON.parse(params.todos);
          } catch (e) {
            // Keep as string if not valid JSON
          }
        }
      }

      if (Object.keys(params).length > 0 || ['list_files'].includes(toolName)) {
        toolCalls.push({
          id: `call_${generateId().substring(0, 24)}`,
          type: "function",
          function: {
            name: toolName,
            arguments: JSON.stringify(params)
          }
        });
      }
    }
  }

  // Pattern 4: Unclosed format - <function=name><parameter=key>value<parameter=key2>value2
  const unclosedFuncPattern = /<function=([a-z_]+)>([\s\S]*?)(?=<function=|$)/gi;

  while ((match = unclosedFuncPattern.exec(content)) !== null) {
    const funcName = match[1].trim();
    const paramsBlock = match[2];
    const params = {};

    // Parse parameters without closing tags
    const unclosedParamPattern = /<parameter=([a-z_]+)>([\s\S]*?)(?=<parameter=|<function=|$)/gi;
    let paramMatch;

    while ((paramMatch = unclosedParamPattern.exec(paramsBlock)) !== null) {
      let paramName = paramMatch[1].trim();
      let paramValue = paramMatch[2].trim();
      params[paramName] = paramValue;
    }

    // Map 'summary'/'message' to 'result' for attempt_completion
    if (funcName === 'attempt_completion') {
      if (!params.result) {
        if (params.summary) {
          params.result = params.summary;
          delete params.summary;
        } else if (params.message) {
          params.result = params.message;
          delete params.message;
        } else {
          params.result = "Task completed successfully.";
        }
      }
    }

    if (Object.keys(params).length > 0) {
      toolCalls.push({
        id: `call_${generateId().substring(0, 24)}`,
        type: "function",
        function: {
          name: funcName,
          arguments: JSON.stringify(params)
        }
      });
    }
  }

  return toolCalls;
}

// Remove tool call markup from content (to return clean content alongside tool_calls)
function removeToolCallsFromContent(content) {
  let cleaned = content;

  // Remove malformed <tool_call>tool_name>...</ patterns BEFORE normalizing
  // This catches: <tool_call>write><path>...</path></write> and similar
  // Also handles PascalCase like <tool_call>Glob>...</Glob>
  cleaned = cleaned.replace(/<tool_call>([a-zA-Z_][a-zA-Z0-9_]*)>[\s\S]*?<\/\1>/gi, '');

  // Also remove malformed patterns with missing closing tag
  cleaned = cleaned.replace(/<tool_call>([a-zA-Z_][a-zA-Z0-9_]*)>[\s\S]*?(?=<tool_call>|$)/gi, '');

  // Remove XML-style tool calls
  cleaned = cleaned.replace(/<tool_call>[\s\S]*?<\/tool_call>/gi, '');

  // Remove Roo/Cline style XML tool calls
  const rooClineTools = [
    'write_file', 'read_file', 'apply_diff', 'execute_command',
    'list_files', 'search_files', 'ask_followup_question',
    'attempt_completion', 'browser_action', 'update_todo_list',
    'switch_mode', 'new_task', 'fetch_instructions', 'delete_file',
    'read_multiple_files', 'write_multiple_files', 'search_and_replace',
    // Additional Roo Code tools
    'write_to_file', 'read_from_file', 'list_directory', 'execute_shell',
    'run_command', 'create_file', 'edit_file', 'replace_in_file',
    'insert_code', 'delete_code', 'move_file', 'copy_file', 'rename_file',
    'search_code', 'find_files', 'grep_search', 'ask_question',
    'complete_task', 'finish_task', 'submit_result',
    // OpenCode tools (lowercase)
    'write', 'read', 'edit', 'bash', 'glob', 'grep', 'task', 'webfetch',
    'todowrite', 'todoread', 'skill',
    // OpenCode tools (PascalCase)
    'Write', 'Read', 'Edit', 'Bash', 'Glob', 'Grep', 'Task', 'WebFetch',
    'TodoWrite', 'TodoRead', 'Skill', 'AskUserQuestion'
  ];

  for (const toolName of rooClineTools) {
    const pattern = new RegExp('<' + toolName + '(?:\\s[^>]*)?>[\\s\\S]*?</' + toolName + '>', 'gi');
    cleaned = cleaned.replace(pattern, '');
  }

  // Remove unclosed format: <function=name><parameter=...>...
  cleaned = cleaned.replace(/<function=[a-z_]+>[\s\S]*$/gi, '');

  // Remove markdown-wrapped JSON tool calls
  cleaned = cleaned.replace(/```(?:json)?\s*\n?\s*\{[\s\S]*?"(?:name|tool_calls)"[\s\S]*?\}\s*\n?```/gi, '');

  // Clean up extra newlines
  cleaned = cleaned.replace(/\n{3,}/g, '\n\n').trim();

  return cleaned;
}

// Check if content contains incomplete tool call (for streaming buffering)
function hasIncompleteToolCall(content) {
  // Check for unclosed XML tags that might indicate a tool call in progress
  const patterns = [
    /<tool_call>(?![\s\S]*<\/tool_call>)/i,              // Unclosed <tool_call>
    /<function=[^>]+>(?![\s\S]*<\/function>)/i,          // Unclosed <function=>
    /<write_file>(?![\s\S]*<\/write_file>)/i,            // Unclosed Roo/Cline tools
    /<write_to_file>(?![\s\S]*<\/write_to_file>)/i,      // write_to_file variant
    /<read_file>(?![\s\S]*<\/read_file>)/i,
    /<read_from_file>(?![\s\S]*<\/read_from_file>)/i,    // read_from_file variant
    /<apply_diff>(?![\s\S]*<\/apply_diff>)/i,
    /<execute_command>(?![\s\S]*<\/execute_command>)/i,
    /<run_command>(?![\s\S]*<\/run_command>)/i,          // run_command variant
    /<attempt_completion>(?![\s\S]*<\/attempt_completion>)/i,
    /<complete_task>(?![\s\S]*<\/complete_task>)/i,      // complete_task variant
    /<edit_file>(?![\s\S]*<\/edit_file>)/i,              // edit_file
    /<replace_in_file>(?![\s\S]*<\/replace_in_file>)/i,  // replace_in_file
    /```(?:json)?\s*\n?\s*\{[^}]*$/i,                     // Unclosed JSON in code block
    // OpenCode tools (lowercase)
    /<write>(?![\s\S]*<\/write>)/i,
    /<read>(?![\s\S]*<\/read>)/i,
    /<edit>(?![\s\S]*<\/edit>)/i,
    /<bash>(?![\s\S]*<\/bash>)/i,
    /<glob>(?![\s\S]*<\/glob>)/i,
    /<grep>(?![\s\S]*<\/grep>)/i,
    /<task>(?![\s\S]*<\/task>)/i,
    // OpenCode tools (PascalCase)
    /<Write>(?![\s\S]*<\/Write>)/,
    /<Read>(?![\s\S]*<\/Read>)/,
    /<Edit>(?![\s\S]*<\/Edit>)/,
    /<Bash>(?![\s\S]*<\/Bash>)/,
    /<Glob>(?![\s\S]*<\/Glob>)/,
    /<Grep>(?![\s\S]*<\/Grep>)/,
    /<Task>(?![\s\S]*<\/Task>)/,
    /<TodoWrite>(?![\s\S]*<\/TodoWrite>)/,
    /<AskUserQuestion>(?![\s\S]*<\/AskUserQuestion>)/,
    // Malformed tool_call patterns
    /<tool_call>[A-Za-z]+>(?![\s\S]*<\/[A-Za-z]+>)/,
  ];

  for (const pattern of patterns) {
    if (pattern.test(content)) {
      return true;
    }
  }

  return false;
}

// Format response in OpenAI format
function formatOpenAIResponse(result, model, requestId, stream = false, fullContent = null) {
  const timestamp = Math.floor(Date.now() / 1000);
  const rawContent = result.content || result.text || "";

  if (stream) {
    if (result.finish_reason !== 'stop') {
      // Intermediate chunk - just send content delta
      return {
        id: `chatcmpl-${requestId}`,
        object: "chat.completion.chunk",
        created: timestamp,
        model: model || "z1",
        choices: [{
          index: 0,
          delta: { content: rawContent },
          finish_reason: null
        }]
      };
    }

    // Final chunk - check for tool calls in the FULL accumulated content
    const contentToCheck = fullContent || rawContent;
    const toolCalls = parseToolCalls(contentToCheck);

    if (toolCalls.length > 0) {
      // Send tool_calls in the final chunk
      return {
        id: `chatcmpl-${requestId}`,
        object: "chat.completion.chunk",
        created: timestamp,
        model: model || "z1",
        choices: [{
          index: 0,
          delta: {
            tool_calls: toolCalls.map((tc, idx) => ({
              index: idx,
              id: tc.id,
              type: "function",
              function: {
                name: tc.function.name,
                arguments: tc.function.arguments
              }
            }))
          },
          finish_reason: "tool_calls"
        }]
      };
    }

    // No tool calls - just send final stop
    return {
      id: `chatcmpl-${requestId}`,
      object: "chat.completion.chunk",
      created: timestamp,
      model: model || "z1",
      choices: [{
        index: 0,
        delta: { content: rawContent },
        finish_reason: "stop"
      }]
    };
  }

  // Full response format (non-streaming)
  const toolCalls = parseToolCalls(rawContent);
  const cleanContent = toolCalls.length > 0 ? removeToolCallsFromContent(rawContent) : rawContent;

  const response = {
    id: `chatcmpl-${requestId}`,
    object: "chat.completion",
    created: timestamp,
    model: model || "z1",
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

  return response;
}

// Format error in OpenAI format
function formatOpenAIError(message, type = "api_error", code = null, status = 500) {
  return {
    error: {
      message,
      type,
      code,
      param: null
    }
  };
}

// ============== WEBSOCKET HANDLING ==============

wss.on("connection", (ws, req) => {
  const clientId = generateClientId();
  const clientIp = req.headers["x-forwarded-for"] || req.socket.remoteAddress || "unknown";

  console.log(`[WS] Client connected: ${clientId} from ${clientIp}`);

  pool.addClient(clientId, ws, clientIp);

  ws.on("message", (data) => {
    try {
      const message = JSON.parse(data.toString());
      handleWebSocketMessage(clientId, message);
    } catch (err) {
      console.error(`[WS] Error parsing message:`, err);
    }
  });

  ws.on("close", () => {
    console.log(`[WS] Client disconnected: ${clientId}`);
    pool.removeClient(clientId);
  });

  ws.on("error", (err) => {
    console.error(`[WS] WebSocket error for ${clientId}:`, err);
  });

  // Keepalive ping
  const pingInterval = setInterval(() => {
    if (ws.readyState === ws.OPEN) {
      pool.recordPing(clientId);
      ws.send(JSON.stringify({ type: "ping" }));
    } else {
      clearInterval(pingInterval);
    }
  }, config.websocket.pingInterval);
});

function handleWebSocketMessage(clientId, message) {
  console.log(`[WS] Message from ${clientId}:`, message.type);

  switch (message.type) {
    case "ready":
      pool.setClientReady(clientId);
      const client = pool.getClient(clientId);
      if (client?.ws) {
        client.ws.send(JSON.stringify({ type: "get-models" }));
      }
      break;

    case "models":
      availableModels = message.models || [];
      pool.updateClientModel(clientId, message.currentModel);
      console.log(`[WS] Models received:`, availableModels);
      break;

    case "feature-status":
      pool.updateClientFeatures(clientId, {
        search: message.searchEnabled,
        deepThink: message.deepThinkEnabled
      });
      break;

    case "response":
      handleResponse(clientId, message);
      break;

    case "stream-chunk":
      handleStreamChunk(message);
      break;

    case "error":
      handleError(clientId, message);
      break;

    case "rate-limit":
      handleRateLimit(clientId, message);
      break;

    case "generation-stopped":
      handleGenerationStopped(message);
      break;

    case "history-cleared":
      console.log(`[WS] History cleared for ${clientId}:`, message.success);
      break;

    case "health-status":
      console.log(`[WS] Health status from ${clientId}:`, message);
      break;

    case "pong":
      pool.recordPong(clientId);
      break;
  }
}

function handleResponse(clientId, message) {
  const requestId = message.requestId;
  const pending = pendingRequests.get(requestId);

  if (pending) {
    // Send final chunk for streaming
    const streamConn = streamingConnections.get(requestId);
    if (streamConn) {
      const fullBuffer = streamingBuffers.get(requestId) || "";
      const lastSent = streamingBuffers.get(requestId + '_sent') || "";
      const remaining = fullBuffer.substring(lastSent.length);

      // Send any remaining content
      if (remaining) {
        const remainingChunk = formatOpenAIResponse({
          content: remaining,
          finish_reason: null
        }, message.model, requestId, true);
        streamConn.write(`data: ${JSON.stringify(remainingChunk)}\n\n`);
      }

      // Send final chunk - pass full buffer to check for tool calls
      const finalChunk = formatOpenAIResponse({
        content: "",
        finish_reason: "stop"
      }, message.model, requestId, true, fullBuffer);
      streamConn.write(`data: ${JSON.stringify(finalChunk)}\n\n`);

      streamingBuffers.delete(requestId);
      streamingBuffers.delete(requestId + '_sent');

      streamConn.write("data: [DONE]\n\n");
      streamConn.end();
      streamingConnections.delete(requestId);
    }

    pending.resolve(message);
    pendingRequests.delete(requestId);
  }

  pool.setClientIdle(clientId);
}

function handleStreamChunk(message) {
  const requestId = message.requestId;
  const streamConn = streamingConnections.get(requestId);

  if (!streamConn) return;

  const content = message.content || "";

  // Accumulate content in buffer
  const currentBuffer = streamingBuffers.get(requestId) || "";
  const newBuffer = currentBuffer + content;
  streamingBuffers.set(requestId, newBuffer);

  // Check if we have an incomplete tool call - if so, buffer and don't stream yet
  if (hasIncompleteToolCall(newBuffer)) {
    console.log(`[Stream] Buffering incomplete tool call for requestId: ${requestId}`);
    return; // Don't stream yet, wait for complete tool call
  }

  // Stream the chunk
  const lastSent = streamingBuffers.get(requestId + '_sent') || "";
  const delta = newBuffer.substring(lastSent.length);

  if (delta) {
    streamingBuffers.set(requestId + '_sent', newBuffer);
    const chunk = formatOpenAIResponse({
      content: delta,
      finish_reason: null
    }, null, requestId, true);
    streamConn.write(`data: ${JSON.stringify(chunk)}\n\n`);
  }
}

function handleError(clientId, message) {
  const requestId = message.requestId;
  const pending = pendingRequests.get(requestId);

  if (pending) {
    pending.reject(new Error(message.error));
    pendingRequests.delete(requestId);
  }

  const streamConn = streamingConnections.get(requestId);
  if (streamConn) {
    streamConn.write(`data: ${JSON.stringify({ error: message.error })}\n\n`);
    streamConn.end();
    streamingConnections.delete(requestId);
  }

  pool.setClientIdle(clientId);
}

function handleRateLimit(clientId, message) {
  pool.setClientRateLimited(clientId, message.message);

  const requestId = message.requestId;
  const pending = pendingRequests.get(requestId);

  if (pending) {
    pending.reject(new Error("Rate limited: " + message.message));
    pendingRequests.delete(requestId);
  }

  const streamConn = streamingConnections.get(requestId);
  if (streamConn) {
    streamConn.write(`data: ${JSON.stringify({ error: "Rate limited" })}\n\n`);
    streamConn.end();
    streamingConnections.delete(requestId);
  }
}

function handleGenerationStopped(message) {
  const requestId = message.requestId;
  const pending = pendingRequests.get(requestId);

  if (pending) {
    pending.resolve({ stopped: true, text: "Generation stopped by user" });
    pendingRequests.delete(requestId);
  }

  const streamConn = streamingConnections.get(requestId);
  if (streamConn) {
    const finalChunk = formatOpenAIResponse({
      content: "",
      finish_reason: "stop"
    }, null, requestId, true);
    streamConn.write(`data: ${JSON.stringify(finalChunk)}\n\n`);
    streamConn.write("data: [DONE]\n\n");
    streamConn.end();
    streamingConnections.delete(requestId);
  }
}

// ============== SEND REQUEST TO CLIENT ==============

async function sendPromptToClient(prompt, options = {}) {
  const { search = false, deepThink = false, stream = false, sessionId = null, freshSession = false, requestId = null } = options;

  const availableClientId = pool.getAvailableClient(sessionId);

  if (!availableClientId) {
    try {
      const queueResult = await pool.queueRequest({ prompt, options });
      return await sendPromptToSpecificClient(queueResult.clientId, prompt, { ...options, requestId });
    } catch (err) {
      throw new Error("No available clients and queue failed: " + err.message);
    }
  }

  return await sendPromptToSpecificClient(availableClientId, prompt, { ...options, requestId });
}

async function sendPromptToSpecificClient(clientId, prompt, options) {
  const { search = false, deepThink = false, stream = false, freshSession = false, requestId: providedRequestId = null } = options;

  const client = pool.getClient(clientId);
  if (!client || !client.ws) {
    throw new Error("Client not available");
  }

  pool.setClientBusy(clientId);

  const requestId = providedRequestId || generateId();
  const timeout = config.timeouts.default;

  return new Promise((resolve, reject) => {
    const timeoutId = setTimeout(() => {
      pendingRequests.delete(requestId);
      pool.setClientIdle(clientId);
      reject(new Error("Request timeout"));
    }, timeout);

    pendingRequests.set(requestId, {
      resolve: (result) => {
        clearTimeout(timeoutId);
        resolve({ ...result, clientId, requestId });
      },
      reject: (err) => {
        clearTimeout(timeoutId);
        reject(err);
      },
      clientId,
      startTime: Date.now()
    });

    // Clear history if fresh session requested
    if (freshSession) {
      client.ws.send(JSON.stringify({ type: "clear-history" }));
    }

    // Send prompt
    const promptMessage = {
      type: "prompt",
      prompt,
      search,
      deepThink,
      requestId,
      stream
    };
    console.log(`[Prompt] Sending to browser with requestId: ${requestId}, stream: ${stream}`);
    client.ws.send(JSON.stringify(promptMessage));
  });
}

// ============== DASHBOARD HTML ==============

const getDashboardHTML = (host) => `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Z.AI Proxy Pool</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      background: linear-gradient(135deg, #1e3a5f 0%, #0d1b2a 50%, #1b263b 100%);
      min-height: 100vh;
      color: #e0e0e0;
      padding: 20px;
    }
    .container { max-width: 1200px; margin: 0 auto; }
    .header {
      text-align: center;
      padding: 40px 20px;
      background: rgba(255,255,255,0.05);
      border-radius: 16px;
      margin-bottom: 30px;
      border: 1px solid rgba(255,255,255,0.1);
    }
    .header h1 {
      font-size: 2.5rem;
      background: linear-gradient(135deg, #3b82f6, #1d4ed8, #60a5fa);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      margin-bottom: 10px;
    }
    .header p { color: #888; font-size: 1.1rem; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
    .card {
      background: rgba(255,255,255,0.05);
      border-radius: 12px;
      padding: 24px;
      border: 1px solid rgba(255,255,255,0.1);
    }
    .card h2 { color: #60a5fa; margin-bottom: 16px; font-size: 1.2rem; }
    .stat-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
    .stat {
      background: rgba(0,0,0,0.2);
      padding: 12px;
      border-radius: 8px;
    }
    .stat .label { color: #888; font-size: 0.85rem; }
    .stat .value { color: #60a5fa; font-weight: 600; font-size: 1.5rem; margin-top: 4px; }
    .client-list { max-height: 300px; overflow-y: auto; }
    .client-item {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 12px;
      background: rgba(0,0,0,0.2);
      border-radius: 8px;
      margin-bottom: 8px;
    }
    .client-dot {
      width: 10px;
      height: 10px;
      border-radius: 50%;
    }
    .client-dot.idle { background: #22c55e; }
    .client-dot.busy { background: #f59e0b; }
    .client-dot.rate-limited { background: #ef4444; }
    .client-dot.unhealthy { background: #6b7280; }
    .client-info { flex: 1; }
    .client-id { font-family: monospace; color: #60a5fa; }
    .client-status { font-size: 0.85rem; color: #888; }
    .code-block {
      background: #0d1117;
      border-radius: 8px;
      padding: 16px;
      overflow-x: auto;
      font-family: 'Monaco', 'Menlo', monospace;
      font-size: 0.85rem;
      border: 1px solid #30363d;
      margin: 12px 0;
    }
    .code-block code { color: #c9d1d9; white-space: pre-wrap; }
    .btn {
      background: linear-gradient(135deg, #3b82f6, #1d4ed8);
      color: white;
      border: none;
      padding: 10px 20px;
      border-radius: 8px;
      cursor: pointer;
      font-size: 0.9rem;
      margin-top: 8px;
    }
    .btn:hover { opacity: 0.9; }
    .bookmarklet-btn {
      display: inline-block;
      background: linear-gradient(135deg, #3b82f6, #1d4ed8);
      color: white;
      padding: 12px 24px;
      border-radius: 8px;
      text-decoration: none;
      font-weight: 600;
    }
    .endpoint {
      background: rgba(0,0,0,0.2);
      padding: 12px;
      border-radius: 8px;
      margin-bottom: 8px;
    }
    .method {
      display: inline-block;
      padding: 4px 8px;
      border-radius: 4px;
      font-size: 0.75rem;
      font-weight: 600;
      margin-right: 8px;
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
      <h1>Z.AI Proxy Pool</h1>
      <p>Multi-browser pool with OpenAI-compatible API</p>
    </div>

    <div class="grid">
      <div class="card">
        <h2>Pool Status</h2>
        <div class="stat-grid">
          <div class="stat">
            <div class="label">Total Clients</div>
            <div class="value" id="totalClients">0</div>
          </div>
          <div class="stat">
            <div class="label">Idle</div>
            <div class="value" id="idleClients">0</div>
          </div>
          <div class="stat">
            <div class="label">Busy</div>
            <div class="value" id="busyClients">0</div>
          </div>
          <div class="stat">
            <div class="label">Queue</div>
            <div class="value" id="queueLength">0</div>
          </div>
        </div>
      </div>

      <div class="card">
        <h2>Statistics</h2>
        <div class="stat-grid">
          <div class="stat">
            <div class="label">Total Requests</div>
            <div class="value" id="totalRequests">0</div>
          </div>
          <div class="stat">
            <div class="label">Success Rate</div>
            <div class="value" id="successRate">-</div>
          </div>
          <div class="stat">
            <div class="label">Avg Latency</div>
            <div class="value" id="avgLatency">-</div>
          </div>
          <div class="stat">
            <div class="label">Rate Limits</div>
            <div class="value" id="rateLimits">0</div>
          </div>
        </div>
      </div>

      <div class="card">
        <h2>Connected Clients</h2>
        <div class="client-list" id="clientList">
          <p style="color: #888;">No clients connected</p>
        </div>
      </div>

      <div class="card">
        <h2>Quick Setup</h2>
        <p style="color: #888; margin-bottom: 12px;">Drag to bookmarks bar:</p>
        <a class="bookmarklet-btn" href="javascript:(function(){var s=document.createElement('script');s.src='http://${host}/inject.js?t='+Date.now();document.head.appendChild(s);})();">Z.AI Proxy</a>
        <p style="color: #888; margin-top: 16px;">Or paste in console:</p>
        <div class="code-block">
          <code>fetch('http://${host}/inject.js').then(r=>r.text()).then(eval)</code>
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
          <span class="method get">GET</span>
          <span class="path">/admin/clients</span>
          <div class="desc">List all connected clients with status</div>
        </div>
        <div class="endpoint">
          <span class="method get">GET</span>
          <span class="path">/admin/stats</span>
          <div class="desc">Pool statistics and metrics</div>
        </div>
      </div>

      <div class="card" style="grid-column: span 2;">
        <h2>Example Usage</h2>
        <div class="code-block">
          <code># OpenAI-compatible request
curl -X POST http://${host}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${config.auth.token}" \\
  -d '{
    "model": "z1",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello!"}
    ],
    "stream": false
  }'

# With streaming
curl -X POST http://${host}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${config.auth.token}" \\
  -d '{"model": "z1", "messages": [{"role": "user", "content": "Tell me a story"}], "stream": true}'

# With Deep Think enabled
curl -X POST http://${host}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${config.auth.token}" \\
  -d '{"model": "z1", "messages": [{"role": "user", "content": "Solve this problem"}], "deepThink": true}'</code>
        </div>
      </div>
    </div>
  </div>

  <script>
    async function updateStatus() {
      try {
        const res = await fetch('/admin/stats');
        const data = await res.json();

        document.getElementById('totalClients').textContent = data.totalClients;
        document.getElementById('idleClients').textContent = data.idleClients;
        document.getElementById('busyClients').textContent = data.busyClients;
        document.getElementById('queueLength').textContent = data.queueLength;
        document.getElementById('totalRequests').textContent = data.stats.totalRequests;
        document.getElementById('rateLimits').textContent = data.stats.rateLimitHits;

        if (data.stats.totalRequests > 0) {
          const rate = ((data.stats.successfulRequests / data.stats.totalRequests) * 100).toFixed(1);
          document.getElementById('successRate').textContent = rate + '%';
        }

        if (data.stats.averageLatency > 0) {
          document.getElementById('avgLatency').textContent = Math.round(data.stats.averageLatency) + 'ms';
        }

        const clientList = document.getElementById('clientList');
        if (data.clients.length === 0) {
          clientList.innerHTML = '<p style="color: #888;">No clients connected</p>';
        } else {
          clientList.innerHTML = data.clients.map(c => \`
            <div class="client-item">
              <div class="client-dot \${c.status}"></div>
              <div class="client-info">
                <div class="client-id">\${c.id}</div>
                <div class="client-status">\${c.status} | \${c.currentModel || 'z1'} | \${c.requestCount} reqs</div>
              </div>
            </div>
          \`).join('');
        }
      } catch (err) {
        console.error('Status update failed:', err);
      }
    }

    updateStatus();
    setInterval(updateStatus, 3000);
  </script>
</body>
</html>
`;

// ============== ROUTES ==============

// Dashboard
app.get("/", (req, res) => {
  const host = req.headers.host || `localhost:${config.server.port}`;
  res.send(getDashboardHTML(host));
});

// Status endpoint (public)
app.get("/status", (req, res) => {
  const status = pool.getStatus();
  res.json({
    connected: status.totalClients > 0,
    totalClients: status.totalClients,
    idleClients: status.idleClients,
    busyClients: status.busyClients,
    queueLength: status.queueLength,
    availableModels,
  });
});

// ============== OPENAI-COMPATIBLE API ==============

// List models
app.get("/v1/models", authMiddleware, (req, res) => {
  const models = availableModels.length > 0 ? availableModels : config.knownModels;

  res.json({
    object: "list",
    data: models.map(m => ({
      id: m.toLowerCase().replace(/\s+/g, '-'),
      object: "model",
      created: Math.floor(Date.now() / 1000),
      owned_by: "z-ai",
      permission: [],
      root: m,
      parent: null
    }))
  });
});

// Chat completions (OpenAI-compatible)
app.post("/v1/chat/completions", authMiddleware, (req, res) => {
  const { model, messages, stream = false, temperature, max_tokens, deepThink, search } = req.body;

  if (!messages || !Array.isArray(messages)) {
    return res.status(400).json(formatOpenAIError("messages is required and must be an array", "invalid_request_error"));
  }

  const status = pool.getStatus();
  if (status.totalClients === 0) {
    return res.status(503).json(formatOpenAIError("No browser clients connected", "service_unavailable"));
  }

  const prompt = messagesToPrompt(messages);
  const requestId = generateId();
  const sessionId = req.headers["x-session-id"];
  const freshSession = req.headers["x-fresh-session"] === "true";

  if (stream) {
    // SSE streaming response
    res.setHeader("Content-Type", "text/event-stream");
    res.setHeader("Cache-Control", "no-cache");
    res.setHeader("Connection", "keep-alive");
    res.setHeader("X-Accel-Buffering", "no");
    res.flushHeaders();

    streamingConnections.set(requestId, res);
    console.log(`[Stream] SSE connection stored for requestId: ${requestId}`);

    // Send initial chunk
    const initialChunk = formatOpenAIResponse({
      content: "",
      finish_reason: null
    }, model, requestId, true);
    res.write(`data: ${JSON.stringify(initialChunk)}\n\n`);

    // Keep-alive
    const keepAliveInterval = setInterval(() => {
      if (streamingConnections.has(requestId)) {
        try {
          const keepAliveChunk = formatOpenAIResponse({
            content: "",
            finish_reason: null
          }, model, requestId, true);
          res.write(`data: ${JSON.stringify(keepAliveChunk)}\n\n`);
        } catch (e) {
          clearInterval(keepAliveInterval);
        }
      } else {
        clearInterval(keepAliveInterval);
      }
    }, 5000);

    res.on("close", () => {
      console.log(`[Stream] Client disconnected for requestId: ${requestId}`);
      clearInterval(keepAliveInterval);
      streamingConnections.delete(requestId);
      const pending = pendingRequests.get(requestId);
      if (pending) {
        const client = pool.getClient(pending.clientId);
        if (client?.ws) {
          client.ws.send(JSON.stringify({ type: "stop-generation", requestId }));
        }
      }
    });

    sendPromptToClient(prompt, {
      search: search ?? false,
      deepThink: deepThink ?? false,
      stream: true,
      sessionId,
      freshSession,
      requestId
    }).catch(err => {
      console.error("[Stream] Error in streaming request:", err);
      clearInterval(keepAliveInterval);
      if (streamingConnections.has(requestId)) {
        try {
          res.write(`data: ${JSON.stringify({ error: { message: err.message } })}\n\n`);
          res.write(`data: [DONE]\n\n`);
          res.end();
        } catch (e) {}
        streamingConnections.delete(requestId);
      }
    });

  } else {
    // Non-streaming response
    (async () => {
      try {
        const result = await sendPromptToClient(prompt, {
          search: search ?? false,
          deepThink: deepThink ?? false,
          stream: false,
          sessionId,
          freshSession
        });

        res.json(formatOpenAIResponse(result, result.model || model, requestId));
      } catch (err) {
        console.error("[API] Error:", err);
        const statusCode = err.message.includes("Rate limited") ? 429 : 500;
        res.status(statusCode).json(formatOpenAIError(err.message, "api_error"));
      }
    })();
  }
});

// ============== LEGACY API ==============

// Legacy prompt endpoint
app.post("/prompt", authMiddleware, async (req, res) => {
  const { prompt, search, deepThink } = req.body;

  if (!prompt) {
    return res.status(400).json({ error: "Prompt is required" });
  }

  const status = pool.getStatus();
  if (status.totalClients === 0) {
    return res.status(503).json({ error: "No connected clients" });
  }

  const sessionId = req.headers["x-session-id"];
  const freshSession = req.headers["x-fresh-session"] === "true";

  try {
    const result = await sendPromptToClient(prompt, {
      search: search ?? false,
      deepThink: deepThink ?? false,
      stream: false,
      sessionId,
      freshSession
    });

    res.json({
      success: true,
      response: result.text,
      model: result.model,
      deepThinkEnabled: result.deepThinkEnabled,
      searchEnabled: result.searchEnabled
    });

  } catch (err) {
    console.error("[Prompt] Error:", err);
    const statusCode = err.message.includes("Rate limited") ? 429 :
                       err.message.includes("timeout") ? 504 : 500;
    res.status(statusCode).json({ success: false, error: err.message });
  }
});

// Legacy models endpoints
app.get("/models", authMiddleware, (req, res) => {
  res.json({
    models: availableModels.length > 0 ? availableModels : config.knownModels,
    currentModel: null
  });
});

// Features endpoint
app.post("/features", authMiddleware, (req, res) => {
  const { search, deepThink } = req.body;

  for (const [clientId, client] of pool.getAllClients()) {
    if (client.ws && client.ready) {
      client.ws.send(JSON.stringify({
        type: "set-features",
        search: search ?? client.features?.search,
        deepThink: deepThink ?? client.features?.deepThink
      }));
    }
  }

  res.json({ success: true, message: "Feature toggle sent to all clients" });
});

// ============== ADMIN ENDPOINTS ==============

app.get("/admin/clients", (req, res) => {
  const status = pool.getStatus();
  res.json({ clients: status.clients });
});

app.get("/admin/stats", (req, res) => {
  res.json(pool.getStatus());
});

app.get("/admin/queue", (req, res) => {
  const status = pool.getStatus();
  res.json({
    queueLength: status.queueLength,
    maxQueueSize: config.queue.maxSize,
    maxWaitTime: config.queue.maxWaitTime
  });
});

app.get("/admin/health", (req, res) => {
  const status = pool.getStatus();
  const healthy = status.totalClients > 0 && status.idleClients > 0;

  res.status(healthy ? 200 : 503).json({
    healthy,
    totalClients: status.totalClients,
    idleClients: status.idleClients,
    queueLength: status.queueLength
  });
});

app.post("/admin/clients/:id/clear", authMiddleware, (req, res) => {
  const clientId = req.params.id;
  const client = pool.getClient(clientId);

  if (!client) {
    return res.status(404).json({ error: "Client not found" });
  }

  if (client.ws) {
    client.ws.send(JSON.stringify({ type: "clear-history" }));
  }

  res.json({ success: true, message: "Clear history request sent" });
});

app.delete("/admin/clients/:id", authMiddleware, (req, res) => {
  const clientId = req.params.id;
  const client = pool.getClient(clientId);

  if (!client) {
    return res.status(404).json({ error: "Client not found" });
  }

  if (client.ws) {
    client.ws.close();
  }
  pool.removeClient(clientId);

  res.json({ success: true, message: "Client disconnected" });
});

// ============== INJECTION SCRIPT ==============

app.get("/inject.js", (req, res) => {
  const host = req.headers.host || `localhost:${config.server.port}`;
  const wsProtocol = req.secure ? "wss" : "ws";
  const script = generateInjectionScript(host, wsProtocol);
  res.type("application/javascript").send(script);
});

// ============== STOP GENERATION ==============

app.post("/stop", authMiddleware, (req, res) => {
  const { requestId, clientId } = req.body;

  if (clientId) {
    const client = pool.getClient(clientId);
    if (client?.ws) {
      client.ws.send(JSON.stringify({ type: "stop-generation", requestId }));
      res.json({ success: true, message: "Stop signal sent" });
    } else {
      res.status(404).json({ error: "Client not found" });
    }
  } else {
    for (const [cId, client] of pool.getAllClients()) {
      if (client.status === 'busy' && client.ws) {
        client.ws.send(JSON.stringify({ type: "stop-generation" }));
      }
    }
    res.json({ success: true, message: "Stop signal sent to all busy clients" });
  }
});

// ============== START SERVER ==============

server.listen(config.server.port, config.server.host, () => {
  console.log(`
╔═══════════════════════════════════════════════════════════════╗
║               Z.AI Proxy Pool Server Started                  ║
╠═══════════════════════════════════════════════════════════════╣
║  Dashboard:     http://localhost:${config.server.port}                      ║
║  OpenAI API:    http://localhost:${config.server.port}/v1/chat/completions  ║
║  Inject Script: http://localhost:${config.server.port}/inject.js            ║
╠═══════════════════════════════════════════════════════════════╣
║  Auth Token:    ${config.auth.token}                            ║
║  Pool Strategy: ${config.pool.rotationStrategy.toUpperCase()}                                       ║
║  Queue Max:     ${config.queue.maxSize} requests                                ║
╠═══════════════════════════════════════════════════════════════╣
║  Features:                                                    ║
║  ✓ Multi-browser client pool with LRU rotation               ║
║  ✓ Request queue (no more 429 on busy)                       ║
║  ✓ OpenAI-compatible /v1/chat/completions                    ║
║  ✓ SSE Streaming support (MutationObserver)                  ║
║  ✓ Session affinity (X-Session-Id header)                    ║
║  ✓ Fresh session support (X-Fresh-Session header)            ║
║  ✓ Stop generation support                                   ║
║  ✓ Deep Think & Search toggles                               ║
║  ✓ Admin endpoints for monitoring                            ║
╚═══════════════════════════════════════════════════════════════╝
`);
});
