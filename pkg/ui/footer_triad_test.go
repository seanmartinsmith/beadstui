package ui

import (
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// triadFixtureIssues builds a small, hand-verifiable corpus exercising every
// bt-p8y2f triad rule in one shot:
//   - B1, R1: open, no deps — actionable, not in-flight -> Ready.
//   - E1: open, directly blocked by B1 (open) via a blocking dep -> Blocked.
//   - E1.1: open, parent-child dep on E1 (NOT a blocking-type dep on its own)
//     but E1 is blocked, so transitive parent-child propagation must mark
//     E1.1 blocked too. A naive "only walk this issue's own blocking deps"
//     check (the footer's prior classifyItemCounts) would wrongly call this
//     Ready — this is the concrete regression the analyzer-based rewrite
//     fixes (see computeFooterTriad's doc).
//   - IP1: in_progress, no deps -> InFlight (would also be graph-actionable,
//     but in-flight takes priority so it is never double-counted as Ready).
//   - IPBlocked: in_progress AND directly blocked by B1 -> still InFlight
//     (status wins over graph state for this bucket).
//   - C1: closed -> excluded from all three buckets.
func triadFixtureIssues() []model.Issue {
	return []model.Issue{
		{ID: "B1", Title: "Blocker", Status: model.StatusOpen},
		{ID: "R1", Title: "Ready, no deps", Status: model.StatusOpen},
		{ID: "E1", Title: "Epic blocked by B1", Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{IssueID: "E1", DependsOnID: "B1", Type: model.DepBlocks},
			}},
		{ID: "E1.1", Title: "Child of blocked epic", Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{IssueID: "E1.1", DependsOnID: "E1", Type: model.DepParentChild},
			}},
		{ID: "IP1", Title: "In progress, unblocked", Status: model.StatusInProgress},
		{ID: "IPBlocked", Title: "In progress but graph-blocked", Status: model.StatusInProgress,
			Dependencies: []*model.Dependency{
				{IssueID: "IPBlocked", DependsOnID: "B1", Type: model.DepBlocks},
			}},
		{ID: "C1", Title: "Closed", Status: model.StatusClosed},
	}
}

// TestComputeFooterTriad_Partition locks the exact bucket assignment for the
// fixture above, including the transitive parent-child propagation case.
func TestComputeFooterTriad_Partition(t *testing.T) {
	issues := triadFixtureIssues()
	analyzer := analysis.NewAnalyzer(issues)

	got := computeFooterTriad(issues, analyzer)
	want := FooterTriad{Ready: 2, InFlight: 2, Blocked: 2}
	if got != want {
		t.Fatalf("computeFooterTriad = %+v, want %+v", got, want)
	}
}

// TestComputeFooterTriad_MatchesAnalysisLayer is the bt-p8y2f acceptance
// spot-check: the triad's Ready/Blocked split must agree with pkg/analysis's
// own actionable/blocked classification (the same one `bt robot triage`
// reports via ComputeTriage's ProjectHealth.Counts), once in-flight issues —
// which the triage HealthCounts does NOT carve out as their own bucket — are
// reconciled back in. Concretely: every Ready issue must be in the
// analyzer's actionable set, and every actionable issue that is NOT
// in-progress must be Ready (no silent drops or extra members).
func TestComputeFooterTriad_MatchesAnalysisLayer(t *testing.T) {
	issues := triadFixtureIssues()
	analyzer := analysis.NewAnalyzer(issues)

	triad := computeFooterTriad(issues, analyzer)

	actionable := analyzer.GetActionableIssues()
	actionableSet := make(map[string]bool, len(actionable))
	for _, iss := range actionable {
		actionableSet[iss.ID] = true
	}

	// The triage-class HealthCounts (what `bt robot triage` reports via
	// ComputeTriage's ProjectHealth.Counts) draws Actionable/Blocked from the
	// SAME analyzer.GetActionableIssues() call, over every non-closed issue
	// regardless of in-progress status. Actionable = 3 (B1, R1, IP1); the
	// remaining 3 non-closed issues (E1, E1.1, IPBlocked) are Blocked.
	health := analysis.ComputeTriage(issues).ProjectHealth.Counts
	const wantActionable, wantBlocked = 3, 3
	if health.Actionable != wantActionable || len(actionable) != wantActionable {
		t.Fatalf("triage-class actionable = %d (HealthCounts) / %d (analyzer set), want %d",
			health.Actionable, len(actionable), wantActionable)
	}
	if health.Blocked != wantBlocked {
		t.Fatalf("triage-class HealthCounts.Blocked = %d, want %d", health.Blocked, wantBlocked)
	}

	// Reconciling the triad against that split: every Ready issue must be
	// analyzer-actionable, and every non-in-flight issue must land in
	// EXACTLY the bucket its actionability says (Ready xor Blocked) — no
	// issue silently drops out of both.
	gotReady, gotBlocked := 0, 0
	for _, issue := range issues {
		if isClosedLikeStatus(issue.Status) || issue.Status == model.StatusInProgress {
			continue
		}
		if actionableSet[issue.ID] {
			gotReady++
		} else {
			gotBlocked++
		}
	}
	if gotReady != triad.Ready {
		t.Errorf("hand-tallied ready (non-in-flight, analyzer-actionable) = %d, computeFooterTriad.Ready = %d", gotReady, triad.Ready)
	}
	if gotBlocked != triad.Blocked {
		t.Errorf("hand-tallied blocked (non-in-flight, not analyzer-actionable) = %d, computeFooterTriad.Blocked = %d", gotBlocked, triad.Blocked)
	}
	// gotReady + gotBlocked covers every non-closed, non-in-flight issue
	// (Actionable=3 total minus IP1's in-flight carve-out = 2 ready; Blocked=3
	// total minus IPBlocked's in-flight carve-out = 2 blocked).
	if gotReady != wantActionable-1 || gotBlocked != wantBlocked-1 {
		t.Fatalf("in-flight carve-out mismatch: gotReady=%d gotBlocked=%d", gotReady, gotBlocked)
	}
}

// TestComputeFooterTriad_EmptyCorpus guards the nil/zero-length degenerate
// cases: no issues, and a nil analyzer (no data loaded yet).
func TestComputeFooterTriad_EmptyCorpus(t *testing.T) {
	if got := computeFooterTriad(nil, analysis.NewAnalyzer(nil)); got != (FooterTriad{}) {
		t.Errorf("empty corpus: got %+v, want zero value", got)
	}
	issues := triadFixtureIssues()
	if got := computeFooterTriad(issues, nil); got.InFlight != 2 {
		t.Errorf("nil analyzer: InFlight should still derive from status alone, got %d want 2", got.InFlight)
	}
}
