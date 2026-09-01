#!/usr/bin/env python3
"""
suverify_single.py — Python port of the VerifyCaptchaV3 integration tester
Prompts user for deviceToken and runs InitCaptcha → VerifyCaptcha flow
"""

import base64
import hmac
import hashlib
import json
import os
import sys
import time
import uuid
import zlib
from datetime import datetime, timezone
from urllib.parse import quote
from typing import Dict, List, Tuple, Optional

# ============================================================================
# CONFIG
# ============================================================================

ACCESS_KEY = "LTAI5tSEBwYMwVKAQGpxmvTd"
SECRET_KEY = "YSKfst7GaVkXwZYvVihJsKF9r89koz"
SCENE_ID = "didk33e0"

INIT_ENDPOINT = "https://no8xfe.captcha-open-southeast.aliyuncs.com/"
VERIFY_ENDPOINT = "https://no8xfe-verify.captcha-open-southeast.aliyuncs.com/"
API_VERSION = "2023-03-05"

# RC4-like cipher constants
ARG_PERM_TABLE = [
    32, 50, 10, 51, 6, 44, 37, 16, 46, 11, 62, 19, 43, 25, 23, 30,
    60, 33, 53, 34, 7, 26, 12, 48, 5, 2, 20, 4, 61, 13, 47, 49,
    18, 29, 27, 22, 1, 17, 39, 56, 41, 38, 55, 31, 15, 58, 52, 40,
    8, 57, 45, 35, 59, 36, 42, 54, 63, 3, 24, 28, 14, 9, 0, 21,
]
ARG_CONSTANT = "4xrihv8zb8tf1mfj"
ENCRYPT_KEY = "3e627e1b4c63f913"

# ============================================================================
# CRYPTO HELPERS
# ============================================================================

def base64_encode(data: bytes) -> str:
    return base64.b64encode(data).decode('ascii')

def base64_decode_strict(s: str) -> bytes:
    return base64.b64decode(s)

def hmac_sha1(key: bytes, msg: bytes) -> bytes:
    return hmac.new(key, msg, hashlib.sha1).digest()

def generate_uuid() -> str:
    return str(uuid.uuid4())

def get_timestamp_utc() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

def current_time_millis() -> int:
    return int(time.time() * 1000)

def url_encode(s: str, safe: str = "") -> str:
    """Custom URL encoding matching Go's urlEncode"""
    safe_chars = set("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_.~")
    safe_chars.update(safe)
    
    result = []
    for c in s:
        if c in safe_chars:
            result.append(c)
        else:
            # UTF-8 encode
            for byte in c.encode('utf-8'):
                result.append(f"%{byte:02X}")
    return "".join(result)

def generate_signature(params: Dict[str, str], sec_key: str) -> str:
    """Generate Aliyun POP signature"""
    sorted_keys = sorted(params.keys())
    canonical_parts = []
    for k in sorted_keys:
        canonical_parts.append(f"{url_encode(k)}={url_encode(params[k])}")
    canonical = "&".join(canonical_parts)
    
    string_to_sign = f"POST&{url_encode('/')}&{url_encode(canonical)}"
    signing_key = (sec_key + "&").encode('utf-8')
    return base64_encode(hmac_sha1(signing_key, string_to_sign.encode('utf-8')))

def build_query_string(params: Dict[str, str]) -> str:
    sorted_keys = sorted(params.keys())
    parts = []
    for k in sorted_keys:
        parts.append(f"{url_encode(k)}={url_encode(params[k])}")
    return "&".join(parts)

# ============================================================================
# RC4-LIKE CIPHER
# ============================================================================

def generate_arg(certify_id: str) -> str:
    """Generate the arg value using the RC4-like cipher"""
    encoded = url_encode(certify_id)
    
    # URL-decode
    o = bytearray()
    i = 0
    while i < len(encoded):
        if encoded[i] == '%' and i + 2 < len(encoded):
            o.append(int(encoded[i+1:i+3], 16))
            i += 3
        else:
            o.append(ord(encoded[i]))
            i += 1
    
    r = ARG_PERM_TABLE.copy()
    n = ARG_CONSTANT
    rlen = 64
    
    # KSA phase
    i, j = 0, 0
    while i < rlen:
        j = (((i + j + r[i] + r[j]) >> 1) + ord(n[i % len(n)])) & (rlen - 1)
        if i != j:
            r[i], r[j] = r[j], r[i]
        i += 1
    
    # PRGA phase
    t = bytearray()
    e, a = 0, 0
    for idx in range(len(o)):
        a = ((e ^ a) + (r[e] ^ r[a])) & (rlen - 1)
        if e != a:
            r[e], r[a] = r[a], r[e]
        m = o[idx]
        m = m + e + r[e] - a - r[a]
        m = m ^ (r[e] + r[a])
        m = m ^ r[(r[e] + r[a]) & (rlen - 1)]
        m = m & 255
        t.append(m)
        e = (e + 1) & (rlen - 1)
    
    return base64_encode(bytes(t))

