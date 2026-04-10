package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// AssertIssueCount verifies the expected number of issues.
func AssertIssueCount(t *testing.T, issues []model.Issue, expected int) {
	t.Helper()
	if len(issues) != expected {
		t.Errorf("expected %d issues, got %d", expected, len(issues))
	}
}

// AssertNoDuplicateIDs verifies all issue IDs are unique.
func AssertNoDuplicateIDs(t *testing.T, issues []model.Issue) {
	t.Helper()
	seen := make(map[string]bool)
	for _, issue := range issues {
		if seen[issue.ID] {
			t.Errorf("duplicate issue ID: %s", issue.ID)
		}
		seen[issue.ID] = true
	}
}

// AssertAllValid verifies all issues pass validation.
func AssertAllValid(t *testing.T, issues []model.Issue) {
	t.Helper()
	for i, issue := range issues {
		if err := issue.Validate(); err != nil {
			t.Errorf("issue %d (%s) invalid: %v", i, issue.ID, err)
		}
	}
}

// AssertDependencyExists verifies that a specific dependency exists.
func AssertDependencyExists(t *testing.T, issues []model.Issue, fromID, toID string) {
	t.Helper()
	for _, issue := range issues {
		if issue.ID == fromID {
			for _, dep := range issue.Dependencies {
				if dep.DependsOnID == toID {
					return
				}
			}
			t.Errorf("expected dependency from %s to %s not found", fromID, toID)
			return
		}
	}
	t.Errorf("issue %s not found", fromID)
}

// AssertNoCycles verifies that the issue graph has no cycles.
// This is a simple DFS-based check suitable for small test graphs.
func AssertNoCycles(t *testing.T, issues []model.Issue) {
	t.Helper()

	// Build adjacency map
	adj := make(map[string][]string)
	for _, issue := range issues {
		for _, dep := range issue.Dependencies {
			adj[issue.ID] = append(adj[issue.ID], dep.DependsOnID)
		}
	}

	// DFS with path tracking
	visited := make(map[string]bool)
	inPath := make(map[string]bool)

	var hasCycle func(id string) bool
	hasCycle = func(id string) bool {
		if inPath[id] {
			return true
		}
		if visited[id] {
			return false
		}
		visited[id] = true
		inPath[id] = true
		for _, dep := range adj[id] {
			if hasCycle(dep) {
				return true
			}
		}
		inPath[id] = false
		return false
	}

	for _, issue := range issues {
		if hasCycle(issue.ID) {
			t.Errorf("cycle detected involving issue %s", issue.ID)
			return
		}
	}
}

// AssertHasCycle verifies that the issue graph contains at least one cycle.
func AssertHasCycle(t *testing.T, issues []model.Issue) {
	t.Helper()

	// Build adjacency map
	adj := make(map[string][]string)
	for _, issue := range issues {
		for _, dep := range issue.Dependencies {
			adj[issue.ID] = append(adj[issue.ID], dep.DependsOnID)
		}
	}

	// DFS with path tracking
	visited := make(map[string]bool)
	inPath := make(map[string]bool)

	var hasCycle func(id string) bool
	hasCycle = func(id string) bool {
		if inPath[id] {
			return true
		}
		if visited[id] {
			return false
		}
		visited[id] = true
		inPath[id] = true
		for _, dep := range adj[id] {
			if hasCycle(dep) {
				return true
			}
		}
		inPath[id] = false
		return false
	}

	for _, issue := range issues {
		if hasCycle(issue.ID) {
			return // Found a cycle, assertion passes
		}
	}
	t.Error("expected cycle but none found")
}

// AssertStatusCounts verifies the count of issues in each status.
func AssertStatusCounts(t *testing.T, issues []model.Issue, open, inProgress, blocked, closed int) {
	t.Helper()
	counts := make(map[model.Status]int)
	for _, issue := range issues {
		counts[issue.Status]++
	}

	if counts[model.StatusOpen] != open {
		t.Errorf("expected %d open issues, got %d", open, counts[model.StatusOpen])
	}
	if counts[model.StatusInProgress] != inProgress {
		t.Errorf("expected %d in_progress issues, got %d", inProgress, counts[model.StatusInProgress])
	}
	if counts[model.StatusBlocked] != blocked {
		t.Errorf("expected %d blocked issues, got %d", blocked, counts[model.StatusBlocked])
	}
	if counts[model.StatusClosed] != closed {
		t.Errorf("expected %d closed issues, got %d", closed, counts[model.StatusClosed])
	}
}

// AssertJSONEqual compares two values after JSON round-tripping.
// Useful for comparing structs that may have different Go representations
// but equivalent JSON forms.
func AssertJSONEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()

	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("failed to marshal expected: %v", err)
	}

	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatalf("failed to marshal actual: %v", err)
	}

	if string(expectedJSON) != string(actualJSON) {
		t.Errorf("JSON mismatch:\nexpected: %s\nactual:   %s", expectedJSON, actualJSON)
	}
}

