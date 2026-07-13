package ui

// model_update_analysis.go contains Update() handlers for analysis, worker,
// and system messages. Extracted from the main Update() switch to keep the
// router thin.

import (
	"fmt"
	"sort"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/debug"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// handleUpdateMsg processes a version update notification.
//
// In addition to flagging the footer ⭐ badge state, this routes the
// "update available" signal into the notifications ring buffer so it
// participates in the existing dismiss/scrollback affordances (bt-9u39).
// The detail-pane "Update Available" inline block was removed in the
// same change; the notification + footer badge now carry the signal.
func (m Model) handleUpdateMsg(msg UpdateMsg) Model {
	already := m.updateAvailable && m.updateTag == msg.TagName
	m.updateAvailable = true
	m.updateTag = msg.TagName
	m.updateURL = msg.URL
	if !already && m.events != nil && msg.TagName != "" {
		now := time.Now()
		title := fmt.Sprintf("%s available — run `bt update`", msg.TagName)
		summary := msg.URL
		if summary == "" {
			summary = "Run `bt update` to install the latest release."
		}
		ev := events.Event{
			ID:      fmt.Sprintf("update-%s", msg.TagName),
			Kind:    events.EventSystem,
			Title:   title,
			Summary: summary,
			At:      now,
			Source:  events.SourceDolt,
		}
		m.events.Append(ev)
	}
	return m
}

// handleUpdateCompleteMsg forwards completion to the update modal.
func (m Model) handleUpdateCompleteMsg(msg UpdateCompleteMsg) (Model, tea.Cmd) {
	if m.activeModal == ModalUpdate {
		var cmd tea.Cmd
		m.updateModal, cmd = m.updateModal.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleUpdateProgressMsg forwards progress to the update modal.
func (m Model) handleUpdateProgressMsg(msg UpdateProgressMsg) (Model, tea.Cmd) {
	if m.activeModal == ModalUpdate {
		var cmd tea.Cmd
		m.updateModal, cmd = m.updateModal.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleStatusClear clears the status message if no newer status has been set.
func (m Model) handleStatusClear(msg statusClearMsg) Model {
	if msg.seq == m.statusSeq {
		m.statusMsg = ""
		m.statusSeverity = SeverityNone
		m.statusIsInline = false
	}
	return m
}

// handleStatusTick auto-dismisses stale non-error status messages and re-arms
// the tick. Runs on a 1s cadence so idle sessions still clear expired status
// (bt-m9te, bt-y0k7).
func (m Model) handleStatusTick(_ statusTickMsg) (Model, tea.Cmd) {
	if m.statusMsg != "" && m.statusSeverity != SeverityDegraded {
		age := statusDismissAge(m.statusSeverity)
		if m.statusSetAt.IsZero() {
			m.statusSetAt = time.Now()
		} else if age > 0 && time.Since(m.statusSetAt) > age {
			m.statusMsg = ""
			m.statusSeverity = SeverityNone
			m.statusIsInline = false
		}
	}
	return m, statusTickCmd()
}

// handleSemanticIndexReady processes the semantic index build completion.
// Returns (Model, tea.Cmd, done). If done is true, caller should return immediately.
func (m Model) handleSemanticIndexReady(msg SemanticIndexReadyMsg) (Model, tea.Cmd, bool) {
	semIdxStart := time.Now()
	defer func() { debug.LogTiming("handleSemanticIndexReady.total", time.Since(semIdxStart)) }()
	m.semanticIndexBuilding = false
	if msg.Error != nil {
		m.semanticSearchEnabled = false
		m.list.Filter = fuzzySearchFilter()
		m.setFailure(fmt.Sprintf("Semantic search unavailable: %v", msg.Error))
		return m, nil, false
	}
	if m.semanticSearch != nil {
		m.semanticSearch.SetIndex(msg.Index, msg.Embedder)
	}
	// Background-sync re-emissions: only surface a status when something
	// actually changed (built or updated). The no-change "up to date" path
	// is the steady state — every periodic refresh hits it and re-setting
	// the message keeps the timer alive forever, defeating the 5s
	// auto-dismiss intent (bt-14wc). Silent success is the right default
	// for invisible background work; the footer mode badge already shows
	// the active state.
	if !msg.Loaded {
		m.setStatus(fmt.Sprintf("Semantic index built (%d embedded)", msg.Stats.Embedded))
	} else if msg.Stats.Changed() {
		m.setStatus(fmt.Sprintf("Semantic index updated (+%d ~%d -%d)", msg.Stats.Added, msg.Stats.Updated, msg.Stats.Removed))
	}

	if m.semanticSearchEnabled && m.list.FilterState() != list.Unfiltered {
		prevState := m.list.FilterState()
		filterText := m.list.FilterInput.Value()
		m.list.SetFilterText(filterText)
		if prevState == list.Filtering {
			m.list.SetFilterState(list.Filtering)
		}
	}
	return m, nil, false
}

// handleHybridMetricsReady processes hybrid search metrics build completion.
func (m Model) handleHybridMetricsReady(msg HybridMetricsReadyMsg) (Model, tea.Cmd) {
	m.semanticHybridBuilding = false
	if msg.Error != nil {
		m.semanticHybridEnabled = false
		m.semanticHybridReady = false
		if m.semanticSearch != nil {
			m.semanticSearch.SetMetricsCache(nil)
			m.semanticSearch.SetHybridConfig(false, m.semanticHybridPreset)
		}
		m.setFailure(fmt.Sprintf("Hybrid search unavailable: %v", msg.Error))
		return m, nil
	}
	if m.semanticSearch != nil && msg.Cache != nil {
		m.semanticSearch.SetMetricsCache(msg.Cache)
	}
	m.semanticHybridReady = msg.Cache != nil
	m.setStatus(fmt.Sprintf("Hybrid search ready (%s)", m.semanticHybridPreset))

	if m.semanticHybridEnabled && m.semanticSearchEnabled && m.list.FilterState() != list.Unfiltered {
		currentTerm := m.list.FilterInput.Value()
		if currentTerm != "" {
			m.semanticSearch.ResetCache()
			return m, ComputeSemanticFilterCmd(m.semanticSearch, currentTerm)
		}
	}
	return m, nil
}

// handleSemanticFilterResult processes async semantic filter results.
func (m Model) handleSemanticFilterResult(msg SemanticFilterResultMsg) Model {
	if m.semanticSearch != nil && msg.Results != nil {
		// SetCachedResults rejects results whose snapshot version is stale.
		// If rejected, skip the SetFilterText follow-up — re-running with
		// stale ranks would either no-op (cache miss) or re-apply old scores.
		preVersion := m.semanticSearch.Snapshot().Version
		m.semanticSearch.SetCachedResults(msg.Term, msg.Results, msg.Version)
		if msg.Version != preVersion {
			return m
		}

		currentTerm := m.list.FilterInput.Value()
		if m.semanticSearchEnabled && currentTerm == msg.Term {
			m.applySemanticScores(msg.Term)
			prevState := m.list.FilterState()
			m.list.SetFilterText(currentTerm)
			if prevState == list.Filtering {
				m.list.SetFilterState(list.Filtering)
			}
		}
	}
	return m
}

// handleSemanticDebounceTick checks if semantic computation should trigger.
// Returns (Model, tea.Cmd, done). If done is true, caller should return.
func (m Model) handleSemanticDebounceTick() (Model, tea.Cmd, bool) {
	if m.semanticSearchEnabled && m.semanticSearch != nil && m.list.FilterState() != list.Unfiltered {
		pendingTerm := m.semanticSearch.GetPendingTerm()
		if pendingTerm != "" && time.Since(m.semanticSearch.GetLastQueryTime()) >= 150*time.Millisecond {
			return m, ComputeSemanticFilterCmd(m.semanticSearch, pendingTerm), true
		}
	}
	return m, nil, false
}

// handleWorkerPollTick updates the worker spinner animation and maintains the
// coalesced "refreshing" display window (bt-uq3i3). While the worker processes
// past workerSpinnerFlashThreshold it (re)extends the window so consecutive
// short refresh cycles render one steady indicator rather than a sub-second
// on/off flash loop; the window is read by workerSpinnerVisible().
//
// The 120ms chain runs only while the worker is processing or the display
// window lingers. When idle it goes DORMANT (no reschedule) and is re-armed by
// RefreshStartedMsg from the worker - a perpetual tick at this cadence cost
// ~6.6% of a core at 1300 issues even when fully idle (bt-2ubez).
func (m Model) handleWorkerPollTick() (Model, tea.Cmd) {
	w := m.data.backgroundWorker
	if w == nil {
		m.data.workerTickArmed = false
		return m, nil
	}
	state := w.State()
	if state == WorkerStopped {
		m.data.workerTickArmed = false
		return m, nil
	}
	if state == WorkerProcessing {
		m.data.workerSpinnerIdx = (m.data.workerSpinnerIdx + 1) % len(workerSpinnerFrames)
		if w.ProcessingDuration() >= workerSpinnerFlashThreshold {
			m.data.workerSpinnerVisibleUntil = time.Now().Add(workerSpinnerMinDisplay)
		}
		m.data.workerTickArmed = true
		return m, workerPollTickCmd()
	}
	// Idle: keep animating only while the coalesced display window lingers.
	m.data.workerSpinnerIdx = 0
	if time.Now().Before(m.data.workerSpinnerVisibleUntil) {
		m.data.workerTickArmed = true
		return m, workerPollTickCmd()
	}
	m.data.workerTickArmed = false
	return m, nil
}

// handlePhase2Ready processes async graph analysis Phase 2 completion.
func (m Model) handlePhase2Ready(msg Phase2ReadyMsg) (Model, tea.Cmd) {
	phase2Start := time.Now()
	defer func() { debug.LogTiming("handlePhase2Ready.total", time.Since(phase2Start)) }()

	// Ignore stale Phase2 completions (from before a file reload)
	if msg.Stats != m.data.analysis {
		return m, nil
	}

	// Multiple call sites dispatch WaitForPhase2Cmd against the same stats
	// pointer (Init + handleSnapshotReady at minimum). Both deliver
	// Phase2ReadyMsg with msg.Stats == m.data.analysis, and historically both
	// re-ran the full O(N) triage / recommendations / alerts pipeline. Once
	// the snapshot is marked Phase2Ready we have already processed this stats
	// pointer; subsequent identical messages are no-ops (bt-kfkrb).
	if m.data.snapshot != nil && m.data.snapshot.Phase2Ready {
		return m, nil
	}

	// Mark snapshot as Phase 2 ready
	if m.data.snapshot != nil {
		m.data.snapshot.Phase2Ready = true
	}

	// Phase 2 analysis complete - update insights with full data
	ins := msg.Insights
	if m.data.snapshot != nil {
		m.data.snapshot.Insights = ins
	}
	insightsStart := time.Now()
	m.insightsPanel.SetInsights(ins)
	m.insightsPanel.issueMap = m.data.issueMap
	bodyHeight := m.height - 1
	if bodyHeight < 5 {
		bodyHeight = 5
	}
	m.insightsPanel.SetSize(m.width, bodyHeight)
	debug.LogTiming("phase2.insightsPanel.setup", time.Since(insightsStart))

	graphStart := time.Now()
	if m.data.snapshot != nil {
		if m.data.snapshot.GraphLayout != nil {
			m.data.snapshot.GraphLayout.UpdatePhase2Ranks(msg.Stats)
		}
		m.graphView.SetSnapshot(m.data.snapshot)
	} else {
		m.graphView.SetIssues(m.data.issues, &ins)
	}
	debug.LogTiming("phase2.graphView.setup", time.Since(graphStart))

	// Generate triage for priority panel, scoped to the active workspace
	// repo filter (bt-dcby.3) rather than the full cross-project corpus.
	// Reusing the global analyzer/stats here wouldn't scope the result -
	// TopPicks/Recommendations/QuickWins are ranked from the analyzer's own
	// issueMap, not the trailing issues slice - so a fresh analyzer must be
	// built over the workspace-filtered set (mirrors bt-gcuv's
	// recomputePriorityHints; workspacePrefilter rather than
	// filteredIssuesForActiveView so these list-row badges survive
	// status/label filter toggles, matching openInsightsView's twin call).
	triageStart := time.Now()
	triage := analysis.ComputeTriageWithOptions(m.workspacePrefilter(m.data.issues), analysis.TriageOptions{WaitForPhase2: true})
	debug.LogTiming("phase2.ComputeTriageFromAnalyzer", time.Since(triageStart))
	triageScores := make(map[string]float64, len(triage.Recommendations))
	triageReasons := make(map[string]analysis.TriageReasons, len(triage.Recommendations))
	quickWinSet := make(map[string]bool, len(triage.QuickWins))
	blockerSet := make(map[string]bool, len(triage.BlockersToClear))
	unblocksMap := make(map[string][]string, len(triage.Recommendations))

	for _, rec := range triage.Recommendations {
		triageScores[rec.ID] = rec.Score
		if len(rec.Reasons) > 0 {
			triageReasons[rec.ID] = analysis.TriageReasons{
				Primary:    rec.Reasons[0],
				All:        rec.Reasons,
				ActionHint: rec.Action,
			}
		}
		unblocksMap[rec.ID] = rec.UnblocksIDs
	}
	for _, qw := range triage.QuickWins {
		quickWinSet[qw.ID] = true
	}
	for _, bl := range triage.BlockersToClear {
		blockerSet[bl.ID] = true
	}

	m.ac.triageScores = triageScores
	m.ac.triageReasons = triageReasons
	m.ac.quickWinSet = quickWinSet
	m.ac.blockerSet = blockerSet
	m.ac.unblocksMap = unblocksMap

	m.insightsPanel.SetTopPicks(triage.QuickRef.TopPicks)
	dataHash := fmt.Sprintf("v%s@%s#%d", triage.Meta.Version, triage.Meta.GeneratedAt.Format("15:04:05"), triage.Meta.IssueCount)
	m.insightsPanel.SetRecommendations(triage.Recommendations, dataHash)

	// Generate priority recommendations, scoped to the currently filtered
	// view (bt-gcuv) rather than the full cross-project m.data.issues.
	recsStart := time.Now()
	m.recomputePriorityHints()
	debug.LogTiming("phase2.GenerateRecommendations", time.Since(recsStart))

	// Refresh alerts with full Phase 2 metrics
	alertsStart := time.Now()
	m.alerts, m.alertsCritical, m.alertsWarning, m.alertsInfo = computeAlerts(m.data.issues, m.workspaceMode)
	debug.LogTiming("phase2.computeAlerts", time.Since(alertsStart))

	// Invalidate label health cache. Scope the issue enumeration to the
	// active workspace repo filter (bt-dcby.3), matching the toggle-key
	// call site in model_update_input.go.
	m.labelHealthCached = false
	if m.focused == focusLabelDashboard {
		cfg := analysis.DefaultLabelHealthConfig()
		m.labelHealthCache = analysis.ComputeAllLabelHealth(m.workspacePrefilter(m.data.issues), cfg, time.Now().UTC(), m.data.analysis)
		m.labelHealthCached = true
		m.labelDashboard.SetData(m.labelHealthCache.Labels)
		m.setStatus(fmt.Sprintf("Labels: %d total • critical %d • warning %d", m.labelHealthCache.TotalLabels, m.labelHealthCache.CriticalCount, m.labelHealthCache.WarningCount))
	}

	// Re-sort if sorting by Phase 2 metrics
	if m.filter.activeRecipe != nil {
		switch m.filter.activeRecipe.Sort.Field {
		case "impact", "pagerank":
			field := m.filter.activeRecipe.Sort.Field
			descending := m.filter.activeRecipe.Sort.Direction == "desc"
			sort.Slice(m.data.issues, func(i, j int) bool {
				ii := m.data.issues[i]
				jj := m.data.issues[j]

				var iScore, jScore float64
				if m.data.analysis != nil {
					if field == "impact" {
						iScore = m.data.analysis.GetCriticalPathScore(ii.ID)
						jScore = m.data.analysis.GetCriticalPathScore(jj.ID)
					} else {
						iScore = m.data.analysis.GetPageRankScore(ii.ID)
						jScore = m.data.analysis.GetPageRankScore(jj.ID)
					}
				}

				var cmp int
				switch {
				case iScore < jScore:
					cmp = -1
				case iScore > jScore:
					cmp = 1
				}
				if cmp == 0 {
					return ii.ID < jj.ID
				}
				if descending {
					return cmp > 0
				}
				return cmp < 0
			})
			for i := range m.data.issues {
				m.data.issueMap[m.data.issues[i].ID] = &m.data.issues[i]
			}
		}
	}

	// Re-apply filters. When nothing is active, refreshListItemsPhase2 is an
	// in-place score refresh (avoids rebuilding the filtered set and losing
	// selection); everything else (recipe, BQL, plain status/label) goes
	// through the shared reapplyActiveFilter dispatcher so a BQL filter
	// doesn't fall through to applyFilter() and zero out (bt-0iajg).
	filterStart := time.Now()
	if m.filter.currentFilter == "" || m.filter.currentFilter == "all" {
		m.refreshListItemsPhase2()
	} else {
		m.reapplyActiveFilter()
	}
	debug.LogTiming("phase2.filter.reapply", time.Since(filterStart))

	return m, nil
}

// recomputePriorityHints (re)generates the priority-hint recommendations
// (the arrows shown when 'p' is toggled on) from the currently filtered/
// visible issue set rather than the full cross-project m.data.issues
// (bt-gcuv). GenerateRecommendations normalizes PageRank/Betweenness/etc.
// against whatever issue set it's given, so a globally-built Analyzer would
// surface "high impact" arrows for issues outside the active project
// filter - building a fresh Analyzer over filteredIssuesForActiveView()
// scopes the recommendations to what the user can actually see.
//
// Callers must also refresh the list delegate (done here) since
// IssueDelegate.PriorityHints is a snapshot taken at SetDelegate time, not
// a live reference to m.ac.priorityHints.
func (m *Model) recomputePriorityHints() {
	issues := m.filteredIssuesForActiveView()
	analyzer := analysis.NewAnalyzer(issues)
	recommendations := analyzer.GenerateRecommendations()
	hints := make(map[string]*analysis.PriorityRecommendation, len(recommendations))
	for i := range recommendations {
		hints[recommendations[i].IssueID] = &recommendations[i]
	}
	m.ac.priorityHints = hints
	m.updateListDelegate()
}

// handlePhase2Update processes BackgroundWorker Phase 2 completion notification.
func (m Model) handlePhase2Update(msg Phase2UpdateMsg) (Model, tea.Cmd) {
	if m.data.snapshot == nil || m.data.snapshot.DataHash != msg.DataHash {
		if m.data.backgroundWorker != nil {
			return m, WaitForBackgroundWorkerMsgCmd(m.data.backgroundWorker)
		}
		return m, nil
	}
	m.data.snapshot.Phase2Ready = true
	if m.data.backgroundWorker != nil {
		return m, WaitForBackgroundWorkerMsgCmd(m.data.backgroundWorker)
	}
	return m, nil
}

// handleHistoryLoaded processes background history loading completion.
func (m Model) handleHistoryLoaded(msg HistoryLoadedMsg) Model {
	m.historyLoading = false
	if msg.Error != nil {
		m.historyLoadFailed = true
		m.setFailure(fmt.Sprintf("History load failed: %v", msg.Error))
	} else if msg.Report != nil {
		m.historyView = NewHistoryModel(msg.Report, m.theme)
		m.historyView.SetContext(m.historyContext())
		m.historyView.SetSize(m.width, m.height-1)
		if m.isSplitView || m.showDetails {
			m.updateViewportContent()
		}
	}
	return m
}

// handleAgentFileCheck processes the AGENTS.md integration check result.
func (m Model) handleAgentFileCheck(msg AgentFileCheckMsg) Model {
	if msg.ShouldPrompt && msg.FilePath != "" {
		m.openModal(ModalAgentPrompt)
		m.agentPromptModal = NewAgentPromptModal(msg.FilePath, msg.FileType, m.theme)
		m.focused = focusAgentPrompt
	}
	return m
}
