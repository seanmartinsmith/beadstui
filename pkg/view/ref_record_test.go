package view

import (
	"reflect"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// mkRefIssue is a terse constructor for ref_record_test fixtures. Only fields
// that ref detection actually reads (ID, Status, Description, Notes,
// Comments, Dependencies) are exposed; other Issue fields stay zero.
func mkRefIssue(id string, status model.Status, desc, notes string) model.Issue {
	return model.Issue{
		ID:          id,
		Title:       id,
		Status:      status,
		Description: desc,
		Notes:       notes,
		CreatedAt:   time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
	}
}

func TestComputeRefRecords_Empty(t *testing.T) {
	if got := ComputeRefRecords(nil); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
	if got := ComputeRefRecords([]model.Issue{}); got != nil {
		t.Errorf("empty input: got %v, want nil", got)
	}
}

func TestComputeRefRecords_SamePrefix_Skipped(t *testing.T) {
	issues := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "see bt-b for details", ""),
		mkRefIssue("bt-b", model.StatusOpen, "", ""),
	}
	got := ComputeRefRecords(issues)
	if got != nil {
		t.Errorf("same-prefix ref should produce no record; got %+v", got)
	}
}

func TestComputeRefRecords_CrossProject_Found(t *testing.T) {
	issues := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "see bd-x for details", ""),
		mkRefIssue("bd-x", model.StatusOpen, "", ""),
	}
	got := ComputeRefRecords(issues)
	if len(got) != 1 {
		t.Fatalf("want 1 record; got %d: %+v", len(got), got)
	}
	if got[0].Source != "bt-a" || got[0].Target != "bd-x" || got[0].Location != "description" {
		t.Errorf("record mismatch: %+v", got[0])
	}
	if !reflect.DeepEqual(got[0].Flags, []string{"cross_project"}) {
		t.Errorf("flags = %v, want [cross_project]", got[0].Flags)
	}
}

func TestComputeRefRecords_Broken(t *testing.T) {
	// bd-anchor establishes "bd" as a known prefix so the bd-missing ref
	// can actually be validated as broken. Without an anchor, prefix
	// scoping filters out all "bd-*" refs.
	issues := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "see bd-missing for context", ""),
		mkRefIssue("bd-anchor", model.StatusOpen, "", ""),
	}
	got := ComputeRefRecords(issues)
	if len(got) != 1 {
		t.Fatalf("want 1 record; got %d: %+v", len(got), got)
	}
	if got[0].Target != "bd-missing" {
		t.Fatalf("target = %q, want bd-missing", got[0].Target)
	}
	if !reflect.DeepEqual(got[0].Flags, []string{"broken", "cross_project"}) {
		t.Errorf("flags = %v, want [broken, cross_project]", got[0].Flags)
	}
}

func TestComputeRefRecords_UnknownPrefix_Skipped(t *testing.T) {
	// "round-trip", "per-issue" etc. look like bead IDs to a naive regex
	// but match no loaded prefix. Prefix scoping drops them.
	issues := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "we need round-trip testing and per-issue handling", ""),
	}
	if got := ComputeRefRecords(issues); got != nil {
		t.Errorf("unknown-prefix matches should not fire; got %+v", got)
	}
}

func TestComputeRefRecords_Stale(t *testing.T) {
	issues := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "depended on bd-x", ""),
		mkRefIssue("bd-x", model.StatusClosed, "", ""),
	}
	got := ComputeRefRecords(issues)
	if len(got) != 1 {
		t.Fatalf("want 1 record; got %d", len(got))
	}
	if !reflect.DeepEqual(got[0].Flags, []string{"stale", "cross_project"}) {
		t.Errorf("flags = %v, want [stale, cross_project]", got[0].Flags)
	}
}

