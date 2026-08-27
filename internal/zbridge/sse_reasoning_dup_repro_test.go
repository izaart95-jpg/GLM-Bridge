// sse_reasoning_dup_repro_test.go
//
// Reproduction + regression tests for the post-d2ee325 regression:
// clients receive reasoning_content as repeated growing prefixes
// (">First: OS release. `First: OS release. `cat /etc/os-release`
// First: OS release. `cat /etc/os-release` — but the instructions say
// use...") even though the upstream Z.AI stream (visible in debug
// logs) is correct.
//
// Mechanism: Z.AI streams the <details> reasoning body one character
// at a time (edit_content appends). Each reasoning line is markdown-
// quoted ("> ..."). While a new line's quote marker is only partially
// arrived (raw tail = "\n>"), stripDetailsTags' TrimPrefix("> ") does
// not match the bare ">" and TrimSpace keeps it, so the stripped
// reasoning snapshot transiently ends in a bare ">". One character
// later the "> " completes and the ">" is stripped away — the
// snapshot sequence is non-monotonic. The sseEmitter then diverged:
// its tracked view kept the stale ">" tail, every later snapshot
// shared only the pre-">" prefix with it, and everything after that
// point was re-emitted on EVERY flush — the exact growing-prefix
// duplication clients render.

package zbridge

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

// utf16Len returns the number of UTF-16 code units of s (JS string length).
func utf16Len(s string) int {
	n := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if l := utf16.RuneLen(r); l < 0 {
			n++ // invalid byte: the JS frontend also sees one unit here
		} else {
			n += l
		}
		i += size
	}
	return n
}

// editAppendEventsFrom builds one SSE event per rune of `add`,
// appending each rune at the running UTF-16 end-of-content offset —
// exactly the way the Z.AI prod-fe reconstructs it: content =
// content.substring(0, edit_index) + edit_content. It returns the
// events and the new accumulated content.
func editAppendEventsFrom(existing, add string) (string, string) {
	var body strings.Builder
	cur := existing
	for i := 0; i < len(add); {
		r, size := utf8.DecodeRuneInString(add[i:])
		payload, _ := json.Marshal(map[string]interface{}{
			"data": map[string]interface{}{
				"edit_content": string(r),
				"edit_index":   utf16Len(cur),
			},
		})
		body.WriteString(sseEvent(string(payload)))
		cur += string(r)
		i += size
	}
	return body.String(), cur
}

// editAppendEvents appends to an empty stream.
func editAppendEvents(full string) string {
	body, _ := editAppendEventsFrom("", full)
	return body
}

// editAppendStream wraps editAppendEvents with the stream terminators.
func editAppendStream(full string) string {
	return editAppendEvents(full) +
		sseEvent(`{"data":{"phase":"done"}}`) +
		"data: [DONE]\n\n"
}

// collectReasoning concatenates every Reasoning delta the parser emits.
func collectReasoning(results []ZAIResult) string {
	var sb strings.Builder
	for _, r := range results {
		sb.WriteString(r.Reasoning)
	}
	return sb.String()
}

// TestSSEReasoningQuotedLinesNoDuplication streams a <details> reasoning
// block whose lines carry "> " quote markers one character at a time.
// The client must receive the reasoning exactly once.
func TestSSEReasoningQuotedLinesNoDuplication(t *testing.T) {
	line1 := "First: OS release. `cat /etc/os-release` — but the instructions say use read tool for text files."
	line2 := "That's for workspace files generally. But for system info gathering, bash is the natural approach."
	inner := "> " + line1 + "\n> " + line2
	full := "<details>" + inner + "</details>The answer."

	for _, hb := range []int{0, 24} {
		withHoldback(t, hb, func() {
			results := runSSEParser(t, editAppendStream(full))

			reasoning := collectReasoning(results)
			want := line1 + "\n" + line2

			if reasoning != want {
				t.Errorf("holdback=%d: client reasoning mismatch\n got (%d chars): %q\nwant (%d chars): %q",
					hb, len(reasoning), reasoning, len(want), want)
			}
			if n := strings.Count(reasoning, "First: OS release."); n > 1 {
				t.Errorf("holdback=%d: reasoning duplicated %d times (growing-prefix symptom)", hb, n)
			}

			// Content channel must carry only the answer.
			clientText, _ := replayClientView(results)
			if clientText != "The answer." {
				t.Errorf("holdback=%d: client content = %q, want %q", hb, clientText, "The answer.")
			}
		})
	}
}

