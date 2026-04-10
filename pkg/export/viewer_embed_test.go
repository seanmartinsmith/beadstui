package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceTitle_Basic(t *testing.T) {
	html := `<html><head><title>Beads Viewer</title></head><body><h1 class="text-xl font-semibold">Beads Viewer</h1></body></html>`

	result := replaceTitle(html, "My Project")
	if !strings.Contains(result, "<title>My Project</title>") {
		t.Errorf("Expected title tag replacement, got: %s", result)
	}
	if !strings.Contains(result, `<h1 class="text-xl font-semibold">My Project</h1>`) {
		t.Errorf("Expected h1 replacement, got: %s", result)
	}
}

func TestReplaceTitle_Empty(t *testing.T) {
	html := `<title>Beads Viewer</title>`
	result := replaceTitle(html, "")
	if result != html {
		t.Errorf("Empty title should return content unchanged, got: %s", result)
	}
}

func TestReplaceTitle_XSSPrevention(t *testing.T) {
	html := `<title>Beads Viewer</title>`
	result := replaceTitle(html, `<script>alert("xss")</script>`)
	if strings.Contains(result, "<script>") {
		t.Errorf("XSS not prevented: %s", result)
	}
	if !strings.Contains(result, "&lt;script&gt;") {
		t.Errorf("Expected HTML-escaped title, got: %s", result)
	}
}

func TestReplaceTitle_SpecialChars(t *testing.T) {
	html := `<title>Beads Viewer</title>`
	result := replaceTitle(html, `Tom & Jerry's "Project"`)
	if !strings.Contains(result, "Tom &amp; Jerry") {
		t.Errorf("Ampersand not escaped, got: %s", result)
	}
	if !strings.Contains(result, "&#34;Project&#34;") {
		t.Errorf("Quotes not escaped, got: %s", result)
	}
}

func TestReplaceTitle_NoMatch(t *testing.T) {
	html := `<title>Something Else</title>`
	result := replaceTitle(html, "My Project")
	// Should not modify content when the original title doesn't match
	if result != html {
		t.Errorf("Should not modify non-matching content, got: %s", result)
	}
}

func TestAddScriptCacheBusting_AllFiles(t *testing.T) {
	html := `<script src="viewer.js"></script>
<script src="charts.js"></script>
<script src="graph.js"></script>
<script src="hybrid_scorer.js"></script>
<script src="wasm_loader.js"></script>`

	result := AddScriptCacheBusting(html)

	// All five JS files should have cache-busting
	for _, jsFile := range []string{"viewer.js", "charts.js", "graph.js", "hybrid_scorer.js", "wasm_loader.js"} {
		if strings.Contains(result, `src="`+jsFile+`"`) {
			t.Errorf("File %s was not cache-busted", jsFile)
		}
		if !strings.Contains(result, jsFile+"?v=") {
			t.Errorf("File %s missing cache-buster parameter", jsFile)
		}
	}
}

func TestAddScriptCacheBusting_SingleQuotes(t *testing.T) {
	html := `<script src='viewer.js'></script>`
	result := AddScriptCacheBusting(html)

	if strings.Contains(result, `src='viewer.js'`) {
		t.Error("Single-quoted src should be cache-busted")
	}
	if !strings.Contains(result, "viewer.js?v=") {
		t.Error("Missing cache-buster for single-quoted src")
	}
}

func TestAddScriptCacheBusting_NoMatch(t *testing.T) {
	html := `<script src="vendor.js"></script>`
	result := AddScriptCacheBusting(html)

	// Vendor files should not be modified
	if result != html {
		t.Errorf("Vendor files should not be cache-busted, got: %s", result)
	}
}

func TestAddScriptCacheBusting_MultipleSameFile(t *testing.T) {
	html := `<script src="viewer.js"></script><script src="viewer.js"></script>`
	result := AddScriptCacheBusting(html)

	// Both instances should be cache-busted
	count := strings.Count(result, "viewer.js?v=")
	if count != 2 {
		t.Errorf("Expected 2 cache-busted instances, got %d", count)
	}
}

func TestHasEmbeddedAssets(t *testing.T) {
	// The binary has embedded assets
	result := HasEmbeddedAssets()
	if !result {
		t.Error("Expected HasEmbeddedAssets() to return true (assets are embedded)")
	}
}

func TestCopyEmbeddedAssets(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	err := CopyEmbeddedAssets(outputDir, "Test Project")
	if err != nil {
		t.Fatalf("CopyEmbeddedAssets failed: %v", err)
	}

	// Verify index.html exists
	indexPath := filepath.Join(outputDir, "index.html")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}

	contentStr := string(content)

	// Verify title was replaced
	if strings.Contains(contentStr, "<title>Beads Viewer</title>") {
		t.Error("Title should have been replaced")
	}
	if !strings.Contains(contentStr, "<title>Test Project</title>") {
		t.Error("Expected custom title in index.html")
	}

	// Verify cache-busting was applied
	if strings.Contains(contentStr, `src="viewer.js"`) {
		t.Error("viewer.js should have cache-busting parameter")
	}
}

func TestCopyEmbeddedAssets_NoTitle(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	err := CopyEmbeddedAssets(outputDir, "")
	if err != nil {
		t.Fatalf("CopyEmbeddedAssets failed: %v", err)
	}

	// Verify index.html still has default title
	indexPath := filepath.Join(outputDir, "index.html")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}

	if !strings.Contains(string(content), "<title>Beads Viewer</title>") {
		t.Error("Default title should be preserved when no custom title provided")
	}
}

func TestAddGitHubWorkflowToBundle(t *testing.T) {
	tmpDir := t.TempDir()

	err := AddGitHubWorkflowToBundle(tmpDir)
	if err != nil {
		t.Fatalf("AddGitHubWorkflowToBundle failed: %v", err)
	}

	// Verify workflow was created
	workflowPath := filepath.Join(tmpDir, ".github", "workflows", "static.yml")
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		t.Error("Workflow file was not created")
	}
}
