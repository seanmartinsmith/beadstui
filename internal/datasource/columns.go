package datasource

import "strings"

// IssuesColumnList is the canonical column list for the issues table.
// Used by both DoltReader and GlobalDoltReader. One place to update
// when beads adds columns upstream.
//
// Column order must match the scan order in DoltReader.LoadIssuesFiltered
// and GlobalDoltReader.scanGlobalIssue.
//
// When a discovered Dolt database lacks a column listed here (schema drift
// across mixed-version databases - bt-ebzy), GlobalDoltReader probes each
// database's column set up front and emits "NULL AS <col>" for missing
// columns so the UNION ALL stays uniform.
var IssuesColumnList = []string{
	"id", "title", "description", "status", "priority", "issue_type",
	"assignee", "estimated_minutes", "created_at", "updated_at",
	"due_at", "closed_at", "external_ref", "compaction_level",
	"compacted_at", "compacted_at_commit", "original_size",
	"design", "acceptance_criteria", "notes", "source_repo",
	"close_reason",
	"await_type", "await_id", "timeout_ns",
	"ephemeral", "is_template", "mol_type",
	"metadata", "created_by_session", "claimed_by_session", "closed_by_session",
	"created_by",
}

// IssuesColumns is the same list joined as a SQL select expression.
// Use this for queries against a single database whose schema is known
// to match. For UNION queries across databases that may differ, build
// per-database expressions via selectColumnExprs.
var IssuesColumns = strings.Join(IssuesColumnList, ", ")
