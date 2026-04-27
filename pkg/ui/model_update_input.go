package ui

// model_update_input.go contains Update() handlers for user input messages:
// tea.KeyPressMsg, tea.MouseWheelMsg, tea.WindowSizeMsg.
// Extracted from the main Update() switch to keep the router thin.

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/seanmartinsmith/beadstui/pkg/agents"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
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
				m.filter.labelFilter = m.labelDrilldownLabel
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
				m.filter.labelFilter = label
				m.applyFilter()
				m.statusMsg = fmt.Sprintf("Filtered to label %s (attention #%d)", label, idx+1)
				m.statusIsError = false
			}
			return m, nil
		}
	}

	// Handle alerts panel modal if open (bv-168, bt-46p6.10)
	if m.activeModal == ModalAlerts {
		// Tab switching + close-on-same-key at the top of the modal block
		// (bt-46p6.10). Runs before per-tab dispatch so these keys behave
		// consistently regardless of which tab has focus.
		switch msg.String() {
		case "tab":
			if m.activeTab == TabAlerts {
				m.activeTab = TabNotifications
			} else {
				m.activeTab = TabAlerts
			}
			return m, nil
		case "!":
			if m.activeTab == TabAlerts {
				m.resetAlertFilters()
				m.closeModal()
			} else {
				m.activeTab = TabAlerts
			}
			return m, nil
		case "1":
			if m.activeTab == TabNotifications {
				m.closeModal()
			} else {
				m.activeTab = TabNotifications
			}
			return m, nil
		}

		// Notifications tab: handle its own navigation + close; do NOT fall
		// through to the alerts handler below.
		if m.activeTab == TabNotifications {
			return m.handleNotificationsKey(msg)
		}

		activeAlerts := m.visibleAlerts()
		s := msg.String()
		switch s {
		case "j", "down":
			if m.alertsCursor < len(activeAlerts)-1 {
				m.alertsCursor++
			}
			return m, nil
		case "k", "up":
			if m.alertsCursor > 0 {
				m.alertsCursor--
			}
			return m, nil
		case "right", "l":
			// Page down
			pageSize := m.alertsVisibleLines()
			currentPageStart := (m.alertsCursor / pageSize) * pageSize
			target := currentPageStart + pageSize + pageSize - 1 // bottom of next page
			if target >= len(activeAlerts) {
				target = len(activeAlerts) - 1
			}
			m.alertsCursor = target
			return m, nil
		case "left", "h":
			// Page up
			pageSize := m.alertsVisibleLines()
			currentPageStart := (m.alertsCursor / pageSize) * pageSize
			target := currentPageStart - pageSize // top of previous page
			if target < 0 {
				target = 0
			}
			m.alertsCursor = target
			return m, nil
		case "enter":
			// Jump to the issue referenced by the selected alert and focus
			// the detail pane (bt-46p6.10 dogfood).
			if m.alertsCursor < len(activeAlerts) {
				issueID := activeAlerts[m.alertsCursor].IssueID
				if issueID != "" {
					// If issue's project is filtered out, add it to activeRepos so it becomes visible
					if m.workspaceMode && m.activeRepos != nil {
						if issue, ok := m.data.issueMap[issueID]; ok {
							repoKey := IssueRepoKey(*issue)
							if repoKey != "" && !m.activeRepos[repoKey] {
								m.activeRepos[repoKey] = true
								m.applyFilter()
							}
						}
					}
					// Filter-aware selection; resets list filter if needed to
					// avoid the Paginator-out-of-bounds crash (bt-nzsy class).
					if m.selectIssueByID(issueID) {
						m.focusDetailAfterJump()
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

				remaining := len(m.visibleAlerts())
				if m.alertsCursor >= remaining {
					m.alertsCursor = remaining - 1
				}
				if m.alertsCursor < 0 {
					m.alertsCursor = 0
				}
				if remaining == 0 {
					m.closeModal()
				}
			}
			return m, nil
		case "C":
			// Clear all visible alerts
			for _, a := range activeAlerts {
				m.dismissedAlerts[alertKey(a)] = true
			}

			m.alertsCursor = 0
			m.closeModal()
			return m, nil
		case "s":
			// Cycle severity filter: all → critical → warning → info → all
			switch m.alertFilterSeverity {
			case "":
				m.alertFilterSeverity = "critical"
			case "critical":
				m.alertFilterSeverity = "warning"
			case "warning":
				m.alertFilterSeverity = "info"
			default:
				m.alertFilterSeverity = ""
			}

			m.alertsCursor = 0
			return m, nil
		case "t":
			// Cycle type filter through active types
			types := m.alertActiveTypes()
			if len(types) == 0 {
				return m, nil
			}
			if m.alertFilterType == "" {
				m.alertFilterType = types[0]
			} else {
				idx := -1
				for i, t := range types {
					if t == m.alertFilterType {
						idx = i
						break
					}
				}
				if idx < 0 || idx >= len(types)-1 {
					m.alertFilterType = "" // wrap to all
				} else {
					m.alertFilterType = types[idx+1]
				}
			}

			m.alertsCursor = 0
			return m, nil
		case "p":
			// Cycle project filter through active projects
			projects := m.alertActiveProjects()
			if len(projects) == 0 {
				return m, nil
			}
			if m.alertFilterProject == "" {
				m.alertFilterProject = projects[0]
			} else {
				idx := -1
				for i, p := range projects {
					if p == m.alertFilterProject {
						idx = i
						break
					}
				}
				if idx < 0 || idx >= len(projects)-1 {
					m.alertFilterProject = "" // wrap to all
				} else {
					m.alertFilterProject = projects[idx+1]
				}
			}

			m.alertsCursor = 0
			return m, nil
		case "S":
			// Cycle severity filter BACKWARDS: all → info → warning → critical → all
			switch m.alertFilterSeverity {
			case "":
				m.alertFilterSeverity = "info"
			case "info":
				m.alertFilterSeverity = "warning"
			case "warning":
				m.alertFilterSeverity = "critical"
			default:
				m.alertFilterSeverity = ""
			}
			m.alertsCursor = 0
			return m, nil
		case "T":
			// Cycle type filter BACKWARDS through active types
			types := m.alertActiveTypes()
			if len(types) == 0 {
				return m, nil
			}
			if m.alertFilterType == "" {
				m.alertFilterType = types[len(types)-1]
			} else {
				idx := -1
				for i, t := range types {
					if t == m.alertFilterType {
						idx = i
						break
					}
				}
				if idx <= 0 {
					m.alertFilterType = "" // wrap to all
				} else {
					m.alertFilterType = types[idx-1]
				}
			}
			m.alertsCursor = 0
			return m, nil
		case "P":
			// Cycle project filter BACKWARDS through active projects
			projects := m.alertActiveProjects()
			if len(projects) == 0 {
				return m, nil
			}
			if m.alertFilterProject == "" {
				m.alertFilterProject = projects[len(projects)-1]
			} else {
				idx := -1
				for i, p := range projects {
					if p == m.alertFilterProject {
						idx = i
						break
					}
				}
				if idx <= 0 {
					m.alertFilterProject = "" // wrap to all
				} else {
					m.alertFilterProject = projects[idx-1]
				}
			}
			m.alertsCursor = 0
			return m, nil
		case "o":
			// Cycle sort: default → oldest → newest → default
			m.alertSortOrder = (m.alertSortOrder + 1) % 3
			m.alertsCursor = 0
			return m, nil
		case "O":
			// Cycle sort BACKWARDS: default → newest → oldest → default
			m.alertSortOrder = (m.alertSortOrder + 2) % 3
			m.alertsCursor = 0
			return m, nil
		case "r", "R":
			// Reset all filters
			m.resetAlertFilters()
			m.alertsCursor = 0
			return m, nil
		case "esc", "q":
			m.resetAlertFilters()
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
				m.list.Filter = multiTokenFilter(idPriorityFilter(m.semanticSearch.Filter))
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
				m.list.Filter = multiTokenFilter(idPriorityFilter(list.DefaultFilter))
				m.statusMsg = "Semantic search unavailable"
				m.statusIsError = true
			}
			if m.semanticHybridEnabled && !m.semanticHybridReady && !m.semanticHybridBuilding {
				m.semanticHybridBuilding = true
				cmds = append(cmds, BuildHybridMetricsCmd(m.issuesForAsync()))
			}
		} else {
			m.list.Filter = multiTokenFilter(idPriorityFilter(list.DefaultFilter))
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

	// Global / binding (bt-cd3x): in the split-view list layout, / from the
	// details pane (or any non-list focus) bounces focus to the list and the
	// router tail forwards / to the Bubbles list's Filter key. Remember prior
	// focus so esc can restore it. Skipped when the list isn't visible.
	if msg.String() == "/" &&
		m.activeModal == ModalNone &&
		m.mode == ViewList &&
		m.isSplitView &&
		m.list.FilterState() != list.Filtering &&
		m.focused != focusList {
		m.focusBeforeSearch = m.focused
		m.focused = focusList
		// Fall through: the router tail (Update) will forward msg to m.list.
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
				m.openInsightsView()
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
			// Open alerts modal on alerts tab (closed → open). Open-already
			// behavior (switch/close) lives in the modal block at line ~213.
			if len(m.visibleAlerts()) == 0 {
				m.statusMsg = "No active alerts"
				m.statusIsError = false
				return m, nil
			}
			m.activeTab = TabAlerts
			m.openModal(ModalAlerts)
			m.alertsCursor = 0
			m.resetAlertFilters()
			return m, nil

		case "1":
			// Open notifications modal (closed → open). Attention view's 1-9
			// label quick-jump is gated on m.mode == ViewAttention and handled
			// earlier at line ~196.
			if m.mode == ViewAttention {
				break
			}
			m.activeTab = TabNotifications
			m.openModal(ModalAlerts)
			m.notificationsCursor = 0
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
			// Set active labels so the picker opens to the current filter
			if m.filter.labelFilter != "" {
				m.labelPicker.SetActiveLabels(strings.Split(m.filter.labelFilter, ","))
			} else {
				m.labelPicker.SetActiveLabels(nil)
			}
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
				m.filter.labelFilter = selectedLabel
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
			// Intercept "/" so the search bar is reachable from the detail pane
			// (bt-jwo3 follow-up). Without this the keystroke goes to viewport
			// scroll, leaving users stuck navigating a single bead's body when
			// they meant to start a new query.
			if msg.String() == "/" {
				m.focused = focusList
				m.list.SetFilterState(list.Filtering)
				return m, nil
			}
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

// splitViewListChromeHeight returns the Y coordinate of the first list
// item (i.e., row 0 of m.list.Items()) relative to the top of the split
// view. The value is the number of terminal rows of chrome rendered above
// the first item (bt-58yw).
//
// Chrome layers, top to bottom:
//  1. RenderTitledPanel top border (always 1 row).
//  2. renderSearchPill (0 rows when FilterState != FilterApplied, can wrap
//     at narrow widths so measured via lipgloss.Height).
//  3. renderSplitView column header (the `TYPE PRI STATUS…` strip). The
//     header is clipped to listInnerWidth by splitViewHeader so it stays
//     1 row, but we still measure via lipgloss.Height as defense in depth
//     against future chrome changes (bt-i138).
//  4. Bubbles list phantom title row. SetShowTitle(false) alone is not
//     enough to suppress this: the list's View() method at
//     bubbles/v2/list/list.go:1048 enters its title-rendering branch
//     whenever SetFilteringEnabled(true) is in effect, and emits an empty
//     string. lipgloss.JoinVertical then treats "" as a 1-row blank line.
//     Always 1 row while filteringEnabled is true (bt's default).
func (m Model) splitViewListChromeHeight() int {
	const (
		panelTopBorder      = 1
		bubblesPhantomTitle = 1
	)
	offset := panelTopBorder + lipgloss.Height(m.splitViewHeader()) + bubblesPhantomTitle
	if pill := m.renderSearchPill(m.list.Width()); pill != "" {
		offset += lipgloss.Height(pill)
	}
	return offset
}

// handleMouseClick processes mouse button press events. Scoped to:
//   - split-view pane focus switching and list-row selection (bt-d8d1)
//   - alerts + notifications tabs inside the shared modal (bt-46p6.14)
//
// Other modals (RepoPicker, LabelPicker) remain keyboard-only; single-pane
// views pass through to preserve existing behavior.
func (m Model) handleMouseClick(msg tea.MouseClickMsg) (Model, tea.Cmd) {
	mouse := msg.Mouse()
	if mouse.Button != tea.MouseLeft {
		return m, nil
	}
	// Footer row is the last line — ignore clicks there.
	if mouse.Y >= m.height-1 {
		return m, nil
	}
	// Shared alerts / notifications modal owns its own click routing so the
	// backdrop stays no-op and row/tab interactions feel consistent with the
	// main split view (bt-46p6.14).
	if m.activeModal == ModalAlerts {
		return m.handleAlertsModalClick(mouse)
	}
	// Other modals (RepoPicker, LabelPicker) stay keyboard-only for now —
	// bt-km6d / bt-arf9 track that work separately.
	if m.activeModal != ModalNone {
		return m, nil
	}
	// Only the default list mode uses click-to-focus on list/detail. Other
	// view modes (insights, board, graph, etc.) keep keyboard-only navigation.
	if m.mode != ViewList {
		return m, nil
	}
	if !m.isSplitView {
		return m, nil
	}
	// Split-view layout: listInnerWidth on the left, detail on the right.
	// The panel has Border(2)+Padding(2) = 4-cell outer chrome per side. The
	// left boundary of the detail pane is roughly listInnerWidth + 4.
	listBoundary := m.list.Width() + 4
	switch {
	case mouse.X < listBoundary:
		if m.focused != focusList {
			m.focused = focusList
		}
		rowOffset := m.splitViewListChromeHeight()
		if mouse.Y >= rowOffset {
			row := mouse.Y - rowOffset + m.list.Paginator.Page*m.list.Paginator.PerPage
			items := m.list.Items()
			if row >= 0 && row < len(items) {
				m.list.Select(row)
				if m.isSplitView {
					m.updateViewportContent()
				}
			}
		}
	default:
		if m.focused != focusDetail {
			m.focused = focusDetail
			m.updateViewportContent()
		}
	}
	return m, nil
}

// handleMouseWheel processes mouse wheel events.
func (m Model) handleMouseWheel(msg tea.MouseWheelMsg) (Model, tea.Cmd) {
	// Intercept mouse wheel when alerts panel is open
	if m.activeModal == ModalAlerts {
		activeAlerts := m.visibleAlerts()
		switch msg.Button {
		case tea.MouseWheelUp:
			if m.alertsCursor > 0 {
				m.alertsCursor--
			}
		case tea.MouseWheelDown:
			if m.alertsCursor < len(activeAlerts)-1 {
				m.alertsCursor++
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

// handleNotificationsKey routes keypresses when the shared modal is on the
// notifications tab (bt-46p6.10). Mirrors the alerts-tab handler shape but
// reads live from events.RingBuffer and uses Dismiss() instead of the
// dismissedAlerts map. The !/1/tab keys are intercepted at the modal block
// before this handler is reached, so they're absent here.
func (m Model) handleNotificationsKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	activeNotifs := m.visibleNotifications()
	switch msg.String() {
	case "j", "down":
		if m.notificationsCursor < len(activeNotifs)-1 {
			m.notificationsCursor++
		}
		return m, nil
	case "k", "up":
		if m.notificationsCursor > 0 {
			m.notificationsCursor--
		}
		return m, nil
	case "enter":
		// Both keyboard enter and double-click on a notification share
		// activateCurrentModalItem so the deep-link semantics (workspace
		// reveal, filter-aware selection, detail focus, comment scroll for
		// EventCommented per bt-46p6.16) stay in one place.
		return m.activateCurrentModalItem()
	case "c":
		if m.notificationsCursor < len(activeNotifs) {
			m.events.Dismiss(activeNotifs[m.notificationsCursor].ID)
			remaining := len(m.visibleNotifications())
			if m.notificationsCursor >= remaining {
				m.notificationsCursor = remaining - 1
			}
			if m.notificationsCursor < 0 {
				m.notificationsCursor = 0
			}
			if remaining == 0 {
				m.closeModal()
			}
		}
		return m, nil
	case "C":
		m.events.DismissAll()
		m.notificationsCursor = 0
		m.closeModal()
		return m, nil
	case "d":
		// Toggle dismissed-event visibility (bt-46p6.13). Reset cursor since
		// the visible-list length changes and the previous index would point
		// at a different row.
		m.notifShowDismissed = !m.notifShowDismissed
		m.notificationsCursor = 0
		return m, nil
	case "esc", "q":
		m.closeModal()
		return m, nil
	}
	return m, nil
}

// modalDoubleClickWindow is the maximum interval between two clicks at the
// same position that still counts as a double-click. 500ms mirrors the
// OS-default across Windows/macOS/GNOME and was validated by hand on a
// range of trackpads during bt-46p6.14 dogfooding.
const modalDoubleClickWindow = 500 * time.Millisecond

// modalChromeAboveItems is the number of terminal rows above the first
// item inside the shared alerts/notifications modal. Layered top-to-bottom:
//  1. Panel top border (RenderTitledPanel, always 1 row).
//  2. Summary line ("N total · K critical · …" / "K created · K closed · …").
//  3. Blank separator written by the tab's "\n\n" after the summary.
//  4. Above-hint / filter-label line (always written, even when empty; the
//     renderer terminates the row with "\n" so it consumes 1 row either way).
//
// Items begin at modal row 4 (0-indexed). padContentLines applies horizontal
// padding only — it does NOT add a vertical pad row, contrary to the prior
// comment that put items at row 5 and produced a real-world off-by-one
// (bt-46p6.13 dogfooding caught this). TestProbeNotificationChrome dumps the
// rendered rows so future drift trips a visible failure rather than silently
// returning the row above the click.
const modalChromeAboveItems = 4

// handleAlertsModalClick routes a MouseClickMsg when the shared alerts /
// notifications modal is open (bt-46p6.14). Mirrors the keyboard handler
// semantics: clicking a row moves the cursor there; double-clicking the
// same row activates it (jumps to the referenced issue and closes the
// modal, same path as the enter key). Clicks outside the modal body are
// no-ops — esc / ! / 1 remain the only close paths.
func (m Model) handleAlertsModalClick(mouse tea.Mouse) (Model, tea.Cmd) {
	// OverlayCenter (pkg/ui/panel.go) composites the modal centered on the
	// background. Background width is m.width and background height is
	// m.height-1 (footer is rendered below it). The panel's outer size is
	// fixed by renderAlertsPanel: width = min(80, m.width-4), height set by
	// alertsPanelHeight.
	panelWidth := min(80, m.width-4)
	panelHeight := m.alertsPanelHeight()
	startRow := (m.height - 1 - panelHeight) / 2
	startCol := (m.width - panelWidth) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	mx := mouse.X - startCol
	my := mouse.Y - startRow
	if mx < 0 || mx >= panelWidth || my < 0 || my >= panelHeight {
		// Backdrop click: do NOT close (bead acceptance: "Click on modal
		// backdrop / outside the content area is a no-op, not a close").
		return m, nil
	}

	idx, ok := m.alertsModalItemAtY(my)
	if !ok {
		// Click landed on modal chrome (border, summary, hint line, footer,
		// padding). Consume but do not affect cursor or selection.
		return m, nil
	}

	now := time.Now()
	isDouble := !m.lastModalClickAt.IsZero() &&
		now.Sub(m.lastModalClickAt) <= modalDoubleClickWindow &&
		m.lastModalClickX == mouse.X &&
		m.lastModalClickY == mouse.Y
	m.lastModalClickAt = now
	m.lastModalClickX = mouse.X
	m.lastModalClickY = mouse.Y

	// Move cursor to clicked row.
	if m.activeTab == TabAlerts {
		m.alertsCursor = idx
	} else {
		m.notificationsCursor = idx
	}

	if !isDouble {
		return m, nil
	}
	// Double-click: activate (equivalent to enter). Reset the double-click
	// timer so a triple-click doesn't re-trigger activation on a closed modal.
	m.lastModalClickAt = time.Time{}
	return m.activateCurrentModalItem()
}

// alertsModalItemAtY maps a Y coordinate inside the shared modal (relative
// to the modal's top border) to an index in the currently visible item
// slice for the active tab. Returns (-1, false) when my points at chrome,
// padding, or the detail/summary line beneath the cursor (treating those
// regions as non-clickable keeps row math stable when the selected row
// expands to 2 lines).
func (m Model) alertsModalItemAtY(my int) (int, bool) {
	if my < modalChromeAboveItems {
		return -1, false
	}
	relY := my - modalChromeAboveItems

	if m.activeTab == TabAlerts {
		active := m.visibleAlerts()
		if len(active) == 0 {
			return -1, false
		}
		pageSize := m.alertsVisibleLines()
		if pageSize < 1 {
			return -1, false
		}
		start := (m.alertsCursor / pageSize) * pageSize
		end := start + pageSize
		if end > len(active) {
			end = len(active)
		}
		titleByID := m.alertIssueTitleMap()
		row := 0
		for i := start; i < end; i++ {
			if row == relY {
				return i, true
			}
			row++
			// The alerts tab renders a 1-row detail line beneath the selected
			// alert when either: the alert has an IssueID with a known title
			// (single-issue alerts), or it's a graph-scope alert with a
			// non-empty Details slice (dependency_loop, centrality_change,
			// etc., bt-7ye5). Mirror both predicates so click-to-row matches
			// the rendered layout. The inline alert-type definition was
			// removed by bt-xyjd, so it no longer factors in here.
			if i == m.alertsCursor {
				hasTitle := active[i].IssueID != "" && titleByID[active[i].IssueID] != ""
				hasDetails := active[i].IssueID == "" && len(active[i].Details) > 0
				if hasTitle || hasDetails {
					if row == relY {
						return -1, false // detail line — "click on selected" no-op
					}
					row++
				}
			}
		}
		return -1, false
	}

	// Notifications tab. pageSize differs by 1 from the alerts tab because
	// renderNotificationsTab reserves a row for the cursor-summary expand.
	active := m.visibleNotifications()
	if len(active) == 0 {
		return -1, false
	}
	pageSize := m.alertsVisibleLines() - 1
	if pageSize < 2 {
		pageSize = 2
	}
	start := (m.notificationsCursor / pageSize) * pageSize
	end := start + pageSize
	if end > len(active) {
		end = len(active)
	}
	row := 0
	for i := start; i < end; i++ {
		if row == relY {
			return i, true
		}
		row++
		if i == m.notificationsCursor {
			// Summary line is rendered only when Summary is non-empty after
			// newline-sanitization + trim (renderNotificationsTab, ~line 617).
			s := strings.TrimSpace(strings.ReplaceAll(active[i].Summary, "\n", " "))
			if s != "" {
				if row == relY {
					return -1, false
				}
				row++
			}
		}
	}
	return -1, false
}

// alertIssueTitleMap returns a {issue_id → title} lookup drawn from the
// list items, matching the map renderAlertsTab builds inline. Extracted
// so the click handler can apply the same "has-detail-line?" predicate
// without duplicating IssueItem iteration logic.
func (m Model) alertIssueTitleMap() map[string]string {
	titles := make(map[string]string, len(m.list.Items()))
	for _, item := range m.list.Items() {
		if it, ok := item.(IssueItem); ok {
			titles[it.Issue.ID] = it.Issue.Title
		}
	}
	return titles
}

// activateCurrentModalItem mirrors the enter-key path for both alerts and
// notifications tabs: jumps to the referenced bead (reveal-filtered if the
// project was hidden in workspace mode), focuses the detail pane, and
// closes the modal. Split out from the key handler so double-click can
// reuse the exact same activation contract.
func (m Model) activateCurrentModalItem() (Model, tea.Cmd) {
	if m.activeTab == TabAlerts {
		activeAlerts := m.visibleAlerts()
		if m.alertsCursor >= 0 && m.alertsCursor < len(activeAlerts) {
			alert := activeAlerts[m.alertsCursor]
			// Graph-scope alerts have no single issue target — the value is in
			// the rankings themselves. Route to the insights view so the user
			// lands on the data the alert is summarizing (bt-46p6.12).
			if alert.Type == drift.AlertCentralityChange {
				m.openInsightsView()
				m.closeModal()
				return m, nil
			}
			if alert.IssueID != "" {
				m.revealBeadIfHidden(alert.IssueID)
				if m.selectIssueByID(alert.IssueID) {
					m.focusDetailAfterJump()
				}
			}
		}
		m.closeModal()
		return m, nil
	}
	activeNotifs := m.visibleNotifications()
	if m.notificationsCursor >= 0 && m.notificationsCursor < len(activeNotifs) {
		notif := activeNotifs[m.notificationsCursor]
		if notif.BeadID != "" {
			m.revealBeadIfHidden(notif.BeadID)
			if m.selectIssueByID(notif.BeadID) {
				// Comment-event deep-link (bt-46p6.16): set the pending
				// scroll target BEFORE focusDetailAfterJump runs, since
				// that helper invokes updateViewportContent which is the
				// surface that consumes pendingCommentScroll.
				if notif.Kind == events.EventCommented && !notif.CommentAt.IsZero() {
					m.pendingCommentScroll = notif.CommentAt
				}
				m.focusDetailAfterJump()
			}
		}
	}
	m.closeModal()
	return m, nil
}

// openInsightsView switches into the insights view and rebuilds the panel
// from the current snapshot. Shared by the "i" key toggle and the
// alert-modal enter path for graph-scope alerts (bt-46p6.12).
func (m *Model) openInsightsView() {
	m.clearAttentionOverlay()
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

// revealBeadIfHidden unhides the bead's repo in workspace mode so a jump
// from the modal doesn't land on a filtered-out issue. No-op when not in
// workspace mode or when the repo is already active. Matches the inline
// reveal blocks in the alerts/notifications enter handlers (bt-46p6.10).
func (m *Model) revealBeadIfHidden(beadID string) {
	if !m.workspaceMode || m.activeRepos == nil {
		return
	}
	issue, ok := m.data.issueMap[beadID]
	if !ok {
		return
	}
	repoKey := IssueRepoKey(*issue)
	if repoKey == "" || m.activeRepos[repoKey] {
		return
	}
	m.activeRepos[repoKey] = true
	m.applyFilter()
}
