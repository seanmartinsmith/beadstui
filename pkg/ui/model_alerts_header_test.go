package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// statusHeaderModel builds a model with a datasource, a handful of issues across
// two sources, and a mix of anomaly + advisory alerts (including one INFO-
// severity anomaly type, which must land on the advisory side). anomalies = 2
// (dependency loop critical + abandoned claim warning); advisories = 3
// (issue-count-change INFO anomaly-type, stale warning, high-leverage info).
func statusHeaderModel(t *testing.T) Model {
	t.Helper()
	now := time.Now()
	issues := []model.Issue{
		{ID: "bt-1", Title: "one", Status: model.StatusOpen, CreatedAt: now},
		{ID: "bt-2", Title: "two", Status: model.StatusOpen, CreatedAt: now},
		{ID: "bt-3", Title: "three", Status: model.StatusOpen, CreatedAt: now},
		{ID: "sym-1", Title: "s1", Status: model.StatusOpen, CreatedAt: now},
		{ID: "sym-2", Title: "s2", Status: model.StatusOpen, CreatedAt: now},
	}
	m := NewModel(issues, nil, "", nil, nil)
	m.width = 120
	m.height = 36
	m.mode = ViewList
	m.ready = true
	m.data.dataSource = &datasource.DataSource{Type: datasource.SourceTypeEmbeddedDolt}
	m.alerts = []drift.Alert{
		{Type: drift.AlertDependencyLoop, Severity: drift.SeverityCritical, Message: "1 new cycle(s) detected", Details: []string{"bt-1 -> bt-2 -> bt-1"}},
		{Type: drift.AlertAbandonedClaim, Severity: drift.SeverityWarning, Message: "abandoned", IssueID: "bt-1"},
		{Type: drift.AlertIssueCountChange, Severity: drift.SeverityInfo, Message: "count change"},
		{Type: drift.AlertStale, Severity: drift.SeverityWarning, Message: "stale", IssueID: "bt-2"},
		{Type: drift.AlertHighLeverage, Severity: drift.SeverityInfo, Message: "leverage", IssueID: "bt-3"},
	}
	return m
}

// TestAlertClassCounts_ReconcilesBadge is the core of the traceable-badge fix
// (bt-2nepr): the header's "N anomalies" equals the footer badge, and
// anomalies + advisories equals the modal's own total.
func TestAlertClassCounts_ReconcilesBadge(t *testing.T) {
	m := statusHeaderModel(t)

	anomalies, advisories := m.alertClassCounts()
	if anomalies != 2 {
		t.Errorf("anomalies = %d, want 2 (dep-loop crit + abandoned warn)", anomalies)
	}
	if advisories != 3 {
		t.Errorf("advisories = %d, want 3 (info anomaly-type + stale + leverage)", advisories)
	}
	if total := len(m.visibleAlerts()); anomalies+advisories != total {
		t.Errorf("anomalies+advisories = %d, want total %d", anomalies+advisories, total)
	}

	// Footer badge input must match the header's anomaly figure exactly.
	_, critical, warning := m.extractAlertCounts()
	if critical+warning != anomalies {
		t.Errorf("badge attention count %d != header anomalies %d", critical+warning, anomalies)
	}
}

// TestAlertsHeader_RendersAllSections asserts the "bt status report" header
// surfaces every section from fixture state: badge reconciliation, Dolt mode +
// server status, per-source counts, corpus scale, watcher/freshness, and the
// phase-2-fallback consequence line.
func TestAlertsHeader_RendersAllSections(t *testing.T) {
	m := statusHeaderModel(t)
	m.activeTab = TabAlerts
	// Server source so the header exercises the server-status word ("connected");
	// embedded/jsonl are in-process and carry no server status.
	m.data.dataSource = &datasource.DataSource{Type: datasource.SourceTypeDolt}
	m.doltConnected = true
	// Freshness + phase-2 fallback signal.
	m.lastDoltVerified = time.Now().Add(-12 * time.Second)
	m.data.snapshot = &DataSnapshot{DatasetTier: datasetTierHuge, CreatedAt: time.Now()}

	plain := ansi.Strip(strings.Join(m.alertsHeaderLines(m.alertsPanelWidth()-4), "\n"))

	for _, tok := range []string{
		"anomalies",       // badge reconciliation (line 1)
		"advisories",      //   "
		"Dolt server",     // dolt mode
		"connected",       // server status
		"corpus",          // corpus scale fact
		"phase-2 skipped", // consequence-not-threshold status line
		"sources",         // per-source health
		"bt 3",            // per-source count
		"updated 12s ago", // data freshness
	} {
		if !strings.Contains(plain, tok) {
			t.Errorf("status header missing %q; got:\n%s", tok, plain)
		}
	}
}

