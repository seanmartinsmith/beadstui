package export

import (
	"strings"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestGenerateInteractiveGraphFilename(t *testing.T) {
	name := GenerateInteractiveGraphFilename("my-project")

	if !strings.HasPrefix(name, "my-project_graph_export__as_of__") {
		t.Errorf("Expected prefix 'my-project_graph_export__as_of__', got: %s", name)
	}
	if !strings.HasSuffix(name, ".html") {
		t.Errorf("Expected .html suffix, got: %s", name)
	}
	if !strings.Contains(name, "__git_head_hash__") {
		t.Errorf("Expected git hash section, got: %s", name)
	}
}

func TestGenerateInteractiveGraphFilename_SpecialChars(t *testing.T) {
	name := GenerateInteractiveGraphFilename("my project/path")

	if strings.Contains(name, " ") {
		t.Errorf("Spaces should be replaced with underscores, got: %s", name)
	}
	if strings.Contains(name, "/") && !strings.HasSuffix(name, ".html") {
		t.Errorf("Slashes should be replaced with underscores, got: %s", name)
	}
	if !strings.HasPrefix(name, "my_project_path_graph_export") {
		t.Errorf("Expected sanitized name prefix, got: %s", name)
	}
}

func TestGenerateInteractiveGraphHTML_EmptyIssues(t *testing.T) {
	opts := InteractiveGraphOptions{
		Issues: []model.Issue{},
	}

	_, err := GenerateInteractiveGraphHTML(opts)
	if err == nil {
		t.Error("Expected error for empty issues")
	}
	if !strings.Contains(err.Error(), "no issues") {
		t.Errorf("Expected 'no issues' error, got: %v", err)
	}
}

func TestGenerateInteractiveGraphHTML_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	issues := []model.Issue{
		{ID: "bv-1", Title: "First Issue", Status: model.StatusOpen, Priority: 1},
		{ID: "bv-2", Title: "Second Issue", Status: model.StatusInProgress, Priority: 2,
			Dependencies: []*model.Dependency{
				{IssueID: "bv-2", DependsOnID: "bv-1", Type: model.DepBlocks},
			},
		},
	}

	opts := InteractiveGraphOptions{
		Issues:      issues,
		Title:       "Test Graph",
		DataHash:    "test-hash",
		Path:        tmpDir + "/test_graph.html",
		ProjectName: "test-project",
	}

	path, err := GenerateInteractiveGraphHTML(opts)
	if err != nil {
		t.Fatalf("GenerateInteractiveGraphHTML failed: %v", err)
	}

	if path == "" {
		t.Error("Expected non-empty path")
	}
}
