package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// runSprintListOrShow handles --robot-sprint-list and --robot-sprint-show (bv-156).
func (rc *robotCtx) runSprintListOrShow(sprintShowID string) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	sprints, err := loader.LoadSprints(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading sprints: %v\n", err)
		os.Exit(1)
	}

	dataHash := analysis.ComputeDataHash(rc.issues)

	if sprintShowID != "" {
		// Find specific sprint
		var found *model.Sprint
		for i := range sprints {
			if sprints[i].ID == sprintShowID {
				found = &sprints[i]
				break
			}
		}
		if found == nil {
			fmt.Fprintf(os.Stderr, "Sprint not found: %s\n", sprintShowID)
			os.Exit(1)
		}
		// Wrap sprint with standard envelope
		type SprintShowOutput struct {
			RobotEnvelope
			Sprint *model.Sprint `json:"sprint"`
		}
		output := SprintShowOutput{
			RobotEnvelope: NewRobotEnvelope(dataHash),
			Sprint:        found,
		}
		encoder := rc.newEncoder()
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding sprint: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Output all sprints as JSON
		output := struct {
			RobotEnvelope
			SprintCount int            `json:"sprint_count"`
			Sprints     []model.Sprint `json:"sprints"`
		}{
			RobotEnvelope: NewRobotEnvelope(dataHash),
			SprintCount:   len(sprints),
			Sprints:       sprints,
		}
		encoder := rc.newEncoder()
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding sprints: %v\n", err)
			os.Exit(1)
		}
	}
	os.Exit(0)
}

