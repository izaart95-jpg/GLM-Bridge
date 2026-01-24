// Injection Script Generator for Z.AI
// Returns the browser-side JavaScript that gets injected into Z.AI's page

const config = require('../config');

function generateInjectionScript(host, wsProtocol) {
  return `
(function() {
  // Prevent double injection
  if (window.__ZAI_PROXY_INJECTED__) {
    console.log('[Z.AI Proxy] Already injected');
    return;
  }
  window.__ZAI_PROXY_INJECTED__ = true;

  const WS_URL = '${wsProtocol}://${host}';
  let ws = null;
  let reconnectAttempts = 0;
  const MAX_RECONNECT = ${config.websocket.maxReconnectAttempts};

  // Feature states
  let currentSearch = false;
  let currentDeepThink = false;

  // Streaming state
  let activeStreamRequestId = null;
  let streamedContent = '';
  let lastSentContent = '';

  // Utility functions
  function getRandomInt(min, max) {
    const array = new Uint32Array(1);
    crypto.getRandomValues(array);
    return min + (array[0] % (max - min));
  }

  function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  function generateTypingDelays(text) {
    const delays = [];
    for (let i = 0; i < text.length; i++) {
      let baseDelay = getRandomInt(25, 75);
      const char = text[i];
      if ('.!?,;:'.includes(char)) {
        baseDelay += getRandomInt(80, 180);
      } else if (char === char.toUpperCase() && char !== char.toLowerCase()) {
        baseDelay += getRandomInt(20, 50);
      }
      if (Math.random() < 0.03) {
        baseDelay += getRandomInt(150, 350);
      }
      if (Math.random() < 0.05) {
        baseDelay = getRandomInt(10, 25);
      }
      delays.push(baseDelay);
    }
    return delays;
  }

  // ============== DOM SELECTORS for Z.AI ==============
  // Based on provided HTML structure

  const SELECTORS = {
    // Main chat input textarea
    textarea: '#chat-input',

    // Send button (enabled state with gradient, disabled is gray)
    sendButton: '#send-message-button:not([disabled])',
    sendButtonDisabled: '#send-message-button[disabled]',

    // Stop button (if exists during generation)
    stopButton: '.stop-button, button[aria-label="Stop"]',

    // New chat button
    newChatButton: '#new-chat-button, .navNewChat',

    // User messages (right-aligned)
    userMessage: '.flex.justify-end .rounded-xl',

    // Assistant responses
    responseMessage: 'p.svelte-121hp7c, [dir="auto"].svelte-121hp7c',

    // Search button
    searchButton: 'button:has(svg path[d*="14.5 14.5L11.3583"])',

    // Deep Think button (data-autothink attribute)
    deepThinkButton: 'button[data-autothink]',

    // File upload button
    uploadButton: 'button[aria-label="More"]',
  };

  function findElement(selectors) {
    const selectorList = selectors.split(', ');
    for (const selector of selectorList) {
      try {
        const el = document.querySelector(selector);
        if (el) return el;
      } catch (e) {}
    }
    return null;
  }

  function findElements(selectors) {
    const selectorList = selectors.split(', ');
    for (const selector of selectorList) {
      try {
        const els = document.querySelectorAll(selector);
        if (els.length > 0) return Array.from(els);
      } catch (e) {}
    }
    return [];
  }

  function findInput() {
    return document.querySelector('#chat-input');
  }

  function findSendButton() {
    const btn = document.querySelector('#send-message-button');
    if (btn && !btn.disabled && !btn.classList.contains('disabled')) {
      return btn;
    }
    return null;
  }

  function findStopButton() {
    return findElement(SELECTORS.stopButton);
  }

  // ============== HUMAN-LIKE INTERACTIONS ==============

  async function humanClick(element) {
    if (!element) return false;
    const rect = element.getBoundingClientRect();
    const x = rect.left + rect.width / 2 + getRandomInt(-5, 5);
    const y = rect.top + rect.height / 2 + getRandomInt(-3, 3);
    const events = ['pointerdown', 'mousedown', 'pointerup', 'mouseup', 'click'];
    for (const eventType of events) {
      const event = new PointerEvent(eventType, {
        bubbles: true,
        cancelable: true,
        view: window,
        clientX: x,
        clientY: y,
        button: 0
      });
      element.dispatchEvent(event);
      await sleep(getRandomInt(10, 30));
    }
    return true;
  }

  function setInputValue(element, value) {
    const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
      window.HTMLTextAreaElement.prototype, 'value'
    ).set;
    nativeInputValueSetter.call(element, value);
    const inputEvent = new Event('input', { bubbles: true, cancelable: true });
    const tracker = element._valueTracker;
    if (tracker) {
      tracker.setValue('');
    }
    element.dispatchEvent(inputEvent);
  }

  async function triggerInputDetection(element, text) {
    element.focus();
    await sleep(50);
    element.value = '';

    // For large prompts (>1000 chars), skip execCommand and use direct value setting
    // execCommand('insertText') is extremely slow for large text (simulates typing)
    const LARGE_PROMPT_THRESHOLD = 1000;

    if (text.length > LARGE_PROMPT_THRESHOLD) {
      console.log('[Z.AI Proxy] Large prompt detected (' + text.length + ' chars), using fast input method');
      setInputValue(element, text);
      await sleep(100);
      return true;
    }

    // For small prompts, try execCommand first (better React detection)
    try {
      document.execCommand('insertText', false, text);
      await sleep(100);
      if (element.value === text) {
        return true;
      }
    } catch (e) {}
    setInputValue(element, text);
    return true;
  }

  // ============== FEATURE TOGGLES ==============

  async function setSearch(enabled) {
    // Find search button - nested structure:
    // <button (wrapper)><button class="flex items-center...bg-transparent OR bg-[#DAEEFF]">
    //   <div><svg>...</svg> <span>Search</span></div></button></button>
    let searchBtn = null;
    let clickTarget = null;

    // Find all buttons and look for the one with Search text
    const allButtons = document.querySelectorAll('button[type="button"]');
    for (const btn of allButtons) {
      const span = btn.querySelector('span');
      if (span && span.textContent.trim() === 'Search') {
        // This could be the inner or outer button
        // Check if this button has the styling classes (inner button)
        if (btn.className.includes('flex items-center')) {
          searchBtn = btn;
          clickTarget = btn;
        } else {
          // This is wrapper, find inner button
          const innerBtn = btn.querySelector('button[type="button"]');
          if (innerBtn) {
            searchBtn = innerBtn;
            clickTarget = innerBtn;
          } else {
            searchBtn = btn;
            clickTarget = btn;
          }
        }
        break;
      }
    }

    // Method 2: Find by SVG path (magnifying glass)
    if (!searchBtn) {
      for (const btn of allButtons) {
        const path = btn.querySelector('svg path');
        if (path) {
          const d = path.getAttribute('d') || '';
          if (d.includes('14.5 14.5L11.3583') || d.includes('7.27778')) {
            if (btn.className.includes('flex items-center')) {
              searchBtn = btn;
              clickTarget = btn;
            } else {
              const innerBtn = btn.querySelector('button[type="button"]');
              searchBtn = innerBtn || btn;
              clickTarget = searchBtn;
            }
            break;
          }
        }
      }
    }

    if (!searchBtn) {
      console.log('[Z.AI Proxy] Search button not found');
      return false;
    }

    console.log('[Z.AI Proxy] Found search button, classes:', searchBtn.className);

    // Check if search is active - when active, button has bg-[#DAEEFF] (light) or bg-white/10 (dark)
    // Inactive: bg-transparent
    const btnClasses = searchBtn.className || '';
    const isActive = btnClasses.includes('bg-[#DAEEFF]') ||
                     btnClasses.includes('bg-white/10') ||
                     !btnClasses.includes('bg-transparent');

    console.log('[Z.AI Proxy] Search current state:', isActive, 'requested:', enabled);

    if (enabled !== isActive) {
      await humanClick(clickTarget);
      await sleep(getRandomInt(300, 500));
      console.log('[Z.AI Proxy] Search toggled');
    }
    currentSearch = enabled;
    return true;
  }

  async function setDeepThink(enabled) {
    // Find Deep Think button by data-autothink attribute
    const deepThinkBtn = document.querySelector('button[data-autothink]');

    if (!deepThinkBtn) {
      console.log('[Z.AI Proxy] Deep Think button not found');
      return false;
    }

    // Check current state from data-autothink attribute
    const isActive = deepThinkBtn.getAttribute('data-autothink') === 'true';

    if (enabled !== isActive) {
      await humanClick(deepThinkBtn);
      await sleep(getRandomInt(300, 500));
    }
    currentDeepThink = enabled;
    return true;
  }

  // ============== RESPONSE EXTRACTION ==============

  function getLastAssistantResponse() {
    // Z.AI structure:
    // - Assistant messages are in .chat-assistant containers
    // - Response content is in #response-content-container
    // - Actual text is in <p dir="auto" class="svelte-121hp7c">

    // Method 1: Find #response-content-container (exact ID match - most reliable)
    const responseContainers = document.querySelectorAll('#response-content-container');
    if (responseContainers.length > 0) {
      const lastContainer = responseContainers[responseContainers.length - 1];
      // Return the container itself - we'll extract all text from it
      return lastContainer;
    }

    // Method 2: Find .chat-assistant containers inside message divs
    const messageContainers = document.querySelectorAll('[id^="message-"]');
    if (messageContainers.length > 0) {
      // Get the last message that has assistant content
      for (let i = messageContainers.length - 1; i >= 0; i--) {
        const container = messageContainers[i];
        // Check if it's an assistant message (has .chat-assistant inside)
        const assistantContent = container.querySelector('.chat-assistant');
        if (assistantContent) {
          // Look for response-content-container first
          const responseContent = container.querySelector('#response-content-container, [id*="response-content"]');
          if (responseContent) {
            return responseContent;
          }
          return assistantContent;
        }
      }
    }

    // Method 3: Find .chat-assistant.markdown-prose containers
    const chatAssistants = document.querySelectorAll('.chat-assistant.markdown-prose');
    if (chatAssistants.length > 0) {
      const lastAssistant = chatAssistants[chatAssistants.length - 1];
      const responseContent = lastAssistant.querySelector('#response-content-container');
      if (responseContent) {
        return responseContent;
      }
      return lastAssistant;
    }

    return null;
  }

  function extractResponseText(element) {
    if (!element) return '';

    // Count paragraphs to see how many we have
    const paragraphs = element.querySelectorAll('p[dir="auto"]');
    const paragraphCount = paragraphs.length;

    // Get full textContent of the element - this captures ALL paragraphs
    const fullText = element.textContent.trim();

    console.log('[Z.AI Proxy] extractResponseText - paragraphs:', paragraphCount, 'text length:', fullText.length);

    return fullText;
  }

  function isGenerating() {
    // Z.AI behavior:
    // - During generation: #send-message-button is REMOVED, replaced with stop button
    // - After generation: #send-message-button comes BACK (disabled or enabled)
    // So: if #send-message-button exists -> NOT generating
    //     if #send-message-button doesn't exist -> IS generating

    const sendBtn = document.querySelector('#send-message-button');
    const generating = sendBtn === null;

    console.log('[Z.AI Proxy] isGenerating:', generating, '(sendBtn exists:', sendBtn !== null, ')');

    return generating;
  }

  async function stopGeneration() {
    const stopBtn = findStopButton();
    if (stopBtn && stopBtn.offsetParent !== null) {
      await humanClick(stopBtn);
      await sleep(300);
      return true;
    }
    return false;
  }

  // ============== STREAMING via DOM Polling ==============

  let streamingInterval = null;
  let lastStreamedContent = '';

  function startStreamingPoller(requestId) {
    // Clear any existing interval
    if (streamingInterval) {
      clearInterval(streamingInterval);
    }

    activeStreamRequestId = requestId;
    streamedContent = '';
    lastSentContent = '';
    lastStreamedContent = '';

    console.log('[Z.AI Proxy] Starting streaming poller for requestId:', requestId);

    // Poll DOM every 100ms for new content
    streamingInterval = setInterval(() => {
      if (!activeStreamRequestId) {
        clearInterval(streamingInterval);
        streamingInterval = null;
        return;
      }

      // Find the latest assistant response
      const responseEl = getLastAssistantResponse();
      if (!responseEl) return;

      // Use extractResponseText to properly get content
      const currentContent = extractResponseText(responseEl);

      // Only send if content changed and not empty
      if (currentContent && currentContent !== lastStreamedContent) {
        const delta = currentContent.substring(lastStreamedContent.length);

        if (delta && ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({
            type: 'stream-chunk',
            requestId: activeStreamRequestId,
            content: delta,
            fullContent: currentContent
          }));
          console.log('[Z.AI Proxy] Stream chunk sent:', delta.substring(0, 50));
        }

        lastStreamedContent = currentContent;
        streamedContent = currentContent;
        lastSentContent = currentContent;
      }
    }, 100); // Poll every 100ms
  }

  function stopStreamingPoller() {
    if (streamingInterval) {
      clearInterval(streamingInterval);
      streamingInterval = null;
    }
    activeStreamRequestId = null;
    console.log('[Z.AI Proxy] Streaming poller stopped');
  }

  // ============== WAIT FOR RESPONSE ==============

  async function waitForResponse(promptText, options = {}) {
    const { timeout = 120000, requestId = null, stream = false } = options;
    const startTime = Date.now();
    let lastContent = '';
    let stableCount = 0;
    const STABLE_THRESHOLD = 6; // Need 6 stable checks at 300ms = 1.8 seconds of stability
    let generationEndTime = null; // Track when generation ended

    console.log('[Z.AI Proxy] Waiting for response...');

    // Start streaming poller if streaming enabled
    if (stream && requestId) {
      startStreamingPoller(requestId);
    }

    while (Date.now() - startTime < timeout) {
      await sleep(300); // Check more frequently (300ms instead of 500ms)

      const responseEl = getLastAssistantResponse();
      if (!responseEl) {
        console.log('[Z.AI Proxy] No response element found yet...');
        continue;
      }

      // Check if still generating (button not present = generating)
      const stillGenerating = isGenerating();
      // ALWAYS read from DOM - don't use streamedContent as it may be incomplete
      const content = extractResponseText(responseEl);

      // Track when generation ends (button comes back)
      if (!stillGenerating && generationEndTime === null) {
        generationEndTime = Date.now();
        console.log('[Z.AI Proxy] Generation ended, waiting for DOM to settle...');
      }

      // Reset if generation restarts
      if (stillGenerating) {
        generationEndTime = null;
        stableCount = 0;
      }

      console.log('[Z.AI Proxy] Content length:', content?.length, '| generating:', stillGenerating, '| stableCount:', stableCount);

      // If button is back but content is empty or very short, keep waiting (HTML still loading)
      if (!stillGenerating && (!content || content.length < 10)) {
        console.log('[Z.AI Proxy] Button back but content still loading...');
        continue;
      }

      // After generation ends, wait at least 1.2 seconds for DOM to fully render
      const timeSinceGenEnd = generationEndTime ? (Date.now() - generationEndTime) : 0;
      if (generationEndTime && timeSinceGenEnd < 1200) {
        console.log('[Z.AI Proxy] Waiting for DOM to settle...', timeSinceGenEnd, 'ms since gen end, content length:', content?.length);
        lastContent = content; // Keep tracking content changes
        continue;
      }

      // Track content stability (only after the 2 second wait)
      if (content && !stillGenerating) {
        if (content === lastContent) {
          stableCount++;
          console.log('[Z.AI Proxy] Content stable, count:', stableCount, '/', STABLE_THRESHOLD);

          if (stableCount >= STABLE_THRESHOLD) {
            // Stop streaming poller
            stopStreamingPoller();
            console.log('[Z.AI Proxy] Response complete! Length:', content.length);
            console.log('[Z.AI Proxy] Full content:', content);

            return {
              text: content,
              deepThinkEnabled: currentDeepThink,
              searchEnabled: currentSearch
            };
          }
        } else {
          // Content changed - reset stable count
          console.log('[Z.AI Proxy] Content changed, resetting stable count');
          lastContent = content;
          stableCount = 0;
        }
      }

      // Error check
      const error = document.querySelector('[class*="error"], .error-message');
      if (error && error.textContent.toLowerCase().includes('error')) {
        stopStreamingPoller();
        const errorText = error.textContent.trim();
        if (errorText.toLowerCase().includes('rate') || errorText.toLowerCase().includes('limit')) {
          return { error: 'rate-limit', message: errorText };
        }
        return { error: 'error', message: errorText };
      }
    }

    stopStreamingPoller();
    console.log('[Z.AI Proxy] Timeout reached. Last content:', lastContent?.substring(0, 100));
    return { error: 'timeout', message: 'Response timeout' };
  }

  async function waitForSendButton(maxWait = 5000) {
    const startTime = Date.now();
    while (Date.now() - startTime < maxWait) {
      const btn = findSendButton();
      if (btn) return btn;
      await sleep(100);
    }
    return null;
  }

  // ============== EXECUTE PROMPT ==============

  async function executePrompt(prompt, options = {}) {
    const { search = false, deepThink = false, requestId = null, stream = false } = options;
    console.log('[Z.AI Proxy] Executing prompt:', prompt.substring(0, 50) + '...');

    try {
      // Set features
      if (search !== currentSearch) await setSearch(search);
      if (deepThink !== currentDeepThink) await setDeepThink(deepThink);

      const input = findInput();
      if (!input) return { error: 'Input textarea not found' };

      input.value = '';
      input.focus();
      await sleep(200);
      await triggerInputDetection(input, prompt);
      await sleep(500);

      let sendBtn = await waitForSendButton(2000);
      if (!sendBtn) {
        setInputValue(input, prompt);
        await sleep(500);
        sendBtn = await waitForSendButton(2000);
      }

      if (!sendBtn) {
        // Try clicking any send button
        const anyBtn = document.querySelector('#send-message-button');
        if (anyBtn && !anyBtn.disabled) {
          anyBtn.click();
        } else {
          return { error: 'Send button not found or disabled' };
        }
      } else {
        await humanClick(sendBtn);
      }

      await sleep(500);

      const result = await waitForResponse(prompt, {
        timeout: 120000,
        requestId,
        stream
      });

      if (result.error) return result;

      return {
        text: result.text,
        deepThinkEnabled: result.deepThinkEnabled,
        searchEnabled: result.searchEnabled
      };

    } catch (err) {
      console.error('[Z.AI Proxy] Error:', err);
      return { error: err.message };
    }
  }

  async function clearHistory() {
    // Find and click the new chat button
    // HTML: <button id="new-chat-button" class="navNewChat...">
    const newChatBtn = document.querySelector('#new-chat-button, .navNewChat, button[aria-label="New Chat"]');
    if (!newChatBtn) {
      console.log('[Z.AI Proxy] New chat button not found');
      return false;
    }

    console.log('[Z.AI Proxy] Clicking new chat button...');
    await humanClick(newChatBtn);
    await sleep(1000);

    // Wait for URL to change from /c/XXXX to / (new chat page)
    const startTime = Date.now();
    while (Date.now() - startTime < 5000) {
      if (window.location.pathname === '/' || !window.location.pathname.startsWith('/c/')) {
        console.log('[Z.AI Proxy] New chat page loaded:', window.location.pathname);
        break;
      }
      await sleep(200);
    }

    console.log('[Z.AI Proxy] New chat started');
    return true;
  }

  // ============== STATUS BADGE ==============

  function createStatusBadge() {
    const existing = document.getElementById('zai-proxy-badge');
    if (existing) existing.remove();
    const badge = document.createElement('div');
    badge.id = 'zai-proxy-badge';
    badge.innerHTML = '<style>#zai-proxy-badge{position:fixed;top:10px;right:10px;background:linear-gradient(135deg,#3b82f6,#1d4ed8);color:white;padding:8px 16px;border-radius:20px;font-family:-apple-system,sans-serif;font-size:12px;font-weight:600;z-index:999999;display:flex;align-items:center;gap:8px;box-shadow:0 4px 12px rgba(59,130,246,0.4);cursor:pointer}#zai-proxy-badge:hover{transform:scale(1.05)}#zai-proxy-badge .dot{width:8px;height:8px;border-radius:50%;background:#22c55e;animation:pulse 2s infinite}#zai-proxy-badge.disconnected .dot{background:#ef4444;animation:none}@keyframes pulse{0%,100%{opacity:1}50%{opacity:0.5}}</style><span class="dot"></span><span class="text">Z.AI Connected</span>';
    badge.onclick = () => badge.remove();
    document.body.appendChild(badge);
    return badge;
  }

  // ============== WEBSOCKET CONNECTION ==============

  function connect() {
    console.log('[Z.AI Proxy] Connecting to', WS_URL);
    ws = new WebSocket(WS_URL);

    ws.onopen = () => {
      console.log('[Z.AI Proxy] Connected');
      reconnectAttempts = 0;
      const badge = createStatusBadge();
      badge.classList.remove('disconnected');
      ws.send(JSON.stringify({ type: 'ready' }));
    };

    ws.onmessage = async (event) => {
      try {
        const message = JSON.parse(event.data);
        console.log('[Z.AI Proxy] Received:', message.type);

        switch (message.type) {
          case 'ping':
            ws.send(JSON.stringify({ type: 'pong' }));
            break;

          case 'get-models':
            // Z.AI doesn't expose model selection in the same way
            ws.send(JSON.stringify({
              type: 'models',
              models: ${JSON.stringify(config.knownModels)},
              currentModel: 'z1'
            }));
            break;

          case 'set-features':
            if (message.search !== undefined) await setSearch(message.search);
            if (message.deepThink !== undefined) await setDeepThink(message.deepThink);
            ws.send(JSON.stringify({
              type: 'feature-status',
              searchEnabled: currentSearch,
              deepThinkEnabled: currentDeepThink
            }));
            break;

          case 'prompt':
            const result = await executePrompt(message.prompt, {
              search: message.search,
              deepThink: message.deepThink || message.thinking,
              requestId: message.requestId,
              stream: message.stream
            });
            if (result.error) {
              if (result.error === 'rate-limit') {
                ws.send(JSON.stringify({
                  type: 'rate-limit',
                  message: result.message,
                  requestId: message.requestId
                }));
              } else {
                ws.send(JSON.stringify({
                  type: 'error',
                  error: result.error || result.message,
                  requestId: message.requestId
                }));
              }
            } else {
              ws.send(JSON.stringify({
                type: 'response',
                requestId: message.requestId,
                text: result.text,
                model: 'z1',
                deepThinkEnabled: result.deepThinkEnabled,
                searchEnabled: result.searchEnabled
              }));
            }
            break;

          case 'stop-generation':
            const stopped = await stopGeneration();
            stopStreamingPoller();
            ws.send(JSON.stringify({
              type: 'generation-stopped',
              requestId: message.requestId,
              stopped
            }));
            break;

          case 'clear-history':
            const cleared = await clearHistory();
            ws.send(JSON.stringify({ type: 'history-cleared', success: cleared }));
            break;

          case 'health-check':
            ws.send(JSON.stringify({
              type: 'health-status',
              healthy: true,
              url: window.location.href,
              model: 'z1'
            }));
            break;

          case 'clear-storage':
            localStorage.clear();
            sessionStorage.clear();
            location.reload();
            break;
        }
      } catch (err) {
        console.error('[Z.AI Proxy] Error:', err);
      }
    };

    ws.onclose = () => {
      console.log('[Z.AI Proxy] Disconnected');
      stopStreamingPoller();
      const badge = document.getElementById('zai-proxy-badge');
      if (badge) {
        badge.classList.add('disconnected');
        badge.querySelector('.text').textContent = 'Disconnected';
      }
      if (reconnectAttempts < MAX_RECONNECT) {
        reconnectAttempts++;
        setTimeout(connect, 3000);
      }
    };

    ws.onerror = (err) => console.error('[Z.AI Proxy] WebSocket error:', err);
  }

  connect();
  console.log('[Z.AI Proxy] Injection complete - MutationObserver streaming enabled');
})();
`;
}

module.exports = { generateInjectionScript };
