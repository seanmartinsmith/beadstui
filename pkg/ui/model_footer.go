package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/search"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
	"github.com/seanmartinsmith/beadstui/pkg/watcher"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
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

// setStatus sets a success/info confirmation toast (~3s auto-fade, no bell).
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

// markNotificationsSeen advances the footer bell's high-water-mark so the
// badge clears, without dismissing any modal items (seen != dismissed,
// bt-a3zi3.1).
func (m *Model) markNotificationsSeen() {
	m.alertsSeenAt = time.Now()
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
	SeveritySuccess                        // confirmation; ~3s; no bell
	SeverityNotice                         // rejection/validation; ~3s; no bell
	SeverityFailure                        // one-shot failure; ~8s; bell
	SeverityDegraded                       // live condition; sticky; bell
)

// glyph is the leading symbol for a toast of this severity ("" = none).
func (s StatusSeverity) glyph() string {
	switch s {
	case SeveritySuccess:
		return activeGlyphs.Success
	case SeverityFailure:
		return activeGlyphs.Cross
	case SeverityDegraded:
		return activeGlyphs.Warning
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
	TimeTravelStats  string // pre-formatted, e.g. "3d: +5 done2 ~3"

	// Background worker badge
	WorkerText  string
	WorkerLevel WorkerLevel

	// Phase 2 progress
	ShowPhase2 bool

	// Watcher mode
	WatcherText string // "" = no badge

	// Self-update badge
	UpdateTag string // "" = no update

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
	// Always rendered as the bell glyph; the count suffix appears only when > 0.
	BellCount int

	// --- Zone 1 lens inputs (bt-2vshd) ------------------------------------
	// The lens (footer_lens.go) renders these as a scope -> filter -> order
	// sentence. They replace the old left-zone badges (filter/project/search/
	// sort/wisp/label/repo/workspace); those fields above are retained only for
	// the tests and callers that still set them, but no longer drive the chrome.

	// ScopeLabel is the leftmost "where am I": a single project name ("bt") or
	// the cross-project scope with a count ("ALL(19)"), or the active repo subset
	// in workspace mode. Empty renders no scope segment.
	ScopeLabel string
	// ScopeCrossProject selects the globe glyph (all-projects) over the folder
	// glyph (single project) in the Nerd Font tier.
	ScopeCrossProject bool
	// StatusFilter is the raw status membership token the lens status chip shows
	// (all/open/in_progress/blocked/closed/deferred/ready). Empty when a BQL
	// query or recipe owns membership instead.
	StatusFilter string
	// SearchQuery is the "/" slot content — the active fuzzy/BQL query (or search
	// mode when no query text yet). Empty renders the /- placeholder.
	SearchQuery string
	// RecipeName is the active recipe chip; empty renders no recipe chip.
	RecipeName string
	// OrderLabel is the "by:" order token for an explicit sort; empty hides it.
	OrderLabel string
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
		fd.TimeTravelStats = fmt.Sprintf("%s %s: +%d %s%d ~%d",
			activeGlyphs.Clock, m.timeTravelSince, d.IssuesAdded, activeGlyphs.Success, d.IssuesClosed, d.IssuesModified)
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
		displayed := make([]string, len(active))
		for i, k := range active {
			// DisplayRepoName aliases beads_global to "atlas" for display
			// (bt-z1pzj); the underlying activeRepos filter keys stay
			// "beads_global" unchanged.
			displayed[i] = model.DisplayRepoName(k)
		}
		fd.RepoFilterLabel = formatRepoList(displayed, 3)
	}

	// Key hints
	fd.Hints = m.extractKeyHints()

	// Per-view center meaning (Phase 3)
	fd.CenterOverride = m.footerCenter()

	// Footer bell: unseen-since-last-look count, scoped to the active project
	// so a single-project/embedded session doesn't badge a cross-project total
	// from the shared ~/.bt/events.jsonl store (bt-to6vn).
	if m.events != nil {
		fd.BellCount = m.unseenNotificationCount(m.alertsSeenAt)
	}

	// --- Zone 1 lens (bt-2vshd) -------------------------------------------
	m.populateLens(&fd)

	return fd
}

