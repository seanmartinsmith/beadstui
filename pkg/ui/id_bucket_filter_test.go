package ui

import (
	"testing"

	"charm.land/bubbles/v2/list"
)

// stubRanker returns the targets in their input order, so a Rank ordering from
// this baseline is intentional and easy to reason about in assertions.
func stubRanker(_ string, targets []string) []list.Rank {
	out := make([]list.Rank, len(targets))
	for i := range targets {
		out[i] = list.Rank{Index: i}
	}
	return out
}

// TestIDPriorityFilter_ExactSuffixPromotes verifies that a suffix-only query
// (`cmg`) promotes the bead whose ID ends in `-cmg` above other beads that
// merely mention the suffix in body text. Regression for bt-i4yn.
func TestIDPriorityFilter_ExactSuffixPromotes(t *testing.T) {
	targets := []string{
		"dotfiles-3mm body references dotfiles-cmg as a related issue",
		"dotfiles-cmg",
		"dotfiles-x1y2 other bead without the token",
		"dotfiles-abc random bead mentioning cmg in prose",
	}

	f := idPriorityFilter(stubRanker)
	ranks := f("cmg", targets)

	if len(ranks) == 0 || ranks[0].Index != 1 {
		t.Fatalf("expected dotfiles-cmg (index 1) at position 0, got %+v", ranks)
	}
}

// TestIDPriorityFilter_NonIDQueryUntouched verifies that a multi-word text
// query falls through to the inner ranker (no bucket pre-emption).
func TestIDPriorityFilter_NonIDQueryUntouched(t *testing.T) {
	targets := []string{"bt-xyz1 first bead title", "bt-xyz2 second"}
	f := idPriorityFilter(stubRanker)
	ranks := f("pagerank bottleneck", targets)

	if len(ranks) != 2 || ranks[0].Index != 0 || ranks[1].Index != 1 {
		t.Fatalf("expected base order preserved, got %+v", ranks)
	}
}

// TestIDPriorityFilter_FullIDMatch verifies that a fully-qualified ID query
// like `bt-i4yn` lands the matching bead at position 0.
func TestIDPriorityFilter_FullIDMatch(t *testing.T) {
	targets := []string{
		"bt-noise different bead",
		"bt-i4yn exact match",
		"bt-prefix yet another",
	}
	f := idPriorityFilter(stubRanker)
	ranks := f("bt-i4yn", targets)

	if len(ranks) == 0 || ranks[0].Index != 1 {
		t.Fatalf("expected bt-i4yn (index 1) at position 0, got %+v", ranks)
	}
}

// TestIDPriorityFilter_AmbiguousSuffixSurfacesAll verifies global-mode behavior:
// when the suffix matches IDs across multiple projects, all of them land at
// the top of the bucket.
func TestIDPriorityFilter_AmbiguousSuffixSurfacesAll(t *testing.T) {
	targets := []string{
		"bt-96y bt project",
		"dotfiles-other unrelated",
		"cass-96y cass project",
		"unrelated bead text",
	}
	f := idPriorityFilter(stubRanker)
	ranks := f("96y", targets)

	if len(ranks) < 2 {
		t.Fatalf("expected at least 2 ID matches, got %+v", ranks)
	}
	// First two entries must be the two -96y beads in some order.
	topTwo := map[int]bool{ranks[0].Index: true, ranks[1].Index: true}
	if !topTwo[0] || !topTwo[2] {
		t.Fatalf("expected indices 0 and 2 (both -96y beads) in top two, got %+v", ranks)
	}
}

// TestIDPriorityFilter_NoIDMatchUnchanged verifies that when the query shape
// looks like an ID but no target has a matching ID, the base ordering is
// preserved (no empty bucket, no reordering).
func TestIDPriorityFilter_NoIDMatchUnchanged(t *testing.T) {
	targets := []string{
		"bt-xyz1 mentions zzz somewhere in body",
		"bt-xyz2 also mentions zzz in the middle",
	}
	f := idPriorityFilter(stubRanker)
	ranks := f("zzz", targets)

	if len(ranks) != 2 || ranks[0].Index != 0 || ranks[1].Index != 1 {
		t.Fatalf("expected base order preserved when no ID matches, got %+v", ranks)
	}
}

