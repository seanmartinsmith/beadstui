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

// depFn builds a cross-prefix dep edge. Dep type doesn't matter to v2 — any
// declared relationship counts — so tests default to DepRelated for clarity.
func depFn(fromID, toID string) *model.Dependency {
	return &model.Dependency{
		IssueID:     fromID,
		DependsOnID: toID,
		Type:        model.DepRelated,
		CreatedAt:   tBase,
	}
}

// withDeps returns a copy of src with Dependencies replaced.
func withDeps(src model.Issue, deps ...*model.Dependency) model.Issue {
	out := src
	out.Dependencies = deps
	return out
}

func TestPairRecordV2_SchemaConstant(t *testing.T) {
	if PairRecordSchemaV2 != "pair.v2" {
		t.Errorf("PairRecordSchemaV2 = %q, want pair.v2", PairRecordSchemaV2)
	}
	if PairIntentSourceDep != "dep" {
		t.Errorf("PairIntentSourceDep = %q, want dep", PairIntentSourceDep)
	}
}

// TestComputePairRecordsV2_Empty — no issues, no pairs, no cross-prefix
// suffix matches: all return nil.
func TestComputePairRecordsV2_Empty(t *testing.T) {
	if got := ComputePairRecordsV2(nil); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
	if got := ComputePairRecordsV2([]model.Issue{}); got != nil {
		t.Errorf("empty input: got %v, want nil", got)
	}
	// Suffix matches but no dep edge → v2 drops the pair.
	input := []model.Issue{
		mkIssue("bt-zsy8", "t", model.StatusOpen, 1, "bt", tBase),
		mkIssue("bd-zsy8", "t", model.StatusOpen, 1, "bd", tLater),
	}
	if got := ComputePairRecordsV2(input); got != nil {
		t.Errorf("suffix match alone should not surface in v2: got %v", got)
	}
}

// TestComputePairRecordsV2_BasicCrossPrefixDep — one dep edge, one record.
// Intent source = "dep"; no title drift even when titles differ.
func TestComputePairRecordsV2_BasicCrossPrefixDep(t *testing.T) {
	a := mkIssue("bt-zsy8", "original title", model.StatusOpen, 1, "bt", tBase)
	b := mkIssue("bd-zsy8", "drifted title", model.StatusOpen, 1, "bd", tLater)
	a = withDeps(a, depFn("bt-zsy8", "bd-zsy8"))
	got := ComputePairRecordsV2([]model.Issue{a, b})
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	rec := got[0]
	if rec.Suffix != "zsy8" {
		t.Errorf("suffix = %q, want zsy8", rec.Suffix)
	}
	if rec.Canonical.ID != "bt-zsy8" {
		t.Errorf("canonical = %q, want bt-zsy8", rec.Canonical.ID)
	}
	if rec.IntentSource != "dep" {
		t.Errorf("intent_source = %q, want dep", rec.IntentSource)
	}
	// v2 drops title drift: titles differ but no drift flag should emit.
	if hasFlag(rec.Drift, "title") {
		t.Errorf("v2 must not emit title drift; got %v", rec.Drift)
	}
}

// TestComputePairRecordsV2_BidirectionalDep — dep from bt→bd AND bd→bt
// collapses to a single record (BFS treats the undirected edge as one).
func TestComputePairRecordsV2_BidirectionalDep(t *testing.T) {
	a := mkIssue("bt-zsy8", "t", model.StatusOpen, 1, "bt", tBase)
	b := mkIssue("bd-zsy8", "t", model.StatusOpen, 1, "bd", tLater)
	a = withDeps(a, depFn("bt-zsy8", "bd-zsy8"))
	b = withDeps(b, depFn("bd-zsy8", "bt-zsy8"))
	got := ComputePairRecordsV2([]model.Issue{a, b})
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (bidirectional collapses)", len(got))
	}
}

// TestComputePairRecordsV2_ThreeWayCycle — 3 members connected in a cycle
// (bt→bd, bd→cass, cass→bt) emit a single record.
func TestComputePairRecordsV2_ThreeWayCycle(t *testing.T) {
	a := mkIssue("bt-x", "t", model.StatusOpen, 1, "bt", tBase)
	b := mkIssue("bd-x", "t", model.StatusOpen, 1, "bd", tLater)
	c := mkIssue("cass-x", "t", model.StatusOpen, 1, "cass", tLatest)
	a = withDeps(a, depFn("bt-x", "bd-x"))
	b = withDeps(b, depFn("bd-x", "cass-x"))
	c = withDeps(c, depFn("cass-x", "bt-x"))
	got := ComputePairRecordsV2([]model.Issue{a, b, c})
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (cycle = one component)", len(got))
	}
	if len(got[0].Mirrors) != 2 {
		t.Errorf("mirrors = %d, want 2", len(got[0].Mirrors))
	}
}

