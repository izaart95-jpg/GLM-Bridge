# Fielin: window.z_um.getToken — `st()` Aliyun device-fingerprint token: reverse-engineering report


## Index

- [1 Executive summary](#1-executive-summary)
- [2 window.z_um.getToken - Definition](#2-windowzumgettoken---definition)
- [3 Deobfuscation](#3-deobfuscation)
- [4 Function cG](#4-function-cg)
- [5 Function i return t holds deviceToken](#5-function-i-return-t-holds-devicetoken)
- [6 VAR T RELATION](#6-var-t-relation)
- [7 PAYLOAD GEN](#7-payload-gen)
- [8 DEOBFUSCATION GENERATOR](#8-deobfuscation-generator)
- [9 Function nD](#9-function-nd)
- [10 The verified `st()` algorithm (as executed in the Node sandbox, region CN)](#10-the-verified-st-algorithm-as-executed-in-the-node-sandbox-region-cn)
- [11 Real-world capture anatomy (why the browser output differs)](#11-real-world-capture-anatomy-why-the-browser-output-differs)
- [12 Test](#12-test)
  - [12.1 NORMAL DEVICE TOKEN:](#121-normal-device-token)
  - [12.2 DECODED:](#122-decoded)
  - [12.3 TEST CALL](#123-test-call)
  - [12.4 RESULTED DECODED TOKEN:](#124-resulted-decoded-token)
  - [12.5 Trailing MD5 component](#125-trailing-md5-component)
  - [12.6 Payload array (variable d) before return a](#126-payload-array-variable-d-before-return-a)
- [13 THE VARS IN THE ARRAY:](#13-the-vars-in-the-array)
  - [13.1 tF = Y[R]](#131-tf--yr)
  - [13.2 Q = tm](#132-q--tm)
  - [13.3 w = rg(T, N)](#133-w--rgt-n)
  - [13.4 tC = h](#134-tc--h)
  - [13.5 p = nK(F)\[(tn(), tn)(1, 33)\]() p = constant for md5 gen](#135-p--nkftn-tn1-33-p--constant-for-md5-gen)
- [14 String table & decoder (`nQ`)](#14-string-table--decoder-nq)
- [15 The 640-byte blob — SOLVED: AES-128-CBC, plus the full AES layer](#15-the-640-byte-blob--solved-aes-128-cbc-plus-the-full-aes-layer)
  - [15.1 Structure](#151-structure)
  - [15.2 The cipher (breakthrough, browser-verified)](#152-the-cipher-breakthrough-browser-verified)
  - [15.3 CBC independently corroborated (same-session blob pair)](#153-cbc-independently-corroborated-same-session-blob-pair)
  - [15.4 The machine pipeline matches (rm/rg = AES-128-CBC)](#154-the-machine-pipeline-matches-rmrg--aes-128-cbc)
  - [15.5 SALT origin — the md5 secret is itself encrypted in the bundle](#155-salt-origin--the-md5-secret-is-itself-encrypted-in-the-bundle)
  - [15.6 deviceConfig origin (InitCaptchaV3 → session key)](#156-deviceconfig-origin-initcaptchav3--session-key)
  - [15.7 Key/IV map](#157-keyiv-map)
  - [15.8 Decrypt recipe (pure Node)](#158-decrypt-recipe-pure-node)
  - [15.9 Historical negative results (why they failed)](#159-historical-negative-results-why-they-failed)
- [16 Blueprint A — reverse a captured payload into its original form](#16-blueprint-a--reverse-a-captured-payload-into-its-original-form)
- [17 Blueprint B — generate payloads in pure Node (no browser)](#17-blueprint-b--generate-payloads-in-pure-node-no-browser)
- [18 Appendix — key source positions (original file coordinates)](#18-appendix--key-source-positions-original-file-coordinates)
- [19 Key constants (runtime-decoded, absent from raw source)](#19-key-constants-runtime-decoded-absent-from-raw-source)
- [20 Standalone reimplementation (reimpl.js)](#20-standalone-reimplementation-reimpljs)
- [21 Reference dump — `AliyunCaptcha.prototype` (live capture, verified end-to-end)](#21-reference-dump--aliyuncaptchaprototype-live-capture-verified-end-to-end)
  - [21.1 What this dump proves (all verified)](#211-what-this-dump-proves-all-verified)
  - [21.2 Decrypted payload field map (`w` plaintext, 140 `#`-fields)](#212-decrypted-payload-field-map-w-plaintext-140--fields)
  - [21.3 Which dump field unlocks what (§16/§17 quick map)](#213-which-dump-field-unlocks-what-1617-quick-map)
  - [21.4 The dump (JSON)](#214-the-dump-json)
  - [21.5 Elided `upLang` i18n strings](#215-elided-uplang-i18n-strings)
  - [21.6 Corrections made to earlier sections by this dump](#216-corrections-made-to-earlier-sections-by-this-dump)

---

## 1 Executive summary

- `st()` returns a `#`-joined 5-field token: `[tF, Q, w, tC, p]`, returned **base64-encoded** (the deviceToken).
- The final field is an integrity hash: `p = MD5([tF, Q, w, tC, secret].join('#'))` with a **hidden constant** `secret = "daye,raolewoba!"` (not present anywhere in the output). **The secret is itself delivered encrypted** — see the next bullet and §15.5.
- **SALT origin (§15.5, verified):** the md5 secret is stored in the bundle as the AES blob `NLAoqT6K03oLbQXW2VS3zA==` and decrypted at load under `ACCESS_SEC` = `FqJB6iRNVYdEGpwb` (fixed IV `0123456789ABCDEF`) → `daye,raolewoba!`. The machine-level site is `n$ = rm(t8.ACCESS_SEC, t8.SALT)` (§18) and `rm()` is the AES-128-CBC **decrypt** primitive (§15.4).
- Verified against a real-world capture (`SG_WEB#3795d2…#<856-char base64>#524#d769460d…`): `MD5("SG_WEB#3795d28242a11619bc25f786f84e53d4-h-1782531783720-ac9e47a76eee443087943a278f191642#<blob>#524#daye,raolewoba!") = d769460d135e774310d665c292c41e95` — exact match.
- **The 640-byte blob is SOLVED (§15.2):** it is **AES-128-CBC, PKCS7** ciphertext. **Key** = the per-session 16-char session key from `AliyunCaptcha.prototype.deviceConfig.deviceConfig.key` (base64-decoded; example `93e513a51c987af1`). **IV** = the fixed literal `0123456789ABCDEF` (its UTF-8 bytes, not hex). The plaintext is the collected device-data JSON.
- **deviceConfig origin (§15.6):** deviceConfig is an encrypted base64 blob returned in the **InitCaptchaV3** response; decrypt it with the bundle **RES** key `87f879f135f27da7` (same IV) → `#`-split fields: `f[0]` = sessionKey, `f[2]` = sessionId, `f[7]` = server timestamp (ms), `f[8]` = session IP. The key it carries is what encrypts the deviceToken ciphertext (`w`).
- **Primitives:** `rm(key, cipherB64)` = AES-128-CBC decrypt (verified on both known inputs — §15.4); `rg(key, data)` = the encrypt counterpart → `w = rg(T, N)` builds the blob (§13.3, §15.4).
- End-to-end pipeline:

  ```
  InitCaptchaV3 ──► deviceConfig b64 blob ──AES-dec(RES key)──► # fields: sessionKey(f0), sessionId(f2), serverTs(f7), ip(f8)
  browser probes ──► b (collected data, .GatherCost) ──"#"-join──► payload string ──rg(T, ·)=AES-enc──► w (640-byte blob; live-decrypt: §21.2)
  SALT blob ──rm(ACCESS_SEC)──► "daye,raolewoba!" ──► p = MD5([tF,Q,w,tC,secret].join('#'))
  token = btoa([tF, Q, w, tC, p].join('#'))
  ```

- Compaction note (this revision): §5's 267-line listing was removed after proving it **byte-identical (whitespace-insensitive) to the inner function embedded in §4** — zero information lost; §4 and §9 were dedented and line-joined at provably safe operator boundaries with **whitespace-stripped equality verified** (token-preserving); §12.2's decoded line was abbreviated after proving it byte-identical to `base64-decode(§12.1)`. No finding, constant, capture, or verification was dropped.

---
## 2 window.z_um.getToken - Definition

```js
// For verify captcha t is certify id but it works even without that
function st(t, r, e) {
            var n, i, a, o, u;
            for (i = 6; i; )
                i < 3 ? i > 1 ? (i -= 1,
                a = function(t, r) {
                    return (cZ || cZ)(t, u.M(r, -1))
                }
                ) : i <= 0 || (i ^= 6,
                o = cG[[a][0](82, 41)](this, 46)) : i <= 4 ? i >= 4 ? (u.C = function(t, r, e) {
                    return t(r, e)
                }
                ,
                i = 2) : !window / !window == 0 ? i = 0 : i -= 3 : i < 7 ? i >= 6 ? (i = 5,
                u = {}) : (i += -1,
                u.M = function(t, r) {
                    return t - r
                }
                ) : (n = o[u.C(a, 12 / (1 | a), 22 / (1 | a))](this, arguments),
                i += -7);
            return n
        }
```

## 3 Deobfuscation

```js
o = cG[[a][0](82, 41)](this, 46))
o = cG.bind(this,46)
n = o[u.C(a, 12 / (1 | a), 22 / (1 | a))](this, arguments)
u.C
function u.C(t, r, e) {return t(r, e)}
a(12,22)
'apply'
o = cG.bind(this,46).apply(this,arguments)
o = cG(46,arguments)
st(t,r,e) = cG(46,t,r,e)
```

---

## 4 Function cG

```js
function cG(t, r, e, n) { var i, a, o, c, s, f, l, h, d, v;
    for (a = 5; a; )
        switch (o = a >> 3, c = 7 & a, o) { case 0: c < 3 ? c < 1 || (c <= 1 ? (a ^= 3, s = r) : (f = e, a ^= 4)) : c >= 6 ? c > 6 ? (a += -7,
            i = function() {
                var t, r, e, n, i, a, o, c, s, f, l, h, d, v, p, b, w, g, m, y, k, M, O, N, S, x, U, I, A, T, R, C, B, F, E, H, Y, q, J, z, j, P, L, V, D, Q, K, Z, G, X, _, W, $, tt, tr, te, tn, ti, ta, to, tu, tc, ts, tf, tl, th, td, tv, tp, tb, tw, tg, tm, ty, tk, tM, tO, tN, tS, tx, tU, tI, tA, tT, tR;
                for (r = 112; r; )
                    switch (e = r >> 6, n = r >> 3 & 7, i = 7 & r, e) { case 0: switch (n) { case 0:
                            i < 1 || (i < 5 ? i >= 4 ? tn ? r += 13 : r += 152 : i < 3 ? i > 1 ? (a = Date[tf.call(5, 81, 63)](),
                            r -= -55) : (r -= -72, o = tf.call(0, 108, 21)) : !tp / !tp == 0 ? r ^= 138 : r = 103 : i <= 6 ? i > 5 ? (r = 64,
                            A = arguments[1]) : (r -= -108, C[S] = o9()) : (c = arguments[({ 0: tf
                            })[0](31, 36)], r ^= 70));
                            break;
                        case 1: i <= 2 ? i >= 2 ? (ta.s = function(t, r) { return t - r
                            }
                            , r -= -60) : i > 0 ? tM ? r += 131 : r += 49 : (r -= -147, s = -96 * d) : i > 5 ? i > 6 ? (r = 11,
                            f = K + (-tf ? 2 : tf)(111, 88)) : (F = void 0 !== arguments[1], r = 63) : i > 4 ? (ta.j(sr, j, to),
                            r = 62) : i <= 3 ? t8[f] ? r = 164 : r -= 9 : (r ^= 58, ta.g = function(t, r) { return t << r
                            }
                            );
                            break;
                        case 2: i <= 3 ? i <= 2 ? i < 1 ? (r -= -89, l = {}) : i < 2 ? (r ^= 177, tn = {}) : (ta.A = function(t, r) { return t && r
                            }
                            , r += 117) : (h = !th, r ^= 80) : i <= 6 ? i > 4 ? i >= 6 ? (r += 75, d = t8[tf.apply(3, [117, 14])]) : (r -= -103,
                            v = t8[J]) : (r -= -31, p = ta.s(57 * T, 57 * d)) : G ? r = 114 : r += 84;
                            break;
                        case 3: i >= 1 ? i <= 2 ? i >= 2 ? (r -= -43,
                            tc = {}) : isNaN(!q * !q) || !q * !q >= 0 ? r += 113 : r -= -12 : i >= 6 ? i >= 7 ? (b = [], r = 89) : (r ^= 12,
                            ta.d = function(t, r) { return t === r
                            }
                            ) : i <= 3 ? (w = ta.j(tf, 121 >> (0 | tf), 21 ^ (0 | tf)), r = 1) : i <= 4 ? (r = 132,
                            l[m] = g) : isNaN(!P * !t8 / (!t8 * !P)) || !P * !t8 / (!t8 * !P) == 1 ? r ^= 82 : r -= -101 : !O / 0 != 1 ? r += 48 : r = 103;
                            break;
                        case 4: i > 1 ? i > 3 ? i >= 6 ? i < 7 ? W ? r = 130 : r += 21 : (r = 95,
                            g = n5(C, null, null, h)) : i > 4 ? (m = N + tf(-tf || 101, -tf || 39), r -= 9) : (r = 151,
                            y = tl[ta.A(tf, tf)(112, 47)](E)) : i >= 3 ? (k = (arguments[tf(~tf ? 31 : 6, ~tf ? 36 : 3)] - 2) * 52,
                            r ^= 168) : tc ? r ^= 91 : r = 26 : i < 1 ? !C * !q / 0 != 1 ? r = 82 : r -= -76 : (r -= -57, M = -((-26 * Date[(tf(),
                            tf)(81, 63)]() - -26 * d) / 26));
                            break;
                        case 5: i >= 7 ? (ta.e = function(t, r) { return t & r
                            }
                            , r += 36) : i >= 4 ? i < 6 ? i <= 4 ? (tU = void 0 !== arguments[0], r += 99) : (O = tf.bind(2, 104, 19)() + z,
                            r += -21) : (r += 90, A = "") : i > 2 ? (tu = te <= 7,
                            r ^= 127) : i > 0 ? i > 1 ? 0 * !V * !C != 9 ? r += 45 : r ^= 12 : (N = tf.bind(6, 121, 21)(), r = 37) : (r += -35,
                            S = td + [tf][0](115, 7));
                            break;
                        case 6: i > 0 ? i > 4 ? i > 5 ? i >= 7 ? !q / 0 != 1 ? r += 47 : r = 35 : (ta.Y = function(t, r) { return t | r
                            }
                            , r += -24) : (x = (~tf ? tf : 0)(88, 75) + tR, r += 8) : i >= 4 ? (U = Y + "h", r ^= 162) : i <= 1 ? (r -= -50,
                            tb = "") : i <= 2 ? (I = ta.K(tI, 40), r = 162) : (r -= -4,
                            q[tf.bind(8, 131, 34)()](tf(94 * (1 | tf), 35 / ta.Y(tf, 1)) + p / 57)) : (V[(-tf ? 8 : tf)(49, 39)] = 501, r += 74);
                            break;
                        case 7: i >= 4 ? i < 6 ? i > 4 ? (r -= 30,
                            q[tf(131 >> (0 | tf), 34 << (0 | tf))](x)) : Math.pow(!tv * !A, 0) ? r ^= 31 : r ^= 71 : i > 6 ? (r += 63,
                            A = F) : (T = Date[tf(81 * (1 | tf), ta.K(63, ta.Y(tf, 1)))](), r -= 42) : i < 2 ? i > 0 ? (R = 50 * a,
                            r = 108) : (l[_] = C, r += 92) : i >= 3 ? (W = 0, r += 71) : (r -= -82, tM = {})
                        }
                        break;
                    case 1: switch (n) { case 0: i < 1 ? !A / 0 != 9 ? r ^= 200 : r += 104 : i > 4 ? i <= 6 ? i <= 5 ? (C = tc, r = 85) : (r ^= 237,
                            ta.K = function(t, r) { return t * r
                            }
                            ) : (B = h, r += 27) : i >= 4 ? (r -= 55, q[tf(~tf ? 131 : 4, ~tf ? 34 : 4)](Q)) : i < 3 ? i <= 1 ? (r += 102,
                            F = c > 1) : (r -= 30, E = function(t) { return "" !== t
                            }
                            ) : (H = t8[ta.j(tf, 110, 74)], r ^= 52);
                            break;
                        case 1: i >= 5 ? i < 6 ? (Y = (ta.I(tf), tf)(124, 81), r -= 25) : i < 7 ? Math.pow(!tS, 0) ? r = 172 : r += -24 : (q = u(P),
                            r = 25) : i <= 2 ? i < 2 ? i <= 0 ? (q[tf(~tf && 131, ~tf && 34)](O), r ^= 218) : (r ^= 92,
                            J = w + tf(ta.g(101, ta.Y(tf, 0)), 39 ^ (0 | tf))) : (r += -29,
                            z = Date[tf(-tf ? 2 : 81, -tf ? 8 : 63)]() - d) : i > 3 ? tU ? r ^= 96 : r = 143 : (j = tr, r = 27);
                            break;
                        case 2:
                            i <= 0 ? Math.pow(!C, 0) ? r ^= 48 : r = 76 : i > 1 ? i >= 5 ? i <= 6 ? i <= 5 ? !C / !C == 0 ? r += -28 : r ^= 3 : (P = t8[ta.j(tf, Math.round(106), Math.floor(82))],
                            r += -57) : (r ^= 222, L = [tf][0](26, 27)) : i > 2 ? i > 3 ? tu ? r -= 10 : r -= 13 : (r ^= 194, ta.I = function(t) {
                                return t()
                            }
                            ) : (r ^= 98, V = {}) : (r += 42, C[tf(85 & ~tf, ta.e(81, ~tf))] = B);
                            break;
                        case 3: i <= 0 ? tr ? r = 142 : r -= -27 : i <= 5 ? i > 2 ? i < 5 ? i > 3 ? (B = 0, r += -11) : (r ^= 47, tx = 0) : (r += 70,
                            D = arguments[tf([31, tf()][0], [36, tf()][0])]) : i > 1 ? (r ^= 30,
                            Q = ta.j(tf, 50, 42) + M) : Math.pow(!b * !btoa, 0) ? r += 20 : r -= -58 : i > 6 ? !g * !n5 / (!n5 * !g) == 0 ? r ^= 66 : r -= 79 : (r = 15,
                            K = ta.j(tf, 38 & ~tf, 46 & ~tf));
                            break;
                        case 4: i <= 2 ? i <= 1 ? i >= 1 ? (r -= -47, Z = tf(~tf && 32, ~tf && 5)) : (G = tv,
                            r += -73) : B ? r = 127 : r ^= 62 : i > 3 ? i < 6 ? i > 4 ? (X = g, r ^= 192) : (r += -44,
                            _ = ti + "ta") : i > 6 ? (W = C[tp], r -= 65) : (r += -63, C[tf((tf(), 70), (tf(),
                            49))] = q[tf(-tf || 76, -tf || 79)]("|")) : (C[({ 0: tf
                            })[0](3, 55)] = tb, r += -19);
                            break;
                        case 5: i < 1 ? (r -= -37, $ = R - tt) : i >= 4 ? i < 6 ? i < 5 ? (r = 104, tt = 50 * d) : (h = !!b,
                            r += -38) : i < 7 ? (r += -22, tr = tw > 27) : (te = ta.O(ty, 7), r -= 68) : i < 3 ? i >= 2 ? (r -= 102,
                            tn = ta.d(ts, void 0)) : (r = 100, ti = tf(32. .valueOf(), 5. .valueOf())) : (r ^= 25, G = "");
                            break;
                        case 6: i <= 0 ? (r += -102, ta = {}) : i <= 4 ? i > 2 ? i > 3 ? (r -= 83, to = tx) : (tr = void 0,
                            r -= 40) : i < 2 ? Math.pow(!C, 0) ? r -= -15 : r -= 31 : (C[[tf][0](39, 37)] = G, r += 20) : i > 5 ? i > 6 ? (tu = !H,
                            r ^= 209) : isNaN(!tu * !Text / (!Text * !tu)) || !tu * !Text / (!Text * !tu) == 1 ? r = 84 : r -= 27 : (tc = t8[tk],
                            r ^= 87);
                            break;
                        case 7:
                            i >= 1 ? i <= 3 ? i < 2 ? isNaN(!tc) || isNaN(!Math) || !tc * !tc + !Math * !Math >= 0 ? r += -52 : r += 18 : i <= 2 ? (V[(~tf ? tf : 8)(57, 44)] = C,
                            r = 42) : (C[(-tf ? 0 : tf)(119, 64)] = Date[tf(~tf && 81, ta.A(~tf, 63))](),
                            r -= 29) : i < 7 ? i <= 5 ? i < 5 ? (r += -18,
                            ts = t8[o + ta.j(tf, [16, tf()][0], [55, tf()][0])]) : (tf = function(t, r) { return cZ(r, t - -5)
                            }
                            , r -= 32) : A ? r = 6 : r -= 80 : (r += -46, B = 1) : tb ? r += 39 : r ^= 253
                        }
                        break;
                    case 2: switch (n) { case 0: i <= 3 ? i < 2 ? i >= 1 ? (r += -63,
                            tl = Object[ta.j(tf, 114 / (1 | tf), 9 * (1 | tf))](C)) : (th = [],
                            r -= 109) : i >= 3 ? !tm / 0 != 8 ? r ^= 42 : r -= -35 : (V[(-tf ? 5 : tf)(75, 22)] = W,
                            r ^= 28) : i > 4 ? i <= 5 ? isNaN(!tb * !Object / (!Object * !tb)) || !tb * !Object / (!Object * !tb) == 1 ? r ^= 180 : r -= 64 : i >= 7 ? (r = 47,
                            ta.O = function(t, r) { return t + r
                            }
                            ) : (r += -94, td = tf(Math.round(63), Math.ceil(70))) : (t8[tf(98 & ~tf, 26 & ~tf)](l), r ^= 225);
                            break;
                        case 1: i < 7 ? i < 1 ? (r = 60, tv = A) : i <= 5 ? i < 4 ? i < 2 ? (tp = ta.O(L, "st"), r = 3) : i < 3 ? (tb = tN,
                            r += -18) : (r += -29, tw = k + 27) : i < 5 ? (r -= -21, C = tM) : (tg = $ / 50, r -= -12) : (r += -67,
                            tr = arguments[2]) : (tm = tU, r -= -14);
                            break;
                        case 2: i >= 4 ? i <= 5 ? i < 5 ? isNaN(!l) || isNaN(!C) || !l * !l + !C * !C >= 0 ? r += -107 : r ^= 222 : (X = v,
                            r = 168) : i >= 7 ? (ty = (y[tf.call(6, 31, 36)] - 80) * 1, r += -40) : (r = 32,
                            C[U] = q[tf(ta.v(76, 0 | tf), 79 >> (0 | tf))]("|")) : i <= 2 ? i <= 1 ? i < 1 ? (tk = Z + "ta",
                            r ^= 229) : (ta.v = function(t, r) { return t >> r
                            }
                            , r ^= 236) : (tM = cK(tN, tv), r = 9) : (tm = arguments[0], r -= -7);
                            break;
                        case 3: i >= 6 ? i > 6 ? 0 * !tb != 4 ? r += -60 : r -= 142 : (r += 6,
                            sn(V)) : i >= 5 ? tm ? r -= 10 : r = 131 : i >= 3 ? i >= 4 ? (r = 160, tn = ts) : (tO = tA - s,
                            r += 15) : i >= 2 ? (tN = tm, r = 7) : i <= 0 ? tx ? r = 116 : r += -61 : (tS = ta.j(tf, -tf ? 2 : 60, -tf ? 9 : 49) + tg,
                            r ^= 215);
                            break;
                        case 4: i > 0 ? i >= 4 ? i > 6 ? F ? r = 14 : r += -104 : i < 5 ? (tx = tT[(tf(), tf)(4, 23)],
                            r += -12) : i <= 5 ? X ? r += 3 : r += -16 : tu ? r = 129 : r += -48 : i >= 2 ? i <= 2 ? (tU = I + 19 > 19,
                            r = 76) : (tI = ta.s(D, 0), r = 50) : (r -= 153, tA = -96 * Date[tf(~tf ? 81 : 4, ~tf ? 63 : 2)]()) : (tT = tn, r ^= 182);
                            break;
                        case 5: i >= 1 ? i <= 2 ? i >= 2 ? (r += -117, tR = -(tO / 96)) : (tm = "", r ^= 51) : i < 4 ? (ta.j = function(t, r, e) {
                                return t(r, e)
                            }
                            , r ^= 167) : (r = 77, q[tf.bind(0, 131, 34)()](tS)) : (r -= 168, t = X)
                        }
                    }
                return t
            }(d, v, l)) : (i = function(t, r, e) { function n(t, r) { return (cZ || cZ)(r, t - 1)
                }
                return sa[(n || n)(24, 12)](this, arguments)
            }(c6, s, f), a += -6) : c <= 3 ? (a -= -4, l = n) : c <= 4 ? 45 === h ? a -= 3 : a ^= 4 : (a -= -4, h = t);
            break;
        case 1: c > 2 ? Math.pow(!blur, 0) ? a += -11 : a += -10 : c < 2 ? c < 1 ? (a += 2, d = r) : 46 === h ? a = 8 : a += -5 : (a ^= 9, v = e)
        }
    return i
}
```

Full source: original file 521,550–527,875 (§18). The inner `i = function(){ … }(d, v, l)` block is the giant machine — analyzed standalone in §5 (deduplicated there) and §6/§7.


---

## 5 Function i return t holds deviceToken

The 267-line listing that stood here — the `var t, r, e, …, tR;` state machine beginning `for (r = 112; r; )` — was **byte-identical, modulo leading whitespace, to the inner function embedded in §4's `cG`** (the `i = function(){ …giant machine… }(d, v, l)` block). Verified by normalized diff before removal (both sides 264 trimmed lines, zero differences); removed as an exact duplicate — no information lost.

- **Invocation:** inside `cG`, `i = function(){ … }(d, v, l)` with the machine's `d = r`, `v = e`, `l = n` — so the inner runs as `i(r, e, n)` = cG's arguments 2–4 = `st(t, r, e)`'s own arguments.
- **Return:** `return t` where `t = X`; `X = v` = `t8['DeviceToken']` (cached path — §6) or `X = g` = `n5(C, null, null, h)` (fresh-generation path — §7).
- **Payload-relevant states, cited where used:** `C = tM` / `tM = cK(tN, tv)` (→ §12.5 answer), `w = rY(JSON.stringify(b))` (→ §11.3), `N = n6(l, 501)` (→ §13.3), `th = [tF, Q, w, tC, n$]` (→ §15.5), `V[(…)]=501` (→ §13.3).
- Full source: embedded in §4 (or original file 521,550–527,875 per §18).

---

## 6 VAR T RELATION

```
t = X
X = v
v = t8[J] = t8['DeviceToken']
```

## 7 PAYLOAD GEN

```
g = n5(C, null, null, h) // C = payload, h = false
g = DeviceToken
```

## 8 DEOBFUSCATION GENERATOR

```js
function n5(t,r,e,n){function i(t,r){return(nV||nV)(t,r- -9)}return nD[i(91,18)](this,13)[(i&&i)(2,8)](this,arguments)}

function nV(t,r){var e,n,i;for(n=3;n;)n<=1?n<1||(i.c=function(t,r){return t-r},n^=3):n>=3?(i={},n-=2):(n-=2,e=(~nQ?nQ:5)(i.c(r,9),t));return e}
i(91,18)
'bind'

i(2,8)
'apply'

nD[i(91, 18)](this, 13)[(i && i)(2, 8)](this, arguments)

nD.bind(this,13).apply(this,arguments)

nD(13,arguments)
```

---

## 9 Function nD

```js
function nD(t, r, e, n, i) {
    var a, o, u, s, f, l, h, d, v, p, b, w, g, m, y, k, M, O, N, S, x, U, I, A, T, R, C, B, F, E, H, Y, q, J, z, j, P, L, V, D, Q, K, Z, G, X, _, W, $, tt, tr, te, tn, ti, ta, to, tu, tc, ts, tf, tl, th, td, tv, tp, tb, tw, tg, tm, ty, tk, tM, tO, tN, tS, tx, tU, tI, tA, tT, tR, tC, tB, tF, tE, tH, tY, tq, tJ;
    for (o = 19; o; )
        switch (u = o >> 6, s = o >> 3 & 7, f = 7 & o, u) { case 0: switch (s) { case 0: f < 2 ? f <= 0 || (l = tY,
                o += 103) : f > 5 ? f >= 7 ? (h = b[tn.call(6, 38, 26)], o += 103) : (d = [tF, Q, w, tC, p], o += 152) : f < 3 ? (o += 16,
                v = tw + "d") : f < 5 ? f > 3 ? 16 === W ? o -= -117 : o = 51 : (p = 0, o -= -137) : (b = r, o -= -104);
                break;
            case 1: ;f > 1 ? f <= 6 ? f >= 4 ? f >= 5 ? f >= 6 ? tm ? o = 69 : o += 114 : (w = rY(JSON[tn(~tn ? 76 : 8, ~tn ? 29 : 5)](b)),
                o ^= 138) : (g = t8[S.X(tn, (tn(), 27), (tn(), 41))], o -= -32) : f < 3 ? (w = rg(T, N), o = 137) : (o ^= 19, tq = S.J(M, ""), tJ = 4,
                m = tq.slice(tq.length - 4).padStart(tJ, "0")) : (y = r, o = 3) : f >= 1 ? (a = rF(X), o = 0) : (k = e, o -= -68);
                break;
            case 2: f >= 6 ? f <= 6 ? o = 17 === W ? 84 : 156 : (o -= 12,
                M = Math[(~tn ? tn : 6)(68, 28)](B)) : f >= 4 ? f < 5 ? (w = w[tn(S.T(-tn, 99), S.T(-tn, 40))](0, 133), o ^= 108) : (O = p << 5,
                o -= -118) : f > 0 ? f < 3 ? f >= 2 ? !v * !tw / 0 == 3 ? o += 44 : o = 101 : (N = n6(b, te), o ^= 27) : (o = 25,
                S = {}) : (x = ti + "EC", o = 126);
                break;
            case 3: f < 4 ? f < 2 ? f >= 1 ? (o += 139, S.u = function(t, r) { return t / r
                }
                ) : isNaN(!m / !m) || !m / !m == 1 ? o ^= 69 : o = 40 : f > 2 ? (U = rg(T, ty), o = 107) : (o += 20,
                I = t8[tn([27, tn()][0], [41, S.O(tn)][0])]) : f <= 4 ? !W * !W + !Function * !Function < 0 ? o ^= 156 : o = 4 : f > 6 ? (o -= -8,
                A = b[({ 0: tn
                })[0](33, 22)](n3)) : f > 5 ? (T = ts, o ^= 49) : (o ^= 29, a = p);
                break;
            case 4: f >= 5 ? f >= 6 ? f > 6 ? (o ^= 39, a = rg(nX, A)) : (p = nK(F)[(tn(), tn)(1, 33)](), o += -32) : (o = 1,
                tY = b) : f > 2 ? f > 3 ? (o = 155, tY = function(t) { var r, e, n, i, a;
                    for (e = 2; e; )
                        e <= 3 ? e <= 1 ? e > 0 && (n = nD[a(18, 91)](this, 15), e ^= 4) : e <= 2 ? (e += 2, i = {}) : (e -= 2, a = function(t, r) {
                            return (-nV ? 3 : nV)(r, i.U(t, -9))
                        }
                        ) : e > 4 ? (e = 0, r = n[({ 0: a
                        })[0](8, 2)](this, arguments)) : (i.U = function(t, r) { return t - r
                        }
                        , e += -1);
                    return r
                }(b)) : (o = 153, R = rR()) : f > 0 ? f < 2 ? (C = L[tn.call(4, 33, 22)]("-"), o += 69) : (B = function(t) { var r, e, n, i;
                    for (e = 1; e; )
                        e <= 0 || (e <= 1 ? (e += 2, n = function(t, r) { return (nV || nV)(r, t - -9)
                        }
                        ) : e < 3 ? (r = i[n(~n ? 8 : 4, 2)](this, arguments), e ^= 2) : (i = nD[n(18 / (1 | n), 91 * (1 | n))](this, 14), e ^= 1));
                    return r
                }(S.J(C, n2)), o = 23) : (o -= -6, F = th[tn(33 & ~tn, 22 & ~tn)](n3));
                break;
            case 5: f <= 2 ? f > 0 ? f <= 1 ? 0 > Math.abs(!p) ? o = 112 : o -= -49 : (o ^= 42, a = w) : (o += -40,
                a = rg(nW, _)) : f <= 3 ? (E = 34 * td, o ^= 74) : f <= 4 ? (o ^= 50, ts = rm(tB, g)) : f >= 6 ? f >= 7 ? (H = S.z(tn, tn)(98, 16),
                o += 80) : (T = S.X(rm, tk, I), o ^= 104) : o = !tv * !Date / 0 != 6 ? 54 : 140;
                break;
            case 6: f <= 5 ? f <= 3 ? f <= 0 ? (Y = t8[S.X(tn, [19, tn()][0], [39, tn()][0])], o += -13) : f >= 2 ? f <= 2 ? (to++,
                o ^= 164) : 21 === W ? o ^= 89 : o = 129 : ts ? o += -19 : o = 149 : f < 5 ? (o -= 21,
                b = r) : !te / !te == 0 ? o = 49 : o += 8 : f <= 6 ? (o = 75, q = n3 + rg(T, S.I(String, tv))) : (J = t8[v], o = 163);
                break;
            case 7: f >= 7 ? (o += 94, z = function(t, r) { var e, n, i, a;
                    for (n = 2; n; )
                        n >= 4 ? i ? n = 0 : n ^= 7 : n < 2 ? n >= 1 && (i = c[S.X(a, 25, 68)](r), n -= -3) : n > 2 ? (tg[t] = "",
                        n ^= 3) : (a = function(t, r) { return tn(r, t - -5)
                        }
                        , n = 1);
                    return e
                }
                ) : f < 2 ? f > 0 ? (o += 58, j = r) : o = 0 * !te == 2 ? 95 : 42 : f >= 6 ? (P = tA / 64,
                o = 154) : f >= 4 ? f > 4 ? o = 511 === te ? 123 : 151 : (delete tg[tU], o = 63) : f >= 3 ? (o ^= 59, a = rF(tc)) : (o -= -33,
                S.J = function(t, r) { return t + r
                }
                )
            }
            break;
        case 1: switch (s) { case 0: f < 1 ? (o = 33, L = [tn.call(6, 10, 10), "h", tH, K]) : f < 2 ? (V = (E - tu) / 34,
                o ^= 201) : f >= 6 ? f >= 7 ? o = !W / 0 != 6 ? 81 : 148 : (D = tn(32. .valueOf(), 8. .valueOf()),
                o -= -46) : f >= 4 ? f >= 5 ? (Q = tm, o = 7) : (K = S.O(rA)[S.J(Z, "ll")]("-", ""), o = 64) : f < 3 ? (Z = tn(26, 37),
                o ^= 6) : (o += -67, a = $[ta]('"', ""));
                break;
            case 1: f >= 3 ? f < 6 ? f > 3 ? f >= 5 ? tY ? o += -40 : o = 36 : (G = n,
                o += 4) : 0 > Math.abs(!q) ? o -= 26 : o += 21 : f >= 7 ? (o ^= 70,
                X = b[tn(~tn ? 36 : 7, ~tn ? 13 : 1)](tp)[tn(33, Math.floor(22))]("-")) : 13 === W ? o -= -69 : o += -56 : f > 0 ? f >= 2 ? (_ = b[tn.bind(9, 33, 22)()](n3),
                o ^= 98) : (W = t, o ^= 22) : (o ^= 22, $ = w[tn(36 & ~tn, S.M(13, ~tn))](function(t) { var r, e, n, i, a;
                    for (e = 2; e; )
                        e <= 0 || (e < 4 ? e < 2 ? (r = JSON[i](t[1])[n(S.u(52, 1 | n), 14 * (1 | n))](n3, ""),
                        e += -1) : e < 3 ? (n = function(t, r) { return tn.apply(7, [t, r - -6])
                        }
                        , e -= -2) : (i = a + "y", e -= 2) : (e -= 1, a = n.call(5, 34, 9)));
                    return r
                })[tn([33, tn()][0], [22, tn()][0])](n3));
                break;
            case 2: f <= 2 ? f <= 0 ? (tt = i, o ^= 96) : f > 1 ? !Q * !Q + !rm * !rm < 0 ? o += 8 : o = 17 : (o += 8,
                b = r) : f >= 4 ? f < 7 ? f < 5 ? (o -= -2, b = r) : f <= 5 ? (o = 133, tr = tn([19, S.O(tn)][0], [25, tn()][0])) : (o = 117,
                te = e) : (tn = function(t, r) { return nQ.bind(6, r - 6, t)()
                }
                , o += -14) : (o -= 67, ti = tn(-tn || 32, -tn || 8));
                break;
            case 3: f > 1 ? f >= 7 ? 18 === W ? o += -24 : o = 145 : f >= 6 ? (ta = tO + "ll", o -= 27) : f > 2 ? f > 4 ? (o -= 93,
                a = S.J(C, m)) : f > 3 ? Math.pow(!S, 0) ? o -= 34 : o -= -10 : (o ^= 56, S.T = function(t, r) { return t || r
                }
                ) : (to = 0, o -= -60) : f < 1 ? (S.t = function(t, r) { return t * r
                }
                , o ^= 34) : (te = e, o -= 6);
                break;
            case 4: f < 4 ? f < 2 ? f > 0 ? (tu = 136, o -= 32) : (o = 59, tc = [Q, w, tT, U, q][tn(-tn || 33, -tn || 22)](n3)) : f <= 2 ? (ts = k,
                o -= 49) : (o ^= 59, S.M = function(t, r) { return t & r
                }
                ) : f > 6 ? (tf = tS + "At", o ^= 227) : f < 6 ? f >= 5 ? (o += 30, tl = t8[D + "EC"]) : (o += -68,
                th = [tF, Q, w, tC, n$]) : (o = 43, td = C[tn(93 * (1 | tn), S.u(38, 1 | tn))]);
                break;
            case 5: f < 1 ? (o += 1, N = n6(l, 501)) : f >= 6 ? f <= 6 ? h ? o = 144 : o += 4 : 19 === W ? o ^= 247 : o = 0 : f <= 1 ? (o += -5,
                w = rg(T, N)) : f < 5 ? f < 4 ? f >= 3 ? (o -= 62,
                tv = Date[(tn && tn)(65, 12)]()) : isNaN(!W) || isNaN(!History) || !W * !W + !History * !History >= 0 ? o -= 101 : o ^= 225 : (o += 4,
                S.O = function(t) { return t()
                }
                ) : (tp = function(t) { var r, e, n, i, a;
                    for (e = 3; e; )
                        e > 1 ? e > 2 ? e >= 4 ? (e -= 3, n = n4(t[1], t[0])) : (i = t[0], e ^= 1) : (a = i + n3, e ^= 6) : e <= 0 || (e += -1,
                        r = a + n);
                    return r
                }
                , o = 79);
                break;
            case 6: f >= 2 ? f > 4 ? f >= 6 ? f <= 6 ? (a = rF(tE), o += -118) : (o = 142,
                tb = w[tn.call(1, 93, 38)]) : o = 0 * !te * !e == 9 ? 124 : 13 : f < 3 ? (o = 144, h = 0) : f > 3 ? (o ^= 118,
                tw = (~tn ? tn : 0)(98, 16)) : (tg = Object[(S.O(tn), tn)(71, 31)]({}, j),
                o = 113) : f > 0 ? Math.pow(!tg * !Object, 0) ? o += -28 : o -= -40 : (o -= -49, S.z = function(t, r) { return t && r
                }
                );
                break;
            case 7: f >= 3 ? f >= 5 ? f > 6 ? (o = 14, tm = G) : f <= 5 ? (o = 27, ty = t8[tn(S.z(~tn, 85), ~tn && 27)]) : (tk = t8[x],
                o -= 100) : f > 3 ? (a = tg, o ^= 124) : (tM = (-tn ? 5 : tn)(19, 25), o = 141) : f >= 1 ? f <= 1 ? (b = r, o += -47) : (o -= 35,
                S.H = function(t, r) { return t - r
                }
                ) : (o += -48, tO = (tn || tn)(26, 37))
            }
            break;
        case 2: switch (s) { case 0: f <= 6 ? f < 4 ? f < 1 ? (tN = t8[tn((S.O(tn), 48), (tn(), 9))],
                o += 15) : f < 2 ? 14 === W ? o ^= 142 : o = 111 : f >= 3 ? !tl * !tl + !t8 * !t8 < 0 ? o += -45 : o -= 76 : (tS = (tn(), tn)(76, 44),
                o ^= 229) : f < 5 ? (tx = y[tf](to), o -= 111) : f <= 5 ? (tU = tr + "st", o += -73) : (o ^= 195,
                tm = rm(tN, tR)) : 504 === te ? o = 56 : o -= 82;
                break;
            case 1: f < 5 ? f >= 2 ? f > 2 ? f >= 4 ? (tI = y[tn(93 * (1 | tn), S.t(38, 1 | tn))], o += 8) : (tA = S.H(64 * O, 64 * p),
                o -= 77) : 20 === W ? o += 24 : o ^= 150 : f <= 0 ? (o = 34, C = C[tn(99, Math.round(40))](0, V)) : (tT = rg(T, uK()),
                o -= 12) : f >= 7 ? (tR = t8[H + "d"], o -= 9) : f >= 6 ? tb > 133 ? o -= 122 : o += -22 : (o ^= 26, delete w[S.J(tM, "st")]);
                break;
            case 2: f < 2 ? f > 0 ? !W * !history / (!history * !W) == 0 ? o ^= 146 : o ^= 223 : (tC = h,
                o = 160) : f >= 4 ? f < 6 ? f > 4 ? (tB = t8[tn.call(9, 48, 9)], o -= 137) : 0 === tI ? o ^= 6 : o = 90 : f >= 7 ? (w = Object[({
                    0: tn
                })[0](15, 35)](w), o = 119) : -99 > S.J(34 * S.H(to, y[S.X(tn, 93, 38)]), -99) ? o += -20 : o += -121 : f <= 2 ? (a = p,
                o = 0) : (b = r, o = 8);
                break;
            case 3: f >= 1 ? f < 5 ? f >= 4 ? 15 === W ? o = 57 : o -= 18 : f >= 2 ? f > 2 ? Math.pow(!tY, 0) ? o -= 154 : o -= 32 : (p = P + tx,
                o += 5) : (o ^= 251, tF = Y[R]) : f >= 7 ? (o = 50, p &= p) : f > 5 ? (o += -40,
                tE = d[tn(~tn ? 33 : 7, ~tn ? 22 : 3)](n3)) : (Object[tn(91, 17)](tg)[S.X(tn, 35, 11)](z), o += -33) : (tH = Date[S.X(tn, 65, 12)](),
                o ^= 218);
                break;
            case 4: f <= 3 ? f > 2 ? (o += -81, Q = rm(tl, J)) : f > 0 ? f > 1 ? isNaN(!W * !W) || !W * !W >= 0 ? o ^= 150 : o += -65 : (o += -69,
                S.I = function(t, r) { return t(r)
                }
                ) : (o += -83, tY = tt) : (o = 108, S.X = function(t, r, e) { return t(r, e)
                }
                )
            }
        }
    return a
}
```

Full source: original file 336,251–342,695 (§18). Var-by-var analysis: §13.


---

## 10 The verified `st()` algorithm (as executed in the Node sandbox, region CN)

```
tF = Y[region]            // Y = WEB_REGION map, decoded via string table: { CN: "WEB", ... }
h  = b["GatherCost"]      // b = collected-data object; undefined when collection is empty
                          // machine branch: h truthy  -> tC = h
                          //                 h falsy   -> (o=144,h=0) forces tC = 0
Q  = null                 // collection field (null in sandbox; real value in browser, see §11)
w  = null                 // payload field (null in sandbox)
th = [tF, Q, w, tC, "daye,raolewoba!"]
F  = th.join("#")         // "WEB###0#daye,raolewoba!"
p  = MD5(F).hex           // e1a2045a96f8b07b7fdc47673fd4700d
d  = [tF, Q, w, tC, p]
tE = d.join("#")          // "WEB###0#e1a2045a96f8b07b7fdc47673fd4700d"
a  = btoa(tE)             // V0VCIyMjMCNlMWEyMDQ1YTk2ZjhiMDdiN2ZkYzQ3NjczZmQ0NzAwZA==
```

Standalone reimplementation (`reimpl.js`) reproduces the sandbox output byte-exactly (`match: true`).

Machine details: `st` → `cG` (GIANT 161-state machine, `cGArgs=[45,"Log2",…]` module-init path) → `n5` → `nD` (state machine, `nDInv=6`). The `h` read is a **decoy**: `b[tn.call(6,38,26)]` = `b["GatherCost"]`; `tn` params are `(t,r)`, so `tn(t,r) = nQ(r-6, t)` — i.e. key `t` applied to table entry `e[r-6]`. In the sandbox `b` has no `GatherCost` property (hasOwnProperty false, no prototype descriptor) → `undefined` → machine forces `h=0`.

## 11 Real-world capture anatomy (why the browser output differs)

Captured (user-supplied, region SG):

```
SG_WEB#3795d28242a11619bc25f786f84e53d4-h-1782531783720-ac9e47a76eee443087943a278f191642#W3ASPlWf…(856 chars)…gvIPw==#524#d769460d135e774310d665c292c41e95
```

| field | sandbox (CN) | real browser (SG) | meaning |
|---|---|---|---|
| `tF` | `WEB` | `SG_WEB` | region prefix |
| `Q` | `null` → `` | `3795d2…-h-1782531783720-ac9e47…` | `uuid1 + "-h-" + Date.now() + "-" + uuid2` |
| `w` | `null` → `` | 640-byte base64 blob | encrypted collected-data payload |
| `tC` | `0` | `524` | `b["GatherCost"]` (truthy branch) |
| `p` | md5 | `d769460d…` | `MD5([tF,Q,w,tC,secret].join('#'))` — **verified** |

Why they differ:

1. **`tF = SG_WEB`** — region string. Verified region logic in source:
   ```js
   function rR(){var t=(rY(JSON.stringify(t8)).ENDPOINTS||[])[0];
     return t&&t.includes("ap-southeast")?"SG":"CN"}
   ```
   Region `SG` (Singapore endpoints) → prefix `SG_WEB`. Sandbox defaulted to `CN` → `WEB`.
2. **`Q`** — the machine builds it as `[id1, "h", Date.now(), id2].join("-")` (`tv=Date[…]()`, `Q=tm`, array state `…,"h",tH,K`); the two 32-hex values are device/session identifiers (one persisted device id, one per-session id), timestamp = epoch millis.
3. **`w`** — the machine's payload state is `w = rY(JSON.stringify(b))` where `b` is the collected device-data object; in the sandbox `b` was empty so `w` stayed `null`. In the browser `w` = the 640-byte encrypted blob — AES-128-CBC of the `#`-joined collected-data string (see §15, live-decrypted in §21).
4. **`tC = 524`** — `b["GatherCost"]` is truthy in the browser (cost/counter of gathered data, here 524); the machine takes the `h truthy → tC = h` branch instead of the forced `h=0` path.

---

## 12 Test

### 12.1 NORMAL DEVICE TOKEN:

```
U0dfV0VCIzM3OTVkMjgyNDJhMTE2MTliYzI1Zjc4NmY4NGU1M2Q0LWgtMTc4MjUzMTc4MzcyMC1hYzllNDdhNzZlZWU0NDMwODc5NDNhMjc4ZjE5MTY0MiNXM0FTUGxXZmx2SWI1YlJWV2RpbnlJdHA1WXIwOGxVMTY1VEs3KzlTWGJYMmlOcTBLU0J3ZUJBTG1hMFlLaFhJNE5iUFVvLzVOeG0xSEsyemRXTHJXQUdGN3FXWWxhSW1xaEtsVkRzbEd4OXdhNnRSSGNIOFlBUWhtNVltWk14ZGM5cCsxSlRpM1FoVDg0YmYvREJpY0tWenZRS2haaUVPaExFM1hQaitCcmViblRzN1cycStBY1BvU2swdldRWnBSL1lIUE5qMkZ0bTRraTJKZnJ3NXNvMTFLTkw1QjN5ZERVbHpLbXV2Tnh0OHZYVHNOMHJtY1ZuRWluT1E1cjMvd2lOVkdrdHpsQ1k3VFR0YTAvUFpVa2VvM2M5N2F6d3dIbE1NZERENEhzREpkYmdTQzRmcE1idGovSGtoUGViWTRVK3orNHJHT3JDclRmSGV2UllyNFlxTGwzZFIvb1pHVFdhanBzRmhoTUoraThTNHN1bmlRZ1dmOXRnK1pMckg4b3hQNFl0TGpCOHZBdk10K1FPNjZSbGFuRjhsK08wZ2gvbVJ0Zmdla1pYaVN3TjdJTVc4dzhwRlE2d2RlOURUeS96Nk5wQ0xNemg3Mm1waHhyMnBHSGpBMTlHL1kweUw5OTBGa2hBNjY5eVJraUdVNWQxeTREWWxEZmNpemU4dVFWY1lLNituckhlSFVJcVlsajFCQjF6QzJvK3NRSmxsWmdORjMxYzNKMHZlRTFONGtyaTdJdzNnYTdsK3BIOWM1bnFVQmFXS1pNRnRvRHpSZnRHWUw4cWhHa0dWaVBadU9FTE0zYlQ3NWdIdmhQTGdsdkRNSmxmeE10Yk1teXZZTis3c2k1eHh2WjhNaFFMeG1VSUJDZ0NLVERrTGVNcm43Z3Y5bzBpaGNFT3JOTGJLZjh2R0VHMnIzSjhWR1pPVnRTSkNkMG53ak9JMlYyOTFSZ2NUazVOTmJWZWp1a2ZEdlNmNmU2eHE2ZjZBVkJpUWE3ZUszMnZVOXZtakVaeWw2d1owVFlTRGdMOVovaTVoQW5NSXJpdzB4aE55eFpVbWpTSThrVmtTRC9lRGg2R3A4UzM1MnlZOWkzZDB0T3FaMWoxWW5yUEZlelA4bmlhUi9ndklQdz09IzUyNCNkNzY5NDYwZDEzNWU3NzQzMTBkNjY1YzI5MmM0MWU5NQ==
```

### 12.2 DECODED:

```
SG_WEB#3795d28242a11619bc25f786f84e53d4-h-1782531783720-ac9e47a76eee443087943a278f191642#W3ASPlWf…gvIPw==#524#d769460d135e774310d665c292c41e95
```
(the 856-char `w` was elided above — this decoded line was **verified byte-identical to base64-decode of the §12.1 token**; full form: `Buffer.from(<§12.1 b64>, 'base64')`)

### 12.3 TEST CALL

```js
nd(13,[],undefined,undefined,false)
```

### 12.4 RESULTED DECODED TOKEN:

```
SG_WEB#3795d28242a11619bc25f786f84e53d4-h-1782531783720-ac9e47a76eee443087943a278f191642##0#160d05bbafcaefb0aa5f74c6164a8a0e
```

### 12.5 Trailing MD5 component

The trailing 32 bit data is md5 and calculated by md5 = [tF, Q, blob, tC, secret].join('#') like this 'SG_WEB#3795d28242a11619bc25f786f84e53d4-h-1782531783720-ac9e47a76eee443087943a278f191642##0#daye,raolewoba!' then appends to it
Looks like C is used to generate encrypted Data and C.GatherCount  is used to generate the GatherCount which is undefined so it sets to 0
Is C the value of cK(tN, tv)?

**Answer (resolved via the machine states, §4/§9):** yes — on the main path `C = tM` and `tM = cK(tN, tv)` (both states in the §4 inner listing), with `C = tc` as the earlier alternate state; `C` is the collected-data payload object that `n5(C, null, null, h)` → `nD(13, C, null, null, h)` receives as `r`, so `b = r = C` and `h = b["GatherCost"]` (the verified string-table name — idx 20, key 38, §14.3; the "GatherCount" in the question above is this same field) → `tC`. The same `C` feeds the encrypted payload: `N = n6(l, 501)` and `w = rg(T, N)` = AES-128-CBC(PKCS7) under session key `T` (§15.4). This also explains why the direct test call `nd(13,[],undefined,undefined,false)` (§12.3) yields `tC=0`: with `r=[]` there is no `GatherCost` property, so the machine forces `h=0` (§10).

### 12.6 Payload array (variable d) before return a

**Before `return a` statement variable d (defined as: d = [tF, Q, w, tC, p]) holds the payload data**
In case of normal call it was like this:

```
[
    "SG_WEB",
    "3795d28242a11619bc25f786f84e53d4-h-1782531783720-ac9e47a76eee443087943a278f191642",
    "W3ASPlWflvIb5bRVWdinyItp5Yr08lU165TK7+9SXbX2iNq0KSBweBALma0YKhXI4NbPUo/5Nxm1HK2zdWLrWAGF7qWYlaImqhKlVDslGx9wa6tRHcH8YAQhm5YmZMxdc9p+1JTi3QhT84bf/DBicKVzvQKhZiEOhLE3XPj+BrebnTs7W2q+AcPoSk0vWQZpR/YHPNj2Ftm4ki2Jfrw5so11KNL5B3ydDUlzKmuvNxt8vXTsN0rmcVnEinOQ5r3/wiNVGktzlCY7TTta0/PZUkeo3c97azwwHlMMdDD4HsAx39qSngjoj9hvRQ50yFDSZx2RM3yFh75F2o+5JVo1iMQ+x5YS7K917IeeQNG9B7VcQZ/MxvPUSS6lsZQzNXZcaA+bcCF1e3K2XOwEw5Amy8vEuoUY7b59RjZW0SYSpjjxOyloQ/7rVzDO+f/t2h/+OhqYi2S9CbgQQ7B9VjSlBJh6t2/3m3hZzc8+9VnU4EKmm4O/fvfsr04WL66aPY7Ujt8yCqArpZc7QGxSFLvhgnamfDK8UtIGsMjrDEdKj4AMeznTQPjv4ujlXzphU24N4SQoTTgvS9Y3TUvffWK7+kH4qhvJiBDT9cB87Mx/P1pqnrEM0PmQZemV8iME/ct1DnCm4EXYuUqEIihf9Kdy/YQGVr4DOGGEsA7LOtFbRKQVAras+AIdchlIidqKZpKUCW5G6e19Tr1uvg3UBfYuBwihre6mI8H23K5wL/xnhyrTR+ctxccdGkXUVp8yT4DQag2r5x8+kHDwDfU3Nt63O9UowFcrxade6vB7Pp6vBOQMsrpKYW2eCnPwMsuLDtvnxgcCUvMVh+XtQ0I2o9P5LQ==",
    524,
    "0f9cdacd5f121f7c0a94c5a57a922b63"
]
```

In case of our test call it was like this:

```
[
    "SG_WEB",
    "3795d28242a11619bc25f786f84e53d4-h-1782531783720-ac9e47a76eee443087943a278f191642",
    null,
    0,
    "160d05bbafcaefb0aa5f74c6164a8a0e"
]
```

---

## 13 THE VARS IN THE ARRAY:

### 13.1 tF = Y[R]

```js
Y = t8[S.X(tn, [19, tn()][0], [39, tn()][0])]

tn(19, 39) = WEB_REGION

Y = t8.WEB_REGION

Y = {"CN": "WEB","SG": "SG_WEB"}

R = 'SG'

tF = 'SG_WEB'
```

### 13.2 Q = tm

```js
Q = tm

tm = rm(tN, tR)

tN = t8[tn(48, 9)]

tn(48,9) = 'ACCESS_SEC'

tN = t8.ACCESS_SEC

FqJB6iRNVYdEGpwb

tR = t8[H + "d"]

H = S.z(tn, tn)(98, 16)

S.z = functio(a,b) {return a || b}

n(98,16)

'sessionI'

tR = t8['sessionId']
```

**Resolved by the live dump (§21):** the token's `Q` is byte-identical to the server-issued `deviceConfig.deviceConfig.sessionId` (`f[2]` of the RES-encrypted deviceConfig), whose format is `<appKey>-h-<server ts ms>-<per-session uuid>` — `uuid1` = **appKey**. `rm(ACCESS_SEC, t8.sessionId)` is the client-side decrypt of the same value; the `[id1,"h",tH,K].join("-")` states build the format, not a competing source.

### 13.3 w = rg(T, N)

```js
ts = rm(tB, g)

tB = t8[tn.call(9, 48, 9)]

tB = t8.ACCESS_SEC

g = t8[S.X(tn, (tn(), 27), (tn(), 41))]

g = t8[tn(27,41) = t8.secretKey

FNW8NwwxZBD3Dagg4V6FIu3Oc2SCmhWgMjILaT1lE9Y=

rm(tB,g)
'e42318c6b13e57fc'

ts =  T

w = rg(ts, N)

N = n6(l, 501)
r = l

function n6(t, r) {
            function e(t, r) {return (nV || nV)(t, r - -7)}
            return nD[e.call(4, 91, 20)](this, 17)[e.apply(9, [2, 10])](this, arguments)
}

e.call(4,91,20) = 'bind', e.apply(9, [2, 10]) = 'apply'
```

**rm()/rg() identified (verified, §15.4):** `rm(key, cipherB64)` = AES-128-CBC **decrypt** (key = first arg as UTF-8 bytes, fixed IV `0123456789ABCDEF`, PKCS7). Verified on both known inputs:

- `rm('FqJB6iRNVYdEGpwb', 'NLAoqT6K03oLbQXW2VS3zA==')` → `daye,raolewoba!` (the SALT/md5 secret — §15.5)
- `rm('FqJB6iRNVYdEGpwb', 'FNW8NwwxZBD3Dagg4V6FIu3Oc2SCmhWgMjILaT1lE9Y=')` → `e42318c6b13e57fc` (= T above)

So `T` is a **decrypted 16-char AES key** (16 UTF-8 bytes → AES-128) and `w = rg(T, N)` is AES-128-CBC **encryption** of the payload `N` — rg is rm's encrypt counterpart, agreeing with the browser-verified blob cipher (§15.2: key = session key from deviceConfig.key). Open correlation: `t8.secretKey` (the `FNW8…` blob — per-session material; the sandbox-session value is shown above) vs `deviceConfig.key` (§15.6) — both deliver a 16-char lowercase-hex session key; same key under two wrappers or two distinct keys is not yet pinned down. The traced `e42318c6b13e57fc` does **not** decrypt the §12 captures (verified negative, §15.2) — those blobs belong to a browser session whose key material was not captured.

### 13.4 tC = h

```js
h = b[tn.call(6, 38, 26)]

r = b

h = b.GatherCost // 0 if b.gatherCost undefined
```

### 13.5 p = nK(F)[(tn(), tn)(1, 33)]() p = constant for md5 gen

---

## 14 String table & decoder (`nQ`)

### 14.1 Decoder (`nQ.eY`) — extracted verbatim from source

```js
var a = "07031d38237e7f0f1c6137052d340139760c7a1a3e1b2908177b2b3c26042f78061f362c7c2a730d7d143d280b1e09163f023b182722002125653a0a7924207719"
      .match(/.{1,2}/g).map(function(t){return parseInt(t,16)});   // 64-byte keyed alphabet
nQ.eY = function(t,r){
  for(var e="",n="",i,o,u=0,c=0;o=t.charAt(c++);
      ~o&&(i=u%4?64*i+o:o,u++%4)&&(e+=String.fromCharCode(255&i>>(-2*u&6)^r)))
    o=a.indexOf(78^o.charCodeAt(0));
  for(var s=0,f=e.length;s<f;s++)n+="%"+("00"+e.charCodeAt(s).toString(16)).slice(-2);
  return decodeURIComponent(n);
};
// nQ(r,n): i = e[r]; return o ? (cache) : i ? nQ.eY(i,n) : undefined   (e = table below)
```

Mechanism: each table char → `6-bit value = alphabet.indexOf(78 ^ char)`; 4 values → 3 bytes (custom keyed base64); every output byte XORed with key `r`; percent-encoded and `decodeURIComponent`d (UTF-8 safe). `nV(t,r) = nQ(r-9, t)`, `tn(t,r) = nQ(r-6, t)`, wrapper `E` similar with different offsets.

### 14.2 Full 38-entry table (raw strings, source order)

```
e[0]="0BHvvLi8SI"        e[19]="UA/Cao5QpAq"
e[1]="I8Q+"              e[20]="YpJ4T2zp5pdUpH"
e[2]="Ygzb5FzV6oc"       e[21]="MB2pIBceMLH"
e[3]="hFzVJgzbrNzlhq"    e[22]="/4Yo"
e[4]="eNHtOT+XeT37wbdEcVE+cVzXev2+rV37O1+ZwgLfrVE"  e[23]="wVHn/4Rf/43l"
e[5]="BpLB52/I4q"        e[24]="K43CyvmHRTh"
e[6]="KukN"              e[25]="/b8iKxIZ"
e[7]="4pUp"              e[26]="YFzn5H"
e[8]="Yo/urCE"           e[27]="Jg+4JFz3rNY"
e[9]="pU584iL04i8"       e[28]="MBcUMMcSSBi"
e[10]="m8hBm8EzvSE"      e[29]="e=0D6g5s6I"
e[11]="cvkxyI"           e[30]="TlQO"
e[12]="mqpTmq2wvQ7"      e[31]="eAjsJCP+6lE"
e[13]="hAMGYN5Na=0qrAlN6oE"  e[32]="cTHVOx2l"
e[14]="Bd0mg0UFpFUYgI"   e[33]="B05BTm0gU0ZhF8"
e[15]="5FJqrFzoYgi"      e[34]="mI7yIIY"
e[16]="4i+RTq"           e[35]="eA+keF+XpA+x"
e[17]="g0lM82L/"         e[36]="roQE"
e[18]="OTRlwq"           e[37]="r=LDJo3"
                         e[38]="Ku8PwH7byS2"
```

### 14.3 Verified decodes (index, key → string)

| idx | raw | key | decode |
|---|---|---|---|
| 1 | `I8Q+` | 76 | `MD5` |
| 2 | `Ygzb5FzV6oc` | 32 | `ACCESS_S` (partial) |
| 3 | `hFzVJgzbrNzlhq` | 48 | `ACCESS_SEC` |
| 13 | `hAMGYN5Na=0qrAlN6oE` | 47 | `__ALIYUN_CRYPT` |
| 16 | `4i+RTq` | 33 | `join` |
| 20 | `YpJ4T2zp5pdUpH` | 38 | `GatherCost` |
| 26 | `YFzn5H` | 50 | `SALT` |
| 27 | `Jg+4JFz3rNY` | 1 | `toString` |
| 33 | `B05BTm0gU0ZhF8` | 19 | `WEB_REGION` |
| 37 | `r=LDJo3` | 62 | `PREID` |

Notes: keys are call-site-specific (e.g. `nV(48,12)` → `e[3]` key 48, `nV(50,35)` → `e[26]` key 50). Many entries are decoys and decode to garbage for the keys actually used; the string cache means only decoded entries matter. None of the obfuscated constants (`FqJB6iRNVYdEGpwb`, `NLAoqT6K03oLbQXW2VS3zA==`, `daye,raolewoba!`) appear in the raw source — all are runtime-decoded.

(The SALT and `t8.secretKey` values are AES-encrypted constants — §15.5/§15.4; `nV(48,12)` → `e[3]` = `ACCESS_SEC`, `nV(50,35)` → `e[26]` = `SALT`.)

---

## 15 The 640-byte blob — SOLVED: AES-128-CBC, plus the full AES layer

### 15.1 Structure

- `base64 -d` → exactly **640 bytes**, 16-byte aligned (**40 AES-sized blocks**).
- Statistical uniformity: 36.7% printable bytes ≈ expected for uniform random (95/256 ≈ 37%) — no text, no JSON (`{"`, `",`, `null`, `true` all absent).
- First 8 bytes `5b 70 12 3e…` — not `Salted__` (so not standard CryptoJS/OpenSSL salted format).
- No zlib/gzip/brotli magic; not plain base64-of-JSON.

### 15.2 The cipher (breakthrough, browser-verified)

**The 640-byte blob is AES-128-CBC, PKCS7.**

- **Key** = the session key from `AliyunCaptcha.prototype.deviceConfig.deviceConfig.key` — a base64 value whose decode is the 16-char ASCII session key (example `93e513a51c987af1`; 16 UTF-8 bytes → AES-128). Note the possible double encoding: `btoa('93e513a51c987af1') = 'OTNlNTEzYTUxYzk4N2FmMQ=='`, `btoa('5ea5a0dd4e774105') = 'NWVhNWEwZGQ0ZTc3NDEwNQ=='`.
- **IV** = the fixed literal `0123456789ABCDEF` — its 16 **UTF-8 bytes** (`30 31 32 … 46`), **not** hex-decoded. Same IV as every other AES use in the bundle (§15.5, §15.6, §13.3).
- Verified live (§21): decrypting the token's `w` field with that session's key yields the collected-data payload — a **`#`-joined field string (140 fields in the live capture), NOT JSON** (see §21.2 for the decrypted field map).

Why the report's own captures still resist: the key is **per-session**, and the §12 captures' session key was not captured. Verified negative in Node: both captured blobs (§12.2's and §12.6's) fail AES-128-CBC/PKCS7 under `93e513a51c987af1`, `5ea5a0dd4e774105`, `e42318c6b13e57fc`, `87f879f135f27da7`, `FqJB6iRNVYdEGpwb`, `daye,raolewoba!` (each as UTF-8 key; the 16-char examples also as hex) — all `bad decrypt`.

### 15.3 CBC independently corroborated (same-session blob pair)

The §12.2 blob and the §12.6-array blob are both 640 bytes and share their **first 224 bytes (14 full blocks) byte-for-byte**, diverging exactly on a block boundary (block 15), with 99.0% of the remaining 416 bytes differing. Under CBC with a fixed IV, that means same key + same IV and plaintexts identical for 224 bytes, first differing inside plaintext block 15 — i.e. the same session re-encrypting a slightly later snapshot of the collected data (a field around plaintext offset 224–239, e.g. a timestamp/counter). ECB or a per-encryption random IV would not produce a block-aligned shared prefix like this.

### 15.4 The machine pipeline matches (rm/rg = AES-128-CBC)

`nD` builds the blob as `w = rg(T, N)` where `T = S.X(rm, tk, I) = rm(t8.ACCESS_SEC, t8.secretKey)` → a 16-char key (§13.3) and `N = n6(l, 501)` is the payload (n6 wraps `nD(17, ·)`). **`rm` = AES-128-CBC decrypt** — verified independently on both known inputs:

- `rm('FqJB6iRNVYdEGpwb', 'NLAoqT6K03oLbQXW2VS3zA==')` → `daye,raolewoba!` (the md5 secret — §15.5)
- `rm('FqJB6iRNVYdEGpwb', 'FNW8NwwxZBD3Dagg4V6FIu3Oc2SCmhWgMjILaT1lE9Y=')` → `e42318c6b13e57fc` (= T, §13.3)

(both with IV `0123456789ABCDEF` UTF-8, PKCS7) — therefore **`rg` = the AES-128-CBC encrypt counterpart**, and `w = AES-128-CBC(key T, fixed IV, PKCS7(N))`, agreeing with §15.2 (browser-verified key = the session key from deviceConfig). Open correlation: `t8.secretKey` (the `FNW8…` blob — per-session material; the traced sandbox session's value is shown in §13.3) vs `deviceConfig.key` (§15.6) — both deliver a 16-char lowercase-hex session key; whether they are the same key under two wrappers (ACCESS_SEC- vs RES-encrypted) or two distinct keys is not yet pinned down. The traced value `e42318c6b13e57fc` does **not** decrypt the §12 captures (verified negative, §15.2) — those blobs belong to a browser session whose key material was not captured.

### 15.5 SALT origin — the md5 secret is itself encrypted in the bundle

The salt mixed into the final md5 is stored in the bundle as the AES blob `NLAoqT6K03oLbQXW2VS3zA==` and decrypted at load under `ACCESS_SEC` (same IV):

```
AES-128-CBC-decrypt(base64 "NLAoqT6K03oLbQXW2VS3zA==",
                    key  "FqJB6iRNVYdEGpwb"    // ACCESS_SEC (t8.ACCESS_SEC)
                    IV   "0123456789ABCDEF")   // utf8 of the literal
  → "daye,raolewoba!"                          // verified in Node
```

- Machine cite: `n$` = `rm(t8[nV(48,12)], t8[nV(50,35)])` = `rm(t8.ACCESS_SEC, t8.SALT)` (§18) — `nV(48,12)` → string-table `e[3]` key 48 = `ACCESS_SEC`; `nV(50,35)` → `e[26]` key 50 = `SALT` (§14.3). Then `th = [tF, Q, w, tC, n$]` (§4 inner) → `F = th.join('#')` → `p = nK(F).toString()` (md5, §13.5).
- The same formula in the breakthrough notation:

  ```js
  const md5 = (s) => crypto.createHash("md5").update(s).digest("hex");
  f5 = md5([f1, f2, f3, f4, SALT].join("#"));   // f1..f4 = tF, Q, w, tC; SALT = "daye,raolewoba!"
  ```

### 15.6 deviceConfig origin (InitCaptchaV3 → session key)

DeviceConfig is an encrypted base64 blob returned in the **InitCaptchaV3 response**, containing device info; the key it carries is used to encrypt the deviceToken ciphertext (`w`). Decrypt it with the bundle **RES** key:

```js
const crypto = require("crypto");
function decryptDeviceConfig(blobB64) {
  const d = crypto.createDecipheriv(
    "aes-128-cbc",
    Buffer.from("87f879f135f27da7"),        // RES key (bundle "RES" const)
    Buffer.from("0123456789ABCDEF")          // IV (utf8 of the literal string)
  );
  const pt = Buffer.concat([d.update(Buffer.from(blobB64, "base64")), d.final()]).toString("utf8");
  return pt.split("#");
  // f[0] = base64(sessionKey 16 chars) → e.g. "5ea5a0dd4e774105"
  // f[2] = sessionId
  // f[7] = server timestamp (ms)
  // f[8] = session ip
}
```

(Snippet roundtrip-verified mechanically in Node; field semantics browser-verified.) `AliyunCaptcha.prototype.deviceConfig.deviceConfig.key` is the runtime property carrying the session key (§15.2). Key-hierarchy separation verified: the RES key does **not** decrypt the SALT or `t8.secretKey` blobs (both `bad decrypt`), and `ACCESS_SEC` does not decrypt deviceConfig's blob — each layer has its own key.

### 15.7 Key/IV map

| material | value (example) | role |
|---|---|---|
| IV — fixed for **all** AES ops | `0123456789ABCDEF` (16 UTF-8 bytes, not hex) | single fixed IV everywhere |
| `ACCESS_SEC` | `FqJB6iRNVYdEGpwb` (16B) | decrypts SALT blob → md5 secret; decrypts `t8.secretKey` → session key T |
| RES key | `87f879f135f27da7` (16B) | decrypts deviceConfig (InitCaptchaV3 response) → session key + device fields |
| SALT blob | `NLAoqT6K03oLbQXW2VS3zA==` | AES blob → `daye,raolewoba!` |
| `t8.secretKey` | `FNW8NwwxZBD3Dagg4V6FIu3Oc2SCmhWgMjILaT1lE9Y=` | AES blob → 16-char key T (`e42318c6b13e57fc`, sandbox-session example) |
| session key (deviceConfig.key / f[0]) | `93e513a51c987af1` / `5ea5a0dd4e774105` (examples) | encrypts the 640-byte blob `w` |
| md5 secret | `daye,raolewoba!` | `p = MD5([tF,Q,w,tC,secret].join('#'))` |

### 15.8 Decrypt recipe (pure Node)

```js
const crypto = require("crypto");
function decryptBlob(wB64, sessionKey) {          // sessionKey = 16-char ASCII
  const d = crypto.createDecipheriv("aes-128-cbc",
    Buffer.from(sessionKey, "utf8"),
    Buffer.from("0123456789ABCDEF", "utf8"));     // IV = utf8 of the literal, NOT hex
  return Buffer.concat([d.update(Buffer.from(wB64, "base64")), d.final()]).toString("utf8");
}
// plaintext = the collected-data payload: a `#`-joined field string (140 fields; see the verified decrypt in §21.2) — not JSON as earlier assumed
// To get the key for an old capture you need that session's deviceConfig
// (InitCaptchaV3 response → §15.6) or its t8.secretKey blob.
```

### 15.9 Historical negative results (why they failed)

All of the following were tried pre-breakthrough and failed (`bad decrypt` / no structure) — now explained: the key is **per-session** (delivered via deviceConfig/secretKey), not any bundle-static constant:

- AES-128-ECB / AES-128-CBC (IV=0, IV=salt, IV=blob[0:16]) with keys: `ACCESS_SEC`, `SALT` (16B), `daye,raolewoba!`, `md5(ACCESS_SEC)`, `md5(secret)`, `md5(SALT64)`, `sha1/sha256` truncated variants, `md5(access+secret)`.
- AES-256 with `sha256(secret)`, `sha256(ACCESS)`, `sha256(SALT64)`, EvpKDF-derived 32+16.
- 3DES / DES with md5-derived keys.
- Repeating-key XOR with all above keys + session fields (`uuid1`, `uuid2`, `md5(uuid1)`, `md5(uuid1+uuid2)`, `md5(Q)`, `md5(ts)`); single-byte XOR best 41.9%.
- IV-prepended ciphertext layouts (`[IV][ct]`, 16+624).
- (Added post-breakthrough, also negative for the captured pair: UTF-8/hex forms of `93e513a51c987af1`, `5ea5a0dd4e774105`, `e42318c6b13e57fc`; `md5(T)`-derived keys; sha256(T); embedded-IV layouts — the captured session's key simply was not captured.)

Earlier conclusion (superseded): *"the blob is ciphertext whose key/IV/mode derive from runtime state inside the page … It cannot be decrypted from the token alone; see §16A step 3."* — correct in spirit (per-session runtime key), now refined by §15.2–15.6: the key is the session key from `deviceConfig.deviceConfig.key` / `t8.secretKey`, both per-session server material.

---

## 16 Blueprint A — reverse a captured payload into its original form

### Step 1 — split & verify (works on any capture, pure Node)

```js
const crypto = require('crypto');
const [tF, Q, w, tC, p] = token.split('#');
const secret = 'daye,raolewoba!';
const F = [tF, Q, w, tC, secret].join('#');
if (crypto.createHash('md5').update(F).digest('hex') !== p) throw new Error('not an st() token');
// token = [tF, Q, w, tC, p].join('#');  structure confirmed
```

### Step 2 — decode `Q`

`Q = <uuid1> -h- <Date.now() ms> - <uuid2>`. `uuid1`/`uuid2` are the device/session identifiers the page embeds (one is the persisted device id — the bundle's `t6`/`device` storage family — the other a per-session id). Which is which is determined by the instrumentation in step 3.

### Step 3 — decrypt `w`

Two routes, in order of preference:

**Route 1 — you captured the session's key material (pure Node, §15.8):** if the capture session's `deviceConfig` (InitCaptchaV3 response) or `t8.secretKey` blob was saved, `w` decrypts directly — sessionKey = base64-decode(`deviceConfig.deviceConfig.key`) (or `rm(ACCESS_SEC, t8.secretKey)` = AES-decrypt the `FNW8…`-style blob, §15.4), then `decryptBlob(w, sessionKey)` (§15.8) → the collected-data JSON. Works offline on old captures; the key is per-session, so a *different* session's key will not open them (verified negative, §15.2).

**Route 2 — instrument the live page (captures a NEW session's key material + plaintext).** In the real page, before the bundle executes (early-injected script, or a patched copy of the bundle hosted locally):

1. Expose `st` and its machine, exactly as in the sandbox harness:
   ```js
   src = src.replace('function st(t,r,e){', 'globalThis.__st=function st(t,r,e){');
   src = src.replace('return n}function sr(t){', 'globalThis.__oName=o.name;return n};function sr(t){');
   src = src.replace('return(cZ||cZ)(', 'return(globalThis.__cZ=cZ)(');
   src = src.replace('function u(t){return function(t){if(Array.isArray(t))return a(t)}(t)||',
                     'function u(t){if(t===undefined)return[];return function(t){if(Array.isArray(t))return a(t)}(t)||');
   ```
   (the last patch is mandatory — every run spreads `undefined` once in the cG machine).
2. Capture the collected data before it is serialized — patch the payload state:
   ```js
   src = src.replace('(w=rY(JSON[tn(~tn?76:8,~tn?29:5)](b)),o^=138)',
     '(globalThis.__wLog=[b,rY,JSON[tn(~tn?76:8,~tn?29:5)](b)],w=rY(JSON[tn(~tn?76:8,~tn?29:5)](b)),o^=138)');
   src = src.replace('(N=n6(l,501),o^=27)', '(globalThis.__NLog=n6(l,501),N=n6(l,501),o^=27)');
   src = src.replace('(b=r,o-=104)', '(globalThis.__bLog=r,b=r,o-=104)');
   ```
   `__wLog[0]` = `b` = **the original form** (the collected device-data object); `__wLog[2]` = its JSON; `__wLog[1]` = the transform function `rY` (dump its `.toString()` and its runtime inputs). Also dump `AliyunCaptcha.prototype.deviceConfig` (its `.key` → base64-decode → session key, §15.2) and the InitCaptchaV3 response body (→ §15.6 `decryptDeviceConfig`).
3. If `rY` wraps `window.__ALIYUN_CRYPT` (CryptoJS — the bundle references it by decoded name `__ALIYUN_CRYPT`), wrap its primitives to log key/IV/mode/plaintext:
   ```js
   const C = window.__ALIYUN_CRYPT;
   for (const m of ['encrypt','decrypt']) if (C.AES && C.AES[m]) {
     const orig = C.AES[m].bind(C.AES);
     C.AES[m] = function(...args){ globalThis.__cryptoLog = args; return orig(...args); };
   }
   ```
   Also hook `CryptoJS.enc.*.parse` if key/IV are passed as strings, and log `SALT`/`ACCESS_SEC` consumers (`t8` object dump: the `tF` constructor wraps the decoded constants — dump `t8`).
4. `plaintext = JSON.parse(decrypt(w, sessionKey))` — the fully reversed original form (sessionKey from `deviceConfig.deviceConfig.key` / `t8.secretKey` — §15.2, §15.8).

---

## 17 Blueprint B — generate payloads in pure Node (no browser)

```js
const crypto = require('crypto');
const IV = Buffer.from('0123456789ABCDEF', 'utf8');        // fixed IV — UTF-8 bytes, not hex
const secret = 'daye,raolewoba!';                          // rm(ACCESS_SEC, SALT blob) — §15.5
const WEB_REGION = { CN: 'WEB', SG: 'SG_WEB' };            // extend with other regions if seen

function encryptBlob(payloadStr, sessionKey) {              // rg() — §15.4; payloadStr = "#"-joined fields
  const c = crypto.createCipheriv('aes-128-cbc', Buffer.from(sessionKey, 'utf8'), IV);
  return Buffer.concat([c.update(Buffer.from(payloadStr, 'utf8')), c.final()]).toString('base64');
}

function makeToken({ region = 'CN', uuid1, uuid2, tC, w }) {
  const tF = WEB_REGION[region];
  const Q = [uuid1, 'h', Date.now(), uuid2].join('-');
  const p = crypto.createHash('md5').update([tF, Q, w, tC, secret].join('#')).digest('hex');
  return [tF, Q, w, tC, p].join('#');
}

// full synthetic flow:
// 1. InitCaptchaV3 → response deviceConfig blob
// 2. sessionKey = base64-decode(decryptDeviceConfig(blob)[0])   (§15.6; or use t8.secretKey → rm(ACCESS_SEC, ·))
// 3. payload = your collected-data string (140 `#`-fields per §21.2 — replay a captured decrypt or synthesize; keep GatherCost consistent)
// 4. w = encryptBlob(b, sessionKey); tC = b["GatherCost"]       (§15.4)
// 5. token = Buffer.from(makeToken({ region, uuid1, uuid2, tC, w })).toString('base64')
```

Requirements, in order of dependency:

1. **`w` (the payload field)** — now fully fabricable: `w = AES-128-CBC-encrypt(sessionKey, fixed IV, PKCS7, payload)` where the payload is the `#`-joined collected-data string (140-field schema in §21.2 — not JSON, see §21.6). For a **server-accepted** token the sessionKey must be the one the server issued for that session (InitCaptchaV3 → deviceConfig `f[0]`); any other 16-char key still yields a structurally valid token (md5 verifies) whose blob the server cannot decrypt.
2. **`b` (the input data)** — the plaintext JSON the real collector builds from browser probes (replay captured `b` values or synthesize; the token is structurally valid either way).
3. **`tC`** — `b["GatherCost"]`; keep consistent with the payload (524 in the capture, `0` when `b` is empty).
4. **`uuid1`/`uuid2`** — your own ids; format 8-4-4-4-12 hex without dashes; `Date.now()` is automatic.
5. **`tF`/region** — match the endpoint region (`ap-southeast` → `SG`).

Completeness checklist: identical `w` transform (AES-128-CBC + session key + fixed IV + PKCS7), identical `b` field schema, `GatherCost` consistency, md5 with the secret — the token is then indistinguishable from a real one.

---

## 18 Appendix — key source positions (original file coordinates)

| item | position |
|---|---|
| `function st` (end of bundle) | ~558,2xx (tail) |
| `cG` GIANT machine | 521,550 – 527,875 |
| `n5` (wraps nD) | 344,884 |
| `nD` state machine | 336,251 – 342,695 |
| `nK` call site (`p=nK(F)[…toString…]()`) | 340,661 |
| `n$` secret build (`rm(t8[nV(48,12)],t8[nV(50,35)])`) | 344,219 |
| `tF` constructor / `new tF` | 170,924 / 182,938 |
| `nQ` + 38-entry table + `eY` decoder | 343,5xx |
| `rR()` region function | 195,5xx |
| Aliyun RPC signing module (`rE`/`rJ`/`rz`, background XHR) | ~194,8xx – 196,5xx |
| `rm`/`rg` AES wrappers (CryptoJS via `__ALIYUN_CRYPT`) | throughout nD/inner (§13.3, §15.4) |
| deviceConfig / InitCaptchaV3 | network response — not a bundle position; see §15.6 |
| RES key const | bundle "RES" constant, runtime-decoded — see §15.6 |

Harness gotchas (reproduced intentionally for future runs):

- `shim.js` replaces `global.process` with `{argv: []}` — capture `process.argv` **before** requiring the shim.
- The bundle schedules a background async XHR (Aliyun RPC request, 4s `Promise.race` timeouts ×2 + 500ms retries) that keeps the process alive ~8.7 s and prints `Error: timeout` — harmless, env-independent.
- Determinism was proven (two fresh VM runs → identical tokens) and env-independence (altered UA/DPR/screen/orientation → byte-identical token and identical machine traces: `nDTrace` 39 states, `giantTrace`, `uUndef` call site).

---

## 19 Key constants (runtime-decoded, absent from raw source)

| constant | value | role |
|---|---|---|
| IV (all AES ops) | `0123456789ABCDEF` (16 UTF-8 bytes, **not** hex-decoded) | fixed IV for every AES use in the bundle |
| `ACCESS_SEC` | `FqJB6iRNVYdEGpwb` (16B) | AES key: decrypts SALT blob → md5 secret; decrypts `t8.secretKey` → session key T (§15) |
| RES key | `87f879f135f27da7` (16B) | AES key: decrypts deviceConfig blob (InitCaptchaV3) → session key + device fields (§15.6) |
| `SALT` | `NLAoqT6K03oLbQXW2VS3zA==` (16B decoded) | AES blob → decrypts to `daye,raolewoba!` (§15.5) |
| `t8.secretKey` | `FNW8NwwxZBD3Dagg4V6FIu3Oc2SCmhWgMjILaT1lE9Y=` (32B) | AES blob → decrypts (ACCESS_SEC) to 16-char key T, e.g. `e42318c6b13e57fc` (§13.3) |
| session key | 16-char lowercase-hex, e.g. `93e513a51c987af1` / `5ea5a0dd4e774105` | encrypts the 640-byte blob `w`; arrives via deviceConfig.key (and/or t8.secretKey) |
| secret | `daye,raolewoba!` | md5 mixing constant (hidden in `F`); = decrypted SALT blob |
| `WEB_REGION[CN]` | `WEB` | token prefix |
| `WEB_REGION[SG]` | `SG_WEB` | token prefix (via `rR()` endpoints) |

---

## 20 Standalone reimplementation (reimpl.js)

```js
const crypto = require('crypto');

const ACCESS_SEC = 'FqJB6iRNVYdEGpwb';
const SALT = 'NLAoqT6K03oLbQXW2VS3zA==';
const WEB_REGION = { CN: 'WEB' };
const REGION = 'CN';

const secret = 'daye,raolewoba!';

function st() {
  const tF = WEB_REGION[REGION];
  const h = 0;
  const th = [tF, null, null, h, secret];
  const F = th.join('#');
  const p = crypto.createHash('md5').update(F).digest('hex');
  const d = [tF, null, null, h, p];
  const tE = d.join('#');
  return Buffer.from(tE).toString('base64');
}

const EXPECTED = 'V0VCIyMjMCNlMWEyMDQ1YTk2ZjhiMDdiN2ZkYzQ3NjczZmQ0NzAwZA==';

const out = st();
console.log('reimpl:', out);
console.log('match :', out === EXPECTED);
if (out !== EXPECTED) process.exit(1);
```

For the full generator (encrypt-capable `w`), see §17; for the decryptors, §15.8/§15.6. This reimpl keeps only the byte-exact CN-sandbox reproduction (verified `match: true`).

---

## 21 Reference dump — `AliyunCaptcha.prototype` (live capture, verified end-to-end)

A live `AliyunCaptcha.prototype` dump (region sgp / `TRACELESS` / SceneId `didk33e0`, feilin build `1.5.1/feilin000.d030213aa…`). **Every crypto claim of this report has been re-verified against this dump in pure Node** — it doubles as a reverse-engineering reference: the field you need for each step of §16/§17 is pointed out inline. Full JSON at the end of this section.

### 21.1 What this dump proves (all verified)

| # | claim | verification |
|---|---|---|
| 1 | token = `btoa([tF, Q, w, tC, p].join('#'))` | `atob(config.DeviceToken)` → 5 fields, exactly `SG_WEB#…#…#3496#95922fe7fba4e8518d86ed88bdd82213` |
| 2 | `p = MD5([f1,f2,f3,f4,SALT].join('#'))`, SALT = `daye,raolewoba!` (same salt as the §12 captures) | Python-verified by the user **and** re-verified in Node: `95922fe7fba4e8518d86ed88bdd82213` — **exact match** |
| 3 | `w` = AES-128-CBC/PKCS7 blob, **key = `deviceConfig.deviceConfig.key`** (`57ad9f73260d1d46`, 16 chars) | decrypts → 625-byte **`#`-joined field string (140 fields), NOT JSON** (§15.2 correction) |
| 4 | deviceConfig blob (`deviceConfig.DeviceConfig`) = AES-128-CBC under **RES key** `87f879f135f27da7` | decrypts → 10 `#`-fields; `f[0]` = `NTdhZDlmNzMyNjBkMWQ0Ng==` → base64-decode → **exactly the session key** |
| 5 | `f[2]` = sessionId = token **Q** | `f[2] === deviceConfig.deviceConfig.sessionId === Q` — byte-identical |
| 6 | `f[7]` = server timestamp (ms), `f[8]` = session IP | `1787994141657` / `223.188.28.68` — match the object's `timestamp`/`ip` fields |
| 7 | `f[3]` = bundle version | `1.5.1/feilin000.d030213aa…` — matches `deviceConfig.deviceConfig.version` |
| 8 | Q's first uuid = **appKey** | `Q.split('-h-')[0] === deviceConfig.appKey` (`3795d28242a11619bc25f786f84e53d4`) |
| 9 | region → `tF` | `ENDPOINTS[0]` contains `ap-southeast` → `tF = SG_WEB` (rR logic, §11.1) |
| 10 | `tC` = GatherCost | `3496` (ms of collection cost; §12 captures: 524 / sandbox: 0) |
| 11 | payload cross-links | contains `saf-captcha` (appName), `768*1366` (screen), `desktop`, `223.188.28.68` (IP), `1787994137531` (initTime), `1787994141657` (dc timestamp), the `logs` array `#`-joined with `\|` separators |

### 21.2 Decrypted payload field map (`w` plaintext, 140 `#`-fields)

| idx | value (this session) | meaning |
|---|---|---|
| 0 | `W.10054` | collector/format version tag |
| 5 | `Linux armv81` | platform |
| 6 | `Chrome` | browser |
| 7 | `149.0.0.0` | browser version |
| 20 | `8` | fontsNum (`preCollectData.fontsNum: 8`) |
| 21 | `OctQh1Jw4wxqnL2N0DHmXA==` | opaque 16-byte probe value (canvas/audio/webgl-class hash, b64) |
| 22 | `4` | probe count/class |
| 32 | `50a98af28ee10a81ef6f31efaae2853b` | 32-hex fingerprint digest |
| 36/37 | `Linux` / `x86_64` | os name / cpu arch |
| 42 | `223.188.28.68` | session IP (= `deviceConfig.deviceConfig.ip`, `f[8]`) |
| 43 | `10-0\|20-1505\|11-1676\|…` | `deviceConfig.logs` `#`-array joined with `\|` (live-growing) |
| 44 | `true` | bool probe |
| 47 | `768*1366` | screen (height*width) |
| 49 | `5` | timezone-ish probe |
| 67 | `saf-captcha` | appName (= `deviceConfig.appName`) |
| 68 | `0` | flag |
| 71 | `4SyHGkKVW8fUJZTYIWWLKAoWeJmau7PQKrOjC8GP` | 40-char token/id (feilin-collector id?) |
| 72 | `1787994137531` | = `deviceConfig.initTime` |
| 73 | `O9l1RIX3GMGG35xDAGAL5y9Zq9vHKgI8LAOBLElIU7` | 42-char companion id |
| 74 | `1787994143355` | second timestamp (≈ dynamicJS load, cf. `logInfo.js.t = 1787994143275`) |
| 75 | `desktop` | device type |
| 76 | `false` | bool probe |
| 78 | `9d4568c009d203ab10e33ea9953a0264` | 32-hex digest #2 |
| 87 | `1787994141657` | = `deviceConfig.deviceConfig.timestamp` (= `f[7]`) |
| 88 | `MCMwIzAjMCMwIzAjMCMwIzAjMSMwIzAjMCMwIzAjMCMwIzAjMCMxIzEjMCMxMTExMTExMDExMTExMTExMTExMTExMTExMQ==` | **feature bitmask, base64 of a `#`-joined bit string** — decodes to `0#0#0#0#0#0#0#0#0#1#0#0#0#0#0#0#0#0#0#1#1#0#11111110111111111111111111` (23 probes) |
| 89/90 | `1` / `1` | flags |
| 91 | `true` | bool probe |
| 109 | `0` | flag |


<details>
<summary><strong>Full decrypt artifacts (click to expand)</strong> — live `w` blob, its 625-byte `#`-joined plaintext, and all 140 fields</summary>

**Live `w` (856-char b64, from the §21.4 DeviceToken):**

```
S7BTao78C17i3237iGGOY6SNMsL40yug1GmKdiAe7b9ceFUTZ0dAF5LNt9ak3H+TQZn4G7Z94hZRdqsNmjCNkhrJOJckzBOixC2TvkF15VKIUtskIOng1vJ669OYYqi+9FHbt3TRJU+gwjCUIqVd4e7NZRqj66ZKPWeXlWUF9m+o0M2oWwkwdTE91A6NcBUPJR/KNwHAoBK75Bmvtip00Jc87mCRsRmxr0Xl1YXxXbJ8KivX9uiRSuTwcoRsWuRpQ8xh8rbcig3igQJe29ACKJWfgr38b2YcZVga69eCw1yJBkSwpGYozZYaXbWsOolRxo9cizRPeUhki7nw12ILlDSiKJNjtX1fdgYin2Cz5LHkigb5krvphY4ZAuZKVWZNWO4etivbapqDYtk42NpWABQogRIXktD2Za06kQsBpgq6+++biLMhm8zyF1EddJaG1Tt6ajKtYiGTEzAU4F3l/HugrjMvGFDrr59ng2hc7rjYQuPQxQs9twSp8ZD6H1kSGS8h7Ltl1gtRrs6O5GBbqS+rnvE5Tn5mPqvy69sffMhzVt2XIBifXEKCvzii9b2CyMo14jg9l5o+iTfTaT38VNwxm0DPUT5b48uD7/D5a4y5E+EmbTowQ7iO27yHZS1OAhiDLiy4oAL16d/WiC4PW+H0ymr1XCCXvUycxYyTk3iKKuq+dDBKHp5+Em1EkYfZrQNHpg3xAoSVG44T6gIRbu/1KWbfJzebtp3T6OZ/THu5Lh+tzzsvfDkH0NQSKLODwD395A7+sIUqt0sU0P+SLZIJie2FkedygrXGkOS5J5z6+lQZzPIsqaHiyxUn1gwR8dOwQM2LBampd4iItTUF2A==
```

**Decrypted `w` plaintext (AES-128-CBC, key `57ad9f73260d1d46`, IV `0123456789ABCDEF`):**

```
W.10054#####Linux armv81#Chrome#149.0.0.0#############8#OctQh1Jw4wxqnL2N0DHmXA==#4##########50a98af28ee10a81ef6f31efaae2853b##8##Linux#x86_64#####223.188.28.68#10-0|20-1505|11-1676|23-2210|30-2221|40-2306|41-5821|70-5824|71-7055|80-7055#true###768*1366##5##################saf-captcha#0###4SyHGkKVW8fUJZTYIWWLKAoWeJmau7PQKrOjC8GP#1787994137531#O9l1RIX7GMGG35xDAGAL5y9Zq9vHKgI8LAOBLElIU7#1787994143355#desktop#false##9d4568c009d203ab10e33ea9953a0264#########1787994141657#MCMwIzAjMCMwIzAjMCMwIzAjMSMwIzAjMCMwIzAjMCMwIzAjMCMxIzEjMCMxMTExMTExMDExMTExMTExMTExMTExMTExMQ==#1#1#true##################0##############################
```

**All 140 fields:**

| idx | value | len |
|---|---|---|
| 0 | `W.10054` | 7 |
| 1 | `` | 0 |
| 2 | `` | 0 |
| 3 | `` | 0 |
| 4 | `` | 0 |
| 5 | `Linux armv81` | 12 |
| 6 | `Chrome` | 6 |
| 7 | `149.0.0.0` | 9 |
| 8 | `` | 0 |
| 9 | `` | 0 |
| 10 | `` | 0 |
| 11 | `` | 0 |
| 12 | `` | 0 |
| 13 | `` | 0 |
| 14 | `` | 0 |
| 15 | `` | 0 |
| 16 | `` | 0 |
| 17 | `` | 0 |
| 18 | `` | 0 |
| 19 | `` | 0 |
| 20 | `8` | 1 |
| 21 | `OctQh1Jw4wxqnL2N0DHmXA==` | 24 |
| 22 | `4` | 1 |
| 23 | `` | 0 |
| 24 | `` | 0 |
| 25 | `` | 0 |
| 26 | `` | 0 |
| 27 | `` | 0 |
| 28 | `` | 0 |
| 29 | `` | 0 |
| 30 | `` | 0 |
| 31 | `` | 0 |
| 32 | `50a98af28ee10a81ef6f31efaae2853b` | 32 |
| 33 | `` | 0 |
| 34 | `8` | 1 |
| 35 | `` | 0 |
| 36 | `Linux` | 5 |
| 37 | `x86_64` | 6 |
| 38 | `` | 0 |
| 39 | `` | 0 |
| 40 | `` | 0 |
| 41 | `` | 0 |
| 42 | `223.188.28.68` | 13 |
| 43 | `10-0|20-1505|11-1676|23-2210|30-2221|40-2306|41-5821|70-5…` | 76 |
| 44 | `true` | 4 |
| 45 | `` | 0 |
| 46 | `` | 0 |
| 47 | `768*1366` | 8 |
| 48 | `` | 0 |
| 49 | `5` | 1 |
| 50 | `` | 0 |
| 51 | `` | 0 |
| 52 | `` | 0 |
| 53 | `` | 0 |
| 54 | `` | 0 |
| 55 | `` | 0 |
| 56 | `` | 0 |
| 57 | `` | 0 |
| 58 | `` | 0 |
| 59 | `` | 0 |
| 60 | `` | 0 |
| 61 | `` | 0 |
| 62 | `` | 0 |
| 63 | `` | 0 |
| 64 | `` | 0 |
| 65 | `` | 0 |
| 66 | `` | 0 |
| 67 | `saf-captcha` | 11 |
| 68 | `0` | 1 |
| 69 | `` | 0 |
| 70 | `` | 0 |
| 71 | `4SyHGkKVW8fUJZTYIWWLKAoWeJmau7PQKrOjC8GP` | 40 |
| 72 | `1787994137531` | 13 |
| 73 | `O9l1RIX7GMGG35xDAGAL5y9Zq9vHKgI8LAOBLElIU7` | 42 |
| 74 | `1787994143355` | 13 |
| 75 | `desktop` | 7 |
| 76 | `false` | 5 |
| 77 | `` | 0 |
| 78 | `9d4568c009d203ab10e33ea9953a0264` | 32 |
| 79 | `` | 0 |
| 80 | `` | 0 |
| 81 | `` | 0 |
| 82 | `` | 0 |
| 83 | `` | 0 |
| 84 | `` | 0 |
| 85 | `` | 0 |
| 86 | `` | 0 |
| 87 | `1787994141657` | 13 |
| 88 | `MCMwIzAjMCMwIzAjMCMwIzAjMSMwIzAjMCMwIzAjMCMwIzAjMCMxIzEjM…` | 96 |
| 89 | `1` | 1 |
| 90 | `1` | 1 |
| 91 | `true` | 4 |
| 92 | `` | 0 |
| 93 | `` | 0 |
| 94 | `` | 0 |
| 95 | `` | 0 |
| 96 | `` | 0 |
| 97 | `` | 0 |
| 98 | `` | 0 |
| 99 | `` | 0 |
| 100 | `` | 0 |
| 101 | `` | 0 |
| 102 | `` | 0 |
| 103 | `` | 0 |
| 104 | `` | 0 |
| 105 | `` | 0 |
| 106 | `` | 0 |
| 107 | `` | 0 |
| 108 | `` | 0 |
| 109 | `0` | 1 |
| 110 | `` | 0 |
| 111 | `` | 0 |
| 112 | `` | 0 |
| 113 | `` | 0 |
| 114 | `` | 0 |
| 115 | `` | 0 |
| 116 | `` | 0 |
| 117 | `` | 0 |
| 118 | `` | 0 |
| 119 | `` | 0 |
| 120 | `` | 0 |
| 121 | `` | 0 |
| 122 | `` | 0 |
| 123 | `` | 0 |
| 124 | `` | 0 |
| 125 | `` | 0 |
| 126 | `` | 0 |
| 127 | `` | 0 |
| 128 | `` | 0 |
| 129 | `` | 0 |
| 130 | `` | 0 |
| 131 | `` | 0 |
| 132 | `` | 0 |
| 133 | `` | 0 |
| 134 | `` | 0 |
| 135 | `` | 0 |
| 136 | `` | 0 |
| 137 | `` | 0 |
| 138 | `` | 0 |
| 139 | `` | 0 |
</details>

(Remaining indices are empty strings — reserved probes. The 140-field schema is the definitive `b`-object layout for §17's generator; field ids per probe registry `o9`.)

### 21.3 Which dump field unlocks what (§16/§17 quick map)

| you need | take from dump | how |
|---|---|---|
| session key (decrypt/encrypt `w`) | `deviceConfig.deviceConfig.key` (or `DeviceConfig` blob `f[0]` → b64-decode) | AES-128-CBC, IV `0123456789ABCDEF`, PKCS7 (§15.8) |
| Q | `deviceConfig.deviceConfig.sessionId` | use as-is (server-issued; `uuid1` = appKey) |
| tF | `deviceConfig.ENDPOINTS[0]` | `includes('ap-southeast') → 'SG_WEB'` else `'WEB'` (rR, §11.1) |
| tC | measured GatherCost | ms of collection (3496 here) |
| SALT | bundle `t8.SALT` blob | `rm(ACCESS_SEC, SALT)` → `daye,raolewoba!` (§15.5) |
| p | compute | `md5([tF,Q,w,tC,SALT].join('#'))` |

### 21.4 The dump (JSON)

```json
{
    "config": {
        "immediate": true,
        "DeviceToken": "U0dfV0VCIzM3OTVkMjgyNDJhMTE2MTliYzI1Zjc4NmY4NGU1M2Q0LWgtMTc4Nzk5NDE0MTY1Ni01MmI1MjhkYTYzNmM0NzI3YTc4NTNhM2Q4MGNhOTEyNSNTN0JUYW83OEMxN2kzMjM3aUdHT1k2U05Nc0w0MHl1ZzFHbUtkaUFlN2I5Y2VGVVRaMGRBRjVMTnQ5YWszSCtUUVpuNEc3Wjk0aFpSZHFzTm1qQ05raHJKT0pja3pCT2l4QzJUdmtGMTVWS0lVdHNrSU9uZzF2SjY2OU9ZWXFpKzlGSGJ0M1RSSlUrZ3dqQ1VJcVZkNGU3TlpScWo2NlpLUFdlWGxXVUY5bStvME0yb1d3a3dkVEU5MUE2TmNCVVBKUi9LTndIQW9CSzc1Qm12dGlwMDBKYzg3bUNSc1JteHIwWGwxWVh4WGJKOEtpdlg5dWlSU3VUd2NvUnNXdVJwUTh4aDhyYmNpZzNpZ1FKZTI5QUNLSldmZ3IzOGIyWWNaVmdhNjllQ3cxeUpCa1N3cEdZb3paWWFYYldzT29sUnhvOWNpelJQZVVoa2k3bncxMklMbERTaUtKTmp0WDFmZGdZaW4yQ3o1TEhraWdiNWtydnBoWTRaQXVaS1ZXWk5XTzRldGl2YmFwcURZdGs0Mk5wV0FCUW9nUklYa3REMlphMDZrUXNCcGdxNisrK2JpTE1obTh6eUYxRWRkSmFHMVR0NmFqS3RZaUdURXpBVTRGM2wvSHVncmpNdkdGRHJyNTluZzJoYzdyallRdVBReFFzOXR3U3A4WkQ2SDFrU0dTOGg3THRsMWd0UnJzNk81R0JicVMrcm52RTVUbjVtUHF2eTY5c2ZmTWh6VnQyWElCaWZYRUtDdnppaTliMkN5TW8xNGpnOWw1bytpVGZUYVQzOFZOd3htMERQVVQ1YjQ4dUQ3L0Q1YTR5NUUrRW1iVG93UTdpTzI3eUhaUzFPQWhpRExpeTRvQUwxNmQvV2lDNFBXK0gweW1yMVhDQ1h2VXljeFl5VGszaUtLdXErZERCS0hwNStFbTFFa1lmWnJRTkhwZzN4QW9TVkc0NFQ2Z0lSYnUvMUtXYmZKemVidHAzVDZPWi9USHU1TGgrdHp6c3ZmRGtIME5RU0tMT0R3RDM5NUE3K3NJVXF0MHNVMFArU0xaSUppZTJGa2VkeWdyWEdrT1M1SjV6NitsUVp6UElzcWFIaXl4VW4xZ3dSOGRPd1FNMkxCYW1wZDRpSXRUVUYyQT09IzM0OTYjOTU5MjJmZTdmYmE0ZTg1MThkODZlZDg4YmRkODIyMTM=",
        "verifyType": "3.0",
        "SceneId": "didk33e0",
        "mode": "popup",
        "element": "#chat-captcha-element",
        "button": "#chat-captcha-trigger",
        "captchaLogoImg": "https://z-cdn.chatglm.cn/z-ai/static/logo.svg",
        "upLang": { "cn": { "…": "…(i18n strings elided — see §21.5)" }, "en": { "…": "…(i18n strings elided — see §21.5)" } },
        "language": "en",
        "timeout": 10000,
        "delayBeforeSuccess": false,
        "region": "sgp",
        "prefix": "no8xfe",
        "isShowErrorTip": true,
        "canInit": true,
        "dynamicJSLoaded": true,
        "imgPreLoaded": false,
        "initBeginTime": 1787994142215,
        "logUploaded": true,
        "logInfo": {
            "sId": "didk33e0",
            "pfx": "no8xfe",
            "mInit": { "t": 1787994144569, "s": true, "msg": "INIT_SUCCESS", "rt": 1253 },
            "hst": "captcha-open-southeast.aliyuncs.com",
            "cId": "XmTPEtGfkZ",
            "js": { "t": 1787994143275, "s": true, "msg": "DYNAMICJS_LOADED", "rt": 644 },
            "rt": 1092
        },
        "apiServers": [ "captcha-open-southeast.aliyuncs.com", "captcha-open-southeast-b.aliyuncs.com" ],
        "_prefix": "no8xfe",
        "urls": [ "https://no8xfe.captcha-open-southeast.aliyuncs.com/", "https://no8xfe.captcha-open-southeast-b.aliyuncs.com/" ],
        "initialRequestTime": 1787994142629,
        "overTime": false,
        "imgServer": "https://static-captcha-sgp.aliyuncs.com/",
        "CaptchaType": "TRACELESS",
        "Image": "",
        "CaptchaJsPath": "/captcha-frontend/dynamicJS/3.29.0/pe.095.a992fe1e000b46dc.js",
        "CaptchaCssPath": "/captcha-frontend/dynamicJS/3.29.0/main.css",
        "CertifyId": "XmTPEtGfkZ",
        "Question": "",
        "PuzzleImage": "",
        "PowVerifyString": "",
        "rem": 1,
        "mainCaptchaType": "TRACELESS",
        "verifyResult": true,
        "verifyCode": "T001",
        "securityToken": "6oOo7e72nA61uVLiZVKiLYqF1m9rOno3vEIPJKaL7KLxCJqb1UBwRpl4p7EcFTgdGxVR5HQiakBOtjaqnKYsCNjPcqaflqbQLZQdX2rYd/8bhnqhIpC7SnRlIxGPsqvX"
    },
    "deviceConfig": {
        "prefix": "no8xfe",
        "region": "sgp",
        "appName": "saf-captcha",
        "appKey": "3795d28242a11619bc25f786f84e53d4",
        "endpoints": [
            "https://cloudauth-device-dualstack.ap-southeast-1.aliyuncs.com",
            "https://ap-southeast-1.device.saf.aliyuncs.com"
        ],
        "ENDPOINTS": [
            "https://cloudauth-device-dualstack.ap-southeast-1.aliyuncs.com",
            "https://ap-southeast-1.device.saf.aliyuncs.com"
        ],
        "logs": [ "10-0", "20-1505", "11-1676", "23-2210", "30-2221", "40-2306", "41-5821", "70-5824", "71-7055", "80-7055", "81-7061" ],
        "initTime": 1787994137531,
        "preCollectData": { "fontsNum": 8 },
        "sceneId": "didk33e0",
        "DeviceConfig": "v68lCB7R4J5vKIxHwOcyGdxCvsXyvsyQGShJ2ro6SXonCmbm5LziuXoCqbKGHAHJ6X9+iM74XRniKPa6HyAIOKTtUVvi+2ZJ7Lf6Nn5Z6vLYxSl0Ks7g7P0brdOoVINDQJ5E2Poe+Iekpo9HFEi41gfVFV7rq4Wwstny3V30+AHO7kdaEMId9olSaVGhItMWl9Dz+fdLujQ2tkdQymUPXzPRP8o9IOdgZkzK3DusKADb2g/zdioMMFEEI8sUPqkZCuI+v1RczOmXLwjZaA7dD5YsTplOCa10YTDwrVSx8BD4mu0vuY7txhleB5GJCQrJ",
        "deviceConfig": {
            "key": "57ad9f73260d1d46",
            "switch": 1,
            "sessionId": "3795d28242a11619bc25f786f84e53d4-h-1787994141656-52b528da636c4727a7853a3d80ca9125",
            "version": "1.5.1/feilin000.d030213aa800ceb269ffd5211ae00a0f4f18970912e2a40d8802ef3488bdba89",
            "pluginElements": "",
            "pluginResource": "",
            "globalVariable": "",
            "timestamp": "1787994141657",
            "ip": "223.188.28.68"
        },
        "timestamp": "1787994141657",
        "feilinLoad": true
    }
}
```

### 21.5 Elided `upLang` i18n strings

(kept verbatim in the original capture; elided here only for readability — no information lost)

```json
"cn": {
    "START_VERIFY": "点击开始验证",
    "POPUP_TITLE": "请完成安全验证",
    "SLIDE_TIP": "请按住滑块，拖动到最右边",
    "CHECK_BOX_TIP": "确认您不是机器人",
    "PUZZLE_TIP": "请拖动滑块完成拼图",
    "INPAINTING_TIP": "请拖动滑块还原完整图片",
    "VERIFYING": "验证中...",
    "SUCCESS": "滑动成功!",
    "SLIDE_FAIL": "验证失败，请刷新重试",
    "CAPTCHA_FAIL": "验证失败，请重试!",
    "CONGESTION": "前方拥堵，请刷新重试",
    "CAPTCHA_COMPLETED": "滑动完成",
    "FINISH_CAPTCHA": "请先完成验证！"
},
"en": {
    "START_VERIFY": "Click to start verification",
    "POPUP_TITLE": "Please complete security verification",
    "SLIDE_TIP": "Please drag slider right",
    "CHECK_BOX_TIP": "Confirm you are not a robot",
    "PUZZLE_TIP": "Please drag the slider to complete the puzzle",
    "INPAINTING_TIP": "Please drag the slider to restore the complete image",
    "VERIFYING": "Verifying...",
    "SUCCESS": "Slide successful!",
    "SLIDE_FAIL": "Verification failed, please refresh and try again",
    "CAPTCHA_FAIL": "Verification failed, please try again!",
    "CONGESTION": "Network congestion, please refresh and try again",
    "CAPTCHA_COMPLETED": "Slide completed",
    "FINISH_CAPTCHA": "Please complete verification first!"
}
```

### 21.6 Corrections made to earlier sections by this dump

1. **§15.2/§15.8/§11.3/§17 — `w`'s plaintext is a `#`-joined 140-field string, NOT JSON.** The earlier "collected-data JSON" phrasing came from the pre-breakthrough assumption (`w = rY(JSON.stringify(b))` machine state); the live decrypt shows the payload is the flattened `#`-joined form of `b`'s probe fields (b itself may be JSON-shaped in the browser, but the encrypted payload is the joined string).
2. **§13.2 — resolved.** `Q = tm` where `tm = rm(tN, tR)` = `rm(ACCESS_SEC, t8.sessionId)` is the *client-side decrypt path*; the live dump shows the server ALSO returns the same sessionId plaintext in `deviceConfig.deviceConfig.sessionId`, and the token's Q equals it byte-for-byte. The `[id1,"h",ts,K].join('-')` machine states are the *format* (appKey + `-h-` + ts + per-session uuid), not a competing source: `Q`'s `uuid1` = **appKey** (3795d2…), `uuid2` = per-session id (52b528da…).
3. **§15.2 example keys** — `93e513a51c987af1` / `5ea5a0dd4e774105` were other sessions' keys; this dump's live key is `57ad9f73260d1d46` (delivered as `f[0] = NTdhZDlmNzMyNjBkMWQ0Ng==`, base64 of the key — confirming the "double encoding" note).
4. **Salt is session-independent** — the same `daye,raolewoba!` verifies this 2026-06 live capture exactly as it did the §12 captures and the sandbox token.

