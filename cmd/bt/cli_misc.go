package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/export"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/version"
	"github.com/seanmartinsmith/beadstui/pkg/view"
)

// runFeedback handles --feedback-accept, --feedback-ignore, --feedback-reset, --feedback-show (bv-90).
func runFeedback(feedbackAccept, feedbackIgnore string, feedbackReset, feedbackShow bool) {
	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting beads directory: %v\n", err)
		os.Exit(1)
	}

	feedback, err := analysis.LoadFeedback(beadsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading feedback: %v\n", err)
		os.Exit(1)
	}

	if feedbackReset {
		feedback.Reset()
		if err := feedback.Save(beadsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving feedback: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Feedback data reset to defaults.")
		os.Exit(0)
	}

	if feedbackShow {
		feedbackJSON := feedback.ToJSON()
		data, _ := json.MarshalIndent(feedbackJSON, "", "  ")
		fmt.Println(string(data))
		os.Exit(0)
	}

	// For accept/ignore, we need to get the issue's score breakdown
	if feedbackAccept != "" || feedbackIgnore != "" {
		issueID := feedbackAccept
		action := "accept"
		if feedbackIgnore != "" {
			issueID = feedbackIgnore
			action = "ignore"
		}

		// Load issues to get score breakdown
		issues, err := datasource.LoadIssues("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading issues: %v\n", err)
			os.Exit(1)
		}

		// Find the issue
		var foundIssue *model.Issue
		for i := range issues {
			if issues[i].ID == issueID {
				foundIssue = &issues[i]
				break
			}
		}

		if foundIssue == nil {
			fmt.Fprintf(os.Stderr, "Issue not found: %s\n", issueID)
			os.Exit(1)
		}

		// Compute impact score for the issue to get breakdown
		an := analysis.NewAnalyzer(issues)
		scores := an.ComputeImpactScores()

		var score float64
		var breakdown analysis.ScoreBreakdown
		for _, s := range scores {
			if s.IssueID == issueID {
				score = s.Score
				breakdown = s.Breakdown
				break
			}
		}

		if err := feedback.RecordFeedback(issueID, action, score, breakdown); err != nil {
			fmt.Fprintf(os.Stderr, "Error recording feedback: %v\n", err)
			os.Exit(1)
		}

		if err := feedback.Save(beadsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving feedback: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Recorded %s feedback for %s (score: %.3f)\n", action, issueID, score)
		fmt.Println(feedback.Summary())
		os.Exit(0)
	}
}

// runPriorityBrief handles --priority-brief (bv-96).
func (rc *robotCtx) runPriorityBrief(outputPath string) {
	fmt.Printf("Generating priority brief to %s...\n", outputPath)
	triage := analysis.ComputeTriage(rc.issues)

	// Marshal triage to JSON for the export function
	triageJSON, err := json.Marshal(triage)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling triage data: %v\n", err)
		os.Exit(1)
	}

	// Generate the brief
	config := export.DefaultPriorityBriefConfig()
	config.DataHash = rc.dataHash
	brief, err := export.GeneratePriorityBriefFromTriageJSON(triageJSON, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating priority brief: %v\n", err)
		os.Exit(1)
	}

	// Write to file
	if err := os.WriteFile(outputPath, []byte(brief), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing priority brief: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Done! Priority brief saved to %s\n", outputPath)
	os.Exit(0)
}

