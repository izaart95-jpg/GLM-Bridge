// agent.go
//
// ============================================================================
// AGENT MODE (MODERN) — XML-Sectioned Prompt Shim for Z.AI Compatibility
// ============================================================================
//
// Port of the modern agentMode compatibility shim from DeepseekFreeAPI
// (internal/dsproxy/agent.go), adapted for the GLM-Free-API / Z.AI bridge.
//
// Enable with --agent-mode / AGENT_MODE=true (modern variant is the default;
// the old [ROLE: ...] rewrite shim stays available via --agent-mode-variant=legacy
// or AGENT_MODE_VARIANT=legacy).
//
// Why the modern shim replaces the legacy one:
//
//   - Legacy flattened the conversation into "[ROLE: x]" prefixed user
//     messages plus a [TOOL CONTRACT] blob. Models suffer context rot on
//     that flat format: marker ambiguity, no recency anchor, full verbatim
//     replay of long tool histories.
//
//   - Modern structures the prompt with explicit XML-like section tags
//     (<system>, <tools>, <history_summary>, <recent>, <current_task>,
//     <output_rules>), summarizes older tool exchanges, anchors the latest
//     user message as the current task, and repeats the output contract at
//     the very end (recency bias).
//
//   - Modern parsing is tolerant where legacy was strict:
//       * markers matched with 2..4 angle brackets per side (models
//         miscount brackets in the wild),
//       * ```json fences adjacent to markers are stripped,
//       * payload shapes beyond {"name","arguments"} are accepted
//         (flat {"tool": ..., params...} and alternate key spellings),
//       * the streaming interceptor holds back a trailing window so a
//         marker split across upstream chunks can never leak as content.
//
// No native tool calling is used: the model emits the tool-call protocol
// below, which is converted back to OpenAI tool_calls on the way out (parsed
// from the finished text, or incrementally while streaming).

package zbridge

import (
    "bytes"
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "regexp"
    "strings"
    "unicode/utf8"
)

const agentToolStart = "<<<TOOL_CALL>>>"
const agentToolEnd = "<<<END_TOOL_CALL>>>"

// ── marker tolerance ─────────────────────────────────────────────────────────
// Models occasionally miscount the angle brackets framing the markers —
// observed in the wild with deepseek-v4-pro emitting "<<TOOL_CALL>>>" (two
// leading '<') while producing a well-formed "<<<END_TOOL_CALL>>>". An
// exact-literal matcher silently misses such blocks and the whole tool call
// leaks to the client as plain content. Both markers are therefore matched
// with a bracket run of 2..4 on each side; emission above stays canonical.
const (
    agentStartWord   = "TOOL_CALL"
    agentEndWord     = "END_TOOL_CALL"
    agentMinBrackets = 2
    agentMaxBrackets = 4
)

// agentWorstMarkerLen is the longest accepted spelling ("<<<<TOOL_CALL>>>>").
const agentWorstMarkerLen = 2*agentMaxBrackets + len(agentStartWord)

// bracketRunBack counts the run of b bytes ending immediately before s[i].
func bracketRunBack(s string, i int, b byte) int {
    n := 0
    for i-n-1 >= 0 && s[i-n-1] == b {
        n++
    }
    return n
}

// bracketRunForward counts the run of b bytes starting at s[0].
func bracketRunForward(s string, b byte) int {
    n := 0
    for n < len(s) && s[n] == b {
        n++
    }
    return n
}

// Sentinel results for findAgentMarker.
const (
    markerNone       = -1 // no framed occurrence of word in s
    markerIncomplete = -2 // a candidate needs more bytes before it can match
)

// findAgentMarker locates the first occurrence of word framed by 2..4 '<'
// immediately before and 2..4 '>' immediately after. It returns the index of
// the first bracket and the full marker length, or markerNone /
// markerIncomplete. Occurrences not so framed (the TOOL_CALL inside an END
// marker, prose, code) are skipped.
//
// Streaming correctness: a trailing '>' run that reaches the end of s has no
// terminating byte yet, so the run may still grow — with final=false that is
// reported as markerIncomplete instead of matching short (which would leak
// the missing brackets as content) or rejecting outright. With final=true
// (finished text) an end-of-string run is taken as is.
func findAgentMarker(s, word string, final bool) (int, int) {
    for from := 0; ; {
        j := strings.Index(s[from:], word)
        if j < 0 {
            return markerNone, 0
        }
        w := from + j
        lead := bracketRunBack(s, w, '<')
        if lead < agentMinBrackets || lead > agentMaxBrackets {
            from = w + len(word)
            continue
        }
        after := s[w+len(word):]
        trail := bracketRunForward(after, '>')
        switch {
        case trail > agentMaxBrackets:
            // Definitively over-long; more bytes cannot shrink the run.
        case trail == len(after) && !final:
            // The '>' run touches the end of the available data and may
            // still grow past min/max — wait for a terminating byte.
            return markerIncomplete, 0
        case trail >= agentMinBrackets:
            return w - lead, lead + len(word) + trail
        }
        from = w + len(word)
    }
}

// agentSpan marks one complete tool-call block in finished text:
// [start,end) covers both markers, [bodyStart,bodyEnd) the JSON between them.
type agentSpan struct {
    start, bodyStart, bodyEnd, end int
}

// findAgentSpans walks every complete tolerant tool-call block in text.
// An unterminated opening marker is ignored, matching the old literal scan.
func findAgentSpans(text string) []agentSpan {
    var spans []agentSpan
    for pos := 0; ; {
        s, slen := findAgentMarker(text[pos:], agentStartWord, true)
        if s < 0 {
            return spans
        }
        bodyStart := pos + s + slen
        e, elen := findAgentMarker(text[bodyStart:], agentEndWord, true)
        if e < 0 {
            return spans
        }
        spans = append(spans, agentSpan{
            start:     pos + s,
            bodyStart: bodyStart,
            bodyEnd:   bodyStart + e,
            end:       bodyStart + e + elen,
        })
        pos = bodyStart + e + elen
    }
}

// ── prompt architecture ──────────────────────────────────────────────────────
// The prompt is structured with explicit XML-like section tags so the model
// can clearly distinguish instructions, tools, conversation history, and the
// current task. This eliminates context rot: the model no longer has to parse
// a flat blob of role-tagged text.
//
// Structure:
//   <system>        — compact output contract
//   <tools>         — available tool definitions
//   <history>       — older conversation turns (summarized if too long)
//   <recent>        — recent turns with grouped tool exchanges
//   <current_task>  — the latest user message (recency anchor)
//   <output_rules>  — final reminder at the very end (heaviest weight)

// agentCallSchema is the exact JSON payload shape required inside a
// tool-call block. It is stated verbatim in the prompt (and repeated in the
// final reminder): a bare "{JSON}" placeholder let models invent flat
// payloads like {"tool": "bash", "command": ...} that the runtime cannot
// map back to OpenAI tool_calls reliably.
const agentCallSchema = `{"name":"<tool_name>","arguments":{<parameter JSON>}}`

