package datasource

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// systemDatabases is the deny-list of non-beads databases on a Dolt server.
var systemDatabases = map[string]bool{
	"information_schema": true,
	"mysql":              true,
	"dolt":               true,
	"dolt_procedures":    true,
	"sys":                true,
}

// GlobalDoltReader connects to a shared Dolt server without selecting a
// specific database, enumerates all beads project databases, and loads
// issues from all of them via UNION ALL queries.
type GlobalDoltReader struct {
	db        *sql.DB
	databases []string // validated beads databases
	dsn       string
}

// DiscoverSharedServer locates the shared Dolt server's host and port.
// Priority: BT_GLOBAL_DOLT_PORT env var > ~/.beads/shared-server/dolt-server.port file.
// Returns an error immediately when BT_TEST_MODE=1, preventing e2e tests from
// accidentally connecting to the developer's shared Dolt server instead of their
// JSONL fixtures.
func DiscoverSharedServer() (host string, port int, err error) {
	if os.Getenv("BT_TEST_MODE") == "1" {
		return "", 0, fmt.Errorf("shared Dolt server discovery disabled in test mode (BT_TEST_MODE=1)")
	}

	host = "127.0.0.1"

	// Env var override takes highest priority
	if v := os.Getenv("BT_GLOBAL_DOLT_PORT"); v != "" {
		p, parseErr := strconv.Atoi(strings.TrimSpace(v))
		if parseErr == nil && p > 0 {
			return host, p, nil
		}
	}

	// Read port from shared server port file
	home, err := os.UserHomeDir()
	if err != nil {
		return "", 0, fmt.Errorf("cannot determine home directory: %w", err)
	}
	portPath := filepath.Join(home, ".beads", "shared-server", "dolt-server.port")
	data, err := os.ReadFile(portPath)
	if err != nil {
		return "", 0, fmt.Errorf("shared Dolt server not running - ensure at least one project is configured with 'bd init --shared-server'")
	}
	p, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || p <= 0 {
		return "", 0, fmt.Errorf("invalid port in %s: %q", portPath, strings.TrimSpace(string(data)))
	}

	return host, p, nil
}

// globalDSN builds a DSN for the shared server without a specific database.
func globalDSN(host string, port int) string {
	return fmt.Sprintf("root@tcp(%s:%d)/?parseTime=true&timeout=2s", host, port)
}

// NewGlobalDataSource creates a DataSource configured for global mode.
func NewGlobalDataSource(host string, port int) DataSource {
	return DataSource{
		Type: SourceTypeDoltGlobal,
		Path: globalDSN(host, port),
	}
}

// NewGlobalDoltReader opens a connection to the shared Dolt server, enumerates
// databases, and validates that they contain beads data.
func NewGlobalDoltReader(source DataSource) (*GlobalDoltReader, error) {
	if source.Type != SourceTypeDoltGlobal {
		return nil, fmt.Errorf("source is not DoltGlobal: %s", source.Type)
	}

	dsn := source.Path // DSN is stored in Path
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("cannot open global Dolt connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("shared Dolt server not responding: %w", err)
	}

	databases, err := EnumerateDatabases(db, source.RepoFilter)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &GlobalDoltReader{db: db, databases: databases, dsn: dsn}, nil
}

// EnumerateDatabases discovers beads databases on the shared server.
// Uses information_schema to find databases containing an issues table in a
// single query. If repoFilter is non-empty, only databases matching it
// (case-insensitive) are included.
func EnumerateDatabases(db *sql.DB, repoFilter string) ([]string, error) {
	query := `SELECT DISTINCT TABLE_SCHEMA FROM information_schema.tables
		WHERE TABLE_NAME = 'issues'
		AND TABLE_SCHEMA NOT IN ('information_schema','mysql','dolt','dolt_procedures','sys')`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("cannot enumerate databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		if systemDatabases[name] {
			continue
		}
		if repoFilter != "" && !strings.EqualFold(name, repoFilter) {
			continue
		}
		databases = append(databases, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading database list: %w", err)
	}

	if len(databases) == 0 {
		if repoFilter != "" {
			return nil, fmt.Errorf("no beads database %q found on shared server", repoFilter)
		}
		return nil, fmt.Errorf("no beads databases found on shared server")
	}

	slog.Info("global mode: discovered databases", "count", len(databases), "databases", databases)

	return databases, nil
}

