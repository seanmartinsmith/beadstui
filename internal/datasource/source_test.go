package datasource

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestDiscoverSources_OnlySQLite tests discovery with only a SQLite source
func TestDiscoverSources_OnlySQLite(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create SQLite database
	dbPath := filepath.Join(beadsDir, "beads.db")
	createTestSQLiteDB(t, dbPath)

	sources, err := DiscoverSources(DiscoveryOptions{
		BeadsDir:               beadsDir,
		ValidateAfterDiscovery: false,
	})
	if err != nil {
		t.Fatalf("DiscoverSources failed: %v", err)
	}

	if len(sources) == 0 {
		t.Fatal("Expected at least one source")
	}

	found := false
	for _, s := range sources {
		if s.Type == SourceTypeSQLite {
			found = true
			if s.Path != dbPath {
				t.Errorf("Expected path %s, got %s", dbPath, s.Path)
			}
			if s.Priority != PrioritySQLite {
				t.Errorf("Expected priority %d, got %d", PrioritySQLite, s.Priority)
			}
		}
	}
	if !found {
		t.Error("SQLite source not found")
	}
}

// TestDiscoverSources_OnlyJSONL tests discovery with only a JSONL source
func TestDiscoverSources_OnlyJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create JSONL file
	jsonlPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(`{"id":"TEST-1","title":"Test","status":"open"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sources, err := DiscoverSources(DiscoveryOptions{
		BeadsDir:               beadsDir,
		ValidateAfterDiscovery: false,
	})
	if err != nil {
		t.Fatalf("DiscoverSources failed: %v", err)
	}

	if len(sources) == 0 {
		t.Fatal("Expected at least one source")
	}

	found := false
	for _, s := range sources {
		if s.Type == SourceTypeJSONLLocal {
			found = true
			if s.Path != jsonlPath {
				t.Errorf("Expected path %s, got %s", jsonlPath, s.Path)
			}
		}
	}
	if !found {
		t.Error("JSONL source not found")
	}
}

// TestDiscoverSources_Multiple tests discovery with multiple sources
func TestDiscoverSources_Multiple(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create SQLite database
	dbPath := filepath.Join(beadsDir, "beads.db")
	createTestSQLiteDB(t, dbPath)

	// Create JSONL file
	jsonlPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(`{"id":"TEST-1","title":"Test","status":"open"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sources, err := DiscoverSources(DiscoveryOptions{
		BeadsDir:               beadsDir,
		ValidateAfterDiscovery: false,
	})
	if err != nil {
		t.Fatalf("DiscoverSources failed: %v", err)
	}

	if len(sources) < 2 {
		t.Fatalf("Expected at least 2 sources, got %d", len(sources))
	}

	foundSQLite := false
	foundJSONL := false
	for _, s := range sources {
		if s.Type == SourceTypeSQLite {
			foundSQLite = true
		}
		if s.Type == SourceTypeJSONLLocal {
			foundJSONL = true
		}
	}

	if !foundSQLite {
		t.Error("SQLite source not found")
	}
	if !foundJSONL {
		t.Error("JSONL source not found")
	}
}

// TestDiscoverSources_Empty tests discovery with no sources
func TestDiscoverSources_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	sources, err := DiscoverSources(DiscoveryOptions{
		BeadsDir:               beadsDir,
		ValidateAfterDiscovery: false,
	})
	if err != nil {
		t.Fatalf("DiscoverSources failed: %v", err)
	}

	if len(sources) != 0 {
		t.Errorf("Expected 0 sources, got %d", len(sources))
	}
}

// TestValidateSQLite_Valid tests validation of a valid SQLite database
func TestValidateSQLite_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "beads.db")
	createTestSQLiteDB(t, dbPath)

	source := DataSource{
		Type: SourceTypeSQLite,
		Path: dbPath,
	}

	err := ValidateSource(&source)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !source.Valid {
		t.Error("Expected source to be valid")
	}
	if source.IssueCount != 2 {
		t.Errorf("Expected 2 issues, got %d", source.IssueCount)
	}
}

// TestValidateSQLite_Empty tests validation of an empty but valid SQLite database
func TestValidateSQLite_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "beads.db")
	createEmptySQLiteDB(t, dbPath)

	source := DataSource{
		Type: SourceTypeSQLite,
		Path: dbPath,
	}

	err := ValidateSource(&source)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !source.Valid {
		t.Error("Expected source to be valid")
	}
	if source.IssueCount != 0 {
		t.Errorf("Expected 0 issues, got %d", source.IssueCount)
	}
}

