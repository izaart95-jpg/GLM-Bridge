// agent_test.go
//
// Tests for the modern agent-mode shim (agent.go), ported from the
// DeepseekFreeAPI reference implementation (internal/dsproxy/*_test.go) and
// extended with GLM-Free-API dispatch/integration coverage.

package zbridge

import (
    "encoding/json"
    "strings"
    "testing"
)

// ── marker finder ────────────────────────────────────────────────────────────

func TestFindAgentMarkerVariants(t *testing.T) {
    cases := []struct {
        in     string
        wantAt int // -1 = no match
        wantLn int
    }{
        {"<<<TOOL_CALL>>>", 0, 15},     // canonical
        {"<<TOOL_CALL>>>", 0, 14},      // live failure: one '<' short
        {"<<<TOOL_CALL>>", 0, 14},      // one '>' short
        {"<<<<TOOL_CALL>>>>", 0, 17},   // worst tolerated spelling
        {"x <<<TOOL_CALL>>> y", 2, 15}, // embedded
        {"<TOOL_CALL>", -1, 0},         // too few brackets
        {"<<<TOOL_CALL>", -1, 0},       // unterminated
        {"TOOL_CALL", -1, 0},           // bare word
        {"<<<<<<<TOOL_CALL>>>", -1, 0}, // bracket run longer than tolerated
    }
    for _, c := range cases {
        at, ln := findAgentMarker(c.in, agentStartWord, true)
        if at != c.wantAt || (c.wantAt >= 0 && ln != c.wantLn) {
            t.Errorf("findAgentMarker(%q) = (%d,%d), want (%d,%d)", c.in, at, ln, c.wantAt, c.wantLn)
        }
    }
    // The TOOL_CALL inside an END marker must never be taken as a start.
    if at, _ := findAgentMarker(agentToolEnd, agentStartWord, true); at != -1 {
        t.Errorf("END marker matched as start at %d", at)
    }
    if n := bracketRunBack("a<<<b", 4, '<'); n != 3 {
        t.Errorf("bracketRunBack = %d, want 3", n)
    }
    if got := agentWorstMarkerLen; got != len("<<<<TOOL_CALL>>>>") {
        t.Errorf("agentWorstMarkerLen = %d, want %d", got, len("<<<<TOOL_CALL>>>>"))
    }

    // Streaming mode: a trailing '>' run touching the end of the data must
    // wait instead of matching short — this keeps the third '>' of a
    // canonical marker from leaking when chunks split the marker.
    for _, tc := range []struct{ in, word string }{
        {"a <<<TOOL_CALL>>", agentStartWord},
        {"x <<<END_TOOL_CALL>>", agentEndWord},
    } {
        if at, _ := findAgentMarker(tc.in, tc.word, false); at != markerIncomplete {
            t.Errorf("findAgentMarker(%q, final=false) = %d, want markerIncomplete", tc.in, at)
        }
    }
    if at, _ := findAgentMarker("<<<TOOL_CALL>> x", agentStartWord, false); at != 0 {
        t.Errorf("terminated short run should match at 0, got %d", at)
    }
}

// ── finished-text parsing ────────────────────────────────────────────────────

// The exact RESPONSE payload reconstructed from the failing debug session
// (deepseek-v4-pro, "Find my public ip").
const malformedPayload = "<<TOOL_CALL>>>\n{\"name\":\"bash\",\"arguments\":{\"command\":\"curl -s https://api.ipify.org\"}}\n<<<END_TOOL_CALL>>>"

func TestParseAgentToolCallsTolerantMarkers(t *testing.T) {
    cases := []struct {
        text     string
        wantArgs string // substring expected inside arguments JSON
    }{
        {malformedPayload, "api.ipify.org"},
        {"Let me check.\n" + malformedPayload + "\n", "api.ipify.org"},
        {"<<<<TOOL_CALL>>>>\n{\"name\":\"bash\",\"arguments\":{}}\n<<<<END_TOOL_CALL>>>>", "{}"},
    }
    for _, c := range cases {
        calls := ParseAgentToolCalls(c.text)
        if len(calls) != 1 {
            t.Fatalf("ParseAgentToolCalls(%q...) => %d calls, want 1", c.text[:12], len(calls))
        }
        fn := calls[0]["function"].(map[string]interface{})
        if fn["name"] != "bash" {
            t.Errorf("name = %v, want bash", fn["name"])
        }
        args := fn["arguments"].(string)
        if !strings.Contains(args, c.wantArgs) {
            t.Errorf("arguments = %s, want substring %s", args, c.wantArgs)
        }
        if stripped := StripAgentToolCalls(c.text); strings.Contains(stripped, "TOOL_CALL") {
            t.Errorf("StripAgentToolCalls left markers: %q", stripped)
        }
    }
}

func TestNormalizeAgentFencesTolerantMarkers(t *testing.T) {
    in := "```json\n<<TOOL_CALL>>>\n{\"name\":\"bash\",\"arguments\":{}}\n<<<END_TOOL_CALL>>>\n```\ndone"
    got := NormalizeAgentFences(in)
    want := "<<TOOL_CALL>>>\n{\"name\":\"bash\",\"arguments\":{}}\n<<<END_TOOL_CALL>>>\ndone"
    if got != want {
        t.Errorf("normalize:\n got %q\nwant %q", got, want)
    }
}

// ── payload tolerance ────────────────────────────────────────────────────────
//
// Second live failure mode: the markers were canonical, but the model invented
// a FLAT payload shape — {"tool": "bash", "command": ..., "timeout": 10}
// instead of {"name": ..., "arguments": {...}}. A strict parser accepts the
// JSON with Name == "" and the whole block leaks to the client as plain
// content with finish_reason "stop", so the tool is never executed.

