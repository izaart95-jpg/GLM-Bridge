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

// findBareCall returns the first envelope-less invocation of a declared tool
// in s. complete is false when an invocation starts but its argument list has
// not closed yet. The caller must then hold the text back rather than emit
// it, because the rest of the call is still arriving.
func findBareCall(s string, names []string, prevByte byte) (call bareCall, found, complete bool) {
    if len(names) == 0 {
        return bareCall{}, false, false
    }
    best := -1
    var bestName string
    for _, name := range names {
        for pos := 0; ; {
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
        return bareCall{}, false, false
    }
    open := best + len(bestName)
    if open >= len(s) {
        return bareCall{start: best, name: bestName}, true, false
    }
    end, closed := bareArgsEnd(s, open)
    if !closed {
        return bareCall{start: best, name: bestName}, true, false
    }
    args, ok := parseBareArgs(s[open:end])
    if !ok {
        // Complete but unreadable: not a call we can build, leave it as text.
        return bareCall{}, false, false
    }
    return bareCall{start: best, end: end, name: bestName, args: args}, true, true
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
    changed := false
    for {
        call, found, complete := findBareCall(rest, names, prev)
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
    call, found, complete := findBareCall(rest, in.toolNames, prev)
    if !found {
        return false, false
    }
    // Either envelope wins when it starts at or before this point: both are
    // explicit, and the canonical one streams properly.
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
            *content = append(*content, rest[:call.start])
            in.offset += call.start
        }
        return true, false
    }
    encoded, err := json.Marshal(map[string]interface{}{"name": call.name, "arguments": call.args})
    if err != nil {
        return false, false
    }
    if call.start > 0 {
        *content = append(*content, rest[:call.start])
    }
    *toolCalls = append(*toolCalls, ParseAgentToolCalls(agentToolStart+string(encoded)+agentToolEnd)...)
    in.offset += call.end
    in.pendingSep = true
    return true, true
}