// TestLooksLikeIDQuery verifies the heuristic accepts bead-ID-shaped tokens
// and rejects multi-word or punctuation-heavy queries.
func TestLooksLikeIDQuery(t *testing.T) {
	cases := []struct {
		term string
		want bool
	}{
		{"cmg", true},
		{"bt-i4yn", true},
		{"bt-mhwy.1", true},
		{"x", false},                  // too short
		{"pagerank bottleneck", false}, // whitespace
		{"Bug#123", false},             // punctuation
		{"", false},
	}
	for _, c := range cases {
		if got := looksLikeIDQuery(c.term); got != c.want {
			t.Errorf("looksLikeIDQuery(%q) = %v, want %v", c.term, got, c.want)
		}
	}
}

// TestExtractIDToken verifies the ID is extracted as the first whitespace-
// separated token when the target is in IssueItem.FilterValue() shape.
func TestExtractIDToken(t *testing.T) {
	cases := []struct {
		target string
		want   string
	}{
		{"bt-i4yn some title words", "bt-i4yn"},
		{"bt-mhwy.1 molecule child", "bt-mhwy.1"},
		{"no-hyphenless-token", "no-hyphenless-token"}, // single token with hyphen
		{"plainword no id", ""},                         // first token has no '-'
		{"", ""},
	}
	for _, c := range cases {
		if got := extractIDToken(c.target); got != c.want {
			t.Errorf("extractIDToken(%q) = %q, want %q", c.target, got, c.want)
		}
	}
}

// TestMultiTokenFilter_SingleTokenPassthrough verifies that a term with no
// comma behaves identically to invoking the inner filter directly — no
// regression for the common case (bt-jwo3).
func TestMultiTokenFilter_SingleTokenPassthrough(t *testing.T) {
	targets := []string{"bt-z5jj first", "bt-uahv second", "bt-other third"}
	inner := func(term string, _ []string) []list.Rank {
		if term != "bt-z5jj" {
			t.Fatalf("inner expected term=bt-z5jj, got %q", term)
		}
		return []list.Rank{{Index: 0, MatchedIndexes: []int{0, 1, 2, 3, 4, 5, 6}}}
	}
	f := multiTokenFilter(inner, 0)
	ranks := f("bt-z5jj", targets)
	if len(ranks) != 1 || ranks[0].Index != 0 {
		t.Fatalf("expected single rank for index 0, got %+v", ranks)
	}
}

// TestMultiTokenFilter_TwoIDsUnion verifies the user's primary use case:
// "z5jj, uahv" populates both beads (bt-jwo3).
func TestMultiTokenFilter_TwoIDsUnion(t *testing.T) {
	targets := []string{
		"bt-z5jj sprint decision bead",
		"bt-uahv data layout split bead",
		"bt-other unrelated",
	}
	f := multiTokenFilter(idPriorityFilter(stubRanker), 0)
	ranks := f("z5jj, uahv", targets)

	got := make(map[int]bool)
	for _, r := range ranks {
		got[r.Index] = true
	}
	if !got[0] || !got[1] {
		t.Fatalf("expected both bt-z5jj (0) and bt-uahv (1) in results, got %+v", ranks)
	}
}

// TestMultiTokenFilter_NoWhitespaceAfterComma verifies the parser tolerates
// "z5jj,uahv" (no space) identically to "z5jj, uahv".
func TestMultiTokenFilter_NoWhitespaceAfterComma(t *testing.T) {
	targets := []string{"bt-z5jj a", "bt-uahv b"}
	f := multiTokenFilter(idPriorityFilter(stubRanker), 0)
	ranks := f("z5jj,uahv", targets)
	if len(ranks) < 2 {
		t.Fatalf("expected at least 2 results, got %+v", ranks)
	}
}

