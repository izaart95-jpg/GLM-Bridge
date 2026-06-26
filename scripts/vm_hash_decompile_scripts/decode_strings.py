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

# The pattern for string building:
# 1. PUSH_CONST '' , PUSH_CONST 0, MAKE_FUNC <varname_idx> <entry> 0/1
#    The function becomes the mapper: fn(o) { return String.fromCharCode(o ^ KEY) }
# 2. After the function entry, there's a JUMP to skip the function body
# 3. At the array building point:
#    PUSH_CONST num1, PUSH_CONST num2, ..., BUILD_ARRAY n
#    PUSH_CONST 'map', CALL_METHOD 1
#    PUSH_CONST 'join', CALL_METHOD 1
#    PUSH_SCOPE, PUSH_CONST varname, SET_PROP

# Let's actually simulate the early part to decode all strings
# We need to find the pattern: BUILD_ARRAY followed by map/join

n = 0
r = F
decoded = {}
current_key = None

def get_xor_key_at(entry_pc):
    """Get the XOR key from a function entry point"""
    n = entry_pc
    # Skip PUSH_CONST 'o' (43, 0)
    n += 2
    # Skip BIND_ARGS (49, nargs)  -- wait BIND_ARGS is op 49
    if r[n] == 49:
        n += 2
    # Now: LOAD_VAR 'o' (33, 0, 1)
    if r[n] == 33 and r[n+1] == 0:
        n += 3
        # PUSH_CONST KEY (43, idx)
        if r[n] == 43:
            key_idx = r[n+1]
            return V[key_idx]
    return None

# Track MAKE_FUNC calls and their associated data
n = 0
func_map = {}  # entry_pc -> (name_var_idx, name)
while n < len(r):
    pc = n
    A = r[n]; n += 1
    if A == 7:  # MAKE_FUNC
        # f = e.pop() (name from stack - was PUSH_CONST var_idx before)
        # Actually looking at opcode 7:
        # f = e.pop()  <- the name (var_idx from V? no, actual var name string)
        # l = r[n++]   <- entry pc
        # h = r[n++]   <- push/store flag
        name_var_entry = r[n]; n += 1  # this is the entry_pc for the function
        push_flag = r[n]; n += 1
        # The name came from the stack (was pushed as PUSH_CONST before this)
        func_map[name_var_entry] = {'push': push_flag}
    else:
        # skip operands for known ops
        ops_with_1_operand = {0,2,3,5,6,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,
                               23,24,25,26,27,28,29,30,31,32,33,35,36,37,38,40,41,
                               42,43,44,45,46,47,48,49,50,51,52,53,54,55,56}
        # actually each op has different operand counts... let me just use the disasm
        pass

# Better approach: actually simulate the VM to decode strings
# The first section (before the main logic) just builds a lookup table
# Let's run a simplified simulation

class Stack:
    def __init__(self):
        self.data = []
    def push(self, v):
        self.data.append(v)
    def pop(self):
        return self.data.pop()
    def peek(self):
        return self.data[-1]

# Simulate the initial string building section
# The scope/variable map
scope = {'_': None}  # window equivalent
args = [None, None]  # er (input), td

r_arr = F
n = 0
stack = Stack()
vars_map = {}  # local variables by name
func_table = {}  # entry_pc -> xor_key

# First pass: collect all XOR functions
n = 0
while n < len(r_arr) - 2:
    pc = n
    A = r_arr[n]; n += 1
    if A == 49:  # BIND_ARGS
        nargs = r_arr[n]; n += 1
        # Check if next is LOAD_VAR 'o' (33, 0, 1) then PUSH_CONST key (43, k)
        if (n + 5 < len(r_arr) and
            r_arr[n] == 33 and r_arr[n+1] == 0 and r_arr[n+2] == 1 and
            r_arr[n+3] == 43):
            key_idx = r_arr[n+4]
            key_val = V[key_idx] if key_idx < len(V) else None
            func_table[pc-1] = key_val  # entry point is pc-1 (where PUSH_CONST 'o' was)

print(f"Found {len(func_table)} XOR decode functions")

# Now let's extract all the strings by finding the BUILD_ARRAY -> map -> join patterns
# and matching them with function entry points

# Approach: scan for MAKE_FUNC instructions, get their entry PCs
# Then find the BUILD_ARRAY + map(fn) + join('') + SET_PROP patterns after them

n = 0
make_func_positions = []
while n < len(r_arr) - 3:
    pc = n
    A = r_arr[n]; n += 1
    if A == 7:  # MAKE_FUNC
        # Before this: PUSH_CONST '' (43, 11), PUSH_CONST 0 (43, 12)
        entry_pc_in_bytecode = r_arr[n]; n += 1
        push_flag = r_arr[n]; n += 1
        make_func_positions.append((pc, entry_pc_in_bytecode, push_flag))

print(f"Found {len(make_func_positions)} MAKE_FUNC calls")

# For each MAKE_FUNC, find its XOR key
for call_pc, entry_pc, push_flag in make_func_positions[:20]:
    # The actual entry of the function bytecode
    # Looking at the pattern: 
    # At call_pc-4: PUSH_CONST '' (43, 11)
    # At call_pc-2: PUSH_CONST 0 (43, 12)
    # At call_pc: MAKE_FUNC 7, entry_pc, push_flag
    
    # The function body: starts at entry_pc with PUSH_CONST 'o', BIND_ARGS 0
    # then LOAD_VAR 'o', PUSH_CONST KEY, XOR, String.fromCharCode, RETURN
    ep = entry_pc  # actual bytecode index
    if ep < len(r_arr):
        # Expected: 43 0 (PUSH_CONST 'o'), then 49 nargs (BIND_ARGS)
        if r_arr[ep] == 43 and r_arr[ep+1] == 0 and r_arr[ep+2] == 49:
            ep2 = ep + 4  # skip PUSH_CONST 'o', BIND_ARGS nargs
            # Now: LOAD_VAR 'o' (33, 0, 1)
            if r_arr[ep2] == 33 and r_arr[ep2+1] == 0 and r_arr[ep2+2] == 1:
                ep2 += 3
                # PUSH_CONST KEY
                if r_arr[ep2] == 43:
                    key_idx = r_arr[ep2+1]
                    key = V[key_idx]
                    print(f"  MAKE_FUNC at {call_pc}: entry={entry_pc}, XOR key={key}")
