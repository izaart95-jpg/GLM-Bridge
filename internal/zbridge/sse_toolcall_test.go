// sse_toolcall_test.go
//
// End-to-end verification of STREAMED tool calls (streamable tool calls):
// a synthetic Z.AI upstream SSE stream containing a tool-call block is run
// through the REAL streamSSEResponse parser, replayed through the exact
// interceptor + delta-emission logic of chatCompletionsHandler, and the
// client-visible SSE chunks are asserted against the OpenAI streaming
// contract:
//
//   - the tool-call header (index + id + type + function.name) is emitted
//     WHILE the model is still writing the block — before <<<END_TOOL_CALL>>>
//     ever arrives upstream (the old behaviour went silent and only emitted
//     one buffered blob after the block closed);
//   - arguments arrive as incremental function.arguments fragments that an
//     OpenAI SDK reassembles by index into the exact JSON object;
//   - the stream closes with finish_reason "tool_calls";
//   - every emitted fragment is valid UTF-8 (no issue #23 garble).

package zbridge

import (
    "encoding/json"
    "fmt"
    "math/rand"
    "strings"
    "testing"
    "unicode/utf8"
)

// sseToolCallTestCase drives the full pipeline: upstream Z.AI SSE bytes →
// real parser → real interceptor → handler-identical SSE emission.
//
// It returns every emitted tool-call delta (in order), the number of logical
// tool calls (header deltas), and — crucially — how many tool-call deltas
// were already emitted while the upstream was still INSIDE the block
// (before the bytes "END_TOOL_CALL" had been fed to the interceptor).
func sseToolCallStream(t *testing.T, upstreamText string, chunkEvery int) (
    toolDeltas []map[string]interface{},
    calls int,
    deltasBeforeBlockClose int,
    finalContent string,
) {
    t.Helper()

    // 1. Build the upstream SSE body, chunking the assistant text into
    //    delta_content events exactly like Z.AI does.
    var body strings.Builder
    for i := 0; i < len(upstreamText); i += chunkEvery {
        end := i + chunkEvery
        if end > len(upstreamText) {
            end = len(upstreamText)
        }
        piece := upstreamText[i:end]
        payload, _ := json.Marshal(map[string]interface{}{"delta_content": piece})
        fmt.Fprintf(&body, "data: {\"data\":%s}\n\n", string(payload))
    }
    body.WriteString(sseEvent(`{"data":{"phase":"done"}}`))
    body.WriteString("data: [DONE]\n\n")

    // 2. Run the real upstream parser.
    results := runSSEParser(t, body.String())

    // 3. Replay the exact chatCompletionsHandler stream logic (agent-mode
    //    branch): fullContent accumulation + interceptor feed + tool-call
    //    delta emission.
    interceptor := newAgentInterceptor()
    fullContent := ""
    sawEndMarker := false
    for _, result := range results {
        if result.Reasoning != "" {
            continue
        }
        if result.FullText != "" {
            fullContent = result.FullText
        } else {
            fullContent += result.Chunk
        }
        delta := result.Chunk
        if delta == "" {
            continue
        }
        if strings.Contains(fullContent, "END_TOOL_CALL") {
            sawEndMarker = true // the handler has now SEEN the closing marker
        }
        contentDelta, toolCalls := interceptor.feed(delta)
        if contentDelta != "" {
            finalContent += contentDelta
        }
        for _, tc := range toolCalls {
            if id, _ := tc["id"].(string); id != "" {
                calls++
            }
            if !sawEndMarker {
                deltasBeforeBlockClose++
            }
            toolDeltas = append(toolDeltas, tc)
        }
    }
    rem, tailCalls := interceptor.finish()
    finalContent += rem
    for _, tc := range tailCalls {
        if id, _ := tc["id"].(string); id != "" {
            calls++
        }
        toolDeltas = append(toolDeltas, tc)
    }
    return toolDeltas, calls, deltasBeforeBlockClose, finalContent
}

