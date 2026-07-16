package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/version"
)

// ════════════════════════════════════════════════════════════════════════════
// ALERTS MODAL STATUS HEADER — "bt status report" (bt-2nepr)
// ════════════════════════════════════════════════════════════════════════════
//
// A persistent header block at the top of the Alerts tab. It reconciles the
// footer anomaly badge (line 1) and gives the evicted daemon chrome an explicit
// home: Dolt mode + server status, per-source issue counts, watcher / worker /
// data freshness, and corpus scale as a STATUS FACT (never a threshold warning).
//
// Design: docs/design/2026-07-16-footer-lens-redesign.md, "Relocations" §1-2/§5
// and "Branding". Only the Alerts tab carries the header; the Notifications tab
// (event ring) is a separate surface and its geometry is untouched.

// isAttentionAnomaly reports whether an alert is an attention-worthy anomaly: an
// anomaly-typed alert (drift.AlertType.IsAnomaly) at critical or warning
// severity. This is exactly the footer badge's input (extractAlertCounts), so
// the status-header reconciliation and the alerts-modal anomaly-class filter
// reproduce the badge number precisely — even though some anomaly types
// (issue_count_change, coupling_growth, dependency_change, actionable_change)
// ALSO emit at info severity. Those info-severity anomalies read as advisory
// noise, not badge-worthy signal, so they fall on the advisory side of the
// reconciliation (bt-2nepr, building on bt-jhzat).
func isAttentionAnomaly(a drift.Alert) bool {
	return a.Type.IsAnomaly() && (a.Severity == drift.SeverityCritical || a.Severity == drift.SeverityWarning)
}

// passesAlertBaseScope reports whether an alert survives the non-stackable
// filters: dismissed state and (workspace) active-repo scope. The stackable
// severity/type/project/class filters are applied on top of this in
// filteredAlerts; the status header counts against this base so the badge
// reconciliation stays stable regardless of how the user has filtered the list.
func (m Model) passesAlertBaseScope(a drift.Alert) bool {
	if m.dismissedAlerts[alertKey(a)] {
		return false
	}
	if m.workspaceMode && m.activeRepos != nil && a.IssueID != "" {
		if issue, ok := m.data.issueMap[a.IssueID]; ok {
			repoKey := IssueRepoKey(*issue)
			if repoKey != "" && !m.activeRepos[repoKey] {
				return false
			}
		}
	}
	return true
}

// alertClassCounts returns (anomalies, advisories) among alerts that pass the
// base scope but IGNORING the stackable severity/type/project/class filters.
// anomalies is the footer badge's number (isAttentionAnomaly); advisories is
// everything else visible. Their sum is the modal's own total, so line 1 of the
// status header always explains the badge (fixes the 4-vs-1487 dogfood
// confusion, bt-2nepr).
func (m Model) alertClassCounts() (anomalies, advisories int) {
	for _, a := range m.alerts {
		if !m.passesAlertBaseScope(a) {
			continue
		}
		if isAttentionAnomaly(a) {
			anomalies++
		} else {
			advisories++
		}
	}
	return
}

// anomalyAlertCount is the number of attention-worthy anomalies among the
// currently visible alerts — the footer badge's number. Used to decide whether
// opening the modal from the badge should pre-filter to the anomaly class.
func (m Model) anomalyAlertCount() int {
	n := 0
	for _, a := range m.visibleAlerts() {
		if isAttentionAnomaly(a) {
			n++
		}
	}
	return n
}

// doltModeLabel names the active data-source mode for the status header.
func (m Model) doltModeLabel() string {
	if m.data.dataSource == nil {
		return "unknown"
	}
	switch m.data.dataSource.Type {
	case datasource.SourceTypeEmbeddedDolt:
		return "embedded"
	case datasource.SourceTypeDolt:
		return "server"
	case datasource.SourceTypeDoltGlobal:
		return "shared-server"
	case datasource.SourceTypeJSONLFallback:
		return "jsonl"
	default:
		return string(m.data.dataSource.Type)
	}
}

// doltStatusLabel returns a server-health word for server/shared-server sources
// ("connected" once the poll loop has verified, else "connecting"). Embedded and
// JSONL sources are in-process — they have no server to be healthy or not — so
// they return "" and the header shows the mode alone.
func (m Model) doltStatusLabel() string {
	if m.data.dataSource == nil {
		return ""
	}
	switch m.data.dataSource.Type {
	case datasource.SourceTypeDolt, datasource.SourceTypeDoltGlobal:
		if m.doltConnected {
			return "connected"
		}
		return "connecting"
	default:
		return ""
	}
}

