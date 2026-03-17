package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/seanmartinsmith/beadstui/pkg/correlation"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// BurndownOutput represents the JSON output for --robot-burndown (bv-159)
type BurndownOutput struct {
	RobotEnvelope
	SprintID          string                `json:"sprint_id"`
	SprintName        string                `json:"sprint_name"`
	StartDate         time.Time             `json:"start_date"`
	EndDate           time.Time             `json:"end_date"`
	TotalDays         int                   `json:"total_days"`
	ElapsedDays       int                   `json:"elapsed_days"`
	RemainingDays     int                   `json:"remaining_days"`
	TotalIssues       int                   `json:"total_issues"`
	CompletedIssues   int                   `json:"completed_issues"`
	RemainingIssues   int                   `json:"remaining_issues"`
	IdealBurnRate     float64               `json:"ideal_burn_rate"`
	ActualBurnRate    float64               `json:"actual_burn_rate"`
	ProjectedComplete *time.Time            `json:"projected_complete,omitempty"`
	OnTrack           bool                  `json:"on_track"`
	DailyPoints       []model.BurndownPoint `json:"daily_points"`
	IdealLine         []model.BurndownPoint `json:"ideal_line"`
	ScopeChanges      []ScopeChangeEvent    `json:"scope_changes,omitempty"`
}

// ScopeChangeEvent represents when issues were added/removed from sprint
type ScopeChangeEvent struct {
	Date       time.Time `json:"date"`
	IssueID    string    `json:"issue_id"`
	IssueTitle string    `json:"issue_title"`
	Action     string    `json:"action"` // "added" or "removed"
}

type sprintSnapshot struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	BeadIDs []string `json:"bead_ids,omitempty"`
}

type scopeCommit struct {
	sha       string
	timestamp time.Time
	order     int // stable ordering when timestamps are tied (git dates are second-granularity)
	events    []ScopeChangeEvent
}

