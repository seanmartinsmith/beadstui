package main

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/view"
)

// runPortfolio emits one view.PortfolioRecord per project — per-project
// health aggregates answering "which project needs attention?" at the org
// level.
//
// Scope: under --global, one record per SourceRepo. Without --global, a
// single record for the current project.
//
// The --shape flag is inherited but effectively a no-op: PortfolioRecord is
// compact-by-construction (no body fields to strip). The envelope's `schema`
// is set unconditionally to portfolio.v1 because the payload IS a versioned
// projection.
func (rc *robotCtx) runPortfolio() {
	issues := rc.analysisIssues()
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()
	pagerank := stats.PageRank()
	now := time.Now().UTC()

	groups := groupIssuesByProject(issues, flagGlobal, rc.repoName)
	records := make([]view.PortfolioRecord, 0, len(groups))
	for project, projectIssues := range groups {
		records = append(records, view.ComputePortfolioRecord(project, projectIssues, issues, pagerank, now))
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Project < records[j].Project
	})

	envelope := NewRobotEnvelope(rc.dataHash)
	envelope.Schema = view.PortfolioRecordSchemaV1

	output := struct {
		RobotEnvelope
		Projects []view.PortfolioRecord `json:"projects"`
	}{
		RobotEnvelope: envelope,
		Projects:      records,
	}

	enc := rc.newEncoder()
	if err := enc.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding portfolio: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// groupIssuesByProject partitions issues by project key.
//
// Global mode: group by SourceRepo. Empty SourceRepo maps to "unknown" —
// logged at debug so noisy environments can investigate; never dropped.
//
// Single-project mode: everything is one group keyed by rc.repoName; falling
// back to the uniform SourceRepo if repoName is empty; "local" as a final
// fallback so every record has a non-empty project field.
func groupIssuesByProject(issues []model.Issue, global bool, repoName string) map[string][]model.Issue {
	if !global {
		key := repoName
		if key == "" {
			if uniform, ok := uniformSourceRepo(issues); ok {
				key = uniform
			}
		}
		if key == "" {
			key = "local"
		}
		return map[string][]model.Issue{key: issues}
	}

	out := make(map[string][]model.Issue)
	unknownCount := 0
	for i := range issues {
		project := issues[i].SourceRepo
		if project == "" {
			project = "unknown"
			unknownCount++
		}
		out[project] = append(out[project], issues[i])
	}
	if unknownCount > 0 {
		slog.Debug("portfolio: issues with empty SourceRepo grouped under 'unknown'",
			"count", unknownCount)
	}
	return out
}

// uniformSourceRepo returns the single SourceRepo value if every issue shares
// it; (false) when the set is empty or heterogeneous.
func uniformSourceRepo(issues []model.Issue) (string, bool) {
	if len(issues) == 0 {
		return "", false
	}
	first := issues[0].SourceRepo
	if first == "" {
		return "", false
	}
	for i := 1; i < len(issues); i++ {
		if issues[i].SourceRepo != first {
			return "", false
		}
	}
	return first, true
}