// The exact RESPONSE text reconstructed from the failing debug session.
const flatPayload = `<<<TOOL_CALL>>>{"tool": "bash", "command": "curl -s ifconfig.me", "timeout": 10}<<<END_TOOL_CALL>>>`

func TestParseAgentToolCallsFlatPayload(t *testing.T) {
    calls := ParseAgentToolCalls(flatPayload)
    if len(calls) != 1 {
        t.Fatalf("ParseAgentToolCalls(flat payload) => %d calls, want 1", len(calls))
    }
    fn := calls[0]["function"].(map[string]interface{})
    if fn["name"] != "bash" {
        t.Errorf("name = %v, want bash", fn["name"])
    }
    args := fn["arguments"].(string)
    if !strings.Contains(args, `"command":"curl -s ifconfig.me"`) || !strings.Contains(args, `"timeout":10`) {
        t.Errorf("arguments = %s, want flat keys folded into an arguments object", args)
    }
    if stripped := StripAgentToolCalls(flatPayload); strings.Contains(stripped, "TOOL_CALL") || strings.TrimSpace(stripped) != "" {
        t.Errorf("StripAgentToolCalls left residue: %q", stripped)
    }
}

func TestParseAgentToolCallsPayloadVariants(t *testing.T) {
    cases := []struct {
        body     string // JSON body between the markers
        wantName string
        wantArgs string // substring the arguments JSON must contain
    }{
        // canonical shape keeps working
        {`{"name":"bash","arguments":{"command":"id"}}`, "bash", `"command":"id"`},
        // alternate explicit-arguments spellings
        {`{"name":"bash","parameters":{"command":"id"}}`, "bash", `"command":"id"`},
        {`{"tool":"bash","arguments":{"command":"id"}}`, "bash", `"command":"id"`},
        // alternate name keys with flat parameters
        {`{"tool_name":"read","path":"/etc/hosts"}`, "read", `"/etc/hosts"`},
        {`{"function":"bash","command":"uname"}`, "bash", `"uname"`},
        // flat payload keyed on "name"
        {`{"name":"bash","command":"pwd","timeout":5}`, "bash", `"command":"pwd"`},
        // a tool parameter literally named "name" must not shadow the tool key
        {`{"tool":"write","name":"a.txt","content":"x"}`, "write", `"content":"x"`},
        // no parameters at all
        {`{"tool":"bash"}`, "bash", `{}`},
    }
    for _, c := range cases {
        text := "<<<TOOL_CALL>>>" + c.body + "<<<END_TOOL_CALL>>>"
        calls := ParseAgentToolCalls(text)
        if len(calls) != 1 {
            t.Errorf("body %s => %d calls, want 1", c.body, len(calls))
            continue
        }
        fn := calls[0]["function"].(map[string]interface{})
        if fn["name"] != c.wantName {
            t.Errorf("body %s => name %v, want %s", c.body, fn["name"], c.wantName)
        }
        if args := fn["arguments"].(string); !strings.Contains(args, c.wantArgs) {
            t.Errorf("body %s => arguments %s, want substring %s", c.body, args, c.wantArgs)
        }
    }
    // No recognizable tool name at all: stays visible text (existing policy).
    if calls := ParseAgentToolCalls(`<<<TOOL_CALL>>>{"command":"ls"}<<<END_TOOL_CALL>>>`); len(calls) != 0 {
        t.Errorf("nameless payload produced %d calls, want 0", len(calls))
    }
}

// goalFragmentChunks are the upstream SSE fragment appends of the failing
// session, byte for byte (initial RESPONSE fragment content "<<", then every
// APPEND/"v" delta from the debug log).
var goalFragmentChunks = []string{
    "<<", "<", "TO", "OL", "_C", "ALL", ">>>",
    "{\"", "tool", "\":", " \"", "bash", "\",", " \"", "command", "\":",
    " \"", "curl", " -", "s", " if", "config", ".me", "\",", " \"",
    "time", "out", "\":", " ", "10", "}",
    "<<", "<", "END", "_TO", "OL", "_C", "ALL", ">>>",
}

func TestStreamInterceptorReplaysFlatPayloadDebugStream(t *testing.T) {
    for _, step := range []int{1, 3, 7} {
        content, calls, args := feedChunks(t, goalFragmentChunks, step)
        if calls != 1 {
            t.Errorf("step=%d: %d tool calls, want 1 (content=%q)", step, calls, content)
        }
        if !strings.Contains(args, "ifconfig.me") || !strings.Contains(args, `"timeout":10`) {
            t.Errorf("step=%d: arguments %q missing flat payload parameters", step, args)
        }
        if strings.Contains(content, "TOOL_CALL") || strings.TrimSpace(content) != "" {
            t.Errorf("step=%d: markers or junk leaked as content: %q", step, content)
        }
    }
}

// The prompt must pin the payload schema: the bare "{JSON}" placeholder is
// what let the model invent the flat shape in the first place.
func TestAgentPromptPinsPayloadSchema(t *testing.T) {
    prompt := buildAgentPrompt(
        []agentMessage{{Role: "user", Content: []byte(`"Find my public ip"`)}},
        []openAITool{{Type: "function", Function: &openAIFnSpec{Name: "bash"}}},
    )
    for _, want := range []string{
        `<<<TOOL_CALL>>>{"name":"<tool_name>","arguments":{<parameter JSON>}}<<<END_TOOL_CALL>>>`,
        `EXACTLY two keys`,
        `"name"`,
        `"arguments"`,
    } {
        if !strings.Contains(prompt, want) {
            t.Errorf("agent prompt missing schema fragment %q", want)
        }
    }
    if strings.Contains(prompt, "{JSON}") {
        t.Errorf("agent prompt still carries the underspecified {JSON} placeholder")
    }
}

