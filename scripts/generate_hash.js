// ============================================================
// Aliyun Captcha VM Hash — Full Reverse Engineering Notes
// ============================================================
//
// ENTRY POINT called by the page:
//   er = P(0, [], F, V, ez, [er, (td && td)(77, 19)]) + er
//   where er = input JSON string, td = the salt token (e.g. '0000')
//   ez = { r: 1 }  (means "return the pop value")
//
// P / t  is a stack-based virtual machine. Its opcodes live in F[]
// and its constant pool lives in V[].
//
// ─────────────────────────────────────────────────────────────
// IS THIS MD5?  NO.
// ─────────────────────────────────────────────────────────────
// window.__ALIYUN_CRYPT is undefined, so no external crypto lib
// is in play. The output is 32 hex chars like MD5 but the algo
// is entirely different — a custom 16-byte stateful stream cipher.
//
// ─────────────────────────────────────────────────────────────
// The Salt Mystery
// ─────────────────────────────────────────────────────────────
// The salt string is most likely passed as-is to the y() function (argument[1]).
// The KSA uses r.charCodeAt(i % m) on the RAW STRING, NOT parseInt.
// The "salt doesn't matter when it's '0000'/'000'/'0'" observation
// is explained purely by the KSA arithmetic: when every salt byte
// is '0' (charCode 48), those 48s are added into j then masked to
// (f-1)=15, which happens to produce the same permutation as any
// other all-same-byte salt that maps to the same residue mod 16.
// '0010' breaks it because charCode('1')=49 ≠ charCode('0')=48.
//
// ─────────────────────────────────────────────────────────────
// ALGORITHM  (decompiled from VM bytecodes, exact formulae)
// ─────────────────────────────────────────────────────────────
//
// function aliHash(inputStr, saltStr):
//
//   Phase 0 — UTF-8 encode  [PC 16007-16025]
//     o = unescape(encodeURIComponent(inputStr))
//     a = o.length          // byte length of UTF-8 encoded input
//     r = saltStr
//     m = r.length          // salt string length
//
//   Phase 1 — State init  [PC 16067-16135]
//     e[i] = (i << 4) + (i % 16)   for i in [0..15]
//     → e = [0, 17, 34, 51, 68, 85, 102, 119, 136, 153, 170, 187, 204, 221, 238, 255]
//     f = 16                // state size
//
//   Phase 2 — KSA (Key Schedule)  [PC 16139-16310]
//     i = 0, j = 0
//     while i < f:
//       j = (((i + j + e[i] + e[j]) >> 1) + r.charCodeAt(i % m)) & (f-1)
//       swap(e[i], e[j])
//       i++
//
//   Phase 3 — PRGA (stream processing)  [PC 16313-16589]
//     // local loop counter shadows outer 'r' (salt); 'o' (input str) still visible
//     idx = 0, p = 0, q = 0
//     while idx < a:          // a = UTF-8 input length
//       q   = ((p ^ q) + (e[p] ^ e[q])) & (f-1)
//       swap(e[p], e[q])
//       C   = o.charCodeAt(idx)
//       C   = (C + p + q) ^ e[p] ^ e[q]    // uses POST-swap e[p] and e[q]
//       C   = C & 255
//       e[p] = C
//       p   = (p + 1) & (f-1)
//       idx++
//
//   Phase 4 — Final diffusion  [PC 16592-16743]
//     for step in [0 .. 2*f-1]:
//       pos = step % f
//       if pos != 0:  e[pos] ^= e[pos-1]
//       else:         e[0]   ^= e[f-1]
//
//   Phase 5 — Hex encode  [PC 16744-16814]
//     return e.map(b => (b < 16 ? '0' : '') + b.toString(16)).join('')
//
// ─────────────────────────────────────────────────────────────
// IMPLEMENTATION
// ─────────────────────────────────────────────────────────────

