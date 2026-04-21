package main

import (
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func TestFilterBySource_Empty(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-a"}, {ID: "cass-b"},
	}
	got := filterBySource(issues, "")
	if len(got) != 2 {
		t.Errorf("empty filter should be no-op; got %d", len(got))
	}
}

func TestFilterBySource_SinglePrefix(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-a"}, {ID: "cass-b"}, {ID: "bd-c"},
	}
	got := filterBySource(issues, "bt")
	if len(got) != 1 || got[0].ID != "bt-a" {
		t.Errorf("single prefix: got %+v, want [bt-a]", got)
	}
}

func TestFilterBySource_CommaSeparated(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-a"}, {ID: "cass-b"}, {ID: "bd-c"},
	}
	got := filterBySource(issues, "bt,cass")
	if len(got) != 2 {
		t.Errorf("want 2 matches; got %d", len(got))
	}
}

func TestFilterBySource_UnknownPrefix_EmptyResult(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-a"}, {ID: "cass-b"},
	}
	got := filterBySource(issues, "nope")
	if len(got) != 0 {
		t.Errorf("unknown prefix should yield empty slice; got %+v", got)
	}
}

func TestFilterBySource_SourceRepoFallback(t *testing.T) {
	// An issue with no conventional ID prefix but SourceRepo set still
	// matches when --source names that repo.
	issues := []model.Issue{
		{ID: "orphan-id", SourceRepo: "cass"},
	}
	got := filterBySource(issues, "cass")
	if len(got) != 1 {
		t.Errorf("SourceRepo fallback should match; got %+v", got)
	}
}

func TestFilterBySource_CaseInsensitive(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-a"}, {ID: "CASS-B", SourceRepo: "CASS"},
	}
	got := filterBySource(issues, "CASS,BT")
	if len(got) != 2 {
		t.Errorf("case-insensitive match failed; got %+v", got)
	}
}

func TestFilterBySource_TrimsWhitespace(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-a"}, {ID: "cass-b"},
	}
	got := filterBySource(issues, " bt , cass ")
	if len(got) != 2 {
		t.Errorf("whitespace trimming failed; got %+v", got)
	}
}

func TestFilterBySource_EmptyTokensDropped(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-a"}, {ID: "cass-b"},
	}
	// Only empty tokens — should behave like empty filter (match everything).
	// splitSourceFilter returns an empty set which filterBySource interprets
	// as "nothing to match", returning empty. Verify behavior.
	got := filterBySource(issues, ",,,")
	if len(got) != 2 {
		t.Errorf("all-empty tokens should be no-op; got %+v", got)
	}
}
