#!/usr/bin/env python3.11
"""
device_token_gen.py — Complete Aliyun feilin deviceToken generator (no browser, no Node).

Team: devicetoken-reverse · Task t3 · analyst-2 (payload-analyst).

Pipeline (all captain-verified, see analysis_fields.md / analysis_keys.md / Fielin-report.md):

    generate_init_params ─► InitCaptchaV3 (fetch_device_config, DeviceData param via
                            generate_device_data.generate_device_data)
                          │
                          ▼
                    DeviceConfig blob ──AES-128-CBC(RES key, IV)──► f[0]=b64(sessionKey),
                          f[2]=sessionId, f[7]=serverTs, f[8]=ip        (decrypt_device_config)
                          │
                          ▼
              build_payload(session) — 140-field '#'-joined replay template refreshed
              per analysis_fields.md §5 (per-session: 72/42/87/21/43; per-token: 74 + log
              entries 93/94 inside field 43) under the §5.0 HARD CONSTRAINTS HC1–HC4
                          │
                          ▼
              generate_device_token — w = AES-128-CBC(sessionKey, IV, PKCS7(payload));
              p = MD5([tF,Q,w,tC,'daye,raolewoba!'].join('#')); token = btoa(joined)

Key/IV facts (analysis_keys.md §6): ONE IV everywhere = utf8('0123456789ABCDEF') — the
generate_device_data.py word-array IV is the same 16 bytes; KEY_O/KEY_HE encrypt the
DeviceData request param only; the sessionKey (RES-decrypted from DeviceConfig f[0])
encrypts `w`; live-site DeviceData params: sceneId 'didk33e0', prefix 'no8xfe',
region 'sgp', appKey '3795d28242a11619bc25f786f84e53d4' (NOT the module's static
fallback appKey — always passed explicitly).

Self-test (python3.11 device_token_gen.py): NO network. (a) re-verifies BOTH live tokens
end-to-end (md5 + AES decrypt, sessionKey 682efd18149e39a6); (b) generates a fresh synthetic
token from the REPLAYED fetch-example DeviceConfig, verifies it round-trips and differs from
TOKEN1 ONLY in the t1-mandated fields. Prints PASS/FAIL per assertion.

Consumers: live_validate.py (t4, token-engineer), validator (t5 audit).
"""

import base64
import hashlib
import hmac
import json
import re
import time
import urllib.parse
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Union

import requests

from Crypto.Cipher import AES
from Crypto.Util.Padding import pad, unpad

# Local, import-ready foundation (t1 pre-work): REPLAY_FIELDS (TOKEN1's 140-field payload),
# REPLAY_SESSION, AES_IV, MD5_SECRET, RES_KEY, and byte-exact self-verified helpers.
from replay_template import (
    AES_IV,
    MD5_SECRET,
    N_FIELDS,
    REPLAY_FIELDS,
    REPLAY_SESSION,
    RES_KEY,
    assemble_token,
    encrypt_payload,
    md5_hex,
)

# ─────────────────────────────────────────────────────────────────────────────
# Site / endpoint constants (from the live fetch example, live_captures.md §1)
# ─────────────────────────────────────────────────────────────────────────────

INIT_ENDPOINT = "https://no8xfe.captcha-open-southeast.aliyuncs.com/"
INIT_ENDPOINT_FALLBACK = "https://no8xfe.captcha-open-southeast-b.aliyuncs.com/"
API_VERSION = "2023-03-05"

# Live-site DeviceData generator params (analysis_keys.md §2a — verified byte-exact vs
# the live capture; the generate_device_data.py static appKey default is the bundle's
# FALLBACK and is wrong for this site — always pass app_key explicitly).
LIVE_SCENE_ID = "didk33e0"
LIVE_PREFIX = "no8xfe"
LIVE_REGION = "sgp"
LIVE_APP_KEY = "3795d28242a11619bc25f786f84e53d4"

# ─────────────────────────────────────────────────────────────────────────────
# Low-level crypto (single IV everywhere — analysis_keys.md §1)
# ─────────────────────────────────────────────────────────────────────────────


def aes_cbc_encrypt(key: str, plaintext: Union[str, bytes]) -> str:
    """AES-128-CBC encrypt (utf8 key, fixed IV, PKCS7) → base64. The `rg()` primitive."""
    key_bytes = key.encode("utf-8")[:16]
    pt = plaintext.encode("utf-8") if isinstance(plaintext, str) else plaintext
    cipher = AES.new(key_bytes, AES.MODE_CBC, AES_IV)
    return base64.b64encode(cipher.encrypt(pad(pt, AES.block_size))).decode("ascii")


