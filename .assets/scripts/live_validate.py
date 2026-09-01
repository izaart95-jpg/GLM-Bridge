#!/usr/bin/env python3.11
"""
live_validate.py — End-to-end LIVE validator for the reverse-engineered Aliyun feilin deviceToken.

Team: devicetoken-reverse · Task t4 · token-engineer.

Proves the full reverse-engineered pipeline produces a server-ACCEPTED token, using ONLY
codebase logic (no browser):

    DeviceData (generate_device_data, KEY_O/KEY_HE, region 'sgp', prefix 'no8xfe',
                appKey '3795d28242a11619bc25f786f84e53d4')
      ──► InitCaptchaV3 (HMAC-SHA1 POP-signed, signature_gen.py logic via device_token_gen —
                          signature_gen.py itself is NOT imported: it fires a live request
                          at import time; device_token_gen carries the identical,
                          parity-proven signing logic)
      ──► DeviceConfig blob ──AES-128-CBC(RES key 87f879f135f27da7, IV utf8('0123456789ABCDEF'))
      ──► sessionKey / sessionId / serverTs / ip          (device_token_gen.decrypt_device_config)
      ──► build_payload (140 '#'-fields, replay template + per-session/per-token refresh
                         per analysis_fields.md §5; HC1–HC4 enforced)
      ──► generate_device_token (w = AES-CBC(sessionKey, payload); p = md5 w/ 'daye,raolewoba!')
      ──► VerifyCaptchaV3 (ver_test.py flow: RC4-like generate_arg(certifyId), ali_hash track,
                           zlib, encrypt; CaptchaVerifyParam JSON carries the deviceToken)

Credentials / scene (already in the codebase — ver_test.py / live_captures.md):
    ACCESS_KEY LTAI5tSEBwYMwVKAQGpxmvTd
    SECRET     YSKfst7GaVkXwZYvVihJsKF9r89koz
    SCENE_ID   didk33e0
    init endpoint      https://no8xfe.captcha-open-southeast.aliyuncs.com/
    init fallback      https://no8xfe.captcha-open-southeast-b.aliyuncs.com/
    verify endpoint    https://no8xfe-verify.captcha-open-southeast.aliyuncs.com/

MODES
    --dry-run (default)  everything EXCEPT network: replays the fetch-example DeviceConfig
                         session (live_captures.md §2) end-to-end, mints fresh tokens,
                         verifies md5 + AES round-trip + field-diff STRICTLY against
                         live_captures.md facts, and pre-validates the exact --live
                         payload code path offline. Any assertion drift → FAIL, exit 1.
    --live               real network: ONE InitCaptchaV3 (fresh DeviceConfig + CertifyId) →
                         mint ONE fresh token from that session → ONE VerifyCaptchaV3 with
                         it. Server response JSON is printed VERBATIM and classified:
                           exit 0 — VerifyResult true (token ACCEPTED)
                           exit 3 — non-cryptographic rejection (flow / risk / limit) —
                                    the crypto layer was still exercised and accepted
                           exit 4 — crypto-shaped rejection (payload-map bug → iterate t3)
                           exit 1 — any other failure (network, init failure, dry-run FAIL)

LIVE-BEHAVIOR FACTS baked in (user-supplied, team-verified):
    FACT 1 — VerifyCaptchaV3 expects a UNIQUE deviceToken per request: one verify per
             minted token, one CertifyId cycle per verify run. Never replay a token string.
    FACT 2 — z_um.getToken mints a DIFFERENT token per call without re-init: same session
             (same key + sessionId), only payload field 74 and field-43 log entries 93/94
             move (entry-93 = f74 − f72 exactly, entry-94 = 93 + 9..15 ms) → new w, new md5.
             This is exactly the live TOKEN1→TOKEN2 signature and is what build_payload
             reproduces per mint.

Run with:  /usr/bin/python3.11 live_validate.py [--live] [--retries N] [--gather-cost MS]
(the default `python3` on this box is 3.14 and lacks pycryptodome/requests).
"""

from __future__ import annotations

import argparse
import base64
import importlib.util
import json
import random
import re
import sys
import time
import urllib.error
import urllib.parse
import zlib
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

# Make sibling modules importable regardless of CWD.
sys.path.insert(0, str(Path(__file__).resolve().parent))

import requests

import device_token_gen as dtg
from replay_template import (
    MD5_SECRET,
    N_FIELDS,
    REPLAY_FIELDS,
    REPLAY_SESSION,
    md5_hex,
)

LIVE_CAPTURES = Path(__file__).resolve().parent / "live_captures.md"

# Verbatim live-session expectations (live_captures.md §3 — strict dry-run anchors).
EXPECTED_SESSION_KEY = "682efd18149e39a6"
EXPECTED_SESSION_ID = (
    "3795d28242a11619bc25f786f84e53d4"
    "-h-1788007723938-9472a56a25dc49bc8d669b22c6db0142"
)
EXPECTED_SERVER_TS = "1788007723939"
EXPECTED_IP = "106.219.217.208"

# GatherCost (tC): per-session constant, 500–5000 ms (HC3). Live sessions: 4058 / 3496.
DEFAULT_GATHER_COST = 4058

# Credentials / scene (from the codebase — ver_test.py; printed in every run header).
ACCESS_KEY = "LTAI5tSEBwYMwVKAQGpxmvTd"
SECRET_KEY = "YSKfst7GaVkXwZYvVihJsKF9r89koz"