// ── streaming interceptor ────────────────────────────────────────────────────

// Replays the RESPONSE fragment chunks exactly as they arrived in the failing
// session's upstream SSE log, byte for byte.
var debugFragmentChunks = []string{
    "<<", "<", "TO", "OL", "_C", "ALL", ">>", ">\n",
    "{\"", "name", "\":\"", "bash", "\",\"",
    "arguments", "\":{\"", "command", "\":\"", "curl",
    " -", "s", " https", "://", "api", ".ip", "ify", ".org", "\"", "}}\n",
    "<<", "<", "END", "_TO", "OL", "_C", "ALL", ">>>",
}

func feedChunks(t *testing.T, chunks []string, step int) (content string, toolCalls int, args string) {
    t.Helper()
    in := &AgentStreamInterceptor{}
    var b strings.Builder
    for i := 0; i < len(chunks); i += step {
        piece := ""
        for j := i; j < i+step && j < len(chunks); j++ {
            piece += chunks[j]
        }
        parsed := in.Feed(piece)
        b.WriteString(parsed.Content)
        for _, call := range parsed.ToolCalls {
            fn := call["function"].(map[string]interface{})
            args, _ = fn["arguments"].(string)
            toolCalls++
        }
    }
    final := in.Finish()
    for _, call := range final.ToolCalls {
        fn := call["function"].(map[string]interface{})
        args, _ = fn["arguments"].(string)
        toolCalls++
    }
    return b.String() + final.Content, toolCalls, args
}

func TestStreamInterceptorReplaysMalformedDebugStream(t *testing.T) {
    for _, step := range []int{1, 3, 7} { // one chunk / log-like bursts / coarse
        content, calls, args := feedChunks(t, debugFragmentChunks, step)
        if calls != 1 {
            t.Errorf("step=%d: %d tool calls, want 1 (content=%q)", step, calls, content)
        }
        if !strings.Contains(args, "api.ipify.org") {
            t.Errorf("step=%d: arguments %q missing ipify command", step, args)
        }
        if strings.Contains(content, "TOOL_CALL") || strings.TrimSpace(content) != "" {
            t.Errorf("step=%d: markers or junk leaked as content: %q", step, content)
        }
    }
}

func TestStreamInterceptorShortTailVariant(t *testing.T) {
    stream := "checking…\n```json\n<<<TOOL_CALL>>\n{\"name\":\"bash\",\"arguments\":{\"command\":\"uname\"}}\n<<<END_TOOL_CALL>>>\n```\n"
    content, calls, _ := feedChunks(t, splitRunes(stream, 3), 1)
    if calls != 1 {
        t.Errorf("%d tool calls, want 1", calls)
    }
    if strings.Contains(content, "```") || strings.Contains(content, "TOOL_CALL") {
        t.Errorf("leaked: %q", content)
    }
    if !strings.Contains(content, "checking") {
        t.Errorf("prose damaged: %q", content)
    }
}

// An unterminated opening marker must not hang the stream nor be parsed.
func TestStreamInterceptorUnterminatedMarkerStaysContent(t *testing.T) {
    in := &AgentStreamInterceptor{}
    parsed := in.Feed("<TOOL_CALL> just talking about tools")
    final := in.Finish()
    out := parsed.Content + final.Content
    if len(parsed.ToolCalls)+len(final.ToolCalls) != 0 {
        t.Errorf("single-bracket text produced tool calls")
    }
    if out != "<TOOL_CALL> just talking about tools" {
        t.Errorf("single-bracket text altered: %q", out)
    }
}

// Invalid JSON inside a recognized block stays visible text (existing policy).
func TestStreamInvalidBlockLeaksAsContent(t *testing.T) {
    content, calls, _ := feedChunks(t, splitRunes("<<TOOL_CALL>>>\nnot json\n<<<END_TOOL_CALL>>>", 4), 1)
    if calls != 0 {
        t.Errorf("%d calls, want 0 for invalid block", calls)
    }
    if !strings.Contains(content, "not json") {
        t.Errorf("invalid block vanished instead of leaking: %q", content)
    }
}

func TestStreamTwoSequentialBlocks(t *testing.T) {
    stream := "<<TOOL_CALL>>>\n{\"name\":\"a\",\"arguments\":{}}\n<<<END_TOOL_CALL>>>\n<<TOOL_CALL>>>\n{\"name\":\"b\",\"arguments\":{}}\n<<<END_TOOL_CALL>>>"
    in := &AgentStreamInterceptor{}
    names := []string{}
    collect := func(parsed AgentParsedChunk) {
        for _, call := range parsed.ToolCalls {
            names = append(names, call["function"].(map[string]interface{})["name"].(string))
        }
    }
    for i := 0; i < len(stream); i += 5 {
        end := i + 5
        if end > len(stream) {
            end = len(stream)
        }
        collect(in.Feed(stream[i:end]))
    }
    collect(in.Finish())
    final := in.Finish()
    if strings.Contains(final.Content, "TOOL_CALL") {
        t.Errorf("trailing markers flushed: %q", final.Content)
    }
    if len(names) != 2 || names[0] != "a" || names[1] != "b" {
        t.Errorf("names = %v, want [a b]", names)
    }
}

// splitRunes cuts s into pieces of n bytes (ASCII input assumed).
func splitRunes(s string, n int) []string {
    var out []string
    for i := 0; i < len(s); i += n {
        end := i + n
        if end > len(s) {
            end = len(s)
        }
        out = append(out, s[i:end])
    }
    return out
}

