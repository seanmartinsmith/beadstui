package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

func TestStatusSeverityGlyph(t *testing.T) {
	cases := map[StatusSeverity]string{
		SeveritySuccess:  activeGlyphs.Success,
		SeverityNotice:   "",
		SeverityFailure:  activeGlyphs.Cross,
		SeverityDegraded: activeGlyphs.Warning,
	}
	for sev, want := range cases {
		if got := sev.glyph(); got != want {
			t.Errorf("severity %d glyph = %q, want %q", sev, got, want)
		}
	}
}

func TestFooterData_StatusBarOverride(t *testing.T) {
	fd := FooterData{
		Width:          80,
		StatusMsg:      "Copied bt-abc1 to clipboard",
		StatusSeverity: SeveritySuccess,
		FilterText:     "OPEN",
		FilterIcon:     "📂",
		TotalItems:     42,
	}
	out := fd.Render()
	if !strings.Contains(out, "Copied bt-abc1 to clipboard") {
		t.Errorf("status message should appear in output")
	}
	// When status is active, filter badge should NOT appear
	if strings.Contains(out, "OPEN") {
		t.Errorf("filter badge should not appear when status message is active")
	}
}

func TestFooterData_ErrorStatusBar(t *testing.T) {
	fd := FooterData{
		Width:          80,
		StatusMsg:      "No issue selected",
		StatusSeverity: SeverityFailure,
		TotalItems:     10,
	}
	out := fd.Render()
	if !strings.Contains(out, "No issue selected") {
		t.Errorf("error status message should appear in output")
	}
}

func ansiStripForTest(s string) string { return ansi.Strip(s) }

// TestFooterExcludesToastContent proves the statusline-embedded toast (Phase
// 4 of bt-a3zi3.1) is gone: an active inline status message must NOT appear
// in the footer's right zone, and the key hints must render normally rather
// than yielding to it (the old "Toast override" borrow-and-truncate behavior
// that caused bt-8scek). Notification content now lives in the floating
// bubble overlay (bt-kuvzj, toast_bubble.go) — the footer keeps only the
// bell.
func TestFooterExcludesToastContent(t *testing.T) {
	fd := FooterData{
		Width:          100,
		StatusMsg:      "write failed: db locked",
		StatusSeverity: SeverityFailure,
		StatusIsInline: true,
		FilterText:     "ALL",
		FilterIcon:     "🌐",
		TotalItems:     42,
		Hints:          []FooterHint{{Key: "⏎", Desc: "open"}, {Key: "?", Desc: "help"}},
	}
	out := ansiStripForTest(fd.Render())
	if strings.Contains(out, "write failed: db locked") {
		t.Errorf("footer must not render toast content; got %q", out)
	}
	if !strings.Contains(out, "open") {
		t.Errorf("key hints should render normally (no longer yield to a toast); got %q", out)
	}
}

func TestFooterBellBadge(t *testing.T) {
	withN := FooterData{Width: 100, FilterText: "ALL", FilterIcon: "🌐", BellCount: 3,
		Hints: []FooterHint{{Key: "?", Desc: "help"}}}
	out := ansiStripForTest(withN.Render())
	if !strings.Contains(out, activeGlyphs.Bell+"3") {
		t.Errorf("bell should show 🔔3; got %q", out)
	}
	zero := withN
	zero.BellCount = 0
	out0 := ansiStripForTest(zero.Render())
	if !strings.Contains(out0, activeGlyphs.Bell) {
		t.Errorf("bell glyph should always render; got %q", out0)
	}
	if strings.Contains(out0, activeGlyphs.Bell+"0") {
		t.Errorf("zero count should render bare 🔔, not 🔔0; got %q", out0)
	}
}

func TestFooterData_NormalFooter(t *testing.T) {
	fd := FooterData{
		Width:        120,
		FilterText:   "OPEN",
		FilterIcon:   "📂",
		HintText:     "l:labels",
		CountOpen:    10,
		CountReady:   5,
		CountBlocked: 2,
		CountClosed:  3,
		TotalItems:   20,
		Hints:        []FooterHint{{Key: "⏎", Desc: "details"}, {Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "OPEN") {
		t.Errorf("filter badge text should appear")
	}
	if !strings.Contains(out, "20 issues") {
		t.Errorf("issue count should appear")
	}
}

func TestFooterData_WorkspaceBadges(t *testing.T) {
	fd := FooterData{
		Width:            120,
		FilterText:       "ALL",
		FilterIcon:       "📋",
		HintText:         "l:labels",
		WorkspaceMode:    true,
		WorkspaceSummary: "3 repos",
		RepoFilterLabel:  "bt, beads",
		TotalItems:       100,
		Hints:            []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "3 repos") {
		t.Errorf("workspace summary should appear")
	}
	if !strings.Contains(out, "bt, beads") {
		t.Errorf("repo filter label should appear")
	}
}

func TestFooterData_WorkerBadgeLevels(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		level WorkerLevel
		want  bool // should produce non-empty output
	}{
		{"none", "", WorkerLevelNone, false},
		{"info", "⠋ refreshing", WorkerLevelInfo, true},
		{"warning", "⚠ bg poll (5s)", WorkerLevelWarning, true},
		{"critical", "⚠ worker unresponsive", WorkerLevelCritical, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := FooterData{WorkerText: tt.text, WorkerLevel: tt.level}
			out := fd.renderWorkerBadge()
			if tt.want && out == "" {
				t.Errorf("expected non-empty worker badge for level %d", tt.level)
			}
			if !tt.want && out != "" {
				t.Errorf("expected empty worker badge for level %d", tt.level)
			}
		})
	}
}

// newBadgeTestModel builds a minimal Model wired with a background worker and a
// data source of the given type, for exercising extractWorkerBadge. The worker
// is intentionally NOT started, so Health.Started is false and the
// "worker unresponsive" tier stays off.
func newBadgeTestModel(t *testing.T, srcType datasource.SourceType) (Model, *BackgroundWorker) {
	t.Helper()
	w, err := NewBackgroundWorker(WorkerConfig{BeadsPath: ""})
	if err != nil {
		t.Fatalf("NewBackgroundWorker: %v", err)
	}
	t.Cleanup(w.Stop)
	m := Model{data: &DataState{
		backgroundWorker: w,
		dataSource:       &datasource.DataSource{Type: srcType},
	}}
	return m, w
}

