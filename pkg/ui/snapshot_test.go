package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/recipe"
)

func TestDataSnapshot_Empty(t *testing.T) {
	var s *DataSnapshot
	if !s.IsEmpty() {
		t.Error("nil snapshot should be empty")
	}

	s = &DataSnapshot{}
	if !s.IsEmpty() {
		t.Error("snapshot with no issues should be empty")
	}

	s = &DataSnapshot{Issues: []model.Issue{{ID: "test-1"}}}
	if s.IsEmpty() {
		t.Error("snapshot with issues should not be empty")
	}
}

func TestFreshnessThresholds_FromEnv(t *testing.T) {
	t.Setenv("BV_FRESHNESS_WARN_S", "15")
	t.Setenv("BV_FRESHNESS_STALE_S", "90")

	if got := freshnessWarnThreshold(); got != 15*time.Second {
		t.Errorf("freshnessWarnThreshold()=%v, want %v", got, 15*time.Second)
	}
	if got := freshnessStaleThreshold(); got != 90*time.Second {
		t.Errorf("freshnessStaleThreshold()=%v, want %v", got, 90*time.Second)
	}

	t.Setenv("BV_FRESHNESS_WARN_S", "-1")
	t.Setenv("BV_FRESHNESS_STALE_S", "nope")

	if got := freshnessWarnThreshold(); got != 30*time.Second {
		t.Errorf("freshnessWarnThreshold() invalid env=%v, want %v", got, 30*time.Second)
	}
	if got := freshnessStaleThreshold(); got != 2*time.Minute {
		t.Errorf("freshnessStaleThreshold() invalid env=%v, want %v", got, 2*time.Minute)
	}
}

func TestDataSnapshot_GetIssue(t *testing.T) {
	issue := model.Issue{ID: "test-1", Title: "Test Issue"}
	s := &DataSnapshot{
		Issues:   []model.Issue{issue},
		IssueMap: map[string]*model.Issue{"test-1": &issue},
	}

	got := s.GetIssue("test-1")
	if got == nil {
		t.Fatal("GetIssue returned nil for existing issue")
	}
	if got.Title != "Test Issue" {
		t.Errorf("GetIssue returned wrong issue: got %q, want %q", got.Title, "Test Issue")
	}

	got = s.GetIssue("nonexistent")
	if got != nil {
		t.Error("GetIssue should return nil for nonexistent issue")
	}

	// Test nil snapshot
	var nilS *DataSnapshot
	if nilS.GetIssue("test-1") != nil {
		t.Error("GetIssue on nil snapshot should return nil")
	}
}

func TestDataSnapshot_Age(t *testing.T) {
	now := time.Now()
	s := &DataSnapshot{CreatedAt: now.Add(-5 * time.Second)}

	age := s.Age()
	if age < 4*time.Second || age > 6*time.Second {
		t.Errorf("Age should be ~5s, got %v", age)
	}

	var nilS *DataSnapshot
	if nilS.Age() != 0 {
		t.Error("Age on nil snapshot should return 0")
	}
}

func TestSnapshotBuilder_Simple(t *testing.T) {
	issues := []model.Issue{
		{ID: "test-1", Title: "Issue 1", Status: model.StatusOpen, Priority: 1},
		{ID: "test-2", Title: "Issue 2", Status: model.StatusClosed, Priority: 2},
	}

	builder := NewSnapshotBuilder(issues)
	snapshot := builder.Build()

	if snapshot == nil {
		t.Fatal("Build returned nil snapshot")
	}

	if len(snapshot.Issues) != 2 {
		t.Errorf("Expected 2 issues, got %d", len(snapshot.Issues))
	}

	if snapshot.CountOpen != 1 {
		t.Errorf("Expected 1 open issue, got %d", snapshot.CountOpen)
	}

	if snapshot.CountClosed != 1 {
		t.Errorf("Expected 1 closed issue, got %d", snapshot.CountClosed)
	}

	if snapshot.CountReady != 1 {
		t.Errorf("Expected 1 ready issue, got %d", snapshot.CountReady)
	}

	if snapshot.IssueMap == nil {
		t.Error("IssueMap should not be nil")
	}

	if snapshot.GetIssue("test-1") == nil {
		t.Error("test-1 should be in IssueMap")
	}

	if snapshot.Analysis == nil {
		t.Error("Analysis should not be nil")
	}
	if snapshot.Insights.Stats != snapshot.Analysis {
		t.Error("Insights.Stats should reference Analysis")
	}

	if snapshot.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestSnapshotBuilder_WithDependencies(t *testing.T) {
	issues := []model.Issue{
		{
			ID:     "test-1",
			Title:  "Blocker",
			Status: model.StatusOpen,
		},
		{
			ID:     "test-2",
			Title:  "Blocked",
			Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{DependsOnID: "test-1", Type: model.DepBlocks},
			},
		},
		{
			ID:     "test-3",
			Title:  "Ready",
			Status: model.StatusOpen,
		},
	}

	builder := NewSnapshotBuilder(issues)
	snapshot := builder.Build()

	// test-1 and test-3 are ready (no blockers)
	// test-2 is blocked by test-1
	if snapshot.CountOpen != 3 {
		t.Errorf("Expected 3 open issues, got %d", snapshot.CountOpen)
	}

	// Only test-1 and test-3 should be counted as ready
	if snapshot.CountReady != 2 {
		t.Errorf("Expected 2 ready issues (test-1, test-3), got %d", snapshot.CountReady)
	}
}

