package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/search"
	"github.com/seanmartinsmith/beadstui/pkg/watcher"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// setTransientStatus sets a status message that auto-clears after the given duration.
// Renders as a full-width banner that replaces the footer (use for errors or important
// user-initiated confirmations). Background notifications should use setInlineTransientStatus.
func (m *Model) setTransientStatus(msg string, d time.Duration) tea.Cmd {
	m.statusMsg = msg
	m.statusIsError = false
	m.statusIsInline = false
	m.statusSetAt = time.Now()
	m.statusSeq++
	seq := m.statusSeq
	return tea.Tick(d, func(time.Time) tea.Msg {
		return statusClearMsg{seq: seq}
	})
}

// setInlineTransientStatus sets a subtle status that renders in the footer hint slot
// (not a full-width banner) and auto-clears after the given duration. Use for background
// notifications that should not clobber key hints (bt-y0k7).
func (m *Model) setInlineTransientStatus(msg string, d time.Duration) tea.Cmd {
	m.statusMsg = msg
	m.statusIsError = false
	m.statusIsInline = true
	m.statusSetAt = time.Now()
	m.statusSeq++
	seq := m.statusSeq
	return tea.Tick(d, func(time.Time) tea.Msg {
		return statusClearMsg{seq: seq}
	})
}

// setStatus sets a status message with auto-dismiss tracking (bt-zdae).
func (m *Model) setStatus(msg string) {
	m.statusMsg = msg
	m.statusIsError = false
	m.statusIsInline = false
	m.statusSetAt = time.Now()
}

// setStatusError sets an error status message (not auto-dismissed).
func (m *Model) setStatusError(msg string) {
	m.statusMsg = msg
	m.statusIsError = true
	m.statusIsInline = false
	m.statusSetAt = time.Now()
}

// statusAutoDismissAge is how long non-transient status messages persist
// before being auto-cleared (bt-zdae).
const statusAutoDismissAge = 5 * time.Second

// statusTickInterval drives the recurring tick that forces idle auto-dismiss
// of status messages (bt-m9te, bt-y0k7).
const statusTickInterval = 1 * time.Second

// statusTickCmd schedules the next idle auto-dismiss check.
func statusTickCmd() tea.Cmd {
	return tea.Tick(statusTickInterval, func(time.Time) tea.Msg {
		return statusTickMsg{}
	})
}

// ---------------------------------------------------------------------------
// FooterData — value struct decoupling footer rendering from Model internals.
// Populated by Model.footerData(), rendered by FooterData.Render().
// ---------------------------------------------------------------------------

// WorkerLevel indicates the severity of the background worker badge.
type WorkerLevel int

const (
	WorkerLevelNone     WorkerLevel = iota
	WorkerLevelInfo                 // spinner, recovery
	WorkerLevelWarning              // transient error, aging
	WorkerLevelCritical             // dead worker, persistent error, stale
)

// DatasetLevel indicates the severity of the dataset size warning.
type DatasetLevel int

const (
	DatasetLevelNone DatasetLevel = iota
	DatasetLevelWarning
	DatasetLevelCritical
)

// FooterData contains all data needed to render the footer, decoupled from Model.
type FooterData struct {
	Width int

	// Status bar — when StatusMsg is set, footer shows only this message
	// (full-width banner) unless StatusIsInline is true, in which case it
	// renders subtly in the hint slot (bt-y0k7).
	StatusMsg      string
	StatusIsErr    bool
	StatusIsInline bool

	// Filter badge
	FilterText string
	FilterIcon string

	// Project badge (single-project mode only)
	ProjectName   string
	WorkspaceMode bool

	// Search badge
	SearchMode string // "" = no search active

	// Sort badge
	SortLabel string // "" = default sort

	// Wisp badge
	ShowWisps bool

	// Context-aware label/hint line
	HintText string

	// Issue counts
	CountOpen    int
	CountReady   int
	CountBlocked int
	CountClosed  int

	// Time travel (overrides normal stats when active)
	TimeTravelActive bool
	TimeTravelStats  string // pre-formatted "⏱ 3d: +5 ✅2 ~3"

	// Background worker badge
	WorkerText  string
	WorkerLevel WorkerLevel

	// Phase 2 progress
	ShowPhase2 bool

	// Watcher mode
	WatcherText string // "" = no badge

	// Self-update badge
	UpdateTag string // "" = no update

	// Dataset warning
	DatasetWarning string
	DatasetLevel   DatasetLevel

	// Alerts
	AlertCount    int
	CriticalCount int
	WarningCount  int

	// Instance warning
	SecondaryPID int // 0 = primary instance

	// Cass session count for selected issue
	SessionCount int

	// Workspace summary
	WorkspaceSummary string

	// Label filter (independent from status filter)
	LabelFilterText string // "" = no label filter

	// Repo filter
	RepoFilterLabel string // "" = no repo filter

	// Key hints (pre-computed list)
	KeyHints []string

	// Total visible items in list
	TotalItems int
}

