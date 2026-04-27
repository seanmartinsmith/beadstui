package ui

// model_update_data.go contains Update() handlers for data lifecycle messages.
// Extracted from the main Update() switch to keep the router thin.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/debug"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// handleSnapshotReady processes a new snapshot from the background worker.
func (m Model) handleSnapshotReady(msg SnapshotReadyMsg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Background worker has a new snapshot ready (bv-m7v8)
	// This is the atomic pointer swap - O(1), sub-microsecond
	if msg.Snapshot == nil {
		if m.data.backgroundWorker != nil {
			return m, WaitForBackgroundWorkerMsgCmd(m.data.backgroundWorker)
		}
		return m, nil
	}

	// Defer re-render while an interactive modal is open (bt-cl2m). Re-emit the
	// same message after a short tick so the snapshot isn't dropped — when the
	// modal closes, the next firing processes it. The first non-deferred snapshot
	// (bootstrap) must always proceed so the TUI can render initial content.
	if m.data.snapshot != nil && m.shouldDeferRefresh() {
		deferred := msg
		cmds = append(cmds, tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg {
			return deferred
		}))
		return m, tea.Batch(cmds...)
	}

	firstSnapshot := m.data.snapshotInitPending && m.data.snapshot == nil
	m.data.snapshotInitPending = false
	wasTimeTravel := m.timeTravelMode

	// Clear ephemeral overlays tied to old data
	m.clearAttentionOverlay()

	// Exit time-travel mode if active (file changed, show current state)
	if m.timeTravelMode {
		m.timeTravelMode = false
		m.timeTravelDiff = nil
		m.timeTravelSince = ""
		m.newIssueIDs = nil
		m.closedIssueIDs = nil
		m.modifiedIssueIDs = nil
	}

	// Store selected issue ID to restore position after swap
	var selectedID string
	if sel := m.list.SelectedItem(); sel != nil {
		if item, ok := sel.(IssueItem); ok {
			selectedID = item.Issue.ID
		}
	}

	// Filter state is preserved inside setListItems (bt-nzsy).

	// Preserve board selection by issue ID (bv-6n4c).
	var boardSelectedID string
	if m.focused == focusBoard {
		if sel := m.board.SelectedIssue(); sel != nil {
			boardSelectedID = sel.ID
		}
	}

	oldSnapshot := m.data.snapshot

	// Swap snapshot pointer
	m.data.snapshot = msg.Snapshot
	if m.data.backgroundWorker != nil {
		latencyStart := msg.FileChangeAt
		if latencyStart.IsZero() {
			latencyStart = msg.SentAt
		}
		if !latencyStart.IsZero() {
			m.data.backgroundWorker.recordUIUpdateLatency(time.Since(latencyStart))
		}
	}
	if oldSnapshot != nil && len(oldSnapshot.pooledIssues) > 0 {
		go loader.ReturnIssuePtrsToPool(oldSnapshot.pooledIssues)
	}

	// bt-d5wr: emit activity events from the snapshot diff.
	// Gated on: (a) not the bootstrap snapshot (no prior to diff against),
	// (b) oldSnapshot is non-nil, (c) ring buffer is initialized,
	// (d) this handler call did NOT begin in time-travel mode (diffing a
	// historical snapshot against a live one produces spurious events).
	if !firstSnapshot && oldSnapshot != nil && m.events != nil && !wasTimeTravel {
		diff := events.Diff(oldSnapshot.Issues, msg.Snapshot.Issues, time.Now(), events.SourceDolt)
		if len(diff) > 0 {
			m.events.AppendMany(diff)
		}
	}

	// Update legacy fields for backwards compatibility during migration
	// Eventually these will be removed when all code reads from snapshot
	m.data.issues = msg.Snapshot.Issues
	m.data.issueMap = msg.Snapshot.IssueMap
	m.data.analyzer = msg.Snapshot.Analyzer
	m.data.analysis = msg.Snapshot.Analysis
	m.ac.countOpen = msg.Snapshot.CountOpen
	m.ac.countReady = msg.Snapshot.CountReady
	m.ac.countBlocked = msg.Snapshot.CountBlocked
	m.ac.countClosed = msg.Snapshot.CountClosed
	if len(m.data.pooledIssues) > 0 {
		go loader.ReturnIssuePtrsToPool(m.data.pooledIssues)
		m.data.pooledIssues = nil
	}
	// Preserve existing triage data unless the snapshot has Phase 2 results.
	// Avoid flicker when Phase 1 snapshots arrive without triage data.
	if msg.Snapshot.Phase2Ready || len(msg.Snapshot.TriageScores) > 0 {
		m.ac.triageScores = msg.Snapshot.TriageScores
		m.ac.triageReasons = msg.Snapshot.TriageReasons
		m.ac.unblocksMap = msg.Snapshot.UnblocksMap
		m.ac.quickWinSet = msg.Snapshot.QuickWinSet
		m.ac.blockerSet = msg.Snapshot.BlockerSet
	}

	// Clear caches that need recomputation
	m.labelHealthCached = false
	m.attentionCached = false
	m.ac.priorityHints = make(map[string]*analysis.PriorityRecommendation)
	m.labelDrilldownCache = make(map[string][]model.Issue)

	// Recompute alerts for refreshed dataset
	m.alerts, m.alertsCritical, m.alertsWarning, m.alertsInfo = computeAlerts(m.data.issues, m.workspaceMode)
	m.dismissedAlerts = make(map[string]bool)

	// Clamp modal cursors if their lists shrank (bt-46p6.10). The shared
	// alerts/notifications modal stays open across refreshes so live updates
	// are visible; cursors that now point past the end get clamped to the
	// last valid row.
	if n := len(m.visibleAlerts()); m.alertsCursor >= n {
		m.alertsCursor = n - 1
	}
	if m.alertsCursor < 0 {
		m.alertsCursor = 0
	}
	if n := len(m.visibleNotifications()); m.notificationsCursor >= n {
		m.notificationsCursor = n - 1
	}
	if m.notificationsCursor < 0 {
		m.notificationsCursor = 0
	}

	// Reset semantic caches for the new dataset.
	if m.semanticSearch != nil {
		m.semanticSearch.ResetCache()
		m.semanticSearch.SetMetricsCache(nil)
	}
	m.semanticHybridReady = false
	m.semanticHybridBuilding = false
	if m.semanticHybridEnabled {
		m.semanticHybridBuilding = true
		cmds = append(cmds, BuildHybridMetricsCmd(m.issuesForAsync()))
	}

	// Regenerate sub-views (Phase 1 data; Phase 2 will update via Phase2ReadyMsg)
	m.insightsPanel.SetInsights(m.data.snapshot.Insights)
	m.insightsPanel.issueMap = m.data.issueMap
	bodyHeight := m.height - 1
	if bodyHeight < 5 {
		bodyHeight = 5
	}
	m.insightsPanel.SetSize(m.width, bodyHeight)

	// Update list/board/graph views while preserving the current recipe/filter state.
	if m.filter.activeRecipe != nil {
		// If the snapshot already includes recipe filtering/sorting, use it directly (bv-cwwd).
		if msg.Snapshot.RecipeName == m.filter.activeRecipe.Name && msg.Snapshot.RecipeHash == recipeFingerprint(m.filter.activeRecipe) {
			filteredItems := make([]list.Item, 0, len(msg.Snapshot.ListItems))
			filteredIssues := make([]model.Issue, 0, len(msg.Snapshot.ListItems))

			for _, item := range msg.Snapshot.ListItems {
				issue := item.Issue

				// Workspace repo filter (nil = all repos). Must use
				// IssueRepoKey so the lookup matches activeRepos keys
				// (workspace DB names) when those differ from the bead
				// ID prefix — e.g. DB "marketplace", IDs "mkt-xxx".
				// Bare item.RepoPrefix is ID-derived only and would
				// silently nuke the whole filtered view (bt-ci7b).
				if m.workspaceMode && m.activeRepos != nil {
					repoKey := IssueRepoKey(issue)
					if repoKey != "" && !m.activeRepos[repoKey] {
						continue
					}
				}

				filteredItems = append(filteredItems, item)
				filteredIssues = append(filteredIssues, issue)
			}

			m.setListItems(filteredItems)
			m.updateSemanticIDs(filteredItems)
			m.board.SetIssues(filteredIssues)

			recipeIns := analysis.Insights{}
			if m.data.analysis != nil {
				recipeIns = m.data.analysis.GenerateInsights(len(filteredIssues))
			}
			m.graphView.SetIssues(filteredIssues, &recipeIns)

			m.filter.currentFilter = "recipe:" + m.filter.activeRecipe.Name

			// Keep selection in bounds
			if len(filteredItems) > 0 && m.list.Index() >= len(filteredItems) {
				m.list.Select(0)
			}
		} else {
			m.applyRecipe(m.filter.activeRecipe)
		}
	} else {
		var filteredItems []list.Item
		var filteredIssues []model.Issue

		filteredItems = make([]list.Item, 0, len(msg.Snapshot.ListItems))
		filteredIssues = make([]model.Issue, 0, len(msg.Snapshot.ListItems))

		for _, item := range msg.Snapshot.ListItems {
			issue := item.Issue

			// Workspace repo filter (nil = all repos). See bt-ci7b: the
			// recipe-mode branch above has the same fix. IssueRepoKey
			// honors issue.SourceRepo first so DB-name vs ID-prefix
			// divergence (marketplace ↔ mkt) doesn't drop every item.
			if m.workspaceMode && m.activeRepos != nil {
				repoKey := IssueRepoKey(issue)
				if repoKey != "" && !m.activeRepos[repoKey] {
					continue
				}
			}

			include := false
			switch m.filter.currentFilter {
			case "all":
				include = true
			case "open":
				include = !isClosedLikeStatus(issue.Status)
			case "closed":
				include = isClosedLikeStatus(issue.Status)
			case "ready":
				// Ready = Open/InProgress AND NO Open Blockers
				if !isClosedLikeStatus(issue.Status) && issue.Status != model.StatusBlocked {
					isBlocked := false
					for _, dep := range issue.Dependencies {
						if dep == nil || !dep.Type.IsBlocking() {
							continue
						}
						if blocker, exists := m.data.issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
							isBlocked = true
							break
						}
					}
					include = !isBlocked
				}
			default:
				// Legacy: label: prefix in currentFilter
				if strings.HasPrefix(m.filter.currentFilter, "label:") {
					lf := strings.TrimPrefix(m.filter.currentFilter, "label:")
					include = matchesLabelFilter(issue, lf)
				}
			}

			// Independent label filter (composes with status)
			if include && m.filter.labelFilter != "" {
				include = matchesLabelFilter(issue, m.filter.labelFilter)
			}

			if include {
				filteredItems = append(filteredItems, item)
				filteredIssues = append(filteredIssues, issue)
			}
		}

		m.sortFilteredItems(filteredItems, filteredIssues)
		m.setListItems(filteredItems)
		m.updateSemanticIDs(filteredItems)
		if m.data.snapshot != nil && m.data.snapshot.BoardState != nil && (!m.workspaceMode || m.activeRepos == nil) && len(filteredIssues) == len(m.data.snapshot.Issues) {
			m.board.SetSnapshot(m.data.snapshot)
		} else {
			m.board.SetIssues(filteredIssues)
		}
		if m.data.snapshot != nil && m.data.snapshot.GraphLayout != nil && len(filteredIssues) == len(m.data.snapshot.Issues) {
			m.graphView.SetSnapshot(m.data.snapshot)
		} else {
			m.graphView.SetIssues(filteredIssues, &m.data.snapshot.Insights)
		}

		// Restore selection by ID against the visible (possibly filtered) view.
		// Indexing into the unfiltered set would drive Paginator.Page out of
		// bounds when the filter narrows results (bt-nzsy follow-up).
		if selectedID != "" {
			for i, it := range m.list.VisibleItems() {
				if item, ok := it.(IssueItem); ok && item.Issue.ID == selectedID {
					m.list.Select(i)
					break
				}
			}
		}

		// Keep selection in bounds
		if visible := m.list.VisibleItems(); len(visible) > 0 && m.list.Index() >= len(visible) {
			m.list.Select(0)
		}
	}

	// Restore selection in recipe mode (applyRecipe rebuilds list items)
	if m.filter.activeRecipe != nil && selectedID != "" {
		for i, it := range m.list.VisibleItems() {
			if item, ok := it.(IssueItem); ok && item.Issue.ID == selectedID {
				m.list.Select(i)
				break
			}
		}
	}

	// Restore board selection after SetIssues/applyRecipe rebuilds columns (bv-6n4c).
	if boardSelectedID != "" {
		_ = m.board.SelectIssueByID(boardSelectedID)
	}

	// If the tree view is active, rebuild it from the new snapshot while preserving
	// user state (selection + persisted expand/collapse) (bv-6n4c).
	if m.focused == focusTree {
		m.tree.BuildFromSnapshot(m.data.snapshot)
		m.tree.SetSize(m.width, m.height-2)
	}

	// Refresh detail pane if visible
	if m.isSplitView || m.showDetails {
		m.updateViewportContent()
	}

	// Keep semantic index current when enabled.
	if m.semanticSearchEnabled && !m.semanticIndexBuilding {
		m.semanticIndexBuilding = true
		cmds = append(cmds, BuildSemanticIndexCmd(m.issuesForAsync()))
	}

	// Reload sprints (bv-161)
	if m.data.beadsPath != "" {
		beadsDir := filepath.Dir(m.data.beadsPath)
		if loaded, err := loader.LoadSprintsFromFile(filepath.Join(beadsDir, loader.SprintsFileName)); err == nil {
			m.sprints = loaded
			// If we have a selected sprint, try to refresh it
			if m.selectedSprint != nil {
				found := false
				for i := range m.sprints {
					if m.sprints[i].ID == m.selectedSprint.ID {
						m.selectedSprint = &m.sprints[i]
						m.sprintViewText = m.renderSprintDashboard()
						found = true
						break
					}
				}
				if !found {
					m.selectedSprint = nil
					m.sprintViewText = "Sprint not found"
				}
			}
		}
	}

	if firstSnapshot {
		// For the initial background snapshot, avoid flashing "Reloaded" at startup.
		if msg.Snapshot.LoadWarningCount > 0 {
			cmds = append(cmds, m.setInlineTransientStatus(
				fmt.Sprintf("Loaded %d issues (%d warnings)", len(m.data.issues), msg.Snapshot.LoadWarningCount), 3*time.Second))
		} else {
			m.statusMsg = ""
		}
	} else if msg.Snapshot.LoadWarningCount > 0 {
		cmds = append(cmds, m.setInlineTransientStatus(
			fmt.Sprintf("Reloaded %d issues (%d warnings)", len(m.data.issues), msg.Snapshot.LoadWarningCount), 3*time.Second))
	} else {
		cmds = append(cmds, m.setInlineTransientStatus(
			fmt.Sprintf("Reloaded %d issues", len(m.data.issues)), 3*time.Second))
	}

	// Wait for Phase 2 if not ready
	if msg.Snapshot.Analysis != nil {
		cmds = append(cmds, WaitForPhase2Cmd(msg.Snapshot.Analysis))
	}

	if m.data.backgroundWorker != nil {
		cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.data.backgroundWorker))
	}

	return m, tea.Batch(cmds...)
}