// TestComputePairRecordsV2_ThreeWayConnectedPartialEdges — a-b and b-c
// connected (no direct a-c edge) still form one component: bt-bd-cass.
func TestComputePairRecordsV2_ThreeWayConnectedPartialEdges(t *testing.T) {
	a := mkIssue("bt-x", "t", model.StatusOpen, 1, "bt", tBase)
	b := mkIssue("bd-x", "t", model.StatusOpen, 1, "bd", tLater)
	c := mkIssue("cass-x", "t", model.StatusOpen, 1, "cass", tLatest)
	// Path a-b-c via b. No direct a-c edge.
	a = withDeps(a, depFn("bt-x", "bd-x"))
	b = withDeps(b, depFn("bd-x", "cass-x"))
	got := ComputePairRecordsV2([]model.Issue{a, b, c})
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (path-connected = one component)", len(got))
	}
	if len(got[0].Mirrors) != 2 {
		t.Errorf("mirrors = %d, want 2", len(got[0].Mirrors))
	}
}

// TestComputePairRecordsV2_ThreeWayDisconnected — same suffix bucket but
// only a-b connected (c isolated within the bucket). Emits one record for
// the a-b pair; c drops out as a singleton component.
func TestComputePairRecordsV2_ThreeWayDisconnected(t *testing.T) {
	a := mkIssue("bt-x", "t", model.StatusOpen, 1, "bt", tBase)
	b := mkIssue("bd-x", "t", model.StatusOpen, 1, "bd", tLater)
	c := mkIssue("cass-x", "t", model.StatusOpen, 1, "cass", tLatest)
	a = withDeps(a, depFn("bt-x", "bd-x"))
	// c has no deps into the bucket — singleton.
	got := ComputePairRecordsV2([]model.Issue{a, b, c})
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (only a-b is a pair)", len(got))
	}
	if len(got[0].Mirrors) != 1 {
		t.Errorf("mirrors = %d, want 1 (c is isolated)", len(got[0].Mirrors))
	}
	for _, m := range append([]PairMember{got[0].Canonical}, got[0].Mirrors...) {
		if m.ID == "cass-x" {
			t.Errorf("cass-x should not appear; got member %q", m.ID)
		}
	}
}

// TestComputePairRecordsV2_TwoComponentsSameSuffix — 4 members of suffix
// "x" split into two disjoint pairs (bt-bd linked; cass-tpane linked).
// Emits two records with the same suffix, sorted by canonical ID.
func TestComputePairRecordsV2_TwoComponentsSameSuffix(t *testing.T) {
	a := mkIssue("bt-x", "t", model.StatusOpen, 1, "bt", tBase)
	b := mkIssue("bd-x", "t", model.StatusOpen, 1, "bd", tLater)
	c := mkIssue("cass-x", "t", model.StatusOpen, 1, "cass", tLatest)
	d := mkIssue("tpane-x", "t", model.StatusOpen, 1, "tpane", tLatest)
	a = withDeps(a, depFn("bt-x", "bd-x"))
	c = withDeps(c, depFn("cass-x", "tpane-x"))
	got := ComputePairRecordsV2([]model.Issue{a, b, c, d})
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (two disjoint pairs in same bucket)", len(got))
	}
	// Component 1 (bt-bd): canonical = bt-x (earliest CreatedAt = tBase).
	// Component 2 (cass-tpane): both at tLatest → prefix tie-break → cass-x.
	// Records sort by (suffix, canonical ID): bt-x < cass-x.
	if got[0].Canonical.ID != "bt-x" {
		t.Errorf("records[0].canonical = %q, want bt-x", got[0].Canonical.ID)
	}
	if got[1].Canonical.ID != "cass-x" {
		t.Errorf("records[1].canonical = %q, want cass-x", got[1].Canonical.ID)
	}
}

// TestComputePairRecordsV2_DanglingDep — dep targets an ID that doesn't
// resolve; the dep is silently ignored and the member stays isolated.
func TestComputePairRecordsV2_DanglingDep(t *testing.T) {
	a := mkIssue("bt-zsy8", "t", model.StatusOpen, 1, "bt", tBase)
	b := mkIssue("bd-zsy8", "t", model.StatusOpen, 1, "bd", tLater)
	// Dep points at a nonexistent id — doesn't create an edge.
	a = withDeps(a, depFn("bt-zsy8", "ghost-zsy8"))
	if got := ComputePairRecordsV2([]model.Issue{a, b}); got != nil {
		t.Errorf("dangling dep must not surface pair; got %v", got)
	}
}