def encrypt(plaintext: bytes) -> str:
    """RC4-like encryption"""
    o = plaintext
    n = ENCRYPT_KEY
    r = ARG_PERM_TABLE.copy()
    rlen = 64
    
    # KSA phase
    o_ksa, t_ksa = 0, 0
    while o_ksa < rlen:
        t_ksa = (((o_ksa + t_ksa + r[o_ksa] + r[t_ksa]) >> 1) + ord(n[o_ksa % len(n)])) & (rlen - 1)
        if o_ksa != t_ksa:
            r[o_ksa], r[t_ksa] = r[t_ksa], r[o_ksa]
        o_ksa += 1
    
    # PRGA phase
    t = bytearray()
    e, a = 0, 0
    for n_prga in range(len(o)):
        a = ((e ^ a) + (r[e] ^ r[a])) & (rlen - 1)
        if e != a:
            r[e], r[a] = r[a], r[e]
        m = o[n_prga]
        m = m + e + r[e] - a - r[a]
        m = m ^ (r[e] + r[a])
        m = m ^ r[(r[e] + r[a]) & (rlen - 1)]
        m = m & 255
        t.append(m)
        e = (e + 1) & (rlen - 1)
    
    return base64_encode(bytes(t))

# ============================================================================
# ALI HASH
# ============================================================================

def ali_hash(input_str: str, salt_str: str) -> str:
    """Custom hash with 16-byte state"""
    o = input_str
    r = salt_str
    a_len = len(o)
    m = len(r)
    
    e = [(i << 4) + (i % 16) for i in range(16)]
    f = 16
    
    # KSA-like phase
    i, j = 0, 0
    while i < f:
        j = (((i + j + e[i] + e[j]) >> 1) + ord(r[i % m])) & (f - 1)
        e[i], e[j] = e[j], e[i]
        i += 1
    
    # Hash phase
    idx, p, q = 0, 0, 0
    while idx < a_len:
        q = ((p ^ q) + (e[p] ^ e[q])) & (f - 1)
        e[p], e[q] = e[q], e[p]
        c = ord(o[idx])
        c = (c + p + q) ^ e[p] ^ e[q]
        c = c & 255
        e[p] = c
        p = (p + 1) & (f - 1)
        idx += 1
    
    # Post-processing
    for step in range(2 * f):
        pos = step % f
        if pos != 0:
            e[pos] ^= e[pos - 1]
        else:
            e[0] ^= e[f - 1]
    
    result = []
    for b in e:
        result.append(f"{b:02x}")
    return "".join(result)

# ============================================================================
# FLOW FUNCTIONS
# ============================================================================

def init_captcha() -> Tuple[str, Dict[str, str], str, int]:
    """Initialize captcha and get CertifyId"""
    params = {
        "AccessKeyId": ACCESS_KEY,
        "Action": "InitCaptchaV3",
        "Format": "JSON",
        "Language": "en",
        "Mode": "popup",
        "SceneId": SCENE_ID,
        "SignatureMethod": "HMAC-SHA1",
        "SignatureNonce": generate_uuid(),
        "SignatureVersion": "1.0",
        "Timestamp": get_timestamp_utc(),
        "UpLang": "true",
        "Version": API_VERSION,
    }
    params["Signature"] = generate_signature(params, SECRET_KEY)
    
    body = build_query_string(params)
    
    # Make HTTP request
    import urllib.request
    req = urllib.request.Request(
        INIT_ENDPOINT,
        data=body.encode('utf-8'),
        headers={"Content-Type": "application/x-www-form-urlencoded; charset=UTF-8"},
        method="POST"
    )
    
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            response_body = resp.read().decode('utf-8')
            status = resp.status
    except Exception as e:
        return "", params, str(e), 0
    
    result = json.loads(response_body)
    certify_id = result.get("CertifyId", "")
    
    print(f"\n--- InitCaptchaV3 ---")
    print(f"Response status: {status}")
    print(f"Response body: {response_body}")
    print(f"CertifyId: {certify_id}")
    
    return certify_id, params, response_body, status

