package zbridge

import (
    "encoding/json"
    "strings"
)

// Zhipu's models sometimes drop the envelope entirely and write the call as a
// plain function invocation at the start of a line, most often right after a
// </details> block closes:
//
//	</details>
//	bash(i="Inspecting the upstream commit", command="git -C ../repo show --stat")
//
// Neither the canonical <<<TOOL_CALL>>> markers nor Zhipu's own <tool_call>
// tags are present, so nothing downstream recognises it and the line reaches
// the client as assistant text. Captured live: one stream in five took this
// shape while the other four used the canonical markers.
//
// Recognising a bare call is only safe because the request declares its tools.
// `grep(pattern="x")` in prose is indistinguishable from a call by shape
// alone, so the name is required to match a declared tool, the invocation
// must start a line, and its argument list must be complete and parseable.
// Without a tool list nothing here fires at all.

// agentToolNames extracts the declared function names from an OpenAI-format
// tools array. Order is preserved; malformed entries are skipped.
func agentToolNames(raw json.RawMessage) []string {
    if len(raw) == 0 {
        return nil
    }
    var tools []struct {
        Function struct {
            Name string `json:"name"`
        } `json:"function"`
    }
    if json.Unmarshal(raw, &tools) != nil {
        return nil
    }
    var names []string
    for _, t := range tools {
        if t.Function.Name != "" {
            names = append(names, t.Function.Name)
        }
    }
    return names
}

// agentLongestName reports the longest declared tool name, used to size the
// streaming hold-back window so a name split across chunks is never flushed
// as content before its opening parenthesis arrives.
func agentLongestName(names []string) int {
    longest := 0
    for _, n := range names {
        if len(n) > longest {
            longest = len(n)
        }
    }
    return longest
}

// bareCall describes one envelope-less invocation found in a text.
type bareCall struct {
    start, end int
    name       string
    args       map[string]interface{}
}

// atLineStart reports whether index i begins a line in s, ignoring horizontal
// whitespace. prevByte is the byte immediately before s when s is a slice of a
// larger buffer (0 when s starts the buffer).
func atLineStart(s string, i int, prevByte byte) bool {
    for i > 0 {
        c := s[i-1]
        if c == ' ' || c == '\t' {
            i--
            continue
        }
        return c == '\n' || c == '\r'
    }
    return prevByte == 0 || prevByte == '\n' || prevByte == '\r'
}

// fenceDelimLine reports whether line is a markdown code-fence delimiter:
// optional leading horizontal whitespace followed by at least three
// backticks. Both the opening line (which may carry a language tag after the
// backticks, e.g. "```go") and the closing line (bare backticks) match — a
// fence toggles either way.
//
// This deliberately does not implement the rest of CommonMark's fence rules
// (matching backtick/tilde counts, ~~~ fences, 4-space indented code blocks).
// Every model capture examined for this fix used plain ``` fences; extending
// coverage to those forms is a separate, larger change and is out of scope
// here — see the PR description for the reasoning.
func fenceDelimLine(line string) bool {
    return strings.HasPrefix(strings.TrimLeft(line, " \t"), "```")
}

// advanceFenceState folds free text s into the running markdown-fence state,
// toggling once per fence-delimiter line (see fenceDelimLine). open is the
// state entering s; the return value is the state after it.
//
// Callers must use this — never re-derive fence state from a suffix window
// of the stream — because a fence opened in text already committed to the
// client is invisible to any later scan that only looks at what has not yet
// been sent.
func advanceFenceState(s string, open bool) bool {
    for len(s) > 0 {
        nl := strings.IndexByte(s, '\n')
        var line string
        if nl < 0 {
            line, s = s, ""
        } else {
            line, s = s[:nl], s[nl+1:]
        }
        if fenceDelimLine(line) {
            open = !open
        }
    }
    return open
}

// advanceFenceStateOverCall folds one complete bare-call span (name through
// its closing bracket, as returned by findBareCall) into the running fence
// state. It mirrors bareArgsEnd's own quote/escape tracking so the two can
// never disagree about where a quoted argument value starts or ends: a ```
// sequence inside a quoted value is argument data, not a markdown fence, and
// must not toggle the state — see the PR description for a live example
// (a heredoc-style shell command whose payload contains a fenced code
// block). Only backticks in the unquoted parts of the call (which is
// realistically just the name and the bracket/comma/key structure) can ever
// toggle it.
func advanceFenceStateOverCall(call string, open bool) bool {
    var quote byte
    lineStart := 0
    for i := 0; i < len(call); i++ {
        c := call[i]
        if quote != 0 {
            switch c {
            case '\\':
                i++
            case quote:
                quote = 0
            }
            continue
        }
        switch c {
        case '"', '\'':
            quote = c
        case '\n':
            if fenceDelimLine(call[lineStart:i]) {
                open = !open
            }
            lineStart = i + 1
        }
    }
    if fenceDelimLine(call[lineStart:]) {
        open = !open
    }
    return open
}

