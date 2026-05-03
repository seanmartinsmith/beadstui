package analysis

import (
	"sort"
	"strings"
	"testing"
	"time"
)

// helper: assert that a match exists with the given id and kind, at any
// offset, at least once in the result.
func hasMatch(matches []SigilMatch, id, kind string) bool {
	for _, m := range matches {
		if m.ID == id && m.Kind == kind {
			return true
		}
	}
	return false
}

// helper: find the single match for id; fail if absent or duplicated.
func singleMatch(t *testing.T, matches []SigilMatch, id string) SigilMatch {
	t.Helper()
	var out SigilMatch
	count := 0
	for _, m := range matches {
		if m.ID == id {
			out = m
			count++
		}
	}
	if count == 0 {
		t.Fatalf("no match for id=%q in %+v", id, matches)
	}
	if count > 1 {
		t.Fatalf("want 1 match for id=%q, got %d: %+v", id, count, matches)
	}
	return out
}

func TestDetectSigils_EmptyBody(t *testing.T) {
	for _, mode := range []SigilMode{SigilModeStrict, SigilModeVerb, SigilModePermissive} {
		if got := DetectSigils("", mode); got != nil {
			t.Errorf("mode=%v: empty body should return nil; got %+v", mode, got)
		}
	}
}

func TestDetectSigils_StrictMarkdownLink(t *testing.T) {
	body := "see [bt-abc](https://example.com/bt-abc) for details"
	got := DetectSigils(body, SigilModeStrict)
	m := singleMatch(t, got, "bt-abc")
	if m.Kind != SigilKindMarkdownLink {
		t.Errorf("kind=%q, want %q", m.Kind, SigilKindMarkdownLink)
	}
	// Offset points at the start of the ID (the 'b' after '[').
	if body[m.Offset:m.Offset+len("bt-abc")] != "bt-abc" {
		t.Errorf("offset %d does not point at bt-abc; body[offset:]=%q", m.Offset, body[m.Offset:m.Offset+6])
	}
}

func TestDetectSigils_StrictInlineCode(t *testing.T) {
	body := "wrap the id: `bt-abc` as inline code"
	got := DetectSigils(body, SigilModeStrict)
	m := singleMatch(t, got, "bt-abc")
	if m.Kind != SigilKindInlineCode {
		t.Errorf("kind=%q, want %q", m.Kind, SigilKindInlineCode)
	}
}

func TestDetectSigils_StrictRefKeyword(t *testing.T) {
	cases := []string{
		"ref:bt-abc",
		"ref: bt-abc",
		"refs:bt-abc",
		"refs: bt-abc",
		"REF: bt-abc",
		"Refs:bt-abc",
	}
	for _, body := range cases {
		got := DetectSigils(body, SigilModeStrict)
		m := singleMatch(t, got, "bt-abc")
		if m.Kind != SigilKindRefKeyword {
			t.Errorf("body=%q: kind=%q, want %q", body, m.Kind, SigilKindRefKeyword)
		}
	}
}

func TestDetectSigils_StrictIgnoresBareMention(t *testing.T) {
	body := "mentioning bt-abc without any markdown context"
	if got := DetectSigils(body, SigilModeStrict); got != nil {
		t.Errorf("strict mode should emit nothing for bare mention; got %+v", got)
	}
}

func TestDetectSigils_VerbProximityBefore(t *testing.T) {
	body := "see bt-abc for details"
	got := DetectSigils(body, SigilModeVerb)
	m := singleMatch(t, got, "bt-abc")
	if m.Kind != SigilKindVerb {
		t.Errorf("kind=%q, want %q", m.Kind, SigilKindVerb)
	}
}

func TestDetectSigils_VerbProximityAfter(t *testing.T) {
	// verb comes AFTER the ID; still within 32 chars same line.
	body := "bt-abc blocks the release"
	got := DetectSigils(body, SigilModeVerb)
	m := singleMatch(t, got, "bt-abc")
	if m.Kind != SigilKindVerb {
		t.Errorf("kind=%q, want %q", m.Kind, SigilKindVerb)
	}
}

func TestDetectSigils_VerbProximityInclusive32(t *testing.T) {
	// "see" occupies indices 0..2 (verb.end = 3 exclusive). A trailing
	// space + 30 filler + leading space before the ID puts "bt-abc" at
	// index 35. Distance = 35 - 3 = 32 chars (inclusive boundary).
	body := "see " + strings.Repeat("x", 30) + " bt-abc"
	got := DetectSigils(body, SigilModeVerb)
	if !hasMatch(got, "bt-abc", SigilKindVerb) {
		t.Errorf("32-char proximity should match; got %+v", got)
	}

	// One extra filler char: distance = 33, outside the budget.
	body33 := "see " + strings.Repeat("x", 31) + " bt-abc"
	got = DetectSigils(body33, SigilModeVerb)
	if hasMatch(got, "bt-abc", SigilKindVerb) {
		t.Errorf("33-char proximity should not match; got %+v", got)
	}
}

