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

// runDescribeAlertTypes emits the full alert-type taxonomy with plain-English
// definitions as JSON. Single source of truth lives in pkg/drift; this is the
// CLI surface for bt-46p6.17. Schema is intentionally flat so jq filters
// stay obvious (e.g. `bt --robot-alerts --describe-types | jq '.types[]'`).
func (rc *robotCtx) runDescribeAlertTypes() {
	type entry struct {
		Type       string `json:"type"`
		Definition string `json:"definition"`
	}
	types := drift.AllAlertTypes()
	entries := make([]entry, 0, len(types))
	for _, at := range types {
		entries = append(entries, entry{
			Type:       string(at),
			Definition: drift.AlertTypeDefinition(at),
		})
	}
	output := struct {
		RobotEnvelope
		Types      []entry  `json:"types"`
		UsageHints []string `json:"usage_hints"`
	}{
		RobotEnvelope: NewRobotEnvelope(rc.dataHash),
		Types:         entries,
		UsageHints: []string{
			"--alert-type=<type>                            # filter alerts by type",
			"jq '.types[] | select(.type==\"stale\")'         # look up one definition",
		},
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding alert-type definitions: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// alertTypeFilterHelp builds the --alert-type help string by enumerating
// every registered alert type with its short definition. Keeps cobra's help
// in sync with pkg/drift without duplicating the strings here.
func alertTypeFilterHelp() string {
	var sb strings.Builder
	sb.WriteString("Filter by alert type. Values:\n")
	for _, at := range drift.AllAlertTypes() {
		sb.WriteString(fmt.Sprintf("  %s — %s\n", string(at), drift.AlertTypeDefinition(at)))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// baselineLoader returns a per-project baseline-section loader suitable for
// drift.ProjectAlerts.
//
// With schema v2 (bt-46p6.8 commit 2), the .bt/baseline.json file holds a
// Projects map keyed by SourceRepo. The loader returns the stored section
// for the requested project, or nil when the baseline has no entry for it
// (new projects added after the snapshot was taken).
func (rc *robotCtx) baselineLoader() func(project string) *baseline.ProjectSection {
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
	return loaded.Project
}
