// ================================================================
// Aliyun CAPTCHA — arg field generation
// Decompiled from: P.apply(void 0, v)
// v = [0, [], bytecode, V_pool, {r:1}, [certifyId, constant]]
// ================================================================

function generateArg(certifyId, constant) {
    // ── Variable setup (PC 0–530) ─────────────────────────────────
    // h = String  (window.String, used for String.fromCharCode)
    // m = atob, C = btoa, l = encodeURIComponent, s = unescape

    // ── Step 1: UTF-8 encode the certifyId (PC 532–581) ──────────
    var o = unescape(encodeURIComponent(certifyId));  // o = utf8 bytes as string
    var n = constant;                                  // n = "4xrihv8zb8tf1mfj"

    // ── Step 2: Build the 64-element permutation table r (PC 583–718) ──
    // Decoded from the XOR-obfuscated byte arrays in bytecode
    var r = [32,50,10,51,6,44,37,16,46,11,62,19,43,25,23,30,
             60,33,53,34,7,26,12,48,5,2,20,4,61,13,47,49,
             18,29,27,22,1,17,39,56,41,38,55,31,15,58,52,40,
             8,57,45,35,59,36,42,54,63,3,24,28,14,9,0,21];

    // ── Step 3: KSA (PC 719–983) ─────────────────────────────────
    // Same family as the hash KSA but over 64 elements,
    // keyed by the constant string n ("4xrihv8zb8tf1mfj").
    var i = 0, j = 0, tmp;
    while (i < r.length) {
        j = (((i + j + r[i] + r[j]) >> 1) + n.charCodeAt(i % n.length)) & (r.length - 1);
        if (i !== j) {
            // XOR swap (exact bytecode pattern at PC 855–969)
            r[i] = r[i] ^ r[j];
            r[j] = r[i] ^ r[j];
            r[i] = r[i] ^ r[j];
        }
        i++;
    }

    // ── Step 4: PRGA — stream-encrypt the input (PC 984–1472) ────
    var t = '';  // accumulator (outer 't' from PC 984–989)
    var e = 0, a = 0, m;
    for (var idx = 0; idx < o.length; idx++) {
        // Update q-pointer (a)
        a = ((e ^ a) + (r[e] ^ r[a])) & (r.length - 1);
        // XOR-swap r[e] and r[a] if different
        if (e !== a) {
            r[e] = r[e] ^ r[a];
            r[a] = r[e] ^ r[a];
            r[e] = r[e] ^ r[a];
        }
        // Compute output byte
        m = o.charCodeAt(idx);
        m = m + e + r[e] - a - r[a];
        m = m ^ (r[e] + r[a]);
        m = m ^ r[(r[e] + r[a]) & (r.length - 1)];
        m = m & 255;
        t += String.fromCharCode(m);
        // Advance p-pointer (e)
        e = (e + 1) & (r.length - 1);
    }

    // ── Step 5: base64-encode the result (PC 1473–1496) ──────────
    return btoa(t);
}

// ================================================================
// VERIFICATION
// ================================================================
// v[5] = ["certID", "4xrihv8zb8tf1mfj"]
var certifyId = "HtnQAvpxNp";
var constant  = "4xrihv8zb8tf1mfj";

var result = generateArg(certifyId, constant);
console.log("certifyId : " + certifyId);
console.log("constant  : " + constant);
console.log("Result    : " + result);