func TestSnapshotBuilder_TombstoneCounts(t *testing.T) {
	issues := []model.Issue{
		{ID: "open-1", Title: "Open", Status: model.StatusOpen},
		{ID: "closed-1", Title: "Closed", Status: model.StatusClosed},
		{ID: "tomb-1", Title: "Removed", Status: model.StatusTombstone},
		{
			ID:     "open-2",
			Title:  "Depends on tombstone",
			Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{DependsOnID: "tomb-1", Type: model.DepBlocks},
			},
		},
		{
			ID:     "open-3",
			Title:  "Depends on open",
			Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{DependsOnID: "open-1", Type: model.DepBlocks},
			},
		},
	}

	snapshot := NewSnapshotBuilder(issues).Build()
	if snapshot == nil {
		t.Fatal("Build returned nil snapshot")
	}

	if snapshot.CountOpen != 3 {
		t.Errorf("Expected 3 open issues (tombstone excluded), got %d", snapshot.CountOpen)
	}
	if snapshot.CountClosed != 2 {
		t.Errorf("Expected 2 closed issues (closed+tombstone), got %d", snapshot.CountClosed)
	}
	if snapshot.CountReady != 2 {
		t.Errorf("Expected 2 ready issues (open-1, open-2), got %d", snapshot.CountReady)
	}
}

func TestDatasetTierForIssueCount_Boundaries(t *testing.T) {
	tests := []struct {
		count int
		want  datasetTier
	}{
		{0, datasetTierUnknown},
		{1, datasetTierSmall},
		{999, datasetTierSmall},
		{1000, datasetTierMedium},
		{4999, datasetTierMedium},
		{5000, datasetTierLarge},
		{19999, datasetTierLarge},
		{20000, datasetTierHuge},
	}

	for _, tc := range tests {
		if got := datasetTierForIssueCount(tc.count); got != tc.want {
			t.Errorf("datasetTierForIssueCount(%d)=%v, want %v", tc.count, got, tc.want)
		}
	}
}

func TestSnapshotBuilder_WithBuildConfig_SkipsPrecomputesForLargeTier(t *testing.T) {
	issues := []model.Issue{
		{ID: "test-1", Title: "Issue 1", Status: model.StatusOpen, Priority: 1},
		{ID: "test-2", Title: "Issue 2", Status: model.StatusClosed, Priority: 2},
	}

	snapshot := NewSnapshotBuilder(issues).
		WithBuildConfig(snapshotBuildConfigForTier(datasetTierLarge)).
		Build()
	if snapshot == nil {
		t.Fatal("Build returned nil snapshot")
	}
	if snapshot.Analysis == nil || snapshot.Analyzer == nil {
		t.Fatal("expected analysis/analyzer to be populated")
	}
	if len(snapshot.ListItems) != 2 {
		t.Fatalf("expected 2 list items, got %d", len(snapshot.ListItems))
	}
	if snapshot.TriageScores != nil || snapshot.TriageReasons != nil || snapshot.UnblocksMap != nil {
		t.Fatalf("expected triage precompute to be skipped")
	}
	if snapshot.TreeRoots != nil || snapshot.TreeNodeMap != nil {
		t.Fatalf("expected tree precompute to be skipped")
	}
	if snapshot.BoardState != nil {
		t.Fatalf("expected board precompute to be skipped")
	}
	if snapshot.GraphLayout != nil {
		t.Fatalf("expected graph layout precompute to be skipped")
	}
	if snapshot.Insights.Stats != snapshot.Analysis {
		t.Fatalf("expected Insights.Stats to reference Analysis")
	}
}

