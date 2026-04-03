package datasource

// IssuesColumns is the canonical column list for the issues table.
// Used by both DoltReader and GlobalDoltReader. One place to update
// when beads adds columns upstream.
//
// Column order must match the scan order in DoltReader.LoadIssuesFiltered
// and GlobalDoltReader.scanIssue.
const IssuesColumns = `id, title, description, status, priority, issue_type,
	assignee, estimated_minutes, created_at, updated_at,
	due_at, closed_at, external_ref, compaction_level,
	compacted_at, compacted_at_commit, original_size,
	design, acceptance_criteria, notes, source_repo,
	close_reason`
