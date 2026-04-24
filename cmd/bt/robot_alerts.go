package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/baseline"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
)

// runAlerts handles --robot-alerts (drift + proactive).
//
// Per bt-46p6.8 / bt-7l5m: alerts are always computed at project scope. In
// --global mode, the output is the union of per-project alerts, each carrying
// a SourceProject field. No global-aggregate density/PR/etc. is computed.
func (rc *robotCtx) runAlerts(alertSeverity, alertType, alertLabel string) {
	driftConfig, err := drift.LoadConfig(rc.projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading drift config: %v\n", err)
		os.Exit(1)
	}

	alerts := drift.ProjectAlerts(
		rc.analysisIssues(),
		flagGlobal,
		rc.repoName,
		driftConfig,
		rc.baselineLoader(),
	)

	// Apply optional filters
	filtered := alerts[:0]
	for _, a := range alerts {
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
	alerts = filtered

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
		Alerts:        alerts,
		UsageHints: []string{
			"--severity=warning --alert-type=stale         # stale warnings only",
			"--alert-type=high_leverage                    # high-unblock opportunities",
			"jq '.alerts | group_by(.source_project)'      # bucket by project (global mode)",
			"jq '.alerts | map(.issue_id)'                # list impacted issues",
		},
	}
	for _, a := range alerts {
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

// baselineLoader returns a per-project baseline loader suitable for
// drift.ProjectAlerts.
//
// Commit 1 of bt-46p6.8: baseline is still a single .bt/baseline.json. In
// single-project mode, the loader returns that baseline for the one project
// key. In --global mode, the loader returns nil for every project — we don't
// yet know which project the snapshot was taken for, so applying it to every
// project would fire false drift alerts. Commit 2 introduces per-project
// baseline sections and lifts this restriction.
func (rc *robotCtx) baselineLoader() func(project string) *baseline.Baseline {
	if flagGlobal {
		return func(string) *baseline.Baseline { return nil }
	}
	path := baseline.DefaultPath(rc.projectDir)
	if !baseline.Exists(path) {
		return nil
	}
	loaded, err := baseline.Load(path)
	if err != nil {
		if os.Getenv("BT_ROBOT") != "1" {
			fmt.Fprintf(os.Stderr, "Warning: Error loading baseline: %v\n", err)
		}
		return nil
	}
	return func(string) *baseline.Baseline { return loaded }
}