// handleSnapshotError processes a snapshot loading error.
func (m Model) handleSnapshotError(msg SnapshotErrorMsg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Background worker encountered an error loading/processing data
	// If recoverable, we'll try again on next file change.
	if m.data.snapshotInitPending && m.data.snapshot == nil {
		m.data.snapshotInitPending = false
	}
	if msg.Err != nil {
		if msg.Recoverable {
			m.statusMsg = fmt.Sprintf("Reload error (retrying): %s", shortError(msg.Err))
		} else {
			m.statusMsg = fmt.Sprintf("Reload error: %s", shortError(msg.Err))
		}
		m.statusIsError = true
	}
	if m.data.backgroundWorker != nil {
		cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.data.backgroundWorker))
	}
	return m, tea.Batch(cmds...)
}

// handleDataSourceReload processes an async reload from a non-file datasource.
func (m Model) handleDataSourceReload(msg DataSourceReloadMsg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Async reload from a non-file datasource (e.g. Dolt) completed.
	if msg.Err != nil {
		m.statusMsg = fmt.Sprintf("Reload failed: %s", shortError(msg.Err))
		m.statusIsError = true
		return m, tea.Batch(cmds...)
	}

	// Defer the re-render while an interactive modal is open (bt-cl2m). Re-emit
	// the message so the data isn't dropped; it lands the next time we tick.
	if m.shouldDeferRefresh() {
		deferred := msg
		cmds = append(cmds, tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg {
			return deferred
		}))
		return m, tea.Batch(cmds...)
	}

	// Filter state is preserved inside setListItems (bt-nzsy).
	m.replaceIssues(msg.Issues)

	cmds = append(cmds, m.setInlineTransientStatus(
		fmt.Sprintf("Reloaded %d issues", len(msg.Issues)), 3*time.Second))
	cmds = append(cmds, WaitForPhase2Cmd(m.data.analysis))
	return m, tea.Batch(cmds...)
}

