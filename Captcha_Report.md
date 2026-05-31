# Aliyun CaptchaJS Imformation Report 

# Part 1: Obfuscation & Deobfuscation of literals 

Starting with,right below 
```javascript
var G = rr;
// I added 
window.__G = G = rr;
// So i can use it from browser console
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

### Jt  


```javascript
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


#### Deobfuscation examples
```AliyunCaptcha.js
var G = rr;
window.__G = G;
```
```cosnole
var G = window.__G;
```
```console
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

## Part 2: Functions , Definitions & Variables

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
```console
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


window.__ae
{
    "_map": "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=",
    "_reverseMap": [
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        null,
        62,
        null,
        null,
        null,
        63,
        52,
        53,
        54,
        55,
        56,
        57,
        58,
        59,
        60,
        61,
        null,
        null,
        null,
        64,
        null,
        null,
        null,
        0,
        1,
        2,
        3,
        4,
        5,
        6,
        7,
        8,
        9,
        10,
        11,
        12,
        13,
        14,
        15,
        16,
        17,
        18,
        19,
        20,
        21,
        22,
        23,
        24,
        25,
        null,
        null,
        null,
        null,
        null,
        null,
        26,
        27,
        28,
        29,
        30,
        31,
        32,
        33,
        34,
        35,
        36,
        37,
        38,
        39,
        40,
        41,
        42,
        43,
        44,
        45,
        46,
        47,
        48,
        49,
        50,
        51
    ]
}


```

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
```javascript

        var Ee = {
            ACTION: gt,
            ACTION_STATE: xt,
            KEY_ID: me(pt, ht.ID),
            KEY_SECRET: me(pt, ht[re(371)])
        }
```
```console
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

```
```console

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

### Part 3: Function Re internals

```javascript
var nr = new Jt({});
// nr is related to Jt
```

In context of function Re i added 
```javascript
window.__value_of_r = r;
```

so the full function becomes
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
      
Upon calling this function i get value of o = e.WEB_AES_FLAG_SECRET_KEY which means me(r.ACCESS_SEC, r.secretKey) is failing when i ran `console.log(window.__value_of_r_secretKey)` i got undefined and when i ran `console.log(window.__value_of_r.ACCESS_SEC)`
it worked  and logged 'FqJB6iRNVYdEGpwb'  which means `window.__value_of_r_secretKey` is failing when i logged  r.ACCESS_KEY i got 
```json
{
    "ID": "tBwmiXXwEdGaRtkcLvIoA/F6r4qktDu7lFi23FmDCbo=",
    "SECRET": "9U6Kg6ljgm9PhHvSsihjy1wJVo71uzWkRDfGiyiMVBg="
} 
```
This is just for information lets just assume  using e.WEB_AES_FLAG_SECRET_KEY since it works
For information here is the  full log of r: 

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
Some other important values related to r:
```javascript
r._extend
// Definition
_extend: function(t) {
                var r = G
                  , e = this;
                new q(t)[r(505)](function(t, r) {
                    e[t] = r
                })
            }
```
	    

I have already reverse engineered ye(o,c) from function Re and decryption script is ready and it works
I didnt write full report how ye works;
If you want can you write a reverse engineered human readable variant of ye and a report along with it 

#### Decryption Script

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

Script is working

---

## Part 4: Information of Rt in context of Rt.appkey
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


// Simplified version of what the code does in real:

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


### Calculation  of literals using Deobfuscation function rr:

#### Step 1:  The jt() array contains the raw string fragments
```javascript
function jt() {
    var t = ["Ac1KzxzPy2u", "DxrOzwfZDc0", ... "ab034ec0", "643f9139", ...];
    return (jt = function() { return t })();
}
```
#### Step 2: The decoder function rr (assigned to G)

Min Index is 312 of function rr
Max Index is 595 of function rr

