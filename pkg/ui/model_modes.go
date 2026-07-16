package ui

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/correlation"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/projects"
)

// enumerateDoltDatabasesFn opens a connection to the shared Dolt server at
// dsn and returns the list of project databases. Package-level so unit tests
// in pkg/ui can swap a stub in (avoiding a live server dependency for the
// cursor-driven-global-dispatch coverage added in bt-ydjw.1).
//
// Returns nil on any error so callers treat enumeration failure the same as
// "no databases known" -- the gate caller's contract is to fall back to the
// polite empty-state when the resolver can't produce a validated DB name.
var enumerateDoltDatabasesFn = func(dsn string) []string {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil
	}
	defer db.Close()
	dbs, err := datasource.EnumerateDatabases(db, "")
	if err != nil {
		return nil
	}
	return dbs
}

// historyDispatchTarget reports whether the History view has a usable data
// source for repoPath under HistoryContext ctx, and (for global Dolt
// sources) which project database to target.
//
// Resolution priority for SourceTypeDoltGlobal:
//  1. ctx.CursorSourceRepo (issue.SourceRepo of the cursored bead - the
//     authoritative DB name in global mode; bt-ydjw.4)
//  2. ctx.CursorPrefix (bead ID prefix; matches DB name by convention only)
//  3. ctx.ActiveProjects[0] when exactly one project is active
//  4. m.currentProjectDB, the boot-time cwd-derived anchor
//
// Every candidate is validated against the shared server's enumerated
// database list before being accepted. This prevents silent wrong-DB
// dispatch when a bead ID prefix doesn't map to a Dolt DB name
// (bt-ydjw.1) and is also what catches the prefix/DB-name mismatch
// directly (bd-*/beads, mkt-*/marketplace, db-*/dev_browser) before
// CursorSourceRepo became the primary candidate.
//
// Returns (projectDB, ok=true) when dispatch is possible. The projectDB
// string is meaningful only for SourceTypeDoltGlobal; SourceTypeDolt and
// the JSONL-on-disk path return ("", true) because LoadHistoryCmd already
// knows how to reach the data without a name. All other configurations
// return ("", false) and the caller is expected to short-circuit to the
// polite empty-state.
//
// Single helper used by both enterHistoryView's gate and its dispatch
// site so the two cannot disagree (gate admits / dispatcher targets
// empty was the bt-ydjw.1 Case B failure mode).
func (m Model) historyDispatchTarget(ctx HistoryContext, repoPath string) (string, bool) {
	if correlation.HasJSONLOnDisk(repoPath) {
		return "", true
	}
	if m.data.dataSource == nil {
		return "", false
	}
	switch m.data.dataSource.Type {
	case datasource.SourceTypeDolt:
		// Single-repo DSN already pins a database; LoadHistoryCmd ignores the
		// projectDB argument in this case.
		return "", true
	case datasource.SourceTypeDoltGlobal:
		knownDBs := enumerateDoltDatabasesFn(m.data.dataSource.Path)
		if len(knownDBs) == 0 {
			return "", false
		}
		candidates := make([]string, 0, 4)
		// SourceRepo carries the authoritative DB name (e.g. "beads" for
		// bd-* beads); try it ahead of the ID prefix which only matches the
		// DB name by convention (bt-ydjw.4).
		if ctx.CursorSourceRepo != "" {
			candidates = append(candidates, ctx.CursorSourceRepo)
		}
		if ctx.CursorPrefix != "" {
			candidates = append(candidates, ctx.CursorPrefix)
		}
		if len(ctx.ActiveProjects) == 1 {
			candidates = append(candidates, ctx.ActiveProjects[0])
		}
		if m.currentProjectDB != "" {
			candidates = append(candidates, m.currentProjectDB)
		}
		for _, c := range candidates {
			for _, db := range knownDBs {
				if db == c {
					return db, true
				}
			}
		}
		return "", false
	}
	return "", false
}

// SetProjectName sets the display name for the current project (shown in footer).
func (m *Model) SetProjectName(name string) {
	m.projectName = name
}