// findBareCall returns the first envelope-less invocation of a declared tool
// in s that is not sitting inside an open ``` fence — a fenced occurrence is
// a documentation example, not a call, so it is skipped and the search
// continues past it (a real call may still follow once the fence closes).
//
// fenceOpen is the fence state immediately before s. fenceAfter is that same
// state immediately after the returned call, or after all of s when no call
// is found or the trailing one is still incomplete; callers MUST carry it
// forward as the next call's fenceOpen rather than recomputing it from a
// shorter window (see advanceFenceState's doc comment for why).
//
// complete is false when an invocation starts but its argument list has not
// closed yet. The caller must then hold the text back rather than emit it,
// because the rest of the call is still arriving.
func findBareCall(s string, names []string, prevByte byte, fenceOpen bool) (call bareCall, found, complete bool, fenceAfter bool) {
    if len(names) == 0 {
        return bareCall{}, false, false, fenceOpen
    }
    scanned := 0 // s[:scanned] has already been folded into state
    searchFrom := 0
    state := fenceOpen
    for {
        best := -1
        var bestName string
        for _, name := range names {
            for pos := searchFrom; ; {
                idx := strings.Index(s[pos:], name)
                if idx < 0 {
                    break
                }
                at := pos + idx
                pos = at + 1
                after := at + len(name)
                if after >= len(s) {
                    // The name ends the text; its delimiter has not arrived.
                    if atLineStart(s, at, prevByte) && (best < 0 || at < best) {
                        best, bestName = at, name
                    }
                    break
                }
                if s[after] != '(' && s[after] != '{' {
                    continue
                }
                if !atLineStart(s, at, prevByte) {
                    continue
                }
                if best < 0 || at < best {
                    best, bestName = at, name
                }
                break
            }
        }
        if best < 0 {
            return bareCall{}, false, false, advanceFenceState(s[scanned:], state)
        }

        // Fold the free text between the previous scan point and this
        // candidate before judging it, then advance the scan point so the
        // same bytes are never folded twice across loop iterations.
        state = advanceFenceState(s[scanned:best], state)
        scanned = best

        if state {
            // Inside an open fence: a documentation example, not a call.
            // Skip past this occurrence and keep looking.
            searchFrom = best + len(bestName)
            continue
        }

        open := best + len(bestName)
        if open >= len(s) {
            return bareCall{start: best, name: bestName}, true, false, state
        }
        end, closed := bareArgsEnd(s, open)
        if !closed {
            return bareCall{start: best, name: bestName}, true, false, state
        }
        args, ok := parseBareArgs(s[open:end])
        if !ok {
            // Complete but unreadable: not a call we can build, leave it as
            // text (matches the pre-fence-awareness behaviour of giving up
            // on this scan entirely rather than guessing past a malformed
            // call).
            return bareCall{}, false, false, state
        }
        // The call span itself is never emitted as content — it becomes a
        // tool_call — but it still occupies space in the model's output, so
        // backticks inside it (outside any quoted value) still affect what
        // a later scan on this stream sees.
        fenceAfter = advanceFenceStateOverCall(s[best:end], state)
        return bareCall{start: best, end: end, name: bestName, args: args}, true, true, fenceAfter
    }
}

// bareArgsEnd finds the index just past the argument list that opens at
// index open, honouring quotes and escapes so a bracket inside a string does
// not close it.
func bareArgsEnd(s string, open int) (int, bool) {
    var closer byte
    switch s[open] {
    case '(':
        closer = ')'
    case '{':
        closer = '}'
    default:
        return 0, false
    }
    opener := s[open]
    depth := 0
    var quote byte
    for i := open; i < len(s); i++ {
        c := s[i]
        if quote != 0 {
            switch c {
            case '\\':
                i++
            case quote:
                quote = 0
            }
            continue
        }
        switch c {
        case '"', '\'':
            quote = c
        case opener:
            depth++
        case closer:
            depth--
            if depth == 0 {
                return i + 1, true
            }
        }
    }
    return 0, false
}

