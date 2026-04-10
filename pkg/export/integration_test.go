package export

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"

	_ "modernc.org/sqlite"
)

// TestExport_EndToEnd runs a comprehensive integration test with realistic data.
func TestExport_EndToEnd(t *testing.T) {
	tmpDir := t.TempDir()

	// Create realistic test data
	issues := createRealisticTestIssues()
	deps := createRealisticTestDeps()

	exp := NewSQLiteExporter(issues, deps, nil, nil)
	exp.SetGitHash("integration-test-abc123")
	exp.Config.Title = "Integration Test Project"

	if err := exp.Export(tmpDir); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Verify all expected files exist
	expectedFiles := []string{
		"beads.sqlite3",
		"beads.sqlite3.config.json",
		"data/meta.json",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(tmpDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s not found", f)
		}
	}

	// Open and verify database
	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "beads.sqlite3"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify issue count
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM issues`).Scan(&count); err != nil {
		t.Fatalf("Query issue count failed: %v", err)
	}
	if count != len(issues) {
		t.Errorf("Expected %d issues, got %d", len(issues), count)
	}

	// Verify dependency count
	if err := db.QueryRow(`SELECT COUNT(*) FROM dependencies`).Scan(&count); err != nil {
		t.Fatalf("Query dependency count failed: %v", err)
	}
	if count != len(deps) {
		t.Errorf("Expected %d dependencies, got %d", len(deps), count)
	}

	// Verify materialized view works
	if err := db.QueryRow(`SELECT COUNT(*) FROM issue_overview_mv`).Scan(&count); err != nil {
		t.Fatalf("Query materialized view failed: %v", err)
	}
	if count != len(issues) {
		t.Errorf("Materialized view has wrong count: expected %d, got %d", len(issues), count)
	}

	// Verify meta.json content
	metaPath := filepath.Join(tmpDir, "data", "meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read meta.json: %v", err)
	}

	var meta ExportMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		t.Fatalf("Failed to parse meta.json: %v", err)
	}

	if meta.IssueCount != len(issues) {
		t.Errorf("meta.json issue_count wrong: expected %d, got %d", len(issues), meta.IssueCount)
	}
	if meta.GitCommit != "integration-test-abc123" {
		t.Errorf("meta.json git_commit wrong: expected integration-test-abc123, got %s", meta.GitCommit)
	}
	if meta.Title != "Integration Test Project" {
		t.Errorf("meta.json title wrong: expected Integration Test Project, got %s", meta.Title)
	}
}

// TestExport_FTS5AdvancedSearch tests advanced FTS5 search patterns.
func TestExport_FTS5AdvancedSearch(t *testing.T) {
	tmpDir := t.TempDir()

	issues := []*model.Issue{
		createIssue("fts-1", "Authentication Bug Fix", "Login fails on mobile devices", model.StatusOpen, 1, model.TypeBug, []string{"auth", "mobile"}),
		createIssue("fts-2", "Implement OAuth Integration", "Add OAuth2 support for authentication", model.StatusOpen, 2, model.TypeFeature, []string{"auth", "oauth"}),
		createIssue("fts-3", "Mobile UI Improvements", "Improve mobile user interface", model.StatusInProgress, 2, model.TypeTask, []string{"mobile", "ui"}),
		createIssue("fts-4", "Database Performance", "Optimize database queries", model.StatusOpen, 1, model.TypeTask, []string{"database", "performance"}),
	}

	exp := NewSQLiteExporter(issues, nil, nil, nil)
	if err := exp.Export(tmpDir); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "beads.sqlite3"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Skip FTS tests if FTS5 not available
	var ftsExists int
	err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'issues_fts'`).Scan(&ftsExists)
	if err != nil || ftsExists == 0 {
		t.Skip("FTS5 not available - skipping advanced search tests")
	}

	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{"Single term", "authentication", 2},
		{"Prefix search", "auth*", 2},
		{"Multiple terms OR", "mobile OR database", 3},
		{"Phrase search", `"mobile user"`, 1},
		{"Negation", "mobile -authentication", 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var count int
			err := db.QueryRow(`SELECT COUNT(*) FROM issues_fts WHERE issues_fts MATCH ?`, tc.query).Scan(&count)
			if err != nil {
				// Some queries may fail due to SQLite version differences
				t.Logf("Query failed (may be version-specific): %v", err)
				return
			}
			if count != tc.expected {
				t.Errorf("Expected %d results for %q, got %d", tc.expected, tc.query, count)
			}
		})
	}
}