function aliHash(inputStr, saltStr) {
    // Phase 0: UTF-8 encode
    var o = unescape(encodeURIComponent(inputStr));
    var a = o.length;
    var r = saltStr;
    var m = r.length;

    // Phase 1: State init  e[i] = (i<<4) + (i%16)
    var e = [], f;
    for (var _i = 0; _i < 16; _i++) {
        e.push((_i << 4) + (_i % 16));
    }
    f = e.length; // 16

    // Phase 2: KSA
    var i = 0, j = 0, tmp;
    while (i < f) {
        j = (((i + j + e[i] + e[j]) >> 1) + r.charCodeAt(i % m)) & (f - 1);
        tmp = e[i]; e[i] = e[j]; e[j] = tmp;
        i++;
    }

    // Phase 3: PRGA
    var idx = 0, p = 0, q = 0, C;
    while (idx < a) {
        q = ((p ^ q) + (e[p] ^ e[q])) & (f - 1);
        tmp = e[p]; e[p] = e[q]; e[q] = tmp;
        C = o.charCodeAt(idx);
        C = (C + p + q) ^ e[p] ^ e[q];
        C = C & 255;
        e[p] = C;
        p = (p + 1) & (f - 1);
        idx++;
    }

    // Phase 4: Final diffusion
    for (var step = 0; step < 2 * f; step++) {
        var pos = step % f;
        if (pos !== 0) {
            e[pos] = e[pos] ^ e[pos - 1];
        } else {
            e[0] = e[0] ^ e[f - 1];
        }
    }

    // Phase 5: Hex encode
    return e.map(function(b) {
        return (b < 16 ? '0' : '') + b.toString(16);
    }).join('');
}

// ─────────────────────────────────────────────────────────────
// VERIFICATION
// ─────────────────────────────────────────────────────────────

var input = '{"TrackList":{"mc":"","tc":"","mu":"","te":"","mp":"","tmv":"","ks":"","fi":"","startTime":1782100652835},"TrackStartTime":1782100652835,"VerifyTime":1782100652862,"arg":"JjObDGdh/ywcWQ=="}';
var expected = '9333ef7396dd56dbb9d6e8f31e8f6014';

console.log('=== Verification ===');
console.log('Expected : ' + expected);

// All zero-byte salts produce the same result (charCode '0'=48, all map same)
['0000', '000', '0'].forEach(function(s) {
    var res = aliHash(input, s);
    console.log('salt=' + JSON.stringify(s).padEnd(20) + ' -> ' + res + '  PASS=' + (res === expected));
});

// Salts with non-zero digits break it (charCode differs)
['0010', '0000000000001', '1'].forEach(function(s) {
    var res = aliHash(input, s);
    console.log('salt=' + JSON.stringify(s).padEnd(20) + ' -> ' + res + '  PASS=' + (res === expected) + ' (expected different)');
});

// ─────────────────────────────────────────────────────────────
// INTERMEDIATE STATE TRACE (for the given example, salt='0000')
// ─────────────────────────────────────────────────────────────
console.log('\n=== Intermediate state trace (salt="0000") ===');

function aliHashTrace(inputStr, saltStr) {
    var o = unescape(encodeURIComponent(inputStr));
    var a = o.length;
    var r = saltStr;
    var m = r.length;

    var e = [], f;
    for (var _i = 0; _i < 16; _i++) e.push((_i << 4) + (_i % 16));
    f = e.length;

    console.log('Phase 1 - initial e: [' + e + ']');
    console.log('  f=' + f + ', a (UTF-8 len)=' + a + ', m (salt len)=' + m);

    var i = 0, j = 0, tmp;
    while (i < f) {
        j = (((i + j + e[i] + e[j]) >> 1) + r.charCodeAt(i % m)) & (f - 1);
        tmp = e[i]; e[i] = e[j]; e[j] = tmp;
        i++;
    }
    console.log('Phase 2 - after KSA:  [' + e + ']');

    var idx = 0, p = 0, q = 0, C;
    while (idx < a) {
        q = ((p ^ q) + (e[p] ^ e[q])) & (f - 1);
        tmp = e[p]; e[p] = e[q]; e[q] = tmp;
        C = o.charCodeAt(idx);
        C = (C + p + q) ^ e[p] ^ e[q];
        C = C & 255;
        e[p] = C;
        p = (p + 1) & (f - 1);
        idx++;
    }
    console.log('Phase 3 - after PRGA: [' + e + ']');

    for (var step = 0; step < 2 * f; step++) {
        var pos = step % f;
        if (pos !== 0) e[pos] ^= e[pos - 1];
        else           e[0]   ^= e[f - 1];
    }
    console.log('Phase 4 - after diff: [' + e + ']');

    return e.map(function(b) { return (b<16?'0':'') + b.toString(16); }).join('');
}

var traced = aliHashTrace(input, '0000');
console.log('Final:  ' + traced);
