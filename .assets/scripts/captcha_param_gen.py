import base64
import hashlib
import hmac
import json
import time
import urllib.parse
import uuid
import zlib
from datetime import datetime, timezone

import requests

# Configuration
access_key = "LTAI5tSEBwYMwVKAQGpxmvTd"
secret = "YSKfst7GaVkXwZYvVihJsKF9r89koz"
scene = "didk33e0"
# device_token = "1234567890"  Works without device_token
# ============================================================================
# PART 1: InitCaptchaV3 - Get CertifyId
# ============================================================================
def generate_nonce():
    return str(uuid.uuid4())


def get_timestamp():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def generate_signature(params, secret_key):
    sorted_params = sorted(params.items())
    canonicalized_query = "&".join(
        [
            f"{urllib.parse.quote(k, safe='')}={urllib.parse.quote(str(v), safe='')}"
            for k, v in sorted_params
        ]
    )
    string_to_sign = f"POST&%2F&{urllib.parse.quote(canonicalized_query, safe='')}"
    signing_key = (secret_key + "&").encode("utf-8")
    signature = hmac.new(signing_key, string_to_sign.encode("utf-8"), hashlib.sha1)
    return base64.b64encode(signature.digest()).decode("utf-8")


# Device Token removed
def make_captcha_request(access_key_id, secret_key, scene_id):
    nonce = generate_nonce()
    timestamp = get_timestamp()
    params = {
        "AccessKeyId": access_key_id,
        "Action": "InitCaptchaV3",
        "Format": "JSON",
        "Language": "en",
        "Mode": "popup",
        "SceneId": scene_id,
        "SignatureMethod": "HMAC-SHA1",
        "SignatureNonce": nonce,
        "SignatureVersion": "1.0",
        "Timestamp": timestamp,
        "UpLang": "true",
        "Version": "2023-03-05",
    }
    signature = generate_signature(params, secret_key)
    params["Signature"] = signature
    body = "&".join(
        [f"{k}={urllib.parse.quote(str(v), safe='')}" for k, v in params.items()]
    )
    response = requests.post(
        "https://no8xfe.captcha-open-southeast.aliyuncs.com/",
        headers={
            "Accept": "*/*",
            "Accept-Language": "en-US,en;q=0.9",
            "Cache-Control": "no-cache",
            "Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
            "Pragma": "no-cache",
        },
        data=body,
        timeout=30,
    )
    return response


response = make_captcha_request(access_key, secret, scene)  # Device token removed
resp_data = response.json()
print(f"Response: {json.dumps(resp_data, indent=2)}")

certify_id = resp_data["CertifyId"]
print(f"\nCertifyId: {certify_id}")

# ============================================================================
# PART 2: Generate arg field
# ============================================================================


def generate_arg(certify_id, constant="4xrihv8zb8tf1mfj"):
    encoded = urllib.parse.quote(certify_id, safe="")
    o = ""
    i = 0
    while i < len(encoded):
        if encoded[i] == "%" and i + 2 < len(encoded):
            o += chr(int(encoded[i + 1 : i + 3], 16))
            i += 3
        else:
            o += encoded[i]
            i += 1
    n = constant
    r = [
        32,
        50,
        10,
        51,
        6,
        44,
        37,
        16,
        46,
        11,
        62,
        19,
        43,
        25,
        23,
        30,
        60,
        33,
        53,
        34,
        7,
        26,
        12,
        48,
        5,
        2,
        20,
        4,
        61,
        13,
        47,
        49,
        18,
        29,
        27,
        22,
        1,
        17,
        39,
        56,
        41,
        38,
        55,
        31,
        15,
        58,
        52,
        40,
        8,
        57,
        45,
        35,
        59,
        36,
        42,
        54,
        63,
        3,
        24,
        28,
        14,
        9,
        0,
        21,
    ]
    i = 0
    j = 0
    while i < len(r):
        j = (((i + j + r[i] + r[j]) >> 1) + ord(n[i % len(n)])) & (len(r) - 1)
        if i != j:
            r[i] ^= r[j]
            r[j] ^= r[i]
            r[i] ^= r[j]
        i += 1
    t = ""
    e = 0
    a = 0
    for idx in range(len(o)):
        a = ((e ^ a) + (r[e] ^ r[a])) & (len(r) - 1)
        if e != a:
            r[e] ^= r[a]
            r[a] ^= r[e]
            r[e] ^= r[a]
        m = ord(o[idx])
        m = m + e + r[e] - a - r[a]
        m = m ^ (r[e] + r[a])
        m = m ^ r[(r[e] + r[a]) & (len(r) - 1)]
        m = m & 255
        t += chr(m)
        e = (e + 1) & (len(r) - 1)
    return base64.b64encode(t.encode("latin-1")).decode("utf-8")