// TestValidateSQLite_Corrupted tests validation of a corrupted SQLite database
func TestValidateSQLite_Corrupted(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "beads.db")

	// Write garbage data
	if err := os.WriteFile(dbPath, []byte("THIS IS NOT A VALID SQLITE DATABASE"), 0644); err != nil {
		t.Fatal(err)
	}

	source := DataSource{
		Type: SourceTypeSQLite,
		Path: dbPath,
	}

	err := ValidateSource(&source)
	if err == nil {
		t.Fatal("Expected validation to fail for corrupted database")
	}

	if source.Valid {
		t.Error("Expected source to be invalid")
	}
}

// TestValidateSQLite_WrongSchema tests validation of SQLite with missing columns
func TestValidateSQLite_WrongSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "beads.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// Create table with wrong schema (missing required columns)
	_, err = db.Exec("CREATE TABLE issues (foo TEXT)")
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	source := DataSource{
		Type: SourceTypeSQLite,
		Path: dbPath,
	}

	err = ValidateSource(&source)
	if err == nil {
		t.Fatal("Expected validation to fail for wrong schema")
	}

	if source.Valid {
		t.Error("Expected source to be invalid")
	}
}

// TestValidateJSONL_Valid tests validation of a valid JSONL file
func TestValidateJSONL_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "issues.jsonl")

	content := `{"id":"TEST-1","title":"Test Issue 1","status":"open"}
{"id":"TEST-2","title":"Test Issue 2","status":"closed"}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	source := DataSource{
		Type: SourceTypeJSONLLocal,
		Path: jsonlPath,
	}

	err := ValidateSource(&source)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !source.Valid {
		t.Error("Expected source to be valid")
	}
	if source.IssueCount != 2 {
		t.Errorf("Expected 2 issues, got %d", source.IssueCount)
	}
}

// TestValidateJSONL_Empty tests validation of an empty JSONL file
func TestValidateJSONL_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "issues.jsonl")

	if err := os.WriteFile(jsonlPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	source := DataSource{
		Type: SourceTypeJSONLLocal,
		Path: jsonlPath,
	}

	err := ValidateSource(&source)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !source.Valid {
		t.Error("Expected empty file to be valid")
	}
	if source.IssueCount != 0 {
		t.Errorf("Expected 0 issues, got %d", source.IssueCount)
	}
}

// TestValidateJSONL_PartialCorrupt tests validation with <10% bad lines
func TestValidateJSONL_PartialCorrupt(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "issues.jsonl")

	// 9 valid, 1 invalid = 10% error rate (at threshold)
	content := `{"id":"TEST-1","title":"Test 1","status":"open"}
{"id":"TEST-2","title":"Test 2","status":"open"}
{"id":"TEST-3","title":"Test 3","status":"open"}
{"id":"TEST-4","title":"Test 4","status":"open"}
{"id":"TEST-5","title":"Test 5","status":"open"}
{"id":"TEST-6","title":"Test 6","status":"open"}
{"id":"TEST-7","title":"Test 7","status":"open"}
{"id":"TEST-8","title":"Test 8","status":"open"}
{"id":"TEST-9","title":"Test 9","status":"open"}
not valid json
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	source := DataSource{
		Type: SourceTypeJSONLLocal,
		Path: jsonlPath,
	}

	err := ValidateSource(&source)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !source.Valid {
		t.Error("Expected source with 10% errors to be valid")
	}
}

