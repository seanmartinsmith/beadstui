package view

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// TestCompactAllNilEmpty — safety for nil and empty input.
func TestCompactAllNilEmpty(t *testing.T) {
	if got := CompactAll(nil); got != nil {
		t.Errorf("CompactAll(nil) = %v, want nil", got)
	}
	if got := CompactAll([]model.Issue{}); got != nil {
		t.Errorf("CompactAll([]) = %v, want nil", got)
	}
}

// TestCompactAllFieldsCopy verifies the field-copying mechanics for one
// fully populated issue.
func TestCompactAllFieldsCopy(t *testing.T) {
	created := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	due := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	issue := model.Issue{
		ID:          "bt-a",
		Title:       "Alpha",
		Description: "should not appear in compact output",
		Design:      "also excluded",
		Status:      model.StatusOpen,
		Priority:    1,
		IssueType:   model.TypeFeature,
		Labels:      []string{"area:cli"},
		Assignee:    "sms",
		SourceRepo:  "bt",
		CreatedAt:   created,
		UpdatedAt:   updated,
		DueDate:     &due,
		CreatedBySession: "sess-create",
		ClaimedBySession: "sess-claim",
		ClosedBySession:  "sess-close",
	}

	got := CompactAll([]model.Issue{issue})
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	c := got[0]

	if c.ID != "bt-a" || c.Title != "Alpha" || c.Status != model.StatusOpen ||
		c.Priority != 1 || c.IssueType != model.TypeFeature {
		t.Errorf("core fields mismatch: %+v", c)
	}
	if c.Assignee != "sms" || c.SourceRepo != "bt" {
		t.Errorf("provenance fields mismatch: assignee=%q source_repo=%q", c.Assignee, c.SourceRepo)
	}
	if len(c.Labels) != 1 || c.Labels[0] != "area:cli" {
		t.Errorf("labels mismatch: %v", c.Labels)
	}
	if !c.CreatedAt.Equal(created) || !c.UpdatedAt.Equal(updated) {
		t.Errorf("timestamps mismatch: created=%v updated=%v", c.CreatedAt, c.UpdatedAt)
	}
	if c.DueDate == nil || !c.DueDate.Equal(due) {
		t.Errorf("due_date mismatch: %v", c.DueDate)
	}
	if c.CreatedBySession != "sess-create" || c.ClaimedBySession != "sess-claim" ||
		c.ClosedBySession != "sess-close" {
		t.Errorf("session fields mismatch: %+v", c)
	}

	// The projection struct has no description/design/notes fields at all;
	// round-trip to JSON and assert those keys are absent.
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var asMap map[string]interface{}
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"description", "design", "acceptance_criteria", "notes", "comments", "close_reason"} {
		if _, has := asMap[k]; has {
			t.Errorf("compact output should not contain key %q", k)
		}
	}
}

// TestCompactAllLabelsAliasing — the compact copy must not share the
// underlying Labels slice with the source issue.
func TestCompactAllLabelsAliasing(t *testing.T) {
	issue := model.Issue{
		ID:     "bt-a",
		Status: model.StatusOpen,
		Labels: []string{"area:cli", "area:tui"},
	}
	got := CompactAll([]model.Issue{issue})
	if len(got) != 1 {
		t.Fatalf("expected one projection")
	}
	got[0].Labels[0] = "mutated"
	if issue.Labels[0] == "mutated" {
		t.Error("CompactIssue.Labels aliases the source issue's labels slice")
	}
}

// TestCompactAllReverseMaps — children_count and unblocks_count come from
// reverse edges.
func TestCompactAllReverseMaps(t *testing.T) {
	epic := model.Issue{ID: "epic-1", Title: "Epic", Status: model.StatusOpen, IssueType: model.TypeEpic}
	child1 := model.Issue{
		ID:     "child-1",
		Status: model.StatusOpen,
		Dependencies: []*model.Dependency{
			{DependsOnID: "epic-1", Type: model.DepParentChild},
		},
	}
	child2 := model.Issue{
		ID:     "child-2",
		Status: model.StatusOpen,
		Dependencies: []*model.Dependency{
			{DependsOnID: "epic-1", Type: model.DepParentChild},
		},
	}
	blocker := model.Issue{ID: "blocker-1", Status: model.StatusOpen}
	dependent := model.Issue{
		ID:     "dep-1",
		Status: model.StatusOpen,
		Dependencies: []*model.Dependency{
			{DependsOnID: "blocker-1", Type: model.DepBlocks},
		},
	}

	got := CompactAll([]model.Issue{epic, child1, child2, blocker, dependent})

	byID := map[string]CompactIssue{}
	for _, c := range got {
		byID[c.ID] = c
	}

	if c := byID["epic-1"]; c.ChildrenCount != 2 {
		t.Errorf("epic-1.children_count = %d, want 2", c.ChildrenCount)
	}
	if c := byID["child-1"]; c.ParentID != "epic-1" {
		t.Errorf("child-1.parent_id = %q, want epic-1", c.ParentID)
	}
	if c := byID["blocker-1"]; c.UnblocksCount != 1 {
		t.Errorf("blocker-1.unblocks_count = %d, want 1", c.UnblocksCount)
	}
	if c := byID["dep-1"]; c.BlockersCount != 1 {
		t.Errorf("dep-1.blockers_count = %d, want 1", c.BlockersCount)
	}
}