func TestDetectSigils_VerbAcrossLines_NoMatch(t *testing.T) {
	body := "see\nbt-abc"
	got := DetectSigils(body, SigilModeVerb)
	if hasMatch(got, "bt-abc", SigilKindVerb) {
		t.Errorf("verb on a different line must not match; got %+v", got)
	}
}

func TestDetectSigils_VerbMarkdownStripped(t *testing.T) {
	// `**see**` should count as "see" (3 chars) not 7, so ID 3 chars away
	// is within 32-char budget.
	body := "**see** bt-abc"
	got := DetectSigils(body, SigilModeVerb)
	m := singleMatch(t, got, "bt-abc")
	if m.Kind != SigilKindVerb {
		t.Errorf("kind=%q, want %q", m.Kind, SigilKindVerb)
	}
}

func TestDetectSigils_VerbMultipleVerbsSameID(t *testing.T) {
	// Two verbs bracketing one ID; ensure exactly one match emits.
	body := "see bt-abc closes"
	got := DetectSigils(body, SigilModeVerb)
	if len(got) != 1 {
		t.Errorf("want 1 match (dedup collapses), got %d: %+v", len(got), got)
	}
	if got[0].Kind != SigilKindVerb {
		t.Errorf("kind=%q, want verb", got[0].Kind)
	}
}

func TestDetectSigils_PermissiveBareMention(t *testing.T) {
	body := "just bt-abc somewhere in the prose"
	got := DetectSigils(body, SigilModePermissive)
	m := singleMatch(t, got, "bt-abc")
	if m.Kind != SigilKindBareMention {
		t.Errorf("kind=%q, want %q", m.Kind, SigilKindBareMention)
	}
}

func TestDetectSigils_PermissiveExcludesInlineCode(t *testing.T) {
	body := "`bt-abc` should NOT emit in permissive"
	if got := DetectSigils(body, SigilModePermissive); got != nil {
		t.Errorf("permissive must suppress inline code; got %+v", got)
	}
}

func TestDetectSigils_FencedCodeSuppressesAllModes(t *testing.T) {
	body := "```\nbt-abc\n```\n"
	for _, mode := range []SigilMode{SigilModeStrict, SigilModeVerb, SigilModePermissive} {
		if got := DetectSigils(body, mode); got != nil {
			t.Errorf("mode=%v: fenced code must suppress; got %+v", mode, got)
		}
	}
}

func TestDetectSigils_FencedCodeTildeSuppresses(t *testing.T) {
	body := "~~~\nbt-abc\n~~~\n"
	for _, mode := range []SigilMode{SigilModeStrict, SigilModeVerb, SigilModePermissive} {
		if got := DetectSigils(body, mode); got != nil {
			t.Errorf("mode=%v: tilde fence must suppress; got %+v", mode, got)
		}
	}
}

func TestDetectSigils_FenceInterleaved(t *testing.T) {
	// Prose before / between / after fences
	body := "see bt-one\n```\nbt-two\n```\nand bt-three too"
	got := DetectSigils(body, SigilModeVerb)
	if !hasMatch(got, "bt-one", SigilKindVerb) {
		t.Errorf("missing bt-one verb sigil: %+v", got)
	}
	for _, m := range got {
		if m.ID == "bt-two" {
			t.Errorf("bt-two inside fence leaked: %+v", m)
		}
	}
}

func TestDetectSigils_UnclosedFence_SuppressesRest(t *testing.T) {
	// Line 1 has a verb-adjacent ID (pre-fence prose). The unclosed
	// ```\n fence opens on line 2; everything after is in-fence and must
	// be suppressed.
	body := "see bt-one first\n```\nbt-two everywhere bt-three"
	got := DetectSigils(body, SigilModeVerb)
	if !hasMatch(got, "bt-one", SigilKindVerb) {
		t.Errorf("bt-one (pre-fence) must emit; got %+v", got)
	}
	for _, m := range got {
		if m.ID == "bt-two" || m.ID == "bt-three" {
			t.Errorf("post-unclosed-fence ID leaked: %+v", m)
		}
	}
}