```javascript
var G = rr;  // At the top of the file

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

#### Step 3: The Ft (which becomes Rt) object definition

```javascript
// This appears in the obfuscated code (around line ~9700+ in your file)
var Ft = {};
Ft[G(548)] = Pt,        // G(548) = "appName"
Ft[G(519)] = Nt,        // G(519) = "appKey"  
Ft[G(430) + "s"] = Ht,  // G(430) = "endpoint" → "endpoints"
Ft[G(421) + G(411)] = Wt // G(421) = "shplEndp", G(411) = "oints" → "shplEndpoints"
```

#### Step 4: The Nt object (which is appKey) definition
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

## Part 5: Function rr index deobfuscated literals json log

```json
{
  "312": "1662178hogNeO",
  "313": "theast-d",
  "314": "1NfQv95E",
  "315": "SG_WEB_P",
  "316": "dZJMm",
  "317": "re-b.ali",
  "318": "HaTtw",
  "319": "0.0.0/fe",
  "320": "theast.a",
  "321": "9NhnQQ+L",
  "322": "2dc7fae1",
  "323": "ual.aliy",
  "324": "z2k=",
  "325": "PiVwNmNO",
  "326": "a.device",
  "327": "ptchaV3",
  "328": "-fronten",
  "329": "SG_WEB",
  "330": "INIT",
  "331": "hai.devi",
  "332": "theast-b",
  "333": "Yex3WaHd",
  "334": "AIN_FAIL",
  "335": "W8YrgOBc",
  "336": "split",
  "337": "3795d282",
  "338": "liyuncs.",
  "339": "DNsKKPRH",
  "340": "S+qKUbJi",
  "341": "2B0=",
  "342": "/captcha",
  "343": "ap-south",
  "344": "aptcha-o",
  "345": "ab034ec0",
  "346": "pbG/bc9x",
  "347": ".saf.ali",
  "348": "FLAG",
  "349": "Log3",
  "350": "2VS3zA==",
  "351": "7JLsB18M",
  "352": "success",
  "353": "theast-p",
  "354": "cha",
  "355": "ONUTG",
  "356": "fail",
  "357": "l.aliyun",
  "358": "aliyuncs",
  "359": "EdGaRtkc",
  "360": "gp.aliyu",
  "361": "web-pre.",
  "362": "heast-1.",
  "363": "b405803a",
  "364": "evice.sa",
  "365": "3xLOkphC",
  "366": ".com",
  "367": "2415840gMPfPM",
  "368": "hanghai.",
  "369": "captcha-",
  "370": "INITV3",
  "371": "VERIFY",
  "372": "UploadLo",
  "373": "com",
  "374": "8KmHIQsc",
  "375": "M0v7u45+",
  "376": "re.aliyu",
  "377": "pen.aliy",
  "378": "open-b.a",
  "379": "DEVICE_U",
  "380": "af.aliyu",
  "381": "-1.devic",
  "382": "COMBAT_U",
  "383": "east-1.d",
  "384": "outheast",
  "385": "web-b.al",
  "386": "ck.ap-so",
  "387": "ncs.com/",
  "388": "d/FeiLin",
  "389": "sg50c495",
  "390": "open-pre",
  "391": "iyuncs.c",
  "392": "ptchaV2",
  "393": "pre-ap-s",
  "394": "Log1",
  "395": "open.ali",
  "396": "VYdEGpwb",
  "397": "-b.aliyu",
  "398": "W.10001.",
  "399": "ss0=",
  "400": "F0tJ3ds4",
  "401": "ncs.com",
  "402": "95bc895c",
  "403": "vkI7QQBX",
  "404": "d/aliyun",
  "405": ".aliyunc",
  "406": "ed381ac9",
  "407": "RES",
  "408": "Vo71uzWk",
  "409": "X1y5Vstb",
  "410": "5+LZJA7u",
  "411": "oints",
  "412": "NIT_FAIL",
  "413": "FAIL",
  "414": "DEVICE_M",
  "415": "SzaFtFlN",
  "416": "W2022020",
  "417": "cloudaut",
  "418": "c0ad7983",
  "419": "chaV2",
  "420": "tBwmiXXw",
  "421": "shplEndp",
  "422": "Cbo=",
  "423": "3.0",
  "424": "9eb33e06",
  "425": "REFRESH_",
  "426": "wZaUWGBt",
  "427": "9ebbf3d0",
  "428": "east-1-g",
  "429": "r4qktDu7",
  "430": "endpoint",
  "431": "5169504KEaUDm",
  "432": "open-sou",
  "433": "/1scBbcb",
  "434": "sg63c0a0",
  "435": "/main.cs",
  "436": "dev.o.al",
  "437": "C/c3QKzT",
  "438": "aptcha-p",
  "439": "VzY=",
  "440": "sh3c47a8",
  "441": "WEB",
  "442": "yuncs.co",
  "443": "com/",
  "444": ".js",
  "445": "853792CgZYEZ",
  "446": "ual-b.al",
  "447": "9U6Kg6lj",
  "448": "chaV3",
  "449": "Gas=",
  "450": "SUCCESS",
  "451": "aptcha-s",
  "452": "ddhs0305",
  "453": "INITV2",
  "454": "807878SVsivw",
  "455": "f84e53d4",
  "456": "PLOAD",
  "457": "xLLw/t15",
  "458": "anghai.a",
  "459": "NiaNh",
  "460": "518MUXIlt",
  "461": "e.saf.al",
  "462": "static-c",
  "463": "83f5e541",
  "464": "k+1RW0cz",
  "465": "75184fAvUYY",
  "466": "InitCapt",
  "467": "open-ga-",
  "468": "NLAoqT6K",
  "469": "LTXPs",
  "470": "aptcha.a",
  "471": "/bfozcSz",
  "472": "fOTuFph6",
  "473": "h-device",
  "474": "utheast-",
  "475": "device.c",
  "476": "SECRET",
  "477": "southeas",
  "478": "LOG",
  "479": "XiMPo",
  "480": "KFr07pEb",
  "481": "12hsb03c",
  "482": "l-b.aliy",
  "483": "open-dua",
  "484": "saf-aliy",
  "485": "www.aliy",
  "486": "WEB_PREI",
  "487": "2.0",
  "488": "1910DXe6",
  "489": "t-1.aliy",
  "490": "34gsf3f3",
  "491": "mosql",
  "492": "n.js",
  "493": "un.com/",
  "494": "bBLLx",
  "495": "cJS/",
  "496": "now",
  "497": "_l_KPLiP",
  "498": "c8a0bc51",
  "499": "ilin",
  "500": "PREID",
  "501": "dev.g.al",
  "502": "-pre.ali",
  "503": "FqJB6iRN",
  "504": "o.alicdn",
  "505": "_each",
  "506": "S_FAIL",
  "507": "GbSWQ",
  "508": "un-com",
  "509": "VBg=",
  "510": "REQ",
  "511": "f.aliyun",
  "512": "saf-capt",
  "513": "REID",
  "514": "202130TawWqM",
  "515": "taPHkC+T",
  "516": "3iDi2RAb",
  "517": "pre-cn-s",
  "518": "cn_dual",
  "519": "appKey",
  "520": "sgp",
  "521": "grM=",
  "522": "BdGKm",
  "523": "7ef9e8a2",
  "524": "cha-waf",
  "525": "https://",
  "526": "643f9139",
  "527": "g3E=",
  "528": "8fgs168b",
  "529": "-dualsta",
  "530": "nA7GX3d6",
  "531": "sgp_dual",
  "532": "1.0",
  "533": "B5zqghyJ",
  "534": "d35db7e3",
  "535": "g.alicdn",
  "536": "Plkdn",
  "537": "RrKCkAux",
  "538": "2023-03-",
  "539": "LvIoA/F6",
  "540": "pro-open",
  "541": "OhpiO",
  "542": "n9jH0yAC",
  "543": "sihjy1wJ",
  "544": "web.aliy",
  "545": "bC6YUaXi",
  "546": "kjgCtS6e",
  "547": "prototyp",
  "548": "appName",
  "549": "03oLbQXW",
  "550": "lFi23FmD",
  "551": "-pre-b.a",
  "552": "1.aliyun",
  "553": "+fR9tYzl",
  "554": "-pre.ap-",
  "555": ".ap-sout",
  "556": "uncs.com",
  "557": "LIMIT_FL",
  "558": "DEVICE_I",
  "559": "VERIFYV3",
  "560": "T68xcVuO",
  "561": "MfA=",
  "562": "INIT_FAI",
  "563": "OTHER",
  "564": "UPLOAD",
  "565": "307dbe32",
  "566": "PIC_FAIL",
  "567": "_c_WBKFR",
  "568": "cs.com",
  "569": "sh87bd15",
  "570": "FP/fp.mi",
  "571": "9fKx+ymx",
  "572": "cn-shang",
  "573": "Aoxz0b7v",
  "574": "d/dynami",
  "575": "gp-pre.a",
  "576": "device.s",
  "577": "umd7+KnO",
  "578": "s.com",
  "579": "http://",
  "580": "RDfGiyiM",
  "581": "icdn.com",
  "582": "VerifyCa",
  "583": "8ZpvzGBX",
  "584": "9g8a0A==",
  "585": "LxErT1sG",
  "586": "01083105",
  "587": "ck.cn-sh",
  "588": "bc25f786",
  "589": "42a11619",
  "590": "2020-10-",
  "591": "Log2",
  "592": "ce.saf.a",
  "593": "b/ksJCrC",
  "594": "gm9PhHvS",
  "595": "DYNAMICJ"
}
```
---

## Part 6: Function Re reverse engineered DeviceData creation 

Function Re generates DeviceData which is sent in function Pe

#### Function Pe Definition
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

#### Function Re Definition
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
Notes 
- 1. `var n` in context of `var n = t.appKey || r.APP_KEY can have multiple values since it likely gets it values from Rt.appKey which have different values based on region and version. This is same for `var i` im not sure but it seems obviuous 
- 2. in context of `var o` the key used is e.WEB_AES_FLAG_SECRET_KEY since r.secretKey is undefined and we should stick to it since it works 



