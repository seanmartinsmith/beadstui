// Adapted from github.com/zjrosen/perles (MIT License). See LICENSE in this directory.
// Interface redesigned for bt: in-memory execution against model.Issue.

package bql

import "github.com/seanmartinsmith/beadstui/pkg/model"

// Executor evaluates a parsed BQL query against issues.
type Executor interface {
	// Execute filters and sorts issues according to the query.
	// ORDER BY is applied if present. EXPAND adds related issues.
	Execute(query *Query, issues []model.Issue, opts ExecuteOpts) []model.Issue

	// Matches evaluates only the filter expression against a single issue.
	// Does not handle ORDER BY or EXPAND (those are set-level operations).
	Matches(query *Query, issue model.Issue, opts ExecuteOpts) bool
}

// ExecuteOpts provides context needed by the executor beyond the issue list.
type ExecuteOpts struct {
	// IssueMap enables dependency lookups for blocked field and EXPAND.
	// Uses pointer values to match bt's Model.issueMap type.
	IssueMap map[string]*model.Issue
}
