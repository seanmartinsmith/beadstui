package ui

// model_update_input.go contains Update() handlers for user input messages:
// tea.KeyPressMsg, tea.MouseWheelMsg, tea.WindowSizeMsg.
// Extracted from the main Update() switch to keep the router thin.

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/agents"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
)

// handleKeyPress processes keyboard input.
// Many branches return early; some fall through so the router can apply the list update tail.
func (m Model) handleKeyPress(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Clear status message on any keypress.
	// Must also reset statusSetAt so the next message gets a fresh auto-dismiss
	// window. Without this, renderFooter's auto-dismiss sees the stale timestamp
	// from a previous message and clears the new one immediately (bt-6k0f).
	m.statusMsg = ""
	m.statusIsError = false
	m.statusSetAt = time.Time{}

	// Handle AGENTS.md prompt modal (bv-i8dk)
	if m.activeModal == ModalAgentPrompt {
		m.agentPromptModal, cmd = m.agentPromptModal.Update(msg)
		cmds = append(cmds, cmd)

		// Check if user made a decision
		switch m.agentPromptModal.Result() {
		case AgentPromptAccept:
			// User accepted - add blurb to file
			filePath := m.agentPromptModal.FilePath()
			if err := agents.AppendBlurbToFile(filePath); err != nil {
				m.statusMsg = "Failed to update " + filepath.Base(filePath) + ": " + err.Error()
				m.statusIsError = true
			} else {
				m.statusMsg = "✓ Added beads instructions to " + filepath.Base(filePath)
				// Record acceptance
				_ = agents.RecordAccept(m.workDir)
			}
			m.closeModal()
			m.focused = focusList
		case AgentPromptDecline:
			// User declined - just dismiss, may ask again next time
			m.closeModal()
			m.focused = focusList
		case AgentPromptNeverAsk:
			// User chose "don't ask again" - save preference
			_ = agents.RecordDecline(m.workDir, true)
			m.closeModal()
			m.focused = focusList
		}
		return m, tea.Batch(cmds...)
	}

	// Handle cass session modal (bv-5bqh)
	if m.activeModal == ModalCassSession {
		m.cassModal, cmd = m.cassModal.Update(msg)
		cmds = append(cmds, cmd)

		// Check for dismiss keys
		switch msg.String() {
		case "V", "esc", "enter", "q":
			m.closeModal()
			m.focused = focusList
			return m, tea.Batch(cmds...)
		}
		return m, tea.Batch(cmds...)
	}

	// Handle self-update modal (bv-182)
	if m.activeModal == ModalUpdate {
		m.updateModal, cmd = m.updateModal.Update(msg)
		cmds = append(cmds, cmd)

		// Handle modal state changes
		switch msg.String() {
		case "esc", "q":
			// Always allow escape to close
			if !m.updateModal.IsInProgress() {
				m.closeModal()
				m.focused = focusList
				return m, tea.Batch(cmds...)
			}
		case "enter":
			// Close on enter if complete or if cancelled
			if m.updateModal.IsComplete() {
				m.closeModal()
				m.focused = focusList
				return m, tea.Batch(cmds...)
			}
			// If confirming and cancelled, close
			if m.updateModal.IsConfirming() && m.updateModal.IsCancelled() {
				m.closeModal()
				m.focused = focusList
				return m, tea.Batch(cmds...)
			}
		case "n", "N":
			// Quick cancel
			if m.updateModal.IsConfirming() {
				m.closeModal()
				m.focused = focusList
				return m, tea.Batch(cmds...)
			}
		}
		return m, tea.Batch(cmds...)
	}

	// Close label health detail modal if open
	if m.activeModal == ModalLabelHealthDetail {
		s := msg.String()
		if s == "esc" || s == "q" || s == "enter" || s == "h" {
			m.closeModal()
			m.labelHealthDetail = nil
			return m, nil
		}
		if s == "d" && m.labelHealthDetail != nil {
			// open drilldown from detail modal
			m.labelDrilldownLabel = m.labelHealthDetail.Label
			m.labelDrilldownIssues = m.filterIssuesByLabel(m.labelDrilldownLabel)
			m.openModal(ModalLabelDrilldown)
			return m, nil
		}
	}

	// Handle label drilldown modal if open
	if m.activeModal == ModalLabelDrilldown {
		s := msg.String()
		switch s {
		case "enter":
			// Apply label filter to main list and close drilldown
			if m.labelDrilldownLabel != "" {
				m.filter.currentFilter = "label:" + m.labelDrilldownLabel
				m.applyFilter()
				m.focused = focusList
			}
			m.closeModal()
			m.labelDrilldownLabel = ""
			m.labelDrilldownIssues = nil
			return m, nil
		case "g":
			// Show graph analysis sub-view (bv-109)
			if m.labelDrilldownLabel != "" {
				sg := analysis.ComputeLabelSubgraph(m.data.issues, m.labelDrilldownLabel)
				pr := analysis.ComputeLabelPageRank(sg)
				cp := analysis.ComputeLabelCriticalPath(sg)
				m.labelGraphAnalysisResult = &LabelGraphAnalysisResult{
					Label:        m.labelDrilldownLabel,
					Subgraph:     sg,
					PageRank:     pr,
					CriticalPath: cp,
				}
				m.openModal(ModalLabelGraphAnalysis)
			}
			return m, nil
		case "esc", "q", "d":
			m.closeModal()
			m.labelDrilldownLabel = ""
			m.labelDrilldownIssues = nil
			return m, nil
		}
	}

	// Handle label graph analysis sub-view (bv-109)
	if m.activeModal == ModalLabelGraphAnalysis {
		s := msg.String()
		switch s {
		case "esc", "q", "g":
			m.closeModal()
			m.labelGraphAnalysisResult = nil
			return m, nil
		}
	}

	// Handle attention view quick jumps (bv-117)
	if m.mode == ViewAttention {
		s := msg.String()
		switch {
		case s == "esc" || s == "q" || s == "d":
			m.mode = ViewList
			m.focused = focusList
			m.insightsPanel.extraText = ""
			return m, nil
		case len(s) == 1 && s[0] >= '1' && s[0] <= '9':
			if len(m.attentionCache.Labels) == 0 {
				return m, nil
			}
			idx := int(s[0] - '1')
			if idx >= 0 && idx < len(m.attentionCache.Labels) {
				label := m.attentionCache.Labels[idx].Label
				m.filter.currentFilter = "label:" + label
				m.applyFilter()
				m.statusMsg = fmt.Sprintf("Filtered to label %s (attention #%d)", label, idx+1)
				m.statusIsError = false
			}
			return m, nil
		}
	}

	// Handle alerts panel modal if open (bv-168)
	if m.activeModal == ModalAlerts {
		// Build list of active (non-dismissed) alerts
		var activeAlerts []drift.Alert
		for _, a := range m.alerts {
			if !m.dismissedAlerts[alertKey(a)] {
				activeAlerts = append(activeAlerts, a)
			}
		}
		s := msg.String()
		switch s {
		case "j", "down":
			if m.alertsCursor < len(activeAlerts)-1 {
				m.alertsCursor++
				// Scroll down if cursor moves past visible area
				visLines := m.alertsVisibleLines()
				if visLines > 0 && m.alertsCursor >= m.alertsScrollOffset+visLines {
					m.alertsScrollOffset = m.alertsCursor - visLines + 1
				}
			}
			return m, nil
		case "k", "up":
			if m.alertsCursor > 0 {
				m.alertsCursor--
				// Scroll up if cursor moves above visible area
				if m.alertsCursor < m.alertsScrollOffset {
					m.alertsScrollOffset = m.alertsCursor
				}
			}
			return m, nil
		case "enter":
			// Jump to the issue referenced by the selected alert
			if m.alertsCursor < len(activeAlerts) {
				issueID := activeAlerts[m.alertsCursor].IssueID
				if issueID != "" {
					// Find the issue in the list and select it
					for i, item := range m.list.Items() {
						if it, ok := item.(IssueItem); ok && it.Issue.ID == issueID {
							m.list.Select(i)
							break
						}
					}
				}
			}
			m.closeModal()
			return m, nil
		case "c":
			// Clear the selected alert
			if m.alertsCursor < len(activeAlerts) {
				key := alertKey(activeAlerts[m.alertsCursor])
				m.dismissedAlerts[key] = true
				// Adjust cursor if needed
				remaining := 0
				for _, a := range m.alerts {
					if !m.dismissedAlerts[alertKey(a)] {
						remaining++
					}
				}
				if m.alertsCursor >= remaining {
					m.alertsCursor = remaining - 1
				}
				if m.alertsCursor < 0 {
					m.alertsCursor = 0
				}
				// Scroll offset may need adjusting
				if m.alertsScrollOffset > m.alertsCursor {
					m.alertsScrollOffset = m.alertsCursor
				}
				// Close panel if no alerts left
				if remaining == 0 {
					m.closeModal()
				}
			}
			return m, nil
		case "C":
			// Clear all alerts
			for _, a := range activeAlerts {
				m.dismissedAlerts[alertKey(a)] = true
			}
			m.alertsCursor = 0
			m.alertsScrollOffset = 0
			m.closeModal()
			return m, nil
		case "esc", "q", "!":
			m.closeModal()
			return m, nil
		}
		return m, nil
	}

	// Handle repo picker overlay (workspace mode) before global keys (esc/q/etc.)
	if m.activeModal == ModalRepoPicker {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m = m.handleRepoPickerKeys(msg)
		return m, nil
	}

	// Handle BQL query modal before global keys
	if m.activeModal == ModalBQLQuery {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m, cmd = m.handleBQLQueryKeys(msg)
		return m, cmd
	}

	// Handle label picker modal before global keys (bt-eorx)
	// Without this early return, typed characters get intercepted by global handlers
	// (e.g., 'g' opens graph, 'i' opens insights) triggering expensive operations.
	if m.activeModal == ModalLabelPicker {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m = m.handleLabelPickerKeys(msg)
		return m, nil
	}

	// Handle recipe picker overlay before global keys (esc/q/etc.)
	if m.activeModal == ModalRecipePicker {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m = m.handleRecipePickerKeys(msg)
		return m, nil
	}

	// Handle quit confirmation first
	if m.activeModal == ModalQuitConfirm {
		switch msg.String() {
		case "esc", "y", "Y":
			return m, tea.Quit
		default:
			m.closeModal()
			m.focused = focusList
			return m, nil
		}
	}

	// Handle help overlay toggle (? or F1)
	if (msg.String() == "?" || msg.String() == "f1") && m.list.FilterState() != list.Filtering {
		if m.activeModal == ModalHelp {
			m.closeModal()
			m.focused = m.restoreFocusFromHelp()
		} else {
			m.focusBeforeHelp = m.focused // Store current focus before switching to help
			m.openModal(ModalHelp)
			m.focused = focusHelp
			m.helpScroll = 0 // Reset scroll position when opening help
		}
		return m, nil
	}

	// Handle tutorial toggle (backtick `) - bv-8y31
	if msg.String() == "`" && m.list.FilterState() != list.Filtering {
		if m.activeModal == ModalTutorial {
			m.closeModal()
			m.focused = focusList
		} else {
			m.closeModal() // Close help or any other modal if open
			m.openModal(ModalTutorial)
			m.tutorialModel.SetSize(m.width, m.height)
			m.focused = focusTutorial
		}
		return m, nil
	}

	// Force refresh (bv-4auz): Ctrl+R / F5 triggers an immediate reload.
	if (msg.String() == "ctrl+r" || msg.String() == "f5") && m.list.FilterState() != list.Filtering {
		now := time.Now()
		if !m.data.lastForceRefresh.IsZero() && now.Sub(m.data.lastForceRefresh) < time.Second {
			return m, nil
		}
		m.data.lastForceRefresh = now

		m.statusMsg = "Refreshing…"
		m.statusIsError = false

		if m.data.backgroundWorker != nil {
			m.data.backgroundWorker.ForceRefresh()
			cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.data.backgroundWorker))
			return m, tea.Batch(cmds...)
		}

		if m.data.beadsPath == "" && m.data.watcher == nil && !m.isDoltSource() {
			m.statusMsg = "Refresh unavailable"
			m.statusIsError = true
			return m, nil
		}

		// Dolt sources without background worker use async reload
		if m.isDoltSource() && m.data.beadsPath == "" {
			cmds = append(cmds, m.reloadFromDataSource())
			return m, tea.Batch(cmds...)
		}

		cmds = append(cmds, func() tea.Msg { return FileChangedMsg{} })
		return m, tea.Batch(cmds...)
	}

	// Handle shortcuts sidebar toggle (; or F2) - bv-3qi5
	if (msg.String() == ";" || msg.String() == "f2") && m.list.FilterState() != list.Filtering {
		m.showShortcutsSidebar = !m.showShortcutsSidebar
		if m.showShortcutsSidebar {
			m.shortcutsSidebar.ResetScroll()
			m.statusMsg = "Shortcuts sidebar: ; hide | ctrl+j/k scroll"
			m.statusIsError = false
		} else {
			m.statusMsg = ""
		}
		return m, nil
	}

	// Handle shortcuts sidebar scrolling (Ctrl+j/k when sidebar visible) - bv-3qi5
	if m.showShortcutsSidebar && m.list.FilterState() != list.Filtering {
		switch msg.String() {
		case "ctrl+j":
			m.shortcutsSidebar.ScrollDown()
			return m, nil
		case "ctrl+k":
			m.shortcutsSidebar.ScrollUp()
			return m, nil
		}
	}

	// Hybrid search toggle/preset cycle (bv-xbar.6)
	if m.focused == focusList && m.list.FilterState() != list.Filtering {
		switch msg.String() {
		case "H":
			m.statusIsError = false
			m.semanticHybridEnabled = !m.semanticHybridEnabled
			if m.semanticSearch == nil {
				m.semanticHybridEnabled = false
				m.statusMsg = "Hybrid search unavailable"
				m.statusIsError = true
				return m, nil
			}
			m.semanticSearch.SetHybridConfig(m.semanticHybridEnabled, m.semanticHybridPreset)
			m.semanticSearch.ResetCache()
			m.clearSemanticScores()
			if m.semanticHybridEnabled && !m.semanticHybridReady && !m.semanticHybridBuilding {
				m.semanticHybridBuilding = true
				m.statusMsg = "Hybrid search: computing metrics…"
				cmds = append(cmds, BuildHybridMetricsCmd(m.issuesForAsync()))
			} else if m.semanticHybridEnabled {
				m.statusMsg = fmt.Sprintf("Hybrid search enabled (%s)", m.semanticHybridPreset)
			} else {
				m.statusMsg = "Semantic search: text-only"
			}
			if m.semanticSearchEnabled && m.list.FilterState() != list.Unfiltered {
				currentTerm := m.list.FilterInput.Value()
				if currentTerm != "" && !m.semanticHybridBuilding {
					cmds = append(cmds, ComputeSemanticFilterCmd(m.semanticSearch, currentTerm))
				}
			}
			m.updateListDelegate()
			return m, tea.Batch(cmds...)
		case "alt+h", "alt+H":
			m.statusIsError = false
			m.semanticHybridPreset = nextHybridPreset(m.semanticHybridPreset)
			if m.semanticSearch != nil {
				m.semanticSearch.SetHybridConfig(m.semanticHybridEnabled, m.semanticHybridPreset)
				m.semanticSearch.ResetCache()
			}
			m.clearSemanticScores()
			if m.semanticHybridEnabled {
				m.statusMsg = fmt.Sprintf("Hybrid preset: %s", m.semanticHybridPreset)
			} else {
				m.statusMsg = fmt.Sprintf("Hybrid preset set (%s)", m.semanticHybridPreset)
			}
			if m.semanticSearchEnabled && m.semanticHybridEnabled && m.list.FilterState() != list.Unfiltered {
				currentTerm := m.list.FilterInput.Value()
				if currentTerm != "" && !m.semanticHybridBuilding {
					cmds = append(cmds, ComputeSemanticFilterCmd(m.semanticSearch, currentTerm))
				}
			}
			m.updateListDelegate()
			return m, tea.Batch(cmds...)
		}
	}

	// Semantic search toggle (bv-9gf.3)
	if msg.String() == "ctrl+s" && m.focused == focusList {
		m.statusIsError = false
		m.semanticSearchEnabled = !m.semanticSearchEnabled
		if m.semanticSearchEnabled {
			if m.semanticSearch != nil {
				m.list.Filter = m.semanticSearch.Filter
				if !m.semanticSearch.Snapshot().Ready && !m.semanticIndexBuilding {
					m.semanticIndexBuilding = true
					m.statusMsg = "Semantic search: building index…"
					cmds = append(cmds, BuildSemanticIndexCmd(m.issuesForAsync()))
				} else if !m.semanticSearch.Snapshot().Ready && m.semanticIndexBuilding {
					m.statusMsg = "Semantic search: indexing…"
				} else {
					m.statusMsg = "Semantic search enabled"
				}
			} else {
				m.semanticSearchEnabled = false
				m.list.Filter = list.DefaultFilter
				m.statusMsg = "Semantic search unavailable"
				m.statusIsError = true
			}
			if m.semanticHybridEnabled && !m.semanticHybridReady && !m.semanticHybridBuilding {
				m.semanticHybridBuilding = true
				cmds = append(cmds, BuildHybridMetricsCmd(m.issuesForAsync()))
			}
		} else {
			m.list.Filter = list.DefaultFilter
			m.statusMsg = "Fuzzy search enabled"
			m.clearSemanticScores()
		}

		// Refresh the current list filter results immediately.
		prevState := m.list.FilterState()
		filterText := m.list.FilterInput.Value()
		if prevState != list.Unfiltered {
			m.list.SetFilterText(filterText)
			if prevState == list.Filtering {
				m.list.SetFilterState(list.Filtering)
			}
		}

		m.updateListDelegate()
		return m, tea.Batch(cmds...)
	}

	// If help is showing, handle navigation keys for scrolling
	if m.focused == focusHelp {
		m = m.handleHelpKeys(msg)
		return m, nil
	}

	// If tutorial is showing, route input to tutorial model (bv-8y31)
	if m.focused == focusTutorial && m.activeModal == ModalTutorial {
		var tutorialCmd tea.Cmd
		m.tutorialModel, tutorialCmd = m.tutorialModel.Update(msg)
		// Check if tutorial wants to close
		if m.tutorialModel.ShouldClose() {
			m.closeModal()
			m.focused = focusList
			m.tutorialModel = NewTutorialModel(m.theme) // Reset for next time
		}
		return m, tutorialCmd
	}

	// Handle time-travel input first (before global keys intercept letters)
	// But allow ctrl+c to always quit
	if m.focused == focusTimeTravelInput {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m = m.handleTimeTravelInputKeys(msg)
		return m, nil
	}

	// Handle keys when not filtering
	if m.list.FilterState() != list.Filtering {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "q":
			// q closes current view or quits if at top level
			if m.showDetails && !m.isSplitView {
				m.showDetails = false
				m.focused = focusList
				return m, nil
			}
			if m.focused == focusInsights {
				m.focused = focusList
				return m, nil
			}
			if m.focused == focusFlowMatrix {
				if m.flowMatrix.showDrilldown {
					m.flowMatrix.showDrilldown = false
					return m, nil
				}
				m.focused = focusList
				return m, nil
			}
			if m.mode == ViewGraph {
				m.mode = ViewList
				m.focused = focusList
				return m, nil
			}
			if m.mode == ViewBoard {
				m.mode = ViewList
				m.focused = focusList
				return m, nil
			}
			if m.mode == ViewLabelDashboard {
				m.mode = ViewList
				m.isSplitView = true // restore split view cleared on entry (bt-trqo)
				m.focused = focusList
				return m, nil
			}
			return m, tea.Quit

		case "esc":
			// Escape closes modals and goes back
			if m.showDetails && !m.isSplitView {
				m.showDetails = false
				m.focused = focusList
				return m, nil
			}
			if m.mode == ViewInsights || m.mode == ViewAttention {
				m.mode = ViewList
				m.focused = focusList
				return m, nil
			}
			if m.mode == ViewFlowMatrix {
				if m.flowMatrix.showDrilldown {
					m.flowMatrix.showDrilldown = false
					return m, nil
				}
				m.mode = ViewList
				m.focused = focusList
				return m, nil
			}
			if m.mode == ViewGraph {
				m.mode = ViewList
				m.focused = focusList
				return m, nil
			}
			if m.mode == ViewBoard {
				m.mode = ViewList
				m.focused = focusList
				return m, nil
			}
			if m.mode == ViewActionable {
				m.mode = ViewList
				m.focused = focusList
				return m, nil
			}
			if m.mode == ViewHistory {
				m.mode = ViewList
				m.focused = focusList
				return m, nil
			}
			// Label picker esc is now handled by the early return above (bt-eorx)
			// Close label dashboard if open (bt-trqo: restore split view)
			if m.mode == ViewLabelDashboard {
				m.mode = ViewList
				m.isSplitView = true
				m.focused = focusList
				return m, nil
			}
			// At main list - first ESC clears filters, second shows quit confirm
			if m.hasActiveFilters() {
				m.clearAllFilters()
				return m, nil
			}
			// No filters active - show quit confirmation
			m.openModal(ModalQuitConfirm)
			m.focused = focusQuitConfirm
			return m, nil

		case "tab":
			if m.isSplitView && m.mode == ViewList {
				if m.focused == focusList {
					m.focused = focusDetail
				} else {
					m.focused = focusList
				}
			}

		case "<":
			// Shrink list pane (move divider left)
			if m.isSplitView {
				m.splitPaneRatio -= 0.05
				if m.splitPaneRatio < 0.2 {
					m.splitPaneRatio = 0.2
				}
				m.recalculateSplitPaneSizes()
			}

		case ">":
			// Expand list pane (move divider right)
			if m.isSplitView {
				m.splitPaneRatio += 0.05
				if m.splitPaneRatio > 0.8 {
					m.splitPaneRatio = 0.8
				}
				m.recalculateSplitPaneSizes()
			}

		case "b":
			m.clearAttentionOverlay()
			if m.mode == ViewBoard {
				m.mode = ViewList
				m.focused = focusList
			} else {
				m.mode = ViewBoard
				m.focused = focusBoard
				m.refreshBoardAndGraphForCurrentFilter()
			}
			return m, nil

		case "g":
			// Toggle graph view
			m.clearAttentionOverlay()
			if m.mode == ViewGraph {
				m.mode = ViewList
				m.focused = focusList
			} else {
				m.mode = ViewGraph
				m.focused = focusGraph
				m.refreshBoardAndGraphForCurrentFilter()
			}
			return m, nil

		case "a":
			// Toggle actionable view
			m.clearAttentionOverlay()
			if m.mode == ViewActionable {
				m.mode = ViewList
				m.focused = focusList
			} else {
				m.mode = ViewActionable
				// Build execution plan
				analyzer := analysis.NewAnalyzer(m.data.issues)
				plan := analyzer.GetExecutionPlan()
				m.actionableView = NewActionableModel(plan, m.theme)
				m.actionableView.SetSize(m.width, m.height-2)
				m.focused = focusActionable
			}
			return m, nil

		case "E":
			// Toggle hierarchical tree view (bv-gllx)
			m.clearAttentionOverlay()
			if m.mode == ViewTree {
				m.mode = ViewList
				m.focused = focusList
			} else {
				m.mode = ViewTree
				// Build tree from snapshot when available (bv-t435)
				if m.data.snapshot != nil {
					m.tree.BuildFromSnapshot(m.data.snapshot)
				} else {
					m.tree.Build(m.data.issues)
				}
				m.tree.SetSize(m.width, m.height-2)
				m.focused = focusTree
			}
			return m, nil

		case "i":
			m.clearAttentionOverlay()
			if m.mode == ViewInsights {
				m.mode = ViewList
				m.focused = focusList
			} else {
				m.mode = ViewInsights
				m.focused = focusInsights
				// Refresh insights using the current snapshot when available (bv-mpqz).
				var ins analysis.Insights
				hasInsights := false
				if m.data.snapshot != nil {
					ins = m.data.snapshot.Insights
					hasInsights = true
				} else if m.data.analysis != nil {
					ins = m.data.analysis.GenerateInsights(len(m.data.issues))
					hasInsights = true
				}
				if hasInsights {
					m.insightsPanel = NewInsightsModel(ins, m.data.issueMap, m.theme)
					// Include priority triage (bv-91) - reuse existing analyzer/stats (bv-runn.12)
					triage := analysis.ComputeTriageFromAnalyzer(m.data.analyzer, m.data.analysis, m.data.issues, analysis.TriageOptions{}, time.Now())
					m.insightsPanel.SetTopPicks(triage.QuickRef.TopPicks)
					// Set full recommendations with breakdown for priority radar (bv-93)
					dataHash := fmt.Sprintf("v%s@%s#%d", triage.Meta.Version, triage.Meta.GeneratedAt.Format("15:04:05"), triage.Meta.IssueCount)
					m.insightsPanel.SetRecommendations(triage.Recommendations, dataHash)
					panelHeight := m.height - 2
					if panelHeight < 3 {
						panelHeight = 3
					}
					m.insightsPanel.SetSize(m.width, panelHeight)
				}
			}
			return m, nil

		case "p":
			// Toggle priority hints
			m.ac.showPriorityHints = !m.ac.showPriorityHints
			// Update delegate with new state
			m.updateListDelegate()
			// Show explanatory status message
			if m.ac.showPriorityHints {
				count := len(m.ac.priorityHints)
				if count > 0 {
					m.statusMsg = fmt.Sprintf("Priority hints: ↑ increase ↓ decrease (%d suggestions)", count)
				} else {
					m.statusMsg = "Priority hints: No misalignments detected (analysis ongoing)"
				}
			} else {
				m.statusMsg = ""
			}
			return m, nil

		case "h":
			// Toggle history view
			m.clearAttentionOverlay()
			if m.mode == ViewHistory {
				m.mode = ViewList
				m.focused = focusList
			} else {
				m.mode = ViewHistory
				// Ensure history model has latest sizing
				bodyHeight := m.height - 1
				if bodyHeight < 5 {
					bodyHeight = 5
				}
				m.historyView.SetSize(m.width, bodyHeight)
				m.focused = focusHistory
			}
			return m, nil

		case "[", "f3":
			// Open label dashboard (phase 1: table view)
			m.clearAttentionOverlay()
			m.mode = ViewLabelDashboard
			m.isSplitView = false
			m.focused = focusLabelDashboard
			// Compute label health (fast; phase1 metrics only needed) with caching
			if !m.labelHealthCached {
				cfg := analysis.DefaultLabelHealthConfig()
				m.labelHealthCache = analysis.ComputeAllLabelHealth(m.data.issues, cfg, time.Now().UTC(), m.data.analysis)
				m.labelHealthCached = true
			}
			m.labelDashboard.SetData(m.labelHealthCache.Labels)
			m.labelDashboard.SetSize(m.width, m.height-1)
			m.statusMsg = fmt.Sprintf("Labels: %d total • critical %d • warning %d", m.labelHealthCache.TotalLabels, m.labelHealthCache.CriticalCount, m.labelHealthCache.WarningCount)
			m.statusIsError = false
			return m, nil

		case "]", "f4":
			// Attention view: compute attention scores (cached) and render as text
			if !m.attentionCached {
				cfg := analysis.DefaultLabelHealthConfig()
				m.attentionCache = analysis.ComputeLabelAttentionScores(m.data.issues, cfg, time.Now().UTC())
				m.attentionCached = true
			}
			attText, _ := ComputeAttentionView(m.data.issues, max(40, m.width-4))
			m.mode = ViewAttention
			m.focused = focusInsights
			m.insightsPanel = NewInsightsModel(analysis.Insights{}, m.data.issueMap, m.theme)
			m.insightsPanel.labelAttention = m.attentionCache.Labels
			m.insightsPanel.extraText = attText
			panelHeight := m.height - 2
			if panelHeight < 3 {
				panelHeight = 3
			}
			m.insightsPanel.SetSize(m.width, panelHeight)
			return m, nil

		case "f":
			// Flow matrix view (cross-label dependencies)
			m.clearAttentionOverlay()
			cfg := analysis.DefaultLabelHealthConfig()
			flow := analysis.ComputeCrossLabelFlow(m.data.issues, cfg)
			m.mode = ViewFlowMatrix
			m.focused = focusFlowMatrix
			m.flowMatrix = NewFlowMatrixModel(m.theme)
			m.flowMatrix.SetData(&flow, m.data.issues)
			panelHeight := m.height - 2
			if panelHeight < 3 {
				panelHeight = 3
			}
			m.flowMatrix.SetSize(m.width, panelHeight)
			return m, nil

		case "!":
			// Toggle alerts panel (bv-168)
			// Only show if there are active alerts
			activeCount := 0
			for _, a := range m.alerts {
				if !m.dismissedAlerts[alertKey(a)] {
					activeCount++
				}
			}
			if activeCount > 0 {
				if m.activeModal == ModalAlerts {
					m.closeModal()
				} else {
					m.openModal(ModalAlerts)
				}
				m.alertsCursor = 0       // Reset cursor when opening
				m.alertsScrollOffset = 0 // Reset scroll position
			} else {
				m.statusMsg = "No active alerts"
				m.statusIsError = false
			}
			return m, nil

		case ":":
			// Open BQL query modal
			m.bqlQuery.SetSize(m.width, m.height-1)
			m.bqlQuery.Reset()
			m.openModal(ModalBQLQuery)
			m.focused = focusBQLQuery
			return m, m.bqlQuery.Focus()

		case "'":
			// Toggle recipe picker overlay
			if m.activeModal == ModalRecipePicker {
				m.closeModal()
				m.focused = focusList
			} else {
				m.openModal(ModalRecipePicker)
				m.recipePicker.SetSize(m.width, m.height-1)
				m.focused = focusRecipePicker
			}
			return m, nil

		case "W":
			// Quick toggle between current project and all projects
			if !m.workspaceMode || len(m.availableRepos) == 0 {
				m.statusMsg = "Project filter available only in multi-project mode"
				m.statusIsError = false
				return m, nil
			}
			if m.currentProjectDB == "" {
				m.statusMsg = "No home project detected (not in a beads directory)"
				m.statusIsError = false
				return m, nil
			}
			if m.activeRepos != nil {
				// Currently filtered - expand to all
				m.activeRepos = nil
				m.statusMsg = "Showing all projects"
			} else {
				// Currently showing all - filter to home project
				m.activeRepos = map[string]bool{m.currentProjectDB: true}
				m.statusMsg = fmt.Sprintf("Showing project: %s", m.currentProjectDB)
			}
			m.statusIsError = false
			if m.filter.activeRecipe != nil {
				m.applyRecipe(m.filter.activeRecipe)
			} else {
				m.applyFilter()
			}
			return m, nil

		case "w":
			// Project picker overlay (multi-project mode), or wisp toggle (bt-9kdo)
			if !m.workspaceMode || len(m.availableRepos) == 0 {
				// bt-9kdo: toggle wisp (ephemeral) visibility
				m.showWisps = !m.showWisps
				m.applyFilter()
				if m.showWisps {
					m.setStatus("wisps: visible")
				} else {
					m.setStatus("wisps: hidden")
				}
				return m, nil
			}
			if m.activeModal == ModalRepoPicker {
				m.closeModal()
				m.focused = focusList
			} else {
				m.openModal(ModalRepoPicker)
				m.repoPicker = NewRepoPickerModel(m.availableRepos, m.theme)
				m.repoPicker.SetActiveRepos(m.activeRepos)
				m.repoPicker.SetSize(m.width, m.height-1)
				m.focused = focusRepoPicker
			}
			return m, nil

		case "x":
			// Export to Markdown file
			m.exportToMarkdown()
			return m, nil

		case "l":
			// Open label picker for quick filter (bv-126)
			if len(m.data.issues) == 0 {
				return m, nil
			}
			// Update labels in case they changed
			labelExtraction := analysis.ExtractLabels(m.data.issues)
			labelCounts := extractLabelCounts(labelExtraction.Stats)
			m.labelPicker.SetLabels(labelExtraction.Labels, labelCounts)
			m.labelPicker.Reset()
			m.labelPicker.SetSize(m.width, m.height-1)
			m.openModal(ModalLabelPicker)
			m.focused = focusLabelPicker
			return m, nil

		}

		// Focus-specific key handling
		switch m.focused {
		case focusBQLQuery:
			// BQL modal already handled in overlay dispatch above; no-op here
			return m, nil

		case focusRecipePicker:
			m = m.handleRecipePickerKeys(msg)

		case focusRepoPicker:
			m = m.handleRepoPickerKeys(msg)

		case focusLabelPicker:
			m = m.handleLabelPickerKeys(msg)

		case focusInsights:
			m = m.handleInsightsKeys(msg)

		case focusBoard:
			m = m.handleBoardKeys(msg)

		case focusLabelDashboard:
			// Exit label dashboard
			if msg.String() == "esc" || msg.String() == "q" || msg.String() == "[" {
				m.isSplitView = true
				m.focused = focusList
				return m, nil
			}
			if selectedLabel, cmd := m.labelDashboard.Update(msg); selectedLabel != "" {
				// Filter list by selected label and jump back to list view
				m.filter.currentFilter = "label:" + selectedLabel
				m.applyFilter()
				m.isSplitView = true
				m.focused = focusList
				return m, cmd
			}
			// Open detail modal on 'h'
			if msg.String() == "h" && len(m.labelDashboard.labels) > 0 {
				idx := m.labelDashboard.cursor
				if idx >= 0 && idx < len(m.labelDashboard.labels) {
					lh := m.labelDashboard.labels[idx]
					m.openModal(ModalLabelHealthDetail)
					m.labelHealthDetail = &lh
					// Precompute cross-label flows for this label
					m.labelHealthDetailFlow = m.getCrossFlowsForLabel(lh.Label)
					return m, nil
				}
			}
			// Open drilldown overlay on 'd'
			if msg.String() == "d" && len(m.labelDashboard.labels) > 0 {
				idx := m.labelDashboard.cursor
				if idx >= 0 && idx < len(m.labelDashboard.labels) {
					lh := m.labelDashboard.labels[idx]
					m.labelDrilldownLabel = lh.Label
					m.labelDrilldownIssues = m.filterIssuesByLabel(lh.Label)
					m.openModal(ModalLabelDrilldown)
					return m, nil
				}
			}

		case focusGraph:
			m = m.handleGraphKeys(msg)

		case focusTree:
			m = m.handleTreeKeys(msg)

		case focusActionable:
			m = m.handleActionableKeys(msg)

		case focusHistory:
			m = m.handleHistoryKeys(msg)

		case focusSprint:
			m = m.handleSprintKeys(msg)

		case focusFlowMatrix:
			m = m.handleFlowMatrixKeys(msg)

		case focusList:
			m = m.handleListKeys(msg)

		case focusDetail:
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

// handleMouseWheel processes mouse wheel events.
func (m Model) handleMouseWheel(msg tea.MouseWheelMsg) (Model, tea.Cmd) {
	// Intercept mouse wheel when alerts panel is open
	if m.activeModal == ModalAlerts {
		var activeAlerts []drift.Alert
		for _, a := range m.alerts {
			if !m.dismissedAlerts[alertKey(a)] {
				activeAlerts = append(activeAlerts, a)
			}
		}
		switch msg.Button {
		case tea.MouseWheelUp:
			if m.alertsCursor > 0 {
				m.alertsCursor--
				if m.alertsCursor < m.alertsScrollOffset {
					m.alertsScrollOffset = m.alertsCursor
				}
			}
		case tea.MouseWheelDown:
			if m.alertsCursor < len(activeAlerts)-1 {
				m.alertsCursor++
				visLines := m.alertsVisibleLines()
				if visLines > 0 && m.alertsCursor >= m.alertsScrollOffset+visLines {
					m.alertsScrollOffset = m.alertsCursor - visLines + 1
				}
			}
		}
		return m, nil
	}

	// Handle mouse wheel scrolling
	switch msg.Button {
	case tea.MouseWheelUp:
		// Scroll up based on current focus
		switch m.focused {
		case focusList:
			if m.list.Index() > 0 {
				m.list.Select(m.list.Index() - 1)
				// Sync detail panel in split view mode
				if m.isSplitView {
					m.updateViewportContent()
				}
			}
		case focusDetail:
			m.viewport.ScrollUp(3)
		case focusInsights:
			m.insightsPanel.MoveUp()
		case focusBoard:
			m.board.MoveUp()
		case focusGraph:
			m.graphView.PageUp()
		case focusTree:
			m.tree.MoveUp()
		case focusActionable:
			m.actionableView.MoveUp()
		case focusHistory:
			m.historyView.MoveUp()
		case focusFlowMatrix:
			m.flowMatrix.MoveUp()
		}
		return m, nil
	case tea.MouseWheelDown:
		// Scroll down based on current focus
		switch m.focused {
		case focusList:
			if m.list.Index() < len(m.list.Items())-1 {
				m.list.Select(m.list.Index() + 1)
				// Sync detail panel in split view mode
				if m.isSplitView {
					m.updateViewportContent()
				}
			}
		case focusDetail:
			m.viewport.ScrollDown(3)
		case focusInsights:
			m.insightsPanel.MoveDown()
		case focusBoard:
			m.board.MoveDown()
		case focusGraph:
			m.graphView.PageDown()
		case focusTree:
			m.tree.MoveDown()
		case focusActionable:
			m.actionableView.MoveDown()
		case focusHistory:
			m.historyView.MoveDown()
		case focusFlowMatrix:
			m.flowMatrix.MoveDown()
		}
		return m, nil
	}
	return m, nil
}

// handleWindowSize processes terminal resize events.
func (m Model) handleWindowSize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height
	m.isSplitView = msg.Width > SplitViewThreshold
	m.ready = true
	bodyHeight := m.height - 1 // keep 1 row for footer
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	if m.isSplitView {
		// Calculate dimensions accounting for 2 panels with borders(2)+padding(2) = 4 overhead each
		// Total overhead = 8
		availWidth := msg.Width - 8
		if availWidth < 10 {
			availWidth = 10
		}

		// Use configurable split ratio (default 0.4, adjustable via [ and ])
		listInnerWidth := int(float64(availWidth) * m.splitPaneRatio)
		detailInnerWidth := availWidth - listInnerWidth

		// listHeight fits header (1) + page line (1) inside a panel with Border (2)
		listHeight := bodyHeight - 4
		if listHeight < 3 {
			listHeight = 3
		}

		m.list.SetSize(listInnerWidth, listHeight)
		m.viewport = viewport.New(viewport.WithWidth(detailInnerWidth), viewport.WithHeight(bodyHeight-2)) // Account for border

		m.renderer.SetWidthWithTheme(detailInnerWidth, m.theme)
	} else {
		listHeight := bodyHeight - 2
		if listHeight < 3 {
			listHeight = 3
		}
		m.list.SetSize(msg.Width, listHeight)
		m.viewport = viewport.New(viewport.WithWidth(msg.Width), viewport.WithHeight(bodyHeight-1))

		// Update renderer for full width
		m.renderer.SetWidthWithTheme(msg.Width, m.theme)
	}

	m.updateListDelegate()

	// Resize label dashboard table and modal overlay sizing
	m.labelDashboard.SetSize(m.width, bodyHeight)

	m.insightsPanel.SetSize(m.width, bodyHeight)
	m.updateViewportContent()
	return m
}
