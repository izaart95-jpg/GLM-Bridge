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

r = F
MAX = len(F)

def opname(A):
    return {0:'SHR_U',2:'SHL',3:'DEC_PROP',4:'NOP',5:'BIT_NOT',6:'CALL_IF',
            7:'MAKE_FUNC',8:'INC_PROP',9:'LT',10:'LTE',11:'RETURN',
            12:'GTE',13:'DUP_TOP',14:'JMP_TBL',15:'EQ_STRICT',16:'BUILD_STR',
            17:'MOD',18:'ADD',19:'THROW',20:'AND',21:'VAR_DECL',22:'SUB',
            23:'XOR',24:'GT',25:'RET_VAL',26:'MUL',27:'SET_PROP',28:'JIF_F',
            29:'EQ_LOOSE',30:'PUSH_SCOPE',31:'JUMP',32:'JIF_T',33:'LOAD_VAR',
            34:'PUSH_WIN',35:'TYPEOF',36:'NEQ_S',37:'CALL_WH',38:'OR',
            39:'TRY',40:'DIV',41:'NEW',42:'IN',43:'PUSH_C',44:'PUSH_OBJ',
            45:'DEL',46:'INST',47:'SET_IDX',48:'SHR',49:'BIND',50:'CALL_M',
            51:'NEQ_L',52:'NOT',53:'POP_N',54:'PUSH_U',55:'BUILD_ARR',56:'GET_PROP'
            }.get(A, f'?{A}')

def dis1(pc):
    if pc >= MAX: return f"{pc}: EOF", pc+1
    A = r[pc]; n = pc+1
    op = opname(A)
    args = []
    if A == 43:
        vi = r[n]; n+=1
        v = repr(V[vi]) if vi < len(V) else f'?'
        args = [f'V[{vi}]={v}']
    elif A == 33:
        vi = r[n]; n+=1; push = r[n]; n+=1
        v = repr(V[vi]) if vi < len(V) else f'?'
        args = [f'V[{vi}]={v}', f'push={push}']
    elif A == 21:
        vi = r[n]; n+=1
        v = repr(V[vi]) if vi < len(V) else f'?'
        args = [f'V[{vi}]={v}']
    elif A in [27,56,0,2,5,9,10,12,15,17,18,20,22,23,24,26,29,36,38,40,48,51,52]:
        push = r[n]; n+=1; args=[f'push={push}']
    elif A in [28,31,32,6,37]:
        t = r[n]; n+=1; args=[f'->{t}']
    elif A == 49:
        cnt = r[n]; n+=1; args=[f'n={cnt}']
    elif A in [55,16]:
        cnt = r[n]; n+=1; push = r[n]; n+=1; args=[f'cnt={cnt}',f'push={push}']
    elif A == 50:
        na = r[n]; n+=1; push = r[n]; n+=1; args=[f'nargs={na}',f'push={push}']
    elif A == 7:
        entry = r[n]; n+=1; push = r[n]; n+=1; args=[f'entry={entry}',f'push={push}']
    elif A in [8,3]:
        pre=r[n];n+=1;push=r[n];n+=1;args=[f'pre={pre}',f'push={push}']
    elif A == 53:
        cnt=r[n];n+=1;push=r[n];n+=1;args=[f'cnt={cnt}',f'push={push}']
    elif A == 25:
        v=r[n];n+=1;args=[f'inline_val={v}']
    return f"{pc:6d}: {op:12} {' '.join(str(a) for a in args)}", n

# Trace from PC 15989 (y function entry) through 16838 - fully
pc = 15989
while pc < 16840:
    line, next_pc = dis1(pc)
    print(line)
    pc = next_pc