// TestExtractWorkerBadge_EmbeddedQuietNoStale: a quiet embedded project must not
// escalate to a staleness warning even though the snapshot is old, because
// embedded refresh is event-driven, not poll-verified (bt-t19xt).
func TestExtractWorkerBadge_EmbeddedQuietNoStale(t *testing.T) {
	m, _ := newBadgeTestModel(t, datasource.SourceTypeEmbeddedDolt)
	// On a server source this age would read STALE; embedded must ignore it.
	m.data.snapshot = &DataSnapshot{CreatedAt: time.Now().Add(-10 * time.Minute)}

	text, level := m.extractWorkerBadge()
	if text != "" || level != WorkerLevelNone {
		t.Fatalf("quiet embedded project must show no staleness badge, got %q level=%d", text, level)
	}
}

// TestExtractWorkerBadge_ServerQuietGoesStale is the control: server/global mode
// freshness semantics are unchanged, so a quiet server project still reads STALE.
func TestExtractWorkerBadge_ServerQuietGoesStale(t *testing.T) {
	m, _ := newBadgeTestModel(t, datasource.SourceTypeDolt)
	m.data.snapshot = &DataSnapshot{CreatedAt: time.Now().Add(-10 * time.Minute)}

	text, level := m.extractWorkerBadge()
	if level != WorkerLevelCritical || !strings.Contains(text, "STALE") {
		t.Fatalf("quiet server project must still show STALE, got %q level=%d", text, level)
	}
}

// TestExtractWorkerBadge_EmbeddedExportFailureWarns: suppressing the age tiers
// for embedded must NOT go silent on real failures - a failed re-export still
// surfaces a warning (bt-t19xt acceptance).
func TestExtractWorkerBadge_EmbeddedExportFailureWarns(t *testing.T) {
	m, w := newBadgeTestModel(t, datasource.SourceTypeEmbeddedDolt)
	m.data.snapshot = &DataSnapshot{CreatedAt: time.Now().Add(-10 * time.Minute)}
	w.recordError(&WorkerError{Phase: "load", Cause: fmt.Errorf("bd export failed"), Time: time.Now()})

	text, level := m.extractWorkerBadge()
	if level != WorkerLevelWarning || !strings.Contains(text, "bg load") {
		t.Fatalf("embedded export failure must still surface a warning, got %q level=%d", text, level)
	}
}

// TestExtractWorkerBadge_EmbeddedPersistentFailureCritical: repeated export
// failures escalate to critical for embedded, same as any source.
func TestExtractWorkerBadge_EmbeddedPersistentFailureCritical(t *testing.T) {
	m, w := newBadgeTestModel(t, datasource.SourceTypeEmbeddedDolt)
	for i := 0; i < freshnessErrorRetries; i++ {
		w.recordError(&WorkerError{Phase: "load", Cause: fmt.Errorf("boom"), Time: time.Now()})
	}
	text, level := m.extractWorkerBadge()
	if level != WorkerLevelCritical || !strings.Contains(text, "bg load") {
		t.Fatalf("persistent embedded export failure must be critical, got %q level=%d", text, level)
	}
}

// forceWorkerProcessing puts the worker into the processing state with the given
// elapsed duration, so the spinner-coalescing tick can be exercised without
// spawning a real load.
func forceWorkerProcessing(w *BackgroundWorker, elapsed time.Duration) {
	w.mu.Lock()
	w.state = WorkerProcessing
	w.processingStart = time.Now().Add(-elapsed)
	w.mu.Unlock()
}

func forceWorkerIdle(w *BackgroundWorker) {
	w.mu.Lock()
	w.state = WorkerIdle
	w.processingStart = time.Time{}
	w.mu.Unlock()
}

// TestWorkerSpinner_CoalescesAcrossRapidCycles proves the presentation-layer
// coalescing (bt-uq3i3): sub-threshold refreshes never flash the spinner, and a
// burst of processing cycles separated by brief idle gaps renders one steady
// "refreshing" indicator rather than a sub-second on/off loop.
func TestWorkerSpinner_CoalescesAcrossRapidCycles(t *testing.T) {
	m, w := newBadgeTestModel(t, datasource.SourceTypeEmbeddedDolt)

	// Sub-threshold processing must NOT open the display window.
	forceWorkerProcessing(w, 100*time.Millisecond)
	m, _ = m.handleWorkerPollTick()
	if m.workerSpinnerVisible() {
		t.Fatal("sub-threshold processing should not show the spinner (no flash)")
	}

	// Processing past the flash threshold opens the window and shows the spinner.
	forceWorkerProcessing(w, workerSpinnerFlashThreshold+50*time.Millisecond)
	m, _ = m.handleWorkerPollTick()
	if !m.workerSpinnerVisible() {
		t.Fatal("processing past the flash threshold should show the spinner")
	}
	if text, level := m.extractWorkerBadge(); level != WorkerLevelInfo || !strings.Contains(text, "refreshing") {
		t.Fatalf("expected refreshing badge while processing, got %q level=%d", text, level)
	}

	// The worker returns to idle between cycles: the spinner must stay up
	// (coalesced) rather than blink off, because the min-display window persists.
	forceWorkerIdle(w)
	m, _ = m.handleWorkerPollTick()
	if !m.workerSpinnerVisible() {
		t.Fatal("spinner must stay up across an idle gap within the min-display window")
	}
	if text, level := m.extractWorkerBadge(); level != WorkerLevelInfo || !strings.Contains(text, "refreshing") {
		t.Fatalf("coalesced spinner should still render while idle within the window, got %q level=%d", text, level)
	}

	// Once the window elapses, the spinner clears.
	m.data.workerSpinnerVisibleUntil = time.Now().Add(-time.Millisecond)
	if m.workerSpinnerVisible() {
		t.Fatal("spinner must clear after the min-display window elapses")
	}
	if text, _ := m.extractWorkerBadge(); strings.Contains(text, "refreshing") {
		t.Fatalf("spinner should be gone after the window elapses, got %q", text)
	}
}