// FilterSystemDatabases removes system/internal databases from a list.
// Exported for testing.
func FilterSystemDatabases(names []string) []string {
	var filtered []string
	for _, name := range names {
		if !systemDatabases[name] {
			filtered = append(filtered, name)
		}
	}
	return filtered
}

// LoadIssues loads all issues from all enumerated databases via UNION ALL.
func (r *GlobalDoltReader) LoadIssues() ([]model.Issue, error) {
	if len(r.databases) == 0 {
		return nil, fmt.Errorf("no databases to query")
	}

	query, err := buildIssuesQuery(r.databases)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("global issues query failed: %w", err)
	}
	defer rows.Close()

	var issues []model.Issue
	for rows.Next() {
		issue, err := scanGlobalIssue(rows)
		if err != nil {
			slog.Warn("skipping issue row", "error", err)
			continue
		}
		issues = append(issues, *issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating global issues: %w", err)
	}

	// Build issueMap after the loop so all pointers target the final
	// backing array. Building it during append causes pointer invalidation
	// when the slice grows - batch-loaded deps/labels/comments would be
	// written to stale copies that the returned slice never sees.
	issueMap := make(map[string]*model.Issue, len(issues))
	for i := range issues {
		issueMap[issues[i].ID] = &issues[i]
	}

	// Batch load labels, dependencies, comments
	if err := r.loadAllLabels(issueMap); err != nil {
		slog.Warn("failed to load labels", "error", err)
	}
	if err := r.loadAllDependencies(issueMap); err != nil {
		slog.Warn("failed to load dependencies", "error", err)
	}
	if err := r.loadAllComments(issueMap); err != nil {
		slog.Warn("failed to load comments", "error", err)
	}

	return issues, nil
}

// LoadIssuesAsOf loads issues from all databases at a historical point in time
// using Dolt's AS OF syntax. Each database is queried individually (not UNION ALL)
// because databases may have different commit histories - a timestamp valid for
// one database may have no corresponding commit in another.
//
// Databases that fail (no commit at timestamp, schema mismatch) are skipped with
// a log warning. The method only returns an error if ALL databases fail.
func (r *GlobalDoltReader) LoadIssuesAsOf(timestamp time.Time) ([]model.Issue, error) {
	if len(r.databases) == 0 {
		return nil, fmt.Errorf("no databases to query")
	}

	tsStr := timestamp.UTC().Format("2006-01-02T15:04:05")
	var allIssues []model.Issue
	var dbErrors []string
	successCount := 0

	for _, dbName := range r.databases {
		query := buildIssuesQueryAsOf(dbName, tsStr)
		rows, err := r.db.Query(query)
		if err != nil {
			slog.Warn("AS OF query failed for database",
				"database", dbName, "timestamp", tsStr, "error", err)
			dbErrors = append(dbErrors, fmt.Sprintf("%s: %v", dbName, err))
			continue
		}

		var dbIssues []model.Issue
		for rows.Next() {
			issue, scanErr := scanGlobalIssue(rows)
			if scanErr != nil {
				slog.Warn("skipping issue row in AS OF query",
					"database", dbName, "error", scanErr)
				continue
			}
			dbIssues = append(dbIssues, *issue)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			slog.Warn("AS OF row iteration error",
				"database", dbName, "error", err)
			dbErrors = append(dbErrors, fmt.Sprintf("%s: %v", dbName, err))
			continue
		}
		rows.Close()

		allIssues = append(allIssues, dbIssues...)
		successCount++
	}

	if successCount == 0 {
		return nil, fmt.Errorf("AS OF query failed for all %d databases at %s: %s",
			len(r.databases), tsStr, strings.Join(dbErrors, "; "))
	}

	if len(dbErrors) > 0 {
		slog.Info("AS OF query partial success",
			"timestamp", tsStr,
			"succeeded", successCount,
			"failed", len(dbErrors))
	}

	return allIssues, nil
}

// Databases returns the list of discovered database names.
// Used by TemporalCache to report which databases are available.
func (r *GlobalDoltReader) Databases() []string {
	return r.databases
}