func TestSnapshotBuilder_WithAnalysis_PopulatesGraphScores(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "A", Title: "A", Status: model.StatusOpen, CreatedAt: now},
		{
			ID:     "B",
			Title:  "B",
			Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{DependsOnID: "A", Type: model.DepBlocks},
			},
			CreatedAt: now.Add(-time.Hour),
		},
		{
			ID:     "C",
			Title:  "C",
			Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{DependsOnID: "B", Type: model.DepBlocks},
			},
			CreatedAt: now.Add(-2 * time.Hour),
		},
	}

	analyzer := analysis.NewAnalyzer(copyIssues(issues))
	cfg := analysis.ConfigForSize(len(issues), 2)
	cfg.ComputePageRank = true
	cfg.ComputeCriticalPath = true
	cfg.ComputeBetweenness = false
	cfg.ComputeEigenvector = false
	cfg.ComputeHITS = false
	cfg.ComputeCycles = false
	statsValue := analyzer.AnalyzeWithConfig(cfg)

	snapshot := NewSnapshotBuilder(copyIssues(issues)).
		WithAnalysis(&statsValue).
		Build()
	if snapshot == nil {
		t.Fatal("Build returned nil snapshot")
	}
	if snapshot.Analysis == nil {
		t.Fatal("expected Analysis to be populated")
	}

	seenNonZero := false
	for _, item := range snapshot.ListItems {
		want := snapshot.Analysis.GetPageRankScore(item.Issue.ID)
		if want > 0 {
			seenNonZero = true
		}
		if item.GraphScore != want {
			t.Fatalf("GraphScore for %s=%v, want %v", item.Issue.ID, item.GraphScore, want)
		}
		if item.Impact != snapshot.Analysis.GetCriticalPathScore(item.Issue.ID) {
			t.Fatalf("Impact for %s=%v, want %v", item.Issue.ID, item.Impact, snapshot.Analysis.GetCriticalPathScore(item.Issue.ID))
		}
	}
	if !seenNonZero {
		t.Fatal("expected non-zero PageRank scores when Analysis is precomputed")
	}
}

func TestSnapshotBuilder_GraphLayout(t *testing.T) {
	issues := []model.Issue{
		{
			ID:     "A",
			Title:  "Depends on B",
			Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{DependsOnID: "B", Type: model.DepBlocks},
			},
		},
		{ID: "B", Title: "Root", Status: model.StatusOpen},
	}

	snapshot := NewSnapshotBuilder(issues).Build()
	if snapshot.GraphLayout == nil {
		t.Fatal("expected GraphLayout to be computed")
	}

	if got := snapshot.GraphLayout.Blockers["A"]; len(got) != 1 || got[0] != "B" {
		t.Fatalf("unexpected blockers for A: %#v", got)
	}
	if got := snapshot.GraphLayout.Dependents["B"]; len(got) != 1 || got[0] != "A" {
		t.Fatalf("unexpected dependents for B: %#v", got)
	}

	if len(snapshot.GraphLayout.SortedIDs) != len(issues) {
		t.Fatalf("expected %d sorted IDs, got %d", len(issues), len(snapshot.GraphLayout.SortedIDs))
	}
}

func TestSnapshotBuilder_BoardState(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "open-1", Status: model.StatusOpen, Priority: 1, CreatedAt: now},
		{ID: "prog-1", Status: model.StatusInProgress, Priority: 2, CreatedAt: now},
		{ID: "blocked-1", Status: model.StatusBlocked, Priority: 3, CreatedAt: now},
		{ID: "closed-1", Status: model.StatusClosed, Priority: 4, CreatedAt: now},
	}

	snapshot := NewSnapshotBuilder(issues).Build()
	if snapshot.BoardState == nil {
		t.Fatal("expected BoardState to be computed")
	}

	cols := snapshot.BoardState.ByStatus
	if got := len(cols[ColOpen]); got != 1 {
		t.Fatalf("expected 1 open issue, got %d", got)
	}
	if got := len(cols[ColInProgress]); got != 1 {
		t.Fatalf("expected 1 in-progress issue, got %d", got)
	}
	if got := len(cols[ColBlocked]); got != 1 {
		t.Fatalf("expected 1 blocked issue, got %d", got)
	}
	if got := len(cols[ColClosed]); got != 1 {
		t.Fatalf("expected 1 closed issue, got %d", got)
	}
}

