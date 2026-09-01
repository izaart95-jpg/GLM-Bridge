#!/usr/bin/env python3.11
"""
replay_template.py — REPLAY foundation for device_token_gen.py (t3 builder).

Team: devicetoken-reverse · pre-work for t3 · derived by analyst-2.

Every constant here was DERIVED from live_captures.md (TOKEN1 + its DeviceConfig) by
a builder script — nothing hand-copied. The __main__ self-test (and
verify_against_live_captures()) re-parses the live file at run time and proves:

  1. REPLAY_FIELDS == TOKEN1's decrypted `w` payload (140 '#'-fields)
  2. re-encrypting REPLAY_FIELDS reproduces TOKEN1's `w` byte-exactly
  3. the re-assembled 5-field token == TOKEN1 byte-exactly (md5 included)
  4. REPLAY_SESSION matches the RES-key DeviceConfig decrypt (HC1 invariants)

Import-safe: _validate() runs at import (structural + hard-constraint checks, no
crypto, no file reads). See analysis_fields.md §5.0 for the HC1–HC4 definitions.

Consumers:
  - device_token_gen.py (t3): `from replay_template import REPLAY_FIELDS, REPLAY_SESSION, ...`
  - validator (t5): independent regression artifact — run `python3.11 replay_template.py`
"""

import base64
import hashlib
import re
from pathlib import Path
from typing import Dict, List, Optional

# ─────────────────────────────────────────────────────────────────────────────
# Bundle-wide constants (captain-verified; see analysis_fields.md §1 / §19)
# ─────────────────────────────────────────────────────────────────────────────

AES_IV: bytes = b"0123456789ABCDEF"     # UTF-8 bytes of the literal — NOT hex-decoded
MD5_SECRET: str = "daye,raolewoba!"     # p = md5([tF,Q,w,tC,secret].join('#'))
RES_KEY: bytes = b"87f879f135f27da7"    # AES key that decrypts the DeviceConfig blob
N_FIELDS: int = 140                     # payload field count (139 '#' separators)
LIVE_CAPTURES: str = str(Path(__file__).resolve().parent / "live_captures.md")

# ─────────────────────────────────────────────────────────────────────────────
# (1) REPLAY_FIELDS — TOKEN1's decrypted `w` payload (session A), 140 elements
# ─────────────────────────────────────────────────────────────────────────────

REPLAY_FIELDS: List[str] = [
  "W.10054",
  "",
  "",
  "",
  "",
  "Linux armv81",
  "Chrome",
  "149.0.0.0",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "8",
  "E2FFHTUwY2c=",
  "4",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "50a98af28ee10a81ef6f31efaae2853b",
  "",
  "8",
  "",
  "Linux",
  "x86_64",
  "",
  "",
  "",
  "",
  "106.219.217.208",
  "10-0|20-2847|11-3546|23-7686|30-7697|40-7759|41-12134|70-12139|71-13504|80-13504|81-13514|93-594050|94-594065",
  "true",
  "",
  "",
  "768*1366",
  "",
  "5",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "saf-captcha",
  "0",
  "",
  "",
  "4SyHGkKVW8fUJZTYIWWLKAoWeJmau7PQKrOjC8GP",
  "1788007717333",
  "O9l1RIX7GMGG35xDAGAL5y9Zq9vHKgI8LAOBLElIU7",
  "1788008311383",
  "desktop",
  "false",
  "",
  "9d4568c009d203ab10e33ea9953a0264",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "1788007723939",
  "MCMwIzAjMCMwIzAjMCMwIzAjMSMwIzAjMCMwIzAjMCMwIzAjMCMxIzEjMCMxMTExMTExMDExMTExMTExMTExMTExMTExMQ==",
  "1",
  "1",
  "true",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "0",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  "",
  ""
]

# ─────────────────────────────────────────────────────────────────────────────
# (2) REPLAY_SESSION — session A metadata (all from the ONE live DeviceConfig;
#     HC1: Q/serverTs/ip/sessionId are linked — never fabricate independently)
# ─────────────────────────────────────────────────────────────────────────────

REPLAY_SESSION: Dict[str, str] = {
    "sessionKey": "682efd18149e39a6",
    "sessionId": "3795d28242a11619bc25f786f84e53d4-h-1788007723938-9472a56a25dc49bc8d669b22c6db0142",
    "serverTs": "1788007723939",
    "ip": "106.219.217.208",
    "initTime": "1788007717333",
    "tC": "4058",
    "region": "SG_WEB"
}