// buildIssuesQueryAsOf generates an AS OF query for a single database.
// Uses the same IssuesColumns as the regular query plus _global_source.
// Dolt AS OF syntax: SELECT ... FROM `db`.issues AS OF '<timestamp>'
func buildIssuesQueryAsOf(dbName, tsStr string) string {
	quoted := backtickQuote(dbName)
	return fmt.Sprintf("SELECT %s, '%s' AS _global_source FROM %s.issues AS OF '%s' WHERE status != 'tombstone'",
		IssuesColumns, escapeSQLString(dbName), quoted, escapeSQLString(tsStr))
}

// buildIssuesQuery generates a UNION ALL query across all databases.
func buildIssuesQuery(databases []string) (string, error) {
	if len(databases) == 0 {
		return "", fmt.Errorf("no databases provided")
	}

	var parts []string
	for _, db := range databases {
		quoted := backtickQuote(db)
		part := fmt.Sprintf("SELECT %s, '%s' AS _global_source FROM %s.issues WHERE status != 'tombstone'",
			IssuesColumns, escapeSQLString(db), quoted)
		parts = append(parts, part)
	}

	return strings.Join(parts, " UNION ALL ") + " ORDER BY updated_at DESC", nil
}

// buildLabelsQuery generates a UNION ALL query for labels across all databases.
func buildLabelsQuery(databases []string) (string, error) {
	if len(databases) == 0 {
		return "", fmt.Errorf("no databases provided")
	}

	var parts []string
	for _, db := range databases {
		quoted := backtickQuote(db)
		part := fmt.Sprintf("SELECT issue_id, label, '%s' AS _db FROM %s.labels",
			escapeSQLString(db), quoted)
		parts = append(parts, part)
	}

	return strings.Join(parts, " UNION ALL "), nil
}

// buildDependenciesQuery generates a UNION ALL query for dependencies across all databases.
func buildDependenciesQuery(databases []string) (string, error) {
	if len(databases) == 0 {
		return "", fmt.Errorf("no databases provided")
	}

	var parts []string
	for _, db := range databases {
		quoted := backtickQuote(db)
		// Dolt uses `type`, not `dependency_type`
		part := fmt.Sprintf("SELECT issue_id, depends_on_id, type, '%s' AS _db FROM %s.dependencies",
			escapeSQLString(db), quoted)
		parts = append(parts, part)
	}

	return strings.Join(parts, " UNION ALL "), nil
}

// buildCommentsQuery generates a UNION ALL query for comments across all databases.
func buildCommentsQuery(databases []string) (string, error) {
	if len(databases) == 0 {
		return "", fmt.Errorf("no databases provided")
	}

	var parts []string
	for _, db := range databases {
		quoted := backtickQuote(db)
		part := fmt.Sprintf("SELECT id, issue_id, author, text, created_at, '%s' AS _db FROM %s.comments",
			escapeSQLString(db), quoted)
		parts = append(parts, part)
	}

	return strings.Join(parts, " UNION ALL ") + " ORDER BY created_at", nil
}

// buildLastModifiedQuery generates a query for the aggregate MAX(updated_at) across all databases.
func buildLastModifiedQuery(databases []string) (string, error) {
	if len(databases) == 0 {
		return "", fmt.Errorf("no databases provided")
	}

	var parts []string
	for _, db := range databases {
		quoted := backtickQuote(db)
		parts = append(parts, fmt.Sprintf("SELECT MAX(updated_at) AS m FROM %s.issues", quoted))
	}

	return "SELECT MAX(m) FROM (" + strings.Join(parts, " UNION ALL ") + ") t", nil
}

// GetLastModified returns the most recent update time across all databases.
func (r *GlobalDoltReader) GetLastModified() (time.Time, error) {
	query, err := buildLastModifiedQuery(r.databases)
	if err != nil {
		return time.Time{}, err
	}

	var updatedAt sql.NullTime
	err = r.db.QueryRow(query).Scan(&updatedAt)
	if err != nil {
		return time.Time{}, err
	}
	if !updatedAt.Valid {
		return time.Time{}, nil // All databases empty
	}
	return updatedAt.Time, nil
}