// handleDoltVerified processes a successful Dolt poll.
func (m Model) handleDoltVerified(msg DoltVerifiedMsg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Dolt poll succeeded - data is verified current (bt-3ynd).
	m.lastDoltVerified = msg.At
	m.doltConnected = true
	if m.data.backgroundWorker != nil {
		cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.data.backgroundWorker))
	}
	return m, tea.Batch(cmds...)
}

// handleDoltConnectionStatus processes Dolt connectivity changes.
func (m Model) handleDoltConnectionStatus(msg DoltConnectionStatusMsg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Dolt poll loop reporting connectivity change (bv-1p3a).
	if msg.Connected {
		m.doltConnected = true
		m.statusMsg = "Dolt server reconnected"
		m.statusIsError = false
	} else {
		m.doltConnected = false
		m.statusMsg = fmt.Sprintf("Dolt server unreachable (retrying in %ds)", msg.BackoffSeconds)
		m.statusIsError = true
	}
	if m.data.backgroundWorker != nil {
		cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.data.backgroundWorker))
	}
	return m, tea.Batch(cmds...)
}

// handleTemporalCacheReady processes temporal cache population completion.
func (m Model) handleTemporalCacheReady(msg TemporalCacheReadyMsg) (Model, tea.Cmd) {
	_ = msg
	// Temporal cache populated - future phases (sparklines, diff, timeline)
	// will use this to refresh their views. For now, just acknowledge.
	if m.data.backgroundWorker != nil {
		return m, WaitForBackgroundWorkerMsgCmd(m.data.backgroundWorker)
	}
	return m, nil
}