arg_value = generate_arg(certify_id)
print(f"arg: {arg_value}")

# ============================================================================
# PART 3: Build Track JSON
# ============================================================================
current_time = int(time.time() * 1000)
track_json = {
    "TrackList": {
        "mc": "",
        "tc": "",
        "mu": "",
        "te": "",
        "mp": "",
        "tmv": "",
        "ks": "",
        "fi": "",
        "startTime": current_time,
    },
    "TrackStartTime": current_time,
    "VerifyTime": current_time + 300,
    "arg": arg_value,
}
json_str = json.dumps(track_json, separators=(",", ":"))
print(f"JSON: {json_str}")

# ============================================================================
# PART 4: Calculate hash
# ============================================================================


def ali_hash(input_str, salt_str):
    o = input_str.encode("utf-8").decode("latin-1")
    a = len(o)
    r = salt_str
    m = len(r)
    e = []
    for _i in range(16):
        e.append((_i << 4) + (_i % 16))
    f = len(e)
    i = 0
    j = 0
    while i < f:
        j = (((i + j + e[i] + e[j]) >> 1) + ord(r[i % m])) & (f - 1)
        e[i], e[j] = e[j], e[i]
        i += 1
    idx = 0
    p = 0
    q = 0
    while idx < a:
        q = ((p ^ q) + (e[p] ^ e[q])) & (f - 1)
        e[p], e[q] = e[q], e[p]
        C = ord(o[idx])
        C = (C + p + q) ^ e[p] ^ e[q]
        C = C & 255
        e[p] = C
        p = (p + 1) & (f - 1)
        idx += 1
    for step in range(2 * f):
        pos = step % f
        if pos != 0:
            e[pos] ^= e[pos - 1]
        else:
            e[0] ^= e[f - 1]
    return "".join(f"{(b & 0xFF):02x}" for b in e)


hash_result = ali_hash(json_str, "0000")
print(f"Hash: {hash_result}")

# ============================================================================
# PART 5: Combine hash + json
# ============================================================================

combined = hash_result + json_str
print(f"Combined json (first 100 chars): {combined[:100]}...")

# ============================================================================
# PART 6: zlib compress
# ============================================================================

compressed = zlib.compress(combined.encode("utf-8"))

# ============================================================================
# PART 7: Final encryption
# ============================================================================


def encrypt(plaintext):
    o = plaintext.decode("latin-1")
    n = "3e627e1b4c63f913"
    r = [
        32,
        50,
        10,
        51,
        6,
        44,
        37,
        16,
        46,
        11,
        62,
        19,
        43,
        25,
        23,
        30,
        60,
        33,
        53,
        34,
        7,
        26,
        12,
        48,
        5,
        2,
        20,
        4,
        61,
        13,
        47,
        49,
        18,
        29,
        27,
        22,
        1,
        17,
        39,
        56,
        41,
        38,
        55,
        31,
        15,
        58,
        52,
        40,
        8,
        57,
        45,
        35,
        59,
        36,
        42,
        54,
        63,
        3,
        24,
        28,
        14,
        9,
        0,
        21,
    ]
    o_ksa = 0
    t_ksa = 0
    while o_ksa < len(r):
        t_ksa = (
            ((o_ksa + t_ksa + r[o_ksa] + r[t_ksa]) >> 1) + ord(n[o_ksa % len(n)])
        ) & (len(r) - 1)
        if o_ksa != t_ksa:
            r[o_ksa] ^= r[t_ksa]
            r[t_ksa] ^= r[o_ksa]
            r[o_ksa] ^= r[t_ksa]
        o_ksa += 1
    t = ""
    n_prga = 0
    e = 0
    a = 0
    while n_prga < len(o):
        a = ((e ^ a) + (r[e] ^ r[a])) & (len(r) - 1)
        if e != a:
            r[e] ^= r[a]
            r[a] ^= r[e]
            r[e] ^= r[a]
        m = ord(o[n_prga])
        m = m + e + r[e]
        m = m - a - r[a]
        m = m ^ (r[e] + r[a])
        m = m ^ r[(r[e] + r[a]) & (len(r) - 1)]
        m = m & 255
        t += chr(m)
        e = (e + 1) & (len(r) - 1)
        n_prga += 1
    return base64.b64encode(t.encode("latin-1")).decode("utf-8")