// TestValidateJSONL_HeavyCorrupt tests validation with >10% bad lines
func TestValidateJSONL_HeavyCorrupt(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "issues.jsonl")

	// 8 valid, 3 invalid = ~27% error rate
	content := `{"id":"TEST-1","title":"Test 1","status":"open"}
{"id":"TEST-2","title":"Test 2","status":"open"}
{"id":"TEST-3","title":"Test 3","status":"open"}
{"id":"TEST-4","title":"Test 4","status":"open"}
{"id":"TEST-5","title":"Test 5","status":"open"}
{"id":"TEST-6","title":"Test 6","status":"open"}
{"id":"TEST-7","title":"Test 7","status":"open"}
{"id":"TEST-8","title":"Test 8","status":"open"}
not valid json 1
not valid json 2
not valid json 3
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	source := DataSource{
		Type: SourceTypeJSONLLocal,
		Path: jsonlPath,
	}

	err := ValidateSource(&source)
	if err == nil {
		t.Fatal("Expected validation to fail for heavily corrupted file")
	}

	if source.Valid {
		t.Error("Expected source to be invalid")
	}
}

// TestValidateJSONL_MissingFields tests validation with missing required fields
func TestValidateJSONL_MissingFields(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "issues.jsonl")

	// Missing "title" field in all entries
	content := `{"id":"TEST-1","status":"open"}
{"id":"TEST-2","status":"open"}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	source := DataSource{
		Type: SourceTypeJSONLLocal,
		Path: jsonlPath,
	}

	err := ValidateSource(&source)
	if err == nil {
		t.Fatal("Expected validation to fail for missing required fields")
	}
}

// TestSelectBestSource_SingleValid tests selection with one valid source
func TestSelectBestSource_SingleValid(t *testing.T) {
	sources := []DataSource{
		{
			Type:     SourceTypeSQLite,
			Path:     "/test/beads.db",
			Priority: PrioritySQLite,
			ModTime:  time.Now(),
			Valid:    true,
		},
	}

	selected, err := SelectBestSource(sources)
	if err != nil {
		t.Fatalf("Selection failed: %v", err)
	}

	if selected.Path != "/test/beads.db" {
		t.Errorf("Expected /test/beads.db, got %s", selected.Path)
	}
}

// TestSelectBestSource_FresherWins tests that newer timestamp wins
func TestSelectBestSource_FresherWins(t *testing.T) {
	now := time.Now()
	sources := []DataSource{
		{
			Type:     SourceTypeJSONLLocal,
			Path:     "/test/old.jsonl",
			Priority: PriorityJSONLLocal,
			ModTime:  now.Add(-1 * time.Hour),
			Valid:    true,
		},
		{
			Type:     SourceTypeJSONLLocal,
			Path:     "/test/new.jsonl",
			Priority: PriorityJSONLLocal,
			ModTime:  now,
			Valid:    true,
		},
	}

	selected, err := SelectBestSource(sources)
	if err != nil {
		t.Fatalf("Selection failed: %v", err)
	}

	if selected.Path != "/test/new.jsonl" {
		t.Errorf("Expected newer source, got %s", selected.Path)
	}
}

// TestSelectBestSource_PriorityTiebreaker tests that priority breaks ties
func TestSelectBestSource_PriorityTiebreaker(t *testing.T) {
	now := time.Now()
	sources := []DataSource{
		{
			Type:     SourceTypeJSONLLocal,
			Path:     "/test/local.jsonl",
			Priority: PriorityJSONLLocal,
			ModTime:  now,
			Valid:    true,
		},
		{
			Type:     SourceTypeSQLite,
			Path:     "/test/beads.db",
			Priority: PrioritySQLite,
			ModTime:  now, // Same time
			Valid:    true,
		},
	}

	selected, err := SelectBestSource(sources)
	if err != nil {
		t.Fatalf("Selection failed: %v", err)
	}

	if selected.Type != SourceTypeSQLite {
		t.Errorf("Expected SQLite (higher priority), got %s", selected.Type)
	}
}

// TestSelectBestSource_AllInvalid tests that error is returned when all invalid
func TestSelectBestSource_AllInvalid(t *testing.T) {
	sources := []DataSource{
		{
			Type:  SourceTypeSQLite,
			Path:  "/test/beads.db",
			Valid: false,
		},
		{
			Type:  SourceTypeJSONLLocal,
			Path:  "/test/issues.jsonl",
			Valid: false,
		},
	}

	_, err := SelectBestSource(sources)
	if err != ErrNoValidSources {
		t.Errorf("Expected ErrNoValidSources, got %v", err)
	}
}

// TestSelectBestSource_SkipsInvalid tests that invalid sources are skipped
func TestSelectBestSource_SkipsInvalid(t *testing.T) {
	now := time.Now()
	sources := []DataSource{
		{
			Type:     SourceTypeSQLite,
			Path:     "/test/beads.db",
			Priority: PrioritySQLite,
			ModTime:  now, // Newest, but invalid
			Valid:    false,
		},
		{
			Type:     SourceTypeJSONLLocal,
			Path:     "/test/issues.jsonl",
			Priority: PriorityJSONLLocal,
			ModTime:  now.Add(-1 * time.Hour), // Older, but valid
			Valid:    true,
		},
	}

	selected, err := SelectBestSource(sources)
	if err != nil {
		t.Fatalf("Selection failed: %v", err)
	}

	if selected.Path != "/test/issues.jsonl" {
		t.Errorf("Expected valid JSONL source, got %s", selected.Path)
	}
}