// footerData extracts all data needed for footer rendering from the Model.
// Auto-dismiss of idle status messages runs on the statusTick path, not here,
// so this method can be safely called as a pure read.
func (m *Model) footerData() FooterData {
	fd := FooterData{
		Width:          m.width,
		StatusMsg:      m.statusMsg,
		StatusIsErr:    m.statusIsError,
		StatusIsInline: m.statusIsInline,
		ShowWisps:      m.showWisps,
		TotalItems:     len(m.list.Items()),
	}

	// Filter badge
	fd.FilterText, fd.FilterIcon = m.extractFilterBadge()

	// Project badge (single-project only)
	if m.projectName != "" && !m.workspaceMode {
		fd.ProjectName = m.projectName
	}
	fd.WorkspaceMode = m.workspaceMode

	// Search badge
	fd.SearchMode = m.extractSearchMode()

	// Sort badge
	if m.filter.sortMode != SortDefault {
		fd.SortLabel = m.filter.sortMode.String()
	}

	// Hint text
	fd.HintText = m.extractHintText()

	// Issue counts
	fd.CountOpen = m.ac.countOpen
	fd.CountReady = m.ac.countReady
	fd.CountBlocked = m.ac.countBlocked
	fd.CountClosed = m.ac.countClosed

	// Time travel
	if m.timeTravelMode && m.timeTravelDiff != nil {
		fd.TimeTravelActive = true
		d := m.timeTravelDiff.Summary
		fd.TimeTravelStats = fmt.Sprintf("⏱ %s: +%d ✅%d ~%d",
			m.timeTravelSince, d.IssuesAdded, d.IssuesClosed, d.IssuesModified)
	}

	// Worker badge
	fd.WorkerText, fd.WorkerLevel = m.extractWorkerBadge()

	// Phase 2 progress
	fd.ShowPhase2 = m.data.snapshot != nil && !m.data.snapshot.Phase2Ready

	// Watcher mode
	fd.WatcherText = m.extractWatcherBadge()

	// Update badge
	if m.updateAvailable {
		fd.UpdateTag = m.updateTag
	}

	// Dataset warning
	fd.DatasetWarning, fd.DatasetLevel = m.extractDatasetWarning()

	// Alerts
	fd.AlertCount, fd.CriticalCount, fd.WarningCount = m.extractAlertCounts()

	// Instance
	if m.data.instanceLock != nil && !m.data.instanceLock.IsFirstInstance() {
		fd.SecondaryPID = m.data.instanceLock.HolderPID()
	}

	// Sessions
	fd.SessionCount = m.getCassSessionCount()

	// Workspace summary
	if m.workspaceMode && m.workspaceSummary != "" {
		fd.WorkspaceSummary = m.workspaceSummary
	}

	// Label filter
	if m.filter.labelFilter != "" {
		parts := strings.Split(m.filter.labelFilter, ",")
		if len(parts) == 1 {
			fd.LabelFilterText = parts[0]
		} else {
			fd.LabelFilterText = fmt.Sprintf("%d labels", len(parts))
		}
	}

	// Repo filter
	if m.workspaceMode && m.activeRepos != nil && len(m.activeRepos) > 0 {
		active := sortedRepoKeys(m.activeRepos)
		fd.RepoFilterLabel = formatRepoList(active, 3)
	}

	// Key hints
	fd.KeyHints = m.extractKeyHints()

	return fd
}

