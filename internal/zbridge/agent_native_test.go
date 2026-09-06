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

// ── parenthesised argument lists ─────────────────────────────────────────────

// Captured from a real session: glm-5.3 wrote the call as a function
// invocation and left the closing tag off, so the block regex missed it
// entirely and the line arrived as assistant text.
const nativeKwargsCapture = `<tool_call>web_search(i="Surveying current free LLM proxy projects", ` +
    `query="free LLM web chat OpenAI-compatible API proxy github 2026 free tier scraper", limit=8)`

// A second capture of the same shape, from another session.
const nativeKwargsCapture2 = `<tool_call>grep(i="Finding locally-built services in compose", ` +
    `pattern="build:|dockerfile_inline|context:", path="docker-compose.yaml")`

func nativeCallArgs(t *testing.T, text string) (string, map[string]interface{}) {
    t.Helper()
    calls := ParseAgentToolCalls(text)
    if len(calls) != 1 {
        t.Fatalf("want 1 tool call, got %d (from %q)", len(calls), text)
    }
    fn := calls[0]["function"].(map[string]interface{})
    var args map[string]interface{}
    if err := json.Unmarshal([]byte(fn["arguments"].(string)), &args); err != nil {
        t.Fatalf("arguments are not valid JSON: %v", err)
    }
    return fn["name"].(string), args
}

func TestTranslateNativeKwargsClosed(t *testing.T) {
    name, args := nativeCallArgs(t, nativeKwargsCapture+"</tool_call>")
    if name != "web_search" {
        t.Fatalf("want name web_search, got %q", name)
    }
    if args["query"] != "free LLM web chat OpenAI-compatible API proxy github 2026 free tier scraper" {
        t.Fatalf("query argument lost: %v", args["query"])
    }
    if args["i"] != "Surveying current free LLM proxy projects" {
        t.Fatalf("i argument lost: %v", args["i"])
    }
    if args["limit"] != float64(8) {
        t.Fatalf("want limit 8 as a number, got %#v", args["limit"])
    }
}

// The closing tag is the half the model omits most often; without recovery the
// whole line reaches the client as text.
func TestTranslateNativeKwargsUnterminated(t *testing.T) {
    for _, capture := range []string{nativeKwargsCapture, nativeKwargsCapture2} {
        name, args := nativeCallArgs(t, "Let me look that up.\n"+capture)
        if name != "web_search" && name != "grep" {
            t.Fatalf("unexpected name %q", name)
        }
        if args["i"] == nil || args["i"] == "" {
            t.Fatalf("%s: intent argument lost: %#v", name, args)
        }
    }
}

func TestNativeKwargsStrippedFromContent(t *testing.T) {
    got := StripAgentToolCalls("Let me look that up.\n" + nativeKwargsCapture)
    if strings.Contains(got, "<tool_call>") || strings.Contains(got, "web_search(") {
        t.Fatalf("unterminated block leaked into content: %q", got)
    }
    if !strings.Contains(got, "Let me look that up.") {
        t.Fatalf("ordinary content lost: %q", got)
    }
}

func TestNativeKwargsScalars(t *testing.T) {
    _, args := nativeCallArgs(t,
        `<tool_call>t(n=8, f=1.5, yes=true, no=false, nothing=null, bare=abc, esc="a\nb")</tool_call>`)
    if args["n"] != float64(8) || args["f"] != 1.5 {
        t.Fatalf("numbers mistyped: %#v", args)
    }
    if args["yes"] != true || args["no"] != false {
        t.Fatalf("booleans mistyped: %#v", args)
    }
    if v, ok := args["nothing"]; !ok || v != nil {
        t.Fatalf("null mistyped: %#v", args)
    }
    if args["bare"] != "abc" {
        t.Fatalf("bare word mistyped: %#v", args["bare"])
    }
    if args["esc"] != "a\nb" {
        t.Fatalf("escape not decoded: %q", args["esc"])
    }
}

