# Assuming context came from function call P(0,[],F.V,ez,[input,salt])

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

# String at 15728 with xor key = V[151] = 93
# nums: V[186]=40, V[261]=51, V[33]=56, V[90]=46, V[262]=62, V[273]=60, V[179]=45, V[33]=56
nums = [V[186], V[261], V[33], V[90], V[262], V[273], V[179], V[33]]
key = V[151]  # = 93
print(f"V[151] = {key}")
s = ''.join(chr(int(n) ^ int(key)) for n in nums)
print(f"String at 15728 (xor=93) -> 'u' = {repr(s)}")

# What's at string decoded earlier as h?
# At pc=15396: nums=[V[241]=176, V[105]=166, V[240]=189, V[168]=179] xor=75
# 176^75=235(?), let me recompute
nums_h = [V[241], V[105], V[240], V[168]]
key_h = 75
s_h = ''.join(chr(int(n) ^ key_h) for n in nums_h)
print(f"String for 'h' = {repr(s_h)}")

# Let me trace back further to find what 'C' function is, what 'h' value comes from
# The string result gets accessed as window[str] (._ = window)
# e['atob'] -> the fn arg[0] which is the JSON input? 
# No wait...

# Let me look at the variables more carefully
# V[279]='u', V[280]='arguments', V[281]='s'

# Argument 0 = o = the JSON input string  
# Argument 1 = r = td = '0000'

# 'o' = input string (JSON)
# 'r' = td = '0000'
# 'a' = [] (empty array, will be filled with 16 random bytes = AES key candidate?)
# 'C' = some function called on the scope (c.C())
# 'm' = result of C()
# 'f' = length of something (a lookup table size?)

# Let me find what 'f', 'i', 'e', 'p' are set to before the final section
# Looking at what builds them...

# From the disassembly list [6]-[7]:
# e = 'prototype'
# f = 'undefined'
# i = 'p'  (V[8]='p')
# p = 'h'  (V[9]='h')

# But these get overwritten. Let me look at the critical section more carefully
# From the extracted strings:
# [171] = 'Math'
# [172] = 'atob'  (V[336]=4 -> wait that doesn't match)

# Let me check string #171 - 'Math'
# pc=15268, xor=95, nums=[...] -> 'Math'
# Then it's accessed as window['Math'] 

# For 'atob': pc=15336, xor=6, -> 'atob'

# So the function accesses window.Math and window.atob etc.
# The 'C' function is some transformation
# Let me look at what h is - appears to be parseInt  based on the 'atob' access pattern

# Actually let me check what's at h - it comes from:
# PUSH_SCOPE V[76]='_'  -> window
# [XORED STRING] GET_PROP -> window[decoded_string]
# h = result

# At string [133]: pc=12204, decoded='p' -> but that's innerText looking at the assignment

# Let me check V[279]='u' and see what u gets set to
# From disasm line 7585-7586: v[279]='u' SET_PROP -> c.u = result of get_prop(window, XORED_STR)
# And the xored string at 15728 with xor=93 -> let me recalculate

print("\nRechecking special variable assignments:")

# 'h' at pc 15396: get_prop from window
nums_h2 = [V[241], V[105], V[240], V[168]]
# fn at ~15370 uses xor key V[94]=75 (from the pattern before)
# But wait - at 15396 we do: BUILD_ARRAY + map(fn) + join + GET_PROP + SET_PROP h
# The fn was the one at entry 15375 with xor=75
key_h2 = 75
s_h2 = ''.join(chr(int(n) ^ key_h2) for n in nums_h2)
print(f"h = window[{repr(s_h2)}]")  # parseInt!

# 'd' at pc 15500: window[decodeURIComponent_string] (already decoded as 'decodeURIComponent')
# But wait - the access is _.d = window['encodeURIComponent']? 
# No - let me re-read:
# c._ = window, then GET_PROP gives window['encodeURIComponent']
# That gets stored in 'd'
print(f"d = window['encodeURIComponent'] (the fn)")

# 'j' at pc 15560: window['decodeURIComponent']
print(f"j = window['decodeURIComponent'] (the fn)")

# 'u' at pc 15728 - string decoded above  
print(f"u = window[{repr(s)}]")  # let me check

# Looking at the scope: c._ = window
# So _.u means window.u? No, 'u' is a JS VM variable name (V[278]='u')
# The value of u = result of window['encodeURIComponent'](something)?
# No - at 15764: c.u = window['_'][decoded_string_at_15728]
# c._ = window so window[string]

s_u = ''.join(chr(int(n) ^ 93) for n in nums)
print(f"u = window[{repr(s_u)}]")