// TestWorkerPollTick_DormantAndRearm verifies the 120ms tick chain stops
// rescheduling once the worker is idle and the coalesced display window has
// elapsed (bt-2ubez: a perpetual idle tick cost ~6.6%/core at 1300 issues),
// and that RefreshStartedMsg re-arms exactly one chain.
func TestWorkerPollTick_DormantAndRearm(t *testing.T) {
	m, w := newBadgeTestModel(t, datasource.SourceTypeEmbeddedDolt)

	// Idle with no display window: the chain must go dormant (no reschedule).
	forceWorkerIdle(w)
	m.data.workerSpinnerVisibleUntil = time.Time{}
	m, cmd := m.handleWorkerPollTick()
	if cmd != nil {
		t.Fatal("idle worker outside the display window must not reschedule the tick chain")
	}
	if m.data.workerTickArmed {
		t.Fatal("tick chain should be disarmed when dormant")
	}

	// Idle but within the display window: keep ticking (coalesced spinner).
	m.data.workerSpinnerVisibleUntil = time.Now().Add(500 * time.Millisecond)
	m, cmd = m.handleWorkerPollTick()
	if cmd == nil {
		t.Fatal("idle worker inside the display window must keep the tick chain alive")
	}
	if !m.data.workerTickArmed {
		t.Fatal("tick chain should be armed while the display window lingers")
	}

	// Processing: keep ticking.
	forceWorkerProcessing(w, workerSpinnerFlashThreshold+50*time.Millisecond)
	m.data.workerSpinnerVisibleUntil = time.Time{}
	m, cmd = m.handleWorkerPollTick()
	if cmd == nil {
		t.Fatal("processing worker must keep the tick chain alive")
	}

	// Dormant again, then RefreshStartedMsg re-arms exactly one chain.
	forceWorkerIdle(w)
	m.data.workerSpinnerVisibleUntil = time.Time{}
	m, _ = m.handleWorkerPollTick()
	if m.data.workerTickArmed {
		t.Fatal("setup: expected dormant chain")
	}
	m, cmd = m.handleRefreshStarted(RefreshStartedMsg{})
	if cmd == nil {
		t.Fatal("RefreshStartedMsg must return commands (re-subscribe + tick)")
	}
	if !m.data.workerTickArmed {
		t.Fatal("RefreshStartedMsg must re-arm the tick chain")
	}
	// A second RefreshStartedMsg while armed must not double-arm: it still
	// re-subscribes to the worker channel but the armed flag stays set.
	m, cmd = m.handleRefreshStarted(RefreshStartedMsg{})
	if cmd == nil {
		t.Fatal("RefreshStartedMsg must always re-subscribe to the worker channel")
	}
	if !m.data.workerTickArmed {
		t.Fatal("armed flag must remain set after redundant RefreshStartedMsg")
	}
}

func TestFooterData_AlertsBadge(t *testing.T) {
	fd := FooterData{AlertCount: 3, CriticalCount: 1, WarningCount: 2}
	out := fd.renderAlertsBadge()
	if !strings.Contains(out, "3 (!)") {
		t.Errorf("alert count and indicator should appear: %s", out)
	}
}

func TestFooterData_NoAlerts(t *testing.T) {
	fd := FooterData{AlertCount: 0}
	out := fd.renderAlertsBadge()
	if out != "" {
		t.Errorf("no alerts should produce empty badge")
	}
}

// TestFooterAlertsBadge_DarkCockpit locks the bt-ujwiq / bt-9gjt0 dark-cockpit
// gate: the alerts badge lights up only for attention-worthy drift (critical or
// warning) and never camps a total that is all info-level (the "51 (!)" the
// dogfood pass flagged). The displayed count is the attention-worthy subset.
func TestFooterAlertsBadge_DarkCockpit(t *testing.T) {
	// 51 info-level alerts, none attention-worthy: the badge stays dark.
	infoOnly := FooterData{AlertCount: 51, CriticalCount: 0, WarningCount: 0}
	if out := infoOnly.renderAlertsBadge(); out != "" {
		t.Errorf("info-only alerts must not light the footer; got %q", out)
	}
	// 51 total, 3 attention-worthy: the badge shows the attention subset (3),
	// not the camping total (51).
	mixed := FooterData{AlertCount: 51, CriticalCount: 1, WarningCount: 2}
	out := ansi.Strip(mixed.renderAlertsBadge())
	if !strings.Contains(out, "3 (!)") {
		t.Errorf("mixed alerts should surface the attention-worthy count (3); got %q", out)
	}
	if strings.Contains(out, "51") {
		t.Errorf("the info-inclusive total must not camp in the badge; got %q", out)
	}
}

// TestExtractAlertCounts_AdvisoriesExcludedFromBadge locks bt-jhzat's
// badge-input composition at fleet scale: extractAlertCounts (the step that
// turns m.alerts into the badge's Critical/Warning input) must exclude
// per-issue advisories (staleness, high-leverage) regardless of their severity,
// while still counting genuine anomalies (cycles, abandoned claims). Advisory
// volume stays in m.alerts (browsable in the modal via visibleAlerts) but never
// feeds the footer badge - so a fleet-scale backlog of stale warnings boots to
// a quiet strip, and a single real anomaly lights it with an honest count.
func TestExtractAlertCounts_AdvisoriesExcludedFromBadge(t *testing.T) {
	// 1000 stale advisories at WARNING severity, zero anomalies.
	stale := make([]drift.Alert, 0, 1001)
	for i := 0; i < 1000; i++ {
		stale = append(stale, drift.Alert{
			Type:     drift.AlertStale,
			Severity: drift.SeverityWarning,
			IssueID:  fmt.Sprintf("bt-%d", i),
		})
	}

	m := Model{alerts: stale}
	total, critical, warning := m.extractAlertCounts()
	if total != 1000 {
		t.Fatalf("advisories must still be visible/countable in the modal total; got total=%d", total)
	}
	if critical+warning != 0 {
		t.Fatalf("1000 stale warnings must not feed the badge; got critical=%d warning=%d", critical, warning)
	}
	// With a zero anomaly count the badge is dark.
	if out := (FooterData{AlertCount: total, CriticalCount: critical, WarningCount: warning}).renderAlertsBadge(); out != "" {
		t.Fatalf("advisory-only fleet must produce a dark badge; got %q", out)
	}

	// Now add exactly one genuine anomaly (a new cycle) to the same 1000.
	withCycle := append(append([]drift.Alert(nil), stale...), drift.Alert{
		Type:     drift.AlertDependencyLoop,
		Severity: drift.SeverityCritical,
		Message:  "1 new cycle(s) detected",
	})
	m = Model{alerts: withCycle}
	total, critical, warning = m.extractAlertCounts()
	if critical != 1 || warning != 0 {
		t.Fatalf("one cycle among 1000 advisories must yield critical=1 warning=0; got critical=%d warning=%d", critical, warning)
	}
	out := ansi.Strip((FooterData{AlertCount: total, CriticalCount: critical, WarningCount: warning}).renderAlertsBadge())
	if !strings.Contains(out, "1 (!)") {
		t.Fatalf("a genuine anomaly must light the badge with an honest count of 1; got %q", out)
	}

	// High-leverage is also advisory; an abandoned P0/P1 claim is an anomaly.
	// A fleet of high-leverage warnings plus one abandoned-claim warning must
	// badge exactly 1.
	mixed := make([]drift.Alert, 0, 501)
	for i := 0; i < 500; i++ {
		mixed = append(mixed, drift.Alert{
			Type:     drift.AlertHighLeverage,
			Severity: drift.SeverityWarning,
			IssueID:  fmt.Sprintf("bt-hl-%d", i),
		})
	}
	mixed = append(mixed, drift.Alert{
		Type:     drift.AlertAbandonedClaim,
		Severity: drift.SeverityWarning,
		IssueID:  "bt-abandoned",
	})
	m = Model{alerts: mixed}
	_, critical, warning = m.extractAlertCounts()
	if critical != 0 || warning != 1 {
		t.Fatalf("500 high-leverage advisories + 1 abandoned claim must yield warning=1; got critical=%d warning=%d", critical, warning)
	}
}

