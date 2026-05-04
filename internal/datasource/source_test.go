package datasource

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestDiscoverSource_OnlyJSONL: a beads dir with a JSONL file and no
// Dolt config resolves to SourceTypeJSONLFallback at that path.
func TestDiscoverSource_OnlyJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	jsonlPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(`{"id":"TEST-1","title":"Test","status":"open"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	src, err := DiscoverSource(DiscoveryOptions{
		BeadsDir: beadsDir,
	})
	if err != nil {
		t.Fatalf("DiscoverSource failed: %v", err)
	}

	if src.Type != SourceTypeJSONLFallback {
		t.Errorf("expected SourceTypeJSONLFallback, got %s", src.Type)
	}
	if src.Path != jsonlPath {
		t.Errorf("expected path %s, got %s", jsonlPath, src.Path)
	}
}

// TestDiscoverSource_Empty: an empty beads dir resolves to ErrNoSource.
func TestDiscoverSource_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := DiscoverSource(DiscoveryOptions{
		BeadsDir: beadsDir,
	})
	if !errors.Is(err, ErrNoSource) {
		t.Errorf("expected ErrNoSource, got %v", err)
	}
}

// TestDiscoverSource_DoltDeclaredButUnreachable: when metadata.json
// declares backend=dolt but no server is reachable, return
// ErrDoltRequired without falling back to any JSONL file present.
func TestDiscoverSource_DoltDeclaredButUnreachable(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Use an unlikely-to-be-bound port so Ping fails deterministically.
	t.Setenv("BEADS_DOLT_SERVER_PORT", "1")
	meta := []byte(`{"backend":"dolt","dolt_database":"beads"}`)
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), meta, 0644); err != nil {
		t.Fatal(err)
	}

	// Even with a JSONL file present, ErrDoltRequired must be returned.
	jsonlPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(`{"id":"TEST-1","title":"Test","status":"open"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := DiscoverSource(DiscoveryOptions{BeadsDir: beadsDir})
	if !errors.Is(err, ErrDoltRequired) {
		t.Errorf("expected ErrDoltRequired, got %v", err)
	}
}

// TestDiscoverSource_NoDoltMetadata_FallsBackToJSONL confirms the
// no-Dolt-config path resolves to JSONL fallback (ADR-003 collapse: no
// RequireDolt flag needed; absence of Dolt config implies legacy mode).
func TestDiscoverSource_NoDoltMetadata_FallsBackToJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	jsonlPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(`{"id":"TEST-1","title":"Test","status":"open"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	src, err := DiscoverSource(DiscoveryOptions{BeadsDir: beadsDir})
	if err != nil {
		t.Fatalf("DiscoverSource failed: %v", err)
	}
	if src.Type != SourceTypeJSONLFallback {
		t.Errorf("expected SourceTypeJSONLFallback, got %s", src.Type)
	}
}

// TestDiscoverSource_HonorsMetadataJSONLExport verifies the
// metadataPreferredSource fold from cmd/bt/pages.go: when metadata.json
// declares jsonl_export and the file exists, that path wins over the
// canonical filename search.
func TestDiscoverSource_HonorsMetadataJSONLExport(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	exportPath := filepath.Join(beadsDir, "custom-export.jsonl")
	if err := os.WriteFile(exportPath, []byte(`{"id":"TEST-1","title":"Test","status":"open"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Decoy: a canonical issues.jsonl that should NOT win over the
	// metadata-declared export.
	decoyPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(decoyPath, []byte(`{"id":"TEST-2","title":"Decoy","status":"open"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := []byte(`{"jsonl_export":"custom-export.jsonl"}`)
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), meta, 0644); err != nil {
		t.Fatal(err)
	}

	src, err := DiscoverSource(DiscoveryOptions{BeadsDir: beadsDir})
	if err != nil {
		t.Fatalf("DiscoverSource failed: %v", err)
	}
	if src.Path != exportPath {
		t.Errorf("expected %s (from metadata.json), got %s", exportPath, src.Path)
	}
}

// TestValidateJSONL_Valid validates a well-formed JSONL file.
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
		Type: SourceTypeJSONLFallback,
		Path: jsonlPath,
	}

	if err := ValidateSource(&source); err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !source.Valid {
		t.Error("Expected source to be valid")
	}
	if source.IssueCount != 2 {
		t.Errorf("Expected 2 issues, got %d", source.IssueCount)
	}
}

// TestValidateJSONL_Empty validates that an empty JSONL file is valid (0 issues).
func TestValidateJSONL_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "issues.jsonl")

	if err := os.WriteFile(jsonlPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	source := DataSource{
		Type: SourceTypeJSONLFallback,
		Path: jsonlPath,
	}

	if err := ValidateSource(&source); err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !source.Valid {
		t.Error("Expected empty file to be valid")
	}
	if source.IssueCount != 0 {
		t.Errorf("Expected 0 issues, got %d", source.IssueCount)
	}
}

// TestValidateJSONL_PartialCorrupt: <=10% bad lines is still valid.
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
		Type: SourceTypeJSONLFallback,
		Path: jsonlPath,
	}

	if err := ValidateSource(&source); err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !source.Valid {
		t.Error("Expected source with 10% errors to be valid")
	}
}

// TestValidateJSONL_HeavyCorrupt: >10% bad lines fails validation.
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
		Type: SourceTypeJSONLFallback,
		Path: jsonlPath,
	}

	if err := ValidateSource(&source); err == nil {
		t.Fatal("Expected validation to fail for heavily corrupted file")
	}

	if source.Valid {
		t.Error("Expected source to be invalid")
	}
}

// TestValidateJSONL_MissingFields: missing required fields fails validation.
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
		Type: SourceTypeJSONLFallback,
		Path: jsonlPath,
	}

	if err := ValidateSource(&source); err == nil {
		t.Fatal("Expected validation to fail for missing required fields")
	}
}