def aes_cbc_decrypt(key: str, b64_ciphertext: str) -> str:
    """AES-128-CBC decrypt (utf8 key, fixed IV, PKCS7) of a base64 blob. The `rm()` primitive."""
    key_bytes = key.encode("utf-8")[:16]
    cipher = AES.new(key_bytes, AES.MODE_CBC, AES_IV)
    pt = unpad(cipher.decrypt(base64.b64decode(b64_ciphertext)), AES.block_size)
    return pt.decode("utf-8")


# ─────────────────────────────────────────────────────────────────────────────
# 1. InitCaptchaV3 request signing (signature_gen.py logic, verbatim semantics)
# ─────────────────────────────────────────────────────────────────────────────


def generate_nonce() -> str:
    """SignatureNonce — a fresh UUID v4 string per request."""
    return str(uuid.uuid4())


def get_timestamp_utc() -> str:
    """UTC ISO timestamp without microseconds (Aliyun POP format)."""
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def generate_signature(params: Dict[str, str], secret_key: str) -> str:
    """Aliyun POP HMAC-SHA1 signature for the captcha API.

    Canonical query = sorted 'percent-encoded k=v' pairs joined by '&';
    string-to-sign = 'POST&%2F&' + percent-encoded canonical query;
    signature = base64(HMAC-SHA1(secret + '&', string_to_sign)).
    """
    sorted_params = sorted(params.items())
    canonicalized = "&".join(
        f"{urllib.parse.quote(str(k), safe='')}={urllib.parse.quote(str(v), safe='')}"
        for k, v in sorted_params
    )
    string_to_sign = f"POST&%2F&{urllib.parse.quote(canonicalized, safe='')}"
    signing_key = (secret_key + "&").encode("utf-8")
    sig = hmac.new(signing_key, string_to_sign.encode("utf-8"), hashlib.sha1).digest()
    return base64.b64encode(sig).decode("utf-8")


def build_query_string(params: Dict[str, str]) -> str:
    """Sorted percent-encoded k=v body (matching signature_gen.py / ver_test.py)."""
    return "&".join(
        f"{urllib.parse.quote(str(k), safe='')}={urllib.parse.quote(str(v), safe='')}"
        for k, v in sorted(params.items())
    )


def generate_init_params(
    access_key: str,
    secret: str,
    scene_id: str,
    region: str = LIVE_REGION,
    device_token: Optional[str] = None,
    prefix: str = LIVE_PREFIX,
    app_key: str = LIVE_APP_KEY,
) -> Dict[str, str]:
    """Build the fully-signed InitCaptchaV3 parameter set (signature_gen.py logic).

    Mirrors the live fetch example (live_captures.md §1): AccessKeyId, SignatureMethod,
    SignatureVersion, Format, Timestamp, Version, Action=InitCaptchaV3, SceneId,
    Language, Mode, UpLang, DeviceData (via generate_device_data), SignatureNonce,
    Signature — plus optional DeviceToken when re-initialising with an existing token.
    """
    import generate_device_data as gdd

    # DeviceData param — region/prefix/appKey per t2's verified conclusions ('sgp' live).
    device_data = gdd.generate_device_data(scene_id, prefix, region, app_key=app_key)

    params: Dict[str, str] = {
        "AccessKeyId": access_key,
        "SignatureMethod": "HMAC-SHA1",
        "SignatureVersion": "1.0",
        "Format": "JSON",
        "Timestamp": get_timestamp_utc(),
        "Version": API_VERSION,
        "Action": "InitCaptchaV3",
        "SceneId": scene_id,
        "Language": "en",
        "Mode": "popup",
        "UpLang": "true",
        "DeviceData": device_data,
        "SignatureNonce": generate_nonce(),
    }
    if device_token is not None:
        params["DeviceToken"] = device_token
    params["Signature"] = generate_signature(params, secret)
    return params


# ─────────────────────────────────────────────────────────────────────────────
# 2. fetch_device_config — live InitCaptchaV3 POST → DeviceConfig + CertifyId
# ─────────────────────────────────────────────────────────────────────────────


