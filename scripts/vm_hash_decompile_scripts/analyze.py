"""
Bytecode may differ for the interpreter based on function P call this disassm assumes you have 
bytecode F from function call P(0,[],F.V,ez,[input,salt])
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

# The key pattern: MAKE_FUNC with an XOR mapper
# Let's trace the string decoding pattern
# Pattern is: [nums] BUILD_ARRAY .map(fn) .join('') -> variable name

# Let's decode all the early string-building sections
r = F
n = 0
strings = {}
func_entries = []

# Each section looks like:
# PUSH_CONST '' PUSH_CONST 0 MAKE_FUNC <name_idx> <entry_pc> (store) JUMP <after_xor_section>
# <entry_pc>: PUSH_CONST 'o', BIND_ARGS 0 -> fn(o) { return String.fromCharCode(o ^ KEY) }
# <nums> BUILD_ARRAY .map(fn) .join('') -> SCOPE[varname] = result

# Let's manually trace the XOR decode functions
# Each fn at entry X does: n XOR K -> fromCharCode

def trace_xor_fn(entry_pc):
    """Find the XOR key used in a mapper function at entry_pc"""
    n = entry_pc
    # PUSH_CONST 'o', BIND_ARGS 0
    n += 2  # PUSH_CONST V[0]='o', next
    n += 2  # BIND_ARGS -1... actually let me just check
    # The pattern from disasm:
    # entry: PUSH_CONST V[0]='o'  BIND_ARGS 0
    #        LOAD_VAR 'o' (push)
    #        PUSH_CONST V[K]
    #        XOR (push)
    #        PUSH_SCOPE, PUSH_CONST 'n', GET_PROP, PUSH_CONST 'fromCharCode', CALL_METHOD 1, RETURN_VAL 2
    # So entry_pc + 0 = opcode PUSH_CONST (43), entry_pc+1 = 0 ('o')
    # entry_pc + 2 = BIND_ARGS (49), entry_pc+3 = -1 or 0
    # But wait BIND_ARGS is at the CALL site, not entry
    # Let me re-read
    pass

# Actually let's just look at what strings get assigned
# The pattern is clear: XOR with a key, then String.fromCharCode
# Keys appear at offsets like: LOAD_VAR 'o', PUSH_CONST KEY, XOR

# Let's trace manually - find all XOR keys
print("=== XOR Decode Functions ===")
n = 0
while n < len(F):
    pc = n
    A = F[n]; n += 1
    if A == 49:  # BIND_ARGS - entry point marker
        nargs = F[n]; n += 1
        # Next should be: LOAD_VAR 'o' (33, 0, 1)
        if n < len(F) and F[n] == 33 and F[n+1] == 0:
            # This is one of our XOR functions
            n2 = n + 3  # skip LOAD_VAR 'o' push
            if F[n2] == 43:  # PUSH_CONST
                key_idx = F[n2+1]
                key_val = V[key_idx]
                print(f"  fn entry before pc={pc-2}: BIND_ARGS {nargs}, XOR key = V[{key_idx}] = {key_val}")
