package main

import (
	"os"
	"sort"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/correlation"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// generateHistoryForExport creates time-travel history data from git history
func generateHistoryForExport(issues []model.Issue) (*TimeTravelHistory, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Check if we're in a git repository
	if err := correlation.ValidateRepository(cwd); err != nil {
		return nil, err
	}

	// Get beads path
	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		return nil, err
	}
	beadsPath, err := loader.FindJSONLPath(beadsDir)
	if err != nil {
		return nil, err
	}

	// Build bead info from issues
	beadInfos := make([]correlation.BeadInfo, len(issues))
	for i, issue := range issues {
		beadInfos[i] = correlation.BeadInfo{
			ID:     issue.ID,
			Title:  issue.Title,
			Status: string(issue.Status),
		}
	}

	// Generate correlation report
	correlator := correlation.NewCorrelator(cwd, beadsPath)
	report, err := correlator.GenerateReport(beadInfos, correlation.CorrelatorOptions{
		Limit: 500, // Reasonable limit for time-travel
	})
	if err != nil {
		return nil, err
	}

	// Convert to time-travel format
	// Group by commit date and track bead changes
	commitMap := make(map[string]*TimeTravelCommit)

	for beadID, history := range report.Histories {
		for _, commit := range history.Commits {
			ttCommit, exists := commitMap[commit.SHA]
			if !exists {
				ttCommit = &TimeTravelCommit{
					SHA:     commit.SHA,
					Date:    commit.Timestamp.Format(time.RFC3339),
					Message: commit.Message,
				}
				commitMap[commit.SHA] = ttCommit
			}

			// Determine if this bead was added or modified in this commit
			// For simplicity, we consider any commit touching a bead as "adding" it
			// (the first time it appears in history)
			ttCommit.BeadsAdded = append(ttCommit.BeadsAdded, beadID)
		}
	}

	// Convert map to sorted slice
	var ttCommits []TimeTravelCommit
	for _, commit := range commitMap {
		// Deduplicate beads_added
		seen := make(map[string]bool)
		var dedupedAdded []string
		for _, id := range commit.BeadsAdded {
			if !seen[id] {
				seen[id] = true
				dedupedAdded = append(dedupedAdded, id)
			}
		}
		commit.BeadsAdded = dedupedAdded
		ttCommits = append(ttCommits, *commit)
	}

	// Sort commits by date
	sort.Slice(ttCommits, func(i, j int) bool {
		return ttCommits[i].Date < ttCommits[j].Date
	})

	return &TimeTravelHistory{
		GeneratedAt: timeNowUTCRFC3339(),
		Commits:     ttCommits,
	}, nil
}

// generateJQHelpers creates a markdown document with jq snippets for agent brief
func generateJQHelpers() string {
	return `# jq Helper Snippets

Quick reference for extracting data from the agent brief JSON files.

## triage.json

### Top Picks
` + "```bash" + `
# Get top 3 recommendations
jq '.quick_ref.top_picks[:3]' triage.json

# Get IDs of top picks
jq '.quick_ref.top_picks[].id' triage.json

# Get top pick with highest unblocks
jq '.quick_ref.top_picks | max_by(.unblocks)' triage.json
` + "```" + `

### Recommendations
` + "```bash" + `
# List all recommendations with scores
jq '.recommendations[] | {id, score, action}' triage.json

# Filter high-score items (score > 0.15)
jq '.recommendations[] | select(.score > 0.15)' triage.json

# Get breakdown metrics
jq '.recommendations[] | {id, pr: .breakdown.pagerank_norm, bw: .breakdown.betweenness_norm}' triage.json
` + "```" + `

### Quick Wins
` + "```bash" + `
# List quick wins
jq '.quick_wins[] | {id, title, reason}' triage.json

# Count quick wins
jq '.quick_wins | length' triage.json
` + "```" + `

### Blockers
` + "```bash" + `
# Get actionable blockers
jq '.blockers_to_clear[] | select(.actionable)' triage.json

# Sort by unblocks count
jq '.blockers_to_clear | sort_by(-.unblocks_count)' triage.json
` + "```" + `

## insights.json

### Graph Metrics
` + "```bash" + `
# Top PageRank issues
jq '.top_pagerank | to_entries | sort_by(-.value)[:5]' insights.json

# Top betweenness centrality
jq '.top_betweenness | to_entries | sort_by(-.value)[:5]' insights.json

# Find hub issues (high in-degree)
jq '.top_in_degree | to_entries | sort_by(-.value)[:3]' insights.json
` + "```" + `

### Project Health
` + "```bash" + `
# Get velocity metrics
jq '.velocity' insights.json

# List critical issues
jq '.critical_issues' insights.json
` + "```" + `

## Combining Files
` + "```bash" + `
# Cross-reference top picks with insights
jq -s '.[0].quick_ref.top_picks[0].id as $id | .[1].top_pagerank[$id] // 0' triage.json insights.json

# Export summary to CSV
jq -r '.recommendations[] | [.id, .score, .action] | @csv' triage.json
` + "```" + `
`
}

// TimeTravelHistory represents the history data format for time-travel animation (bv-z38b)
type TimeTravelHistory struct {
	GeneratedAt string             `json:"generated_at"`
	Commits     []TimeTravelCommit `json:"commits"`
}

// TimeTravelCommit represents a single commit in the time-travel history
type TimeTravelCommit struct {
	SHA         string   `json:"sha"`
	Date        string   `json:"date"`
	Message     string   `json:"message,omitempty"`
	BeadsAdded  []string `json:"beads_added,omitempty"`
	BeadsClosed []string `json:"beads_closed,omitempty"`
}
