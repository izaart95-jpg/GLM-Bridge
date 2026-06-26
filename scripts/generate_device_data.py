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

APP_KEY     = "ab034ec0643f91399eb33e062dc7fae1"   # G(345)+G(526)+G(424)+G(322)
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