// TestFooterRetiering_SelectionSurvivesDaemonChromeDrops locks the bt-ujwiq /
// decision bt-9gjt0 content re-tiering: under width pressure the footer protects
// the user's work (selection state carried by the center override) and drops bt's
// own daemon chrome (secondary-instance, background-worker, watcher, session,
// self-update badges) FIRST. Anchored at the dogfood widths the bead names. The
// fixture is deliberately chrome-heavy so real width pressure exists even at 100.
func TestFooterRetiering_SelectionSurvivesDaemonChromeDrops(t *testing.T) {
	base := FooterData{
		FilterText:     "bt",
		FilterIcon:     "📂",
		HintText:       "l:labels",
		TotalItems:     169,
		CenterOverride: "bt-0qzp · 3/169", // the user's work: which bead + position
		WorkerText:     "⚠ worker unresponsive",
		WorkerLevel:    WorkerLevelCritical,
		SecondaryPID:   48213,
		WatcherText:    "polling nfs 5s",
		SessionCount:   3,
		UpdateTag:      "v0.2.0",
		Hints: []FooterHint{
			{Key: "esc", Desc: "back"}, {Key: "C", Desc: "copy"}, {Key: "?", Desc: "help"},
		},
	}
	for _, w := range []int{60, 80, 100} {
		fd := base
		fd.Width = w
		out := ansi.Strip(fd.Render())
		if !strings.Contains(out, "bt-0qzp") {
			t.Errorf("width=%d: selection state must survive degradation; got %q", w, out)
		}
		if strings.Contains(out, "48213") {
			t.Errorf("width=%d: secondary-instance chrome must drop first; got %q", w, out)
		}
		if strings.Contains(out, "unresponsive") {
			t.Errorf("width=%d: worker chrome must drop first; got %q", w, out)
		}
		if strings.Contains(out, "v0.2.0") {
			t.Errorf("width=%d: self-update chrome must drop first; got %q", w, out)
		}
	}
}

// TestFooterRetiering_HealthyStateIsQuiet proves the dark-cockpit steady state:
// with only info-level drift and no daemon conditions, the footer shows scope +
// position + hints and carries NO persistent daemon or alert badge (bt-ujwiq).
func TestFooterRetiering_HealthyStateIsQuiet(t *testing.T) {
	fd := FooterData{
		Width:         100,
		FilterText:    "bt",
		FilterIcon:    "📂",
		HintText:      "l:labels",
		TotalItems:    169,
		AlertCount:    44, // all info-level: browsable in the modal, never camps
		CriticalCount: 0,
		WarningCount:  0,
		Hints:         []FooterHint{{Key: "⏎", Desc: "open"}, {Key: "?", Desc: "help"}},
	}
	out := ansi.Strip(fd.Render())
	if strings.Contains(out, "(!)") || strings.Contains(out, "44") {
		t.Errorf("healthy state must not camp an alert count; got %q", out)
	}
	if !strings.Contains(out, "169 issues") {
		t.Errorf("healthy state should show the scoped position/total; got %q", out)
	}
}

func TestFooterData_TimeTravelOverridesStats(t *testing.T) {
	fd := FooterData{
		Width:            120,
		FilterText:       "OPEN",
		FilterIcon:       "📂",
		HintText:         "l:labels",
		TimeTravelActive: true,
		TimeTravelStats:  "⏱ 3d: +5 ✅2 ~3",
		TotalItems:       50,
		Hints:            []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "⏱ 3d") {
		t.Errorf("time travel stats should appear")
	}
}

func TestFooterData_SearchBadge(t *testing.T) {
	fd := FooterData{
		Width:      120,
		FilterText: "ALL",
		FilterIcon: "📋",
		HintText:   "l:labels",
		SearchMode: "semantic",
		TotalItems: 30,
		Hints:      []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "semantic") {
		t.Errorf("search mode should appear in output")
	}
}

func TestFooterData_SortBadge(t *testing.T) {
	fd := FooterData{
		Width:      120,
		FilterText: "ALL",
		FilterIcon: "📋",
		HintText:   "l:labels",
		SortLabel:  "priority",
		TotalItems: 30,
		Hints:      []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "priority") {
		t.Errorf("sort label should appear in output")
	}
}

func TestFooterData_ProgressiveHintTruncation(t *testing.T) {
	// Provide many hints in a narrow terminal — should truncate to fit
	fd := FooterData{
		Width:      40, // very narrow
		FilterText: "ALL",
		FilterIcon: "📋",
		HintText:   "l:labels",
		TotalItems: 10,
		Hints: []FooterHint{
			{Key: "⏎", Desc: "details"},
			{Key: "t", Desc: "diff"},
			{Key: "S", Desc: "triage"},
			{Key: "l", Desc: "labels"},
			{Key: "?", Desc: "help"},
		},
	}
	out := fd.Render()
	// Just verify it renders without panic and produces output
	if lipgloss.Width(out) == 0 {
		t.Errorf("footer should produce non-empty output even when narrow")
	}
}