// SetFilter sets the current filter and applies it (exposed for testing)
func (m *Model) SetFilter(f string) {
	m.filter.currentFilter = f
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
			m.workspaceSummary = fmt.Sprintf("%d/%d", info.RepoCount-info.FailedCount, info.RepoCount)
		} else {
			m.workspaceSummary = fmt.Sprintf("%d", info.RepoCount)
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

// historyContext builds the HistoryContext snapshot used by the History
// view's empty-state renderer (bt-ezk8). nil-safe via Model value receiver.
func (m Model) historyContext() HistoryContext {
	ctx := HistoryContext{WorkspaceMode: m.workspaceMode}
	if sel := m.list.SelectedItem(); sel != nil {
		if item, ok := sel.(IssueItem); ok {
			// Suffix may contain dots (e.g. bt-ugbp.1); split before first hyphen only.
			// Leave CursorPrefix at zero value when the ID lacks a hyphen - that's
			// the natural "no usable prefix" signal.
			if pfx, _, ok := strings.Cut(item.Issue.ID, "-"); ok {
				ctx.CursorPrefix = pfx
			}
			// SourceRepo is the actual Dolt database name in global mode; the
			// ID prefix is not (bd-* live in `beads`, mkt-* in `marketplace`).
			// Capture it as the authoritative dispatch candidate (bt-ydjw.4).
			ctx.CursorSourceRepo = item.Issue.SourceRepo
		}
	}
	if m.workspaceMode && m.activeRepos != nil {
		ctx.ActiveProjects = sortedRepoKeys(m.activeRepos)
	}
	return ctx
}

// enterHistoryView transitions to the History view in a loading state and
// dispatches the report generation as a tea.Cmd. The view shows a spinner
// immediately; HistoryLoadedMsg replaces it with the rendered chrome via
// handleHistoryLoaded when the report arrives.
//
// Path resolution (cursor prefix -> active project filter -> cwd, per
// bt-u8iz Phase 3) happens HERE on the main goroutine. The async closure
// (LoadHistoryCmd) cannot safely call os.Getwd or hit the projects registry
// because both depend on main-goroutine state.
//
// Returns nil only when path resolution itself fails synchronously; callers
// should fold the cmd into the surrounding tea.Batch.
func (m *Model) enterHistoryView() tea.Cmd {
	cwd, err := os.Getwd()
	if err != nil {
		m.setFailure("Cannot get working directory for history")
		return nil
	}

	ctx := m.historyContext()
	repoPath := resolveHistoryPath(ctx, cwd)

	// bt-ydjw.1: single call to historyDispatchTarget computes BOTH gate and
	// dispatch arguments, so the two paths cannot disagree. The phase-1
	// polite empty-state stays as a defensive fallback when the resolver
	// reports no validated path (no JSONL on disk AND no Dolt DataSource,
	// or a global Dolt source whose enumeration doesn't contain any of the
	// cursor / active-project / boot-anchor candidates).
	projectDB, ok := m.historyDispatchTarget(ctx, repoPath)
	if !ok {
		m.historyView = NewHistoryModel(nil, m.theme)
		m.historyView.SetContext(ctx)
		m.historyView.SetSize(m.width, m.height-1)
		m.historyDoltOnly = true
		m.historyLoading = false
		m.historyLoadFailed = false
		m.mode = ViewHistory
		m.focused = focusHistory
		m.setStatus("History: Dolt-only repo (commit correlator pending bt-08sh)")
		return nil
	}
	m.historyDoltOnly = false

	// Initialize a placeholder HistoryModel with the textinput properly
	// constructed. Without this the cursor (inside textinput) is nil and any
	// key handler that calls historyView.StartSearch crashes - the loading
	// screen path never used the historyView at all in the sync version, so
	// this gap only surfaced once we transitioned to ViewHistory before the
	// report was ready. handleHistoryLoaded replaces this stub with the real
	// HistoryModel when HistoryLoadedMsg arrives.
	m.historyView = NewHistoryModel(nil, m.theme)
	m.historyView.SetContext(ctx)
	m.historyView.SetSize(m.width, m.height-1)

	m.historyLoading = true
	m.historyLoadFailed = false
	m.mode = ViewHistory
	m.focused = focusHistory

	m.setStatus("Loading history...")

	return LoadHistoryCmd(repoPath, m.data.beadsPath, m.issuesForAsync(), m.data.dataSource, projectDB)
}

// resolveHistoryPath implements the cursor -> filter -> cwd priority order.
// Returns cwd as the safe fallback that preserves bt-ezk8 behavior when
// the registry has nothing useful to say.
func resolveHistoryPath(ctx HistoryContext, cwd string) string {
	if ctx.CursorPrefix != "" {
		if path, ok := projects.LookupAndValidate(ctx.CursorPrefix); ok {
			return path
		}
	}
	if len(ctx.ActiveProjects) == 1 {
		if path, ok := projects.LookupAndValidate(ctx.ActiveProjects[0]); ok {
			return path
		}
	}
	return cwd
}

// enterMemoriesView transitions to the Memories view in a loading state and
// dispatches discovery+aggregation as a tea.Cmd (bt-2ea7t.4). Mirrors
// enterHistoryView's async shape: the view shows a loading screen
// immediately; MemoriesLoadedMsg replaces it with the rendered master/detail
// surface via handleMemoriesLoaded when the aggregate arrives.
//
// repoRoot resolution (resolveHistoryRepoPath) happens HERE, on the main
// goroutine - same reasoning as enterHistoryView: the async closure
// (LoadMemoriesCmd) must not call os.Getwd itself.
func (m *Model) enterMemoriesView() tea.Cmd {
	repoRoot := resolveHistoryRepoPath(m.data.beadsPath)

	m.memories = NewMemoriesModel(m.theme)
	m.memories.SetSize(m.bodyWidth(), m.height-1)

	m.memoriesLoading = true
	m.mode = ViewMemories
	m.focused = focusMemories

	m.setStatus("Loading memories...")

	return LoadMemoriesCmd(repoRoot)
}

// enterTimeTravelMode loads historical data and computes diff
func (m *Model) enterTimeTravelMode(revision string) {
	cwd, err := os.Getwd()
	if err != nil {
		m.setFailure(activeGlyphs.Cross + " Time-travel failed: cannot get working directory")
		return
	}

	gitLoader := loader.NewGitLoader(cwd)

	// Check if we're in a git repo first
	if _, err := gitLoader.ResolveRevision("HEAD"); err != nil {
		m.setFailure(activeGlyphs.Cross + " Time-travel requires a git repository")
		return
	}

	// Check if beads files exist at the revision
	hasBeads, err := gitLoader.HasBeadsAtRevision(revision)
	if err != nil || !hasBeads {
		m.setFailure(fmt.Sprintf("%s No beads history at %s (try fewer commits back)", activeGlyphs.Cross, revision))
		return
	}

	// Load historical issues
	historicalIssues, err := gitLoader.LoadAt(revision)
	if err != nil {
		m.setFailure(fmt.Sprintf("%s Time-travel failed: %v", activeGlyphs.Cross, err))
		return
	}

	// Create snapshots and compute diff
	fromSnapshot := analysis.NewSnapshot(historicalIssues)
	toSnapshot := analysis.NewSnapshot(m.data.issues)
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
	m.setStatus(fmt.Sprintf("%s Time-travel: comparing with %s (+%d %s%d ~%d)",
		activeGlyphs.Clock, revision, diff.Summary.IssuesAdded, activeGlyphs.Success, diff.Summary.IssuesClosed, diff.Summary.IssuesModified))

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
	m.setStatus(activeGlyphs.Clock + " Time-travel mode disabled")

	// Rebuild list without diff info
	m.rebuildListWithDiffInfo()
}

// rebuildListWithDiffInfo recreates list items with current diff state.
// Dispatches through reapplyActiveFilter so recipe, BQL, and plain
// status/label filters are all honored (bt-k9f6f) instead of only recipe
// vs. plain applyFilter().
func (m *Model) rebuildListWithDiffInfo() {
	m.reapplyActiveFilter()
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
	case focusClaimConfirm:
		return "claim_confirm"
	case focusTimeTravelInput:
		return "time_travel_input"
	case focusHistory:
		return "history"
	case focusAttention:
		return "attention"
	case focusLabelPicker:
		return "label_picker"
	case focusEpics:
		return "epics"
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
	case focusMemories:
		return "memories"
	default:
		return "unknown"
	}
}

