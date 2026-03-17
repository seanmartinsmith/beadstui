package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/baseline"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// runAlerts handles --robot-alerts (drift + proactive).
func (rc *robotCtx) runAlerts(alertSeverity, alertType, alertLabel string) {
	driftConfig, err := drift.LoadConfig(rc.projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading drift config: %v\n", err)
		os.Exit(1)
	}

	analyzer := analysis.NewAnalyzer(rc.issues)
	stats := analyzer.Analyze()

	openCount, closedCount, blockedCount := 0, 0, 0
	for _, issue := range rc.issues {
		switch issue.Status {
		case model.StatusClosed:
			closedCount++
		case model.StatusBlocked:
			blockedCount++
		case model.StatusOpen, model.StatusInProgress:
			openCount++
		default:
			// Ignore tombstones and any unknown statuses for summary counts.
		}
	}
	actionableCount := len(analyzer.GetActionableIssues())
	cycles := stats.Cycles()
	curStats := baseline.GraphStats{
		NodeCount:       stats.NodeCount,
		EdgeCount:       stats.EdgeCount,
		Density:         stats.Density,
		OpenCount:       openCount,
		ClosedCount:     closedCount,
		BlockedCount:    blockedCount,
		CycleCount:      len(cycles),
		ActionableCount: actionableCount,
	}

	// Default behavior (no baseline): drift comparisons are suppressed by using
	// baseline=current for stats, while still allowing cycle/staleness/cascade alerts.
	bl := &baseline.Baseline{Stats: curStats}
	cur := &baseline.Baseline{Stats: curStats, Cycles: cycles}

	baselinePath := baseline.DefaultPath(rc.projectDir)

	// If a baseline exists, compare against it for real drift deltas.
	if baseline.Exists(baselinePath) {
		loaded, err := baseline.Load(baselinePath)
		if err != nil {
			envRobot := os.Getenv("BT_ROBOT") == "1"
			if !envRobot {
				fmt.Fprintf(os.Stderr, "Warning: Error loading baseline: %v\n", err)
			}
		} else {
			bl = loaded
			topMetrics := baseline.TopMetrics{
				PageRank:     buildMetricItems(stats.PageRank(), 10),
				Betweenness:  buildMetricItems(stats.Betweenness(), 10),
				CriticalPath: buildMetricItems(stats.CriticalPathScore(), 10),
				Hubs:         buildMetricItems(stats.Hubs(), 10),
				Authorities:  buildMetricItems(stats.Authorities(), 10),
			}
			cur = &baseline.Baseline{Stats: curStats, TopMetrics: topMetrics, Cycles: cycles}
		}
	}

	calc := drift.NewCalculator(bl, cur, driftConfig)
	calc.SetIssues(rc.issues)
	driftResult := calc.Calculate()

	// Apply optional filters
	filtered := driftResult.Alerts[:0]
	for _, a := range driftResult.Alerts {
		if alertSeverity != "" && string(a.Severity) != alertSeverity {
			continue
		}
		if alertType != "" && string(a.Type) != alertType {
			continue
		}
		if alertLabel != "" {
			found := false
			for _, d := range a.Details {
				if strings.Contains(strings.ToLower(d), strings.ToLower(alertLabel)) {
					found = true
					break
				}
			}
			if !found && a.Label != "" && !strings.Contains(strings.ToLower(a.Label), strings.ToLower(alertLabel)) {
				continue
			}
		}
		filtered = append(filtered, a)
	}
	driftResult.Alerts = filtered

	output := struct {
		RobotEnvelope
		Alerts  []drift.Alert `json:"alerts"`
		Summary struct {
			Total    int `json:"total"`
			Critical int `json:"critical"`
			Warning  int `json:"warning"`
			Info     int `json:"info"`
		} `json:"summary"`
		UsageHints []string `json:"usage_hints"`
	}{
		RobotEnvelope: NewRobotEnvelope(rc.dataHash),
		Alerts:        driftResult.Alerts,
		UsageHints: []string{
			"--severity=warning --alert-type=stale_issue   # stale warnings only",
			"--alert-type=blocking_cascade                 # high-unblock opportunities",
			"jq '.alerts | map(.issue_id)'                # list impacted issues",
		},
	}
	for _, a := range driftResult.Alerts {
		switch a.Severity {
		case drift.SeverityCritical:
			output.Summary.Critical++
		case drift.SeverityWarning:
			output.Summary.Warning++
		case drift.SeverityInfo:
			output.Summary.Info++
		}
		output.Summary.Total++
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding alerts: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
