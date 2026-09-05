package zbridge

import (
    "encoding/json"
    "regexp"
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

var nativeToolCallRe = regexp.MustCompile(`(?s)<tool_call>\s*([A-Za-z0-9_.\-]+)\s*(.*?)</tool_call>`)

var nativeArgPairRe = regexp.MustCompile(`(?s)<arg_key>(.*?)</arg_key>\s*<arg_value>(.*?)</arg_value>`)

// HasNativeToolCall reports whether s contains a complete native block.
func HasNativeToolCall(s string) bool {
    return nativeToolCallRe.MatchString(s)
}

// nativeToolCallPrefix reports whether s ends inside what could still grow
// into a native tool-call block, so a streaming caller knows to hold the tail
// back instead of leaking half a block to the client.
func nativeToolCallPrefix(s string) bool {
    idx := strings.LastIndex(s, "<tool_call>")
    if idx < 0 {
        // A bare "<tool_c…" tail may still complete on the next chunk.
        for k := 1; k < len(nativeToolOpen) && k <= len(s); k++ {
            if strings.HasSuffix(s, nativeToolOpen[:k]) {
                return true
            }
        }
        return false
    }
    return !strings.Contains(s[idx:], "</tool_call>")
}

const nativeToolOpen = "<tool_call>"
const nativeToolClose = "</tool_call>"

// TranslateNativeToolCalls rewrites every complete native block into the
// canonical marker form. Text outside such blocks is returned untouched, and
// input without a native block is returned unchanged.
func TranslateNativeToolCalls(text string) string {
    if !strings.Contains(text, nativeToolOpen) {
        return text
    }
    return nativeToolCallRe.ReplaceAllStringFunc(text, func(block string) string {
        m := nativeToolCallRe.FindStringSubmatch(block)
        if len(m) != 3 {
            return block
        }
        name := strings.TrimSpace(m[1])
        if name == "" {
            return block
        }
        args := map[string]interface{}{}
        for _, pair := range nativeArgPairRe.FindAllStringSubmatch(m[2], -1) {
            key := strings.TrimSpace(pair[1])
            if key == "" {
                continue
            }
            // Values are raw text, including newlines and quotes; json.Marshal
            // on the whole map escapes them correctly.
            args[key] = strings.Trim(pair[2], "\n")
        }
        payload := map[string]interface{}{"name": name, "arguments": args}
        encoded, err := json.Marshal(payload)
        if err != nil {
            return block
        }
        return agentToolStart + string(encoded) + agentToolEnd
    })
}

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
            // Truncated block at end of stream: nothing to translate, let the
            // normal path emit it as content rather than swallowing output.
            return false, false
        }
        // Emit what precedes the block and hold the rest until it closes, so
        // half a block never reaches the client as text.
        if nStart > 0 {
            *content = append(*content, rest[:nStart])
            in.offset += nStart
        }
        return true, false
    }
    blockEnd := nStart + endIdx + len(nativeToolClose)
    if nStart > 0 {
        *content = append(*content, rest[:nStart])
    }
    *toolCalls = append(*toolCalls, ParseAgentToolCalls(rest[nStart:blockEnd])...)
    in.offset += blockEnd
    in.pendingSep = true
    return true, true
}