func TestDetectSigils_EmptyLinkTextSkipped(t *testing.T) {
	body := "[](bt-abc) and [bt-abc]() contexts"
	// [](bt-abc): empty text, no markdown_link emission.
	// [bt-abc](): valid link text, empty URL, emits markdown_link.
	got := DetectSigils(body, SigilModeStrict)
	m := singleMatch(t, got, "bt-abc")
	if m.Kind != SigilKindMarkdownLink {
		t.Errorf("kind=%q, want %q", m.Kind, SigilKindMarkdownLink)
	}
}

func TestDetectSigils_DedupPriority(t *testing.T) {
	// Same ID appears in a markdown link and bare in the same body.
	// markdown_link priority wins.
	body := "[bt-abc](https://x) and see bt-abc later"
	got := DetectSigils(body, SigilModeVerb)
	m := singleMatch(t, got, "bt-abc")
	if m.Kind != SigilKindMarkdownLink {
		t.Errorf("kind=%q, want markdown_link (higher priority)", m.Kind)
	}
}

func TestDetectSigils_TruncationFlag(t *testing.T) {
	// Build a body exactly 1MB+1 with a bead-ID inside the first 1MB.
	body := "see bt-abc ... " + strings.Repeat("x", MaxSigilBodyBytes)
	got := DetectSigils(body, SigilModeVerb)
	if len(got) == 0 {
		t.Fatalf("expected at least one match")
	}
	if !got[0].Truncated {
		t.Errorf("truncated flag must be set on 1MB+ body; got %+v", got[0])
	}
}

func TestDetectSigils_ExactBoundary_NotTruncated(t *testing.T) {
	// Body exactly 1MB — not truncated.
	body := "see bt-abc " + strings.Repeat("x", MaxSigilBodyBytes-len("see bt-abc "))
	if len(body) != MaxSigilBodyBytes {
		t.Fatalf("body len = %d; want %d", len(body), MaxSigilBodyBytes)
	}
	got := DetectSigils(body, SigilModeVerb)
	if len(got) == 0 {
		t.Fatalf("expected a match")
	}
	if got[0].Truncated {
		t.Errorf("exact-boundary body must not be truncated; got %+v", got[0])
	}
}

func TestDetectSigils_InvalidUTF8_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on invalid UTF-8: %v", r)
		}
	}()
	// Lone surrogate bytes (invalid UTF-8) sandwiching a bead-ID.
	body := "\xed\xa0\x80 see bt-abc \xed\xa0\x80"
	got := DetectSigils(body, SigilModeVerb)
	if !hasMatch(got, "bt-abc", SigilKindVerb) {
		t.Errorf("verb sigil should still fire around invalid UTF-8; got %+v", got)
	}
}

func TestDetectSigils_RTLOverride_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on RTL override: %v", r)
		}
	}()
	body := "see ‮bt-abc‬"
	_ = DetectSigils(body, SigilModeVerb)
}

func TestDetectSigils_ZeroWidthJoiner_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on ZWJ: %v", r)
		}
	}()
	// ZWJ between verb and id should NOT be stripped (we only strip *,_,~).
	// The scanner just walks past the non-ASCII bytes.
	body := "see‍bt-abc"
	_ = DetectSigils(body, SigilModeVerb)
}

func TestDetectSigils_EmbeddedIDNotMatched(t *testing.T) {
	// "xbt-abc" should not match bt-abc (no boundary).
	body := "abt-abc is not a ref"
	got := DetectSigils(body, SigilModePermissive)
	for _, m := range got {
		if m.ID == "bt-abc" {
			t.Errorf("embedded ID leaked: %+v", m)
		}
	}
}

func TestDetectSigils_DottedSuffix(t *testing.T) {
	body := "see bt-mhwy.2 for epic parent"
	got := DetectSigils(body, SigilModeVerb)
	if !hasMatch(got, "bt-mhwy.2", SigilKindVerb) {
		t.Errorf("dotted suffix not matched: %+v", got)
	}
}

func TestDetectSigils_MultipleIDsSameLine(t *testing.T) {
	body := "see bt-one and bt-two"
	got := DetectSigils(body, SigilModeVerb)
	if !hasMatch(got, "bt-one", SigilKindVerb) {
		t.Errorf("missing bt-one: %+v", got)
	}
	if !hasMatch(got, "bt-two", SigilKindVerb) {
		t.Errorf("missing bt-two: %+v", got)
	}
}

func TestDetectSigils_CRLFLines(t *testing.T) {
	// CRLF line endings.
	body := "see bt-one\r\nalso bt-two under closes\r\n"
	got := DetectSigils(body, SigilModeVerb)
	if !hasMatch(got, "bt-one", SigilKindVerb) {
		t.Errorf("bt-one CRLF line missing: %+v", got)
	}
}

