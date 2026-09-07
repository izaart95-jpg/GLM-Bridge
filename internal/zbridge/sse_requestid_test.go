package zbridge

import (
    "log"
    "strings"
    "testing"
)

// streamSSEResponse prefixes its debug output with the request id when one is
// given, so concurrent streams can be attributed in a shared log (issue #36).
func TestSSEDebugLinesCarryRequestId(t *testing.T) {
    origW := log.Writer()
    origFlags := log.Flags()
    defer log.SetOutput(origW)
    defer log.SetFlags(origFlags)

    var sb strings.Builder
    log.SetOutput(&sb)
    log.SetFlags(0)

    ch := make(chan ZAIResult, 16)
    body := "data: {\"data\":{\"delta_content\":\"hello\"}}\n\n" +
        "data: {\"data\":{\"phase\":\"done\"}}\n\n" +
        "data: [DONE]\n\n"
    err := streamSSEResponse(strings.NewReader(body), ch, "req-abc123")
    close(ch)
    if err != nil {
        t.Fatalf("streamSSEResponse: %v", err)
    }
    for range ch { // drain; the real flow's sender closes ch after the call
    }

    out := sb.String()
    if !strings.Contains(out, "req-abc123") {
        t.Fatalf("request id missing from debug output:\n%s", out)
    }
    for _, line := range strings.Split(out, "\n") {
        if strings.Contains(line, "Z.AI SSE line") && !strings.Contains(line, "[req-abc123]") {
            t.Fatalf("SSE debug line without request id: %q", line)
        }
    }
}

// An empty request id keeps the legacy "[DEBUG]" prefix rather than emitting
// empty brackets, so the no-id call sites (tests, direct callers) stay clean.
func TestSSEDebugLinesLegacyPrefix(t *testing.T) {
    origW := log.Writer()
    origFlags := log.Flags()
    defer log.SetOutput(origW)
    defer log.SetFlags(origFlags)

    var sb strings.Builder
    log.SetOutput(&sb)
    log.SetFlags(0)

    ch := make(chan ZAIResult, 16)
    body := "data: {\"data\":{\"delta_content\":\"hi\"}}\n\ndata: [DONE]\n\n"
    err := streamSSEResponse(strings.NewReader(body), ch, "")
    close(ch)
    if err != nil {
        t.Fatalf("streamSSEResponse: %v", err)
    }
    for range ch {
    }

    if strings.Contains(sb.String(), "[]") {
        t.Fatalf("empty id produced empty brackets: %q", sb.String())
    }
    if !strings.Contains(sb.String(), "[DEBUG]") {
        t.Fatalf("legacy [DEBUG] prefix lost: %q", sb.String())
    }
}