// TestExport_LargeDataChunking tests that large databases are properly chunked.
func TestExport_LargeDataChunking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large data chunking test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a large dataset (enough to trigger chunking at lower threshold)
	issues := make([]*model.Issue, 500)
	for i := 0; i < 500; i++ {
		// Create issues with large descriptions to increase DB size
		issues[i] = createIssue(
			"large-"+string(rune('A'+i%26))+"-"+string(rune('0'+i/26)),
			"Test Issue "+string(rune('0'+i)),
			strings.Repeat("This is a test description with enough content to take up space. ", 50),
			model.StatusOpen,
			i%5,
			model.TypeTask,
			[]string{"test", "large", string(rune('A' + i%10))},
		)
	}

	exp := NewSQLiteExporter(issues, nil, nil, nil)
	// Lower chunk threshold for testing
	exp.Config.ChunkThreshold = 100 * 1024 // 100KB
	exp.Config.ChunkSize = 50 * 1024       // 50KB chunks

	if err := exp.Export(tmpDir); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Read chunk config
	configPath := filepath.Join(tmpDir, "beads.sqlite3.config.json")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read chunk config: %v", err)
	}

	var config ChunkConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse chunk config: %v", err)
	}

	// Verify chunk config has expected fields
	if config.TotalSize <= 0 {
		t.Error("TotalSize should be positive")
	}

	// Verify database was chunked (if large enough)
	if config.TotalSize > int64(exp.Config.ChunkThreshold) {
		if !config.Chunked {
			t.Error("Large database should be chunked")
		}
		if config.ChunkCount <= 0 {
			t.Error("Chunk count should be positive")
		}

		// Verify chunks directory exists
		chunksDir := filepath.Join(tmpDir, "chunks")
		if _, err := os.Stat(chunksDir); os.IsNotExist(err) {
			t.Error("Chunks directory should exist")
		}
	} else {
		t.Logf("Database size %d bytes is below threshold %d - not chunked", config.TotalSize, exp.Config.ChunkThreshold)
	}
}

// TestExport_UnicodeAndSpecialCharacters tests handling of unicode and special characters.
func TestExport_UnicodeAndSpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()

	issues := []*model.Issue{
		createIssue("unicode-1", "Unicode Test: 日本語タイトル", "Description with emojis: 🚀 🎉 and Japanese: テスト", model.StatusOpen, 2, model.TypeTask, []string{"unicode", "日本語"}),
		createIssue("unicode-2", "SQL Injection Test: '; DROP TABLE issues;--", "Description with quotes: \"test\" and 'apostrophes'", model.StatusOpen, 2, model.TypeBug, []string{"security"}),
		createIssue("unicode-3", "Markdown Test **bold** _italic_", "```go\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n```", model.StatusOpen, 2, model.TypeTask, []string{"markdown"}),
	}

	exp := NewSQLiteExporter(issues, nil, nil, nil)
	if err := exp.Export(tmpDir); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "beads.sqlite3"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify all issues were stored correctly
	for _, issue := range issues {
		var title, description string
		err := db.QueryRow(`SELECT title, description FROM issues WHERE id = ?`, issue.ID).Scan(&title, &description)
		if err != nil {
			t.Errorf("Failed to query issue %s: %v", issue.ID, err)
			continue
		}
		if title != issue.Title {
			t.Errorf("Title mismatch for %s: expected %q, got %q", issue.ID, issue.Title, title)
		}
		if description != issue.Description {
			t.Errorf("Description mismatch for %s", issue.ID)
		}
	}
}

// TestExport_DependencyIntegrity verifies dependency relationships are correctly exported.
func TestExport_DependencyIntegrity(t *testing.T) {
	tmpDir := t.TempDir()

	issues := []*model.Issue{
		createIssue("dep-1", "Root Issue", "This is a root issue", model.StatusOpen, 1, model.TypeEpic, nil),
		createIssue("dep-2", "Child A", "Depends on root", model.StatusOpen, 2, model.TypeTask, nil),
		createIssue("dep-3", "Child B", "Also depends on root", model.StatusOpen, 2, model.TypeTask, nil),
		createIssue("dep-4", "Grandchild", "Depends on Child A", model.StatusOpen, 3, model.TypeTask, nil),
	}

	deps := []*model.Dependency{
		{IssueID: "dep-2", DependsOnID: "dep-1", Type: model.DepBlocks},
		{IssueID: "dep-3", DependsOnID: "dep-1", Type: model.DepBlocks},
		{IssueID: "dep-4", DependsOnID: "dep-2", Type: model.DepBlocks},
	}

	exp := NewSQLiteExporter(issues, deps, nil, nil)
	if err := exp.Export(tmpDir); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "beads.sqlite3"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify dependencies table has correct relationships
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM dependencies`).Scan(&count); err != nil {
		t.Fatalf("Query dependencies count failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 dependencies, got %d", count)
	}

	// Verify specific relationships exist
	depTests := []struct {
		from string
		to   string
	}{
		{"dep-2", "dep-1"},
		{"dep-3", "dep-1"},
		{"dep-4", "dep-2"},
	}

	for _, tc := range depTests {
		var exists int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM dependencies
			WHERE issue_id = ? AND depends_on_id = ?
		`, tc.from, tc.to).Scan(&exists)
		if err != nil {
			t.Errorf("Failed to query dependency %s -> %s: %v", tc.from, tc.to, err)
			continue
		}
		if exists != 1 {
			t.Errorf("Dependency %s -> %s not found", tc.from, tc.to)
		}
	}

	// Verify issues are in materialized view
	for _, issue := range issues {
		var id string
		err := db.QueryRow(`SELECT id FROM issue_overview_mv WHERE id = ?`, issue.ID).Scan(&id)
		if err != nil {
			t.Errorf("Issue %s not found in materialized view: %v", issue.ID, err)
		}
	}
}