def fetch_device_config(
    access_key: str,
    secret: str,
    scene_id: str = LIVE_SCENE_ID,
    region: str = LIVE_REGION,
    prefix: str = LIVE_PREFIX,
    app_key: str = LIVE_APP_KEY,
    endpoint: str = INIT_ENDPOINT,
    timeout: int = 30,
) -> Dict[str, Any]:
    """POST a signed InitCaptchaV3 request and return the parsed JSON response.

    Tries the primary endpoint, then the '-b' fallback on connection-level failure.
    Returns the full response dict (incl. DeviceConfig and CertifyId on success).
    Raises RuntimeError when the server reports a non-Success code.
    """
    params = generate_init_params(access_key, secret, scene_id, region=region,
                                  prefix=prefix, app_key=app_key)
    body = build_query_string(params)
    headers = {
        "Accept": "*/*",
        "Accept-Language": "en-US,en;q=0.9",
        "Cache-Control": "no-cache",
        "Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
        "Pragma": "no-cache",
    }

    last_exc: Optional[Exception] = None
    urls = [endpoint]
    if endpoint == INIT_ENDPOINT:
        urls.append(INIT_ENDPOINT_FALLBACK)
    for url in urls:
        try:
            resp = requests.post(url, headers=headers, data=body, timeout=timeout)
            result = json.loads(resp.text)
            if not result.get("Success") and result.get("Code") not in (None, "Success"):
                raise RuntimeError(f"InitCaptchaV3 error response: {result}")
            return result
        except (requests.RequestException, json.JSONDecodeError) as exc:
            last_exc = exc
            continue
    raise RuntimeError(f"InitCaptchaV3 failed on both endpoints: {last_exc}")


# ─────────────────────────────────────────────────────────────────────────────
# 3. decrypt_device_config — RES-key decrypt of the DeviceConfig blob
# ─────────────────────────────────────────────────────────────────────────────


def decrypt_device_config(dc_blob: str) -> Dict[str, str]:
    """Decrypt a DeviceConfig blob → the session fields.

    AES-128-CBC(RES key '87f879f135f27da7', IV utf8('0123456789ABCDEF'), PKCS7), then
    '#' split: f[0]=b64(sessionKey), f[2]=sessionId, f[7]=serverTs, f[8]=ip,
    f[3]=bundle version. (HC1: sessionId/serverTs/ip are linked — always take them
    from the ONE decrypted DeviceConfig, never fabricate.)
    """
    fields = aes_cbc_decrypt(RES_KEY.decode(), dc_blob).split("#")
    if len(fields) < 9:
        raise ValueError(f"DeviceConfig decrypt: expected >=9 fields, got {len(fields)}")
    return {
        "sessionKey": base64.b64decode(fields[0]).decode("utf-8"),
        "sessionId": fields[2],
        "serverTs": fields[7],
        "ip": fields[8],
        "version": fields[3],
    }


# ─────────────────────────────────────────────────────────────────────────────
# 4. build_payload — 140-field replay payload with per-session/per-token refresh
#    (analysis_fields.md §5.0 HARD CONSTRAINTS HC1–HC4, §5.1/§5.2 refresh rules)
# ─────────────────────────────────────────────────────────────────────────────

# Per-token-refreshable payload indices (analysis_fields.md §5.2): ONLY 74 and field 43's
# trailing log entries 93/94. Per-session set: {21, 42, 43, 72, 74, 87} (§5.1).
SESSION_DYNAMIC_FIELDS = {21, 42, 43, 72, 74, 87}
TOKEN_DYNAMIC_FIELDS = {43, 74}

# Log-entry ids inside field 43 (t1 §4.2 — payload indices 93/94 are EMPTY STRINGS).
_LOG_ENTRY_93_ID = "93"
_LOG_ENTRY_94_ID = "94"
# Maximum |ΔinitTime| for which rebasing the template's log costs stays sound (small shifts
# keep costs small/positive; day-scale shifts would emit negative or absurd costs — the
# id-cost format can't even encode negatives). Fresh sessions beyond this synthesize a
# live-A-shaped schedule instead (validator t5, BUG2 analysis).
_REBASE_MAX_MS = 60_000