const agentSystemPrefix = "<system>\n" +
    "You are a helpful assistant with access to tools. Follow these rules strictly:\n" +
    "\n" +
    "REPLY FORMAT \u2014 exactly ONE of:\n" +
    "(A) TOOL CALL: <<<TOOL_CALL>>>" + agentCallSchema + "<<<END_TOOL_CALL>>> \u2014 nothing before or after.\n" +
    "    The JSON object has EXACTLY two keys: \"name\" (the tool to call, spelled exactly as in <tools>) and \"arguments\" (an object with ONLY that tool's parameters).\n" +
    "(B) FINAL ANSWER: plain text, only when no tool applies.\n" +
    "\n" +
    "RULES:\n" +
    "- Never announce plans (\u201cI\u2019ll...\u201d, \u201cLet me...\u201d). Emit the block \u2014 that IS the action.\n" +
    "- Never print code fences (" + "```bash" + ", " + "```json" + "). Only the runtime executes tools.\n" +
    "- Never wrap tool-call markers in code fences.\n" +
    "- Never invent results. Stop at <<<END_TOOL_CALL>>> and wait for tool output.\n" +
    "- Never call a tool not listed in <tools>.\n" +
    "</system>"

// agentFinalReminder is appended at the very end of the prompt. Models weight
// the end of the prompt most heavily (recency bias), so the output contract
// is repeated here as the last thing the model sees.
const agentFinalReminder = `<output_rules>
RESPOND WITH EXACTLY ONE OF:
1. <<<TOOL_CALL>>>{"name":"<tool_name>","arguments":{...}}<<<END_TOOL_CALL>>> (no fences, no other text)
2. Plain text final answer (only if no tool applies to this step)
The tool-call JSON uses EXACTLY the keys "name" and "arguments" — never a "tool" key, never bare top-level parameters.
</output_rules>`

// ── OpenAI wire types ────────────────────────────────────────────────────────

// agentMessage is one OpenAI-style message of the incoming request. Content
// stays raw so both string and typed-part arrays are accepted. Unlike the
// minimal Message type used for the Z.AI wire, it carries the tool fields
// the modern prompt builder needs to replay prior tool exchanges.
type agentMessage struct {
    Role       string              `json:"role"`
    Content    json.RawMessage     `json:"content"`
    ToolCallID string              `json:"tool_call_id,omitempty"`
    ToolCalls  []assistantToolCall `json:"tool_calls,omitempty"`
    Name       string              `json:"name,omitempty"`
}

// openAITool is one entry of the OpenAI `tools` array. Both the nested form
// ({type:"function",function:{...}}) and flat definitions are accepted,
// mirroring the JS `(tool?.function || tool)` handling.
type openAITool struct {
    Type       string          `json:"type"`
    Function   *openAIFnSpec   `json:"function,omitempty"`
    Name       string          `json:"name,omitempty"`
    Descr      string          `json:"description,omitempty"`
    Parameters json.RawMessage `json:"parameters,omitempty"`
}

type openAIFnSpec struct {
    Name        string          `json:"name"`
    Description string          `json:"description,omitempty"`
    Parameters  json.RawMessage `json:"parameters,omitempty"`
}

func (t *openAITool) fnName() string {
    if t.Function != nil && t.Function.Name != "" {
        return t.Function.Name
    }
    return t.Name
}

func (t *openAITool) fnDescription() string {
    if t.Function != nil && t.Function.Description != "" {
        return t.Function.Description
    }
    return t.Descr
}

func (t *openAITool) fnParameters() json.RawMessage {
    if t.Function != nil && len(t.Function.Parameters) > 0 {
        return t.Function.Parameters
    }
    return t.Parameters
}

// assistantToolCall is a tool call inside an assistant message of the
// incoming request (the client replaying previous calls).
type assistantToolCall struct {
    ID       string `json:"id"`
    Type     string `json:"type"`
    Function struct {
        Name      string          `json:"name"`
        Arguments json.RawMessage `json:"arguments"` // JSON-encoded string per spec
    } `json:"function"`
}

// ── prompt building ──────────────────────────────────────────────────────────

// contentToText flattens OpenAI message content (string or typed parts) to text.
func contentToText(raw json.RawMessage) string {
    trimmed := bytes.TrimSpace(raw)
    if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
        return ""
    }
    var s string
    if json.Unmarshal(trimmed, &s) == nil {
        return s
    }
    var parts []struct {
        Type string `json:"type"`
        Text string `json:"text"`
    }
    if json.Unmarshal(trimmed, &parts) == nil {
        texts := make([]string, 0, len(parts))
        for _, p := range parts {
            if p.Text != "" {
                texts = append(texts, p.Text)
            }
        }
        return strings.Join(texts, "\n")
    }
    return string(trimmed)
}

func jsonIndent(raw json.RawMessage) string {
    var buf bytes.Buffer
    if err := json.Indent(&buf, bytes.TrimSpace(raw), "", "  "); err != nil {
        return string(bytes.TrimSpace(raw))
    }
    return buf.String()
}

// renderAgentTools renders the OpenAI tools array as the [TOOL CONTRACT] block.
func renderAgentTools(tools []openAITool) string {
    if len(tools) == 0 {
        return "(no tools provided)"
    }
    var b strings.Builder
    for i, tool := range tools {
        name := tool.fnName()
        if name == "" {
            continue
        }
        b.WriteString(fmt.Sprintf("### Tool %d: %s", i+1, name))
        if desc := tool.fnDescription(); desc != "" {
            b.WriteString("\nDescription: " + desc)
        }
        if params := tool.fnParameters(); len(params) > 0 && !bytes.Equal(bytes.TrimSpace(params), []byte("null")) {
            b.WriteString("\nParameters JSON Schema:\n" + jsonIndent(params))
        }
        b.WriteString("\n")
    }
    return strings.TrimSuffix(b.String(), "\n")
}

// agentCallPayload is the JSON object emitted inside a tool-call block.
// A struct (not a map) keeps the documented name-first key order.
type agentCallPayload struct {
    Name      string          `json:"name"`
    Arguments json.RawMessage `json:"arguments"`
}

// renderToolCallBlock renders a single assistant tool-call block in the wire
// protocol format (used both in prompt history and response parsing).
func renderToolCallBlock(call assistantToolCall) string {
    payload, err := json.Marshal(agentCallPayload{
        Name:      call.Function.Name,
        Arguments: json.RawMessage(agentParseArguments(call.Function.Arguments)),
    })
    if err != nil {
        return ""
    }
    return fmt.Sprintf("%s\n%s\n%s", agentToolStart, payload, agentToolEnd)
}

// renderAssistantTurn renders an assistant message with optional text and tool
// calls inside an XML-like tag.
func renderAssistantTurn(m agentMessage) string {
    text := contentToText(m.Content)
    var blocks []string
    if text != "" {
        blocks = append(blocks, text)
    }
    for _, call := range m.ToolCalls {
        if block := renderToolCallBlock(call); block != "" {
            blocks = append(blocks, block)
        }
    }
    content := strings.Join(blocks, "\n")
    return fmt.Sprintf("<assistant>\n%s\n</assistant>", content)
}