func TestComputeRefRecords_OrphanedChild(t *testing.T) {
	child := mkRefIssue("bd-child", model.StatusOpen, "", "")
	child.Dependencies = []*model.Dependency{
		{IssueID: "bd-child", DependsOnID: "bd-parent", Type: model.DepParentChild},
	}
	issues := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "references bd-child for followup", ""),
		child,
		mkRefIssue("bd-parent", model.StatusClosed, "", ""),
	}
	got := ComputeRefRecords(issues)
	var found *RefRecord
	for i := range got {
		if got[i].Source == "bt-a" && got[i].Target == "bd-child" {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("missing bt-a → bd-child record; got %+v", got)
	}
	if !reflect.DeepEqual(found.Flags, []string{"orphaned_child", "cross_project"}) {
		t.Errorf("flags = %v, want [orphaned_child, cross_project]", found.Flags)
	}
}

func TestComputeRefRecords_ExternalDepResolved(t *testing.T) {
	src := mkRefIssue("bt-a", model.StatusOpen, "", "")
	src.Dependencies = []*model.Dependency{
		{IssueID: "bt-a", DependsOnID: "external:cass:x", Type: model.DepBlocks},
	}
	issues := []model.Issue{src, mkRefIssue("cass-x", model.StatusOpen, "", "")}
	got := ComputeRefRecords(issues)
	for _, rec := range got {
		if rec.Source == "bt-a" && rec.Location == "deps" {
			t.Errorf("resolvable external: dep should produce no record; got %+v", rec)
		}
	}
}

func TestComputeRefRecords_ExternalDepBroken(t *testing.T) {
	src := mkRefIssue("bt-a", model.StatusOpen, "", "")
	src.Dependencies = []*model.Dependency{
		{IssueID: "bt-a", DependsOnID: "external:cass:missing", Type: model.DepBlocks},
	}
	issues := []model.Issue{src}
	got := ComputeRefRecords(issues)
	if len(got) != 1 {
		t.Fatalf("want 1 record; got %+v", got)
	}
	if got[0].Location != "deps" || got[0].Target != "external:cass:missing" {
		t.Errorf("record mismatch: %+v", got[0])
	}
	if !reflect.DeepEqual(got[0].Flags, []string{"broken", "cross_project"}) {
		t.Errorf("flags = %v", got[0].Flags)
	}
}

func TestComputeRefRecords_DedupWithinLocation(t *testing.T) {
	issues := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "see bd-x. also bd-x later. and bd-x again.", ""),
		mkRefIssue("bd-x", model.StatusOpen, "", ""),
	}
	got := ComputeRefRecords(issues)
	if len(got) != 1 {
		t.Errorf("three mentions in one location should dedup to 1; got %d", len(got))
	}
}

func TestComputeRefRecords_MultipleLocations(t *testing.T) {
	issues := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "see bd-x", "also bd-x in notes"),
		mkRefIssue("bd-x", model.StatusOpen, "", ""),
	}
	got := ComputeRefRecords(issues)
	if len(got) != 2 {
		t.Fatalf("want 2 records (description + notes); got %d: %+v", len(got), got)
	}
	locations := map[string]bool{}
	for _, r := range got {
		locations[r.Location] = true
	}
	if !locations["description"] || !locations["notes"] {
		t.Errorf("missing location: %+v", got)
	}
}

func TestComputeRefRecords_MalformedIDsSkipped(t *testing.T) {
	issues := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "noHyphen -leading trailing- plainword", ""),
	}
	if got := ComputeRefRecords(issues); got != nil {
		t.Errorf("malformed IDs should produce no record; got %+v", got)
	}
}

func TestComputeRefRecords_URLsSkipped(t *testing.T) {
	issues := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "see https://github.com/foo-bar/baz for context", ""),
	}
	if got := ComputeRefRecords(issues); got != nil {
		t.Errorf("URL-embedded IDs should not fire; got %+v", got)
	}
}

