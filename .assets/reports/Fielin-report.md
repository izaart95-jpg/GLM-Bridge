# Fielin: window.z_um.getToken — `st()` Aliyun device-fingerprint token: reverse-engineering report



## Index

- [1 Executive summary](#1-executive-summary)
- [2 window.z_um.getToken - Definition](#2-windowzumgettoken---definition)
- [3 Deobfuscation](#3-deobfuscation)
- [4 Function cG](#4-function-cg)
- [5 Function i return t holds deviceToken](#5-function-i-return-t-holds-devicetoken)
- [6 VAR T RELATION](#6-var-t-relation)
- [7 PAYLOA GEN](#7-payloa-gen)
- [8 DEOBFUSCATION GENERATOR](#8-deobfuscation-generator)
- [9 Function nD](#9-function-nd)
- [10 The verified st() algorithm](#10-the-verified-st-algorithm)
- [11 Real-world capture anatomy](#11-real-world-capture-anatomy)
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
  - [13.5 p = nK(F)[(tn(), tn)(1, 33)]()](#135-p--nkftn-tn1-33)
- [14 String table & decoder (nQ)](#14-string-table--decoder-nq)
- [15 The 640-byte blob — encrypted payload analysis](#15-the-640-byte-blob--encrypted-payload-analysis)
- [16 Blueprint A — reverse a captured payload into its original form](#16-blueprint-a--reverse-a-captured-payload-into-its-original-form)
- [17 Blueprint B — generate payloads in pure Node (no browser)](#17-blueprint-b--generate-payloads-in-pure-node-no-browser)
- [18 Appendix — key source positions (original file coordinates)](#18-appendix--key-source-positions-original-file-coordinates)
- [19 Key constants (runtime-decoded, absent from raw source)](#19-key-constants-runtime-decoded-absent-from-raw-source)
- [20 Standalone reimplementation (reimpl.js)](#20-standalone-reimplementation-reimpljs)

---

## 1 Executive summary

- `st()` returns a `#`-joined 5-field token: `[tF, Q, w, tC, p]`.
- The final field is an integrity hash: `p = MD5([tF, Q, w, tC, secret].join('#'))` with a **hidden constant** `secret = "daye,raolewoba!"` (not present anywhere in the output).
- This was **verified against a real-world capture** (`SG_WEB#3795d2…-h-1782531783720-ac9e47… #<856-char base64>#524#d769460d…`): `MD5("SG_WEB#3795d28242a11619bc25f786f84e53d4-h-1782531783720-ac9e47a76eee443087943a278f191642#<blob>#524#daye,raolewoba!") = d769460d135e774310d665c292c41e95` — exact match.
- The 640-byte base64 blob **is an encrypted payload** (see §15).

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

## 4 Function cG

```js
function cG(t, r, e, n) {
            var i, a, o, c, s, f, l, h, d, v;
            for (a = 5; a; )
                switch (o = a >> 3,
                c = 7 & a,
                o) {
                case 0:
                    c < 3 ? c < 1 || (c <= 1 ? (a ^= 3,
                    s = r) : (f = e,
                    a ^= 4)) : c >= 6 ? c > 6 ? (a += -7,
                    i = function() {
                        var t, r, e, n, i, a, o, c, s, f, l, h, d, v, p, b, w, g, m, y, k, M, O, N, S, x, U, I, A, T, R, C, B, F, E, H, Y, q, J, z, j, P, L, V, D, Q, K, Z, G, X, _, W, $, tt, tr, te, tn, ti, ta, to, tu, tc, ts, tf, tl, th, td, tv, tp, tb, tw, tg, tm, ty, tk, tM, tO, tN, tS, tx, tU, tI, tA, tT, tR;
                        for (r = 112; r; )
                            switch (e = r >> 6,
                            n = r >> 3 & 7,
                            i = 7 & r,
                            e) {
                            case 0:
                                switch (n) {
                                case 0:
                                    i < 1 || (i < 5 ? i >= 4 ? tn ? r += 13 : r += 152 : i < 3 ? i > 1 ? (a = Date[tf.call(5, 81, 63)](),
                                    r -= -55) : (r -= -72,
                                    o = tf.call(0, 108, 21)) : !tp / !tp == 0 ? r ^= 138 : r = 103 : i <= 6 ? i > 5 ? (r = 64,
                                    A = arguments[1]) : (r -= -108,
                                    C[S] = o9()) : (c = arguments[({
                                        0: tf
                                    })[0](31, 36)],
                                    r ^= 70));
                                    break;
                                case 1:
                                    i <= 2 ? i >= 2 ? (ta.s = function(t, r) {
                                        return t - r
                                    }
                                    ,
                                    r -= -60) : i > 0 ? tM ? r += 131 : r += 49 : (r -= -147,
                                    s = -96 * d) : i > 5 ? i > 6 ? (r = 11,
                                    f = K + (-tf ? 2 : tf)(111, 88)) : (F = void 0 !== arguments[1],
                                    r = 63) : i > 4 ? (ta.j(sr, j, to),
                                    r = 62) : i <= 3 ? t8[f] ? r = 164 : r -= 9 : (r ^= 58,
                                    ta.g = function(t, r) {
                                        return t << r
                                    }
                                    );
                                    break;
                                case 2:
                                    i <= 3 ? i <= 2 ? i < 1 ? (r -= -89,
                                    l = {}) : i < 2 ? (r ^= 177,
                                    tn = {}) : (ta.A = function(t, r) {
                                        return t && r
                                    }
                                    ,
                                    r += 117) : (h = !th,
                                    r ^= 80) : i <= 6 ? i > 4 ? i >= 6 ? (r += 75,
                                    d = t8[tf.apply(3, [117, 14])]) : (r -= -103,
                                    v = t8[J]) : (r -= -31,
                                    p = ta.s(57 * T, 57 * d)) : G ? r = 114 : r += 84;
                                    break;
                                case 3:
                                    i >= 1 ? i <= 2 ? i >= 2 ? (r -= -43,
                                    tc = {}) : isNaN(!q * !q) || !q * !q >= 0 ? r += 113 : r -= -12 : i >= 6 ? i >= 7 ? (b = [],
                                    r = 89) : (r ^= 12,
                                    ta.d = function(t, r) {
                                        return t === r
                                    }
                                    ) : i <= 3 ? (w = ta.j(tf, 121 >> (0 | tf), 21 ^ (0 | tf)),
                                    r = 1) : i <= 4 ? (r = 132,
                                    l[m] = g) : isNaN(!P * !t8 / (!t8 * !P)) || !P * !t8 / (!t8 * !P) == 1 ? r ^= 82 : r -= -101 : !O / 0 != 1 ? r += 48 : r = 103;
                                    break;
                                case 4:
                                    i > 1 ? i > 3 ? i >= 6 ? i < 7 ? W ? r = 130 : r += 21 : (r = 95,
                                    g = n5(C, null, null, h)) : i > 4 ? (m = N + tf(-tf || 101, -tf || 39),
                                    r -= 9) : (r = 151,
                                    y = tl[ta.A(tf, tf)(112, 47)](E)) : i >= 3 ? (k = (arguments[tf(~tf ? 31 : 6, ~tf ? 36 : 3)] - 2) * 52,
                                    r ^= 168) : tc ? r ^= 91 : r = 26 : i < 1 ? !C * !q / 0 != 1 ? r = 82 : r -= -76 : (r -= -57,
                                    M = -((-26 * Date[(tf(),
                                    tf)(81, 63)]() - -26 * d) / 26));
                                    break;
                                case 5:
                                    i >= 7 ? (ta.e = function(t, r) {
                                        return t & r
                                    }
                                    ,
                                    r += 36) : i >= 4 ? i < 6 ? i <= 4 ? (tU = void 0 !== arguments[0],
                                    r += 99) : (O = tf.bind(2, 104, 19)() + z,
                                    r += -21) : (r += 90,
                                    A = "") : i > 2 ? (tu = te <= 7,
                                    r ^= 127) : i > 0 ? i > 1 ? 0 * !V * !C != 9 ? r += 45 : r ^= 12 : (N = tf.bind(6, 121, 21)(),
                                    r = 37) : (r += -35,
                                    S = td + [tf][0](115, 7));
                                    break;
                                case 6:
                                    i > 0 ? i > 4 ? i > 5 ? i >= 7 ? !q / 0 != 1 ? r += 47 : r = 35 : (ta.Y = function(t, r) {
                                        return t | r
                                    }
                                    ,
                                    r += -24) : (x = (~tf ? tf : 0)(88, 75) + tR,
                                    r += 8) : i >= 4 ? (U = Y + "h",
                                    r ^= 162) : i <= 1 ? (r -= -50,
                                    tb = "") : i <= 2 ? (I = ta.K(tI, 40),
                                    r = 162) : (r -= -4,
                                    q[tf.bind(8, 131, 34)()](tf(94 * (1 | tf), 35 / ta.Y(tf, 1)) + p / 57)) : (V[(-tf ? 8 : tf)(49, 39)] = 501,
                                    r += 74);
                                    break;
                                case 7:
                                    i >= 4 ? i < 6 ? i > 4 ? (r -= 30,
                                    q[tf(131 >> (0 | tf), 34 << (0 | tf))](x)) : Math.pow(!tv * !A, 0) ? r ^= 31 : r ^= 71 : i > 6 ? (r += 63,
                                    A = F) : (T = Date[tf(81 * (1 | tf), ta.K(63, ta.Y(tf, 1)))](),
                                    r -= 42) : i < 2 ? i > 0 ? (R = 50 * a,
                                    r = 108) : (l[_] = C,
                                    r += 92) : i >= 3 ? (W = 0,
                                    r += 71) : (r -= -82,
                                    tM = {})
                                }
                                break;
                            case 1:
                                switch (n) {
                                case 0:
                                    i < 1 ? !A / 0 != 9 ? r ^= 200 : r += 104 : i > 4 ? i <= 6 ? i <= 5 ? (C = tc,
                                    r = 85) : (r ^= 237,
                                    ta.K = function(t, r) {
                                        return t * r
                                    }
                                    ) : (B = h,
                                    r += 27) : i >= 4 ? (r -= 55,
                                    q[tf(~tf ? 131 : 4, ~tf ? 34 : 4)](Q)) : i < 3 ? i <= 1 ? (r += 102,
                                    F = c > 1) : (r -= 30,
                                    E = function(t) {
                                        return "" !== t
                                    }
                                    ) : (H = t8[ta.j(tf, 110, 74)],
                                    r ^= 52);
                                    break;
                                case 1:
                                    i >= 5 ? i < 6 ? (Y = (ta.I(tf),
                                    tf)(124, 81),
                                    r -= 25) : i < 7 ? Math.pow(!tS, 0) ? r = 172 : r += -24 : (q = u(P),
                                    r = 25) : i <= 2 ? i < 2 ? i <= 0 ? (q[tf(~tf && 131, ~tf && 34)](O),
                                    r ^= 218) : (r ^= 92,
                                    J = w + tf(ta.g(101, ta.Y(tf, 0)), 39 ^ (0 | tf))) : (r += -29,
                                    z = Date[tf(-tf ? 2 : 81, -tf ? 8 : 63)]() - d) : i > 3 ? tU ? r ^= 96 : r = 143 : (j = tr,
                                    r = 27);
                                    break;
                                case 2:
                                    i <= 0 ? Math.pow(!C, 0) ? r ^= 48 : r = 76 : i > 1 ? i >= 5 ? i <= 6 ? i <= 5 ? !C / !C == 0 ? r += -28 : r ^= 3 : (P = t8[ta.j(tf, Math.round(106), Math.floor(82))],
                                    r += -57) : (r ^= 222,
                                    L = [tf][0](26, 27)) : i > 2 ? i > 3 ? tu ? r -= 10 : r -= 13 : (r ^= 194,
                                    ta.I = function(t) {
                                        return t()
                                    }
                                    ) : (r ^= 98,
                                    V = {}) : (r += 42,
                                    C[tf(85 & ~tf, ta.e(81, ~tf))] = B);
                                    break;
                                case 3:
                                    i <= 0 ? tr ? r = 142 : r -= -27 : i <= 5 ? i > 2 ? i < 5 ? i > 3 ? (B = 0,
                                    r += -11) : (r ^= 47,
                                    tx = 0) : (r += 70,
                                    D = arguments[tf([31, tf()][0], [36, tf()][0])]) : i > 1 ? (r ^= 30,
                                    Q = ta.j(tf, 50, 42) + M) : Math.pow(!b * !btoa, 0) ? r += 20 : r -= -58 : i > 6 ? !g * !n5 / (!n5 * !g) == 0 ? r ^= 66 : r -= 79 : (r = 15,
                                    K = ta.j(tf, 38 & ~tf, 46 & ~tf));
                                    break;
                                case 4:
                                    i <= 2 ? i <= 1 ? i >= 1 ? (r -= -47,
                                    Z = tf(~tf && 32, ~tf && 5)) : (G = tv,
                                    r += -73) : B ? r = 127 : r ^= 62 : i > 3 ? i < 6 ? i > 4 ? (X = g,
                                    r ^= 192) : (r += -44,
                                    _ = ti + "ta") : i > 6 ? (W = C[tp],
                                    r -= 65) : (r += -63,
                                    C[tf((tf(),
                                    70), (tf(),
                                    49))] = q[tf(-tf || 76, -tf || 79)]("|")) : (C[({
                                        0: tf
                                    })[0](3, 55)] = tb,
                                    r += -19);
                                    break;
                                case 5:
                                    i < 1 ? (r -= -37,
                                    $ = R - tt) : i >= 4 ? i < 6 ? i < 5 ? (r = 104,
                                    tt = 50 * d) : (h = !!b,
                                    r += -38) : i < 7 ? (r += -22,
                                    tr = tw > 27) : (te = ta.O(ty, 7),
                                    r -= 68) : i < 3 ? i >= 2 ? (r -= 102,
                                    tn = ta.d(ts, void 0)) : (r = 100,
                                    ti = tf(32. .valueOf(), 5. .valueOf())) : (r ^= 25,
                                    G = "");
                                    break;
                                case 6:
                                    i <= 0 ? (r += -102,
                                    ta = {}) : i <= 4 ? i > 2 ? i > 3 ? (r -= 83,
                                    to = tx) : (tr = void 0,
                                    r -= 40) : i < 2 ? Math.pow(!C, 0) ? r -= -15 : r -= 31 : (C[[tf][0](39, 37)] = G,
                                    r += 20) : i > 5 ? i > 6 ? (tu = !H,
                                    r ^= 209) : isNaN(!tu * !Text / (!Text * !tu)) || !tu * !Text / (!Text * !tu) == 1 ? r = 84 : r -= 27 : (tc = t8[tk],
                                    r ^= 87);
                                    break;
                                case 7:
                                    i >= 1 ? i <= 3 ? i < 2 ? isNaN(!tc) || isNaN(!Math) || !tc * !tc + !Math * !Math >= 0 ? r += -52 : r += 18 : i <= 2 ? (V[(~tf ? tf : 8)(57, 44)] = C,
                                    r = 42) : (C[(-tf ? 0 : tf)(119, 64)] = Date[tf(~tf && 81, ta.A(~tf, 63))](),
                                    r -= 29) : i < 7 ? i <= 5 ? i < 5 ? (r += -18,
                                    ts = t8[o + ta.j(tf, [16, tf()][0], [55, tf()][0])]) : (tf = function(t, r) {
                                        return cZ(r, t - -5)
                                    }
                                    ,
                                    r -= 32) : A ? r = 6 : r -= 80 : (r += -46,
                                    B = 1) : tb ? r += 39 : r ^= 253
                                }
                                break;
                            case 2:
                                switch (n) {
                                case 0:
                                    i <= 3 ? i < 2 ? i >= 1 ? (r += -63,
                                    tl = Object[ta.j(tf, 114 / (1 | tf), 9 * (1 | tf))](C)) : (th = [],
                                    r -= 109) : i >= 3 ? !tm / 0 != 8 ? r ^= 42 : r -= -35 : (V[(-tf ? 5 : tf)(75, 22)] = W,
                                    r ^= 28) : i > 4 ? i <= 5 ? isNaN(!tb * !Object / (!Object * !tb)) || !tb * !Object / (!Object * !tb) == 1 ? r ^= 180 : r -= 64 : i >= 7 ? (r = 47,
                                    ta.O = function(t, r) {
                                        return t + r
                                    }
                                    ) : (r += -94,
                                    td = tf(Math.round(63), Math.ceil(70))) : (t8[tf(98 & ~tf, 26 & ~tf)](l),
                                    r ^= 225);
                                    break;
                                case 1:
                                    i < 7 ? i < 1 ? (r = 60,
                                    tv = A) : i <= 5 ? i < 4 ? i < 2 ? (tp = ta.O(L, "st"),
                                    r = 3) : i < 3 ? (tb = tN,
                                    r += -18) : (r += -29,
                                    tw = k + 27) : i < 5 ? (r -= -21,
                                    C = tM) : (tg = $ / 50,
                                    r -= -12) : (r += -67,
                                    tr = arguments[2]) : (tm = tU,
                                    r -= -14);
                                    break;
                                case 2:
                                    i >= 4 ? i <= 5 ? i < 5 ? isNaN(!l) || isNaN(!C) || !l * !l + !C * !C >= 0 ? r += -107 : r ^= 222 : (X = v,
                                    r = 168) : i >= 7 ? (ty = (y[tf.call(6, 31, 36)] - 80) * 1,
                                    r += -40) : (r = 32,
                                    C[U] = q[tf(ta.v(76, 0 | tf), 79 >> (0 | tf))]("|")) : i <= 2 ? i <= 1 ? i < 1 ? (tk = Z + "ta",
                                    r ^= 229) : (ta.v = function(t, r) {
                                        return t >> r
                                    }
                                    ,
                                    r ^= 236) : (tM = cK(tN, tv),
                                    r = 9) : (tm = arguments[0],
                                    r -= -7);
                                    break;
                                case 3:
                                    i >= 6 ? i > 6 ? 0 * !tb != 4 ? r += -60 : r -= 142 : (r += 6,
                                    sn(V)) : i >= 5 ? tm ? r -= 10 : r = 131 : i >= 3 ? i >= 4 ? (r = 160,
                                    tn = ts) : (tO = tA - s,
                                    r += 15) : i >= 2 ? (tN = tm,
                                    r = 7) : i <= 0 ? tx ? r = 116 : r += -61 : (tS = ta.j(tf, -tf ? 2 : 60, -tf ? 9 : 49) + tg,
                                    r ^= 215);
                                    break;
                                case 4:
                                    i > 0 ? i >= 4 ? i > 6 ? F ? r = 14 : r += -104 : i < 5 ? (tx = tT[(tf(),
                                    tf)(4, 23)],
                                    r += -12) : i <= 5 ? X ? r += 3 : r += -16 : tu ? r = 129 : r += -48 : i >= 2 ? i <= 2 ? (tU = I + 19 > 19,
                                    r = 76) : (tI = ta.s(D, 0),
                                    r = 50) : (r -= 153,
                                    tA = -96 * Date[tf(~tf ? 81 : 4, ~tf ? 63 : 2)]()) : (tT = tn,
                                    r ^= 182);
                                    break;
                                case 5:
                                    i >= 1 ? i <= 2 ? i >= 2 ? (r += -117,
                                    tR = -(tO / 96)) : (tm = "",
                                    r ^= 51) : i < 4 ? (ta.j = function(t, r, e) {
                                        return t(r, e)
                                    }
                                    ,
                                    r ^= 167) : (r = 77,
                                    q[tf.bind(0, 131, 34)()](tS)) : (r -= 168,
                                    t = X)
                                }
                            }
                        return t
                    }(d, v, l)) : (i = function(t, r, e) {
                        function n(t, r) {
                            return (cZ || cZ)(r, t - 1)
                        }
                        return sa[(n || n)(24, 12)](this, arguments)
                    }(c6, s, f),
                    a += -6) : c <= 3 ? (a -= -4,
                    l = n) : c <= 4 ? 45 === h ? a -= 3 : a ^= 4 : (a -= -4,
                    h = t);
                    break;
                case 1:
                    c > 2 ? Math.pow(!blur, 0) ? a += -11 : a += -10 : c < 2 ? c < 1 ? (a += 2,
                    d = r) : 46 === h ? a = 8 : a += -5 : (a ^= 9,
                    v = e)
                }
            return i
        }
```

## 5 Function i return t holds deviceToken

```js
                        var t, r, e, n, i, a, o, c, s, f, l, h, d, v, p, b, w, g, m, y, k, M, O, N, S, x, U, I, A, T, R, C, B, F, E, H, Y, q, J, z, j, P, L, V, D, Q, K, Z, G, X, _, W, $, tt, tr, te, tn, ti, ta, to, tu, tc, ts, tf, tl, th, td, tv, tp, tb, tw, tg, tm, ty, tk, tM, tO, tN, tS, tx, tU, tI, tA, tT, tR;
                        for (r = 112; r; )
                            switch (e = r >> 6,
                            n = r >> 3 & 7,
                            i = 7 & r,
                            e) {
                            case 0:
                                switch (n) {
                                case 0:
                                    i < 1 || (i < 5 ? i >= 4 ? tn ? r += 13 : r += 152 : i < 3 ? i > 1 ? (a = Date[tf.call(5, 81, 63)](),
                                    r -= -55) : (r -= -72,
                                    o = tf.call(0, 108, 21)) : !tp / !tp == 0 ? r ^= 138 : r = 103 : i <= 6 ? i > 5 ? (r = 64,
                                    A = arguments[1]) : (r -= -108,
                                    C[S] = o9()) : (c = arguments[({
                                        0: tf
                                    })[0](31, 36)],
                                    r ^= 70));
                                    break;
                                case 1:
                                    i <= 2 ? i >= 2 ? (ta.s = function(t, r) {
                                        return t - r
                                    }
                                    ,
                                    r -= -60) : i > 0 ? tM ? r += 131 : r += 49 : (r -= -147,
                                    s = -96 * d) : i > 5 ? i > 6 ? (r = 11,
                                    f = K + (-tf ? 2 : tf)(111, 88)) : (F = void 0 !== arguments[1],
                                    r = 63) : i > 4 ? (ta.j(sr, j, to),
                                    r = 62) : i <= 3 ? t8[f] ? r = 164 : r -= 9 : (r ^= 58,
                                    ta.g = function(t, r) {
                                        return t << r
                                    }
                                    );
                                    break;
                                case 2:
                                    i <= 3 ? i <= 2 ? i < 1 ? (r -= -89,
                                    l = {}) : i < 2 ? (r ^= 177,
                                    tn = {}) : (ta.A = function(t, r) {
                                        return t && r
                                    }
                                    ,
                                    r += 117) : (h = !th,
                                    r ^= 80) : i <= 6 ? i > 4 ? i >= 6 ? (r += 75,
                                    d = t8[tf.apply(3, [117, 14])]) : (r -= -103,
                                    v = t8[J]) : (r -= -31,
                                    p = ta.s(57 * T, 57 * d)) : G ? r = 114 : r += 84;
                                    break;
                                case 3:
                                    i >= 1 ? i <= 2 ? i >= 2 ? (r -= -43,
                                    tc = {}) : isNaN(!q * !q) || !q * !q >= 0 ? r += 113 : r -= -12 : i >= 6 ? i >= 7 ? (b = [],
                                    r = 89) : (r ^= 12,
                                    ta.d = function(t, r) {
                                        return t === r
                                    }
                                    ) : i <= 3 ? (w = ta.j(tf, 121 >> (0 | tf), 21 ^ (0 | tf)),
                                    r = 1) : i <= 4 ? (r = 132,
                                    l[m] = g) : isNaN(!P * !t8 / (!t8 * !P)) || !P * !t8 / (!t8 * !P) == 1 ? r ^= 82 : r -= -101 : !O / 0 != 1 ? r += 48 : r = 103;
                                    break;
                                case 4:
                                    i > 1 ? i > 3 ? i >= 6 ? i < 7 ? W ? r = 130 : r += 21 : (r = 95,
                                    g = n5(C, null, null, h)) : i > 4 ? (m = N + tf(-tf || 101, -tf || 39),
                                    r -= 9) : (r = 151,
                                    y = tl[ta.A(tf, tf)(112, 47)](E)) : i >= 3 ? (k = (arguments[tf(~tf ? 31 : 6, ~tf ? 36 : 3)] - 2) * 52,
                                    r ^= 168) : tc ? r ^= 91 : r = 26 : i < 1 ? !C * !q / 0 != 1 ? r = 82 : r -= -76 : (r -= -57,
                                    M = -((-26 * Date[(tf(),
                                    tf)(81, 63)]() - -26 * d) / 26));
                                    break;
                                case 5:
                                    i >= 7 ? (ta.e = function(t, r) {
                                        return t & r
                                    }
                                    ,
                                    r += 36) : i >= 4 ? i < 6 ? i <= 4 ? (tU = void 0 !== arguments[0],
                                    r += 99) : (O = tf.bind(2, 104, 19)() + z,
                                    r += -21) : (r += 90,
                                    A = "") : i > 2 ? (tu = te <= 7,
                                    r ^= 127) : i > 0 ? i > 1 ? 0 * !V * !C != 9 ? r += 45 : r ^= 12 : (N = tf.bind(6, 121, 21)(),
                                    r = 37) : (r += -35,
                                    S = td + [tf][0](115, 7));
                                    break;
                                case 6:
                                    i > 0 ? i > 4 ? i > 5 ? i >= 7 ? !q / 0 != 1 ? r += 47 : r = 35 : (ta.Y = function(t, r) {
                                        return t | r
                                    }
                                    ,
                                    r += -24) : (x = (~tf ? tf : 0)(88, 75) + tR,
                                    r += 8) : i >= 4 ? (U = Y + "h",
                                    r ^= 162) : i <= 1 ? (r -= -50,
                                    tb = "") : i <= 2 ? (I = ta.K(tI, 40),
                                    r = 162) : (r -= -4,
                                    q[tf.bind(8, 131, 34)()](tf(94 * (1 | tf), 35 / ta.Y(tf, 1)) + p / 57)) : (V[(-tf ? 8 : tf)(49, 39)] = 501,
                                    r += 74);
                                    break;
                                case 7:
                                    i >= 4 ? i < 6 ? i > 4 ? (r -= 30,
                                    q[tf(131 >> (0 | tf), 34 << (0 | tf))](x)) : Math.pow(!tv * !A, 0) ? r ^= 31 : r ^= 71 : i > 6 ? (r += 63,
                                    A = F) : (T = Date[tf(81 * (1 | tf), ta.K(63, ta.Y(tf, 1)))](),
                                    r -= 42) : i < 2 ? i > 0 ? (R = 50 * a,
                                    r = 108) : (l[_] = C,
                                    r += 92) : i >= 3 ? (W = 0,
                                    r += 71) : (r -= -82,
                                    tM = {})
                                }
                                break;
                            case 1:
                                switch (n) {
                                case 0:
                                    i < 1 ? !A / 0 != 9 ? r ^= 200 : r += 104 : i > 4 ? i <= 6 ? i <= 5 ? (C = tc,
                                    r = 85) : (r ^= 237,
                                    ta.K = function(t, r) {
                                        return t * r
                                    }
                                    ) : (B = h,
                                    r += 27) : i >= 4 ? (r -= 55,
                                    q[tf(~tf ? 131 : 4, ~tf ? 34 : 4)](Q)) : i < 3 ? i <= 1 ? (r += 102,
                                    F = c > 1) : (r -= 30,
                                    E = function(t) {
                                        return "" !== t
                                    }
                                    ) : (H = t8[ta.j(tf, 110, 74)],
                                    r ^= 52);
                                    break;
                                case 1:
                                    i >= 5 ? i < 6 ? (Y = (ta.I(tf),
                                    tf)(124, 81),
                                    r -= 25) : i < 7 ? Math.pow(!tS, 0) ? r = 172 : r += -24 : (q = u(P),
                                    r = 25) : i <= 2 ? i < 2 ? i <= 0 ? (q[tf(~tf && 131, ~tf && 34)](O),
                                    r ^= 218) : (r ^= 92,
                                    J = w + tf(ta.g(101, ta.Y(tf, 0)), 39 ^ (0 | tf))) : (r += -29,
                                    z = Date[tf(-tf ? 2 : 81, -tf ? 8 : 63)]() - d) : i > 3 ? tU ? r ^= 96 : r = 143 : (j = tr,
                                    r = 27);
                                    break;
                                case 2:
                                    i <= 0 ? Math.pow(!C, 0) ? r ^= 48 : r = 76 : i > 1 ? i >= 5 ? i <= 6 ? i <= 5 ? !C / !C == 0 ? r += -28 : r ^= 3 : (P = t8[ta.j(tf, Math.round(106), Math.floor(82))],
                                    r += -57) : (r ^= 222,
                                    L = [tf][0](26, 27)) : i > 2 ? i > 3 ? tu ? r -= 10 : r -= 13 : (r ^= 194,
                                    ta.I = function(t) {
                                        return t()
                                    }
                                    ) : (r ^= 98,
                                    V = {}) : (r += 42,
                                    C[tf(85 & ~tf, ta.e(81, ~tf))] = B);
                                    break;
                                case 3:
                                    i <= 0 ? tr ? r = 142 : r -= -27 : i <= 5 ? i > 2 ? i < 5 ? i > 3 ? (B = 0,
                                    r += -11) : (r ^= 47,
                                    tx = 0) : (r += 70,
                                    D = arguments[tf([31, tf()][0], [36, tf()][0])]) : i > 1 ? (r ^= 30,
                                    Q = ta.j(tf, 50, 42) + M) : Math.pow(!b * !btoa, 0) ? r += 20 : r -= -58 : i > 6 ? !g * !n5 / (!n5 * !g) == 0 ? r ^= 66 : r -= 79 : (r = 15,
                                    K = ta.j(tf, 38 & ~tf, 46 & ~tf));
                                    break;
                                case 4:
                                    i <= 2 ? i <= 1 ? i >= 1 ? (r -= -47,
                                    Z = tf(~tf && 32, ~tf && 5)) : (G = tv,
                                    r += -73) : B ? r = 127 : r ^= 62 : i > 3 ? i < 6 ? i > 4 ? (X = g,
                                    r ^= 192) : (r += -44,
                                    _ = ti + "ta") : i > 6 ? (W = C[tp],
                                    r -= 65) : (r += -63,
                                    C[tf((tf(),
                                    70), (tf(),
                                    49))] = q[tf(-tf || 76, -tf || 79)]("|")) : (C[({
                                        0: tf
                                    })[0](3, 55)] = tb,
                                    r += -19);
                                    break;
                                case 5:
                                    i < 1 ? (r -= -37,
                                    $ = R - tt) : i >= 4 ? i < 6 ? i < 5 ? (r = 104,
                                    tt = 50 * d) : (h = !!b,
                                    r += -38) : i < 7 ? (r += -22,
                                    tr = tw > 27) : (te = ta.O(ty, 7),
                                    r -= 68) : i < 3 ? i >= 2 ? (r -= 102,
                                    tn = ta.d(ts, void 0)) : (r = 100,
                                    ti = tf(32. .valueOf(), 5. .valueOf())) : (r ^= 25,
                                    G = "");
                                    break;
                                case 6:
                                    i <= 0 ? (r += -102,
                                    ta = {}) : i <= 4 ? i > 2 ? i > 3 ? (r -= 83,
                                    to = tx) : (tr = void 0,
                                    r -= 40) : i < 2 ? Math.pow(!C, 0) ? r -= -15 : r -= 31 : (C[[tf][0](39, 37)] = G,
                                    r += 20) : i > 5 ? i > 6 ? (tu = !H,
                                    r ^= 209) : isNaN(!tu * !Text / (!Text * !tu)) || !tu * !Text / (!Text * !tu) == 1 ? r = 84 : r -= 27 : (tc = t8[tk],
                                    r ^= 87);
                                    break;
                                case 7:
                                    i >= 1 ? i <= 3 ? i < 2 ? isNaN(!tc) || isNaN(!Math) || !tc * !tc + !Math * !Math >= 0 ? r += -52 : r += 18 : i <= 2 ? (V[(~tf ? tf : 8)(57, 44)] = C,
                                    r = 42) : (C[(-tf ? 0 : tf)(119, 64)] = Date[tf(~tf && 81, ta.A(~tf, 63))](),
                                    r -= 29) : i < 7 ? i <= 5 ? i < 5 ? (r += -18,
                                    ts = t8[o + ta.j(tf, [16, tf()][0], [55, tf()][0])]) : (tf = function(t, r) {
                                        return cZ(r, t - -5)
                                    }
                                    ,
                                    r -= 32) : A ? r = 6 : r -= 80 : (r += -46,
                                    B = 1) : tb ? r += 39 : r ^= 253
                                }
                                break;
                            case 2:
                                switch (n) {
                                case 0:
                                    i <= 3 ? i < 2 ? i >= 1 ? (r += -63,
                                    tl = Object[ta.j(tf, 114 / (1 | tf), 9 * (1 | tf))](C)) : (th = [],
                                    r -= 109) : i >= 3 ? !tm / 0 != 8 ? r ^= 42 : r -= -35 : (V[(-tf ? 5 : tf)(75, 22)] = W,
                                    r ^= 28) : i > 4 ? i <= 5 ? isNaN(!tb * !Object / (!Object * !tb)) || !tb * !Object / (!Object * !tb) == 1 ? r ^= 180 : r -= 64 : i >= 7 ? (r = 47,
                                    ta.O = function(t, r) {
                                        return t + r
                                    }
                                    ) : (r += -94,
                                    td = tf(Math.round(63), Math.ceil(70))) : (t8[tf(98 & ~tf, 26 & ~tf)](l),
                                    r ^= 225);
                                    break;
                                case 1:
                                    i < 7 ? i < 1 ? (r = 60,
                                    tv = A) : i <= 5 ? i < 4 ? i < 2 ? (tp = ta.O(L, "st"),
                                    r = 3) : i < 3 ? (tb = tN,
                                    r += -18) : (r += -29,
                                    tw = k + 27) : i < 5 ? (r -= -21,
                                    C = tM) : (tg = $ / 50,
                                    r -= -12) : (r += -67,
                                    tr = arguments[2]) : (tm = tU,
                                    r -= -14);
                                    break;
                                case 2:
                                    i >= 4 ? i <= 5 ? i < 5 ? isNaN(!l) || isNaN(!C) || !l * !l + !C * !C >= 0 ? r += -107 : r ^= 222 : (X = v,
                                    r = 168) : i >= 7 ? (ty = (y[tf.call(6, 31, 36)] - 80) * 1,
                                    r += -40) : (r = 32,
                                    C[U] = q[tf(ta.v(76, 0 | tf), 79 >> (0 | tf))]("|")) : i <= 2 ? i <= 1 ? i < 1 ? (tk = Z + "ta",
                                    r ^= 229) : (ta.v = function(t, r) {
                                        return t >> r
                                    }
                                    ,
                                    r ^= 236) : (tM = cK(tN, tv),
                                    r = 9) : (tm = arguments[0],
                                    r -= -7);
                                    break;
                                case 3:
                                    i >= 6 ? i > 6 ? 0 * !tb != 4 ? r += -60 : r -= 142 : (r += 6,
                                    sn(V)) : i >= 5 ? tm ? r -= 10 : r = 131 : i >= 3 ? i >= 4 ? (r = 160,
                                    tn = ts) : (tO = tA - s,
                                    r += 15) : i >= 2 ? (tN = tm,
                                    r = 7) : i <= 0 ? tx ? r = 116 : r += -61 : (tS = ta.j(tf, -tf ? 2 : 60, -tf ? 9 : 49) + tg,
                                    r ^= 215);
                                    break;
                                case 4:
                                    i > 0 ? i >= 4 ? i > 6 ? F ? r = 14 : r += -104 : i < 5 ? (tx = tT[(tf(),
                                    tf)(4, 23)],
                                    r += -12) : i <= 5 ? X ? r += 3 : r += -16 : tu ? r = 129 : r += -48 : i >= 2 ? i <= 2 ? (tU = I + 19 > 19,
                                    r = 76) : (tI = ta.s(D, 0),
                                    r = 50) : (r -= 153,
                                    tA = -96 * Date[tf(~tf ? 81 : 4, ~tf ? 63 : 2)]()) : (tT = tn,
                                    r ^= 182);
                                    break;
                                case 5:
                                    i >= 1 ? i <= 2 ? i >= 2 ? (r += -117,
                                    tR = -(tO / 96)) : (tm = "",
                                    r ^= 51) : i < 4 ? (ta.j = function(t, r, e) {
                                        return t(r, e)
                                    }
                                    ,
                                    r ^= 167) : (r = 77,
                                    q[tf.bind(0, 131, 34)()](tS)) : (r -= 168,
                                    t = X)
                                }
                            }
                        return t
                    }(d, v, l)
```

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

## 9 Function nD

```js
function nD(t, r, e, n, i) {
            var a, o, u, s, f, l, h, d, v, p, b, w, g, m, y, k, M, O, N, S, x, U, I, A, T, R, C, B, F, E, H, Y, q, J, z, j, P, L, V, D, Q, K, Z, G, X, _, W, $, tt, tr, te, tn, ti, ta, to, tu, tc, ts, tf, tl, th, td, tv, tp, tb, tw, tg, tm, ty, tk, tM, tO, tN, tS, tx, tU, tI, tA, tT, tR, tC, tB, tF, tE, tH, tY, tq, tJ;
            for (o = 19; o; )
                switch (u = o >> 6,
                s = o >> 3 & 7,
                f = 7 & o,
                u) {
                case 0:
                    switch (s) {
                    case 0:
                        f < 2 ? f <= 0 || (l = tY,
                        o += 103) : f > 5 ? f >= 7 ? (h = b[tn.call(6, 38, 26)],
                        o += 103) : (d = [tF, Q, w, tC, p],
                        o += 152) : f < 3 ? (o += 16,
                        v = tw + "d") : f < 5 ? f > 3 ? 16 === W ? o -= -117 : o = 51 : (p = 0,
                        o -= -137) : (b = r,
                        o -= -104);
                        break;
                    case 1:
                        ;f > 1 ? f <= 6 ? f >= 4 ? f >= 5 ? f >= 6 ? tm ? o = 69 : o += 114 : (w = rY(JSON[tn(~tn ? 76 : 8, ~tn ? 29 : 5)](b)),
                        o ^= 138) : (g = t8[S.X(tn, (tn(),
                        27), (tn(),
                        41))],
                        o -= -32) : f < 3 ? (w = rg(T, N),
                        o = 137) : (o ^= 19,
                        tq = S.J(M, ""),
                        tJ = 4,
                        m = tq.slice(tq.length - 4).padStart(tJ, "0")) : (y = r,
                        o = 3) : f >= 1 ? (a = rF(X),
                        o = 0) : (k = e,
                        o -= -68);
                        break;
                    case 2:
                        f >= 6 ? f <= 6 ? o = 17 === W ? 84 : 156 : (o -= 12,
                        M = Math[(~tn ? tn : 6)(68, 28)](B)) : f >= 4 ? f < 5 ? (w = w[tn(S.T(-tn, 99), S.T(-tn, 40))](0, 133),
                        o ^= 108) : (O = p << 5,
                        o -= -118) : f > 0 ? f < 3 ? f >= 2 ? !v * !tw / 0 == 3 ? o += 44 : o = 101 : (N = n6(b, te),
                        o ^= 27) : (o = 25,
                        S = {}) : (x = ti + "EC",
                        o = 126);
                        break;
                    case 3:
                        f < 4 ? f < 2 ? f >= 1 ? (o += 139,
                        S.u = function(t, r) {
                            return t / r
                        }
                        ) : isNaN(!m / !m) || !m / !m == 1 ? o ^= 69 : o = 40 : f > 2 ? (U = rg(T, ty),
                        o = 107) : (o += 20,
                        I = t8[tn([27, tn()][0], [41, S.O(tn)][0])]) : f <= 4 ? !W * !W + !Function * !Function < 0 ? o ^= 156 : o = 4 : f > 6 ? (o -= -8,
                        A = b[({
                            0: tn
                        })[0](33, 22)](n3)) : f > 5 ? (T = ts,
                        o ^= 49) : (o ^= 29,
                        a = p);
                        break;
                    case 4:
                        f >= 5 ? f >= 6 ? f > 6 ? (o ^= 39,
                        a = rg(nX, A)) : (p = nK(F)[(tn(),
                        tn)(1, 33)](),
                        o += -32) : (o = 1,
                        tY = b) : f > 2 ? f > 3 ? (o = 155,
                        tY = function(t) {
                            var r, e, n, i, a;
                            for (e = 2; e; )
                                e <= 3 ? e <= 1 ? e > 0 && (n = nD[a(18, 91)](this, 15),
                                e ^= 4) : e <= 2 ? (e += 2,
                                i = {}) : (e -= 2,
                                a = function(t, r) {
                                    return (-nV ? 3 : nV)(r, i.U(t, -9))
                                }
                                ) : e > 4 ? (e = 0,
                                r = n[({
                                    0: a
                                })[0](8, 2)](this, arguments)) : (i.U = function(t, r) {
                                    return t - r
                                }
                                ,
                                e += -1);
                            return r
                        }(b)) : (o = 153,
                        R = rR()) : f > 0 ? f < 2 ? (C = L[tn.call(4, 33, 22)]("-"),
                        o += 69) : (B = function(t) {
                            var r, e, n, i;
                            for (e = 1; e; )
                                e <= 0 || (e <= 1 ? (e += 2,
                                n = function(t, r) {
                                    return (nV || nV)(r, t - -9)
                                }
                                ) : e < 3 ? (r = i[n(~n ? 8 : 4, 2)](this, arguments),
                                e ^= 2) : (i = nD[n(18 / (1 | n), 91 * (1 | n))](this, 14),
                                e ^= 1));
                            return r
                        }(S.J(C, n2)),
                        o = 23) : (o -= -6,
                        F = th[tn(33 & ~tn, 22 & ~tn)](n3));
                        break;
                    case 5:
                        f <= 2 ? f > 0 ? f <= 1 ? 0 > Math.abs(!p) ? o = 112 : o -= -49 : (o ^= 42,
                        a = w) : (o += -40,
                        a = rg(nW, _)) : f <= 3 ? (E = 34 * td,
                        o ^= 74) : f <= 4 ? (o ^= 50,
                        ts = rm(tB, g)) : f >= 6 ? f >= 7 ? (H = S.z(tn, tn)(98, 16),
                        o += 80) : (T = S.X(rm, tk, I),
                        o ^= 104) : o = !tv * !Date / 0 != 6 ? 54 : 140;
                        break;
                    case 6:
                        f <= 5 ? f <= 3 ? f <= 0 ? (Y = t8[S.X(tn, [19, tn()][0], [39, tn()][0])],
                        o += -13) : f >= 2 ? f <= 2 ? (to++,
                        o ^= 164) : 21 === W ? o ^= 89 : o = 129 : ts ? o += -19 : o = 149 : f < 5 ? (o -= 21,
                        b = r) : !te / !te == 0 ? o = 49 : o += 8 : f <= 6 ? (o = 75,
                        q = n3 + rg(T, S.I(String, tv))) : (J = t8[v],
                        o = 163);
                        break;
                    case 7:
                        f >= 7 ? (o += 94,
                        z = function(t, r) {
                            var e, n, i, a;
                            for (n = 2; n; )
                                n >= 4 ? i ? n = 0 : n ^= 7 : n < 2 ? n >= 1 && (i = c[S.X(a, 25, 68)](r),
                                n -= -3) : n > 2 ? (tg[t] = "",
                                n ^= 3) : (a = function(t, r) {
                                    return tn(r, t - -5)
                                }
                                ,
                                n = 1);
                            return e
                        }
                        ) : f < 2 ? f > 0 ? (o += 58,
                        j = r) : o = 0 * !te == 2 ? 95 : 42 : f >= 6 ? (P = tA / 64,
                        o = 154) : f >= 4 ? f > 4 ? o = 511 === te ? 123 : 151 : (delete tg[tU],
                        o = 63) : f >= 3 ? (o ^= 59,
                        a = rF(tc)) : (o -= -33,
                        S.J = function(t, r) {
                            return t + r
                        }
                        )
                    }
                    break;
                case 1:
                    switch (s) {
                    case 0:
                        f < 1 ? (o = 33,
                        L = [tn.call(6, 10, 10), "h", tH, K]) : f < 2 ? (V = (E - tu) / 34,
                        o ^= 201) : f >= 6 ? f >= 7 ? o = !W / 0 != 6 ? 81 : 148 : (D = tn(32. .valueOf(), 8. .valueOf()),
                        o -= -46) : f >= 4 ? f >= 5 ? (Q = tm,
                        o = 7) : (K = S.O(rA)[S.J(Z, "ll")]("-", ""),
                        o = 64) : f < 3 ? (Z = tn(26, 37),
                        o ^= 6) : (o += -67,
                        a = $[ta]('"', ""));
                        break;
                    case 1:
                        f >= 3 ? f < 6 ? f > 3 ? f >= 5 ? tY ? o += -40 : o = 36 : (G = n,
                        o += 4) : 0 > Math.abs(!q) ? o -= 26 : o += 21 : f >= 7 ? (o ^= 70,
                        X = b[tn(~tn ? 36 : 7, ~tn ? 13 : 1)](tp)[tn(33, Math.floor(22))]("-")) : 13 === W ? o -= -69 : o += -56 : f > 0 ? f >= 2 ? (_ = b[tn.bind(9, 33, 22)()](n3),
                        o ^= 98) : (W = t,
                        o ^= 22) : (o ^= 22,
                        $ = w[tn(36 & ~tn, S.M(13, ~tn))](function(t) {
                            var r, e, n, i, a;
                            for (e = 2; e; )
                                e <= 0 || (e < 4 ? e < 2 ? (r = JSON[i](t[1])[n(S.u(52, 1 | n), 14 * (1 | n))](n3, ""),
                                e += -1) : e < 3 ? (n = function(t, r) {
                                    return tn.apply(7, [t, r - -6])
                                }
                                ,
                                e -= -2) : (i = a + "y",
                                e -= 2) : (e -= 1,
                                a = n.call(5, 34, 9)));
                            return r
                        })[tn([33, tn()][0], [22, tn()][0])](n3));
                        break;
                    case 2:
                        f <= 2 ? f <= 0 ? (tt = i,
                        o ^= 96) : f > 1 ? !Q * !Q + !rm * !rm < 0 ? o += 8 : o = 17 : (o += 8,
                        b = r) : f >= 4 ? f < 7 ? f < 5 ? (o -= -2,
                        b = r) : f <= 5 ? (o = 133,
                        tr = tn([19, S.O(tn)][0], [25, tn()][0])) : (o = 117,
                        te = e) : (tn = function(t, r) {
                            return nQ.bind(6, r - 6, t)()
                        }
                        ,
                        o += -14) : (o -= 67,
                        ti = tn(-tn || 32, -tn || 8));
                        break;
                    case 3:
                        f > 1 ? f >= 7 ? 18 === W ? o += -24 : o = 145 : f >= 6 ? (ta = tO + "ll",
                        o -= 27) : f > 2 ? f > 4 ? (o -= 93,
                        a = S.J(C, m)) : f > 3 ? Math.pow(!S, 0) ? o -= 34 : o -= -10 : (o ^= 56,
                        S.T = function(t, r) {
                            return t || r
                        }
                        ) : (to = 0,
                        o -= -60) : f < 1 ? (S.t = function(t, r) {
                            return t * r
                        }
                        ,
                        o ^= 34) : (te = e,
                        o -= 6);
                        break;
                    case 4:
                        f < 4 ? f < 2 ? f > 0 ? (tu = 136,
                        o -= 32) : (o = 59,
                        tc = [Q, w, tT, U, q][tn(-tn || 33, -tn || 22)](n3)) : f <= 2 ? (ts = k,
                        o -= 49) : (o ^= 59,
                        S.M = function(t, r) {
                            return t & r
                        }
                        ) : f > 6 ? (tf = tS + "At",
                        o ^= 227) : f < 6 ? f >= 5 ? (o += 30,
                        tl = t8[D + "EC"]) : (o += -68,
                        th = [tF, Q, w, tC, n$]) : (o = 43,
                        td = C[tn(93 * (1 | tn), S.u(38, 1 | tn))]);
                        break;
                    case 5:
                        f < 1 ? (o += 1,
                        N = n6(l, 501)) : f >= 6 ? f <= 6 ? h ? o = 144 : o += 4 : 19 === W ? o ^= 247 : o = 0 : f <= 1 ? (o += -5,
                        w = rg(T, N)) : f < 5 ? f < 4 ? f >= 3 ? (o -= 62,
                        tv = Date[(tn && tn)(65, 12)]()) : isNaN(!W) || isNaN(!History) || !W * !W + !History * !History >= 0 ? o -= 101 : o ^= 225 : (o += 4,
                        S.O = function(t) {
                            return t()
                        }
                        ) : (tp = function(t) {
                            var r, e, n, i, a;
                            for (e = 3; e; )
                                e > 1 ? e > 2 ? e >= 4 ? (e -= 3,
                                n = n4(t[1], t[0])) : (i = t[0],
                                e ^= 1) : (a = i + n3,
                                e ^= 6) : e <= 0 || (e += -1,
                                r = a + n);
                            return r
                        }
                        ,
                        o = 79);
                        break;
                    case 6:
                        f >= 2 ? f > 4 ? f >= 6 ? f <= 6 ? (a = rF(tE),
                        o += -118) : (o = 142,
                        tb = w[tn.call(1, 93, 38)]) : o = 0 * !te * !e == 9 ? 124 : 13 : f < 3 ? (o = 144,
                        h = 0) : f > 3 ? (o ^= 118,
                        tw = (~tn ? tn : 0)(98, 16)) : (tg = Object[(S.O(tn),
                        tn)(71, 31)]({}, j),
                        o = 113) : f > 0 ? Math.pow(!tg * !Object, 0) ? o += -28 : o -= -40 : (o -= -49,
                        S.z = function(t, r) {
                            return t && r
                        }
                        );
                        break;
                    case 7:
                        f >= 3 ? f >= 5 ? f > 6 ? (o = 14,
                        tm = G) : f <= 5 ? (o = 27,
                        ty = t8[tn(S.z(~tn, 85), ~tn && 27)]) : (tk = t8[x],
                        o -= 100) : f > 3 ? (a = tg,
                        o ^= 124) : (tM = (-tn ? 5 : tn)(19, 25),
                        o = 141) : f >= 1 ? f <= 1 ? (b = r,
                        o += -47) : (o -= 35,
                        S.H = function(t, r) {
                            return t - r
                        }
                        ) : (o += -48,
                        tO = (tn || tn)(26, 37))
                    }
                    break;
                case 2:
                    switch (s) {
                    case 0:
                        f <= 6 ? f < 4 ? f < 1 ? (tN = t8[tn((S.O(tn),
                        48), (tn(),
                        9))],
                        o += 15) : f < 2 ? 14 === W ? o ^= 142 : o = 111 : f >= 3 ? !tl * !tl + !t8 * !t8 < 0 ? o += -45 : o -= 76 : (tS = (tn(),
                        tn)(76, 44),
                        o ^= 229) : f < 5 ? (tx = y[tf](to),
                        o -= 111) : f <= 5 ? (tU = tr + "st",
                        o += -73) : (o ^= 195,
                        tm = rm(tN, tR)) : 504 === te ? o = 56 : o -= 82;
                        break;
                    case 1:
                        f < 5 ? f >= 2 ? f > 2 ? f >= 4 ? (tI = y[tn(93 * (1 | tn), S.t(38, 1 | tn))],
                        o += 8) : (tA = S.H(64 * O, 64 * p),
                        o -= 77) : 20 === W ? o += 24 : o ^= 150 : f <= 0 ? (o = 34,
                        C = C[tn(99, Math.round(40))](0, V)) : (tT = rg(T, uK()),
                        o -= 12) : f >= 7 ? (tR = t8[H + "d"],
                        o -= 9) : f >= 6 ? tb > 133 ? o -= 122 : o += -22 : (o ^= 26,
                        delete w[S.J(tM, "st")]);
                        break;
                    case 2:
                        f < 2 ? f > 0 ? !W * !history / (!history * !W) == 0 ? o ^= 146 : o ^= 223 : (tC = h,
                        o = 160) : f >= 4 ? f < 6 ? f > 4 ? (tB = t8[tn.call(9, 48, 9)],
                        o -= 137) : 0 === tI ? o ^= 6 : o = 90 : f >= 7 ? (w = Object[({
                            0: tn
                        })[0](15, 35)](w),
                        o = 119) : -99 > S.J(34 * S.H(to, y[S.X(tn, 93, 38)]), -99) ? o += -20 : o += -121 : f <= 2 ? (a = p,
                        o = 0) : (b = r,
                        o = 8);
                        break;
                    case 3:
                        f >= 1 ? f < 5 ? f >= 4 ? 15 === W ? o = 57 : o -= 18 : f >= 2 ? f > 2 ? Math.pow(!tY, 0) ? o -= 154 : o -= 32 : (p = P + tx,
                        o += 5) : (o ^= 251,
                        tF = Y[R]) : f >= 7 ? (o = 50,
                        p &= p) : f > 5 ? (o += -40,
                        tE = d[tn(~tn ? 33 : 7, ~tn ? 22 : 3)](n3)) : (Object[tn(91, 17)](tg)[S.X(tn, 35, 11)](z),
                        o += -33) : (tH = Date[S.X(tn, 65, 12)](),
                        o ^= 218);
                        break;
                    case 4:
                        f <= 3 ? f > 2 ? (o += -81,
                        Q = rm(tl, J)) : f > 0 ? f > 1 ? isNaN(!W * !W) || !W * !W >= 0 ? o ^= 150 : o += -65 : (o += -69,
                        S.I = function(t, r) {
                            return t(r)
                        }
                        ) : (o += -83,
                        tY = tt) : (o = 108,
                        S.X = function(t, r, e) {
                            return t(r, e)
                        }
                        )
                    }
                }
            return a
        }
```

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
3. **`w`** — the machine's payload state is `w = rY(JSON.stringify(b))` where `b` is the collected device-data object; in the sandbox `b` was empty so `w` stayed `null`. In the browser `w` = the 640-byte encrypted blob (see §15).
4. **`tC = 524`** — `b["GatherCost"]` is truthy in the browser (cost/counter of gathered data, here 524); the machine takes the `h truthy → tC = h` branch instead of the forced `h=0` path.

## 12 Test

### 12.1 NORMAL DEVICE TOKEN:

```
U0dfV0VCIzM3OTVkMjgyNDJhMTE2MTliYzI1Zjc4NmY4NGU1M2Q0LWgtMTc4MjUzMTc4MzcyMC1hYzllNDdhNzZlZWU0NDMwODc5NDNhMjc4ZjE5MTY0MiNXM0FTUGxXZmx2SWI1YlJWV2RpbnlJdHA1WXIwOGxVMTY1VEs3KzlTWGJYMmlOcTBLU0J3ZUJBTG1hMFlLaFhJNE5iUFVvLzVOeG0xSEsyemRXTHJXQUdGN3FXWWxhSW1xaEtsVkRzbEd4OXdhNnRSSGNIOFlBUWhtNVltWk14ZGM5cCsxSlRpM1FoVDg0YmYvREJpY0tWenZRS2haaUVPaExFM1hQaitCcmViblRzN1cycStBY1BvU2swdldRWnBSL1lIUE5qMkZ0bTRraTJKZnJ3NXNvMTFLTkw1QjN5ZERVbHpLbXV2Tnh0OHZYVHNOMHJtY1ZuRWluT1E1cjMvd2lOVkdrdHpsQ1k3VFR0YTAvUFpVa2VvM2M5N2F6d3dIbE1NZERENEhzREpkYmdTQzRmcE1idGovSGtoUGViWTRVK3orNHJHT3JDclRmSGV2UllyNFlxTGwzZFIvb1pHVFdhanBzRmhoTUoraThTNHN1bmlRZ1dmOXRnK1pMckg4b3hQNFl0TGpCOHZBdk10K1FPNjZSbGFuRjhsK08wZ2gvbVJ0Zmdla1pYaVN3TjdJTVc4dzhwRlE2d2RlOURUeS96Nk5wQ0xNemg3Mm1waHhyMnBHSGpBMTlHL1kweUw5OTBGa2hBNjY5eVJraUdVNWQxeTREWWxEZmNpemU4dVFWY1lLNituckhlSFVJcVlsajFCQjF6QzJvK3NRSmxsWmdORjMxYzNKMHZlRTFONGtyaTdJdzNnYTdsK3BIOWM1bnFVQmFXS1pNRnRvRHpSZnRHWUw4cWhHa0dWaVBadU9FTE0zYlQ3NWdIdmhQTGdsdkRNSmxmeE10Yk1teXZZTis3c2k1eHh2WjhNaFFMeG1VSUJDZ0NLVERrTGVNcm43Z3Y5bzBpaGNFT3JOTGJLZjh2R0VHMnIzSjhWR1pPVnRTSkNkMG53ak9JMlYyOTFSZ2NUazVOTmJWZWp1a2ZEdlNmNmU2eHE2ZjZBVkJpUWE3ZUszMnZVOXZtakVaeWw2d1owVFlTRGdMOVovaTVoQW5NSXJpdzB4aE55eFpVbWpTSThrVmtTRC9lRGg2R3A4UzM1MnlZOWkzZDB0T3FaMWoxWW5yUEZlelA4bmlhUi9ndklQdz09IzUyNCNkNzY5NDYwZDEzNWU3NzQzMTBkNjY1YzI5MmM0MWU5NQ==
```

### 12.2 DECODED:

```
SG_WEB#3795d28242a11619bc25f786f84e53d4-h-1782531783720-ac9e47a76eee443087943a278f191642#W3ASPlWflvIb5bRVWdinyItp5Yr08lU165TK7+9SXbX2iNq0KSBweBALma0YKhXI4NbPUo/5Nxm1HK2zdWLrWAGF7qWYlaImqhKlVDslGx9wa6tRHcH8YAQhm5YmZMxdc9p+1JTi3QhT84bf/DBicKVzvQKhZiEOhLE3XPj+BrebnTs7W2q+AcPoSk0vWQZpR/YHPNj2Ftm4ki2Jfrw5so11KNL5B3ydDUlzKmuvNxt8vXTsN0rmcVnEinOQ5r3/wiNVGktzlCY7TTta0/PZUkeo3c97azwwHlMMdDD4HsDJdbgSC4fpMbtj/HkhPebY4U+z+4rGOrCrTfHevRYr4YqLl3dR/oZGTWajpsFhhMJ+i8S4suniQgWf9tg+ZLrH8oxP4YtLjB8vAvMt+QO66RlanF8l+O0gh/mRtfgekZXiSwN7IMW8w8pFQ6wde9DTy/z6NpCLMzh72mphxr2pGHjA19G/Y0yL990FkhA669yRkiGU5d1y4DYlDfcize8uQVcYK6+nrHeHUIqYlj1BB1zC2o+sQJllZgNF31c3J0veE1N4kri7Iw3ga7l+pH9c5nqUBaWKZMFtoDzRftGYL8qhGkGViPZuOELM3bT75gHvhPLglvDMJlfxMtbMmyvYN+7si5xxvZ8MhQLxmUIBCgCKTDkLeMrn7gv9o0ihcEOrNLbKf8vGEG2r3J8VGZOVtSJCd0nwjOI2V291RgcTk5NNbVejukfDvSf6e6xq6f6AVBiQa7eK32vU9vmjEZyl6wZ0TYSDgL9Z/i5hAnMIriw0xhNyxZUmjSI8kVkSD/eDh6Gp8S352yY9i3d0tOqZ1j1YnrPFezP8niaR/gvIPw==#524#d769460d135e774310d665c292c41e95
```

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

### 13.4 tC = h

```js
h = b[tn.call(6, 38, 26)]

r = b

h = b.GatherCost // 0 if b.gatherCost undefined
```

### 13.5 p = nK(F)[(tn(), tn)(1, 33)]() p = constant for md5 gen

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

## 15 The 640-byte blob — encrypted payload analysis

Facts:

- `base64 -d` → exactly **640 bytes**, 16-byte aligned (**40 AES-sized blocks**).
- Statistical uniformity: 36.7% printable bytes ≈ expected for uniform random (95/256 ≈ 37%) — no text, no JSON (`{"`, `",`, `null`, `true` all absent).
- First 8 bytes `5b 70 12 3e…` — not `Salted__` (so not standard CryptoJS/OpenSSL salted format).
- No zlib/gzip/brotli magic; not plain base64-of-JSON.

Negative results (all tried, all failed `bad decrypt` or no structure):

- AES-128-ECB / AES-128-CBC (IV=0, IV=salt, IV=blob[0:16]) with keys: `ACCESS_SEC`, `SALT` (16B), `daye,raolewoba!`, `md5(ACCESS_SEC)`, `md5(secret)`, `md5(SALT64)`, `sha1/sha256` truncated variants, `md5(access+secret)`.
- AES-256 with `sha256(secret)`, `sha256(ACCESS)`, `sha256(SALT64)`, EvpKDF-derived 32+16.
- 3DES / DES with md5-derived keys.
- Repeating-key XOR with all above keys + session fields (`uuid1`, `uuid2`, `md5(uuid1)`, `md5(uuid1+uuid2)`, `md5(Q)`, `md5(ts)`); single-byte XOR best 41.9%.
- IV-prepended ciphertext layouts (`[IV][ct]`, 16+624).

**Conclusion:** the blob is ciphertext whose key/IV/mode derive from runtime state inside the page (per-session values not present in the token itself — e.g. derived from `SALT`/`ACCESS_SEC` combined with session random, or a custom cipher on the same line as the string machinery). It cannot be decrypted from the token alone; see §16A step 3.

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

### Step 3 — decrypt `w` (requires the live page; this is the only way to obtain the runtime key material)

In the real page, before the bundle executes (early-injected script, or a patched copy of the bundle hosted locally):

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
   ```
   `__wLog[0]` = `b` = **the original form** (the collected device-data object); `__wLog[2]` = its JSON; `__wLog[1]` = the transform function `rY` (dump its `.toString()` and its runtime inputs).
3. If `rY` wraps `window.__ALIYUN_CRYPT` (CryptoJS — the bundle references it by decoded name `__ALIYUN_CRYPT`), wrap its primitives to log key/IV/mode/plaintext:
   ```js
   const C = window.__ALIYUN_CRYPT;
   for (const m of ['encrypt','decrypt']) if (C.AES && C.AES[m]) {
     const orig = C.AES[m].bind(C.AES);
     C.AES[m] = function(...args){ globalThis.__cryptoLog = args; return orig(...args); };
   }
   ```
   Also hook `CryptoJS.enc.*.parse` if key/IV are passed as strings, and log `SALT`/`ACCESS_SEC` consumers (`t8` object dump: the `tF` constructor wraps the decoded constants — dump `t8`).
4. `plaintext = JSON.parse(decrypt(w))` — the fully reversed original form.

## 17 Blueprint B — generate payloads in pure Node (no browser)

```js
const crypto = require('crypto');
const secret = 'daye,raolewoba!';
const WEB_REGION = { CN: 'WEB', SG: 'SG_WEB' };   // extend with other regions if seen

function makeToken({ region = 'CN', uuid1, uuid2, tC, w }) {
  const tF = WEB_REGION[region];
  const Q = [uuid1, 'h', Date.now(), uuid2].join('-');
  const p = crypto.createHash('md5').update([tF, Q, w, tC, secret].join('#')).digest('hex');
  return [tF, Q, w, tC, p].join('#');
}
```

Requirements, in order of dependency:

1. **`w` (the payload field)** — cannot be fabricated without the crypto recovered in §16A.3 (key/IV/mode + the exact `rY` transform). With those, `w = rY(JSON.stringify(b))` where `b` is your data object.
2. **`b` (the input data)** — the plaintext JSON the real collector builds from browser probes (the collection functions live in the same bundle; instrument the `b` object in step A.3 and replay its field set). Pure Node cannot conjure the measurements — you either replay captured `b` values or synthesize your own dataset; the token is structurally valid either way.
3. **`tC`** — `b["GatherCost"]`; keep consistent with the payload (524 in the capture, `0` when `b` is empty). If you replay a captured `b`, reuse its `GatherCost`.
4. **`uuid1`/`uuid2`** — your own ids; format `8-4-4-4-12` hex without dashes; `Date.now()` is automatic.
5. **`tF`/region** — match the endpoint region (`ap-southeast` → `SG`).

Completeness checklist for the generator: identical `w` transform, identical `b` field schema, `GatherCost` consistency, md5 with the secret — the token will then be indistinguishable from a real one.

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

Harness gotchas (reproduced intentionally for future runs):

- `shim.js` replaces `global.process` with `{argv: []}` — capture `process.argv` **before** requiring the shim.
- The bundle schedules a background async XHR (Aliyun RPC request, 4s `Promise.race` timeouts ×2 + 500ms retries) that keeps the process alive ~8.7 s and prints `Error: timeout` — harmless, env-independent.
- Determinism was proven (two fresh VM runs → identical tokens) and env-independence (altered UA/DPR/screen/orientation → byte-identical token and identical machine traces: `nDTrace` 39 states, `giantTrace`, `uUndef` call site).

## 19 Key constants (runtime-decoded, absent from raw source)

| constant | value | role |
|---|---|---|
| `ACCESS_SEC` | `FqJB6iRNVYdEGpwb` (16B) | key material (16 B → AES-128-sized) |
| `SALT` | `NLAoqT6K03oLbQXW2VS3zA==` (16B decoded) | salt material |
| secret | `daye,raolewoba!` | md5 mixing constant (hidden in `F`) |
| `WEB_REGION[CN]` | `WEB` | token prefix |
| `WEB_REGION[SG]` | `SG_WEB` | token prefix (via `rR()` endpoints) |

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
