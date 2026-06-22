package ui

// model_update_input.go contains Update() handlers for user input messages:
// tea.KeyPressMsg, tea.MouseWheelMsg, tea.WindowSizeMsg.
// Extracted from the main Update() switch to keep the router thin.

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/seanmartinsmith/beadstui/pkg/agents"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/debug"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// currentSearchMode derives the active search ranker from the underlying
// boolean state. The Ctrl+S cycle constrains valid combinations so the
// dead-corner state (semantic off + hybrid on) is unreachable in normal
// operation; if it ever appears (e.g. legacy state from a prior session),
// it's treated as fuzzy and the next cycle press normalizes it.
func (m Model) currentSearchMode() searchMode {
	if m.semanticSearchEnabled && m.semanticHybridEnabled {
		return searchModeHybrid
	}
	if m.semanticSearchEnabled {
		return searchModeSemantic
	}
	return searchModeFuzzy
}

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
	m.statusSeverity = SeverityNone
	m.statusSetAt = time.Time{}

	// Safety net: if focus is anywhere but the list while the Bubbles filter
	// is still in Filtering state, commit it to FilterApplied so global
	// hotkeys and Tab work correctly. This catches any focus-change path
	// (mouse click, programmatic) that didn't call commitFilterIfTyping
	// directly. See bt-ocmw.
	if m.focused != focusList {
		m.commitFilterIfTyping()
	}

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
				m.setFailure("Failed to update " + filepath.Base(filePath) + ": " + err.Error())
			} else {
				m.setStatus("✓ Added beads instructions to " + filepath.Base(filePath))
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

	// Handle the tier-2 epic focus card (bt-gfxhz.3). Swallows all keys while
	// open; esc closes and restores the underlying surface (overview or list).
	if m.activeModal == ModalEpicCard {
		m = m.handleEpicCardKeys(msg)
		return m, nil
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
				m.setStatus(fmt.Sprintf("Filtered to label %s (attention #%d)", label, idx+1))
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
	if key.Matches(msg, m.keys.Global.Help) && m.list.FilterState() != list.Filtering {
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
	if key.Matches(msg, m.keys.Global.Tutorial) && m.list.FilterState() != list.Filtering {
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
	if key.Matches(msg, m.keys.Global.Refresh) && m.list.FilterState() != list.Filtering {
		now := time.Now()
		if !m.data.lastForceRefresh.IsZero() && now.Sub(m.data.lastForceRefresh) < time.Second {
			return m, nil
		}
		m.data.lastForceRefresh = now

		m.setStatus("Refreshing…")

		if m.data.backgroundWorker != nil {
			m.data.backgroundWorker.ForceRefresh()
			cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.data.backgroundWorker))
			return m, tea.Batch(cmds...)
		}

		if m.data.beadsPath == "" && m.data.watcher == nil && !m.isDoltSource() {
			m.setFailure("Refresh unavailable")
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

	// Undocumented: ctrl+p toggles a top-right [WxH] dimension chip so
	// dimension-sensitive bugs can be reported with exact terminal cell
	// counts. Bypasses the keymap so it stays out of `?` help and is
	// effectively hidden from users who don't know to press it.
	if msg.String() == "ctrl+p" && m.list.FilterState() != list.Filtering {
		m.showDebugDims = !m.showDebugDims
		return m, nil
	}

	// Handle shortcuts sidebar toggle (; or F2) - bv-3qi5
	if key.Matches(msg, m.keys.Global.Sidebar) && m.list.FilterState() != list.Filtering {
		m.showShortcutsSidebar = !m.showShortcutsSidebar
		if m.showShortcutsSidebar {
			m.shortcutsSidebar.ResetScroll()
			m.setStatus("Shortcuts sidebar: ; hide | ctrl+j/k scroll")
		} else {
			m.setStatus("")
		}
		// Re-flow layout so list/viewport reserve (or release) sidebar
		// width. Without this the toggle leaves stale dimensions and the
		// sidebar wraps onto the body (bt-lin9). Use the full immediate
		// path (both phases) since this is a discrete user action, not a
		// drag burst (bt-kfkrb).
		var resizeCmd tea.Cmd
		m, resizeCmd = m.handleWindowSize(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		m = m.applyWindowSizeHeavy()
		_ = resizeCmd // discard the settle tick - heavy path already ran
		return m, nil
	}

	// Handle shortcuts sidebar scrolling (Ctrl+j/k when sidebar visible) - bv-3qi5
	if m.showShortcutsSidebar && m.list.FilterState() != list.Filtering {
		switch {
		case key.Matches(msg, m.keys.Global.SidebarScrollDown):
			m.shortcutsSidebar.ScrollDown()
			return m, nil
		case key.Matches(msg, m.keys.Global.SidebarScrollUp):
			m.shortcutsSidebar.ScrollUp()
			return m, nil
		}
	}

	// H = hybrid preset cycle (bt-krwp). Only meaningful when in hybrid mode;
	// outside hybrid it surfaces a status hint redirecting to Ctrl+S. The
	// previous H = "toggle hybrid layer" + alt+H = "cycle preset" pair was
	// collapsed into a single Ctrl+S three-state cycle (fuzzy → hybrid →
	// semantic) so this binding could be reclaimed for preset cycling, which
	// is the only operation that doesn't already have a key.
	if m.focused == focusList && m.list.FilterState() != list.Filtering && key.Matches(msg, m.keys.Global.HybridPreset) {
		if !m.semanticHybridEnabled {
			m.setStatus("Not in hybrid mode — press Ctrl+S to cycle there")
			return m, nil
		}
		m.semanticHybridPreset = nextHybridPreset(m.semanticHybridPreset)
		if m.semanticSearch != nil {
			m.semanticSearch.SetHybridConfig(true, m.semanticHybridPreset)
			m.semanticSearch.ResetCache()
		}
		m.clearSemanticScores()
		m.setStatus(fmt.Sprintf("Hybrid preset: %s", m.semanticHybridPreset))
		if m.list.FilterState() != list.Unfiltered {
			currentTerm := m.list.FilterInput.Value()
			if currentTerm != "" && !m.semanticHybridBuilding {
				cmds = append(cmds, ComputeSemanticFilterCmd(m.semanticSearch, currentTerm))
			}
		}
		m.updateListDelegate()
		return m, tea.Batch(cmds...)
	}

	// Ctrl+S cycles search modes: fuzzy → hybrid → semantic → fuzzy (bt-krwp).
	// Single key, three states. The previous binary toggle (Ctrl+S = semantic
	// on/off, H = hybrid on/off) had a dead corner state — hybrid on +
	// semantic off was a no-op gated inside the ranker but the H binding
	// silently flipped the bit. Cycle order puts hybrid one keystroke from
	// fuzzy because hybrid is the most useful daily mode; semantic-text-only
	// is the niche case (skip the graph signal) reachable with two presses.
	if key.Matches(msg, m.keys.Global.SearchMode) && m.focused == focusList {
		next := nextSearchMode(m.currentSearchMode())

		// Non-fuzzy modes need the semantic backend wired up. Fail fast if
		// not, leaving state as fuzzy.
		if next != searchModeFuzzy && m.semanticSearch == nil {
			m.semanticSearchEnabled = false
			m.semanticHybridEnabled = false
			m.list.Filter = fuzzySearchFilter()
			m.setFailure("Semantic search unavailable")
			m.clearSemanticScores()
			m.updateListDelegate()
			return m, nil
		}

		switch next {
		case searchModeFuzzy:
			m.semanticSearchEnabled = false
			m.semanticHybridEnabled = false
			m.list.Filter = fuzzySearchFilter()
			m.setStatus("Fuzzy search — fast substring/character match, best for IDs and known phrases")
			m.clearSemanticScores()
			if m.semanticSearch != nil {
				m.semanticSearch.SetHybridConfig(false, m.semanticHybridPreset)
			}
		case searchModeSemantic:
			m.semanticSearchEnabled = true
			m.semanticHybridEnabled = false
			m.semanticSearch.SetHybridConfig(false, m.semanticHybridPreset)
			m.semanticSearch.ResetCache()
			m.list.Filter = semanticSearchFilter(m.semanticSearch)
			if !m.semanticSearch.Snapshot().Ready && !m.semanticIndexBuilding {
				m.semanticIndexBuilding = true
				m.setStatus("Semantic search: building index…")
				cmds = append(cmds, BuildSemanticIndexCmd(m.issuesForAsync()))
			} else if m.semanticIndexBuilding {
				m.setStatus("Semantic search: indexing…")
			} else {
				m.setStatus("Semantic search — finds items by meaning, use when fuzzy misses the right bead")
			}
		case searchModeHybrid:
			m.semanticSearchEnabled = true
			m.semanticHybridEnabled = true
			m.semanticSearch.SetHybridConfig(true, m.semanticHybridPreset)
			m.semanticSearch.ResetCache()
			m.list.Filter = semanticSearchFilter(m.semanticSearch)
			switch {
			case !m.semanticSearch.Snapshot().Ready && !m.semanticIndexBuilding:
				m.semanticIndexBuilding = true
				m.setStatus("Hybrid search: building index…")
				cmds = append(cmds, BuildSemanticIndexCmd(m.issuesForAsync()))
			case !m.semanticHybridReady && !m.semanticHybridBuilding:
				m.semanticHybridBuilding = true
				m.setStatus("Hybrid search: computing metrics…")
				cmds = append(cmds, BuildHybridMetricsCmd(m.issuesForAsync()))
			default:
				m.setStatus(fmt.Sprintf("Hybrid search [preset: %s] — semantic + graph weight, best general-purpose mode", m.semanticHybridPreset))
			}
		}

		// Recompute scores for the active filter term if any.
		if m.semanticSearchEnabled && m.list.FilterState() != list.Unfiltered {
			currentTerm := m.list.FilterInput.Value()
			if currentTerm != "" && !m.semanticHybridBuilding && !m.semanticIndexBuilding {
				cmds = append(cmds, ComputeSemanticFilterCmd(m.semanticSearch, currentTerm))
			}
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

	// Non-modal in-view typing/dwell sub-states are input-capturing contexts:
	// while one is active, every key must reach the active view's handler
	// before the global view-switch dispatcher below, or letters that collide
	// with global hotkeys (b, g, h, i, p, a, E, ...) silently switch views
	// instead of being typed or consumed in-pane. History search is the
	// original case (bt-mc4y); board search and history file-tree focus are
	// the same class (bt-s2xpy). Each handler owns its own esc/enter/tab
	// resolution (esc cancels the sub-state rather than leaving the view), so
	// the user is never trapped; only ctrl+c bypasses for quit. Modals route
	// earlier (activeModal != ModalNone), so this is scoped to non-modal views.
	if m.activeModal == ModalNone {
		switch {
		case m.mode == ViewHistory && (m.historyView.IsSearchActive() || m.historyView.FileTreeHasFocus()):
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m = m.handleHistoryKeys(msg)
			return m, nil
		case m.mode == ViewBoard && m.board.IsSearchMode():
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m = m.handleBoardKeys(msg)
			return m, nil
		}
	}

	// Global / binding (bt-cd3x): in the split-view list layout, / from the
	// details pane (or any non-list focus) bounces focus to the list and the
	// router tail forwards / to the Bubbles list's Filter key. Remember prior
	// focus so esc can restore it. Skipped when the list isn't visible.
	if key.Matches(msg, m.keys.Global.SearchBounce) &&
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

	// Quit (Ctrl+C) lifted above the filter-state guard so it works
	// mid-filter. The prior placement inside the not-filtering switch
	// silently swallowed Ctrl+C while the user was typing a filter
	// query — surfaced during bt-ift6.1 dogfood, absorbed during
	// bt-ift6.2 while this code path is open.
	if key.Matches(msg, m.keys.Global.Quit) {
		return m, tea.Quit
	}

	// Handle keys when not filtering
	if m.list.FilterState() != list.Filtering {
		// View-switch globals dispatched via key.Matches against
		// m.keys.Global (ADR-004 Decision 1). The prior tab / < / > cases
		// moved into list_keys.go's handleListKeys per the no-match-and-
		// fall-through rule.
		switch {
		case key.Matches(msg, m.keys.Global.Back):
			// q: context-aware cascade. Single binding, single Help.Desc;
			// the cascade lives here in the dispatcher (ADR-004 Decision 1).
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
			if m.mode == ViewEpics {
				m.mode = ViewList
				m.focused = focusList
				return m, nil
			}
			return m, tea.Quit

		case key.Matches(msg, m.keys.Global.Cancel):
			// esc: context-aware cascade closing modals and going back.
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
			if m.mode == ViewEpics {
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

		case key.Matches(msg, m.keys.Global.Board):
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

		case key.Matches(msg, m.keys.Global.Graph):
			// Toggle graph view
			m.clearAttentionOverlay()
			if m.mode == ViewGraph {
				m.mode = ViewList
				m.focused = focusList
			} else {
				// Carry the list's current selection into the graph so
				// the ego node matches what the user was looking at, instead
				// of defaulting to the first node in sorted order. Reads
				// m.list.SelectedItem() so it works from focusList and
				// focusDetail alike (selection is shared) (bt-8col).
				var selectedID string
				if sel := m.list.SelectedItem(); sel != nil {
					if it, ok := sel.(IssueItem); ok {
						selectedID = it.Issue.ID
					}
				}
				m.mode = ViewGraph
				m.focused = focusGraph
				m.refreshBoardAndGraphForCurrentFilter()
				if selectedID != "" {
					m.graphView.SelectByID(selectedID)
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.Global.Actionable):
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

		case key.Matches(msg, m.keys.Global.Tree):
			// Toggle hierarchical tree view (bv-gllx)
			m.clearAttentionOverlay()
			if m.mode == ViewTree {
				m.mode = ViewList
				m.focused = focusList
			} else {
				m.mode = ViewTree
				// Build tree from snapshot when available (bv-t435), routing
				// through the activeRepos-aware helper so the tree honors the
				// workspace project filter (bt-dcby.2).
				m.rebuildTreeForCurrentFilter()
				m.tree.SetSize(m.width, m.height-2)
				// Sync tree cursor to the issues-pane selection so pressing T
				// lands on the bead the user was looking at, with its ancestor
				// path expanded so it is immediately visible (bt-w8j8).
				// ExpandPathTo first (rebuilds flatList), then SelectByID
				// (sets cursor index after rebuild).
				if sel := m.list.SelectedItem(); sel != nil {
					if issueItem, ok := sel.(IssueItem); ok {
						m.tree.ExpandPathTo(issueItem.Issue.ID)
						m.tree.SelectByID(issueItem.Issue.ID)
					}
				}
				m.focused = focusTree
			}
			return m, nil

		case key.Matches(msg, m.keys.Global.Insights):
			m.clearAttentionOverlay()
			if m.mode == ViewInsights {
				// Capture cursor on toggle-out so the next `i` restores
				// the same pane and row (bt-fdwz).
				panel := m.insightsPanel.FocusedPanel()
				m.insightsCursor = insightsCursor{
					panel: panel,
					index: m.insightsPanel.SelectedIndexFor(panel),
					valid: true,
				}
				m.mode = ViewList
				m.focused = focusList
			} else {
				m.openInsightsView()
			}
			return m, nil

		case key.Matches(msg, m.keys.Global.PriorityHints):
			// Toggle priority hints
			m.ac.showPriorityHints = !m.ac.showPriorityHints
			// Update delegate with new state
			m.updateListDelegate()
			// Show explanatory status message
			if m.ac.showPriorityHints {
				count := len(m.ac.priorityHints)
				if count > 0 {
					m.setStatus(fmt.Sprintf("Priority hints: ↑ increase ↓ decrease (%d suggestions)", count))
				} else {
					m.setStatus("Priority hints: No misalignments detected (analysis ongoing)")
				}
			} else {
				m.setStatus("")
			}
			return m, nil

		case key.Matches(msg, m.keys.Global.History):
			// Toggle history view. Routes through enterHistoryView so the
			// per-bead resolveHistoryPath registry lookup (bt-u8iz Phase 3)
			// runs - the previous direct mode-flip used the stale historyView
			// preloaded by LoadHistoryCmd against cwd, which left InsideWorkTree
			// false in global mode and short-circuited registry resolution.
			//
			// Async dispatch (bt-uizm): enterHistoryView now returns a tea.Cmd
			// that loads the report off the event loop, so the keypress no
			// longer blocks the UI for seconds while git history is parsed.
			m.clearAttentionOverlay()
			if m.mode == ViewHistory {
				m.mode = ViewList
				m.focused = focusList
				m.historyDoltOnly = false
				return m, nil
			}
			return m, m.enterHistoryView()

		case key.Matches(msg, m.keys.Global.LabelDashboard):
			// Toggle label dashboard (phase 1: table view). Second press returns
			// to the list (bt-yzfp2), restoring the split view that entry clears,
			// mirroring the Esc cascade.
			m.clearAttentionOverlay()
			if m.mode == ViewLabelDashboard {
				m.mode = ViewList
				m.isSplitView = true
				m.focused = focusList
				return m, nil
			}
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
			m.setStatus(fmt.Sprintf("Labels: %d total • critical %d • warning %d", m.labelHealthCache.TotalLabels, m.labelHealthCache.CriticalCount, m.labelHealthCache.WarningCount))
			return m, nil

		case key.Matches(msg, m.keys.Global.Attention):
			// Toggle attention view. Second press returns to the list (bt-yzfp2).
			if m.mode == ViewAttention {
				m.mode = ViewList
				m.focused = focusList
				return m, nil
			}
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

		case key.Matches(msg, m.keys.Global.FlowMatrix):
			// Toggle flow matrix view (cross-label dependencies). Second press
			// returns to the list (bt-yzfp2).
			m.clearAttentionOverlay()
			if m.mode == ViewFlowMatrix {
				m.mode = ViewList
				m.focused = focusList
				return m, nil
			}
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

		case key.Matches(msg, m.keys.Global.Epics):
			// Toggle epics overview. Second press returns to the list,
			// mirroring the other view toggles (bt-ryi5z).
			m.clearAttentionOverlay()
			if m.mode == ViewEpics {
				m.mode = ViewList
				m.focused = focusList
				return m, nil
			}
			m.mode = ViewEpics
			m.focused = focusEpics
			m.epicsTree.cursor = 0
			m.refreshEpicsForCurrentFilter()
			return m, nil

		case key.Matches(msg, m.keys.Global.Alerts):
			// Open alerts modal on alerts tab (closed → open). Open-already
			// behavior (switch/close) lives in the modal block at line ~213.
			if len(m.visibleAlerts()) == 0 {
				m.setStatus("No active alerts")
				return m, nil
			}
			m.activeTab = TabAlerts
			m.openModal(ModalAlerts)
			m.alertsCursor = 0
			m.resetAlertFilters()
			return m, nil

		case key.Matches(msg, m.keys.Global.Notifications):
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

		case key.Matches(msg, m.keys.Global.BQL):
			// Open BQL query modal
			m.bqlQuery.SetSize(m.width, m.height-1)
			m.bqlQuery.Reset()
			m.openModal(ModalBQLQuery)
			m.focused = focusBQLQuery
			return m, m.bqlQuery.Focus()

		case key.Matches(msg, m.keys.Global.Recipes):
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

		case key.Matches(msg, m.keys.Global.WorkspaceHomeAll):
			// Quick toggle between current project and all projects
			if !m.workspaceMode || len(m.availableRepos) == 0 {
				m.setStatus("Project filter available only in multi-project mode")
				return m, nil
			}
			if m.currentProjectDB == "" {
				m.setStatus("No home project detected (not in a beads directory)")
				return m, nil
			}
			if m.activeRepos != nil {
				// Currently filtered - expand to all
				m.activeRepos = nil
				m.setStatus("Showing all projects")
			} else {
				// Currently showing all - filter to home project
				m.activeRepos = map[string]bool{m.currentProjectDB: true}
				m.setStatus(fmt.Sprintf("Showing project: %s", m.currentProjectDB))
			}
			if m.filter.activeRecipe != nil {
				m.applyRecipe(m.filter.activeRecipe)
			} else {
				m.applyFilter()
			}
			return m, nil

		case key.Matches(msg, m.keys.Global.ProjectsOrWisps):
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

		case key.Matches(msg, m.keys.Global.Export):
			// Export to Markdown file
			m.exportToMarkdown()
			return m, nil

		case key.Matches(msg, m.keys.Global.LabelPicker):
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

		case focusEpics:
			m = m.handleEpicsKeys(msg)

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
			// Action keys advertised in the shortcuts sidebar's "Actions" group
			// (context: list/detail/split). These all read m.list.SelectedItem()
			// or change global mode, so focus is irrelevant - without this
			// dispatch they get swallowed by viewport.Update (bt-x5b7).
			//
			// "tab", "<", ">" are forwarded to handleListKeys per ADR-004
			// Decision 1 — they migrated from the global dispatcher to
			// list-scoped bindings as part of bt-ift6.1's no-match-and-fall-
			// through cleanup. Without this forwarding, pressing tab from
			// focusDetail would reach viewport.Update instead of toggling
			// focus back to focusList.
			//
			// The forwarded set is dispatched via key.Matches against the
			// same ListNormalKeys bindings handleListKeys consumes, so this
			// gate cannot drift from the per-view Map (bt-ift6.2).
			ln := m.keys.ListNormal
			switch {
			case key.Matches(msg,
				ln.CopyID, ln.CopyIssue, ln.OpenInEditor, ln.RecipeTriage,
				ln.TimeTravelInput, ln.EpicCard,
				ln.SelfUpdate, ln.CassSession,
				ln.SplitFocusToggle, ln.SplitShrinkLeft, ln.SplitShrinkRight):
				m = m.handleListKeys(msg)
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
// Chrome layers, top to bottom (post bt-fxbl unification):
//  1. RenderTitledPanel top border (always 1 row).
//  2. renderSearchRow (always 1 row — bt-fxbl made this fixed-height across
//     all FilterStates so the chrome below never shifts; rendered via
//     m.renderSearchRow, measured via lipgloss.Height for defense in depth
//     against future styling that could wrap).
//  3. renderSplitView column header (the `TYPE PRI STATUS…` strip, 1 row;
//     clipped to listInnerWidth so it never wraps, bt-i138).
//
// Note: there is no longer a Bubbles "phantom title row" — bt-fxbl set
// l.SetShowFilter(false) in NewModel, which (combined with
// SetShowTitle(false)) skips Bubbles' titleView path entirely in
// list.View() at bubbles/v2/list/list.go:1048. This collapses what used
// to be 4 chrome layers into 3.
func (m Model) splitViewListChromeHeight() int {
	const panelTopBorder = 1
	offset := panelTopBorder
	offset += lipgloss.Height(m.renderSearchRow(m.list.Width()))
	offset += lipgloss.Height(m.splitViewHeader())
	return offset
}

// handleMouseClick processes mouse button press events. Scoped to:
//   - split-view pane focus switching and list-row selection (bt-d8d1)
//   - alerts + notifications tabs inside the shared modal (bt-46p6.14)
//   - labels and project picker modals (bt-wnda, bt-hpsq)
//
// Other modals (BQL query, agent prompt, etc.) consume the click as a no-op
// so it doesn't bleed through to the background. Single-pane views pass
// through to preserve existing behavior.
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
	if m.activeModal == ModalLabelPicker {
		return m.handleLabelPickerModalClick(mouse)
	}
	if m.activeModal == ModalRepoPicker {
		return m.handleRepoPickerModalClick(mouse)
	}
	if m.activeModal != ModalNone {
		return m, nil
	}
	// History view click handling (bt-y3ip): route to HistoryModel.ClickAt,
	// which encapsulates the per-pane layout math. Click on a bead row in
	// BEADS WITH HISTORY moves the cursor; click in a content row of the
	// COMMITS pane selects the commit; click on COMMIT DETAILS focuses the
	// pane (Tab equivalent). Borders, headers, and the timeline pane are
	// focus-only.
	if m.mode == ViewHistory {
		hit := m.historyView.ClickAt(mouse.X, mouse.Y)
		if hit.Pane == noPane {
			return m, nil
		}
		m.historyView.SetFocus(hit.Pane)
		// focusHistory wraps every history-view focus state at the Model
		// level; the inner pane is tracked on HistoryModel.focused.
		m.focused = focusHistory
		if hit.HasItem {
			switch hit.Pane {
			case historyFocusList:
				m.historyView.SelectBead(hit.Item)
			case historyFocusMiddle:
				if m.historyView.IsGitMode() {
					m.historyView.SelectRelatedBead(hit.Item)
				} else {
					m.historyView.SelectCommit(hit.Item)
				}
			}
		}
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
		// Click on the search row reopens the filter input for editing
		// (bt-49nn). Chrome layers above the first list item are: panel
		// top border (Y=0), search row (Y=1), column header (Y=2). A
		// click at Y=1 anywhere in the list pane should route to filter
		// reopen instead of selecting a row. Mirrors the detail-pane "/"
		// shortcut at the focusDetail handler above (bt-jwo3): preserves
		// any existing FilterValue and just flips state to Filtering.
		//
		// We forward a synthetic "/" keypress to the Bubbles list rather
		// than calling SetFilterState directly. SetFilterState alone
		// flips the state flag but skips Bubbles' filter-begin setup
		// (populating filteredItems with all items when buffer is empty,
		// GoToStart, FilterInput.Focus/CursorEnd, updateKeybindings) —
		// without that, an empty buffer in Filtering state renders as
		// "no matches" instead of "all visible". The keyboard `/` path
		// goes through Update naturally; the click path now matches it
		// (bt-r2ev Bug A).
		const searchRowY = 1
		if mouse.Y == searchRowY {
			if m.list.FilterState() != list.Filtering {
				// Capture cursor before filter-begin runs (bt-qka1); mirrors the
				// keyboard "/" restore path in model.go Update. Bubbles' filter-
				// begin calls GoToStart, which resets the visible cursor to 0.
				// After the synthetic keypress with an empty buffer, VisibleItems
				// contains all items, so the captured index is still valid.
				savedIdx := m.list.Index()
				m.list, _ = m.list.Update(tea.KeyPressMsg{Code: '/'})
				// Restore cursor; clamp in case list somehow shrank.
				if m.list.FilterState() == list.Filtering {
					visible := m.list.VisibleItems()
					restoreIdx := savedIdx
					if restoreIdx >= len(visible) {
						restoreIdx = len(visible) - 1
					}
					if restoreIdx >= 0 {
						m.list.Select(restoreIdx)
					}
				}
			}
			return m, nil
		}
		rowOffset := m.splitViewListChromeHeight()
		if mouse.Y >= rowOffset {
			mouseRow := mouse.Y - rowOffset
			// Bound mouseRow against the rows actually rendered on the current
			// page, not against len(visible). At large unfiltered lists, a
			// click in the gap between the last rendered row and the footer
			// otherwise computes a `row` index that's still < len(visible) and
			// triggers Select() into a later page (bt-9kj7, sister of bt-0lsm).
			visible := m.list.VisibleItems()
			perPage := m.list.Paginator.PerPage
			pageStart := m.list.Paginator.Page * perPage
			remainingOnPage := len(visible) - pageStart
			if remainingOnPage > perPage {
				remainingOnPage = perPage
			}
			if mouseRow >= 0 && mouseRow < remainingOnPage {
				// Commit any in-progress filter before selecting the row, so
				// the click commits + selects in one gesture. Without this,
				// a click on a row while in Filtering state keeps focus on
				// focusList and bypasses commitFilterIfTyping (bt-ocmw),
				// leaving the user stuck in Filtering (bt-r2ev Bug B).
				m.commitFilterIfTyping()
				row := mouseRow + pageStart
				m.list.Select(row)
				if m.isSplitView {
					m.updateViewportContent()
				}
			} else {
				// Click landed in the gap between the last rendered row and
				// the footer. Same fix-shape as a row click: commit any
				// in-progress filter so the gesture isn't a dead-zone for
				// users in Filtering state (bt-r2ev Bug B).
				m.commitFilterIfTyping()
			}
		}
	default:
		if m.focused != focusDetail {
			// Commit any in-progress filter so the search input releases and
			// global hotkeys/Tab work from the detail pane (bt-ocmw).
			m.commitFilterIfTyping()
			m.focused = focusDetail
			m.updateViewportContent()
		}
	}
	return m, nil
}

// handleMouseWheel processes mouse wheel events.
func (m Model) handleMouseWheel(msg tea.MouseWheelMsg) (Model, tea.Cmd) {
	// Intercept mouse wheel when the shared alerts/notifications modal is open.
	// The modal hosts two tabs (alerts + notifications) with separate cursor
	// state — route the wheel by activeTab so notifications scrolls too
	// (bt-tftj).
	if m.activeModal == ModalAlerts {
		switch m.activeTab {
		case TabNotifications:
			activeNotifs := m.visibleNotifications()
			switch msg.Button {
			case tea.MouseWheelUp:
				if m.notificationsCursor > 0 {
					m.notificationsCursor--
				}
			case tea.MouseWheelDown:
				if m.notificationsCursor < len(activeNotifs)-1 {
					m.notificationsCursor++
				}
			}
		default:
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
		}
		return m, nil
	}

	// Label picker modal owns wheel scrolling so the user can scroll through
	// the (potentially hundreds of) labels with the trackpad (bt-wnda).
	if m.activeModal == ModalLabelPicker {
		switch msg.Button {
		case tea.MouseWheelUp:
			m.labelPicker.MoveUp()
		case tea.MouseWheelDown:
			m.labelPicker.MoveDown()
		}
		return m, nil
	}

	// Repo (project) picker mirrors the labels-modal wheel pattern (bt-hpsq).
	if m.activeModal == ModalRepoPicker {
		switch msg.Button {
		case tea.MouseWheelUp:
			m.repoPicker.MoveUp()
		case tea.MouseWheelDown:
			m.repoPicker.MoveDown()
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
			// Route the wheel to whichever pane the cursor is over
			// (bt-y3ip): wheel-on-list moves the bead cursor, wheel-on-
			// middle moves the commit cursor, wheel-on-detail scrolls the
			// detail content. Falls back to the previous global behaviour
			// (bead-list scroll) when ScrollAtX returns false -- e.g.,
			// click coords outside any pane in the wide-bead timeline gap.
			if !m.historyView.ScrollAtX(msg.X, -1) {
				m.historyView.MoveUp()
			}
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
			// Mirror MouseWheelUp: route to the pane under the cursor (bt-y3ip).
			if !m.historyView.ScrollAtX(msg.X, +1) {
				m.historyView.MoveDown()
			}
		case focusFlowMatrix:
			m.flowMatrix.MoveDown()
		}
		return m, nil
	}
	return m, nil
}

// handleWindowSize processes terminal resize events using a two-phase debounce
// (bt-kfkrb). Phase 1 (this function) runs on every WindowSizeMsg and applies
// cheap layout changes immediately: dimensions, split-view flag, list size,
// viewport allocation, and panel chrome. It defers the two expensive operations
// -- SetWidthWithTheme (creates a new Glamour TermRenderer) and
// updateViewportContent (renders all markdown through that renderer) -- to
// Phase 2 via a generation-counter settle tick.
func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (Model, tea.Cmd) {
	wsStart := time.Now()
	defer func() {
		debug.LogTiming(fmt.Sprintf("handleWindowSize w=%d h=%d", msg.Width, msg.Height), time.Since(wsStart))
	}()
	m.width = msg.Width
	m.height = msg.Height
	// Layout against bodyWidth so the shortcuts sidebar (when visible) gets
	// reserved space rather than overflowing onto the body (bt-lin9).
	bodyW := m.bodyWidth()
	m.isSplitView = bodyW > SplitViewThreshold
	m.ready = true
	bodyHeight := m.height - 1 // keep 1 row for footer
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	// Phase 1: cheap layout — list sizing and viewport allocation only.
	// SetWidthWithTheme (Glamour), insightsPanel.SetSize, and
	// updateViewportContent are deferred to phase 2 so per-event cost during
	// a drag is negligible.
	if m.isSplitView {
		availWidth := bodyW - 8
		if availWidth < 10 {
			availWidth = 10
		}
		listInnerWidth := int(float64(availWidth) * m.splitPaneRatio)
		detailInnerWidth := availWidth - listInnerWidth
		listHeight := bodyHeight - 4
		if listHeight < 3 {
			listHeight = 3
		}
		m.list.SetSize(listInnerWidth, listHeight)
		m.viewport = viewport.New(viewport.WithWidth(detailInnerWidth), viewport.WithHeight(bodyHeight-2))
	} else {
		listHeight := bodyHeight - 2
		if listHeight < 3 {
			listHeight = 3
		}
		m.list.SetSize(bodyW, listHeight)
		m.viewport = viewport.New(viewport.WithWidth(bodyW), viewport.WithHeight(bodyHeight-1))
	}
	m.updateListDelegate()
	m.labelDashboard.SetSize(bodyW, bodyHeight)
	m.labelPicker.SetSize(m.width, bodyHeight)
	m.repoPicker.SetSize(m.width, bodyHeight)

	// Bump the generation counter and schedule phase 2 after the settle delay.
	// Any prior pending resizeSettledMsg with an older gen is ignored by
	// handleResizeSettled, so only the last resize in a burst runs the heavy path.
	m.resizeGen++
	gen := m.resizeGen
	cmd := tea.Tick(resizeSettleDelay, func(time.Time) tea.Msg {
		return resizeSettledMsg{gen: gen}
	})
	return m, cmd
}

// applyWindowSizeHeavy runs the expensive phase of resize: rebuilds the Glamour
// renderer via SetWidthWithTheme and refreshes viewport content. Called from
// handleResizeSettled (debounced path) and from callers that need an immediate
// full reflow (e.g., sidebar width toggle), where the settle tick is discarded.
func (m Model) applyWindowSizeHeavy() Model {
	heavyStart := time.Now()
	bodyW := m.bodyWidth()
	bodyHeight := m.height - 1
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	// Width-only short-circuit (bt-kfkrb): updateViewportContent + Glamour
	// rebuild only depend on bodyW; if width hasn't changed since the last
	// applyHeavy this is a height-only resize and the body wraps identically.
	// Skip the ~100 ms re-render and only refresh size-dependent panels.
	if m.lastHeavyWidth == bodyW && m.lastHeavyWidth != 0 {
		insightsStart := time.Now()
		m.insightsPanel.SetSize(bodyW, bodyHeight)
		debug.LogTiming("applyHeavy.insightsPanel.SetSize", time.Since(insightsStart))
		debug.LogTiming("applyHeavy.total[width-unchanged]", time.Since(heavyStart))
		return m
	}

	rendererStart := time.Now()
	if m.isSplitView {
		availWidth := bodyW - 8
		if availWidth < 10 {
			availWidth = 10
		}
		listInnerWidth := int(float64(availWidth) * m.splitPaneRatio)
		detailInnerWidth := availWidth - listInnerWidth
		m.renderer.SetWidthWithTheme(detailInnerWidth, m.theme)
	} else {
		m.renderer.SetWidthWithTheme(bodyW, m.theme)
	}
	debug.LogTiming("applyHeavy.renderer.SetWidthWithTheme", time.Since(rendererStart))

	insightsStart := time.Now()
	m.insightsPanel.SetSize(bodyW, bodyHeight)
	debug.LogTiming("applyHeavy.insightsPanel.SetSize", time.Since(insightsStart))

	vpStart := time.Now()
	m.updateViewportContent()
	debug.LogTiming("applyHeavy.updateViewportContent", time.Since(vpStart))

	m.lastHeavyWidth = bodyW

	debug.LogTiming("applyHeavy.total", time.Since(heavyStart))
	return m
}

// handleResizeSettled is phase 2 of the debounced resize. It runs only when
// no newer WindowSizeMsg has arrived since this message was scheduled.
func (m Model) handleResizeSettled(msg resizeSettledMsg) Model {
	if msg.gen != m.resizeGen {
		// A newer resize is in flight; this settle message is stale.
		return m
	}
	m = m.applyWindowSizeHeavy()
	// The epics tree caches its rendered body sized to the terminal; re-window
	// it on resize so it tracks width/height (mirrors how Tree re-renders).
	if m.mode == ViewEpics {
		m.refreshEpicsForCurrentFilter()
	}
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
	// fixed by renderAlertsPanel: width = alertsPanelWidth(), height set by
	// alertsPanelHeight.
	panelWidth := m.alertsPanelWidth()
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

// handleLabelPickerModalClick routes a MouseClickMsg when the label picker
// modal is open (bt-wnda). Click on a label row moves the cursor and toggles
// selection (mirrors space). Click on the search row focuses the input.
// Clicks on chrome/backdrop are no-ops, matching the alerts-modal precedent.
func (m Model) handleLabelPickerModalClick(mouse tea.Mouse) (Model, tea.Cmd) {
	panelWidth, panelHeight := m.labelPicker.Dimensions()
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
		// Backdrop click: no-op (consistent with alerts modal — esc is the
		// only close path).
		return m, nil
	}

	// Search input row: focus the search input on click. This is the mouse
	// equivalent of pressing "/".
	if m.labelPicker.IsSearchRow(my) {
		if !m.labelPicker.IsSearchFocused() {
			m.labelPicker.FocusSearch()
		}
		return m, nil
	}

	// Label row: move cursor and toggle selection (clicking acts like
	// space — surfaces the multi-select pattern through the trackpad).
	idx, ok := m.labelPicker.ItemAtPanelY(my)
	if !ok {
		return m, nil
	}
	m.labelPicker.SetCursor(idx)
	m.labelPicker.ToggleSelected()
	return m, nil
}

// handleRepoPickerModalClick routes a MouseClickMsg when the project
// (repo) picker is open (bt-hpsq). Mirrors handleLabelPickerModalClick:
// click on a project row selects it (cursor + space toggle); chrome and
// backdrop clicks are no-ops.
func (m Model) handleRepoPickerModalClick(mouse tea.Mouse) (Model, tea.Cmd) {
	panelWidth, panelHeight := m.repoPicker.Dimensions()
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
		return m, nil
	}

	idx, ok := m.repoPicker.ItemAtPanelY(my)
	if !ok {
		return m, nil
	}
	m.repoPicker.SetCursor(idx)
	m.repoPicker.ToggleSelected()
	return m, nil
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
	// Day-separator trim must mirror renderNotificationsTab (bt-l5zk) so
	// click-to-row math stays aligned with the visible layout.
	end = trimEndForDaySeparators(active, start, end, pageSize)
	row := 0
	var prevDate string
	for i := start; i < end; i++ {
		curDate := active[i].At.Format("2006-01-02")
		if curDate != prevDate {
			// Separator row: clicks here are no-ops, mirroring chrome.
			if row == relY {
				return -1, false
			}
			row++
			prevDate = curDate
		}
		if row == relY {
			return i, true
		}
		row++
		if i == m.notificationsCursor {
			// Summary line is rendered only when Summary is non-empty after
			// newline-sanitization + trim (renderNotificationsTab).
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
		// Restore prior cursor position if the user has been here before
		// (bt-fdwz). Bounds clamping happens inside RestoreCursor against
		// the freshly built panel item counts.
		if m.insightsCursor.valid {
			m.insightsPanel.RestoreCursor(m.insightsCursor.panel, m.insightsCursor.index)
		}
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