// TestSSEToolCallsStreamWhileBlockIsOpen proves the fix end to end: the
// client must see tool-call SSE deltas while the model is still writing the
// tool-call block — not one buffered blob after <<<END_TOOL_CALL>>>.
func TestSSEToolCallsStreamWhileBlockIsOpen(t *testing.T) {
    withAgentVariant(t, "modern") // pin the default modern shim
    // The exact scenario from the goal: a calculate tool with an arguments
    // object the model writes out incrementally.
    upstream := "<<<TOOL_CALL>>>\n" +
        `{"name":"calculate","arguments":{"operation": "multiply", "a": 234, "b": 567}}` +
        "\n<<<END_TOOL_CALL>>>"

    toolDeltas, calls, beforeClose, content := sseToolCallStream(t, upstream, 13)
    if calls != 1 {
        t.Fatalf("logical tool calls = %d, want 1", calls)
    }
    if beforeClose == 0 {
        t.Fatal("no tool-call delta streamed before <<<END_TOOL_CALL>>> arrived — buffered behaviour regressed")
    }

    // Header delta carries the OpenAI fields: index, id, type, name.
    var header map[string]interface{}
    headerIndex := 0
    for _, tc := range toolDeltas {
        if id, _ := tc["id"].(string); id != "" {
            header = tc
            if idx, ok := tc["index"].(int); ok {
                headerIndex = idx
            }
        }
    }
    if header == nil {
        t.Fatal("no header delta emitted")
    }
    if id, _ := header["id"].(string); !strings.HasPrefix(id, "call_") {
        t.Errorf("header id = %v, want call_*", header["id"])
    }
    if header["type"] != "function" {
        t.Errorf("header type = %v, want function", header["type"])
    }
    fn, _ := header["function"].(map[string]interface{})
    if fn["name"] != "calculate" {
        t.Errorf("header name = %v, want calculate", fn["name"])
    }

    // Fragments reassemble by index into the exact arguments JSON.
    argsByIndex := map[int]string{}
    for _, tc := range toolDeltas {
        idx, _ := tc["index"].(int)
        fn, _ := tc["function"].(map[string]interface{})
        frag, _ := fn["arguments"].(string)
        if !utf8.ValidString(frag) {
            t.Fatalf("argument fragment is invalid UTF-8 (client garble): %q", frag)
        }
        argsByIndex[idx] += frag
    }
    got := argsByIndex[headerIndex]
    want := `{"operation": "multiply", "a": 234, "b": 567}`
    if got != want {
        t.Errorf("reassembled arguments = %q, want %q", got, want)
    }

    // No marker or JSON leaks as content.
    if strings.Contains(content, "TOOL_CALL") || strings.Contains(content, "calculate") {
        t.Errorf("tool-call block leaked as content: %q", content)
    }
}

// TestSSEToolCallsByteSplitFragments: worst-case upstream chunking (a few
// bytes per event) — the streamed arguments must still reassemble exactly.
func TestSSEToolCallsByteSplitFragments(t *testing.T) {
    withAgentVariant(t, "modern")
    upstream := "<<<TOOL_CALL>>>\n" +
        `{"name":"calculate","arguments":{"a": 234, "b": 567, "operation": "multiply"}}` +
        "\n<<<END_TOOL_CALL>>>"
    toolDeltas, calls, beforeClose, content := sseToolCallStream(t, upstream, 4)
    if calls != 1 {
        t.Fatalf("logical tool calls = %d, want 1", calls)
    }
    if beforeClose == 0 {
        t.Fatal("no delta streamed before the block closed")
    }
    var idx0 string
    for _, tc := range toolDeltas {
        if tc["index"] == 0 {
            fn, _ := tc["function"].(map[string]interface{})
            idx0 += fn["arguments"].(string)
        }
    }
    if idx0 != `{"a": 234, "b": 567, "operation": "multiply"}` {
        t.Errorf("reassembled arguments = %q", idx0)
    }
    if strings.Contains(content, "TOOL_CALL") {
        t.Errorf("marker leaked as content: %q", content)
    }
}

// TestSSEToolCallNonStreamParity: the same upstream text must parse to the
// equivalent complete call on the non-stream path (agentExtractToolCalls),
// i.e. streaming did not change the finished-text contract. (The
// finished-text parser compacts the JSON; the streamed arguments preserve the
// model's spacing verbatim, so compare the decoded objects.)
func TestSSEToolCallNonStreamParity(t *testing.T) {
    text := "<<<TOOL_CALL>>>\n{\"name\":\"calculate\",\"arguments\":{\"a\": 234, \"b\": 567, \"operation\": \"multiply\"}}\n<<<END_TOOL_CALL>>>"
    calls := ParseAgentToolCalls(text)
    if len(calls) != 1 {
        t.Fatalf("ParseAgentToolCalls => %d calls", len(calls))
    }
    fn, _ := calls[0]["function"].(map[string]interface{})
    if fn["name"] != "calculate" {
        t.Errorf("name = %v", fn["name"])
    }
    var want, got map[string]interface{}
    if err := json.Unmarshal([]byte(`{"a":234,"b":567,"operation":"multiply"}`), &want); err != nil {
        t.Fatal(err)
    }
    if err := json.Unmarshal([]byte(fn["arguments"].(string)), &got); err != nil {
        t.Fatalf("non-stream arguments are not valid JSON: %v", err)
    }
    if fmt.Sprint(got) != fmt.Sprint(want) {
        t.Errorf("arguments = %v, want %v", got, want)
    }
}

