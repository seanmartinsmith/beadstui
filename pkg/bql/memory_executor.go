package bql

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// MemoryExecutor evaluates BQL queries against in-memory issue slices.
type MemoryExecutor struct{}

// NewMemoryExecutor creates a new in-memory BQL executor.
func NewMemoryExecutor() *MemoryExecutor {
	return &MemoryExecutor{}
}

// Execute filters, sorts, and optionally expands the issue list.
func (e *MemoryExecutor) Execute(query *Query, issues []model.Issue, opts ExecuteOpts) []model.Issue {
	var result []model.Issue

	// Filter
	for _, issue := range issues {
		if query.Filter == nil || evalExpr(query.Filter, issue, opts, 0) {
			result = append(result, issue)
		}
	}

	// EXPAND: add related issues from dependency graph
	if query.HasExpand() && opts.IssueMap != nil {
		result = expandIssues(result, query.Expand, opts)
	}

	// ORDER BY
	if len(query.OrderBy) > 0 {
		sortIssues(result, query.OrderBy)
	}

	return result
}

// Matches evaluates just the filter expression against a single issue.
func (e *MemoryExecutor) Matches(query *Query, issue model.Issue, opts ExecuteOpts) bool {
	if query.Filter == nil {
		return true
	}
	return evalExpr(query.Filter, issue, opts, 0)
}

// maxEvalDepth is the maximum recursion depth for expression evaluation.
const maxEvalDepth = 100

// evalExpr recursively evaluates an expression against an issue.
func evalExpr(expr Expr, issue model.Issue, opts ExecuteOpts, depth int) bool {
	if depth > maxEvalDepth {
		return false // query too deeply nested, fail closed
	}
	switch e := expr.(type) {
	case *BinaryExpr:
		left := evalExpr(e.Left, issue, opts, depth+1)
		if e.Op == TokenAnd {
			if !left {
				return false // short-circuit
			}
			return evalExpr(e.Right, issue, opts, depth+1)
		}
		// OR
		if left {
			return true // short-circuit
		}
		return evalExpr(e.Right, issue, opts, depth+1)

	case *NotExpr:
		return !evalExpr(e.Expr, issue, opts, depth+1)

	case *CompareExpr:
		return evalCompare(e, issue, opts)

	case *InExpr:
		return evalIn(e, issue)
	}

	return false
}

// evalCompare evaluates a field comparison.
func evalCompare(e *CompareExpr, issue model.Issue, opts ExecuteOpts) bool {
	// Handle computed fields
	switch e.Field {
	case "blocked":
		isBlocked := isIssueBlocked(issue, opts.IssueMap)
		return isBlocked == e.Value.Bool
	}

	// Handle label field (array membership)
	if e.Field == "label" {
		return evalLabelCompare(e, issue)
	}

	// Get field value
	fv := fieldValue(issue, e.Field)

	// Type-specific comparison
	switch e.Value.Type {
	case ValueBool:
		// Bool fields already handled (blocked above)
		return false

	case ValuePriority:
		intVal, ok := fv.(int)
		if !ok {
			return false
		}
		return compareInts(intVal, e.Value.Int, e.Op)

	case ValueInt:
		intVal, ok := fv.(int)
		if !ok {
			return false
		}
		return compareInts(intVal, e.Value.Int, e.Op)

	case ValueDate:
		timeVal, ok := fv.(time.Time)
		if !ok {
			// Try *time.Time
			if tp, ok2 := fv.(*time.Time); ok2 && tp != nil {
				timeVal = *tp
			} else {
				return false
			}
		}
		target, err := resolveDateValue(e.Value.String)
		if err != nil {
			return false
		}
		return compareTimes(timeVal, target, e.Op)

	default: // ValueString
		strVal := fmt.Sprint(fv)
		return compareStrings(strVal, e.Value.String, e.Op)
	}
}

// evalLabelCompare checks label membership.
func evalLabelCompare(e *CompareExpr, issue model.Issue) bool {
	switch e.Op {
	case TokenEq:
		for _, l := range issue.Labels {
			if l == e.Value.String {
				return true
			}
		}
		return false
	case TokenNeq:
		for _, l := range issue.Labels {
			if l == e.Value.String {
				return false
			}
		}
		return true
	case TokenContains:
		for _, l := range issue.Labels {
			if strings.Contains(strings.ToLower(l), strings.ToLower(e.Value.String)) {
				return true
			}
		}
		return false
	case TokenNotContains:
		for _, l := range issue.Labels {
			if strings.Contains(strings.ToLower(l), strings.ToLower(e.Value.String)) {
				return false
			}
		}
		return true
	}
	return false
}

