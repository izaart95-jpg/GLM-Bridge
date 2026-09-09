package zbridge

import (
    "encoding/json"
    "regexp"
    "strconv"
    "strings"
)

// Zhipu's own models fall back to the tool-call syntax they were trained on
// instead of the <<<TOOL_CALL>>>{json}<<<END_TOOL_CALL>>> shape the agent
// prompt asks for:
//
//	<tool_call>bash
//	<arg_key>command</arg_key>
//	<arg_value>ls -la</arg_value>
//	</tool_call>
//
// Nothing downstream recognises it, so the block reaches the client verbatim
// as assistant text — the model looks like it narrated a tool call instead of
// making one. Worse, once that text is in the transcript the model imitates
// its own broken output, so a single slip poisons the rest of the session.
//
// Translating the block into the canonical marker form lets the existing
// parser, streaming interceptor and strip pass handle it unchanged.
//
// The same models reach for a second shape, the call written as a function
// invocation with keyword arguments, usually on one line and usually without
// the closing tag:
//
//	<tool_call>web_search(i="Surveying free proxies", query="proxy github", limit=8)
//
// Both halves of that need handling. Without the <arg_key>/<arg_value> tags
// the argument scan finds nothing and the call arrives with an empty argument
// object; without the closing tag the block regex does not match at all and
// the whole line reaches the client as text.

var nativeArgPairRe = regexp.MustCompile(`(?s)<arg_key>(.*?)</arg_key>\s*<arg_value>(.*?)</arg_value>`)

var nativeNameRe = regexp.MustCompile(`^\s*([A-Za-z0-9_.\-]+)`)

const nativeToolOpen = "<tool_call>"
const nativeToolClose = "</tool_call>"

// HasNativeToolCall reports whether s contains a complete native block.
func HasNativeToolCall(s string) bool {
    for _, sp := range findNativeSpans(s) {
        if sp.terminated {
            return true
        }
    }
    return false
}

// nativeToolCallPrefix reports whether s ends inside what could still grow
// into a native tool-call block, so a streaming caller knows to hold the tail
// back instead of leaking half a block to the client.
func nativeToolCallPrefix(s string) bool {
    idx := strings.LastIndex(s, nativeToolOpen)
    if idx < 0 {
        // A bare "<tool_c…" tail may still complete on the next chunk.
        for k := 1; k < len(nativeToolOpen) && k <= len(s); k++ {
            if strings.HasSuffix(s, nativeToolOpen[:k]) {
                return true
            }
        }
        return false
    }
    return !strings.Contains(s[idx:], nativeToolClose)
}

// ── block scanning ───────────────────────────────────────────────────────────

// nativeSpan is one <tool_call> block found in a text. end points just past
// the closing tag, or at the end of the text when the block never closed,
// which only the last span can be.
type nativeSpan struct {
    start, end int
    body       string
    terminated bool
}

// findNativeSpans locates every <tool_call> block, closed or not.
func findNativeSpans(text string) []nativeSpan {
    var spans []nativeSpan
    for pos := 0; ; {
        idx := strings.Index(text[pos:], nativeToolOpen)
        if idx < 0 {
            return spans
        }
        start := pos + idx
        bodyStart := start + len(nativeToolOpen)
        close := strings.Index(text[bodyStart:], nativeToolClose)
        if close < 0 {
            return append(spans, nativeSpan{start: start, end: len(text), body: text[bodyStart:]})
        }
        spans = append(spans, nativeSpan{
            start:      start,
            end:        bodyStart + close + len(nativeToolClose),
            body:       text[bodyStart : bodyStart+close],
            terminated: true,
        })
        pos = bodyStart + close + len(nativeToolClose)
    }
}

// parseNativeBlock reads a block body in whichever dialect the model reached
// for. Four shapes appear in practice, and the payload's first character
// picks between them:
//
//	bash<arg_key>command</arg_key><arg_value>ls</arg_value>   tagged pairs
//	bash(command="ls", i="listing")                           keyword arguments
//	bash{"command":"ls"}                                      name then JSON
//	{"name":"bash","arguments":{"command":"ls"}}              JSON envelope
//
// ok is false when no name can be read; args is empty when the payload uses
// no recognised argument form, which keeps a call with an unreadable tail
// from being dropped entirely.
func parseNativeBlock(body string) (name string, args map[string]interface{}, ok bool) {
    trimmed := strings.TrimSpace(body)
    if n, a, damaged := parseDamagedCanonical(trimmed); damaged {
        return n, a, true
    }
    if strings.HasPrefix(trimmed, "{") {
        return parseNativeJSONEnvelope(trimmed)
    }
    m := nativeNameRe.FindStringSubmatch(trimmed)
    if m == nil {
        return "", nil, false
    }
    return m[1], nativeArgsFrom(trimmed[len(m[0]):]), true
}

