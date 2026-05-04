package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/ui"
)

// createTestInsights creates a test Insights struct with sample data
func createTestInsights() analysis.Insights {
	return analysis.Insights{
		Bottlenecks: []analysis.InsightItem{
			{ID: "bottleneck-1", Value: 0.85},
			{ID: "bottleneck-2", Value: 0.65},
			{ID: "bottleneck-3", Value: 0.45},
		},
		Keystones: []analysis.InsightItem{
			{ID: "keystone-1", Value: 5.0},
			{ID: "keystone-2", Value: 3.0},
		},
		Influencers: []analysis.InsightItem{
			{ID: "influencer-1", Value: 0.92},
		},
		Hubs: []analysis.InsightItem{
			{ID: "hub-1", Value: 2.5},
			{ID: "hub-2", Value: 1.8},
		},
		Authorities: []analysis.InsightItem{
			{ID: "auth-1", Value: 3.2},
		},
		Cores: []analysis.InsightItem{
			{ID: "core-1", Value: 3},
			{ID: "core-2", Value: 2},
		},
		Articulation: []string{"art-1"},
		Slack: []analysis.InsightItem{
			{ID: "slack-1", Value: 4},
			{ID: "slack-2", Value: 2},
		},
		Cycles: [][]string{
			{"cycle-a", "cycle-b", "cycle-c"},
			{"cycle-x", "cycle-y"},
		},
		ClusterDensity: 0.42,
		Stats: analysis.NewGraphStatsForTest(
			map[string]float64{"bottleneck-1": 0.15},                       // pageRank
			map[string]float64{"bottleneck-1": 0.85, "bottleneck-2": 0.65}, // betweenness
			map[string]float64{"influencer-1": 0.92},                       // eigenvector
			map[string]float64{"hub-1": 2.5, "hub-2": 1.8},                 // hubs
			map[string]float64{"auth-1": 3.2},                              // authorities
			map[string]float64{"keystone-1": 5.0, "keystone-2": 3.0},       // criticalPathScore
			map[string]int{"bottleneck-1": 2},                              // outDegree
			map[string]int{"bottleneck-1": 3},                              // inDegree
			nil,                                                            // cycles
			0,                                                              // density
			nil,                                                            // topologicalOrder
		),
	}
}

// createTestIssueMap creates a map of test issues
func createTestIssueMap() map[string]*model.Issue {
	issues := []model.Issue{
		{ID: "bottleneck-1", Title: "Critical Junction", Status: model.StatusInProgress, IssueType: model.TypeBug},
		{ID: "bottleneck-2", Title: "Secondary Junction", Status: model.StatusOpen, IssueType: model.TypeFeature},
		{ID: "bottleneck-3", Title: "Minor Junction", Status: model.StatusOpen},
		{ID: "keystone-1", Title: "Foundation Component", Status: model.StatusOpen, IssueType: model.TypeTask},
		{ID: "keystone-2", Title: "Base Layer", Status: model.StatusClosed},
		{ID: "influencer-1", Title: "Central Hub", Status: model.StatusInProgress},
		{ID: "hub-1", Title: "Feature Epic", Status: model.StatusOpen, Dependencies: []*model.Dependency{
			{DependsOnID: "auth-1", Type: model.DepBlocks},
		}},
		{ID: "hub-2", Title: "Another Epic", Status: model.StatusOpen},
		{ID: "auth-1", Title: "Core Service", Status: model.StatusClosed},
		{ID: "cycle-a", Title: "Cycle Part A", Status: model.StatusBlocked},
		{ID: "cycle-b", Title: "Cycle Part B", Status: model.StatusBlocked},
		{ID: "cycle-c", Title: "Cycle Part C", Status: model.StatusBlocked},
		{ID: "cycle-x", Title: "Cycle X", Status: model.StatusBlocked},
		{ID: "cycle-y", Title: "Cycle Y", Status: model.StatusBlocked},
		{ID: "core-1", Title: "Core Node 1", Status: model.StatusOpen},
		{ID: "core-2", Title: "Core Node 2", Status: model.StatusOpen},
		{ID: "art-1", Title: "Articulation", Status: model.StatusOpen},
		{ID: "slack-1", Title: "Slack Node 1", Status: model.StatusOpen},
		{ID: "slack-2", Title: "Slack Node 2", Status: model.StatusOpen},
	}

	issueMap := make(map[string]*model.Issue)
	for i := range issues {
		issueMap[issues[i].ID] = &issues[i]
	}
	return issueMap
}

