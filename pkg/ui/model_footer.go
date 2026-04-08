package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/search"
	"github.com/seanmartinsmith/beadstui/pkg/watcher"

	"github.com/charmbracelet/bubbles/list"
	 tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// setTransientStatus sets a status message that auto-clears after the given duration.
func (m *Model) setTransientStatus(msg string, d time.Duration) tea.Cmd {
	m.statusMsg = msg
	m.statusIsError = false
	m.statusSeq++
	seq := m.statusSeq
	return tea.Tick(d, func(time.Time) tea.Msg {
		return statusClearMsg{seq: seq}
	})
}

func (m *Model) renderFooter() string {
	// ══════════════════════════════════════════════════════════════════════════
	// POLISHED FOOTER - Stripe-level status bar with visual hierarchy
	// ══════════════════════════════════════════════════════════════════════════

	// If there's a status message, show it prominently with polished styling
	if m.statusMsg != "" {
		var msgStyle lipgloss.Style
		if m.statusIsError {
			msgStyle = lipgloss.NewStyle().
				Background(ColorPrioCriticalBg).
				Foreground(ColorPrioCritical).
				Bold(true).
				Padding(0, 2)
		} else {
			msgStyle = lipgloss.NewStyle().
				Background(ColorStatusOpenBg).
				Foreground(ColorSuccess).
				Bold(true).
				Padding(0, 2)
		}
		prefix := "✓ "
		if m.statusIsError {
			prefix = "✗ "
		}
		displayMsg := prefix + m.statusMsg
		if maxMsgWidth := m.width - 4; lipgloss.Width(displayMsg) > maxMsgWidth {
			displayMsg = truncateString(displayMsg, maxMsgWidth)
		}
		msgSection := msgStyle.Render(displayMsg)
		remaining := m.width - lipgloss.Width(msgSection)
		if remaining < 0 {
			remaining = 0
		}
		filler := lipgloss.NewStyle().Width(remaining).Render("")
		return lipgloss.JoinHorizontal(lipgloss.Bottom, msgSection, filler)
	}

	// ─────────────────────────────────────────────────────────────────────────
	// FILTER BADGE - Current view/filter state + quick hint for label dashboard
	// ─────────────────────────────────────────────────────────────────────────
	var filterTxt string
	var filterIcon string
	if m.focused == focusLabelDashboard {
		filterTxt = "LABELS: j/k nav • h detail • d drilldown • enter filter"
		filterIcon = "🏷️"
	} else if m.showLabelGraphAnalysis && m.labelGraphAnalysisResult != nil {
		filterTxt = fmt.Sprintf("GRAPH %s: esc/q/g close", m.labelGraphAnalysisResult.Label)
		filterIcon = "📊"
	} else if m.showLabelDrilldown && m.labelDrilldownLabel != "" {
		filterTxt = fmt.Sprintf("LABEL %s: enter filter • g graph • esc/q/d close", m.labelDrilldownLabel)
		filterIcon = "🏷️"
	} else {
		switch m.currentFilter {
		case "all":
			filterTxt = "ALL"
			filterIcon = "📋"
		case "open":
			filterTxt = "OPEN"
			filterIcon = "📂"
		case "closed":
			filterTxt = "CLOSED"
			filterIcon = "✅"
		case "ready":
			filterTxt = "READY"
			filterIcon = "🚀"
		default:
			if strings.HasPrefix(m.currentFilter, "bql:") {
				bqlStr := m.currentFilter[4:]
				if len(bqlStr) > 30 {
					bqlStr = bqlStr[:27] + "..."
				}
				filterTxt = "BQL: " + bqlStr
				filterIcon = "🔍"
			} else if strings.HasPrefix(m.currentFilter, "recipe:") {
				filterTxt = strings.ToUpper(m.currentFilter[7:])
				filterIcon = "📑"
			} else {
				filterTxt = m.currentFilter
				filterIcon = "🔍"
			}
		}
	}

	filterBadge := lipgloss.NewStyle().
		Background(ColorPrimary).
		Foreground(ColorBgContrast).
		Bold(true).
		Padding(0, 1).
		Render(fmt.Sprintf("%s %s", filterIcon, filterTxt))

	// Project name badge - at-a-glance workspace identity
	projectBadge := ""
	if m.projectName != "" && !m.workspaceMode {
		projectBadge = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Padding(0, 1).
			Render("~ " + m.projectName)
	}

	// Search mode badge when filtering
	searchBadge := ""
	if m.list.FilterState() != list.Unfiltered {
		mode := "fuzzy"
		if m.semanticSearchEnabled {
			mode = "semantic"
			if m.semanticIndexBuilding {
				mode = "semantic (indexing)"
			}
			if m.semanticHybridEnabled {
				mode = fmt.Sprintf("hybrid/%s", m.semanticHybridPreset)
				if m.semanticHybridBuilding {
					mode = fmt.Sprintf("hybrid/%s (metrics)", m.semanticHybridPreset)
				}
			}
		}
		searchBadge = lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorSecondary).
			Padding(0, 1).
			Render(fmt.Sprintf("🔎 %s", mode))
	}

	// Sort badge - only show when not default (bv-3ita)
	sortBadge := ""
	if m.sortMode != SortDefault {
		sortBadge = lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorSecondary).
			Padding(0, 1).
			Render(fmt.Sprintf("↕ %s", m.sortMode.String()))
	}

	labelHint := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1).
		Render("L:labels • h:detail")

	// Board-specific hints (bv-yg39, bv-naov)
	if m.isBoardView {
		if m.board.IsSearchMode() {
			// Search mode active - show search hints
			matchInfo := ""
			if m.board.SearchMatchCount() > 0 {
				matchInfo = fmt.Sprintf(" [%d/%d]", m.board.SearchCursorPos(), m.board.SearchMatchCount())
			}
			labelHint = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 1).
				Render(fmt.Sprintf("/%s%s • n/N:match • enter:done • esc:cancel", m.board.SearchQuery(), matchInfo))
		} else {
			// Normal board mode - show navigation hints with filter indicator (bv-naov)
			filterInfo := ""
			if m.currentFilter != "all" && m.currentFilter != "" {
				shown := m.board.TotalCount()
				total := len(m.issues)
				filterInfo = fmt.Sprintf("[%s:%d/%d] ", m.currentFilter, shown, total)
			}
			labelHint = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 1).
				Render(fmt.Sprintf("%s1-4:col • o/c/r:filter • L:labels • /:search • ?:help", filterInfo))
		}
	} else if m.showAttentionView {
		labelHint = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(0, 1).
			Render("A:attention • 1-9 filter • esc close")
	}

	// ─────────────────────────────────────────────────────────────────────────
	// STATS SECTION - Issue counts with visual indicators
	// ─────────────────────────────────────────────────────────────────────────
	var statsSection string
	if m.timeTravelMode && m.timeTravelDiff != nil {
		d := m.timeTravelDiff.Summary
		timeTravelStyle := lipgloss.NewStyle().
			Background(ColorPrioHighBg).
			Foreground(ColorWarning).
			Padding(0, 1)
		statsSection = timeTravelStyle.Render(fmt.Sprintf("⏱ %s: +%d ✅%d ~%d",
			m.timeTravelSince, d.IssuesAdded, d.IssuesClosed, d.IssuesModified))
	} else {
		// Polished stats with mini indicators
		statsStyle := lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorText).
			Padding(0, 1)

		openStyle := lipgloss.NewStyle().Foreground(ColorStatusOpen)
		readyStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
		blockedStyle := lipgloss.NewStyle().Foreground(ColorWarning)
		closedStyle := lipgloss.NewStyle().Foreground(ColorMuted)

		statsContent := fmt.Sprintf("%s%d %s%d %s%d %s%d",
			openStyle.Render("○"),
			m.countOpen,
			readyStyle.Render("◉"),
			m.countReady,
			blockedStyle.Render("◈"),
			m.countBlocked,
			closedStyle.Render("●"),
			m.countClosed)
		statsSection = statsStyle.Render(statsContent)
	}

	// ─────────────────────────────────────────────────────────────────────────
	// FRESHNESS / WORKER BADGE - Staleness + errors + background worker activity (bv-h305)
	// ─────────────────────────────────────────────────────────────────────────
	workerSection := ""
	if m.backgroundWorker != nil {
		formatAge := func(d time.Duration) string {
			switch {
			case d < time.Second:
				return "<1s"
			case d < time.Minute:
				return fmt.Sprintf("%ds", int(d.Seconds()))
			case d < time.Hour:
				return fmt.Sprintf("%dm", int(d.Minutes()))
			case d < 24*time.Hour:
				return fmt.Sprintf("%dh", int(d.Hours()))
			default:
				return fmt.Sprintf("%dd", int(d.Hours()/24))
			}
		}

		// Freshness age: prefer lastDoltVerified (bt-3ynd) over snapshot.CreatedAt.
		// When Dolt polling is active, "verified" means "poll confirmed data is current"
		// even if no new snapshot was built (no data changed). This prevents false STALE
		// indicators when data simply hasn't changed.
		var freshnessAge time.Duration
		hasFreshnessAge := false
		if !m.lastDoltVerified.IsZero() {
			freshnessAge = time.Since(m.lastDoltVerified)
			hasFreshnessAge = true
		} else if m.snapshot != nil && !m.snapshot.CreatedAt.IsZero() {
			freshnessAge = time.Since(m.snapshot.CreatedAt)
			hasFreshnessAge = true
		}

		state := m.backgroundWorker.State()
		health := m.backgroundWorker.Health()
		lastErr := m.backgroundWorker.LastError()

		var style lipgloss.Style
		var text string
		switch {
		case health.Started && !health.Alive:
			style = lipgloss.NewStyle().
				Background(ColorPrioCriticalBg).
				Foreground(ColorPrioCritical).
				Bold(true).
				Padding(0, 1)
			text = "⚠ worker unresponsive"

		case state == WorkerProcessing && m.backgroundWorker.ProcessingDuration() >= 250*time.Millisecond:
			// Only show spinner after grace period to avoid flicker for quick dedup operations
			style = lipgloss.NewStyle().
				Background(ColorBgHighlight).
				Foreground(ColorInfo).
				Bold(true).
				Padding(0, 1)
			frame := workerSpinnerFrames[m.workerSpinnerIdx%len(workerSpinnerFrames)]
			text = fmt.Sprintf("%s refreshing", frame)

		case lastErr != nil && lastErr.Retries >= freshnessErrorRetries:
			style = lipgloss.NewStyle().
				Background(ColorPrioCriticalBg).
				Foreground(ColorPrioCritical).
				Bold(true).
				Padding(0, 1)
			text = fmt.Sprintf("✗ bg %s (%dx)", lastErr.Phase, lastErr.Retries)

		case lastErr != nil:
			style = lipgloss.NewStyle().
				Background(ColorBgHighlight).
				Foreground(ColorWarning).
				Bold(true).
				Padding(0, 1)
			text = fmt.Sprintf("⚠ bg %s (%s)", lastErr.Phase, formatAge(time.Since(lastErr.Time)))

		case hasFreshnessAge && freshnessAge >= freshnessStaleThreshold():
			style = lipgloss.NewStyle().
				Background(ColorBgHighlight).
				Foreground(ColorDanger).
				Bold(true).
				Padding(0, 1)
			text = fmt.Sprintf("⚠ STALE: %s ago", formatAge(freshnessAge))

		case hasFreshnessAge && freshnessAge >= freshnessWarnThreshold():
			style = lipgloss.NewStyle().
				Background(ColorBgHighlight).
				Foreground(ColorWarning).
				Padding(0, 1)
			text = fmt.Sprintf("⚠ %s ago", formatAge(freshnessAge))

		default:
			if health.RecoveryCount > 0 {
				style = lipgloss.NewStyle().
					Background(ColorBgHighlight).
					Foreground(ColorWarning).
					Padding(0, 1)
				text = fmt.Sprintf("↻ recovered x%d", health.RecoveryCount)
			} else {
				// Fresh: no indicator.
				text = ""
			}
		}

		if text != "" {
			workerSection = style.Render(text)
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// PHASE 2 PROGRESS - show while metrics are still computing (bv-tspo)
	// ─────────────────────────────────────────────────────────────────────────
	phase2Section := ""
	if m.snapshot != nil && !m.snapshot.Phase2Ready {
		phase2Style := lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorInfo).
			Padding(0, 1)
		phase2Section = phase2Style.Render("◌ metrics…")
	}

	// ─────────────────────────────────────────────────────────────────────────
	// WATCHER MODE - show polling mode when fsnotify isn't reliable (bv-3zwy)
	// ─────────────────────────────────────────────────────────────────────────
	watcherSection := ""
	{
		var (
			polling      bool
			fsType       watcher.FilesystemType
			pollInterval time.Duration
		)

		switch {
		case m.backgroundWorker != nil:
			polling, fsType, pollInterval = m.backgroundWorker.WatcherInfo()
		case m.watcher != nil:
			polling = m.watcher.IsPolling()
			fsType = m.watcher.FilesystemType()
			pollInterval = m.watcher.PollInterval()
		}

		if polling {
			watcherStyle := lipgloss.NewStyle().
				Background(ColorBgHighlight).
				Foreground(ColorMuted).
				Padding(0, 1)
			label := "polling"
			if fsType != watcher.FSTypeUnknown && fsType != watcher.FSTypeLocal {
				label = fmt.Sprintf("polling %s", fsType.String())
			}
			if pollInterval > 0 {
				label = fmt.Sprintf("%s %s", label, pollInterval.String())
			}
			watcherSection = watcherStyle.Render(label)
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// UPDATE BADGE - New version available
	// ─────────────────────────────────────────────────────────────────────────
	updateSection := ""
	if m.updateAvailable {
		updateStyle := lipgloss.NewStyle().
			Background(ColorTypeFeature).
			Foreground(ColorBg).
			Bold(true).
			Padding(0, 1)
		updateSection = updateStyle.Render(fmt.Sprintf("⭐ %s", m.updateTag))
	}

	// ─────────────────────────────────────────────────────────────────────────
	// LARGE DATASET WARNING - Tiered performance mode (bv-9thm)
	// ─────────────────────────────────────────────────────────────────────────
	datasetSection := ""
	if m.snapshot != nil && m.snapshot.LargeDatasetWarning != "" {
		bg := ColorPrioHighBg
		fg := ColorWarning
		if m.snapshot.DatasetTier == datasetTierHuge {
			bg = ColorPrioCriticalBg
			fg = ColorPrioCritical
		}
		datasetStyle := lipgloss.NewStyle().
			Background(bg).
			Foreground(fg).
			Bold(true).
			Padding(0, 1)
		datasetSection = datasetStyle.Render(m.snapshot.LargeDatasetWarning)
	}

	// ─────────────────────────────────────────────────────────────────────────
	// ALERTS BADGE - Project health alerts (bv-168)
	// ─────────────────────────────────────────────────────────────────────────
	alertsSection := ""
	// Count active (non-dismissed) alerts
	activeAlerts := 0
	activeCritical := 0
	activeWarning := 0
	for _, a := range m.alerts {
		if !m.dismissedAlerts[alertKey(a)] {
			activeAlerts++
			switch a.Severity {
			case drift.SeverityCritical:
				activeCritical++
			case drift.SeverityWarning:
				activeWarning++
			}
		}
	}
	if activeAlerts > 0 {
		var alertStyle lipgloss.Style
		var alertIcon string
		if activeCritical > 0 {
			alertStyle = lipgloss.NewStyle().
				Background(ColorPrioCriticalBg).
				Foreground(ColorPrioCritical).
				Bold(true).
				Padding(0, 1)
			alertIcon = "⚠"
		} else if activeWarning > 0 {
			alertStyle = lipgloss.NewStyle().
				Background(ColorPrioHighBg).
				Foreground(ColorWarning).
				Bold(true).
				Padding(0, 1)
			alertIcon = "⚡"
		} else {
			alertStyle = lipgloss.NewStyle().
				Background(ColorBgHighlight).
				Foreground(ColorInfo).
				Padding(0, 1)
			alertIcon = "ℹ"
		}
		alertsSection = alertStyle.Render(fmt.Sprintf("%s %d alerts (!)", alertIcon, activeAlerts))
	}

	// ─────────────────────────────────────────────────────────────────────────
	// INSTANCE WARNING - Secondary instance indicator (bv-vrvn)
	// ─────────────────────────────────────────────────────────────────────────
	instanceSection := ""
	if m.instanceLock != nil && !m.instanceLock.IsFirstInstance() {
		instanceStyle := lipgloss.NewStyle().
			Background(ColorPrioHighBg).
			Foreground(ColorWarning).
			Bold(true).
			Padding(0, 1)
		instanceSection = instanceStyle.Render(fmt.Sprintf("⚠ PID %d", m.instanceLock.HolderPID()))
	}

	// ─────────────────────────────────────────────────────────────────────────
	// SESSION INDICATOR - Cass coding sessions for selected bead (bv-y836)
	// ─────────────────────────────────────────────────────────────────────────
	sessionSection := ""
	if sessionCount := m.getCassSessionCount(); sessionCount > 0 {
		sessionStyle := lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorInfo).
			Padding(0, 1)
		countStr := fmt.Sprintf("%d", sessionCount)
		if sessionCount > 9 {
			countStr = "9+"
		}
		sessionSection = sessionStyle.Render(fmt.Sprintf("📎%s", countStr))
	}

	// ─────────────────────────────────────────────────────────────────────────
	// WORKSPACE BADGE - Multi-repo mode indicator
	// ─────────────────────────────────────────────────────────────────────────
	workspaceSection := ""
	if m.workspaceMode && m.workspaceSummary != "" {
		workspaceStyle := lipgloss.NewStyle().
			Background(ThemeBg("#8abeb7")).
			Foreground(ColorBg).
			Bold(true).
			Padding(0, 1)
		workspaceSection = workspaceStyle.Render(fmt.Sprintf("📦 %s", m.workspaceSummary))
	}

	// ─────────────────────────────────────────────────────────────────────────
	// PROJECT FILTER BADGE - Active project selection (multi-project mode)
	// ─────────────────────────────────────────────────────────────────────────
	repoFilterSection := ""
	if m.workspaceMode && m.activeRepos != nil && len(m.activeRepos) > 0 {
		active := sortedRepoKeys(m.activeRepos)
		label := formatRepoList(active, 3)
		repoStyle := lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorInfo).
			Bold(true).
			Padding(0, 1)
		repoFilterSection = repoStyle.Render(fmt.Sprintf("🗂 %s", label))
	}

	// ─────────────────────────────────────────────────────────────────────────
	// KEYBOARD HINTS - Context-aware navigation help
	// ─────────────────────────────────────────────────────────────────────────
	keyStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Background(ColorBgSubtle).
		Padding(0, 0)
	sepStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	sep := sepStyle.Render(" │ ")

	var keyHints []string
	if m.showHelp {
		keyHints = append(keyHints, "Press any key to close")
	} else if m.showRecipePicker {
		keyHints = append(keyHints, keyStyle.Render("j/k")+" nav", keyStyle.Render("⏎")+" apply", keyStyle.Render("esc")+" cancel")
	} else if m.showRepoPicker {
		keyHints = append(keyHints, keyStyle.Render("j/k")+" nav", keyStyle.Render("space")+" toggle", keyStyle.Render("⏎")+" apply", keyStyle.Render("esc")+" cancel")
	} else if m.showLabelPicker {
		keyHints = append(keyHints, "type to filter", keyStyle.Render("j/k")+" nav", keyStyle.Render("⏎")+" apply", keyStyle.Render("esc")+" cancel")
	} else if m.focused == focusInsights {
		keyHints = append(keyHints, keyStyle.Render("h/l")+" panels", keyStyle.Render("e")+" explain", keyStyle.Render("⏎")+" jump", keyStyle.Render("?")+" help")
		keyHints = append(keyHints, keyStyle.Render("A")+" attention", keyStyle.Render("F")+" flow")
	} else if m.focused == focusFlowMatrix {
		keyHints = append(keyHints, keyStyle.Render("j/k")+" nav", keyStyle.Render("tab")+" panel", keyStyle.Render("⏎")+" drill", keyStyle.Render("esc")+" back", keyStyle.Render("f")+" close")
	} else if m.isGraphView {
		keyHints = append(keyHints, keyStyle.Render("hjkl")+" nav", keyStyle.Render("H/L")+" scroll", keyStyle.Render("⏎")+" view", keyStyle.Render("g")+" list")
	} else if m.isBoardView {
		keyHints = append(keyHints, keyStyle.Render("hjkl")+" nav", keyStyle.Render("G")+" bottom", keyStyle.Render("⏎")+" view", keyStyle.Render("b")+" list")
	} else if m.isActionableView {
		keyHints = append(keyHints, keyStyle.Render("j/k")+" nav", keyStyle.Render("⏎")+" view", keyStyle.Render("a")+" list", keyStyle.Render("?")+" help")
	} else if m.isHistoryView {
		keyHints = append(keyHints, keyStyle.Render("j/k")+" nav", keyStyle.Render("tab")+" focus", keyStyle.Render("⏎")+" jump", keyStyle.Render("H")+" close")
	} else if m.list.FilterState() == list.Filtering {
		mode := "fuzzy"
		if m.semanticSearchEnabled {
			mode = "semantic"
			if m.semanticIndexBuilding {
				mode = "semantic (indexing)"
			}
		}
		keyHints = append(keyHints, keyStyle.Render("esc")+" cancel", keyStyle.Render("ctrl+s")+" "+mode, keyStyle.Render("⏎")+" select")
		if m.semanticSearchEnabled {
			keyHints = append(keyHints, keyStyle.Render("H")+" hybrid", keyStyle.Render("alt+h")+" preset")
		}
	} else if m.showTimeTravelPrompt {
		keyHints = append(keyHints, keyStyle.Render("⏎")+" compare", keyStyle.Render("esc")+" cancel")
	} else {
		if m.timeTravelMode {
			keyHints = append(keyHints, keyStyle.Render("t")+" exit diff", keyStyle.Render("C")+" copy", keyStyle.Render("abgi")+" views", keyStyle.Render("?")+" help")
		} else if m.isSplitView {
			keyHints = append(keyHints, keyStyle.Render("tab")+" focus", keyStyle.Render("C")+" copy", keyStyle.Render("x")+" export", keyStyle.Render("Ctrl+R")+" refresh", keyStyle.Render("?")+" help")
		} else if m.showDetails {
			keyHints = append(keyHints, keyStyle.Render("esc")+" back", keyStyle.Render("C")+" copy", keyStyle.Render("O")+" edit", keyStyle.Render("Ctrl+R")+" refresh", keyStyle.Render("?")+" help")
		} else {
			keyHints = append(keyHints, keyStyle.Render("⏎")+" details", keyStyle.Render("t")+" diff", keyStyle.Render("S")+" triage", keyStyle.Render("l")+" labels", keyStyle.Render("Ctrl+R")+" refresh", keyStyle.Render("?")+" help")
			if m.workspaceMode {
				keyHints = append(keyHints, keyStyle.Render("w")+" projects")
			}
		}
	}

	// Progressive truncation: drop middle hints until they fit, keeping
	// the first (primary action) and last ("?" help) visible.
	keysStyle := lipgloss.NewStyle().
		Foreground(ColorSubtext).
		Padding(0, 1)

	countBadge := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Padding(0, 1).
		Render(fmt.Sprintf("%d issues", len(m.list.Items())))

	keysSection := keysStyle.Render(strings.Join(keyHints, sep))
	if len(keyHints) > 2 {
		// Estimate available space for key hints
		availableWidth := m.width - lipgloss.Width(countBadge) - 2
		for len(keyHints) > 2 && lipgloss.Width(keysSection) > availableWidth {
			// Remove second-to-last hint (keep first + "?" help)
			keyHints = append(keyHints[:len(keyHints)-2], keyHints[len(keyHints)-1])
			keysSection = keysStyle.Render(strings.Join(keyHints, sep))
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// ASSEMBLE FOOTER with proper spacing
	// ─────────────────────────────────────────────────────────────────────────
	leftWidth := lipgloss.Width(filterBadge) + lipgloss.Width(labelHint) + lipgloss.Width(statsSection)
	if projectBadge != "" {
		leftWidth += lipgloss.Width(projectBadge) + 1
	}
	if phase2Section != "" {
		leftWidth += lipgloss.Width(phase2Section) + 1
	}
	if watcherSection != "" {
		leftWidth += lipgloss.Width(watcherSection) + 1
	}
	if workerSection != "" {
		leftWidth += lipgloss.Width(workerSection) + 1
	}
	if searchBadge != "" {
		leftWidth += lipgloss.Width(searchBadge) + 1
	}
	if sortBadge != "" {
		leftWidth += lipgloss.Width(sortBadge) + 1
	}
	if alertsSection != "" {
		leftWidth += lipgloss.Width(alertsSection) + 1
	}
	if instanceSection != "" {
		leftWidth += lipgloss.Width(instanceSection) + 1
	}
	if sessionSection != "" {
		leftWidth += lipgloss.Width(sessionSection) + 1
	}
	if workspaceSection != "" {
		leftWidth += lipgloss.Width(workspaceSection) + 1
	}
	if repoFilterSection != "" {
		leftWidth += lipgloss.Width(repoFilterSection) + 1
	}
	if updateSection != "" {
		leftWidth += lipgloss.Width(updateSection) + 1
	}
	if datasetSection != "" {
		leftWidth += lipgloss.Width(datasetSection) + 1
	}
	rightWidth := lipgloss.Width(countBadge) + lipgloss.Width(keysSection)

	remaining := m.width - leftWidth - rightWidth - 1
	if remaining < 0 {
		remaining = 0
	}
	filler := lipgloss.NewStyle().Width(remaining).Render("")

	// Build the footer
	var parts []string
	parts = append(parts, filterBadge)
	if projectBadge != "" {
		parts = append(parts, projectBadge)
	}
	if searchBadge != "" {
		parts = append(parts, searchBadge)
	}
	if sortBadge != "" {
		parts = append(parts, sortBadge)
	}
	parts = append(parts, labelHint)
	if alertsSection != "" {
		parts = append(parts, alertsSection)
	}
	if instanceSection != "" {
		parts = append(parts, instanceSection)
	}
	if sessionSection != "" {
		parts = append(parts, sessionSection)
	}
	if workspaceSection != "" {
		parts = append(parts, workspaceSection)
	}
	if repoFilterSection != "" {
		parts = append(parts, repoFilterSection)
	}
	if updateSection != "" {
		parts = append(parts, updateSection)
	}
	if datasetSection != "" {
		parts = append(parts, datasetSection)
	}
	parts = append(parts, statsSection)
	if phase2Section != "" {
		parts = append(parts, phase2Section)
	}
	if watcherSection != "" {
		parts = append(parts, watcherSection)
	}
	if workerSection != "" {
		parts = append(parts, workerSection)
	}
	parts = append(parts, filler, countBadge, keysSection)

	return lipgloss.JoinHorizontal(lipgloss.Bottom, parts...)
}

func nextHybridPreset(current search.PresetName) search.PresetName {
	presets := search.ListPresets()
	if len(presets) == 0 {
		return search.PresetDefault
	}
	for i, preset := range presets {
		if preset == current {
			return presets[(i+1)%len(presets)]
		}
	}
	return presets[0]
}