# ─────────────────────────────────────────────────────────────────────────────
# Import-time validation (structural + hard constraints; cheap, no crypto)
# ─────────────────────────────────────────────────────────────────────────────


def _validate() -> None:
    assert len(REPLAY_FIELDS) == N_FIELDS, (
        f"REPLAY_FIELDS: expected {N_FIELDS} fields, got {len(REPLAY_FIELDS)}"
    )
    assert all(isinstance(x, str) for x in REPLAY_FIELDS), "non-str field in REPLAY_FIELDS"

    # HC4: byte count — 648-byte payload + 8-byte PKCS7 -> 656-byte ciphertext
    _pt_len = len("#".join(REPLAY_FIELDS).encode("utf-8"))
    assert _pt_len == 648, f"HC4: payload is {_pt_len} B, expected 648"

    # Format anchors
    assert REPLAY_FIELDS[0] == "W.10054", "field 0 format tag mismatch"
    _entries = REPLAY_FIELDS[43].split("|")
    assert all(re.fullmatch(r"\d+-\d+", e) for e in _entries), "field 43 log format broken"
    assert REPLAY_FIELDS[88] and len(REPLAY_FIELDS[88]) == 96, "field 88 bitmask b64 len != 96"
    assert REPLAY_FIELDS[93] == "" and REPLAY_FIELDS[94] == "", (
        "fields 93/94 must be empty (log values live INSIDE field 43)"
    )

    # HC1: template payload cross-linked with REPLAY_SESSION
    assert REPLAY_FIELDS[42] == REPLAY_SESSION["ip"], "HC1: field 42 != session ip"
    assert REPLAY_FIELDS[87] == REPLAY_SESSION["serverTs"], "HC1: field 87 != session serverTs"
    assert REPLAY_FIELDS[72] == REPLAY_SESSION["initTime"], "field 72 != session initTime"
    _emb = int(REPLAY_SESSION["sessionId"].split("-h-")[1].split("-")[0])
    assert _emb == int(REPLAY_SESSION["serverTs"]) - 1, (
        "HC1: sessionId embedded ts != serverTs - 1"
    )

    # HC2: initTime precedes serverTs by 4–7 s
    _gap = int(REPLAY_SESSION["serverTs"]) - int(REPLAY_SESSION["initTime"])
    assert 4000 <= _gap <= 7000, f"HC2: f72->serverTs gap {_gap} ms outside 4–7 s"

    # HC3: tC per-session constant, sane range
    assert 500 <= int(REPLAY_SESSION["tC"]) <= 5000, "HC3: tC outside 500–5000"

    # §5.2 per-token refresh arithmetic: entry-93 == field74 − field72; 94 = 93 + 9..15
    _e93 = int(_entries[-2].split("-")[1])
    _e94 = int(_entries[-1].split("-")[1])
    assert _e93 == int(REPLAY_FIELDS[74]) - int(REPLAY_FIELDS[72]), (
        "entry-93 != field74 - field72"
    )
    assert _e93 < _e94 <= _e93 + 15, "entry-94 outside (entry-93, entry-93+15]"


_validate()

# ─────────────────────────────────────────────────────────────────────────────
# Crypto helpers — identical recipes to the t3 brief (pycryptodome)
# ─────────────────────────────────────────────────────────────────────────────


def md5_hex(s: str) -> str:
    """MD5 hex of a UTF-8 string."""
    return hashlib.md5(s.encode("utf-8")).hexdigest()


def encrypt_payload(fields: List[str], session_key: str) -> str:
    """w = AES-128-CBC(sessionKey, AES_IV, PKCS7('#'.join(fields))) → base64."""
    from Crypto.Cipher import AES
    from Crypto.Util.Padding import pad

    pt = "#".join(fields).encode("utf-8")
    cipher = AES.new(session_key.encode("utf-8")[:16], AES.MODE_CBC, AES_IV)
    return base64.b64encode(cipher.encrypt(pad(pt, AES.block_size))).decode("ascii")


def assemble_token(region: str, session_id: str, w_b64: str, gather_cost) -> str:
    """token = b64([region, sessionId, w, tC, md5([tF,Q,w,tC,SALT].join('#'))].join('#'))."""
    t_c = str(gather_cost)
    p = md5_hex("#".join([region, session_id, w_b64, t_c, MD5_SECRET]))
    return base64.b64encode("#".join([region, session_id, w_b64, t_c, p]).encode("utf-8")).decode("ascii")