// Golden file helpers

// GoldenFile handles golden file comparisons.
type GoldenFile struct {
	t      *testing.T
	dir    string
	name   string
	update bool
}

// NewGoldenFile creates a golden file helper.
// If GENERATE_GOLDEN env var is set, golden files will be updated.
func NewGoldenFile(t *testing.T, dir, name string) *GoldenFile {
	t.Helper()
	return &GoldenFile{
		t:      t,
		dir:    dir,
		name:   name,
		update: os.Getenv("GENERATE_GOLDEN") != "",
	}
}

// Path returns the full path to the golden file.
func (g *GoldenFile) Path() string {
	return filepath.Join(g.dir, g.name)
}

// Assert compares actual content against the golden file.
// If GENERATE_GOLDEN is set, updates the golden file instead.
func (g *GoldenFile) Assert(actual string) {
	g.t.Helper()

	path := g.Path()

	if g.update {
		// Update golden file
		if err := os.MkdirAll(g.dir, 0755); err != nil {
			g.t.Fatalf("failed to create golden dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(actual), 0644); err != nil {
			g.t.Fatalf("failed to write golden file: %v", err)
		}
		g.t.Logf("updated golden file: %s", path)
		return
	}

	// Compare against golden file
	expected, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			g.t.Fatalf("golden file does not exist: %s\nRun with GENERATE_GOLDEN=1 to create it", path)
		}
		g.t.Fatalf("failed to read golden file: %v", err)
	}

	if string(expected) != actual {
		// Find first difference for helpful error message
		expectedLines := strings.Split(string(expected), "\n")
		actualLines := strings.Split(actual, "\n")

		for i := 0; i < len(expectedLines) || i < len(actualLines); i++ {
			var expLine, actLine string
			if i < len(expectedLines) {
				expLine = expectedLines[i]
			}
			if i < len(actualLines) {
				actLine = actualLines[i]
			}
			if expLine != actLine {
				g.t.Errorf("golden file mismatch at line %d:\nexpected: %s\nactual:   %s\n\nFull diff (expected vs actual):\n%s\nvs\n%s",
					i+1, expLine, actLine, string(expected), actual)
				return
			}
		}
		g.t.Errorf("golden file mismatch (length differs)")
	}
}

// AssertJSON compares actual value as JSON against the golden file.
func (g *GoldenFile) AssertJSON(actual interface{}) {
	g.t.Helper()

	data, err := json.MarshalIndent(actual, "", "  ")
	if err != nil {
		g.t.Fatalf("failed to marshal actual value: %v", err)
	}

	g.Assert(string(data))
}

// TempDir helpers

// TempBeadsDir creates a temporary directory with a .beads subdirectory
// and returns the path. The directory is cleaned up after the test.
func TempBeadsDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads dir: %v", err)
	}
	return dir
}

// WriteBeadsFile writes issues to a beads.jsonl file in the given directory.
func WriteBeadsFile(t *testing.T, dir string, issues []model.Issue) string {
	t.Helper()

	beadsPath := filepath.Join(dir, ".beads", "beads.jsonl")
	content := ToJSONL(issues)

	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write beads file: %v", err)
	}
	return beadsPath
}

// WriteIssuesFile writes issues to a custom path.
func WriteIssuesFile(t *testing.T, path string, issues []model.Issue) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	content := ToJSONL(issues)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write issues file: %v", err)
	}
}

// IssueMap helpers

// BuildIssueMap creates a map from ID to Issue for quick lookups.
func BuildIssueMap(issues []model.Issue) map[string]*model.Issue {
	m := make(map[string]*model.Issue, len(issues))
	for i := range issues {
		m[issues[i].ID] = &issues[i]
	}
	return m
}

// FindIssue returns the issue with the given ID, or nil if not found.
func FindIssue(issues []model.Issue, id string) *model.Issue {
	for i := range issues {
		if issues[i].ID == id {
			return &issues[i]
		}
	}
	return nil
}

// CountByStatus returns a map of status -> count.
func CountByStatus(issues []model.Issue) map[model.Status]int {
	counts := make(map[model.Status]int)
	for _, issue := range issues {
		counts[issue.Status]++
	}
	return counts
}

// CountByType returns a map of issue_type -> count.
func CountByType(issues []model.Issue) map[model.IssueType]int {
	counts := make(map[model.IssueType]int)
	for _, issue := range issues {
		counts[issue.IssueType]++
	}
	return counts
}

// GetIDs returns a slice of all issue IDs.
func GetIDs(issues []model.Issue) []string {
	ids := make([]string, len(issues))
	for i, issue := range issues {
		ids[i] = issue.ID
	}
	return ids
}

// IssueID generates a standard test issue ID with the given index.
// Format: "test-{index}" for consistency across tests.
func IssueID(index int) string {
	return fmt.Sprintf("test-%d", index)
}
