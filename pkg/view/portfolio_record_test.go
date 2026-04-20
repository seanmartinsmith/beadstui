package view

import (
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func TestPortfolioRecordSchemaConstant(t *testing.T) {
	if PortfolioRecordSchemaV1 != "portfolio.v1" {
		t.Errorf("PortfolioRecordSchemaV1 = %q, want portfolio.v1", PortfolioRecordSchemaV1)
	}
}

func TestComputePortfolioRecord_EmptyProject(t *testing.T) {
	now := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	rec := ComputePortfolioRecord("empty", nil, nil, nil, now)

	if rec.Project != "empty" {
		t.Errorf("Project = %q, want 'empty'", rec.Project)
	}
	if rec.Counts != (PortfolioCounts{}) {
		t.Errorf("empty project counts = %+v, want zero", rec.Counts)
	}
	if rec.Priority != (PortfolioPriority{}) {
		t.Errorf("empty project priority = %+v, want zero", rec.Priority)
	}
	if rec.TopBlocker != nil {
		t.Errorf("empty project TopBlocker = %+v, want nil", rec.TopBlocker)
	}
	if rec.Stalest != nil {
		t.Errorf("empty project Stalest = %+v, want nil", rec.Stalest)
	}
	// Health on an empty project is all-ones because the denominators collapse
	// to the unitary case: closure_ratio defaults to 1 when no closures and no
	// open; blocker_ratio is 0 when no open; stale_norm is 0 when no stalest.
	if rec.HealthScore != 1.0 {
		t.Errorf("empty project HealthScore = %v, want 1.0", rec.HealthScore)
	}
}

func TestComputePortfolioRecord_Counts(t *testing.T) {
	now := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	issues := []model.Issue{
		{ID: "p-1", Title: "open P0", Status: model.StatusOpen, Priority: 0, UpdatedAt: now.Add(-48 * time.Hour)},
		{ID: "p-2", Title: "open P1", Status: model.StatusOpen, Priority: 1, UpdatedAt: now.Add(-48 * time.Hour)},
		{ID: "p-3", Title: "open P2", Status: model.StatusOpen, Priority: 2, UpdatedAt: now.Add(-48 * time.Hour)},
		{ID: "p-4", Title: "in_progress", Status: model.StatusInProgress, Priority: 1, UpdatedAt: now.Add(-48 * time.Hour)},
		{ID: "p-5", Title: "closed recent", Status: model.StatusClosed, Priority: 2,
			UpdatedAt: now.Add(-24 * time.Hour),
			ClosedAt:  ptrTime(now.Add(-24 * time.Hour))},
	}
	// p-2 blocked by open p-1.
	issues[1].Dependencies = []*model.Dependency{
		{IssueID: "p-2", DependsOnID: "p-1", Type: model.DepBlocks},
	}

	rec := ComputePortfolioRecord("p", issues, issues, nil, now)

	if rec.Counts.Open != 3 {
		t.Errorf("Open = %d, want 3", rec.Counts.Open)
	}
	if rec.Counts.InProgress != 1 {
		t.Errorf("InProgress = %d, want 1", rec.Counts.InProgress)
	}
	if rec.Counts.Blocked != 1 {
		t.Errorf("Blocked = %d, want 1", rec.Counts.Blocked)
	}
	if rec.Counts.Closed30d != 1 {
		t.Errorf("Closed30d = %d, want 1", rec.Counts.Closed30d)
	}
	if rec.Priority.P0 != 1 {
		t.Errorf("P0 = %d, want 1", rec.Priority.P0)
	}
	if rec.Priority.P1 != 1 {
		t.Errorf("P1 = %d, want 1 (in_progress P1 must NOT count)", rec.Priority.P1)
	}
}

func TestClassifyTrend(t *testing.T) {
	cases := []struct {
		name   string
		weekly []int // newest first, length 8
		want   string
	}{
		{"up: recent 2x prior", []int{3, 3, 1, 1, 1, 1, 0, 0}, "up"},
		{"down: recent 0 vs prior 8", []int{0, 0, 2, 2, 2, 2, 0, 0}, "down"},
		{"flat: equal", []int{2, 2, 2, 2, 2, 2, 0, 0}, "flat"},
		{"flat: zero prior, some recent", []int{3, 0, 0, 0, 0, 0, 0, 0}, "flat"},
		{"flat: all zeros", []int{0, 0, 0, 0, 0, 0, 0, 0}, "flat"},
		{"boundary exactly +20%", []int{6, 0, 4, 4, 0, 2, 0, 0}, "up"},  // recent=6, prior=(4+4+0+2)/2=5, delta=0.2 → up
		{"boundary exactly -20%", []int{4, 0, 4, 4, 0, 2, 0, 0}, "down"}, // recent=4, prior=5, delta=-0.2 → down
		{"too short defaults flat", []int{5, 0}, "flat"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			weekly := make([]analysis.VelocityWeek, len(tc.weekly))
			for i, v := range tc.weekly {
				weekly[i] = analysis.VelocityWeek{Closed: v}
			}
			got := classifyTrend(weekly)
			if got != tc.want {
				t.Errorf("classifyTrend(%v) = %q, want %q", tc.weekly, got, tc.want)
			}
		})
	}
}

