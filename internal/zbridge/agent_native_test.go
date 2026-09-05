package zbridge

import (
    "encoding/json"
    "strings"
    "testing"
)

// Captured from a real session: glm-5.3 emitted its native syntax mid-run and
// the whole block reached the client as assistant text.
const nativeCapture = `<tool_call>bash
<arg_key>command</arg_key>
<arg_value>cd ~/Projects/many-proxys && grep -n 'generate-nix' AGENTS.md | wc -l</arg_value>
<arg_key>i</arg_key>
<arg_value>Check which pending edits actually applied</arg_value>
<arg_key>cwd</arg_key>
<arg_value>/home/finn/Projects/many-proxys</arg_value>
</tool_call>`

func TestTranslateNativeToolCall(t *testing.T) {
    calls := ParseAgentToolCalls(nativeCapture)
    if len(calls) != 1 {
        t.Fatalf("want 1 tool call, got %d", len(calls))
    }
    fn := calls[0]["function"].(map[string]interface{})
    if fn["name"] != "bash" {
        t.Fatalf("want name bash, got %v", fn["name"])
    }
    var args map[string]interface{}
    if err := json.Unmarshal([]byte(fn["arguments"].(string)), &args); err != nil {
        t.Fatalf("arguments are not valid JSON: %v", err)
    }
    if !strings.Contains(args["command"].(string), "grep -n 'generate-nix'") {
        t.Fatalf("command argument lost: %v", args["command"])
    }
    if args["cwd"] != "/home/finn/Projects/many-proxys" {
        t.Fatalf("cwd argument lost: %v", args["cwd"])
    }
}

// The block must not also survive as assistant text, or the model imitates its
// own broken output on the next turn.
func TestNativeToolCallStripped(t *testing.T) {
    if got := StripAgentToolCalls("before " + nativeCapture + " after"); strings.Contains(got, "<tool_call>") {
        t.Fatalf("native block leaked into content: %q", got)
    }
}

func TestTranslateNativeMultipleCalls(t *testing.T) {
    in := nativeCapture + "\n" + `<tool_call>read_file
<arg_key>path</arg_key>
<arg_value>config.json</arg_value>
</tool_call>`
    calls := ParseAgentToolCalls(in)
    if len(calls) != 2 {
        t.Fatalf("want 2 tool calls, got %d", len(calls))
    }
}

// Values carry heredocs, quotes and newlines; they must survive JSON encoding.
func TestTranslateNativeMultilineValue(t *testing.T) {
    in := `<tool_call>bash
<arg_key>command</arg_key>
<arg_value>python3 - <<'EOF'
s = "it's \"quoted\""
print(s)
EOF</arg_value>
</tool_call>`
    calls := ParseAgentToolCalls(in)
    if len(calls) != 1 {
        t.Fatalf("want 1 tool call, got %d", len(calls))
    }
    fn := calls[0]["function"].(map[string]interface{})
    var args map[string]interface{}
    if err := json.Unmarshal([]byte(fn["arguments"].(string)), &args); err != nil {
        t.Fatalf("arguments are not valid JSON: %v", err)
    }
    cmd := args["command"].(string)
    for _, want := range []string{"<<'EOF'", `it's`, `\"quoted\"`, "print(s)", "EOF"} {
        if !strings.Contains(cmd, want) {
            t.Fatalf("command lost %q: %q", want, cmd)
        }
    }
}

// Ordinary text and canonical marker blocks must pass through untouched.
func TestTranslateNativeLeavesOtherTextAlone(t *testing.T) {
    plain := "Here is some prose mentioning <tool> and </call> but no block."
    if got := TranslateNativeToolCalls(plain); got != plain {
        t.Fatalf("plain text rewritten: %q", got)
    }
    canonical := agentToolStart + `{"name":"bash","arguments":{"command":"ls"}}` + agentToolEnd
    if got := TranslateNativeToolCalls(canonical); got != canonical {
        t.Fatalf("canonical block rewritten: %q", got)
    }
}

// ── streaming ────────────────────────────────────────────────────────────────

func TestStreamNativeToolCall(t *testing.T) {
    // Sizes chosen so the opening marker, the arg tags and the closing marker
    // each land split across a chunk boundary.
    for _, size := range []int{1, 3, 7, 13, 64, 4096} {
        content, calls, args := feedChunks(t, splitRunes("Working on it.\n"+nativeCapture, size), 1)
        if calls != 1 {
            t.Fatalf("size %d: want 1 tool call, got %d (content %q)", size, calls, content)
        }
        if !strings.Contains(args, "generate-nix") {
            t.Fatalf("size %d: arguments lost: %q", size, args)
        }
        if strings.Contains(content, "<tool_call>") || strings.Contains(content, "arg_key") {
            t.Fatalf("size %d: native syntax leaked as content: %q", size, content)
        }
        if !strings.Contains(content, "Working on it.") {
            t.Fatalf("size %d: ordinary content lost: %q", size, content)
        }
    }
}

func TestStreamCanonicalStillWins(t *testing.T) {
    canonical := agentToolStart + `{"name":"ls","arguments":{"path":"/tmp"}}` + agentToolEnd
    content, calls, _ := feedChunks(t, splitRunes(canonical, 5), 1)
    if calls != 1 {
        t.Fatalf("canonical block not parsed while streaming: %d calls, content %q", calls, content)
    }
    if strings.Contains(content, "TOOL_CALL") {
        t.Fatalf("canonical marker leaked as content: %q", content)
    }
}

// A block that never closes must still surface as text rather than vanish.
func TestStreamTruncatedNativeBlockNotSwallowed(t *testing.T) {
    content, calls, _ := feedChunks(t, splitRunes("prefix <tool_call>bash\n<arg_key>command</arg_key>", 9), 1)
    if calls != 0 {
        t.Fatalf("truncated block produced %d calls", calls)
    }
    if !strings.Contains(content, "prefix") {
        t.Fatalf("content before the truncated block lost: %q", content)
    }
}