// TestInsightsModelEmpty verifies behavior with empty insights
func TestInsightsModelEmpty(t *testing.T) {
	theme := createTheme()
	emptyInsights := analysis.Insights{}
	emptyMap := make(map[string]*model.Issue)

	m := ui.NewInsightsModel(emptyInsights, emptyMap, theme)
	m.SetSize(120, 40)

	// Navigation should not panic on empty panels
	m.MoveUp()
	m.MoveDown()
	m.NextPanel()
	m.PrevPanel()

	// SelectedIssueID should return empty string
	if id := m.SelectedIssueID(); id != "" {
		t.Errorf("Expected empty ID for empty insights, got %s", id)
	}

	// View should not panic
	_ = m.View()
}

// TestInsightsModelPanelNavigation verifies panel navigation
func TestInsightsModelPanelNavigation(t *testing.T) {
	theme := createTheme()
	ins := createTestInsights()
	issueMap := createTestIssueMap()

	m := ui.NewInsightsModel(ins, issueMap, theme)
	m.SetSize(120, 40)

	// Start on Bottlenecks panel (index 0)
	id := m.SelectedIssueID()
	if id != "bottleneck-1" {
		t.Errorf("Expected bottleneck-1 on start, got %s", id)
	}

	// NextPanel should move to Keystones
	m.NextPanel()
	id = m.SelectedIssueID()
	if id != "keystone-1" {
		t.Errorf("Expected keystone-1 after NextPanel, got %s", id)
	}

	// NextPanel to Influencers
	m.NextPanel()
	id = m.SelectedIssueID()
	if id != "influencer-1" {
		t.Errorf("Expected influencer-1 after NextPanel, got %s", id)
	}

	// NextPanel to Hubs
	m.NextPanel()
	id = m.SelectedIssueID()
	if id != "hub-1" {
		t.Errorf("Expected hub-1 after NextPanel, got %s", id)
	}

	// NextPanel to Authorities
	m.NextPanel()
	id = m.SelectedIssueID()
	if id != "auth-1" {
		t.Errorf("Expected auth-1 after NextPanel, got %s", id)
	}

	// NextPanel to Cores
	m.NextPanel()
	id = m.SelectedIssueID()
	if id != "core-1" {
		t.Errorf("Expected core-1 after NextPanel, got %s", id)
	}

	// NextPanel to Articulation
	m.NextPanel()
	id = m.SelectedIssueID()
	if id != "art-1" {
		t.Errorf("Expected art-1 after NextPanel, got %s", id)
	}

	// NextPanel to Slack
	m.NextPanel()
	id = m.SelectedIssueID()
	if id != "slack-1" {
		t.Errorf("Expected slack-1 after NextPanel, got %s", id)
	}

	// PrevPanel should go back to Articulation
	m.PrevPanel()
	id = m.SelectedIssueID()
	if id != "art-1" {
		t.Errorf("Expected art-1 after PrevPanel, got %s", id)
	}
}

// TestInsightsModelItemNavigation verifies up/down navigation within panels
func TestInsightsModelItemNavigation(t *testing.T) {
	theme := createTheme()
	ins := createTestInsights()
	issueMap := createTestIssueMap()

	m := ui.NewInsightsModel(ins, issueMap, theme)
	m.SetSize(120, 40)

	// Start on Bottlenecks panel, first item
	id := m.SelectedIssueID()
	if id != "bottleneck-1" {
		t.Errorf("Expected bottleneck-1, got %s", id)
	}

	// MoveDown to second item
	m.MoveDown()
	id = m.SelectedIssueID()
	if id != "bottleneck-2" {
		t.Errorf("Expected bottleneck-2 after MoveDown, got %s", id)
	}

	// MoveDown to third item
	m.MoveDown()
	id = m.SelectedIssueID()
	if id != "bottleneck-3" {
		t.Errorf("Expected bottleneck-3 after MoveDown, got %s", id)
	}

	// MoveDown at bottom should stay at bottom
	m.MoveDown()
	id = m.SelectedIssueID()
	if id != "bottleneck-3" {
		t.Errorf("Expected to stay at bottleneck-3, got %s", id)
	}

	// MoveUp should go back
	m.MoveUp()
	id = m.SelectedIssueID()
	if id != "bottleneck-2" {
		t.Errorf("Expected bottleneck-2 after MoveUp, got %s", id)
	}

	// MoveUp to first
	m.MoveUp()
	id = m.SelectedIssueID()
	if id != "bottleneck-1" {
		t.Errorf("Expected bottleneck-1 after MoveUp, got %s", id)
	}

	// MoveUp at top should stay at top
	m.MoveUp()
	id = m.SelectedIssueID()
	if id != "bottleneck-1" {
		t.Errorf("Expected to stay at bottleneck-1, got %s", id)
	}
}

