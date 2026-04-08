package ui

import (
	"fmt"
	"os"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/correlation"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// SetProjectName sets the display name for the current project (shown in footer).
func (m *Model) SetProjectName(name string) {
	m.projectName = name
}

// SetFilter sets the current filter and applies it (exposed for testing)
func (m *Model) SetFilter(f string) {
	m.currentFilter = f
	m.applyFilter()
}

// FilteredIssues returns the currently visible issues (exposed for testing)
func (m Model) FilteredIssues() []model.Issue {
	items := m.list.Items()
	issues := make([]model.Issue, 0, len(items))
	for _, item := range items {
		if issueItem, ok := item.(IssueItem); ok {
			issues = append(issues, issueItem.Issue)
		}
	}
	return issues
}

// EnableWorkspaceMode configures the model for workspace (multi-repo) view
func (m *Model) EnableWorkspaceMode(info WorkspaceInfo) {
	m.workspaceMode = info.Enabled
	m.availableRepos = normalizeRepoPrefixes(info.RepoPrefixes)
	m.activeRepos = nil // nil means all repos are active

	if info.RepoCount > 0 {
		if info.FailedCount > 0 {
			m.workspaceSummary = fmt.Sprintf("%d/%d projects", info.RepoCount-info.FailedCount, info.RepoCount)
		} else {
			m.workspaceSummary = fmt.Sprintf("%d projects", info.RepoCount)
		}
	}

	// Update delegate to show repo badges
	m.updateListDelegate()
}

// IsWorkspaceMode returns whether workspace mode is active
func (m Model) IsWorkspaceMode() bool {
	return m.workspaceMode
}

// SetCurrentProjectDB records the auto-detected project DB name for the W toggle.
func (m *Model) SetCurrentProjectDB(db string) {
	m.currentProjectDB = db
}

// SetActiveRepos sets the active repo filter. nil means all repos visible.
func (m *Model) SetActiveRepos(repos map[string]bool) {
	m.activeRepos = repos
}

// enterHistoryView loads correlation data and shows the history view
func (m *Model) enterHistoryView() {
	cwd, err := os.Getwd()
	if err != nil {
		m.statusMsg = "Cannot get working directory for history"
		m.statusIsError = true
		return
	}

	// Convert model.Issue to correlation.BeadInfo
	beads := make([]correlation.BeadInfo, len(m.issues))
	for i, issue := range m.issues {
		beads[i] = correlation.BeadInfo{
			ID:     issue.ID,
			Title:  issue.Title,
			Status: string(issue.Status),
		}
	}

	// Load correlation data
	correlator := correlation.NewCorrelator(cwd, m.beadsPath)
	opts := correlation.CorrelatorOptions{
		Limit: 500, // Reasonable limit for TUI performance
	}

	report, err := correlator.GenerateReport(beads, opts)
	if err != nil {
		m.statusMsg = fmt.Sprintf("History load failed: %v", err)
		m.statusIsError = true
		return
	}

	// Initialize or update history view
	m.historyView = NewHistoryModel(report, m.theme)
	m.historyView.SetSize(m.width, m.height-1)
	m.isHistoryView = true
	m.focused = focusHistory

	m.statusMsg = fmt.Sprintf("Loaded history: %d beads with commits", report.Stats.BeadsWithCommits)
	m.statusIsError = false
}

// enterTimeTravelMode loads historical data and computes diff
func (m *Model) enterTimeTravelMode(revision string) {
	cwd, err := os.Getwd()
	if err != nil {
		m.statusMsg = "❌ Time-travel failed: cannot get working directory"
		m.statusIsError = true
		return
	}

	gitLoader := loader.NewGitLoader(cwd)

	// Check if we're in a git repo first
	if _, err := gitLoader.ResolveRevision("HEAD"); err != nil {
		m.statusMsg = "❌ Time-travel requires a git repository"
		m.statusIsError = true
		return
	}

	// Check if beads files exist at the revision
	hasBeads, err := gitLoader.HasBeadsAtRevision(revision)
	if err != nil || !hasBeads {
		m.statusMsg = fmt.Sprintf("❌ No beads history at %s (try fewer commits back)", revision)
		m.statusIsError = true
		return
	}

	// Load historical issues
	historicalIssues, err := gitLoader.LoadAt(revision)
	if err != nil {
		m.statusMsg = fmt.Sprintf("❌ Time-travel failed: %v", err)
		m.statusIsError = true
		return
	}

	// Create snapshots and compute diff
	fromSnapshot := analysis.NewSnapshot(historicalIssues)
	toSnapshot := analysis.NewSnapshot(m.issues)
	diff := analysis.CompareSnapshots(fromSnapshot, toSnapshot)

	// Build lookup sets for badges
	m.newIssueIDs = make(map[string]bool)
	for _, issue := range diff.NewIssues {
		m.newIssueIDs[issue.ID] = true
	}

	m.closedIssueIDs = make(map[string]bool)
	for _, issue := range diff.ClosedIssues {
		m.closedIssueIDs[issue.ID] = true
	}

	m.modifiedIssueIDs = make(map[string]bool)
	for _, mod := range diff.ModifiedIssues {
		m.modifiedIssueIDs[mod.IssueID] = true
	}

	m.timeTravelMode = true
	m.timeTravelDiff = diff
	m.timeTravelSince = revision

	// Success feedback
	m.statusMsg = fmt.Sprintf("⏱️ Time-travel: comparing with %s (+%d ✅%d ~%d)",
		revision, diff.Summary.IssuesAdded, diff.Summary.IssuesClosed, diff.Summary.IssuesModified)
	m.statusIsError = false

	// Rebuild list items with diff info
	m.rebuildListWithDiffInfo()
}

// exitTimeTravelMode clears time-travel state
func (m *Model) exitTimeTravelMode() {
	m.timeTravelMode = false
	m.timeTravelDiff = nil
	m.timeTravelSince = ""
	m.newIssueIDs = nil
	m.closedIssueIDs = nil
	m.modifiedIssueIDs = nil

	// Feedback
	m.statusMsg = "⏱️ Time-travel mode disabled"
	m.statusIsError = false

	// Rebuild list without diff info
	m.rebuildListWithDiffInfo()
}

// rebuildListWithDiffInfo recreates list items with current diff state
func (m *Model) rebuildListWithDiffInfo() {
	if m.activeRecipe != nil {
		m.applyRecipe(m.activeRecipe)
	} else {
		m.applyFilter()
	}
}

// IsTimeTravelMode returns whether time-travel mode is active
func (m Model) IsTimeTravelMode() bool {
	return m.timeTravelMode
}

// TimeTravelDiff returns the current diff (nil if not in time-travel mode)
func (m Model) TimeTravelDiff() *analysis.SnapshotDiff {
	return m.timeTravelDiff
}

// FocusState returns the current focus state as a string for testing (bv-5e5q).
// This enables testing focus transitions without exposing the internal focus type.
func (m Model) FocusState() string {
	switch m.focused {
	case focusList:
		return "list"
	case focusDetail:
		return "detail"
	case focusBoard:
		return "board"
	case focusGraph:
		return "graph"
	case focusTree:
		return "tree"
	case focusLabelDashboard:
		return "label_dashboard"
	case focusInsights:
		return "insights"
	case focusActionable:
		return "actionable"
	case focusRecipePicker:
		return "recipe_picker"
	case focusRepoPicker:
		return "repo_picker"
	case focusHelp:
		return "help"
	case focusQuitConfirm:
		return "quit_confirm"
	case focusTimeTravelInput:
		return "time_travel_input"
	case focusHistory:
		return "history"
	case focusAttention:
		return "attention"
	case focusLabelPicker:
		return "label_picker"
	case focusSprint:
		return "sprint"
	case focusAgentPrompt:
		return "agent_prompt"
	case focusFlowMatrix:
		return "flow_matrix"
	case focusTutorial:
		return "tutorial"
	case focusCassModal:
		return "cass_modal"
	case focusUpdateModal:
		return "update_modal"
	default:
		return "unknown"
	}
}

// IsBoardView returns true if the board view is active (bv-5e5q).
func (m Model) IsBoardView() bool {
	return m.isBoardView
}

// IsGraphView returns true if the graph view is active (bv-5e5q).
func (m Model) IsGraphView() bool {
	return m.isGraphView
}

// IsActionableView returns true if the actionable view is active (bv-5e5q).
func (m Model) IsActionableView() bool {
	return m.isActionableView
}

// IsHistoryView returns true if the history view is active (bv-5e5q).
func (m Model) IsHistoryView() bool {
	return m.isHistoryView
}