// sourceCount pairs a source (repo) key with its loaded issue count.
type sourceCount struct {
	key string
	n   int
}

// sourceIssueCounts tallies loaded issues per source, highest count first. This
// is the corpus-scale + per-source-health data available NOW (bt-2nepr §3/§5).
// The per-source UNAVAILABLE detail ("N sources unavailable") routes here later
// via bt-2ea7t.10, and DB sizes / load timings via bt-2ea7t.7 — this function is
// the seam both hang off (counts derived from the loaded set, no new
// instrumentation).
func (m Model) sourceIssueCounts() []sourceCount {
	counts := map[string]int{}
	for i := range m.data.issues {
		k := IssueRepoKey(m.data.issues[i])
		if k == "" {
			k = ExtractRepoPrefix(m.data.issues[i].ID)
		}
		if k == "" {
			continue
		}
		counts[k]++
	}
	out := make([]sourceCount, 0, len(counts))
	for k, n := range counts {
		out = append(out, sourceCount{key: k, n: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].n != out[j].n {
			return out[i].n > out[j].n
		}
		return out[i].key < out[j].key
	})
	return out
}

// dataFreshnessAge returns how long ago the loaded data was last verified
// current, preferring the Dolt poll timestamp and falling back to the snapshot
// build time. ok is false when neither is known.
func (m Model) dataFreshnessAge() (time.Duration, bool) {
	if !m.lastDoltVerified.IsZero() {
		return time.Since(m.lastDoltVerified), true
	}
	if m.data.snapshot != nil && !m.data.snapshot.CreatedAt.IsZero() {
		return time.Since(m.data.snapshot.CreatedAt), true
	}
	return 0, false
}