// parseNativeJSONEnvelope reads the {"name":…, "arguments":…} shape. The
// arguments key is spelled three different ways by different fine-tunes, and
// some emit them as a JSON string rather than an object.
func parseNativeJSONEnvelope(payload string) (string, map[string]interface{}, bool) {
    var env map[string]interface{}
    if json.Unmarshal([]byte(payload), &env) != nil {
        return "", nil, false
    }
    name, _ := env["name"].(string)
    if name == "" {
        return "", nil, false
    }
    for _, key := range []string{"arguments", "parameters", "args"} {
        switch v := env[key].(type) {
        case map[string]interface{}:
            return name, v, true
        case string:
            var nested map[string]interface{}
            if json.Unmarshal([]byte(v), &nested) == nil {
                return name, nested, true
            }
        }
    }
    return name, map[string]interface{}{}, true
}

// nativeArgsFrom reads a payload in whichever argument shape it uses. An
// unrecognised payload yields an empty argument set, which is what this path
// did before the other dialects were handled.
func nativeArgsFrom(payload string) map[string]interface{} {
    args := map[string]interface{}{}
    if pairs := nativeArgPairRe.FindAllStringSubmatch(payload, -1); len(pairs) > 0 {
        for _, pair := range pairs {
            key := strings.TrimSpace(pair[1])
            if key == "" {
                continue
            }
            // Values are raw text, including newlines and quotes; json.Marshal
            // on the whole map escapes them correctly.
            args[key] = strings.Trim(pair[2], "\n")
        }
        return args
    }
    trimmed := strings.TrimSpace(payload)
    if strings.HasPrefix(trimmed, "{") {
        var obj map[string]interface{}
        if json.Unmarshal([]byte(trimmed), &obj) == nil {
            return obj
        }
    }
    if kwargs, ok := parseNativeKwargs(trimmed); ok {
        return kwargs
    }
    return args
}

// ── translation ──────────────────────────────────────────────────────────────

// TranslateNativeToolCalls rewrites every native block into the canonical
// marker form, including a block left unterminated at the end of the text.
// Text outside such blocks is returned untouched, and input without a native
// block is returned unchanged.
func TranslateNativeToolCalls(text string) string {
    if !strings.Contains(text, nativeToolOpen) {
        return text
    }
    var b strings.Builder
    prev := 0
    for _, sp := range findNativeSpans(text) {
        name, args, ok := parseNativeBlock(sp.body)
        // An unterminated block is only translated when its payload is
        // finished; see nativeBodyComplete.
        if !ok || (!sp.terminated && !nativeBodyComplete(sp.body)) {
            continue
        }
        encoded, err := json.Marshal(map[string]interface{}{"name": name, "arguments": args})
        if err != nil {
            continue
        }
        b.WriteString(text[prev:sp.start])
        b.WriteString(agentToolStart + string(encoded) + agentToolEnd)
        prev = sp.end
    }
    if prev == 0 {
        return text
    }
    b.WriteString(text[prev:])
    return b.String()
}

// ── unterminated blocks ──────────────────────────────────────────────────────

// nativeBodyComplete reports whether an unterminated body stops on a boundary
// where nothing is half-written: every argument it began is finished, so the
// closing tag is the only thing missing.
//
// This is the safety condition for recovery, and the reason each dialect is
// checked rather than assumed. A body that stops mid-argument must stay text:
// turning a truncated `rm -rf /home/user/proj` into a tool call hands the
// client a command the model never finished writing.
func nativeBodyComplete(body string) bool {
    trimmed := strings.TrimSpace(body)
    if trimmed == "" {
        return false
    }
    if _, _, ok := parseDamagedCanonical(trimmed); ok {
        // The canonical closing marker ends it; nothing is still arriving.
        return true
    }
    if strings.HasPrefix(trimmed, "{") {
        return json.Valid([]byte(trimmed))
    }
    m := nativeNameRe.FindStringSubmatch(trimmed)
    if m == nil {
        return false
    }
    rest := strings.TrimSpace(trimmed[len(m[0]):])
    switch {
    case rest == "":
        // Just the name so far; the arguments may still be arriving.
        return false
    case strings.HasPrefix(rest, "("):
        _, ok := parseNativeKwargs(rest)
        return ok
    case strings.HasPrefix(rest, "{"):
        return json.Valid([]byte(rest))
    case nativeArgPairRe.MatchString(rest):
        return strings.TrimSpace(nativeArgPairRe.ReplaceAllString(rest, "")) == ""
    }
    return false
}