// TestFooterData_NeverWrapsAcrossWidths is the core guarantee of the smart
// footer: at every terminal width the rendered footer occupies exactly one row
// and never exceeds the column count (which the terminal would wrap, stealing a
// content line). Mirrors the real cross-project pain — 4921 issues, large stat
// numbers, an active alerts badge — that the 7-issue render harness understates.
func TestFooterData_NeverWrapsAcrossWidths(t *testing.T) {
	base := FooterData{
		FilterText:    "ALL",
		FilterIcon:    "📋",
		HintText:      "l:labels",
		CountOpen:     1811,
		CountReady:    1684,
		CountBlocked:  0,
		CountClosed:   3110,
		TotalItems:    4921,
		AlertCount:    1410,
		WarningCount:  1410,
		CriticalCount: 0,
		Hints: []FooterHint{
			{Key: "⏎", Desc: "open detail"},
			{Key: "o", Desc: "open issues"},
			{Key: "c", Desc: "copy"},
			{Key: "t", Desc: "diff"},
			{Key: "?", Desc: "help"},
		},
	}
	for w := 24; w <= 220; w++ {
		fd := base
		fd.Width = w
		out := fd.Render()
		if strings.Contains(out, "\n") {
			t.Fatalf("width=%d: footer contains a newline (wrapped to 2 rows): %q", w, out)
		}
		if got := ansi.StringWidth(out); got > w {
			t.Fatalf("width=%d: footer display width %d exceeds terminal width (would wrap): %q",
				w, got, ansi.Strip(out))
		}
	}
}

// TestFooterData_NeverWrapsPathologicalBadge ensures a single oversized badge
// (e.g. a long BQL filter string) can't defeat the one-row guarantee — the
// final ANSI-aware truncate is the backstop.
func TestFooterData_NeverWrapsPathologicalBadge(t *testing.T) {
	fd := FooterData{
		Width:      50,
		FilterText: "BQL: status=open AND label=area:tui AND priority<=2 AND assignee=sms",
		FilterIcon: "🔍",
		HintText:   "l:labels",
		TotalItems: 4921,
		Hints:      []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if strings.Contains(out, "\n") {
		t.Fatalf("pathological filter wrapped the footer: %q", out)
	}
	if got := ansi.StringWidth(out); got > fd.Width {
		t.Fatalf("pathological filter overran width: %d > %d: %q", got, fd.Width, ansi.Strip(out))
	}
}

// TestFooterCounts_ScopedToActiveFilter proves the footer's status breakdown
// reflects exactly what the list shows (active scope + filter), not the global
// corpus — the bt-gcuv generalization. The breakdown is computed at the
// setListItems chokepoint, so applying a filter must reshape it.
func TestFooterCounts_ScopedToActiveFilter(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil, nil)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = nm.(Model)

	// All: open + closed accounts for every visible item.
	m.filter.currentFilter = "all"
	m.applyFilter()
	if got := m.ac.countOpen + m.ac.countClosed; got != len(m.list.Items()) {
		t.Fatalf("all filter: open+closed=%d but %d items visible", got, len(m.list.Items()))
	}

	// Open: the scoped breakdown must contain zero closed.
	m.filter.currentFilter = "open"
	m.applyFilter()
	if m.ac.countClosed != 0 {
		t.Errorf("open filter: scoped breakdown should show 0 closed, got %d", m.ac.countClosed)
	}
	if m.ac.countOpen != len(m.list.Items()) {
		t.Errorf("open filter: countOpen=%d should equal visible items %d", m.ac.countOpen, len(m.list.Items()))
	}

	// Closed: the scoped breakdown must contain zero open/ready/blocked.
	m.filter.currentFilter = "closed"
	m.applyFilter()
	if m.ac.countOpen != 0 || m.ac.countReady != 0 || m.ac.countBlocked != 0 {
		t.Errorf("closed filter: scoped breakdown should be all-closed, got open=%d ready=%d blocked=%d",
			m.ac.countOpen, m.ac.countReady, m.ac.countBlocked)
	}
}

// TestFooterData_CenterOverride proves the Phase 3 per-view center override
// replaces the scoped status stats + count: the override string appears and the
// "N issues" count badge is suppressed (the override carries its own count).
func TestFooterData_CenterOverride(t *testing.T) {
	fd := FooterData{
		Width:          120,
		FilterText:     "bt",
		FilterIcon:     "📂",
		HintText:       "l:labels",
		CountOpen:      163,
		CountReady:     2,
		CountClosed:    4,
		TotalItems:     169,
		CenterOverride: "bt-0qzp · 3/169",
		Hints:          []FooterHint{{Key: "esc", Desc: "back"}, {Key: "?", Desc: "help"}},
	}
	out := ansi.Strip(fd.Render())
	if !strings.Contains(out, "bt-0qzp · 3/169") {
		t.Errorf("center override should appear in footer: %q", out)
	}
	if strings.Contains(out, "169 issues") {
		t.Errorf("count badge should be suppressed when a center override is set: %q", out)
	}
	// The scoped status glyphs belong to the default center; the override takes
	// their place, so the open-count number should not surface as a stat segment.
	if strings.Contains(out, "○163") {
		t.Errorf("scoped status stats should not render alongside a center override: %q", out)
	}
}