func TestComputeRefRecords_WordBoundaries(t *testing.T) {
	cases := []struct {
		body    string
		wantHit bool
	}{
		{"see bd-a.", true},
		{"(bd-a)", true},
		{"bd-a,", true},
		{"abt-a", false},  // embedded, not a standalone ref
		{"x-bd-a", false}, // embedded in a longer token
		{"bd-a", true},
	}
	for _, tc := range cases {
		// bd-anchor establishes "bd" as a known prefix.
		issues := []model.Issue{
			mkRefIssue("bt-src", model.StatusOpen, tc.body, ""),
			mkRefIssue("bd-anchor", model.StatusOpen, "", ""),
		}
		got := ComputeRefRecords(issues)
		hit := false
		for _, r := range got {
			if r.Target == "bd-a" {
				hit = true
			}
		}
		if hit != tc.wantHit {
			t.Errorf("body=%q hit=%v want=%v (records=%+v)", tc.body, hit, tc.wantHit, got)
		}
	}
}

func TestComputeRefRecords_DottedSuffix(t *testing.T) {
	issues := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "related to bd-mhwy.2 discussion", ""),
		mkRefIssue("bd-mhwy.2", model.StatusOpen, "", ""),
	}
	got := ComputeRefRecords(issues)
	if len(got) != 1 || got[0].Target != "bd-mhwy.2" {
		t.Errorf("want one bd-mhwy.2 record; got %+v", got)
	}
}

func TestComputeRefRecords_FlagOrder(t *testing.T) {
	// orphaned_child fires when an open child has a closed DepParentChild
	// parent. Closed targets can't be orphaned (they're already closed), so
	// the target is deliberately open.
	parent := mkRefIssue("bd-parent", model.StatusClosed, "", "")
	openTarget := mkRefIssue("bd-target", model.StatusOpen, "", "")
	openTarget.Dependencies = []*model.Dependency{
		{IssueID: "bd-target", DependsOnID: "bd-parent", Type: model.DepParentChild},
	}
	issues := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "see bd-target", ""),
		openTarget,
		parent,
	}
	got := ComputeRefRecords(issues)
	if len(got) != 1 {
		t.Fatalf("want 1 record; got %d: %+v", len(got), got)
	}
	// Open orphaned_child — flags: [orphaned_child, cross_project]
	if !reflect.DeepEqual(got[0].Flags, []string{"orphaned_child", "cross_project"}) {
		t.Errorf("flags = %v, want [orphaned_child, cross_project]", got[0].Flags)
	}

	// Now a broken + cross_project case — verify broken comes first.
	brokenOnly := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "see bd-nothing", ""),
		mkRefIssue("bd-anchor", model.StatusOpen, "", ""),
	}
	got = ComputeRefRecords(brokenOnly)
	if len(got) != 1 {
		t.Fatalf("want 1 broken record; got %d", len(got))
	}
	if got[0].Flags[0] != "broken" || got[0].Flags[len(got[0].Flags)-1] != "cross_project" {
		t.Errorf("broken must lead; cross_project must trail; got %v", got[0].Flags)
	}
}

func TestComputeRefRecords_SortedOutput(t *testing.T) {
	issues := []model.Issue{
		mkRefIssue("bt-a", model.StatusOpen, "see bd-zzz and bd-aaa", ""),
		mkRefIssue("bt-b", model.StatusOpen, "also bd-aaa", ""),
		mkRefIssue("bd-aaa", model.StatusOpen, "", ""),
		mkRefIssue("bd-zzz", model.StatusOpen, "", ""),
	}
	got := ComputeRefRecords(issues)
	for i := 1; i < len(got); i++ {
		prev, cur := got[i-1], got[i]
		if prev.Source > cur.Source {
			t.Errorf("source not sorted at %d: %q > %q", i, prev.Source, cur.Source)
		}
		if prev.Source == cur.Source && prev.Target > cur.Target {
			t.Errorf("target not sorted at %d: %q > %q", i, prev.Target, cur.Target)
		}
		if prev.Source == cur.Source && prev.Target == cur.Target && prev.Location > cur.Location {
			t.Errorf("location not sorted at %d", i)
		}
	}
}

func TestRefRecord_SchemaConstant(t *testing.T) {
	if RefRecordSchemaV1 != "ref.v1" {
		t.Errorf("RefRecordSchemaV1 = %q, want ref.v1", RefRecordSchemaV1)
	}
}