func TestSnapshotBuilder_TreeNodes(t *testing.T) {
	issues := []model.Issue{
		{ID: "epic", Title: "Epic", Status: model.StatusOpen, IssueType: model.TypeEpic},
		{
			ID:        "feature",
			Title:     "Feature",
			Status:    model.StatusOpen,
			IssueType: model.TypeFeature,
			Dependencies: []*model.Dependency{
				{DependsOnID: "epic", Type: model.DepParentChild},
			},
		},
		{
			ID:        "task",
			Title:     "Task",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{
				{DependsOnID: "feature", Type: model.DepParentChild},
			},
		},
	}

	snapshot := NewSnapshotBuilder(issues).Build()
	if snapshot == nil {
		t.Fatal("Build returned nil snapshot")
	}
	if len(snapshot.TreeRoots) != 1 {
		t.Fatalf("expected 1 tree root, got %d", len(snapshot.TreeRoots))
	}
	if snapshot.TreeNodeMap == nil {
		t.Fatal("expected TreeNodeMap to be populated")
	}

	root := snapshot.TreeRoots[0]
	if root == nil || root.Issue == nil || root.Issue.ID != "epic" {
		t.Fatalf("expected epic root, got %#v", root)
	}
	if len(root.Children) != 1 || root.Children[0].Issue.ID != "feature" {
		t.Fatalf("expected epic -> feature, got %#v", root.Children)
	}
	if len(root.Children[0].Children) != 1 || root.Children[0].Children[0].Issue.ID != "task" {
		t.Fatalf("expected feature -> task, got %#v", root.Children[0].Children)
	}
	if snapshot.TreeNodeMap["task"] == nil {
		t.Fatal("expected TreeNodeMap to contain task")
	}
}

func TestSnapshotBuilder_ListItems(t *testing.T) {
	issues := []model.Issue{
		{ID: "test-1", Title: "Issue 1", Status: model.StatusOpen, Priority: 1},
	}

	builder := NewSnapshotBuilder(issues)
	snapshot := builder.Build()

	if len(snapshot.ListItems) != 1 {
		t.Fatalf("Expected 1 list item, got %d", len(snapshot.ListItems))
	}

	item := snapshot.ListItems[0]
	if item.Issue.ID != "test-1" {
		t.Errorf("List item has wrong ID: got %q, want %q", item.Issue.ID, "test-1")
	}
}

func TestSnapshotBuilder_WithRecipe_FiltersListItems(t *testing.T) {
	issues := []model.Issue{
		{ID: "open-1", Status: model.StatusOpen, Priority: 2},
		{ID: "closed-1", Status: model.StatusClosed, Priority: 1},
	}

	r := &recipe.Recipe{
		Name: "open-only",
		Filters: recipe.FilterConfig{
			Status: []string{"open"},
		},
	}

	snapshot := NewSnapshotBuilder(issues).WithRecipe(r).Build()
	if snapshot == nil {
		t.Fatal("Build returned nil snapshot")
	}
	if len(snapshot.ListItems) != 1 {
		t.Fatalf("Expected 1 list item, got %d", len(snapshot.ListItems))
	}
	if got := snapshot.ListItems[0].Issue.ID; got != "open-1" {
		t.Fatalf("Expected open-1, got %s", got)
	}
}