// --- Extract helpers (Model methods that compute FooterData fields) ---

func (m *Model) extractFilterBadge() (text, icon string) {
	if m.focused == focusLabelDashboard {
		return "LABELS: j/k nav • h detail • d drilldown • enter filter", "🏷️"
	}
	if m.activeModal == ModalLabelGraphAnalysis && m.labelGraphAnalysisResult != nil {
		return fmt.Sprintf("GRAPH %s: esc/q/g close", m.labelGraphAnalysisResult.Label), "📊"
	}
	if m.activeModal == ModalLabelDrilldown && m.labelDrilldownLabel != "" {
		return fmt.Sprintf("LABEL %s: enter filter • g graph • esc/q/d close", m.labelDrilldownLabel), "🏷️"
	}
	switch m.filter.currentFilter {
	case "all":
		return "ALL", "📋"
	case "open":
		return "OPEN", "📂"
	case "closed":
		return "CLOSED", "✅"
	case "ready":
		return "READY", "🚀"
	default:
		if strings.HasPrefix(m.filter.currentFilter, "bql:") {
			bqlStr := m.filter.currentFilter[4:]
			if len(bqlStr) > 30 {
				bqlStr = bqlStr[:27] + "..."
			}
			return "BQL: " + bqlStr, "🔍"
		}
		if strings.HasPrefix(m.filter.currentFilter, "recipe:") {
			return strings.ToUpper(m.filter.currentFilter[7:]), "📑"
		}
		return m.filter.currentFilter, "🔍"
	}
}

func (m *Model) extractSearchMode() string {
	if m.list.FilterState() == list.Unfiltered {
		return ""
	}
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
	return mode
}

func (m *Model) extractHintText() string {
	if m.mode == ViewBoard {
		if m.board.IsSearchMode() {
			matchInfo := ""
			if m.board.SearchMatchCount() > 0 {
				matchInfo = fmt.Sprintf(" [%d/%d]", m.board.SearchCursorPos(), m.board.SearchMatchCount())
			}
			return fmt.Sprintf("/%s%s • n/N:match • enter:done • esc:cancel", m.board.SearchQuery(), matchInfo)
		}
		filterInfo := ""
		if m.filter.currentFilter != "all" && m.filter.currentFilter != "" {
			shown := m.board.TotalCount()
			total := len(m.data.issues)
			filterInfo = fmt.Sprintf("[%s:%d/%d] ", m.filter.currentFilter, shown, total)
		}
		return fmt.Sprintf("%s1-4:col • o/c/r:filter • l:labels • /:search • ?:help", filterInfo)
	}
	if m.mode == ViewAttention {
		return "A:attention • 1-9 filter • esc close"
	}
	return "l:labels"
}

func (m *Model) extractWorkerBadge() (string, WorkerLevel) {
	if m.data.backgroundWorker == nil {
		return "", WorkerLevelNone
	}

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

	var freshnessAge time.Duration
	hasFreshnessAge := false
	if !m.lastDoltVerified.IsZero() {
		freshnessAge = time.Since(m.lastDoltVerified)
		hasFreshnessAge = true
	} else if m.data.snapshot != nil && !m.data.snapshot.CreatedAt.IsZero() {
		freshnessAge = time.Since(m.data.snapshot.CreatedAt)
		hasFreshnessAge = true
	}

	state := m.data.backgroundWorker.State()
	health := m.data.backgroundWorker.Health()
	lastErr := m.data.backgroundWorker.LastError()

	switch {
	case health.Started && !health.Alive:
		return "⚠ worker unresponsive", WorkerLevelCritical

	case state == WorkerProcessing && m.data.backgroundWorker.ProcessingDuration() >= 250*time.Millisecond:
		frame := workerSpinnerFrames[m.data.workerSpinnerIdx%len(workerSpinnerFrames)]
		return fmt.Sprintf("%s refreshing", frame), WorkerLevelInfo

	case lastErr != nil && lastErr.Retries >= freshnessErrorRetries:
		return fmt.Sprintf("✗ bg %s (%dx)", lastErr.Phase, lastErr.Retries), WorkerLevelCritical

	case lastErr != nil:
		return fmt.Sprintf("⚠ bg %s (%s)", lastErr.Phase, formatAge(time.Since(lastErr.Time))), WorkerLevelWarning

	case hasFreshnessAge && freshnessAge >= freshnessStaleThreshold():
		return fmt.Sprintf("⚠ STALE: %s ago", formatAge(freshnessAge)), WorkerLevelCritical

	case hasFreshnessAge && freshnessAge >= freshnessWarnThreshold():
		return fmt.Sprintf("⚠ %s ago", formatAge(freshnessAge)), WorkerLevelWarning

	default:
		if health.RecoveryCount > 0 {
			return fmt.Sprintf("↻ recovered x%d", health.RecoveryCount), WorkerLevelWarning
		}
		return "", WorkerLevelNone
	}
}