// parseBareArgs reads either argument form a bare call may carry.
func parseBareArgs(payload string) (map[string]interface{}, bool) {
    trimmed := strings.TrimSpace(payload)
    if strings.HasPrefix(trimmed, "{") {
        var obj map[string]interface{}
        if json.Unmarshal([]byte(trimmed), &obj) != nil {
            return nil, false
        }
        return obj, true
    }
    return parseNativeKwargs(trimmed)
}

// TranslateBareToolCalls rewrites every envelope-less invocation of a declared
// tool into the canonical marker form. Without a tool list, or without a
// complete invocation, the text is returned unchanged.
func TranslateBareToolCalls(text string, names []string) string {
    if len(names) == 0 {
        return text
    }
    var b strings.Builder
    rest := text
    var prev byte
    fenceOpen := false
    changed := false
    for {
        call, found, complete, fenceAfter := findBareCall(rest, names, prev, fenceOpen)
        if !found || !complete {
            b.WriteString(rest)
            break
        }
        encoded, err := json.Marshal(map[string]interface{}{"name": call.name, "arguments": call.args})
        if err != nil {
            b.WriteString(rest)
            break
        }
        b.WriteString(rest[:call.start])
        b.WriteString(agentToolStart + string(encoded) + agentToolEnd)
        changed = true
        fenceOpen = fenceAfter
        if call.end > 0 {
            prev = rest[call.end-1]
        }
        rest = rest[call.end:]
    }
    if !changed {
        return text
    }
    return b.String()
}

// commitContent appends s to content and folds it into the interceptor's
// running bare-call fence state (in.bareFenceOpen — see its doc comment on
// AgentStreamInterceptor). This is the ONLY place that state may change for
// ordinary text: every append to *content anywhere in this package's drain
// path must go through here rather than appending directly, or the fence
// state silently desyncs from what the client actually receives and the
// false-positive this file exists to prevent can resurface. s is assumed to
// be free text, not the span of a recognised call — see
// advanceFenceStateOverCall for that case.
func (in *AgentStreamInterceptor) commitContent(s string, content *[]string) {
    if s == "" {
        return
    }
    *content = append(*content, s)
    in.bareFenceOpen = advanceFenceState(s, in.bareFenceOpen)
}

// drainBareCall handles an envelope-less invocation met while streaming. It
// mirrors drainNativeBlock: the call is buffered whole, because its payload is
// not JSON and the incremental scanner cannot follow it.
//
// handled is false when the caller should continue with the other scans. When
// handled is true, done reports whether the call was consumed (keep scanning)
// or is still arriving (wait for more chunks).
func (in *AgentStreamInterceptor) drainBareCall(rest string, canonicalStart, nativeStart int, final bool,
    content *[]string, toolCalls *[]map[string]interface{}) (handled, done bool) {
    if len(in.toolNames) == 0 {
        return false, false
    }
    var prev byte
    if in.offset > 0 {
        prev = in.buffer[in.offset-1]
    }
    call, found, complete, fenceAfter := findBareCall(rest, in.toolNames, prev, in.bareFenceOpen)
    if !found {
        // All of rest was free text as far as bare-call scanning goes, but
        // it may not all be committed this pass (the general hold-back path
        // further down in drain() may keep a tail). Do not fold it here —
        // whatever actually gets committed will be folded through
        // commitContent at the point it is committed.
        return false, false
    }
    // Either envelope wins when it starts at or before this point: both are
    // explicit, and the canonical one streams properly. Leave in.bareFenceOpen
    // untouched — the winning path commits its own preceding text.
    if canonicalStart >= 0 && canonicalStart <= call.start {
        return false, false
    }
    if nativeStart >= 0 && nativeStart <= call.start {
        return false, false
    }
    if !complete {
        if final {
            // The argument list never closed. Treat it as prose rather than
            // inventing a call from a half-written one.
            return false, false
        }
        if call.start > 0 {
            in.commitContent(rest[:call.start], content)
            in.offset += call.start
        }
        return true, false
    }
    encoded, err := json.Marshal(map[string]interface{}{"name": call.name, "arguments": call.args})
    if err != nil {
        return false, false
    }
    if call.start > 0 {
        in.commitContent(rest[:call.start], content)
    }
    *toolCalls = append(*toolCalls, ParseAgentToolCalls(agentToolStart+string(encoded)+agentToolEnd)...)
    // fenceAfter already accounts for both the leading text just committed
    // above and the call span itself (see findBareCall) — this replaces,
    // rather than adds to, whatever commitContent just set.
    in.bareFenceOpen = fenceAfter
    in.offset += call.end
    in.pendingSep = true
    return true, true
}
