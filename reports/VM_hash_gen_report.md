# Complete VM Intelligence Report
`Bytecode: F`
`Interpreter: P`
`String Pool: V`
`PC:0`
`ez: {r:1}`

## The VM dispatch machine, explained
The function `t(n, e, r, i, a, o)` is a **stack-based bytecode interpreter**. Its instruction set:

| Opcode | Mnemonic | What it does? |
|--------|----------|---------------|
| `43` | `PUSH_C` | Push `V[r[n++]]` (a constant from the pool) |
| `33` | `LOAD_VAR` | Push `scope[V[idx]]` | 
| `27` | `SET_PROP` | `scope[key] = value` |
| `56` | `GET_PROP` | Push `obj[key]` |
| `50` | `CALL_M` | Call method: `obj[method](...args)` |
| `55` |`BUILD_ARR` | Pop N items → push Array |
| `7` | `MAKE_FUNC` | Create closure at bytecode entry-point |
| `37/6` | CALL_WH/IFCall | sub-block (looping or once) |
| `23/20/38`| `XOR/AND/OR` | Bitwise ops | 
| `49` | `BIND_ARGS` | Bind function parameters from stack into scope |

The `CALL_WH`/`CALL_IF` pattern is how all loops are encoded — a sub-function is called repeatedly until it returns a non-`undefined` sentinel (`RET_VAL inline_val=1`).

---

## Full algorithm, decompiled from bytecode
The VM runs `c.y(o, r)` (PC 15989–16838) where `o` = JSON input string, `r` = salt string.
### **Phase 1 — UTF-8 encode** (PC 16007)
```js
o = unescape(encodeURIComponent(inputStr));  // multi-byte → byte string
a = o.length;   // UTF-8 byte length
m = r.length;   // salt string length
```
### **Phase 2 — State init** (PC 16067): 16-byte S-box seeded as `e[i] = (i << 4) + (i % 16)` → `[0, 17, 34 … 255]`. `f = 16`.
### **Phase 3 — KSA** (PC 16139): a non-standard key schedule using both `e[i]` and `e[j]`:
```js
j = (((i + j + e[i] + e[j]) >> 1) + r.charCodeAt(i % m)) & (f-1);
swap(e[i], e[j]);
```
### **Phase 4 — PRGA** (PC 16313): stream absorption with the input bytes:
```js
q = ((p ^ q) + (e[p] ^ e[q])) & (f-1);
swap(e[p], e[q]);
C = o.charCodeAt(idx);
C = (C + p + q) ^ e[p] ^ e[q];   // post-swap e[p], e[q]
C = C & 255;
e[p] = C;
p = (p + 1) & (f-1);
```
### **Phase 5 — Final diffusion** (PC 16592): a chain XOR pass done twice:
```js
for step in [0 .. 2f-1]:
    pos = step % f
    if pos:  e[pos] ^= e[pos-1]
    else:    e[0]   ^= e[f-1]
```
### **Phase 6 — Hex encode**: `e.map(b => (b<16?'0':'')+b.toString(16)).join('')`

---
## The salt "mystery" solved
The salt is used raw as a string via `r.charCodeAt(i % m)` — **no `parseInt` is ever called inside the hash**. The "all-zeros salts give the same result" observation is pure arithmetic:
- `charCodeAt('0') = 48`. When every salt byte is `48`, the KSA adds `48` at each step and masks to `& 15`, which gives a **fixed permutation** regardless of how many zeros you use.
- `'0010'` has charCode `49` at position 2, which shifts `j` differently → different permutation → different hash.
- `'0000000000001'` has a `'1'` at the end, but because `i % m` cycles through all positions including that last one, the `49` gets incorporated.

---

#### Intermediate state proof
```
Phase 1 init:  [0, 17, 34, 51, 68, 85, 102, 119, 136, 153, 170, 187, 204, 221, 238, 255]
Phase 2 KSA:   [204, 136, 238, 51, 34, 170, 17, 68, 85, 187, 102, 119, 0, 153, 255, 221]
Phase 3 PRGA:  [124, 71, 124, 64, 121, 174, 192, 6, 239, 13, 81, 37, 246, 124, 126, 155]
Phase 4 diff:  [147, 51, 239, 115, 150, 221, 86, 219, 185, 214, 232, 243, 30, 143, 96, 20]
Hex output:    9333ef7396dd56dbb9d6e8f31e8f6014  ✓
```