func (m *Model) extractWatcherBadge() string {
	var (
		polling      bool
		fsType       watcher.FilesystemType
		pollInterval time.Duration
	)

	switch {
	case m.data.backgroundWorker != nil:
		polling, fsType, pollInterval = m.data.backgroundWorker.WatcherInfo()
	case m.data.watcher != nil:
		polling = m.data.watcher.IsPolling()
		fsType = m.data.watcher.FilesystemType()
		pollInterval = m.data.watcher.PollInterval()
	}

	if !polling {
		return ""
	}

	label := "polling"
	if fsType != watcher.FSTypeUnknown && fsType != watcher.FSTypeLocal {
		label = fmt.Sprintf("polling %s", fsType.String())
	}
	if pollInterval > 0 {
		label = fmt.Sprintf("%s %s", label, pollInterval.String())
	}
	return label
}

func (m *Model) extractDatasetWarning() (string, DatasetLevel) {
	if m.data.snapshot == nil || m.data.snapshot.LargeDatasetWarning == "" {
		return "", DatasetLevelNone
	}
	level := DatasetLevelWarning
	if m.data.snapshot.DatasetTier == datasetTierHuge {
		level = DatasetLevelCritical
	}
	return m.data.snapshot.LargeDatasetWarning, level
}

func (m *Model) extractAlertCounts() (total, critical, warning int) {
	for _, a := range m.visibleAlerts() {
		total++
		switch a.Severity {
		case drift.SeverityCritical:
			critical++
		case drift.SeverityWarning:
			warning++
		}
	}
	return
}

