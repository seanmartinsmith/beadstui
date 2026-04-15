package ui

// model_update_analysis.go contains Update() handlers for analysis, worker,
// and system messages. Extracted from the main Update() switch to keep the
// router thin.

import (
	"fmt"
	"sort"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/list"
)

// handleUpdateMsg processes a version update notification.
func (m Model) handleUpdateMsg(msg UpdateMsg) Model {
	m.updateAvailable = true
	m.updateTag = msg.TagName
	m.updateURL = msg.URL
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
		m.statusIsError = false
	}
	return m
}

// handleSemanticIndexReady processes the semantic index build completion.
// Returns (Model, tea.Cmd, done). If done is true, caller should return immediately.
func (m Model) handleSemanticIndexReady(msg SemanticIndexReadyMsg) (Model, tea.Cmd, bool) {
	m.semanticIndexBuilding = false
	if msg.Error != nil {
		m.semanticSearchEnabled = false
		m.list.Filter = list.DefaultFilter
		m.statusMsg = fmt.Sprintf("Semantic search unavailable: %v", msg.Error)
		m.statusIsError = true
		return m, nil, false
	}
	if m.semanticSearch != nil {
		m.semanticSearch.SetIndex(msg.Index, msg.Embedder)
	}
	if !msg.Loaded {
		m.statusMsg = fmt.Sprintf("Semantic index built (%d embedded)", msg.Stats.Embedded)
	} else if msg.Stats.Changed() {
		m.statusMsg = fmt.Sprintf("Semantic index updated (+%d ~%d -%d)", msg.Stats.Added, msg.Stats.Updated, msg.Stats.Removed)
	} else {
		m.statusMsg = "Semantic index up to date"
	}
	m.statusIsError = false

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
		m.statusMsg = fmt.Sprintf("Hybrid search unavailable: %v", msg.Error)
		m.statusIsError = true
		return m, nil
	}
	if m.semanticSearch != nil && msg.Cache != nil {
		m.semanticSearch.SetMetricsCache(msg.Cache)
	}
	m.semanticHybridReady = msg.Cache != nil
	m.statusMsg = fmt.Sprintf("Hybrid search ready (%s)", m.semanticHybridPreset)
	m.statusIsError = false

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
		m.semanticSearch.SetCachedResults(msg.Term, msg.Results)

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

// handleWorkerPollTick updates the worker spinner animation.
func (m Model) handleWorkerPollTick() (Model, tea.Cmd) {
	if m.data.backgroundWorker != nil {
		state := m.data.backgroundWorker.State()
		if state == WorkerProcessing {
			m.data.workerSpinnerIdx = (m.data.workerSpinnerIdx + 1) % len(workerSpinnerFrames)
		} else {
			m.data.workerSpinnerIdx = 0
		}
		if state != WorkerStopped {
			return m, workerPollTickCmd()
		}
	}
	return m, nil
}

// handlePhase2Ready processes async graph analysis Phase 2 completion.
func (m Model) handlePhase2Ready(msg Phase2ReadyMsg) (Model, tea.Cmd) {
	// Ignore stale Phase2 completions (from before a file reload)
	if msg.Stats != m.data.analysis {
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
	m.insightsPanel.SetInsights(ins)
	m.insightsPanel.issueMap = m.data.issueMap
	bodyHeight := m.height - 1
	if bodyHeight < 5 {
		bodyHeight = 5
	}
	m.insightsPanel.SetSize(m.width, bodyHeight)
	if m.data.snapshot != nil {
		if m.data.snapshot.GraphLayout != nil {
			m.data.snapshot.GraphLayout.UpdatePhase2Ranks(msg.Stats)
		}
		m.graphView.SetSnapshot(m.data.snapshot)
	} else {
		m.graphView.SetIssues(m.data.issues, &ins)
	}

	// Generate triage for priority panel
	triage := analysis.ComputeTriageFromAnalyzer(m.data.analyzer, m.data.analysis, m.data.issues, analysis.TriageOptions{}, time.Now())
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

	// Generate priority recommendations
	recommendations := m.data.analyzer.GenerateRecommendations()
	m.ac.priorityHints = make(map[string]*analysis.PriorityRecommendation, len(recommendations))
	for i := range recommendations {
		m.ac.priorityHints[recommendations[i].IssueID] = &recommendations[i]
	}

	// Refresh alerts with full Phase 2 metrics
	m.alerts, m.alertsCritical, m.alertsWarning, m.alertsInfo = computeAlerts(m.data.issues, m.data.analysis, m.data.analyzer)


	// Invalidate label health cache
	m.labelHealthCached = false
	if m.focused == focusLabelDashboard {
		cfg := analysis.DefaultLabelHealthConfig()
		m.labelHealthCache = analysis.ComputeAllLabelHealth(m.data.issues, cfg, time.Now().UTC(), m.data.analysis)
		m.labelHealthCached = true
		m.labelDashboard.SetData(m.labelHealthCache.Labels)
		m.statusMsg = fmt.Sprintf("Labels: %d total • critical %d • warning %d", m.labelHealthCache.TotalLabels, m.labelHealthCache.CriticalCount, m.labelHealthCache.WarningCount)
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

	// Re-apply filters
	if m.filter.activeRecipe != nil {
		m.applyRecipe(m.filter.activeRecipe)
	} else if m.filter.currentFilter == "" || m.filter.currentFilter == "all" {
		m.refreshListItemsPhase2()
	} else {
		m.applyFilter()
	}

	return m, nil
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
		m.statusMsg = fmt.Sprintf("History load failed: %v", msg.Error)
		m.statusIsError = true
	} else if msg.Report != nil {
		m.historyView = NewHistoryModel(msg.Report, m.theme)
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