// renderUserTurn renders a user message inside an XML-like tag.
func renderUserTurn(m agentMessage) string {
    text := contentToText(m.Content)
    if text == "" {
        return ""
    }
    return fmt.Sprintf("<user>\n%s\n</user>", text)
}

// renderSystemTurn renders a system message inside an XML-like tag.
func renderSystemTurn(m agentMessage) string {
    text := contentToText(m.Content)
    if text == "" {
        return ""
    }
    return fmt.Sprintf("<system_message>\n%s\n</system_message>", text)
}

// renderToolResult renders a tool result inside an XML-like tag with the
// call_id attribute for unambiguous matching.
func renderToolResult(m agentMessage) string {
    text := contentToText(m.Content)
    attr := ""
    if m.ToolCallID != "" {
        attr = fmt.Sprintf(` call_id="%s"`, m.ToolCallID)
    }
    return fmt.Sprintf("<tool_result%s>\n%s\n</tool_result>", attr, text)
}

// renderAgentMessage renders one OpenAI message using XML-like section tags.
// This replaces the old [ROLE: ...] format with clearly delimited sections
// that the model can parse unambiguously.
func renderAgentMessage(m agentMessage) string {
    role := strings.TrimSpace(m.Role)
    if role == "" {
        role = "user"
    }
    switch role {
    case "system":
        return renderSystemTurn(m)
    case "user":
        return renderUserTurn(m)
    case "assistant":
        return renderAssistantTurn(m)
    case "tool":
        return renderToolResult(m)
    default:
        // Unknown role: render as user with role annotation.
        text := contentToText(m.Content)
        return fmt.Sprintf("<user role=%s>\n%s\n</user>", role, text)
    }
}

// ── history summarization ────────────────────────────────────────────────────
//
// For long conversations (many tool exchanges), the full replay causes context
// rot: the model loses focus on the current task. We summarize older turns
// into a compact context block while keeping the most recent turns verbatim.

// maxRecentToolExchanges is the number of recent tool-exchange pairs to keep
// in full detail. Older ones get summarized.
const maxRecentToolExchanges = 6

// toolExchange records one assistant→tool exchange for summarization.
type toolExchange struct {
    toolName string
    summary  string // truncated tool result
}

// summarizeOldHistory extracts tool-exchange summaries from older messages and
// returns a compact <history_summary> block. Returns empty string if there's
// nothing to summarize.
func summarizeOldHistory(exchanges []toolExchange) string {
    if len(exchanges) == 0 {
        return ""
    }
    var b strings.Builder
    b.WriteString("<history_summary>\nPreviously completed tool calls:\n")
    for i, ex := range exchanges {
        b.WriteString(fmt.Sprintf("%d. %s → %s\n", i+1, ex.toolName, ex.summary))
    }
    b.WriteString("</history_summary>")
    return b.String()
}

// extractToolExchanges scans messages and returns (old exchanges beyond the
// recent window, messages to render verbatim).
func extractToolExchanges(messages []agentMessage) (old []toolExchange, recent []agentMessage) {
    // First pass: identify tool-exchange boundaries.
    // A tool exchange = assistant with tool_calls followed by 1+ tool results.
    type exchange struct{ start, end int } // indices into messages
    var exchanges []exchange
    i := 0
    for i < len(messages) {
        if messages[i].Role == "assistant" && len(messages[i].ToolCalls) > 0 {
            ex := exchange{start: i}
            i++
            // skip tool results
            for i < len(messages) && messages[i].Role == "tool" {
                i++
            }
            ex.end = i
            exchanges = append(exchanges, ex)
        } else {
            i++
        }
    }

    // If there aren't enough exchanges to summarize, keep everything.
    if len(exchanges) <= maxRecentToolExchanges {
        return nil, messages
    }

    // Summarize exchanges before the recent window.
    splitIdx := exchanges[len(exchanges)-maxRecentToolExchanges].start
    for _, ex := range exchanges[:len(exchanges)-maxRecentToolExchanges] {
        // Collect tool names and truncated results from this exchange.
        assistant := messages[ex.start]
        names := make([]string, 0, len(assistant.ToolCalls))
        for _, tc := range assistant.ToolCalls {
            names = append(names, tc.Function.Name)
        }
        toolName := strings.Join(names, ", ")
        // Grab first tool result as summary.
        summary := "ok"
        if ex.end > ex.start+1 {
            result := contentToText(messages[ex.start+1].Content)
            if len(result) > 80 {
                result = result[:77] + "..."
            }
            summary = result
        }
        old = append(old, toolExchange{toolName: toolName, summary: summary})
    }
    recent = messages[splitIdx:]
    return old, recent
}

// buildAgentPrompt constructs the prompt sent to Z.AI. The prompt is
// structured with explicit XML-like section tags so the model can clearly
// distinguish instructions, tools, history, and the current task.
//
// Structure:
//
//	<system>             — compact output contract
//	<tools>              — available tool definitions
//	<history_summary>    — summarized older turns (if conversation is long)
//	<recent>             — recent turns in full detail with grouped tool exchanges
//	<current_task>       — the latest user message (recency anchor)
//	<output_rules>       — final reminder at the very end (heaviest weight)
func buildAgentPrompt(messages []agentMessage, tools []openAITool) string {
    var b strings.Builder

    // 1. System instructions (compact).
    b.WriteString(agentSystemPrefix)
    b.WriteString("\n\n")

    // 2. Tool contract.
    b.WriteString("<tools>\n")
    b.WriteString(renderAgentTools(tools))
    b.WriteString("\n</tools>\n\n")

    // 3. Split messages into old (summarizable) and recent.
    oldExchanges, recentMessages := extractToolExchanges(messages)

    if summary := summarizeOldHistory(oldExchanges); summary != "" {
        b.WriteString(summary)
        b.WriteString("\n\n")
    }

    // 4. Render recent conversation turns with grouped tool exchanges.
    if len(recentMessages) > 0 {
        b.WriteString("<recent>\n")
        renderRecentConversation(&b, recentMessages)
        b.WriteString("</recent>\n\n")
    }

    // 5. Extract the LAST user message as the explicit current task.
    //    This is the most important change: the model knows exactly which
    //    message to respond to.
    lastUserIdx := -1
    for i := len(messages) - 1; i >= 0; i-- {
        if messages[i].Role == "user" {
            lastUserIdx = i
            break
        }
    }
    if lastUserIdx >= 0 {
        text := contentToText(messages[lastUserIdx].Content)
        if text != "" {
            b.WriteString("<current_task>\n")
            b.WriteString(text)
            b.WriteString("\n</current_task>\n\n")
        }
    }

    // 6. Final output contract reminder (recency anchor).
    b.WriteString(agentFinalReminder)

    return b.String()
}