// IsBoardView returns true if the board view is active (bv-5e5q).
func (m Model) IsBoardView() bool {
	return m.mode == ViewBoard
}

// IsGraphView returns true if the graph view is active (bv-5e5q).
func (m Model) IsGraphView() bool {
	return m.mode == ViewGraph
}

// IsActionableView returns true if the actionable view is active (bv-5e5q).
func (m Model) IsActionableView() bool {
	return m.mode == ViewActionable
}

// IsHistoryView returns true if the history view is active (bv-5e5q).
func (m Model) IsHistoryView() bool {
	return m.mode == ViewHistory
}

// IsEpicsView returns true if the epics overview is active.
func (m Model) IsEpicsView() bool {
	return m.mode == ViewEpics
}

// IsAttentionView returns true if the attention view is active.
func (m Model) IsAttentionView() bool {
	return m.mode == ViewAttention
}

// ShowDetails returns true if the detail pane is visible.
// Stable accessor for Phase 1 refactor - this field will move into PaneFocus state.
func (m Model) ShowDetails() bool {
	return m.showDetails
}

// CurrentFilter returns the active filter string (e.g. "open", "closed", "ready",
// "label:X", "bql:..."). Stable accessor for Phase 1 refactor - this field will
// move into FilterState.
func (m Model) CurrentFilter() string {
	return m.filter.currentFilter
}