func computeSprintScopeChanges(repoPath string, sprint *model.Sprint, issueMap map[string]model.Issue, now time.Time) ([]ScopeChangeEvent, error) {
	if sprint == nil || sprint.ID == "" {
		return nil, nil
	}
	if sprint.StartDate.IsZero() || sprint.EndDate.IsZero() {
		return nil, nil
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		// Not a git repository (common for ad-hoc exports/tests); scope changes are optional.
		return nil, nil
	}

	// Bound the history window to the sprint to keep this fast.
	since := sprint.StartDate.AddDate(0, 0, -1)
	until := sprint.EndDate
	if until.After(now) {
		until = now
	}

	args := []string{
		"-c", "color.ui=false",
		"log",
		"-p",
		"-U0",
		"--format=%H%x00%cI",
		fmt.Sprintf("--since=%s", since.Format(time.RFC3339)),
		fmt.Sprintf("--until=%s", until.Format(time.RFC3339)),
		"--",
		filepath.ToSlash(filepath.Join(".beads", loader.SprintsFileName)),
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w: %s", filepath.ToSlash(filepath.Join(".beads", loader.SprintsFileName)), err, bytes.TrimSpace(out))
	}

	var commits []scopeCommit
	var currentTS time.Time
	var currentSHA string
	var haveCommit bool
	var oldSnap, newSnap sprintSnapshot
	var haveOld, haveNew bool

	processCommit := func() {
		if !haveCommit {
			return
		}

		if haveOld && haveNew && oldSnap.ID == sprint.ID && newSnap.ID == sprint.ID {
			added := setDifference(newSnap.BeadIDs, oldSnap.BeadIDs)
			removed := setDifference(oldSnap.BeadIDs, newSnap.BeadIDs)
			if len(added) == 0 && len(removed) == 0 {
				return
			}

			sort.Strings(added)
			sort.Strings(removed)

			events := make([]ScopeChangeEvent, 0, len(added)+len(removed))
			for _, id := range removed {
				title := ""
				if iss, ok := issueMap[id]; ok {
					title = iss.Title
				}
				events = append(events, ScopeChangeEvent{
					Date:       currentTS.UTC(),
					IssueID:    id,
					IssueTitle: title,
					Action:     "removed",
				})
			}
			for _, id := range added {
				title := ""
				if iss, ok := issueMap[id]; ok {
					title = iss.Title
				}
				events = append(events, ScopeChangeEvent{
					Date:       currentTS.UTC(),
					IssueID:    id,
					IssueTitle: title,
					Action:     "added",
				})
			}

			commits = append(commits, scopeCommit{
				sha:       currentSHA,
				timestamp: currentTS.UTC(),
				order:     len(commits),
				events:    events,
			})
		}
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	// Sprints JSONL lines can contain large bead ID lists; allow a generous buffer.
	const maxCapacity = 10 * 1024 * 1024 // 10MB
	scanner.Buffer(make([]byte, 64*1024), maxCapacity)
	for scanner.Scan() {
		line := scanner.Text()

		sha, ts, ok := parseGitHeaderLine(line)
		if ok {
			processCommit()

			currentTS = ts
			currentSHA = sha
			haveCommit = true
			oldSnap, newSnap = sprintSnapshot{}, sprintSnapshot{}
			haveOld, haveNew = false, false
			continue
		}

		if !haveCommit {
			continue
		}

		if strings.HasPrefix(line, "-{") {
			if snap, ok := parseSprintJSONLine(strings.TrimPrefix(line, "-")); ok && snap.ID == sprint.ID {
				oldSnap = snap
				haveOld = true
			}
			continue
		}
		if strings.HasPrefix(line, "+{") {
			if snap, ok := parseSprintJSONLine(strings.TrimPrefix(line, "+")); ok && snap.ID == sprint.ID {
				newSnap = snap
				haveNew = true
			}
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	processCommit()

	if len(commits) == 0 {
		return nil, nil
	}

	// Ensure stable chronological output regardless of git log ordering nuances.
	sort.Slice(commits, func(i, j int) bool {
		if !commits[i].timestamp.Equal(commits[j].timestamp) {
			return commits[i].timestamp.Before(commits[j].timestamp)
		}
		// When commit timestamps are identical (common in tests), preserve the
		// original git log order reversed into chronological order.
		return commits[i].order > commits[j].order
	})

	var scopeChanges []ScopeChangeEvent
	for _, c := range commits {
		scopeChanges = append(scopeChanges, c.events...)
	}

	return scopeChanges, nil
}

func parseGitHeaderLine(line string) (sha string, ts time.Time, ok bool) {
	parts := strings.SplitN(line, "\x00", 2)
	if len(parts) != 2 {
		return "", time.Time{}, false
	}
	if len(parts[0]) != 40 {
		return "", time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[1]))
	if err != nil {
		return "", time.Time{}, false
	}
	return parts[0], parsed, true
}

func parseSprintJSONLine(line string) (sprintSnapshot, bool) {
	var snap sprintSnapshot
	if err := json.Unmarshal([]byte(line), &snap); err != nil {
		return sprintSnapshot{}, false
	}
	if snap.ID == "" {
		return sprintSnapshot{}, false
	}
	return snap, true
}

func setDifference(a, b []string) []string {
	if len(a) == 0 {
		return nil
	}
	mb := make(map[string]bool, len(b))
	for _, v := range b {
		mb[v] = true
	}
	var out []string
	for _, v := range a {
		if v == "" {
			continue
		}
		if !mb[v] {
			out = append(out, v)
		}
	}
	return out
}

// calculateBurndownAt is a deterministic variant of calculateBurndown for testing.
func calculateBurndownAt(sprint *model.Sprint, issues []model.Issue, now time.Time) BurndownOutput {

	// Build issue map for sprint beads
	issueMap := make(map[string]model.Issue, len(issues))
	for _, iss := range issues {
		issueMap[iss.ID] = iss
	}

	// Count total and completed issues in sprint
	var sprintIssues []model.Issue
	for _, beadID := range sprint.BeadIDs {
		if iss, ok := issueMap[beadID]; ok {
			sprintIssues = append(sprintIssues, iss)
		}
	}

	totalIssues := len(sprintIssues)
	completedIssues := 0
	for _, iss := range sprintIssues {
		if iss.Status == model.StatusClosed {
			completedIssues++
		}
	}
	remainingIssues := totalIssues - completedIssues

	// Calculate days
	totalDays := 0
	elapsedDays := 0
	remainingDays := 0

	if !sprint.StartDate.IsZero() && !sprint.EndDate.IsZero() {
		totalDays = int(sprint.EndDate.Sub(sprint.StartDate).Hours()/24) + 1
		if now.Before(sprint.StartDate) {
			elapsedDays = 0
			remainingDays = totalDays
		} else if now.After(sprint.EndDate) {
			elapsedDays = totalDays
			remainingDays = 0
		} else {
			elapsedDays = int(now.Sub(sprint.StartDate).Hours()/24) + 1
			remainingDays = totalDays - elapsedDays
		}
	}

	// Calculate burn rates
	idealBurnRate := 0.0
	if totalDays > 0 {
		idealBurnRate = float64(totalIssues) / float64(totalDays)
	}

	actualBurnRate := 0.0
	if elapsedDays > 0 {
		actualBurnRate = float64(completedIssues) / float64(elapsedDays)
	}

	// Calculate projected completion
	var projectedComplete *time.Time
	onTrack := true
	if actualBurnRate > 0 && remainingIssues > 0 {
		daysToComplete := float64(remainingIssues) / actualBurnRate
		projected := now.AddDate(0, 0, int(daysToComplete)+1)
		projectedComplete = &projected
		onTrack = !projected.After(sprint.EndDate)
	} else if remainingIssues == 0 {
		// Already complete
		onTrack = true
	} else if elapsedDays > 0 && completedIssues == 0 {
		// No progress made
		onTrack = false
	}

	// Generate daily burndown points
	dailyPoints := generateDailyBurndown(sprint, sprintIssues, now)

	// Generate ideal line
	idealLine := generateIdealLine(sprint, totalIssues)

	return BurndownOutput{
		SprintID:          sprint.ID,
		SprintName:        sprint.Name,
		StartDate:         sprint.StartDate,
		EndDate:           sprint.EndDate,
		TotalDays:         totalDays,
		ElapsedDays:       elapsedDays,
		RemainingDays:     remainingDays,
		TotalIssues:       totalIssues,
		CompletedIssues:   completedIssues,
		RemainingIssues:   remainingIssues,
		IdealBurnRate:     idealBurnRate,
		ActualBurnRate:    actualBurnRate,
		ProjectedComplete: projectedComplete,
		OnTrack:           onTrack,
		DailyPoints:       dailyPoints,
		IdealLine:         idealLine,
		ScopeChanges:      nil,
	}
}

// generateDailyBurndown creates actual burndown points based on issue closure dates
func generateDailyBurndown(sprint *model.Sprint, issues []model.Issue, now time.Time) []model.BurndownPoint {
	if sprint.StartDate.IsZero() || sprint.EndDate.IsZero() {
		return nil
	}

	var points []model.BurndownPoint
	totalIssues := len(issues)

	// Iterate through each day of the sprint
	for d := sprint.StartDate; !d.After(sprint.EndDate) && !d.After(now); d = d.AddDate(0, 0, 1) {
		dayEnd := d.Add(24*time.Hour - time.Second)
		completed := 0

		for _, iss := range issues {
			if iss.Status == model.StatusClosed && iss.ClosedAt != nil && !iss.ClosedAt.After(dayEnd) {
				completed++
			}
		}

		points = append(points, model.BurndownPoint{
			Date:      d,
			Remaining: totalIssues - completed,
			Completed: completed,
		})
	}

	return points
}

// generateIdealLine creates the ideal burndown line
func generateIdealLine(sprint *model.Sprint, totalIssues int) []model.BurndownPoint {
	if sprint.StartDate.IsZero() || sprint.EndDate.IsZero() || totalIssues == 0 {
		return nil
	}

	var points []model.BurndownPoint
	totalDays := int(sprint.EndDate.Sub(sprint.StartDate).Hours()/24) + 1
	burnPerDay := float64(totalIssues) / float64(totalDays)

	for i := 0; i <= totalDays; i++ {
		d := sprint.StartDate.AddDate(0, 0, i)
		remaining := totalIssues - int(float64(i)*burnPerDay)
		if remaining < 0 {
			remaining = 0
		}
		points = append(points, model.BurndownPoint{
			Date:      d,
			Remaining: remaining,
			Completed: totalIssues - remaining,
		})
	}

	return points
}

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
