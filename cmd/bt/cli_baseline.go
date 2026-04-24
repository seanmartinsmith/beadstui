package main

import (
	"fmt"
	"os"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/baseline"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
)

// runSaveBaseline handles --save-baseline.
//
// Schema v2 (bt-46p6.8): captures one ProjectSection per project. In --global
// mode, partitions by SourceRepo; in single-project mode, writes a single
// section keyed by rc.repoName.
//
// forceFullAnalysis is accepted for API continuity but no longer controls
// individual per-project analyzers — each project's Analyzer runs its
// default pipeline because the analyzers are instantiated inside
// drift.SnapshotProjects. If forcing full analysis per project becomes
// important again, plumb a config option into SnapshotProjects.
func (rc *robotCtx) runSaveBaseline(description string, forceFullAnalysis bool) {
	_ = forceFullAnalysis

	projects := drift.SnapshotProjects(rc.analysisIssues(), flagGlobal, rc.repoName)
	if len(projects) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no issues to snapshot")
		os.Exit(1)
	}

	bl := baseline.New(projects, description)

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
//
// Schema v2 (bt-46p6.8): drift detection runs per-project through
// drift.ProjectAlerts using the baseline's per-project sections. Projects
// present in the current data but missing from the baseline emit only
// proactive alerts (staleness, cascade, abandoned, new cycles) since no
// drift-delta baseline exists for them.
func (rc *robotCtx) runCheckDrift(robotDriftCheck, forceFullAnalysis bool) {
	_ = forceFullAnalysis

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

	envRobot := os.Getenv("BT_ROBOT") == "1"
	driftConfig, err := drift.LoadConfig(rc.projectDir)
	if err != nil {
		if !envRobot {
			fmt.Fprintf(os.Stderr, "Warning: Error loading drift config: %v\n", err)
		}
		driftConfig = drift.DefaultConfig()
	}

	alerts := drift.ProjectAlerts(
		rc.analysisIssues(),
		flagGlobal,
		rc.repoName,
		driftConfig,
		bl.Project,
	)

	critical, warning, info := 0, 0, 0
	for _, a := range alerts {
		switch a.Severity {
		case drift.SeverityCritical:
			critical++
		case drift.SeverityWarning:
			warning++
		case drift.SeverityInfo:
			info++
		}
	}

	exit := 0
	switch {
	case critical > 0:
		exit = 1
	case warning > 0:
		exit = 2
	}

	if robotDriftCheck {
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
			HasDrift:    len(alerts) > 0,
			ExitCode:    exit,
			Alerts:      alerts,
		}
		output.Summary.Critical = critical
		output.Summary.Warning = warning
		output.Summary.Info = info
		output.Baseline.CreatedAt = bl.CreatedAt.Format(time.RFC3339)
		output.Baseline.CommitSHA = bl.CommitSHA

		encoder := rc.newEncoder()
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding drift result: %v\n", err)
			os.Exit(1)
		}
	} else {
		if len(alerts) == 0 {
			fmt.Print("No drift detected. Project metrics are within baseline thresholds.\n")
		} else {
			fmt.Printf("Drift: %d critical, %d warning, %d info\n\n", critical, warning, info)
			for _, a := range alerts {
				proj := a.SourceProject
				if proj == "" {
					proj = "?"
				}
				fmt.Printf("  [%s] [%s] %s — %s\n", a.Severity, proj, a.Type, a.Message)
				for _, d := range a.Details {
					fmt.Printf("      - %s\n", d)
				}
			}
			fmt.Println()
		}
	}

	os.Exit(exit)
}
