package ui

import (
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"

	tea "charm.land/bubbletea/v2"
)

// buildRecommendationFixture returns a "blocker" issue plus three dependents
// under the given repo prefix. This mirrors
// analysis.TestGenerateRecommendationsHighImpactLowPriority's fixture shape
// (a stale, medium-priority issue blocking several others), which reliably
// clears GenerateRecommendations' default confidence threshold so the
// blocker issue always produces a PriorityRecommendation when analyzed.
func buildRecommendationFixture(prefix string) []model.Issue {
	now := time.Now()
	blockerID := prefix + "-blocker"
	return []model.Issue{
		{ID: blockerID, Title: prefix + " blocker", Status: model.StatusOpen, Priority: 3, UpdatedAt: now.AddDate(0, 0, -20)},
		{ID: prefix + "-dep1", Title: prefix + " dep1", Status: model.StatusOpen, Priority: 1, Dependencies: []*model.Dependency{
			{DependsOnID: blockerID, Type: model.DepBlocks},
		}},
		{ID: prefix + "-dep2", Title: prefix + " dep2", Status: model.StatusOpen, Priority: 1, Dependencies: []*model.Dependency{
			{DependsOnID: blockerID, Type: model.DepBlocks},
		}},
		{ID: prefix + "-dep3", Title: prefix + " dep3", Status: model.StatusOpen, Priority: 1, Dependencies: []*model.Dependency{
			{DependsOnID: blockerID, Type: model.DepBlocks},
		}},
	}
}

// TestRecomputePriorityHintsRespectsActiveRepos is the direct repro for
// bt-gcuv: pressing 'p' in global/workspace mode with a project filter
// active computed recommendations from all cross-project issues instead of
// the filtered set. recomputePriorityHints must build its Analyzer from
// filteredIssuesForActiveView(), so a projb-only recommendation can never
// appear once activeRepos narrows the view to proja.
func TestRecomputePriorityHintsRespectsActiveRepos(t *testing.T) {
	var issues []model.Issue
	issues = append(issues, buildRecommendationFixture("proja")...)
	issues = append(issues, buildRecommendationFixture("projb")...)

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"proja-", "projb-"},
	})

	// Sanity check: with no project filter, both projects' blockers clear
	// the recommendation threshold. This proves the fixture is valid and
	// that a later absence of projb-blocker is due to the repo filter, not
	// the fixture failing to generate a recommendation at all.
	m.recomputePriorityHints()
	if _, ok := m.ac.priorityHints["proja-blocker"]; !ok {
		t.Fatalf("expected proja-blocker recommendation with no filter, hints=%v", m.ac.priorityHints)
	}
	if _, ok := m.ac.priorityHints["projb-blocker"]; !ok {
		t.Fatalf("expected projb-blocker recommendation with no filter, hints=%v", m.ac.priorityHints)
	}

	// bt-gcuv: filtering to proja must scope recommendations to proja only.
	m.activeRepos = map[string]bool{"proja": true}
	m.recomputePriorityHints()

	if _, ok := m.ac.priorityHints["projb-blocker"]; ok {
		t.Fatalf("projb recommendation leaked into proja-filtered priority hints: %v", m.ac.priorityHints)
	}
	if _, ok := m.ac.priorityHints["proja-blocker"]; !ok {
		t.Fatalf("expected proja-blocker recommendation to survive the proja filter")
	}
	for id := range m.ac.priorityHints {
		if got := ExtractRepoPrefix(id); got != "proja" {
			t.Fatalf("non-proja issue %q leaked into filtered priority hints", id)
		}
	}
}

// TestApplyFilterRecomputesPriorityHintsWhenShown covers the bt-gcuv
// acceptance criterion "changing project filter recalculates priority
// hints": applyFilter is the common path the repo picker (Enter) and the
// W (home/all) toggle both route through, so it must refresh
// m.ac.priorityHints whenever hints are currently displayed.
func TestApplyFilterRecomputesPriorityHintsWhenShown(t *testing.T) {
	var issues []model.Issue
	issues = append(issues, buildRecommendationFixture("proja")...)
	issues = append(issues, buildRecommendationFixture("projb")...)

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"proja-", "projb-"},
	})

	m.ac.showPriorityHints = true
	m.recomputePriorityHints() // seed with the global (unfiltered) computation
	if _, ok := m.ac.priorityHints["projb-blocker"]; !ok {
		t.Fatalf("expected seeded global hints to include projb-blocker")
	}

	// Changing the project filter through applyFilter (as the repo picker
	// and W toggle do) must recalculate the hints rather than leaving the
	// stale global-set recommendation on screen.
	m.activeRepos = map[string]bool{"proja": true}
	m.applyFilter()

	if _, ok := m.ac.priorityHints["projb-blocker"]; ok {
		t.Fatalf("priority hints were not recomputed after the project filter changed: %v", m.ac.priorityHints)
	}
	if _, ok := m.ac.priorityHints["proja-blocker"]; !ok {
		t.Fatalf("expected proja-blocker to remain after the filter change")
	}
}

