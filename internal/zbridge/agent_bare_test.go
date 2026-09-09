package zbridge

import (
    "strconv"
    "encoding/json"
    "strings"
    "testing"
)

// Captured live from LOG_LEVEL=debug: glm-5.3 closed its <details> block and
// wrote the call straight into the answer with no envelope at all. One stream
// in five took this shape; the other four used the canonical markers.
const bareCapture = "\nbash(i=\"Inspecting today's upstream commit for overlap with local patches\", " +
    "command=\"git -C ../aistudio2api show --stat --oneline origin/main | head -30\")"

var bareTools = []string{"bash", "read_file"}

func bareCalls(t *testing.T, text string, names []string) []map[string]interface{} {
    t.Helper()
    return ParseAgentToolCalls(TranslateBareToolCalls(text, names))
}

func bareArgs(t *testing.T, call map[string]interface{}) map[string]interface{} {
    t.Helper()
    fn := call["function"].(map[string]interface{})
    var args map[string]interface{}
    if err := json.Unmarshal([]byte(fn["arguments"].(string)), &args); err != nil {
        t.Fatalf("arguments are not valid JSON: %v", err)
    }
    return args
}

func TestBareCallFromLiveCapture(t *testing.T) {
    calls := bareCalls(t, "Ich sehe nach.\n"+bareCapture, bareTools)
    if len(calls) != 1 {
        t.Fatalf("want 1 tool call, got %d", len(calls))
    }
    fn := calls[0]["function"].(map[string]interface{})
    if fn["name"] != "bash" {
        t.Fatalf("want name bash, got %v", fn["name"])
    }
    args := bareArgs(t, calls[0])
    if args["command"] != "git -C ../aistudio2api show --stat --oneline origin/main | head -30" {
        t.Fatalf("command lost: %#v", args["command"])
    }
    if args["i"] != "Inspecting today's upstream commit for overlap with local patches" {
        t.Fatalf("intent lost: %#v", args["i"])
    }
    got := StripAgentToolCalls(TranslateBareToolCalls("Ich sehe nach.\n"+bareCapture, bareTools))
    if strings.Contains(got, "bash(") {
        t.Fatalf("call leaked into content: %q", got)
    }
    if !strings.Contains(got, "Ich sehe nach.") {
        t.Fatalf("ordinary content lost: %q", got)
    }
}

// The whole safety of this path is that the name must be one the request
// declared. Prose that merely looks like a call must survive untouched.
func TestBareCallRequiresDeclaredTool(t *testing.T) {
    for _, names := range [][]string{nil, {}, {"read_file"}, {"web_search"}} {
        if calls := bareCalls(t, bareCapture, names); len(calls) != 0 {
            t.Fatalf("names %v: undeclared tool produced %d calls", names, len(calls))
        }
        if got := TranslateBareToolCalls(bareCapture, names); got != bareCapture {
            t.Fatalf("names %v: text rewritten: %q", names, got)
        }
    }
}

func TestBareCallRequiresLineStart(t *testing.T) {
    for _, prose := range []string{
        `Du kannst bash(command="ls") benutzen, um das zu sehen.`,
        "Der Aufruf ist dann bash(command=\"ls\").",
        "`bash(command=\"ls\")` als Beispiel.",
    } {
        if calls := bareCalls(t, prose, bareTools); len(calls) != 0 {
            t.Fatalf("prose %q produced %d calls", prose, len(calls))
        }
    }
}

// Leading indentation still counts as the start of a line: the models indent
// the call under a list item often enough.
func TestBareCallAllowsLeadingIndent(t *testing.T) {
    if calls := bareCalls(t, "Schritt 1:\n    bash(command=\"ls\")", bareTools); len(calls) != 1 {
        t.Fatalf("indented call not recognised: %d calls", len(calls))
    }
}

// A call whose argument list never closes must stay text rather than become a
// command the model never finished writing.
func TestBareCallTruncatedStaysText(t *testing.T) {
    for _, truncated := range []string{
        "\nbash(command=\"rm -rf /home/finn/Proj",
        "\nbash(command=\"ls\", cwd=",
        "\nbash(",
        "\nbash",
    } {
        if calls := bareCalls(t, truncated, bareTools); len(calls) != 0 {
            t.Fatalf("truncated %q produced %d calls", truncated, len(calls))
        }
        if got := TranslateBareToolCalls(truncated, bareTools); got != truncated {
            t.Fatalf("truncated %q rewritten: %q", truncated, got)
        }
    }
}