// evalIn evaluates an IN expression.
func evalIn(e *InExpr, issue model.Issue) bool {
	// Label field: check if any label matches any value
	if e.Field == "label" {
		for _, v := range e.Values {
			for _, l := range issue.Labels {
				if l == v.String {
					if e.Not {
						return false
					}
					// For non-NOT IN, finding one match is enough
					if !e.Not {
						return true
					}
				}
			}
		}
		return e.Not // NOT IN: true if no match found; IN: false if no match
	}

	fv := fieldValue(issue, e.Field)
	strVal := fmt.Sprint(fv)

	// Priority field: compare as ints
	if e.Field == "priority" {
		intVal, ok := fv.(int)
		if !ok {
			return e.Not
		}
		for _, v := range e.Values {
			if intVal == v.Int {
				return !e.Not
			}
		}
		return e.Not
	}

	// String comparison for other fields
	for _, v := range e.Values {
		if strVal == v.String {
			return !e.Not
		}
	}
	return e.Not
}

// fieldValue extracts a field value from an issue by BQL field name.
func fieldValue(issue model.Issue, field string) any {
	switch field {
	case "id":
		return issue.ID
	case "title":
		return issue.Title
	case "description":
		return issue.Description
	case "design":
		return issue.Design
	case "notes":
		return issue.Notes
	case "status":
		return string(issue.Status)
	case "priority":
		return issue.Priority
	case "type":
		return string(issue.IssueType)
	case "assignee":
		return issue.Assignee
	case "source_repo":
		return issue.SourceRepo
	case "created_at":
		return issue.CreatedAt
	case "updated_at":
		return issue.UpdatedAt
	case "due_date":
		return issue.DueDate
	case "closed_at":
		return issue.ClosedAt
	default:
		return ""
	}
}

// isIssueBlocked checks if an issue has open blocking dependencies.
func isIssueBlocked(issue model.Issue, issueMap map[string]*model.Issue) bool {
	for _, dep := range issue.Dependencies {
		if dep == nil || !dep.Type.IsBlocking() {
			continue
		}
		if blocker, exists := issueMap[dep.DependsOnID]; exists && blocker != nil {
			if !blocker.Status.IsClosed() && !blocker.Status.IsTombstone() {
				return true
			}
		}
	}
	return false
}

// compareInts compares two ints using the given operator.
func compareInts(a, b int, op TokenType) bool {
	switch op {
	case TokenEq:
		return a == b
	case TokenNeq:
		return a != b
	case TokenLt:
		return a < b
	case TokenGt:
		return a > b
	case TokenLte:
		return a <= b
	case TokenGte:
		return a >= b
	}
	return false
}

// compareStrings compares two strings using the given operator.
func compareStrings(a, b string, op TokenType) bool {
	switch op {
	case TokenEq:
		return strings.EqualFold(a, b)
	case TokenNeq:
		return !strings.EqualFold(a, b)
	case TokenContains:
		return strings.Contains(strings.ToLower(a), strings.ToLower(b))
	case TokenNotContains:
		return !strings.Contains(strings.ToLower(a), strings.ToLower(b))
	}
	return false
}

// compareTimes compares two times using the given operator.
// For = and !=, both values are truncated to date-only (midnight) so that
// queries like "created_at = today" match any time on that day.
func compareTimes(a, b time.Time, op TokenType) bool {
	switch op {
	case TokenEq:
		return truncateToDate(a).Equal(truncateToDate(b))
	case TokenNeq:
		return !truncateToDate(a).Equal(truncateToDate(b))
	case TokenLt:
		return a.Before(b)
	case TokenGt:
		return a.After(b)
	case TokenLte:
		return !a.After(b)
	case TokenGte:
		return !a.Before(b)
	}
	return false
}