// TestApplyFilterDoesNotRecomputePriorityHintsWhenHidden locks in the
// perf-conscious gating: applyFilter runs on many high-frequency actions
// (sort changes, label picker toggles, drilldowns), so it must not pay the
// synchronous Analyzer/GenerateRecommendations cost while the hints
// overlay isn't even visible.
func TestApplyFilterDoesNotRecomputePriorityHintsWhenHidden(t *testing.T) {
	issues := buildRecommendationFixture("proja")

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	m.ac.showPriorityHints = false
	sentinel := &analysis.PriorityRecommendation{IssueID: "sentinel"}
	m.ac.priorityHints = map[string]*analysis.PriorityRecommendation{"sentinel": sentinel}

	m.applyFilter()

	if got, ok := m.ac.priorityHints["sentinel"]; !ok || got != sentinel {
		t.Fatalf("applyFilter should not touch priority hints while hidden, got %v", m.ac.priorityHints)
	}
}

// TestPriorityHintsToggleUsesActiveRepoFilter is the end-to-end repro via
// the actual 'p' keypress (model_update_input.go), covering the exact user
// flow described in bt-gcuv: filter to one project in workspace mode, press
// p, expect arrows scoped to that project only.
func TestPriorityHintsToggleUsesActiveRepoFilter(t *testing.T) {
	var issues []model.Issue
	issues = append(issues, buildRecommendationFixture("proja")...)
	issues = append(issues, buildRecommendationFixture("projb")...)

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"proja-", "projb-"},
	})
	m.activeRepos = map[string]bool{"proja": true}

	updated2, _ := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated2.(Model)

	if !m.ac.showPriorityHints {
		t.Fatalf("expected priority hints toggled on")
	}
	if _, ok := m.ac.priorityHints["projb-blocker"]; ok {
		t.Fatalf("pressing p with a project filter active surfaced a cross-project hint: %v", m.ac.priorityHints)
	}
	if _, ok := m.ac.priorityHints["proja-blocker"]; !ok {
		t.Fatalf("expected proja-blocker hint when filtered to proja")
	}
}

func TestApplyFilterRespectsWorkspaceRepoFilter(t *testing.T) {
	issues := []model.Issue{
		{ID: "api-AUTH-1", Title: "API", Status: model.StatusOpen},
		{ID: "web-UI-1", Title: "Web", Status: model.StatusOpen},
	}

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"api-", "web-"},
	})

	// Filter to api only
	m.activeRepos = map[string]bool{"api": true}
	m.applyFilter()

	if got := len(m.list.Items()); got != 1 {
		t.Fatalf("expected 1 visible item after repo filter, got %d", got)
	}
	item, ok := m.list.Items()[0].(IssueItem)
	if !ok {
		t.Fatalf("expected IssueItem")
	}
	if item.Issue.ID != "api-AUTH-1" {
		t.Fatalf("expected api issue, got %s", item.Issue.ID)
	}

	// Clear repo filter (nil = all repos)
	m.activeRepos = nil
	m.applyFilter()
	if got := len(m.list.Items()); got != 2 {
		t.Fatalf("expected 2 visible items with no repo filter, got %d", got)
	}
}

