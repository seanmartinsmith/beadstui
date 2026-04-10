package loader_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/loader"
)

func TestLoadIssuesFromFile_WithBOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bom.jsonl")

	// UTF-8 BOM is EF BB BF
	bom := []byte{0xEF, 0xBB, 0xBF}
	jsonContent := []byte(`{"id":"1","title":"First","status":"open","issue_type":"task"}` + "\n")
	fullContent := append(bom, jsonContent...)

	if err := os.WriteFile(path, fullContent, 0644); err != nil {
		t.Fatal(err)
	}

	issues, err := loader.LoadIssuesFromFile(path)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(issues) != 1 {
		t.Errorf("Expected 1 issue, got %d. First issue might have been skipped due to BOM.", len(issues))
	} else if issues[0].ID != "1" {
		t.Errorf("Expected ID '1', got '%s'", issues[0].ID)
	}
}
