package main

import (
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestCalculateBurndownAt_OnTrackWithProgress(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 4) // inclusive = 5 days
	now := start.AddDate(0, 0, 1) // day 2

	closedAt := start.Add(12 * time.Hour)
	issues := []model.Issue{
		{ID: "A", Title: "Done", Status: model.StatusClosed, Priority: 1, IssueType: model.TypeTask, ClosedAt: &closedAt},
		{ID: "B", Title: "Remaining", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask},
	}

	sprint := &model.Sprint{
		ID:        "sprint-1",
		Name:      "Sprint 1",
		StartDate: start,
		EndDate:   end,
		BeadIDs:   []string{"A", "B"},
	}

	out := calculateBurndownAt(sprint, issues, now)

	if out.TotalDays != 5 {
		t.Fatalf("TotalDays=%d; want 5", out.TotalDays)
	}
	if out.ElapsedDays != 2 {
		t.Fatalf("ElapsedDays=%d; want 2", out.ElapsedDays)
	}
	if out.RemainingDays != 3 {
		t.Fatalf("RemainingDays=%d; want 3", out.RemainingDays)
	}

	if out.TotalIssues != 2 || out.CompletedIssues != 1 || out.RemainingIssues != 1 {
		t.Fatalf("issues totals mismatch: total=%d completed=%d remaining=%d", out.TotalIssues, out.CompletedIssues, out.RemainingIssues)
	}

	if out.ProjectedComplete == nil {
		t.Fatalf("ProjectedComplete is nil; want non-nil")
	}
	wantProjected := now.AddDate(0, 0, 3) // see calculateBurndownAt: int(daysToComplete)+1
	if !out.ProjectedComplete.Equal(wantProjected) {
		t.Fatalf("ProjectedComplete=%s; want %s", out.ProjectedComplete.UTC().Format(time.RFC3339), wantProjected.Format(time.RFC3339))
	}
	if !out.OnTrack {
		t.Fatalf("OnTrack=false; want true")
	}

	if got, want := len(out.DailyPoints), out.ElapsedDays; got != want {
		t.Fatalf("DailyPoints=%d; want %d", got, want)
	}
	if got, want := len(out.IdealLine), out.TotalDays+1; got != want {
		t.Fatalf("IdealLine=%d; want %d", got, want)
	}
}

func TestCalculateBurndownAt_NoProgressSetsOnTrackFalse(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 4)
	now := start.AddDate(0, 0, 2) // day 3

	issues := []model.Issue{
		{ID: "A", Title: "Open 1", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask},
		{ID: "B", Title: "Open 2", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask},
	}

	sprint := &model.Sprint{
		ID:        "sprint-1",
		Name:      "Sprint 1",
		StartDate: start,
		EndDate:   end,
		BeadIDs:   []string{"A", "B"},
	}

	out := calculateBurndownAt(sprint, issues, now)

	if out.ElapsedDays <= 0 {
		t.Fatalf("ElapsedDays=%d; want >0", out.ElapsedDays)
	}
	if out.CompletedIssues != 0 {
		t.Fatalf("CompletedIssues=%d; want 0", out.CompletedIssues)
	}
	if out.ProjectedComplete != nil {
		t.Fatalf("ProjectedComplete=%v; want nil", out.ProjectedComplete)
	}
	if out.OnTrack {
		t.Fatalf("OnTrack=true; want false")
	}
}