// TestTreeViewRespectsActiveRepos asserts the tree view rebuilds from the
// activeRepos-filtered slice, matching the dogfood repro on bt-dcby.2:
// before the fix the tree showed all beads regardless of project filter.
func TestTreeViewRespectsActiveRepos(t *testing.T) {
	issues := []model.Issue{
		{ID: "api-AUTH-1", Title: "API auth", Status: model.StatusOpen},
		{ID: "api-AUTH-2", Title: "API token", Status: model.StatusOpen},
		{ID: "web-UI-1", Title: "Web UI", Status: model.StatusOpen},
	}

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"api-", "web-"},
	})

	// Enter tree mode so the helper actually rebuilds.
	m.mode = ViewTree
	m.activeRepos = map[string]bool{"api": true}
	m.rebuildTreeForCurrentFilter()

	if got := m.tree.NodeCount(); got != 2 {
		t.Fatalf("expected 2 visible tree nodes for api filter, got %d", got)
	}
	for i := 0; i < m.tree.NodeCount(); i++ {
		node := m.tree.flatList[i]
		if node == nil || node.Issue == nil {
			t.Fatalf("nil node/issue at index %d", i)
		}
		if got := IssueRepoKey(*node.Issue); got != "api" {
			t.Fatalf("non-api bead leaked into tree: id=%s repo=%s", node.Issue.ID, got)
		}
	}

	// Toggling activeRepos to nil should restore the full set on the next
	// applyFilter. This covers the user flow: open tree, toggle project filter
	// off via shortcut, expect tree to rebuild without re-pressing E.
	m.activeRepos = nil
	m.applyFilter()
	if got := m.tree.NodeCount(); got != 3 {
		t.Fatalf("expected 3 visible tree nodes after clearing activeRepos, got %d", got)
	}

	// Tightening to the web project should narrow the tree, again live.
	m.activeRepos = map[string]bool{"web": true}
	m.applyFilter()
	if got := m.tree.NodeCount(); got != 1 {
		t.Fatalf("expected 1 visible tree node after web filter, got %d", got)
	}
	if id := m.tree.flatList[0].Issue.ID; id != "web-UI-1" {
		t.Fatalf("expected web-UI-1, got %s", id)
	}
}

// TestRebuildTreeForCurrentFilterIsNoOpOutsideTreeMode asserts that the
// helper does not pay the build cost (or stomp tree state) when the active
// view is not the tree. Non-tree filter changes should not touch the tree.
func TestRebuildTreeForCurrentFilterIsNoOpOutsideTreeMode(t *testing.T) {
	issues := []model.Issue{
		{ID: "api-AUTH-1", Title: "API auth", Status: model.StatusOpen},
	}

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	// mode defaults to ViewList; tree was never built.
	if m.tree.IsBuilt() {
		t.Fatalf("tree should not be built before any rebuild call")
	}

	m.rebuildTreeForCurrentFilter()
	if m.tree.IsBuilt() {
		t.Errorf("tree should remain unbuilt outside ViewTree mode")
	}
}

// bt-dcby.3: the five surfaces below (actionable view, insights/triage,
// label health, label attention, cross-label flow) fed analysis from the
// unfiltered m.data.issues instead of the workspace-filtered set, the same
// root-cause shape bt-gcuv fixed for priority hints. Tests mirror the
// fixtures/patterns above.

// buildLabelFlowFixture returns two issues per project prefix with distinct,
// project-scoped labels and one cross-label blocking dependency, giving
// ComputeAllLabelHealth / ComputeLabelAttentionScores / ComputeCrossLabelFlow
// real per-project signal (label taxonomy, health counts, one cross-label
// dependency) to compute over.
func buildLabelFlowFixture(prefix string) []model.Issue {
	now := time.Now()
	blockerID := prefix + "-blocker"
	blockedID := prefix + "-blocked"
	return []model.Issue{
		{
			ID: blockerID, Title: prefix + " blocker", Status: model.StatusOpen,
			Priority: 2, Labels: []string{prefix + "-labelA"}, UpdatedAt: now,
		},
		{
			ID: blockedID, Title: prefix + " blocked", Status: model.StatusOpen,
			Priority: 2, Labels: []string{prefix + "-labelB"}, UpdatedAt: now,
			Dependencies: []*model.Dependency{
				{DependsOnID: blockerID, Type: model.DepBlocks},
			},
		},
	}
}

// TestActionableViewRespectsActiveRepos is the repro for surface 1
// (model_update_input.go's Actionable-view toggle): the execution plan must
// be built from filteredIssuesForActiveView(), not the full cross-project
// m.data.issues, so a projb-only item can never appear once activeRepos
// narrows the view to proja.
func TestActionableViewRespectsActiveRepos(t *testing.T) {
	var issues []model.Issue
	issues = append(issues, buildRecommendationFixture("proja")...)
	issues = append(issues, buildRecommendationFixture("projb")...)

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"proja-", "projb-"},
	})
	m.activeRepos = map[string]bool{"proja": true}

	updated2, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated2.(Model)

	if m.mode != ViewActionable {
		t.Fatalf("expected actionable view mode, got %v", m.mode)
	}
	found := false
	for _, track := range m.actionableView.plan.Tracks {
		for _, item := range track.Items {
			if ExtractRepoPrefix(item.ID) != "proja" {
				t.Fatalf("non-proja issue leaked into actionable plan: %s", item.ID)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("expected at least one proja item in the execution plan")
	}
}

// TestOpenInsightsViewRespectsActiveRepos is the repro for surface 2's
// openInsightsView call site ('i' toggle, model_update_input.go):
// TopPicks/Recommendations must be ranked from a fresh analyzer built over
// the workspace-filtered set, not the global analyzer/stats.
func TestOpenInsightsViewRespectsActiveRepos(t *testing.T) {
	var issues []model.Issue
	issues = append(issues, buildRecommendationFixture("proja")...)
	issues = append(issues, buildRecommendationFixture("projb")...)

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"proja-", "projb-"},
	})
	m.activeRepos = map[string]bool{"proja": true}

	updated2, _ := m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m = updated2.(Model)

	if m.mode != ViewInsights {
		t.Fatalf("expected insights view mode, got %v", m.mode)
	}
	for _, pick := range m.insightsPanel.topPicks {
		if ExtractRepoPrefix(pick.ID) != "proja" {
			t.Fatalf("non-proja top pick leaked into insights panel: %s", pick.ID)
		}
	}
	for _, rec := range m.insightsPanel.recommendations {
		if ExtractRepoPrefix(rec.ID) != "proja" {
			t.Fatalf("non-proja recommendation leaked into insights panel: %s", rec.ID)
		}
	}
}