func (m *Model) extractKeyHints() []string {
	keyStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Background(ColorBgSubtle).
		Padding(0, 0)

	var hints []string
	if m.activeModal == ModalHelp {
		hints = append(hints, "Press any key to close")
	} else if m.activeModal == ModalRecipePicker {
		hints = append(hints, keyStyle.Render("j/k")+" nav", keyStyle.Render("⏎")+" apply", keyStyle.Render("esc")+" cancel")
	} else if m.activeModal == ModalRepoPicker {
		hints = append(hints, keyStyle.Render("j/k")+" nav", keyStyle.Render("space")+" toggle", keyStyle.Render("⏎")+" apply", keyStyle.Render("esc")+" cancel")
	} else if m.activeModal == ModalLabelPicker {
		hints = append(hints, "type to filter", keyStyle.Render("space")+" toggle", keyStyle.Render("⏎")+" apply", keyStyle.Render("esc")+" close")
	} else if m.mode == ViewInsights {
		hints = append(hints, keyStyle.Render("h/l")+" panels", keyStyle.Render("e")+" explain", keyStyle.Render("⏎")+" jump", keyStyle.Render("?")+" help")
		hints = append(hints, keyStyle.Render("A")+" attention", keyStyle.Render("F")+" flow")
	} else if m.mode == ViewFlowMatrix {
		hints = append(hints, keyStyle.Render("j/k")+" nav", keyStyle.Render("tab")+" panel", keyStyle.Render("⏎")+" drill", keyStyle.Render("esc")+" back", keyStyle.Render("f")+" close")
	} else if m.mode == ViewGraph {
		hints = append(hints, keyStyle.Render("hjkl")+" nav", keyStyle.Render("H/L")+" scroll", keyStyle.Render("⏎")+" view", keyStyle.Render("g")+" list")
	} else if m.mode == ViewBoard {
		hints = append(hints, keyStyle.Render("hjkl")+" nav", keyStyle.Render("G")+" bottom", keyStyle.Render("⏎")+" view", keyStyle.Render("b")+" list")
	} else if m.mode == ViewActionable {
		hints = append(hints, keyStyle.Render("j/k")+" nav", keyStyle.Render("⏎")+" view", keyStyle.Render("a")+" list", keyStyle.Render("?")+" help")
	} else if m.mode == ViewHistory {
		hints = append(hints, keyStyle.Render("j/k")+" nav", keyStyle.Render("tab")+" focus", keyStyle.Render("⏎")+" jump", keyStyle.Render("h")+" close")
	} else if m.list.FilterState() == list.Filtering {
		// Footer reflects the new Ctrl+S three-state cycle (bt-krwp). The mode
		// label after "ctrl+s" is the *current* mode — pressing the key cycles
		// to the next. H is only meaningful in hybrid mode (preset cycle); it's
		// hidden from the footer in other modes to reduce noise.
		mode := "fuzzy"
		if m.semanticSearchEnabled {
			if m.semanticHybridEnabled {
				mode = "hybrid"
				if m.semanticHybridBuilding {
					mode = "hybrid (metrics)"
				}
			} else {
				mode = "semantic"
				if m.semanticIndexBuilding {
					mode = "semantic (indexing)"
				}
			}
		}
		hints = append(hints, keyStyle.Render("esc")+" cancel", keyStyle.Render("ctrl+s")+" "+mode, keyStyle.Render("⏎")+" select")
		if m.semanticHybridEnabled {
			hints = append(hints, keyStyle.Render("H")+" preset")
		}
	} else if m.activeModal == ModalTimeTravelInput {
		hints = append(hints, keyStyle.Render("⏎")+" compare", keyStyle.Render("esc")+" cancel")
	} else {
		if m.timeTravelMode {
			hints = append(hints, keyStyle.Render("t")+" exit diff", keyStyle.Render("C")+" copy", keyStyle.Render("abgi")+" views", keyStyle.Render("?")+" help")
		} else if m.isSplitView {
			hints = append(hints, keyStyle.Render("tab")+" focus", keyStyle.Render("C")+" copy", keyStyle.Render("x")+" export", keyStyle.Render("Ctrl+R")+" refresh", keyStyle.Render("?")+" help")
		} else if m.showDetails {
			hints = append(hints, keyStyle.Render("esc")+" back", keyStyle.Render("C")+" copy", keyStyle.Render("O")+" edit", keyStyle.Render("Ctrl+R")+" refresh", keyStyle.Render("?")+" help")
		} else {
			hints = append(hints, keyStyle.Render("⏎")+" details", keyStyle.Render("t")+" diff", keyStyle.Render("S")+" triage", keyStyle.Render("l")+" labels", keyStyle.Render("Ctrl+R")+" refresh", keyStyle.Render("?")+" help")
			if m.workspaceMode {
				hints = append(hints, keyStyle.Render("w")+" projects")
			}
		}
	}
	return hints
}

// ---------------------------------------------------------------------------
// Render — pure rendering from FooterData, no Model access.
// ---------------------------------------------------------------------------

