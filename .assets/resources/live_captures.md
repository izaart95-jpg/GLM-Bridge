# Live captures & session material — captain-verified

Shared reference for all team tasks. Everything here was provided by the user in the
team brief and verified by the captain in Python (AES-128-CBC, PKCS7,
IV = UTF-8 bytes of `0123456789ABCDEF`, RES key `87f879f135f27da7`,
md5 secret `daye,raolewoba!`).

---

## 1. InitCaptchaV3 request (user's live fetch example)

```
POST https://no8xfe.captcha-open-southeast.aliyuncs.com/
Content-Type: application/x-www-form-urlencoded; charset=UTF-8

body:
AccessKeyId=LTAI5tSEBwYMwVKAQGpxmvTd&SignatureMethod=HMAC-SHA1&SignatureVersion=1.0&Format=JSON&Timestamp=2026-08-29T12%3A48%3A37Z&Version=2023-03-05&Action=InitCaptchaV3&SceneId=didk33e0&Language=en&Mode=popup&UpLang=true&DeviceData=TEQYvgJq1LrMqFaBybfIzPxz2ygFyAct7X%2Fw%2BLacfXWd9rGSwE%2Fx6ZCONucD1fehS2Qpig6tUVsFK111d9wIk5pWp6rwYjzFCRgL7pNp8bzGsvOSdUXgQTopQm90YPSdCiRAlgENdODLvY7P8jrfO9eC15tPCPwLxcRIrcspVvQYqVfk9%2FyFeIlePKmTRjkM&SignatureNonce=aae91cc7-685b-4193-887a-ca47f3ee1d00&Signature=Signature
```

Raw (unencoded) DeviceData param from that body:

```
TEQYvgJq1LrMqFaBybfIzPxz2ygFyAct7X/w+LacfXWd9rGSwE/x6ZCONucD1fehS2Qpig6tUVsFK111d9wIk5pWp6rwYjzFCRgL7pNp8bzGsvOSdUXgQTopQm90YPSdCiRAlgENdODLvY7P8jrfO9eC15tPCPwLxcRIrcspVvQYqVfk9/yFeIlePKmTRjkM
```

## 2. InitCaptchaV3 response (live)

```json
{
    "CertifyId": "QOpv38o7Qs",
    "Message": "success",
    "RequestId": "04DE4994-1E5A-4387-81FB-2F8020F81B56",
    "Code": "Success",
    "LimitFlow": false,
    "Success": true,
    "StaticPath": "3.29.0/pe.064.91720ad47545c881",
    "CaptchaType": "TRACELESS",
    "DeviceConfig": "wN32kvF2iZuLdXqR9SvIIDy5WLu2p9xzJPVdtSxOaTAAFuQhfAzge1b2KK6jxvBs8dGMNR++31iG+CCS6kMRf9JYA4zuU4B5PpOiPliAnU23UQw5FN+DGQQeOwJ48T3HJy9LrGS4uz2g49qaDhLhatXK3dUOKqahUgwpj5hsLPx3qWfVeSCA8hKNrk54gtBkCoox4qBXZMbYM7zfOIU3rIskHnDS6ODyK+wX03GIGTRXDivevyfMLIsU73fFeBPic2vVwcgAhn6YQHo3ZPF5OvxCp39IYwlpND1oJqCg4jKJaCAzAy7FjHP+jXxansE8"
}
```

## 3. DeviceConfig decrypted (captain-verified, RES key `87f879f135f27da7`)

AES-128-CBC decrypt of the §2 `DeviceConfig` blob, split on `#`:

| field | value |
|---|---|
| f[0] | `NjgyZWZkMTgxNDllMzlhNg==` → base64-decode → **sessionKey `682efd18149e39a6`** |
| f[1] | `MQ==` (= 1, switch) |
| f[2] | **sessionId** `3795d28242a11619bc25f786f84e53d4-h-1788007723938-9472a56a25dc49bc8d669b22c6db0142` |
| f[3] | version `1.5.1/feilin001.874f974c24cb17ca9480aadc03f6652a9f7e071628b484381d9efc0060379b50` |
| f[4] | `` |
| f[5] | `` |
| f[6] | `` |
| f[7] | serverTs `1788007723939` |
| f[8] | ip `106.219.217.208` |
| f[9] | `0` |

