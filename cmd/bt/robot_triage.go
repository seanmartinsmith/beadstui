package main

import (
	"fmt"
	"os"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/correlation"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/metrics"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// runTriage handles --robot-triage, --robot-next, --robot-triage-by-track, --robot-triage-by-label.
func (rc *robotCtx) runTriage(robotNext, robotTriageByTrack, robotTriageByLabel bool, historyLimit int, asOf, asOfResolved string) {
	// Attempt to load history for staleness analysis
	// We use a best-effort approach here - if history isn't available or fails,
	// we just proceed without staleness data.
	var historyReport *correlation.HistoryReport

	// bv-perf: Skip history loading if no open issues exist
	// ComputeStaleness only processes open issues, so loading git history
	// is wasted work when all issues are closed.
	hasOpenIssues := false
	for _, issue := range rc.issues {
		if issue.Status != model.StatusClosed && issue.Status != model.StatusTombstone {
			hasOpenIssues = true
			break
		}
	}

	if hasOpenIssues {
		if cwd, err := os.Getwd(); err == nil {
			if beadsDir, err := loader.GetBeadsDir(""); err == nil {
				if beadsPath, err := loader.FindJSONLPath(beadsDir); err == nil {
					// Use a smaller limit for triage to keep it fast, unless overridden
					limit := historyLimit
					if limit == 500 { // If default
						limit = 200 // Use smaller default for triage
					}

					// Validate repo first
					if correlation.ValidateRepository(cwd) == nil {
						beadInfos := make([]correlation.BeadInfo, len(rc.issues))
						for i, issue := range rc.issues {
							beadInfos[i] = correlation.BeadInfo{
								ID:     issue.ID,
								Title:  issue.Title,
								Status: string(issue.Status),
							}
						}

						correlator := correlation.NewCorrelator(cwd, beadsPath)
						opts := correlation.CorrelatorOptions{Limit: limit}

						// Swallow errors for triage flow - staleness is optional
						if report, err := correlator.GenerateReport(beadInfos, opts); err == nil {
							historyReport = report
						}
					}
				}
			}
		}
	}

	// bv-87: Support track/label-aware grouping for multi-agent coordination
	opts := analysis.TriageOptions{
		GroupByTrack:  robotTriageByTrack,
		GroupByLabel:  robotTriageByLabel,
		WaitForPhase2: true, // Triage needs full graph metrics
		UseFastConfig: true, // Use minimal Phase 2 config for robot mode (bv-t1js)
		History:       historyReport,
	}
	triage := analysis.ComputeTriageWithOptions(rc.issues, opts)

	// bv-90: Load feedback data for output
	var feedbackInfo *analysis.FeedbackJSON
	if robotTriageBeadsDir, err := loader.GetBeadsDir(""); err == nil {
		if feedbackData, err := analysis.LoadFeedback(robotTriageBeadsDir); err == nil && len(feedbackData.Events) > 0 {
			info := feedbackData.ToJSON()
			feedbackInfo = &info
		}
	}

	if robotNext {
		// Minimal output: just the top pick
		envelope := NewRobotEnvelope(rc.dataHash)
		if len(triage.QuickRef.TopPicks) == 0 {
			output := struct {
				RobotEnvelope
				AsOf       string `json:"as_of,omitempty"`
				AsOfCommit string `json:"as_of_commit,omitempty"`
				Message    string `json:"message"`
			}{
				RobotEnvelope: envelope,
				AsOf:          asOf,
				AsOfCommit:    asOfResolved,
				Message:       "No actionable items available",
			}
			encoder := rc.newEncoder()
			if err := encoder.Encode(output); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding robot-next: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		top := triage.QuickRef.TopPicks[0]
		output := struct {
			RobotEnvelope
			AsOf       string   `json:"as_of,omitempty"`
			AsOfCommit string   `json:"as_of_commit,omitempty"`
			ID         string   `json:"id"`
			Title      string   `json:"title"`
			Score      float64  `json:"score"`
			Reasons    []string `json:"reasons"`
			Unblocks   int      `json:"unblocks"`
			ClaimCmd   string   `json:"claim_command"`
			ShowCmd    string   `json:"show_command"`
		}{
			RobotEnvelope: envelope,
			AsOf:          asOf,
			AsOfCommit:    asOfResolved,
			ID:            top.ID,
			Title:         top.Title,
			Score:         top.Score,
			Reasons:       top.Reasons,
			Unblocks:      top.Unblocks,
			ClaimCmd:      fmt.Sprintf("bd update %s --status=in_progress", top.ID),
			ShowCmd:       fmt.Sprintf("bd show %s", top.ID),
		}

		encoder := rc.newEncoder()
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding robot-next: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Full triage output with usage hints
	output := struct {
		GeneratedAt string                 `json:"generated_at"`
		DataHash    string                 `json:"data_hash"`
		AsOf        string                 `json:"as_of,omitempty"`        // Historical snapshot ref (e.g., HEAD~30)
		AsOfCommit  string                 `json:"as_of_commit,omitempty"` // Resolved commit SHA
		Triage      analysis.TriageResult  `json:"triage"`
		Feedback    *analysis.FeedbackJSON `json:"feedback,omitempty"` // bv-90: Feedback loop state
		UsageHints  []string               `json:"usage_hints"`        // bv-84: Agent-friendly hints
	}{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		DataHash:    rc.dataHash,
		AsOf:        asOf,
		AsOfCommit:  asOfResolved,
		Triage:      triage,
		Feedback:    feedbackInfo,
		UsageHints: []string{
			"jq '.triage.quick_ref.top_picks[:3]' - Top 3 picks for immediate work",
			"jq '.triage.recommendations[3:10] | map({id,title,score})' - Next candidates after top picks",
			"jq '.triage.blockers_to_clear | map(.id)' - High-impact blockers to clear",
			"jq '.triage.recommendations[] | select(.type == \"bug\")' - Bug-focused recommendations",
			"jq '.triage.quick_ref.top_picks[] | select(.unblocks > 2)' - High-impact picks",
			"jq '.triage.quick_wins' - Low-effort, high-impact items",
			"--robot-next - Get only the single top recommendation",
			"--robot-triage-by-track - Group by execution track for multi-agent coordination",
			"--robot-triage-by-label - Group by label for area-focused agents",
			"jq '.triage.recommendations_by_track[].top_pick' - Top pick per track",
			"jq '.triage.recommendations_by_label[].claim_command' - Claim commands per label",
			"jq '.feedback.weight_adjustments' - View feedback-adjusted weights (bv-90)",
		},
	}
	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding robot-triage: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runSuggest handles --robot-suggest (bv-180).
func (rc *robotCtx) runSuggest(suggestType string, suggestConfidence float64, suggestBead string) {
	config := analysis.DefaultSuggestAllConfig()
	config.MinConfidence = suggestConfidence
	config.FilterBead = suggestBead

	// Parse filter type
	switch suggestType {
	case "duplicate", "duplicates":
		config.FilterType = analysis.SuggestionPotentialDuplicate
	case "dependency", "dependencies":
		config.FilterType = analysis.SuggestionMissingDependency
	case "label", "labels":
		config.FilterType = analysis.SuggestionLabelSuggestion
	case "cycle", "cycles":
		config.FilterType = analysis.SuggestionCycleWarning
	case "":
		// All types
	default:
		fmt.Fprintf(os.Stderr, "Invalid suggest-type: %s (use: duplicate, dependency, label, cycle)\n", suggestType)
		os.Exit(1)
	}

	output := analysis.GenerateRobotSuggestOutput(rc.issues, config, rc.dataHash)

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding suggestions: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runMetrics handles --robot-metrics (bv-84tp).
func (rc *robotCtx) runMetrics() {
	output := metrics.GetAllMetrics()
	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding metrics: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