// TestInsightsModelCyclesPanelNavigation verifies navigation in cycles panel
func TestInsightsModelCyclesPanelNavigation(t *testing.T) {
	theme := createTheme()
	ins := createTestInsights()
	issueMap := createTestIssueMap()

	m := ui.NewInsightsModel(ins, issueMap, theme)
	m.SetSize(120, 40)

	// Navigate to Cycles panel (8 NextPanels from start)
	for i := 0; i < 8; i++ {
		m.NextPanel()
	}

	// Should be on first cycle, returning first item
	id := m.SelectedIssueID()
	if id != "cycle-a" {
		t.Errorf("Expected cycle-a, got %s", id)
	}

	// MoveDown to second cycle
	m.MoveDown()
	id = m.SelectedIssueID()
	if id != "cycle-x" {
		t.Errorf("Expected cycle-x (first of second cycle), got %s", id)
	}

	// MoveUp back to first cycle
	m.MoveUp()
	id = m.SelectedIssueID()
	if id != "cycle-a" {
		t.Errorf("Expected cycle-a after MoveUp, got %s", id)
	}
}

// TestInsightsModelToggleFunctions verifies toggle methods
func TestInsightsModelToggleFunctions(t *testing.T) {
	theme := createTheme()
	ins := createTestInsights()
	issueMap := createTestIssueMap()

	m := ui.NewInsightsModel(ins, issueMap, theme)
	m.SetSize(120, 40)

	// Toggle explanations - should not panic
	m.ToggleExplanations()
	_ = m.View()
	m.ToggleExplanations()
	_ = m.View()

	// Toggle calculation - should not panic
	m.ToggleCalculation()
	_ = m.View()
	m.ToggleCalculation()
	_ = m.View()

	// Toggle heatmap view (bv-95) - should not panic
	m.ToggleHeatmap()
	_ = m.View()
	m.ToggleHeatmap()
	_ = m.View()
}

// TestInsightsModelSetInsights verifies SetInsights updates data
func TestInsightsModelSetInsights(t *testing.T) {
	theme := createTheme()
	ins := createTestInsights()
	issueMap := createTestIssueMap()

	m := ui.NewInsightsModel(ins, issueMap, theme)
	m.SetSize(120, 40)

	// Verify initial data
	id := m.SelectedIssueID()
	if id != "bottleneck-1" {
		t.Errorf("Expected bottleneck-1, got %s", id)
	}

	// Update with new insights
	newInsights := analysis.Insights{
		Bottlenecks: []analysis.InsightItem{
			{ID: "new-bottleneck", Value: 0.99},
		},
	}
	m.SetInsights(newInsights)

	// Should now show new data
	id = m.SelectedIssueID()
	if id != "new-bottleneck" {
		t.Errorf("Expected new-bottleneck after SetInsights, got %s", id)
	}
}

// TestInsightsModelViewRendering verifies View doesn't panic
func TestInsightsModelViewRendering(t *testing.T) {
	theme := createTheme()
	ins := createTestInsights()
	issueMap := createTestIssueMap()

	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"standard", 120, 40},
		{"narrow", 80, 40},
		{"wide", 200, 50},
		{"short", 120, 20},
		{"minimal", 60, 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ui.NewInsightsModel(ins, issueMap, theme)
			m.SetSize(tt.width, tt.height)
			// Should not panic
			_ = m.View()
		})
	}
}

// TestInsightsModelAllPanelsRender verifies all panel types render without panic
func TestInsightsModelAllPanelsRender(t *testing.T) {
	theme := createTheme()
	ins := createTestInsights()
	issueMap := createTestIssueMap()

	m := ui.NewInsightsModel(ins, issueMap, theme)
	m.SetSize(120, 40)

	// Render each panel type
	for i := 0; i < 6; i++ {
		_ = m.View()
		m.NextPanel()
	}
}