// populateLens fills the FooterData lens inputs (scope / status / search / recipe
// / order) from Model state (bt-2vshd). Kept separate from footerData so the lens
// grammar has one obvious source and the extract is easy to test.
func (m *Model) populateLens(fd *FooterData) {
	// Scope: single project name, or cross-project ALL(N) / active repo subset.
	if m.workspaceMode {
		fd.ScopeCrossProject = true
		if fd.RepoFilterLabel != "" {
			// An active repo subset IS the honest scope in workspace mode; it
			// replaces the old separate repo-filter badge (bt-2vshd).
			fd.ScopeLabel = fd.RepoFilterLabel
		} else if n := len(m.availableRepos); n > 0 {
			fd.ScopeLabel = fmt.Sprintf("ALL(%d)", n)
		} else {
			fd.ScopeLabel = "ALL"
		}
	} else {
		fd.ScopeCrossProject = false
		fd.ScopeLabel = m.projectName
	}

	// Filter bucket. BQL and recipe own membership when active and render in
	// their own slots; otherwise the plain status filter owns the status chip.
	cf := m.filter.currentFilter
	switch {
	case strings.HasPrefix(cf, "bql:"):
		fd.SearchQuery = cf[len("bql:"):]
	case strings.HasPrefix(cf, "recipe:"):
		fd.RecipeName = cf[len("recipe:"):]
	case strings.HasPrefix(cf, "label:"):
		// Legacy label-in-filter: the label shows via LabelFilterText already,
		// and membership is otherwise unfiltered by status.
		fd.StatusFilter = "all"
	default:
		fd.StatusFilter = cf
	}

	// The "/" slot carries the fuzzy-search query when a Bubbles filter is
	// active and no BQL query already claimed the slot. Falls back to the search
	// mode label when there is no query text yet (so /semantic still surfaces).
	if fd.SearchQuery == "" && m.list.FilterState() != list.Unfiltered {
		if q := strings.TrimSpace(m.list.FilterValue()); q != "" {
			fd.SearchQuery = q
		} else if fd.SearchMode != "" {
			fd.SearchQuery = fd.SearchMode
		}
	}

	// Order bucket: explicit (non-default) sort only.
	fd.OrderLabel = lensSortLabel(m.filter.sortMode)
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
			return fmt.Sprintf("%s %s %d/%d", sel.Issue.ID, activeGlyphs.Sep, m.list.Index()+1, total)
		}
		return sel.Issue.ID
	}

	switch m.mode {
	case ViewGraph:
		return fmt.Sprintf("%s %s %s",
			countLabel(m.graphView.TotalCount(), "node"),
			activeGlyphs.Sep,
			countLabel(m.graphView.EdgeCount(), "edge"))
	case ViewBoard:
		return fmt.Sprintf("%s %s %s",
			countLabel(m.board.VisibleColumnCount(), "col"),
			activeGlyphs.Sep,
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
		return "LABELS: j/k nav • h detail • d drilldown • enter filter", activeGlyphs.Tag
	}
	if m.activeModal == ModalLabelGraphAnalysis && m.labelGraphAnalysisResult != nil {
		return fmt.Sprintf("GRAPH %s: esc/q/g close", m.labelGraphAnalysisResult.Label), activeGlyphs.Graph
	}
	if m.activeModal == ModalLabelDrilldown && m.labelDrilldownLabel != "" {
		return fmt.Sprintf("LABEL %s: enter filter • g graph • esc/q/d close", m.labelDrilldownLabel), activeGlyphs.Tag
	}
	switch m.filter.currentFilter {
	case "all":
		return "ALL", activeGlyphs.FilterAll
	case "open":
		return "OPEN", activeGlyphs.FilterOpen
	case "closed":
		return "CLOSED", activeGlyphs.FilterClosed
	case "ready":
		return "READY", activeGlyphs.FilterReady
	default:
		if strings.HasPrefix(m.filter.currentFilter, "bql:") {
			bqlStr := m.filter.currentFilter[4:]
			if len(bqlStr) > 30 {
				bqlStr = bqlStr[:27] + "..."
			}
			return "BQL: " + bqlStr, activeGlyphs.FilterBQL
		}
		if strings.HasPrefix(m.filter.currentFilter, "recipe:") {
			return strings.ToUpper(m.filter.currentFilter[7:]), activeGlyphs.FilterRecipe
		}
		return m.filter.currentFilter, activeGlyphs.Search
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

	// Freshness age drives the STALE / aging tiers below. Those tiers encode
	// server-mode semantics: the poll loop re-verifies freshness every few
	// seconds, so a growing age means polling has stalled. Embedded mode is
	// event-driven (manifest watch -> re-export) and a quiet project is
	// unchanged for long stretches BY DESIGN, so wall-age is not staleness there
	// - freshness instead means "watcher alive + last export succeeded", which
	// the worker-health and error tiers above already cover. Skip the age tiers
	// for embedded so a quiet project shows no false STALE warning (bt-t19xt).
	// Server- and global-mode behavior is unchanged.
	var freshnessAge time.Duration
	hasFreshnessAge := false
	if !m.isEmbeddedSource() {
		if !m.lastDoltVerified.IsZero() {
			freshnessAge = time.Since(m.lastDoltVerified)
			hasFreshnessAge = true
		} else if m.data.snapshot != nil && !m.data.snapshot.CreatedAt.IsZero() {
			freshnessAge = time.Since(m.data.snapshot.CreatedAt)
			hasFreshnessAge = true
		}
	}

	health := m.data.backgroundWorker.Health()
	lastErr := m.data.backgroundWorker.LastError()

	switch {
	case health.Started && !health.Alive:
		return activeGlyphs.Warning + " worker unresponsive", WorkerLevelCritical

	case m.workerSpinnerVisible():
		frame := workerSpinnerFrames[m.data.workerSpinnerIdx%len(workerSpinnerFrames)]
		return fmt.Sprintf("%s refreshing", frame), WorkerLevelInfo

	case lastErr != nil && lastErr.Retries >= freshnessErrorRetries:
		return fmt.Sprintf("%s bg %s (%dx)", activeGlyphs.Cross, lastErr.Phase, lastErr.Retries), WorkerLevelCritical

	case lastErr != nil:
		return fmt.Sprintf("%s bg %s (%s)", activeGlyphs.Warning, lastErr.Phase, formatAge(time.Since(lastErr.Time))), WorkerLevelWarning

	case hasFreshnessAge && freshnessAge >= freshnessStaleThreshold():
		return fmt.Sprintf("%s STALE: %s ago", activeGlyphs.Warning, formatAge(freshnessAge)), WorkerLevelCritical

	case hasFreshnessAge && freshnessAge >= freshnessWarnThreshold():
		return fmt.Sprintf("%s %s ago", activeGlyphs.Warning, formatAge(freshnessAge)), WorkerLevelWarning

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

// extractAlertCounts feeds the footer attention badge. total is every visible
// alert (the modal's own tally); critical/warning are the ANOMALY-typed subset
// only. Per-issue advisories (staleness, high-leverage) stay browsable in the
// modal but are excluded from the badge input regardless of their severity, so
// a normal backlog - which generates hundreds of "stale warning" advisories as
// its steady state - no longer floods a four-digit count into the footer at
// fleet scale (bt-jhzat). The badge lights only for genuine anomalies: new
// cycles (critical), baseline drift deltas, and abandoned P0/P1 claims (warning).
func (m *Model) extractAlertCounts() (total, critical, warning int) {
	for _, a := range m.visibleAlerts() {
		total++
		// isAttentionAnomaly == anomaly-typed AND critical/warning severity —
		// the same predicate the alerts-modal status header reconciles against
		// (bt-2nepr), so the badge and the header's "N anomalies" always agree.
		if !isAttentionAnomaly(a) {
			continue
		}
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

// viewKeyMap maps m.mode to the matching pkg/ui/keys map for the L1 footer
// slot, falling back to the Global map for views with no dedicated Map yet
// (Attention, LabelDashboard) so the L1 slot is never empty — a few global
// keys beat a blank footer.
func (m Model) viewKeyMap() help.KeyMap {
	if km := m.viewSpecificKeyMap(); km != nil {
		return km
	}
	return m.keys.Global
}

// viewSpecificKeyMap returns the per-view key.Map for m.mode, or nil for views
// with no dedicated Map yet (Attention, LabelDashboard). The L1 footer adds the
// Global fallback (viewKeyMap); the ; sidebar composes Global ++ this map
// (sidebarHelpGroups), so this must return nil rather than the Global map to
// avoid a doubled Global section in the sidebar.
//
// ViewList has a sub-state branch per ADR-004 Decision 7 — when filter typing is
// active, the truthful help is ListSearchKeys (apply / cancel / result-nav),
// otherwise ListNormalKeys (the full action set).
func (m Model) viewSpecificKeyMap() help.KeyMap {
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
	return nil
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
		if m.repoPicker.IsSearchFocused() {
			return m.keys.RepoPickerSearch
		}
		return m.keys.RepoPickerNav
	case ModalEpicCard:
		return m.keys.EpicCard
	case ModalFieldSelect:
		return m.keys.FieldSelect
	case ModalFieldPicker:
		return m.keys.FieldPicker
	case ModalFieldInput:
		return m.keys.FieldInput
	case ModalLongformEdit:
		return m.keys.LongformEdit
	}
	// Other modals (help, alerts, tutorial, quit-confirm, agent prompt, …)
	// carry their own internal footers; the L1 slot stays empty for them.
	return nil
}

// sidebarHelpGroups returns the FullHelp() binding groups the ; shortcuts
// sidebar renders for the current state (bt-ift6.10). When a modal owns the
// sidebar (ADR-004 Decision 4) it shows that modal's FullHelp() alone;
// otherwise it shows the active view's bindings only (bt-dx7k) - the Global
// map now lives on the ? overlay so the two surfaces carry complementary
// content. Reads from the same key.Map source as the L1 footer (ShortHelp)
// and ? overlay (FullHelp), so the three surfaces cannot drift.
func (m Model) sidebarHelpGroups() [][]key.Binding {
	if m.activeModal != ModalNone {
		if km := m.modalKeyMap(); km != nil {
			return km.FullHelp()
		}
		return nil
	}
	// View-only (bt-dx7k): the Global map now lives on the ? overlay; ; shows
	// just the active view's actions. nil view map -> empty (the sidebar renders
	// an empty-view fallback; see ShortcutsSidebar.View).
	if km := m.viewSpecificKeyMap(); km != nil {
		return km.FullHelp()
	}
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

	// Zone 1 (lens) replaces the old filter / project / search / sort / wisp /
	// label left-zone badges (bt-2vshd). renderLens(fd, lensLvl) draws the
	// scope -> filter -> order sentence; lensLvl and the right-zone hint density
	// are advanced by the degradation cascade further down. The killed left
	// badges (and the [19] workspace badge) fold into the lens's scope + chips.

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
			seg(lipgloss.NewStyle().Foreground(ColorStatusOpen), activeGlyphs.StatOpen, fd.CountOpen),
			seg(lipgloss.NewStyle().Foreground(ColorSuccess), activeGlyphs.StatReady, fd.CountReady),
			seg(lipgloss.NewStyle().Foreground(ColorWarning), activeGlyphs.StatBlocked, fd.CountBlocked),
			seg(lipgloss.NewStyle().Foreground(ColorMuted), activeGlyphs.StatClosed, fd.CountClosed),
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
		phase2Section = phase2Style.Render(activeGlyphs.Phase2Dot + " metrics…")
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
		updateSection = updateStyle.Render(fmt.Sprintf("%s %s", activeGlyphs.Star, fd.UpdateTag))
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
		instanceSection = instanceStyle.Render(fmt.Sprintf("%s PID %d", activeGlyphs.Warning, fd.SecondaryPID))
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
		sessionSection = sessionStyle.Render(fmt.Sprintf("%s%s", activeGlyphs.Session, countStr))
	}

	// Workspace summary, label filter, and repo filter fold into the lens
	// (bt-2vshd): scope shows ALL(N) / the active repo subset, and the lb: chip
	// carries the label filter. The old standalone badges (incl. the [19]
	// workspace badge) are gone.

	// Zone 3 (right) is the static "? help · ; keys" pair + anomaly badge + bell
	// (bt-2vshd). The per-view action pills — and their pill degradation
	// machinery — are retired from the footer chrome; the per-view key.Maps now
	// feed only the ? overlay and the ; sidebar. renderStaticHints builds the
	// pair; the anomaly (alertsSection) and bell render to its right.

	countStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Padding(0, 1)
	countBadge := countStyle.Render(fmt.Sprintf("%d issues", fd.TotalItems))
	countBadgeShort := countStyle.Render(fmt.Sprintf("%d", fd.TotalItems))
	if hasCenterOverride {
		// The override carries its own count semantics (position/edges/cards);
		// the global "N issues" badge would be redundant or wrong here.
		countBadge = ""
		countBadgeShort = ""
	}

	// Daemon chrome is the only remaining droppable badge group now that the
	// scope / mode / label chips live in the lens (bt-2vshd). It stays tier 3 —
	// bt's own telemetry, dropped first under width pressure. The large-dataset
	// badge was retired outright (bt-2nepr / bt-ajbxw reframe): corpus size is a
	// status fact in the alerts-modal status header, never a footer warning.
	type footerBadge struct {
		content string
		tier    int
	}
	optional := map[string]*footerBadge{
		"instanceSection": {instanceSection, 3},
		"workerSection":   {workerSection, 3},
		"watcherSection":  {watcherSection, 3},
		"sessionSection":  {sessionSection, 3},
		"updateSection":   {updateSection, 3},
		"phase2Section":   {phase2Section, 3},
	}
	optionalWidth := func() int {
		w := 0
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

	// Bell badge (Phase 4): always rendered; the count appears only when > 0.
	// Pinned (last to drop) alongside the ? ; pair. Built BEFORE the cascade so
	// its width can be reserved — otherwise the toast consumes the whole right
	// zone and the final ansi.Truncate clips the bell (bt-a3zi3.1).
	bellText := activeGlyphs.Bell
	if fd.BellCount > 0 {
		bellText = fmt.Sprintf("%s%d", activeGlyphs.Bell, fd.BellCount)
	}
	bellStyle := lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)
	if fd.BellCount > 0 {
		bellStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Padding(0, 1)
	}
	bellSection := bellStyle.Render(bellText)
	bellWidth := lipgloss.Width(bellSection)

	// Degradation cascade (bt-2vshd drop order): lens placeholders -> daemon /
	// degraded badges -> triad segments (total survives) -> hint labels -> lens
	// filter words (scope survives) -> last resort scope · total · ? ; · !N. The
	// engine shape is unchanged: reduce in order until the content fits; a final
	// ansi truncate makes wrapping impossible. lensLvl and hintsCompact are the
	// lens/right-zone density knobs the cascade advances.
	lensLvl := lensFull
	hintsCompact := false

	rightWidth := func() int {
		w := lipgloss.Width(renderStaticHints(hintsCompact)) + bellWidth
		if alertsSection != "" {
			w += lipgloss.Width(alertsSection)
		}
		return w
	}
	contentWidth := func() int {
		return lipgloss.Width(renderLens(fd, lensLvl)) +
			lipgloss.Width(statsSection) + lipgloss.Width(countBadge) +
			optionalWidth() + rightWidth()
	}

	reductions := []func(){
		func() { lensLvl = lensNoPlace }, // 1. lens placeholders (lb:- , /-)
		func() { dropTier(3) },           // 2. daemon / degraded badges
		func() { // 3a. triad: skip zero-count segments (total survives via countBadge)
			if !hasCenterOverride {
				statsSection = buildStats(true)
			}
		},
		func() { // 3b. triad drops whole
			if !hasCenterOverride {
				statsSection = ""
			}
		},
		func() { hintsCompact = true },      // 4. hint labels: "? help · ; keys" -> "? ;"
		func() { lensLvl = lensStatusOnly }, // 5. lens filter words drop; status + scope remain
		func() { countBadge = countBadgeShort }, // 6. "4921 issues" -> "4921"
		func() { lensLvl = lensScopeOnly },      // 7. scope survives alone
		func() { // 8. last resort: a selection center-override yields
			if hasCenterOverride {
				statsSection = ""
			}
		},
	}
	for _, reduce := range reductions {
		if contentWidth() <= fd.Width {
			break
		}
		reduce()
	}

	lensSection := renderLens(fd, lensLvl)
	hintsSection := renderStaticHints(hintsCompact)

	// nonHint sums everything except the ?/; slot, which an inline toast borrows
	// (Phase 4 semantics unchanged; bt-2vshd does not touch toast plumbing). The
	// anomaly badge and bell sit to the right of that slot and are reserved here
	// so the toast can never squeeze them out.
	nonHint := lipgloss.Width(lensSection) + lipgloss.Width(statsSection) +
		lipgloss.Width(countBadge) + optionalWidth() +
		lipgloss.Width(alertsSection) + bellWidth

	rightZone := hintsSection
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
		avail := fd.Width - nonHint
		toast := toastStyle.Render(text)
		if avail > 0 && lipgloss.Width(toast) > avail {
			toast = ansi.Truncate(toast, avail, "")
		}
		rightZone = toast
	}

	// Filler pushes the count + right zone to the right edge.
	remaining := fd.Width - nonHint - lipgloss.Width(rightZone)
	if remaining < 0 {
		remaining = 0
	}
	filler := lipgloss.NewStyle().Width(remaining).Render("")

	// Build the footer in display order (content may be empty after compression):
	// lens (Zone 1) · center + daemon chrome · filler · total · ?/; hints (Zone 3)
	// · anomaly · bell.
	var parts []string
	addIf := func(s string) {
		if s != "" {
			parts = append(parts, s)
		}
	}
	addIf(lensSection)
	addIf(statsSection)
	addIf(optional["phase2Section"].content)
	addIf(optional["watcherSection"].content)
	addIf(optional["workerSection"].content)
	addIf(optional["instanceSection"].content)
	addIf(optional["sessionSection"].content)
	addIf(optional["updateSection"].content)
	parts = append(parts, filler)
	addIf(countBadge)
	addIf(rightZone)
	addIf(alertsSection)
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

// renderAlertsBadge renders the drift-alert badge under dark-cockpit discipline
// (bt-ujwiq / decision bt-9gjt0): the footer lights up only when something is
// attention-worthy - a critical or warning drift. Info-level drift stays
// browsable in the alerts modal but never camps the footer, so the badge no
// longer sits there as a permanent total (the "51 (!)" the dogfood pass flagged).
// The count shown is the attention-worthy subset (critical + warning), not the
// AlertCount total (which still tallies info toward the modal's own count).
// bt-jhzat narrows that input further upstream in extractAlertCounts to
// ANOMALY-typed alerts, so per-issue advisories (staleness, high-leverage)
// never light the badge even at warning severity - they stay in the modal.
func (fd FooterData) renderAlertsBadge() string {
	attention := fd.CriticalCount + fd.WarningCount
	if attention == 0 {
		return ""
	}
	// Zone 3 sigil form (bt-2vshd): plain "<glyph>N", no background fill, no
	// "(!)" suffix — critical in red (Warning glyph), warning in yellow (Bolt).
	// Color carries the severity; the glyph + count is the traceable badge that
	// opens the alerts modal filtered to exactly these N anomalies.
	var alertStyle lipgloss.Style
	var alertIcon string
	if fd.CriticalCount > 0 {
		alertStyle = lipgloss.NewStyle().Foreground(ColorPrioCritical).Bold(true).Padding(0, 1)
		alertIcon = activeGlyphs.Warning
	} else {
		alertStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Padding(0, 1)
		alertIcon = activeGlyphs.Bolt
	}
	return alertStyle.Render(fmt.Sprintf("%s%d", alertIcon, attention))
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