// Issues returns the full (unfiltered) issue slice. Stable accessor for Phase 1
// refactor - this field will move into DataState.
func (m Model) Issues() []model.Issue {
	return m.data.issues
}

// toggleFullscreenPane drives the two-stage per-pane focus/fullscreen model
// (bt-566fk, refining bt-530vn). In split view a number key first focuses its
// pane; only a second press (pane already focused) maximizes it, and a third
// press exits back to the focused split. Pressing the *other* pane's key while
// one is fullscreen exits fullscreen and focuses that pane in the restored
// split - it needs its own second press to maximize. One consistent rule: a
// number always focuses its pane before it maximizes it.
//
// At narrow width there is no split, so the single visible pane already fills
// the body and the focus stage has nothing to distinguish. There the key keeps
// the direct btop-style toggle: press to maximize, the other key swaps straight
// over, re-press exits. So this composes with isSplitView (bt-9a3wv's
// width-driven auto-collapse) rather than being orthogonal to it: the wide case
// two-stages, the narrow case collapses to a direct toggle.
func (m *Model) toggleFullscreenPane(target fullscreenPane) {
	// Re-pressing the maximized pane's key exits fullscreen, keeping focus on
	// that pane (mirrors the Esc/q dispatcher cascade). Applies at any width.
	if m.fullscreen == target {
		m.exitFullscreenPane()
		return
	}

	// Narrow width (no split): the lone visible pane already fills the body, so
	// "focus" has no second pane to read against. Keep the direct toggle -
	// press the target to maximize it, and pressing the other key while one is
	// maximized swaps straight over (bt-566fk Q3).
	if !m.isSplitView {
		m.fullscreen = target
		m.focused = focusForPane(target)
		m.refreshFullscreenLayout()
		return
	}

	// Split view, a *different* pane is fullscreen: exit fullscreen and focus
	// the target in the restored split. A second press then maximizes it
	// (bt-566fk Q1 - a number focuses first, even coming out of a fullscreen).
	if m.fullscreen != fullscreenNone {
		m.focused = focusForPane(target)
		m.exitFullscreenPane()
		return
	}

	// Split view, nothing fullscreen. Stage 1: focus the target if it isn't
	// already focused - both panes stay visible, only the highlighted border
	// moves, so no resize/content refresh is needed (View() re-reads m.focused).
	if m.focused != focusForPane(target) {
		m.focused = focusForPane(target)
		return
	}

	// Stage 2: target already focused -> maximize it.
	m.fullscreen = target
	m.refreshFullscreenLayout()
}

// focusForPane maps a fullscreenPane target to the focus value for that pane.
func focusForPane(target fullscreenPane) focus {
	if target == fullscreenDetails {
		return focusDetail
	}
	return focusList
}

// exitFullscreenPane restores the width-driven split/single-pane layout.
// Called both by re-pressing the active pane's key and by the Esc/q
// dispatcher cascade (model_update_input.go).
func (m *Model) exitFullscreenPane() {
	m.fullscreen = fullscreenNone
	m.refreshFullscreenLayout()
}

// refreshFullscreenLayout resizes list/viewport for the current fullscreen
// state (or the restored split/single layout on exit) and immediately
// re-renders the detail markdown at the new width. Mirrors
// recalculateSplitPaneSizes' on-demand (non-debounced) refresh: a single
// deliberate toggle doesn't need the resize-drag debounce bt-kfkrb exists
// for. Also stamps lastHeavyWidth so the next real WindowSizeMsg's
// applyWindowSizeHeavy short-circuit doesn't skip a render this toggle
// already performed under a different (fullscreen-vs-not) branch at the same
// bodyW.
func (m *Model) refreshFullscreenLayout() {
	bodyW := m.bodyWidth()
	bodyHeight := m.height - 1
	if bodyHeight < 5 {
		bodyHeight = 5
	}
	m.applyListDetailSizing(bodyW, bodyHeight)
	m.renderer.SetWidthWithTheme(m.viewport.Width(), m.theme)
	m.updateViewportContent()
	m.lastHeavyWidth = bodyW
}