// TestFooterData_CenterOverrideTimeTravelPrecedence ensures the corpus-wide time
// travel diff out-ranks a per-view center override (the diff is the more
// important signal while time-travelling).
func TestFooterData_CenterOverrideTimeTravelPrecedence(t *testing.T) {
	fd := FooterData{
		Width:            120,
		FilterText:       "bt",
		FilterIcon:       "📂",
		HintText:         "l:labels",
		TimeTravelActive: true,
		TimeTravelStats:  "⏱ 3d: +5 ✅2 ~3",
		CenterOverride:   "47 nodes · 61 edges",
		TotalItems:       169,
		Hints:            []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := ansi.Strip(fd.Render())
	if !strings.Contains(out, "⏱ 3d") {
		t.Errorf("time travel diff should win over center override: %q", out)
	}
	if strings.Contains(out, "47 nodes") {
		t.Errorf("center override should yield to active time travel: %q", out)
	}
}

// TestFooterData_CenterOverrideNeverWraps extends the one-row guarantee to the
// override center zone: at every width the footer stays a single row within the
// column count, and the override drops cleanly under extreme pressure.
func TestFooterData_CenterOverrideNeverWraps(t *testing.T) {
	base := FooterData{
		FilterText:     "bt",
		FilterIcon:     "📂",
		HintText:       "l:labels",
		TotalItems:     169,
		CenterOverride: "bt-0qzp · 3/169",
		Hints: []FooterHint{
			{Key: "esc", Desc: "back"},
			{Key: "C", Desc: "copy"},
			{Key: "O", Desc: "edit"},
			{Key: "?", Desc: "help"},
		},
	}
	for w := 24; w <= 220; w++ {
		fd := base
		fd.Width = w
		out := fd.Render()
		if strings.Contains(out, "\n") {
			t.Fatalf("width=%d: footer with center override wrapped to 2 rows: %q", w, out)
		}
		if got := ansi.StringWidth(out); got > w {
			t.Fatalf("width=%d: footer with center override overran width %d: %q", w, got, ansi.Strip(out))
		}
	}
}

// TestFooterCenter_PerView proves footerCenter() returns view-appropriate
// meaning: detail = bead id + position, graph = nodes/edges, board =
// columns/cards, and plain list = "" (keeps the default scoped counts).
func TestFooterCenter_PerView(t *testing.T) {
	newModel := func() Model {
		m := NewModel(harnessIssues(), nil, "", nil, nil)
		nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		return nm.(Model)
	}

	// Plain list keeps the default (no override).
	t.Run("list", func(t *testing.T) {
		m := newModel()
		if got := m.footerCenter(); got != "" {
			t.Errorf("list view should have no center override, got %q", got)
		}
	})

	// Detail (full-screen) = selected bead id + 1-based position / visible total.
	t.Run("detail", func(t *testing.T) {
		m := newModel()
		harnessSelect(&m, "bt-0qzp")
		m.showDetails = true
		m.focused = focusDetail
		got := m.footerCenter()
		if !strings.HasPrefix(got, "bt-0qzp · ") {
			t.Errorf("detail center should lead with the selected bead id, got %q", got)
		}
		total := len(m.list.VisibleItems())
		if want := fmt.Sprintf("bt-0qzp · %d/%d", m.list.Index()+1, total); got != want {
			t.Errorf("detail center = %q, want %q", got, want)
		}
	})

	// Graph = nodes/edges.
	t.Run("graph", func(t *testing.T) {
		m := newModel()
		m.mode = ViewGraph
		m.focused = focusGraph
		m.refreshBoardAndGraphForCurrentFilter()
		want := fmt.Sprintf("%s · %s",
			countLabel(m.graphView.TotalCount(), "node"),
			countLabel(m.graphView.EdgeCount(), "edge"))
		if got := m.footerCenter(); got != want {
			t.Errorf("graph center = %q, want %q", got, want)
		}
	})

	// Board = visible columns / cards.
	t.Run("board", func(t *testing.T) {
		m := newModel()
		m.mode = ViewBoard
		m.focused = focusBoard
		m.refreshBoardAndGraphForCurrentFilter()
		want := fmt.Sprintf("%s · %s",
			countLabel(m.board.VisibleColumnCount(), "col"),
			countLabel(m.board.TotalCount(), "card"))
		if got := m.footerCenter(); got != want {
			t.Errorf("board center = %q, want %q", got, want)
		}
	})

	// A modal over any view suppresses the override (keep underlying counts).
	t.Run("modal_suppresses", func(t *testing.T) {
		m := newModel()
		m.mode = ViewGraph
		m.focused = focusGraph
		m.refreshBoardAndGraphForCurrentFilter()
		m.activeModal = ModalHelp
		if got := m.footerCenter(); got != "" {
			t.Errorf("modal should suppress center override, got %q", got)
		}
	})
}

// TestCountLabel_Pluralization covers the singular/plural boundary.
func TestCountLabel_Pluralization(t *testing.T) {
	cases := []struct {
		n    int
		word string
		want string
	}{
		{0, "node", "0 nodes"},
		{1, "node", "1 node"},
		{2, "edge", "2 edges"},
		{4, "col", "4 cols"},
		{1, "card", "1 card"},
	}
	for _, c := range cases {
		if got := countLabel(c.n, c.word); got != c.want {
			t.Errorf("countLabel(%d, %q) = %q, want %q", c.n, c.word, got, c.want)
		}
	}
}

// TestFooterPinnedToBottomRow asserts the View()-level invariant that the
// footer is always the final terminal row, across every view mode. It guards
// the bt-yyked fix: under-filling views (detail/actionable) used to leave the
// footer floating mid-screen, and over-filling views (graph/insights) used to
// push it past the bottom where it was clipped away entirely.
func TestFooterPinnedToBottomRow(t *testing.T) {
	cases := []struct {
		name  string
		h     int
		setup func(*Model)
	}{
		{"list", 24, nil},
		{"detail", 28, func(m *Model) {
			harnessSelect(m, "bt-0qzp")
			m.showDetails = true
			m.focused = focusDetail
			m.updateViewportContent()
		}},
		{"graph", 32, func(m *Model) {
			m.mode = ViewGraph
			m.focused = focusGraph
			m.refreshBoardAndGraphForCurrentFilter()
		}},
		{"graph_scrunched", 20, func(m *Model) {
			m.mode = ViewGraph
			m.focused = focusGraph
			m.refreshBoardAndGraphForCurrentFilter()
		}},
		{"board", 32, func(m *Model) {
			m.mode = ViewBoard
			m.focused = focusBoard
			m.refreshBoardAndGraphForCurrentFilter()
		}},
		{"actionable", 32, func(m *Model) {
			m.mode = ViewActionable
			m.focused = focusActionable
		}},
		{"insights", 32, func(m *Model) { m.openInsightsView() }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := NewModel(harnessIssues(), nil, "", nil, nil)
			nm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: c.h})
			m = nm.(Model)
			if c.setup != nil {
				c.setup(&m)
			}
			content := ansi.Strip(m.View().Content)
			lines := strings.Split(content, "\n")
			if len(lines) != c.h {
				t.Fatalf("rendered %d rows, want exactly terminal height %d", len(lines), c.h)
			}
			gotLast := strings.TrimRight(lines[len(lines)-1], " ")
			wantFooter := strings.TrimRight(ansi.Strip(m.renderFooter()), " ")
			if gotLast != wantFooter {
				t.Errorf("footer is not the final row.\n last row: %q\n footer:   %q", gotLast, wantFooter)
			}
		})
	}
}