// Brackets and quotes inside the command must not end the argument list early.
func TestBareCallNestedBrackets(t *testing.T) {
    in := "\nbash(command=\"awk '{print $1}' f.txt | grep -E '(a|b)' \", i=\"filter\")"
    calls := bareCalls(t, in, bareTools)
    if len(calls) != 1 {
        t.Fatalf("want 1 call, got %d", len(calls))
    }
    if cmd := bareArgs(t, calls[0])["command"]; !strings.Contains(cmd.(string), "{print $1}") {
        t.Fatalf("command truncated at a bracket: %q", cmd)
    }
}

func TestBareCallJSONArguments(t *testing.T) {
    calls := bareCalls(t, "\nbash{\"command\":\"ls -la\"}", bareTools)
    if len(calls) != 1 {
        t.Fatalf("want 1 call, got %d", len(calls))
    }
    if bareArgs(t, calls[0])["command"] != "ls -la" {
        t.Fatalf("JSON arguments lost")
    }
}

// An explicit envelope always wins: it is the documented shape and it streams.
func TestBareCallCanonicalWins(t *testing.T) {
    in := agentToolStart + `{"name":"read_file","arguments":{"path":"a.txt"}}` + agentToolEnd +
        "\nbash(command=\"ls\")"
    calls := bareCalls(t, in, bareTools)
    if len(calls) != 2 {
        t.Fatalf("want 2 calls, got %d", len(calls))
    }
    if calls[0]["function"].(map[string]interface{})["name"] != "read_file" {
        t.Fatalf("canonical block did not keep its place")
    }
}

// ── streaming ────────────────────────────────────────────────────────────────

func feedChunksTools(t *testing.T, names []string, chunks []string, step int) (string, int, string) {
    t.Helper()
    in := &AgentStreamInterceptor{}
    in.setToolNames(names)
    var b, acc strings.Builder
    calls := 0
    collect := func(parsed AgentParsedChunk) {
        b.WriteString(parsed.Content)
        for _, call := range parsed.ToolCalls {
            if id, _ := call["id"].(string); id != "" {
                calls++
            }
            fn := call["function"].(map[string]interface{})
            frag, _ := fn["arguments"].(string)
            acc.WriteString(frag)
        }
    }
    for i := 0; i < len(chunks); i += step {
        piece := ""
        for j := i; j < i+step && j < len(chunks); j++ {
            piece += chunks[j]
        }
        collect(in.Feed(piece))
    }
    collect(in.Finish())
    return b.String(), calls, acc.String()
}

func TestStreamBareCall(t *testing.T) {
    for _, size := range []int{1, 3, 7, 13, 64, 4096} {
        content, calls, args := feedChunksTools(t, bareTools,
            splitRunes("Ich sehe nach."+bareCapture, size), 1)
        if calls != 1 {
            t.Fatalf("size %d: want 1 call, got %d (content %q)", size, calls, content)
        }
        if !strings.Contains(args, "show --stat --oneline") {
            t.Fatalf("size %d: arguments lost: %q", size, args)
        }
        if strings.Contains(content, "bash(") {
            t.Fatalf("size %d: call leaked as content: %q", size, content)
        }
        if !strings.Contains(content, "Ich sehe nach.") {
            t.Fatalf("size %d: ordinary content lost: %q", size, content)
        }
    }
}

// Without a tool list the streaming path must behave exactly as before.
func TestStreamBareCallInertWithoutTools(t *testing.T) {
    for _, size := range []int{1, 7, 4096} {
        content, calls, _ := feedChunksTools(t, nil, splitRunes(bareCapture, size), 1)
        if calls != 0 {
            t.Fatalf("size %d: produced %d calls without a tool list", size, calls)
        }
        if !strings.Contains(content, "bash(") {
            t.Fatalf("size %d: content swallowed: %q", size, content)
        }
    }
}

// A truncated call at end of stream must surface rather than vanish.
func TestStreamBareCallTruncatedNotSwallowed(t *testing.T) {
    content, calls, _ := feedChunksTools(t, bareTools,
        splitRunes("prefix\nbash(command=\"git -C ../ai", 5), 1)
    if calls != 0 {
        t.Fatalf("truncated stream produced %d calls", calls)
    }
    if !strings.Contains(content, "prefix") {
        t.Fatalf("content before the truncated call lost: %q", content)
    }
}

// ── through the real SSE parser ──────────────────────────────────────────────