func TestDetectSigils_SingleHugeLine(t *testing.T) {
	// 100KB of filler, no newlines, one ID inside. Must not hang or panic.
	filler := strings.Repeat("x", 100_000)
	body := "see bt-abc " + filler
	got := DetectSigils(body, SigilModeVerb)
	if !hasMatch(got, "bt-abc", SigilKindVerb) {
		t.Errorf("bt-abc missing on huge-line input; got %d matches", len(got))
	}
}

func TestDetectSigils_InlineCodeStorm(t *testing.T) {
	// 100K alternating `a`b ... no panic, no timeout.
	var b strings.Builder
	for i := 0; i < 100_000; i++ {
		b.WriteByte('`')
		b.WriteByte('a')
	}
	_ = DetectSigils(b.String(), SigilModeVerb)
}

func TestDetectSigils_NestedFencesStorm(t *testing.T) {
	// 10K nested ~~~/``` alternations — hit the fence stack cap cleanly.
	var b strings.Builder
	for i := 0; i < 10_000; i++ {
		if i%2 == 0 {
			b.WriteString("~~~\n")
		} else {
			b.WriteString("```\n")
		}
	}
	b.WriteString("bt-escape\n")
	// Doesn't panic. Result may or may not include bt-escape depending on
	// fence balance; the test only asserts no panic.
	_ = DetectSigils(b.String(), SigilModeVerb)
}

func TestDetectSigils_LinkSequenceStorm(t *testing.T) {
	// 10K [x](y) links — walk efficiently.
	var b strings.Builder
	for i := 0; i < 10_000; i++ {
		b.WriteString("[x](y)")
	}
	_ = DetectSigils(b.String(), SigilModeStrict)
}

func TestDetectSigils_MaxDepthFenceGuard(t *testing.T) {
	// Push past maxFenceDepth and confirm the scanner still terminates
	// without panic; content after the cap is treated as still-in-fence.
	var b strings.Builder
	for i := 0; i < maxFenceDepth+10; i++ {
		b.WriteString("```\n")
	}
	b.WriteString("see bt-abc\n")
	_ = DetectSigils(b.String(), SigilModeVerb)
}

// TestDetectSigils_LinearScaling asserts body x 10 runs in <= 25 x body's
// runtime. Guards against accidental quadratic behavior.
//
// Methodology: collect 51 paired samples (small, large timed back-to-back in
// each iteration), compute the ratio for each pair, then take the MEDIAN ratio.
// Using median-of-ratios (not ratio-of-medians) self-normalises each sample
// against its own scheduler window, so CPU load spikes that slow both inputs
// proportionally cancel out. Median of 51 discards up to 25 outliers on either
// side. Observed ratios under normal load are 7-13x, so the 25x threshold
// has ~2x headroom above the noise ceiling while remaining far below the ~100x
// signature of a genuine O(n^2) regression.
func TestDetectSigils_LinearScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	build := func(size int) string {
		// Repeated realistic prose: "see bt-abc blocks bd-xyz "
		chunk := "see bt-abc blocks bd-xyz "
		n := size / len(chunk)
		return strings.Repeat(chunk, n)
	}
	small := build(10_000)
	large := build(100_000)

	// Warm up both inputs to populate instruction caches and amortise any
	// first-call overhead before collecting samples.
	for i := 0; i < 3; i++ {
		_ = DetectSigils(small, SigilModeVerb)
		_ = DetectSigils(large, SigilModeVerb)
	}

	// calibrateBatch finds the minimum batch size such that timing the batch
	// reliably takes at least minBatchDur. Requiring 3 consecutive rounds to
	// all exceed the floor avoids accepting a spurious fast tick on Windows
	// (where time.Now() resolution can be ~15ms).
	const minBatchDur = 5 * time.Millisecond
	calibrateBatch := func(fn func()) int {
		for b := 1; b <= 1_000_000; b *= 2 {
			passed := 0
			for round := 0; round < 3; round++ {
				t0 := time.Now()
				for k := 0; k < b; k++ {
					fn()
				}
				if time.Since(t0) >= minBatchDur {
					passed++
				}
			}
			if passed == 3 {
				return b
			}
		}
		return 1_000_000
	}

	smallBatch := calibrateBatch(func() { _ = DetectSigils(small, SigilModeVerb) })
	largeBatch := calibrateBatch(func() { _ = DetectSigils(large, SigilModeVerb) })

	// Collect 51 paired (small, large) samples. Timing small then large in
	// each iteration keeps both measurements in the same scheduler window.
	// Ratio is computed per-pair so CPU-load spikes that slow both inputs
	// proportionally cancel out.
	const numSamples = 51
	ratios := make([]float64, numSamples)
	var lastSmall, lastLarge time.Duration
	for i := 0; i < numSamples; i++ {
		t0 := time.Now()
		for k := 0; k < smallBatch; k++ {
			_ = DetectSigils(small, SigilModeVerb)
		}
		smallDur := time.Since(t0) / time.Duration(smallBatch)

		t1 := time.Now()
		for k := 0; k < largeBatch; k++ {
			_ = DetectSigils(large, SigilModeVerb)
		}
		largeDur := time.Since(t1) / time.Duration(largeBatch)

		if smallDur == 0 {
			// Timer resolution too coarse for this batch; treat as max ratio
			// to avoid division by zero but don't penalise the test on this.
			ratios[i] = 0
		} else {
			ratios[i] = float64(largeDur) / float64(smallDur)
		}
		lastSmall, lastLarge = smallDur, largeDur
	}

	// Sort ratios and take the median (index 25 of 51).
	sort.Slice(ratios, func(a, b int) bool { return ratios[a] < ratios[b] })
	medianRatio := ratios[numSamples/2]

	if medianRatio == 0 {
		t.Fatalf("medianRatio is 0: smallBatch=%d produced zero-duration samples; timer resolution too coarse",
			smallBatch)
	}

	// Allow 25x slack. Observed median ratios under normal load are 7-13x,
	// so 25x is ~2x above the noise ceiling. A genuine O(n^2) regression
	// produces ~100x and would still fail clearly.
	const threshold = 25.0
	if medianRatio > threshold {
		t.Errorf("scaling regression: median ratio=%.2f (> %.0fx); last pair small=%v large=%v",
			medianRatio, threshold, lastSmall, lastLarge)
	}
	t.Logf("scaling: median-ratio=%.2f (samples=%d smallBatch=%d largeBatch=%d)",
		medianRatio, numSamples, smallBatch, largeBatch)
}