// TestFallbackChain_FirstValid tests fallback when first source works
func TestFallbackChain_FirstValid(t *testing.T) {
	now := time.Now()
	sources := []DataSource{
		{
			Type:     SourceTypeSQLite,
			Path:     "/test/beads.db",
			Priority: PrioritySQLite,
			ModTime:  now,
			Valid:    true,
		},
		{
			Type:     SourceTypeJSONLLocal,
			Path:     "/test/issues.jsonl",
			Priority: PriorityJSONLLocal,
			ModTime:  now.Add(-1 * time.Hour),
			Valid:    true,
		},
	}

	loadCalls := 0
	selected, err := SelectWithFallback(sources, func(s DataSource) error {
		loadCalls++
		return nil // Success
	}, DefaultSelectionOptions())

	if err != nil {
		t.Fatalf("Fallback failed: %v", err)
	}

	if loadCalls != 1 {
		t.Errorf("Expected 1 load call, got %d", loadCalls)
	}
	if selected.Type != SourceTypeSQLite {
		t.Errorf("Expected first source, got %s", selected.Type)
	}
}

// TestFallbackChain_SecondValid tests fallback when first fails
func TestFallbackChain_SecondValid(t *testing.T) {
	now := time.Now()
	sources := []DataSource{
		{
			Type:     SourceTypeSQLite,
			Path:     "/test/beads.db",
			Priority: PrioritySQLite,
			ModTime:  now,
			Valid:    true,
		},
		{
			Type:     SourceTypeJSONLLocal,
			Path:     "/test/issues.jsonl",
			Priority: PriorityJSONLLocal,
			ModTime:  now.Add(-1 * time.Hour),
			Valid:    true,
		},
	}

	loadCalls := 0
	selected, err := SelectWithFallback(sources, func(s DataSource) error {
		loadCalls++
		if s.Type == SourceTypeSQLite {
			return os.ErrNotExist // First source fails
		}
		return nil // Second source works
	}, DefaultSelectionOptions())

	if err != nil {
		t.Fatalf("Fallback failed: %v", err)
	}

	if loadCalls != 2 {
		t.Errorf("Expected 2 load calls, got %d", loadCalls)
	}
	if selected.Type != SourceTypeJSONLLocal {
		t.Errorf("Expected fallback to JSONL, got %s", selected.Type)
	}
}

// TestFallbackChain_AllFail tests fallback when all sources fail
func TestFallbackChain_AllFail(t *testing.T) {
	now := time.Now()
	sources := []DataSource{
		{
			Type:     SourceTypeSQLite,
			Path:     "/test/beads.db",
			Priority: PrioritySQLite,
			ModTime:  now,
			Valid:    true,
		},
		{
			Type:     SourceTypeJSONLLocal,
			Path:     "/test/issues.jsonl",
			Priority: PriorityJSONLLocal,
			ModTime:  now.Add(-1 * time.Hour),
			Valid:    true,
		},
	}

	_, err := SelectWithFallback(sources, func(s DataSource) error {
		return os.ErrNotExist // All fail
	}, DefaultSelectionOptions())

	if err == nil {
		t.Fatal("Expected error when all sources fail")
	}
}

// Helper to create a test SQLite database with sample data
func createTestSQLiteDB(t *testing.T, path string) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE issues (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL,
			priority INTEGER DEFAULT 3,
			issue_type TEXT DEFAULT 'task',
			tombstone INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO issues (id, title, status) VALUES
		('TEST-1', 'Test Issue 1', 'open'),
		('TEST-2', 'Test Issue 2', 'closed')
	`)
	if err != nil {
		t.Fatal(err)
	}
}

// Helper to create an empty SQLite database
func createEmptySQLiteDB(t *testing.T, path string) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE issues (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL,
			priority INTEGER DEFAULT 3,
			issue_type TEXT DEFAULT 'task',
			tombstone INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatal(err)
	}
}