func TestComputeHealthScore(t *testing.T) {
	cases := []struct {
		name    string
		counts  PortfolioCounts
		stalest *PortfolioStaleRef
		want    float64
	}{
		{"empty is 1.0", PortfolioCounts{}, nil, 1.0},
		{"closed-only gives full closure_ratio", PortfolioCounts{Closed30d: 10}, nil, 1.0},
		{"all-blocked tanks blocker_ratio", PortfolioCounts{Open: 4, Blocked: 4}, nil, 0.333},
		{"stale 180+ days caps stale_norm", PortfolioCounts{Open: 1, Closed30d: 1}, &PortfolioStaleRef{AgeDays: 365}, 0.5},
		{"clamps to [0,1]", PortfolioCounts{Open: 10, Blocked: 10}, &PortfolioStaleRef{AgeDays: 200}, 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeHealthScore(tc.counts, tc.stalest)
			if diff := got - tc.want; diff < -0.001 || diff > 0.001 {
				t.Errorf("computeHealthScore(%+v, %+v) = %v, want %v", tc.counts, tc.stalest, got, tc.want)
			}
		})
	}
}

func TestComputePortfolioRecord_TopBlockerExcludesIsolatedLeaves(t *testing.T) {
	now := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	issues := []model.Issue{
		// High PageRank but nobody depends on it — must be excluded.
		{ID: "p-leaf", Title: "High rank leaf", Status: model.StatusOpen, Priority: 0, UpdatedAt: now},
		// Lower PageRank but blocks p-leaf → the real top blocker.
		{ID: "p-hub", Title: "Modest hub", Status: model.StatusOpen, Priority: 1, UpdatedAt: now},
	}
	issues[0].Dependencies = []*model.Dependency{
		{IssueID: "p-leaf", DependsOnID: "p-hub", Type: model.DepBlocks},
	}

	pagerank := map[string]float64{"p-leaf": 0.9, "p-hub": 0.1}
	rec := ComputePortfolioRecord("p", issues, issues, pagerank, now)

	if rec.TopBlocker == nil {
		t.Fatalf("TopBlocker is nil; expected p-hub")
	}
	if rec.TopBlocker.ID != "p-hub" {
		t.Errorf("TopBlocker.ID = %q, want p-hub (p-leaf must be filtered by unblocks>0)", rec.TopBlocker.ID)
	}
}

func TestComputePortfolioRecord_Stalest(t *testing.T) {
	now := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	issues := []model.Issue{
		{ID: "p-recent", Status: model.StatusOpen, UpdatedAt: now.Add(-5 * 24 * time.Hour)},
		{ID: "p-old", Status: model.StatusOpen, UpdatedAt: now.Add(-50 * 24 * time.Hour)},
		{ID: "p-oldest-closed", Status: model.StatusClosed, UpdatedAt: now.Add(-120 * 24 * time.Hour)},
	}
	rec := ComputePortfolioRecord("p", issues, issues, nil, now)

	if rec.Stalest == nil {
		t.Fatalf("Stalest is nil")
	}
	if rec.Stalest.ID != "p-old" {
		t.Errorf("Stalest.ID = %q, want p-old (closed issues must be excluded)", rec.Stalest.ID)
	}
	if rec.Stalest.AgeDays != 50 {
		t.Errorf("Stalest.AgeDays = %d, want 50", rec.Stalest.AgeDays)
	}
}

func TestComputePortfolioRecord_StalestReturnsNilWhenNoOpen(t *testing.T) {
	now := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	issues := []model.Issue{
		{ID: "p-closed", Status: model.StatusClosed, UpdatedAt: now.Add(-5 * 24 * time.Hour)},
	}
	rec := ComputePortfolioRecord("p", issues, issues, nil, now)
	if rec.Stalest != nil {
		t.Errorf("Stalest = %+v, want nil when no open issues", rec.Stalest)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
