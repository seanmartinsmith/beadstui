package drift

import (
	"sort"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/baseline"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// ProjectAlerts computes per-project scoped drift alerts (bt-46p6.8, bt-7l5m).
//
// Partitions allIssues by SourceRepo (global=true) or collapses to a single
// group keyed by fallbackProject (global=false). Runs one Calculator per
// group on its own analyzer output, with its own baseline if loadBaseline
// supplies one. Every returned Alert carries SourceProject = the project key.
//
// The input should already be the globally-resolved issue set (external:
// deps rewritten to canonical IDs). In single-project mode, the input is
// typically identical to rc.issues.
//
// loadBaseline may be nil. When nil — or when it returns nil for a given
// project — the Calculator runs with baseline == current, so drift-delta
// alerts (coupling_growth, centrality_change, dependency_change,
// issue_count_change, blocked_increase, actionable_change) cannot fire for
// that project. Non-drift alerts (dependency_loop, stale, high_leverage,
// abandoned_claim) still fire normally.
//
// Iteration order over projects is stable (sorted by key) so callers can
// rely on deterministic alert ordering across runs.
//
// Interpretation note: bt-7l5m's "each project graph includes its real
// cross-project edges" is implemented partition-only — per-project analyzers
// see only that project's issues. Satellite-node inclusion (Option B) is
// tracked as follow-up audit in bt-53vw.
func ProjectAlerts(
	allIssues []model.Issue,
	global bool,
	fallbackProject string,
	config *Config,
	loadBaseline func(project string) *baseline.Baseline,
) []Alert {
	if config == nil {
		config = DefaultConfig()
	}
	groups := groupByProject(allIssues, global, fallbackProject)

	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out []Alert
	for _, project := range keys {
		out = append(out, calculateProject(project, groups[project], config, loadBaseline)...)
	}
	return out
}

// calculateProject runs the full drift pipeline for a single project group
// and stamps SourceProject on every returned alert.
func calculateProject(
	project string,
	issues []model.Issue,
	config *Config,
	loadBaseline func(project string) *baseline.Baseline,
) []Alert {
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	openCount, closedCount, blockedCount := countStatuses(issues)
	cycles := stats.Cycles()

	currentStats := baseline.GraphStats{
		NodeCount:       stats.NodeCount,
		EdgeCount:       stats.EdgeCount,
		Density:         stats.Density,
		OpenCount:       openCount,
		ClosedCount:     closedCount,
		BlockedCount:    blockedCount,
		CycleCount:      len(cycles),
		ActionableCount: len(analyzer.GetActionableIssues()),
	}

	bl := &baseline.Baseline{Stats: currentStats}
	cur := &baseline.Baseline{Stats: currentStats, Cycles: cycles}

	if loadBaseline != nil {
		if loaded := loadBaseline(project); loaded != nil {
			bl = loaded
			cur.TopMetrics = baseline.TopMetrics{
				PageRank:     topMetricItems(stats.PageRank(), 10),
				Betweenness:  topMetricItems(stats.Betweenness(), 10),
				CriticalPath: topMetricItems(stats.CriticalPathScore(), 10),
				Hubs:         topMetricItems(stats.Hubs(), 10),
				Authorities:  topMetricItems(stats.Authorities(), 10),
			}
		}
	}

	calc := NewCalculator(bl, cur, config)
	calc.SetIssues(issues)
	result := calc.Calculate()

	for i := range result.Alerts {
		result.Alerts[i].SourceProject = project
	}
	return result.Alerts
}

// groupByProject partitions issues by project key for scoped alert computation.
//
// Global mode: group by SourceRepo. Empty SourceRepo maps to "unknown" so no
// issue is silently dropped.
//
// Single-project mode: everything is one group keyed by fallbackProject,
// falling back to a uniform SourceRepo if fallbackProject is empty, then
// "local" as a final fallback so every group has a non-empty key.
func groupByProject(issues []model.Issue, global bool, fallback string) map[string][]model.Issue {
	if !global {
		key := fallback
		if key == "" {
			if uniform, ok := uniformSourceRepo(issues); ok {
				key = uniform
			}
		}
		if key == "" {
			key = "local"
		}
		return map[string][]model.Issue{key: issues}
	}

	out := make(map[string][]model.Issue)
	for i := range issues {
		p := issues[i].SourceRepo
		if p == "" {
			p = "unknown"
		}
		out[p] = append(out[p], issues[i])
	}
	return out
}

// uniformSourceRepo returns the single SourceRepo value if every issue shares
// it; (false) when the set is empty or heterogeneous.
func uniformSourceRepo(issues []model.Issue) (string, bool) {
	if len(issues) == 0 {
		return "", false
	}
	first := issues[0].SourceRepo
	if first == "" {
		return "", false
	}
	for i := 1; i < len(issues); i++ {
		if issues[i].SourceRepo != first {
			return "", false
		}
	}
	return first, true
}

// countStatuses tallies open/closed/blocked counts in a single pass. Matches
// the semantics of the existing robot_alerts.go summary counting: StatusOpen
// and StatusInProgress both count as open; tombstones and unknown statuses
// are ignored for aggregates.
func countStatuses(issues []model.Issue) (open, closed, blocked int) {
	for _, iss := range issues {
		switch iss.Status {
		case model.StatusOpen, model.StatusInProgress:
			open++
		case model.StatusClosed:
			closed++
		case model.StatusBlocked:
			blocked++
		}
	}
	return
}

// topMetricItems builds a sorted-descending MetricItem list from a map of
// scores, limited to the top N. Mirrors cmd/bt/helpers.go#buildMetricItems so
// drift stays self-contained (drift is imported by both cmd/bt and pkg/ui).
func topMetricItems(metrics map[string]float64, limit int) []baseline.MetricItem {
	if len(metrics) == 0 {
		return nil
	}
	items := make([]baseline.MetricItem, 0, len(metrics))
	for id, value := range metrics {
		items = append(items, baseline.MetricItem{ID: id, Value: value})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Value > items[j].Value
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}