// TestMultiTokenFilter_EmptyTokensSkipped verifies trailing commas and double
// commas don't produce empty-string queries that match everything.
func TestMultiTokenFilter_EmptyTokensSkipped(t *testing.T) {
	targets := []string{"bt-z5jj a", "bt-uahv b", "bt-other c"}
	calls := 0
	inner := func(term string, _ []string) []list.Rank {
		calls++
		if term == "" {
			t.Fatalf("inner called with empty term — empty token leaked through")
		}
		return nil
	}
	f := multiTokenFilter(inner, 0)
	_ = f("z5jj,,uahv,", targets)
	if calls != 2 {
		t.Fatalf("expected inner called exactly twice (z5jj, uahv), got %d", calls)
	}
}

// TestMultiTokenFilter_DedupesByIndex verifies that when multiple tokens hit
// the same target, the result has one entry, not two.
func TestMultiTokenFilter_DedupesByIndex(t *testing.T) {
	targets := []string{"bt-z5jj sprint", "bt-uahv layout"}
	inner := func(term string, _ []string) []list.Rank {
		// Both tokens claim to match index 0.
		return []list.Rank{{Index: 0, MatchedIndexes: []int{0}}}
	}
	f := multiTokenFilter(inner, 0)
	ranks := f("foo, bar", targets)
	if len(ranks) != 1 {
		t.Fatalf("expected dedup to 1 rank, got %d: %+v", len(ranks), ranks)
	}
}

// TestMultiTokenFilter_MergesMatchedIndexes verifies that when two tokens
// both match the same target, their MatchedIndexes are unioned so highlight
// rendering covers all matched chars.
func TestMultiTokenFilter_MergesMatchedIndexes(t *testing.T) {
	targets := []string{"bt-z5jj-uahv combined"}
	inner := func(term string, _ []string) []list.Rank {
		switch term {
		case "z5jj":
			return []list.Rank{{Index: 0, MatchedIndexes: []int{3, 4, 5, 6}}}
		case "uahv":
			return []list.Rank{{Index: 0, MatchedIndexes: []int{8, 9, 10, 11}}}
		}
		return nil
	}
	f := multiTokenFilter(inner, 0)
	ranks := f("z5jj, uahv", targets)
	if len(ranks) != 1 {
		t.Fatalf("expected 1 rank, got %d", len(ranks))
	}
	got := ranks[0].MatchedIndexes
	want := []int{3, 4, 5, 6, 8, 9, 10, 11}
	if len(got) != len(want) {
		t.Fatalf("expected merged indexes %v, got %v", want, got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("expected merged[%d]=%d, got %d (full: %v)", i, v, got[i], got)
		}
	}
}

// TestMultiTokenFilter_NoMatch verifies a multi-token query where neither
// token matches anything returns empty (not the full target list).
func TestMultiTokenFilter_NoMatch(t *testing.T) {
	targets := []string{"bt-aaa one", "bt-bbb two"}
	inner := func(_ string, _ []string) []list.Rank { return nil }
	f := multiTokenFilter(inner, 0)
	ranks := f("zzz, qqq", targets)
	if len(ranks) != 0 {
		t.Fatalf("expected no matches, got %+v", ranks)
	}
}

// TestMultiTokenFilter_SingleTokenIgnoresCap verifies that a single-token
// query bypasses the per-token cap entirely — the cap exists to bound
// per-token noise in multi-token unions, and single-token has no union to
// defend against (bt-da4f).
func TestMultiTokenFilter_SingleTokenIgnoresCap(t *testing.T) {
	// Inner returns 50 ranks for any non-empty term.
	targets := make([]string, 50)
	for i := range targets {
		targets[i] = "target"
	}
	inner := func(_ string, _ []string) []list.Rank {
		ranks := make([]list.Rank, 50)
		for i := range ranks {
			ranks[i] = list.Rank{Index: i}
		}
		return ranks
	}
	// Cap of 25 is set; single-token "foo" should still return all 50.
	f := multiTokenFilter(inner, 25)
	ranks := f("foo", targets)
	if len(ranks) != 50 {
		t.Fatalf("single-token query must bypass cap; expected 50, got %d", len(ranks))
	}
}