def build_data(certify_id: str) -> Tuple[str, str]:
    """Build the Data parameter for VerifyCaptchaV3"""
    arg_value = generate_arg(certify_id)
    ct = current_time_millis()
    
    track = {
        "TrackList": {
            "StartTime": ct
        },
        "TrackStartTime": ct,
        "VerifyTime": ct + 300,
        "Arg": arg_value
    }
    
    json_bytes = json.dumps(track, separators=(',', ':'), ensure_ascii=False).encode('utf-8')
    h = ali_hash(json_bytes.decode('utf-8'), "0000")
    combined = h + json_bytes.decode('utf-8')
    compressed = zlib.compress(combined.encode('utf-8'))
    fb64 = base64_encode(compressed)
    final_val = encrypt(fb64.encode('utf-8'))
    
    return final_val, json_bytes.decode('utf-8')

def verify_captcha(certify_id: str, data_value: str, device_token: str) -> Tuple[Dict[str, str], int, str]:
    """Verify captcha with the device token"""
    cvp = {
        "certifyId": certify_id,
        "data": data_value,
        "deviceToken": device_token,
        "sceneId": SCENE_ID
    }
    cvp_json = json.dumps(cvp, separators=(',', ':'), ensure_ascii=False)
    
    params = {
        "AccessKeyId": ACCESS_KEY,
        "Action": "VerifyCaptchaV3",
        "Format": "JSON",
        "SignatureMethod": "HMAC-SHA1",
        "SignatureVersion": "1.0",
        "Timestamp": get_timestamp_utc(),
        "Version": API_VERSION,
        "SceneId": SCENE_ID,
        "CertifyId": certify_id,
        "CaptchaVerifyParam": cvp_json,
        "SignatureNonce": generate_uuid(),
    }
    params["Signature"] = generate_signature(params, SECRET_KEY)
    
    body = build_query_string(params)
    
    # Make HTTP request
    import urllib.request
    req = urllib.request.Request(
        VERIFY_ENDPOINT,
        data=body.encode('utf-8'),
        headers={"Content-Type": "application/x-www-form-urlencoded; charset=UTF-8"},
        method="POST"
    )
    
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            response_body = resp.read().decode('utf-8')
            status = resp.status
    except Exception as e:
        return params, 0, str(e)
    
    print(f"\n--- VerifyCaptchaV3 ---")
    print(f"Response status: {status}")
    print(f"Response body: {response_body}")
    
    return params, status, response_body

# ============================================================================
# MAIN
# ============================================================================

def main():
    print("=" * 60)
    print("VerifyCaptchaV3 Integration Tester (Python)")
    print("=" * 60)
    
    # Prompt for device token
    device_token = input("\nEnter device token: ").strip()
    
    if not device_token:
        print("ERROR: Device token is required")
        sys.exit(1)
    
    print(f"\nDevice token ({len(device_token)} chars): {device_token[:80]}...")
    
    # Step 1: InitCaptcha
    print("\n[Step 1/2] Initializing captcha...")
    certify_id, init_params, init_resp, init_status = init_captcha()
    
    if not certify_id:
        print(f"ERROR: InitCaptcha failed (status {init_status})")
        sys.exit(1)
    
    # Step 2: Build data
    print("\n[Step 2/2] Building data and verifying...")
    data, track_json = build_data(certify_id)
    print(f"Track JSON: {track_json}")
    print(f"Data ({len(data)} chars): {data[:80]}...")
    
    # Step 3: VerifyCaptcha
    ver_params, ver_status, ver_resp = verify_captcha(certify_id, data, device_token)
    
    # Check result
    try:
        result = json.loads(ver_resp)
        if result.get("Success") and result.get("Result", {}).get("VerifyResult"):
            print("\n" + "=" * 60)
            print("RESULT: PASS (Success && VerifyResult)")
            print("Security token issued")
            print("=" * 60)
            sys.exit(0)
        else:
            print("\n" + "=" * 60)
            print("RESULT: FAIL (VerifyResult=false)")
            print("Token rejected")
            print("=" * 60)
            sys.exit(1)
    except:
        print("\n" + "=" * 60)
        print("RESULT: ERROR - could not parse response")
        print("=" * 60)
        sys.exit(1)

if __name__ == "__main__":
    main()