// nativeTailRecoverable reports whether text ends in an unterminated block
// that may be translated despite the missing closing tag.
func nativeTailRecoverable(text string) bool {
    spans := findNativeSpans(text)
    if len(spans) == 0 {
        return false
    }
    last := spans[len(spans)-1]
    if last.terminated {
        return false
    }
    _, _, ok := parseNativeBlock(last.body)
    return ok && nativeBodyComplete(last.body)
}

// ── damaged canonical blocks ─────────────────────────────────────────────────

// parseDamagedCanonical salvages a block that mixes both syntaxes:
//
//	<tool_call>bash
//	 arguments":{"i":"…","command":"grep …"} <<<END_TOOL_CALL>>>
//
// The canonical closing marker is present but its opening marker is not, and
// the JSON has lost its `{"name":"bash",` head. That is not a shape any model
// writes. It is what an upstream rewrite leaves behind when it replaces one
// attempt with another and the already-forwarded prefix cannot be taken back
// (issue #23). The name survives in the native opener and the arguments object
// survives whole, so the call is recoverable.
//
// The closing marker is required as the anchor, and the arguments object must
// parse as JSON. That parse is the integrity check: a splice landing inside
// the object would almost certainly unbalance its braces or quotes, so a
// payload that still parses is very unlikely to be the corrupted one.
func parseDamagedCanonical(body string) (string, map[string]interface{}, bool) {
    if !strings.Contains(body, agentToolEnd) {
        return "", nil, false
    }
    m := nativeNameRe.FindStringSubmatch(body)
    if m == nil {
        return "", nil, false
    }
    name := m[1]
    rest := body[len(m[0]):]
    key := strings.Index(rest, "arguments\"")
    if key < 0 {
        return "", nil, false
    }
    brace := strings.Index(rest[key:], "{")
    if brace < 0 {
        return "", nil, false
    }
    obj, ok := balancedJSONObject(rest[key+brace:])
    if !ok {
        return "", nil, false
    }
    var args map[string]interface{}
    if json.Unmarshal([]byte(obj), &args) != nil {
        return "", nil, false
    }
    return name, args, true
}

// balancedJSONObject returns the object starting at s[0] == '{', honouring
// strings and escapes so a brace inside a value does not close it.
func balancedJSONObject(s string) (string, bool) {
    if len(s) == 0 || s[0] != '{' {
        return "", false
    }
    depth := 0
    inString := false
    for i := 0; i < len(s); i++ {
        c := s[i]
        if inString {
            switch c {
            case '\\':
                i++
            case '"':
                inString = false
            }
            continue
        }
        switch c {
        case '"':
            inString = true
        case '{':
            depth++
        case '}':
            depth--
            if depth == 0 {
                return s[:i+1], true
            }
        }
    }
    return "", false
}

// ── parenthesised argument lists ─────────────────────────────────────────────

// parseNativeKwargs reads `(key="value", n=8, flag=true)`. Values are quoted
// strings in either quote style with backslash escapes honoured, numbers,
// booleans or null; anything else is kept as a bare string.
//
// ok is false when the list is not closed or an entry is malformed. The caller
// then leaves the block as text instead of inventing arguments for it, which
// is also what keeps a still-arriving call from being recovered early, since
// its argument list has no closing parenthesis yet.
func parseNativeKwargs(s string) (map[string]interface{}, bool) {
    s = strings.TrimSpace(s)
    if len(s) < 2 || !strings.HasPrefix(s, "(") || !strings.HasSuffix(s, ")") {
        return nil, false
    }
    body := s[1 : len(s)-1]
    args := map[string]interface{}{}
    for i := 0; ; {
        for i < len(body) && (isNativeSpace(body[i]) || body[i] == ',') {
            i++
        }
        if i >= len(body) {
            break
        }
        keyStart := i
        for i < len(body) && isNativeIdent(body[i]) {
            i++
        }
        if i == keyStart {
            return nil, false
        }
        key := body[keyStart:i]
        for i < len(body) && isNativeSpace(body[i]) {
            i++
        }
        if i >= len(body) || body[i] != '=' {
            return nil, false
        }
        i++
        for i < len(body) && isNativeSpace(body[i]) {
            i++
        }
        value, next, ok := readNativeValue(body, i)
        if !ok {
            return nil, false
        }
        args[key] = value
        i = next
    }
    if len(args) == 0 {
        return nil, false
    }
    return args, true
}