// TestComputePairRecordsV2_SamePrefixDepIgnored — dep between two issues
// with the same prefix is not cross-prefix intent and doesn't create an
// edge. Suffix bucket with only same-prefix members drops out.
func TestComputePairRecordsV2_SamePrefixDepIgnored(t *testing.T) {
	// Two bt issues in the same bucket + one bd issue with no deps.
	a := mkIssue("bt-x", "t", model.StatusOpen, 1, "bt", tBase)
	a2 := mkIssue("bt-x", "t", model.StatusOpen, 1, "bt-fork", tLater)
	b := mkIssue("bd-x", "t", model.StatusOpen, 1, "bd", tLater)
	a = withDeps(a, depFn("bt-x", "bt-x"))
	// No cross-prefix edges → no records.
	if got := ComputePairRecordsV2([]model.Issue{a, a2, b}); got != nil {
		t.Errorf("same-prefix deps must not surface pair; got %v", got)
	}
}

// TestComputePairRecordsV2_OutOfBucketDepIgnored — dep between a pair member
// and an issue in a different suffix bucket doesn't create an in-bucket
// edge. Members remain unpaired.
func TestComputePairRecordsV2_OutOfBucketDepIgnored(t *testing.T) {
	a := mkIssue("bt-zsy8", "t", model.StatusOpen, 1, "bt", tBase)
	b := mkIssue("bd-zsy8", "t", model.StatusOpen, 1, "bd", tLater)
	other := mkIssue("bd-abc", "t", model.StatusOpen, 1, "bd", tBase)
	// Cross-prefix dep but across different suffix buckets.
	a = withDeps(a, depFn("bt-zsy8", "bd-abc"))
	if got := ComputePairRecordsV2([]model.Issue{a, b, other}); got != nil {
		t.Errorf("out-of-bucket dep must not surface pair; got %v", got)
	}
}

// TestComputePairRecordsV2_DriftStatusPriorityClosedOpen — v2 keeps the
// three surviving drift dimensions and emits them in fixed order. Title
// drift never appears even if titles differ.
func TestComputePairRecordsV2_DriftStatusPriorityClosedOpen(t *testing.T) {
	a := mkIssue("bt-x", "original", model.StatusOpen, 0, "bt", tBase)
	b := mkIssue("bd-x", "totally different", model.StatusClosed, 2, "bd", tLater)
	a = withDeps(a, depFn("bt-x", "bd-x"))
	got := ComputePairRecordsV2([]model.Issue{a, b})
	if len(got) != 1 {
		t.Fatalf("len(got) = %d", len(got))
	}
	want := []string{"status", "priority", "closed_open"}
	if !equalStrings(got[0].Drift, want) {
		t.Errorf("drift = %v, want %v (title must be absent)", got[0].Drift, want)
	}
}

// TestComputePairRecordsV2_IntentSourceOnEveryRecord — every emitted
// record carries intent_source="dep". Drops-through-JSON check ensures
// serialization doesn't strip the field.
func TestComputePairRecordsV2_IntentSourceOnEveryRecord(t *testing.T) {
	a := mkIssue("bt-x", "t", model.StatusOpen, 1, "bt", tBase)
	b := mkIssue("bd-x", "t", model.StatusOpen, 1, "bd", tLater)
	c := mkIssue("cass-y", "t", model.StatusOpen, 1, "cass", tBase)
	d := mkIssue("bt-y", "t", model.StatusOpen, 1, "bt", tLater)
	a = withDeps(a, depFn("bt-x", "bd-x"))
	c = withDeps(c, depFn("cass-y", "bt-y"))
	got := ComputePairRecordsV2([]model.Issue{a, b, c, d})
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	for i, rec := range got {
		if rec.IntentSource != "dep" {
			t.Errorf("records[%d].intent_source = %q, want dep", i, rec.IntentSource)
		}
	}
}

// TestComputePairRecordsV2_SortedBySuffix — deterministic output ordering
// across runs, mirroring v1.
func TestComputePairRecordsV2_SortedBySuffix(t *testing.T) {
	aZ := mkIssue("bt-zsy8", "t", model.StatusOpen, 1, "bt", tBase)
	bZ := mkIssue("bd-zsy8", "t", model.StatusOpen, 1, "bd", tBase)
	aA := mkIssue("bt-abc", "t", model.StatusOpen, 1, "bt", tBase)
	bA := mkIssue("bd-abc", "t", model.StatusOpen, 1, "bd", tBase)
	aZ = withDeps(aZ, depFn("bt-zsy8", "bd-zsy8"))
	aA = withDeps(aA, depFn("bt-abc", "bd-abc"))
	got := ComputePairRecordsV2([]model.Issue{aZ, bZ, aA, bA})
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Suffix != "abc" || got[1].Suffix != "zsy8" {
		t.Errorf("order = [%q, %q], want [abc, zsy8]", got[0].Suffix, got[1].Suffix)
	}
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