// handleFileChanged processes file-system change notifications.
func (m Model) handleFileChanged(msg FileChangedMsg) (Model, tea.Cmd) {
	_ = msg // msg has no fields, just a signal
	var cmds []tea.Cmd

	// File changed on disk - reload issues and recompute analysis
	// In background mode the BackgroundWorker owns file watching and snapshot building.
	if m.data.backgroundWorker != nil {
		if m.data.watcher != nil {
			cmds = append(cmds, WatchFileCmd(m.data.watcher))
		}
		return m, tea.Batch(cmds...)
	}
	if m.data.beadsPath == "" {
		// Re-start watch for next change
		if m.data.watcher != nil {
			cmds = append(cmds, WatchFileCmd(m.data.watcher))
		}
		return m, tea.Batch(cmds...)
	}

	// Defer the synchronous reload while an interactive modal is open (bt-cl2m).
	// Re-emit FileChangedMsg after a short tick — when the modal closes, the
	// reload will re-run. The watcher is restarted so subsequent file changes
	// continue to be detected.
	if m.shouldDeferRefresh() {
		cmds = append(cmds, tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg {
			return FileChangedMsg{}
		}))
		if m.data.watcher != nil {
			cmds = append(cmds, WatchFileCmd(m.data.watcher))
		}
		return m, tea.Batch(cmds...)
	}
	reloadStart := time.Now()
	profileRefresh := debug.Enabled()
	var refreshTimings map[string]time.Duration
	recordTiming := func(name string, d time.Duration) {
		if !profileRefresh {
			return
		}
		if refreshTimings == nil {
			refreshTimings = make(map[string]time.Duration, 12)
		}
		refreshTimings[name] = d
		debug.LogTiming("refresh."+name, d)
	}
	if profileRefresh {
		debug.Log("refresh: file change detected path=%s", m.data.beadsPath)
	}

	// Clear ephemeral overlays tied to old data
	m.clearAttentionOverlay()

	// Exit time-travel mode if active (file changed, show current state)
	if m.timeTravelMode {
		m.timeTravelMode = false
		m.timeTravelDiff = nil
		m.timeTravelSince = ""
		m.newIssueIDs = nil
		m.closedIssueIDs = nil
		m.modifiedIssueIDs = nil
	}

	// Reload issues from disk
	// Use custom warning handler to prevent stderr pollution during TUI render (bv-fix)
	var reloadWarnings []string
	var loadStart time.Time
	if profileRefresh {
		loadStart = time.Now()
	}
	loadedIssues, err := loader.LoadIssuesFromFileWithOptionsPooled(m.data.beadsPath, loader.ParseOptions{
		WarningHandler: func(msg string) {
			reloadWarnings = append(reloadWarnings, msg)
		},
		BufferSize: envMaxLineSizeBytes(),
	})
	if profileRefresh {
		recordTiming("load_issues", time.Since(loadStart))
	}
	if err != nil {
		m.statusMsg = fmt.Sprintf("Reload error: %v", err)
		m.statusIsError = true
		// Re-start watch for next change
		if m.data.watcher != nil {
			cmds = append(cmds, WatchFileCmd(m.data.watcher))
		}
		return m, tea.Batch(cmds...)
	}
	if len(m.data.pooledIssues) > 0 {
		loader.ReturnIssuePtrsToPool(m.data.pooledIssues)
	}
	m.data.pooledIssues = loadedIssues.PoolRefs
	newIssues := loadedIssues.Issues

	// Store selected issue ID to restore position after reload
	var selectedID string
	if sel := m.list.SelectedItem(); sel != nil {
		if item, ok := sel.(IssueItem); ok {
			selectedID = item.Issue.ID
		}
	}

	// Filter state is preserved inside setListItems (bt-nzsy).

	// Apply default sorting (Open first, Priority, Date)
	var sortStart time.Time
	if profileRefresh {
		sortStart = time.Now()
	}
	sort.Slice(newIssues, func(i, j int) bool {
		iClosed := isClosedLikeStatus(newIssues[i].Status)
		jClosed := isClosedLikeStatus(newIssues[j].Status)
		if iClosed != jClosed {
			return !iClosed
		}
		if newIssues[i].Priority != newIssues[j].Priority {
			return newIssues[i].Priority < newIssues[j].Priority
		}
		return newIssues[i].CreatedAt.After(newIssues[j].CreatedAt)
	})
	if profileRefresh {
		recordTiming("sort_issues", time.Since(sortStart))
	}

	// Recompute analysis (async Phase 1/Phase 2) with caching
	m.data.issues = newIssues
	var analysisStart time.Time
	if profileRefresh {
		analysisStart = time.Now()
	}
	cachedAnalyzer := analysis.NewCachedAnalyzer(newIssues, nil)
	m.data.analyzer = cachedAnalyzer.Analyzer
	m.data.analysis = cachedAnalyzer.AnalyzeAsync(context.Background())
	cacheHit := cachedAnalyzer.WasCacheHit()
	if profileRefresh {
		recordTiming("phase1_setup", time.Since(analysisStart))
		debug.Log("refresh.phase1_cache_hit=%t issues=%d", cacheHit, len(newIssues))
	}
	m.labelHealthCached = false
	m.attentionCached = false

	// Rebuild lookup map
	var mapStart time.Time
	if profileRefresh {
		mapStart = time.Now()
	}
	m.data.issueMap = make(map[string]*model.Issue, len(newIssues))
	for i := range m.data.issues {
		m.data.issueMap[m.data.issues[i].ID] = &m.data.issues[i]
	}
	if profileRefresh {
		recordTiming("issue_map", time.Since(mapStart))
	}

	// Clear stale priority hints (will be repopulated after Phase 2)
	m.ac.priorityHints = make(map[string]*analysis.PriorityRecommendation)

	// Recompute stats
	var statsStart time.Time
	if profileRefresh {
		statsStart = time.Now()
	}
	m.ac.countOpen, m.ac.countReady, m.ac.countBlocked, m.ac.countClosed = 0, 0, 0, 0
	for i := range m.data.issues {
		issue := &m.data.issues[i]
		if isClosedLikeStatus(issue.Status) {
			m.ac.countClosed++
			continue
		}
		m.ac.countOpen++
		if issue.Status == model.StatusBlocked {
			m.ac.countBlocked++
			continue
		}
		isBlocked := false
		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			if blocker, exists := m.data.issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
				isBlocked = true
				break
			}
		}
		if !isBlocked {
			m.ac.countReady++
		}
	}
	if profileRefresh {
		recordTiming("counts", time.Since(statsStart))
	}

	// Recompute alerts for refreshed dataset
	var alertsStart time.Time
	if profileRefresh {
		alertsStart = time.Now()
	}
	m.alerts, m.alertsCritical, m.alertsWarning, m.alertsInfo = computeAlerts(m.data.issues, m.workspaceMode)
	if profileRefresh {
		recordTiming("alerts", time.Since(alertsStart))
	}
	m.dismissedAlerts = make(map[string]bool)

	// Keep the shared alerts/notifications modal open across reloads so live
	// updates are visible (bt-46p6.10); clamp cursors if lists shrank.
	if n := len(m.visibleAlerts()); m.alertsCursor >= n {
		m.alertsCursor = n - 1
	}
	if m.alertsCursor < 0 {
		m.alertsCursor = 0
	}
	if n := len(m.visibleNotifications()); m.notificationsCursor >= n {
		m.notificationsCursor = n - 1
	}
	if m.notificationsCursor < 0 {
		m.notificationsCursor = 0
	}

	// Rebuild list items (preserve triage data to avoid flicker)
	var listStart time.Time
	if profileRefresh {
		listStart = time.Now()
	}
	items := make([]list.Item, len(m.data.issues))
	for i := range m.data.issues {
		item := IssueItem{
			Issue:      m.data.issues[i],
			GraphScore: m.data.analysis.GetPageRankScore(m.data.issues[i].ID),
			Impact:     m.data.analysis.GetCriticalPathScore(m.data.issues[i].ID),
			RepoPrefix: ExtractRepoPrefix(m.data.issues[i].ID),
		}
		item.TriageScore = m.ac.triageScores[m.data.issues[i].ID]
		if reasons, exists := m.ac.triageReasons[m.data.issues[i].ID]; exists {
			item.TriageReason = reasons.Primary
			item.TriageReasons = reasons.All
		}
		item.IsQuickWin = m.ac.quickWinSet[m.data.issues[i].ID]
		item.IsBlocker = m.ac.blockerSet[m.data.issues[i].ID]
		item.UnblocksCount = len(m.ac.unblocksMap[m.data.issues[i].ID])
		if m.data.issues[i].IssueType == model.TypeEpic {
			item.EpicDone, item.EpicTotal = epicProgress(m.data.issues[i].ID, m.data.issues)
		}
		item.GateAwaitType = gateAwaitFromBlockers(m.data.issues[i], m.data.issueMap)
		items[i] = item
	}
	if profileRefresh {
		recordTiming("list_items", time.Since(listStart))
	}
	m.updateSemanticIDs(items)
	m.clearSemanticScores()
	if m.semanticSearch != nil {
		m.semanticSearch.ResetCache()
		m.semanticSearch.SetMetricsCache(nil)
	}
	m.semanticHybridReady = false
	m.semanticHybridBuilding = false
	if m.semanticHybridEnabled {
		m.semanticHybridBuilding = true
		cmds = append(cmds, BuildHybridMetricsCmd(m.issuesForAsync()))
	}
	m.setListItems(items)

	// Restore selection by ID against the visible (possibly filtered) view.
	if selectedID != "" {
		for i, item := range m.list.VisibleItems() {
			if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selectedID {
				m.list.Select(i)
				break
			}
		}
	}

	// Regenerate sub-views (with Phase 1 data; Phase 2 will update via Phase2ReadyMsg)
	// Preserve triage data already computed to avoid UI flicker.
	needsInsights := m.mode == ViewInsights
	needsGraph := m.mode == ViewGraph
	var ins analysis.Insights
	if needsInsights || needsGraph {
		var insightsStart time.Time
		if profileRefresh {
			insightsStart = time.Now()
		}
		ins = m.data.analysis.GenerateInsights(len(m.data.issues))
		if profileRefresh {
			recordTiming("insights_generate", time.Since(insightsStart))
		}
	}
	if needsInsights {
		oldTopPicks := m.insightsPanel.topPicks
		oldRecs := m.insightsPanel.recommendations
		oldRecMap := m.insightsPanel.recommendationMap
		oldHash := m.insightsPanel.triageDataHash

		m.insightsPanel = NewInsightsModel(ins, m.data.issueMap, m.theme)
		m.insightsPanel.topPicks = oldTopPicks
		m.insightsPanel.recommendations = oldRecs
		m.insightsPanel.recommendationMap = oldRecMap
		m.insightsPanel.triageDataHash = oldHash
		bodyHeight := m.height - 1
		if bodyHeight < 5 {
			bodyHeight = 5
		}
		m.insightsPanel.SetSize(m.width, bodyHeight)
	}
	if m.mode == ViewAttention {
		var attentionStart time.Time
		if profileRefresh {
			attentionStart = time.Now()
		}
		cfg := analysis.DefaultLabelHealthConfig()
		m.attentionCache = analysis.ComputeLabelAttentionScores(m.data.issues, cfg, time.Now().UTC())
		m.attentionCached = true
		attText, _ := ComputeAttentionView(m.data.issues, max(40, m.width-4))
		m.insightsPanel = NewInsightsModel(analysis.Insights{}, m.data.issueMap, m.theme)
		m.insightsPanel.labelAttention = m.attentionCache.Labels
		m.insightsPanel.extraText = attText
		panelHeight := m.height - 2
		if panelHeight < 3 {
			panelHeight = 3
		}
		m.insightsPanel.SetSize(m.width, panelHeight)
		if profileRefresh {
			recordTiming("attention_view", time.Since(attentionStart))
		}
	}
	if needsGraph || m.mode == ViewBoard {
		var graphStart time.Time
		if profileRefresh {
			graphStart = time.Now()
		}
		m.refreshBoardAndGraphForCurrentFilter()
		if profileRefresh {
			recordTiming("board_graph", time.Since(graphStart))
		}
	}

	// Re-apply recipe filter if active
	if m.filter.activeRecipe != nil {
		m.applyRecipe(m.filter.activeRecipe)
	}

	// Re-apply BQL filter if active
	if m.filter.activeBQLExpr != nil && strings.HasPrefix(m.filter.currentFilter, "bql:") {
		queryStr := strings.TrimPrefix(m.filter.currentFilter, "bql:")
		m.applyBQL(m.filter.activeBQLExpr, queryStr)
	}

	// Reload sprints (bv-161)
	if m.data.beadsPath != "" {
		beadsDir := filepath.Dir(m.data.beadsPath)
		if loaded, err := loader.LoadSprintsFromFile(filepath.Join(beadsDir, loader.SprintsFileName)); err == nil {
			m.sprints = loaded
			// If we have a selected sprint, try to refresh it
			if m.selectedSprint != nil {
				found := false
				for i := range m.sprints {
					if m.sprints[i].ID == m.selectedSprint.ID {
						m.selectedSprint = &m.sprints[i]
						m.sprintViewText = m.renderSprintDashboard()
						found = true
						break
					}
				}
				if !found {
					m.selectedSprint = nil
					m.sprintViewText = "Sprint not found"
				}
			}
		}
	}

	// Keep semantic index current when enabled.
	if m.semanticSearchEnabled && !m.semanticIndexBuilding {
		m.semanticIndexBuilding = true
		cmds = append(cmds, BuildSemanticIndexCmd(m.issuesForAsync()))
	}

	if cacheHit {
		m.statusMsg = fmt.Sprintf("Reloaded %d issues (cached)", len(newIssues))
	} else {
		m.statusMsg = fmt.Sprintf("Reloaded %d issues", len(newIssues))
	}
	if len(reloadWarnings) > 0 {
		m.statusMsg += fmt.Sprintf(" (%d warnings)", len(reloadWarnings))
	}
	reloadDuration := time.Since(reloadStart)
	if profileRefresh {
		recordTiming("total", reloadDuration)
	}
	if reloadDuration >= 500*time.Millisecond {
		m.statusMsg += fmt.Sprintf(" in %s", formatReloadDuration(reloadDuration))
	}
	if profileRefresh && len(refreshTimings) > 0 {
		addTiming := func(label, key string) {
			if d, ok := refreshTimings[key]; ok && d > 0 {
				m.statusMsg += fmt.Sprintf(" %s=%s", label, formatReloadDuration(d))
			}
		}
		m.statusMsg += " [debug"
		addTiming("load", "load_issues")
		addTiming("sort", "sort_issues")
		addTiming("phase1", "phase1_setup")
		addTiming("alerts", "alerts")
		addTiming("list", "list_items")
		addTiming("graph", "board_graph")
		addTiming("total", "total")
		m.statusMsg += "]"
	}
	// Auto-enable background mode after slow sync reloads (opt-out via BT_BACKGROUND_MODE=0).
	autoEnabled := false
	slowReload := reloadDuration >= time.Second
	if slowReload && m.data.backgroundWorker == nil && m.data.beadsPath != "" {
		autoAllowed := true
		if v := strings.TrimSpace(os.Getenv("BT_BACKGROUND_MODE")); v != "" {
			switch strings.ToLower(v) {
			case "0", "false", "no", "off":
				autoAllowed = false
			}
		}
		if autoAllowed {
			autoBeadsDir := ""
			if m.data.beadsPath != "" {
				autoBeadsDir = filepath.Dir(m.data.beadsPath)
			}
			bw, err := NewBackgroundWorker(WorkerConfig{
				BeadsPath:     m.data.beadsPath,
				BeadsDir:      autoBeadsDir,
				DataSource:    m.data.dataSource,
				DebounceDelay: 200 * time.Millisecond,
			})
			if err == nil {
				if m.data.watcher != nil {
					m.data.watcher.Stop()
				}
				m.data.watcher = nil
				m.data.backgroundWorker = bw
				m.data.snapshotInitPending = true
				autoEnabled = true
				cmds = append(cmds, StartBackgroundWorkerCmd(m.data.backgroundWorker))
				cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.data.backgroundWorker))
			} else {
				m.statusMsg += fmt.Sprintf("; background mode unavailable: %v", err)
			}
		}
	}
	if slowReload {
		if autoEnabled {
			m.statusMsg += "; background mode auto-enabled"
		} else {
			m.statusMsg += "; consider BT_BACKGROUND_MODE=1"
		}
	}
	m.statusIsError = false
	m.statusIsInline = true // background-initiated reload — don't clobber hints (bt-y0k7)
	// Schedule auto-clear of the reload status message
	m.statusSeq++
	seq := m.statusSeq
	cmds = append(cmds, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return statusClearMsg{seq: seq}
	}))
	// Invalidate label-derived caches
	m.labelHealthCached = false
	m.labelDrilldownCache = make(map[string][]model.Issue)
	m.updateViewportContent()

	// Re-start watching for next change + wait for Phase 2
	if m.data.watcher != nil && !autoEnabled {
		cmds = append(cmds, WatchFileCmd(m.data.watcher))
	}
	cmds = append(cmds, WaitForPhase2Cmd(m.data.analysis))
	return m, tea.Batch(cmds...)
}