### Function Re reconstruction in python

`generate_device_data.py`

```python

"""
AliyunCaptcha - DeviceData Generator & Inspector
Replicates: Re() -> ye() -> be() -> ye() chain from AliyunCaptcha.js

Static values extracted from AliyunCaptcha.js via browser deobfuscation.
Dynamic values (sceneId, prefix, region) are per-captcha-instance.

Public API:
  generate_device_data(scene_id, prefix, region)  → full DeviceData (ye + be + ye)
  generate_only_ye(scene_id, prefix, region)       → first ye() pass only, no be()
  decrypt_ye(ciphertext_b64, key_str)              → decrypt one ye() layer
  decrypt_full(device_data)                        → decrypt both layers, return dict
"""

from Crypto.Cipher import AES
from Crypto.Util.Padding import pad, unpad
import base64


# ─────────────────────────────────────────────
#  STATIC CONSTANTS (never change, baked into JS)
# ─────────────────────────────────────────────

APP_KEY     = "3795d28242a11619bc25f786f84e53d4"    # Note: Default is G(345)+G(526)+G(424)+G(322) but derived from t.appKey or r.APP_KEY which likely points to Rt.appKey and is different per region and version
PLATFORM    = "W.10001.c"                           # G(398)+"c"
APP_NAME    = "saf-captcha"                         # appName from deviceConfig
APP_VERSION = "W20220202"                           # G(416)+"2"
DEVICE_TYPE = "W"                                   # Vt["WEB"]

# Key used in first AES pass (ye inside Re)
# = ke.WEB_AES_FLAG_SECRET_KEY = me(ACCESS_SEC, WEB_AES_SECRET_KEY["FLAG"])
KEY_O = "c175a358550d02e2"

# Key used in second AES pass (be -> ye)
# = he = me(ACCESS_SEC, WEB_AES_SECRET_KEY["REQ"])
KEY_HE = "45f8ac1e1de14397"

# Fixed IV — from pe.iv.words in the browser
# words: [808530483, 875902519, 943276354, 1128547654]
# hex:   d35db7e3 9ebbf3d0 01083105 43434346  → but use exact bytes from words
_IV_WORDS = [808530483, 875902519, 943276354, 1128547654]

def _build_iv(words: list) -> bytes:
    """Convert CryptoJS WordArray words to 16-byte IV."""
    out = bytearray()
    for w in words:
        out += w.to_bytes(4, 'big')
    return bytes(out[:16])

IV = _build_iv(_IV_WORDS)


# ─────────────────────────────────────────────
#  CORE CRYPTO — replicates ye(key, plaintext)
# ─────────────────────────────────────────────

def ye(key_str: str, plaintext_str: str) -> str:
    """
    Replicates ye(t, r) from AliyunCaptcha.js:

      case "2": S = ue.parse(t)        → UTF-8 encode key → WordArray
      case "4": C = r                  → plaintext string
      case "0": b = ce.encrypt(C, S, pe) → AES-CBC encrypt with fixed IV + PKCS7
      case "5": return b.toString()    → CipherParams.toString() → Base64 ciphertext

    Key must be exactly 16 bytes (AES-128).
    Returns: Base64 string of raw AES-CBC ciphertext (no OpenSSL "Salted__" prefix,
             because key is passed as WordArray → SerializableCipher path).
    """
    if key_str is None or len(key_str) < 16:
        return None
    if plaintext_str is None or len(plaintext_str) == 0:
        return None

    key = key_str.encode('utf-8')[:16]          # AES-128: first 16 bytes
    plaintext = plaintext_str.encode('utf-8')
    cipher = AES.new(key, AES.MODE_CBC, IV)
    encrypted = cipher.encrypt(pad(plaintext, AES.block_size))
    return base64.b64encode(encrypted).decode('utf-8')


def decrypt_ye(ciphertext_b64: str, key_str: str = KEY_O) -> str:
    """
    Decrypt one ye() layer.

    Args:
        ciphertext_b64: Base64 string produced by ye()
        key_str:        16-char AES key. Defaults to KEY_O (first-pass key).
                        Pass KEY_HE to decrypt be() outer layer.
    Returns:
        Plaintext string.
    """
    key = key_str.encode('utf-8')[:16]
    data = base64.b64decode(ciphertext_b64)
    cipher = AES.new(key, AES.MODE_CBC, IV)
    return unpad(cipher.decrypt(data), AES.block_size).decode('utf-8')


def decrypt_full(device_data: str) -> dict:
    """
    Fully decrypt a DeviceData string — reverses both encryption layers.

    Layer 1 (outer): be() → ye(KEY_HE, joined_array)
    Layer 2 (inner): ye(KEY_O, plaintext_c)

    Args:
        device_data: Base64 DeviceData string from generate_device_data()

    Returns dict with keys:
        joined_array  → the "#"-joined array before outer encryption
        array_parts   → list of the 6 array elements
        plaintext_c   → the inner plaintext before first encryption
        app_key       → extracted appKey
        device_type   → "W"
        encrypted_c   → the inner ciphertext sitting in position [2]
        app_version   → extracted APP_VERSION
        scene_id      → extracted sceneId
        prefix        → extracted prefix
        region        → extracted region
        app_name      → extracted appName
    """
    # Outer layer: decrypt be() → ye(KEY_HE, ...)
    joined_array = decrypt_ye(device_data, KEY_HE)
    array_parts = joined_array.split("#")
    # array_parts = [appKey, "W", encrypted_c, APP_VERSION, "CLOUD", ""]

    encrypted_c = array_parts[2]

    # Inner layer: decrypt ye(KEY_O, plaintext_c)
    plaintext_c = decrypt_ye(encrypted_c, KEY_O)
    # plaintext_c = "PLATFORM#appName#sceneId#captcha-normal#prefix#region"
    c_parts = plaintext_c.split("#")

    return {
        "joined_array" : joined_array,
        "array_parts"  : array_parts,
        "plaintext_c"  : plaintext_c,
        "app_key"      : array_parts[0],
        "device_type"  : array_parts[1],
        "encrypted_c"  : encrypted_c,
        "app_version"  : array_parts[3],
        "platform"     : c_parts[0],
        "app_name"     : c_parts[1],
        "scene_id"     : c_parts[2],
        "prefix"       : c_parts[4],
        "region"       : c_parts[5],
    }


# ─────────────────────────────────────────────
#  be() — replicates be(array) from JS
# ─────────────────────────────────────────────

def be(array: list) -> str:
    """
    Replicates be(t) from AliyunCaptcha.js:

      return ye(he, t.join("#"))

    Joins the array with "#" and encrypts with KEY_HE.
    """
    joined = "#".join(array)
    return ye(KEY_HE, joined)


# ─────────────────────────────────────────────
#  Re() — the main DeviceData generator
# ─────────────────────────────────────────────

def generate_device_data(
    scene_id: str,
    prefix: str = "",
    region: str = "cn",
    app_name: str = APP_NAME,
    app_key: str = APP_KEY,
) -> str:
    """
    Replicates Re(t, r, e) from AliyunCaptcha.js.

    Re() does:
      n = appKey
      i = appName
      o = WEB_AES_FLAG_SECRET_KEY          (me() fails → fallback to static key)
      c = PLATFORM + "#" + i + "#" + sceneId + "#captcha-normal#" + prefix + "#" + region
      c = ye(o, c)                          ← first AES pass
      return be([n, "W", c, APP_VERSION, "CLOUD", ""])  ← join + second AES pass

    Args:
        scene_id: Your captcha SceneId (e.g. "didk33e0")
        prefix:   er.prefix from captcha config (usually empty string "")
        region:   er.region from captcha config ("cn" or "sg")
        app_name: appName from deviceConfig (default "saf-captcha")
        app_key:  appKey from deviceConfig (default static value)

    Returns:
        DeviceData string (Base64) — attach as r.DeviceData in the init payload.
    """

    # Build the plaintext that gets first-pass encrypted
    # r.PLATFORM + "#" + i + "#" + (r.sceneId||"") + "#captcha-normal#" + De.prefix + "#" + De.region
    plaintext_c = f"{PLATFORM}#{app_name}#{scene_id}#captcha-normal#{prefix}#{region}"

    # First AES pass: ye(o, c)
    encrypted_c = ye(KEY_O, plaintext_c)

    # be([appKey, "W", encrypted_c, APP_VERSION, "CLOUD", ""])
    # → joins with "#" → ye(he, joined)
    device_data = be([app_key, DEVICE_TYPE, encrypted_c, APP_VERSION, "CLOUD", ""])

    return device_data


# ─────────────────────────────────────────────
#  generate_only_ye() — first pass only, no be()
# ─────────────────────────────────────────────

def generate_only_ye(
    scene_id: str,
    prefix: str = "",
    region: str = "cn",
    app_name: str = APP_NAME,
) -> str:
    """
    Runs only the first ye() pass from Re() — skips be().

    Produces the encrypted_c value that sits at array_parts[2] inside
    a full DeviceData. Useful if you need to inspect or inject just the
    inner ciphertext, or call be() manually yourself.

    Returns: Base64 string of ye(KEY_O, plaintext_c)
    """
    plaintext_c = f"{PLATFORM}#{app_name}#{scene_id}#captcha-normal#{prefix}#{region}"
    return ye(KEY_O, plaintext_c)


# ─────────────────────────────────────────────
#  VERIFICATION (internal helper)
# ─────────────────────────────────────────────

def _verify(device_data: str, scene_id: str, prefix: str, region: str):
    """Decrypt DeviceData back and print each layer."""
    print("\n── Verification ──")
    result = decrypt_full(device_data)
    print(f"Layer 2 (joined array): {result['joined_array']}")
    print(f"Layer 1 (plaintext c):  {result['plaintext_c']}")
    expected = f"{PLATFORM}#{APP_NAME}#{scene_id}#captcha-normal#{prefix}#{region}"
    match = "✅ MATCH" if result['plaintext_c'] == expected else "❌ MISMATCH"
    print(f"Result: {match}")


# ─────────────────────────────────────────────
#  MAIN
# ─────────────────────────────────────────────

if __name__ == "__main__":

    scene_id = "didk33e0"
    prefix   = ""
    region   = "cn"

    # 1. Full DeviceData (ye + be + ye)
    print("── 1. generate_device_data() ──")
    device_data = generate_device_data(scene_id, prefix, region)
    print(f"DeviceData: {device_data}")
    _verify(device_data, scene_id, prefix, region)

    # 2. First ye() pass only — no be()
    print("\n── 2. generate_only_ye() ──")
    only_ye = generate_only_ye(scene_id, prefix, region)
    print(f"encrypted_c: {only_ye}")
    print(f"Decrypted back: {decrypt_ye(only_ye, KEY_O)}")

    # 3. decrypt_ye() — single layer
    print("\n── 3. decrypt_ye() ──")
    outer_plaintext = decrypt_ye(device_data, KEY_HE)
    print(f"Outer layer decrypted: {outer_plaintext}")

    # 4. decrypt_full() — both layers, structured
    print("\n── 4. decrypt_full() ──")
    parsed = decrypt_full(device_data)
    for k, v in parsed.items():
        print(f"  {k:15s}: {v}")
```

