import hmac
import hashlib
import base64
import urllib.parse
import uuid
from datetime import datetime, timezone
import requests
import json
import zlib
import time

# Configuration
ACCESS_KEY = "LTAI5tSEBwYMwVKAQGpxmvTd"
SECRET = "YSKfst7GaVkXwZYvVihJsKF9r89koz"
SCENE = "didk33e0"
DEVICE_TOKEN = "1234567890"

# ============================================================================
# PART 1: InitCaptchaV3 - Get CertifyId
# ============================================================================

def generate_nonce():
    """Generate a unique nonce using UUID."""
    return str(uuid.uuid4())

def get_timestamp():
    """Get current UTC timestamp in ISO format."""
    return datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')

def generate_signature(params, secret_key):
    """
    Generate HMAC-SHA1 signature for Alibaba Cloud API request.
    
    Args:
        params: Dictionary of request parameters
        secret_key: Secret key for signing
    
    Returns:
        Base64 encoded signature string
    """
    sorted_params = sorted(params.items())
    canonicalized_query = '&'.join([
        f"{urllib.parse.quote(k, safe='')}={urllib.parse.quote(str(v), safe='')}"
        for k, v in sorted_params
    ])
    string_to_sign = f"POST&%2F&{urllib.parse.quote(canonicalized_query, safe='')}"
    signing_key = (secret_key + "&").encode('utf-8')
    signature = hmac.new(signing_key, string_to_sign.encode('utf-8'), hashlib.sha1)
    return base64.b64encode(signature.digest()).decode('utf-8')

def make_captcha_request(access_key_id, secret_key, scene_id, device_token):
    """
    Make InitCaptchaV3 API request to get CertifyId.
    
    Args:
        access_key_id: Alibaba Cloud access key ID
        secret_key: Alibaba Cloud secret key
        scene_id: CAPTCHA scene ID
        device_token: Device token identifier
    
    Returns:
        Response object from the API call
    """
    nonce = generate_nonce()
    timestamp = get_timestamp()
    
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
    
    signature = generate_signature(params, secret_key)
    params['Signature'] = signature
    
    body = '&'.join([
        f"{k}={urllib.parse.quote(str(v), safe='')}" 
        for k, v in params.items()
    ])
    
    return requests.post(
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

# ============================================================================
# PART 2: Generate arg field
# ============================================================================

def generate_arg(certify_id, constant="4xrihv8zb8tf1mfj"):
    """
    Generate the 'arg' parameter for CAPTCHA verification.
    
    Args:
        certify_id: The certification ID from InitCaptchaV3
        constant: Constant string for key generation
    
    Returns:
        Base64 encoded arg value
    """
    encoded = urllib.parse.quote(certify_id, safe='')
    decoded = ''
    i = 0
    while i < len(encoded):
        if encoded[i] == '%' and i + 2 < len(encoded):
            decoded += chr(int(encoded[i+1:i+3], 16))
            i += 3
        else:
            decoded += encoded[i]
            i += 1
    
    # Initialize permutation array
    r = [32, 50, 10, 51, 6, 44, 37, 16, 46, 11, 62, 19, 43, 25, 23, 30,
         60, 33, 53, 34, 7, 26, 12, 48, 5, 2, 20, 4, 61, 13, 47, 49,
         18, 29, 27, 22, 1, 17, 39, 56, 41, 38, 55, 31, 15, 58, 52, 40,
         8, 57, 45, 35, 59, 36, 42, 54, 63, 3, 24, 28, 14, 9, 0, 21]
    
    # KSA phase
    i = 0
    j = 0
    while i < len(r):
        j = (((i + j + r[i] + r[j]) >> 1) + ord(constant[i % len(constant)])) & (len(r) - 1)
        if i != j:
            r[i] ^= r[j]
            r[j] ^= r[i]
            r[i] ^= r[j]
        i += 1
    
    # PRGA phase
    result = ''
    e = 0
    a = 0
    for idx, char in enumerate(decoded):
        a = ((e ^ a) + (r[e] ^ r[a])) & (len(r) - 1)
        if e != a:
            r[e] ^= r[a]
            r[a] ^= r[e]
            r[e] ^= r[a]
        
        m = ord(char)
        m = m + e + r[e] - a - r[a]
        m = m ^ (r[e] + r[a])
        m = m ^ r[(r[e] + r[a]) & (len(r) - 1)]
        m = m & 255
        result += chr(m)
        e = (e + 1) & (len(r) - 1)
    
    return base64.b64encode(result.encode('latin-1')).decode('utf-8')

# ============================================================================
# PART 3: Build Track JSON
# ============================================================================

def build_track_json(certify_id):
    """
    Build the track JSON for CAPTCHA verification.
    
    Args:
        certify_id: The certification ID
    
    Returns:
        JSON string of track data
    """
    current_time = int(time.time() * 1000)
    arg_value = generate_arg(certify_id)
    
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
            "startTime": current_time
        },
        "TrackStartTime": current_time,
        "VerifyTime": current_time + 300,
        "arg": arg_value
    }
    return json.dumps(track_json, separators=(',', ':'))