// renderRecentConversation renders recent messages with tool exchanges grouped.
// Tool calls and their results are wrapped in <tool_exchange> tags so the
// model can clearly see the call→result pairing.
func renderRecentConversation(b *strings.Builder, messages []agentMessage) {
    i := 0
    for i < len(messages) {
        m := messages[i]

        // Skip the last user message — it goes in <current_task>.
        isLastUser := false
        if m.Role == "user" {
            isLastUser = true
            for j := i + 1; j < len(messages); j++ {
                if messages[j].Role == "user" {
                    isLastUser = false
                    break
                }
            }
        }

        if isLastUser {
            i++
            continue
        }

        // Group assistant tool-calls with following tool results.
        if m.Role == "assistant" && len(m.ToolCalls) > 0 {
            b.WriteString("<tool_exchange>\n")
            // Render assistant's tool calls.
            b.WriteString(renderAssistantTurn(m))
            b.WriteString("\n")
            i++
            // Render tool results.
            for i < len(messages) && messages[i].Role == "tool" {
                b.WriteString(renderToolResult(messages[i]))
                b.WriteString("\n")
                i++
            }
            b.WriteString("</tool_exchange>\n")
            continue
        }

        // Regular message.
        if rendered := renderAgentMessage(m); rendered != "" {
            b.WriteString(rendered)
            b.WriteString("\n")
        }
        i++
    }
}

// wrapAgentPromptAsMessages wraps the built agent prompt as a Z.AI messages
// array containing a single user message. Z.AI's /api/v2/chat/completions
// only accepts role="user", and the modern shim folds the entire
// conversation into one prompt, so one user message carries it all.
func wrapAgentPromptAsMessages(prompt string) ([]byte, error) {
    return json.Marshal([]map[string]interface{}{
        {"role": "user", "content": prompt},
    })
}

// ── response parsing ─────────────────────────────────────────────────────────

var (
    agentFenceLead = regexp.MustCompile(`(?i)^` + "```" + `(?:json)?\s*`)
    agentFenceTail = regexp.MustCompile(`(?i)\s*` + "```" + `$`)
)

// ── fence tolerance ──────────────────────────────────────────────────────────
// Models often wrap tool-call blocks in ```json … ``` fences even when told
// not to. These helpers strip fence lines sitting DIRECTLY against the
// markers (never ordinary code blocks elsewhere in the answer).

// Tolerant bracket runs around both markers, e.g. "<<TOOL_CALL>>>" or
// "<<<END_TOOL_CALL>>>" as well as the canonical spellings.
const agentMarkerPat = "(?:<{2,4})TOOL_CALL(?:>{2,4})"
const agentEndMarkerPat = "(?:<{2,4})END_TOOL_CALL(?:>{2,4})"

var (
    // fence line immediately before a tool-call opening marker
    agentFenceBeforeCallRe = regexp.MustCompile("(?:\\A|\r?\n)[ \t]*```(?:json)?[ \t]*\r?\n(" + agentMarkerPat + ")")
    // fence line right after a tool-call closing marker (keeps the newline that follows)
    agentFenceAfterEndRe = regexp.MustCompile("(" + agentEndMarkerPat + ")[ \t]*\r?\n[ \t]*```(?:json)?[ \t]*((?:\r?\n)?)")
    // bare fence line hanging at the very end of a streamed content piece
    agentTrailFenceRe = regexp.MustCompile("(?:\\A|\r?\n)[ \t]*```(?:json)?[ \t]*(?:\r?\n)?\\z")
)

const agentFenceJSON = "```json"

// agentStreamKeep is the minimum number of trailing bytes the streaming
// interceptor keeps un-flushed while no marker has matched: enough to cover
// a fence line plus a partially received marker at its worst tolerated
// spelling, so neither can ever leak as content. The actual cut is pulled
// back to a rune boundary, so up to 3 extra bytes may be held.
const agentStreamKeep = agentWorstMarkerLen + len("```json\n") + 5

// NormalizeAgentFences removes fence lines adjacent to tool-call markers from
// finished text (non-streaming path).
func NormalizeAgentFences(text string) string {
    for {
        t := agentFenceAfterEndRe.ReplaceAllString(text, "${1}${2}")
        t = agentFenceBeforeCallRe.ReplaceAllString(t, "$1")
        if t == text {
            return t
        }
        text = t
    }
}

// TrimTrailingAgentFence drops one fence line hanging at the end of s
// (the fence the model placed immediately before <<<TOOL_CALL>>>).
func TrimTrailingAgentFence(s string) string {
    return agentTrailFenceRe.ReplaceAllString(s, "")
}

// agentPossibleFencePrefix reports whether s is empty or could still grow
// into a bare ``` / ```json fence line — i.e. it's too early to treat the
// bytes after a tool-call block as ordinary content.
func agentPossibleFencePrefix(s string) bool {
    if s == "" {
        return true // can't judge yet; wait for more chunks
    }
    for k := 1; k <= len(s) && k <= len(agentFenceJSON)+1; k++ {
        if strings.HasPrefix("```json\n", s[:k]) || strings.HasPrefix("```\n", s[:k]) {
            return true
        }
    }
    return false
}

// SkipLeadingAgentFence returns the length of a bare fence line at the start
// of s (the ``` the model places immediately after <<<END_TOOL_CALL>>>), or 0
// if s does not begin with one.
func SkipLeadingAgentFence(s string) int {
    i := 0
    for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
        i++
    }
    if !strings.HasPrefix(s[i:], "```") {
        return 0
    }
    j := i + 3
    if strings.HasPrefix(s[j:], "json") {
        j += len("json")
    }
    for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
        j++
    }
    if j < len(s) && s[j] != '\n' && s[j] != '\r' {
        return 0 // not a bare fence line (e.g. an ordinary ```bash block)
    }
    if j < len(s) { // consume one line terminator
        if s[j] == '\r' {
            j++
        }
        if j < len(s) && s[j] == '\n' {
            j++
        }
    }
    return j
}

// ── payload tolerance ────────────────────────────────────────────────────────
// The contract asks the model for {"name": "<tool>", "arguments": {...}}, but
// models observed in the wild invent their own payload shapes when the schema
// is under-specified — most commonly the FLAT form {"tool": "bash",
// "command": "...", "timeout": 10} where the tool name sits under "tool" and
// the parameters are the remaining top-level keys. A strict {name, arguments}
// unmarshal accepts such objects with Name == "", so the whole block leaks to
// the client as plain content with finish_reason "stop" and the tool is never
// executed. We therefore accept every shape that unambiguously names a tool
// and its parameters.

// agentNameKeys are accepted spellings of the "which tool" key, in priority
// order. Explicit tool-* keys outrank "name": in a flat payload a "name"
// entry is more likely a tool PARAMETER named "name" than the tool itself,
// while a "tool" entry is never a canonical-shape artifact.
var agentNameKeys = []string{"tool", "tool_name", "function", "function_name", "name"}

// agentArgKeys are accepted spellings of the explicit "parameters" key.
var agentArgKeys = []string{"arguments", "parameters", "args", "params", "input"}