// TestStreamChunkSweepInvariants sweeps EVERY chunk size and randomized
// split patterns through the interceptor and asserts the streaming
// invariants hold at every boundary:
//   1. exactly one header delta per block, with a stable id and name;
//   2. fragments reassemble, by index, into the exact arguments JSON;
//   3. every fragment is valid UTF-8 (issue #23);
//   4. no marker/JSON bytes leak as content.
func TestStreamChunkSweepInvariants(t *testing.T) {
    withAgentVariant(t, "modern")
    streams := []string{
        "<<<TOOL_CALL>>>\n{\"name\":\"calculate\",\"arguments\":{\"a\": 234, \"b\": 567, \"operation\": \"multiply\"}}\n<<<END_TOOL_CALL>>>",
        "Reasoning first.\n<<<TOOL_CALL>>>\n{\"name\":\"translate\",\"arguments\":{\"text\":\"你好，世界\",\"to\":\"en\"}}\n<<<END_TOOL_CALL>>>\nafter",
        "<<TOOL_CALL>>>\n{\"tool\":\"bash\",\"parameters\":{\"command\":\"uname -m\",\"timeout\":10}}\n<<<END_TOOL_CALL>>>",
        "<<<TOOL_CALL>>>\n{\"name\":\"echo\",\"arguments\":{}}\n<<<END_TOOL_CALL>>>",
    }
    wantArgs := []string{
        `{"a": 234, "b": 567, "operation": "multiply"}`,
        `{"text":"你好，世界","to":"en"}`,
        `{"command":"uname -m","timeout":10}`,
        `{}`,
    }

    for si, stream := range streams {
        for size := 1; size <= len(stream); size++ {
            chunks := splitRunes(stream, size)
            in := &AgentStreamInterceptor{}
            var content strings.Builder
            argsByIndex := map[int]string{}
            headers := 0
            var name, id string
            collect := func(tcs []map[string]interface{}) {
                for _, tc := range tcs {
                    idx, _ := tc["index"].(int)
                    fn, _ := tc["function"].(map[string]interface{})
                    frag, _ := fn["arguments"].(string)
                    if !utf8.ValidString(frag) {
                        t.Fatalf("stream %d size %d: invalid UTF-8 fragment %q", si, size, frag)
                    }
                    argsByIndex[idx] += frag
                    if tcid, _ := tc["id"].(string); tcid != "" {
                        headers++
                        id = tcid
                        name, _ = fn["name"].(string)
                    }
                }
            }
            for _, piece := range chunks {
                parsed := in.Feed(piece)
                content.WriteString(parsed.Content)
                collect(parsed.ToolCalls)
            }
            f := in.Finish()
            content.WriteString(f.Content)
            collect(f.ToolCalls)

            if headers != 1 {
                t.Fatalf("stream %d size %d: %d header deltas, want 1", si, size, headers)
            }
            if name == "" || id == "" {
                t.Fatalf("stream %d size %d: header lost name/id", si, size)
            }
            if got := argsByIndex[0]; got != wantArgs[si] {
                t.Fatalf("stream %d size %d: reassembled %q, want %q", si, size, got, wantArgs[si])
            }
            out := content.String()
            if strings.Contains(out, "TOOL_CALL") {
                t.Fatalf("stream %d size %d: marker leaked: %q", si, size, out)
            }
        }
    }

    // Randomized chunk boundaries (deterministic seed).
    rnd := rand.New(rand.NewSource(234567))
    for trial := 0; trial < 200; trial++ {
        stream := streams[trial%len(streams)]
        want := wantArgs[trial%len(wantArgs)]
        in := &AgentStreamInterceptor{}
        args := ""
        headers := 0
        for pos := 0; pos < len(stream); {
            n := 1 + rnd.Intn(12)
            if pos+n > len(stream) {
                n = len(stream) - pos
            }
            parsed := in.Feed(stream[pos : pos+n])
            for _, tc := range parsed.ToolCalls {
                fn, _ := tc["function"].(map[string]interface{})
                frag, _ := fn["arguments"].(string)
                if !utf8.ValidString(frag) {
                    t.Fatalf("trial %d: invalid UTF-8 fragment %q", trial, frag)
                }
                args += frag
                if id, _ := tc["id"].(string); id != "" {
                    headers++
                }
            }
            pos += n
        }
        f := in.Finish()
        for _, tc := range f.ToolCalls {
            fn, _ := tc["function"].(map[string]interface{})
            args += fn["arguments"].(string)
            if id, _ := tc["id"].(string); id != "" {
                headers++
            }
        }
        if headers != 1 {
            t.Fatalf("trial %d: %d headers, want 1", trial, headers)
        }
        if args != want {
            t.Fatalf("trial %d: reassembled %q, want %q", trial, args, want)
        }
    }
}