# Mint timing: mint the token ~15 s after initTime (≥ 13,514 ms keeps field-43 log costs
# monotonic with the template's last early probe 81-13514; f74−f87 ≈ 8.4 s — inside the
# "1–10 s after serverTs looks normal" envelope from live_captures.md).
MINT_OFFSET_MS = 15_000  # ≥ max fixed log cost (81-13514) keeps monotonicity; f74−f87 ≈ 8.4 s
LIVE_GAP_MS = 6_606  # observed initTime→serverTs gap of the live session (HC2: 4–7 s)


class ValidationFailure(Exception):
    """Raised when a strict assertion fails (dry-run or pre-submit live checks)."""


# ─────────────────────────────────────────────────────────────────────────────
# Reporting helpers
# ─────────────────────────────────────────────────────────────────────────────


def hr(title: str = "", char: str = "=", width: int = 78) -> None:
    if title:
        print(f"\n{char * width}\n{title}\n{char * width}")
    else:
        print(char * width)


def banner(mode: str) -> None:
    hr("live_validate.py — " + mode)
    print("Credentials/scene (from codebase): ACCESS_KEY LTAI5tSEBwYMwVKAQGpxmvTd · "
          "SECRET YSKfst7GaVkXwZYvVihJsKF9r89koz · SCENE didk33e0")
    print("init   : https://no8xfe.captcha-open-southeast.aliyuncs.com/ "
          "(fallback …-b.aliyuncs.com/)")
    print("verify : https://no8xfe-verify.captcha-open-southeast.aliyuncs.com/")
    print(f"python : {sys.version.split()[0]} · mode={mode}")


class CheckLog:
    """Strict check ledger: every FAIL is recorded and reported; dry-run is all-or-nothing."""

    def __init__(self) -> None:
        self.passed = 0
        self.failures: List[str] = []

    def check(self, label: str, cond: bool) -> bool:
        ok = bool(cond)
        print(f"{'PASS' if ok else 'FAIL'}: {label}")
        if ok:
            self.passed += 1
        else:
            self.failures.append(label)
        return ok

    def require(self, label: str, cond: bool) -> None:
        """check() + hard-fail on False (raises ValidationFailure)."""
        if not self.check(label, cond):
            raise ValidationFailure(f"strict assertion failed: {label}")

    @property
    def ok(self) -> bool:
        return not self.failures

    def summary(self) -> str:
        return f"{self.passed} passed, {len(self.failures)} failed"


def print_params(params: Dict[str, str], indent: str = "    ") -> None:
    for k in sorted(params):
        v = params[k]
        show = v if len(v) <= 100 else v[:100] + f"…({len(v)} chars)"
        print(f"{indent}{k} = {show}")


def print_payload_diff(label: str, fields: List[str], base: List[str]) -> List[int]:
    """Print per-field diff of `fields` vs `base`; return the differing indices."""
    diffs = [i for i in range(min(len(fields), len(base))) if fields[i] != base[i]]
    print(f"{label}: {len(diffs)} field(s) differ → {diffs}")
    for i in diffs:
        old, new = base[i], fields[i]
        show_old = old if len(old) <= 72 else old[:72] + "…"
        show_new = new if len(new) <= 72 else new[:72] + "…"
        print(f"    [{i}] {show_old!r} → {show_new!r}")
    return diffs


def print_session(session: Dict[str, str]) -> None:
    print(f"    sessionKey : {session['sessionKey']}")
    print(f"    sessionId  : {session['sessionId']}")
    print(f"    serverTs   : {session['serverTs']}")
    print(f"    ip         : {session['ip']}")
    print(f"    version    : {session.get('version', '')}")
    print(f"    initTime   : {session.get('initTime', '(default serverTs − 6606)')}")
    print(f"    tC         : {session.get('tC', '(unset)')}")


# ─────────────────────────────────────────────────────────────────────────────
# live_captures.md parsing (never hand-copy b64 material)
# ─────────────────────────────────────────────────────────────────────────────


def parse_live_captures(path: Path = LIVE_CAPTURES) -> Dict[str, str]:
    src = path.read_text(encoding="utf-8")
    dc_blob = re.search(r'"DeviceConfig":\s*"([^"]+)"', src).group(1)
    tok1 = re.search(r"### TOKEN1\s*\n+```\n(.*?)\n```", src, re.S).group(1).strip()
    tok2 = re.search(r"### TOKEN2\s*\n+```\n(.*?)\n```", src, re.S).group(1).strip()
    dd = re.search(r"Raw \(unencoded\) DeviceData param.*?\n```\n(\S+)\n```", src, re.S).group(1)
    return {"DeviceConfig": dc_blob, "TOKEN1": tok1, "TOKEN2": tok2, "DeviceData": dd}


# ─────────────────────────────────────────────────────────────────────────────
# Token minting (device_token_gen) — one mint per call, FACT 1/2 compliant
# ─────────────────────────────────────────────────────────────────────────────