// agentExtractCall resolves (name, arguments) from one decoded tool-call
// payload object, accepting the canonical shape, alternate key spellings,
// and flat payloads where the parameters are the remaining top-level keys.
func agentExtractCall(obj map[string]json.RawMessage) (name string, args json.RawMessage, ok bool) {
    // Locate the tool name under any accepted key spelling.
    nameKey := ""
    for _, k := range agentNameKeys {
        raw, present := obj[k]
        if !present {
            continue
        }
        var s string
        if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
            name, nameKey = strings.TrimSpace(s), k
            break
        }
    }
    if nameKey == "" {
        return "", nil, false
    }

    // An explicit arguments object wins over the flat fallback.
    for _, k := range agentArgKeys {
        if raw, present := obj[k]; present && !isJSONNull(raw) {
            return name, raw, true
        }
    }

    // Flat payload: every remaining top-level key is a parameter.
    rest := make(map[string]json.RawMessage, len(obj)-1)
    for k, v := range obj {
        if k != nameKey {
            rest[k] = v
        }
    }
    if len(rest) == 0 {
        return name, json.RawMessage("{}"), true
    }
    marshaled, err := json.Marshal(rest)
    if err != nil {
        return name, json.RawMessage("{}"), true
    }
    return name, marshaled, true
}

// isJSONNull reports whether raw is whitespace, JSON null, or empty.
func isJSONNull(raw json.RawMessage) bool {
    t := bytes.TrimSpace(raw)
    return len(t) == 0 || bytes.Equal(t, []byte("null"))
}

// agentLooseParse parses one tool-call body, tolerating markdown fences and
// the payload shape deviations listed at agentNameKeys / agentArgKeys.
func agentLooseParse(body string) (name string, args json.RawMessage, ok bool) {
    raw := strings.TrimSpace(body)
    raw = agentFenceLead.ReplaceAllString(raw, "")
    raw = agentFenceTail.ReplaceAllString(raw, "")
    var obj map[string]json.RawMessage
    if err := json.Unmarshal([]byte(raw), &obj); err != nil || len(obj) == 0 {
        return "", nil, false
    }
    return agentExtractCall(obj)
}

// agentParseArguments normalizes model-provided arguments to compact JSON
// text (objects pass through, JSON-encoded strings are parsed, unparsable
// strings stay quoted).
func agentParseArguments(raw json.RawMessage) string {
    t := bytes.TrimSpace(raw)
    if len(t) == 0 || bytes.Equal(t, []byte("null")) {
        return "{}"
    }
    if t[0] == '"' {
        var s string
        if err := json.Unmarshal(t, &s); err == nil {
            var c bytes.Buffer
            if json.Compact(&c, []byte(strings.TrimSpace(s))) == nil && json.Valid(c.Bytes()) {
                return c.String()
            }
            quoted, _ := json.Marshal(s)
            return string(quoted)
        }
    }
    var c bytes.Buffer
    if json.Compact(&c, t) == nil {
        return c.String()
    }
    return "{}"
}

// agentStreamArguments mirrors the stream path: non-string values are
// compacted, string values are used verbatim.
func agentStreamArguments(raw json.RawMessage) string {
    t := bytes.TrimSpace(raw)
    if len(t) == 0 || bytes.Equal(t, []byte("null")) {
        return "{}"
    }
    if t[0] == '"' {
        var s string
        if err := json.Unmarshal(t, &s); err == nil {
            return s
        }
    }
    var c bytes.Buffer
    if json.Compact(&c, t) == nil {
        return c.String()
    }
    return "{}"
}

// agentRandomHex returns n random bytes as lowercase hex (call-id suffixes).
func agentRandomHex(n int) string {
    b := make([]byte, n)
    if _, err := rand.Read(b); err != nil {
        panic(err)
    }
    return hex.EncodeToString(b)
}

// ParseAgentToolCalls extracts every complete tool-call block from finished
// text and returns OpenAI-format tool_calls objects.
func ParseAgentToolCalls(text string) []map[string]interface{} {
    text = NormalizeAgentFences(text)
    var calls []map[string]interface{}
    for _, span := range findAgentSpans(text) {
        name, args, ok := agentLooseParse(text[span.bodyStart:span.bodyEnd])
        if !ok || name == "" {
            continue
        }
        calls = append(calls, map[string]interface{}{
            "id":   "call_" + agentRandomHex(12),
            "type": "function",
            "function": map[string]interface{}{
                "name":      name,
                "arguments": agentParseArguments(args),
            },
        })
    }
    return calls
}

// StripAgentToolCalls removes all tool-call blocks from finished text.
func StripAgentToolCalls(text string) string {
    text = NormalizeAgentFences(text)
    var kept strings.Builder
    prev := 0
    for _, span := range findAgentSpans(text) {
        kept.WriteString(text[prev:span.start])
        prev = span.end
    }
    kept.WriteString(text[prev:])
    return strings.TrimSpace(kept.String())
}

// ── streaming interceptor ────────────────────────────────────────────────────

// AgentStreamInterceptor incrementally separates ordinary text from tool-call
// blocks. It retains a short suffix so a marker split across upstream chunks
// is never leaked to the client.
//
// Tool calls STREAM like the OpenAI / Anthropic APIs: as soon as the block's
// {"name": ..., "arguments": { has arrived the interceptor emits the header
// delta (index + id + type + function.name), then forwards the arguments
// object byte-by-byte as id-less fragments while the model is still writing
// it — the client sees live tool-call SSE instead of a long silence followed
// by one buffered blob. Payload shapes that cannot be streamed safely
// (non-object arguments, flat payloads, a name that never resolves) fall
// back to the old behaviour: the whole block is buffered between the markers
// and emitted as one complete call.
type AgentStreamInterceptor struct {
    buffer     string
    offset     int
    callIndex  int
    pendingSep bool // a tool-call block just closed: watch for a stray fence

    // ── incremental tool-call streaming state ──
    inCall         bool   // between the opening and closing markers
    tcBlockStart   int    // offset of the opening marker (for leak-as-content)
    tcName         string // resolved tool name, once complete
    tcNameFound    bool
    tcArgsFound    bool // the arguments value's opening '{' was located
    tcArgsPos      int  // absolute offset of that '{' in buffer
    tcArgsStreamed int  // bytes of the arguments object already emitted
    tcBraceDepth   int  // brace depth while scanning arguments
    tcInString     bool // inside a JSON string while scanning arguments
    tcEscapeNext   bool // next byte is escaped while scanning arguments
    tcArgsDone     bool // the arguments object's closing '}' was emitted
    tcFallback     bool // un-streamable shape: buffer the block, parse at the end
}

type AgentParsedChunk struct {
    Content   string
    ToolCalls []map[string]interface{}
}

func (in *AgentStreamInterceptor) Feed(chunk string) AgentParsedChunk {
    in.buffer += chunk
    return in.drain(false)
}

// Finish drains the interceptor at end of upstream stream, treating the
// buffered tail as complete data: a marker whose trailing '>' run touches the
// very end can now match, and whatever remains unparsed is ordinary content.
// Tool calls discovered here must still be forwarded to the client.
func (in *AgentStreamInterceptor) Finish() AgentParsedChunk {
    parsed := in.drain(true)
    in.offset = len(in.buffer)
    return parsed
}

