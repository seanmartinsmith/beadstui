package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/baseline"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/recipe"
)

// timeNowUTCRFC3339 returns the current UTC time formatted as RFC3339.
func timeNowUTCRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// filterByRepo filters issues to only include those from a specific repository.
// The filter matches issue IDs that start with the given prefix.
// If the prefix doesn't end with a separator character, it normalizes by checking
// common patterns (prefix-, prefix:, etc.).
func filterByRepo(issues []model.Issue, repoFilter string) []model.Issue {
	if repoFilter == "" {
		return issues
	}

	// Normalize the filter - ensure it's a proper prefix
	filter := repoFilter
	filterLower := strings.ToLower(filter)
	// If filter doesn't end with common separators, try matching as-is or with separators
	needsFlexibleMatch := !strings.HasSuffix(filter, "-") &&
		!strings.HasSuffix(filter, ":") &&
		!strings.HasSuffix(filter, "_")

	var result []model.Issue
	for _, issue := range issues {
		idLower := strings.ToLower(issue.ID)

		// Check if issue ID starts with the filter (case-insensitive)
		if strings.HasPrefix(idLower, filterLower) {
			result = append(result, issue)
			continue
		}

		// If flexible matching is needed, try with common separators
		if needsFlexibleMatch {
			if strings.HasPrefix(idLower, filterLower+"-") ||
				strings.HasPrefix(idLower, filterLower+":") ||
				strings.HasPrefix(idLower, filterLower+"_") {
				result = append(result, issue)
				continue
			}
		}

		// Also check SourceRepo field if set (case-insensitive)
		if issue.SourceRepo != "" && issue.SourceRepo != "." {
			sourceRepoLower := strings.ToLower(issue.SourceRepo)
			if strings.HasPrefix(sourceRepoLower, filterLower) {
				result = append(result, issue)
			}
		}
	}

	return result
}

// naturalLess compares two strings using natural sort order (numeric parts sorted numerically)
func naturalLess(s1, s2 string) bool {
	// Simple heuristic: if both strings end with numbers, compare the prefix then the number
	// e.g. "bv-2" vs "bv-10" -> "bv-" == "bv-", 2 < 10

	// Helper to split into prefix and numeric suffix
	split := func(s string) (string, int, bool) {
		lastDigit := -1
		for i := len(s) - 1; i >= 0; i-- {
			if s[i] >= '0' && s[i] <= '9' {
				lastDigit = i
			} else {
				break
			}
		}
		if lastDigit == -1 {
			return s, 0, false
		}
		// If the whole string is number, prefix is empty
		prefix := s[:lastDigit]
		numStr := s[lastDigit:]
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return s, 0, false
		}
		return prefix, num, true
	}

	p1, n1, ok1 := split(s1)
	p2, n2, ok2 := split(s2)

	if ok1 && ok2 && p1 == p2 {
		return n1 < n2
	}

	return s1 < s2
}

// repeatChar creates a string of n repeated characters
func repeatChar(c rune, n int) string {
	result := make([]rune, n)
	for i := range result {
		result[i] = c
	}
	return string(result)
}

// formatDuration formats a duration for display, right-aligned
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%6.2fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%6dms", d.Milliseconds())
}

// formatCycle formats a cycle for display
func formatCycle(cycle []string) string {
	if len(cycle) == 0 {
		return "(empty)"
	}
	result := cycle[0]
	for i := 1; i < len(cycle); i++ {
		result += " → " + cycle[i]
	}
	result += " → " + cycle[0]
	return result
}

