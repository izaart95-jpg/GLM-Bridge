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

def decode_chars(nums, xor_key):
    try:
        return ''.join(chr(int(n) ^ int(xor_key)) for n in nums)
    except:
        return f"ERROR({nums}, {xor_key})"

# Pattern scan:
# PUSH_CONST 0 (V[12]=0) at pc X
# MAKE_FUNC (7) at X+2: entry_pc EP, push 1
# Then: [PUSH_CONST nums] ... BUILD_ARRAY(55) count push
# PUSH_CONST 'map'(V[22]), CALL_METHOD 1 1
# PUSH_CONST 'join'(V[23]), CALL_METHOD 1 1
# Then: PUSH_SCOPE(30), PUSH_CONST varname(43,idx), SET_PROP(27,push)

# Let's scan for BUILD_ARRAY followed by map+join pattern
n = 0
results = []

while n < len(r) - 20:
    pc = n
    A = r[n]

    if A == 55:  # BUILD_ARRAY
        count = r[n+1]
        push = r[n+2]

        # Look back to collect the pushed constants
        # Actually let's look forward for map+join
        after = n + 3
        if (after + 10 < len(r) and
            r[after] == 43 and r[after+1] == 22 and  # PUSH_CONST 'map'
            r[after+2] == 50 and r[after+3] == 1 and r[after+4] == 1 and  # CALL_METHOD 1 1
            r[after+5] == 43 and r[after+6] == 23 and  # PUSH_CONST 'join'
            r[after+7] == 50 and r[after+8] == 1 and r[after+9] == 1):  # CALL_METHOD 1 1

            # Found a .map(fn).join('') pattern
            # Look back 'count*2' positions (each PUSH_CONST = 2 bytecodes)
            start_of_nums = n - count * 2
            if start_of_nums >= 0:
                nums = []
                valid = True
                for i in range(count):
                    bc_pos = start_of_nums + i * 2
                    if r[bc_pos] != 43:  # should all be PUSH_CONST
                        valid = False
                        break
                    nums.append(V[r[bc_pos + 1]])

                if valid:
                    # Now find what XOR function was used
                    # It was set up by a MAKE_FUNC before these pushes
                    # The MAKE_FUNC stores fn in scope[''] (since name is V[12]=0 which is int 0)
                    # Actually looking again: name=V[12]=0 (integer 0) -> c[0] = fn
                    # That's a bit odd. Let me check the actual SET_PROP pattern after join

                    # What scope variable is this assigned to?
                    after2 = after + 10
                    assign_var = None
                    if (after2 + 3 < len(r) and
                        r[after2] == 30 and  # PUSH_SCOPE
                        r[after2+1] == 43 and  # PUSH_CONST
                        r[after2+3] == 27):  # SET_PROP
                        var_idx = r[after2+2]
                        assign_var = V[var_idx] if var_idx < len(V) else f"V[{var_idx}]"

                    results.append({
                        'pc': pc,
                        'count': count,
                        'nums': nums,
                        'assign_var': assign_var,
                    })
    n += 1

# For each BUILD_ARRAY result, find which MAKE_FUNC (XOR function) was last defined
# The MAKE_FUNC creates a fn and stores it in c[0] (since name=integer 0)
# Then it's used as the .map() callback

# Find all MAKE_FUNCs and their positions
func_map = {}  # position -> (entry_pc, push_flag)
n = 0
while n < len(r) - 2:
    A = r[n]
    if A == 7:  # MAKE_FUNC
        entry_pc = r[n+1]
        push_flag = r[n+2]
        func_map[n] = (entry_pc, push_flag)
    n += 1

# Get XOR key from entry point
def get_xor_key(entry_pc):
    ep = entry_pc
    if ep + 8 < len(r):
        # 43 0 (PUSH_CONST 'o'), 49 nargs (BIND_ARGS), 33 0 1 (LOAD_VAR 'o'), 43 k (PUSH_CONST key)
        if r[ep] == 43 and r[ep+1] == 0 and r[ep+2] == 49 and r[ep+4] == 33:
            key_pc = ep + 7
            if r[key_pc] == 43:
                return V[r[key_pc+1]]
    return None

# For each result, find the nearest preceding MAKE_FUNC (with push=1, stored on stack)
make_func_positions = sorted(func_map.keys())

for res in results:
    pc = res['pc']
    # Find nearest MAKE_FUNC before the PUSH_CONSTs (before pc - count*2)
    search_before = pc - res['count'] * 2
    preceding = [p for p in make_func_positions if p < search_before]
    if preceding:
        mf_pc = max(preceding)
        entry_pc, push_flag = func_map[mf_pc]
        xor_key = get_xor_key(entry_pc)
        res['xor_key'] = xor_key
        res['mf_pc'] = mf_pc
        res['entry_pc'] = entry_pc
        if xor_key is not None:
            try:
                decoded = decode_chars(res['nums'], xor_key)
                res['decoded'] = decoded
            except:
                res['decoded'] = '?'
        else:
            res['decoded'] = f'no_key(entry={entry_pc})'
    else:
        res['xor_key'] = None
        res['decoded'] = 'no_preceding_func'

# Print results
print(f"Found {len(results)} string-building sequences\n")
for i, res in enumerate(results):
    varname = res.get('assign_var', 'unknown')
    decoded = res.get('decoded', '?')
    xor_key = res.get('xor_key', '?')
    nums_str = str(res['nums'][:5]) + ('...' if len(res['nums']) > 5 else '')
    print(f"[{i:3d}] pc={res['pc']:6d} var={repr(varname):20s} xor={str(xor_key):5s} len={res['count']:3d} -> {repr(decoded)}")