## Part 7: Signature and Nonce generation

Every request is signed with a HMAC-SHA1 Signature and Signature Nonce

Shown in function je

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

Notes:
- 1. `o.KEY_SECRET` points to `var Ee.KEY_SECRET` which is  "YSKfst7GaVkXwZYvVihJsKF9r89koz"

### Reverse Engineered Signature Generation Example in Python
`Signature.py`
```python
import hmac
import hashlib
import base64
import urllib.parse
import uuid
from datetime import datetime, timezone
import requests
                                                                                                                                                                          def generate_nonce():
    """Generate UUID v4 as SignatureNonce"""
    return str(uuid.uuid4())

def get_timestamp():
    """Get UTC timestamp in ISO format without microseconds"""
    return datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')

def generate_signature(params, secret_key):
    """
    Generate HMAC-SHA1 signature for Aliyun CAPTCHA API
    """
    # Sort parameters alphabetically
    sorted_params = sorted(params.items())

    # Build canonicalized query string
    canonicalized_query = '&'.join([
        f"{urllib.parse.quote(k, safe='')}={urllib.parse.quote(str(v), safe='')}"
        for k, v in sorted_params
    ])

    # String to sign: POST&%2F& + urlencode(canonicalized_query)
    string_to_sign = f"POST&%2F&{urllib.parse.quote(canonicalized_query, safe='')}"

    # Calculate HMAC-SHA1 (key = secret_key + "&")
    signing_key = (secret_key + "&").encode('utf-8')
    signature = hmac.new(signing_key, string_to_sign.encode('utf-8'), hashlib.sha1)
    signature_b64 = base64.b64encode(signature.digest()).decode('utf-8')

    return signature_b64

def make_captcha_request(access_key_id, secret_key, scene_id, device_token):
    """
    Make a valid CAPTCHA initialization request
    """
    # Generate new nonce and timestamp for each request
    nonce = generate_nonce()
    timestamp = get_timestamp()

    # Build parameters (without Signature)
    params = {
        'AccessKeyId': access_key_id,
        'Action': 'InitCaptchaV3',
        'Format': 'JSON',
        'Language': 'en',
        'Mode': 'popup',
        'SceneId': scene_id,
        'SignatureMethod': 'HMAC-SHA1',
        'SignatureNonce': nonce,
        'SignatureVersion': '1.0',
        'Timestamp': timestamp,
        'UpLang': 'true',
        'Version': '2023-03-05',
        'DeviceToken': device_token,
    }

    # Generate signature using all parameters
    signature = generate_signature(params, secret_key)
    params['Signature'] = signature

    # Build form data
    body = '&'.join([f"{k}={urllib.parse.quote(str(v), safe='')}" for k, v in params.items()])

    # Make request
    response = requests.post(
        'https://no8xfe.captcha-open-southeast.aliyuncs.com/',
        headers={
            'Accept': '*/*',
            'Accept-Language': 'en-US,en;q=0.9',
            'Cache-Control': 'no-cache',
            'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8',
            'Pragma': 'no-cache',
        },
        data=body,
        timeout=30
    )

    return response

# Usage
access_key = "LTAI5tSEBwYMwVKAQGpxmvTd"  # Your AccessKeyId
secret = "YSKfst7GaVkXwZYvVihJsKF9r89koz"  # You need the actual secret key
scene = "didk33e0"
device_token = "U0dfV0VCIzM3OTVkMjgyNDJhMTE2MTliYzI1Zjc4NmY4NGU1M2Q0LWgtMTc3ODc1OTg5ODA4Ny00MWYzYjlmNzAyZmI0MDBiOTkyMDQ1ODYwMWNkOTE5MyNrbDVhUnF6bjdGc0hKYnhLYXh0ZC82WWFwMHFUY1U1SUllRmZpUERMWCt3cVc5NWsrV1JQTHYxNGlRUUtWNUQ3Ui9nTXJDVy9zRzhsWlBEWW1vWFpGYTN4Y2RSMW0vc1pxNS95NmFnTzFKWHM0MkJ2VjdkVnBuZFdHTEtTQjFxL1RGRWNROFl3Qy9OM0dqTTJTY1F0L1lHbkl6ODVQTmRTdnVPRmpJMXZTRVRLWi9POHZsSjRYUzBBcy9UREJGcWlJc0l2eUI2blpYU0RDcmZ1YlUxb3lGNUQvRy9USUFFQThIWWQyUTMrR3YyT0g3U0pnN1RYbWQwSDRqSnNHTVZQdU50RU5oWDdHQXFIVGk3VEJwLzV1S0gvMlpvN2V3eVVjL0c1L0s5am4vUUh5cnlZZHRhdERzVlRQYU1JTG51Q3VrVVlrM1JpRE5VeWtkOHE4eWF5N2d1UkJvQXpQaUZ4aVdEbUtuVERwZUIzbnlXZ0ZBL21ZYWJmZ1dHU09ic2NQRENKY2xGTGRQK0dUZ3p1UWdXZG55QlpVNEVnaTVuMkQzUlNvYTlab244ckZXNFA5Sm1HZ0crQ1IwZVpWeVlYSDFIV2liYTRVVDVxZjh3SkdKclRZODZUbjJoWXNsL3dGT3ZXeTJPek8xZlVTNHd1d3V3REVra0gySTdpY2pldm1lM3doajdNNlV5OXpNUkFJa0JCTjRhUjdqWndyTnlXcFowK0JINjFWN0J4bVlXQVdqemtCcWpVSDREbEZZd3p6aVlUT1FUVlZFTDJ4dmEzV2NkK0dlOTlNVHNVOG9sRVgvWDQ0QzJBS3hpYTBDSzV0ZkZlQzB2Mkh1NUpLSW01Yk9Vd20wVjdLVGovczNHSWVNb25Vb2sxV20yMmVPd0J3aFJDbVhxWVlUZ3VqQjlPN0tPSmlzc1NyMHBOaXBHbXVVTGViYldCSXlrcnlOOUN1VTBsOWsraWsvZ1V5U1ZyRzNveStsaWk1ZVRybVZMdzFDOThGTjh6K09LOEFaU2ZrZDBudVk1UWZIdDFYSk5OYWxrRm1LOWk4ZUFIeXJzaDUydUJFek4rQUVBc1ZFQjUrdEJ6bllmOHQvVllUK05sUW9rcDkzY29jSWhVY0tjYUtMRHJ5Zz09IzY2NSNjMzEzMmRkYmZlMjVjNzZiMzkyNDYwNjgxN2ExZGYxMQ=="

response = make_captcha_request(access_key, secret, scene, device_token)
print(response.json())
```

