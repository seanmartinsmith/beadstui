package loader_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/loader"
)

func TestParseIssuesWithOptions_LineTooLong(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large_line.jsonl")

	// Create a line that is slightly larger than our custom buffer size
	const bufferSize = 1024 // 1KB for test
	longLine := `{"id":"long","title":"` + strings.Repeat("a", bufferSize) + `"}`

	err := os.WriteFile(path, []byte(longLine+"\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var warnings []string
	opts := loader.ParseOptions{
		BufferSize: bufferSize,
		WarningHandler: func(msg string) {
			warnings = append(warnings, msg)
		},
	}

	issues, err := loader.LoadIssuesFromFileWithOptions(path, opts)
	if err != nil {
		t.Fatalf("Expected success (skipping long line), got error: %v", err)
	}

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues, got %d", len(issues))
	}

	expectedWarning := "skipping line 1: line too long"
	found := false
	for _, w := range warnings {
		if strings.Contains(w, expectedWarning) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning containing %q, got: %v", expectedWarning, warnings)
	}
}
