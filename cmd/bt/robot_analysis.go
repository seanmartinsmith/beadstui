package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// limitMaps trims a float64 map to the top N entries by value.
func limitMaps(m map[string]float64, limit int) map[string]float64 {
	if limit <= 0 || limit >= len(m) {
		return m
	}
	type kv struct {
		k string
		v float64
	}
	var items []kv
	for k, v := range m {
		items = append(items, kv{k, v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].v == items[j].v {
			return items[i].k < items[j].k
		}
		return items[i].v > items[j].v
	})
	trim := make(map[string]float64, limit)
	for i := 0; i < limit; i++ {
		trim[items[i].k] = items[i].v
	}
	return trim
}

// limitMapInt trims an int map to limit entries (arbitrary order for ties).
func limitMapInt(m map[string]int, limit int) map[string]int {
	if limit <= 0 || len(m) <= limit {
		return m
	}
	trim := make(map[string]int, limit)
	count := 0
	for k, v := range m {
		trim[k] = v
		count++
		if count >= limit {
			break
		}
	}
	return trim
}

// limitSlice trims a string slice to limit entries.
func limitSlice(s []string, limit int) []string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit]
}

// runInsights handles --robot-insights.
func (rc *robotCtx) runInsights(forceFullAnalysis bool, asOf, asOfResolved, labelScope string) {
	analyzer := analysis.NewAnalyzer(rc.issues)
	if forceFullAnalysis {
		cfg := analysis.FullAnalysisConfig()
		analyzer.SetConfig(&cfg)
	}
	stats := analyzer.Analyze()
	// Generate top 50 lists for summary, but full stats are included in the struct
	insights := stats.GenerateInsights(50)

	// Add project-level velocity snapshot (using dedicated helper for efficiency)
	if v := analysis.ComputeProjectVelocity(rc.issues, time.Now(), 8); v != nil {
		snap := &analysis.VelocitySnapshot{
			Closed7:   v.ClosedLast7Days,
			Closed30:  v.ClosedLast30Days,
			AvgDays:   v.AvgDaysToClose,
			Estimated: v.Estimated,
		}
		if len(v.Weekly) > 0 {
			snap.Weekly = make([]int, len(v.Weekly))
			for i := range v.Weekly {
				snap.Weekly[i] = v.Weekly[i].Closed
			}
		}
		insights.Velocity = snap
	}

	// Default cap to keep payload small; allow override via env
	mapLimit := 200
	if v := os.Getenv("BT_INSIGHTS_MAP_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			mapLimit = n
		}
	}

	fullStats := struct {
		PageRank          map[string]float64 `json:"pagerank"`
		Betweenness       map[string]float64 `json:"betweenness"`
		Eigenvector       map[string]float64 `json:"eigenvector"`
		Hubs              map[string]float64 `json:"hubs"`
		Authorities       map[string]float64 `json:"authorities"`
		CriticalPathScore map[string]float64 `json:"critical_path_score"`
		CoreNumber        map[string]int     `json:"core_number"`
		Slack             map[string]float64 `json:"slack"`
		Articulation      []string           `json:"articulation_points"`
	}{
		PageRank:          limitMaps(stats.PageRank(), mapLimit),
		Betweenness:       limitMaps(stats.Betweenness(), mapLimit),
		Eigenvector:       limitMaps(stats.Eigenvector(), mapLimit),
		Hubs:              limitMaps(stats.Hubs(), mapLimit),
		Authorities:       limitMaps(stats.Authorities(), mapLimit),
		CriticalPathScore: limitMaps(stats.CriticalPathScore(), mapLimit),
		CoreNumber:        limitMapInt(stats.CoreNumber(), mapLimit),
		Slack:             limitMaps(stats.Slack(), mapLimit),
		Articulation:      limitSlice(stats.ArticulationPoints(), mapLimit),
	}

	// Get top what-if deltas for issues with highest downstream impact (bv-83)
	topWhatIfs := analyzer.TopWhatIfDeltas(10)

	// Generate advanced insights with canonical structure (bv-181)
	advancedInsights := analyzer.GenerateAdvancedInsights(analysis.DefaultAdvancedInsightsConfig())

	output := struct {
		GeneratedAt    string                  `json:"generated_at"`
		DataHash       string                  `json:"data_hash"`
		AsOf           string                  `json:"as_of,omitempty"`        // Historical snapshot ref
		AsOfCommit     string                  `json:"as_of_commit,omitempty"` // Resolved commit SHA
		AnalysisConfig analysis.AnalysisConfig `json:"analysis_config"`
		Status         analysis.MetricStatus   `json:"status"`
		LabelScope     string                  `json:"label_scope,omitempty"`   // bv-122: Label filter applied
		LabelContext   *analysis.LabelHealth   `json:"label_context,omitempty"` // bv-122: Health context for scoped label
		analysis.Insights
		FullStats        interface{}                `json:"full_stats"`
		TopWhatIfs       []analysis.WhatIfEntry     `json:"top_what_ifs,omitempty"`      // Issues with highest downstream impact (bv-83)
		AdvancedInsights *analysis.AdvancedInsights `json:"advanced_insights,omitempty"` // bv-181: Canonical advanced features
		UsageHints       []string                   `json:"usage_hints"`                 // bv-84: Agent-friendly hints
	}{
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		DataHash:         rc.dataHash,
		AsOf:             asOf,
		AsOfCommit:       asOfResolved,
		AnalysisConfig:   stats.Config,
		Status:           stats.Status(),
		LabelScope:       labelScope,
		LabelContext:     rc.labelScopeContext,
		Insights:         insights,
		FullStats:        fullStats,
		TopWhatIfs:       topWhatIfs,
		AdvancedInsights: advancedInsights,
		UsageHints: []string{
			"jq '.Bottlenecks[:5] | map(.ID)' - Top 5 bottleneck IDs",
			"jq '.CriticalPath[:3]' - Top 3 critical path items",
			"jq '.top_what_ifs[] | select(.delta.direct_unblocks > 2)' - High-impact items",
			"jq '.full_stats.pagerank | to_entries | sort_by(-.value)[:5]' - Top PageRank",
			"jq '.full_stats.core_number | to_entries | sort_by(-.value)[:5]' - Strongly embedded nodes (k-core)",
			"jq '.full_stats.articulation_points' - Structural cut points",
			"jq '.Slack[:5]' - Nodes with slack (good parallel work candidates)",
			"jq '.Cycles | length' - Count of detected cycles",
			"jq '.advanced_insights.cycle_break' - Cycle break suggestions (bv-181)",
			"BT_INSIGHTS_MAP_LIMIT=50 bt --robot-insights - Reduce map sizes",
		},
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding insights: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runPlan handles --robot-plan.
func (rc *robotCtx) runPlan(forceFullAnalysis bool, asOf, asOfResolved, labelScope string) {
	analyzer := analysis.NewAnalyzer(rc.issues)
	// For --robot-plan we primarily need Phase 1 metrics (degree/topo/density).
	// However, we still emit a stable status contract for agents. If the user
	// explicitly asks for full analysis, honor it; otherwise, skip expensive
	// centrality metrics and record the skip reasons deterministically.
	cfg := analysis.ConfigForSize(len(rc.issues), countEdges(rc.issues))
	if forceFullAnalysis {
		cfg = analysis.FullAnalysisConfig()
	} else {
		const skipReason = "not computed for --robot-plan"
		cfg.ComputePageRank = false
		cfg.PageRankSkipReason = skipReason
		cfg.ComputeBetweenness = false
		cfg.BetweennessMode = analysis.BetweennessSkip
		cfg.BetweennessSkipReason = skipReason
		cfg.ComputeHITS = false
		cfg.HITSSkipReason = skipReason
		cfg.ComputeEigenvector = false
		cfg.ComputeCriticalPath = false
		cfg.ComputeCycles = false
		cfg.CyclesSkipReason = skipReason
	}

	plan := analyzer.GetExecutionPlan()

	stats := analyzer.AnalyzeAsyncWithConfig(context.Background(), cfg)
	stats.WaitForPhase2()
	status := stats.Status()

	// Wrap with metadata
	output := struct {
		GeneratedAt    string                  `json:"generated_at"`
		DataHash       string                  `json:"data_hash"`
		AsOf           string                  `json:"as_of,omitempty"`        // Historical snapshot ref
		AsOfCommit     string                  `json:"as_of_commit,omitempty"` // Resolved commit SHA
		AnalysisConfig analysis.AnalysisConfig `json:"analysis_config"`
		Status         analysis.MetricStatus   `json:"status"`
		LabelScope     string                  `json:"label_scope,omitempty"`   // bv-122: Label filter applied
		LabelContext   *analysis.LabelHealth   `json:"label_context,omitempty"` // bv-122: Health context for scoped label
		Plan           analysis.ExecutionPlan  `json:"plan"`
		UsageHints     []string                `json:"usage_hints"` // bv-84: Agent-friendly hints
	}{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		DataHash:       rc.dataHash,
		AsOf:           asOf,
		AsOfCommit:     asOfResolved,
		AnalysisConfig: cfg,
		Status:         status,
		LabelScope:     labelScope,
		LabelContext:   rc.labelScopeContext,
		Plan:           plan,
		UsageHints: []string{
			"jq '.plan.tracks | length' - Number of parallel execution tracks",
			"jq '.plan.tracks[0].items | map(.id)' - First track item IDs",
			"jq '.plan.tracks[].items[] | select(.unblocks | length > 0)' - Items that unblock others",
			"jq '.plan.summary' - High-level execution summary",
			"jq '[.plan.tracks[].items[]] | length' - Total items across all tracks",
		},
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding execution plan: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runPriority handles --robot-priority.
func (rc *robotCtx) runPriority(forceFullAnalysis bool, asOf, asOfResolved, labelScope string, robotMinConf float64, robotMaxResults int, robotByLabel, robotByAssignee string) {
	analyzer := analysis.NewAnalyzer(rc.issues)
	cfg := analysis.ConfigForSize(len(rc.issues), countEdges(rc.issues))
	if forceFullAnalysis {
		cfg = analysis.FullAnalysisConfig()
	}
	analyzer.SetConfig(&cfg)
	stats := analyzer.AnalyzeAsyncWithConfig(context.Background(), cfg)
	stats.WaitForPhase2()
	status := stats.Status()

	// Use enhanced recommendations with what-if deltas and top reasons (bv-83)
	recommendations := analyzer.GenerateEnhancedRecommendations()

	// Apply robot filters (bv-84)
	filtered := make([]analysis.EnhancedPriorityRecommendation, 0, len(recommendations))
	issueMap := make(map[string]model.Issue, len(rc.issues))
	for _, iss := range rc.issues {
		issueMap[iss.ID] = iss
	}
	for _, rec := range recommendations {
		// Filter by minimum confidence
		if robotMinConf > 0 && rec.Confidence < robotMinConf {
			continue
		}
		// Filter by label
		if robotByLabel != "" {
			if iss, ok := issueMap[rec.IssueID]; ok {
				hasLabel := false
				for _, lbl := range iss.Labels {
					if lbl == robotByLabel {
						hasLabel = true
						break
					}
				}
				if !hasLabel {
					continue
				}
			} else {
				continue
			}
		}
		// Filter by assignee
		if robotByAssignee != "" {
			if iss, ok := issueMap[rec.IssueID]; ok {
				if iss.Assignee != robotByAssignee {
					continue
				}
			} else {
				continue
			}
		}
		filtered = append(filtered, rec)
	}
	recommendations = filtered

	// Apply max results limit
	maxResults := 10 // Default cap
	if robotMaxResults > 0 {
		maxResults = robotMaxResults
	}
	if len(recommendations) > maxResults {
		recommendations = recommendations[:maxResults]
	}

	// Count high confidence recommendations
	highConfidence := 0
	for _, rec := range recommendations {
		if rec.Confidence >= 0.7 {
			highConfidence++
		}
	}

	// Build output with summary
	output := struct {
		GeneratedAt       string                                    `json:"generated_at"`
		DataHash          string                                    `json:"data_hash"`
		AsOf              string                                    `json:"as_of,omitempty"`        // Historical snapshot ref
		AsOfCommit        string                                    `json:"as_of_commit,omitempty"` // Resolved commit SHA
		AnalysisConfig    analysis.AnalysisConfig                   `json:"analysis_config"`
		Status            analysis.MetricStatus                     `json:"status"`
		LabelScope        string                                    `json:"label_scope,omitempty"`   // bv-122: Label filter applied
		LabelContext      *analysis.LabelHealth                     `json:"label_context,omitempty"` // bv-122: Health context for scoped label
		Recommendations   []analysis.EnhancedPriorityRecommendation `json:"recommendations"`
		FieldDescriptions map[string]string                         `json:"field_descriptions"`
		Filters           struct {
			MinConfidence float64 `json:"min_confidence,omitempty"`
			MaxResults    int     `json:"max_results"`
			ByLabel       string  `json:"by_label,omitempty"`
			ByAssignee    string  `json:"by_assignee,omitempty"`
		} `json:"filters"`
		Summary struct {
			TotalIssues     int `json:"total_issues"`
			Recommendations int `json:"recommendations"`
			HighConfidence  int `json:"high_confidence"`
		} `json:"summary"`
		Usage []string `json:"usage_hints"` // bv-84: Agent-friendly hints
	}{
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
		DataHash:          rc.dataHash,
		AsOf:              asOf,
		AsOfCommit:        asOfResolved,
		AnalysisConfig:    cfg,
		Status:            status,
		LabelScope:        labelScope,
		LabelContext:      rc.labelScopeContext,
		Recommendations:   recommendations,
		FieldDescriptions: analysis.DefaultFieldDescriptions(),
		Usage: []string{
			"jq '.recommendations[] | select(.confidence > 0.7)' - Filter high confidence",
			"jq '.recommendations[0].explanation.what_if' - Get top item's impact",
			"jq '.recommendations | map({id: .issue_id, score: .impact_score})' - Extract IDs and scores",
			"jq '.recommendations[] | select(.explanation.what_if.parallelization_gain > 0)' - Find items that increase parallel work capacity",
			"--robot-min-confidence 0.6 - Pre-filter by confidence",
			"--robot-max-results 5 - Limit to top N results",
			"--robot-by-label bug - Filter by specific label",
		},
	}
	output.Filters.MinConfidence = robotMinConf
	output.Filters.MaxResults = maxResults
	output.Filters.ByLabel = robotByLabel
	output.Filters.ByAssignee = robotByAssignee
	output.Summary.TotalIssues = len(rc.issues)
	output.Summary.Recommendations = len(recommendations)
	output.Summary.HighConfidence = highConfidence

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding priority recommendations: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
