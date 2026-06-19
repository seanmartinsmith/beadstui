package ui

import (
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func TestEpicsOverviewRows(t *testing.T) {
	now := time.Now()
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	pc := func(child, parent string) []*model.Dependency {
		return []*model.Dependency{{IssueID: child, DependsOnID: parent, Type: model.DepParentChild}}
	}
	all := []model.Issue{
		{ID: "ep1", IssueType: model.TypeEpic, Status: model.StatusOpen},
		{ID: "ep1.a", Status: model.StatusClosed, Dependencies: pc("ep1.a", "ep1")},
		{ID: "ep1.b", Status: model.StatusInProgress, UpdatedAt: ago(5 * 24 * time.Hour), Dependencies: pc("ep1.b", "ep1")},
		{ID: "ep1.c", Status: model.StatusOpen, Dependencies: pc("ep1.c", "ep1")},
		{ID: "ep2", IssueType: model.TypeEpic, Status: model.StatusClosed}, // all children done
		{ID: "ep2.a", Status: model.StatusClosed, Dependencies: pc("ep2.a", "ep2")},
	}
	rows := epicsOverviewRows(all, EpicsActive, now)
	if len(rows) != 1 || rows[0].Epic.ID != "ep1" {
		t.Fatalf("EpicsActive want [ep1], got %v", rows)
	}
	r := rows[0]
	if r.Done != 1 || r.Total != 3 {
		t.Errorf("progress = %d/%d, want 1/3", r.Done, r.Total)
	}
	if r.AtRisk != 1 {
		t.Errorf("at-risk = %d, want 1 (ep1.b stale)", r.AtRisk)
	}
	if all2 := epicsOverviewRows(all, EpicsAll, now); len(all2) != 2 {
		t.Errorf("EpicsAll want 2 epics, got %d", len(all2))
	}
	if comp := epicsOverviewRows(all, EpicsCompleted, now); len(comp) != 1 || comp[0].Epic.ID != "ep2" {
		t.Errorf("EpicsCompleted want [ep2], got %v", comp)
	}
}