// Shaped after a live LOG_LEVEL=debug capture: one SSE event ended with
//
//	…).\n</details>\nbash
//
// and the next carried `(i="…", command="git -C ../a`, with an edit_content
// event completing the command afterwards. The name, the opening bracket and
// the argument list therefore arrive in three separate upstream events, which
// is what makes this worth running through streamSSEResponse rather than
// asserting on a finished string.
//
// The events are synthetic: the captured log carries no request id, so lines
// from concurrent requests cannot be separated with confidence. Their shape is
// taken from the capture; their isolation is not.
func TestBareCallThroughSSEPipeline(t *testing.T) {
    quote := func(s string) string {
        b, err := json.Marshal(s)
        if err != nil {
            t.Fatalf("marshal: %v", err)
        }
        return string(b)
    }
    part1 := "<details type=\"reasoning\" done=\"true\">\n> Checking the upstream commit.\n</details>\nbash"
    part2 := "(i=\"Inspecting the upstream commit\", command=\"git -C ../a"
    part3 := "istudio2api show --stat --oneline origin/main | head -30\")"

    // part1+part2 is pure ASCII, so its UTF-16 length is its byte length.
    editIndex := len(part1) + len(part2)

    body := sseEvent(`{"type":"chat:completion","data":{"delta_content":`+quote(part1)+`,"phase":"answer"}}`) +
        sseEvent(`{"type":"chat:completion","data":{"delta_content":`+quote(part2)+`,"phase":"answer"}}`) +
        sseEvent(`{"type":"chat:completion","data":{"edit_index":`+itoa(editIndex)+`,"edit_content":`+quote(part3)+`,"phase":"other"}}`) +
        sseEvent(`{"type":"chat:completion","data":{"phase":"done","done":true}}`)

    in := &AgentStreamInterceptor{}
    in.setToolNames([]string{"bash"})
    var content, args strings.Builder
    calls := 0
    collect := func(p AgentParsedChunk) {
        content.WriteString(p.Content)
        for _, call := range p.ToolCalls {
            if id, _ := call["id"].(string); id != "" {
                calls++
            }
            fn := call["function"].(map[string]interface{})
            frag, _ := fn["arguments"].(string)
            args.WriteString(frag)
        }
    }
    for _, r := range runSSEParser(t, body) {
        if r.Chunk != "" {
            collect(in.Feed(r.Chunk))
        }
    }
    collect(in.Finish())

    if calls != 1 {
        t.Fatalf("want 1 tool call, got %d (content %q)", calls, content.String())
    }
    if !strings.Contains(args.String(), "show --stat --oneline origin/main") {
        t.Fatalf("command lost across the edit event: %q", args.String())
    }
    if strings.Contains(content.String(), "bash(") {
        t.Fatalf("call leaked as content: %q", content.String())
    }
}

func itoa(n int) string { return strconv.Itoa(n) }

// ── fence-awareness (security boundary: doc examples must not fire) ─────────

// A declared-tool invocation written purely as a documentation example
// inside a fenced code block must stay text, in both argument forms. Without
// fence-awareness, atLineStart + a declared name + complete arguments is
// indistinguishable from a real call, so a model explaining its own tool
// syntax could trigger a real invocation.
func TestBareCallInsideFencedDocExample(t *testing.T) {
    cases := []string{
        "Example:\n```text\nweb_search{\"query\":\"hello\"}\n```\n",
        "Example:\n```text\nweb_search(query=\"test\", limit=1)\n```\n",
    }
    names := []string{"web_search"}
    for _, in := range cases {
        if calls := bareCalls(t, in, names); len(calls) != 0 {
            t.Fatalf("fenced example fired as a call: %q -> %d calls", in, len(calls))
        }
        if got := TranslateBareToolCalls(in, names); got != in {
            t.Fatalf("fenced example rewritten: %q -> %q", in, got)
        }
    }
}

// A fenced example must not blind the parser to a real call in the same
// message once the fence closes — skip the fenced candidate, keep scanning.
func TestBareCallSkipsFencedThenFindsReal(t *testing.T) {
    in := "Example:\n```text\nweb_search(query=\"documentation\")\n```\n" +
        "\nweb_search(query=\"real\")"
    names := []string{"web_search"}
    calls := bareCalls(t, in, names)
    if len(calls) != 1 {
        t.Fatalf("want 1 call (the one after the fence), got %d", len(calls))
    }
    if args := bareArgs(t, calls[0]); args["query"] != "real" {
        t.Fatalf("wrong call fired: %#v", args)
    }
}