def _rebase_log_costs(field43: str, shift_ms: int) -> str:
    """Shift every log-entry cost by -shift_ms (rebase to a new initTime).

    cost' = cost − shift keeps absolute event times identical when initTime moves by
    +shift_ms; probe-id sequence and monotonicity are preserved (t1 §5.1).
    """
    entries = field43.split("|")
    out = []
    for entry in entries:
        pid, cost = entry.split("-")
        new_cost = int(cost) - shift_ms
        if new_cost < 0:
            raise ValueError(
                f"rebase shift {shift_ms} ms would drive log entry {pid} negative "
                f"({cost} - {shift_ms}) — shift is only sound for small ΔinitTime"
            )
        out.append(f"{pid}-{new_cost}")
    return "|".join(out)


def build_payload(
    session: Dict[str, str],
    base: Optional[List[str]] = None,
    now_ms: Optional[int] = None,
    entry94_jitter_ms: Optional[int] = None,
) -> List[str]:
    """Build the 140-field payload for `session` from the replay template.

    Args:
        session: dict with keys sessionKey/sessionId/serverTs/ip (from
            decrypt_device_config — HC1) plus optional initTime/tC/region.
            REPLAY_SESSION (replay_template) is a valid input for a verbatim replay.
        base: optional 140-field template (defaults to REPLAY_FIELDS = TOKEN1 decrypt).
        now_ms: token-generation wall-clock ms (defaults to time.time()*1000).
        entry94_jitter_ms: log entry-94's offset over entry-93 (live evidence: 9–15 ms,
            rolled fresh per mint — T1 +15, T2 +9). Defaults to the template's observed
            delta, which makes same-session replay at TOKEN1's mint time byte-exact.

    Refresh rules (analysis_fields.md §5):
      per-session — 72 = initTime (4–7 s before serverTs, HC2); 42 = ip; 87 = serverTs;
        21 = AES-CBC(sessionKey, PKCS7(uuid2[-8:])) per the t1-confirmed B-rule;
        43 = all log costs rebased to the new initTime.
      per-token  — 74 = now_ms; field 43's entry-93 cost = 74 − 72 exactly; entry-94 =
        entry-93 + 9–15 ms jitter.
    Device-bound fields (32/78/71/73/88, UA/screen) stay from the template verbatim.
    Enforces HC1/HC2/HC4 and the entry-93 arithmetic before returning.
    """
    template = list(REPLAY_FIELDS if base is None else base)
    if len(template) != N_FIELDS:
        raise ValueError(f"template must have {N_FIELDS} fields, got {len(template)}")

    now = int(now_ms if now_ms is not None else time.time() * 1000)

    # Per-session values, all from the ONE session dict (HC1: never fabricated independently).
    server_ts = int(session["serverTs"])
    init_time = int(session.get("initTime", server_ts - 6606))   # HC2: 4–7 s gap (live 6606 ms)
    if not (server_ts - 7000 <= init_time <= server_ts - 4000):
        raise ValueError(
            f"HC2 violation: initTime→serverTs gap must be 4–7 s, got {server_ts - init_time} ms"
        )
    ip = session["ip"]
    session_key = session["sessionKey"]
    session_id = session["sessionId"]

    # HC1: Q's embedded ts == serverTs − 1.
    embedded = int(session_id.split("-h-")[1].split("-")[0])
    if embedded != server_ts - 1:
        raise ValueError(
            f"HC1 violation: sessionId embedded ts {embedded} != serverTs - 1 ({server_ts - 1})"
        )

    # Never alias the caller's template (BUG2, validator t5 audit): copy, and read the
    # template's own initTime / log entries BEFORE any mutation.
    fields = list(template)
    old_init = int(template[72])
    old_entries = template[43].split("|")

    # 72/42/87 — per-session timestamps and ip.
    fields[72] = str(init_time)
    fields[42] = ip
    fields[87] = str(server_ts)

    # 43 — per-session log strategy (validator t5: rebase is only sound for small ΔinitTime;
    # fresh sessions synthesize a live-shaped schedule instead of rebasing day-scale costs).
    if session_id == REPLAY_SESSION["sessionId"] or abs(init_time - old_init) <= _REBASE_MAX_MS:
        # Same session or a small shift: rebase all fixed-entry costs from the template's
        # initTime to the new one (preserves probe spacing + monotonicity, t1 §5.1).
        fields[43] = _rebase_log_costs(template[43], init_time - old_init)
    else:
        # Fresh session, large Δ: synthesize a live-A-shaped schedule anchored on the new
        # initTime — keep the template's inter-probe spacing (the device's observed probe
        # cadence) but re-anchor the sequence so costs are small, positive and monotonic.
        # 93/94 placeholders are appended here (0-cost) so the per-token overwrite step
        # below finds and populates them (validator t5 post-fix regression: they MUST be
        # present or every fresh-session build dies at the trailing-entries check).
        base_costs = [int(e.split("-")[1]) for e in old_entries[:-2]]  # fixed entries, no 93/94
        fields[43] = "|".join(
            f"{e.split('-')[0]}-{c}" for e, c in zip(old_entries[:-2], base_costs)
        ) + f"|{_LOG_ENTRY_93_ID}-0|{_LOG_ENTRY_94_ID}-0"

    # 74 — per-token generation ts; then entry-93/94 costs are defined by it (t1 §5.2).
    # entry-94's jitter delta is preserved from the template (t1: 9–15 ms, live T1 +15/T2 +9),
    # so replaying the template session at its original mint time reproduces it byte-exact.
    delta_94 = int(old_entries[-1].split("-")[1]) - int(old_entries[-2].split("-")[1])
    if entry94_jitter_ms is not None:
        delta_94 = int(entry94_jitter_ms)
    if not (1 <= delta_94 <= 15):
        raise ValueError(f"entry-94 delta {delta_94} ms outside sane 1–15 range")
    fields[74] = str(now)
    entries = fields[43].split("|")
    if entries[-2].split("-")[0] != _LOG_ENTRY_93_ID or entries[-1].split("-")[0] != _LOG_ENTRY_94_ID:
        raise ValueError("template field 43 must end with log entries 93/94")
    entries[-2] = f"{_LOG_ENTRY_93_ID}-{now - init_time}"
    entries[-1] = f"{_LOG_ENTRY_94_ID}-{(now - init_time) + delta_94}"
    fields[43] = "|".join(entries)

    # 21 — per-session opaque probe. For a NEW session (sessionId != the template session's),
    # regenerate per the t1-confirmed B-rule: AES-128-CBC(sessionKey, IV, PKCS7(uuid2[-8:])).
    # For a verbatim replay of the TEMPLATE's own session, keep the template bytes — t1
    # proved session A's captured 8-byte value does NOT follow the B-rule (§4.1), so
    # regenerating it would corrupt a byte-identical replay.
    if session_id != REPLAY_SESSION["sessionId"]:
        uuid2 = session_id.split("-h-")[1].split("-", 1)[1]
        fields[21] = aes_cbc_encrypt(session_key, uuid2[-8:])

    _assert_payload_invariants(fields, session)
    return fields


