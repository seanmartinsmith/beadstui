package search

import "testing"

func TestShortQueryLexicalBoost(t *testing.T) {
	doc := "Performance benchmarks for graph rendering"
	if boost := ShortQueryLexicalBoost("benchmarks", doc); boost <= 0 {
		t.Fatalf("expected boost for literal short query match")
	}
	if boost := ShortQueryLexicalBoost("long descriptive query about rendering performance", doc); boost != 0 {
		t.Fatalf("expected no boost for long query")
	}
}

func TestApplyShortQueryLexicalBoostResorts(t *testing.T) {
	results := []SearchResult{
		{IssueID: "a", Score: 0.2},
		{IssueID: "b", Score: 0.5},
	}
	docs := map[string]string{
		"a": "benchmarks",
		"b": "unrelated",
	}
	updated := ApplyShortQueryLexicalBoost(results, "benchmarks", docs)
	if updated[0].IssueID != "a" {
		t.Fatalf("expected boosted match to rank first, got %s", updated[0].IssueID)
	}
}
