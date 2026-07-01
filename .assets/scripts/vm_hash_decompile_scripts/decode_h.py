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

# Let me find the exact bytes around pc 15380-15422
# Looking for the MAKE_FUNC and BUILD_ARRAY leading to h
r = F

# Scan around index 15380 in F (note F indices = bytecode positions, but we need array indices)
# The disasm showed pc=15396 as BUILD_ARRAY count=4 (push)
# That means F[15396] = 55, F[15397] = 4, F[15398] = 1

# Let me verify
print(f"F[15396] = {F[15396]} (expect 55=BUILD_ARRAY)")
print(f"F[15397] = {F[15397]} (expect 4=count)")

# The MAKE_FUNC for the mapper function of h - find it
# Scanning for the MAKE_FUNC before F[15396]
# Looking backwards from 15396 for a MAKE_FUNC (7)
search_range = range(15380, 15400)
for i in search_range:
    print(f"F[{i}] = {F[i]}")

print("...")
# The string bytes for h:
print(f"\nNums: V[241]={V[241]}, V[105]={V[105]}, V[240]={V[240]}, V[168]={V[168]}")

# Now find the MAKE_FUNC before this - go back
# The structure is: PUSH_CONST '' (43,11) PUSH_CONST 0 (43,12) MAKE_FUNC (7) entry push
# Then jump to after the function body
# Let's find MAKE_FUNC between 15370 and 15395
for i in range(15365, 15396):
    if F[i] == 7:
        print(f"MAKE_FUNC at F[{i}], entry={F[i+1]}, push={F[i+2]}")
        entry = F[i+1]
        # Check entry
        print(f"  Entry bytecode F[{entry}]={F[entry]}, F[{entry+1}]={F[entry+1]}")
        if F[entry] == 43 and F[entry+1] == 0:  # PUSH_CONST 'o'
            n = entry + 2
            if F[n] == 49:  # BIND_ARGS
                nargs = F[n+1]
                n += 2
                if F[n] == 33 and F[n+1] == 0:  # LOAD_VAR 'o'
                    n += 3
                    if F[n] == 43:  # PUSH_CONST KEY
                        key_idx = F[n+1]
                        print(f"  XOR key = V[{key_idx}] = {V[key_idx]}")