// ── fence helpers ────────────────────────────────────────────────────────────

func TestTrimTrailingAgentFence(t *testing.T) {
    cases := []struct{ in, want string }{
        {"I'll check.\n```json", "I'll check."},
        {"I'll check.\n```", "I'll check."},
        {"plain", "plain"},
        {"hello ```bash\nx=1", "hello ```bash\nx=1"}, // real code block untouched
    }
    for _, c := range cases {
        if got := TrimTrailingAgentFence(c.in); got != c.want {
            t.Errorf("trim(%q)=%q want %q", c.in, got, c.want)
        }
    }
}

func TestSkipLeadingAgentFence(t *testing.T) {
    if n := SkipLeadingAgentFence("```\nrest"); n != 4 {
        t.Errorf("bare fence: got %d", n)
    }
    if n := SkipLeadingAgentFence("```json\ntext"); n != 8 {
        t.Errorf("json fence: got %d", n)
    }
    if n := SkipLeadingAgentFence("```python\nx"); n != 0 {
        t.Errorf("lang fence must not match: got %d", n)
    }
    if n := SkipLeadingAgentFence("text"); n != 0 {
        t.Errorf("text: got %d", n)
    }
}

func TestInterceptorSwallowsFencesAcrossChunks(t *testing.T) {
    stream := "Let me check your architecture.\n\n```json\n<<<TOOL_CALL>>>\n{\"name\":\"bash\",\"arguments\":{\"command\":\"uname -m\"}}\n<<<END_TOOL_CALL>>>\n```\ntrailing note"
    in := &AgentStreamInterceptor{}
    var content strings.Builder
    var calls int
    for i := 0; i < len(stream); i += 3 { // nasty 3-byte chunking
        end := i + 3
        if end > len(stream) {
            end = len(stream)
        }
        parsed := in.Feed(stream[i:end])
        content.WriteString(parsed.Content)
        calls += len(parsed.ToolCalls)
    }
    final := in.Finish()
    content.WriteString(final.Content)
    calls += len(final.ToolCalls)
    out := content.String()
    if calls != 1 {
        t.Errorf("expected 1 tool call, got %d", calls)
    }
    if strings.Contains(out, "```") || strings.Contains(out, "json\n<<<") {
        t.Errorf("fence leaked into content: %q", out)
    }
    if !strings.Contains(out, "Let me check") || !strings.Contains(out, "trailing note") {
        t.Errorf("prose damaged: %q", out)
    }
}

func TestNonStreamParseStripWithFences(t *testing.T) {
    text := "Sure!\n```json\n<<<TOOL_CALL>>>\n{\"name\":\"bash\",\"arguments\":{\"command\":\"arch\"}}\n<<<END_TOOL_CALL>>>\n```\nRunning now."
    calls := ParseAgentToolCalls(text)
    if len(calls) != 1 || calls[0]["function"].(map[string]interface{})["name"] != "bash" {
        t.Fatalf("parse failed: %#v", calls)
    }
    if stripped := StripAgentToolCalls(text); strings.Contains(stripped, "```") || strings.Contains(stripped, "TOOL_CALL") {
        t.Errorf("strip left junk: %q", stripped)
    }
}

// ── prompt structure (context rot) ───────────────────────────────────────────

