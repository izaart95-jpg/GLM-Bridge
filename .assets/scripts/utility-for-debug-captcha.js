// Script 1
(function() {
          const TARGET_STATIC_PATH = "3.25.0/pe.059.f123b6c8830e46be";
        
          function patchResponse(text) {
            try {
              const json = JSON.parse(text);
              if (
                json.CaptchaType === "TRACELESS" &&
                json.Success === true &&
                typeof json.StaticPath === "string"
              ) {
                const original = json.StaticPath;
                json.StaticPath = TARGET_STATIC_PATH;
                console.log(`[Patcher] StaticPath patched: ${original} ? ${TARGET_STATIC_PATH}`);
                return JSON.stringify(json);
              }
            } catch (e) {}
            return null; // not patched
          }
        
          // ??? Patch fetch ???????????????????????????????????????????????????????????
          const _fetch = window.fetch;
          window.fetch = async function(...args) {
            const response = await _fetch.apply(this, args);
            const clone = response.clone();
            const text = await clone.text();
            const patched = patchResponse(text);
            if (patched !== null) {
              return new Response(patched, {
                status: response.status,
                statusText: response.statusText,
                headers: response.headers,
              });
            }
            return response;
          };
        
          // ??? Patch XHR ?????????????????????????????????????????????????????????????
          const _open = XMLHttpRequest.prototype.open;
          const _send = XMLHttpRequest.prototype.send;
        
          XMLHttpRequest.prototype.open = function(...args) {
            this._patcherURL = args[1];
            return _open.apply(this, args);
          };
        
          XMLHttpRequest.prototype.send = function(...args) {
            this.addEventListener("readystatechange", function() {
              if (this.readyState === 4) {
                const patched = patchResponse(this.responseText);
                if (patched !== null) {
                  Object.defineProperty(this, "responseText", { get: () => patched });
                  Object.defineProperty(this, "response", { get: () => patched });
                }
              }
            });
            return _send.apply(this, args);
          };
        
          console.log("[Patcher] fetch + XHR response interceptor active.");
        })();  
  


// Script 2

(function() {
    const pattern = /feilin[^/]*\.js$/i; // Matches any filename starting with "feilin" and ending with ".js"

    // 1. BLOCK <script src="..."> tags (overrides the src setter)
    const scriptProto = HTMLScriptElement.prototype;
    const originalSrcSetter = Object.getOwnPropertyDescriptor(scriptProto, 'src')?.set;
    if (originalSrcSetter) {
        Object.defineProperty(scriptProto, 'src', {
            set: function(url) {
                const filename = typeof url === 'string' ? url.split('/').pop().split('?')[0] : '';
                if (filename && pattern.test(filename)) {
                    console.log(`[Blocked] Script tag src: ${url}`);
                    return; // Drops the request silently
                }
                originalSrcSetter.call(this, url);
            },
            get: function() { return this.getAttribute('src'); },
            configurable: true,
            enumerable: true
        });
    }

    // 2. BLOCK fetch() and dynamic import()
    const origFetch = window.fetch;
    window.fetch = function(...args) {
        let url = args[0];
        if (typeof url === 'string') {
            const filename = url.split('/').pop().split('?')[0];
            if (pattern.test(filename)) {
                console.log(`[Blocked] Fetch/Import: ${url}`);
                return Promise.reject(new Error('Request blocked by interceptor'));
            }
        }
        return origFetch.call(this, ...args);
    };

    // 3. BLOCK XMLHttpRequest
    const origOpen = XMLHttpRequest.prototype.open;
    XMLHttpRequest.prototype.open = function(method, url, ...rest) {
        if (typeof url === 'string') {
            const filename = url.split('/').pop().split('?')[0];
            if (pattern.test(filename)) {
                console.log(`[Blocked] XHR: ${url}`);
                this._blocked = true;
            }
        }
        return origOpen.call(this, method, url, ...rest);
    };
    const origSend = XMLHttpRequest.prototype.send;
    XMLHttpRequest.prototype.send = function(...args) {
        if (this._blocked) {
            this.abort();
            // Dispatch an error so the website's error handlers are triggered
            this.dispatchEvent(new Event('error', { bubbles: false, cancelable: true }));
            return;
        }
        return origSend.call(this, ...args);
    };

    console.log('✅ Network interceptor active. Blocking "feilin*.js" files.');
})();



// Script 3
// Hook Array.prototype.join to catch the deviceToken assembly
const nativeJoin = Array.prototype.join;
Array.prototype.join = function(separator) {
    const result = nativeJoin.call(this, separator);
    
    // Check if the result matches the deviceToken structure
    if (result.includes('SG_WEB_PREID') && separator === '#') {
        console.log('%c[Hook] deviceToken assembly caught!', 'color: #fff; background: #007acc; font-size: 14px; padding: 2px 4px;');
        console.log('Separator:', separator);
        console.table(this); // Displays the 5 parts cleanly
        console.trace('Callstack for deviceToken generation:');
        debugger; // Pause execution
    }
    
    return result;
};
