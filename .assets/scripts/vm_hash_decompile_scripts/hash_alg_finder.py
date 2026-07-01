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

# Now let's look at the main function that's being computed
# The initial part sets up:
# - f = 256 (lookup table size) -> from the final section
# - e = a lookup/state table array of 256 bytes (the S-box for RC4/custom cipher)
# - C = the key scheduling function
# - i = Math
# - h = btoa
# - etc.

# Let me find where 'f' and 'e' get their values in the final section
# Looking at disasm around 15780+

# The key insight from the loop structures:
# Starting at PC 15814:
#   s = 0; while s < 16: push random byte to 'a', s++
# So 'a' = array of 16 random bytes? No wait - it checks 'C()'
# Actually: C() is called, result in m
# if m:
#   m = m[a[a.length-1]]  -- last element of a? Indexed by something
#   Then proceeds with the m value as the key
# If not m (i.e., if __ALIYUN_CRYPT is not defined):
#   Generate 16 random bytes into a
#   a = a.map(fn).join('') -> convert each byte to 2-hex-digit string
# Then jump to final section

# Let me read PC 15871-15980 more carefully from the raw bytecodes

print("Bytecodes at PC 15871-15980:")
for i in range(15871, 15982):
    print(f"  F[{i}] = {F[i]}")