func TestFooterData_UpdateBadge(t *testing.T) {
	fd := FooterData{
		Width:      120,
		FilterText: "ALL",
		FilterIcon: "📋",
		HintText:   "l:labels",
		UpdateTag:  "v0.2.0",
		TotalItems: 10,
		Hints:      []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "v0.2.0") {
		t.Errorf("update tag should appear in output")
	}
}

func TestFooterData_SecondaryInstance(t *testing.T) {
	fd := FooterData{
		Width:        120,
		FilterText:   "ALL",
		FilterIcon:   "📋",
		HintText:     "l:labels",
		SecondaryPID: 12345,
		TotalItems:   10,
		Hints:        []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "12345") {
		t.Errorf("secondary PID should appear in output")
	}
}

func TestDegradedClearsOnSnapshot(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil, nil)
	// Simulate the scenario: Dolt was unreachable during initial load, so a
	// degraded toast was set before any snapshot arrived. snapshotInitPending
	// must be true so handleSnapshotReady takes the firstSnapshot branch
	// (which sets statusMsg="" without overwriting severity via
	// setInlineTransientStatus), letting clearStatus be the last status write.
	m.data.snapshotInitPending = true
	m.setDegraded("Dolt server unreachable (retrying)")
	if m.statusMsg == "" {
		t.Fatal("precondition: degraded toast should be set")
	}
	// SnapshotReadyMsg{} has a nil Snapshot, causing an early return before
	// the recovery path. Provide a minimal non-nil snapshot so the handler
	// accepts it as a genuine successful load. bt-a3zi3.1.
	nm, _ := m.handleSnapshotReady(SnapshotReadyMsg{Snapshot: &DataSnapshot{}})
	if nm.statusSeverity != SeverityNone || nm.statusMsg != "" {
		t.Errorf("degraded toast should clear on snapshot; got severity=%d msg=%q",
			nm.statusSeverity, nm.statusMsg)
	}
}

// TestDegradedClearsOnNonFirstSnapshot covers the recovery path the firstSnapshot
// case above does not: a Degraded toast set AFTER a prior snapshot already exists
// (the runtime "Dolt blipped mid-session" case). On that path handleSnapshotReady
// clears the degraded toast (line 369-371) and then, because firstSnapshot is false,
// calls setInlineTransientStatus("Reloaded N issues") which lands severity on
// SeveritySuccess (NOT None). The contract is: the degraded toast is gone — severity
// is no longer Degraded and the degraded message text is cleared. bt-a3zi3.1.
func TestDegradedClearsOnNonFirstSnapshot(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil, nil)
	// Non-first path: a prior snapshot already exists and init is no longer
	// pending, so firstSnapshot evaluates false. ModalNone (the default) keeps
	// shouldDeferRefresh() false so the handler does not defer.
	m.data.snapshot = &DataSnapshot{}
	m.data.snapshotInitPending = false
	m.setDegraded("Dolt server unreachable (retrying)")
	degradedMsg := m.statusMsg
	if m.statusSeverity != SeverityDegraded || degradedMsg == "" {
		t.Fatal("precondition: degraded toast should be set")
	}
	nm, _ := m.handleSnapshotReady(SnapshotReadyMsg{Snapshot: &DataSnapshot{}})
	if nm.statusSeverity == SeverityDegraded {
		t.Errorf("degraded toast must clear on non-first snapshot recovery; severity still Degraded")
	}
	if nm.statusMsg == degradedMsg {
		t.Errorf("degraded message text should be gone after recovery; got %q", nm.statusMsg)
	}
}

func TestErrorSettersBellAppend(t *testing.T) {
	newM := func() *Model {
		m := NewModel(harnessIssues(), nil, "", nil, nil)
		return &m
	}
	t.Run("notice does not touch the bell", func(t *testing.T) {
		m := newM()
		before := m.events.Len()
		m.setNotice("No issue selected")
		if m.events.Len() != before {
			t.Errorf("Notice appended an event; bell must stay clean")
		}
		if m.statusSeverity != SeverityNotice {
			t.Errorf("severity = %d, want Notice", m.statusSeverity)
		}
	})
	t.Run("failure appends one system event", func(t *testing.T) {
		m := newM()
		before := m.events.Len()
		m.setFailure("Export failed: disk full")
		if m.events.Len() != before+1 {
			t.Fatalf("Failure appended %d events, want 1", m.events.Len()-before)
		}
		if m.statusSeverity != SeverityFailure {
			t.Errorf("severity = %d, want Failure", m.statusSeverity)
		}
	})
	t.Run("degraded appends one system event and is sticky", func(t *testing.T) {
		m := newM()
		before := m.events.Len()
		m.setDegraded("Dolt server unreachable (retrying)")
		if m.events.Len() != before+1 {
			t.Fatalf("Degraded appended %d events, want 1", m.events.Len()-before)
		}
		if statusDismissAge(SeverityDegraded) != 0 {
			t.Errorf("Degraded must be sticky (dismiss age 0)")
		}
	})
}

// TestFooterNotificationsNeverWrap proves that the toast + bell never break the
// one-row / never-wrap footer invariant across a matrix of widths and
// notification states. A failure here indicates a real rendering defect in the
// footer's right-zone layout.
func TestFooterNotificationsNeverWrap(t *testing.T) {
	widths := []int{60, 70, 80, 100, 120, 160}
	setups := map[string]func(*Model){
		"idle":           func(m *Model) {},
		"success":        func(m *Model) { m.setStatus("reloaded +3 -1") },
		"failure":        func(m *Model) { m.setFailure("write failed: db locked") },
		"degraded":       func(m *Model) { m.setDegraded("Dolt server unreachable (retrying in 5s)") },
		"degraded_query": func(m *Model) { m.setDegraded("Dolt poll query failed (retrying in 5s): syntax error near SELECT") },
		"bell": func(m *Model) {
			for i := 0; i < 4; i++ {
				m.events.Append(events.NewSystemEvent("event"))
			}
		},
	}
	for name, setup := range setups {
		for _, w := range widths {
			t.Run(fmt.Sprintf("%s_%d", name, w), func(t *testing.T) {
				m := NewModel(harnessIssues(), nil, "", nil, nil)
				nm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 24})
				m = nm.(Model)
				setup(&m)
				footer := ansi.Strip(m.renderFooter())
				if strings.Contains(footer, "\n") {
					t.Fatalf("footer wrapped at width %d: %q", w, footer)
				}
				if got := lipgloss.Width(m.renderFooter()); got > w {
					t.Errorf("footer width %d exceeds terminal %d", got, w)
				}
				if !strings.Contains(footer, activeGlyphs.Bell) {
					t.Errorf("bell must always render (pinned, last to drop) at width %d state %s; got %q", w, name, footer)
				}
			})
		}
	}
}