## 4. Live deviceTokens (both md5-verified; `w` decrypts with `682efd18149e39a6`)

### TOKEN1

```
U0dfV0VCIzM3OTVkMjgyNDJhMTE2MTliYzI1Zjc4NmY4NGU1M2Q0LWgtMTc4ODAwNzcyMzkzOC05NDcyYTU2YTI1ZGM0OWJjOGQ2NjliMjJjNmRiMDE0MiN6a1hnQTBlZEliVDR0RkN2Ly94VHlvTmVQRC9YbEZzbWNUZTRVOWI3bEtDK1gxbmFmeG93a1oxYVRrWHFIRHhWUnpKYzZvcnZaUlorYmxtbzZzQ29pdDh1VEx6eWR6MzlPSGFBelpoR216TlMwcS9xYkc4R2pRS1JTdDdjTTBianpIZGE2UE52U1A1TmZvVHlNWGVadmV3a2N0OFNYSEQyWHVVWHRxanQweXFkTWdBWWVvc2w5M0k4NkVNNUc2NTVjS0hlamJnVFdSQ1dvb2xjTWVnOTlleU9qeEl5TXBXbE50SHQ5SkMybDcxOW9FandkZlpIVWpNUEJuOW9sT2JyYnd5UVdwdG5lQkNYQVJCaGpEMGNLeDIxN1VRTDBocGZ0MVMweDRROXZxZWlSSVVVN2FEN2kyNksxVUJ0T3VocVR2OWdPMUE3dVdpYzg5R0YwK2s3YWh2RjJMRWFWOEhCZkxCZHNSVEtmaG0rZXg3MWJVQ0hHWjF1WGlGUSszeTdPd0dOd2JMcVAzQkRSZUNVcWNuMStOdTBXT1kreldlZlZ0NnhrVldNQ3A1ZzZVYU1lL3VHd0d6M3crN3FCcUdIbG1DVTdGdlJOMVZiM2V6VEFQWlJtSU1ZUllzd3h2YXhheGJlMlE5Skx3UFdlOEg4bGxiWkxXZExYTFlocDNKWER1SDhGbUpES3AzRXJtVm1URGVxNkZkMkIvSFcrc0cxNExDU0V2UnBObTVFQ1FWREFZd0xQejFPZ3R3a0lqTS9zSFZpeXJGclBiY0krZys4OGxZdWQ4V3ZFZk1lc1lSVWxWZHhyb25MOHo3OGtHVWVtK25Sc2VVcVY1ZDlONjVqL2dPWnNtNm51dSsvMWt2aFgrbzdvdndzL2tDQTc0OGc1WFpSMUJCOHliZGZCOHBpcmlMUkVHR2lQSjEzYkw3WUZjUGRGSlNMRE1GV2srYmdhbDNpNXlvcHUzTGV5Y3hHRGJ3em5aQ3VyWVlhYXNtVkx4TURDU1VwOWpFYVdTOXcxOXd2TWdPYlpadHVWQXZ6eERscyt2dE82QmJ4RkZxNzJ2QjluOXVxSGJWMnhURlk0NUJxay9hMDhTQWpTVkFOMkJzZ1lKRFNBbGlmZWV4UHNPS2tDY1ZTU05wWTU1UlVxK0ozTkl4S1duST0jNDA1OCNhZjQzNDI5YzY4NjkzZTlhN2I1ZjIzMmM1OThlZGVjNQ==
```

### TOKEN2