// TestContextRotHypothesis tests whether the agent mode properly handles
// tool results when reasoning/thinking is enabled.
//
// The new prompt structure uses XML-like section tags (<tool_result>,
// <tool_exchange>, <current_task>) instead of [ROLE: ...] tags, which
// eliminates marker ambiguity and gives the model clear structure.
func TestContextRotHypothesis(t *testing.T) {
    messages := []agentMessage{
        {
            Role:    "system",
            Content: json.RawMessage(`"You are a helpful assistant."`),
        },
        {
            Role:    "user",
            Content: json.RawMessage(`"Find me public ip"`),
        },
        {
            Role:    "assistant",
            Content: json.RawMessage(`""`),
            ToolCalls: []assistantToolCall{
                {
                    ID:   "call_123",
                    Type: "function",
                    Function: struct {
                        Name      string          `json:"name"`
                        Arguments json.RawMessage `json:"arguments"`
                    }{
                        Name:      "bash",
                        Arguments: json.RawMessage(`{"command":"curl -s https://api.ipify.org"}`),
                    },
                },
            },
        },
        {
            Role:       "tool",
            Content:    json.RawMessage(`"110.226.238.87"`),
            ToolCallID: "call_123",
        },
        {
            Role:    "assistant",
            Content: json.RawMessage(`"Your IP address is 110.226.238.87."`),
        },
        {
            Role:    "user",
            Content: json.RawMessage(`"Fair enough now find my os arch and read my codebase explain me it in summary"`),
        },
    }

    tools := []openAITool{
        {
            Type: "function",
            Function: &openAIFnSpec{
                Name:        "bash",
                Description: "Execute a bash command",
                Parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}}}`),
            },
        },
    }

    prompt := buildAgentPrompt(messages, tools)

    // Check that tool results use the new <tool_result> XML tag
    if !strings.Contains(prompt, "<tool_result") {
        t.Error("Prompt should contain <tool_result> tag")
    }

    // Check that call_id is present in the tool_result tag
    if !strings.Contains(prompt, `call_id="call_123"`) {
        t.Error("Prompt should contain call_id attribute in tool_result")
    }

    // Check that assistant's tool call is rendered
    if !strings.Contains(prompt, "<<<TOOL_CALL>>>") {
        t.Error("Prompt should contain <<<TOOL_CALL>>> from assistant message")
    }

    // Check that the current_task section contains the last user message
    if !strings.Contains(prompt, "<current_task>") {
        t.Error("Prompt should have <current_task> section")
    }
    if !strings.Contains(prompt, "find my os arch") {
        t.Error("Prompt should contain the final user message in <current_task>")
    }

    // The key improvement: tool results are in <tool_result> tags, clearly
    // separated from assistant messages in <tool_exchange> blocks
    if !strings.Contains(prompt, "<tool_exchange>") {
        t.Error("Prompt should group tool calls with results in <tool_exchange>")
    }

    // Check that the tool result content is visible in the prompt
    if !strings.Contains(prompt, "110.226.238.87") {
        t.Error("Prompt should contain the tool result content (IP address)")
    }

    // Verify the new structure: no [ROLE: ...] tags
    roleMarkers := []string{"[ROLE: system]", "[ROLE: user]", "[ROLE: assistant]"}
    for _, marker := range roleMarkers {
        if strings.Contains(prompt, marker) {
            t.Errorf("Prompt should not contain old %s marker", marker)
        }
    }
}

// TestToolResultVisibility checks that tool results are clearly visible
// in the generated prompt, not buried in a way the model might miss.
func TestToolResultVisibility(t *testing.T) {
    messages := []agentMessage{
        {
            Role:    "user",
            Content: json.RawMessage(`"What is my IP?"`),
        },
        {
            Role:    "assistant",
            Content: json.RawMessage(`""`),
            ToolCalls: []assistantToolCall{
                {
                    ID:   "call_abc",
                    Type: "function",
                    Function: struct {
                        Name      string          `json:"name"`
                        Arguments json.RawMessage `json:"arguments"`
                    }{
                        Name:      "bash",
                        Arguments: json.RawMessage(`{"command":"curl -s https://api.ipify.org"}`),
                    },
                },
            },
        },
        {
            Role:       "tool",
            Content:    json.RawMessage(`"192.168.1.100"`),
            ToolCallID: "call_abc",
        },
    }

    tools := []openAITool{
        {
            Type: "function",
            Function: &openAIFnSpec{
                Name:        "bash",
                Description: "Execute a bash command",
            },
        },
    }

    prompt := buildAgentPrompt(messages, tools)

    if !strings.Contains(prompt, "<tool_result") {
        t.Error("No <tool_result> tag found in prompt")
    }
    if !strings.Contains(prompt, "192.168.1.100") {
        t.Error("Tool result content (IP) not visible in prompt")
    }
    if !strings.Contains(prompt, "<tool_exchange>") {
        t.Error("Tool calls and results should be grouped in <tool_exchange>")
    }
}

// TestMultipleToolResults verifies that multiple tool results are properly
// distinguished in the prompt with call_id attributes.
func TestMultipleToolResults(t *testing.T) {
    messages := []agentMessage{
        {
            Role:    "user",
            Content: json.RawMessage(`"Get my IP and OS"`),
        },
        {
            Role:    "assistant",
            Content: json.RawMessage(`""`),
            ToolCalls: []assistantToolCall{
                {
                    ID:   "call_1",
                    Type: "function",
                    Function: struct {
                        Name      string          `json:"name"`
                        Arguments json.RawMessage `json:"arguments"`
                    }{
                        Name:      "bash",
                        Arguments: json.RawMessage(`{"command":"curl -s https://api.ipify.org"}`),
                    },
                },
                {
                    ID:   "call_2",
                    Type: "function",
                    Function: struct {
                        Name      string          `json:"name"`
                        Arguments json.RawMessage `json:"arguments"`
                    }{
                        Name:      "bash",
                        Arguments: json.RawMessage(`{"command":"uname -a"}`),
                    },
                },
            },
        },
        {
            Role:       "tool",
            Content:    json.RawMessage(`"10.0.0.1"`),
            ToolCallID: "call_1",
        },
        {
            Role:       "tool",
            Content:    json.RawMessage(`"Linux server 5.15.0 x86_64"`),
            ToolCallID: "call_2",
        },
    }

    tools := []openAITool{
        {
            Type: "function",
            Function: &openAIFnSpec{
                Name:        "bash",
                Description: "Execute a bash command",
            },
        },
    }

    prompt := buildAgentPrompt(messages, tools)

    if !strings.Contains(prompt, `call_id="call_1"`) {
        t.Error("Missing call_id=\"call_1\" in tool_result tag")
    }
    if !strings.Contains(prompt, `call_id="call_2"`) {
        t.Error("Missing call_id=\"call_2\" in tool_result tag")
    }
    if !strings.Contains(prompt, "10.0.0.1") {
        t.Error("Missing first tool result content")
    }
    if !strings.Contains(prompt, "Linux server 5.15.0 x86_64") {
        t.Error("Missing second tool result content")
    }
    if !strings.Contains(prompt, "<tool_exchange>") {
        t.Error("Tool calls should be grouped in <tool_exchange>")
    }
}

// TestPromptStructureAnalysis analyzes the new prompt structure and verifies
// it addresses the context rot issues.
func TestPromptStructureAnalysis(t *testing.T) {
    messages := []agentMessage{
        {
            Role:    "system",
            Content: json.RawMessage(`"You are a helpful assistant with access to bash."`),
        },
        {
            Role:    "user",
            Content: json.RawMessage(`"Find my public IP"`),
        },
        {
            Role:    "assistant",
            Content: json.RawMessage(`""`),
            ToolCalls: []assistantToolCall{
                {
                    ID:   "call_ip",
                    Type: "function",
                    Function: struct {
                        Name      string          `json:"name"`
                        Arguments json.RawMessage `json:"arguments"`
                    }{
                        Name:      "bash",
                        Arguments: json.RawMessage(`{"command":"curl -s https://api.ipify.org"}`),
                    },
                },
            },
        },
        {
            Role:       "tool",
            Content:    json.RawMessage(`"203.0.113.42"`),
            ToolCallID: "call_ip",
        },
        {
            Role:    "assistant",
            Content: json.RawMessage(`"Your public IP is 203.0.113.42."`),
        },
        {
            Role:    "user",
            Content: json.RawMessage(`"Now find my OS architecture"`),
        },
    }

    tools := []openAITool{
        {
            Type: "function",
            Function: &openAIFnSpec{
                Name:        "bash",
                Description: "Execute a bash command",
            },
        },
    }

    prompt := buildAgentPrompt(messages, tools)

    // Verify the new XML-like section tags are present
    sections := []string{
        "<system>",
        "<tools>",
        "<recent>",
        "<current_task>",
        "<output_rules>",
    }
    for _, section := range sections {
        if !strings.Contains(prompt, section) {
            t.Errorf("Prompt missing section: %s", section)
        }
    }

    // Verify tool exchange grouping
    if !strings.Contains(prompt, "<tool_exchange>") {
        t.Error("Missing <tool_exchange> for grouped tool calls")
    }
    if !strings.Contains(prompt, "</tool_exchange>") {
        t.Error("Missing closing </tool_exchange>")
    }

    // Verify tool_result has call_id
    if !strings.Contains(prompt, `<tool_result call_id="call_ip"`) {
        t.Error("Missing <tool_result> with call_id")
    }

    // Check for OLD format markers (should NOT be present)
    oldMarkers := []string{
        "[ROLE: system]",
        "[ROLE: user]",
        "[ROLE: assistant]",
        "[ROLE: tool_result]",
        "<<TOOL_RESULT>>",
        "[TOOL CONTRACT]",
    }
    for _, marker := range oldMarkers {
        if strings.Contains(prompt, marker) {
            t.Errorf("Prompt should NOT contain old format marker: %s", marker)
        }
    }
}

// TestMarkerAmbiguity verifies that the new prompt structure has zero
// marker ambiguity — old [ROLE: ...] tags are completely eliminated.
func TestMarkerAmbiguity(t *testing.T) {
    oldRoleTags := []string{
        "[ROLE: system]",
        "[ROLE: user]",
        "[ROLE: assistant]",
        "[ROLE: tool_result]",
        "[ROLE: tool]",
    }

    // Count in the modern system prefix
    for _, tag := range oldRoleTags {
        if count := strings.Count(agentSystemPrefix, tag); count > 0 {
            t.Errorf("System prefix still contains %s (%d times)", tag, count)
        }
    }

    messages := []agentMessage{
        {
            Role:    "user",
            Content: json.RawMessage(`"test"`),
        },
        {
            Role:    "assistant",
            Content: json.RawMessage(`""`),
            ToolCalls: []assistantToolCall{
                {
                    ID:   "call_1",
                    Type: "function",
                    Function: struct {
                        Name      string          `json:"name"`
                        Arguments json.RawMessage `json:"arguments"`
                    }{
                        Name:      "bash",
                        Arguments: json.RawMessage(`{"command":"echo test"}`),
                    },
                },
            },
        },
        {
            Role:       "tool",
            Content:    json.RawMessage(`"test output"`),
            ToolCallID: "call_1",
        },
    }

    tools := []openAITool{
        {
            Type: "function",
            Function: &openAIFnSpec{
                Name:        "bash",
                Description: "Execute a bash command",
            },
        },
    }

    prompt := buildAgentPrompt(messages, tools)

    for _, tag := range oldRoleTags {
        if count := strings.Count(prompt, tag); count > 0 {
            t.Errorf("Full prompt contains %s (%d times) — should be zero", tag, count)
        }
    }

    newMarkers := []string{
        "<system>",
        "<tools>",
        "<recent>",
        "<current_task>",
        "<output_rules>",
        "<tool_exchange>",
        "<tool_result",
        "<assistant>",
    }
    for _, marker := range newMarkers {
        if !strings.Contains(prompt, marker) {
            t.Errorf("Full prompt missing new marker: %s", marker)
        }
    }
    // Note: <user> tag only appears when there are multiple user messages
    // (the last user message goes into <current_task> instead)
}

// TestHistorySummarization verifies that conversations with more than
// maxRecentToolExchanges tool exchanges summarize the older ones into a
// <history_summary> block instead of replaying them verbatim.
func TestHistorySummarization(t *testing.T) {
    var messages []agentMessage
    messages = append(messages, agentMessage{Role: "user", Content: json.RawMessage(`"start"`)})
    // Build maxRecentToolExchanges + 3 tool exchanges.
    for i := 0; i < maxRecentToolExchanges+3; i++ {
        id := "call_sum_" + string(rune('a'+i))
        messages = append(messages,
            agentMessage{
                Role:    "assistant",
                Content: json.RawMessage(`""`),
                ToolCalls: []assistantToolCall{{
                    ID:   id,
                    Type: "function",
                    Function: struct {
                        Name      string          `json:"name"`
                        Arguments json.RawMessage `json:"arguments"`
                    }{
                        Name:      "bash",
                        Arguments: json.RawMessage(`{"command":"step ` + string(rune('a'+i)) + `"}`),
                    },
                }},
            },
            agentMessage{
                Role:       "tool",
                Content:    json.RawMessage(`"result of step ` + string(rune('a'+i)) + `"`),
                ToolCallID: id,
            },
        )
    }
    messages = append(messages, agentMessage{Role: "user", Content: json.RawMessage(`"final question"`)})

    prompt := buildAgentPrompt(messages, []openAITool{{Type: "function", Function: &openAIFnSpec{Name: "bash"}}})

    if !strings.Contains(prompt, "<history_summary>") {
        t.Error("long conversation should produce a <history_summary> block")
    }
    if !strings.Contains(prompt, "Previously completed tool calls:") {
        t.Error("history summary missing header")
    }
    // The oldest exchange result is summarized...
    if !strings.Contains(prompt, "result of step a") {
        t.Error("oldest exchange should appear in the summary")
    }
    // ...while the most recent window stays verbatim in <recent>.
    if !strings.Contains(prompt, "<recent>") {
        t.Error("recent window missing")
    }
    if !strings.Contains(prompt, "<current_task>\nfinal question") {
        t.Error("final user message should anchor <current_task>")
    }
}

// ── GLM-Free-API integration: variant dispatch ───────────────────────────────

// withAgentVariant swaps the configured agent variant for the duration of a
// test and restores it afterwards.
func withAgentVariant(t *testing.T, variant string) {
    t.Helper()
    prevMode, prevVariant := config.AgentMode, config.AgentModeVariant
    config.AgentMode = true
    config.AgentModeVariant = variant
    t.Cleanup(func() {
        config.AgentMode = prevMode
        config.AgentModeVariant = prevVariant
    })
}

func TestAgentVariantSelection(t *testing.T) {
    prevMode, prevVariant := config.AgentMode, config.AgentModeVariant
    defer func() {
        config.AgentMode = prevMode
        config.AgentModeVariant = prevVariant
    }()

    cases := []struct {
        mode       bool
        variant    string
        wantModern bool
    }{
        {true, "modern", true},
        {true, "", true}, // empty defaults to modern
        {true, "legacy", false},
        {true, "LEGACY", false}, // case-insensitive
        {false, "modern", false},
        {false, "legacy", false},
    }
    for _, c := range cases {
        config.AgentMode = c.mode
        config.AgentModeVariant = c.variant
        if got := config.agentModern(); got != c.wantModern {
            t.Errorf("agentModern(mode=%v variant=%q) = %v, want %v", c.mode, c.variant, got, c.wantModern)
        }
        if got := config.agentLegacy(); got != (c.mode && !c.wantModern) {
            t.Errorf("agentLegacy(mode=%v variant=%q) = %v, want %v", c.mode, c.variant, got, c.mode && !c.wantModern)
        }
    }
}

func TestWrapAgentPromptAsMessages(t *testing.T) {
    raw, err := wrapAgentPromptAsMessages("hello agent prompt")
    if err != nil {
        t.Fatalf("wrapAgentPromptAsMessages: %v", err)
    }
    var msgs []map[string]interface{}
    if err := json.Unmarshal(raw, &msgs); err != nil {
        t.Fatalf("unmarshal wrapped messages: %v", err)
    }
    if len(msgs) != 1 {
        t.Fatalf("wrapped messages = %d entries, want 1", len(msgs))
    }
    if msgs[0]["role"] != "user" {
        t.Errorf("wrapped role = %v, want user (Z.AI only accepts user)", msgs[0]["role"])
    }
    if msgs[0]["content"] != "hello agent prompt" {
        t.Errorf("wrapped content = %v, want the prompt verbatim", msgs[0]["content"])
    }
}

func TestAgentTransformMessagesModern(t *testing.T) {
    withAgentVariant(t, "modern")

    messages := json.RawMessage(`[
        {"role":"system","content":"You are helpful."},
        {"role":"user","content":"Find my public ip"},
        {"role":"assistant","content":"","tool_calls":[{"id":"call_9","type":"function","function":{"name":"bash","arguments":"{\"command\":\"curl ifconfig.me\"}"}}]},
        {"role":"tool","tool_call_id":"call_9","content":"203.0.113.7"},
        {"role":"user","content":"Thanks, now get my hostname"}
    ]`)
    tools := json.RawMessage(`[{"type":"function","function":{"name":"bash","description":"Execute a bash command","parameters":{"type":"object","properties":{"command":{"type":"string"}}}}}]`)

    raw, err := agentTransformMessages(messages, tools)
    if err != nil {
        t.Fatalf("agentTransformMessages(modern): %v", err)
    }

    var msgs []map[string]interface{}
    if err := json.Unmarshal(raw, &msgs); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if len(msgs) != 1 {
        t.Fatalf("modern transform produced %d messages, want 1 folded prompt", len(msgs))
    }
    if msgs[0]["role"] != "user" {
        t.Errorf("role = %v, want user", msgs[0]["role"])
    }
    prompt, _ := msgs[0]["content"].(string)
    for _, want := range []string{
        "<system>",
        "<tools>",
        "### Tool 1: bash",
        "<tool_exchange>",
        `call_id="call_9"`,
        "203.0.113.7",
        "<current_task>\nThanks, now get my hostname",
        "<output_rules>",
    } {
        if !strings.Contains(prompt, want) {
            t.Errorf("modern prompt missing %q", want)
        }
    }
    // Legacy markers must not leak into the modern prompt.
    if strings.Contains(prompt, "[ROLE:") {
        t.Error("modern prompt contains legacy [ROLE:] markers")
    }
}

func TestAgentTransformMessagesLegacyStillWorks(t *testing.T) {
    withAgentVariant(t, "legacy")

    messages := json.RawMessage(`[
        {"role":"system","content":"You are helpful."},
        {"role":"user","content":"hi"}
    ]`)
    tools := json.RawMessage(`[{"type":"function","function":{"name":"bash","description":"Execute a bash command"}}]`)

    raw, err := agentTransformMessages(messages, tools)
    if err != nil {
        t.Fatalf("agentTransformMessages(legacy): %v", err)
    }

    var msgs []map[string]interface{}
    if err := json.Unmarshal(raw, &msgs); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    // Legacy: system prefix message + rewritten system + user + tool contract.
    if len(msgs) != 4 {
        t.Fatalf("legacy transform produced %d messages, want 4", len(msgs))
    }
    for _, m := range msgs {
        if m["role"] != "user" {
            t.Errorf("legacy transform left non-user role: %v", m["role"])
        }
    }
    first, _ := msgs[0]["content"].(string)
    if !strings.Contains(first, "[SYSTEM] AGENT MODE") {
        t.Error("legacy first message should carry the legacy system prefix")
    }
    second, _ := msgs[1]["content"].(string)
    if !strings.HasPrefix(second, "[ROLE: system]") {
        t.Errorf("legacy system rewrite = %q, want [ROLE: system] prefix", second)
    }
    last, _ := msgs[3]["content"].(string)
    if !strings.Contains(last, "[TOOL CONTRACT]") || !strings.Contains(last, "### Tool 1: bash") {
        t.Error("legacy tool contract message missing")
    }
}

// TestAgentInterceptorAdapters verifies both interceptor adapters behave
// correctly through the shared agentInterceptor interface.
func TestAgentInterceptorAdapters(t *testing.T) {
    stream := "Sure thing.\n<<<TOOL_CALL>>>\n{\"name\":\"bash\",\"arguments\":{\"command\":\"id\"}}\n<<<END_TOOL_CALL>>>"

    // countCalls counts logical tool calls: deltas carrying an id start a
    // new call (the legacy adapter additionally streams id-less argument
    // fragments for the same call; the modern adapter emits one complete
    // delta per call).
    run := func(in agentInterceptor) (content string, calls int, argFrags int) {
        for i := 0; i < len(stream); i += 4 {
            end := i + 4
            if end > len(stream) {
                end = len(stream)
            }
            c, tcs := in.feed(stream[i:end])
            content += c
            for _, tc := range tcs {
                if id, _ := tc["id"].(string); id != "" {
                    calls++
                } else {
                    argFrags++
                }
            }
        }
        c, tcs := in.finish()
        content += c
        for _, tc := range tcs {
            if id, _ := tc["id"].(string); id != "" {
                calls++
            } else {
                argFrags++
            }
        }
        return
    }

    // Modern adapter: one complete call, no marker leakage.
    content, calls, argFrags := run(&modernAgentInterceptor{in: &AgentStreamInterceptor{}})
    if calls != 1 {
        t.Errorf("modern adapter: %d calls, want 1", calls)
    }
    if argFrags != 0 {
        t.Errorf("modern adapter: %d id-less argument fragments, want 0 (complete-call emission)", argFrags)
    }
    if strings.Contains(content, "TOOL_CALL") {
        t.Errorf("modern adapter leaked markers: %q", content)
    }
    if !strings.Contains(content, "Sure thing.") {
        t.Errorf("modern adapter damaged prose: %q", content)
    }

    // Legacy adapter: same input (canonical markers) also yields one call,
    // streamed incrementally (header delta + argument fragments).
    content, calls, _ = run(&legacyAgentInterceptor{in: newAgentStreamInterceptor()})
    if calls != 1 {
        t.Errorf("legacy adapter: %d calls, want 1", calls)
    }
    if strings.Contains(content, "TOOL_CALL") {
        t.Errorf("legacy adapter leaked markers: %q", content)
    }
}

// TestAgentExtractStripDispatch checks the extract/strip dispatch helpers
// route to the right implementation per variant.
func TestAgentExtractStripDispatch(t *testing.T) {
    // Flat payload: the MODERN tolerant parser resolves the tool name and
    // folds the stray keys into arguments.
    withAgentVariant(t, "modern")
    calls := agentExtractToolCalls(flatPayload)
    if len(calls) != 1 {
        t.Fatalf("modern extract: %d calls for flat payload, want 1", len(calls))
    }
    if fn, _ := calls[0]["function"].(map[string]interface{}); fn["name"] != "bash" {
        t.Errorf("modern extract: name = %v, want bash", fn["name"])
    }
    if got := agentStripToolCalls(flatPayload); strings.TrimSpace(got) != "" {
        t.Errorf("modern strip left residue: %q", got)
    }

    withAgentVariant(t, "legacy")
    // The legacy strict parser accepts the flat JSON but cannot resolve a
    // tool name from it (name stays "") — exactly the failure mode the
    // modern tolerant parser fixes.
    legacyCalls := agentExtractToolCalls(flatPayload)
    for _, tc := range legacyCalls {
        fn, _ := tc["function"].(map[string]interface{})
        if name, _ := fn["name"].(string); name != "" {
            t.Errorf("legacy extract unexpectedly resolved name %q from flat payload", name)
        }
    }
    // Canonical payload works on legacy too.
    canonical := "<<<TOOL_CALL>>>\n{\"name\":\"bash\",\"arguments\":{\"command\":\"id\"}}\n<<<END_TOOL_CALL>>>"
    if calls := agentExtractToolCalls(canonical); len(calls) != 1 {
        t.Errorf("legacy extract: %d calls for canonical payload, want 1", len(calls))
    }
    if got := agentStripToolCalls(canonical); strings.TrimSpace(got) != "" {
        t.Errorf("legacy strip left residue: %q", got)
    }
}