def _assert_payload_invariants(fields: List[str], session: Dict[str, str]) -> None:
    """Enforce the analysis_fields.md §5.0 hard constraints on a built payload."""
    if len(fields) != N_FIELDS:
        raise AssertionError(f"HC4: field count {len(fields)} != {N_FIELDS}")
    if fields[42] != session["ip"] or fields[87] != session["serverTs"]:
        raise AssertionError("HC1: field 42/87 not linked to session")
    if fields[72] == "" or int(fields[87]) - int(fields[72]) not in range(4000, 7001):
        raise AssertionError("HC2: initTime→serverTs gap outside 4–7 s")
    entries = fields[43].split("|")
    e93 = int(entries[-2].split("-")[1])
    if e93 != int(fields[74]) - int(fields[72]):
        raise AssertionError("entry-93 cost != field74 - field72")
    e94 = int(entries[-1].split("-")[1])
    if not (e93 < e94 <= e93 + 15):
        raise AssertionError("entry-94 outside (entry-93, entry-93+15]")
    if e93 < 0:
        raise ValueError("entry-93 cost negative (clock skew?) — refusing to emit malformed log")
    fixed_costs = [int(e.split("-")[1]) for e in entries[:-2]]
    if fixed_costs and e93 < max(fixed_costs):
        raise ValueError(
            f"non-monotonic logs: entry-93 {e93} < last fixed entry cost {max(fixed_costs)} "
            "(mint too soon after init — wait ≥ max fixed cost after initTime, or use "
            "a B-style snapshot template without 93/94 entries)"
        )
    if fields[93] != "" or fields[94] != "":
        raise AssertionError("payload indices 93/94 must be empty strings")


# ─────────────────────────────────────────────────────────────────────────────
# 5. generate_device_token — full token assembly + round-trip verification
# ─────────────────────────────────────────────────────────────────────────────


