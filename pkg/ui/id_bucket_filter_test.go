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
	f := multiTokenFilter(inner)
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
	f := multiTokenFilter(idPriorityFilter(stubRanker))
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
	f := multiTokenFilter(idPriorityFilter(stubRanker))
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
	f := multiTokenFilter(inner)
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
	f := multiTokenFilter(inner)
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
	f := multiTokenFilter(inner)
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
	f := multiTokenFilter(inner)
	ranks := f("zzz, qqq", targets)
	if len(ranks) != 0 {
		t.Fatalf("expected no matches, got %+v", ranks)
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