// ── quoted-argument isolation (``` inside call data is not a fence) ─────────

// A ``` sequence that is argument data — e.g. a heredoc payload for a real
// shell call — must not be mistaken for a markdown fence. If it were, it
// would flip in.bareFenceOpen and could make the genuinely real call right
// after it register as "inside a fence" and get skipped.
func TestBareCallBacktickInQuotedArgDoesNotToggleFence(t *testing.T) {
    in := "bash(command=\"cat <<'EOF'\n```python\nprint('hi')\n```\nEOF\")\n" +
        "\nweb_search(query=\"hello\")"
    names := []string{"bash", "web_search"}
    calls := bareCalls(t, in, names)
    if len(calls) != 2 {
        t.Fatalf("want 2 calls (bash and web_search), got %d: %#v", len(calls), calls)
    }
    if fn := calls[0]["function"].(map[string]interface{}); fn["name"] != "bash" {
        t.Fatalf("first call = %v, want bash", fn["name"])
    }
    if cmd := bareArgs(t, calls[0])["command"]; !strings.Contains(cmd.(string), "```python") {
        t.Fatalf("heredoc payload truncated: %q", cmd)
    }
    if fn := calls[1]["function"].(map[string]interface{}); fn["name"] != "web_search" {
        t.Fatalf("second call = %v, want web_search — the heredoc's ``` wrongly toggled fence state", fn["name"])
    }
}

// Mirror image: a real fence around a documentation example must still be
// caught even when the fenced text itself looks like a call with a quoted
// argument, so quote-awareness cannot be used to suppress fence detection
// generally.
func TestBareCallRealFenceStillCaughtNearQuotedExample(t *testing.T) {
    in := "```text\nbash(command=\"echo hello\")\n```\n" +
        "\nweb_search(query=\"hello\")"
    names := []string{"bash", "web_search"}
    calls := bareCalls(t, in, names)
    if len(calls) != 1 {
        t.Fatalf("want 1 call (web_search only), got %d: %#v", len(calls), calls)
    }
    if calls[0]["function"].(map[string]interface{})["name"] != "web_search" {
        t.Fatalf("fenced bash example fired as a call")
    }
}

// ── fence state across a mid-stream rearm (issue #23 interaction) ───────────

// A ``` fence opened before an upstream deep edit_content rewind must still
// be open after rearmAgentInterceptor rebuilds the interceptor, or a
// documentation example split across the rewind boundary fires as a real
// call. This is the scenario that made the bug hard to pin down in the
// field (a long capture full of edit_content events): the fence check alone
// is not sufficient without this.
func TestBareCallFenceStatePreservedAcrossRearm(t *testing.T) {
    withAgentVariant(t, "modern")

    names := []string{"bash"}
    live := newAgentInterceptor(names)

    // Open a fence and pad well past the streaming hold-back window so the
    // fence line itself is safely committed, not sitting in the tail this
    // Feed call keeps buffered for the next one.
    content1, calls1 := live.feed(
        "Here is the syntax:\n```\n" +
            strings.Repeat("padding so the fence line clears the hold-back window. ", 3) + "\n")
    if len(calls1) != 0 {
        t.Fatalf("part 1 produced %d calls before any rewind", len(calls1))
    }

    // Simulate the upstream deep rewind (issue #23): the caller detects
    // FullText no longer extends what was already forwarded and swaps in a
    // fresh interceptor.
    live = rearmAgentInterceptor(live)

    // The model restates the example after the rewind, still inside the
    // fence opened in part 1. If bareFenceOpen was lost on rearm, this
    // reads as a real, complete call and fires — with a destructive
    // argument, so a false fire here is not a subtle test failure.
    content2, calls2 := live.feed("bash(command=\"rm -rf /\")\n```\n")
    if len(calls2) != 0 {
        t.Fatalf("documentation example fired as a call after rearm: %d calls (content so far: %q)",
            len(calls2), content1+content2)
    }

    // Once the fence actually closes and a real, unfenced call follows, it
    // must still be recognised — the fix must not simply wedge fenceOpen at
    // true forever.
    content3, calls3 := live.feed("\nbash(command=\"ls\")")
    finalContent, finalCalls := live.finish()
    if len(calls3)+len(finalCalls) != 1 {
        t.Fatalf("want exactly 1 real call after the fence closed, got %d+%d (content so far: %q)",
            len(calls3), len(finalCalls), content1+content2+content3+finalContent)
    }
}
