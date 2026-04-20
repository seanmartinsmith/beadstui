package view

import (
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// mkIssue is a tiny helper for assembling issues with the few fields the
// pair projection actually reads. Unlisted fields stay zero.
func mkIssue(id, title string, status model.Status, priority int, source string, createdAt time.Time) model.Issue {
	return model.Issue{
		ID:         id,
		Title:      title,
		Status:     status,
		Priority:   priority,
		SourceRepo: source,
		CreatedAt:  createdAt,
	}
}

var (
	tBase   = time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	tLater  = time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)
	tLatest = time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
)

func TestPairRecord_SchemaConstant(t *testing.T) {
	if PairRecordSchemaV1 != "pair.v1" {
		t.Errorf("PairRecordSchemaV1 = %q, want pair.v1", PairRecordSchemaV1)
	}
}

func TestComputePairRecords_Empty(t *testing.T) {
	if got := ComputePairRecords(nil); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
	if got := ComputePairRecords([]model.Issue{}); got != nil {
		t.Errorf("empty input: got %v, want nil", got)
	}
	// No pairs present → nil, not empty slice.
	input := []model.Issue{
		mkIssue("bt-a", "t", model.StatusOpen, 1, "bt", tBase),
		mkIssue("bd-b", "t", model.StatusOpen, 1, "bd", tBase),
	}
	if got := ComputePairRecords(input); got != nil {
		t.Errorf("distinct suffixes: got %v, want nil", got)
	}
}

func TestComputePairRecords_SinglePair_InSync(t *testing.T) {
	input := []model.Issue{
		mkIssue("bt-zsy8", "Shared title", model.StatusOpen, 1, "bt", tBase),
		mkIssue("bd-zsy8", "Shared title", model.StatusOpen, 1, "bd", tLater),
	}
	got := ComputePairRecords(input)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	rec := got[0]
	if rec.Suffix != "zsy8" {
		t.Errorf("suffix = %q, want zsy8", rec.Suffix)
	}
	if rec.Canonical.ID != "bt-zsy8" {
		t.Errorf("canonical.id = %q, want bt-zsy8 (earliest CreatedAt)", rec.Canonical.ID)
	}
	if len(rec.Mirrors) != 1 {
		t.Fatalf("len(mirrors) = %d, want 1", len(rec.Mirrors))
	}
	if rec.Mirrors[0].ID != "bd-zsy8" {
		t.Errorf("mirrors[0].id = %q, want bd-zsy8", rec.Mirrors[0].ID)
	}
	if len(rec.Drift) != 0 {
		t.Errorf("drift = %v, want empty (in-sync pair)", rec.Drift)
	}
}

func TestComputePairRecords_SinglePair_DriftAllDimensions(t *testing.T) {
	t.Run("status", func(t *testing.T) {
		input := []model.Issue{
			mkIssue("bt-x", "same title", model.StatusOpen, 1, "bt", tBase),
			mkIssue("bd-x", "same title", model.StatusInProgress, 1, "bd", tLater),
		}
		got := ComputePairRecords(input)
		if len(got) != 1 {
			t.Fatalf("len(got) = %d", len(got))
		}
		if !hasFlag(got[0].Drift, "status") {
			t.Errorf("drift missing status: %v", got[0].Drift)
		}
		if hasFlag(got[0].Drift, "closed_open") {
			t.Errorf("drift should not include closed_open for open↔in_progress: %v", got[0].Drift)
		}
	})
	t.Run("priority", func(t *testing.T) {
		input := []model.Issue{
			mkIssue("bt-x", "same title", model.StatusOpen, 0, "bt", tBase),
			mkIssue("bd-x", "same title", model.StatusOpen, 2, "bd", tLater),
		}
		got := ComputePairRecords(input)
		if !hasFlag(got[0].Drift, "priority") {
			t.Errorf("drift missing priority: %v", got[0].Drift)
		}
	})
	t.Run("closed_open", func(t *testing.T) {
		input := []model.Issue{
			mkIssue("bt-x", "same title", model.StatusOpen, 1, "bt", tBase),
			mkIssue("bd-x", "same title", model.StatusClosed, 1, "bd", tLater),
		}
		got := ComputePairRecords(input)
		if !hasFlag(got[0].Drift, "status") {
			t.Errorf("drift missing status for open↔closed: %v", got[0].Drift)
		}
		if !hasFlag(got[0].Drift, "closed_open") {
			t.Errorf("drift missing closed_open: %v", got[0].Drift)
		}
	})
	t.Run("title", func(t *testing.T) {
		input := []model.Issue{
			mkIssue("bt-x", "original title", model.StatusOpen, 1, "bt", tBase),
			mkIssue("bd-x", "drifted title", model.StatusOpen, 1, "bd", tLater),
		}
		got := ComputePairRecords(input)
		if !hasFlag(got[0].Drift, "title") {
			t.Errorf("drift missing title: %v", got[0].Drift)
		}
	})
}

func TestComputePairRecords_DriftFlagOrder(t *testing.T) {
	// All four dimensions drift; assert fixed output order.
	input := []model.Issue{
		mkIssue("bt-x", "original", model.StatusOpen, 0, "bt", tBase),
		mkIssue("bd-x", "different", model.StatusClosed, 2, "bd", tLater),
	}
	got := ComputePairRecords(input)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d", len(got))
	}
	want := []string{"status", "priority", "closed_open", "title"}
	if !equalStrings(got[0].Drift, want) {
		t.Errorf("drift order = %v, want %v", got[0].Drift, want)
	}
}