// TestAlertsBadgeOpen_PreFiltersToAnomalies covers the badge-activation wiring:
// pressing ! while the badge is lit lands the modal pre-filtered to exactly the
// attention anomalies (badge number), so the badge is traceable.
func TestAlertsBadgeOpen_PreFiltersToAnomalies(t *testing.T) {
	m := statusHeaderModel(t)
	m = pressRune(m, '!')
	if m.activeModal != ModalAlerts || m.activeTab != TabAlerts {
		t.Fatalf("setup: modal/tab not as expected (%v/%v)", m.activeModal, m.activeTab)
	}
	if m.alertFilterClass != "anomaly" {
		t.Fatalf("expected pre-filter class %q, got %q", "anomaly", m.alertFilterClass)
	}
	if got := len(m.visibleAlerts()); got != 2 {
		t.Fatalf("expected 2 visible (the attention anomalies), got %d", got)
	}
	for _, a := range m.visibleAlerts() {
		if !isAttentionAnomaly(a) {
			t.Errorf("pre-filtered list leaked a non-anomaly: %v/%v", a.Type, a.Severity)
		}
	}
}

// TestAlertsBadgeOpen_NoAnomaliesOpensUnfiltered: dark cockpit — with only
// advisories, ! still opens (advisories are browsable) but does NOT pre-filter,
// so the user sees the full advisory set rather than an empty list.
func TestAlertsBadgeOpen_NoAnomaliesOpensUnfiltered(t *testing.T) {
	m := statusHeaderModel(t)
	m.alerts = []drift.Alert{
		{Type: drift.AlertStale, Severity: drift.SeverityWarning, Message: "stale a", IssueID: "bt-1"},
		{Type: drift.AlertStale, Severity: drift.SeverityWarning, Message: "stale b", IssueID: "bt-2"},
	}
	m = pressRune(m, '!')
	if m.activeModal != ModalAlerts {
		t.Fatalf("setup: modal did not open")
	}
	if m.alertFilterClass != "" {
		t.Fatalf("dark cockpit must open unfiltered, got class %q", m.alertFilterClass)
	}
	if got := len(m.visibleAlerts()); got != 2 {
		t.Fatalf("expected 2 visible advisories, got %d", got)
	}
}

// TestAlertsBadgeAbsentAtZero: the footer anomaly badge is empty when no
// attention anomaly exists (dark cockpit, bt-9gjt0), and lights up when one does.
func TestAlertsBadgeAbsentAtZero(t *testing.T) {
	m := statusHeaderModel(t)
	m.alerts = []drift.Alert{
		{Type: drift.AlertStale, Severity: drift.SeverityWarning, Message: "stale", IssueID: "bt-1"},
		{Type: drift.AlertIssueCountChange, Severity: drift.SeverityInfo, Message: "info anomaly-type"},
	}
	if badge := m.footerData().renderAlertsBadge(); badge != "" {
		t.Errorf("advisory-only footer must show no anomaly badge, got %q", badge)
	}

	m2 := statusHeaderModel(t) // has 2 attention anomalies
	if badge := m2.footerData().renderAlertsBadge(); badge == "" {
		t.Error("footer must light the anomaly badge when attention anomalies exist")
	}
}