// finishCall closes the current tool-call block: scanning resumes after it
// and the pendingSep pass swallows a stray fence the model may append.
func (in *AgentStreamInterceptor) finishCall() {
    in.inCall = false
    in.pendingSep = true
    in.tcName = ""
    in.tcNameFound = false
    in.tcArgsFound = false
    in.tcArgsPos = 0
    in.tcArgsStreamed = 0
    in.tcBraceDepth = 0
    in.tcInString = false
    in.tcEscapeNext = false
    in.tcArgsDone = false
    in.tcFallback = false
}

// jsonStringValueAt reads a `"key": "value"` string value starting the scan
// at pos (just past the quoted key). ok is false when the value is not a
// string or its closing quote has not arrived yet.
func jsonStringValueAt(s string, pos int) (string, bool) {
    for pos < len(s) && isASCIISpace(s[pos]) {
        pos++
    }
    if pos >= len(s) || s[pos] != ':' {
        return "", false
    }
    pos++
    for pos < len(s) && isASCIISpace(s[pos]) {
        pos++
    }
    if pos >= len(s) || s[pos] != '"' {
        return "", false
    }
    pos++
    start := pos
    for pos < len(s) {
        switch s[pos] {
        case '\\':
            pos += 2
            continue
        case '"':
            return s[start:pos], true
        }
        pos++
    }
    return "", false
}

// agentStreamExtractName resolves the tool name from partial block body text,
// accepting the same key spellings as agentExtractCall (priority order). The
// search stops before any arguments key so a nested "name" parameter inside
// the arguments object can never be mistaken for the tool name. ok is false
// while no accepted key carries a complete string value yet.
func agentStreamExtractName(body string) (name string, ok bool) {
    searchEnd := len(body)
    for _, k := range agentArgKeys {
        if i := strings.Index(body, `"`+k+`"`); i >= 0 && i < searchEnd {
            searchEnd = i
        }
    }
    region := body[:searchEnd]
    for _, k := range agentNameKeys {
        keyIdx := strings.Index(region, `"`+k+`"`)
        if keyIdx < 0 {
            continue
        }
        if v, ok := jsonStringValueAt(region, keyIdx+len(k)+2); ok {
            return v, true
        }
    }
    return "", false
}

// Outcomes of agentStreamFindArgs.
const (
    argsPending   = iota // no complete arguments value start visible yet
    argsObject           // value starts with '{' — bracePos is its offset
    argsNonObject        // value exists but is not an object — cannot stream
)

// agentStreamFindArgs locates the arguments value in partial block body text
// using the accepted key spellings (agentArgKeys, priority order).
func agentStreamFindArgs(body string) (bracePos int, state int) {
    for _, k := range agentArgKeys {
        keyIdx := strings.Index(body, `"`+k+`"`)
        if keyIdx < 0 {
            continue
        }
        pos := keyIdx + len(k) + 2
        for pos < len(body) && isASCIISpace(body[pos]) {
            pos++
        }
        if pos >= len(body) {
            return 0, argsPending // key arrived, colon pending
        }
        if body[pos] != ':' {
            return 0, argsNonObject // malformed — buffered parse will judge
        }
        pos++
        for pos < len(body) && isASCIISpace(body[pos]) {
            pos++
        }
        if pos >= len(body) {
            return 0, argsPending // value start pending
        }
        if body[pos] != '{' {
            return 0, argsNonObject // string/number arguments — buffer mode
        }
        return pos, argsObject
    }
    return 0, argsPending
}

// States of agentEndMarkerPrefix.
const (
    agentPrefixNo       = 0 // s can never grow into a tolerated marker
    agentPrefixMaybe    = 1 // s is a strict prefix of some tolerated marker
    agentPrefixComplete = 2 // s starts with a complete tolerated marker
)

// agentEndMarkerPrefix classifies s (which starts with '<') against the
// tolerated END_TOOL_CALL marker spellings (agentMinBrackets..agentMaxBrackets
// on each side). A "maybe" verdict means the bytes so far are consistent with
// a marker that is still arriving across upstream chunks — the caller must
// hold them back rather than stream them as argument content.
func agentEndMarkerPrefix(s string) (state int, markerLen int) {
    L := bracketRunForward(s, '<')
    if L > agentMaxBrackets {
        return agentPrefixNo, 0
    }
    rest := s[L:]
    word := agentEndWord
    k := 0
    for k < len(rest) && k < len(word) && rest[k] == word[k] {
        k++
    }
    if k == 0 {
        if len(rest) == 0 {
            return agentPrefixMaybe, 0 // bare '<' run, word may still start
        }
        return agentPrefixNo, 0 // '<' followed by a non-'E' byte: ordinary
    }
    if L < agentMinBrackets {
        return agentPrefixNo, 0 // lead run finalized below tolerance
    }
    if k < len(word) {
        if k == len(rest) {
            return agentPrefixMaybe, 0 // word still arriving
        }
        return agentPrefixNo, 0 // word mismatched mid-way
    }
    after := rest[len(word):]
    trail := bracketRunForward(after, '>')
    if trail > agentMaxBrackets {
        return agentPrefixNo, 0
    }
    if trail < agentMinBrackets && trail == len(after) {
        return agentPrefixMaybe, 0 // '>' run may still grow to tolerance
    }
    if trail < agentMinBrackets {
        return agentPrefixNo, 0 // non-'>' byte right after the word
    }
    return agentPrefixComplete, L + len(word) + trail
}

// runeSafeCut returns the largest prefix length of s that ends on a rune
// boundary, holding back at most utf8.UTFMax-1 trailing bytes of an
// incomplete multi-byte sequence. Argument fragments must be valid UTF-8 on
// their own: an upstream chunk boundary can split a rune, and forwarding the
// halves separately would surface as U+FFFD garble on the client (issue #23).
func runeSafeCut(s string) int {
    n := len(s)
    for n > 0 && !utf8.ValidString(s[:n]) && len(s)-n < utf8.UTFMax {
        n--
    }
    return n
}

func (in *AgentStreamInterceptor) drain(final bool) AgentParsedChunk {
    var content []string
    var toolCalls []map[string]interface{}

    for {
        // Immediately after a tool-call block, swallow blank space and stray
        // ```json / ``` fence lines the model appends despite instructions
        // (possibly split across chunks). Ordinary content elsewhere —
        // including its leading spaces and real code blocks — is untouched.
        if in.pendingSep {
            for {
                for in.offset < len(in.buffer) && isASCIISpace(in.buffer[in.offset]) {
                    in.offset++
                }
                n := SkipLeadingAgentFence(in.buffer[in.offset:])
                if n == 0 {
                    break
                }
                in.offset += n
            }
            if agentPossibleFencePrefix(in.buffer[in.offset:]) && !final {
                break // could still become a fence; wait for more chunks
            }
            in.pendingSep = false
        }

        if in.inCall {
            if !in.drainToolCall(final, &content, &toolCalls) {
                break // block still streaming: wait for more chunks
            }
            continue
        }

        rest := in.buffer[in.offset:]
        start, markerLen := findAgentMarker(rest, agentStartWord, final)
        if start < 0 {
            if final {
                // End of data: everything left is ordinary content.
                if rest != "" {
                    content = append(content, rest)
                    in.offset = len(in.buffer)
                }
                break
            }
            // Hold back a window big enough for a fence line + partial marker
            // so neither can leak as content while split across chunks. A
            // marker reported incomplete keeps its bytes inside this window,
            // so nothing here can be part of a future match. The cut is
            // backed up to a rune boundary so a multi-byte character is
            // never split across emissions (invalid UTF-8 would render as
            // replacement-char garble on the client — issue #23).
            const keep = agentStreamKeep
            if len(rest) > keep {
                cut := len(rest) - keep
                for cut > 0 && !utf8.RuneStart(rest[cut]) {
                    cut--
                }
                if cut > 0 {
                    content = append(content, rest[:cut])
                    in.offset += cut
                }
            }
            break
        }
        if start > 0 {
            piece := TrimTrailingAgentFence(rest[:start])
            if piece != "" {
                content = append(content, piece)
            }
            in.offset += start
        }
        // Enter the tool-call block: the opening marker is consumed now and
        // the body streams incrementally until the closing marker.
        in.inCall = true
        in.tcBlockStart = in.offset
        in.offset += markerLen
    }
    return AgentParsedChunk{Content: strings.Join(content, ""), ToolCalls: toolCalls}
}

