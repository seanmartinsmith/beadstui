package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBaselineSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".bt", "baseline.json")

	original := &Baseline{
		Version:       CurrentVersion,
		CreatedAt:     time.Now().Truncate(time.Second),
		CommitSHA:     "abc123def456",
		CommitMessage: "Test commit",
		Branch:        "main",
		Description:   "Test baseline",
		Projects: map[string]*ProjectSection{
			"bt": {
				Stats: GraphStats{
					NodeCount:       100,
					EdgeCount:       250,
					Density:         0.025,
					OpenCount:       50,
					ClosedCount:     40,
					BlockedCount:    10,
					CycleCount:      2,
					ActionableCount: 30,
				},
				TopMetrics: TopMetrics{
					PageRank: []MetricItem{
						{ID: "bt-1", Value: 0.15},
						{ID: "bt-2", Value: 0.12},
					},
					Betweenness: []MetricItem{
						{ID: "bt-3", Value: 0.8},
					},
				},
				Cycles: [][]string{
					{"A", "B", "C", "A"},
				},
			},
			"cass": {
				Stats: GraphStats{NodeCount: 20, EdgeCount: 30},
			},
		},
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if !Exists(path) {
		t.Error("baseline file should exist after save")
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Version != original.Version {
		t.Errorf("version mismatch: got %d, want %d", loaded.Version, original.Version)
	}
	if loaded.CommitSHA != original.CommitSHA {
		t.Errorf("commit SHA mismatch: got %s, want %s", loaded.CommitSHA, original.CommitSHA)
	}

	btSection := loaded.Project("bt")
	if btSection == nil {
		t.Fatal("loaded baseline missing 'bt' project section")
	}
	if btSection.Stats.NodeCount != 100 {
		t.Errorf("bt node count: got %d, want 100", btSection.Stats.NodeCount)
	}
	if btSection.Stats.Density != 0.025 {
		t.Errorf("bt density: got %f, want 0.025", btSection.Stats.Density)
	}
	if len(btSection.TopMetrics.PageRank) != 2 {
		t.Errorf("bt pagerank count: got %d, want 2", len(btSection.TopMetrics.PageRank))
	}
	if len(btSection.Cycles) != 1 {
		t.Errorf("bt cycles count: got %d, want 1", len(btSection.Cycles))
	}

	if loaded.Project("cass") == nil {
		t.Error("cass section missing after round-trip")
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/path/baseline.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestLoadRejectsOldSchema(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "baseline.json")

	// Simulated v1 baseline: top-level stats, no projects map.
	v1 := map[string]any{
		"version":    1,
		"created_at": time.Now(),
		"stats":      map[string]any{"node_count": 10},
	}
	data, err := json.Marshal(v1)
	if err != nil {
		t.Fatalf("marshal v1 fixture: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write v1 fixture: %v", err)
	}

	_, err = Load(path)
	if err == nil {
		t.Fatal("expected error loading v1 baseline; got nil")
	}
	if !strings.Contains(err.Error(), "schema v1") || !strings.Contains(err.Error(), "save-baseline") {
		t.Errorf("error message should surface schema mismatch and remediation: %v", err)
	}
}

func TestExistsNonExistent(t *testing.T) {
	if Exists("/nonexistent/path/baseline.json") {
		t.Error("Exists should return false for non-existent file")
	}
}

func TestDefaultPath(t *testing.T) {
	path := DefaultPath("/project")
	expected := filepath.Join("/project", ".bt", "baseline.json")
	if path != expected {
		t.Errorf("got %s, want %s", path, expected)
	}
}

func TestBaselineSummary(t *testing.T) {
	b := &Baseline{
		Version:       CurrentVersion,
		CreatedAt:     time.Date(2025, 11, 30, 10, 0, 0, 0, time.UTC),
		CommitSHA:     "abc123def456789",
		CommitMessage: "Fix the bug",
		Branch:        "feature-x",
		Description:   "Before refactoring",
		Projects: map[string]*ProjectSection{
			"bt": {
				Stats: GraphStats{
					NodeCount:       100,
					EdgeCount:       200,
					Density:         0.02,
					OpenCount:       60,
					ClosedCount:     30,
					BlockedCount:    10,
					ActionableCount: 45,
					CycleCount:      1,
				},
				TopMetrics: TopMetrics{
					PageRank: []MetricItem{
						{ID: "bt-1", Value: 0.15},
						{ID: "bt-2", Value: 0.10},
					},
				},
			},
		},
	}

	summary := b.Summary()

	if !strings.Contains(summary, "abc123de") {
		t.Error("summary should contain shortened commit SHA")
	}
	if !strings.Contains(summary, "feature-x") {
		t.Error("summary should contain branch name")
	}
	if !strings.Contains(summary, "[bt]") {
		t.Error("summary should contain project header")
	}
	if !strings.Contains(summary, "100 nodes") {
		t.Error("summary should contain node count")
	}
	if !strings.Contains(summary, "bt-1") {
		t.Error("summary should contain top PageRank item")
	}
}

func TestNew(t *testing.T) {
	projects := map[string]*ProjectSection{
		"bt": {
			Stats:      GraphStats{NodeCount: 50, EdgeCount: 100},
			TopMetrics: TopMetrics{PageRank: []MetricItem{{ID: "X", Value: 0.5}}},
			Cycles:     [][]string{{"A", "B", "A"}},
		},
	}

	b := New(projects, "Test description")

	if b.Version != CurrentVersion {
		t.Errorf("version should be %d, got %d", CurrentVersion, b.Version)
	}
	if b.Project("bt").Stats.NodeCount != 50 {
		t.Errorf("node count should be 50, got %d", b.Project("bt").Stats.NodeCount)
	}
	if b.Description != "Test description" {
		t.Errorf("description mismatch: got %s", b.Description)
	}
	if b.CreatedAt.IsZero() {
		t.Error("created_at should be set")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "a", "b", "c", "baseline.json")

	b := &Baseline{
		Version:   CurrentVersion,
		CreatedAt: time.Now(),
		Projects: map[string]*ProjectSection{
			"local": {Stats: GraphStats{NodeCount: 1}},
		},
	}

	if err := b.Save(deepPath); err != nil {
		t.Fatalf("Save should create directories: %v", err)
	}
	if !Exists(deepPath) {
		t.Error("file should exist after save")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(path, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestProjectMissingReturnsNil(t *testing.T) {
	b := &Baseline{
		Version: CurrentVersion,
		Projects: map[string]*ProjectSection{
			"bt": {Stats: GraphStats{NodeCount: 1}},
		},
	}
	if b.Project("cass") != nil {
		t.Error("Project on missing key should return nil")
	}

	var nilBaseline *Baseline
	if nilBaseline.Project("bt") != nil {
		t.Error("Project on nil receiver should return nil")
	}
}
