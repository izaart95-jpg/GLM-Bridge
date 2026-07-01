"""
Bytecode may differ for the interpreter based on function P call this disassm assumes you have 
bytecode F from function call P(0,[],F.V,ez,[input,salt])
"""


import json, sys

# Load F and V
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

# Opcode names
OPC = {
    0: "SHR_UNSIGNED",   # >>>
    2: "SHL",            # <<
    3: "DEC_PROP",       # --prop
    4: "NOP",
    5: "BIT_NOT",        # ~
    6: "CALL_IF",        # call sub if result not undefined
    7: "MAKE_FUNC",
    8: "INC_PROP",       # ++prop
    9: "LT",             # <
    10: "LTE",           # <=
    11: "RETURN",
    12: "GTE",           # >=
    13: "DUP_TOP",
    14: "JUMP_TABLE",
    15: "EQ_STRICT",     # ===
    16: "BUILD_STRING",
    17: "MOD",           # %
    18: "ADD",           # +
    19: "THROW",
    20: "AND",           # &
    21: "VAR_DECL",      # declare var
    22: "SUB",           # -
    23: "XOR",           # ^
    24: "GT",            # >
    25: "RETURN_VAL",
    26: "MUL",           # *
    27: "SET_PROP",
    28: "JUMP_IF_FALSE",
    29: "EQ_LOOSE",      # ==
    30: "PUSH_SCOPE",
    31: "JUMP",
    32: "JUMP_IF_TRUE",
    33: "LOAD_VAR",
    34: "PUSH_WINDOW",
    35: "TYPEOF",
    36: "NEQ_STRICT",    # !==
    37: "CALL_WHILE",    # call sub, loop while return undefined
    38: "OR",            # |
    39: "TRY",
    40: "DIV",           # /
    41: "NEW",
    42: "IN",            # in
    43: "PUSH_CONST",    # push V[r[n]]
    44: "PUSH_OBJ",
    45: "DELETE_PROP",
    46: "INSTANCEOF",
    47: "SET_INDEX",
    48: "SHR",           # >>
    49: "BIND_ARGS",     # bind arguments from stack
    50: "CALL_METHOD",
    51: "NEQ_LOOSE",     # !=
    52: "NOT",           # !
    53: "POP_MANY",
    54: "PUSH_UNDEF",
    55: "BUILD_ARRAY",
    56: "GET_PROP",      # obj[key]
}

r = F
n = 0
lines = []
max_pc = len(r)