## Architecture Overview
The bytecode is split into two completely separate concerns inside one function body:
```
PC 0       → JUMP to 15172     ← skip the fingerprint fn body
PC 8-13    → MAKE_FUNC C, entry=15   ← define fingerprint collector as scope.C
PC 15-15171 → C() body         ← the entire bot detection pipeline (~15,000 bytecodes)
PC 15172   → call C(), check result
PC 15980   → define y(), call y(input, salt) → the hash
PC 16833   → return result
```
`C()` is the fingerprinting function. `y()` is the hash function i already decoded. They connect at one point: if `C()` returns falsy (integer `0`, meaning clean browser), `y(input, salt)` runs and produces our hash. If `C()` returns truthy (bot detected or cached key), a different path executes.

---

## The C() Return Contract
| C() returns | Meaning | What happens |
|-------------|---------|--------------|
| `0` | Clean browser, no tools detected | `y(input, salt)` runs → normal hash |
| `207` | Bot tool detected (Hlclient/Sekiro) | Path tries `207[a[last]]` which crashes → still falls through to `y()`, but server-side flag is encoded in the submitted data |
| `__ALIYUN_CRYPT` object |  cached AES key exists | Pulls cached key from `a[a.length-1]`, formats as hex, uses that instead of computing fresh |

This means the **hash is deterministic and always computed the same way** when __ALIYUN_CRYPT is absent. The bot detection result doesn't change the 32-char output directly — it influences the AES layer upstream (the data field encryption i traced earlier), not this hash.

----

## Full Fingerprint Signal Map
**178 decoded strings across 31 probe groups.** Every string is XOR-obfuscated with a different key, each decode function defined inline as a fresh closure. Here is every signal grouped by what it actually detects:

## Group 1 — Prototype chain integrity (PC 617–10103)
Checks `typeof`, `toString()`, and `instanceof` for 16 browser object classes. Each check compares the live result against a hardcoded expected string:
| **Object** | **Expected toString** | **Why it catches bots** |
|------------|-----------------------|-------------------------|
| `window` | `[object Window]` |  PhantomJS returned `[object global]` |
| `document` | `[object HTMLDocument]` | jsdom returns `[object HTMLDocument]` correctly but proto chain differs |
| `document` proto | `[object DocumentPrototype]` | PhantomJS/old WebKit had wrong proto label |
| `navigator` | `[object Navigator]` | Headless Chrome with `--disable-features` can differ|
| `screen` | `[object Screen]` | Headless Chrome returns `[object Object]` on Screen proto |
| `Element` | `[object Element]` | Puppeteer/JSDOM proto chain diverges here |
| `HTMLElement` | `[object HTMLElement]` | jsdom passes but `HTMLElementPrototype check fails |
| `HTMLHeadElement` | `[object HTMLHeadElement]`|  Subclass depth test — very rarely emulated correctly |
| `HTMLBodyElement` | `[object HTMLBodyElement]` | Same |
| `Audio` | `[object HTMLAudioElement]` | Not available in Firefox headless without audio subsystem |
| `Image` | `[object HTMLImageElement]` | Constructor check |
| `HTMLScriptElement` | `[object HTMLScriptElement]` | 
| `Node` | `[object Node]` | Core class, but NodePrototype string check catches edge cases |
| `Text` | `[object Text]` | DOM text node — absent in many lightweight |
| `runtimesStyleSheet` | `[object StyleSheet]` | CSS layer — almost never emulated in headless |
| `Comment` | `[object Comment]` |CommentPrototype toString almost always wrong in fakes |

Each probe also checks `onmousedown`, `onmouseup`, `onkeydown`, `onkeyup` handler presence, plus `MouseEvent` and `KeyboardEvent` class + prototype chains.

## Group 2 — Live DOM manipulation (PC 12065–13691)
This is the most reliable signal set. Unlike static class checks, these actually **build and query DOM nodes** and verify the browser produces correct output:

## Check 1 — innerHTML comment collapse (PC 12065):
```js
var el = document.createElement('p');
el.innerHTML = '<!---->--<!---->';
// Real browsers: el.innerText === '--'
// jsdom:         el.innerText === '' or differs
```
## Check 2 — DOM tree serialization (PC 12767):
```js
var div = document.createElement('div');
var inner = document.createElement('div');
var p = document.createElement('p');
div.appendChild(inner);
// insertBefore used to insert p before inner
var clone = div.cloneNode(true);
// Expected outerHTML: '<div><div></div><p></p></div>'
```

Both the live tree `outerHTML` and the `cloneNode()` result are verified. Puppeteer passes this; jsdom often does not due to serialization ordering differences.

## Group 3 — Node.js environment detection (PC 13764–14657)
Three independent checks that will catch any Node.js-based runner that hasn't patched its globals:

## Check 1 — Global object (PC 13764):
```js
typeof window.global !== 'undefined'
// or window.global.toString() === '[object global]'
// or window.process.toString() === '[object process]'
```

In a real browser `window.global` and `window.process` are `undefined`. In Node.js/Puppeteer they are the Node global objects. The VM checks both `typeof` and `toString()` of both.

## Check 2 — Error stack trace (PC 14238):
```js
var stack = new Error().stack;
// Checks for any of these substrings:
// 'at Module._compile (module.js:'      ← Node require() context
// 'internal/modules/cjs/loader'         ← Node CJS loader
// 'at evalmachine.<anonymous>:'         ← vm.runInContext / eval in Node
```

This is the hardest check to bypass. The stack string reflects the actual call chain. Running inside `vm.Script`, `eval()` inside a Node process, or any Puppeteer `page.evaluate()` call that bubbles through Node will expose one of these strings.

## Group 4 — Known tool blacklist (PC 14747–15167)
Direct window property checks for named tools. Returns sentinel value `207` if any are found:
| **PC** | **Property** | **Tool** |
|-------|--------------|-----------|
| 14747 | `window.chrome` | Chrome extension context / some Puppeteer flags| 
| 14821 | `window.chrome` | Re-checked with different string encoding | 
| 14877 | `window.chrome.tabs` | Chrome extension with tabs API (automation via extension) |
| 14961 | `window.GM_info` | Greasemonkey / Tampermonkey userscript runner |
| 15053 | `window.SekiroClientSekiro` | RPC bridge (common in Chinese bypass setups) |
| 15137 | `window.HlclientHlclient` | RPC injection tool (same ecosystem) |

The `SekiroClient` and `Hlclient` checks are by exact global name. Both tools work by injecting a named client object into `window` to ferry JS execution over a WebSocket to an external controller.

---

## The `__ALIYUN_CRYPT` Path (cached key branch)
When `window.__ALIYUN_CRYPT` exists, `C()` returns it directly (it's a truthy object). Then at PC 15887–15977:

```js
m = C()                        // m = __ALIYUN_CRYPT object
a = m[a[a.length - 1]]         // index into cached key array
a = a.map(b => (b<16?'0':'')+b.toString(16)).join('')  // format as 32-char hex
// skip y(), return a directly
```

This is the **AES key reuse path**. The `__ALIYUN_CRYPT` object holds previously-derived AES keys indexed by some identifier. When the session has an existing key, the hash output is replaced with that cached key formatted as hex. This is why you saw `window.__ALIYUN_CRYPT` being undefined in your test — your fresh session had no cache, so the `y()` path ran instead and produced the custom hash output.

---

## Key Bypass Implications
From the disassembly, these are the signals **most likely to differentiate a bypass from a real browser:**
1. **Error stack trace** (PC 14238) — hardest to fake. Any Node.js-based runner exposes itself here unless you patch `Error.prepareStackTrace` globally before the script runs.
2. **DOM innerHTML collapse** (PC 12065) — requires a real layout engine. You cannot fake `innerText` behavior of comment nodes without a real DOM.
3. `window.global` / `window.process` (PC 13764) — easy to patch: delete `window.process; delete window.global` in `page.evaluateOnNewDocument`.
4. **Prototype toString strings** — patchable per-class, but requires patching all 16 classes including their `Prototype` variants (e.g. HTMLHeadElementPrototype), which Puppeteer's default stealth does not cover.
5. `SekiroClient` / `Hlclient` (PC 15053, 15137) — trivially patchable by renaming the tool's injected variable, but the check tells you Aliyun specifically tracks these two tools by name in this version of the bytecode.
