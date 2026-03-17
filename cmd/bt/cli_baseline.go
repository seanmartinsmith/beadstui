package main

import (
	"fmt"
	"os"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/baseline"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// runSaveBaseline handles --save-baseline.
func (rc *robotCtx) runSaveBaseline(description string, forceFullAnalysis bool) {
	analyzer := analysis.NewAnalyzer(rc.issues)
	if forceFullAnalysis {
		cfg := analysis.FullAnalysisConfig()
		analyzer.SetConfig(&cfg)
	}
	stats := analyzer.Analyze()

	// Compute status counts from issues
	openCount, closedCount, blockedCount := 0, 0, 0
	for _, issue := range rc.issues {
		switch issue.Status {
		case model.StatusOpen, model.StatusInProgress:
			openCount++
		case model.StatusClosed:
			closedCount++
		case model.StatusBlocked:
			blockedCount++
		}
	}

	// Get actionable count from analyzer
	actionableCount := len(analyzer.GetActionableIssues())

	// Get cycles (method returns a copy)
	cycles := stats.Cycles()

	// Build GraphStats from analysis
	graphStats := baseline.GraphStats{
		NodeCount:       stats.NodeCount,
		EdgeCount:       stats.EdgeCount,
		Density:         stats.Density,
		OpenCount:       openCount,
		ClosedCount:     closedCount,
		BlockedCount:    blockedCount,
		CycleCount:      len(cycles),
		ActionableCount: actionableCount,
	}

	// Build TopMetrics from analysis (top 10 for each)
	topMetrics := baseline.TopMetrics{
		PageRank:     buildMetricItems(stats.PageRank(), 10),
		Betweenness:  buildMetricItems(stats.Betweenness(), 10),
		CriticalPath: buildMetricItems(stats.CriticalPathScore(), 10),
		Hubs:         buildMetricItems(stats.Hubs(), 10),
		Authorities:  buildMetricItems(stats.Authorities(), 10),
	}

	bl := baseline.New(graphStats, topMetrics, cycles, description)

	baselinePath := baseline.DefaultPath(rc.projectDir)
	if err := bl.Save(baselinePath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving baseline: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Baseline saved to %s\n", baselinePath)
	fmt.Print(bl.Summary())
	os.Exit(0)
}

// runCheckDrift handles --check-drift and --robot-drift.
func (rc *robotCtx) runCheckDrift(robotDriftCheck, forceFullAnalysis bool) {
	baselinePath := baseline.DefaultPath(rc.projectDir)
	if !baseline.Exists(baselinePath) {
		fmt.Fprintln(os.Stderr, "Error: No baseline found.")
		fmt.Fprintln(os.Stderr, "Create one with: bt --save-baseline \"description\"")
		os.Exit(1)
	}

	bl, err := baseline.Load(baselinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading baseline: %v\n", err)
		os.Exit(1)
	}

	// Run analysis on current issues
	analyzer := analysis.NewAnalyzer(rc.issues)
	if forceFullAnalysis {
		cfg := analysis.FullAnalysisConfig()
		analyzer.SetConfig(&cfg)
	}
	stats := analyzer.Analyze()

	// Compute status counts from issues
	openCount, closedCount, blockedCount := 0, 0, 0
	for _, issue := range rc.issues {
		switch issue.Status {
		case model.StatusOpen, model.StatusInProgress:
			openCount++
		case model.StatusClosed:
			closedCount++
		case model.StatusBlocked:
			blockedCount++
		}
	}
	actionableCount := len(analyzer.GetActionableIssues())
	cycles := stats.Cycles()

	// Build current snapshot as baseline for comparison
	currentStats := baseline.GraphStats{
		NodeCount:       stats.NodeCount,
		EdgeCount:       stats.EdgeCount,
		Density:         stats.Density,
		OpenCount:       openCount,
		ClosedCount:     closedCount,
		BlockedCount:    blockedCount,
		CycleCount:      len(cycles),
		ActionableCount: actionableCount,
	}
	currentMetrics := baseline.TopMetrics{
		PageRank:     buildMetricItems(stats.PageRank(), 10),
		Betweenness:  buildMetricItems(stats.Betweenness(), 10),
		CriticalPath: buildMetricItems(stats.CriticalPathScore(), 10),
		Hubs:         buildMetricItems(stats.Hubs(), 10),
		Authorities:  buildMetricItems(stats.Authorities(), 10),
	}
	current := baseline.New(currentStats, currentMetrics, cycles, "current")

	// Load drift config and run calculator
	envRobot := os.Getenv("BT_ROBOT") == "1"
	driftConfig, err := drift.LoadConfig(rc.projectDir)
	if err != nil {
		if !envRobot {
			fmt.Fprintf(os.Stderr, "Warning: Error loading drift config: %v\n", err)
		}
		driftConfig = drift.DefaultConfig()
	}

	calc := drift.NewCalculator(bl, current, driftConfig)
	result := calc.Calculate()

	if robotDriftCheck {
		// JSON output
		output := struct {
			GeneratedAt string `json:"generated_at"`
			HasDrift    bool   `json:"has_drift"`
			ExitCode    int    `json:"exit_code"`
			Summary     struct {
				Critical int `json:"critical"`
				Warning  int `json:"warning"`
				Info     int `json:"info"`
			} `json:"summary"`
			Alerts   []drift.Alert `json:"alerts"`
			Baseline struct {
				CreatedAt string `json:"created_at"`
				CommitSHA string `json:"commit_sha,omitempty"`
			} `json:"baseline"`
		}{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			HasDrift:    result.HasDrift,
			ExitCode:    result.ExitCode(),
			Alerts:      result.Alerts,
		}
		output.Summary.Critical = result.CriticalCount
		output.Summary.Warning = result.WarningCount
		output.Summary.Info = result.InfoCount
		output.Baseline.CreatedAt = bl.CreatedAt.Format(time.RFC3339)
		output.Baseline.CommitSHA = bl.CommitSHA

		encoder := rc.newEncoder()
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding drift result: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Human-readable output
		fmt.Print(result.Summary())
	}

	os.Exit(result.ExitCode())
}