// TestFooterLargeDatasetBadgeGone: the large-dataset footer badge is retired
// (bt-2nepr / bt-ajbxw). Even with a huge-tier snapshot carrying a
// LargeDatasetWarning, the rendered footer surfaces no such warning text.
func TestFooterLargeDatasetBadgeGone(t *testing.T) {
	m := seedModel()
	m.data.snapshot = &DataSnapshot{
		DatasetTier:         datasetTierHuge,
		LargeDatasetWarning: "large: 25000 issues (open-only)",
		CreatedAt:           time.Now(),
	}
	footer := ansi.Strip(m.renderFooter())
	for _, tok := range []string{"25000", "large:", "open-only"} {
		if strings.Contains(footer, tok) {
			t.Errorf("footer must not render the retired large-dataset badge (%q); got:\n%s", tok, footer)
		}
	}
}

// TestAlertsClassFilterCycle drives the a/A keys through the anomaly/advisory
// class filter, alongside the existing s/t/p/o dimensions.
func TestAlertsClassFilterCycle(t *testing.T) {
	m := statusHeaderModel(t)
	m = pressRune(m, '!')
	// Opening from the lit badge pre-sets "anomaly"; start the cycle from a
	// clean state to exercise every transition deterministically.
	m.alertFilterClass = ""

	m = pressRune(m, 'a')
	if m.alertFilterClass != "anomaly" {
		t.Fatalf("a from all: want anomaly, got %q", m.alertFilterClass)
	}
	m = pressRune(m, 'a')
	if m.alertFilterClass != "advisory" {
		t.Fatalf("a from anomaly: want advisory, got %q", m.alertFilterClass)
	}
	m = pressRune(m, 'a')
	if m.alertFilterClass != "" {
		t.Fatalf("a from advisory: want all, got %q", m.alertFilterClass)
	}

	// Backwards.
	m = pressRune(m, 'A')
	if m.alertFilterClass != "advisory" {
		t.Fatalf("A from all: want advisory, got %q", m.alertFilterClass)
	}
	m = pressRune(m, 'A')
	if m.alertFilterClass != "anomaly" {
		t.Fatalf("A from advisory: want anomaly, got %q", m.alertFilterClass)
	}

	// reset clears the class filter with the other dimensions.
	m.alertFilterSeverity = "critical"
	m = pressRune(m, 'r')
	if m.alertFilterClass != "" || m.alertFilterSeverity != "" {
		t.Fatalf("r must reset class+severity, got class=%q sev=%q", m.alertFilterClass, m.alertFilterSeverity)
	}
}

// TestAlertsModalStatusHeaderBothTiers renders the alerts modal (with the status
// header) at the design widths in both glyph tiers and asserts the panel stays a
// clean grid: exactly panelHeight rows, every row exactly panelWidth cells (no
// wrap, no mid-token clip, no border drift), and the header sections present.
func TestAlertsModalStatusHeaderBothTiers(t *testing.T) {
	widths := []int{50, 70, 100, 130, 160}
	for _, tier := range []struct {
		name string
		g    GlyphSet
	}{{"nerdfont", nerdfontGlyphs}, {"ascii", asciiGlyphs}} {
		t.Run(tier.name, func(t *testing.T) {
			setGlyphs(t, tier.g)
			for _, w := range widths {
				m := statusHeaderModel(t)
				m.width = w
				m.height = 36
				m.activeTab = TabAlerts
				m.activeModal = ModalAlerts

				panel := m.renderAlertsPanel()
				lines := strings.Split(panel, "\n")
				pw := m.alertsPanelWidth()
				ph := m.alertsPanelHeight()

				if len(lines) != ph {
					t.Errorf("w=%d tier=%s: panel has %d rows, want %d", w, tier.name, len(lines), ph)
				}
				for i, ln := range lines {
					if lw := ansi.StringWidth(ln); lw != pw {
						t.Errorf("w=%d tier=%s: row %d width %d != panelWidth %d: %q",
							w, tier.name, i, lw, pw, ansi.Strip(ln))
					}
				}
				plain := ansi.Strip(panel)
				for _, tok := range []string{"anomal", "advisor", "Dolt", "corpus"} {
					if !strings.Contains(plain, tok) {
						t.Errorf("w=%d tier=%s: header missing %q:\n%s", w, tier.name, tok, plain)
					}
				}
			}
		})
	}
}
