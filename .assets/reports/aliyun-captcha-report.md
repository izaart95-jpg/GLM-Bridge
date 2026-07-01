# Aliyun CaptchaJS Information Report 

## Table of Contents
1. [Obfuscation & Deobfuscation of Literals](#1-obfuscation--deobfuscation-of-literals-aliyuncaptchajs)
2. [Functions, Definitions & Variables](#2-functions-definitions--variables-aliyuncaptchajs)
3. [Function `Re` Internals: DeviceData Creation](#3-function-re-internals-devicedata-creation-aliyuncaptchajs)
4. [Information of `Rt` in Context of `Rt.appkey`](#4-information-of-rt-in-context-of-rtappkey-aliyuncaptchajs)
5. [Function `rr` Index Deobfuscated Literals](#5-function-rr-index-deobfuscated-literals-json-log-aliyuncaptchajs)
6. [DeviceData Generation & Encryption](#6-devicedata-generation--encryption-aliyuncaptchajs)
7. [Signature and Nonce Generation](#7-signature-and-nonce-generation-aliyuncaptchajs)
8. [Traceless Captcha Flow](#8-traceless-captchaflow)
9. [Verify Captcha Payload Observation](#9-verify-captcha-payload-observation-pehashjs)
10. [Zlib Compression & Data Transformation](#10-zlib-compression--data-transformation)
11. [VM-Based Hash Calculation Reverse Engineering](#11-vm-based-hash-calculation-reverse-engineering)
12. [TrackJson `arg` Value Generation](#12-trackjson-arg-value-generation)

---

## 1. Obfuscation & Deobfuscation of Literals [File:AliyunCaptcha.js] 

**Starting with**

```javascript
var G = rr; // <- Deobfuscator function
window.__G = G = rr; // Added proxy to access it globally 
```

### Function rr Defintion 
```javascript
function rr(t, r) {

            var e = jt();

            return rr = function(r, n) {

                var i = e[r -= 312];

                if (void 0 === rr.avoPoV) {

                    rr.nyBjcA = function(t) {

                        for (var r, e, n = "", i = "", o = 0, c = 0; e = t.charAt(c++); ~e && (r = o % 4 ? 64 * r + e : e,

                        o++ % 4) ? n += String.fromCharCode(255 & r >> (-2 * o & 6)) : 0)

                            e = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/=".indexOf(e);

                        for (var u = 0, a = n.length; u < a; u++)

                            i += "%" + ("00" + n.charCodeAt(u).toString(16)).slice(-2);

                        return decodeURIComponent(i)

                    }

                    ,

                    t = arguments,

                    rr.avoPoV = !0

                }

                var o = r + e[0]

                  , c = t[o];

                return c ? i = c : (i = rr.nyBjcA(i),

                t[o] = i),

                i

            }

            ,

            rr(t, r)

        }


```

### `Jt`  Prototype


```javascript
// Jt.prototype
Jt[G(547) + "e"] = {
            ENDPOINTS: [G(525) + G(417) + G(473) + G(405) + G(578)],
            CN_DEFAULT_ENDPOINTS: [G(525) + G(417) + G(473) + G(405) + G(578)],
            INTL_DEFAULT_ENDPOINTS: [G(525) + G(417) + G(473) + G(555) + G(362) + G(358) + G(366)],
            CN_ENDPOINTS: _t,
            INTL_ENDPOINTS: St,
            WAF_ENDPOINTS: [G(525) + G(475) + G(344) + G(377) + G(556)],
            cdnServers: [G(535) + G(366)],
            cdnDevServers: [G(501) + G(581)],
            dynamicJsPath: function(t) {
                var r = 479
                  , e = 318
                  , n = 342
                  , i = 328
                  , o = 388
                  , c = 507
                  , u = 444
                  , a = 494
                  , s = 318
                  , f = 507
                  , l = G
                  , p = {};
                p[l(494)] = function(t, r) {
                    return t + r
                }
                ,
                p[l(r)] = function(t, r) {
                    return t + r
                }
                ,
                p[l(e)] = l(n) + l(i) + l(o) + "/",
                p[l(c)] = l(u);
                var v = p;
                return v[l(a)](v[l(r)](v[l(s)], t), v[l(f)])
            },
            fallbackVersion: G(319) + G(499),
            https: G(525),
            http: G(579),
            API_VERSION: G(590) + "15",
            APP_VERSION: G(416) + "2",
            PLATFORM: G(398) + "c",
            APP_NAME: G(484) + G(508),
            DEVICE_TYPE: Vt,
            APP_KEY: G(345) + G(526) + G(424) + G(322),
            ACCESS_KEY: Yt,
            WEB_AES_SECRET_KEY: Zt,
            AES_IV: G(534) + G(427) + G(586),
            SALT: G(468) + G(549) + G(350),
            SESSION_ID_SALT: G(409) + G(533) + G(584),
            ACCESS_SEC: G(503) + G(396),
            ACTION: Xt,
            ACTION_STATE: Qt,
            WEB_REGION: $t,
            WEB_REGION_PREID: tr,
            UID_NAME_COOKIE: G(567) + "o",
            UID_NAME_LOCAL: G(497) + "s",
            initTime: Date[G(496)](),
            preCollectData: {},
            logs: [],
            _extend: function(t) {
                var r = G
                  , e = this;
                new q(t)[r(505)](function(t, r) {
                    e[t] = r
                })
            }
        };
```        


### Deobfuscation examples
```js
// setup
var G = rr;
window.__G = G; // AliyunCaptcha.js
var G = window.__G; // browser console

// Examples
PLATFORM: G(398) + "c"
'W.10001.c'

fallbackVersion: G(319) + G(499)
'0.0.0/feilin'

https: G(525)
'https://'

http: G(579)
'http://'

API_VERSION: G(590) + "15"
'2020-10-15'

APP_VERSION: G(416) + "2"
'W20220202'

APP_NAME: G(484) + G(508)
'saf-aliyun-com'

APP_KEY: G(345) + G(526) + G(424) + G(322)
'ab034ec0643f91399eb33e062dc7fae1'

AES_IV: G(534) + G(427) + G(586)
'd35db7e39ebbf3d001083105'

SALT: G(468) + G(549) + G(350)
'NLAoqT6K03oLbQXW2VS3zA=='

SESSION_ID_SALT: G(409) + G(533) + G(584)
'X1y5VstbB5zqghyJ9g8a0A=='

ACCESS_SEC: G(503) + G(396)
'FqJB6iRNVYdEGpwb'

UID_NAME_COOKIE: G(567) + "o"
'_c_WBKFRo'

var Zt = {};

        Zt[G(510)] = G(374) + G(410) + G(333) + G(546) + G(346) + G(399),
        Zt[G(407)] = G(321) + G(537) + G(426) + G(415) + G(593) + G(521),
        Zt[G(348)] = G(464) + G(516) + G(437) + G(325) + G(488) + G(449),
        Zt[G(564)] = G(553) + G(480) + G(577) + G(365) + G(340) + G(561),
        Zt[G(500)] = G(457) + G(403) + G(433) + G(571) + G(400) + G(341);
var WEB_AES_SECRET_KEY = Zt;
undefined
console.log(WEB_AES_SECRET_KEY)
{

    "REQ": "8KmHIQsc5+LZJA7uYex3WaHdkjgCtS6epbG/bc9xss0=",

    "RES": "9NhnQQ+LRrKCkAuxwZaUWGBtSzaFtFlNb/ksJCrCgrM=",

    "FLAG": "k+1RW0cz3iDi2RAbC/c3QKzTPiVwNmNO1910DXe6Gas=",

    "UPLOAD": "+fR9tYzlKFr07pEbumd7+KnO3xLOkphCS+qKUbJiMfA=",

    "PREID": "xLLw/t15vkI7QQBX/1scBbcb9fKx+ymxF0tJ3ds42B0="

}


var Vt = {};
Vt[G(441)] = "W";
'W'
var DEVICE_TYPE = Vt;
undefined
console.log(DEVICE_TYPE);
{

    "WEB": "W"

}

```

---

## 2. Functions, Definitions & Variables [AliyunCaptcha.js]

### Function `me`
 ```javascript  
   function me(t, r) {
            var e = 499
              , n = 531
              , i = 368
              , o = 487
              , c = 423
              , u = 373
              , a = 468
              , s = 487
              , f = 487
              , l = 436
              , p = 373
              , v = 429
              , h = 465
              , d = 376
              , y = re
              , g = {};
            g[y(e)] = y(n) + y(i),
            g[y(o)] = function(t, r) {
                return t === r
            }
            ,
            g[y(c)] = function(t, r) {
                return t !== r
            }
            ,
            g[y(u)] = function(t, r) {
                return t <= r
            }
            ;
            for (var m = g, x = m[y(e)][y(a)]("|"), w = 0; ; ) {
                switch (x[w++]) {
                case "0":
                    if (m[y(s)](r, void 0) || m[y(f)](r, null))
                        return null;
                    continue;
                case "1":
                    if (m[y(o)](t, void 0) || m[y(c)](t[y(l)], 16) || m[y(p)](r[y(l)], 0))
                        return null;
                    continue;
                case "2":
                    var b = r;
                    continue;
                case "3":
                    var S = ue[y(v)](t);
                    continue;
                case "4":
                    return C[y(h)](ue);
                case "5":
                    var C = ce[y(d)](b, S, pe);
                    continue
                }
                break
            }
        }
```

### Crypto variables extraction
```javascript
var ce = te()[re(454)]
          , ue = te()[re(439)][re(383)]
          , ae = te()[re(439)][re(505)]
          , se = te()[re(439)][re(517)]
          , fe = te()[re(504)][re(494)]
          , le = ae[re(397) + "y"](se[re(429)](dt))
          , pe = {
            iv: ue[re(429)](le),
            padding: fe
        }
          , ve = nr[re(523) + re(521) + "EY"]
          , he = me(nr[re(541) + "EC"], ve[re(435)])
          , de = me(nr[re(541) + "EC"], ve[re(483)]);
```
### Console Evaluations
```js
window.__value_of_ve
{
    "REQ": "8KmHIQsc5+LZJA7uYex3WaHdkjgCtS6epbG/bc9xss0=",
    "RES": "9NhnQQ+LRrKCkAuxwZaUWGBtSzaFtFlNb/ksJCrCgrM=",
    "FLAG": "k+1RW0cz3iDi2RAbC/c3QKzTPiVwNmNO1910DXe6Gas=",
    "UPLOAD": "+fR9tYzlKFr07pEbumd7+KnO3xLOkphCS+qKUbJiMfA=",
    "PREID": "xLLw/t15vkI7QQBX/1scBbcb9fKx+ymxF0tJ3ds42B0="
}


window.__value_he
'45f8ac1e1de14397'

window.__value_de
'87f879f135f27da7'

window.__se
{stringify: ƒ, parse: ƒ}
window.__se.stringify
, u = n.enc = {}
                  , a = u.Hex = {
                    stringify: function(t) {
                        for (var r = t.words, e = t.sigBytes, n = [], i = 0; i < e; i++) {
                            var o = r[i >>> 2] >>> 24 - i % 4 * 8 & 255;
                            n.push((o >>> 4).toString(16)),
                            n.push((15 & o).toString(16))
                        }
                        return n.join("")
                    },
		    
window.__se_parse
parse: function(t) {
                        for (var r = t.length, e = [], n = 0; n < r; n += 2)
                            e[n >>> 3] |= parseInt(t.substr(n, 2), 16) << 24 - n % 8 * 4;
                        return new c.init(e,r / 2)
                    }
		    

window.__ce
{encrypt: ƒ, decrypt: ƒ}


context of ce:
        7165: function(t, r, e) {
            var n;
            t.exports = (n = e(9021),
            e(9506),
            void (n.lib.Cipher || function(t) {
                var r = n
                  , e = r.lib
                  , i = e.Base
                  , o = e.WordArray
                  , c = e.BufferedBlockAlgorithm
                  , u = r.enc
                  , a = (u.Utf8,
                u.Base64)
                  , s = r.algo.EvpKDF
                  , f = e.Cipher = c.extend({
                    cfg: i.extend(),
                    createEncryptor: function(t, r) {
                        return this.create(this._ENC_XFORM_MODE, t, r)
                    },
                    createDecryptor: function(t, r) {
                        return this.create(this._DEC_XFORM_MODE, t, r)
                    },
                    init: function(t, r, e) {
                        this.cfg = this.cfg.extend(e),
                        this._xformMode = t,
                        this._key = r,
                        this.reset()
                    },
                    reset: function() {
                        c.reset.call(this),
                        this._doReset()
                    },
                    process: function(t) {
                        return this._append(t),
                        this._process()
                    },
                    finalize: function(t) {
                        return t && this._append(t),
                        this._doFinalize()
                    },
                    keySize: 4,
                    ivSize: 4,
                    _ENC_XFORM_MODE: 1,
                    _DEC_XFORM_MODE: 2,
                    _createHelper: function() {
                        function t(t) {
                            return "string" == typeof t ? x : g
                        }
                        return function(r) {
                            return {
                                encrypt: function(e, n, i) {
                                    return t(n).encrypt(r, e, n, i)
                                },
                                decrypt: function(e, n, i) {
                                    return t(n).decrypt(r, e, n, i)
                                }
                            }
                        }
                    }()
                })




window.__fe
{pad: ƒ, unpad: ƒ}
window.__fe.pad and window.__fe.unpad

                  , h = (r.pad = {}).Pkcs7 = {
                    pad: function(t, r) {
                        for (var e = 4 * r, n = e - t.sigBytes % e, i = n << 24 | n << 16 | n << 8 | n, c = [], u = 0; u < n; u += 4)
                            c.push(i);
                        var a = o.create(c, n);
                        t.concat(a)
                    },
                    unpad: function(t) {
                        var r = 255 & t.words[t.sigBytes - 1 >>> 2];
                        t.sigBytes -= r
                    }
                }

window.__le
'0123456789ABCDEF'

window.__ie
function ie(t, r) {
            var e = 469
              , n = 436
              , i = 421
              , o = 414
              , c = ge
              , u = {
                csLej: function(t, r) {
                    return t == r
                },
                tyWOd: function(t, r) {
                    return t > r
                },
                dUZcv: function(t, r) {
                    return t(r)
                },
                Tuisd: function(t, r) {
                    return t < r
                }
            };
            (u[c(360)](null, r) || u[c(e)](r, t[c(n)])) && (r = t[c(n)]);
            for (var a = 0, s = u[c(i)](Array, r); u[c(o)](a, r); a++)
                s[a] = t[a];
            return s
        }


window.__pe

{
    "iv": {
        "words": [
            808530483,
            875902519,
            943276354,
            1128547654
        ],
        "sigBytes": 16
    },
    "padding": {}
}

window.__ue
{stringify: ƒ, parse: ƒ}

window.__ue.stringify
, f = u.Utf8 = {
stringify: function(t) {
                        try {
                            return decodeURIComponent(escape(s.stringify(t)))
                        } catch (t) {
                            throw new Error("Malformed UTF-8 data")
                        }
                    },
window.__ue.parse
parse: function(t) {
                        return s.parse(unescape(encodeURIComponent(t)))
                    }


window.__ae // Base64 encoder/decoder?
```
```js
// valueof ae

{    "_map": "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=",    "_reverseMap": [        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        null,        62,        null,        null,        null,        63,        52,        53,        54,        55,        56,        57,        58,        59,        60,        61,        null,        null,        null,        64,        null,        null,        null,        0,        1,        2,        3,        4,        5,        6,        7,        8,        9,        10,        11,        12,        13,        14,        15,        16,        17,        18,        19,        20,        21,        22,        23,        24,        25,        null,        null,        null,        null,        null,        null,        26,        27,        28,        29,        30,        31,        32,        33,        34,        35,        36,        37,        38,        39,        40,        41,        42,        43,        44,        45,        46,        47,        48,        49,        50,        51    ]}
```
### Function `ye`
```javascript
function ye(t, r) {
            var e = 442
              , n = 406
              , i = 535
              , o = 385
              , c = 480
              , u = 492
              , a = 468
              , s = 479
              , f = 385
              , l = 436
              , p = 436
              , v = 429
              , h = 385
              , d = 465
              , y = re
              , g = {};
            g[y(e)] = y(n) + y(i),
            g[y(o)] = function(t, r) {
                return t === r
            }
            ,
            g[y(c)] = function(t, r) {
                return t !== r
            }
            ,
            g[y(u)] = function(t, r) {
                return t <= r
            }
            ;
            for (var m = g, x = m[y(e)][y(a)]("|"), w = 0; ; ) {
                switch (x[w++]) {
                case "0":
                    var b = ce[y(s)](C, S, pe);
                    continue;
                case "1":
                    if (m[y(f)](t, void 0) || m[y(c)](t[y(l)], 16) || m[y(u)](r[y(p)], 0))
                        return null;
                    continue;
                case "2":
                    var S = ue[y(v)](t);
                    continue;
                case "3":
                    if (m[y(f)](r, void 0) || m[y(h)](r, null))
                        return null;
                    continue;
                case "4":
                    var C = r;
                    continue;
                case "5":
                    return b[y(d)]()
                }
                break
            }
        }

```
### Function `be`
```javascript
function be(t) {
            var r = 431
              , e = re;
            return {
                PMZDa: function(t, r, e) {
                    return t(r, e)
                }
            }[e(453)](ye, he, t[e(r)]("#"))
        }
```
### Objecy `Ee`
```javascript

        var Ee = {
            ACTION: gt,
            ACTION_STATE: xt,
            KEY_ID: me(pt, ht.ID),
            KEY_SECRET: me(pt, ht[re(371)])
        }

console.log(Ee)
{
    "ACTION": {
        "INIT": "InitCaptcha",
        "INITV2": "InitCaptchaV2",
        "INITV3": "InitCaptchaV3",
        "VERIFY": "VerifyCaptchaV2",
        "VERIFYV3": "VerifyCaptchaV3",
        "LOG": "UploadLog"
    },
    "ACTION_STATE": {
        "SUCCESS": "success",
        "FAIL": "fail"
    },
    "KEY_ID": "LTAI5tSEBwYMwVKAQGpxmvTd",
    "KEY_SECRET": "YSKfst7GaVkXwZYvVihJsKF9r89koz"
}
```
### Object `ke`
```javascript
  , ke = {
            ACTION: nr[re(476)],
            ACTION_STATE: nr[re(407) + re(509)],
            DEVICE_TYPE: nr[re(477) + re(388)],
            WEB_AES_SECRET_KEY: nr[re(523) + re(521) + "EY"],
            KEY_ID: me(nr[re(541) + "EC"], nr[re(485) + "EY"].ID),
            KEY_SECRET: me(nr[re(541) + "EC"], nr[re(485) + "EY"][re(371)]),
            WEB_AES_FLAG_SECRET_KEY: me(nr[re(541) + "EC"], nr[re(523) + re(521) + "EY"][re(524)])
        };

console.log(ke)

{
    "ACTION": {
        "INIT": "Log1",
        "DEVICE_UPLOAD": "Log2",
        "COMBAT_UPLOAD": "Log3"
    },
    "ACTION_STATE": {
        "SUCCESS": "success",
        "FAIL": "fail"
    },
    "DEVICE_TYPE": {
        "WEB": "W"
    },
    "WEB_AES_SECRET_KEY": {
        "REQ": "8KmHIQsc5+LZJA7uYex3WaHdkjgCtS6epbG/bc9xss0=",
        "RES": "9NhnQQ+LRrKCkAuxwZaUWGBtSzaFtFlNb/ksJCrCgrM=",
        "FLAG": "k+1RW0cz3iDi2RAbC/c3QKzTPiVwNmNO1910DXe6Gas=",
        "UPLOAD": "+fR9tYzlKFr07pEbumd7+KnO3xLOkphCS+qKUbJiMfA=",
        "PREID": "xLLw/t15vkI7QQBX/1scBbcb9fKx+ymxF0tJ3ds42B0="
    },
    "KEY_ID": "LTAI5tGjnK9uu9GbT9GQw72p",
    "KEY_SECRET": "fpOKzILEajkqgSpr9VvU98FwAgIRcX",
    "WEB_AES_FLAG_SECRET_KEY": "c175a358550d02e2"
}
```

---

## 3. Function `Re` Internals: DeviceData Creation [AliyunCaptcha.js]

```javascript
var nr = new Jt({});
// nr is related to Jt
```

In context of function Re, a proxy was added: window.__value_of_r = r;
```javascript
function Re(t, r, e) {
            r._extend(Be({}, t));
            var n = t.appKey || r.APP_KEY
              , i = t.appName || r.APP_NAME
              , o = me(r.ACCESS_SEC, r.secretKey) || e.WEB_AES_FLAG_SECRET_KEY
              , c = r.PLATFORM + "#" + i + "#" + (r.sceneId || "") + "#captcha-normal#" + De.prefix + "#" + De.region;
            window.__value_of_r = r;
            return c = ye(o, c),
            window.__value_of_r_secretKey = r.secretKey;
	    console.log('Value of o', o)
            be([n, e.DEVICE_TYPE.WEB, c, r.APP_VERSION, "CLOUD", ""])
        }
```
**Notes:**

- `var n` (`t.appKey || r.APP_KEY`) can have multiple values based on region and version via `Rt.appKey`. Same applies to `var i`.
- `var o` uses `e.WEB_AES_FLAG_SECRET_KEY` because `r.secretKey` is undefined. Testing `window.__value_of_r_secretKey` returned undefined, while `window.__value_of_r.ACCESS_SEC` returned `'FqJB6iRNVYdEGpwb'`.
- Logging `r.ACCESS_KEY` returns: `{"ID": "tBwmiXXwEdGaRtkcLvIoA/F6r4qktDu7lFi23FmDCbo=", "SECRET": "9U6Kg6ljgm9PhHvSsihjy1wJVo71uzWkRDfGiyiMVBg="}`.

Full log of `r`:
```json
{
    "preCollectData": {
        "fontsNum": 4
    },
    "ENDPOINTS": [
        "https://cloudauth-device-dualstack.cn-shanghai.aliyuncs.com",
        "https://cn-shanghai.device.saf.aliyuncs.com"
    ],
    "sceneId": "didk33e0",
    "appName": "saf-captcha",
    "appKey": "ab034ec0643f91399eb33e062dc7fae1",
    "endpoints": [
        "https://cloudauth-device-dualstack.cn-shanghai.aliyuncs.com",
        "https://cn-shanghai.device.saf.aliyuncs.com"
    ]
}

```

`r._extend` Definition:
```javascript
_extend: function(t) {
                var r = G
                  , e = this;
                new q(t)[r(505)](function(t, r) {
                    e[t] = r
                })
            }
```
	    
### Decryption Script (Python)

```python3
from Crypto.Cipher import AES
from Crypto.Util.Padding import unpad
import base64

def decrypt_ye(encrypted_b64, key_str, iv_words):
    """
    Decrypt data encrypted by the ye function
    """                                                                                              # Convert key string to bytes (AES-128 = 16 bytes)
    key = key_str.encode('utf-8')

    # Convert iv words to bytes
    iv_bytes = []
    for word in iv_words:
        # Each word is 32-bit integer, big-endian
        iv_bytes.extend([
            (word >> 24) & 0xFF,
            (word >> 16) & 0xFF,
            (word >> 8) & 0xFF,
            word & 0xFF
        ])
    iv = bytes(iv_bytes[:16])  # Take first 16 bytes

    # Base64 decode the encrypted data
    encrypted = base64.b64decode(encrypted_b64)

    # Decrypt using AES-CBC
    cipher = AES.new(key, AES.MODE_CBC, iv)
    decrypted_padded = cipher.decrypt(encrypted)

    # Remove PKCS7 padding
    decrypted = unpad(decrypted_padded, AES.block_size)

    # Return as string
    return decrypted.decode('utf-8')

# Your data
encrypted = "V0z3lM83c/TjjZu6HWeg95/xMYKFKoBmiixMa2THPUZkDnFK7GAZFGfreQXa7zu4FGY0zm1H4bU9uybpP2Fv0w=="
key = 'c175a358550d02e2'  # 16 bytes
iv_words = [808530483, 875902519, 943276354, 1128547654]

# Decrypt
result = decrypt_ye(encrypted, key, iv_words)
print(result)
```

---

## 4. Information of `Rt` in Context of `Rt.appkey` [AliyunCaptcha.js]
```json
{
    "appName": {
        "1.0": "saf-captcha-waf",
        "2.0": "saf-captcha",
        "3.0": "saf-captcha"
    },
    "appKey": {
        "1.0": {
            "cn": "sh87bd1512hsb03cb405803a307dbe32",
            "sgp": "sg63c0a034gsf3f3ed381ac9c8a0bc51",
            "ga": "sg63c0a034gsf3f3ed381ac9c8a0bc51"
        },
        "2.0": {
            "cn": "ab034ec0643f91399eb33e062dc7fae1",
            "sgp": "3795d28242a11619bc25f786f84e53d4",
            "ga": "3795d28242a11619bc25f786f84e53d4"
        },
        "3.0": {
            "cn": "ab034ec0643f91399eb33e062dc7fae1",
            "sgp": "3795d28242a11619bc25f786f84e53d4",
            "ga": "3795d28242a11619bc25f786f84e53d4"
        }
    },
    "endpoints": {
        "1.0": {
            "cn": [
                "https://device.captcha-open.aliyuncs.com"
            ],
            "sgp": [
                "https://cloudauth-device-dualstack.ap-southeast-1.aliyuncs.com",
                "https://ap-southeast-1.device.saf.aliyuncs.com"
            ],
            "ga": [
                "https://cloudauth-device-dualstack.ap-southeast-1.aliyuncs.com",
                "https://ap-southeast-1.device.saf.aliyuncs.com"
            ]
        },
        "2.0": {
            "cn": [
                "https://cloudauth-device-dualstack.cn-shanghai.aliyuncs.com",
                "https://cn-shanghai.device.saf.aliyuncs.com"
            ],
            "sgp": [
                "https://cloudauth-device-dualstack.ap-southeast-1.aliyuncs.com",
                "https://ap-southeast-1.device.saf.aliyuncs.com"
            ],
            "ga": [
                "https://ap-southeast-1-ga.device.saf.aliyuncs.com",
                "https://cloudauth-device-dualstack.ap-southeast-1.aliyuncs.com"
            ]
        },
        "3.0": {
            "cn": [
                "https://cloudauth-device-dualstack.cn-shanghai.aliyuncs.com",
                "https://cn-shanghai.device.saf.aliyuncs.com"
            ],
            "sgp": [
                "https://cloudauth-device-dualstack.ap-southeast-1.aliyuncs.com",
                "https://ap-southeast-1.device.saf.aliyuncs.com"
            ],
            "ga": [
                "https://ap-southeast-1-ga.device.saf.aliyuncs.com",
                "https://cloudauth-device-dualstack.ap-southeast-1.aliyuncs.com"
            ]
        }
    },
    "shplEndpoints": {
        "1.0": {
            "cn": [
                "https://device.captcha-open.aliyuncs.com"
            ],
            "sgp": [
                "https://cloudauth-device-dualstack.ap-southeast-1.aliyuncs.com",
                "https://ap-southeast-1.device.saf.aliyuncs.com"
            ],
            "ga": [
                "https://cloudauth-device-dualstack.ap-southeast-1.aliyuncs.com",
                "https://ap-southeast-1.device.saf.aliyuncs.com"
            ]
        },
        "2.0": {
            "cn": [
                "https://cn-shanghai.device.saf.aliyuncs.com"
            ],
            "sgp": [
                "https://ap-southeast-1.device.saf.aliyuncs.com"
            ],
            "ga": [
                "https://ap-southeast-1-ga.device.saf.aliyuncs.com"
            ]
        },
        "3.0": {
            "cn": [
                "https://cn-shanghai.device.saf.aliyuncs.com"
            ],
            "sgp": [
                "https://ap-southeast-1.device.saf.aliyuncs.com"
            ],
            "ga": [
                "https://ap-southeast-1-ga.device.saf.aliyuncs.com"
            ]
        }
    }
}

```


### Simplified Deobfuscated Logic

```javascript
var Rt = {
    appKey: {
        "1.0": {
            "cn": G(569) + G(481) + G(363) + G(565),      // sh87bd15 + 12hsb03c + b405803a + 307dbe32
            "sgp": G(434) + G(490) + G(523) + G(498),     // sg63c0a0 + 34gsf3f3 + 7ef9e8a2 + c8a0bc51
            "ga": G(434) + G(490) + G(523) + G(498)       // Same as sgp
        },
        "2.0": {
            "cn": G(345) + G(526) + G(424) + G(322),      // ab034ec0 + 643f9139 + 9eb33e06 + 2dc7fae1
            "sgp": G(337) + G(589) + G(588) + G(455),     // 3795d282 + 42a11619 + bc25f786 + f84e53d4
            "ga": G(337) + G(589) + G(588) + G(455)       // Same as sgp
        },
        "3.0": {
            "cn": G(345) + G(526) + G(424) + G(322),      // Same as 2.0
            "sgp": G(337) + G(589) + G(588) + G(455),     // Same as 2.0
            "ga": G(337) + G(589) + G(588) + G(455)       // Same as 2.0
        }
    }
}
```


## Calculation  of literals using `rr`:

**Step 1: `jt()` array contains raw fragments**
```javascript
function jt() {
    var t = ["Ac1KzxzPy2u", "DxrOzwfZDc0", ... "ab034ec0", "643f9139", ...];
    return (jt = function() { return t })();
}
```
**Step 2: Decoder function `rr`**

- Min Index: 312
- Max Index: 595
```javascript
var G = rr; 
function rr(t, r) {
    var e = jt();  // Gets the string array
    return rr = function(r, n) {
        var i = e[r -= 312];  // Subtract 312 to get array index
        if (void 0 === rr.avoPoV) {
            rr.nyBjcA = function(t) {
                // Base64-like decoding function
                for (var r, e, n = "", i = "", o = 0, c = 0; e = t.charAt(c++); ~e && (r = o % 4 ? 64 * r + e : e, o++ % 4) ? n += String.fromCharCode(255 & r >> (-2 * o & 6)) : 0)
                    e = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/=".indexOf(e);
                for (var u = 0, a = n.length; u < a; u++)
                    i += "%" + ("00" + n.charCodeAt(u).toString(16)).slice(-2);
                return decodeURIComponent(i)
            },
            t = arguments,
            rr.avoPoV = !0
        }
        var o = r + e[0],
            c = t[o];
        return c ? i = c : (i = rr.nyBjcA(i), t[o] = i),
        i
    }, rr(t, r)
}
```

**Step 3: `Ft` (becomes `Rt`) definition**
```javascript
var Ft = {};
Ft[G(548)] = Pt,        // G(548) = "appName"
Ft[G(519)] = Nt,        // G(519) = "appKey"  
Ft[G(430) + "s"] = Ht,  // G(430) = "endpoint" → "endpoints"
Ft[G(421) + G(411)] = Wt // G(421) = "shplEndp", G(411) = "oints" → "shplEndpoints"
```

**Step 4: `Nt` (appKey) definition**
```javascript
var Nt = {};
// For 1.0 version
Nt[G(532)] = {};  // G(532) = "1.0"
Nt[G(532)][G(441)] = G(569) + G(481) + G(363) + G(565);  // G(441) = "WEB" or "cn"?
// Actual: "sh87bd15" + "12hsb03c" + "b405803a" + "307dbe32"

Nt[G(532)][G(520)] = G(434) + G(490) + G(523) + G(498);  // G(520) = "sgp"
// Actual: "sg63c0a0" + "34gsf3f3" + "7ef9e8a2" + "c8a0bc51"

// For 2.0 version
Nt[G(487)] = {};  // G(487) = "2.0"
Nt[G(487)][G(441)] = G(345) + G(526) + G(424) + G(322);  // "ab034ec0" + "643f9139" + "9eb33e06" + "2dc7fae1"
Nt[G(487)][G(520)] = G(337) + G(589) + G(588) + G(455);  // "3795d282" + "42a11619" + "bc25f786" + "f84e53d4"

// For 3.0 version (same as 2.0)
Nt[G(423)] = {};  // G(423) = "3.0"
Nt[G(423)][G(441)] = G(345) + G(526) + G(424) + G(322);
Nt[G(423)][G(520)] = G(337) + G(589) + G(588) + G(455);
```

---

## 5. Function `rr` Index Deobfuscated Literals JSON Log [AliyunCaptcha.js]

```json
{  "312": "1662178hogNeO",  "313": "theast-d",  "314": "1NfQv95E",  "315": "SG_WEB_P",  "316": "dZJMm",  "317": "re-b.ali",  "318": "HaTtw",  "319": "0.0.0/fe",  "320": "theast.a",  "321": "9NhnQQ+L",  "322": "2dc7fae1",  "323": "ual.aliy",  "324": "z2k=",  "325": "PiVwNmNO",  "326": "a.device",  "327": "ptchaV3",  "328": "-fronten",  "329": "SG_WEB",  "330": "INIT",  "331": "hai.devi",  "332": "theast-b",  "333": "Yex3WaHd",  "334": "AIN_FAIL",  "335": "W8YrgOBc",  "336": "split",  "337": "3795d282",  "338": "liyuncs.",  "339": "DNsKKPRH",  "340": "S+qKUbJi",  "341": "2B0=",  "342": "/captcha",  "343": "ap-south",  "344": "aptcha-o",  "345": "ab034ec0",  "346": "pbG/bc9x",  "347": ".saf.ali",  "348": "FLAG",  "349": "Log3",  "350": "2VS3zA==",  "351": "7JLsB18M",  "352": "success",  "353": "theast-p",  "354": "cha",  "355": "ONUTG",  "356": "fail",  "357": "l.aliyun",  "358": "aliyuncs",  "359": "EdGaRtkc",  "360": "gp.aliyu",  "361": "web-pre.",  "362": "heast-1.",  "363": "b405803a",  "364": "evice.sa",  "365": "3xLOkphC",  "366": ".com",  "367": "2415840gMPfPM",  "368": "hanghai.",  "369": "captcha-",  "370": "INITV3",  "371": "VERIFY",  "372": "UploadLo",  "373": "com",  "374": "8KmHIQsc",  "375": "M0v7u45+",  "376": "re.aliyu",  "377": "pen.aliy",  "378": "open-b.a",  "379": "DEVICE_U",  "380": "af.aliyu",  "381": "-1.devic",  "382": "COMBAT_U",  "383": "east-1.d",  "384": "outheast",  "385": "web-b.al",  "386": "ck.ap-so",  "387": "ncs.com/",  "388": "d/FeiLin",  "389": "sg50c495",  "390": "open-pre",  "391": "iyuncs.c",  "392": "ptchaV2",  "393": "pre-ap-s",  "394": "Log1",  "395": "open.ali",  "396": "VYdEGpwb",  "397": "-b.aliyu",  "398": "W.10001.",  "399": "ss0=",  "400": "F0tJ3ds4",  "401": "ncs.com",  "402": "95bc895c",  "403": "vkI7QQBX",  "404": "d/aliyun",  "405": ".aliyunc",  "406": "ed381ac9",  "407": "RES",  "408": "Vo71uzWk",  "409": "X1y5Vstb",  "410": "5+LZJA7u",  "411": "oints",  "412": "NIT_FAIL",  "413": "FAIL",  "414": "DEVICE_M",  "415": "SzaFtFlN",  "416": "W2022020",  "417": "cloudaut",  "418": "c0ad7983",  "419": "chaV2",  "420": "tBwmiXXw",  "421": "shplEndp",  "422": "Cbo=",  "423": "3.0",  "424": "9eb33e06",  "425": "REFRESH_",  "426": "wZaUWGBt",  "427": "9ebbf3d0",  "428": "east-1-g",  "429": "r4qktDu7",  "430": "endpoint",  "431": "5169504KEaUDm",  "432": "open-sou",  "433": "/1scBbcb",  "434": "sg63c0a0",  "435": "/main.cs",  "436": "dev.o.al",  "437": "C/c3QKzT",  "438": "aptcha-p",  "439": "VzY=",  "440": "sh3c47a8",  "441": "WEB",  "442": "yuncs.co",  "443": "com/",  "444": ".js",  "445": "853792CgZYEZ",  "446": "ual-b.al",  "447": "9U6Kg6lj",  "448": "chaV3",  "449": "Gas=",  "450": "SUCCESS",  "451": "aptcha-s",  "452": "ddhs0305",  "453": "INITV2",  "454": "807878SVsivw",  "455": "f84e53d4",  "456": "PLOAD",  "457": "xLLw/t15",  "458": "anghai.a",  "459": "NiaNh",  "460": "518MUXIlt",  "461": "e.saf.al",  "462": "static-c",  "463": "83f5e541",  "464": "k+1RW0cz",  "465": "75184fAvUYY",  "466": "InitCapt",  "467": "open-ga-",  "468": "NLAoqT6K",  "469": "LTXPs",  "470": "aptcha.a",  "471": "/bfozcSz",  "472": "fOTuFph6",  "473": "h-device",  "474": "utheast-",  "475": "device.c",  "476": "SECRET",  "477": "southeas",  "478": "LOG",  "479": "XiMPo",  "480": "KFr07pEb",  "481": "12hsb03c",  "482": "l-b.aliy",  "483": "open-dua",  "484": "saf-aliy",  "485": "www.aliy",  "486": "WEB_PREI",  "487": "2.0",  "488": "1910DXe6",  "489": "t-1.aliy",  "490": "34gsf3f3",  "491": "mosql",  "492": "n.js",  "493": "un.com/",  "494": "bBLLx",  "495": "cJS/",  "496": "now",  "497": "_l_KPLiP",  "498": "c8a0bc51",  "499": "ilin",  "500": "PREID",  "501": "dev.g.al",  "502": "-pre.ali",  "503": "FqJB6iRN",  "504": "o.alicdn",  "505": "_each",  "506": "S_FAIL",  "507": "GbSWQ",  "508": "un-com",  "509": "VBg=",  "510": "REQ",  "511": "f.aliyun",  "512": "saf-capt",  "513": "REID",  "514": "202130TawWqM",  "515": "taPHkC+T",  "516": "3iDi2RAb",  "517": "pre-cn-s",  "518": "cn_dual",  "519": "appKey",  "520": "sgp",  "521": "grM=",  "522": "BdGKm",  "523": "7ef9e8a2",  "524": "cha-waf",  "525": "https://",  "526": "643f9139",  "527": "g3E=",  "528": "8fgs168b",  "529": "-dualsta",  "530": "nA7GX3d6",  "531": "sgp_dual",  "532": "1.0",  "533": "B5zqghyJ",  "534": "d35db7e3",  "535": "g.alicdn",  "536": "Plkdn",  "537": "RrKCkAux",  "538": "2023-03-",  "539": "LvIoA/F6",  "540": "pro-open",  "541": "OhpiO",  "542": "n9jH0yAC",  "543": "sihjy1wJ",  "544": "web.aliy",  "545": "bC6YUaXi",  "546": "kjgCtS6e",  "547": "prototyp",  "548": "appName",  "549": "03oLbQXW",  "550": "lFi23FmD",  "551": "-pre-b.a",  "552": "1.aliyun",  "553": "+fR9tYzl",  "554": "-pre.ap-",  "555": ".ap-sout",  "556": "uncs.com",  "557": "LIMIT_FL",  "558": "DEVICE_I",  "559": "VERIFYV3",  "560": "T68xcVuO",  "561": "MfA=",  "562": "INIT_FAI",  "563": "OTHER",  "564": "UPLOAD",  "565": "307dbe32",  "566": "PIC_FAIL",  "567": "_c_WBKFR",  "568": "cs.com",  "569": "sh87bd15",  "570": "FP/fp.mi",  "571": "9fKx+ymx",  "572": "cn-shang",  "573": "Aoxz0b7v",  "574": "d/dynami",  "575": "gp-pre.a",  "576": "device.s",  "577": "umd7+KnO",  "578": "s.com",  "579": "http://",  "580": "RDfGiyiM",  "581": "icdn.com",  "582": "VerifyCa",  "583": "8ZpvzGBX",  "584": "9g8a0A==",  "585": "LxErT1sG",  "586": "01083105",  "587": "ck.cn-sh",  "588": "bc25f786",  "589": "42a11619",  "590": "2020-10-",  "591": "Log2",  "592": "ce.saf.a",  "593": "b/ksJCrC",  "594": "gm9PhHvS",  "595": "DYNAMICJ"}
```
---

## DeviceData Generation [AliyunCaptcha.js]

**Function Re generates DeviceData** which is sent by function Pe

**Function `Pe` Definition**

```javascript
function Pe() {
            return Pe = k()(P().mark(function t(r, e, n, i) {
                var o, c, u, a, s, f, l, p, v, h, d, y, g, m, x;
                return P().wrap(function(t) {
                    for (; ; )
                        switch (t.prev = t.next) {
                        case 0:
                            return De._extend({
                                initBeginTime: Date.now(),
                                logUploaded: !1,
                                logInfo: {}
                            }),
                            xr("sId", r.SceneId),
                            o = e.https,
                            c = e.initPath,
                            u = e.isDev,
                            a = e.verifyType,
                            s = o,
                            f = Qe(e),
                            l = tn(r, e),
                            p = l.action,
                            xr("pfx", v = l._prefix),
                            f = N()(f).call(f, function(t) {
                                return v + "." + t
                            }),
                            h = N()(f).call(f, function(t) {
                                return lr(s, t, c)
                            }),
                            De._extend({
                                urls: h
                            }),
                            d = i.deviceConfig,
                            y = i.deviceCallback,
                            "1.0" === a ? (delete r.DeviceToken,
                            Ie = new Jt) : e.userId && e.userUserId && (De._extend({
                                userId: void 0,
                                userUserId: void 0
                            }),
                            Ie = new Jt),
                            $e(d.endpoints, d.appName),
                            g = Re(d, Ie, ke),
                            e.isFromTraceless || void 0 !== Ie.DeviceConfig || (r.DeviceData = g),
                            t.next = 1,
                            Ne(p, r, h, e, Ee);
                        case 1:
                            !(m = t.sent).Success || m.LimitFlow || m.LimitedFlowToken ? (m.LimitedFlowToken ? m.CertifyId = m.LimitedFlowToken : m.CertifyId || (m.CertifyId = dr().substring(0, 5)),
                            xr("cId", m.CertifyId),
                            n(Ee.ACTION_STATE.FAIL, m)) : (e._extend({
                                log: nn
                            }),
                            xr("cId", m.CertifyId),
                            !e.isFromTraceless && De._extend({
                                initialRequestTime: Date.now(),
                                overTime: !1
                            }),
                            m.DeviceConfig && void 0 === Ie.DeviceConfig && Ie._extend({
                                DeviceConfig: m.DeviceConfig
                            }),
                            rn(m.DeviceConfig, y, u, "captcha"),
                            x = Se(m, e),
                            n(Ee.ACTION_STATE.SUCCESS, x));
                        case 2:
                        case "end":
                            return t.stop()
                        }
                }, t)
            })),
            Pe.apply(this, arguments)
        }
```

**Function `Re` Definition**
```javascript
        function Re(t, r, e) {
            r._extend(Be({}, t));
            var n = t.appKey || r.APP_KEY
              , i = t.appName || r.APP_NAME
              , o = me(r.ACCESS_SEC, r.secretKey) || e.WEB_AES_FLAG_SECRET_KEY
              , c = r.PLATFORM + "#" + i + "#" + (r.sceneId || "") + "#captcha-normal#" + De.prefix + "#" + De.region;
            return c = ye(o, c),
            be([n, e.DEVICE_TYPE.WEB, c, r.APP_VERSION, "CLOUD", ""])
        }
```

**Notes:** 
- 1. `var n` in context of `var n = t.appKey || r.APP_KEY can have multiple values since it likely gets it values from Rt.appKey which have different values based on region and version. This is same for `var i` im not sure but it seems obviuous 
- 2. in context of `var o` the key used is e.WEB_AES_FLAG_SECRET_KEY since r.secretKey is undefined and we should stick to it since it works 


### Function Re reconstructed in python

[generate_device_data.py](../generate_device_data.py)

## 7. Signature and Nonce Generation [AliyunCaptcha.js]

Every request is signed with an HMAC-SHA1 Signature and Signature Nonce.

**Function `je` definition**
```javascript
function je() {
            return je = k()(P().mark(function t(r, e, n, i, o) {
                var c, u;
                return P().wrap(function(t) {
                    for (; ; )
                        switch (t.prev = t.next) {
                        case 0:
                            return (c = {}).AccessKeyId = o.KEY_ID,
                            c.SignatureMethod = "HMAC-SHA1",
                            c.SignatureVersion = "1.0",
                            c.Format = "JSON",
                            c.Timestamp = hr(),
                            c.Version = lt,
                            c.Action = r,
                            ir(e) || (c = or(c, e)),
                            u = function() {
                                var t = k()(P().mark(function t(r) {
                                    var e, a, s, f, l, p, v, h;
                                    return P().wrap(function(t) {
                                        for (; ; )
                                            switch (t.prev = t.next) {
                                            case 0:
                                                return c.SignatureNonce = dr(),
                                                a = Ce(c, o.KEY_SECRET),
                                                c.Signature = a,
                                                s = Date.now(),
                                                t.next = 1,
                                                Ke(n[r], c, i);
                                            case 1:
                                                if (f = t.sent,
                                                l = Date.now(),
                                                p = f.Code,
                                                v = f.Success,
                                                h = Gr()(e = n[r]).call(e, "-b") ? "bInit" : "mInit",
                                                !("Success" === p && v || r >= n.length - 1)) {
                                                    t.next = 2;
                                                    break
                                                }
                                                return "Success" === p && v ? (xr(h, {
                                                    t: l,
                                                    s: !0,
                                                    msg: "INIT_SUCCESS",
                                                    rt: l - s
                                                }),
                                                Ge(r)) : xr(h, {
                                                    t: l,
                                                    s: !1,
                                                    msg: f.err,
                                                    rt: l - s
                                                }),
                                                t.abrupt("return", f);
                                            case 2:
                                                if (xr(h, {
                                                    t: l,
                                                    s: !1,
                                                    msg: f.err || f.Message,
                                                    rt: l - s
                                                }),
                                                !("403" === p && f.LimitedFlow || "ThrottlingByStrategy" === p)) {
                                                    t.next = 3;
                                                    break
                                                }
                                                return t.abrupt("return", f);
                                            case 3:
                                                return t.next = 4,
                                                u(r + 1);
                                            case 4:
                                                return t.abrupt("return", t.sent);
                                            case 5:
                                            case "end":
                                                return t.stop()
                                            }
                                    }, t)
                                }));
                                return function(r) {
                                    return t.apply(this, arguments)
                                }
                            }(),
                            t.next = 1,
                            u(0);
                        case 1:
                            return t.abrupt("return", t.sent);
                        case 2:
                        case "end":
                            return t.stop()
                        }
                }, t)
            })),
            je.apply(this, arguments)
        }
```

**Note:** `o.KEY_SECRET` points to `var Ee.KEY_SECRET` which is `"YSKfst7GaVkXwZYvVihJsKF9r89koz"`.

### Python Implementation (`signature_gen.py`)
[Signature.py](../signature_gen.py)

---

## 8. Traceless CaptchaFlow
### 1. AliyunCaptcha.js Loads

### 2. Loads `feilin.*.js`

`feilin.js` likely generates `DeviceToken` and exposes a call:
- `window.um?.getToken`
- `window.z_um?.getToken`

### 3. InitCaptchaV3
fetch request
```javascript
fetch("https://no8xfe.captcha-open-southeast.aliyuncs.com/", {"headers": {"accept": "*/*",},
  "body": "AccessKeyId=LTAI5tSEBwYMwVKAQGpxmvTd&SignatureMethod=HMAC-SHA1&SignatureVersion=1.0&Format=JSON&Timestamp=2026-05-25T02%3A42%3A52Z&Version=2023-03-05&Action=InitCaptchaV3&SceneId=didk33e0&Language=en&Mode=popup&UpLang=true&DeviceData=TEQYvgJq1LrMqFaBybfIzPxz2ygFyAct7X%2Fw%2BLacfXWd9rGSwE%2Fx6ZCONucD1fehS2Qpig6tUVsFK111d9wIk5pWp6rwYjzFCRgL7pNp8bzGsvOSdUXgQTopQm90YPSdCiRAlgENdODLvY7P8jrfO9eC15tPCPwLxcRIrcspVvQYqVfk9%2FyFeIlePKmTRjkM&SignatureNonce=9aab2319-86bf-4019-839e-82225f796296&Signature=UDiQcb82lN5kZT0ZPNOmw0hT8sw%3D","method":"POST","mode": "cors","credentials": "omit"});
```
**Note:**
DeviceData is sent on first request, it is determined by if the previous response inlucded CaptchaTypre Traceless or not since before  0 there is nothin same as this there was no request so it is fired with dev_data and response contains device config
On next requst DeviceToken is sent if previous response inclueded TRACELESS captcha type

**Successful Response:**
```json
{
    "CertifyId": "gl62Aqi6e1",
    "Message": "success",
    "RequestId": "65A5902F-1573-4BA3-BB81-898D35553B23",
    "Code": "Success",
    "LimitFlow": false,
    "Success": true,
    "StaticPath": "3.25.0/pe.050.9453665072ad79a7",
    "CaptchaType": "TRACELESS",
    "DeviceConfig": "ckj8rA4/WEfd9fOcvSSuIJ6e0IDZR3X79geskaQ1tN6EJBdqOGupxhCCoHnlEZsxgowJxh2znY9HEY2Ruj07n/YhavNvqMCzc3ppJQkefLtCHXx7PUpO8NhrB+OV2e3AnSJ1oRFgSX7sWxZoUSoLNYNh8QbZOYMHO+ShVv5BADyF02OloVszuUsNbbDeDYId2moM/OpnagWMpLGNJJiVFOfHevUTtYQTsNN/X9iCN3RayeHFtcgxBjhEZcfHPzNVYn6bCGyZ2mCVE8J1swWnPFKiSJuiWiAxX4Qsn5pOtWs="
}
```

```javascript
Object.keys(AliyunCaptcha.prototype)
["config","deviceConfig","startPOWCalculation","init","bindEvents","show","hide","loading","onBizSuccess","onBizFail","initPopup","initEmbed","initFloat","destroyCaptcha","refresh","onCloseClick","startTracelessVerification"
]
```

`StaticPath` loads a script from `https://g.alicdn.com/captcha-frontend/dynamicJS/{StaticPath}.js`. This script calculates the captcha payload `data` field sent in `VerifyCaptchaV3`. All these dynamic JS files calculate data identically (just renamed variables/functions). MITMProxy can force a single JS file every time.

### 4. VerifyCaptchaV3
```javascript
fetch("https://no8xfe.captcha-open-southeast.aliyuncs.com/", {
  "headers": { "accept": "*/*" },
  "body": "AccessKeyId=LTAI5tSEBwYMwVKAQGpxmvTd&SignatureMethod=HMAC-SHA1&SignatureVersion=1.0&Format=JSON&Timestamp=2026-05-25T02%3A43%3A00Z&Version=2023-03-05&Action=VerifyCaptchaV3&SceneId=didk33e0&CertifyId=gl62Aqi6e1&CaptchaVerifyParam=%7B%22sceneId%22%3A%22didk33e0%22%2C%22certifyId%22%3A%22gl62Aqi6e1%22%2C%22deviceToken%22%3A%22U0dfV0VCIzM3OTVkMjgyNDJhMTE2MTliYzI1Zjc4NmY4NGU1M2Q0LWgtMTc3OTY3Njk3NzYzMC1kMjA1YTU2MjFmMTY0NDA2YjI5ZTQyMGRkNTUxMzI1MSM3dTU1SjBYc09Uc1IzaXVKS0Zhb09kY1dwVEtxcDBsN2NlRTBHd3VhVy9UVjZ4ZnpVWVhBMWtZQnNWR2pza3A1MW0wak9lMDNadFdHdDV6b3pTOElNOEFPR21ydHUwbnU2ZDdWZUlLbVIrMzNPVkZUYW1VcGtRK0VtVHBTQ1NMa3ZjcXB0NDdFTXdyOFNOMW42SnpWZ0sxMDN6QzA1Q1g1T1Y1bnBSV3hZREdzUFhRVnpTcytxWEFqczhJS3ZiU1dZb0tUNVFFM2ZVR2ZwamNlclNoaWkzSG50OU0ySmlmWmd2amxzSlowYVNqQ0JiL1kwTGlGUlRST1I2Y2FydThMbEdPTFA4TUUxem9hQ1JKWjlSeEE3ekcyVjRmdnBBa0tsU1U4aFZTVkpDMHl2N1ZCaFhuSDMrSWFTWUh1TitRR00vdndoYllqOVNPVUZhQjJDYllQU0xpandobS9HTk1QcVBBQ2xVNllNQndDTVRuYk40MDJmTVorRTVTVVU0eGNXYnBvTnJhdlg1MDVLbktUTEcwbGVmTmpmZVd4VThDL1doWDBVQ1FjZXl1S1owMjM1QjFlZys2cEI3Y2RFR0ZML2xkRTY5VzlkN1U3bzRMYnVYT1IwemZWeDlQKzhmNStBL3dySmZyZXcrSkJBelE4Wm8zYkxScEtac2RjMFZpSENza2ZQeUkwWSsyQStCaHpvd0Q4QXdMQ2tIZisyZ2tBK2xVaGdyRGhEcWlqNU5nL21YSTdwdVFEbUJkRjFKUllzY3ZCbU53OUt4T09xQVBMN2tvaFRqZGw4TjljWUVTdm0zRTQ0bEJ5d21WZEQyOVFmeWwzOWYrbmlHdlJGc0lDaDJMK3ZIMStlTFNBN3B4MFpWV2pMUVI1eUpyeHZ0eUxQVVgxQ2JpZE9FNkp1bEZpd1BpbnlYK0RvTHBEQVZTdnFqRjNWMVY5bVJtMjVqL1YwaDBhMkhkWWV1UTNQNlVOWUtDNk1wc3BPM2JqMmdSR1J1VDA2V050NVRmZWdETWNWeWVFeEZpYWxPWHZia3dhbjZ3ZWYvbEVGUHF3RUVPZ1ROWDAxdEtXUm1VNWxSOGJKQVdjaUhCTnJQOEpOY0FyMGV0Z2xjd1RxVVhOTngrZ2xrYzdkMzJ3Zk5tZ1BCUjV3Q3h3NjdIWTUzVTI3dHNQMjRVVHVOaGc3SkhjNk5kdDBmQ0JvUDRXdmxpaFdEbFBQQ3Vic1FObFZBS1VYZ0JoYUFDMUVMbmt0Ukt6eEZXNWJ1TzlTOUVDT0RBQWdTNVBrZzIzVldpRTJCOW9DbkFyRERpdVpwZDY4bUpHdnljemdZeXFhd0pWSlp6TzVoaTNoazkvNDdlcy85dExGbzAvMmlmcTVTS01kRTdhVkZlNnRtREVCdz09IzAjN2JkNzMyZDIwZTc1NmEyNmY0OTU1ZGJhYjU4MjhjOWE%3D%22%2C%22data%22%3A%22JRMlgg0gDgVASQIXRQRNMFsDRJoGeSB5eQp9YWNPH%2Fw6iT2%2B96hKSApjEY9S1AZt4rwnZQKgGGOjoeIYWpEaAJdJFhEbK15pcCcKYi0%2FNEgaXU4YLj19be54MEMWEBqDqzmjFdV5KuAcSYQ%2FYWYLNeJ3jZ8VzTdU%2FwO4QqMVb8BXhzAENTBkSQ1iXQhwvkMi01UB7yClAj59DW9NKmNkaj1eS%2BQpfSlvKJ1iF2IXEDcbqi4rbP%2B3BDaUnxMkbo0aLSs8RXxbQH8%3D%22%7D&SignatureNonce=bae00c65-c089-41fc-a704-84576ebea034&Signature=azbdVXlzzucKOIz1kICXOmafPJg%3D",
  "method": "POST",
  "mode": "cors",
  "credentials": "omit"
});
```
Response:
```json
{
    "RequestId": "F06551B1-2C26-453F-B570-AB7687D82551",
    "Message": "success",
    "HttpStatusCode": 200,
    "Code": "Success",
    "Success": true,
    "Result": {
        "securityToken": "6oOo7e72nA61uVLiZVKiLYqF1m9rOno3vEIPJKaL7KLxCJqb1UBwRpl4p7EcFTgd3yG06TCPBjR35MbCZ5lDrdjPcqaflqbQLZQdX2rYd/8bhnqhIpC7SnRlIxGPsqvX",
        "VerifyCode": "T001",
        "VerifyResult": true,
        "certifyId": "gl62Aqi6e1"
    }
}
```

### 5. `captcha_verify_param` generation and usage

`captcha_verify_param` is a base64 encoded string containing scene id, certify id, and token, sent in completions requests.
```base64
eyJjZXJ0aWZ5SWQiOiJnbDYyQXFpNmUxIiwic2NlbmVJZCI6ImRpZGszM2UwIiwiaXNTaWduIjp0cnVlLCJzZWN1cml0eVRva2VuIjoiNm9PbzdlNzJuQTYxdVZMaVpWS2lMWXFGMW05ck9ubzN2RUlQSkthTDdLTHhDSnFiMVVCd1JwbDRwN0VjRlRnZDN5RzA2VENQQmpSMzVNYkNaNWxEcmRqUGNxYWZscWJRTFpRZFgycllkLzhiaG5xaElwQzdTblJsSXhHUHNxdlgifQ==
```
Decoded:
```json
{"certifyId":"gl62Aqi6e1","sceneId":"didk33e0","isSign":true,"securityToken":"6oOo7e72nA61uVLiZVKiLYqF1m9rOno3vEIPJKaL7KLxCJqb1UBwRpl4p7EcFTgd3yG06TCPBjR35MbCZ5lDrdjPcqaflqbQLZQdX2rYd/8bhnqhIpC7SnRlIxGPsqvX"}
```

--- 

## 9. Verify Captcha Payload Observation File:pe.059.f123b6c8830e46be.js

### The request

```javascript
fetch("https://no8xfe.captcha-open-southeast.aliyuncs.com/", {
  "headers": {
    "accept": "*/*",
    "accept-language": "en-US,en;q=0.9",
    "content-type": "application/x-www-form-urlencoded; charset=UTF-8",
  },
  "referrerPolicy": "no-referrer",
  "body": "AccessKeyId=LTAI5tSEBwYMwVKAQGpxmvTd&SignatureMethod=HMAC-SHA1&SignatureVersion=1.0&Format=JSON&Timestamp=2026-06-19T06%3A42%3A09Z&Version=2023-03-05&Action=VerifyCaptchaV3&SceneId=didk33e0&CertifyId=KdeAGaejyN&CaptchaVerifyParam=%7B%22sceneId%22%3A%22didk33e0%22%2C%22certifyId%22%3A%22KdeAGaejyN%22%2C%22deviceToken%22%3A%22U0dfV0VCIzM3OTVkMjgyNDJhMTE2MTliYzI1Zjc4NmY4NGU1M2Q0LWgtMTc4MTg1MTQ4NDgxOC0xZDgwY2E3MWE3ZDQ0MzA5OTI4NDEzZjAyY2FhODA0MyM4RDJWN05odUhRWHhkQzhJbWxaa0tiMjNCaUM0WjYwUWtqTlNjOEZnbFhNOFpsb2dFbzZxMVZDcjdDazhVdEM3emlGbDROa3ViUHFnK0NvVkk0RXJmYkowSUwvUlg1RnhITDFYbHd1U3E3MzZWUDhrTnFZZENCdDFtelIrTHRPMGNkRDJBRitIYVZQZkZhTFJac0pMS1FVR1QwWjd6TFFMVVlacTdCeVNYTGhKZWMvd1NkV0FtdjZEUkNLYUpZc0ZvTlkzbGJnbFdHOFVEUDBWdFVjUGt1MjJmVTlobVJmQno4UnlaWmVIeTgzRkF3dXF4bjhsY1htUGZBQ25BSUlLR3pXS2UzWTlocEFOemxwNEoxQStaaTNROFJUeE5tam1vbmxiTjdkb3h3bmozbkpPcERmQlhNeFNNaUxmdHF6MWxtRlFZbzhEbFpNQzVqaEI3UnFFbDZlellHMWJWU3UxQVA4VWFYY1VFdFhSYU43V2pEQk5YaThDWnZVTmRBY1lRRjEyZWwwS0ZZS3A4VGNpVkI1YXBKQk4vQk9Pa1BMcmYzcTYwcHA5cElZSkpZa1pDekRZbGFpVHljMWhEdTYyM2Q4WjN3UWtjOG9scEdKZHY5OElnSnp1a1Riem1jYjlaWEtuWVVVK1FaWEpEbUlBYTVzTy9iaDRYOTlJalZIZmlvc1huN1QzR0dUNU51Ny9tckRyQ2VYR2YyOWJNdzdqeVZHKzVISFVhTHltSVNwTTRZdFQ3NVFpTWVmeGlJaG1XTWNReFBIQ05WWG5RUDFUdnVHTGxieHhUa1RRZ0R1Tldrc0hvalFWbkd1QlB0Sk5zMHpTekxvSzFhQ1dZaGhnbVlXOStNWXVWR1Zkekx0TnJJY2wyRlF2U1F2N1drdVRVOXFtUFlQNEJBbCtIMWh1QVdadFVxWllENzZVLzhUWi9XV0NTM1hsU05ycEFFTTVUa1FlVVdHN0hkaHUwYmNzUHJqUU44NTE4Ym11a1M5YUZxKy9YQnJxcjRzMjFXTWNXaGRXcVpnS3NJaGVneGh3YnlMYW12ajN3YVQ1a1d3eEZzT0RRMGVycEJUdUFlRjRYbVVhNmtXQmZSNWZaVXlmIzAjMDdiOTE3ODA5YTRkNWEzMjE5YjEyZjk4NTI1ZTNjMWY%3D%22%2C%22data%22%3A%22JRMlgg03DgRAeAILRQpEEHPjZZg9Ez1fbA0qd1yjNhsriXK5OHh3ZgkJc2NaO%2BpbP7wtcQGL6Gvi5LEZqOMpPHYyKHIfAmJWGUMiBRsQD0kkUUkiCA1Jd8BAFFEFUmL%2BQMu4MyFaXRQ8WWI%2BZ3Ib%2BByNmoNjgz5I0ECwZobWT5V6ii5ADARXc1BcB3ZnpVMtJn97EAZbYV5%2BLy9SEyhzMjNYnyF8fCUVJIx0UDQ2LMIfjHwgCMu0MPWbvABhaIf6ZjkXeldLQW4%3D%22%7D&SignatureNonce=0ecd84d6-b4ce-473c-b4d3-1cb4d0dcbca9&Signature=Oyh7ojzWm1sLLw%2BdNzrs8Ulfb88%3D",
  "method": "POST",
  "mode": "cors",
  "credentials": "omit"
});
```


### Successfull Response
```json
{"RequestId":"037B581E-C73A-4CFB-A23D-AB00F498CC0D","Message":"success","HttpStatusCode":200,"Code":"Success","Success":true,"Result":{"securityToken":"6oOo7e72nA61uVLiZVKiLYqF1m9rOno3vEIPJKaL7KLxCJqb1UBwRpl4p7EcFTgdXev7OeyuHCpA5PzF5PUSndjPcqaflqbQLZQdX2rYd/8bhnqhIpC7SnRlIxGPsqvX","VerifyCode":"T001","VerifyResult":true,"certifyId":"KdeAGaejyN"}}
```

### Payload Data
```json
{"data":"JRMlgg03DgRAeAILRQpEEHPjZZg9Ez1fbA0qd1yjNhsriXK5OHh3ZgkJc2NaO+pbP7wtcQGL6Gvi5LEZqOMpPHYyKHIfAmJWGUMiBRsQD0kkUUkiCA1Jd8BAFFEFUmL+QMu4MyFaXRQ8WWI+Z3Ib+ByNmoNjgz5I0ECwZobWT5V6ii5ADARXc1BcB3ZnpVMtJn97EAZbYV5+Ly9SEyhzMjNYnyF8fCUVJIx0UDQ2LMIfjHwgCMu0MPWbvABhaIf6ZjkXeldLQW4="}
```
The data is a base64 encoded, custom cipher encrypted, zlib compressed string with a custom hash prepended.
Example decoded structure:
```text
hash+json
'9333ef7396dd56dbb9d6e8f31e8f6014{"TrackList":{"mc":"","tc":"","mu":"","te":"","mp":"","tmv":"","ks":"","fi":"","startTime":1782100652835},"TrackStartTime":1782100652835,"VerifyTime":1782100652862,"arg":"JjObDGdh/ywcWQ=="}'
```
- `mc` = mouse clicks?
- `tc` = touch?
- `mu` = mouse up?
- `arg` = base64 encoded 10-byte value.

### Variables and definitions
```javascript
var ta = ({
            0: te
        })[0](40, 62)
          , to = window[ta + (~te ? te : 5)(29, 264)]
          , tc = n[(te && te)(30, 188)]
          , ts = to[te.apply(2, [97, 51])]
          , tu = to[te(8. .valueOf(), 15. .valueOf())][(-te ? 4 : te)(19, 273)]
          , tf = to[te(8, 15)][te(~te && 90, ~te && 86)]
          , tl = to[te((te(),
        8), (te(),
        15))][(te || te)(64, 38)]
          , th = to[({
            0: te
        })[0](62, 187)][te.apply(5, [53, 149])]
          , tp = to[te.apply(1, [37, 261])]
          , td = tf[(te(),
        te)(3, 204) + "y"](tl[te(63 * (1 | te), 50 * (1 | te))](tc))
          , tA = {};
        tA.iv = tu[[te][0](63, 50)](td),
        tA[te(-te ? 5 : 82, 234 * !-te)] = th;
        var tv = tA;

```

### Variable values
```
ta = '__ALIYUN'
to = window.__ALIYUN_CRYPT
tc = 'd35db7e39ebbf3d001083105'
ts = {encrypt: ƒ, decrypt: ƒ}
tu = {stringify: ƒ, parse: ƒ}
tf = {_map: 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=', _reverseMap: Array(123), stringify: ƒ, parse: ƒ}
th = {pad: ƒ, unpad: ƒ}
tp = ƒ (r,e){return new p.HMAC.init(t,e).finalize(r)}
td = '0123456789ABCDEF'
tA = {iv: t.init, padding: {…}}
// tA.iv.words = [808530483, 875902519, 943276354, 1128547654]
// tA.iv.sigBytes = 16

w = 'DNsKKPRHfOTuFph6taPHkC+TbC6YUaXi1NfQv95Ez2k='
x = 'd35db7e39ebbf3d001083105'
v = {ID: '7JLsB18MnA7GX3d6LxErT1sGT68xcVuOAoxz0b7vVzY=', SECRET: 'n9jH0yACW8YrgOBcM0v7u45+/bfozcSz8ZpvzGBXg3E='}
M = 'VerifyCap'
```

## 10. Zlib Compression & Data Transformation
Zlib compression is done by function `K`. `ty(t)` converts string to `Uint8Array`, `G.deflate` zlib compresses the data.
```javascript
// Example inputs and logs
console.log(t);
// '99603d2757539169328062db7568942c{"TrackList":{"mc":"","tc":"","mu":"","te":"","mp":"","tmv":"","ks":"","fi":"","startTime":1781878507084},"TrackStartTime":1781878507084,"VerifyTime":1781878507099,"arg":"VzN0TRFfHQ5OVw=="}'

console.log('Before Deflate', n);
// Uint8Array(221) [57, 57, 54, 48, 51, 100, 50, 55, ...]

G.deflate(n);
// Uint8Array(149) [120, 156, 117, 141, 203, 10, 194, 48, ...]

// Function tQ definition
function tQ(t, n) {
                    var e = new tW(n);
                    if (e.push(t, !0),
                    e.err)
                        throw e.msg || R[e.err];
                    return e.result
                }
// Function tU definition
var tU = function(t) {   if ("function" == typeof TextEncoder && TextEncoder.prototype.encode)
                        return (new TextEncoder).encode(t);
                    var n, e, r, i, a, o = t.length, c = 0;
                    for (i = 0; i < o; i++)
                        55296 == (64512 & (e = t.charCodeAt(i))) && i + 1 < o && 56320 == (64512 & (r = t.charCodeAt(i + 1))) && (e = 65536 + (e - 55296 << 10) + (r - 56320),
                        i++),
                        c += e < 128 ? 1 : e < 2048 ? 2 : e < 65536 ? 3 : 4;
                    for (n = new Uint8Array(c),
                    a = 0,
                    i = 0; a < c; i++)
                        55296 == (64512 & (e = t.charCodeAt(i))) && i + 1 < o && 56320 == (64512 & (r = t.charCodeAt(i + 1))) && (e = 65536 + (e - 55296 << 10) + (r - 56320),
                        i++),
                        e < 128 ? n[a++] = e : (e < 2048 ? n[a++] = 192 | e >>> 6 : (e < 65536 ? n[a++] = 224 | e >>> 12 : (n[a++] = 240 | e >>> 18,
                        n[a++] = 128 | e >>> 12 & 63),
                        n[a++] = 128 | e >>> 6 & 63),
                        n[a++] = 128 | 63 & e);
                    return n
                }


// Zlib related vars
var tP = Object.prototype.toString
, tL = F.Z_NO_FLUSH
, tR = F.Z_SYNC_FLUSH
, tF = F.Z_FULL_FLUSH
, tV = F.Z_FINISH
, tG = F.Z_OK
, tD = F.Z_STREAM_END
, tq = F.Z_DEFAULT_COMPRESSION
, tH = F.Z_DEFAULT_STRATEGY
, tZ = F.Z_DEFLATED;
```


### Transformation of `trackJson` via Function `nf`

Called via: `C = nx.A(nf, n_)` where `nx.A = function(t,n) { return t(n)}`. So `C = nf(n_)` (`n_` is `trackJson` without hash).
**Function nf definition**
```javascript

function nf(t) {
            for (e = 9; e; )
                switch (r = e >> 3,
                i = 7 & e,
                r) {
                case 0:
                    if (i < 6)
                        if (i < 2)
                            i >= 1 && (e = 10);
                        else if (i > 3)
                            if (i > 4) {
                                e -= -3;
                                try {
                                    return function(t) {
                                        var n, e, r, i;
                                        for (e = 3; e; )
                                            e < 1 || (e <= 1 ? (e += 3,
                                            r = function(t, n) {
                                                return i.B(te, n, t - -3)
                                            }
                                            ) : e < 4 ? e < 3 ? (e ^= 3,
                                            i.B = function(t, n, e) {
                                                return t(n, e)
                                            }
                                            ) : (i = {},
                                            e += -1) : (n = tr[(r && r)(52, 64)](this, 26)[i.B(r, Math.floor(237), 19)](this, arguments),
                                            e = 0));
                                        return n
                                    }(t)
                                } catch (t) {
                                    if (c.e(s, a))
                                        throw t
                                }
                            } else
                                !c * !c / 0 != 6 ? e ^= 1 : e += 3;
                        else
                            i > 2 ? (e = 14,
                            c.D = function(t, n) {
                                return t <= n
                            }
                            ) : (e += 14,
                            c.n = function(t, n) {
                                return t && n
                            }
                            );
                    else
                        i > 6 ? (e += -4,
                        c.R = function(t, n) {
                            return t !== n
                        }
                        ) : (a = c.y(c.W(c.c(arguments[o.call(1, 229, 51)], 1), 61), 58) > 58 && c.R(arguments[1], void 0) ? arguments[1] : 3,
                        e = 1);
                    break;
                case 1:
                    i > 5 ? i <= 6 ? (c.e = function(t, n) {
                        return t === n
                    }
                    ,
                    e -= 2) : (e -= 2,
                    c.W = function(t, n) {
                        return t * n
                    }
                    ) : i > 2 ? i < 5 ? i < 4 ? c.D(c.W(s - a, 50) + -6, -6) ? e ^= 15 : e = 0 : (o = function(t, n) {
                        return c.n(te, te)(n, t - 1)
                    }
                    ,
                    e ^= 10) : (e += -6,
                    c.c = function(t, n) {
                        return t - n
                    }
                    ) : i >= 1 ? i < 2 ? (e -= 7,
                    c = {}) : (s = 1,
                    e += 1) : (e = 11,
                    s++);
                    break;
                case 2:
                    e ^= 31,
                    c.y = function(t, n) {
                        return t + n
                    }
                }
            return n
        }
```


TrackJson gets transformed to final data with this code  n = tr[(r && r)(52, 64)](this, 26)[i.B(r, Math.floor(237), 19)](this, arguments)
```js
console.log(n);
JRMlgg0gGwVASQITewRNMEYjU1P7AMM7DGwWaiVYKOEKajqEHFh8RA1neYRfNAdR0XUQfSWw7h2PmQTbVpMbBWVFEHMdI1sENzToeHBDa0gdW0MRcxRcTwxEEp82HSfvd8K7Ri0JKSw7UW8jV31K21N3YXwcwT937U+vWYTVSox0hsdjInZNDW8WB3FzQ0Qkw0sBAAJLKT5CKRN3xwaLCX8bYxoLkQ9oKo5XTDQAZitmn0Qhdc/1Suikdj8OaWwhRjtEPlwSHmc=
```

### Function `nf` Behavior Tests
```js
console.log(t);
// {TrackList: {…}, TrackStartTime: 1782021569701, VerifyTime: 1782021569717, arg: 'ZjyUTmpv9h8dBw=='}

nf(t);
// 'JRMlgg0gGwVQeQITewRNMEbgUlclHnFEaXA2MWKm3fMsc1CfFaYZZjkfd3ASHyFF9bUHGneULV+hjAnpgZcDBJZsEmwZIGwMdCsQFi0xAFYVUUQYLwB4U9Y0MEgfFifSRy9VO85ZBx85VXMrSUtHPgKyQZJfzTF5DwWuBYXbfJwJmygJIBo0Y35rFRkTplQm0EhfERO9Kz98GhVWwX+bJjFlaxt4niZQxnEVEyYCehkVUVEwa8yXCjKUlyBtVk0KRx54Bg=='

nf('abcdef123456789');
// 'JRMkbRAlaBM6LgIPamJf6WPtb2UkBx5abH4bHmS2BThZbDKC7ah/Xt0bN595ORYu8k0JPXOv6X2Vr8cJg5E7BpBSMVY='
```
**Test:** `Function K` logs show that `nf` passes `hash+json` to `K`:
```text
[FUNCTION K LOG] ARGUMENTS: 'c6d9b3718aa286fe7290fbab8e7f66e2{"TrackList":{"mc":"","tc":"","mu":"","te":"","mp":"","tmv":"","ks":"","fi":"","startTime":1782022235313},"TrackStartTime":1782022235313,"VerifyTime":1782022235354,"arg":"fMR+GVZSAyc4Ng=="}'

[FUNCTION K LOG] ARGUMENTS: 'f4aaf499d633c77792ccf444f4dd0b33"abcdef123456789"'
```

### Relationship between `nf` and `tr`
```js
ttr(26, 'test');
// 'JRMkWWkGDigveSB2eABaM2cdclsffhBebh0gL0OrLcAuWVtA/UUFYQYbfIx0PgkT1VhpMBetN2vjjcQOioEnDQ=='

nf('test');
// 'JRMkWWkGDigveSB2eABaM2cdclsffhBebh0gL0OrLcAuWVtA/UUFYQYbfIx0PgkT1VhpMBetN2vjjcQOioEnDQ=='
```

**Deobfuscation of `nf` internals mapping to `tr`:**

```js
n = tr[(r && r)(52, 64)](this, 26)[i.B(r, Math.floor(237), 19)](this, arguments), e = 0


// i.B = function(t, n, e) { return t(n, e) }
// r = function(t, n) { return i.B(te, n, t - -3) }
// Simplifies to: r(t, n) = te(n, t + 3)

// r(52, 64) = te(64, 52 + 3) = te(64, 55) = 'bind'
// r(237, 19) = te(19, 237 + 3) = te(19, 240) = 'apply'

// n = tr['bind'](this, 26)['apply'](this, arguments)
// Which resolves to:
// n = tr(26, t)
```

### Function `tr` behaviour
Different calls inside `tr` handle decryption, XHR, device token generation, and signature generation.

- `tr(35)` returns `true`
- `tr(9, 'FqJB6iRNVYdEGpwb', '7JLsB18MnA7GX3d6LxErT1sGT68xcVuOAoxz0b7vVzY=')` returns `'LTAI5tSEBwYMwVKAQGpxmvTd'` (AccessKeyId)
- `tr(9, 'FqJB6iRNVYdEGpwb', 'n9jH0yACW8YrgOBcM0v7u45+/bfozcSz8ZpvzGBXg3E=')` returns `'YSKfst7GaVkXwZYvVihJsKF9r89koz'` (Secret)
- `tr(20, '6iL4denBvY')` returns the encoded `deviceToken` (`SG_WEB#...`)
- `tr(36, '6iL4denBvY')` returns the encoded `deviceToken` (`SG_WEB#...`)
- `tr(26, trackJson)` returns the encrypted payload `data` field.
- `tr(30, hash+json)` converts string to `Uint8Array`.
- `tr(22, '{sceneId...}'` handles sending `VerifyCaptchaV3`.
- `tr(13, params, 'YSKfst...')` generates Signature.

**DeviceToken Decoded Structure:**
```text
SG_WEB#3795d28242a11619bc25f786f84e53d4-h-1782093357620-0ce9c665567a47d790e73d4a71804304#kPsQc9v70vapMtIzaUSYfx9AgupQK7vEnP9Zhnh1oTl80JY5iaqeBHeXLHGhCYIfDu6ZH+AsPVN5HsDZPy1Cf9ecVq1oQz4kpRLIBIu5tamTxwFbtDncJ3OaDE7jt8mwgHCd+aNOCHejReSP4/Fh91TC5EmjB76Lk45S2o8sVfh+Lstug/MnwzkAIG5/DA9w9pe45guGDJWmMHpJMryrn4TjbLK/qLCkXdqiHti1mJZFlTwsw2EUr02mPqTb3stYU/H4D0H5UdzHRz/gMcsPfJqwXK2VeUOiMN4X7tW3sjqfZvlMpkWzZbVuLEdKC+ncvjl5YWCFsDnF6k4jXNH2OnPvf4lIHd+qOnEofmznJCmXgBRNsW+xtXOtZx9/KWyTV5ZXlw5veGvMulxGA44gVYkAjdLPt3xfKWNQXPPx+iT5gkm6eElxAExNUOjuZS9GUo74707Tgnd8cYFzPqALcT6TYuhdEKXE4izh18lJ4yP83IjBy1aXDw9XgEIK6NGc8qM25bUeIRiHf0uzKKHy6XFn9VrgIPjTPR4RaEEoWEH77GpuZdh14b/XED6ozTahyswieU6JOkdFOZDWtKTllz1c0f4ZXjqib37uQEJ96rYPo4Wc60eWL6EIU/T9+i/eorxHO1LXOTz2ahdc/vmjHWtOqkEZRWKSALXo6lnsKnZ8hfiqvv7ThIiW4ZoGg2U0rBykjj37JxAfsfUtPFnhCBDXUSjFaGyLV5JVbcrJtQWW2TPDPbkBkuxpj27ZSjWCAG0bI9KFWu1L3jHZUq/B6/afz6hEwY2/FVs7s004wylBtGaVmD/sjyR0QyI3D13mbT9wJNEKiDSKgE9u5lO+nrK0p9qUE9D+UJe29CWRuVI=#448#bbb4e2a921971f01679864ad9801099c
```

The middle part (`kPsQc9v...`) is AES encrypted with key `31646664393836656436633062643262` and IV words `[808530483, 875902519, 943276354, 1128547654]` (sigBytes: 16).

**Function `ti`**
```javascript
// Function ti string table deobfuscator 

function ti(t, n) {
            var e = ["A=ZCC6i1", "4CavcUIg8/d", "Lk=/cq", "4S2djA0TjTE", "LkEZOv8wLQW", "IvWamq3P", "+UbStUZp", "Q6Zj53fOs3W", "+UsX", "tE2+QP2UU2E", "J5V94Pbh", "Vv8yVQe8oSe", "oq6UH5O1aqh", "13esP=M6G16", "Q37+Q3M", "B/fZMq", "MU7X8UM", "5EBQGJa16E5", "CU7xA=DGB28", "9SnuLknK9n", "aS5gaHBSOHO", "s1G7", "B3bAQ=e", "MC8F", "4YBbU/0psNO", "spDjBE1J62h", "OQEhOQOY", "lv67", "qmWwVUE6ovM", "ApIn5=bqB=h", "6PDNsNZp8Cq", "HH6d", "VSqZH08", "sCaysToRtUn", "8kp/8UhdcgW", "B1fBAG1cC=h", "oveX9vE0", "ovMeHSMXoAi", "OSeW", "UP7XxTfDCPq", "GE1C5PaBB2W", "qIe6HW", "53sGBEf1UYi", "Q27aQ1i", "VH5v", "3nEQ", "CpZ9+61H5=e", "sqOo3q", "VS=Koq", "UY2xU2I+x2i", "4HoKjHDTjPq", "m1po53qomBW", "G3GMq=imGqq", "xN1yMq", "30BM61fIOnq", "xTxfsJ1ZjU8", "UTpKMTBE", "GN1T811T8J8", "lvENO0iZcW", "Vg82cgVk", "sTGwMCfd", "MUZT4CxWjTV", "om=iLvW7Vkh", "Og3eoW", "og5kVknuLW", "9S1Sj/sdjNh", "366C3=745Ed", "5Bo1Cp1c8Ed", "xY1k", "LQWX", "VvOkVW", "8Top5P7g8Th", "9q5eLA3Y95", "tP7ptW", "A2BXG27vABO", "OS3N9AVbIve", "xNDT+m0XxPd", "6Ba3UBb4", "sT1Y8/1YGNq", "JmefLNZK", "mWOtanECq5i", "QTsP5n", "Cpbc56a6U16", "qZ3P", "HgebakEGLq", "56xGG/25", "4J5hMTZ7xCd", "ok8plSqT", "Ovn0M5", "8Nsgsn", "cAiF", "BEZ1a5", "GE1t63MV328", "tE2+U229AW", "lgWh", "IGo0", "J6E8QG6Vmq5", "53BIGB0y53O", "U=s9C=Z1U3M", "6/7hB/1X4/O", "LnMFcm=v", "8CGf+YM", "VnnRlQV2Lq", "jkxvt3bf+Yn", "cmiNLkpy", "+J7KtP7uM/d", "3Z6H3Z5AH5d", "OmEucQq", "mkEyOQd0JgM", "xCfNjn", "LAMqoS3/Vm3", "A1a6C5", "tNok4C7m4Jq", "3W6m3n66", "IIiUUnn83p3", "tJfT+Pxe8Un", "Bpo1Q=M", "LgWD", "oUOZaJ3gaQq", "cgqy9mMw95", "P5hV1GnLm5O", "m5qLI05qIn5", "QE0V52GZQEV", "AEZ9Bp5W", "QBI6Cq", "+Aqn9C=k+/8", "mS=RamOr", "4NsSt/Zw+YV", "Q=MN", "MC0XjU7dtTW", "cQOWOHE", "Up18ATfUCBW", "Q1bV62Zm", "sNbs8T7Ra2=", "G=BsC33", "tPbvtTe", "spxlG/xM6BE", "MT7T8n", "+CZh5Y20t5", "OHo5", "oki/V5", "IG56JWhIP5", "sB7UGnV", "M3bCx=BAA68", "Q=IMBn", "Lkebon", "amndLmdvVgV", "Jq=5InhMHWi", "tNxN", "t/xZtmx2xU=", "8QpgsU874Yq", "OPVgamapMY6", "G6o1UP0156=", "C17LtB7VQpM", "tNpdAUoXsUV", "C1a45=oPPpE", "tYxhMJoRxW", "+YBx8YGk4Cd", "lvnEOQeb", "+TIiQT7Dj/V", "UEG8CW", "Omd2", "lgWfaQnrLq", "66ffApoqBN8", "632o5505CB5", "53ZaG6f3", "9g5V9v=vaQW", "x67lG3h", "Q5O61G6cmEE", "j=2t567G4=E", "Q=aVB5", "VA3XVA3XVAq", "tJWTcPV2+Je", "tJaKjTGb45", "U3xj4=2+C65", "aH6hIkq/Vgq", "jUZD", "cAnZPmEKHgE", "oHhvLn", "C67s5=2A46d", "Qpbt", "C2Da5BxV", "lAnwoHdimSV", "tY2XMPxu", "CCGfx/fR", "L/D0GYI/8U8", "s3sJ51sU+=q", "41Bt6E1B623", "sYoZsYM", "61sLBW", "s/IN8Jxh8UW", "avqwaHeHOvq", "1qn13qV91n=", "6EbHU60PAEO", "3n5A3nncmZd", "4Jbj4Ybr", "x3ZX+CsZ4q", "MPxZtY2=tY6", "L0eZl5", "avZh+C0=OAO", "6CZ7tUBu6T3", "5p1JCBoa5SM", "8BIM8BxGtCe", "IWVcmJVMIWd", "cviy3Qhk", "xUa2xUbft3h", "xCINM5", "HZsL52aAQS=", "+P0psW", "CYGy8NZ74Ph", "A104BB7vCW", "qGO4In", "LUedLmpXLNW", "8Jpw83p=+NM", "cmiXHgqEck3", "UB7sG38k", "5=B55EBv61E", "GCfh4N1Y", "G2ej51IIBp8", "1GMC35MPHZh", "xCod4UGQ4n", "C2sxB3x+", "xJGT4PV", "G=7CQ1DaqNi", "m5ia3q", "xPBg8Ypnsq", "jYoH+NaD8/d", "VkONakeha5", "ag6=1m6YcQO", "OH=plvdgcCM", "G6pj", "I55CHqV3a5d", "VQ6ilnVfOg6", "MYakjNi", "GB0MBI2tB3n", "6YoP6=M", "MNbSt5", "B1bACpB6", "IgqYVm=0", "CE7ct1DOQBM", "4=0GxHqYLC=", "qIW53I5UJn", "9HOYon", "OCoitC0RBCn", "8J0/t5", "B3aA62GV", "tPaKxCGwt5", "oW5ravMwjgq", "JQhi9kOhVZO", "+Ps08n", "t/1dlvWR", "GBoPU2s66/8", "QNaS+TpujY=", "4UO0+A6n+SO", "+603G/sDsI5", "OQ0f", "6posQ/GsBBe", "5p7LG32o", "5=G1", "Vg5SaH6", "L55ravMwjgi", "s6aB5YBmB6V", "jUx5MUV", "QUbu4Y2Y+2n", "xTZbxn", "+EpjAPa4", "GYx2lW", "lviv", "Aq0GCBsHmW=", "MCIYjY1rs5", "M=1UBE29An", "sU0/sN0SxJE", "xT0k8YBKsT5", "j=GsB6GLG5", "VmEk", "lkOR9n", "BpDI", "4k8hov8dxk5", "oHnZ", "Q6b4t101CB=", "sN0TGNi", "+BZN+Uq"];
            return (ti = function(n, r) {
                var i = e[n -= 0];
                if (i) {
                    if (void 0 === ti.Sc) {
                        ti.Sc = !0;
                        var a = "c1f9f8dd8381fae0e6dfdc89ffd1d3fc85f7f1e186f2e5f388c3c49bfdc884dade80e3db8dc0e99fd9d6c9e8d5f4fbe2e7eac6d7f582fee4d487c2c7d8d2c5f6ca".match(/.{1,2}/g).map(function(t) {
                            return parseInt(t, 16)
                        });
                        ti.lF = function(t, n) {
                            for (var e, r, i = "", o = "", c = 0, s = 0; r = t.charAt(s++); ~r && (e = c % 4 ? 64 * e + r : r,
                            c++ % 4) && (i += String.fromCharCode(255 & e >> (-2 * c & 6) ^ n)))
                                r = a.indexOf(176 ^ r.charCodeAt(0));
                            for (var u = 0, f = i.length; u < f; u++)
                                o += "%" + ("00" + i.charCodeAt(u).toString(16)).slice(-2);
                            return decodeURIComponent(o)
                        }
                        ,
                        t = {}
                    }
                    var o = t[n];
                    return o ? i = o : (i = ti.lF(i, r),
                    t[n] = i),
                    i
                }
            }
            )(t, n)
        }


```
**Note:**
When using function ti or wrapper function of ti first use a valid xor key or else you might get garbage
Single Args works too:
```js
td(10,78)
'4c63f913'
ti(3,78)
'4c63f913'
ti(78)
'gqfbqfG`'
ti(3)
'4c63f913'
```

## 11. VM-Based Hash Calculation Reverse Engineering
### ZLib Compression Call & Hash Setup
```js
nj = [nx.A(K, er), nx.F(O, (td || td)(10, 78))];
// nx.A = function(t,n) { return t(n)}
// nx.F = function(t, n) { return t + n }
// td = function(t, n) {return nx.j(ti, ti)(nx.w(t, 7), n)}
// nx.j = function(t, n) {return t || n}
// nx.w = function(t, n) {return t - n}
// td = function(t, n) {return ti(t - 7, n)}

// Resolves to:
nj = [K(er), O + td(10, 78)];

// Value of td(10,78)
console.log(td(10,78)) 
'4c63f913'

O = td(219, 14)
'3e627e1b'

// Resolves to:
 nj = [K(er),'3e627e1b4c63f913'];


console.log(nj)
["eJx1jcEKwjAQRP9lzzm4aWLSQg+9iIgg2CJe05KWUCKaREVK/91AchI8vX3MsEO5LnuumGC8ZIgo+bBhapS66OUWBVugc2qYj8YHqBawA1QABEKmfWbX2e/Z7Ssds08cTaIPyoXO2NhHIWlcLLhEKVaShto/OYGLdmb8/EYUKQHlpvj9cH60++u72TXN7TTVNaxfJOs7Cg==","3e627e1b4c63f913"]

// Bash
// ~ $ echo //eJx1jcEKwjAQRP9lzzm4aWLSQg+9iIgg2CJe05KWUCKaREVK/91AchI8vX3MsEO5LnuumGC8ZIgo+bBhapS66OUWBVugc2qYj8YHqBawA1QABEKmfWbX2e/Z7Ssds08cT// aIPyoXO2NhHIWlcLLhEKVaShto/OYGLdmb8/EYUKQHlpvj9cH60++u72TXN7TTVNaxfJOs7Cg== | base64 -d | xxd | head -1

// 00000000: 789c 758d c10a c230 1044 ff65 cf39 b869  x.u....0.D.e.9.i
// Valid zlib headers: Proof function nf calls function K 

// value of er
er = P(0, [], F, V, ez, [er, (td && td)(77, 19)]) + er // This is hash calculation call

// P is Interpreter function shown below
// Deobfuscation Time
// P(0, [], F, V, ez, [er, (td && td)(77, 19)]) + er
// td && td = td

// Resolution
// P(0, [], F, V, ez, [er,td(77, 19)]) + er
// td(77,19) = '0000'
// resolution
P(0, [], F, V, ez, [er,'0000']) + er

// ez = {"r": 1}
// td(77, 19) = '0000' (Salt string)
// F = bytecode in project file F_bytecode.txt
// V = Constant Pool
// console.log(V)
[    "o",    "r",    "a",    "m",    "C",    "e",    "f",    "i",    "p",    "h",    "d",    "",    0,    147,    "n",    "fromCharCode",    252,    241,    249,    246,    240,    231,    "map",    "join",    202,    172,    191,    164,    169,    190,    163,    165,    87,    56,    53,    61,    50,    52,    35,    156,    250,    233,    242,    255,    232,    245,    243,    108,    24,    3,    63,    30,    5,    2,    11,    206,    167,    160,    170,    171,    182,    129,    168,    124,    12,    14,    19,    8,    25,    33,    84,    79,    69,    68,    71,    72,    "_",    55,    64,    94,    89,    83,    88,    1,    22,    97,    127,    120,    114,    121,    46,    117,    65,    76,    75,    77,    90,    74,    115,    54,    95,    82,    110,    29,    36,    166,    152,    159,    149,    158,    134,    78,    66,    85,    119,    20,    26,    18,    44,    67,    70,    73,    104,    113,    162,    205,    192,    200,    199,    193,    214,    130,    234,    239,    238,    230,    215,    207,    204,    151,    211,    248,    244,    226,    227,    181,    146,    132,    148,    133,    91,    93,    86,    201,    253,    247,    178,    221,    150,    153,    136,    220,    184,    137,    145,    161,    157,    131,    179,    154,    142,    135,    28,    125,    106,    123,    155,    128,    32,    45,    37,    42,    59,    111,    57,    38,    40,    216,    185,    174,    177,    183,    141,    195,    236,    251,    228,    212,    210,    218,    173,    188,    138,    139,    143,    144,    175,    187,    180,    4,    7,    34,    194,    223,    229,    15,    49,    58,    96,    10,    101,    16,    41,    107,    48,    9,    31,    27,    21,    6,    109,    140,    224,    225,    118,    122,    196,    197,    203,    186,    189,    176,    13,    17,    43,    99,    23,    47,    103,    112,    237,    80,    105,    102,    98,    213,    126,    81,    100,    92,    116,    51,    62,    219,    209,    254,    208,    39,    222,    198,    217,    235,    false,    60,    "Boolean",    "Number",    "String",    "j",    "t",    "u",    "arguments",    "s",    "random",    256,    "floor",    "push",    "length",    "0",    "toString",    "y",    "charCodeAt"]

```
**Zlib Decompress test**

```python
import zlib                                                                                                                        import base64
b64_data = "eJx1jcEKwjAQRP9lzzm4aWLSQg+9iIgg2CJe05KWUCKaREVK/91AchI8vX3MsEO5LnuumGC8ZIgo+bBhapS66OUWBVugc2qYj8YHqBawA1QABEKmfWbX2e/Z7Ssds08cTaIPyoXO2NhHIWlcLLhEKVaSh\
to/OYGLdmb8/EYUKQHlpvj9cH60++u72TXN7TTVNaxfJOs7Cg=="
decompressed = zlib.decompress(base64.b64decode(b64_data))
print(decompressed.decode('utf-8'))

# Results 
# 25e9b5a47459411185c04af8e3b86174{"TrackList":{"mc":"","tc":"","mu":"","te":"","mp":"","tmv":"","ks":"","fi":"","startTime":1782111358187},"TrackStartTime":1782111358187,"VerifyTime":1782111358212,"arg":"JRqSHXwAFAAnOg=="}
```

### Virtual Machine Interpreter (Function `P`) 
```javascript
// Function P
var P = function t(n, e, r, i, a, o) {
            var c, s, u, f, l, h, p, d, A;
            function v(t) {
                var n, e;
                for (e = 0,
                n = []; (e - t) * 10 + -30 < -30; e++)
                    n.push(e);
                return n
            }
            function b(t, n) {
                return t && "__proto__" != n ? t.hasOwnProperty(n) ? t : b(Object.getPrototypeOf(t), n) : null
            }
            for ((c = Object.create(a.s || {}))._ = window,
            c["*"] = a.t || this,
            a.e && (c[i[r[n + 1]]] = a.e),
            c.arguments = o,
            s = 0,
            u = void 0; (n - r.length) * 93 + 61 < 61; )
                if (10 == (A = r[n++]) ? (f = e.pop(),
                h = ((l = e.pop()) - f) * 48 + -38 <= -38,
                (p = r[n++]) && e.push(h)) : 20 == A ? (f = e.pop(),
                h = ~(~(l = e.pop()) | ~f),
                (p = r[n++]) && e.push(h)) : 28 == A ? (f = e.pop(),
                l = r[n++],
                f || (n = l)) : 39 == A ? (f = r[n++],
                l = r[n++],
                h = r[n++],
                p = r[n++],
                n = f,
                function() {
                    try {
                        ((f = t(l, e, r, i, {
                            s: c,
                            t: c["*"]
                        })) - 0) * 21 + -79 > -79 && (s = 1,
                        u = a.r ? e.pop() : f)
                    } catch (n) {
                        ((f = t(h, e, r, i, {
                            s: c,
                            t: c["*"],
                            e: n
                        })) - 0) * 50 + -57 > -57 && (s = 1,
                        u = a.r ? e.pop() : f)
                    } finally {
                        ((f = t(p, e, r, i, {
                            s: c,
                            t: c["*"]
                        })) - 0) * 30 + -22 > -22 && (s = 1,
                        u = a.r ? e.pop() : f)
                    }
                }()) : 44 == A ? e.push({}) : 48 == A ? (f = e.pop(),
                h = (l = e.pop()) >> f,
                (p = r[n++]) && e.push(h)) : 37 == A ? (f = r[n++],
                (l = t(n, e, r, i, {
                    s: c,
                    t: c["*"]
                })) ? 1 === l ? s = 1 : (s = 1,
                u = a.r ? e.pop() : l) : n = f) : 55 == A ? (f = r[n++],
                l = [],
                v(f).forEach(function() {
                    l.unshift(e.pop())
                }),
                r[n++] && e.push(l)) : 52 == A ? (l = !(f = e.pop()),
                r[n++] && e.push(l)) : 9 == A ? (f = e.pop(),
                h = ((l = e.pop()) - f) * 20 + 93 < 93,
                (p = r[n++]) && e.push(h)) : 50 == A ? (f = r[n++],
                l = e.pop(),
                h = e.pop(),
                o = [],
                v(f).forEach(function() {
                    o.unshift(e.pop())
                }),
                p = null === h ? l.apply(c, o) : h[l].apply(h, o),
                r[n++] && e.push(p)) : 19 == A ? function() {
                    throw e.pop()
                }() : 27 == A ? (f = e.pop(),
                l = e.pop(),
                h = e.pop(),
                (p = l === c && b(l, f) || l)[f] = h,
                (d = r[n++]) && e.push(h)) : 7 == A ? (f = e.pop(),
                l = r[n++],
                h = r[n++],
                p = function(n) {
                    return function() {
                        return t(n, e, r, i, {
                            s: c,
                            t: this,
                            r: 1
                        }, arguments)
                    }
                }(l),
                h ? e.push(p) : c[f] = p) : 0 == A ? (f = e.pop(),
                h = (l = e.pop()) >>> f,
                (p = r[n++]) && e.push(h)) : 47 == A ? (f = e.pop(),
                l = e.pop(),
                (h = e.pop())[l] = f,
                r[n++] && e.push(h)) : 40 == A ? (f = e.pop(),
                h = -95 * (l = e.pop()) / (-95 * f),
                (p = r[n++]) && e.push(h)) : 35 == A ? (l = typeof (f = e.pop()),
                r[n++] && e.push(l)) : 32 == A ? (f = e.pop(),
                l = r[n++],
                f && (n = l)) : 38 == A ? (f = e.pop(),
                h = ~(~(l = e.pop()) & ~f),
                (p = r[n++]) && e.push(h)) : 30 == A ? e.push(c) : 21 == A ? c[l = i[f = r[n++]]] = void 0 : 5 == A ? (l = ~(f = e.pop()),
                r[n++] && e.push(l)) : 3 == A ? (f = r[n++],
                l = e.pop(),
                p = b(h = e.pop(), l) || h,
                d = f ? --p[l] : p[l]--,
                r[n++] && e.push(d)) : 56 == A ? (f = e.pop(),
                l = e.pop(),
                r[n++] && e.push(l[f])) : 34 == A ? e.push(window) : 26 == A ? (f = e.pop(),
                h = ((l = e.pop()) + 12) * f - 12 * f,
                (p = r[n++]) && e.push(h)) : 23 == A ? (f = e.pop(),
                h = (l = e.pop()) & ~f | ~l & f,
                (p = r[n++]) && e.push(h)) : 45 == A ? (f = e.pop(),
                l = e.pop(),
                h = delete l[f],
                r[n++] && e.push(h)) : 49 == A ? (f = r[n++],
                o || (o = [].concat(e)),
                v(f + 1).forEach(function(t) {
                    c[e.pop()] = o[-((-51 * f - -51 * t) / 51)]
                })) : 46 == A ? (f = e.pop(),
                h = (l = e.pop())instanceof f,
                (p = r[n++]) && e.push(h)) : 8 == A ? (f = r[n++],
                l = e.pop(),
                p = b(h = e.pop(), l) || h,
                d = f ? ++p[l] : p[l]++,
                r[n++] && e.push(d)) : 36 == A ? (f = e.pop(),
                h = (l = e.pop()) !== f,
                (p = r[n++]) && e.push(h)) : 33 == A ? (f = i[r[n++]],
                h = (l = c)[f],
                r[n++] && e.push(h)) : 2 == A ? (f = e.pop(),
                h = (l = e.pop()) << f,
                (p = r[n++]) && e.push(h)) : 4 == A || (41 == A ? (f = e.pop(),
                l = e.pop(),
                h = r[n++],
                o = [],
                v(h).forEach(function() {
                    o.unshift(e.pop())
                }),
                p = new (v(h).reduce(function(t, n) {
                    return t.bind(l, o[n])
                }, l[f])),
                r[n++] && e.push(p)) : 11 == A ? s = 1 : 14 == A ? (f = e.pop(),
                l = e.pop(),
                h = r[n++],
                -1 == l && (l = h),
                n = l = f[l]) : 15 == A ? (f = e.pop(),
                h = (l = e.pop()) === f,
                (p = r[n++]) && e.push(h)) : 12 == A ? (f = e.pop(),
                h = ((l = e.pop()) - f) * 92 + 67 >= 67,
                (p = r[n++]) && e.push(h)) : 54 == A ? (f = e.pop(),
                l = void 0,
                r[n++] && e.push(l)) : 53 == A ? (f = r[n++],
                l = e.pop(),
                v(f).forEach(function() {
                    e.pop()
                }),
                r[n++] && e.push(l)) : 24 == A ? (f = e.pop(),
                h = ((l = e.pop()) - f) * 36 + 69 > 69,
                (p = r[n++]) && e.push(h)) : 42 == A ? (f = e.pop(),
                h = (l = e.pop())in f,
                (p = r[n++]) && e.push(h)) : 18 == A ? (f = e.pop(),
                h = (l = e.pop()) + f,
                (p = r[n++]) && e.push(h)) : 16 == A ? (f = r[n++],
                l = "",
                v(f).forEach(function() {
                    l += e.pop()
                }),
                r[n++] && e.push(l)) : 22 == A ? (f = e.pop(),
                h = -((-70 * (l = e.pop()) - -70 * f) / 70),
                (p = r[n++]) && e.push(h)) : 25 == A ? (s = 1,
                u = a.r ? e.pop() : r[n++]) : 51 == A ? (f = e.pop(),
                h = (l = e.pop()) != f,
                (p = r[n++]) && e.push(h)) : 31 == A ? n = f = r[n++] : 29 == A ? (f = e.pop(),
                h = (l = e.pop()) == f,
                (p = r[n++]) && e.push(h)) : 17 == A ? (f = e.pop(),
                h = -(-(l = e.pop()) % f),
                (p = r[n++]) && e.push(h)) : 6 == A ? (f = r[n++],
                void 0 === (l = t(n, e, r, i, {
                    s: c,
                    t: c["*"]
                })) ? n = f : (s = 1,
                u = a.r ? e.pop() : l)) : 43 == A ? (f = r[n++],
                e.push(i[f])) : 13 == A ? e.push(e[-((-41 * e.length - -41) / 41)]) : 1 == A ? e.pop() : function() {
                    throw Error()
                }()),
                s)
                    return u
        }

```

### How to reverse engineered hash calculation

1. Disassebmle Virtual Machine: `disasm.py`
2. Finding xor encoded function/strings using `analyze.py`
3. Decode all the XOR-encoded strings that get assigned to variable names using `decode_strings.py`
4. Extract all encoded strings by finding BUILD_ARRAY + map(fn) + join patterns using `full_decode.py`
5. Extract all the encoded byte arrays and decode them to strings using `extract_strings.py`
6. Analyse critical parts disassembly that does hashing using bash commands like `grep -n "" disasm.txt | tail -300 | head -200`, `tail -100 disasm.txt`.`grep -n "" disasm.txt | grep -E "^(7[6-9][0-9]{2}|8[0-2][0-9]{2}):" | head -100`. `grep -n "" disasm.txt | grep -E "^(7[4-6][0-9]{2}):" | head -100` etc.
7. Decode the strings around pc 15396 and 15464 that appear to be key strings in the algorithm using: `key_strings_decode.py`
8. Look at the section around PC 15760-15870 to understand the main hash structure using `grep -n "" disasm.txt | sed -n '7550,7650p'`
9. Decoding the remaining key strings again for precision - especially at 15728 and find what 'h' and 'd' and 'u' are using: `key_strings_decode2.py`
10. Tracing back what h string should decode to - re-examine the XOR function used for h using: `decode_h.py`
11. Recalculating h string with correct XOR key 210 using: `h_calculator.py`
12. Tracing the actual hash algorithm - look at what happens with 'e', 'f', 'C' which are the core of the hash using: `hash_alg_finder.py`
13. Tracing the actual VM hash algorithm - let's look at what 'y' function does at PC 15982 using: `y_function_trace.py`
14. Now we need full disassembly of the y() function from PC 15989 to 16838 we can get that using: `disasm_y_func.py`
15. By analysing the disassembly we can reconstruct it into javascript by converting it into psuedocode then in javascript eg:
```js
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
// WHY DOES parseInt MATTER?
// ─────────────────────────────────────────────────────────────
// The call site is:  (td && td)(77, 19)
// When td is a STRING like '0000', this is  '0000'(77, 19) which
// would throw in strict contexts, but in this VM the outer wrapper
// evaluates td(77,19) differently. From the user's console tests:
//
//   t(0,[], F, V, ez, [input, '0000'])          -> '9333ef...'
//   t(0,[], F, V, ez, [input, '0000000000001']) -> '4ed342...' (salt≠0)
//   t(0,[], F, V, ez, [input, '0010'])          -> '70eb2c...' (salt≠0)
//
// The salt string is passed as-is to the y() function (argument[1]).
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
console.log('Match:  ' + (traced === expected));
JSEOF
node /tmp/aliyun_hash_final.js
Output

=== Verification ===
Expected : 9333ef7396dd56dbb9d6e8f31e8f6014
salt="0000"               -> 9333ef7396dd56dbb9d6e8f31e8f6014  PASS=true
salt="000"                -> 9333ef7396dd56dbb9d6e8f31e8f6014  PASS=true
salt="0"                  -> 9333ef7396dd56dbb9d6e8f31e8f6014  PASS=true
salt="0010"               -> 70eb2c0d763b5e3b78be269cc9a0fd28  PASS=false (expected different)
salt="0000000000001"      -> 4ed342e636bf55896c0244fa419b6f1f  PASS=false (expected different)
salt="1"                  -> 1002e563d3c7d303b20a36d2044d70ec  PASS=false (expected different)

=== Intermediate state trace (salt="0000") ===
Phase 1 - initial e: [0,17,34,51,68,85,102,119,136,153,170,187,204,221,238,255]
  f=16, a (UTF-8 len)=189, m (salt len)=4
Phase 2 - after KSA:  [204,136,238,51,34,170,17,68,85,187,102,119,0,153,255,221]
Phase 3 - after PRGA: [124,71,124,64,121,174,192,6,239,13,81,37,246,124,126,155]
Phase 4 - after diff: [147,51,239,115,150,221,86,219,185,214,232,243,30,143,96,20]
Final:  9333ef7396dd56dbb9d6e8f31e8f6014
Match:  true
```

### Hash function reverse engineering completed more details at [VMachine_HASH](./VM_hash_gen_report.md)

## 12. TrackJson `arg` Value Generation

The `arg` property inside `TrackJson` is assigned via heavily obfuscated code:
```js
// Original snippet:
n_[nx.c(s, 99 * !-s, 88 * !-s)] = I,

// nx.c = function(t, n, e) { return t(n, e) }
// s = function(t, n) { return (td || td)(w.U(t, 2), n)}
// w.U = function(t, n) {return t - n}
// s = function(t, n) {return td(t - 2, n);}

// -s evaluates to NaN (unary minus on function)
// !NaN is true
// 99 * true = 99
// 88 * true = 88

// Resolves to:
n_[s(99,88)] = I;
// s(99,88) resolves to 'arg'
// Thus: n_['arg'] = I
```

```js
Subject:
n_[nx.c(s, 99 * !-s, 88 * !-s)] = I,
```
**OPTIONAL EXPLANATION: WHY -s returns NaN**
```optional
// Explanation: Why -s returns NaN
// The unary minus operator (-) tries to convert its operand to a number
// When you apply - to a function, JavaScript:
// 1. First calls `ToNumber()` on the function
// 2. `ToNumber()` calls `ToPrimitive()` with hint "number"
// 3. For functions, `ToPrimitive()` calls `valueOf()` then `toString()`
//  4. Functions convert to strings (their source code)
// 5. The string cannot be parsed as a number
// 6. Result: `NaN`

// Example

A = '10'
Number(A) // parsed string to number succesfully
10
-A = -5

B = '10A'
Number(B) // Cant parse because it contains A which is not a number
NaN
-B
NaN


// Final thing
In JavaScript, NaN is falsy (coerces to false in boolean contexts), but it is not equal to false:

!NaN = true
!false = true
false === Nan result is  false
```

### How the variable I is defined?
Variable `I` is generated by the virtual machine `P`:

```js
P[(s && s)(242, 19)](void 0, v)) : a < 6 ? (P[(s && s)(242, 19)](void 0, v)

P[(s && s)(242, 19)](void 0, v)
// s(242, 19) resolves to 'apply'
// Resolves to:
P.apply(void 0, v)
// resolves to
P['apply'](unndefined,v)
```

`v` is an arguments object containing keys `["0", "1", "2", "3", "4", "5"]`:

- `0`: `0`
- `1`: `[]`
- `2`: Bytecode array (shown in `L_bytecode.txt`)
- `3`: Constant pool `R` (see below)
- `4`: JSON `{r:1}`
- `5`: Array `m` containing `['dynamic-string', 'constant_string']`
  - `m[0]` = CertifyID from init captcha response
  - `m[1]` = Constant string (e.g., `'4xrihv8zb8tf1mfj'`)
m[y] = m[1] = constant_string in this case 4xrihv8zb8tf1mfj
m[0] = dynamic_string AKA CertifyID from init captcha response 
```js
P.apply(undefined,v) resolves to just P(args from var v)
```

### Constant String Variations
There are 10 possible values for m[1]. Example cases:
```js
// Case 0:
m[y] = B + (~s ? s : 0)(277, 56)
// B = fxt8jzp3, s(277,56) = p0ienz71
// Resolves: m[y] = 'fxt8jzp3p0ienz71'

// Case 1:
m[y] = M + nx.c(s, nx.j(-s, 59), nx.j(-s, 26))
// M = 4xrihv8z, s(59,26) = b8tf1mfj
// Resolves: m[y] = '4xrihv8zb8tf1mfj'
// This is  my case

// Case 3:
m[y] = k + (~s ? s : 8)(22, 44)
// k = nx.c(s, 10, 11))
// s(10,11) = rxy2kxiu
// s(22,44) = '8g52k8hy'
// Resolves: m[y] = 'rxy2kxiu8g52k8hy'

// Case 4:
```

**R constant Pool**
```
["o", "n", "r", "t", "e", "_", "Boolean", "a", "Number", "h", "String", "m", "", 0, 59, "fromCharCode", 90, 79, 84, 89, "map", "join", "C", 118, 20, 2, 25, 23, "l", 239, 138, 129, 140, 128, 139, 186, 189, 166, 172, 130, 159, 155, "c", 252, 152, 153, 147, 169, 174, 181, 191, 145, 146, 136, "f", 109, 8, 30, 14, 12, 29, "s", 220, 178, 185, 175, "arguments", 1, 32, 50, 10, 51, 6, 44, 37, 16, 46, 11, 62, 19, 43, 60, 33, 53, 34, 7, 26, 48, 5, 4, 61, 13, 47, 49, 18, 27, 22, 17, 39, 56, 41, 38, 55, 31, 15, 58, 52, 40, 57, 45, 35, 36, 42, 54, 63, 3, 24, 28, 9, 21, "length", "charCodeAt", 255, null, "call"]
          
```