while n < max_pc:
    pc = n
    if n >= len(r):
        break
    A = r[n]; n += 1

    if A == 43:   # PUSH_CONST
        idx = r[n]; n += 1
        val = V[idx] if idx < len(V) else f"V[{idx}]"
        lines.append(f"{pc:5d}: PUSH_CONST V[{idx}] = {repr(val)}")
    elif A == 21:  # VAR_DECL
        idx = r[n]; n += 1
        val = V[idx] if idx < len(V) else f"V[{idx}]"
        lines.append(f"{pc:5d}: VAR_DECL {repr(val)} = undefined")
    elif A == 33:  # LOAD_VAR
        idx = r[n]; n += 1; push = r[n]; n += 1
        val = V[idx] if idx < len(V) else f"V[{idx}]"
        lines.append(f"{pc:5d}: LOAD_VAR {repr(val)} {'(push)' if push else '(no push)'}")
    elif A == 27:  # SET_PROP
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: SET_PROP {'(push)' if push else ''}")
    elif A == 56:  # GET_PROP
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: GET_PROP {'(push)' if push else ''}")
    elif A == 50:  # CALL_METHOD
        nargs = r[n]; n += 1; push = r[n]; n += 1
        lines.append(f"{pc:5d}: CALL_METHOD nargs={nargs} {'(push)' if push else ''}")
    elif A == 55:  # BUILD_ARRAY
        count = r[n]; n += 1; push = r[n]; n += 1
        lines.append(f"{pc:5d}: BUILD_ARRAY count={count} {'(push)' if push else ''}")
    elif A == 16:  # BUILD_STRING
        count = r[n]; n += 1; push = r[n]; n += 1
        lines.append(f"{pc:5d}: BUILD_STRING count={count} {'(push)' if push else ''}")
    elif A == 7:   # MAKE_FUNC
        nname = r[n]; n += 1  # name var idx
        entry = r[n]; n += 1  # entry point
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: MAKE_FUNC name_var={nname} entry={entry} {'(push)' if push else '(assign to name)'}")
    elif A == 49:  # BIND_ARGS
        nargs = r[n]; n += 1
        lines.append(f"{pc:5d}: BIND_ARGS nargs={nargs}")
    elif A == 31:  # JUMP
        target = r[n]; n += 1
        lines.append(f"{pc:5d}: JUMP -> {target}")
    elif A == 28:  # JUMP_IF_FALSE
        target = r[n]; n += 1
        lines.append(f"{pc:5d}: JUMP_IF_FALSE -> {target}")
    elif A == 32:  # JUMP_IF_TRUE
        target = r[n]; n += 1
        lines.append(f"{pc:5d}: JUMP_IF_TRUE -> {target}")
    elif A == 25:  # RETURN_VAL
        val = r[n]; n += 1
        lines.append(f"{pc:5d}: RETURN_VAL (inline={val})")
    elif A == 18:  # ADD
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: ADD {'(push)' if push else ''}")
    elif A == 22:  # SUB
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: SUB {'(push)' if push else ''}")
    elif A == 26:  # MUL
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: MUL {'(push)' if push else ''}")
    elif A == 23:  # XOR
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: XOR {'(push)' if push else ''}")
    elif A == 20:  # AND
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: AND {'(push)' if push else ''}")
    elif A == 38:  # OR
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: OR {'(push)' if push else ''}")
    elif A == 2:   # SHL
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: SHL {'(push)' if push else ''}")
    elif A == 48:  # SHR
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: SHR {'(push)' if push else ''}")
    elif A == 0:   # SHR_UNSIGNED
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: SHR_UNSIGNED {'(push)' if push else ''}")
    elif A == 17:  # MOD
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: MOD {'(push)' if push else ''}")
    elif A == 40:  # DIV
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: DIV {'(push)' if push else ''}")
    elif A == 15:  # EQ_STRICT ===
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: EQ_STRICT {'(push)' if push else ''}")
    elif A == 36:  # NEQ_STRICT !==
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: NEQ_STRICT {'(push)' if push else ''}")
    elif A == 9:   # LT <
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: LT {'(push)' if push else ''}")
    elif A == 12:  # GTE >=
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: GTE {'(push)' if push else ''}")
    elif A == 24:  # GT >
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: GT {'(push)' if push else ''}")
    elif A == 11:  # RETURN (no value)
        lines.append(f"{pc:5d}: RETURN")
    elif A == 30:  # PUSH_SCOPE
        lines.append(f"{pc:5d}: PUSH_SCOPE (push current scope c)")
    elif A == 34:  # PUSH_WINDOW
        lines.append(f"{pc:5d}: PUSH_WINDOW")
    elif A == 44:  # PUSH_OBJ
        lines.append(f"{pc:5d}: PUSH_OBJ {{}}")
    elif A == 52:  # NOT
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: NOT {'(push)' if push else ''}")
    elif A == 35:  # TYPEOF
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: TYPEOF {'(push)' if push else ''}")
    elif A == 13:  # DUP_TOP
        lines.append(f"{pc:5d}: DUP_TOP")
    elif A == 1:   # POP
        lines.append(f"{pc:5d}: POP")
    elif A == 4:   # NOP
        lines.append(f"{pc:5d}: NOP")
    elif A == 53:  # POP_MANY
        nargs = r[n]; n += 1; push = r[n]; n += 1
        lines.append(f"{pc:5d}: POP_MANY n={nargs} {'(push top)' if push else ''}")
    elif A == 54:  # PUSH_UNDEF
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: PUSH_UNDEF {'(push)' if push else ''}")
    elif A == 19:  # THROW
        lines.append(f"{pc:5d}: THROW")
    elif A == 41:  # NEW
        nargs = r[n]; n += 1; push = r[n]; n += 1
        lines.append(f"{pc:5d}: NEW nargs={nargs} {'(push)' if push else ''}")
    elif A == 42:  # IN
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: IN {'(push)' if push else ''}")
    elif A == 46:  # INSTANCEOF
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: INSTANCEOF {'(push)' if push else ''}")
    elif A == 45:  # DELETE_PROP
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: DELETE_PROP {'(push)' if push else ''}")
    elif A == 47:  # SET_INDEX
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: SET_INDEX {'(push)' if push else ''}")
    elif A == 5:   # BIT_NOT
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: BIT_NOT {'(push)' if push else ''}")
    elif A == 8:   # INC_PROP
        pre = r[n]; n += 1; push = r[n]; n += 1
        lines.append(f"{pc:5d}: {'PRE' if pre else 'POST'}_INC_PROP {'(push)' if push else ''}")
    elif A == 3:   # DEC_PROP
        pre = r[n]; n += 1; push = r[n]; n += 1
        lines.append(f"{pc:5d}: {'PRE' if pre else 'POST'}_DEC_PROP {'(push)' if push else ''}")
    elif A == 29:  # EQ_LOOSE
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: EQ_LOOSE {'(push)' if push else ''}")
    elif A == 51:  # NEQ_LOOSE
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: NEQ_LOOSE {'(push)' if push else ''}")
    elif A == 10:  # LTE
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: LTE {'(push)' if push else ''}")
    elif A == 37:  # CALL_WHILE
        target = r[n]; n += 1
        lines.append(f"{pc:5d}: CALL_WHILE target={target}")
    elif A == 6:   # CALL_IF
        target = r[n]; n += 1
        lines.append(f"{pc:5d}: CALL_IF target={target}")
    elif A == 39:  # TRY
        try_pc = r[n]; n += 1
        catch_pc = r[n]; n += 1
        finally_pc = r[n]; n += 1
        after_pc = r[n]; n += 1
        lines.append(f"{pc:5d}: TRY try={try_pc} catch={catch_pc} finally={finally_pc} after={after_pc}")
    elif A == 14:  # JUMP_TABLE
        push = r[n]; n += 1
        lines.append(f"{pc:5d}: JUMP_TABLE push={push}")
    else:
        lines.append(f"{pc:5d}: UNKNOWN_OP {A}")

with open('disasm.txt', 'w') as f:
    f.write('\n'.join(lines))
print(f"Disassembled {len(lines)} instructions, {len(F)} bytecodes")
print("All instructions:")
for l in lines:
    print(l)