func TestSnapshotBuilder_IncrementalListClearsEphemeralFields(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "A", Title: "A", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask, CreatedAt: now},
		{ID: "B", Title: "B", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(-time.Hour)},
	}

	prev := NewSnapshotBuilder(copyIssues(issues)).Build()
	if len(prev.ListItems) == 0 {
		t.Fatal("expected previous list items")
	}

	prev.ListItems[0].SearchScoreSet = true
	prev.ListItems[0].SearchScore = 0.9
	prev.ListItems[0].SearchComponents = map[string]float64{"signal": 1}
	prev.ListItems[0].DiffStatus = DiffStatusModified
	prev.ListItems[0].TriageScore = 0.5
	prev.ListItems[0].TriageReason = "reason"
	prev.ListItems[0].TriageReasons = []string{"reason"}
	prev.ListItems[0].IsQuickWin = true
	prev.ListItems[0].IsBlocker = true
	prev.ListItems[0].UnblocksCount = 2

	diffValue := analysis.ComputeIssueDiff(prev.Issues, issues)
	cfg := snapshotBuildConfigDefault()
	cfg.PrecomputeTriage = false

	next := NewSnapshotBuilder(copyIssues(issues)).
		WithBuildConfig(cfg).
		WithPreviousSnapshot(prev, &diffValue).
		Build()

	if !next.IncrementalListUsed {
		t.Fatal("expected incremental list path")
	}

	for _, item := range next.ListItems {
		if item.SearchScoreSet || item.SearchComponents != nil {
			t.Fatalf("expected search fields cleared, got %#v", item)
		}
		if item.DiffStatus != DiffStatusNone {
			t.Fatalf("expected DiffStatusNone, got %v", item.DiffStatus)
		}
		if item.TriageScore != 0 || item.TriageReason != "" || len(item.TriageReasons) != 0 {
			t.Fatalf("expected triage fields cleared, got %#v", item)
		}
		if item.IsQuickWin || item.IsBlocker || item.UnblocksCount != 0 {
			t.Fatalf("expected triage flags cleared, got %#v", item)
		}
	}
}

func TestSnapshotBuilder_IncrementalListMatchesFull(t *testing.T) {
	now := time.Now()
	issues := make([]model.Issue, 0, 10)
	for i := 0; i < 10; i++ {
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("T-%02d", i),
			Title:     fmt.Sprintf("Issue %d", i),
			Status:    model.StatusOpen,
			Priority:  i,
			IssueType: model.TypeTask,
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
		})
	}

	prev := NewSnapshotBuilder(copyIssues(issues)).Build()
	updated := copyIssues(issues)
	updated[0].Title = "Issue 0 updated"

	diffValue := analysis.ComputeIssueDiff(prev.Issues, updated)
	cfg := snapshotBuildConfigDefault()
	cfg.PrecomputeTriage = false

	incremental := NewSnapshotBuilder(copyIssues(updated)).
		WithBuildConfig(cfg).
		WithPreviousSnapshot(prev, &diffValue).
		Build()
	full := NewSnapshotBuilder(copyIssues(updated)).
		WithBuildConfig(cfg).
		Build()

	if incremental.IssueDiff == nil {
		t.Fatal("expected IssueDiff to be set")
	}
	if got := incremental.IssueDiffStats.Total; got != len(updated) {
		t.Fatalf("IssueDiffStats.Total=%d, want %d", got, len(updated))
	}
	if got := incremental.IssueDiffStats.Changed; got != 1 {
		t.Fatalf("IssueDiffStats.Changed=%d, want 1", got)
	}

	if !reflect.DeepEqual(incremental.ListItems, full.ListItems) {
		if len(incremental.ListItems) != len(full.ListItems) {
			t.Fatalf("incremental list items differ from full rebuild: len=%d want %d", len(incremental.ListItems), len(full.ListItems))
		}
		for i := range incremental.ListItems {
			if !reflect.DeepEqual(incremental.ListItems[i], full.ListItems[i]) {
				t.Fatalf("incremental list items differ from full rebuild at index %d: incremental=%#v full=%#v", i, incremental.ListItems[i], full.ListItems[i])
			}
		}
		t.Fatalf("incremental list items differ from full rebuild")
	}
}

func TestSortIssuesByRecipe_PriorityAsc(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Priority: 2},
		{ID: "Z", Priority: 1},
	}

	r := &recipe.Recipe{Sort: recipe.SortConfig{Field: "priority", Direction: "asc"}}
	sortIssuesByRecipe(issues, nil, r)

	if issues[0].ID != "Z" || issues[1].ID != "A" {
		t.Fatalf("expected Z then A, got %s then %s", issues[0].ID, issues[1].ID)
	}
}

func TestSortIssuesByRecipe_PriorityDesc_TieBreakByID(t *testing.T) {
	issues := []model.Issue{
		{ID: "B", Priority: 1},
		{ID: "A", Priority: 1},
	}

	r := &recipe.Recipe{Sort: recipe.SortConfig{Field: "priority", Direction: "desc"}}
	sortIssuesByRecipe(issues, nil, r)

	if issues[0].ID != "A" || issues[1].ID != "B" {
		t.Fatalf("expected A then B, got %s then %s", issues[0].ID, issues[1].ID)
	}
}

