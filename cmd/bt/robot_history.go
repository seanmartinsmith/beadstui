package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/correlation"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/recipe"
	"github.com/seanmartinsmith/beadstui/pkg/version"
)

// runHistory handles --robot-history and --bead-history.
func (rc *robotCtx) runHistory(beadHistory, historySince string, historyLimit int, minConfidence float64) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	// Validate repository
	if err := correlation.ValidateRepository(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Resolve beads file path (bv-history fix, respects BEADS_DIR)
	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting beads directory: %v\n", err)
		os.Exit(1)
	}
	beadsPath, err := loader.FindJSONLPath(beadsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding beads file: %v\n", err)
		os.Exit(1)
	}

	// Build correlator options
	opts := correlation.CorrelatorOptions{
		BeadID: beadHistory,
		Limit:  historyLimit,
	}

	// Parse --history-since if provided
	if historySince != "" {
		since, err := recipe.ParseRelativeTime(historySince, time.Now())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing --history-since: %v\n", err)
			os.Exit(1)
		}
		if !since.IsZero() {
			opts.Since = &since
		}
	}

	// Convert issues to BeadInfo for correlator
	beadInfos := make([]correlation.BeadInfo, len(rc.issues))
	for i, issue := range rc.issues {
		beadInfos[i] = correlation.BeadInfo{
			ID:     issue.ID,
			Title:  issue.Title,
			Status: string(issue.Status),
		}
	}

	// Generate report with explicit beads path
	correlator := correlation.NewCorrelator(cwd, beadsPath)
	report, err := correlator.GenerateReport(beadInfos, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating history report: %v\n", err)
		os.Exit(1)
	}

	// Apply confidence filter if specified
	if minConfidence > 0 {
		scorer := correlation.NewScorer()
		report.Histories = scorer.FilterHistoriesByConfidence(report.Histories, minConfidence)

		// Rebuild commit index after filtering
		report.CommitIndex = make(correlation.CommitIndex)
		for beadID, history := range report.Histories {
			for _, commit := range history.Commits {
				report.CommitIndex[commit.SHA] = append(report.CommitIndex[commit.SHA], beadID)
			}
		}

		// Update stats
		report.Stats.BeadsWithCommits = 0
		for _, history := range report.Histories {
			if len(history.Commits) > 0 {
				report.Stats.BeadsWithCommits++
			}
		}
	}

	// Output JSON
	encoder := rc.newEncoder()
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding history report: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runCorrelationAudit handles --robot-explain-correlation, --robot-confirm-correlation,
// --robot-reject-correlation, and --robot-correlation-stats (bv-e1u6).
func (rc *robotCtx) runCorrelationAudit(explainArg, confirmArg, rejectArg string, showStats bool, feedbackBy, feedbackReason string) {
	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting beads directory: %v\n", err)
		os.Exit(1)
	}

	feedbackStore := correlation.NewFeedbackStore(beadsDir)
	if err := feedbackStore.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading feedback: %v\n", err)
		os.Exit(1)
	}

	// Handle --robot-correlation-stats
	if showStats {
		stats := feedbackStore.GetStats()
		encoder := rc.newEncoder()
		if err := encoder.Encode(stats); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding stats: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Parse SHA:beadID format
	parseCorrelationArg := func(arg string) (string, string, error) {
		parts := strings.SplitN(arg, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("expected format: SHA:beadID, got: %s", arg)
		}
		return parts[0], parts[1], nil
	}

	// Handle --robot-explain-correlation
	if explainArg != "" {
		commitSHA, beadID, err := parseCorrelationArg(explainArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Generate history report to find the correlation
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}
		beadsPath, err := loader.FindJSONLPath(beadsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding beads file: %v\n", err)
			os.Exit(1)
		}
		correlator := correlation.NewCorrelator(cwd, beadsPath)

		beadInfos := make([]correlation.BeadInfo, len(rc.issues))
		for i, issue := range rc.issues {
			beadInfos[i] = correlation.BeadInfo{
				ID:     issue.ID,
				Title:  issue.Title,
				Status: string(issue.Status),
			}
		}

		opts := correlation.CorrelatorOptions{BeadID: beadID}
		report, err := correlator.GenerateReport(beadInfos, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
			os.Exit(1)
		}

		// Find the specific commit
		history, ok := report.Histories[beadID]
		if !ok {
			fmt.Fprintf(os.Stderr, "Bead not found: %s\n", beadID)
			os.Exit(1)
		}

		var targetCommit *correlation.CorrelatedCommit
		for i := range history.Commits {
			if strings.HasPrefix(history.Commits[i].SHA, commitSHA) || history.Commits[i].ShortSHA == commitSHA {
				targetCommit = &history.Commits[i]
				break
			}
		}

		if targetCommit == nil {
			fmt.Fprintf(os.Stderr, "Commit %s not found in bead %s correlations\n", commitSHA, beadID)
			os.Exit(1)
		}

		// Generate explanation
		scorer := correlation.NewScorer()
		explanation := scorer.BuildExplanation(*targetCommit, beadID)

		// Check for existing feedback
		if fb, ok := feedbackStore.Get(targetCommit.SHA, beadID); ok {
			explanation.Recommendation = fmt.Sprintf("Already has feedback: %s", fb.Type)
		}

		encoder := rc.newEncoder()
		if err := encoder.Encode(explanation); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding explanation: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Helper to resolve original confidence from history report
	resolveOriginalConf := func(beadsDir string, commitSHA, beadID string) (float64, string) {
		cwd, err := os.Getwd()
		if err != nil {
			return 0, commitSHA
		}
		beadsPath, err := loader.FindJSONLPath(beadsDir)
		if err != nil {
			return 0, commitSHA
		}
		correlator := correlation.NewCorrelator(cwd, beadsPath)

		beadInfos := make([]correlation.BeadInfo, len(rc.issues))
		for i, issue := range rc.issues {
			beadInfos[i] = correlation.BeadInfo{ID: issue.ID, Title: issue.Title, Status: string(issue.Status)}
		}

		opts := correlation.CorrelatorOptions{BeadID: beadID}
		report, err := correlator.GenerateReport(beadInfos, opts)
		if err != nil {
			return 0, commitSHA
		}

		if history, ok := report.Histories[beadID]; ok {
			for _, c := range history.Commits {
				if strings.HasPrefix(c.SHA, commitSHA) || c.ShortSHA == commitSHA {
					return c.Confidence, c.SHA // Use full SHA
				}
			}
		}
		return 0, commitSHA
	}

	// Handle --robot-confirm-correlation
	if confirmArg != "" {
		commitSHA, beadID, err := parseCorrelationArg(confirmArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		by := feedbackBy
		if by == "" {
			by = "cli"
		}

		originalConf, fullSHA := resolveOriginalConf(beadsDir, commitSHA, beadID)
		commitSHA = fullSHA

		if err := feedbackStore.Confirm(commitSHA, beadID, by, originalConf, feedbackReason); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving feedback: %v\n", err)
			os.Exit(1)
		}

		result := map[string]interface{}{
			"status":    "confirmed",
			"commit":    commitSHA,
			"bead":      beadID,
			"by":        by,
			"reason":    feedbackReason,
			"orig_conf": originalConf,
		}
		encoder := rc.newEncoder()
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding result: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --robot-reject-correlation
	if rejectArg != "" {
		commitSHA, beadID, err := parseCorrelationArg(rejectArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		by := feedbackBy
		if by == "" {
			by = "cli"
		}

		originalConf, fullSHA := resolveOriginalConf(beadsDir, commitSHA, beadID)
		commitSHA = fullSHA

		if err := feedbackStore.Reject(commitSHA, beadID, by, originalConf, feedbackReason); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving feedback: %v\n", err)
			os.Exit(1)
		}

		result := map[string]interface{}{
			"status":    "rejected",
			"commit":    commitSHA,
			"bead":      beadID,
			"by":        by,
			"reason":    feedbackReason,
			"orig_conf": originalConf,
		}
		encoder := rc.newEncoder()
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding result: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}

// runOrphans handles --robot-orphans (bv-jdop).
func (rc *robotCtx) runOrphans(historyLimit, orphansMinScore int) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	// Validate repository
	if err := correlation.ValidateRepository(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Get beads path
	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting beads directory: %v\n", err)
		os.Exit(1)
	}
	beadsPath, err := loader.FindJSONLPath(beadsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding beads file: %v\n", err)
		os.Exit(1)
	}

	// Convert issues to BeadInfo
	beadInfos := make([]correlation.BeadInfo, len(rc.issues))
	for i, issue := range rc.issues {
		beadInfos[i] = correlation.BeadInfo{
			ID:     issue.ID,
			Title:  issue.Title,
			Status: string(issue.Status),
		}
	}

	// Generate history report first (to get existing correlations)
	correlator := correlation.NewCorrelator(cwd, beadsPath)
	correlatorOpts := correlation.CorrelatorOptions{
		Limit: historyLimit,
	}

	report, err := correlator.GenerateReport(beadInfos, correlatorOpts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating history report: %v\n", err)
		os.Exit(1)
	}

	// Detect orphans using OrphanDetector
	detector := correlation.NewOrphanDetector(report, cwd)
	extractOpts := correlation.ExtractOptions{
		Limit: historyLimit,
	}
	orphanReport, err := detector.DetectOrphans(extractOpts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detecting orphans: %v\n", err)
		os.Exit(1)
	}

	// Filter by minimum score
	var filteredCandidates []correlation.OrphanCandidate
	for _, candidate := range orphanReport.Candidates {
		if candidate.SuspicionScore >= orphansMinScore {
			filteredCandidates = append(filteredCandidates, candidate)
		}
	}
	orphanReport.Candidates = filteredCandidates

	// Update stats for filtered results
	orphanReport.Stats.CandidateCount = len(filteredCandidates)
	if len(filteredCandidates) > 0 {
		totalSuspicion := 0
		for _, c := range filteredCandidates {
			totalSuspicion += c.SuspicionScore
		}
		orphanReport.Stats.AvgSuspicion = float64(totalSuspicion) / float64(len(filteredCandidates))
	}

	// Wrap orphan report with standard envelope fields
	type OrphanOutputEnvelope struct {
		*correlation.OrphanReport
		OutputFormat string `json:"output_format,omitempty"`
		Version      string `json:"version,omitempty"`
	}
	output := OrphanOutputEnvelope{
		OrphanReport: orphanReport,
		OutputFormat: robotOutputFormat,
		Version:      version.Version,
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding orphan report: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runFileBeads handles --robot-file-beads and --robot-file-hotspots (bv-hmib).
func (rc *robotCtx) runFileBeads(robotFileBeadsPath string, fileHotspots bool, fileBeadsLimit, hotspotsLimit, historyLimit int) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	// Validate repository
	if err := correlation.ValidateRepository(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Resolve beads file path
	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting beads directory: %v\n", err)
		os.Exit(1)
	}
	beadsPath, err := loader.FindJSONLPath(beadsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding beads file: %v\n", err)
		os.Exit(1)
	}

	// Convert issues to BeadInfo for correlator
	beadInfos := make([]correlation.BeadInfo, len(rc.issues))
	for i, issue := range rc.issues {
		beadInfos[i] = correlation.BeadInfo{
			ID:     issue.ID,
			Title:  issue.Title,
			Status: string(issue.Status),
		}
	}

	// Generate history report first
	correlator := correlation.NewCorrelator(cwd, beadsPath)
	report, err := correlator.GenerateReport(beadInfos, correlation.CorrelatorOptions{
		Limit: historyLimit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating history report: %v\n", err)
		os.Exit(1)
	}

	// Create file lookup
	fileLookup := correlation.NewFileLookup(report)

	encoder := rc.newEncoder()

	if fileHotspots {
		// Output hotspots
		type HotspotsOutput struct {
			RobotEnvelope
			Hotspots []correlation.FileHotspot  `json:"hotspots"`
			Stats    correlation.FileIndexStats `json:"stats"`
		}

		hotspots := fileLookup.GetHotspots(hotspotsLimit)
		output := HotspotsOutput{
			RobotEnvelope: NewRobotEnvelope(report.DataHash),
			Hotspots:      hotspots,
			Stats:         fileLookup.GetStats(),
		}

		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding hotspots: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Output file-beads lookup
		result := fileLookup.LookupByFile(robotFileBeadsPath)

		// Limit closed beads if specified
		if len(result.ClosedBeads) > fileBeadsLimit {
			result.ClosedBeads = result.ClosedBeads[:fileBeadsLimit]
		}

		type FileBeadsOutput struct {
			RobotEnvelope
			FilePath    string                      `json:"file_path"`
			TotalBeads  int                         `json:"total_beads"`
			OpenBeads   []correlation.BeadReference `json:"open_beads"`
			ClosedBeads []correlation.BeadReference `json:"closed_beads"`
		}

		output := FileBeadsOutput{
			RobotEnvelope: NewRobotEnvelope(report.DataHash),
			FilePath:      robotFileBeadsPath,
			TotalBeads:    result.TotalBeads,
			OpenBeads:     result.OpenBeads,
			ClosedBeads:   result.ClosedBeads,
		}

		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding file beads: %v\n", err)
			os.Exit(1)
		}
	}
	os.Exit(0)
}

// runImpact handles --robot-impact (bv-19pq).
func (rc *robotCtx) runImpact(robotImpactFiles string, historyLimit int) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	if err := correlation.ValidateRepository(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting beads directory: %v\n", err)
		os.Exit(1)
	}
	beadsPath, err := loader.FindJSONLPath(beadsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding beads file: %v\n", err)
		os.Exit(1)
	}

	beadInfos := make([]correlation.BeadInfo, len(rc.issues))
	for i, issue := range rc.issues {
		beadInfos[i] = correlation.BeadInfo{
			ID:     issue.ID,
			Title:  issue.Title,
			Status: string(issue.Status),
		}
	}

	correlator := correlation.NewCorrelator(cwd, beadsPath)
	report, err := correlator.GenerateReport(beadInfos, correlation.CorrelatorOptions{
		Limit: historyLimit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating history report: %v\n", err)
		os.Exit(1)
	}

	fileLookup := correlation.NewFileLookup(report)
	files := strings.Split(robotImpactFiles, ",")
	for i := range files {
		files[i] = strings.TrimSpace(files[i])
	}

	impactResult := fileLookup.ImpactAnalysis(files)

	type ImpactOutput struct {
		RobotEnvelope
		Files         []string                   `json:"files"`
		RiskLevel     string                     `json:"risk_level"`
		RiskScore     float64                    `json:"risk_score"`
		Summary       string                     `json:"summary"`
		Warnings      []string                   `json:"warnings"`
		AffectedBeads []correlation.AffectedBead `json:"affected_beads"`
	}

	output := ImpactOutput{
		RobotEnvelope: NewRobotEnvelope(report.DataHash),
		Files:         impactResult.Files,
		RiskLevel:     impactResult.RiskLevel,
		RiskScore:     impactResult.RiskScore,
		Summary:       impactResult.Summary,
		Warnings:      impactResult.Warnings,
		AffectedBeads: impactResult.AffectedBeads,
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding impact analysis: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runFileRelations handles --robot-file-relations (bv-7a2f).
func (rc *robotCtx) runFileRelations(filePath string, threshold float64, relationsLimit, historyLimit int) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	if err := correlation.ValidateRepository(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting beads directory: %v\n", err)
		os.Exit(1)
	}
	beadsPath, err := loader.FindJSONLPath(beadsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding beads file: %v\n", err)
		os.Exit(1)
	}

	beadInfos := make([]correlation.BeadInfo, len(rc.issues))
	for i, issue := range rc.issues {
		beadInfos[i] = correlation.BeadInfo{
			ID:     issue.ID,
			Title:  issue.Title,
			Status: string(issue.Status),
		}
	}

	correlator := correlation.NewCorrelator(cwd, beadsPath)
	report, err := correlator.GenerateReport(beadInfos, correlation.CorrelatorOptions{
		Limit: historyLimit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating history report: %v\n", err)
		os.Exit(1)
	}

	fileLookup := correlation.NewFileLookup(report)
	result := fileLookup.GetRelatedFiles(filePath, threshold, relationsLimit)

	type RelationsOutput struct {
		RobotEnvelope
		FilePath     string                      `json:"file_path"`
		TotalCommits int                         `json:"total_commits"`
		Threshold    float64                     `json:"threshold"`
		RelatedFiles []correlation.CoChangeEntry `json:"related_files"`
	}

	output := RelationsOutput{
		RobotEnvelope: NewRobotEnvelope(report.DataHash),
		FilePath:      result.FilePath,
		TotalCommits:  result.TotalCommits,
		Threshold:     result.Threshold,
		RelatedFiles:  result.RelatedFiles,
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding file relations: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runRelatedWork handles --robot-related (bv-jtdl).
func (rc *robotCtx) runRelatedWork(beadID string, relatedMinRelevance, relatedMaxResults, historyLimit int, includeClosed bool) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	if err := correlation.ValidateRepository(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting beads directory: %v\n", err)
		os.Exit(1)
	}
	beadsPath, err := loader.FindJSONLPath(beadsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding beads file: %v\n", err)
		os.Exit(1)
	}

	beadInfos := make([]correlation.BeadInfo, len(rc.issues))
	for i, issue := range rc.issues {
		beadInfos[i] = correlation.BeadInfo{
			ID:     issue.ID,
			Title:  issue.Title,
			Status: string(issue.Status),
		}
	}

	correlatorObj := correlation.NewCorrelator(cwd, beadsPath)
	report, err := correlatorObj.GenerateReport(beadInfos, correlation.CorrelatorOptions{
		Limit: historyLimit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating history report: %v\n", err)
		os.Exit(1)
	}

	// Build dependency graph from issues
	depGraph := make(map[string][]string)
	for _, issue := range rc.issues {
		for _, dep := range issue.Dependencies {
			depGraph[issue.ID] = append(depGraph[issue.ID], dep.DependsOnID)
		}
	}

	// Configure options
	opts := correlation.RelatedWorkOptions{
		MinRelevance:      relatedMinRelevance,
		MaxResults:        relatedMaxResults,
		ConcurrencyWindow: 7 * 24 * time.Hour,
		IncludeClosed:     includeClosed,
		DependencyGraph:   depGraph,
	}

	result := report.FindRelatedWork(beadID, opts)
	if result == nil {
		fmt.Fprintf(os.Stderr, "Bead not found in history: %s\n", beadID)
		os.Exit(1)
	}

	// Add envelope fields to output
	type RelatedWorkOutput struct {
		*correlation.RelatedWorkResult
		DataHash     string `json:"data_hash"`
		OutputFormat string `json:"output_format,omitempty"`
		Version      string `json:"version,omitempty"`
	}

	output := RelatedWorkOutput{
		RelatedWorkResult: result,
		DataHash:          report.DataHash,
		OutputFormat:      robotOutputFormat,
		Version:           version.Version,
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding related work: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runBlockerChain handles --robot-blocker-chain (bv-nlo0).
func (rc *robotCtx) runBlockerChain(issueID string) {
	result := rc.analyzer.GetBlockerChain(issueID)

	if result == nil {
		fmt.Fprintf(os.Stderr, "Issue not found: %s\n", issueID)
		os.Exit(1)
	}

	type BlockerChainOutput struct {
		RobotEnvelope
		Result *analysis.BlockerChainResult `json:"result"`
	}

	// Compute data hash for consistency
	dataHash := rc.dataHash

	output := BlockerChainOutput{
		RobotEnvelope: NewRobotEnvelope(dataHash),
		Result:        result,
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding blocker chain: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runImpactNetwork handles --robot-impact-network (bv-48kr).
func (rc *robotCtx) runImpactNetwork(networkArg string, networkDepth, historyLimit int) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	// Find beads path
	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting beads directory: %v\n", err)
		os.Exit(1)
	}
	beadsPath, err := loader.FindJSONLPath(beadsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding beads file: %v\n", err)
		os.Exit(1)
	}

	// Convert to BeadInfo slice
	beadInfos := make([]correlation.BeadInfo, len(rc.issues))
	for i, issue := range rc.issues {
		beadInfos[i] = correlation.BeadInfo{
			ID:     issue.ID,
			Title:  issue.Title,
			Status: string(issue.Status),
		}
	}

	// Generate history report
	correlator := correlation.NewCorrelator(cwd, beadsPath)
	report, err := correlator.GenerateReport(beadInfos, correlation.CorrelatorOptions{
		Limit: historyLimit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating history report: %v\n", err)
		os.Exit(1)
	}

	// Build impact network
	builder := correlation.NewNetworkBuilderWithIssues(report, rc.issues)
	network := builder.Build()

	// Determine if specific bead or full network
	beadID := ""
	if networkArg != "all" {
		beadID = networkArg
	}

	// Cap depth to reasonable range
	depth := networkDepth
	if depth < 1 {
		depth = 1
	}
	if depth > 3 {
		depth = 3
	}

	// Generate result and wrap with envelope fields
	result := network.ToResult(beadID, depth)

	type ImpactNetworkEnvelope struct {
		*correlation.ImpactNetworkResult
		OutputFormat string `json:"output_format,omitempty"`
		Version      string `json:"version,omitempty"`
	}
	output := ImpactNetworkEnvelope{
		ImpactNetworkResult: result,
		OutputFormat:        robotOutputFormat,
		Version:             version.Version,
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding impact network: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runCausality handles --robot-causality (bv-j74w).
func (rc *robotCtx) runCausality(beadID string, historyLimit int) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	if err := correlation.ValidateRepository(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting beads directory: %v\n", err)
		os.Exit(1)
	}
	beadsPath, err := loader.FindJSONLPath(beadsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding beads file: %v\n", err)
		os.Exit(1)
	}

	beadInfos := make([]correlation.BeadInfo, len(rc.issues))
	for i, issue := range rc.issues {
		beadInfos[i] = correlation.BeadInfo{
			ID:     issue.ID,
			Title:  issue.Title,
			Status: string(issue.Status),
		}
	}

	correlatorObj := correlation.NewCorrelator(cwd, beadsPath)
	report, err := correlatorObj.GenerateReport(beadInfos, correlation.CorrelatorOptions{
		Limit: historyLimit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating history report: %v\n", err)
		os.Exit(1)
	}

	// Build blocker titles map for better descriptions
	blockerTitles := make(map[string]string)
	for _, issue := range rc.issues {
		blockerTitles[issue.ID] = issue.Title
	}

	opts := correlation.CausalityOptions{
		IncludeCommits: true,
		BlockerTitles:  blockerTitles,
	}

	result := report.BuildCausalityChain(beadID, opts)
	if result == nil {
		fmt.Fprintf(os.Stderr, "Bead not found: %s\n", beadID)
		os.Exit(1)
	}

	// Wrap with envelope fields
	type CausalityEnvelope struct {
		*correlation.CausalityResult
		OutputFormat string `json:"output_format,omitempty"`
		Version      string `json:"version,omitempty"`
	}
	output := CausalityEnvelope{
		CausalityResult: result,
		OutputFormat:     robotOutputFormat,
		Version:          version.Version,
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding causality result: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
