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
  