// drainToolCall processes the body of one open tool-call block. It returns
// false when more upstream bytes are needed, true once the block is fully
// consumed (drain continues scanning for what follows).
//
// Streaming contract (mirrors the OpenAI chunk sequence):
//   - the header delta carries index, id, type and function.name — emitted
//     as soon as the name AND an object-valued arguments key have arrived;
//   - each subsequent delta carries index and a function.arguments fragment;
//   - when header and the first fragment surface in the same pass they ride
//     one delta (exactly what OpenAI emits when a chunk covers both).
// emitArgsFragment forwards argsText[:emitEnd] as the next streamed delta: it
// merges into the header emitted earlier in the same pass (exactly what
// OpenAI does when one chunk covers the name and the first argument bytes),
// or rides its own id-less fragment delta when the header went out earlier.
func (in *AgentStreamInterceptor) emitArgsFragment(argsText string, emitEnd int, headerIdx int, toolCalls *[]map[string]interface{}) {
    if emitEnd <= in.tcArgsStreamed {
        return // nothing new to forward
    }
    frag := argsText[in.tcArgsStreamed:emitEnd]
    in.tcArgsStreamed = emitEnd
    if headerIdx >= 0 {
        (*toolCalls)[headerIdx]["function"].(map[string]interface{})["arguments"] = frag
        return
    }
    *toolCalls = append(*toolCalls, map[string]interface{}{
        "index": in.callIndex,
        "function": map[string]interface{}{
            "arguments": frag,
        },
    })
}

func (in *AgentStreamInterceptor) drainToolCall(final bool, content *[]string, toolCalls *[]map[string]interface{}) bool {
    headerIdx := -1 // toolCalls slot of the header emitted in this pass

    for {
        body := in.buffer[in.offset:]

        // ── Fallback mode: buffer everything, parse the complete block ──
        if in.tcFallback {
            idx, markerLen := findAgentMarker(body, agentEndWord, final)
            if idx < 0 {
                if !final {
                    return false
                }
                // Stream ended mid-block: drop the tail (existing policy).
                in.offset = len(in.buffer)
                in.finishCall()
                return true
            }
            end := in.offset + idx
            raw := strings.TrimSpace(in.buffer[in.offset:end])
            if name, args, ok := agentLooseParse(raw); ok && name != "" {
                *toolCalls = append(*toolCalls, map[string]interface{}{
                    "index": in.callIndex,
                    "id":    "call_" + agentRandomHex(12),
                    "type":  "function",
                    "function": map[string]interface{}{
                        "name":      name,
                        "arguments": agentStreamArguments(args),
                    },
                })
                in.callIndex++
            } else {
                // invalid model block: leave it as visible text
                *content = append(*content, in.buffer[in.tcBlockStart:end+markerLen])
            }
            in.offset = end + markerLen
            in.finishCall()
            return true
        }

        // ── Phase 1: resolve the tool name ──
        if !in.tcNameFound {
            if name, ok := agentStreamExtractName(body); ok {
                in.tcName = name
                in.tcNameFound = true
            } else {
                // No complete name yet: if the block already closed (or the
                // stream ended) it never will — parse the whole body instead.
                if idx, _ := findAgentMarker(body, agentEndWord, final); idx >= 0 || final {
                    in.tcFallback = true
                    continue
                }
                return false
            }
        }

        // ── Phase 2: locate the arguments object, then emit the header ──
        if !in.tcArgsFound {
            pos, state := agentStreamFindArgs(body)
            switch state {
            case argsObject:
                in.tcArgsFound = true
                in.tcArgsPos = in.offset + pos
                *toolCalls = append(*toolCalls, map[string]interface{}{
                    "index": in.callIndex,
                    "id":    "call_" + agentRandomHex(12),
                    "type":  "function",
                    "function": map[string]interface{}{
                        "name":      in.tcName,
                        "arguments": "",
                    },
                })
                headerIdx = len(*toolCalls) - 1
            case argsNonObject:
                in.tcFallback = true // e.g. arguments as a JSON-encoded string
                continue
            default: // argsPending
                if idx, _ := findAgentMarker(body, agentEndWord, final); idx >= 0 || final {
                    in.tcFallback = true // flat payload or name-only block
                    continue
                }
                return false
            }
        }

        // ── Phase 3: stream the arguments object byte-by-byte ──
        if !in.tcArgsDone {
            argsText := in.buffer[in.tcArgsPos:]
            i := in.tcArgsStreamed
            truncatedAt, truncatedLen := -1, 0
        scan:
            for i < len(argsText) {
                c := argsText[i]
                if in.tcEscapeNext {
                    in.tcEscapeNext = false
                    i++
                    continue
                }
                if c == '\\' {
                    in.tcEscapeNext = true
                    i++
                    continue
                }
                if c == '"' {
                    in.tcInString = !in.tcInString
                    i++
                    continue
                }
                if !in.tcInString {
                    switch c {
                    case '<':
                        // Outside a string this can only be the closing marker
                        // arriving before the braces balanced (the model bailed
                        // mid-JSON). Markers INSIDE a string value are content
                        // and deliberately never match here; a partial marker
                        // split across upstream chunks is held back, never
                        // streamed as argument bytes.
                        state, mLen := agentEndMarkerPrefix(argsText[i:])
                        switch {
                        case state == agentPrefixComplete:
                            truncatedAt, truncatedLen = i, mLen
                            break scan
                        case state == agentPrefixMaybe && !final:
                            in.emitArgsFragment(argsText, i, headerIdx, toolCalls)
                            return false // marker still arriving: wait for it
                        default:
                            i++ // ordinary byte inside malformed JSON
                            continue
                        }
                    case '{':
                        in.tcBraceDepth++
                    case '}':
                        in.tcBraceDepth--
                        if in.tcBraceDepth == 0 {
                            i++
                            in.tcArgsDone = true
                            break scan
                        }
                    }
                }
                i++
            }

            emitEnd := i
            if !in.tcArgsDone && truncatedAt < 0 {
                // Also at final: an incomplete trailing rune can never
                // complete — forwarding it would surface as U+FFFD garble.
                emitEnd = runeSafeCut(argsText[:i])
                // Held-back bytes are only the tail of one incomplete rune
                // (never a quote/backslash/brace), so re-scanning them next
                // pass cannot corrupt the string/escape/depth state.
            }
            in.emitArgsFragment(argsText, emitEnd, headerIdx, toolCalls)

            switch {
            case in.tcArgsDone:
                in.offset = in.tcArgsPos + in.tcArgsStreamed
            case truncatedAt >= 0:
                // Malformed call (unbalanced JSON): the header already went
                // out, so close the call at the marker instead of leaking the
                // marker bytes into the arguments string.
                in.offset = in.tcArgsPos + truncatedAt + truncatedLen
                in.callIndex++
                in.finishCall()
                return true
            default:
                return false // arguments still streaming
            }
        }

        // ── Phase 4: arguments complete — consume the closing marker ──
        rest := in.buffer[in.offset:]
        idx, markerLen := findAgentMarker(rest, agentEndWord, final)
        if idx < 0 {
            if final {
                // Truncated block: the streamed call stands; drop the tail.
                in.offset = len(in.buffer)
                in.callIndex++
                in.finishCall()
                return true
            }
            return false
        }
        in.offset += idx + markerLen
        in.callIndex++
        in.finishCall()
        return true
    }
}