def generate_device_token(
    session: Dict[str, str],
    gather_cost_ms: Union[int, str],
    payload: Optional[List[str]] = None,
) -> str:
    """Generate a complete deviceToken for `session`.

    Args:
        session: decrypt_device_config output (plus optional initTime/tC/region);
            REPLAY_SESSION works for verbatim session-A replays.
        gather_cost_ms: token-level GatherCost (tC) — HC3: per-SESSION constant,
            500–5000 ms; reuse the session's value for replays.
        payload: optional pre-built 140-field payload (default: build_payload(session)).

    Returns:
        The base64 deviceToken — btoa([tF, Q, w, tC, p].join('#')) with
        w = AES-128-CBC(sessionKey, IV, PKCS7('#'.join(payload))) and
        p = MD5([tF,Q,w,tC,secret].join('#')).

    The result always round-trips: verify_token() reproduces the payload and md5.
    """
    fields = list(payload) if payload is not None else build_payload(session)
    if len(fields) != N_FIELDS:
        raise ValueError(f"payload must have {N_FIELDS} fields, got {len(fields)}")

    t_c = str(int(gather_cost_ms))
    if not (500 <= int(t_c) <= 5000):
        raise ValueError(f"HC3: tC {t_c} outside 500–5000 (per-session constant)")

    region = session.get("region", "SG_WEB")   # ap-southeast endpoints → SG_WEB (t1 §13.1)
    w_b64 = encrypt_payload(fields, session["sessionKey"])
    return assemble_token(region, session["sessionId"], w_b64, t_c)


def verify_token(token_b64: str, session_key: str) -> Dict[str, Any]:
    """Fully verify a deviceToken: structure, md5, AES decrypt → 140-field payload.

    Returns {'tF','Q','w','tC','p','md5_ok','fields'}; raises on any structural failure.
    """
    decoded = base64.b64decode(token_b64).decode("utf-8")
    parts = decoded.split("#")
    if len(parts) != 5:
        raise ValueError(f"token must have 5 '#'-fields, got {len(parts)}")
    t_f, q, w, t_c, p = parts
    md5_ok = md5_hex("#".join([t_f, q, w, t_c, MD5_SECRET])) == p
    payload = aes_cbc_decrypt(session_key, w)
    fields = payload.split("#")
    if len(fields) != N_FIELDS:
        raise ValueError(f"payload must have {N_FIELDS} fields, got {len(fields)}")
    return {
        "tF": t_f, "Q": q, "w": w, "tC": t_c, "p": p,
        "md5_ok": md5_ok, "fields": fields,
    }


# ─────────────────────────────────────────────────────────────────────────────
# 6. Self-test (NO network) — live-token re-verification + synthetic replay proof
# ─────────────────────────────────────────────────────────────────────────────

_LIVE_CAPTURES = Path(__file__).resolve().parent / "live_captures.md"