// TestInsightsModelEmptyCycles verifies cycles panel with no cycles
func TestInsightsModelEmptyCycles(t *testing.T) {
	theme := createTheme()
	ins := analysis.Insights{
		Bottlenecks: []analysis.InsightItem{{ID: "test", Value: 1.0}},
		Cycles:      [][]string{}, // No cycles
	}
	issueMap := createTestIssueMap()

	m := ui.NewInsightsModel(ins, issueMap, theme)
	m.SetSize(120, 40)

	// Navigate to cycles panel
	for i := 0; i < 5; i++ {
		m.NextPanel()
	}

	// SelectedIssueID should return empty for empty cycles
	id := m.SelectedIssueID()
	if id != "" {
		t.Errorf("Expected empty ID for empty cycles, got %s", id)
	}

	// View should not panic
	_ = m.View()
}

// TestInsightsModelMissingIssue verifies handling when issue not in map
func TestInsightsModelMissingIssue(t *testing.T) {
	theme := createTheme()
	ins := analysis.Insights{
		Bottlenecks: []analysis.InsightItem{
			{ID: "missing-issue", Value: 0.5},
		},
	}
	// Issue map doesn't contain "missing-issue"
	issueMap := make(map[string]*model.Issue)

	m := ui.NewInsightsModel(ins, issueMap, theme)
	m.SetSize(120, 40)

	// Should still return the ID even if issue not in map
	id := m.SelectedIssueID()
	if id != "missing-issue" {
		t.Errorf("Expected missing-issue, got %s", id)
	}

	// View should not panic even with missing issue
	_ = m.View()
}

// TestInsightsModelSetSizeBeforeView verifies SetSize must be called before View
func TestInsightsModelSetSizeBeforeView(t *testing.T) {
	theme := createTheme()
	ins := createTestInsights()
	issueMap := createTestIssueMap()

	m := ui.NewInsightsModel(ins, issueMap, theme)
	// Don't call SetSize

	// View should return empty string when not ready
	view := m.View()
	if view != "" {
		t.Errorf("Expected empty view before SetSize, got %d chars", len(view))
	}
}

// TestInsightsModelDetailPanel verifies detail panel rendering
func TestInsightsModelDetailPanel(t *testing.T) {
	theme := createTheme()

	// Create issue with full details
	issue := model.Issue{
		ID:                 "detailed-issue",
		Title:              "Detailed Issue Title",
		Description:        "This is a detailed description of the issue.",
		Design:             "Design notes go here.",
		AcceptanceCriteria: "AC: Must work correctly.",
		Notes:              "Additional notes.",
		Status:             model.StatusInProgress,
		IssueType:          model.TypeFeature,
		Priority:           2,
		Assignee:           "testuser",
		Dependencies: []*model.Dependency{
			{DependsOnID: "dep-1", Type: model.DepBlocks},
		},
	}
	issueMap := map[string]*model.Issue{
		"detailed-issue": &issue,
		"dep-1":          {ID: "dep-1", Title: "Dependency One"},
	}

	ins := analysis.Insights{
		Bottlenecks: []analysis.InsightItem{
			{ID: "detailed-issue", Value: 0.75},
		},
		Stats: analysis.NewGraphStatsForTest(
			map[string]float64{"detailed-issue": 0.1},  // pageRank
			map[string]float64{"detailed-issue": 0.75}, // betweenness
			map[string]float64{"detailed-issue": 0.5},  // eigenvector
			map[string]float64{"detailed-issue": 1.0},  // hubs
			map[string]float64{"detailed-issue": 2.0},  // authorities
			map[string]float64{"detailed-issue": 3.0},  // criticalPathScore
			map[string]int{"detailed-issue": 1},        // outDegree
			map[string]int{"detailed-issue": 2},        // inDegree
			nil, 0, nil,
		),
	}

	m := ui.NewInsightsModel(ins, issueMap, theme)
	// Wide enough to show detail panel
	m.SetSize(150, 40)

	// Should not panic with full details
	_ = m.View()
}

// TestInsightsModelCalculationProofAllPanels verifies calculation proof for each panel type
func TestInsightsModelCalculationProofAllPanels(t *testing.T) {
	theme := createTheme()
	ins := createTestInsights()
	issueMap := createTestIssueMap()

	m := ui.NewInsightsModel(ins, issueMap, theme)
	// Wide enough to show detail panel with calculation proof
	m.SetSize(180, 50)

	// Test each panel's calculation proof
	for i := 0; i < 6; i++ {
		_ = m.View()
		m.NextPanel()
	}
}