// truncateToDate returns t with time components zeroed (midnight in t's location).
func truncateToDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// resolveDateValue converts a BQL date literal to a time.Time.
func resolveDateValue(dateStr string) (time.Time, error) {
	now := time.Now()

	switch dateStr {
	case "today":
		y, m, d := now.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location()), nil
	case "yesterday":
		y, m, d := now.AddDate(0, 0, -1).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location()), nil
	}

	// Relative offsets: -7d, -24h, -3m
	if len(dateStr) > 1 && dateStr[0] == '-' {
		suffix := dateStr[len(dateStr)-1]
		numStr := dateStr[1 : len(dateStr)-1]
		var n int
		if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil {
			return time.Time{}, fmt.Errorf("invalid date offset: %q", dateStr)
		}

		switch suffix {
		case 'd', 'D':
			return now.AddDate(0, 0, -n), nil
		case 'h', 'H':
			return now.Add(-time.Duration(n) * time.Hour), nil
		case 'm', 'M':
			return now.AddDate(0, -n, 0), nil
		}
	}

	// ISO date format: YYYY-MM-DD
	if len(dateStr) == 10 && dateStr[4] == '-' && dateStr[7] == '-' {
		t, err := time.Parse("2006-01-02", dateStr)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported date value: %q", dateStr)
}

// sortIssues sorts issues by ORDER BY terms.
func sortIssues(issues []model.Issue, orderBy []OrderTerm) {
	sort.SliceStable(issues, func(i, j int) bool {
		for _, term := range orderBy {
			cmp := compareFieldValues(
				fieldValue(issues[i], term.Field),
				fieldValue(issues[j], term.Field),
			)
			if cmp == 0 {
				continue
			}
			if term.Desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false // equal
	})
}

// compareFieldValues compares two field values generically.
func compareFieldValues(a, b any) int {
	switch av := a.(type) {
	case int:
		bv, ok := b.(int)
		if !ok {
			return 0
		}
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		}
		return 0

	case string:
		bv, ok := b.(string)
		if !ok {
			return 0
		}
		return strings.Compare(strings.ToLower(av), strings.ToLower(bv))

	case time.Time:
		bv, ok := b.(time.Time)
		if !ok {
			return 0
		}
		switch {
		case av.Before(bv):
			return -1
		case av.After(bv):
			return 1
		}
		return 0

	case *time.Time:
		bv, ok := b.(*time.Time)
		if !ok {
			return 0
		}
		if av == nil && bv == nil {
			return 0
		}
		if av == nil {
			return -1
		}
		if bv == nil {
			return 1
		}
		switch {
		case av.Before(*bv):
			return -1
		case av.After(*bv):
			return 1
		}
		return 0
	}

	return 0
}

// expandIssues adds related issues from the dependency graph.
func expandIssues(matched []model.Issue, expand *ExpandClause, opts ExecuteOpts) []model.Issue {
	if opts.IssueMap == nil {
		return matched
	}

	seen := make(map[string]bool, len(matched))
	for _, issue := range matched {
		seen[issue.ID] = true
	}

	// Build reverse dependency map for "down" traversal
	var reverseDeps map[string][]string
	if expand.Type == ExpandDown || expand.Type == ExpandAll {
		reverseDeps = make(map[string][]string)
		for id, issue := range opts.IssueMap {
			if issue == nil {
				continue
			}
			for _, dep := range issue.Dependencies {
				if dep != nil && dep.Type.IsBlocking() {
					reverseDeps[dep.DependsOnID] = append(reverseDeps[dep.DependsOnID], id)
				}
			}
		}
	}

	// BFS expansion
	queue := make([]string, 0, len(matched))
	for _, issue := range matched {
		queue = append(queue, issue.ID)
	}

	maxDepth := int(expand.Depth)
	if expand.Depth == DepthUnlimited {
		maxDepth = 100 // safety cap
	}

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var nextQueue []string
		for _, id := range queue {
			issuep, ok := opts.IssueMap[id]
			if !ok || issuep == nil {
				continue
			}

			// Expand UP: follow dependencies (what this issue depends on)
			if expand.Type == ExpandUp || expand.Type == ExpandAll {
				for _, dep := range issuep.Dependencies {
					if dep == nil || !dep.Type.IsBlocking() {
						continue
					}
					if !seen[dep.DependsOnID] {
						if related, ok := opts.IssueMap[dep.DependsOnID]; ok && related != nil {
							matched = append(matched, *related)
							seen[dep.DependsOnID] = true
							nextQueue = append(nextQueue, dep.DependsOnID)
						}
					}
				}
			}

			// Expand DOWN: follow reverse dependencies (what depends on this issue)
			if expand.Type == ExpandDown || expand.Type == ExpandAll {
				for _, depID := range reverseDeps[id] {
					if !seen[depID] {
						if related, ok := opts.IssueMap[depID]; ok && related != nil {
							matched = append(matched, *related)
							seen[depID] = true
							nextQueue = append(nextQueue, depID)
						}
					}
				}
			}
		}
		queue = nextQueue
	}

	return matched
}