func TestSnapshotBuilder_WithPrecomputedAnalysis(t *testing.T) {
	issues := []model.Issue{
		{ID: "test-1", Title: "Issue 1", Status: model.StatusOpen},
	}

	// Create a snapshot using the synchronous analysis
	builder := NewSnapshotBuilder(issues)
	snapshot := builder.Build()

	if snapshot.Analysis == nil {
		t.Error("Analysis should be computed")
	}
}

func TestSnapshotSwap_PreservesBoardSelectionByID(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "open-1", Title: "Open", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "prog-1", Title: "Prog 1", Status: model.StatusInProgress, Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(-2 * time.Hour)},
	}

	m := NewModel(issues, nil, "")
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = newM.(Model)

	if m.focused != focusBoard {
		t.Fatalf("expected focusBoard, got %v", m.focused)
	}

	// Select prog-1 in the in-progress column.
	m.board.MoveRight()
	if sel := m.board.SelectedIssue(); sel == nil || sel.ID != "prog-1" {
		t.Fatalf("expected board selection prog-1, got %#v", sel)
	}

	// Insert a new in-progress issue that sorts ahead of prog-1.
	updatedIssues := []model.Issue{
		{ID: "open-1", Title: "Open", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "prog-2", Title: "Prog 2", Status: model.StatusInProgress, Priority: 0, IssueType: model.TypeTask, CreatedAt: now.Add(-1 * time.Minute)},
		{ID: "prog-1", Title: "Prog 1", Status: model.StatusInProgress, Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(-2 * time.Hour)},
	}
	snapshot := NewSnapshotBuilder(updatedIssues).Build()

	newM, _ = m.Update(SnapshotReadyMsg{Snapshot: snapshot})
	m = newM.(Model)

	if m.focused != focusBoard {
		t.Fatalf("expected focusBoard after swap, got %v", m.focused)
	}
	if sel := m.board.SelectedIssue(); sel == nil || sel.ID != "prog-1" {
		t.Fatalf("expected board selection prog-1 after swap, got %#v", sel)
	}
}

func TestSnapshotSwap_UsesSnapshotInsights(t *testing.T) {
	issues := []model.Issue{
		{ID: "test-1", Title: "Issue 1", Status: model.StatusOpen, Priority: 1},
	}

	m := NewModel(issues, nil, "")

	snapshot := NewSnapshotBuilder(issues).Build()
	snapshot.Insights.Bottlenecks = []analysis.InsightItem{{ID: "sentinel", Value: 1}}

	newM, _ := m.Update(SnapshotReadyMsg{Snapshot: snapshot})
	m = newM.(Model)

	if len(m.insightsPanel.insights.Bottlenecks) == 0 || m.insightsPanel.insights.Bottlenecks[0].ID != "sentinel" {
		t.Fatalf("expected insights to come from snapshot")
	}
}

func TestSnapshotSwap_UsesSnapshotGraphLayoutWhenUnfiltered(t *testing.T) {
	issues := []model.Issue{
		{
			ID:     "A",
			Title:  "Depends on B",
			Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{DependsOnID: "B", Type: model.DepBlocks},
			},
		},
		{ID: "B", Title: "Root", Status: model.StatusOpen},
	}

	m := NewModel(issues, nil, "")
	m.currentFilter = "all"

	snapshot := NewSnapshotBuilder(issues).Build()
	if snapshot.GraphLayout == nil {
		t.Fatal("expected snapshot GraphLayout")
	}

	// Sentinel tweak: if the UI rebuilds graph relationships from issues (SetIssues),
	// blockers["A"] will be ["B"]. If it uses the snapshot layout (SetSnapshot),
	// it will preserve this sentinel.
	snapshot.GraphLayout.Blockers["A"] = []string{"SENTINEL"}

	newM, _ := m.Update(SnapshotReadyMsg{Snapshot: snapshot})
	m = newM.(Model)

	if got := m.graphView.SelectedIssue(); got == nil {
		t.Fatal("expected graph view to have a selection")
	}
	if got := m.graphView.blockers["A"]; len(got) != 1 || got[0] != "SENTINEL" {
		t.Fatalf("expected graph view to use snapshot GraphLayout, got blockers[A]=%#v", got)
	}
}