func TestStreamNativeKwargsUnterminated(t *testing.T) {
    for _, size := range []int{1, 3, 7, 13, 64, 4096} {
        content, calls, args := feedChunks(t, splitRunes("Working on it.\n"+nativeKwargsCapture, size), 1)
        if calls != 1 {
            t.Fatalf("size %d: want 1 tool call, got %d (content %q)", size, calls, content)
        }
        if !strings.Contains(args, "OpenAI-compatible") {
            t.Fatalf("size %d: arguments lost: %q", size, args)
        }
        if strings.Contains(content, "<tool_call>") || strings.Contains(content, "web_search(") {
            t.Fatalf("size %d: native syntax leaked as content: %q", size, content)
        }
        if !strings.Contains(content, "Working on it.") {
            t.Fatalf("size %d: ordinary content lost: %q", size, content)
        }
    }
}

// ── every dialect the models reach for ───────────────────────────────────────

// The shim asks for one format; the models emit whichever they were trained
// on. Each of these must land as a tool call rather than as assistant text,
// closed or not.
func TestNativeDialects(t *testing.T) {
    for _, tc := range []struct {
        label string
        block string
    }{
        {"tagged pairs", "<tool_call>bash\n<arg_key>command</arg_key>\n<arg_value>ls -la</arg_value>\n</tool_call>"},
        {"keyword args", `<tool_call>bash(command="ls -la")</tool_call>`},
        {"name then JSON", `<tool_call>bash{"command":"ls -la"}</tool_call>`},
        {"JSON envelope", `<tool_call>{"name":"bash","arguments":{"command":"ls -la"}}</tool_call>`},
        {"JSON envelope, string args", `<tool_call>{"name":"bash","arguments":"{\"command\":\"ls -la\"}"}</tool_call>`},
        {"JSON envelope, parameters key", `<tool_call>{"name":"bash","parameters":{"command":"ls -la"}}</tool_call>`},
    } {
        for _, variant := range []struct {
            suffix string
            text   string
        }{
            {" closed", tc.block},
            {" unterminated", strings.TrimSuffix(tc.block, "</tool_call>")},
        } {
            name, args := nativeCallArgs(t, "Working on it.\n"+variant.text)
            if name != "bash" {
                t.Fatalf("%s%s: want name bash, got %q", tc.label, variant.suffix, name)
            }
            if args["command"] != "ls -la" {
                t.Fatalf("%s%s: command lost: %#v", tc.label, variant.suffix, args)
            }
            if got := StripAgentToolCalls("Working on it.\n" + variant.text); strings.Contains(got, "<tool_call>") {
                t.Fatalf("%s%s: block leaked as content: %q", tc.label, variant.suffix, got)
            }
        }
    }
}

// Recovery must never fabricate a call from a payload that stopped mid-write,
// in any dialect.
func TestNativeTruncatedDialectsNotRecovered(t *testing.T) {
    for _, truncated := range []string{
        `<tool_call>bash(command="rm -rf /home/finn/Proj`,
        `<tool_call>bash(command="ls", cwd=`,
        `<tool_call>bash{"command":"rm -rf /home/finn/Proj`,
        `<tool_call>{"name":"bash","arguments":{"command":"rm -rf`,
        `<tool_call>bash`,
        "<tool_call>bash\n<arg_key>command</arg_key>",
    } {
        if calls := ParseAgentToolCalls(truncated); len(calls) != 0 {
            t.Fatalf("truncated payload %q produced %d calls", truncated, len(calls))
        }
        if got := StripAgentToolCalls("before " + truncated); !strings.Contains(got, "before") {
            t.Fatalf("content before a truncated block lost: %q", got)
        }
    }
}

