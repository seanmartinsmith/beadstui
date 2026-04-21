package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/view"
)

// robotListCmd outputs filtered issues as JSON/TOON for agent consumption.
// Simpler alternative to robot bql for the common 80% case.
//
// Output shape defaults to compact (view.CompactIssue) so large result sets
// don't burn an agent's context window; pass --full to restore the
// pre-compact shape with description/design/notes bodies preserved.
var robotListCmd = &cobra.Command{
	Use:   "list",
	Short: "Output filtered issue list as JSON",
	Long:  "List issues with simple flag-based filters. For complex queries, use 'bt robot bql'.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadIssues(); err != nil {
			return err
		}

		statusFilter, _ := cmd.Flags().GetString("status")
		priorityFilter, _ := cmd.Flags().GetString("priority")
		typeFilter, _ := cmd.Flags().GetString("type")
		hasLabelFilter, _ := cmd.Flags().GetString("has-label")
		limit, _ := cmd.Flags().GetInt("limit")

		// robot list bypasses robotPreRun, so --source must be applied here.
		baseIssues := filterBySource(appCtx.issues, robotFlagSource)
		issues := filterIssuesForList(baseIssues, statusFilter, priorityFilter, typeFilter, hasLabelFilter)

		total := len(issues)
		truncated := false
		if limit > 0 && total > limit {
			issues = issues[:limit]
			truncated = true
		}

		dataHash := analysis.ComputeDataHash(issues)
		envelope := NewRobotEnvelope(dataHash)

		var payload any = issues
		if robotOutputShape == robotShapeCompact {
			envelope.Schema = view.CompactIssueSchemaV1
			// Compact over the source-filtered set so reverse-map counts
			// (children, unblocks, is_blocked) reflect the scoped graph,
			// then project only the flag-filtered subset by ID.
			allCompact := view.CompactAll(baseIssues)
			byID := make(map[string]view.CompactIssue, len(allCompact))
			for _, c := range allCompact {
				byID[c.ID] = c
			}
			filtered := make([]view.CompactIssue, 0, len(issues))
			for _, iss := range issues {
				if c, ok := byID[iss.ID]; ok {
					filtered = append(filtered, c)
				}
			}
			payload = filtered
		}

		output := struct {
			RobotEnvelope
			Query     listQuery `json:"query"`
			Total     int       `json:"total"`
			Truncated bool      `json:"truncated"`
			Limit     int       `json:"limit"`
			Count     int       `json:"count"`
			Issues    any       `json:"issues"`
		}{
			RobotEnvelope: envelope,
			Query: listQuery{
				Status:   statusFilter,
				Priority: priorityFilter,
				Type:     typeFilter,
				HasLabel: hasLabelFilter,
				Repo:     flagRepo,
				Source:   robotFlagSource,
				Global:   flagGlobal,
				Limit:    limit,
			},
			Total:     total,
			Truncated: truncated,
			Limit:     limit,
			Count:     len(issues),
			Issues:    payload,
		}
		enc := newRobotEncoder(os.Stdout)
		if err := enc.Encode(output); err != nil {
			return fmt.Errorf("encoding robot-list: %w", err)
		}
		os.Exit(0)
		return nil
	},
}

// listQuery records the filters applied, echoed back in the response envelope.
type listQuery struct {
	Status   string `json:"status"`
	Priority string `json:"priority"`
	Type     string `json:"type"`
	HasLabel string `json:"has_label"`
	Repo     string `json:"repo"`
	Source   string `json:"source"`
	Global   bool   `json:"global"`
	Limit    int    `json:"limit"`
}

// filterIssuesForList applies simple flag-based filters to the issue list.
func filterIssuesForList(issues []model.Issue, status, priority, issueType, hasLabel string) []model.Issue {
	result := issues

	if status != "" {
		statuses := strings.Split(strings.ToLower(status), ",")
		statusSet := make(map[model.Status]bool, len(statuses))
		for _, s := range statuses {
			statusSet[model.Status(strings.TrimSpace(s))] = true
		}
		var filtered []model.Issue
		for _, issue := range result {
			if statusSet[issue.Status] {
				filtered = append(filtered, issue)
			}
		}
		result = filtered
	}

	if priority != "" {
		low, high, err := parsePriorityRange(priority)
		if err == nil {
			var filtered []model.Issue
			for _, issue := range result {
				if issue.Priority >= low && issue.Priority <= high {
					filtered = append(filtered, issue)
				}
			}
			result = filtered
		}
	}

	if issueType != "" {
		t := model.IssueType(strings.ToLower(issueType))
		var filtered []model.Issue
		for _, issue := range result {
			if issue.IssueType == t {
				filtered = append(filtered, issue)
			}
		}
		result = filtered
	}

	if hasLabel != "" {
		var filtered []model.Issue
		for _, issue := range result {
			for _, l := range issue.Labels {
				if l == hasLabel {
					filtered = append(filtered, issue)
					break
				}
			}
		}
		result = filtered
	}

	return result
}

// parsePriorityRange parses "0", "0-1", "2-3" into low/high bounds.
func parsePriorityRange(s string) (low, high int, err error) {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "-"); idx >= 0 {
		low, err = strconv.Atoi(strings.TrimSpace(s[:idx]))
		if err != nil {
			return 0, 0, fmt.Errorf("invalid priority range low: %w", err)
		}
		high, err = strconv.Atoi(strings.TrimSpace(s[idx+1:]))
		if err != nil {
			return 0, 0, fmt.Errorf("invalid priority range high: %w", err)
		}
		return low, high, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid priority: %w", err)
	}
	return v, v, nil
}