// readNativeValue reads one value starting at i and returns the offset just
// past it.
func readNativeValue(s string, i int) (interface{}, int, bool) {
    if i >= len(s) {
        return nil, i, false
    }
    if quote := s[i]; quote == '"' || quote == '\'' {
        var b strings.Builder
        for i++; i < len(s); {
            switch s[i] {
            case '\\':
                if i+1 >= len(s) {
                    return nil, i, false
                }
                b.WriteString(unescapeNative(s[i+1]))
                i += 2
            case quote:
                return b.String(), i + 1, true
            default:
                b.WriteByte(s[i])
                i++
            }
        }
        // Closing quote missing: the value is still being written.
        return nil, i, false
    }
    start := i
    for i < len(s) && s[i] != ',' {
        i++
    }
    raw := strings.TrimSpace(s[start:i])
    if raw == "" {
        return nil, i, false
    }
    return nativeScalar(raw), i, true
}

func unescapeNative(c byte) string {
    switch c {
    case 'n':
        return "\n"
    case 't':
        return "\t"
    case 'r':
        return "\r"
    default:
        return string(c)
    }
}

// nativeScalar types an unquoted value the way JSON would.
func nativeScalar(raw string) interface{} {
    switch raw {
    case "true", "True":
        return true
    case "false", "False":
        return false
    case "null", "None", "nil":
        return nil
    }
    if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
        return n
    }
    if f, err := strconv.ParseFloat(raw, 64); err == nil {
        return f
    }
    return raw
}

func isNativeSpace(c byte) bool {
    return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isNativeIdent(c byte) bool {
    return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '-'
}

// ── streaming ────────────────────────────────────────────────────────────────

// drainNativeBlock handles a native tool-call block met while streaming.
//
// The block is buffered whole rather than streamed argument-by-argument: its
// payload is XML, so the incremental JSON scanner the canonical path uses
// cannot follow it. That matches the interceptor's existing fallback for
// un-streamable shapes — the client gets one complete tool call instead of
// live fragments, which is what the alternative (raw XML as assistant text)
// fails to give at all.
//
// handled is false when the caller should continue with the canonical scan.
// When handled is true, done reports whether the block was consumed (keep
// scanning) or is still arriving (wait for more chunks).
func (in *AgentStreamInterceptor) drainNativeBlock(rest string, canonicalStart int, final bool,
    content *[]string, toolCalls *[]map[string]interface{}) (handled, done bool) {
    nStart := strings.Index(rest, nativeToolOpen)
    if nStart < 0 {
        return false, false
    }
    // A canonical marker at or before this point wins: it is the documented
    // shape and streams properly.
    if canonicalStart >= 0 && canonicalStart <= nStart {
        return false, false
    }
    endIdx := strings.Index(rest[nStart:], nativeToolClose)
    if endIdx < 0 {
        if final {
            // End of stream with the block still open. The model routinely
            // drops the closing tag on the parenthesised form, so when the
            // payload itself is finished the tag is the only thing missing and
            // the call is recovered. A payload that stops mid-argument is left
            // to the normal path, which emits it as content rather than
            // swallowing output or fabricating a half-written call.
            if !nativeTailRecoverable(rest[nStart:]) {
                return false, false
            }
            if nStart > 0 {
                in.commitContent(rest[:nStart], content)
            }
            *toolCalls = append(*toolCalls, ParseAgentToolCalls(rest[nStart:])...)
            in.offset += len(rest)
            in.pendingSep = true
            return true, true
        }
        // Emit what precedes the block and hold the rest until it closes, so
        // half a block never reaches the client as text.
        if nStart > 0 {
            in.commitContent(rest[:nStart], content)
            in.offset += nStart
        }
        return true, false
    }
    blockEnd := nStart + endIdx + len(nativeToolClose)
    if nStart > 0 {
        in.commitContent(rest[:nStart], content)
    }
    *toolCalls = append(*toolCalls, ParseAgentToolCalls(rest[nStart:blockEnd])...)
    in.offset += blockEnd
    in.pendingSep = true
    return true, true
}
