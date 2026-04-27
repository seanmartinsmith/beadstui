package datasource

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// parseIssueMetadata decodes an upstream-beads JSON metadata blob into the
// map-of-RawMessage form carried on Issue.Metadata. Empty / invalid blobs
// return nil — callers leave the field zero rather than surface a scan error.
func parseIssueMetadata(raw sql.NullString) map[string]json.RawMessage {
	if !raw.Valid || raw.String == "" || raw.String == "{}" {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw.String), &m); err != nil {
		return nil
	}
	return m
}

// DoltReader provides read access to a Dolt SQL server.
type DoltReader struct {
	db  *sql.DB
	dsn string
}

// NewDoltReader opens a MySQL connection to the running Dolt server.
func NewDoltReader(source DataSource) (*DoltReader, error) {
	if source.Type != SourceTypeDolt {
		return nil, fmt.Errorf("source is not Dolt: %s", source.Type)
	}

	db, err := sql.Open("mysql", source.Path) // Path holds the DSN
	if err != nil {
		return nil, fmt.Errorf("cannot open Dolt connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot reach Dolt server: %w", err)
	}

	// Verify this is actually a beads database, not a random MySQL service (bt-07jp)
	var tableName string
	err = db.QueryRow("SHOW TABLES LIKE 'issues'").Scan(&tableName)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("connected but no 'issues' table found - wrong database?")
	}

	return &DoltReader{db: db, dsn: source.Path}, nil
}

// Close closes the database connection.
func (r *DoltReader) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// LoadIssues reads all non-tombstone issues.
func (r *DoltReader) LoadIssues() ([]model.Issue, error) {
	return r.LoadIssuesFiltered(nil)
}

// LoadIssuesFiltered reads issues matching an optional filter function.
func (r *DoltReader) LoadIssuesFiltered(filter func(*model.Issue) bool) ([]model.Issue, error) {
	query := `SELECT ` + IssuesColumns + `
		FROM issues
		WHERE status != 'tombstone'
		ORDER BY updated_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return r.loadIssuesSimple(filter)
	}
	defer rows.Close()

	var issues []model.Issue
	for rows.Next() {
		var issue model.Issue
		var estimatedMinutes, compactionLevel, originalSize sql.NullInt64
		var createdAt, updatedAt, dueAt, closedAt, compactedAt sql.NullTime
		var description, assignee, externalRef, design, acceptanceCriteria, notes, sourceRepo, compactedAtCommit, closeReason sql.NullString
		var issueType string

		// Gate/molecule columns
		var awaitType, awaitID, molType sql.NullString
		var timeoutNs sql.NullInt64
		var ephemeral, isTemplate sql.NullBool

		// Session provenance columns (bt-5hl9): direct columns since bd-34v.
		var metadataRaw, createdBySession, claimedBySession, closedBySession sql.NullString

		// Author / creation-time actor (bt-aw4h) — sourced from the
		// beads `created_by` column. Separate from the `owner` column
		// which holds the GitHub commit identity.
		var createdBy sql.NullString

		err := rows.Scan(
			&issue.ID, &issue.Title, &description, &issue.Status, &issue.Priority, &issueType,
			&assignee, &estimatedMinutes, &createdAt, &updatedAt,
			&dueAt, &closedAt, &externalRef, &compactionLevel,
			&compactedAt, &compactedAtCommit, &originalSize,
			&design, &acceptanceCriteria, &notes, &sourceRepo,
			&closeReason,
			&awaitType, &awaitID, &timeoutNs,
			&ephemeral, &isTemplate, &molType,
			&metadataRaw, &createdBySession, &claimedBySession, &closedBySession,
			&createdBy,
		)
		if err != nil {
			continue
		}

		if description.Valid {
			issue.Description = description.String
		}
		issue.IssueType = model.IssueType(issueType)
		if assignee.Valid {
			issue.Assignee = assignee.String
		}
		if estimatedMinutes.Valid {
			v := int(estimatedMinutes.Int64)
			issue.EstimatedMinutes = &v
		}
		if createdAt.Valid {
			issue.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			issue.UpdatedAt = updatedAt.Time
		}
		if dueAt.Valid {
			t := dueAt.Time
			issue.DueDate = &t
		}
		if closedAt.Valid {
			t := closedAt.Time
			issue.ClosedAt = &t
		}
		if closeReason.Valid && closeReason.String != "" {
			s := closeReason.String
			issue.CloseReason = &s
		}
		if externalRef.Valid {
			s := externalRef.String
			issue.ExternalRef = &s
		}
		if compactionLevel.Valid {
			issue.CompactionLevel = int(compactionLevel.Int64)
		}
		if compactedAt.Valid {
			t := compactedAt.Time
			issue.CompactedAt = &t
		}
		if compactedAtCommit.Valid {
			s := compactedAtCommit.String
			issue.CompactedAtCommit = &s
		}
		if originalSize.Valid {
			issue.OriginalSize = int(originalSize.Int64)
		}
		if design.Valid {
			issue.Design = design.String
		}
		if acceptanceCriteria.Valid {
			issue.AcceptanceCriteria = acceptanceCriteria.String
		}
		if notes.Valid {
			issue.Notes = notes.String
		}
		if sourceRepo.Valid {
			issue.SourceRepo = sourceRepo.String
		}

		// Gate fields
		if awaitType.Valid && awaitType.String != "" {
			s := awaitType.String
			issue.AwaitType = &s
		}
		if awaitID.Valid && awaitID.String != "" {
			s := awaitID.String
			issue.AwaitID = &s
		}
		if timeoutNs.Valid && timeoutNs.Int64 != 0 {
			v := timeoutNs.Int64
			issue.TimeoutNs = &v
		}

		// Molecule/wisp fields
		if ephemeral.Valid && ephemeral.Bool {
			v := ephemeral.Bool
			issue.Ephemeral = &v
		}
		if isTemplate.Valid && isTemplate.Bool {
			v := isTemplate.Bool
			issue.IsTemplate = &v
		}
		if molType.Valid && molType.String != "" {
			s := molType.String
			issue.MolType = &s
		}

		// Session provenance (bt-5hl9): direct columns since bd-34v Phase 1a/1b
		// (fork-bd, tracked by bd-6in). Empty for beads predating the columns.
		issue.Metadata = parseIssueMetadata(metadataRaw)
		if createdBySession.Valid && createdBySession.String != "" {
			issue.CreatedBySession = createdBySession.String
		}
		if claimedBySession.Valid && claimedBySession.String != "" {
			issue.ClaimedBySession = claimedBySession.String
		}
		if closedBySession.Valid && closedBySession.String != "" {
			issue.ClosedBySession = closedBySession.String
		}
		if createdBy.Valid {
			issue.Author = createdBy.String
		}

		// Labels come from a separate table in Dolt
		issue.Labels = r.loadLabels(issue.ID)

		// Dependencies
		issue.Dependencies = r.loadDependencies(issue.ID)

		// Comments
		issue.Comments = r.loadComments(issue.ID)

		if filter != nil && !filter(&issue) {
			continue
		}

		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating issues: %w", err)
	}

	return issues, nil
}

// loadIssuesSimple is a fallback with fewer columns.
func (r *DoltReader) loadIssuesSimple(filter func(*model.Issue) bool) ([]model.Issue, error) {
	query := `
		SELECT id, title, description, status, priority, issue_type, created_at, updated_at
		FROM issues
		WHERE status != 'tombstone'
		ORDER BY updated_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var issues []model.Issue
	for rows.Next() {
		var issue model.Issue
		var description sql.NullString
		var createdAt, updatedAt sql.NullTime
		var issueType string

		err := rows.Scan(
			&issue.ID, &issue.Title, &description, &issue.Status, &issue.Priority, &issueType,
			&createdAt, &updatedAt,
		)
		if err != nil {
			continue
		}

		if description.Valid {
			issue.Description = description.String
		}
		issue.IssueType = model.IssueType(issueType)
		if createdAt.Valid {
			issue.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			issue.UpdatedAt = updatedAt.Time
		}

		issue.Labels = r.loadLabels(issue.ID)

		if filter != nil && !filter(&issue) {
			continue
		}

		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating issues: %w", err)
	}

	return issues, nil
}