// TestPhase2ReadyTriageBadgesRespectActiveRepos is the repro for surface 2's
// other call site (handlePhase2Ready, model_update_analysis.go): the triage
// scores/quick-win/blocker badges applied to list rows must be scoped to the
// workspace-filtered set, mirroring TestOpenInsightsViewRespectsActiveRepos
// so both entry points into the same triage data stay consistent.
func TestPhase2ReadyTriageBadgesRespectActiveRepos(t *testing.T) {
	var issues []model.Issue
	issues = append(issues, buildRecommendationFixture("proja")...)
	issues = append(issues, buildRecommendationFixture("projb")...)

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"proja-", "projb-"},
	})
	m.activeRepos = map[string]bool{"proja": true}

	ins := m.data.analysis.GenerateInsights(len(issues))
	updated2, _ := m.Update(Phase2ReadyMsg{Stats: m.data.analysis, Insights: ins})
	m = updated2.(Model)

	if len(m.ac.triageScores) == 0 {
		t.Fatalf("expected triage scores to be populated after Phase2Ready")
	}
	for id := range m.ac.triageScores {
		if ExtractRepoPrefix(id) != "proja" {
			t.Fatalf("non-proja issue leaked into triageScores: %s", id)
		}
	}
	for id := range m.ac.quickWinSet {
		if ExtractRepoPrefix(id) != "proja" {
			t.Fatalf("non-proja issue leaked into quickWinSet: %s", id)
		}
	}
	for id := range m.ac.blockerSet {
		if ExtractRepoPrefix(id) != "proja" {
			t.Fatalf("non-proja issue leaked into blockerSet: %s", id)
		}
	}
}

// TestLabelDashboardRespectsActiveRepos is the repro for surface 3
// (Label Health, '[' toggle, model_update_input.go): the label taxonomy and
// counts must come from the workspace-filtered set so a projb-only label
// can never appear once activeRepos narrows the view to proja.
func TestLabelDashboardRespectsActiveRepos(t *testing.T) {
	var issues []model.Issue
	issues = append(issues, buildLabelFlowFixture("proja")...)
	issues = append(issues, buildLabelFlowFixture("projb")...)

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"proja-", "projb-"},
	})
	m.activeRepos = map[string]bool{"proja": true}

	updated2, _ := m.Update(tea.KeyPressMsg{Code: '[', Text: "["})
	m = updated2.(Model)

	if m.mode != ViewLabelDashboard {
		t.Fatalf("expected label dashboard mode, got %v", m.mode)
	}
	if len(m.labelHealthCache.Labels) == 0 {
		t.Fatalf("expected label health data to be populated")
	}
	for _, lh := range m.labelHealthCache.Labels {
		if ExtractRepoPrefix(lh.Label) != "proja" {
			t.Fatalf("non-proja label leaked into label health dashboard: %s", lh.Label)
		}
		for _, issueID := range lh.Issues {
			if ExtractRepoPrefix(issueID) != "proja" {
				t.Fatalf("non-proja issue leaked into label health issues: %s", issueID)
			}
		}
	}
}

