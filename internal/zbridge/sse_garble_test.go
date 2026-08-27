// sse_garble_test.go
//
// Reproduction + regression tests for issue #23 (输出乱码的问题):
// garbled / unreadable characters interspersed in streamed output.
//
// Root causes proven here (all verified against the official Z.AI web
// frontend, prod-fe bundle):
//
//  1. edit_index is a UTF-16 code-unit offset — the frontend applies
//     `content.substring(0, edit_index) + edit_content` (JavaScript string
//     indexing). The old parser treated it as a rune count, splicing at
//     the wrong position whenever non-BMP characters (emoji, CJK ext-B)
//     were present.
//  2. The `content` SSE field is a FULL replacement (frontend:
//     `lt.content = rr`); the old parser appended it, duplicating text.
//  3. The old parser diffed accumulated text by BYTE LENGTH and sliced
//     `content[sentLen:]`. After any edit that changed the byte length —
//     or a <details> tag streamed character by character — the slice
//     landed inside a multi-byte rune, emitting invalid UTF-8 that JSON
//     renderers display as U+FFFD replacement garble (the reported
//     "无法查看的乱码"). Partial "<det" tag fragments also leaked.
//
// The tests feed synthetic Z.AI SSE streams (shaped exactly like the
// documented upstream protocol) through the REAL streamSSEResponse
// parser, then replay the emitted ZAIResults through the exact logic of
// chatCompletionsHandler / anthropicStreamResponse, and assert what an
// OpenAI-compatible client actually receives.

package zbridge

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// sseEvent wraps one upstream payload as an SSE `data:` line.
func sseEvent(payload string) string {
	return "data: " + payload + "\n\n"
}

// runSSEParser runs the real streamSSEResponse over a synthetic body and
// collects every ZAIResult the channel receives.
func runSSEParser(t *testing.T, body string) []ZAIResult {
	t.Helper()
	ch := make(chan ZAIResult, 4096)
	errCh := make(chan error, 1)
	go func() {
		// sendToZAI closes the channel after sendToZAIStream returns;
		// mirror that here so the drain loop terminates.
		errCh <- streamSSEResponse(strings.NewReader(body), ch)
		close(ch)
	}()
	var results []ZAIResult
	for r := range ch {
		results = append(results, r)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("streamSSEResponse returned error: %v", err)
	}
	return results
}