# ============================================================================
# PART 4: Calculate hash
# ============================================================================

def ali_hash(input_str, salt_str="0000"):
    """
    Calculate custom hash for CAPTCHA verification.
    
    Args:
        input_str: Input string to hash
        salt_str: Salt string
    
    Returns:
        Hex string of hash
    """
    o = input_str.encode('utf-8').decode('latin-1')
    a = len(o)
    r = salt_str
    m = len(r)
    
    # Initialize array
    e = [(_i << 4) + (_i % 16) for _i in range(16)]
    f = len(e)
    
    # KSA phase
    i = 0
    j = 0
    while i < f:
        j = (((i + j + e[i] + e[j]) >> 1) + ord(r[i % m])) & (f - 1)
        e[i], e[j] = e[j], e[i]
        i += 1
    
    # PRGA phase
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
    
    # Final mixing
    for step in range(2 * f):
        pos = step % f
        if pos != 0:
            e[pos] ^= e[pos - 1]
        else:
            e[0] ^= e[f - 1]
    
    return ''.join(f"{(b & 0xFF):02x}" for b in e)

# ============================================================================
# PART 5: Final encryption
# ============================================================================

def encrypt(plaintext, key="3e627e1b4c63f913"):
    """
    Encrypt data using custom RC4-like algorithm.
    
    Args:
        plaintext: Data to encrypt (bytes)
        key: Encryption key
    
    Returns:
        Base64 encoded encrypted string
    """
    o = plaintext.decode('latin-1')
    n = key
    r = [32, 50, 10, 51, 6, 44, 37, 16, 46, 11, 62, 19, 43, 25, 23, 30,
         60, 33, 53, 34, 7, 26, 12, 48, 5, 2, 20, 4, 61, 13, 47, 49,
         18, 29, 27, 22, 1, 17, 39, 56, 41, 38, 55, 31, 15, 58, 52, 40,
         8, 57, 45, 35, 59, 36, 42, 54, 63, 3, 24, 28, 14, 9, 0, 21]
    
    # KSA phase
    o_ksa = 0
    t_ksa = 0
    while o_ksa < len(r):
        t_ksa = (((o_ksa + t_ksa + r[o_ksa] + r[t_ksa]) >> 1) + 
                 ord(n[o_ksa % len(n)])) & (len(r) - 1)
        if o_ksa != t_ksa:
            r[o_ksa] ^= r[t_ksa]
            r[t_ksa] ^= r[o_ksa]
            r[o_ksa] ^= r[t_ksa]
        o_ksa += 1
    
    # PRGA phase
    result = ""
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
        m = m + e + r[e] - a - r[a]
        m = m ^ (r[e] + r[a])
        m = m ^ r[(r[e] + r[a]) & (len(r) - 1)]
        m = m & 255
        result += chr(m)
        e = (e + 1) & (len(r) - 1)
        n_prga += 1
    
    return base64.b64encode(result.encode('latin-1')).decode('utf-8')

