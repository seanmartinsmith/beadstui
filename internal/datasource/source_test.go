package datasource

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
			Type:     SourceTypeJSONLLocal,
			Path:     "/test/issues.jsonl",
			Priority: PriorityJSONLLocal,
			ModTime:  time.Now(),
			Valid:    true,
		},
	}

	selected, err := SelectBestSource(sources)
	if err != nil {
		t.Fatalf("Selection failed: %v", err)
	}

	if selected.Path != "/test/issues.jsonl" {
		t.Errorf("Expected /test/issues.jsonl, got %s", selected.Path)
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
			Type:     SourceTypeJSONLWorktree,
			Path:     "/test/worktree.jsonl",
			Priority: PriorityJSONLWorktree,
			ModTime:  now, // Same time
			Valid:    true,
		},
	}

	selected, err := SelectBestSource(sources)
	if err != nil {
		t.Fatalf("Selection failed: %v", err)
	}

	if selected.Type != SourceTypeJSONLWorktree {
		t.Errorf("Expected JSONLWorktree (higher priority), got %s", selected.Type)
	}
}

// TestSelectBestSource_AllInvalid tests that error is returned when all invalid
func TestSelectBestSource_AllInvalid(t *testing.T) {
	sources := []DataSource{
		{
			Type:  SourceTypeJSONLWorktree,
			Path:  "/test/worktree.jsonl",
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
			Type:     SourceTypeJSONLWorktree,
			Path:     "/test/worktree.jsonl",
			Priority: PriorityJSONLWorktree,
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
			Type:     SourceTypeJSONLWorktree,
			Path:     "/test/worktree.jsonl",
			Priority: PriorityJSONLWorktree,
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
	if selected.Type != SourceTypeJSONLWorktree {
		t.Errorf("Expected first source, got %s", selected.Type)
	}
}

// TestFallbackChain_SecondValid tests fallback when first fails
func TestFallbackChain_SecondValid(t *testing.T) {
	now := time.Now()
	sources := []DataSource{
		{
			Type:     SourceTypeJSONLWorktree,
			Path:     "/test/worktree.jsonl",
			Priority: PriorityJSONLWorktree,
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
		if s.Type == SourceTypeJSONLWorktree {
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
			Type:     SourceTypeJSONLWorktree,
			Path:     "/test/worktree.jsonl",
			Priority: PriorityJSONLWorktree,
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

// TestDiscoverSources_RequireDolt_Unreachable tests that RequireDolt returns
// ErrDoltRequired when no Dolt server is available (no metadata or server down).
func TestDiscoverSources_RequireDolt_Unreachable(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a JSONL file that would normally be discovered
	jsonlPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(`{"id":"TEST-1","title":"Test","status":"open"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sources, err := DiscoverSources(DiscoveryOptions{
		BeadsDir:    beadsDir,
		RequireDolt: true,
	})

	if err != ErrDoltRequired {
		t.Errorf("Expected ErrDoltRequired, got err=%v sources=%v", err, sources)
	}
	if sources != nil {
		t.Errorf("Expected nil sources, got %d", len(sources))
	}
}

// TestDiscoverSources_RequireDolt_False_LegacyPreserved tests that legacy
// JSONL discovery still works when RequireDolt is false (no Dolt metadata).
func TestDiscoverSources_RequireDolt_False_LegacyPreserved(t *testing.T) {
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
		RequireDolt:            false,
		ValidateAfterDiscovery: false,
	})
	if err != nil {
		t.Fatalf("DiscoverSources failed: %v", err)
	}

	found := false
	for _, s := range sources {
		if s.Type == SourceTypeJSONLLocal {
			found = true
		}
	}
	if !found {
		t.Error("Expected JSONL source to be discovered with RequireDolt=false")
	}
}