func TestPhase2ReadyMsg_DoesNotRebuildGraphViewWhenSnapshotHasLayout(t *testing.T) {
	issues := []model.Issue{
		{
			ID:     "A",
			Title:  "Depends on B",
			Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{DependsOnID: "B", Type: model.DepBlocks},
			},
		},
		{ID: "B", Title: "Root", Status: model.StatusOpen},
	}

	m := NewModel(issues, nil, "")
	m.currentFilter = "all"

	snapshot := NewSnapshotBuilder(issues).Build()
	snapshot.GraphLayout.Blockers["A"] = []string{"SENTINEL"}

	newM, _ := m.Update(SnapshotReadyMsg{Snapshot: snapshot})
	m = newM.(Model)

	// Simulate Phase 2 completion message; Stats identity must match m.analysis.
	ins := m.analysis.GenerateInsights(len(m.issues))
	newM, _ = m.Update(Phase2ReadyMsg{Stats: m.analysis, Insights: ins})
	m = newM.(Model)

	if got := m.graphView.blockers["A"]; len(got) != 1 || got[0] != "SENTINEL" {
		t.Fatalf("expected Phase2ReadyMsg to preserve snapshot GraphLayout, got blockers[A]=%#v", got)
	}
}

func TestSnapshotSwap_PreservesInsightsNavigationState(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "a", Title: "A", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "b", Title: "B", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(-1 * time.Hour)},
	}

	m := NewModel(issues, nil, "")
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m = newM.(Model)

	if m.focused != focusInsights {
		t.Fatalf("expected focusInsights, got %v", m.focused)
	}

	// Simulate user navigating within the insights dashboard.
	m.insightsPanel.focusedPanel = PanelCycles

	updated := append([]model.Issue(nil), issues...)
	updated[0].Title = "A (updated)"
	snapshot := NewSnapshotBuilder(updated).Build()

	newM, _ = m.Update(SnapshotReadyMsg{Snapshot: snapshot})
	m = newM.(Model)

	if m.focused != focusInsights {
		t.Fatalf("expected focusInsights after swap, got %v", m.focused)
	}
	if m.insightsPanel.focusedPanel != PanelCycles {
		t.Fatalf("expected focusedPanel preserved (%v), got %v", PanelCycles, m.insightsPanel.focusedPanel)
	}
}

func TestSnapshotSwap_RebuildsTreeWhenFocusedAndPreservesSelection(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{
			ID:        "parent",
			Title:     "Parent",
			Status:    model.StatusOpen,
			Priority:  1,
			IssueType: model.TypeTask,
			CreatedAt: now.Add(-3 * time.Hour),
		},
		{
			ID:        "child",
			Title:     "Child",
			Status:    model.StatusOpen,
			Priority:  2,
			IssueType: model.TypeTask,
			CreatedAt: now.Add(-2 * time.Hour),
			Dependencies: []*model.Dependency{
				{DependsOnID: "parent", Type: model.DepParentChild},
			},
		},
	}

	m := NewModel(issues, nil, "")

	// Isolate persistent tree state from the repo's .beads.
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}
	m.tree.SetBeadsDir(beadsDir)

	// Enter tree view and select the child.
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
	m = newM.(Model)
	if m.focused != focusTree {
		t.Fatalf("expected focusTree, got %v", m.focused)
	}
	m.tree.MoveDown()
	selected := m.tree.SelectedIssue()
	if selected == nil {
		t.Fatal("expected non-nil tree selection")
	}
	selectedID := selected.ID

	// New snapshot keeps the selected issue but adds another sibling.
	updated := []model.Issue{
		issues[0],
		issues[1],
		{
			ID:        "child-2",
			Title:     "Child 2",
			Status:    model.StatusOpen,
			Priority:  1,
			IssueType: model.TypeTask,
			CreatedAt: now.Add(-1 * time.Hour),
			Dependencies: []*model.Dependency{
				{DependsOnID: "parent", Type: model.DepParentChild},
			},
		},
	}
	snapshot := NewSnapshotBuilder(updated).Build()

	newM, _ = m.Update(SnapshotReadyMsg{Snapshot: snapshot})
	m = newM.(Model)
	if m.focused != focusTree {
		t.Fatalf("expected focusTree after swap, got %v", m.focused)
	}
	if sel := m.tree.SelectedIssue(); sel == nil || sel.ID != selectedID {
		t.Fatalf("expected tree selection preserved (%s), got %#v", selectedID, sel)
	}
}