// TestAttentionViewRespectsActiveRepos is the repro for surface 4
// (Label Attention, ']' toggle, model_update_input.go).
func TestAttentionViewRespectsActiveRepos(t *testing.T) {
	var issues []model.Issue
	issues = append(issues, buildLabelFlowFixture("proja")...)
	issues = append(issues, buildLabelFlowFixture("projb")...)

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"proja-", "projb-"},
	})
	m.activeRepos = map[string]bool{"proja": true}

	updated2, _ := m.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	m = updated2.(Model)

	if m.mode != ViewAttention {
		t.Fatalf("expected attention view mode, got %v", m.mode)
	}
	if len(m.attentionCache.Labels) == 0 {
		t.Fatalf("expected attention scores to be populated")
	}
	for _, score := range m.attentionCache.Labels {
		if ExtractRepoPrefix(score.Label) != "proja" {
			t.Fatalf("non-proja label leaked into attention view: %s", score.Label)
		}
	}
}

// TestFlowMatrixRespectsActiveRepos is the repro for surface 5
// (Cross-Label Flow, 'f' toggle, model_update_input.go).
func TestFlowMatrixRespectsActiveRepos(t *testing.T) {
	var issues []model.Issue
	issues = append(issues, buildLabelFlowFixture("proja")...)
	issues = append(issues, buildLabelFlowFixture("projb")...)

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"proja-", "projb-"},
	})
	m.activeRepos = map[string]bool{"proja": true}

	updated2, _ := m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	m = updated2.(Model)

	if m.mode != ViewFlowMatrix {
		t.Fatalf("expected flow matrix mode, got %v", m.mode)
	}
	if m.flowMatrix.flow == nil {
		t.Fatalf("expected flow data to be populated")
	}
	for _, lbl := range m.flowMatrix.flow.Labels {
		if ExtractRepoPrefix(lbl) != "proja" {
			t.Fatalf("non-proja label leaked into flow matrix: %s", lbl)
		}
	}
	for _, dep := range m.flowMatrix.flow.Dependencies {
		if ExtractRepoPrefix(dep.FromLabel) != "proja" || ExtractRepoPrefix(dep.ToLabel) != "proja" {
			t.Fatalf("non-proja dependency leaked into flow matrix: %+v", dep)
		}
	}
}

// buildSharedLabelFlowFixture returns a bug-blocks-feature dependency pair
// per project, using the SAME label names ("bug"/"feature") across projects.
// getCrossFlowsForLabel's return type (labelFlowSummary) aggregates by label
// name with a plain count, not issue IDs, so a project-prefixed label (as in
// buildLabelFlowFixture) can never collide across projects and the leak
// would go undetected. Sharing the label name is what makes the aggregate
// count observably wrong when projb's contribution isn't excluded.
func buildSharedLabelFlowFixture(prefix string) []model.Issue {
	now := time.Now()
	bugID := prefix + "-bug-issue"
	featureID := prefix + "-feature-issue"
	return []model.Issue{
		{ID: bugID, Title: prefix + " bug", Status: model.StatusOpen, Priority: 2, Labels: []string{"bug"}, UpdatedAt: now},
		{
			ID: featureID, Title: prefix + " feature", Status: model.StatusOpen, Priority: 2, Labels: []string{"feature"}, UpdatedAt: now,
			Dependencies: []*model.Dependency{
				{DependsOnID: bugID, Type: model.DepBlocks},
			},
		},
	}
}

// incomingCountFor looks up the aggregate count for a source label within a
// labelFlowSummary's Incoming list (0 if absent).
func incomingCountFor(summary labelFlowSummary, label string) int {
	for _, c := range summary.Incoming {
		if c.Label == label {
			return c.Count
		}
	}
	return 0
}

// TestGetCrossFlowsForLabelRespectsActiveRepos is the repro for the second
// cross-label-flow call site, the label-drilldown helper getCrossFlowsForLabel
// (model.go), which is invoked independently of the 'f' toggle above.
func TestGetCrossFlowsForLabelRespectsActiveRepos(t *testing.T) {
	var issues []model.Issue
	issues = append(issues, buildSharedLabelFlowFixture("proja")...)
	issues = append(issues, buildSharedLabelFlowFixture("projb")...)

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"proja-", "projb-"},
	})

	// Sanity check: with no project filter, both projects' "bug" issues
	// block "feature", so the incoming count aggregates both contributions.
	summary := m.getCrossFlowsForLabel("feature")
	if got := incomingCountFor(summary, "bug"); got != 2 {
		t.Fatalf("expected aggregated incoming count 2 with no filter, got %d", got)
	}

	// bt-dcby.3: filtering to proja must scope the flow summary to proja's
	// own contribution only, not the cross-project aggregate.
	m.activeRepos = map[string]bool{"proja": true}
	summary = m.getCrossFlowsForLabel("feature")
	if got := incomingCountFor(summary, "bug"); got != 1 {
		t.Fatalf("expected incoming count scoped to 1 after proja filter, got %d (projb contribution leaked)", got)
	}
}