// TestMultiTokenFilter_MultiTokenAppliesCap verifies that multi-token queries
// cap each per-token result set before union (bt-krwp's noise reduction —
// preserved by bt-da4f).
func TestMultiTokenFilter_MultiTokenAppliesCap(t *testing.T) {
	// Inner returns 50 distinct ranks for each token. Token "a" returns
	// indexes 0-49; token "b" returns indexes 100-149. With cap=25, union
	// should be 25 (from "a") + 25 (from "b") = 50, not 100.
	targets := make([]string, 200)
	for i := range targets {
		targets[i] = "target"
	}
	inner := func(term string, _ []string) []list.Rank {
		base := 0
		if term == "b" {
			base = 100
		}
		ranks := make([]list.Rank, 50)
		for i := range ranks {
			ranks[i] = list.Rank{Index: base + i}
		}
		return ranks
	}
	f := multiTokenFilter(inner, 25)
	ranks := f("a, b", targets)
	if len(ranks) != 50 {
		t.Fatalf("multi-token must cap each token at 25, expected union=50, got %d", len(ranks))
	}
	// First 25 should be from token "a" (indexes 0-24), next 25 from "b" (100-124).
	for i := 0; i < 25; i++ {
		if ranks[i].Index != i {
			t.Fatalf("expected ranks[%d].Index=%d (from token a), got %d", i, i, ranks[i].Index)
		}
	}
	for i := 25; i < 50; i++ {
		want := 100 + (i - 25)
		if ranks[i].Index != want {
			t.Fatalf("expected ranks[%d].Index=%d (from token b), got %d", i, want, ranks[i].Index)
		}
	}
}

// TestMultiTokenFilter_ZeroCapDisables verifies perTokenCap=0 disables capping
// even for multi-token queries (used by fuzzy mode).
func TestMultiTokenFilter_ZeroCapDisables(t *testing.T) {
	targets := make([]string, 200)
	for i := range targets {
		targets[i] = "target"
	}
	inner := func(term string, _ []string) []list.Rank {
		base := 0
		if term == "b" {
			base = 100
		}
		ranks := make([]list.Rank, 50)
		for i := range ranks {
			ranks[i] = list.Rank{Index: base + i}
		}
		return ranks
	}
	f := multiTokenFilter(inner, 0)
	ranks := f("a, b", targets)
	if len(ranks) != 100 {
		t.Fatalf("perTokenCap=0 must disable cap; expected 100, got %d", len(ranks))
	}
}

// TestWhitespaceAndFilter_SingleWordPassthrough verifies a single-word term
// (no whitespace) is passed unchanged to the inner ranker (bt-6pzni Change B).
func TestWhitespaceAndFilter_SingleWordPassthrough(t *testing.T) {
	targets := []string{"bt-abc migration", "bt-xyz release", "bt-qwe other"}
	calls := 0
	var gotTerm string
	inner := func(term string, _ []string) []list.Rank {
		calls++
		gotTerm = term
		return []list.Rank{{Index: 0}, {Index: 1}}
	}
	f := whitespaceAndFilter(inner)
	ranks := f("migration", targets)
	if calls != 1 {
		t.Fatalf("single-word must call inner exactly once, got %d", calls)
	}
	if gotTerm != "migration" {
		t.Fatalf("inner must receive original term %q, got %q", "migration", gotTerm)
	}
	if len(ranks) != 2 {
		t.Fatalf("expected inner result passthrough (2 ranks), got %d", len(ranks))
	}
}