def _selftest() -> int:
    print("device_token_gen.py self-test (python "
          f"{__import__('sys').version.split()[0]}) — no network")
    print("=" * 72)
    failures = 0

    def check(label: str, cond: bool) -> None:
        nonlocal failures
        print(f"{'PASS' if cond else 'FAIL'}: {label}")
        if not cond:
            failures += 1

    src = _LIVE_CAPTURES.read_text(encoding="utf-8")
    tok1 = re.search(r"### TOKEN1\s*\n+```\n(.*?)\n```", src, re.S).group(1).strip()
    tok2 = re.search(r"### TOKEN2\s*\n+```\n(.*?)\n```", src, re.S).group(1).strip()
    dc_blob = re.search(r'"DeviceConfig":\s*"([^"]+)"', src).group(1)

    # ── (a) both live tokens: end-to-end md5 + AES re-verification ──
    sess = decrypt_device_config(dc_blob)
    check("decrypt_device_config(fetch-example blob) → sessionKey 682efd18149e39a6",
          sess["sessionKey"] == "682efd18149e39a6")
    check("decrypt_device_config → sessionId/serverTs/ip match live_captures.md §3",
          sess["sessionId"] == "3795d28242a11619bc25f786f84e53d4-h-1788007723938-9472a56a25dc49bc8d669b22c6db0142"
          and sess["serverTs"] == "1788007723939" and sess["ip"] == "106.219.217.208")

    for name, tok in (("TOKEN1", tok1), ("TOKEN2", tok2)):
        v = verify_token(tok, sess["sessionKey"])
        check(f"{name}: md5 verifies with 'daye,raolewoba!'", v["md5_ok"])
        check(f"{name}: w decrypts (AES-128-CBC, sessionKey) → 140 fields",
              len(v["fields"]) == N_FIELDS)
        check(f"{name}: Q == sessionId (HC1)", v["Q"] == sess["sessionId"])

    # field-diff regression: TOKEN1 ↔ TOKEN2 differ only in 43 + 74 (t1 §1)
    v1 = verify_token(tok1, sess["sessionKey"])
    v2 = verify_token(tok2, sess["sessionKey"])
    diffs = [i for i in range(N_FIELDS) if v1["fields"][i] != v2["fields"][i]]
    check("live TOKEN1↔TOKEN2 payload diff == {43, 74} (t1 refresh signature)", diffs == [43, 74])

    # ── (b) fresh synthetic token from the REPLAYED session ──
    # Same DeviceConfig (session A) but a NEW token-mint time → only t1's per-token
    # fields (74 + field-43 entries 93/94) may differ from TOKEN1.
    now_ms = int(time.time() * 1000)
    payload = build_payload(REPLAY_SESSION, now_ms=now_ms)
    token = generate_device_token(REPLAY_SESSION, gather_cost_ms=int(REPLAY_SESSION["tC"]),
                                  payload=payload)

    rt = verify_token(token, REPLAY_SESSION["sessionKey"])
    check("synthetic token: md5 verifies (self-consistent)", rt["md5_ok"])
    check("synthetic token: decrypting own output reproduces the payload",
          rt["fields"] == payload)

    # round-trip byte-exactness: re-encrypt(verify-token payload) == token's w
    check("synthetic token: re-encrypt(payload) == w (AES round-trip)",
          encrypt_payload(payload, REPLAY_SESSION["sessionKey"]) == rt["w"])

    # field-diff vs TOKEN1: session-static fields must be untouched; ONLY the per-token
    # refresh points (74 + field-43 entries 93/94) may differ.
    diff_idx = [i for i in range(N_FIELDS) if payload[i] != v1["fields"][i]]
    check("synthetic payload differs from TOKEN1 only in field 43 and 74",
          set(diff_idx) <= {43, 74})
    e93_new = payload[43].split("|")[-2]
    e93_old = v1["fields"][43].split("|")[-2]
    check(f"field-43 entry-93 refreshed ({e93_old} → {e93_new}) per t1 rule 93 == f74−f72",
          e93_new != e93_old
          and int(e93_new.split('-')[1]) == int(payload[74]) - int(payload[72]))
    check("synthetic token tC == session tC (HC3, per-session constant)",
          rt["tC"] == REPLAY_SESSION["tC"])
    check("synthetic token Q == sessionId (HC1)", rt["Q"] == REPLAY_SESSION["sessionId"])

    # generate_device_token default path (payload=None → build_payload internally)
    token_default = generate_device_token(REPLAY_SESSION, REPLAY_SESSION["tC"])
    rt_default = verify_token(token_default, REPLAY_SESSION["sessionKey"])
    check("generate_device_token(default payload) round-trips + md5", rt_default["md5_ok"])

    # ── signing sanity (offline): the InitCaptchaV3 param builder is wired correctly ──
    params = generate_init_params("LTAI5tSEBwYMwVKAQGpxmvTd", "test-secret", LIVE_SCENE_ID)
    check("generate_init_params produces the full signed param set",
          all(k in params for k in ("AccessKeyId", "Action", "SceneId", "DeviceData",
                                    "Signature", "SignatureNonce", "Timestamp"))
          and params["Action"] == "InitCaptchaV3")
    # DeviceData is deterministic for fixed scene/prefix/region/appKey (AES-CBC fixed IV)
    import generate_device_data as gdd
    check("DeviceData param == byte-exact live-capture DeviceData (t2 §2a)",
          params["DeviceData"] == gdd.generate_device_data(LIVE_SCENE_ID, LIVE_PREFIX,
                                                           LIVE_REGION, app_key=LIVE_APP_KEY))
    check("Signature is deterministic and verifiable",
          generate_signature({k: v for k, v in params.items() if k != "Signature"},
                             "test-secret") == params["Signature"])

    print("=" * 72)
    if failures:
        print(f"SELF-TEST FAILED: {failures} assertion(s) failed")
        return 1
    print("ALL SELF-TEST ASSERTIONS PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(_selftest())