final_data = base64.b64encode(compressed)
final_value = encrypt(final_data)
print(f"\n{'='*60}")
print(f"FINAL VALUE (captchaVerifyParam data):")
print(f"{'='*60}")
print(final_value)
print(f"{'='*60}")

# ============================================================================
# PART 8: VerifyCaptchaV3 - Use final_value as data payload
# ============================================================================

# Prompt user for deviceToken for the verify request
print("\nPlease provide the DeviceToken for VerifyCaptchaV3:")
verify_device_token = input("Enter DeviceToken: ").strip()

# Build the CaptchaVerifyParam JSON structure
captcha_verify_param = {
    "sceneId": scene,
    "certifyId": certify_id,
    "deviceToken": verify_device_token,
    "data": final_value,
}

# Convert to JSON string (compact, no spaces)
captcha_verify_param_str = json.dumps(captcha_verify_param, separators=(",", ":"))
# Build parameters for VerifyCaptchaV3 (without Signature)
nonce = generate_nonce()
timestamp = get_timestamp()

params = {
    "AccessKeyId": access_key,
    "Action": "VerifyCaptchaV3",
    "Format": "JSON",
    "SignatureMethod": "HMAC-SHA1",
    "SignatureVersion": "1.0",
    "Timestamp": timestamp,
    "Version": "2023-03-05",
    "SceneId": scene,
    "CertifyId": certify_id,
    "CaptchaVerifyParam": captcha_verify_param_str,
    "SignatureNonce": nonce,
}

# Generate signature using all parameters (including SignatureNonce)
signature = generate_signature(params, secret)
params["Signature"] = signature

# Build form data with URL encoding
body = "&".join(
    [f"{k}={urllib.parse.quote(str(v), safe='')}" for k, v in params.items()]
)

# Make POST request
response = requests.post(
    "https://no8xfe-verify.captcha-open-southeast.aliyuncs.com/",
    headers={
        "Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
        "Referer": "",
    },
    data=body,
    timeout=30,
)

print(f"\n=== Response ===")
print(f"Status Code: {response.status_code}")
try:
    response_json = response.json()
    print(json.dumps(response_json, indent=2))

    # Check if verification was successful
    if response_json.get("Success") and response_json.get("Result", {}).get(
        "VerifyResult"
    ):
        print("\n✅ CAPTCHA Verification SUCCESSFUL!")
        print(f"VerifyCode: {response_json['Result'].get('VerifyCode')}")

        # ============================================================================
        # NEW: Build final JSON with certifyId, securityToken, sceneId, isSign
        # ============================================================================
        result = response_json.get("Result", {})
        security_token = result.get("securityToken")
        response_certify_id = result.get("certifyId")

        if security_token and response_certify_id:
            final_payload = {
                "certifyId": response_certify_id,
                "sceneId": scene,
                "isSign": True,
                "securityToken": security_token,
            }
            final_payload_json = json.dumps(final_payload, separators=(",", ":"))
            final_payload_b64 = base64.b64encode(
                final_payload_json.encode("utf-8")
            ).decode("utf-8")

            print(f"\n{'='*60}")
            print("FINAL BASE64 ENCODED PAYLOAD:")
            print(f"{'='*60}")
            print(final_payload_b64)
        else:
            print("\n⚠️ securityToken or certifyId missing from response")

    else:
        print("\n❌ CAPTCHA Verification FAILED")
        print(f"Message: {response_json.get('Message')}")

except Exception as e:
    print(f"Error parsing response: {e}")
    print(response.text)