// TestInsightsModelLongCycleChain verifies rendering of long cycle chains
func TestInsightsModelLongCycleChain(t *testing.T) {
	theme := createTheme()

	// Create a long cycle
	longCycle := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	ins := analysis.Insights{
		Cycles: [][]string{longCycle},
	}

	// Create issue map with all cycle members
	issueMap := make(map[string]*model.Issue)
	for _, id := range longCycle {
		issueMap[id] = &model.Issue{ID: id, Title: "Cycle member " + id}
	}

	m := ui.NewInsightsModel(ins, issueMap, theme)
	m.SetSize(120, 40)

	// Navigate to cycles panel
	for i := 0; i < 5; i++ {
		m.NextPanel()
	}

	// View should not panic with long cycle
	_ = m.View()
}

// TestInsightsModelScrolling verifies scrolling behavior with many items
func TestInsightsModelScrolling(t *testing.T) {
	theme := createTheme()

	// Create many bottleneck items
	var bottlenecks []analysis.InsightItem
	issueMap := make(map[string]*model.Issue)
	for i := 0; i < 50; i++ {
		id := string(rune('A' + i%26))
		if i >= 26 {
			id = id + string(rune('A'+i%26))
		}
		bottlenecks = append(bottlenecks, analysis.InsightItem{ID: id, Value: float64(50 - i)})
		issueMap[id] = &model.Issue{ID: id, Title: "Issue " + id}
	}

	ins := analysis.Insights{
		Bottlenecks: bottlenecks,
	}

	m := ui.NewInsightsModel(ins, issueMap, theme)
	m.SetSize(120, 30) // Short height to trigger scrolling

	// Navigate down through all items
	for i := 0; i < 55; i++ {
		m.MoveDown()
		_ = m.View() // Scrolling happens during view
	}

	// Navigate back up
	for i := 0; i < 55; i++ {
		m.MoveUp()
		_ = m.View()
	}
}

// TestInsightsResponsiveLayoutNoTruncation regression-tests bt-y0fv.1: at narrow
// widths the insights view must not render the canonical 3x4 grid in a way that
// truncates pane titles mid-word. The fix tightens the detail-panel activation
// threshold and adds 2-col / 1-col fallbacks below the 3-col floor.
//
// The canonical truncation symptoms reported in the bead were title prefixes
// like "Articul" / "Centralit" / "Influence" / "Eigenve" — i.e. mid-word cuts
// of metric panel titles ("Cut Points" subtitle "Articulation Vertices",
// "Influencers" / "Bottlenecks Centrality", etc.). The render path uses
// runewidth.Truncate with a "…" suffix when title overflows; the regression
// test asserts both that the full canonical titles appear and that the
// ellipsis-truncated forms do not.
func TestInsightsResponsiveLayoutNoTruncation(t *testing.T) {
	theme := createTheme()
	ins := createTestInsights()
	issueMap := createTestIssueMap()

	// Width 121 was the canonical breakage point reported in the bead: the
	// detail panel kicks in at width > 120 and steals ~41 cols, leaving
	// mainWidth=80 -> colWidth=24. After the fix, detail is suppressed below
	// width 141 so the 3-col grid keeps the full mainWidth.
	//
	// Width 100 stays in 3-col without detail. Width 80 falls through to the
	// 2-col fallback. Width 50 falls through to the 1-col stack.
	cases := []struct {
		name       string
		width      int
		height     int
		wantTitles []string // substrings that MUST appear in the rendered view
	}{
		{
			name:   "canonical_breakage_121",
			width:  121,
			height: 50,
			wantTitles: []string{
				"Bottlenecks",
				"Keystones",
				"Influencers",
				"Hubs",
				"Authorities",
				"Cores",
				"Cut Points",
				"Slack",
				"Cycles",
			},
		},
		{
			name:   "narrow_3col_100",
			width:  100,
			height: 50,
			wantTitles: []string{
				"Bottlenecks",
				"Influencers",
				"Authorities",
				"Cut Points",
				"Cycles",
			},
		},
		{
			name:   "fallback_2col_80",
			width:  80,
			height: 50,
			wantTitles: []string{
				"Bottlenecks",
				"Influencers",
				"Authorities",
				"Cut Points",
				"Cycles",
			},
		},
		{
			name:   "fallback_1col_50",
			width:  50,
			height: 80,
			wantTitles: []string{
				"Bottlenecks",
				"Influencers",
				"Authorities",
				"Cut Points",
				"Cycles",
			},
		},
		{
			name:   "wide_with_detail_160",
			width:  160,
			height: 50,
			wantTitles: []string{
				"Bottlenecks",
				"Influencers",
				"Cut Points",
				"Cycles",
			},
		},
	}

	// Mid-word truncation patterns that the bug produced. RenderTitledPanel
	// truncates with a "…" suffix when title overflows, so the substring
	// "Articul…" or "Centralit…" etc. appearing in the stripped output would
	// indicate the bug has regressed. Subtitles inside panels are body-
	// truncated without ellipsis, so we test those by asserting the FULL
	// subtitle is present (subtitle truncation would clip the trailing word).
	truncationPatterns := []string{
		"Articul…",
		"Centralit…",
		"Influence…",
		"Eigenve…",
		"Bottlene…",
		"Keystone…",
		"Authorit…",
		"Influenc…",
		"Cut Poin…",
	}

	// Subtitles that must render in full (subtitle clipping was the visible
	// "Eigenve" / "Articul" symptom in the bead).
	wantSubtitles := []string{
		"Betweenness Centrality",
		"Eigenvector Centrality",
		"Articulation Vertices",
		"HITS Hub Score",
		"HITS Authority Score",
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			m := ui.NewInsightsModel(ins, issueMap, theme)
			m.SetSize(tt.width, tt.height)
			rendered := m.View()
			plain := ansi.Strip(rendered)

			for _, title := range tt.wantTitles {
				if !strings.Contains(plain, title) {
					t.Errorf("expected title %q to appear in rendered view at width=%d, but missing", title, tt.width)
				}
			}

			for _, sub := range wantSubtitles {
				if !strings.Contains(plain, sub) {
					t.Errorf("expected subtitle %q to appear in full at width=%d, but missing (subtitle truncation regression)", sub, tt.width)
				}
			}

			for _, pat := range truncationPatterns {
				if strings.Contains(plain, pat) {
					t.Errorf("rendered view at width=%d contains mid-word truncation pattern %q (regression of bt-y0fv.1)", tt.width, pat)
				}
			}
		})
	}
}