def verify_captcha(access_key, secret, scene, certify_id, final_value):
    """
    Verify CAPTCHA using VerifyCaptchaV3 API.
    
    Args:
        access_key: Alibaba Cloud access key
        secret: Alibaba Cloud secret
        scene: Scene ID
        certify_id: Certification ID
        final_value: Encrypted verification data
    
    Returns:
        Dictionary with verification result
    """
    captcha_verify_param = {
        "sceneId": scene,
        "certifyId": certify_id,
        "deviceToken": DEVICE_TOKEN,  # Using original device token
        "data": final_value
    }
    
    captcha_verify_param_str = json.dumps(captcha_verify_param, separators=(',', ':'))
    nonce = generate_nonce()
    timestamp = get_timestamp()
    
    params = {
        'AccessKeyId': access_key,
        'Action': 'VerifyCaptchaV3',
        'Format': 'JSON',
        'SignatureMethod': 'HMAC-SHA1',
        'SignatureVersion': '1.0',
        'Timestamp': timestamp,
        'Version': '2023-03-05',
        'SceneId': scene,
        'CertifyId': certify_id,
        'CaptchaVerifyParam': captcha_verify_param_str,
        'SignatureNonce': nonce,
    }
    
    signature = generate_signature(params, secret)
    params['Signature'] = signature
    
    body = '&'.join([
        f"{k}={urllib.parse.quote(str(v), safe='')}" 
        for k, v in params.items()
    ])
    
    response = requests.post(
        'https://no8xfe-verify.captcha-open-southeast.aliyuncs.com/',
        headers={
            'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8',
            'Referer': '',
        },
        data=body,
        timeout=30
    )
    
    return response.json()

# ============================================================================
# Main execution flow
# ============================================================================

def main():
    # Step 1: Initialize CAPTCHA and get CertifyId
    print("Step 1: Initializing CAPTCHA...")
    response = make_captcha_request(ACCESS_KEY, SECRET, SCENE, DEVICE_TOKEN)
    resp_data = response.json()
    print(f"Response: {json.dumps(resp_data, indent=2)}")
    
    certify_id = resp_data['CertifyId']
    print(f"\nCertifyId: {certify_id}")
    
    # Step 2: Build track JSON
    print("\nStep 2: Building track JSON...")
    json_str = build_track_json(certify_id)
    print(f"JSON: {json_str}")
    
    # Step 3: Calculate hash
    print("\nStep 3: Calculating hash...")
    hash_result = ali_hash(json_str, "0000")
    print(f"Hash: {hash_result}")
    
    # Step 4: Combine and compress
    print("\nStep 4: Compressing data...")
    combined = hash_result + json_str
    compressed = zlib.compress(combined.encode('utf-8'))
    final_data = base64.b64encode(compressed)
    final_value = encrypt(final_data)
    
    print(f"\n{'='*60}")
    print("FINAL VALUE (captchaVerifyParam data):")
    print(f"{'='*60}")
    print(final_value)
    print(f"{'='*60}")
    
    # Step 5: Verify CAPTCHA
    print("\nStep 5: Verifying CAPTCHA...")
    verification_result = verify_captcha(ACCESS_KEY, SECRET, SCENE, certify_id, final_value)
    
    print(f"\n=== Response ===")
    print(json.dumps(verification_result, indent=2))
    
    # Step 6: Check verification result
    if verification_result.get('Success') and verification_result.get('Result', {}).get('VerifyResult'):
        print("\n✅ CAPTCHA Verification SUCCESSFUL!")
        print(f"VerifyCode: {verification_result['Result'].get('VerifyCode')}")
        
        # Build final payload
        result = verification_result.get('Result', {})
        security_token = result.get('securityToken')
        response_certify_id = result.get('certifyId')
        
        if security_token and response_certify_id:
            final_payload = {
                "certifyId": response_certify_id,
                "sceneId": SCENE,
                "isSign": True,
                "securityToken": security_token
            }
            final_payload_json = json.dumps(final_payload, separators=(',', ':'))
            final_payload_b64 = base64.b64encode(final_payload_json.encode('utf-8')).decode('utf-8')
            
            print(f"\n{'='*60}")
            print("FINAL BASE64 ENCODED PAYLOAD:")
            print(f"{'='*60}")
            print(final_payload_b64)
        else:
            print("\n⚠️ securityToken or certifyId missing from response")
    else:
        print("\n❌ CAPTCHA Verification FAILED")
        print(f"Message: {verification_result.get('Message')}")

if __name__ == "__main__":
    main()
