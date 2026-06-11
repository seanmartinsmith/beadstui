package datasource

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
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
	db            *sql.DB
	dsn           string
	availableCols map[string]bool // issues-table columns present on this server (bt-edi)
	depCols       map[string]bool // dependencies-table columns present (bt-yboer)
	labelCols     map[string]bool // labels-table columns present (bt-2qwo1)
	commentCols   map[string]bool // comments-table columns present (bt-2qwo1)
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

	// Probe issues-table columns so the scan path can NULL-substitute any
	// missing ones (bt-edi). Mirrors the multi-DB behavior in global_dolt.go;
	// keeps bt resilient when upstream beads drops or renames a column.
	availableCols, err := loadTableColumns(db, "issues")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot probe issues columns: %w", err)
	}

	// Probe dependencies-table columns so the dependency read adapts to the
	// schema-v50 polymorphic target split (bt-yboer). A missing table yields
	// an empty set, which loadDependencies tolerates.
	depCols, err := loadTableColumns(db, "dependencies")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot probe dependencies columns: %w", err)
	}

	// Probe labels/comments columns so those reads NULL-substitute a dropped or
	// renamed column instead of failing the query (bt-2qwo1), mirroring the
	// issues and dependencies probes above.
	labelCols, err := loadTableColumns(db, "labels")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot probe labels columns: %w", err)
	}
	commentCols, err := loadTableColumns(db, "comments")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot probe comments columns: %w", err)
	}

	return &DoltReader{
		db:            db,
		dsn:           source.Path,
		availableCols: availableCols,
		depCols:       depCols,
		labelCols:     labelCols,
		commentCols:   commentCols,
	}, nil
}

// loadTableColumns returns the set of column names present on the named table
// in the current database. Empty set if the table does not exist.
//
// Uses string interpolation, not a placeholder: parameterized queries can be
// unreliable against Dolt's information_schema (see databasesWithTable). table
// is an internal constant ("issues", "dependencies"), never user input.
func loadTableColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(fmt.Sprintf(
		`SELECT COLUMN_NAME FROM information_schema.columns
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '%s'`, escapeSQLString(table)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// Close closes the database connection.
func (r *DoltReader) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// DB returns the underlying *sql.DB so that out-of-package callers (e.g.
// pkg/correlation's Dolt-native extractor, wired through bt-08sh.4) can
// borrow the already-open connection instead of opening a second one.
//
// Borrowed, not owned: the caller must not Close() the returned handle.
func (r *DoltReader) DB() *sql.DB { return r.db }

// LoadIssues reads all non-tombstone issues.
func (r *DoltReader) LoadIssues() ([]model.Issue, error) {
	return r.LoadIssuesFiltered(nil)
}

// buildSingleDBIssuesQuery returns the SELECT for the single-DB scan path,
// NULL-substituting any IssuesColumnList entries absent from `available`.
// Extracted so the schema-drift behavior is unit-testable without a live
// Dolt server (bt-edi).
func buildSingleDBIssuesQuery(available map[string]bool) string {
	return `SELECT ` + selectColumnExprs(IssuesColumnList, available) + `
		FROM issues
		WHERE status != 'tombstone'
		ORDER BY updated_at DESC`
}

// LoadIssuesFiltered reads issues matching an optional filter function.
func (r *DoltReader) LoadIssuesFiltered(filter func(*model.Issue) bool) ([]model.Issue, error) {
	query := buildSingleDBIssuesQuery(r.availableCols)

	rows, err := r.db.Query(query)
	if err != nil {
		// The primary query already NULL-substitutes missing columns (bt-edi),
		// so reaching here means a non-column failure. Surface it instead of
		// degrading silently, then fall back to the reduced column set (bt-ws2g).
		slog.Warn("single-DB issues query failed; falling back to reduced column set",
			"error", err)
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

		// Load relations even on the fallback path so a degraded read still
		// shows edges and comments instead of a plausible-but-wrong empty graph
		// (bt-ws2g). These readers are independently schema-drift tolerant.
		issue.Labels = r.loadLabels(issue.ID)
		issue.Dependencies = r.loadDependencies(issue.ID)
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

// loadLabels reads labels from the separate labels table. The label column is
// selected defensively (NULL-substituted if absent) so a schema drift degrades
// to empty labels rather than failing the query (bt-2qwo1).
func (r *DoltReader) loadLabels(issueID string) []string {
	query := "SELECT " + selectColumnExprs([]string{"label"}, r.labelCols) + " FROM labels WHERE issue_id = ?"
	rows, err := r.db.Query(query, issueID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var labels []string
	for rows.Next() {
		var label sql.NullString
		if err := rows.Scan(&label); err != nil {
			continue
		}
		if !label.Valid || label.String == "" {
			continue
		}
		labels = append(labels, label.String)
	}
	return labels
}

// loadDependencies reads dependencies (uses `type` column, not `dependency_type`).
// The target id is resolved via dependsOnTargetExpr to absorb the schema-v50
// depends_on_id -> {issue,external,wisp} split (bt-yboer). Wisp-only edges
// resolve to NULL and are skipped: bt has no wisp surface.
func (r *DoltReader) loadDependencies(issueID string) []*model.Dependency {
	query := "SELECT " + dependsOnTargetExpr(r.depCols) + ", type FROM dependencies WHERE issue_id = ?"
	rows, err := r.db.Query(query, issueID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var deps []*model.Dependency
	for rows.Next() {
		var dependsOnID sql.NullString
		var depType string
		if err := rows.Scan(&dependsOnID, &depType); err != nil {
			continue
		}
		if !dependsOnID.Valid || dependsOnID.String == "" {
			continue
		}
		deps = append(deps, &model.Dependency{
			IssueID:     issueID,
			DependsOnID: dependsOnID.String,
			Type:        model.DependencyType(depType),
		})
	}
	return deps
}

// loadComments reads comments for an issue. Columns are selected defensively
// (NULL-substituted if absent) so a schema drift degrades to empty fields
// rather than failing the query (bt-2qwo1).
func (r *DoltReader) loadComments(issueID string) []*model.Comment {
	query := "SELECT " + selectColumnExprs([]string{"id", "author", "text", "created_at"}, r.commentCols) +
		" FROM comments WHERE issue_id = ? ORDER BY created_at"
	rows, err := r.db.Query(query, issueID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var comments []*model.Comment
	for rows.Next() {
		var comment model.Comment
		var id, author, text sql.NullString
		var createdAt sql.NullTime
		if err := rows.Scan(&id, &author, &text, &createdAt); err != nil {
			continue
		}
		comment.ID = id.String
		comment.Author = author.String
		comment.Text = text.String
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