// Render produces the footer string from pre-computed FooterData.
func (fd FooterData) Render() string {
	// Full-width banner: reserved for errors or explicit user-initiated confirmations.
	// Inline status renders subtly (bt-y0k7) — handled below by overriding HintText.
	if fd.StatusMsg != "" && !fd.StatusIsInline {
		return fd.renderStatusBar()
	}
	if fd.StatusMsg != "" && fd.StatusIsInline {
		prefix := "✓ "
		if fd.StatusIsErr {
			prefix = "✗ "
		}
		fd.HintText = prefix + fd.StatusMsg
	}

	// Filter badge
	filterBadge := lipgloss.NewStyle().
		Background(ColorPrimary).
		Foreground(ColorBgContrast).
		Bold(true).
		Padding(0, 1).
		Render(fmt.Sprintf("%s %s", fd.FilterIcon, fd.FilterText))

	// Project name badge (bt-m9te: use a background so padding renders as visible
	// separator cells, preventing the icon/name from smushing against adjacent badges).
	projectBadge := ""
	if fd.ProjectName != "" && !fd.WorkspaceMode {
		projectBadge = lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorSecondary).
			Padding(0, 1).
			Render(fd.ProjectName)
	}

	// Search mode badge
	searchBadge := ""
	if fd.SearchMode != "" {
		searchBadge = lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorSecondary).
			Padding(0, 1).
			Render(fmt.Sprintf("🔎 %s", fd.SearchMode))
	}

	// Sort badge
	sortBadge := ""
	if fd.SortLabel != "" {
		sortBadge = lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorSecondary).
			Padding(0, 1).
			Render(fmt.Sprintf("↕ %s", fd.SortLabel))
	}

	// Wisp badge
	wispBadge := ""
	if fd.ShowWisps {
		wispBadge = lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorSecondary).
			Padding(0, 1).
			Render("wisps")
	}

	// Label hint
	labelHint := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1).
		Render(fd.HintText)

	// Stats section
	var statsSection string
	if fd.TimeTravelActive {
		timeTravelStyle := lipgloss.NewStyle().
			Background(ColorPrioHighBg).
			Foreground(ColorWarning).
			Padding(0, 1)
		statsSection = timeTravelStyle.Render(fd.TimeTravelStats)
	} else {
		statsStyle := lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorText).
			Padding(0, 1)
		openStyle := lipgloss.NewStyle().Foreground(ColorStatusOpen)
		readyStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
		blockedStyle := lipgloss.NewStyle().Foreground(ColorWarning)
		closedStyle := lipgloss.NewStyle().Foreground(ColorMuted)
		statsContent := fmt.Sprintf("%s%d %s%d %s%d %s%d",
			openStyle.Render("○"), fd.CountOpen,
			readyStyle.Render("◉"), fd.CountReady,
			blockedStyle.Render("◈"), fd.CountBlocked,
			closedStyle.Render("●"), fd.CountClosed)
		statsSection = statsStyle.Render(statsContent)
	}

	// Worker badge
	workerSection := fd.renderWorkerBadge()

	// Phase 2 progress
	phase2Section := ""
	if fd.ShowPhase2 {
		phase2Style := lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorInfo).
			Padding(0, 1)
		phase2Section = phase2Style.Render("◌ metrics…")
	}

	// Watcher badge
	watcherSection := ""
	if fd.WatcherText != "" {
		watcherStyle := lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorMuted).
			Padding(0, 1)
		watcherSection = watcherStyle.Render(fd.WatcherText)
	}

	// Update badge
	updateSection := ""
	if fd.UpdateTag != "" {
		updateStyle := lipgloss.NewStyle().
			Background(ColorTypeFeature).
			Foreground(ColorBg).
			Bold(true).
			Padding(0, 1)
		updateSection = updateStyle.Render(fmt.Sprintf("⭐ %s", fd.UpdateTag))
	}

	// Dataset warning
	datasetSection := ""
	if fd.DatasetWarning != "" {
		bg, fg := ColorPrioHighBg, ColorWarning
		if fd.DatasetLevel == DatasetLevelCritical {
			bg, fg = ColorPrioCriticalBg, ColorPrioCritical
		}
		datasetStyle := lipgloss.NewStyle().
			Background(bg).
			Foreground(fg).
			Bold(true).
			Padding(0, 1)
		datasetSection = datasetStyle.Render(fd.DatasetWarning)
	}

	// Alerts badge
	alertsSection := fd.renderAlertsBadge()

	// Instance warning
	instanceSection := ""
	if fd.SecondaryPID > 0 {
		instanceStyle := lipgloss.NewStyle().
			Background(ColorPrioHighBg).
			Foreground(ColorWarning).
			Bold(true).
			Padding(0, 1)
		instanceSection = instanceStyle.Render(fmt.Sprintf("⚠ PID %d", fd.SecondaryPID))
	}

	// Session indicator
	sessionSection := ""
	if fd.SessionCount > 0 {
		sessionStyle := lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorInfo).
			Padding(0, 1)
		countStr := fmt.Sprintf("%d", fd.SessionCount)
		if fd.SessionCount > 9 {
			countStr = "9+"
		}
		sessionSection = sessionStyle.Render(fmt.Sprintf("📎%s", countStr))
	}

	// Workspace badge
	workspaceSection := ""
	if fd.WorkspaceSummary != "" {
		workspaceStyle := lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(ColorBg).
			Bold(true).
			Padding(0, 1)
		workspaceSection = workspaceStyle.Render(fmt.Sprintf("📦 %s", fd.WorkspaceSummary))
	}

	// Repo filter badge
	// Label filter badge
	labelFilterSection := ""
	if fd.LabelFilterText != "" {
		labelStyle := lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorInfo).
			Bold(true).
			Padding(0, 1)
		labelFilterSection = labelStyle.Render(fmt.Sprintf("🏷 %s", fd.LabelFilterText))
	}

	repoFilterSection := ""
	if fd.RepoFilterLabel != "" {
		repoStyle := lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorInfo).
			Bold(true).
			Padding(0, 1)
		repoFilterSection = repoStyle.Render(fmt.Sprintf("🗂 %s", fd.RepoFilterLabel))
	}

	// Key hints
	sepStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	sep := sepStyle.Render(" │ ")
	keysStyle := lipgloss.NewStyle().
		Foreground(ColorSubtext).
		Padding(0, 1)

	countBadge := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Padding(0, 1).
		Render(fmt.Sprintf("%d issues", fd.TotalItems))

	keyHints := make([]string, len(fd.KeyHints))
	copy(keyHints, fd.KeyHints)
	keysSection := keysStyle.Render(strings.Join(keyHints, sep))

	// Progressive truncation: drop middle hints until they fit
	if len(keyHints) > 2 {
		availableWidth := fd.Width - lipgloss.Width(countBadge) - 2
		for len(keyHints) > 2 && lipgloss.Width(keysSection) > availableWidth {
			keyHints = append(keyHints[:len(keyHints)-2], keyHints[len(keyHints)-1])
			keysSection = keysStyle.Render(strings.Join(keyHints, sep))
		}
	}

	// Width-aware compression (bt-m9te): assign each optional badge a priority
	// tier (0 = always keep, 1/2/3 = drop progressively as width narrows). When
	// total width exceeds available space, drop highest-tier badges first. This
	// prevents line-wrapping at narrow widths.
	type footerBadge struct {
		content string
		tier    int
	}
	// Tier 1 (drop first): least critical, rarely-changing info.
	// Tier 2 (drop second): useful but secondary info.
	// Tier 3 (drop third): contextual chrome that duplicates info in keysSection.
	optional := map[string]*footerBadge{
		"projectBadge":       {projectBadge, 3},
		"searchBadge":        {searchBadge, 3},
		"sortBadge":          {sortBadge, 3},
		"wispBadge":          {wispBadge, 3},
		"labelFilterSection": {labelFilterSection, 3},
		"workspaceSection":   {workspaceSection, 2},
		"repoFilterSection":  {repoFilterSection, 2},
		"sessionSection":     {sessionSection, 2},
		"updateSection":      {updateSection, 1},
		"datasetSection":     {datasetSection, 1},
		"watcherSection":     {watcherSection, 1},
		"phase2Section":      {phase2Section, 1},
		// Tier 0 (always keep): alerts, instance, worker status, stats.
		"alertsSection":   {alertsSection, 0},
		"instanceSection": {instanceSection, 0},
		"workerSection":   {workerSection, 0},
	}

	measure := func() int {
		w := lipgloss.Width(filterBadge) + lipgloss.Width(labelHint) + lipgloss.Width(statsSection)
		for _, b := range optional {
			if b.content != "" {
				w += lipgloss.Width(b.content) + 1
			}
		}
		w += lipgloss.Width(countBadge) + lipgloss.Width(keysSection) + 1
		return w
	}

	for tier := 3; tier >= 1; tier-- {
		if measure() <= fd.Width {
			break
		}
		for _, b := range optional {
			if b.tier == tier {
				b.content = ""
			}
		}
	}

	// Recompute assembled widths after any drops.
	leftWidth := lipgloss.Width(filterBadge) + lipgloss.Width(labelHint) + lipgloss.Width(statsSection)
	for _, b := range optional {
		if b.content != "" {
			leftWidth += lipgloss.Width(b.content) + 1
		}
	}
	rightWidth := lipgloss.Width(countBadge) + lipgloss.Width(keysSection)

	remaining := fd.Width - leftWidth - rightWidth - 1
	if remaining < 0 {
		remaining = 0
	}
	filler := lipgloss.NewStyle().Width(remaining).Render("")

	// Build the footer in display order (content may be empty after compression).
	var parts []string
	parts = append(parts, filterBadge)
	addIf := func(s string) {
		if s != "" {
			parts = append(parts, s)
		}
	}
	addIf(optional["projectBadge"].content)
	addIf(optional["searchBadge"].content)
	addIf(optional["sortBadge"].content)
	addIf(optional["wispBadge"].content)
	addIf(optional["labelFilterSection"].content)
	parts = append(parts, labelHint)
	addIf(optional["alertsSection"].content)
	addIf(optional["instanceSection"].content)
	addIf(optional["sessionSection"].content)
	addIf(optional["workspaceSection"].content)
	addIf(optional["repoFilterSection"].content)
	addIf(optional["updateSection"].content)
	addIf(optional["datasetSection"].content)
	parts = append(parts, statsSection)
	addIf(optional["phase2Section"].content)
	addIf(optional["watcherSection"].content)
	addIf(optional["workerSection"].content)
	parts = append(parts, filler, countBadge, keysSection)

	return lipgloss.JoinHorizontal(lipgloss.Bottom, parts...)
}