### Reverse Engineered Signature Generation Psuedocode in Javascript
`Signature.js`
```javascript
const crypto = require('crypto');

function generateAliyunSignature(params, secretKey) {
    // 1. Remove Signature if exists
    delete params.Signature;

    // 2. Sort keys alphabetically
    const sortedKeys = Object.keys(params).sort();

    // 3. Build canonicalized query string
    const canonicalized = sortedKeys.map(key => {
        return encodeURIComponent(key) + "=" + encodeURIComponent(params[key]);
    }).join("&");

    // 4. Build string to sign
    const stringToSign = "POST&%2F&" + encodeURIComponent(canonicalized);

    // 5. Generate HMAC-SHA1 signature (IMPORTANT: add trailing &)
    const signatureKey = secretKey + "&";
    const hmac = crypto.createHmac('sha1', signatureKey);
    hmac.update(stringToSign);

    return hmac.digest('base64');                                                                                                                                         }                                                                                                                                                                                                                                                                                                                                                   // Example usage:

const params = {                                                                                                                                                              AccessKeyId: "LTAI5tSEBwYMwVKAQGpxmvTd",                                                                                                                                  Action: "InitCaptchaV3",                                                                                                                                                  Format: "JSON",
    SignatureMethod: "HMAC-SHA1",
    SignatureVersion: "1.0",
    Timestamp: new Date().toISOString().replace(/\.\d+Z$/, 'Z'),
    Version: "2023-03-05",
    SceneId: "didk33e0",
    Language: "en",
    Mode: "popup",
    UpLang: "true",
    DeviceData: "your_device_data_here"
};

// Generate nonce (UUID v4)
const signatureNonce = crypto.randomUUID();
params.SignatureNonce = signatureNonce;

// Generate signature
const signature = generateAliyunSignature(params, "YSKfst7GaVkXwZYvVihJsKF9r89koz");
params.Signature = signature;

console.log("Generated params with signature:", params);
```