// loadLabels reads labels from the separate labels table.
func (r *DoltReader) loadLabels(issueID string) []string {
	rows, err := r.db.Query("SELECT label FROM labels WHERE issue_id = ?", issueID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var labels []string
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			continue
		}
		labels = append(labels, label)
	}
	return labels
}

// loadDependencies reads dependencies (uses `type` column, not `dependency_type`).
func (r *DoltReader) loadDependencies(issueID string) []*model.Dependency {
	rows, err := r.db.Query("SELECT depends_on_id, type FROM dependencies WHERE issue_id = ?", issueID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var deps []*model.Dependency
	for rows.Next() {
		var dep model.Dependency
		var depType string
		if err := rows.Scan(&dep.DependsOnID, &depType); err != nil {
			continue
		}
		dep.IssueID = issueID
		dep.Type = model.DependencyType(depType)
		deps = append(deps, &dep)
	}
	return deps
}

// loadComments reads comments for an issue.
func (r *DoltReader) loadComments(issueID string) []*model.Comment {
	rows, err := r.db.Query("SELECT id, author, text, created_at FROM comments WHERE issue_id = ? ORDER BY created_at", issueID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var comments []*model.Comment
	for rows.Next() {
		var comment model.Comment
		var createdAt sql.NullTime
		if err := rows.Scan(&comment.ID, &comment.Author, &comment.Text, &createdAt); err != nil {
			continue
		}
		if createdAt.Valid {
			comment.CreatedAt = createdAt.Time
		}
		comment.IssueID = issueID
		comments = append(comments, &comment)
	}
	return comments
}

// CountIssues returns the count of non-tombstone issues.
func (r *DoltReader) CountIssues() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM issues WHERE status != 'tombstone'").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetIssueByID retrieves a single issue by ID.
func (r *DoltReader) GetIssueByID(id string) (*model.Issue, error) {
	issues, err := r.LoadIssuesFiltered(func(issue *model.Issue) bool {
		return issue.ID == id
	})
	if err != nil {
		return nil, err
	}
	if len(issues) == 0 {
		return nil, fmt.Errorf("issue not found: %s", id)
	}
	return &issues[0], nil
}

// GetLastModified returns the most recent modification time across issues and comments.
// Comments don't bump issues.updated_at, so we check both tables to detect
// comment-only changes (bt-ju7o).
func (r *DoltReader) GetLastModified() (time.Time, error) {
	var modTime sql.NullTime
	err := r.db.QueryRow(`
		SELECT GREATEST(
			COALESCE((SELECT MAX(updated_at) FROM issues), '1970-01-01'),
			COALESCE((SELECT MAX(created_at) FROM comments), '1970-01-01')
		)`).Scan(&modTime)
	if err != nil {
		return time.Time{}, err
	}
	if !modTime.Valid {
		return time.Time{}, nil
	}
	return modTime.Time, nil
}