// replayClientView replicates VERBATIM the accumulation logic of
// chatCompletionsHandler / anthropicStreamResponse after the issue #23
// fix: the parser owns diffing, the handler forwards result.Chunk and
// tracks the result.FullText snapshot.
func replayClientView(results []ZAIResult) (clientText string, deltas []string) {
	fullContent := ""
	for _, result := range results {
		if result.Reasoning != "" {
			continue // reasoning travels on its own channel
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
		deltas = append(deltas, delta)
	}
	return strings.Join(deltas, ""), deltas
}

// assertValidUTF8 fails the test if any client-visible delta contains
// invalid UTF-8 (which json.Marshal would replace with U+FFFD — the
// garbled characters reported in issue #23).
func assertValidUTF8(t *testing.T, deltas []string) {
	t.Helper()
	for i, d := range deltas {
		if !utf8.ValidString(d) {
			t.Errorf("client delta #%d is INVALID UTF-8 (renders as replacement-char garble): %q", i, d)
		}
	}
}

// withHoldback runs fn with a temporary StreamHoldback value.
func withHoldback(t *testing.T, n int, fn func()) {
	t.Helper()
	old := config.StreamHoldback
	config.StreamHoldback = n
	defer func() { config.StreamHoldback = old }()
	fn()
}

// ── Baseline: pure delta_content append must stay clean ─────────────────────

func TestSSEBaselinePureDeltaAppend(t *testing.T) {
	body := sseEvent(`{"data":{"delta_content":"你好，"}}`) +
		sseEvent(`{"data":{"delta_content":"世界 🙂"}}`) +
		sseEvent(`{"data":{"phase":"done"}}`) +
		"data: [DONE]\n\n"

	for _, hb := range []int{0, 24} {
		withHoldback(t, hb, func() {
			results := runSSEParser(t, body)
			clientText, deltas := replayClientView(results)
			assertValidUTF8(t, deltas)
			if clientText != "你好，世界 🙂" {
				t.Errorf("holdback=%d: client text = %q, want %q", hb, clientText, "你好，世界 🙂")
			}
		})
	}
}

// ── Issue #23 core repro: edit_content revision garbled the output ──────────
//
// Upstream streams "Hello 你好", then revises via edit_content from UTF-16
// index 6 to "世界！". With hold-back disabled (worst case: the rewrite
// touches text already forwarded to the client) the client must still
// receive only valid UTF-8 — the old code sliced mid-rune here and
// emitted raw continuation bytes.

func TestSSEEditContentRevisionNeverEmitsGarble(t *testing.T) {
	body := sseEvent(`{"data":{"delta_content":"Hello 你好"}}`) +
		sseEvent(`{"data":{"edit_content":"世界！","edit_index":6}}`) +
		sseEvent(`{"data":{"phase":"done"}}`) +
		"data: [DONE]\n\n"

	withHoldback(t, 0, func() {
		results := runSSEParser(t, body)
		clientText, deltas := replayClientView(results)
		assertValidUTF8(t, deltas)
		// The stale "你好" cannot be taken back from an append-only
		// client, but the corrected tail must follow it intact.
		if !strings.HasSuffix(clientText, "世界！") {
			t.Errorf("client text = %q, must end with the corrected tail %q", clientText, "世界！")
		}
	})
}

// With the default hold-back window the tail revision happens inside the
// pending region, so the client must converge EXACTLY on the upstream's
// final text — no stale fragment, no garble.
func TestSSEEditContentRevisionAbsorbedByHoldback(t *testing.T) {
	body := sseEvent(`{"data":{"delta_content":"Hello 你好"}}`) +
		sseEvent(`{"data":{"edit_content":"世界！","edit_index":6}}`) +
		sseEvent(`{"data":{"phase":"done"}}`) +
		"data: [DONE]\n\n"

	withHoldback(t, 24, func() {
		results := runSSEParser(t, body)
		clientText, deltas := replayClientView(results)
		assertValidUTF8(t, deltas)
		want := "Hello 世界！"
		if clientText != want {
			t.Errorf("client text = %q, want %q", clientText, want)
		}
	})
}

// ── Deep edit far beyond the hold-back window must stay readable ────────────

func TestSSEDeepEditStaysValidUTF8(t *testing.T) {
	// 30 runes of Chinese (90 bytes), then the model backtracks to rune 2
	// and retypes a LONGER passage (ASCII + Chinese mix, as real output
	// contains markdown/spaces). The old byte-length diff sliced the new
	// content at the old byte offset — inside a multi-byte rune.
	longPrefix := strings.Repeat("长", 30)
	body := sseEvent(fmt.Sprintf(`{"data":{"delta_content":"%s"}}`, longPrefix)) +
		sseEvent(`{"data":{"edit_content":"短文abc` + longPrefix + `","edit_index":2}}`) +
		sseEvent(`{"data":{"phase":"done"}}`) +
		"data: [DONE]\n\n"

	withHoldback(t, 0, func() {
		results := runSSEParser(t, body)
		_, deltas := replayClientView(results)
		assertValidUTF8(t, deltas)
	})
}

// ── <details> reasoning tag split across SSE events ─────────────────────────
//
// The upstream streams the "<details" opener one fragment at a time. The
// old parser emitted the partial "<det" fragment as content, then sliced
// mid-rune when the tag completed and content shrank back — the exact
// "穿插着一些无法查看的乱码" pattern from the issue report.

func TestSSEDetailsTagSplitAcrossEvents(t *testing.T) {
	body := sseEvent(`{"data":{"delta_content":"答案"}}`) +
		sseEvent(`{"data":{"delta_content":"<det"}}`) +
		sseEvent(`{"data":{"delta_content":"ails>思考过程</details>"}}`) +
		sseEvent(`{"data":{"delta_content":"正文内容"}}`) +
		sseEvent(`{"data":{"phase":"done"}}`) +
		"data: [DONE]\n\n"

	for _, hb := range []int{0, 24} {
		withHoldback(t, hb, func() {
			results := runSSEParser(t, body)
			clientText, deltas := replayClientView(results)

			assertValidUTF8(t, deltas)
			if strings.Contains(clientText, "<det") || strings.Contains(clientText, "<details") {
				t.Errorf("holdback=%d: <details> tag fragment leaked into client content: %q", hb, clientText)
			}
			want := "答案正文内容"
			if clientText != want {
				t.Errorf("holdback=%d: client text = %q, want %q", hb, clientText, want)
			}

			// Reasoning must still arrive intact on the reasoning channel.
			reasoning := ""
			for _, r := range results {
				reasoning += r.Reasoning
			}
			if reasoning != "思考过程" {
				t.Errorf("holdback=%d: reasoning = %q, want %q", hb, reasoning, "思考过程")
			}
		})
	}
}

// ── edit_index is UTF-16, not runes (owner's hypothesis, confirmed) ─────────
//
// The official frontend does content.substring(0, edit_index) — UTF-16
// code units. "A🙂" is 2 runes but 3 UTF-16 units; an edit right after
// the emoji arrives with edit_index=3. The old rune-based conversion
// spliced one character too late and silently corrupted the text.

func TestSSEEditIndexIsUTF16Units(t *testing.T) {
	body := sseEvent(`{"data":{"delta_content":"A🙂B"}}`) +
		// UTF-16 index 3 = after "A" (1 unit) + "🙂" (2 units)
		sseEvent(`{"data":{"edit_content":"C","edit_index":3}}`) +
		sseEvent(`{"data":{"phase":"done"}}`) +
		"data: [DONE]\n\n"

	// Hold-back absorbs the tail rewrite, so the client must converge
	// exactly. Under the old rune-based conversion edit_index=3 spliced
	// AFTER the "B" (3 runes), appending instead of replacing — the
	// client would have seen "A🙂BC".
	withHoldback(t, 24, func() {
		results := runSSEParser(t, body)
		clientText, deltas := replayClientView(results)
		assertValidUTF8(t, deltas)
		want := "A🙂C"
		if clientText != want {
			t.Errorf("client text = %q, want %q (edit_index must be interpreted as UTF-16 units)", clientText, want)
		}
	})
}

// Pure-BMP text: UTF-16 units == runes, must keep working.
func TestSSEEditIndexBMPText(t *testing.T) {
	body := sseEvent(`{"data":{"delta_content":"你好世界"}}`) +
		sseEvent(`{"data":{"edit_content":"地球","edit_index":2}}`) +
		sseEvent(`{"data":{"phase":"done"}}`) +
		"data: [DONE]\n\n"

	// Hold-back absorbs the tail rewrite: the client must converge exactly.
	withHoldback(t, 24, func() {
		results := runSSEParser(t, body)
		clientText, deltas := replayClientView(results)
		assertValidUTF8(t, deltas)
		want := "你好地球"
		if clientText != want {
			t.Errorf("client text = %q, want %q", clientText, want)
		}
	})
}

// ── `content` events are full replacements, not appends ─────────────────────
//
// Official frontend: `lt.content = rr` ("全量更新"). The old parser
// appended `content` events, duplicating the whole message whenever the
// upstream sent a snapshot.

func TestSSEContentFieldIsFullReplacement(t *testing.T) {
	body := sseEvent(`{"data":{"delta_content":"partial text"}}`) +
		sseEvent(`{"data":{"content":"完整的最终文本"}}`) +
		sseEvent(`{"data":{"phase":"done"}}`) +
		"data: [DONE]\n\n"

	// Hold-back keeps "partial text" pending, so the replacement happens
	// before anything reaches the client: it must see ONLY the snapshot.
	// (The old parser APPENDED the snapshot, duplicating everything.)
	withHoldback(t, 24, func() {
		results := runSSEParser(t, body)
		clientText, deltas := replayClientView(results)
		assertValidUTF8(t, deltas)
		if strings.Contains(clientText, "partial text") {
			t.Errorf("content snapshot must REPLACE, client text = %q", clientText)
		}
		if !strings.HasSuffix(clientText, "完整的最终文本") {
			t.Errorf("client text = %q, must end with the snapshot", clientText)
		}
	})
}

// ── Long stream with interleaved edits: parser contract holds ───────────────
//
// A realistic-length stream (well past the hold-back window) with mixed
// Chinese/ASCII/emoji and periodic tail edits. Every chunk reaching the
// client must be valid UTF-8, FullText snapshots must equal the running
// concatenation of chunks, and the final text must match upstream.

func TestSSELongStreamWithTailEditsContract(t *testing.T) {
	var b strings.Builder
	// 40 events of mixed text, then a tail edit inside the hold-back zone.
	for i := 0; i < 40; i++ {
		b.WriteString(sseEvent(fmt.Sprintf(`{"data":{"delta_content":"第%d段。"}}`, i)))
	}
	// Tail backtrack: replace the last two events' text (within the 24-rune
	// hold-back zone). Segments 0-9 are 4 runes, 10-37 are 5 runes each:
	// 10×4 + 28×5 = 180 UTF-16 units before segments 38-39.
	b.WriteString(sseEvent(`{"data":{"edit_content":"最终段。","edit_index":180}}`))
	b.WriteString(sseEvent(`{"data":{"delta_content":"收尾 🙂"}}`))
	b.WriteString(sseEvent(`{"data":{"phase":"done"}}`))
	b.WriteString("data: [DONE]\n\n")

	withHoldback(t, 24, func() {
		results := runSSEParser(t, b.String())
		clientText, deltas := replayClientView(results)

		assertValidUTF8(t, deltas)

		// Parser contract: every emitted chunk must be a suffix of its
		// FullText snapshot (never a mid-string slice), and the final
		// snapshot must equal the upstream's final content.
		for i, r := range results {
			if r.Reasoning != "" || r.Chunk == "" {
				continue
			}
			if !strings.HasSuffix(r.FullText, r.Chunk) {
				t.Fatalf("result #%d: chunk %q is not a suffix of snapshot %q", i, r.Chunk, r.FullText)
			}
		}
		last := ""
		for _, r := range results {
			if r.FullText != "" {
				last = r.FullText
			}
		}

		want := strings.Builder{}
		for i := 0; i < 38; i++ {
			want.WriteString(fmt.Sprintf("第%d段。", i))
		}
		want.WriteString("最终段。收尾 🙂")
		if last != want.String() {
			t.Errorf("final snapshot = %q, want %q", last, want.String())
		}
		if clientText != want.String() {
			t.Errorf("client text = %q, want %q", clientText, want.String())
		}
	})
}

// ── Unit tests for the helper functions ─────────────────────────────────────

func TestUTF16IndexToByteIndex(t *testing.T) {
	cases := []struct {
		s    string
		idx  int
		want int
	}{
		{"", 5, 0},                // empty string
		{"abc", 0, 0},             // zero
		{"abc", -3, 0},            // negative
		{"abc", 2, 2},             // ASCII
		{"abc", 99, 3},            // beyond end -> clamp
		{"你好", 1, 3},            // BMP Chinese: 1 unit = 1 rune = 3 bytes
		{"你好", 2, 6},            // end
		{"A🙂B", 0, 0},            // before emoji
		{"A🙂B", 1, 1},            // after 'A'
		{"A🙂B", 2, 1},            // inside the surrogate pair -> clamp to rune start
		{"A🙂B", 3, 5},            // after the emoji (1 + 4 bytes)
		{"A🙂B", 4, 6},            // after 'B'
		{"A🙂B", 100, 6},          // beyond end
		{"你好🙂世界", 4, 10},     // 2+2 units then emoji: bytes 6+4=10
	}
	for _, c := range cases {
		if got := utf16IndexToByteIndex(c.s, c.idx); got != c.want {
			t.Errorf("utf16IndexToByteIndex(%q, %d) = %d, want %d", c.s, c.idx, got, c.want)
		}
		// The result must always be a rune boundary.
		got := utf16IndexToByteIndex(c.s, c.idx)
		if got < len(c.s) && !utf8.RuneStart(c.s[got]) {
			t.Errorf("utf16IndexToByteIndex(%q, %d) = %d lands mid-rune", c.s, c.idx, got)
		}
	}
}

func TestCommonPrefixLenRuneSafe(t *testing.T) {
	cases := []struct{ a, b string; want int }{
		{"", "", 0},
		{"abc", "abd", 2},
		{"你好世界", "你好地球", 6},
		{"A🙂B", "A🙂C", 5},
		{"abc", "abc", 3},
		{"abc", "xabc", 0},
	}
	for _, c := range cases {
		if got := commonPrefixLen(c.a, c.b); got != c.want {
			t.Errorf("commonPrefixLen(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSSEEmitterDelta(t *testing.T) {
	// Normal growth.
	e := &sseEmitter{}
	if got := e.delta("abc"); got != "abc" {
		t.Errorf("growth from empty: got %q, want %q", got, "abc")
	}
	if got := e.delta("abcdef"); got != "def" {
		t.Errorf("growth: got %q, want %q", got, "def")
	}
	if got := e.delta("abcdef"); got != "" {
		t.Errorf("no-op: got %q, want empty", got)
	}

	// Multibyte growth stays rune-aligned.
	e2 := &sseEmitter{}
	if got := e2.delta("你好"); got != "你好" {
		t.Errorf("multibyte growth: got %q", got)
	}
	if got := e2.delta("你好世界"); got != "世界" {
		t.Errorf("multibyte growth delta: got %q, want %q", got, "世界")
	}

	// Deep truncation: target is a prefix of the client view — nothing is
	// sent and the view is NOT rewound, so the following growth does not
	// re-emit text the client already has.
	e3 := &sseEmitter{}
	e3.delta("你好世界天地")
	if got := e3.delta("你好世界"); got != "" {
		t.Errorf("shrink must emit nothing, got %q", got)
	}
	if got := e3.delta("你好世界天地人"); got != "人" {
		t.Errorf("growth after shrink: got %q, want %q", got, "人")
	}

	// Rewrite inside the view: re-sync from the common prefix, valid UTF-8.
	e4 := &sseEmitter{}
	e4.delta("Hello 你好")
	got := e4.delta("Hello 世界！")
	if got != "世界！" {
		t.Errorf("rewrite re-sync: got %q, want %q", got, "世界！")
	}
	if !utf8.ValidString(got) {
		t.Errorf("rewrite re-sync produced invalid UTF-8: %q", got)
	}

	// Full rewrite.
	e5 := &sseEmitter{}
	e5.delta("abc")
	if got := e5.delta("xyz"); got != "xyz" {
		t.Errorf("full rewrite: got %q, want %q", got, "xyz")
	}
}

func TestHoldBackTail(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"", 5, ""},
		{"abc", 0, "abc"},
		{"abc", 2, "a"},
		{"abc", 99, ""},
		{"你好世界", 2, "你好"},
		{"A🙂B", 1, "A🙂"},
		{"A🙂B", 2, "A"},
	}
	for _, c := range cases {
		got := holdBackTail(c.s, c.n)
		if got != c.want {
			t.Errorf("holdBackTail(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("holdBackTail(%q, %d) produced invalid UTF-8: %q", c.s, c.n, got)
		}
	}
}

func TestHoldBackPartialDetailsTag(t *testing.T) {
	cases := []struct{ s, want string }{
		{"abc", "abc"},
		{"abc<", "abc"},
		{"abc<d", "abc"},
		{"abc<det", "abc"},
		{"abc<details", "abc"},               // complete opener literal held
		{"abc</det", "abc"},
		{"abc</details>", "abc</details>"},   // complete close tag stays (literal text)
		{"abc<details>", "abc<details>"},     // splitDetails owns complete openers; not this layer
		{"abc<div>", "abc<div>"},             // unrelated tag stays
		{"abc<details> x", "abc<details> x"}, // opener already followed by more text stays
	}
	for _, c := range cases {
		if got := holdBackPartialDetailsTag(c.s); got != c.want {
			t.Errorf("holdBackPartialDetailsTag(%q) = %q, want %q", c.s, got, c.want)
		}
	}
}

func TestSplitDetails(t *testing.T) {
	cases := []struct {
		raw        string
		wantReason string
		wantContent string
	}{
		{"plain text", "", "plain text"},
		{"<details>think</details>answer", "think", "answer"},
		{"before<details>think</details>after", "think", "beforeafter"},
		// A partial tag FRAGMENT ("<det") passes through splitDetails —
		// it is held back at the emission layer (holdBackPartialDetailsTag)
		// while streaming and only released as literal text by the final
		// flush if the stream really ends there.
		{"answer<det", "", "answer<det"},
		// Complete opener literal without '>' yet: held pending entirely.
		{"answer<details", "", "answer"},
		{"answer<details sty", "", "answer"},
		// opener complete, reasoning still streaming
		{"a<details>thinking so far", "thinking so far", "a"},
		// multiple blocks
		{"<details>t1</details>x<details>t2</details>y", "t1t2", "xy"},
		// attributes on the opener
		{"<details type=\"thinking\">t</details>ans", "t", "ans"},
	}
	for _, c := range cases {
		r, content := splitDetails(c.raw)
		if r != c.wantReason || content != c.wantContent {
			t.Errorf("splitDetails(%q) = (%q, %q), want (%q, %q)",
				c.raw, r, content, c.wantReason, c.wantContent)
		}
	}
}
