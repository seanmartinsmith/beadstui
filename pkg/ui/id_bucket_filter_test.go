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