// countWord formats "N word" with singular/plural forms ("1 anomaly" /
// "4 anomalies", "1 issue" / "169 issues").
func countWord(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// formatShortAge renders a compact duration ("<1s", "42s", "6m", "3h", "5d").
func formatShortAge(d time.Duration) string {
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

// alertsHeaderRows returns the number of terminal rows the status-header block
// occupies on the Alerts tab, INCLUDING its trailing blank separator. Zero on
// the Notifications tab. Both the renderer (renderAlertsTab) and the mouse
// hit-test (alertsModalItemAtY / summary-row click) read this so the alert-row
// geometry stays aligned with what is drawn.
func (m Model) alertsHeaderRows() int {
	if m.activeTab != TabAlerts {
		return 0
	}
	n := len(m.alertsHeaderLines(m.alertsPanelWidth() - 4))
	if n == 0 {
		return 0
	}
	return n + 1 // + trailing blank separator
}

// alertsItemsChromeRows is the panel-relative row of the first item in the
// shared modal: the fixed chrome (border + summary + blank + above-hint,
// modalChromeAboveItems) plus, on the Alerts tab, the status-header block
// (alertsHeaderRows, which is 0 on the Notifications tab). Shared by the mouse
// hit-test and its tests so the geometry has a single source of truth (bt-2nepr).
func (m Model) alertsItemsChromeRows() int {
	return modalChromeAboveItems + m.alertsHeaderRows()
}

// alertsHeaderLines builds the "bt status report" content lines (no trailing
// blank), already styled and laid out for innerWidth. Line 1 is the badge
// reconciliation (design "Line 1") plus branding/identity; the remaining lines
// carry Dolt mode + corpus scale, per-source counts, and watcher/freshness. The
// set is capped from the bottom so line 1 always survives on short terminals.
func (m Model) alertsHeaderLines(innerWidth int) []string {
	if innerWidth < 8 {
		return nil
	}
	t := m.theme
	// Content is indented one column (matching the summary/"No active alerts"
	// rows); padContentLines adds a second in renderAlertsPanel.
	contentW := innerWidth - 1
	if contentW < 6 {
		contentW = 6
	}

	mutedStyle := lipgloss.NewStyle().Foreground(t.Muted)
	valStyle := lipgloss.NewStyle().Foreground(t.Secondary)
	sep := mutedStyle.Render(" " + activeGlyphs.Sep + " ")

	// layout places left + right on one line within contentW. When there is no
	// room for the right chunk it is dropped and the left is truncated.
	layout := func(left, right string) string {
		lw := lipgloss.Width(left)
		rw := lipgloss.Width(right)
		if right == "" || lw+1+rw > contentW {
			return " " + truncateRunesHelper(left, contentW, activeGlyphs.Ellipsis)
		}
		gap := contentW - lw - rw
		return " " + left + strings.Repeat(" ", gap) + right
	}
	fit := func(s string) string {
		return " " + truncateRunesHelper(s, contentW, activeGlyphs.Ellipsis)
	}

	var lines []string

	// 1. Badge reconciliation + branding (design "Line 1"). The anomaly count is
	//    exactly the footer badge; the two together are the modal's total.
	anomalies, advisories := m.alertClassCounts()
	anomText := countWord(anomalies, "anomaly", "anomalies")
	var anomStyled string
	if anomalies > 0 {
		anomStyled = lipgloss.NewStyle().Foreground(t.Blocked).Bold(true).Render(anomText)
	} else {
		anomStyled = mutedStyle.Render(anomText)
	}
	recon := anomStyled + sep + valStyle.Render(countWord(advisories, "advisory", "advisories"))
	brand := mutedStyle.Render("bt " + version.Version)
	lines = append(lines, layout(recon, brand))

	// 2. Dolt mode + server status · corpus scale (status fact, not a warning).
	doltParts := []string{"Dolt " + m.doltModeLabel()}
	if s := m.doltStatusLabel(); s != "" {
		doltParts = append(doltParts, s)
	}
	total := len(m.data.issues)
	corpus := countWord(total, "issue", "issues") + " (" + datasetTierForIssueCount(total).String() + ")"
	// Phase-2 fell back on the huge tier (snapshot built with SkipPhase2). This
	// is the consequence-as-status-fact: corpus size alone is never a warning,
	// but an actual analysis fallback earns a line here (bt-2nepr; supersedes the
	// large-dataset footer badge, bt-ajbxw).
	if m.data.snapshot != nil && m.data.snapshot.DatasetTier == datasetTierHuge {
		corpus += " phase-2 skipped"
	}
	doltLine := mutedStyle.Render(strings.Join(doltParts, " ")) + sep +
		mutedStyle.Render("corpus ") + valStyle.Render(corpus)
	lines = append(lines, layout(doltLine, ""))

	// 3. Per-source issue counts (per-source health home; unavailable-source
	//    detail routes here later via bt-2ea7t.10, sizes/timings via bt-2ea7t.7).
	if srcs := m.sourceIssueCounts(); len(srcs) > 0 {
		const maxShown = 4
		shown := srcs
		extra := 0
		if len(shown) > maxShown {
			extra = len(shown) - maxShown
			shown = shown[:maxShown]
		}
		parts := make([]string, 0, len(shown)+1)
		for _, sc := range shown {
			parts = append(parts, fmt.Sprintf("%s %d", model.DisplayRepoName(sc.key), sc.n))
		}
		if extra > 0 {
			parts = append(parts, fmt.Sprintf("+%d more", extra))
		}
		srcLine := mutedStyle.Render("sources ") + valStyle.Render(strings.Join(parts, " "+activeGlyphs.Sep+" "))
		lines = append(lines, layout(srcLine, ""))
	}

	// 4. Watcher / background worker / data freshness.
	var freshParts []string
	if w := m.extractWatcherBadge(); w != "" {
		freshParts = append(freshParts, w)
	}
	if wt, _ := m.extractWorkerBadge(); wt != "" {
		freshParts = append(freshParts, wt)
	}
	if age, ok := m.dataFreshnessAge(); ok {
		freshParts = append(freshParts, "updated "+formatShortAge(age)+" ago")
	}
	if len(freshParts) > 0 {
		lines = append(lines, fit(mutedStyle.Render(strings.Join(freshParts, " "+activeGlyphs.Sep+" "))))
	}

	// Cap from the bottom so the reconciliation (line 1) always survives on short
	// terminals. Reserve at least 2 rows for the alert list itself.
	maxContent := m.alertsVisibleLines() - 2
	if maxContent < 1 {
		maxContent = 1
	}
	if len(lines) > maxContent {
		lines = lines[:maxContent]
	}
	return lines
}