---

## Part 8: CaptchaFlow
### 1. AliyunCaptccha.js Loads

### 2. Loads fielin.*.js
feiling.js generates DeviceToken and exposes a call
`window.um?.getToken`
`window.z_um?.getToken`
DeviceToken is calulated by fielin js and used in client server requests

### 3. InitCaptchaV3
fetch request
```javascript
fetch("https://no8xfe.captcha-open-southeast.aliyuncs.com/", {
  "headers": {
    "accept": "*/*",
    "accept-language": "en-US,en;q=0.9,ur-IN;q=0.8,ur-PK;q=0.7,ur;q=0.6",
    "content-type": "application/x-www-form-urlencoded; charset=UTF-8",
    "priority": "u=1, i",
    "sec-ch-ua": "\"Not-A.Brand\";v=\"24\", \"Chromium\";v=\"146\"",
    "sec-ch-ua-mobile": "?0",
    "sec-ch-ua-platform": "\"Linux\"",
    "sec-fetch-dest": "empty",
    "sec-fetch-mode": "cors",
    "sec-fetch-site": "cross-site"
  },
  "body": "AccessKeyId=LTAI5tSEBwYMwVKAQGpxmvTd&SignatureMethod=HMAC-SHA1&SignatureVersion=1.0&Format=JSON&Timestamp=2026-05-25T02%3A42%3A52Z&Version=2023-03-05&Action=InitCaptchaV3&SceneId=didk33e0&Language=en&Mode=popup&UpLang=true&DeviceData=TEQYvgJq1LrMqFaBybfIzPxz2ygFyAct7X%2Fw%2BLacfXWd9rGSwE%2Fx6ZCONucD1fehS2Qpig6tUVsFK111d9wIk5pWp6rwYjzFCRgL7pNp8bzGsvOSdUXgQTopQm90YPSdCiRAlgENdODLvY7P8jrfO9eC15tPCPwLxcRIrcspVvQYqVfk9%2FyFeIlePKmTRjkM&SignatureNonce=9aab2319-86bf-4019-839e-82225f796296&Signature=UDiQcb82lN5kZT0ZPNOmw0hT8sw%3D",
  "method": "POST",
  "mode": "cors",
  "credentials": "omit"
});
```
DeviceData is sent on first request it is determined by if the previous response inlucded CaptchaTypre Traceless or not since before there was no request it is fired 
On second requst DeviceToken is sent if previous inclueded TRACELESS captcha type
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
[
    "config",
    "deviceConfig",
    "startPOWCalculation",
    "init",
    "bindEvents",
    "show",
    "hide",
    "loading",
    "onBizSuccess",
    "onBizFail",
    "initPopup",
    "initEmbed",
    "initFloat",
    "destroyCaptcha",
    "refresh",
    "onCloseClick",
    "startTracelessVerification"
]
```

The StaticPath is randomised for eg 3.25.0/pe.050.9453665072ad79a7
It is used to load a javascript from path
for StaticPath 3.25.0/pe.050.9453665072ad79a7 it loads

https://g.alicdn.com/captcha-frontend/dynamicJS/3.25.0/pe.050.9453665072ad79a7.js

The javascript calculates data field which is very hard tor reverse engineer

Btw all javasripts calculates data in same way just function names are variables are renamed
using mitmproxy i tested to use a single js every time and it works
### 3. VerifyCaptchaV3
```javascript
fetch("https://no8xfe.captcha-open-southeast.aliyuncs.com/", {
  "headers": {
    "accept": "*/*",
    "accept-language": "en-US,en;q=0.9,ur-IN;q=0.8,ur-PK;q=0.7,ur;q=0.6",
    "content-type": "application/x-www-form-urlencoded; charset=UTF-8",
    "priority": "u=1, i",
    "sec-ch-ua": "\"Not-A.Brand\";v=\"24\", \"Chromium\";v=\"146\"",
    "sec-ch-ua-mobile": "?0",
    "sec-ch-ua-platform": "\"Linux\"",
    "sec-fetch-dest": "empty",
    "sec-fetch-mode": "cors",
    "sec-fetch-site": "cross-site"
  },
  "body": "AccessKeyId=LTAI5tSEBwYMwVKAQGpxmvTd&SignatureMethod=HMAC-SHA1&SignatureVersion=1.0&Format=JSON&Timestamp=2026-05-25T02%3A43%3A00Z&Version=2023-03-05&Action=VerifyCaptchaV3&SceneId=didk33e0&CertifyId=gl62Aqi6e1&CaptchaVerifyParam=%7B%22sceneId%22%3A%22didk33e0%22%2C%22certifyId%22%3A%22gl62Aqi6e1%22%2C%22deviceToken%22%3A%22U0dfV0VCIzM3OTVkMjgyNDJhMTE2MTliYzI1Zjc4NmY4NGU1M2Q0LWgtMTc3OTY3Njk3NzYzMC1kMjA1YTU2MjFmMTY0NDA2YjI5ZTQyMGRkNTUxMzI1MSM3dTU1SjBYc09Uc1IzaXVKS0Zhb09kY1dwVEtxcDBsN2NlRTBHd3VhVy9UVjZ4ZnpVWVhBMWtZQnNWR2pza3A1MW0wak9lMDNadFdHdDV6b3pTOElNOEFPR21ydHUwbnU2ZDdWZUlLbVIrMzNPVkZUYW1VcGtRK0VtVHBTQ1NMa3ZjcXB0NDdFTXdyOFNOMW42SnpWZ0sxMDN6QzA1Q1g1T1Y1bnBSV3hZREdzUFhRVnpTcytxWEFqczhJS3ZiU1dZb0tUNVFFM2ZVR2ZwamNlclNoaWkzSG50OU0ySmlmWmd2amxzSlowYVNqQ0JiL1kwTGlGUlRST1I2Y2FydThMbEdPTFA4TUUxem9hQ1JKWjlSeEE3ekcyVjRmdnBBa0tsU1U4aFZTVkpDMHl2N1ZCaFhuSDMrSWFTWUh1TitRR00vdndoYllqOVNPVUZhQjJDYllQU0xpandobS9HTk1QcVBBQ2xVNllNQndDTVRuYk40MDJmTVorRTVTVVU0eGNXYnBvTnJhdlg1MDVLbktUTEcwbGVmTmpmZVd4VThDL1doWDBVQ1FjZXl1S1owMjM1QjFlZys2cEI3Y2RFR0ZML2xkRTY5VzlkN1U3bzRMYnVYT1IwemZWeDlQKzhmNStBL3dySmZyZXcrSkJBelE4Wm8zYkxScEtac2RjMFZpSENza2ZQeUkwWSsyQStCaHpvd0Q4QXdMQ2tIZisyZ2tBK2xVaGdyRGhEcWlqNU5nL21YSTdwdVFEbUJkRjFKUllzY3ZCbU53OUt4T09xQVBMN2tvaFRqZGw4TjljWUVTdm0zRTQ0bEJ5d21WZEQyOVFmeWwzOWYrbmlHdlJGc0lDaDJMK3ZIMStlTFNBN3B4MFpWV2pMUVI1eUpyeHZ0eUxQVVgxQ2JpZE9FNkp1bEZpd1BpbnlYK0RvTHBEQVZTdnFqRjNWMVY5bVJtMjVqL1YwaDBhMkhkWWV1UTNQNlVOWUtDNk1wc3BPM2JqMmdSR1J1VDA2V050NVRmZWdETWNWeWVFeEZpYWxPWHZia3dhbjZ3ZWYvbEVGUHF3RUVPZ1ROWDAxdEtXUm1VNWxSOGJKQVdjaUhCTnJQOEpOY0FyMGV0Z2xjd1RxVVhOTngrZ2xrYzdkMzJ3Zk5tZ1BCUjV3Q3h3NjdIWTUzVTI3dHNQMjRVVHVOaGc3SkhjNk5kdDBmQ0JvUDRXdmxpaFdEbFBQQ3Vic1FObFZBS1VYZ0JoYUFDMUVMbmt0Ukt6eEZXNWJ1TzlTOUVDT0RBQWdTNVBrZzIzVldpRTJCOW9DbkFyRERpdVpwZDY4bUpHdnljemdZeXFhd0pWSlp6TzVoaTNoazkvNDdlcy85dExGbzAvMmlmcTVTS01kRTdhVkZlNnRtREVCdz09IzAjN2JkNzMyZDIwZTc1NmEyNmY0OTU1ZGJhYjU4MjhjOWE%3D%22%2C%22data%22%3A%22JRMlgg0gDgVASQIXRQRNMFsDRJoGeSB5eQp9YWNPH%2Fw6iT2%2B96hKSApjEY9S1AZt4rwnZQKgGGOjoeIYWpEaAJdJFhEbK15pcCcKYi0%2FNEgaXU4YLj19be54MEMWEBqDqzmjFdV5KuAcSYQ%2FYWYLNeJ3jZ8VzTdU%2FwO4QqMVb8BXhzAENTBkSQ1iXQhwvkMi01UB7yClAj59DW9NKmNkaj1eS%2BQpfSlvKJ1iF2IXEDcbqi4rbP%2B3BDaUnxMkbo0aLSs8RXxbQH8%3D%22%7D&SignatureNonce=bae00c65-c089-41fc-a704-84576ebea034&Signature=azbdVXlzzucKOIz1kICXOmafPJg%3D",
  "method": "POST",
  "mode": "cors",
  "credentials": "omit"
});
```

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


### 4. captcha_verify_param generation and usage

captcha_verify_param is base64 encoded string which contains scene id certify id and token and is snet in completions requests
```base64
eyJjZXJ0aWZ5SWQiOiJnbDYyQXFpNmUxIiwic2NlbmVJZCI6ImRpZGszM2UwIiwiaXNTaWduIjp0cnVlLCJzZWN1cml0eVRva2VuIjoiNm9PbzdlNzJuQTYxdVZMaVpWS2lMWXFGMW05ck9ubzN2RUlQSkthTDdLTHhDSnFiMVVCd1JwbDRwN0VjRlRnZDN5RzA2VENQQmpSMzVNYkNaNWxEcmRqUGNxYWZscWJRTFpRZFgycllkLzhiaG5xaElwQzdTblJsSXhHUHNxdlgifQ==
```
```utf
{"certifyId":"gl62Aqi6e1","sceneId":"didk33e0","isSign":true,"securityToken":"6oOo7e72nA61uVLiZVKiLYqF1m9rOno3vEIPJKaL7KLxCJqb1UBwRpl4p7EcFTgd3yG06TCPBjR35MbCZ5lDrdjPcqaflqbQLZQdX2rYd/8bhnqhIpC7SnRlIxGPsqvX"}
```