// Close closes the database connection.
func (r *GlobalDoltReader) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// scanGlobalIssue scans a single issue row from the global UNION ALL query.
// The row has all IssuesColumns plus _global_source at the end.
func scanGlobalIssue(rows *sql.Rows) (*model.Issue, error) {
	var issue model.Issue
	var estimatedMinutes, compactionLevel, originalSize sql.NullInt64
	var createdAt, updatedAt, dueAt, closedAt, compactedAt sql.NullTime
	var description, assignee, externalRef, design, acceptanceCriteria, notes, sourceRepo, compactedAtCommit, closeReason sql.NullString
	var issueType string
	var globalSource string

	// Gate/molecule columns (must match IssuesColumns order)
	var awaitType, awaitID, molType sql.NullString
	var timeoutNs sql.NullInt64
	var ephemeral, isTemplate sql.NullBool

	err := rows.Scan(
		&issue.ID, &issue.Title, &description, &issue.Status, &issue.Priority, &issueType,
		&assignee, &estimatedMinutes, &createdAt, &updatedAt,
		&dueAt, &closedAt, &externalRef, &compactionLevel,
		&compactedAt, &compactedAtCommit, &originalSize,
		&design, &acceptanceCriteria, &notes, &sourceRepo,
		&closeReason,
		&awaitType, &awaitID, &timeoutNs,
		&ephemeral, &isTemplate, &molType,
		&globalSource,
	)
	if err != nil {
		return nil, err
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

	// SourceRepo always comes from the database name in global mode
	issue.SourceRepo = globalSource

	return &issue, nil
}

// loadAllLabels batch-loads labels for all issues via UNION ALL.
func (r *GlobalDoltReader) loadAllLabels(issueMap map[string]*model.Issue) error {
	query, err := buildLabelsQuery(r.databases)
	if err != nil {
		return err
	}

	rows, err := r.db.Query(query)
	if err != nil {
		return fmt.Errorf("global labels query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var issueID, label, db string
		if err := rows.Scan(&issueID, &label, &db); err != nil {
			continue
		}
		if issue, ok := issueMap[issueID]; ok {
			issue.Labels = append(issue.Labels, label)
		}
	}
	return rows.Err()
}

// loadAllDependencies batch-loads dependencies for all issues via UNION ALL.
func (r *GlobalDoltReader) loadAllDependencies(issueMap map[string]*model.Issue) error {
	query, err := buildDependenciesQuery(r.databases)
	if err != nil {
		return err
	}

	rows, err := r.db.Query(query)
	if err != nil {
		return fmt.Errorf("global dependencies query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var issueID, dependsOnID, depType, db string
		if err := rows.Scan(&issueID, &dependsOnID, &depType, &db); err != nil {
			continue
		}
		if issue, ok := issueMap[issueID]; ok {
			issue.Dependencies = append(issue.Dependencies, &model.Dependency{
				IssueID:     issueID,
				DependsOnID: dependsOnID,
				Type:        model.DependencyType(depType),
			})
		}
	}
	return rows.Err()
}

// loadAllComments batch-loads comments for all issues via UNION ALL.
func (r *GlobalDoltReader) loadAllComments(issueMap map[string]*model.Issue) error {
	query, err := buildCommentsQuery(r.databases)
	if err != nil {
		return err
	}

	rows, err := r.db.Query(query)
	if err != nil {
		return fmt.Errorf("global comments query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var comment model.Comment
		var createdAt sql.NullTime
		var db string
		if err := rows.Scan(&comment.ID, &comment.IssueID, &comment.Author, &comment.Text, &createdAt, &db); err != nil {
			continue
		}
		if createdAt.Valid {
			comment.CreatedAt = createdAt.Time
		}
		if issue, ok := issueMap[comment.IssueID]; ok {
			issue.Comments = append(issue.Comments, &comment)
		}
	}
	return rows.Err()
}

// backtickQuote wraps a database name in backticks for safe SQL use.
func backtickQuote(name string) string {
	// Escape any backticks within the name
	escaped := strings.ReplaceAll(name, "`", "``")
	return "`" + escaped + "`"
}

// escapeSQLString escapes single quotes in a string for SQL string literals.
func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
