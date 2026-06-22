package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/search"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
	"github.com/seanmartinsmith/beadstui/pkg/watcher"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// setInlineTransientStatus sets a subtle status that renders in the footer hint slot
// (not a full-width banner) and auto-clears after the given duration. Use for background
// notifications that should not clobber key hints (bt-y0k7).
func (m *Model) setInlineTransientStatus(msg string, d time.Duration) tea.Cmd {
	m.statusMsg = msg
	m.statusSeverity = SeveritySuccess
	m.statusIsInline = true
	m.statusSetAt = time.Now()
	m.statusSeq++
	seq := m.statusSeq
	return tea.Tick(d, func(time.Time) tea.Msg {
		return statusClearMsg{seq: seq}
	})
}

// setStatus sets a success/info confirmation toast (✓, ~3s auto-fade, no bell).
func (m *Model) setStatus(msg string) {
	m.statusMsg = msg
	m.statusSeverity = SeveritySuccess
	m.statusIsInline = true
	m.statusSetAt = time.Now()
}

// setStatusError sets a failure toast. TEMPORARY: Task 4 reclassifies callers
// to setNotice/setFailure/setDegraded; until then this maps to Failure to
// preserve "error" behavior.
func (m *Model) setStatusError(msg string) {
	m.statusMsg = msg
	m.statusSeverity = SeverityFailure
	m.statusIsInline = true
	m.statusSetAt = time.Now()
}

// setNotice sets a Notice toast (rejection/validation; ~3s; no bell entry).
func (m *Model) setNotice(msg string) {
	m.statusMsg = msg
	m.statusSeverity = SeverityNotice
	m.statusIsInline = true
	m.statusSetAt = time.Now()
}

// setFailure sets a Failure toast (one-shot op failure; ~8s) and records it
// in the events ring buffer so it survives in the alerts modal.
func (m *Model) setFailure(msg string) {
	m.statusMsg = msg
	m.statusSeverity = SeverityFailure
	m.statusIsInline = true
	m.statusSetAt = time.Now()
	if m.events != nil {
		m.events.Append(events.NewSystemEvent(msg))
	}
}

// setDegraded sets a Degraded toast (live condition; sticky until the
// recovery path clears it) and records it in the ring buffer.
func (m *Model) setDegraded(msg string) {
	m.statusMsg = msg
	m.statusSeverity = SeverityDegraded
	m.statusIsInline = true
	m.statusSetAt = time.Now()
	if m.events != nil {
		m.events.Append(events.NewSystemEvent(msg))
	}
}

// clearStatus clears any active toast (used by the recovery path to drop a
// sticky Degraded toast once the condition resolves).
func (m *Model) clearStatus() {
	m.statusMsg = ""
	m.statusSeverity = SeverityNone
	m.statusIsInline = false
}

// statusDismissAge is how long a toast of the given severity stays before
// the idle tick clears it. Degraded returns 0 (sticky - cleared only by the
// recovery path; see handleSnapshotReady).
func statusDismissAge(s StatusSeverity) time.Duration {
	switch s {
	case SeverityFailure:
		return 8 * time.Second
	case SeverityDegraded:
		return 0
	default: // Success, Notice
		return 3 * time.Second
	}
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

// StatusSeverity classifies a footer toast (bt-a3zi3.1). It drives the
// glyph, the auto-dismiss lifetime (see statusDismissAge), and whether the
// toast is also recorded in the events ring buffer (Failure/Degraded are).
type StatusSeverity int

const (
	SeverityNone     StatusSeverity = iota // no status message
	SeveritySuccess                        // ✓ confirmation; ~3s; no bell
	SeverityNotice                         // rejection/validation; ~3s; no bell
	SeverityFailure                        // ✗ one-shot failure; ~8s; bell
	SeverityDegraded                       // ⚠ live condition; sticky; bell
)

// glyph is the leading symbol for a toast of this severity ("" = none).
func (s StatusSeverity) glyph() string {
	switch s {
	case SeveritySuccess:
		return "✓"
	case SeverityFailure:
		return "✗"
	case SeverityDegraded:
		return "⚠"
	default:
		return ""
	}
}

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
	StatusSeverity StatusSeverity
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

	// Key hints (pre-computed, structured so the renderer can degrade them
	// from full "key desc" pills down to key-only glyphs as width tightens).
	Hints []FooterHint

	// Total visible items in list
	TotalItems int

	// Per-view center-zone override (Phase 3). When non-empty, it replaces the
	// scoped status stats + "N issues" count with view-specific "what am I
	// looking at" meaning (detail = bead id + position, graph = nodes/edges,
	// board = columns/cards). "" = default scoped counts. Supplied by
	// footerCenter(); the override carries its own count, so the count badge is
	// suppressed when it is set.
	CenterOverride string

	// Unread bell (Phase 4): events newer than alertsSeenAt and not dismissed.
	// Always rendered as 🔔; the count suffix appears only when > 0.
	BellCount int
}