// TestInsightsResponsiveColumnCount verifies the layout picks the right
// number of columns for representative widths. This is a behavioral check
// against the responsive thresholds (bt-y0fv.1) and is implemented by
// counting how many top-line border corners ("╭") appear on the first
// rendered row of the metric grid.
func TestInsightsResponsiveColumnCount(t *testing.T) {
	theme := createTheme()
	ins := createTestInsights()
	issueMap := createTestIssueMap()

	cases := []struct {
		name     string
		width    int
		wantCols int
	}{
		// Below 60: single-column stack.
		{"col_1_w50", 50, 1},
		// Between 60 and 89: two-column stack.
		{"col_2_w70", 70, 2},
		{"col_2_w80", 80, 2},
		// 90+: three-column grid. Detail panel activates only when there is
		// still room for a 3-col main grid (mainWidth >= 90) after subtracting
		// it; at narrower widths the detail panel is suppressed.
		{"col_3_w100", 100, 3},
		{"col_3_w121", 121, 3}, // canonical breakage point: detail suppressed
		{"col_3_w160_detail", 160, 3}, // detail active, still 3 metric cols
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			m := ui.NewInsightsModel(ins, issueMap, theme)
			m.SetSize(tt.width, 50)
			rendered := m.View()
			plain := ansi.Strip(rendered)

			// First non-empty line = top border row of the first metric row.
			// Count "╭" occurrences. The detail panel adds one more "╭" on
			// the same row when active, so subtract 1 when width >= 141.
			lines := strings.Split(plain, "\n")
			var firstRow string
			for _, line := range lines {
				if strings.Contains(line, "╭") {
					firstRow = line
					break
				}
			}
			if firstRow == "" {
				t.Fatalf("no top-border row found in rendered view at width=%d", tt.width)
			}

			gotCols := strings.Count(firstRow, "╭")
			// Detail panel renders in the same top-border row as the metric
			// panels when active; detect it by the "Details" border title.
			if strings.Contains(firstRow, "Details") {
				gotCols -= 1
			}
			if gotCols != tt.wantCols {
				t.Errorf("at width=%d: want %d metric columns, got %d (first border row: %q)",
					tt.width, tt.wantCols, gotCols, firstRow)
			}
		})
	}
}