func isASCIISpace(b byte) bool {
    return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}

// ============================================================================
// SHIM DISPATCH GLUE — lets the request handlers stay variant-agnostic
// ============================================================================
//
// The two agent-mode implementations (modern in this file, legacy in
// agent_legacy.go) expose slightly different APIs. These thin
// adapters/dispatchers present one uniform surface so
// chatCompletionsHandler / anthropicMessagesHandler pick the active shim
// purely from config.

// transformMessagesForAgentModern folds the whole conversation + tool contract
// into one XML-sectioned prompt and wraps it as a single Z.AI user message.
func transformMessagesForAgentModern(rawMessages json.RawMessage, toolsRaw json.RawMessage) ([]byte, error) {
    var msgs []agentMessage
    if err := json.Unmarshal(rawMessages, &msgs); err != nil {
        return nil, fmt.Errorf("agent transform (modern): parse messages: %w", err)
    }
    var tools []openAITool
    if len(toolsRaw) > 0 {
        _ = json.Unmarshal(toolsRaw, &tools)
    }
    prompt := buildAgentPrompt(msgs, tools)
    return wrapAgentPromptAsMessages(prompt)
}

// agentTransformMessages rewrites the incoming OpenAI messages array for the
// active agent shim, returning the JSON-encoded messages to send upstream.
//
// Both shims flatten message content to plain text, which drops the
// image_url parts that processVisionMessages rewrote to uploaded Z.AI file
// ids. Without those references the model never receives the pixels and
// answers "I don't see any image" even though the files array is attached —
// so the image parts are re-attached to the final user message afterwards.
func agentTransformMessages(rawMessages, toolsRaw json.RawMessage) ([]byte, error) {
    var out []byte
    var err error
    if config.agentModern() {
        out, err = transformMessagesForAgentModern(rawMessages, toolsRaw)
    } else {
        var tools []interface{}
        if len(toolsRaw) > 0 {
            _ = json.Unmarshal(toolsRaw, &tools)
        }
        out, err = transformMessagesForAgent(rawMessages, tools)
    }
    if err != nil {
        return out, err
    }
    if imageParts := extractImageParts(rawMessages); len(imageParts) > 0 {
        attached, aerr := attachImageParts(out, imageParts)
        if aerr == nil {
            return attached, nil
        }
        // On attach failure fall through to the text-only transform: the
        // request still completes (without vision) instead of erroring.
        logError("agent vision attach failed: " + aerr.Error())
    }
    return out, nil
}

// agentExtractToolCalls parses tool-call blocks out of finished assistant text
// using the active shim's parser.
func agentExtractToolCalls(text string) []map[string]interface{} {
    if config.agentModern() {
        return ParseAgentToolCalls(text)
    }
    return extractAgentToolCalls(text)
}

// agentStripToolCalls removes tool-call blocks from finished assistant text
// using the active shim's stripper.
func agentStripToolCalls(text string) string {
    if config.agentModern() {
        return StripAgentToolCalls(text)
    }
    return stripAgentToolCallBlocks(text)
}

// agentInterceptor is the uniform streaming-interceptor surface used by both
// the OpenAI and Anthropic handlers. feed processes one upstream chunk;
// finish drains the tail at end of stream.
type agentInterceptor interface {
    feed(chunk string) (content string, toolCalls []map[string]interface{})
    finish() (content string, toolCalls []map[string]interface{})
}

// modernAgentInterceptor adapts AgentStreamInterceptor (complete-call emission,
// tolerant markers, hold-back window) to the agentInterceptor interface.
type modernAgentInterceptor struct{ in *AgentStreamInterceptor }

func (m *modernAgentInterceptor) feed(chunk string) (string, []map[string]interface{}) {
    p := m.in.Feed(chunk)
    return p.Content, p.ToolCalls
}

func (m *modernAgentInterceptor) finish() (string, []map[string]interface{}) {
    p := m.in.Finish()
    return p.Content, p.ToolCalls
}

// legacyAgentInterceptor adapts the legacy agentStreamInterceptor
// (incremental argument streaming) to the agentInterceptor interface. Its
// finish returns only trailing content; end-of-stream tool calls are caught
// by the caller's extractAgentToolCalls safety net.
type legacyAgentInterceptor struct{ in *agentStreamInterceptor }

func (l *legacyAgentInterceptor) feed(chunk string) (string, []map[string]interface{}) {
    content, toolCalls, _ := l.in.feed(chunk)
    return content, toolCalls
}

func (l *legacyAgentInterceptor) finish() (string, []map[string]interface{}) {
    return l.in.flushFinal(), nil
}

// newAgentInterceptor constructs the streaming interceptor for the active shim.
func newAgentInterceptor() agentInterceptor {
    if config.agentModern() {
        return &modernAgentInterceptor{in: &AgentStreamInterceptor{}}
    }
    return &legacyAgentInterceptor{in: newAgentStreamInterceptor()}
}

// rearmAgentInterceptor replaces a stale interceptor (an upstream
// edit_content rewrite rewound text it had already consumed — issue #23)
// while preserving the client-visible call-index sequence: a call streamed
// before the rewind keeps its index, and the re-emitted block takes the
// next one instead of colliding inside SDK accumulation.
func rearmAgentInterceptor(old agentInterceptor) agentInterceptor {
    fresh := newAgentInterceptor()
    switch o := old.(type) {
    case *modernAgentInterceptor:
        if f, ok := fresh.(*modernAgentInterceptor); ok {
            f.in.callIndex = o.in.callIndex
        }
    case *legacyAgentInterceptor:
        if f, ok := fresh.(*legacyAgentInterceptor); ok {
            f.in.callIndex = o.in.callIndex
        }
    }
    return fresh
}