// FooterHint is one key-binding hint for the L1 status-bar slot. Key is the
// glyph(s) ("⏎", "Ctrl+R", "?"); Desc is the human label ("open detail").
// Styling and full-vs-key-only rendering are decided in Render(), not here,
// so the degradation engine can choose the densest form that fits.
type FooterHint struct {
	Key  string
	Desc string
}

// footerData extracts all data needed for footer rendering from the Model.
// Auto-dismiss of idle status messages runs on the statusTick path, not here,
// so this method can be safely called as a pure read.
func (m *Model) footerData() FooterData {
	fd := FooterData{
		Width:          m.width,
		StatusMsg:      m.statusMsg,
		StatusSeverity: m.statusSeverity,
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
	fd.Hints = m.extractKeyHints()

	// Per-view center meaning (Phase 3)
	fd.CenterOverride = m.footerCenter()

	// Footer bell: unseen-since-last-look count from the ring buffer.
	if m.events != nil {
		fd.BellCount = m.events.UnseenCount(m.alertsSeenAt)
	}

	return fd
}

// footerCenter supplies the center-zone string for views whose "what am I
// looking at" summary is more useful than the default scoped status counts
// (Phase 3). Detail = bead id + position, graph = nodes/edges, board = visible
// columns + cards. Returns "" for views (list, tree, insights, …) that keep the
// scoped counts. Mirrors viewKeyMap(); detail is a sub-state of ViewList rather
// than its own mode, so it is handled before the mode switch.
func (m *Model) footerCenter() string {
	// A modal overlays the underlying view; keep that view's default counts.
	if m.activeModal != ModalNone {
		return ""
	}

	// Detail: full-screen detail or split-view with the detail pane focused.
	if m.mode == ViewList && ((m.showDetails && !m.isSplitView) || (m.isSplitView && m.focused == focusDetail)) {
		sel, ok := m.list.SelectedItem().(IssueItem)
		if !ok {
			return ""
		}
		if total := len(m.list.VisibleItems()); total > 0 {
			return fmt.Sprintf("%s · %d/%d", sel.Issue.ID, m.list.Index()+1, total)
		}
		return sel.Issue.ID
	}

	switch m.mode {
	case ViewGraph:
		return fmt.Sprintf("%s · %s",
			countLabel(m.graphView.TotalCount(), "node"),
			countLabel(m.graphView.EdgeCount(), "edge"))
	case ViewBoard:
		return fmt.Sprintf("%s · %s",
			countLabel(m.board.VisibleColumnCount(), "col"),
			countLabel(m.board.TotalCount(), "card"))
	}
	return ""
}

// countLabel formats "N word" with a plural "s" suffix when N != 1
// (1 node / 47 nodes, 1 col / 4 cols).
func countLabel(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
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

// extractKeyHints renders the L1 status-bar hint slot from the active
// help.KeyMap's ShortHelp(). Per ADR-004 Decision 1, the prior 12-branch
// if/else chain was deleted wholesale in bt-ift6.1; modal map takes
// precedence over the view map, both routed through l1KeyMap().
//
// Foundation phase note: most per-view Maps and modal Maps are stubs in
// bt-ift6.1 (only Global + Tree are wired). L1 will show empty hints in
// views/modals whose Maps haven't been populated yet — bt-ift6.2-.9
// fill them in. setInlineTransientStatus pre-empts ShortHelp() during
// its display window unchanged (bt-y0k7).
func (m *Model) extractKeyHints() []FooterHint {
	km := m.l1KeyMap()
	if km == nil {
		return nil
	}
	bindings := km.ShortHelp()
	if len(bindings) == 0 {
		return nil
	}
	hints := make([]FooterHint, 0, len(bindings))
	for _, b := range bindings {
		if !b.Enabled() {
			continue
		}
		h := b.Help()
		if h.Key == "" {
			continue
		}
		hints = append(hints, FooterHint{Key: h.Key, Desc: h.Desc})
	}
	return hints
}

// l1KeyMap returns the help.KeyMap whose ShortHelp() drives the L1
// footer hint slot. Modal map takes precedence when active; otherwise
// the view-scoped map. Returns nil when no map exists for the current
// state — L1 shows nothing in that case (foundation phase; per-view
// Maps land in bt-ift6.2-.9).
func (m Model) l1KeyMap() help.KeyMap {
	if m.activeModal != ModalNone {
		return m.modalKeyMap()
	}
	return m.viewKeyMap()
}

// viewKeyMap maps m.mode to the matching pkg/ui/keys map. Only views
// whose Maps are wired return non-nil; the rest fall through to nil
// until their conversion child lands.
//
// ViewList has a sub-state branch per ADR-004 Decision 7 — when filter
// typing is active, the truthful help is ListSearchKeys (apply / cancel /
// result-nav), otherwise ListNormalKeys (the full action set).
func (m Model) viewKeyMap() help.KeyMap {
	switch m.mode {
	case ViewList:
		if m.list.FilterState() == list.Filtering {
			return m.keys.ListSearch
		}
		return m.keys.ListNormal
	case ViewTree:
		return m.keys.Tree
	case ViewBoard:
		if m.board.IsSearchMode() {
			return m.keys.BoardSearch
		}
		return m.keys.BoardNormal
	case ViewGraph:
		return m.keys.Graph
	case ViewInsights:
		return m.keys.Insights
	case ViewActionable:
		return m.keys.Actionable
	case ViewFlowMatrix:
		return m.keys.FlowMatrix
	case ViewHistory:
		switch {
		case m.historyView.IsSearchActive():
			return m.keys.HistorySearch
		case m.historyView.FileTreeHasFocus():
			return m.keys.HistoryFileTree
		default:
			return m.keys.HistoryNormal
		}
	case ViewEpics:
		return m.keys.Epics
	}
	// Unmapped views (Attention, LabelDashboard) fall back to global nav so the
	// L1 slot is never empty — a few global keys beat a blank footer. Their
	// view-specific nav still lives in the body/filter slot until dedicated
	// Maps land.
	return m.keys.Global
}

// modalKeyMap maps m.activeModal to the matching modal map. bt-ift6.9
// wires modal Maps (LabelPickerNavKeys, BQLQueryKeys, RecipePickerKeys,
// etc.). Until then, modals return nil and L1 shows nothing while a
// modal is open.
func (m Model) modalKeyMap() help.KeyMap {
	switch m.activeModal {
	case ModalLabelPicker:
		if m.labelPicker.IsSearchFocused() {
			return m.keys.LabelPickerSearch
		}
		return m.keys.LabelPickerNav
	case ModalRecipePicker:
		return m.keys.RecipePicker
	case ModalBQLQuery:
		return m.keys.BQLQuery
	case ModalTimeTravelInput:
		return m.keys.TimeTravelInput
	case ModalRepoPicker:
		return m.keys.RepoPicker
	case ModalEpicCard:
		return m.keys.EpicCard
	}
	// Other modals (help, alerts, tutorial, quit-confirm, agent prompt, …)
	// carry their own internal footers; the L1 slot stays empty for them.
	return nil
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

	// Stats section — built via a closure so the degradation engine can rebuild
	// it at a denser tier (skip zero-count segments) before dropping it whole.
	buildStats := func(skipZeros bool) string {
		if fd.TimeTravelActive {
			return lipgloss.NewStyle().
				Background(ColorPrioHighBg).
				Foreground(ColorWarning).
				Padding(0, 1).
				Render(fd.TimeTravelStats)
		}
		statsStyle := lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorText).
			Padding(0, 1)
		seg := func(style lipgloss.Style, glyph string, n int) string {
			if skipZeros && n == 0 {
				return ""
			}
			return fmt.Sprintf("%s%d", style.Render(glyph), n)
		}
		var segs []string
		for _, s := range []string{
			seg(lipgloss.NewStyle().Foreground(ColorStatusOpen), "○", fd.CountOpen),
			seg(lipgloss.NewStyle().Foreground(ColorSuccess), "◉", fd.CountReady),
			seg(lipgloss.NewStyle().Foreground(ColorWarning), "◈", fd.CountBlocked),
			seg(lipgloss.NewStyle().Foreground(ColorMuted), "●", fd.CountClosed),
		} {
			if s != "" {
				segs = append(segs, s)
			}
		}
		if len(segs) == 0 {
			return ""
		}
		return statsStyle.Render(strings.Join(segs, " "))
	}
	statsSection := buildStats(false)

	// Per-view center override (Phase 3): detail/graph/board replace the scoped
	// status stats with view-specific meaning ("bt-0qzp · 3/169",
	// "47 nodes · 61 edges", "4 cols · 169 cards"). It occupies the same center
	// slot as the stats and degrades the same way (dropped wholesale under
	// extreme width pressure). Time travel keeps precedence — its diff is a
	// corpus-wide signal that out-ranks per-view counts.
	hasCenterOverride := fd.CenterOverride != "" && !fd.TimeTravelActive
	if hasCenterOverride {
		statsSection = lipgloss.NewStyle().
			Background(ColorBgHighlight).
			Foreground(ColorText).
			Padding(0, 1).
			Render(fd.CenterOverride)
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

	// --- Right zone: key hints --------------------------------------------
	// Rendered from structured FooterHint values so the degradation engine can
	// pick the densest form that fits: full "key desc" pills, then key-only
	// glyphs, then fewer pills, then none. The first and last hint (last is
	// usually "?") survive longest. Styling lives here, not in extractKeyHints.
	sep := lipgloss.NewStyle().Foreground(ColorMuted).Render(" │ ")
	keysStyle := lipgloss.NewStyle().Foreground(ColorSubtext).Padding(0, 1)
	keyGlyph := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBgSubtle)

	pill := func(h FooterHint, keysOnly bool) string {
		if keysOnly || h.Desc == "" {
			return keyGlyph.Render(h.Key)
		}
		return keyGlyph.Render(h.Key) + " " + h.Desc
	}
	buildKeys := func(hs []FooterHint, keysOnly bool) string {
		parts := make([]string, len(hs))
		for i, h := range hs {
			parts[i] = pill(h, keysOnly)
		}
		return keysStyle.Render(strings.Join(parts, sep))
	}
	// renderKeys returns the styled key section that best fills avail columns:
	// full labels with as many pills as fit (down to 2, keeping first+last),
	// then the same key-only, then a single key-only hint, then nothing.
	renderKeys := func(avail int) string {
		if avail <= 0 || len(fd.Hints) == 0 {
			return ""
		}
		// Candidate lists from full down to 2 pills, dropping interior hints
		// while keeping the first and the last.
		var seqs [][]FooterHint
		hs := append([]FooterHint(nil), fd.Hints...)
		seqs = append(seqs, hs)
		for len(hs) > 2 {
			next := append([]FooterHint{}, hs[:len(hs)-2]...)
			hs = append(next, hs[len(hs)-1])
			seqs = append(seqs, hs)
		}
		for _, keysOnly := range []bool{false, true} {
			for _, cand := range seqs {
				if s := buildKeys(cand, keysOnly); lipgloss.Width(s) <= avail {
					return s
				}
			}
		}
		// Last resort: just the final hint (usually "?"), key-only.
		if s := buildKeys(fd.Hints[len(fd.Hints)-1:], true); lipgloss.Width(s) <= avail {
			return s
		}
		return ""
	}

	countStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Padding(0, 1)
	countBadge := countStyle.Render(fmt.Sprintf("%d issues", fd.TotalItems))
	countBadgeShort := countStyle.Render(fmt.Sprintf("%d", fd.TotalItems))
	if hasCenterOverride {
		// The override carries its own count semantics (position/edges/cards);
		// the global "N issues" badge would be redundant or wrong here.
		countBadge = ""
		countBadgeShort = ""
	}

	// Scope-icon-only fallback for the filter badge (last-ditch left-zone shrink).
	filterIcon := filterBadge
	if fd.FilterIcon != "" {
		filterIcon = lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(ColorBgContrast).
			Bold(true).
			Padding(0, 1).
			Render(fd.FilterIcon)
	}

	// Width-aware compression (bt-m9te + smart-footer redesign): optional badges
	// carry a priority tier (0 = always keep, 1/2/3 = drop progressively). The
	// degradation engine reduces non-key content in priority order until the
	// always-present core plus a minimal key reserve fits, then fills the
	// remaining width with as many key hints as fit. A final ansi truncate makes
	// wrapping structurally impossible.
	type footerBadge struct {
		content string
		tier    int
	}
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
		// Tier 0 (always keep): alerts, instance, worker status.
		"alertsSection":   {alertsSection, 0},
		"instanceSection": {instanceSection, 0},
		"workerSection":   {workerSection, 0},
	}

	// nonKeyWidth sums everything except the key hints (which fill the remainder).
	nonKeyWidth := func() int {
		w := lipgloss.Width(filterBadge) + lipgloss.Width(labelHint) +
			lipgloss.Width(statsSection) + lipgloss.Width(countBadge)
		for _, b := range optional {
			if b.content != "" {
				w += lipgloss.Width(b.content)
			}
		}
		return w
	}
	dropTier := func(t int) {
		for _, b := range optional {
			if b.tier == t {
				b.content = ""
			}
		}
	}

	// Reserve a minimal sliver for the most important hint when hints exist, so
	// the cascade frees space for it rather than dropping it last.
	keyReserve := 0
	if len(fd.Hints) > 0 {
		keyReserve = lipgloss.Width(buildKeys(fd.Hints[len(fd.Hints)-1:], true))
	}

	// Ordered reductions: low-value chrome first, identity-critical content last.
	reductions := []func(){
		func() { dropTier(3) },
		func() { dropTier(2) },
		func() {
			if !hasCenterOverride { // override has no zero-count segments to drop
				statsSection = buildStats(true)
			}
		},
		func() { dropTier(1) },
		func() { statsSection = "" },            // drop per-status stats / center override; total survives
		func() { countBadge = countBadgeShort }, // "4921 issues" -> "4921"
		func() { labelHint = "" },               // "l:labels" duplicates the l key hint
		func() { filterBadge = filterIcon },     // scope glyph only
	}
	for _, reduce := range reductions {
		if nonKeyWidth()+keyReserve <= fd.Width {
			break
		}
		reduce()
	}

	keysSection := renderKeys(fd.Width - nonKeyWidth())

	// Toast override (Phase 4): an active inline toast borrows the right zone,
	// replacing the key hints; it yields back when the toast clears.
	rightZone := keysSection
	if fd.StatusMsg != "" && fd.StatusIsInline {
		glyph := fd.StatusSeverity.glyph()
		text := fd.StatusMsg
		if glyph != "" {
			text = glyph + " " + text
		}
		var toastStyle lipgloss.Style
		switch fd.StatusSeverity {
		case SeverityFailure:
			toastStyle = lipgloss.NewStyle().Foreground(ColorPrioCritical).Bold(true).Padding(0, 1)
		case SeverityDegraded:
			toastStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Padding(0, 1)
		case SeverityNotice:
			toastStyle = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)
		default: // Success
			toastStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Padding(0, 1)
		}
		avail := fd.Width - nonKeyWidth()
		toast := toastStyle.Render(text)
		if avail > 0 && lipgloss.Width(toast) > avail {
			toast = ansi.Truncate(toast, avail, "")
		}
		rightZone = toast
	}

	// Bell badge (Phase 4): always rendered; the count appears only when > 0.
	// Pinned (last to drop) alongside the ? hint.
	bellText := "🔔"
	if fd.BellCount > 0 {
		bellText = fmt.Sprintf("🔔%d", fd.BellCount)
	}
	bellStyle := lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)
	if fd.BellCount > 0 {
		bellStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Padding(0, 1)
	}
	bellSection := bellStyle.Render(bellText)

	// Filler pushes the count + key hints to the right edge.
	remaining := fd.Width - nonKeyWidth() - lipgloss.Width(rightZone) - lipgloss.Width(bellSection)
	if remaining < 0 {
		remaining = 0
	}
	filler := lipgloss.NewStyle().Width(remaining).Render("")

	// Build the footer in display order (content may be empty after compression).
	var parts []string
	addIf := func(s string) {
		if s != "" {
			parts = append(parts, s)
		}
	}
	addIf(filterBadge)
	addIf(optional["projectBadge"].content)
	addIf(optional["searchBadge"].content)
	addIf(optional["sortBadge"].content)
	addIf(optional["wispBadge"].content)
	addIf(optional["labelFilterSection"].content)
	addIf(labelHint)
	addIf(optional["alertsSection"].content)
	addIf(optional["instanceSection"].content)
	addIf(optional["sessionSection"].content)
	addIf(optional["workspaceSection"].content)
	addIf(optional["repoFilterSection"].content)
	addIf(optional["updateSection"].content)
	addIf(optional["datasetSection"].content)
	addIf(statsSection)
	addIf(optional["phase2Section"].content)
	addIf(optional["watcherSection"].content)
	addIf(optional["workerSection"].content)
	parts = append(parts, filler)
	addIf(countBadge)
	addIf(rightZone)
	addIf(bellSection)

	footer := lipgloss.JoinHorizontal(lipgloss.Bottom, parts...)

	// Final safety net: a single pathological badge (long BQL filter, long worker
	// error) can still overrun. Hard-truncate ANSI-aware so the footer can never
	// wrap to a second row and steal a content line.
	if ansi.StringWidth(footer) > fd.Width {
		footer = ansi.Truncate(footer, fd.Width, "")
	}
	return footer
}

func (fd FooterData) renderStatusBar() string {
	var msgStyle lipgloss.Style
	if fd.StatusSeverity >= SeverityFailure {
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
	prefix := fd.StatusSeverity.glyph()
	if prefix != "" {
		prefix += " "
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
	return alertStyle.Render(fmt.Sprintf("%s %d (!)", alertIcon, fd.AlertCount))
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
