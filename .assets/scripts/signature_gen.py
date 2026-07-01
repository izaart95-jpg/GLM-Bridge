# This script includes core signature gen logic and is currently configured for InitCaptchaV3
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
device_token = "Insert-Dev-Token-Here"

response = make_captcha_request(access_key, secret, scene, device_token)
print(response.json())