func (fd FooterData) renderStatusBar() string {
	var msgStyle lipgloss.Style
	if fd.StatusIsErr {
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
	if fd.StatusIsErr {
		prefix = "✗ "
	}
	displayMsg := prefix + fd.StatusMsg
	if maxMsgWidth := fd.Width - 4; lipgloss.Width(displayMsg) > maxMsgWidth {
		displayMsg = truncateString(displayMsg, maxMsgWidth)
	}
	msgSection := msgStyle.Render(displayMsg)
	remaining := fd.Width - lipgloss.Width(msgSection)
	if remaining < 0 {
		remaining = 0
	}
	filler := lipgloss.NewStyle().Width(remaining).Render("")
	return lipgloss.JoinHorizontal(lipgloss.Bottom, msgSection, filler)
}

func (fd FooterData) renderWorkerBadge() string {
	if fd.WorkerText == "" {
		return ""
	}
	var style lipgloss.Style
	switch fd.WorkerLevel {
	case WorkerLevelCritical:
		style = lipgloss.NewStyle().
			Background(ColorPrioCriticalBg).
			Foreground(ColorPrioCritical).
			Bold(true).
			Padding(0, 1)
	case WorkerLevelWarning:
		style = lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorWarning).
			Bold(true).
			Padding(0, 1)
	case WorkerLevelInfo:
		style = lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorInfo).
			Bold(true).
			Padding(0, 1)
	default:
		return ""
	}
	return style.Render(fd.WorkerText)
}

func (fd FooterData) renderAlertsBadge() string {
	if fd.AlertCount == 0 {
		return ""
	}
	var alertStyle lipgloss.Style
	var alertIcon string
	if fd.CriticalCount > 0 {
		alertStyle = lipgloss.NewStyle().
			Background(ColorPrioCriticalBg).
			Foreground(ColorPrioCritical).
			Bold(true).
			Padding(0, 1)
		alertIcon = "⚠"
	} else if fd.WarningCount > 0 {
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
	return alertStyle.Render(fmt.Sprintf("%s %d alerts (!)", alertIcon, fd.AlertCount))
}

// renderFooter is the Model method that produces the footer string.
// It delegates to FooterData for the actual rendering (bt-oim6).
func (m *Model) renderFooter() string {
	return m.footerData().Render()
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
