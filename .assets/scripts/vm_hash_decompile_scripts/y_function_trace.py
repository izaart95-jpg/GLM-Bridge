"""
Assuming the context came from function call P(0,[],F.V,ez,[input,salt])
"""

import json

with open('F_bytecode.txt') as f:
    F = json.load(f)

V = [
    "o","r","a","m","C","e","f","i","p","h","d","",0,147,"n","fromCharCode",
    252,241,249,246,240,231,"map","join",202,172,191,164,169,190,163,165,
    87,56,53,61,50,52,35,156,250,233,242,255,232,245,243,108,24,3,63,30,
    5,2,11,206,167,160,170,171,182,129,168,124,12,14,19,8,25,33,84,79,69,
    68,71,72,"_",55,64,94,89,83,88,1,22,97,127,120,114,121,46,117,65,76,
    75,77,90,74,115,54,95,82,110,29,36,166,152,159,149,158,134,78,66,85,
    119,20,26,18,44,67,70,73,104,113,162,205,192,200,199,193,214,130,234,
    239,238,230,215,207,204,151,211,248,244,226,227,181,146,132,148,133,
    91,93,86,201,253,247,178,221,150,153,136,220,184,137,145,161,157,131,
    179,154,142,135,28,125,106,123,155,128,32,45,37,42,59,111,57,38,40,
    216,185,174,177,183,141,195,236,251,228,212,210,218,173,188,138,139,
    143,144,175,187,180,4,7,34,194,223,229,15,49,58,96,10,101,16,41,107,
    48,9,31,27,21,6,109,140,224,225,118,122,196,197,203,186,189,176,13,17,
    43,99,23,47,103,112,237,80,105,102,98,213,126,81,100,92,116,51,62,219,
    209,254,208,39,222,198,217,235,False,60,"Boolean","Number","String",
    "j","t","u","arguments","s","random",256,"floor","push","length","0",
    "toString","y","charCodeAt"
]

# Let's trace the bytecodes from 15982 onward (the main y function)
# Entry is at 15989 (from F[15986]=15989)
print("F[15982:16020]:")
for i in range(15982, 16020):
    print(f"  F[{i}] = {F[i]}", end="")
    if F[i] == 43 and i+1 < len(F):
        vi = F[i+1]
        if vi < len(V):
            print(f"  (PUSH_CONST V[{vi}]={repr(V[vi])})", end="")
    print()

print("\n\nTracing y function body (PC 15989+):")
# From the disasm we saw:
# 15989: PUSH_CONST V[0]='o', PUSH_CONST V[1]='r', BIND_ARGS 1
# Then: VAR_DECL o, r, a, m, C, e, f, i (local scope vars)
# Then: ...

# Let me decode the exact sequence at 15989
for i in range(15989, 16075):
    print(f"  F[{i}] = {F[i]}", end="")
    if F[i] in [43, 33, 21, 27, 56]:
        vi = F[i+1] if i+1 < len(F) else -1
        if vi < len(V) and vi >= 0:
            print(f"  -> V[{vi}]={repr(V[vi])}", end="")
    print()