// runBurndown handles --robot-burndown (bv-159).
func (rc *robotCtx) runBurndown(sprintID string) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	sprints, err := loader.LoadSprints(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading sprints: %v\n", err)
		os.Exit(1)
	}

	// Find the target sprint
	var targetSprint *model.Sprint
	if sprintID == "current" {
		// Find active sprint
		for i := range sprints {
			if sprints[i].IsActive() {
				targetSprint = &sprints[i]
				break
			}
		}
		if targetSprint == nil {
			fmt.Fprintf(os.Stderr, "No active sprint found\n")
			os.Exit(1)
		}
	} else {
		// Find sprint by ID
		for i := range sprints {
			if sprints[i].ID == sprintID {
				targetSprint = &sprints[i]
				break
			}
		}
		if targetSprint == nil {
			fmt.Fprintf(os.Stderr, "Sprint not found: %s\n", sprintID)
			os.Exit(1)
		}
	}

	// Build burndown data
	now := time.Now()
	burndown := calculateBurndownAt(targetSprint, rc.issues, now)
	burndown.RobotEnvelope = NewRobotEnvelope(analysis.ComputeDataHash(rc.issues))
	issueMap := make(map[string]model.Issue, len(rc.issues))
	for _, iss := range rc.issues {
		issueMap[iss.ID] = iss
	}
	if scopeChanges, err := computeSprintScopeChanges(cwd, targetSprint, issueMap, now); err == nil && len(scopeChanges) > 0 {
		burndown.ScopeChanges = scopeChanges
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(burndown); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding burndown: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runForecast handles --robot-forecast (bv-158).
func (rc *robotCtx) runForecast(forecastTarget, forecastLabel, forecastSprint string, forecastAgents int) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	// Build graph stats for depth calculation
	analyzer := analysis.NewAnalyzer(rc.analysisIssues())
	graphStats := analyzer.Analyze()

	// Filter issues by label and sprint if specified
	targetIssues := make([]model.Issue, 0, len(rc.issues))
	var sprintBeadIDs map[string]bool
	if forecastSprint != "" {
		sprints, err := loader.LoadSprints(cwd)
		if err == nil {
			for _, s := range sprints {
				if s.ID == forecastSprint {
					sprintBeadIDs = make(map[string]bool)
					for _, bid := range s.BeadIDs {
						sprintBeadIDs[bid] = true
					}
					break
				}
			}
		}
		if sprintBeadIDs == nil {
			fmt.Fprintf(os.Stderr, "Sprint not found: %s\n", forecastSprint)
			os.Exit(1)
		}
	}

	for _, iss := range rc.issues {
		// Filter by label
		if forecastLabel != "" {
			hasLabel := false
			for _, l := range iss.Labels {
				if l == forecastLabel {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				continue
			}
		}
		// Filter by sprint
		if sprintBeadIDs != nil && !sprintBeadIDs[iss.ID] {
			continue
		}
		targetIssues = append(targetIssues, iss)
	}

	now := time.Now()
	agents := forecastAgents
	if agents <= 0 {
		agents = 1
	}

	type ForecastSummary struct {
		TotalMinutes  int       `json:"total_minutes"`
		TotalDays     float64   `json:"total_days"`
		AvgConfidence float64   `json:"avg_confidence"`
		EarliestETA   time.Time `json:"earliest_eta"`
		LatestETA     time.Time `json:"latest_eta"`
	}
	type ForecastOutput struct {
		RobotEnvelope
		Agents        int                    `json:"agents"`
		Filters       map[string]string      `json:"filters,omitempty"`
		ForecastCount int                    `json:"forecast_count"`
		Forecasts     []analysis.ETAEstimate `json:"forecasts"`
		Summary       *ForecastSummary       `json:"summary,omitempty"`
	}

	var forecasts []analysis.ETAEstimate
	var outputErr error

	if forecastTarget == "all" {
		// Forecast all open issues
		for _, iss := range targetIssues {
			if iss.Status == model.StatusClosed {
				continue
			}
			eta, err := analysis.EstimateETAForIssue(rc.issues, &graphStats, iss.ID, agents, now)
			if err != nil {
				continue
			}
			forecasts = append(forecasts, eta)
		}
	} else {
		// Single issue forecast
		eta, err := analysis.EstimateETAForIssue(rc.issues, &graphStats, forecastTarget, agents, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		forecasts = append(forecasts, eta)
	}

	// Build summary if multiple forecasts
	var summary *ForecastSummary
	if len(forecasts) > 1 {
		totalMin := 0
		totalConf := 0.0
		earliest := forecasts[0].ETADate
		latest := forecasts[0].ETADate
		for _, f := range forecasts {
			totalMin += f.EstimatedMinutes
			totalConf += f.Confidence
			if f.ETADate.Before(earliest) {
				earliest = f.ETADate
			}
			if f.ETADate.After(latest) {
				latest = f.ETADate
			}
		}
		summary = &ForecastSummary{
			TotalMinutes:  totalMin,
			TotalDays:     float64(totalMin) / (60.0 * 8.0), // 8hr workday
			AvgConfidence: totalConf / float64(len(forecasts)),
			EarliestETA:   earliest,
			LatestETA:     latest,
		}
	}

	// Build output
	filters := make(map[string]string)
	if forecastLabel != "" {
		filters["label"] = forecastLabel
	}
	if forecastSprint != "" {
		filters["sprint"] = forecastSprint
	}

	output := ForecastOutput{
		RobotEnvelope: NewRobotEnvelope(analysis.ComputeDataHash(rc.issues)),
		Agents:        agents,
		ForecastCount: len(forecasts),
		Forecasts:     forecasts,
		Summary:       summary,
	}
	if len(filters) > 0 {
		output.Filters = filters
	}

	encoder := rc.newEncoder()
	if outputErr = encoder.Encode(output); outputErr != nil {
		fmt.Fprintf(os.Stderr, "Error encoding forecast: %v\n", outputErr)
		os.Exit(1)
	}
	os.Exit(0)
}

// runCapacity handles --robot-capacity (bv-160).
func (rc *robotCtx) runCapacity(capacityAgents int, capacityLabel string) {
	// Build graph stats for analysis
	analyzer := analysis.NewAnalyzer(rc.analysisIssues())
	graphStats := analyzer.Analyze()

	// Filter issues by label if specified
	targetIssues := rc.issues
	if capacityLabel != "" {
		filtered := make([]model.Issue, 0)
		for _, iss := range rc.issues {
			for _, l := range iss.Labels {
				if l == capacityLabel {
					filtered = append(filtered, iss)
					break
				}
			}
		}
		targetIssues = filtered
	}

	// Calculate open issues only
	openIssues := make([]model.Issue, 0)
	issueMap := make(map[string]model.Issue)
	for _, iss := range targetIssues {
		issueMap[iss.ID] = iss
		if iss.Status != model.StatusClosed {
			openIssues = append(openIssues, iss)
		}
	}

	now := time.Now()
	agents := capacityAgents
	if agents <= 0 {
		agents = 1
	}

	// Calculate total work remaining
	totalMinutes := 0
	for _, iss := range openIssues {
		eta, err := analysis.EstimateETAForIssue(targetIssues, &graphStats, iss.ID, 1, now)
		if err == nil {
			totalMinutes += eta.EstimatedMinutes
		}
	}

	// Analyze parallelizability by finding dependency chains
	blockedBy := make(map[string][]string)
	blocks := make(map[string][]string)
	for _, iss := range openIssues {
		for _, dep := range iss.Dependencies {
			if dep == nil {
				continue
			}
			depID := dep.DependsOnID
			if _, exists := issueMap[depID]; exists {
				blockedBy[iss.ID] = append(blockedBy[iss.ID], depID)
				blocks[depID] = append(blocks[depID], iss.ID)
			}
		}
	}

	// Find issues with no blockers (can start immediately)
	actionable := make([]string, 0)
	for _, iss := range openIssues {
		hasOpenBlocker := false
		for _, depID := range blockedBy[iss.ID] {
			if dep, ok := issueMap[depID]; ok && dep.Status != model.StatusClosed {
				hasOpenBlocker = true
				break
			}
		}
		if !hasOpenBlocker {
			actionable = append(actionable, iss.ID)
		}
	}

	// Calculate critical path (longest chain)
	var longestChain []string
	var dfs func(id string, path []string)
	visited := make(map[string]bool)
	dfs = func(id string, path []string) {
		if visited[id] {
			return
		}
		visited[id] = true
		path = append(path, id)
		if len(path) > len(longestChain) {
			longestChain = make([]string, len(path))
			copy(longestChain, path)
		}
		for _, nextID := range blocks[id] {
			if dep, ok := issueMap[nextID]; ok && dep.Status != model.StatusClosed {
				dfs(nextID, path)
			}
		}
		visited[id] = false
	}
	for _, startID := range actionable {
		dfs(startID, nil)
	}

	// Calculate serial minutes (work on critical path)
	serialMinutes := 0
	for _, id := range longestChain {
		eta, err := analysis.EstimateETAForIssue(targetIssues, &graphStats, id, 1, now)
		if err == nil {
			serialMinutes += eta.EstimatedMinutes
		}
	}

	// Parallelizable percentage
	parallelizablePct := 0.0
	if totalMinutes > 0 {
		parallelizablePct = float64(totalMinutes-serialMinutes) / float64(totalMinutes) * 100
	}

	// Calculate estimated completion with N agents
	parallelMinutes := totalMinutes - serialMinutes
	effectiveMinutes := serialMinutes + parallelMinutes/agents
	estimatedDays := float64(effectiveMinutes) / (60.0 * 8.0)

	// Find bottlenecks (issues blocking the most other issues)
	type Bottleneck struct {
		ID          string   `json:"id"`
		Title       string   `json:"title"`
		BlocksCount int      `json:"blocks_count"`
		Blocks      []string `json:"blocks,omitempty"`
	}
	bottlenecks := make([]Bottleneck, 0)
	for _, iss := range openIssues {
		if len(blocks[iss.ID]) > 1 {
			blockedIssues := blocks[iss.ID]
			bottlenecks = append(bottlenecks, Bottleneck{
				ID:          iss.ID,
				Title:       iss.Title,
				BlocksCount: len(blockedIssues),
				Blocks:      blockedIssues,
			})
		}
	}
	sort.Slice(bottlenecks, func(i, j int) bool {
		return bottlenecks[i].BlocksCount > bottlenecks[j].BlocksCount
	})
	if len(bottlenecks) > 5 {
		bottlenecks = bottlenecks[:5]
	}

	// Build output
	type CapacityOutput struct {
		RobotEnvelope
		Agents            int          `json:"agents"`
		Label             string       `json:"label,omitempty"`
		OpenIssueCount    int          `json:"open_issue_count"`
		TotalMinutes      int          `json:"total_minutes"`
		TotalDays         float64      `json:"total_days"`
		SerialMinutes     int          `json:"serial_minutes"`
		ParallelMinutes   int          `json:"parallel_minutes"`
		ParallelizablePct float64      `json:"parallelizable_pct"`
		EstimatedDays     float64      `json:"estimated_days"`
		CriticalPathLen   int          `json:"critical_path_length"`
		CriticalPath      []string     `json:"critical_path,omitempty"`
		ActionableCount   int          `json:"actionable_count"`
		Actionable        []string     `json:"actionable,omitempty"`
		Bottlenecks       []Bottleneck `json:"bottlenecks,omitempty"`
	}

	output := CapacityOutput{
		RobotEnvelope:     NewRobotEnvelope(analysis.ComputeDataHash(rc.issues)),
		Agents:            agents,
		OpenIssueCount:    len(openIssues),
		TotalMinutes:      totalMinutes,
		TotalDays:         float64(totalMinutes) / (60.0 * 8.0),
		SerialMinutes:     serialMinutes,
		ParallelMinutes:   parallelMinutes,
		ParallelizablePct: parallelizablePct,
		EstimatedDays:     estimatedDays,
		CriticalPathLen:   len(longestChain),
		CriticalPath:      longestChain,
		ActionableCount:   len(actionable),
		Actionable:        actionable,
		Bottlenecks:       bottlenecks,
	}
	if capacityLabel != "" {
		output.Label = capacityLabel
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding capacity: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