// A closed block followed by an unterminated one: the first is translated
// regardless, the second only on its own merits.
func TestNativeMixedTerminationInOneText(t *testing.T) {
    in := `<tool_call>read_file{"path":"a.txt"}</tool_call>` + "\nthen\n" +
        `<tool_call>grep(pattern="x", path="b.txt")`
    calls := ParseAgentToolCalls(in)
    if len(calls) != 2 {
        t.Fatalf("want 2 tool calls, got %d", len(calls))
    }
    if got := StripAgentToolCalls(in); !strings.Contains(got, "then") {
        t.Fatalf("text between the blocks lost: %q", got)
    }
}

func TestStreamNativeDialects(t *testing.T) {
    for _, block := range []string{
        `<tool_call>bash{"command":"ls -la"}`,
        `<tool_call>{"name":"bash","arguments":{"command":"ls -la"}}`,
        `<tool_call>bash(command="ls -la")`,
    } {
        for _, size := range []int{1, 3, 7, 13, 64, 4096} {
            content, calls, args := feedChunks(t, splitRunes("Working on it.\n"+block, size), 1)
            if calls != 1 {
                t.Fatalf("%q at size %d: want 1 call, got %d (content %q)", block, size, calls, content)
            }
            if !strings.Contains(args, "ls -la") {
                t.Fatalf("%q at size %d: arguments lost: %q", block, size, args)
            }
            if strings.Contains(content, "<tool_call>") {
                t.Fatalf("%q at size %d: leaked as content: %q", block, size, content)
            }
        }
    }
}

// ── damaged canonical blocks ─────────────────────────────────────────────────

// Captured live: the block carries Zhipu's native opener AND the canonical
// closing marker, and the JSON has lost its {"name":…, head. No model writes
// this; it is what an upstream rewrite leaves behind (issue #23).
const damagedCapture = "<tool_call>bash\n arguments\":{\"i\":\"Inspecting merge conflict in openai.go\"," +
    "\"command\":\"grep -n -A6 -B6 '<<<<<<<' ../aistudio2api/internal/api/openai.go\"} <<<END_TOOL_CALL>>>"

func TestDamagedCanonicalSalvaged(t *testing.T) {
    name, args := nativeCallArgs(t, "Ich sehe nach.\n"+damagedCapture)
    if name != "bash" {
        t.Fatalf("want name bash, got %q", name)
    }
    if !strings.Contains(args["command"].(string), "grep -n -A6 -B6") {
        t.Fatalf("command lost: %#v", args["command"])
    }
    if args["i"] != "Inspecting merge conflict in openai.go" {
        t.Fatalf("intent lost: %#v", args["i"])
    }
    got := StripAgentToolCalls("Ich sehe nach.\n" + damagedCapture)
    for _, leftover := range []string{"<tool_call>", "arguments\":", "END_TOOL_CALL"} {
        if strings.Contains(got, leftover) {
            t.Fatalf("%q left in content: %q", leftover, got)
        }
    }
    if !strings.Contains(got, "Ich sehe nach.") {
        t.Fatalf("ordinary content lost: %q", got)
    }
}

// The closing marker is the anchor and the JSON parse is the integrity check.
// Without either, the block stays text rather than becoming a call built from
// a payload that may itself be spliced.
func TestDamagedCanonicalNeedsAnchorAndValidJSON(t *testing.T) {
    for _, tc := range []struct {
        label string
        block string
    }{
        {"no closing marker", "<tool_call>bash\n arguments\":{\"command\":\"ls\"}"},
        {"unbalanced JSON", "<tool_call>bash\n arguments\":{\"command\":\"ls\" <<<END_TOOL_CALL>>>"},
        {"no arguments key", "<tool_call>bash\n whatever <<<END_TOOL_CALL>>>"},
        {"no name", "<tool_call>\n arguments\":{\"command\":\"ls\"} <<<END_TOOL_CALL>>>"},
    } {
        if calls := ParseAgentToolCalls(tc.block); len(calls) != 0 {
            t.Fatalf("%s: produced %d calls", tc.label, len(calls))
        }
    }
}