```
U0dfV0VCIzM3OTVkMjgyNDJhMTE2MTliYzI1Zjc4NmY4NGU1M2Q0LWgtMTc4ODAwNzcyMzkzOC05NDcyYTU2YTI1ZGM0OWJjOGQ2NjliMjJjNmRiMDE0MiN6a1hnQTBlZEliVDR0RkN2Ly94VHlvTmVQRC9YbEZzbWNUZTRVOWI3bEtDK1gxbmFmeG93a1oxYVRrWHFIRHhWUnpKYzZvcnZaUlorYmxtbzZzQ29pdDh1VEx6eWR6MzlPSGFBelpoR216TlMwcS9xYkc4R2pRS1JTdDdjTTBianpIZGE2UE52U1A1TmZvVHlNWGVadmV3a2N0OFNYSEQyWHVVWHRxanQweXFkTWdBWWVvc2w5M0k4NkVNNUc2NTVjS0hlamJnVFdSQ1dvb2xjTWVnOTlleU9qeEl5TXBXbE50SHQ5SkMybDcxOW9FandkZlpIVWpNUEJuOW9sT2JyYnd5UVdwdG5lQkNYQVJCaGpEMGNLeDIxN1VRTDBocGZ0MVMweDRROXZxZWlSSVVVN2FEN2kyNksxVUJ0T3VocU9xYXF2cm5KVGhDbC9oRGVPMEJ0YU52cWE1V3RTSlBKTms4S1M4ZTJIcEtnSWoydGlybnI0bVJSMGpheW9IaTJLVjVHakVxRmhpZlV4a09aS21rL3pRRkRhTGZTUEJJb2pjbVRmMEQ4SE5xNXpGa2FEZXVCbUxKR1BEVFEyMW9mZ1hRcld3MmxOYWRsUzFBazNWS0JCeFlwMyt5MmtiaUtyQVRaWU5sbm9SaDJza29hUnJmTlJVREl6UXZiQ3FhUTAyZnVJamtaWDhCeVVoRi9NOXZkaEhFR216MkkxY3FqRG5hTCt4NnJhUUpJbjVjWlBKMUU3cmRXam94ZnNmbGNHVFQwazcwZFk0cE11N0YwVVpacmV3bHRERDVXbTBLb24zVkNMTzdUV2pCSWdYbWRnS0VtcTNxR1BlZ05jWm5vc1Q1eUpJV0V0U2pNUVg1TEptUEE3L2tEcjUvVW4vYW5Ub3k0ZVZqS0NZTk1sRlJXUmMvc0NaWmhnSFFVWDBDaGExWFhiMWU3MCtQRW1xelFPRUsxcG9ybEcwbDNETWtmK0NHN2xad0NRU1BmUGZGK1Nsajd3V2N0WnBNL3B4c0h2RnBZbG9YVkxzN2xnSVpYcHEwUVRSNUcvcndTQ0FoM0JUaXdoeEpxdDM1RVNHczZSbGhtR2NPSll1TlBqNXE4VVVOM2l2SlFtNnpFdUJna1NTUUFvbkl1cjYyV3d5TVFGUkoxTnBzKzRmbz0jNDA1OCM1MTI4YzM0N2VjMTQ1MmQ5MjZjYjhlZTI1YjRlMDE2MQ==
```

### Token facts (captain-verified)

| field | TOKEN1 | TOKEN2 |
|---|---|---|
| tF | `SG_WEB` | `SG_WEB` |
| Q | `3795d28242a11619bc25f786f84e53d4-h-1788007723938-9472a56a25dc49bc8d669b22c6db0142` | same |
| w | 876-char b64, 656-byte ciphertext | 876-char b64, 656-byte ciphertext |
| tC | `4058` | `4058` |
| p | `af43429c68693e9a7b5f232c598edec5` | `5128c347ec1452d926cb8ee25b4e0161` |
| md5 check | PASS | PASS |

Plaintext inside `w` is **648 bytes after PKCS7-strip** (656 = ciphertext length; 139 `#`
separators → 140 fields).

Payload (decrypted `w`) differences TOKEN1→TOKEN2 — **the ONLY diffs are payload fields 43 and 74**; everything else is byte-identical (fields at index 93/94 are empty strings — the `93-…`/`94-…` values below are log ENTRIES inside field 43, not standalone fields):