// TestDetectSigils_NoPanic_AdversarialInputs combines the pathological
// cases into one sweep to catch interaction bugs.
func TestDetectSigils_NoPanic_AdversarialInputs(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on adversarial input: %v", r)
		}
	}()
	adversarial := []string{
		"",
		"\x00\x00\x00",
		"\xff\xfe\xfd",                            // invalid UTF-8
		"\xed\xa0\x80" + strings.Repeat("x", 100), // lone surrogate
		strings.Repeat("`", 10_000),               // all backticks, no newlines
		strings.Repeat("[x", 10_000),              // unclosed brackets
		strings.Repeat("](y)", 10_000),            // unmatched closers
		strings.Repeat("~", 10_000),               // many tildes, no newlines
		strings.Repeat("a*", 10_000),              // format-char storm
		"```" + strings.Repeat("x", 100_000),      // unclosed fence + huge prose
	}
	for _, body := range adversarial {
		for _, mode := range []SigilMode{SigilModeStrict, SigilModeVerb, SigilModePermissive} {
			_ = DetectSigils(body, mode)
		}
	}
}

// Benchmark_DetectSigils_10KB: must be under 500µs per mode.
func Benchmark_DetectSigils_10KB(b *testing.B) {
	body := strings.Repeat("see bt-abc blocks bd-xyz ", 10_000/25)
	for _, mode := range []struct {
		name string
		m    SigilMode
	}{
		{"strict", SigilModeStrict},
		{"verb", SigilModeVerb},
		{"permissive", SigilModePermissive},
	} {
		b.Run(mode.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = DetectSigils(body, mode.m)
			}
		})
	}
}

// Benchmark_DetectSigils_100KB: must be under 5ms per mode.
func Benchmark_DetectSigils_100KB(b *testing.B) {
	body := strings.Repeat("see bt-abc blocks bd-xyz ", 100_000/25)
	for _, mode := range []struct {
		name string
		m    SigilMode
	}{
		{"strict", SigilModeStrict},
		{"verb", SigilModeVerb},
		{"permissive", SigilModePermissive},
	} {
		b.Run(mode.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = DetectSigils(body, mode.m)
			}
		})
	}
}

func Benchmark_DetectSigils_PathologicalNestedFences(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 10_000; i++ {
		if i%2 == 0 {
			sb.WriteString("~~~\n")
		} else {
			sb.WriteString("```\n")
		}
	}
	body := sb.String()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = DetectSigils(body, SigilModeVerb)
	}
}

func Benchmark_DetectSigils_PathologicalInlineCodeStorm(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 100_000; i++ {
		sb.WriteByte('`')
		sb.WriteByte('a')
	}
	body := sb.String()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = DetectSigils(body, SigilModeVerb)
	}
}