# ─────────────────────────────────────────────────────────────────────────────
# (3) Full crypto regression — parses live_captures.md at run time (no hand-copy)
# ─────────────────────────────────────────────────────────────────────────────


def verify_against_live_captures(path: Optional[str] = None) -> Dict[str, str]:
    """Re-derive everything from live_captures.md and prove the constants.

    Returns a dict of check-label -> "PASS" (raises AssertionError on any failure).
    """
    from Crypto.Cipher import AES
    from Crypto.Util.Padding import unpad

    src_path = Path(path) if path else Path(LIVE_CAPTURES)
    src = src_path.read_text(encoding="utf-8")
    tok1_b64 = re.search(r"### TOKEN1\s*\n+```\n(.*?)\n```", src, re.S).group(1).strip()
    dc_blob = re.search(r'"DeviceConfig":\s*"([^"]+)"', src).group(1)

    # TOKEN1 structure
    t_f, q, w, t_c, p = base64.b64decode(tok1_b64).decode().split("#")
    checks: Dict[str, str] = {}

    # DeviceConfig (RES key) decrypt == REPLAY_SESSION (HC1 linkage)
    dc = unpad(
        AES.new(RES_KEY, AES.MODE_CBC, AES_IV).decrypt(base64.b64decode(dc_blob)), 16
    ).decode().split("#")
    assert base64.b64decode(dc[0]).decode() == REPLAY_SESSION["sessionKey"], "sessionKey mismatch"
    assert dc[2] == REPLAY_SESSION["sessionId"] == q, "sessionId/Q mismatch"
    assert dc[7] == REPLAY_SESSION["serverTs"], "serverTs mismatch"
    assert dc[8] == REPLAY_SESSION["ip"], "ip mismatch"
    checks["DeviceConfig RES decrypt == REPLAY_SESSION (HC1)"] = "PASS"

    # outer token fields match the session
    assert t_f == REPLAY_SESSION["region"], "region mismatch"
    assert t_c == REPLAY_SESSION["tC"], "tC mismatch"
    checks["TOKEN1 outer fields (tF/Q/tC) == REPLAY_SESSION"] = "PASS"

    # (1) decrypt w -> REPLAY_FIELDS
    pt = unpad(
        AES.new(REPLAY_SESSION["sessionKey"].encode(), AES.MODE_CBC, AES_IV).decrypt(
            base64.b64decode(w)
        ),
        16,
    ).decode()
    assert pt.split("#") == REPLAY_FIELDS, "TOKEN1 decrypt != REPLAY_FIELDS"
    checks["TOKEN1 w decrypt == REPLAY_FIELDS (140 fields)"] = "PASS"

    # (2) re-encrypt REPLAY_FIELDS -> byte-exact w
    w_rebuilt = encrypt_payload(REPLAY_FIELDS, REPLAY_SESSION["sessionKey"])
    assert w_rebuilt == w, "re-encrypt(payload) != TOKEN1 w"
    checks["AES re-encrypt(REPLAY_FIELDS) == TOKEN1 w (byte-exact)"] = "PASS"

    # (3) full token re-assembly == TOKEN1 byte-exactly (md5 included)
    tok_rebuilt = assemble_token(t_f, q, w_rebuilt, t_c)
    assert tok_rebuilt == tok1_b64, "re-assembled token != TOKEN1"
    checks["re-assembled 5-field token == TOKEN1 (byte-exact, incl. md5)"] = "PASS"

    # md5 spot-check of the original token
    assert md5_hex("#".join([t_f, q, w, t_c, MD5_SECRET])) == p, "md5 mismatch"
    checks["md5([tF,Q,w,tC,SALT].join('#')) == p"] = "PASS"

    return checks


if __name__ == "__main__":
    print(f"replay_template.py self-test (python {__import__('sys').version.split()[0]})")
    print(f"REPLAY_FIELDS : {len(REPLAY_FIELDS)} fields, payload "
          f"{len('#'.join(REPLAY_FIELDS).encode())} B + 8 B PKCS7 -> 656 B ct")
    print(f"REPLAY_SESSION: sessionKey={REPLAY_SESSION['sessionKey']} region={REPLAY_SESSION['region']} "
          f"tC={REPLAY_SESSION['tC']}")
    print()
    results = verify_against_live_captures()
    for label, status in results.items():
        print(f"{status}: {label}")
    print()
    print("ALL CHECKS PASS — replay_template.py is a valid t3 foundation / regression artifact.")