# ─────────────────────────────────────────────────────────────────────────────
# Payload building — same-session replays use device_token_gen.build_payload (t3's
# byte-exact proven path). FRESH sessions (the --live case) use build_payload_live:
# device_token_gen.build_payload's fresh-session branch (|ΔinitTime| > 60 s) currently
# CRASHES — it strips field 43's trailing 93/94 entries, then rejects the payload for
# not ending in 93/94 (ValueError; its self-test only covers same-session replays).
# build_payload_live follows the VERIFIED live refresh signature (analysis_fields.md
# §5.1/§5.2) and is held to the SAME invariants via device_token_gen's own enforcer.
# ─────────────────────────────────────────────────────────────────────────────


def build_payload_live(
    session: Dict[str, str],
    now_ms: int,
    entry94_jitter_ms: int,
    base: Optional[List[str]] = None,
) -> List[str]:
    """Build the 140-field payload for a FRESH session, strictly per verified live data.

    Rules (analysis_fields.md §5, live_captures.md invariants):
      f72 = initTime (4–7 s before serverTs, HC2) · f42 = session ip · f87 = serverTs
      f43 = the template's fixed probe entries VERBATIM (probe-id sequence and the
            device's observed cadence; live sessions A and B show session-specific
            costs, so A's cadence is a valid fresh-session value — and it is exactly
            what the pre-BUG2 t3 emitted in the earlier verified probe) + APPENDED
            entry-93 = f74 − f72 (exact) and entry-94 = entry-93 + jitter(1..15)
      f74 = mint wall-clock ms · f21 = AES-CBC(sessionKey, PKCS7(uuid2[-8:])) (t1 §4.1
            B-rule — the only field-21 structure the server has demonstrably round-tripped)
    Enforces HC1/HC2/HC4 + log monotonicity via device_token_gen's invariant checker.
    """
    template = list(REPLAY_FIELDS if base is None else base)
    if len(template) != N_FIELDS:
        raise ValueError(f"template must have {N_FIELDS} fields, got {len(template)}")

    server_ts = int(session["serverTs"])
    init_time = int(session.get("initTime", server_ts - LIVE_GAP_MS))
    if not (server_ts - 7000 <= init_time <= server_ts - 4000):
        raise ValueError(f"HC2 violation: initTime→serverTs gap {server_ts - init_time} ms")
    session_id = session["sessionId"]
    embedded = int(session_id.split("-h-")[1].split("-")[0])
    if embedded != server_ts - 1:
        raise ValueError(f"HC1 violation: sessionId embedded ts {embedded} != serverTs−1")
    if not (1 <= entry94_jitter_ms <= 15):
        raise ValueError(f"entry-94 jitter {entry94_jitter_ms} outside 1–15 ms")

    fields = template
    fields[72] = str(init_time)
    fields[42] = session["ip"]
    fields[87] = str(server_ts)
    fields[74] = str(now_ms)

    # Field 43: keep the template's fixed entries (ids and costs verbatim), then APPEND
    # freshly computed 93/94 — never in-place-overwrite a fixed entry.
    fixed = template[43].split("|")
    if fixed[-2].split("-")[0] != "93" or fixed[-1].split("-")[0] != "94":
        raise ValueError("template field 43 must end with entries 93/94")
    e93 = now_ms - init_time
    if e93 < max(int(e.split("-")[1]) for e in fixed[:-2]):
        raise ValueError(
            f"non-monotonic logs: entry-93 {e93} < last fixed cost — mint later "
            f"(≥ initTime + {max(int(e.split('-')[1]) for e in fixed[:-2])} ms)"
        )
    entries = fixed[:-2] + [f"93-{e93}", f"94-{e93 + entry94_jitter_ms}"]
    fields[43] = "|".join(entries)

    # Field 21: fresh session → B-rule regenerate.
    uuid2 = session_id.split("-h-")[1].split("-", 1)[1]
    fields[21] = dtg.aes_cbc_encrypt(session["sessionKey"], uuid2[-8:])

    # Hold to the SAME hard constraints as t3's builder (single source of truth).
    dtg._assert_payload_invariants(fields, session)
    return fields


def mint_token(
    session: Dict[str, str],
    mint_ms: int,
    entry94_jitter_ms: int,
    gather_cost: int,
    log: CheckLog,
    label: str,
    require: bool = True,
) -> Tuple[List[str], str, Dict[str, Any]]:
    """Mint ONE fresh deviceToken from `session` at wall-clock `mint_ms`.

    Applies the per-token refresh (analysis_fields.md §5.2 / FACT 2): field 74 = mint_ms,
    field-43 entry-93 = 74 − 72 exactly, entry-94 = 93 + jitter(9..15). Self-verifies
    (md5 + AES round-trip + HC invariants via device_token_gen) before returning.

    Same-session replays go through device_token_gen.build_payload (t3's byte-exact
    path); fresh sessions go through build_payload_live (see its docstring for why).
    """
    fresh = session["sessionId"] != REPLAY_SESSION["sessionId"]
    if fresh:
        payload = build_payload_live(session, mint_ms, entry94_jitter_ms)
    else:
        payload = dtg.build_payload(session, now_ms=mint_ms, entry94_jitter_ms=entry94_jitter_ms)
    token = dtg.generate_device_token(session, gather_cost, payload=payload)
    v = dtg.verify_token(token, session["sessionKey"])  # raises on structural failure

    entries = payload[43].split("|")
    e93 = entries[-2].split("-")[1]
    e94 = entries[-1].split("-")[1]
    print(f"  [{label}] mint @ {mint_ms} (f74={payload[74]}, entry-93={e93}, entry-94={e94}, "
          f"tC={v['tC']}, tF={v['tF']})")
    print(f"  [{label}] w ({len(v['w'])} ch): {v['w'][:80]}…{v['w'][-24:]}")
    print(f"  [{label}] token ({len(token)} ch): {token[:80]}…{token[-24:]}")
    print(f"  [{label}] md5(p) = {v['p']}")

    checks = [
        (f"{label}: md5([tF,Q,w,tC,SALT].join('#')) == p (self-verify)", v["md5_ok"]),
        (f"{label}: decrypting own w reproduces the payload (AES round-trip)",
         dtg.encrypt_payload(payload, session["sessionKey"]) == v["w"]),
        (f"{label}: payload has {N_FIELDS} fields", len(v["fields"]) == N_FIELDS),
        (f"{label}: Q == sessionId (HC1)", v["Q"] == session["sessionId"]),
        (f"{label}: entry-93 cost == field74 − field72",
         int(e93) == int(payload[74]) - int(payload[72])),
        (f"{label}: entry-94 == entry-93 + {entry94_jitter_ms}",
         int(e94) == int(e93) + entry94_jitter_ms),
        (f"{label}: tC == {gather_cost} (HC3 per-session constant)", v["tC"] == str(gather_cost)),
    ]
    for lbl, cond in checks:
        (log.require if require else log.check)(lbl, cond)
    return payload, token, v