// applyRecipeFilters filters issues based on recipe configuration
func applyRecipeFilters(issues []model.Issue, r *recipe.Recipe) []model.Issue {
	if r == nil {
		return issues
	}

	f := r.Filters
	now := time.Now()

	// Build a set of open blocker IDs for actionable filtering
	openBlockers := make(map[string]bool)
	for _, issue := range issues {
		if issue.Status != model.StatusClosed {
			openBlockers[issue.ID] = true
		}
	}

	var result []model.Issue
	for _, issue := range issues {
		// Status filter
		if len(f.Status) > 0 {
			match := false
			for _, s := range f.Status {
				if strings.EqualFold(string(issue.Status), s) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		// Priority filter
		if len(f.Priority) > 0 {
			match := false
			for _, p := range f.Priority {
				if issue.Priority == p {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		// Tags filter (must have all)
		if len(f.Tags) > 0 {
			match := true
			for _, tag := range f.Tags {
				found := false
				for _, label := range issue.Labels {
					if strings.EqualFold(label, tag) {
						found = true
						break
					}
				}
				if !found {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		// ExcludeTags filter
		if len(f.ExcludeTags) > 0 {
			excluded := false
			for _, excludeTag := range f.ExcludeTags {
				for _, label := range issue.Labels {
					if strings.EqualFold(label, excludeTag) {
						excluded = true
						break
					}
				}
				if excluded {
					break
				}
			}
			if excluded {
				continue
			}
		}

		// CreatedAfter filter
		if f.CreatedAfter != "" {
			threshold, err := recipe.ParseRelativeTime(f.CreatedAfter, now)
			if err == nil && !issue.CreatedAt.IsZero() && issue.CreatedAt.Before(threshold) {
				continue
			}
		}

		// CreatedBefore filter
		if f.CreatedBefore != "" {
			threshold, err := recipe.ParseRelativeTime(f.CreatedBefore, now)
			if err == nil && !issue.CreatedAt.IsZero() && issue.CreatedAt.After(threshold) {
				continue
			}
		}

		// UpdatedAfter filter
		if f.UpdatedAfter != "" {
			threshold, err := recipe.ParseRelativeTime(f.UpdatedAfter, now)
			if err == nil && !issue.UpdatedAt.IsZero() && issue.UpdatedAt.Before(threshold) {
				continue
			}
		}

		// UpdatedBefore filter
		if f.UpdatedBefore != "" {
			threshold, err := recipe.ParseRelativeTime(f.UpdatedBefore, now)
			if err == nil && !issue.UpdatedAt.IsZero() && issue.UpdatedAt.After(threshold) {
				continue
			}
		}

		// HasBlockers filter
		if f.HasBlockers != nil {
			hasOpenBlockers := false
			for _, dep := range issue.Dependencies {
				if dep.Type == model.DepBlocks && openBlockers[dep.DependsOnID] {
					hasOpenBlockers = true
					break
				}
			}
			if *f.HasBlockers != hasOpenBlockers {
				continue
			}
		}

		// Actionable filter (no open blockers)
		if f.Actionable != nil && *f.Actionable {
			hasOpenBlockers := false
			for _, dep := range issue.Dependencies {
				if dep.Type == model.DepBlocks && openBlockers[dep.DependsOnID] {
					hasOpenBlockers = true
					break
				}
			}
			if hasOpenBlockers {
				continue
			}
		}

		// TitleContains filter
		if f.TitleContains != "" {
			if !strings.Contains(strings.ToLower(issue.Title), strings.ToLower(f.TitleContains)) {
				continue
			}
		}

		// IDPrefix filter
		if f.IDPrefix != "" {
			if !strings.HasPrefix(issue.ID, f.IDPrefix) {
				continue
			}
		}

		result = append(result, issue)
	}

	return result
}

// applyRecipeSort sorts issues based on recipe configuration
func applyRecipeSort(issues []model.Issue, r *recipe.Recipe) []model.Issue {
	if r == nil || r.Sort.Field == "" {
		return issues
	}

	s := r.Sort
	ascending := s.Direction != "desc"

	// For priority, default to ascending (P0 first)
	if s.Field == "priority" && s.Direction == "" {
		ascending = true
	}
	// For dates, default to descending (newest first)
	if (s.Field == "created" || s.Field == "updated") && s.Direction == "" {
		ascending = false
	}

	sort.SliceStable(issues, func(i, j int) bool {
		var less bool

		switch s.Field {
		case "priority":
			less = issues[i].Priority < issues[j].Priority
		case "created":
			less = issues[i].CreatedAt.Before(issues[j].CreatedAt)
		case "updated":
			less = issues[i].UpdatedAt.Before(issues[j].UpdatedAt)
		case "title":
			less = strings.ToLower(issues[i].Title) < strings.ToLower(issues[j].Title)
		case "id":
			less = naturalLess(issues[i].ID, issues[j].ID)
		case "status":
			less = issues[i].Status < issues[j].Status
		default:
			// Unknown sort field, maintain order
			return false
		}

		if ascending {
			return less
		}
		return !less
	})

	return issues
}

// printDiffSummary prints a human-readable diff summary
func printDiffSummary(diff *analysis.SnapshotDiff, since string) {
	fmt.Printf("Changes since %s\n", since)
	fmt.Println("=" + repeatChar('=', len("Changes since "+since)))
	fmt.Println()

	// Health trend
	trendEmoji := "→"
	switch diff.Summary.HealthTrend {
	case "improving":
		trendEmoji = "↑"
	case "degrading":
		trendEmoji = "↓"
	}
	fmt.Printf("Health Trend: %s %s\n\n", trendEmoji, diff.Summary.HealthTrend)

	// Summary counts
	fmt.Println("Summary:")
	if diff.Summary.IssuesAdded > 0 {
		fmt.Printf("  + %d new issues\n", diff.Summary.IssuesAdded)
	}
	if diff.Summary.IssuesClosed > 0 {
		fmt.Printf("  ✓ %d issues closed\n", diff.Summary.IssuesClosed)
	}
	if diff.Summary.IssuesRemoved > 0 {
		fmt.Printf("  - %d issues removed\n", diff.Summary.IssuesRemoved)
	}
	if diff.Summary.IssuesReopened > 0 {
		fmt.Printf("  ↺ %d issues reopened\n", diff.Summary.IssuesReopened)
	}
	if diff.Summary.IssuesModified > 0 {
		fmt.Printf("  ~ %d issues modified\n", diff.Summary.IssuesModified)
	}
	if diff.Summary.CyclesIntroduced > 0 {
		fmt.Printf("  ⚠ %d new cycles introduced\n", diff.Summary.CyclesIntroduced)
	}
	if diff.Summary.CyclesResolved > 0 {
		fmt.Printf("  ✓ %d cycles resolved\n", diff.Summary.CyclesResolved)
	}
	fmt.Println()

	// New issues
	if len(diff.NewIssues) > 0 {
		fmt.Println("New Issues:")
		for _, issue := range diff.NewIssues {
			fmt.Printf("  + [%s] %s (P%d)\n", issue.ID, issue.Title, issue.Priority)
		}
		fmt.Println()
	}

	// Closed issues
	if len(diff.ClosedIssues) > 0 {
		fmt.Println("Closed Issues:")
		for _, issue := range diff.ClosedIssues {
			fmt.Printf("  ✓ [%s] %s\n", issue.ID, issue.Title)
		}
		fmt.Println()
	}

	// Reopened issues
	if len(diff.ReopenedIssues) > 0 {
		fmt.Println("Reopened Issues:")
		for _, issue := range diff.ReopenedIssues {
			fmt.Printf("  ↺ [%s] %s\n", issue.ID, issue.Title)
		}
		fmt.Println()
	}

	// Modified issues (show first 10)
	if len(diff.ModifiedIssues) > 0 {
		fmt.Println("Modified Issues:")
		shown := 0
		for _, mod := range diff.ModifiedIssues {
			if shown >= 10 {
				fmt.Printf("  ... and %d more\n", len(diff.ModifiedIssues)-10)
				break
			}
			fmt.Printf("  ~ [%s] %s\n", mod.IssueID, mod.Title)
			for _, change := range mod.Changes {
				fmt.Printf("      %s: %s → %s\n", change.Field, change.OldValue, change.NewValue)
			}
			shown++
		}
		fmt.Println()
	}

	// New cycles
	if len(diff.NewCycles) > 0 {
		fmt.Println("⚠ New Circular Dependencies:")
		for _, cycle := range diff.NewCycles {
			fmt.Printf("  %s\n", formatCycle(cycle))
		}
		fmt.Println()
	}

	// Metric deltas
	fmt.Println("Metric Changes:")
	if diff.MetricDeltas.TotalIssues != 0 {
		fmt.Printf("  Total issues: %+d\n", diff.MetricDeltas.TotalIssues)
	}
	if diff.MetricDeltas.OpenIssues != 0 {
		fmt.Printf("  Open issues: %+d\n", diff.MetricDeltas.OpenIssues)
	}
	if diff.MetricDeltas.BlockedIssues != 0 {
		fmt.Printf("  Blocked issues: %+d\n", diff.MetricDeltas.BlockedIssues)
	}
	if diff.MetricDeltas.CycleCount != 0 {
		fmt.Printf("  Cycles: %+d\n", diff.MetricDeltas.CycleCount)
	}
}

// truncateTitle truncates a title to maxLen runes, adding ellipsis if needed.
// It safely handles UTF-8 and ensures maxLen is reasonable.
func truncateTitle(title string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4 // Minimum sensible length: "X..."
	}
	runes := []rune(title)
	if len(runes) <= maxLen {
		return title
	}
	return string(runes[:maxLen-3]) + "..."
}

// escapeMarkdownTableCell escapes characters that would break markdown table formatting
func escapeMarkdownTableCell(s string) string {
	// Replace pipe characters and newlines that break tables
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// isSuspiciousIssueCount returns true if the current count looks wildly off from expected.
func isSuspiciousIssueCount(current, expected int) bool {
	if current == 0 {
		return true
	}
	if expected <= 0 {
		return false
	}
	threshold := expected / 5
	if threshold < 5 {
		threshold = 5
	}
	return current < threshold
}

// buildMetricItems converts a metrics map to a sorted slice of MetricItems
func buildMetricItems(metrics map[string]float64, limit int) []baseline.MetricItem {
	if len(metrics) == 0 {
		return nil
	}

	// Convert to slice for sorting
	items := make([]baseline.MetricItem, 0, len(metrics))
	for id, value := range metrics {
		items = append(items, baseline.MetricItem{ID: id, Value: value})
	}

	// Sort by value descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].Value > items[j].Value
	})

	// Limit to top N
	if len(items) > limit {
		items = items[:limit]
	}

	return items
}

// buildAttentionReason creates a human-readable reason for attention score
func buildAttentionReason(score analysis.LabelAttentionScore) string {
	var parts []string

	// High PageRank
	if score.PageRankSum > 0.5 {
		parts = append(parts, "High PageRank")
	}

	// Blocked issues
	if score.BlockedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked", score.BlockedCount))
	}

	// Stale issues
	if score.StaleCount > 0 {
		parts = append(parts, fmt.Sprintf("%d stale", score.StaleCount))
	}

	// Low velocity (VelocityFactor = ClosedLast30Days + 1, so 1.0 means zero closures)
	if score.VelocityFactor <= 1.0 {
		parts = append(parts, "low velocity")
	}

	// If no specific reasons, note the open count
	if len(parts) == 0 {
		return fmt.Sprintf("%d open issues", score.OpenCount)
	}

	return strings.Join(parts, ", ")
}

// countEdges counts blocking dependencies for config sizing
func countEdges(issues []model.Issue) int {
	count := 0
	for _, issue := range issues {
		for _, dep := range issue.Dependencies {
			if dep != nil && dep.Type == model.DepBlocks {
				count++
			}
		}
	}
	return count
}

// absInt returns the absolute value of an integer.
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// isStdoutTTY returns true if stdout is connected to a terminal.
func isStdoutTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