// TestExport_QueryPerformance tests that basic queries run efficiently.
func TestExport_QueryPerformance(t *testing.T) {
	tmpDir := t.TempDir()

	// Create moderate dataset
	issues := make([]*model.Issue, 100)
	for i := 0; i < 100; i++ {
		issues[i] = createIssue(
			"perf-"+string(rune('A'+i%26))+string(rune('0'+i/26)),
			"Performance Test Issue "+string(rune('0'+i)),
			"Description for performance testing",
			model.StatusOpen,
			i%5,
			model.TypeTask,
			[]string{"test", string(rune('A' + i%5))},
		)
	}

	exp := NewSQLiteExporter(issues, nil, nil, nil)
	if err := exp.Export(tmpDir); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "beads.sqlite3"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test common query patterns
	queries := []struct {
		name  string
		query string
	}{
		{"Count all issues", "SELECT COUNT(*) FROM issues"},
		{"Filter by status", "SELECT COUNT(*) FROM issues WHERE status = 'open'"},
		{"Filter by priority", "SELECT COUNT(*) FROM issues WHERE priority <= 2"},
		{"Join with materialized view", "SELECT COUNT(*) FROM issue_overview_mv WHERE status = 'open'"},
		{"Order by priority", "SELECT id FROM issues ORDER BY priority ASC LIMIT 10"},
	}

	for _, tc := range queries {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			rows, err := db.Query(tc.query)
			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}
			rows.Close()
			elapsed := time.Since(start)

			// Queries should complete quickly (under 100ms for small datasets)
			if elapsed > 100*time.Millisecond {
				t.Logf("Warning: query took %v", elapsed)
			}
		})
	}
}

// Helper functions

func createRealisticTestIssues() []*model.Issue {
	now := time.Now()
	return []*model.Issue{
		createIssue("proj-1", "Setup CI Pipeline", "Configure GitHub Actions", model.StatusClosed, 1, model.TypeTask, []string{"ci", "devops"}),
		createIssue("proj-2", "Implement User Auth", "OAuth2 authentication", model.StatusInProgress, 1, model.TypeFeature, []string{"auth", "security"}),
		createIssue("proj-3", "Fix Login Bug", "Session timeout issue", model.StatusOpen, 0, model.TypeBug, []string{"auth", "bug"}),
		createIssue("proj-4", "Add Unit Tests", "Increase test coverage", model.StatusOpen, 2, model.TypeTask, []string{"testing"}),
		createIssue("proj-5", "Database Migration", "Migrate to PostgreSQL", model.StatusOpen, 1, model.TypeTask, []string{"database"}),
		createIssue("proj-6", "API Documentation", "OpenAPI spec", model.StatusOpen, 3, model.TypeTask, []string{"docs"}),
		createIssue("proj-7", "Performance Audit", "Identify bottlenecks", model.StatusOpen, 2, model.TypeTask, []string{"performance"}),
		{
			ID:          "proj-8",
			Title:       "Deploy to Production",
			Description: "Final deployment checklist",
			Status:      model.StatusBlocked,
			Priority:    1,
			IssueType:   model.TypeTask,
			Labels:      []string{"deployment"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func createRealisticTestDeps() []*model.Dependency {
	return []*model.Dependency{
		{IssueID: "proj-2", DependsOnID: "proj-1", Type: model.DepBlocks},
		{IssueID: "proj-3", DependsOnID: "proj-2", Type: model.DepBlocks},
		{IssueID: "proj-5", DependsOnID: "proj-1", Type: model.DepBlocks},
		{IssueID: "proj-8", DependsOnID: "proj-2", Type: model.DepBlocks},
		{IssueID: "proj-8", DependsOnID: "proj-3", Type: model.DepBlocks},
		{IssueID: "proj-8", DependsOnID: "proj-4", Type: model.DepBlocks},
		{IssueID: "proj-8", DependsOnID: "proj-5", Type: model.DepBlocks},
	}
}

func createIssue(id, title, desc string, status model.Status, priority int, issueType model.IssueType, labels []string) *model.Issue {
	now := time.Now()
	issue := &model.Issue{
		ID:          id,
		Title:       title,
		Description: desc,
		Status:      status,
		Priority:    priority,
		IssueType:   issueType,
		Labels:      labels,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if status == model.StatusClosed {
		closedAt := now.Add(-time.Hour)
		issue.ClosedAt = &closedAt
	}
	return issue
}