// runAgentBrief handles --agent-brief (bv-131).
func (rc *robotCtx) runAgentBrief(outputDir string) {
	fmt.Printf("Generating agent brief bundle to %s/...\n", outputDir)

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
		os.Exit(1)
	}

	// Generate triage data
	triage := analysis.ComputeTriage(rc.issues)
	triageJSON, err := json.MarshalIndent(triage, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling triage: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "triage.json"), triageJSON, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing triage.json: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  → triage.json")

	// Generate insights
	analyzer := analysis.NewAnalyzer(rc.analysisIssues())
	stats := analyzer.Analyze()
	insights := stats.GenerateInsights(50)
	insightsJSON, err := json.MarshalIndent(insights, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling insights: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "insights.json"), insightsJSON, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing insights.json: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  → insights.json")

	// Generate priority brief
	config := export.DefaultPriorityBriefConfig()
	config.DataHash = rc.dataHash
	brief, err := export.GeneratePriorityBriefFromTriageJSON(triageJSON, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating brief: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "brief.md"), []byte(brief), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing brief.md: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  → brief.md")

	// Generate jq helpers
	helpers := generateJQHelpers()
	if err := os.WriteFile(filepath.Join(outputDir, "helpers.md"), []byte(helpers), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing helpers.md: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  → helpers.md")

	// Generate meta.json with hash and config
	meta := struct {
		GeneratedAt string   `json:"generated_at"`
		DataHash    string   `json:"data_hash"`
		IssueCount  int      `json:"issue_count"`
		Version     string   `json:"version"`
		Files       []string `json:"files"`
	}{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		DataHash:    rc.dataHash,
		IssueCount:  len(rc.issues),
		Version:     version.Version,
		Files:       []string{"triage.json", "insights.json", "brief.md", "helpers.md", "meta.json"},
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(outputDir, "meta.json"), metaJSON, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing meta.json: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  → meta.json")

	fmt.Printf("\nDone! Agent brief bundle saved to %s/\n", outputDir)
	os.Exit(0)
}

// runEmitScript handles --emit-script (bv-89).
func (rc *robotCtx) runEmitScript(scriptLimit int, scriptFormat string) {
	triage := analysis.ComputeTriage(rc.issues)

	// Determine script limit
	limit := scriptLimit
	if limit <= 0 {
		limit = 5
	}

	// Collect top recommendations
	recs := triage.Recommendations
	if len(recs) > limit {
		recs = recs[:limit]
	}

	// Build script header with hash/config
	var sb strings.Builder
	switch scriptFormat {
	case "fish":
		sb.WriteString("#!/usr/bin/env fish\n")
	case "zsh":
		sb.WriteString("#!/usr/bin/env zsh\n")
	default:
		sb.WriteString("#!/usr/bin/env bash\n")
		sb.WriteString("set -euo pipefail\n")
	}

	sb.WriteString(fmt.Sprintf("# Generated by bt --emit-script at %s\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("# Data hash: %s\n", rc.dataHash))
	sb.WriteString(fmt.Sprintf("# Top %d recommendations from %d actionable items\n", len(recs), len(triage.Recommendations)))
	sb.WriteString("#\n")
	sb.WriteString("# Usage: source this script or run it directly\n")
	sb.WriteString("# Each command will claim and show the recommended issue\n")
	sb.WriteString("#\n\n")

	if len(recs) == 0 {
		sb.WriteString("echo 'No actionable recommendations available'\n")
		sb.WriteString("exit 0\n")
	} else {
		// Generate commands for each recommendation
		for i, rec := range recs {
			sb.WriteString(fmt.Sprintf("# %d. %s (score: %.3f)\n", i+1, rec.Title, rec.Score))
			if len(rec.Reasons) > 0 {
				sb.WriteString(fmt.Sprintf("#    Reason: %s\n", rec.Reasons[0]))
			}
			if len(rec.UnblocksIDs) > 0 {
				sb.WriteString(fmt.Sprintf("#    Unblocks: %d downstream items\n", len(rec.UnblocksIDs)))
			}

			// Claim command
			sb.WriteString(fmt.Sprintf("# To claim: bd update %s --status=in_progress\n", rec.ID))
			// Show command
			sb.WriteString(fmt.Sprintf("bd show %s\n", rec.ID))
			sb.WriteString("\n")
		}

		// Add summary section
		sb.WriteString("# === Quick Actions ===\n")
		sb.WriteString("# To claim the top pick:\n")
		if len(recs) > 0 {
			sb.WriteString(fmt.Sprintf("# bd update %s --status=in_progress\n", recs[0].ID))
		}
		sb.WriteString("#\n")
		sb.WriteString("# To claim all listed items (uncomment to enable):\n")
		for _, rec := range recs {
			sb.WriteString(fmt.Sprintf("# bd update %s --status=in_progress\n", rec.ID))
		}
	}

	fmt.Print(sb.String())
	os.Exit(0)
}

// compactDiffOutput mirrors analysis.SnapshotDiff on the wire but replaces
// the four issue-slot slices with []view.CompactIssue so large diffs don't
// burn an agent's context on full issue bodies. The remaining fields
// (graph changes, metric deltas, summary, modified issues) carry over
// untouched so agents that jq on structured diffs keep working.
type compactDiffOutput struct {
	FromTimestamp  time.Time                `json:"from_timestamp"`
	ToTimestamp    time.Time                `json:"to_timestamp"`
	FromRevision   string                   `json:"from_revision,omitempty"`
	ToRevision     string                   `json:"to_revision,omitempty"`
	NewIssues      []view.CompactIssue      `json:"new_issues"`
	ClosedIssues   []view.CompactIssue      `json:"closed_issues"`
	RemovedIssues  []view.CompactIssue      `json:"removed_issues"`
	ReopenedIssues []view.CompactIssue      `json:"reopened_issues"`
	ModifiedIssues []analysis.ModifiedIssue `json:"modified_issues"`
	NewCycles      [][]string               `json:"new_cycles"`
	ResolvedCycles [][]string               `json:"resolved_cycles"`
	MetricDeltas   analysis.MetricDeltas    `json:"metric_deltas"`
	Summary        analysis.DiffSummary     `json:"summary"`
}

// unionIssuesByID returns the deduplicated set of issues across two
// snapshots keyed by ID, with the "to" snapshot taking precedence on
// overlap so the compact projection reflects the latest state.
func unionIssuesByID(from, to []model.Issue) []model.Issue {
	out := make([]model.Issue, 0, len(from)+len(to))
	seen := make(map[string]int, len(from)+len(to))
	for _, iss := range from {
		seen[iss.ID] = len(out)
		out = append(out, iss)
	}
	for _, iss := range to {
		if idx, ok := seen[iss.ID]; ok {
			out[idx] = iss
			continue
		}
		seen[iss.ID] = len(out)
		out = append(out, iss)
	}
	return out
}

// compactByID keys a compact projection slice by issue ID for O(1) lookup.
func compactByID(in []view.CompactIssue) map[string]view.CompactIssue {
	out := make(map[string]view.CompactIssue, len(in))
	for _, c := range in {
		out[c.ID] = c
	}
	return out
}

// pickCompact selects the compact projection for each issue in `src`,
// falling back to an on-the-fly single-issue compaction when an ID isn't
// in the pre-computed map (should not happen given unionIssuesByID feeds
// CompactAll, but safe).
func pickCompact(src []model.Issue, byID map[string]view.CompactIssue) []view.CompactIssue {
	if len(src) == 0 {
		return nil
	}
	out := make([]view.CompactIssue, 0, len(src))
	for i := range src {
		if c, ok := byID[src[i].ID]; ok {
			out = append(out, c)
			continue
		}
		// Fallback: compact this one issue alone. Reverse-map counts for
		// this fallback will be zero, which is acceptable for a case that
		// shouldn't occur in practice.
		for _, c := range view.CompactAll([]model.Issue{src[i]}) {
			out = append(out, c)
		}
	}
	return out
}

// compactSnapshotDiff projects the four issue-slot slices on a SnapshotDiff
// while preserving all other fields verbatim.
func compactSnapshotDiff(diff *analysis.SnapshotDiff, byID map[string]view.CompactIssue) compactDiffOutput {
	if diff == nil {
		return compactDiffOutput{}
	}
	return compactDiffOutput{
		FromTimestamp:  diff.FromTimestamp,
		ToTimestamp:    diff.ToTimestamp,
		FromRevision:   diff.FromRevision,
		ToRevision:     diff.ToRevision,
		NewIssues:      pickCompact(diff.NewIssues, byID),
		ClosedIssues:   pickCompact(diff.ClosedIssues, byID),
		RemovedIssues:  pickCompact(diff.RemovedIssues, byID),
		ReopenedIssues: pickCompact(diff.ReopenedIssues, byID),
		ModifiedIssues: diff.ModifiedIssues,
		NewCycles:      diff.NewCycles,
		ResolvedCycles: diff.ResolvedCycles,
		MetricDeltas:   diff.MetricDeltas,
		Summary:        diff.Summary,
	}
}

// runDiffSince handles --diff-since and --robot-diff.
func (rc *robotCtx) runDiffSince(diffSince string, robotDiff bool, asOf, asOfResolved string) {
	envRobot := os.Getenv("BT_ROBOT") == "1"
	stdoutIsTTY := isStdoutTTY()

	// Auto-enable robot diff for non-interactive/agent contexts
	if !robotDiff && (envRobot || !stdoutIsTTY) {
		robotDiff = true
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	gitLoader := loader.NewGitLoader(cwd)

	// Load historical issues
	historicalIssues, err := gitLoader.LoadAt(diffSince)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading issues at %s: %v\n", diffSince, err)
		os.Exit(1)
	}

	// Get revision info for timestamp
	revision, err := gitLoader.ResolveRevision(diffSince)
	if err != nil {
		revision = diffSince
	}

	// Create snapshots
	fromSnapshot := analysis.NewSnapshotAt(historicalIssues, time.Time{}, revision)
	toSnapshot := analysis.NewSnapshot(rc.analysisIssues())

	// Compute diff
	diff := analysis.CompareSnapshots(fromSnapshot, toSnapshot)

	if robotDiff {
		// JSON output
		generated := time.Now().UTC().Format(time.RFC3339)
		fromHash := analysis.ComputeDataHash(historicalIssues)

		if robotOutputShape == robotShapeCompact {
			// Compact mode: project each []Issue slot into []CompactIssue
			// using the UNION of from+to issues as the graph context so
			// reverse-map counts (children, unblocks, is_blocked) stay
			// accurate across snapshots.
			union := unionIssuesByID(historicalIssues, rc.issues)
			compactMap := compactByID(view.CompactAll(union))

			output := struct {
				RobotEnvelope
				GeneratedAt      string            `json:"generated_at"`
				ResolvedRevision string            `json:"resolved_revision"`
				AsOf             string            `json:"as_of,omitempty"`
				AsOfCommit       string            `json:"as_of_commit,omitempty"`
				FromDataHash     string            `json:"from_data_hash"`
				ToDataHash       string            `json:"to_data_hash"`
				Diff             compactDiffOutput `json:"diff"`
			}{
				RobotEnvelope: RobotEnvelope{
					GeneratedAt:  generated,
					DataHash:     rc.dataHash,
					OutputFormat: robotOutputFormat,
					Version:      version.Version,
					Schema:       view.CompactIssueSchemaV1,
				},
				GeneratedAt:      generated,
				ResolvedRevision: revision,
				AsOf:             asOf,
				AsOfCommit:       asOfResolved,
				FromDataHash:     fromHash,
				ToDataHash:       rc.dataHash,
				Diff:             compactSnapshotDiff(diff, compactMap),
			}

			encoder := rc.newEncoder()
			if err := encoder.Encode(output); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding diff: %v\n", err)
				os.Exit(1)
			}
		} else {
			output := struct {
				GeneratedAt      string                 `json:"generated_at"`
				ResolvedRevision string                 `json:"resolved_revision"`
				AsOf             string                 `json:"as_of,omitempty"`        // "to" snapshot ref (if --as-of used)
				AsOfCommit       string                 `json:"as_of_commit,omitempty"` // Resolved commit SHA for "to"
				FromDataHash     string                 `json:"from_data_hash"`
				ToDataHash       string                 `json:"to_data_hash"`
				Diff             *analysis.SnapshotDiff `json:"diff"`
			}{
				GeneratedAt:      generated,
				ResolvedRevision: revision,
				AsOf:             asOf,
				AsOfCommit:       asOfResolved,
				FromDataHash:     fromHash,
				ToDataHash:       rc.dataHash,
				Diff:             diff,
			}

			encoder := rc.newEncoder()
			if err := encoder.Encode(output); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding diff: %v\n", err)
				os.Exit(1)
			}
		}
	} else {
		// Human-readable output
		printDiffSummary(diff, diffSince)
	}
	os.Exit(0)
}

// runRobotGraph handles --robot-graph (bv-136).
func (rc *robotCtx) runRobotGraph(graphFormat, labelScope, graphRoot string, graphDepth int) {
	analyzer := analysis.NewAnalyzer(rc.analysisIssues())
	stats := analyzer.Analyze()

	// Determine format
	var format export.GraphExportFormat
	switch strings.ToLower(graphFormat) {
	case "dot":
		format = export.GraphFormatDOT
	case "mermaid":
		format = export.GraphFormatMermaid
	default:
		format = export.GraphFormatJSON
	}

	config := export.GraphExportConfig{
		Format:   format,
		Label:    labelScope,
		Root:     graphRoot,
		Depth:    graphDepth,
		DataHash: rc.dataHash,
	}

	result, err := export.ExportGraph(rc.issues, &stats, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error exporting graph: %v\n", err)
		os.Exit(1)
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding graph: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runExportGraph handles --export-graph (bv-94) - PNG/SVG/HTML export.
func (rc *robotCtx) runExportGraph(exportGraphPath, labelScope, graphTitle, graphPreset string) {
	analyzer := analysis.NewAnalyzer(rc.analysisIssues())
	stats := analyzer.Analyze()

	// Apply label filter if specified
	exportIssues := rc.issues
	if labelScope != "" {
		var filtered []model.Issue
		for _, iss := range rc.issues {
			for _, lbl := range iss.Labels {
				if strings.EqualFold(lbl, labelScope) {
					filtered = append(filtered, iss)
					break
				}
			}
		}
		exportIssues = filtered
	}

	if len(exportIssues) == 0 {
		fmt.Fprintf(os.Stderr, "No issues to export (check filters)\n")
		os.Exit(1)
	}

	// Get project name from current directory
	cwd, _ := os.Getwd()
	projectName := filepath.Base(cwd)

	// Check if HTML export requested (interactive graph)
	if strings.HasSuffix(strings.ToLower(exportGraphPath), ".html") || exportGraphPath == "html" || exportGraphPath == "interactive" {
		title := graphTitle
		if title == "" {
			title = projectName
		}

		// Compute triage for the graph export
		triageOpts := analysis.TriageOptions{WaitForPhase2: true}
		triage := analysis.ComputeTriageWithOptions(exportIssues, triageOpts)

		opts := export.InteractiveGraphOptions{
			Issues:      exportIssues,
			Stats:       &stats,
			Triage:      &triage,
			Title:       title,
			DataHash:    rc.dataHash,
			Path:        exportGraphPath,
			ProjectName: projectName,
		}
		// Auto-generate filename if just "html" or "interactive"
		if exportGraphPath == "html" || exportGraphPath == "interactive" {
			opts.Path = ""
		}
		outputPath, err := export.GenerateInteractiveGraphHTML(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting interactive graph: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Interactive graph exported to %s (%d nodes, %d edges)\n", outputPath, len(exportIssues), stats.EdgeCount)
		os.Exit(0)
	}

	// Static PNG/SVG export (use .html for better interactive graphs)
	opts := export.GraphSnapshotOptions{
		Path:     exportGraphPath,
		Title:    graphTitle,
		Preset:   graphPreset,
		Issues:   exportIssues,
		Stats:    &stats,
		DataHash: rc.dataHash,
	}

	err := export.SaveGraphSnapshot(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error exporting graph snapshot: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Graph exported to %s (%d nodes) - tip: use .html for interactive graphs\n", exportGraphPath, len(exportIssues))
	os.Exit(0)
}
