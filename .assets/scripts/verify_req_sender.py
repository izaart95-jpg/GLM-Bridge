import hmac
import hashlib
import base64
import urllib.parse
import uuid
from datetime import datetime, timezone
import requests
import json

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
    # Sort parameters alphabetically (excluding Signature)
    sorted_params = sorted([(k, v) for k, v in params.items() if k != 'Signature'])

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

def make_captcha_request(access_key_id, secret_key, scene_id, certify_id, data, device_token):
    """
    Make a valid CAPTCHA verification request with user-provided values
    """
    # Generate new nonce and timestamp for each request
    nonce = generate_nonce()
    timestamp = get_timestamp()

    # Build the CaptchaVerifyParam JSON structure
    captcha_verify_param = {
        "sceneId": scene_id,
        "certifyId": certify_id,
        "deviceToken": device_token,
        "data": data
    }

    # Convert to JSON string (compact, no spaces)
    captcha_verify_param_str = json.dumps(captcha_verify_param, separators=(',', ':'))

    # Build parameters (without Signature)
    params = {
        'AccessKeyId': access_key_id,
        'Action': 'VerifyCaptchaV3',
        'Format': 'JSON',
        'SignatureMethod': 'HMAC-SHA1',
        'SignatureVersion': '1.0',
        'Timestamp': timestamp,
        'Version': '2023-03-05',
        'SceneId': scene_id,
        'CertifyId': certify_id,
        'CaptchaVerifyParam': captcha_verify_param_str,
        'SignatureNonce': nonce,  # Include SignatureNonce in parameters
    }

    # Generate signature using all parameters (including SignatureNonce)
    signature = generate_signature(params, secret_key)
    params['Signature'] = signature

    # Build form data with URL encoding
    body = '&'.join([f"{k}={urllib.parse.quote(str(v), safe='')}" for k, v in params.items()])

    # Make POST request (not GET)
    response = requests.post(
        'https://no8xfe-verify.captcha-open-southeast.aliyuncs.com/',
        headers={
            'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8',
            'Referer': '',  # Empty referer as in the curl
        },
        data=body,  # Using data parameter for form-urlencoded
        timeout=30
    )

    return response

# Your credentials
access_key = "LTAI5tSEBwYMwVKAQGpxmvTd"
secret = "YSKfst7GaVkXwZYvVihJsKF9r89koz"
scene = "didk33e0"

# Prompt user for input
print("=== Aliyun CAPTCHA Verification Request ===")
print("Please provide the following values:")
print(f"AccessKeyId: {access_key}")
print(f"SceneId: {scene}")
print()

certify_id = input("Enter CertifyId: ").strip()
data = input("Enter data: ").strip()
device_token = input("Enter DeviceToken: ").strip()

print("\nMaking request with:")
print(f"CertifyId: {certify_id}")
print(f"Data: {data[:50]}..." if len(data) > 50 else f"Data: {data}")
print(f"DeviceToken: {device_token[:50]}..." if len(device_token) > 50 else f"DeviceToken: {device_token}")

# Make the request
response = make_captcha_request(access_key, secret, scene, certify_id, data, device_token)

print("\n=== Response ===")
print(f"Status Code: {response.status_code}")
try:
    response_json = response.json()
    print(json.dumps(response_json, indent=2))

    # Check if verification was successful
    if response_json.get('Success') and response_json.get('Result', {}).get('VerifyResult'):
        print("\n✅ CAPTCHA Verification SUCCESSFUL!")
        print(f"VerifyCode: {response_json['Result'].get('VerifyCode')}")
    else:
        print("\n❌ CAPTCHA Verification FAILED")
        print(f"Message: {response_json.get('Message')}")

except Exception as e:
    print(f"Error parsing response: {e}")
    print(response.text)