// TestSSEReasoningQuoteMarkerPartialArrival isolates the trigger: a
// partially-streamed "> " marker must never reach the client, and the
// tail rewrite it causes must not duplicate anything after it.
func TestSSEReasoningQuoteMarkerPartialArrival(t *testing.T) {
	withHoldback(t, 0, func() {
		full := "<details>> abc\n> def</details>ok"
		results := runSSEParser(t, editAppendStream(full))
		reasoning := collectReasoning(results)
		want := "abc\ndef"
		if reasoning != want {
			t.Errorf("client reasoning = %q, want %q", reasoning, want)
		}
		if strings.Contains(reasoning, ">") {
			t.Errorf("stray quote marker leaked into reasoning: %q", reasoning)
		}
	})
}

// TestSSEReasoningGrowthDeltasAreIncremental asserts the append-only
// client view never diverges from the expected reasoning: every
// partial concatenation of Reasoning deltas must stay a prefix of the
// final text (a re-emitted growing prefix violates this immediately).
func TestSSEReasoningGrowthDeltasAreIncremental(t *testing.T) {
	withHoldback(t, 0, func() {
		line1 := "First: OS release. `cat /etc/os-release`."
		want := line1 + "\nsecond line here"
		full := "<details>> " + line1 + "\n> second line here</details>ans"
		results := runSSEParser(t, editAppendStream(full))

		view := ""
		for i, r := range results {
			if r.Reasoning == "" {
				continue
			}
			view += r.Reasoning
			if !strings.HasPrefix(want, view) {
				t.Fatalf("delta #%d made the client view diverge from the expected reasoning:\n view = %q", i, view)
			}
		}
		if view != want {
			t.Errorf("final client reasoning = %q, want %q", view, want)
		}
	})
}

// TestSSEReasoningTailBacktrackAbsorbed verifies that a trailing
// edit_content backtrack inside the reasoning (the model revising the
// tail of its thinking) is absorbed by the hold-back window, so the
// client converges exactly — no stale fragment, no duplication.
func TestSSEReasoningTailBacktrackAbsorbed(t *testing.T) {
	withHoldback(t, 24, func() {
		// Reasoning grows past the hold-back window, then the model
		// backtracks inside the window and retypes the tail, then the
		// block closes and the answer follows.
		quoted := "> " + strings.Repeat("thinking text ", 12)
		body, cur := editAppendEventsFrom("", "<details>"+quoted+"wrong tail")
		// Backtrack: replace "wrong tail" with "right end".
		prefix := "<details>" + quoted
		payload, _ := json.Marshal(map[string]interface{}{
			"data": map[string]interface{}{
				"edit_content": "right end",
				"edit_index":   utf16Len(prefix),
			},
		})
		body += sseEvent(string(payload))
		cur = prefix + "right end"
		tail, _ := editAppendEventsFrom(cur, "</details>ans")
		body += tail
		body += sseEvent(`{"data":{"phase":"done"}}`)
		body += "data: [DONE]\n\n"

		results := runSSEParser(t, body)
		reasoning := collectReasoning(results)
		want := strings.Repeat("thinking text ", 12) + "right end"
		if reasoning != want {
			t.Errorf("client reasoning = %q, want %q", reasoning, want)
		}
		if strings.Contains(reasoning, "wrong tail") {
			t.Errorf("stale backtracked tail leaked into reasoning: %q", reasoning)
		}
		clientText, _ := replayClientView(results)
		if clientText != "ans" {
			t.Errorf("client content = %q, want %q", clientText, "ans")
		}
	})
}

// TestHoldBackPartialQuoteMarker covers the new helper directly.
func TestHoldBackPartialQuoteMarker(t *testing.T) {
	cases := []struct{ s, want string }{
		{"", ""},
		{"abc", "abc"},
		{">", ""},                  // bare marker at text start: held
		{"abc\n>", "abc\n"},        // bare marker on its own line: held
		{"abc\n> x", "abc\n> x"},   // marker already completed by text: kept
		{"abc\n>x", "abc\n>x"},     // not a quote marker pattern mid-stream: kept
		{"a > b", "a > b"},         // mid-line ">" is genuine text: kept
		{"a > b\n>", "a > b\n"},    // trailing bare marker still held
		{"abc>", "abc>"},           // not at line start: genuine text, kept
		{"<details>", "<details>"}, // not at line start: kept
	}
	for _, c := range cases {
		if got := holdBackPartialQuoteMarker(c.s); got != c.want {
			t.Errorf("holdBackPartialQuoteMarker(%q) = %q, want %q", c.s, got, c.want)
		}
	}
}