func TestComputePairRecords_ThreeWay(t *testing.T) {
	input := []model.Issue{
		mkIssue("bt-zsy8", "Shared", model.StatusOpen, 1, "bt", tLater),
		mkIssue("bd-zsy8", "Shared", model.StatusOpen, 1, "bd", tBase), // canonical (earliest)
		mkIssue("cass-zsy8", "Shared", model.StatusOpen, 1, "cass", tLatest),
	}
	got := ComputePairRecords(input)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d", len(got))
	}
	rec := got[0]
	if rec.Canonical.ID != "bd-zsy8" {
		t.Errorf("canonical = %q, want bd-zsy8 (earliest CreatedAt)", rec.Canonical.ID)
	}
	if len(rec.Mirrors) != 2 {
		t.Fatalf("len(mirrors) = %d, want 2", len(rec.Mirrors))
	}
	// Mirrors sort: prefix ascending among the non-canonical set.
	// bt < cass so bt-zsy8 comes first.
	if rec.Mirrors[0].ID != "bt-zsy8" {
		t.Errorf("mirrors[0] = %q, want bt-zsy8", rec.Mirrors[0].ID)
	}
	if rec.Mirrors[1].ID != "cass-zsy8" {
		t.Errorf("mirrors[1] = %q, want cass-zsy8", rec.Mirrors[1].ID)
	}
}

func TestComputePairRecords_CanonicalTieBreak(t *testing.T) {
	// Identical CreatedAt — prefix ascending wins.
	input := []model.Issue{
		mkIssue("cass-x", "t", model.StatusOpen, 1, "cass", tBase),
		mkIssue("bd-x", "t", model.StatusOpen, 1, "bd", tBase),
		mkIssue("bt-x", "t", model.StatusOpen, 1, "bt", tBase),
	}
	got := ComputePairRecords(input)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d", len(got))
	}
	if got[0].Canonical.ID != "bd-x" {
		t.Errorf("canonical = %q, want bd-x (prefix tie-break)", got[0].Canonical.ID)
	}
	if got[0].Mirrors[0].ID != "bt-x" || got[0].Mirrors[1].ID != "cass-x" {
		t.Errorf("mirror order = %v, want [bt-x, cass-x]",
			[]string{got[0].Mirrors[0].ID, got[0].Mirrors[1].ID})
	}
}

func TestComputePairRecords_SamePrefixDropped(t *testing.T) {
	// Two bt-x rows from different source_repos = single-prefix bucket → drop.
	input := []model.Issue{
		mkIssue("bt-x", "t", model.StatusOpen, 1, "bt", tBase),
		mkIssue("bt-x", "t", model.StatusOpen, 1, "bt-fork", tLater),
	}
	if got := ComputePairRecords(input); got != nil {
		t.Errorf("same-prefix bucket should be dropped; got %v", got)
	}
}

func TestComputePairRecords_MalformedIDsSkipped(t *testing.T) {
	input := []model.Issue{
		mkIssue("noHyphen", "t", model.StatusOpen, 1, "bt", tBase),
		mkIssue("-leading", "t", model.StatusOpen, 1, "bd", tBase),
		mkIssue("trailing-", "t", model.StatusOpen, 1, "bt", tBase),
		// One valid pair to confirm the valid input still emits.
		mkIssue("bt-ok", "t", model.StatusOpen, 1, "bt", tBase),
		mkIssue("bd-ok", "t", model.StatusOpen, 1, "bd", tLater),
	}
	got := ComputePairRecords(input)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (malformed IDs silently skipped)", len(got))
	}
	if got[0].Suffix != "ok" {
		t.Errorf("suffix = %q, want ok", got[0].Suffix)
	}
}

func TestComputePairRecords_DottedSuffix(t *testing.T) {
	input := []model.Issue{
		mkIssue("bt-mhwy.2", "t", model.StatusOpen, 1, "bt", tBase),
		mkIssue("bd-mhwy.2", "t", model.StatusOpen, 1, "bd", tLater),
	}
	got := ComputePairRecords(input)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Suffix != "mhwy.2" {
		t.Errorf("suffix = %q, want mhwy.2", got[0].Suffix)
	}
}

func TestComputePairRecords_RecordsSortedBySuffix(t *testing.T) {
	input := []model.Issue{
		mkIssue("bt-zsy8", "t", model.StatusOpen, 1, "bt", tBase),
		mkIssue("bd-zsy8", "t", model.StatusOpen, 1, "bd", tBase),
		mkIssue("bt-abc", "t", model.StatusOpen, 1, "bt", tBase),
		mkIssue("bd-abc", "t", model.StatusOpen, 1, "bd", tBase),
	}
	got := ComputePairRecords(input)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Suffix != "abc" || got[1].Suffix != "zsy8" {
		t.Errorf("record order = [%q, %q], want [abc, zsy8]", got[0].Suffix, got[1].Suffix)
	}
}

// hasFlag reports whether drift contains the named flag.
func hasFlag(drift []string, name string) bool {
	for _, f := range drift {
		if f == name {
			return true
		}
	}
	return false
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