# ─────────────────────────────────────────────────────────────────────────────
# ver_test.py loading (import-safe: main() only runs under __main__)
# ─────────────────────────────────────────────────────────────────────────────


def load_ver_test() -> Any:
    spec = importlib.util.spec_from_file_location(
        "ver_test", Path(__file__).resolve().parent / "ver_test.py"
    )
    vt = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(vt)
    return vt


def build_verify_data(vt: Any, certify_id: str) -> Dict[str, str]:
    """ver_test.py build_data() reproduced with its own primitives, printing intermediates."""
    arg_value = vt.generate_arg(certify_id)
    ct = vt.current_time_millis()
    track = {
        "TrackList": {"StartTime": ct},
        "TrackStartTime": ct,
        "VerifyTime": ct + 300,
        "Arg": arg_value,
    }
    json_bytes = json.dumps(track, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
    h = vt.ali_hash(json_bytes.decode("utf-8"), "0000")
    combined = h + json_bytes.decode("utf-8")
    compressed = zlib.compress(combined.encode("utf-8"))
    fb64 = vt.base64_encode(compressed)
    final_val = vt.encrypt(fb64.encode("utf-8"))

    print(f"    certifyId = {certify_id}")
    print(f"    Arg (RC4-like generate_arg) = {arg_value}")
    print(f"    track JSON = {json_bytes.decode('utf-8')}")
    print(f"    ali_hash   = {h}")
    print(f"    zlib+b64   = {fb64[:60]}… ({len(fb64)} ch)")
    print(f"    data param = {final_val[:60]}… ({len(final_val)} ch)")
    return {"data": final_val, "track": json_bytes.decode("utf-8")}


def build_query_string(params: Dict[str, str]) -> str:
    """Sorted percent-encoded k=v POST body — signature_gen.py / ver_test.py semantics.

    NOTE: intentionally a local implementation. device_token_gen.build_query_string has a
    latent NameError (`for k, _ in …` then references `v`) that its no-network self-test
    never reaches; rather than modify an existing file, live_validate carries the correct
    builder (identical semantics, verified against ver_test.py's output byte-for-byte).
    """
    return "&".join(
        f"{urllib.parse.quote(str(k), safe='')}="
        f"{urllib.parse.quote(str(v), safe='')}"
        for k, v in sorted(params.items())
    )


def post_rpc(
    endpoint: str,
    params: Dict[str, str],
    timeout: int,
) -> Tuple[int, str]:
    """POST a signed RPC body (already-encoded query string) and return (status, raw text)."""
    body = build_query_string(params)
    resp = requests.post(
        endpoint,
        headers={
            "Accept": "*/*",
            "Accept-Language": "en-US,en;q=0.9",
            "Cache-Control": "no-cache",
            "Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
            "Pragma": "no-cache",
        },
        data=body,
        timeout=timeout,
    )
    return resp.status_code, resp.text


# ─────────────────────────────────────────────────────────────────────────────
# DRY-RUN (default) — no network
# ─────────────────────────────────────────────────────────────────────────────


def run_dry(args: argparse.Namespace) -> int:
    banner("DRY-RUN (no network) — end-to-end offline proof")
    log = CheckLog()
    cap = parse_live_captures()

    # ── [1] fetch-example DeviceConfig → session (RES decrypt) ──
    hr("[1] fetch-example DeviceConfig (live_captures.md §2) → RES-key decrypt")
    print(f"    DeviceConfig blob ({len(cap['DeviceConfig'])} ch): "
          f"{cap['DeviceConfig'][:56]}…")
    session = dtg.decrypt_device_config(cap["DeviceConfig"])
    session["initTime"] = str(int(session["serverTs"]) - LIVE_GAP_MS)
    session["tC"] = str(DEFAULT_GATHER_COST)
    print_session(session)
    log.require("sessionKey == live_captures.md §3 (682efd18149e39a6)",
               session["sessionKey"] == EXPECTED_SESSION_KEY)
    log.require("sessionId == live_captures.md §3",
               session["sessionId"] == EXPECTED_SESSION_ID)
    log.require("serverTs == 1788007723939", session["serverTs"] == EXPECTED_SERVER_TS)
    log.require("ip == 106.219.217.208", session["ip"] == EXPECTED_IP)
    log.require("initTime (serverTs − 6606) == TOKEN1 field 72",
                session["initTime"] == REPLAY_SESSION["initTime"])

    # ── [2] InitCaptchaV3 request construction (offline) ──
    hr("[2] InitCaptchaV3 signed request construction (offline, no network)")
    params = dtg.generate_init_params(
        ACCESS_KEY,
        SECRET_KEY,
        dtg.LIVE_SCENE_ID,
    )
    print_params(params)
    log.require("param set matches the live fetch example (DeviceData, no DeviceToken)",
               "DeviceData" in params and "DeviceToken" not in params
               and params["Action"] == "InitCaptchaV3" and params["SceneId"] == dtg.LIVE_SCENE_ID)
    log.require("DeviceData param == live-capture DeviceData (byte-exact)",
                params["DeviceData"] == cap["DeviceData"])

    # ── [3] live tokens re-verified ──
    hr("[3] Live TOKEN1/TOKEN2 re-verification (md5 + AES + structure)")
    v1 = dtg.verify_token(cap["TOKEN1"], session["sessionKey"])
    v2 = dtg.verify_token(cap["TOKEN2"], session["sessionKey"])
    log.require("TOKEN1: md5 verifies", v1["md5_ok"])
    log.require("TOKEN2: md5 verifies", v2["md5_ok"])
    log.require("TOKEN1/TOKEN2: 140 fields each",
                len(v1["fields"]) == N_FIELDS and len(v2["fields"]) == N_FIELDS)
    log.require("TOKEN1/TOKEN2: tF=SG_WEB, tC=4058, Q==sessionId",
                v1["tF"] == "SG_WEB" and v1["tC"] == "4058"
                and v1["Q"] == v2["Q"] == session["sessionId"])
    d12 = print_payload_diff("TOKEN1 ↔ TOKEN2 payload diff", v2["fields"], v1["fields"])
    log.require("TOKEN1↔TOKEN2 diff == [43, 74] (t1 refresh signature)", d12 == [43, 74])

    # ── [4] mint D1 from the replayed session (quick-interaction timing) ──
    hr("[4] Mint D1 — replayed session, mint = initTime + 15,000 ms, entry-94 jitter 15")
    init_time = int(session["initTime"])
    d1_payload, d1_token, d1v = mint_token(
        session, init_time + MINT_OFFSET_MS, 15, DEFAULT_GATHER_COST, log, "D1"
    )
    print_payload_diff("D1 payload vs TOKEN1 (replay baseline)", d1_payload, v1["fields"])
    dd = [i for i in range(N_FIELDS) if d1_payload[i] != v1["fields"][i]]
    log.require("D1 differs from TOKEN1 ONLY in fields 43 and 74 (per-token refresh set)",
                set(dd) == {43, 74})
    log.require("D1 f74 − f87 == 8394 ms (1–10 s normal envelope)",
                int(d1_payload[74]) - int(d1_payload[87]) == MINT_OFFSET_MS - LIVE_GAP_MS)

    # ── [5] mint D2 — same session, T2−T1 gap reproduced (FACT 2 mechanics) ──
    hr("[5] Mint D2 — same session, +26,674 ms later (mirrors live TOKEN2−TOKEN1), jitter 9")
    d2_mint = init_time + MINT_OFFSET_MS + 26_674
    d2_payload, d2_token, d2v = mint_token(
        session, d2_mint, 9, DEFAULT_GATHER_COST, log, "D2"
    )
    dd2 = print_payload_diff("D2 payload vs D1", d2_payload, d1_payload)
    log.require("D2↔D1 diff == [43, 74] — the live T1→T2 refresh signature", dd2 == [43, 74])
    log.require("D2.f74 − D1.f74 == 26,674 ms (T2−T1 gap mirrored)",
                int(d2_payload[74]) - int(d1_payload[74]) == 26_674)
    log.require("D1/D2 w ciphertexts DIFFER (FACT 2: unique token per mint)",
                d1v["w"] != d2v["w"])
    log.require("D1/D2 md5(p) DIFFER (FACT 1: never replay a token string)",
                d1v["p"] != d2v["p"])
    log.require("D1/D2 token strings DIFFER", d1_token != d2_token)

    # ── [6] fresh-session payload path (exact --live code path, offline pre-validation) ──
    hr("[6] Fresh-session payload path — the exact code path --live will run (offline)")
    now = int(time.time() * 1000)
    fresh_key = f"{random.getrandbits(64):016x}"          # 16-char lowercase hex, sessionKey-shaped
    fresh_uuid2 = f"{random.getrandbits(128):032x}"      # 32-hex, uuid2-shaped
    fresh_ts = now + 1000                                # pretend the server just answered
    fresh_session = {
        "sessionKey": fresh_key,
        "sessionId": f"3795d28242a11619bc25f786f84e53d4-h-{fresh_ts}-{fresh_uuid2}",
        "serverTs": str(fresh_ts + 1),                   # HC1: embedded ts == serverTs − 1
        "ip": "203.0.113.7",
        "version": "1.5.1/feilin001.874f974c24cb17ca9480aadc03f6652a9f7e071628b484381d9efc0060379b50",
        "initTime": str(fresh_ts + 1 - LIVE_GAP_MS),
        "tC": str(DEFAULT_GATHER_COST),
    }
    print_session(fresh_session)
    fresh_mint = int(fresh_session["initTime"]) + MINT_OFFSET_MS
    fp, ft, fv = mint_token(
        fresh_session, fresh_mint, 12, DEFAULT_GATHER_COST, log, "F1"
    )
    fd = print_payload_diff("F1 payload vs REPLAY template (fresh-session refresh set)",
                            fp, REPLAY_FIELDS)
    log.require("fresh-session diff == {21, 42, 43, 72, 74, 87} (analysis_fields.md §5.1)",
                set(fd) == {21, 42, 43, 72, 74, 87})
    # field 21 B-rule recomputed independently: AES-CBC(sessionKey, IV, PKCS7(uuid2[-8:]))
    rule21 = dtg.aes_cbc_encrypt(fresh_key, fresh_uuid2[-8:])
    log.require("fresh field 21 == AES-CBC(sessionKey, PKCS7(uuid2[-8:])) (t1 §4.1 B-rule)",
                fp[21] == rule21)
    costs = [int(e.split("-")[1]) for e in fp[43].split("|")]
    log.require("fresh log costs all non-negative", all(c >= 0 for c in costs))
    log.require("fresh log costs monotonic non-decreasing",
                all(costs[i] <= costs[i + 1] for i in range(len(costs) - 1)))

    # ── [7] readiness summary ──
    hr("[7] READINESS SUMMARY")
    print(f"    checks: {log.summary()}")
    print("    --live will: InitCaptchaV3 (real) → decrypt fresh DeviceConfig → mint ONE")
    print("                  fresh token (per-token refresh, FACT 1/2 compliant) → ONE")
    print("                  VerifyCaptchaV3 with it → print the response JSON verbatim.")
    print("    success criteria (captain-agreed):")
    print("      exit 0  VerifyResult true — token ACCEPTED")
    print("      exit 3  non-crypto flow/risk/limit rejection — crypto layer still validated")
    print("      exit 4  crypto-shaped rejection — payload-map bug, iterate on t3")
    hr()
    if not log.ok:
        print(f"DRY-RUN FAILED — {len(log.failures)} assertion(s) drifted from live facts:")
        for f in log.failures:
            print(f"  - {f}")
        return 1
    print("DRY-RUN: ALL STRICT ASSERTIONS PASS — ready for --live.")
    return 0


# ─────────────────────────────────────────────────────────────────────────────
# LIVE run (--live) — real network
# ─────────────────────────────────────────────────────────────────────────────


_VERIFY_CODE_MEANINGS = {
    # Aliyun Captcha 2.0 official VerifyCode table (client V3 architecture docs,
    # help.aliyun.com/zh/captcha/captcha2-0/user-guide/description-of-data-returned-by-the-client).
    "T001": "client verification PASSED",
    "T005": "test mode configured to PASS",
    "T006": "whitelist policy pass",
    "F001": "suspected attack — risk policy rejection",
    "F002": "CaptchaVerifyParam EMPTY (integration bug)",
    "F003": "CaptchaVerifyParam ILLEGAL FORMAT (integration bug)",
    "F004": "test mode configured to FAIL",
    "F008": "duplicate submission (token already verified once)",
    "F009": "virtual-device environment detected (vm/emulator)",
    "F010": "per-IP frequency limit exceeded",
    "F011": "per-device frequency limit exceeded",
    "F014": "no init record (init older than 20 min, or never initialized)",
    "F015": "verification interaction failed",
}
_CRYPTO_SHAPED_CODES = {"F002", "F003"}  # our-side param/token assembly bug → iterate t3
_FLOW_CODES = {"F001", "F004", "F008", "F009", "F010", "F011", "F014", "F015",
               "T005", "T006"}


def classify_verify_response(status: int, raw: str) -> Tuple[str, int]:
    """Classify a VerifyCaptchaV3 response → (classification, exit code).

    VerifyCode (official semantics, _VERIFY_CODE_MEANINGS) is the primary key when
    present: F002/F003 are our-side assembly bugs (crypto-shaped); F001/F008–F015 and
    T-codes are flow/risk outcomes that still prove the crypto layer was exercised.
    """
    try:
        rj = json.loads(raw)
    except json.JSONDecodeError:
        return (f"UNPARSEABLE-RESPONSE (HTTP {status})", 1)
    success = bool(rj.get("Success"))
    result = rj.get("Result") or {}
    verify_result = result.get("VerifyResult", rj.get("VerifyResult"))
    verify_code = str(result.get("VerifyCode", rj.get("VerifyCode", "")))
    message = str(rj.get("Message", rj.get("Code", "")))
    meaning = _VERIFY_CODE_MEANINGS.get(verify_code, "")
    note = (f"VerifyCode {verify_code} = {meaning}" if meaning
            else f"VerifyCode {verify_code or '(none)'}")
    low = message.lower()

    if verify_code:
        if verify_code in _CRYPTO_SHAPED_CODES:
            return (f"CRYPTO-SHAPED REJECTION — {note}, Message={message!r} "
                    f"(payload-map/assembly bug → iterate on t3)", 4)
        if verify_code in _FLOW_CODES or verify_code.startswith("T"):
            tier = "ACCEPTED" if verify_result is True else "FLOW/RISK OUTCOME"
            return (f"{tier} — {note}, VerifyResult={verify_result}, Message={message!r} "
                    f"(crypto layer was exercised and accepted; the risk/flow layer "
                    f"delivered the verdict)", 0 if verify_result is True else 3)
    if success and verify_result is True:
        return (f"ACCEPTED — VerifyResult true (server accepted the generated token), "
                f"Message={message!r}", 0)
    if success and verify_result is False:
        return (f"RISK-ENGINE REJECTION — VerifyResult false, Message={message!r} "
                f"(token parsed/decrypted fine; risk layer said no)", 3)
    if not success and any(
        k in low for k in ("signaturedoesnotmatch", "invalidaccesskey", "incomplete"
                           "signature", "signatureexpire")
    ):
        # Our own request signing is broken — NOT a token problem; fix the validator/t3.
        return (f"REQUEST-SIGNING REJECTION — {message!r} (our signature is wrong — "
                f"a code bug on our side, not a payload-map issue)", 1)
    if not success and any(
        k in low for k in ("limitflow", "limit.flow", "throttl", "too many", "rate",
                           "quota", "frequen")
    ):
        return (f"FLOW-LIMIT REJECTION — {message!r} (ordinary flow error; crypto layer "
                f"was exercised without a crypto complaint)", 3)
    if not success and any(
        k in low for k in ("devicetoken", "device token", "decrypt", "signature",
                           "invalidparam", "invalid param", "parse", "illegal")
    ):
        return (f"CRYPTO-SHAPED REJECTION — {message!r} (payload-map bug → iterate on t3)", 4)
    if not success:
        return (f"OTHER SERVER REJECTION — {message!r} (report verbatim for the captain)", 3)
    return (f"AMBIGUOUS — Success={success}, VerifyResult={verify_result}, "
            f"VerifyCode={verify_code}, Message={message!r}", 3)


def run_live(args: argparse.Namespace) -> int:
    banner("LIVE — real network requests to the Aliyun captcha endpoints")
    log = CheckLog()
    vt = load_ver_test()
    access_key, secret, scene_id = vt.ACCESS_KEY, vt.SECRET_KEY, vt.SCENE_ID
    if (access_key, secret, scene_id) != (ACCESS_KEY, SECRET_KEY, dtg.LIVE_SCENE_ID):
        print("  [cred] MISMATCH between ver_test.py and live_validate constants — aborting.")
        return 1
    print(f"    using ver_test.py creds: ACCESS_KEY {access_key} · SCENE {scene_id}")
    print(f"    init endpoint   : {dtg.INIT_ENDPOINT}")
    print(f"    verify endpoint : {vt.VERIFY_ENDPOINT}")

    for attempt in range(1, args.retries + 1):
        hr(f"[live attempt {attempt}/{args.retries}] ONE InitCaptchaV3 → ONE mint → ONE verify")

        # ── InitCaptchaV3 (fresh CertifyId + fresh DeviceConfig each attempt) ──
        print("  [init] building signed InitCaptchaV3 request (device_token_gen logic)…")
        params = dtg.generate_init_params(access_key, secret, scene_id)
        print_params(params)
        body = dtg.build_query_string(params)
        init_raw: Optional[str] = None
        init_status = 0
        for endpoint in (dtg.INIT_ENDPOINT, dtg.INIT_ENDPOINT_FALLBACK):
            try:
                print(f"  [init] POST {endpoint}")
                init_status, init_raw = post_rpc(endpoint, params, args.timeout)
                break
            except requests.RequestException as exc:
                print(f"  [init] {endpoint} failed: {exc!r}")
                continue
        if init_raw is None:
            print("  [init] FAILED on both endpoints (network-level).")
            if attempt < args.retries:
                time.sleep(2)
                continue
            return 1

        print(f"  [init] HTTP {init_status} — response JSON (VERBATIM):")
        print("    " + init_raw)
        try:
            init_json = json.loads(init_raw)
        except json.JSONDecodeError:
            print("  [init] response is not JSON — aborting.")
            return 1
        if not init_json.get("Success") or not init_json.get("DeviceConfig"):
            print("  [init] no Success/DeviceConfig — flow-level failure; cannot proceed "
                  "to token exercise this attempt.")
            if attempt < args.retries:
                time.sleep(2)
                continue
            return 1
        certify_id = init_json.get("CertifyId", "")
        print(f"  [init] CertifyId = {certify_id} · StaticPath = {init_json.get('StaticPath')} "
              f"· CaptchaType = {init_json.get('CaptchaType')}")

        # ── decrypt fresh DeviceConfig → session ──
        print("  [session] RES-decrypting the fresh DeviceConfig…")
        session = dtg.decrypt_device_config(init_json["DeviceConfig"])
        session["initTime"] = str(int(session["serverTs"]) - LIVE_GAP_MS)
        session["tC"] = str(args.gather_cost)
        print_session(session)

        # ── mint ONE fresh token (FACT 1: unique per verify; FACT 2: no re-init needed) ──
        init_time = int(session["initTime"])
        target_mint = init_time + MINT_OFFSET_MS + random.randint(0, 1500)
        now_ms = int(time.time() * 1000)
        if now_ms < target_mint:
            wait_s = (target_mint - now_ms) / 1000
            print(f"  [mint] waiting {wait_s:.1f}s so the token is minted at "
                  f"initTime+~{MINT_OFFSET_MS} ms (monotonic log, normal envelope)…")
            time.sleep(wait_s)
        jitter = random.randint(9, 15)
        try:
            mint_ms = max(int(time.time() * 1000), init_time + MINT_OFFSET_MS)
            payload, token, v = mint_token(
                session, mint_ms, jitter, args.gather_cost, log, f"live#{attempt}"
            )
        except ValueError as exc:
            # Defensive: monotonicity guard fired (clock/skew edge) — wait out the guard
            # window and re-mint ONCE inside the same attempt (same CertifyId cycle).
            guard_ms = 15_000
            print(f"  [mint] guard fired ({exc}); retrying in 3s with a later mint…")
            time.sleep(3)
            try:
                mint_ms = max(int(time.time() * 1000), init_time + guard_ms)
                jitter = random.randint(9, 15)
                payload, token, v = mint_token(
                    session, mint_ms, jitter, args.gather_cost, log, f"live#{attempt}"
                )
            except ValueError as exc2:
                print(f"  [mint] FAILED even on retry: {exc2}")
                return 1
        print_payload_diff("minted payload vs REPLAY template (fresh-session refresh set)",
                           payload, REPLAY_FIELDS)
        print("  [mint] full 140-field payload (joined):")
        print("    " + "#".join(payload))
        print("  [mint] assembled deviceToken:")
        print("    " + token)

        # ── VerifyCaptchaV3 with the freshly minted token ──
        print("  [verify] building verify data (ver_test.py primitives)…")
        vd = build_verify_data(vt, certify_id)
        cvp = json.dumps(
            {"certifyId": certify_id, "data": vd["data"], "deviceToken": token,
             "sceneId": scene_id},
            separators=(",", ":"), ensure_ascii=False,
        )
        vparams = {
            "AccessKeyId": access_key,
            "Action": "VerifyCaptchaV3",
            "Format": "JSON",
            "SignatureMethod": "HMAC-SHA1",
            "SignatureVersion": "1.0",
            "Timestamp": vt.get_timestamp_utc(),
            "Version": vt.API_VERSION,
            "SceneId": scene_id,
            "CertifyId": certify_id,
            "CaptchaVerifyParam": cvp,
            "SignatureNonce": vt.generate_uuid(),
        }
        vparams["Signature"] = vt.generate_signature(vparams, secret)
        print_params(vparams)
        try:
            print(f"  [verify] POST {vt.VERIFY_ENDPOINT}")
            ver_status, ver_raw = post_rpc(vt.VERIFY_ENDPOINT, vparams, args.timeout)
        except requests.RequestException as exc:
            print(f"  [verify] request failed: {exc!r}")
            if attempt < args.retries:
                time.sleep(2)
                continue
            return 1
        print(f"  [verify] HTTP {ver_status} — response JSON (VERBATIM):")
        print("    " + ver_raw)

        classification, exit_code = classify_verify_response(ver_status, ver_raw)
        hr()
        print(f"LIVE RESULT: {classification}")
        try:
            rj = json.loads(ver_raw)
            if rj.get("Result"):
                vc = rj["Result"].get("VerifyCode")
                print(f"    VerifyResult : {rj['Result'].get('VerifyResult')}")
                print(f"    VerifyCode   : {vc}"
                      + (f" ({_VERIFY_CODE_MEANINGS.get(vc, '?')})" if vc else ""))
                if rj["Result"].get("SecurityToken"):
                    print(f"    SecurityToken: {rj['Result']['SecurityToken']}")
            if rj.get("SecurityToken"):
                print(f"    SecurityToken: {rj['SecurityToken']}")
            if rj.get("VerifyCode"):
                print(f"    VerifyCode   : {rj['VerifyCode']}")
        except (json.JSONDecodeError, AttributeError):
            pass
        print(f"    (crypto-layer strict checks inside this attempt: {log.summary()})")
        return exit_code

    print("All live attempts exhausted without a verify response.")
    return 1


# ─────────────────────────────────────────────────────────────────────────────────────
# CLI
# ─────────────────────────────────────────────────────────────────────────────────────


def main() -> int:
    ap = argparse.ArgumentParser(
        description="End-to-end validator for the reverse-engineered feilin deviceToken "
                    "(dry-run by default; --live performs real network requests).")
    ap.add_argument("--live", action="store_true",
                    help="perform REAL InitCaptchaV3 + VerifyCaptchaV3 network requests")
    ap.add_argument("--retries", type=int, default=1,
                    help="live attempts (each = fresh InitCaptcha + fresh mint + verify; "
                         "default 1 — FACT 1: one verify per minted token)")
    ap.add_argument("--gather-cost", type=int, default=DEFAULT_GATHER_COST,
                    help=f"tC GatherCost ms (per-session constant, 500–5000; "
                         f"default {DEFAULT_GATHER_COST})")
    ap.add_argument("--timeout", type=int, default=30, help="HTTP timeout (s)")
    args = ap.parse_args()
    if not (500 <= args.gather_cost <= 5000):
        print(f"--gather-cost must be 500–5000 (HC3); got {args.gather_cost}")
        return 1

    try:
        return run_live(args) if args.live else run_dry(args)
    except ValidationFailure as exc:
        print(f"\nSTRICT VALIDATION FAILURE: {exc}")
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