- field 43 (logs, `probeId-cost` pairs joined with `|`): `…|81-13514|93-594050|94-594065` → `…|81-13514|93-620724|94-620733`
- field 74: `1788008311383` → `1788008338057`

Verified arithmetic (token-engineer, re-confirmed): **log-entry-93 cost = field74 − field72 (initTime)**:
`1788008311383 − 594050 = 1788007717333` (= field 72 of both tokens) and
`1788008338057 − 620724 = 1788007717333`. Log-entry-94 = entry-93 + 9–15 ms jitter.

Timing relations and invariants (validator-measured across both live + §21.4 reference sessions):

- **Q's embedded ts == serverTs − 1** (live: 1788007723938 = 1788007723939−1; ref: 1787994141656 = 1787994141657−1). Q and serverTs come from the same server response — treat them as linked, never independent.
- **f72 (initTime) < serverTs** by 4–7 s (live 6606 ms; ref 4126 ms). Replays must keep a realistic few-second gap.
- **f74 is unbounded vs serverTs in real usage** — ref: f74−serverTs = 1698 ms; LIVE: 587444 ms (~9.8 min; page left open between init and token mint). So f74 and log-93/94 (= f74−f72) can legitimately be huge; the "quick interaction" constraint is weak. Fresh synthetic tokens should still stay in the 1–10 s range to look normal.
- **f87 == serverTs exactly**; **f42 == DeviceConfig f[8] (ip) exactly**; **Q == DeviceConfig f[2] exactly** — hard replay invariants in both sessions.
- **tC (GatherCost) is per-SESSION constant**: 4058 in both live tokens (minted 26 s apart); §12.1: 524; §21.4: 3496. NOT a function of token-mint time, and NOT max(log costs) (live entry `81-13514` > tC 4058) — independent collector metric frozen at collection.
- w plaintext = **648 bytes + 8 bytes PKCS7** → 656-byte ciphertext; 139 `#` separators → 140 fields.

**DeviceData generator params for the live site** (decrypt of the §1 request's DeviceData param, KEY_HE→KEY_O): sceneId `didk33e0`, prefix `no8xfe`, region `sgp`, appKey `3795d28242a11619bc25f786f84e53d4` (NOT the `ab034e…` static default in generate_device_data.py — always pass appKey explicitly).

---

## Appendix — captain's decrypt of TOKEN1 `w` (for cross-checking your own work)

Plaintext (140 `#`-fields, 656 bytes → 625 visible after trim; regenerate yourself with
`AES-128-CBC(key=b'682efd18149e39a6', iv=b'0123456789ABCDEF')` over `base64.b64decode(w)`):

```
W.10054#####Linux armv81#Chrome#149.0.0.0#############8#E2FFHTUwY2c=#4##########50a98af28ee10a81ef6f31efaae2853b##8##Linux#x86_64#####106.219.217.208#10-0|20-2847|11-3546|23-7686|30-7697|40-7759|41-12134|70-12139|71-13504|80-13504|81-13514|93-594050|94-594065#true###768*1366##5##################saf-captcha#0###4SyHGkKVW8fUJZTYIWWLKAoWeJmau7PQKrOjC8GP#1788007717333#O9l1RIX7GMGG35xDAGAL5y9Zq9vHKgI8LAOBLElIU7#1788008311383#desktop#false##9d4568c009d203ab10e33ea9953a0264#########1788007723939#MCMwIzAjMCMwIzAjMCMwIzAjMSMwIzAjMCMwIzAjMCMwIzAjMCMxIzEjMCMxMTExMTExMDExMTExMTExMTExMTExMTExMQ==#1#1#true##################0##############################
```

(§21.4 of Fielin-report.md holds a second, older reference session — key `57ad9f73260d1d46`, sessionId `…-h-1787994141656-52b528da…`, ip `223.188.28.68` — useful as a third data point for cross-session field comparison.)
