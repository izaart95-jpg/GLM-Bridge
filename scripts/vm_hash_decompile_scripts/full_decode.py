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

r = F

# Build function entry -> xor_key map
func_xor = {}
n = 0
while n < len(r) - 5:
    pc = n
    A = r[n]; n += 1
    if A == 7:  # MAKE_FUNC: pop name, read entry_pc, push_flag
        entry_pc = r[n]; n += 1
        push_flag = r[n]; n += 1
        # Resolve xor key from entry
        ep = entry_pc
        if (ep + 6 < len(r) and r[ep] == 43 and r[ep+1] == 0 and 
            r[ep+2] == 49 and r[ep+4] == 33 and r[ep+5] == 0 and
            r[ep+6] == 1 and r[ep+7] == 43):
            key_idx = r[ep+8]
            func_xor[entry_pc] = V[key_idx]

# Now the actual string-building pattern:
# ... PUSH_CONST num1, ..., PUSH_CONST numN, BUILD_ARRAY count (55, count, push)
# PUSH_CONST 'map' (43, 22), CALL_METHOD 1 1 (50, 1, 1)
# PUSH_CONST 'join' (43, 23), CALL_METHOD 1 1
# [optional: PUSH_SCOPE (30), PUSH_CONST varname (43, idx), SET_PROP (27, push)]

# But the function reference comes from the MAKE_FUNC which was stored in scope
# via: MAKE_FUNC at position -> stored in c[varname] or pushed to stack
# The mapper fn was stored using MAKE_FUNC with push=0 (assign to scope var named from pop)
# Let's look: PUSH_CONST '' -> PUSH_CONST 0 -> MAKE_FUNC entry push
# push=1 means push to stack, push=0 means assign to scope[name]
# Wait, looking at the code: h ? e.push(p) : c[f] = p
# where f = e.pop() (name string), l = r[n++] (entry), h = r[n++] (push flag)

# The name 'f' comes from e.pop() BEFORE reading entry and push from bytecode
# So the sequence is:
# PUSH_CONST (name_string)   <- pushed to stack as name
# PUSH_CONST '' (empty? or delimiter)
# PUSH_CONST 0  
# 7 (MAKE_FUNC) l=entry h=push_flag
# If h=0: c[name] = fn (scope assignment), name comes from e.pop() but wait:
# Actually reading more carefully:
# 7 == A: f = e.pop() (the name), l = r[n++] (entry), h = r[n++] (push or not)
# So just ONE item popped from stack

# Let me re-read opcode 7:
# 7 == A ? (f = e.pop(), l = r[n++], h = r[n++], p = function(n){...}(l), h ? e.push(p) : c[f] = p)
# f = name from stack, l = entry PC, h = push flag
# So BEFORE the MAKE_FUNC, exactly ONE push happens to provide the name

# Going back, the pattern is:
# PUSH_CONST <name_var_V_idx>  -> pushes V[idx] which is the variable name string
# 7 (MAKE_FUNC) entry_pc push_flag

# Let's now trace the whole string initialization section
# by finding: PUSH_CONST name, MAKE_FUNC entry 0  -> scope assignment
# then: [nums] BUILD_ARRAY, PUSH_CONST 'map', CALL_METHOD 1, PUSH_CONST 'join', CALL_METHOD 1

# Actually a simpler approach: just simulate the VM for the first pass
# The key insight is that this VM builds a set of strings from XOR-decoded char arrays
# and assigns them to scope variables (o, r, a, m, C, e, f, i, p, h, d)

# These variable names are the single-letter vars declared at the top!
# V[0]='o', V[1]='r', V[2]='a', V[3]='m', V[4]='C', V[5]='e', V[6]='f', V[7]='i', V[8]='p', V[9]='h', V[10]='d'

# The function names pushed to stack to name them are these indices
# Let's find all strings that get assigned to these vars

# Strategy: scan for BUILD_ARRAY followed by map/join and then look at what MAKE_FUNC 
# (with 0 push flag) was defined right before it, and what XOR key it uses

def decode_chars(nums, xor_key):
    return ''.join(chr(n ^ xor_key) for n in nums)

# Let me scan more carefully
# Find: MAKE_FUNC entry=EP push=0 (store in scope)
# Then immediately after: [PUSH_CONST nums] BUILD_ARRAY count CALL_METHOD 'map' CALL_METHOD 'join'
# PUSH_SCOPE, PUSH_CONST varname, SET_PROP

strings_decoded = []
n = 0
# Track the most recent MAKE_FUNC entry
last_make_func_entry = None
last_name_pushed = None

# Quick pass: find all PUSH_CONST -> MAKE_FUNC sequences
make_funcs = []  # (pc, name_V_idx, entry_pc, push_flag)
n = 0
while n < len(r) - 3:
    pc = n
    A = r[n]; n += 1
    if A == 43:  # PUSH_CONST
        idx = r[n]; n += 1
        # Check if next is MAKE_FUNC (7)
        if r[n] == 7:
            entry_pc = r[n+1]
            push_flag = r[n+2]
            make_funcs.append((pc, idx, entry_pc, push_flag))

print(f"Found {len(make_funcs)} PUSH_CONST->MAKE_FUNC sequences")

# For each MAKE_FUNC with push=0 (store in scope), the name is V[idx]
# and the function XOR-decodes with the key from its entry
for mf_pc, name_idx, entry_pc, push_flag in make_funcs[:30]:
    name = V[name_idx]
    xor_key = func_xor.get(entry_pc, '?')
    stored = "scope" if not push_flag else "stack"
    print(f"  pc={mf_pc}: name=V[{name_idx}]={repr(name)}, entry={entry_pc}, xor={xor_key}, stored={stored}")