// TestWhitespaceAndFilter_MultiWordIntersection verifies that a two-word query
// returns only items that match BOTH words (bt-6pzni Change B). Items matching
// only one word are excluded.
func TestWhitespaceAndFilter_MultiWordIntersection(t *testing.T) {
	// targets: 0=matches both, 1=matches only "migration", 2=matches only "release", 3=neither
	targets := []string{
		"migration release notes",   // 0: both
		"migration path overview",   // 1: first word only
		"release process doc",       // 2: second word only
		"unrelated documentation",   // 3: neither
	}
	inner := func(term string, _ []string) []list.Rank {
		switch term {
		case "migration":
			return []list.Rank{
				{Index: 0, MatchedIndexes: []int{0, 1, 2, 3, 4, 5, 6, 7, 8}},
				{Index: 1, MatchedIndexes: []int{0, 1, 2, 3, 4, 5, 6, 7, 8}},
			}
		case "release":
			return []list.Rank{
				{Index: 0, MatchedIndexes: []int{10, 11, 12, 13, 14, 15, 16}},
				{Index: 2, MatchedIndexes: []int{0, 1, 2, 3, 4, 5, 6}},
			}
		}
		return nil
	}
	f := whitespaceAndFilter(inner)
	ranks := f("migration release", targets)

	if len(ranks) != 1 {
		t.Fatalf("expected exactly 1 result (index 0 matches both), got %d: %+v", len(ranks), ranks)
	}
	if ranks[0].Index != 0 {
		t.Fatalf("expected result index 0, got %d", ranks[0].Index)
	}
	// MatchedIndexes should be the union of both words' positions.
	wantMinLen := 9 + 7 // len("migration") + len("release") positions
	if len(ranks[0].MatchedIndexes) < wantMinLen {
		t.Fatalf("expected merged MatchedIndexes covering both words (>= %d), got %d: %v",
			wantMinLen, len(ranks[0].MatchedIndexes), ranks[0].MatchedIndexes)
	}
}

// TestWhitespaceAndFilter_ThreeWordIntersection verifies three-word AND
// semantics: all three words must match (bt-6pzni Change B).
func TestWhitespaceAndFilter_ThreeWordIntersection(t *testing.T) {
	targets := []string{
		"migration release notes",   // 0: all three
		"migration release other",   // 1: first two only
		"migration notes only",      // 2: first and third only
		"release notes doc",         // 3: second and third only
	}
	inner := func(term string, _ []string) []list.Rank {
		switch term {
		case "migration":
			return []list.Rank{{Index: 0}, {Index: 1}, {Index: 2}}
		case "release":
			return []list.Rank{{Index: 0}, {Index: 1}, {Index: 3}}
		case "notes":
			return []list.Rank{{Index: 0}, {Index: 2}, {Index: 3}}
		}
		return nil
	}
	f := whitespaceAndFilter(inner)
	ranks := f("migration release notes", targets)

	if len(ranks) != 1 {
		t.Fatalf("expected exactly 1 result (index 0 matches all three), got %d: %+v", len(ranks), ranks)
	}
	if ranks[0].Index != 0 {
		t.Fatalf("expected result index 0, got %d", ranks[0].Index)
	}
}

// TestWhitespaceAndFilter_NoMatch verifies that when no item matches all words,
// the filter returns empty (not the full corpus) (bt-6pzni Change B).
func TestWhitespaceAndFilter_NoMatch(t *testing.T) {
	targets := []string{"bt-aaa one", "bt-bbb two", "bt-ccc three"}
	inner := func(term string, _ []string) []list.Rank {
		switch term {
		case "xqzwvb":
			return nil // nothing matches
		case "foobarbaz":
			return []list.Rank{{Index: 0}}
		}
		return nil
	}
	f := whitespaceAndFilter(inner)
	ranks := f("xqzwvb foobarbaz", targets)
	if len(ranks) != 0 {
		t.Fatalf("expected empty result for nonsense query, got %d: %+v", len(ranks), ranks)
	}
}

// TestWhitespaceAndFilter_QuotedTermPassthrough verifies that a term containing
// double-quotes is passed unchanged to the inner ranker (quotedExactFilter
// sits inside this wrapper and handles quotes itself) (bt-6pzni Change B).
func TestWhitespaceAndFilter_QuotedTermPassthrough(t *testing.T) {
	called := 0
	var gotTerm string
	inner := func(term string, _ []string) []list.Rank {
		called++
		gotTerm = term
		return []list.Rank{{Index: 0}}
	}
	f := whitespaceAndFilter(inner)
	_ = f(`"release notes"`, []string{"release notes doc"})
	if called != 1 {
		t.Fatalf("quoted term must call inner exactly once, got %d", called)
	}
	if gotTerm != `"release notes"` {
		t.Fatalf("quoted term must be passed unchanged, got %q", gotTerm)
	}
}