// TestCompactAllIsBlockedSemantics — a closed blocker does not block; an
// open one does.
func TestCompactAllIsBlockedSemantics(t *testing.T) {
	// Blocker is closed: target is not blocked.
	closedBlocker := model.Issue{ID: "cb", Status: model.StatusClosed}
	dep1 := model.Issue{
		ID:     "dep-closed",
		Status: model.StatusOpen,
		Dependencies: []*model.Dependency{
			{DependsOnID: "cb", Type: model.DepBlocks},
		},
	}

	// Blocker is open: target is blocked.
	openBlocker := model.Issue{ID: "ob", Status: model.StatusOpen}
	dep2 := model.Issue{
		ID:     "dep-open",
		Status: model.StatusOpen,
		Dependencies: []*model.Dependency{
			{DependsOnID: "ob", Type: model.DepBlocks},
		},
	}

	// Blocker is in_progress: target is still blocked.
	inProgressBlocker := model.Issue{ID: "ip", Status: model.StatusInProgress}
	dep3 := model.Issue{
		ID:     "dep-in-progress",
		Status: model.StatusOpen,
		Dependencies: []*model.Dependency{
			{DependsOnID: "ip", Type: model.DepBlocks},
		},
	}

	// Unknown blocker (external): conservatively treated as blocking.
	depExternal := model.Issue{
		ID:     "dep-external",
		Status: model.StatusOpen,
		Dependencies: []*model.Dependency{
			{DependsOnID: "external:unknown", Type: model.DepBlocks},
		},
	}

	got := CompactAll([]model.Issue{closedBlocker, dep1, openBlocker, dep2, inProgressBlocker, dep3, depExternal})
	byID := map[string]CompactIssue{}
	for _, c := range got {
		byID[c.ID] = c
	}

	cases := []struct {
		id   string
		want bool
	}{
		{"dep-closed", false},
		{"dep-open", true},
		{"dep-in-progress", true},
		{"dep-external", true},
	}
	for _, tc := range cases {
		if got := byID[tc.id].IsBlocked; got != tc.want {
			t.Errorf("%s.is_blocked = %v, want %v", tc.id, got, tc.want)
		}
	}
}

// TestCompactAllRelatesCount — relates is a local count only (no reverse).
func TestCompactAllRelatesCount(t *testing.T) {
	a := model.Issue{
		ID:     "a",
		Status: model.StatusOpen,
		Dependencies: []*model.Dependency{
			{DependsOnID: "b", Type: model.DepRelated},
			{DependsOnID: "c", Type: model.DepRelated},
		},
	}
	b := model.Issue{ID: "b", Status: model.StatusOpen}
	c := model.Issue{ID: "c", Status: model.StatusOpen}

	got := CompactAll([]model.Issue{a, b, c})
	byID := map[string]CompactIssue{}
	for _, ci := range got {
		byID[ci.ID] = ci
	}
	if byID["a"].RelatesCount != 2 {
		t.Errorf("a.relates_count = %d, want 2", byID["a"].RelatesCount)
	}
	if byID["b"].RelatesCount != 0 {
		t.Errorf("b.relates_count = %d, want 0 (relates is local-only)", byID["b"].RelatesCount)
	}
}

// TestCompactAllSessionFieldsDirect — session IDs are passed through from
// the direct columns on model.Issue (bt-5hl9; bd-34v Phase 1a/1b).
func TestCompactAllSessionFieldsDirect(t *testing.T) {
	issue := model.Issue{
		ID:               "x",
		Status:           model.StatusOpen,
		CreatedBySession: "cc-create",
		ClaimedBySession: "cc-claim",
		ClosedBySession:  "cc-close",
	}
	got := CompactAll([]model.Issue{issue})[0]
	if got.CreatedBySession != "cc-create" {
		t.Errorf("created_by_session = %q, want cc-create", got.CreatedBySession)
	}
	if got.ClaimedBySession != "cc-claim" {
		t.Errorf("claimed_by_session = %q, want cc-claim", got.ClaimedBySession)
	}
	if got.ClosedBySession != "cc-close" {
		t.Errorf("closed_by_session = %q, want cc-close", got.ClosedBySession)
	}

	// Empty session fields are omitted from JSON output (omitempty).
	empty := model.Issue{ID: "y", Status: model.StatusOpen}
	gotEmpty := CompactAll([]model.Issue{empty})[0]
	if gotEmpty.CreatedBySession != "" || gotEmpty.ClaimedBySession != "" || gotEmpty.ClosedBySession != "" {
		t.Errorf("empty source should produce empty session fields, got %+v", gotEmpty)
	}
}

// TestCompactAllSchemaConstant — schema version is the v1 contract.
func TestCompactAllSchemaConstant(t *testing.T) {
	if CompactIssueSchemaV1 != "compact.v1" {
		t.Errorf("CompactIssueSchemaV1 = %q, want compact.v1", CompactIssueSchemaV1)
	}
}