// TestFooterToastBellCenterOverrideNeverWrap locks the center-zone x right-zone
// coexistence the model-driven sweep omits: a FooterData carrying a CenterOverride,
// a non-zero BellCount, AND the (post bt-kuvzj, now inert in the footer) toast
// fields must, at every width, stay a single row within the column count, still
// render the pinned bell, and never leak the toast text — the toast fields are
// still exercised here (rather than dropped from the fixture) so this test keeps
// proving they have zero footer footprint, not just that they're absent because
// nobody set them.
func TestFooterToastBellCenterOverrideNeverWrap(t *testing.T) {
	base := FooterData{
		FilterText:     "bt",
		FilterIcon:     "📂",
		HintText:       "l:labels",
		TotalItems:     169,
		CenterOverride: "bt-0qzp · 3/169",
		StatusMsg:      "write failed: db locked",
		StatusSeverity: SeverityFailure,
		StatusIsInline: true,
		BellCount:      3,
		Hints: []FooterHint{
			{Key: "esc", Desc: "back"},
			{Key: "C", Desc: "copy"},
			{Key: "O", Desc: "edit"},
			{Key: "?", Desc: "help"},
		},
	}
	for _, w := range []int{40, 60, 70, 80, 100, 120, 160} {
		fd := base
		fd.Width = w
		out := fd.Render()
		if strings.Contains(out, "\n") {
			t.Fatalf("width=%d: toast+bell+center-override footer wrapped to 2 rows: %q", w, out)
		}
		if got := lipgloss.Width(out); got > w {
			t.Fatalf("width=%d: toast+bell+center-override footer overran width %d: %q", w, got, ansi.Strip(out))
		}
		if !strings.Contains(ansi.Strip(out), activeGlyphs.Bell) {
			t.Errorf("width=%d: bell must always render (pinned, last to drop) alongside the toast; got %q", w, ansi.Strip(out))
		}
		if strings.Contains(ansi.Strip(out), "write failed: db locked") {
			t.Errorf("width=%d: footer must not render toast content post bt-kuvzj; got %q", w, ansi.Strip(out))
		}
	}
}

func TestMarkNotificationsSeenClearsBell(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil, nil)
	m.events.Append(events.NewSystemEvent("something happened"))
	if m.footerData().BellCount == 0 {
		t.Fatal("precondition: an unseen event should bump the bell")
	}
	m.markNotificationsSeen()
	if got := m.footerData().BellCount; got != 0 {
		t.Errorf("BellCount after mark-seen = %d, want 0", got)
	}
}

// TestHandleDoltConnectionStatus_QueryPhase proves phase propagation end to
// end: a query-phase doltPollError produces a distinct degraded message that
// surfaces the underlying cause, while a connect-phase failure keeps the
// original generic wording unchanged (bt-ndi5t).
func TestHandleDoltConnectionStatus_QueryPhase(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil, nil)
	queryErr := &doltPollError{phase: DoltPollPhaseQuery, err: fmt.Errorf("syntax error near SELECT")}
	nm, _ := m.handleDoltConnectionStatus(DoltConnectionStatusMsg{
		Connected:      false,
		Error:          queryErr,
		Phase:          DoltPollPhaseQuery,
		BackoffSeconds: 5,
	})
	if !strings.Contains(nm.statusMsg, "Dolt poll query failed (retrying in 5s)") {
		t.Errorf("query-phase message = %q, want the query-failed wording", nm.statusMsg)
	}
	if !strings.Contains(nm.statusMsg, "syntax error near SELECT") {
		t.Errorf("query-phase message = %q, want the underlying cause surfaced", nm.statusMsg)
	}
	if strings.Contains(nm.statusMsg, "unreachable") {
		t.Errorf("query-phase message = %q, must not claim the server is unreachable", nm.statusMsg)
	}

	connectErr := &doltPollError{phase: DoltPollPhaseConnect, err: fmt.Errorf("dial tcp: connection refused")}
	m2 := NewModel(harnessIssues(), nil, "", nil, nil)
	nm2, _ := m2.handleDoltConnectionStatus(DoltConnectionStatusMsg{
		Connected:      false,
		Error:          connectErr,
		Phase:          DoltPollPhaseConnect,
		BackoffSeconds: 5,
	})
	if nm2.statusMsg != "Dolt server unreachable (retrying in 5s)" {
		t.Errorf("connect-phase message = %q, want unchanged generic wording", nm2.statusMsg)
	}
}

// TestDoltPollErrorDetail covers the truncation and unwrap behavior used to
// build the query-phase footer suffix.
func TestDoltPollErrorDetail(t *testing.T) {
	if got := doltPollErrorDetail(nil); got != "" {
		t.Errorf("nil error detail = %q, want empty", got)
	}

	wrapped := &doltPollError{phase: DoltPollPhaseQuery, err: fmt.Errorf("short cause")}
	if got := doltPollErrorDetail(wrapped); got != ": short cause" {
		t.Errorf("detail = %q, want %q", got, ": short cause")
	}

	long := strings.Repeat("x", doltPollErrorDetailMaxRunes+40)
	longWrapped := &doltPollError{phase: DoltPollPhaseQuery, err: fmt.Errorf("%s", long)}
	got := doltPollErrorDetail(longWrapped)
	body := strings.TrimPrefix(got, ": ")
	if lipgloss.Width(body) > doltPollErrorDetailMaxRunes {
		t.Errorf("detail not truncated: width=%d, want <= %d", lipgloss.Width(body), doltPollErrorDetailMaxRunes)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated detail should end with ellipsis, got %q", got)
	}
}