// TestWhitespaceAndFilter_SpanFloor verifies that per-word matches with an
// excessively large character span are discarded as low-quality fuzzy matches
// (bt-6pzni Change B, FuzzyMatchSpanFactor calibration).
func TestWhitespaceAndFilter_SpanFloor(t *testing.T) {
	// "migration" is 9 chars; FuzzyMatchSpanFactor=4 → max span = 36.
	// index 0: span = 8 (tight, within floor) → kept.
	// index 1: span = 100 (scattered, exceeds floor) → dropped.
	targets := []string{"migration analysis", "this is a very long string where m-i-g-r-a-t-i-o-n chars are scattered"}
	inner := func(term string, _ []string) []list.Rank {
		if term != "migration" {
			return nil
		}
		return []list.Rank{
			{Index: 0, MatchedIndexes: []int{0, 1, 2, 3, 4, 5, 6, 7, 8}}, // span=8
			{Index: 1, MatchedIndexes: []int{0, 10, 20, 30, 40, 50, 60, 70, 100}}, // span=100
		}
	}
	f := whitespaceAndFilter(inner)
	ranks := f("migration", targets)
	// Single word → passes through to inner unchanged. Span floor only
	// applies inside the multi-word path.
	if len(ranks) != 2 {
		t.Fatalf("single-word must bypass span floor (passthrough), expected 2, got %d", len(ranks))
	}
}

// TestWhitespaceAndFilter_SpanFloorMultiWord verifies span floor is applied
// inside the multi-word intersection path.
func TestWhitespaceAndFilter_SpanFloorMultiWord(t *testing.T) {
	// Query: "migration release". "migration" is 9 chars, max span = 36.
	// target 0: migration match span=8 (tight) + release match → kept.
	// target 1: migration match span=100 (exceeds floor) → dropped from migration set.
	// target 2: only matches "release", not "migration" → dropped by AND.
	targets := []string{
		"migration release",
		"migration release scattered",
		"release only",
	}
	inner := func(term string, _ []string) []list.Rank {
		switch term {
		case "migration":
			return []list.Rank{
				{Index: 0, MatchedIndexes: []int{0, 1, 2, 3, 4, 5, 6, 7, 8}},    // span=8, kept
				{Index: 1, MatchedIndexes: []int{0, 10, 20, 30, 40, 50, 60, 70, 100}}, // span=100, dropped
			}
		case "release":
			return []list.Rank{
				{Index: 0, MatchedIndexes: []int{10, 11, 12, 13, 14, 15, 16}},
				{Index: 1, MatchedIndexes: []int{10, 11, 12, 13, 14, 15, 16}},
				{Index: 2, MatchedIndexes: []int{0, 1, 2, 3, 4, 5, 6}},
			}
		}
		return nil
	}
	f := whitespaceAndFilter(inner)
	ranks := f("migration release", targets)
	// Only target 0 survives: passes span floor for "migration" AND matches "release".
	if len(ranks) != 1 || ranks[0].Index != 0 {
		t.Fatalf("expected only index 0 to survive span floor + AND, got %+v", ranks)
	}
}

// TestSplitWhitespaceTokens covers the whitespace tokenizer directly.
func TestSplitWhitespaceTokens(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"single", []string{"single"}},
		{"  padded  ", []string{"padded"}},
		{"a b", []string{"a", "b"}},
		{"a  b", []string{"a", "b"}},
		{"a b c", []string{"a", "b", "c"}},
		{"migration release notes", []string{"migration", "release", "notes"}},
	}
	for _, c := range cases {
		got := splitWhitespaceTokens(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitWhitespaceTokens(%q) = %v (len %d), want %v (len %d)", c.in, got, len(got), c.want, len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitWhitespaceTokens(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

// TestSplitCommaTokens covers parser edge cases directly.
func TestSplitCommaTokens(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"single", []string{"single"}},
		{"  padded  ", []string{"padded"}},
		{"a,b", []string{"a", "b"}},
		{"a, b", []string{"a", "b"}},
		{"a , b", []string{"a", "b"}},
		{"a,,b", []string{"a", "b"}},
		{",a,", []string{"a"}},
		{",,,", nil},
	}
	for _, c := range cases {
		got := splitCommaTokens(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitCommaTokens(%q) = %v (len %d), want %v (len %d)", c.in, got, len(got), c.want, len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitCommaTokens(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}
