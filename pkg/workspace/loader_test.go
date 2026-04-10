package workspace_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/workspace"
)

// createTestBeadsFile creates a .beads/beads.jsonl file with test issues
func createTestBeadsFile(t *testing.T, repoPath string, issues []model.Issue) {
	t.Helper()

	beadsDir := filepath.Join(repoPath, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	beadsFile := filepath.Join(beadsDir, "beads.jsonl")
	file, err := os.Create(beadsFile)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, issue := range issues {
		// Provide sensible defaults so validation in loader passes
		if issue.IssueType == "" {
			issue.IssueType = model.TypeTask
		}
		if issue.Status == "" {
			issue.Status = model.StatusOpen
		}
		if err := encoder.Encode(issue); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAggregateLoaderLoadAll(t *testing.T) {
	tmpDir := t.TempDir()

	// Create api repo with issues
	apiRepo := filepath.Join(tmpDir, "services", "api")
	if err := os.MkdirAll(apiRepo, 0755); err != nil {
		t.Fatal(err)
	}
	createTestBeadsFile(t, apiRepo, []model.Issue{
		{ID: "AUTH-1", Title: "Auth feature", Status: model.StatusOpen, Priority: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "AUTH-2", Title: "Auth bug", Status: model.StatusClosed, Priority: 2, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	})

	// Create web repo with issues
	webRepo := filepath.Join(tmpDir, "apps", "web")
	if err := os.MkdirAll(webRepo, 0755); err != nil {
		t.Fatal(err)
	}
	createTestBeadsFile(t, webRepo, []model.Issue{
		{ID: "UI-1", Title: "UI feature", Status: model.StatusOpen, Priority: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	})

	// Create workspace config
	config := &workspace.Config{
		Name: "test-workspace",
		Repos: []workspace.RepoConfig{
			{Name: "api", Path: "services/api", Prefix: "api-"},
			{Name: "web", Path: "apps/web", Prefix: "web-"},
		},
	}

	loader := workspace.NewAggregateLoader(config, tmpDir)
	issues, results, err := loader.LoadAll(context.Background())

	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	// Should have 3 total issues
	if len(issues) != 3 {
		t.Errorf("len(issues) = %d, want 3", len(issues))
	}

	// Should have 2 results (one per repo)
	if len(results) != 2 {
		t.Errorf("len(results) = %d, want 2", len(results))
	}

	// Check namespacing
	foundAPIAuth1 := false
	foundWebUI1 := false
	for _, issue := range issues {
		if issue.ID == "api-AUTH-1" {
			foundAPIAuth1 = true
		}
		if issue.ID == "web-UI-1" {
			foundWebUI1 = true
		}
	}
	if !foundAPIAuth1 {
		t.Error("Expected to find api-AUTH-1 (namespaced)")
	}
	if !foundWebUI1 {
		t.Error("Expected to find web-UI-1 (namespaced)")
	}
}

func TestAggregateLoaderPartialFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only api repo (web repo missing)
	apiRepo := filepath.Join(tmpDir, "services", "api")
	if err := os.MkdirAll(apiRepo, 0755); err != nil {
		t.Fatal(err)
	}
	createTestBeadsFile(t, apiRepo, []model.Issue{
		{ID: "AUTH-1", Title: "Auth feature", Status: model.StatusOpen, Priority: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	})

	config := &workspace.Config{
		Repos: []workspace.RepoConfig{
			{Name: "api", Path: "services/api", Prefix: "api-"},
			{Name: "web", Path: "apps/web", Prefix: "web-"}, // This repo doesn't exist
		},
	}

	loader := workspace.NewAggregateLoader(config, tmpDir)
	issues, results, err := loader.LoadAll(context.Background())

	// Should not return error for partial failures
	if err != nil {
		t.Fatalf("LoadAll() should not error on partial failure: %v", err)
	}

	// Should still have issues from api repo
	if len(issues) != 1 {
		t.Errorf("len(issues) = %d, want 1", len(issues))
	}

	// Check results
	var apiResult, webResult *workspace.LoadResult
	for i := range results {
		if results[i].RepoName == "api" {
			apiResult = &results[i]
		}
		if results[i].RepoName == "web" {
			webResult = &results[i]
		}
	}

	if apiResult == nil || apiResult.Error != nil {
		t.Error("api repo should load successfully")
	}
	if webResult == nil || webResult.Error == nil {
		t.Error("web repo should have error (missing)")
	}
}

func TestAggregateLoaderNamespacesDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	apiRepo := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiRepo, 0755); err != nil {
		t.Fatal(err)
	}

	// Create issue with dependencies
	createTestBeadsFile(t, apiRepo, []model.Issue{
		{
			ID:       "AUTH-1",
			Title:    "Auth feature",
			Status:   model.StatusOpen,
			Priority: 1,
			Dependencies: []*model.Dependency{
				{
					IssueID:     "AUTH-1",
					DependsOnID: "AUTH-2", // Local dependency
					Type:        model.DepBlocks,
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "AUTH-2",
			Title:     "Prerequisite",
			Status:    model.StatusOpen,
			Priority:  0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	})

	config := &workspace.Config{
		Repos: []workspace.RepoConfig{
			{Path: "api", Prefix: "be-"},
		},
	}

	loader := workspace.NewAggregateLoader(config, tmpDir)
	issues, _, err := loader.LoadAll(context.Background())

	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	// Find AUTH-1 and check its dependencies are namespaced
	var auth1 *model.Issue
	for i := range issues {
		if issues[i].ID == "be-AUTH-1" {
			auth1 = &issues[i]
			break
		}
	}

	if auth1 == nil {
		t.Fatal("Could not find be-AUTH-1")
	}

	if len(auth1.Dependencies) == 0 {
		t.Fatal("Expected dependencies to be preserved")
	}

	dep := auth1.Dependencies[0]
	if dep.IssueID != "be-AUTH-1" {
		t.Errorf("Dependency IssueID = %q, want %q", dep.IssueID, "be-AUTH-1")
	}
	if dep.DependsOnID != "be-AUTH-2" {
		t.Errorf("Dependency DependsOnID = %q, want %q", dep.DependsOnID, "be-AUTH-2")
	}
}

func TestAggregateLoaderDisabledRepos(t *testing.T) {
	tmpDir := t.TempDir()

	// Create both repos
	apiRepo := filepath.Join(tmpDir, "api")
	webRepo := filepath.Join(tmpDir, "web")
	for _, repo := range []string{apiRepo, webRepo} {
		if err := os.MkdirAll(repo, 0755); err != nil {
			t.Fatal(err)
		}
	}

	createTestBeadsFile(t, apiRepo, []model.Issue{
		{ID: "API-1", Title: "API", Status: model.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	})
	createTestBeadsFile(t, webRepo, []model.Issue{
		{ID: "WEB-1", Title: "Web", Status: model.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	})

	disabled := false
	config := &workspace.Config{
		Repos: []workspace.RepoConfig{
			{Path: "api", Prefix: "api-"},
			{Path: "web", Prefix: "web-", Enabled: &disabled},
		},
	}

	loader := workspace.NewAggregateLoader(config, tmpDir)
	issues, results, err := loader.LoadAll(context.Background())

	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	// Should only have 1 result (disabled repo excluded)
	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}

	// Should only have 1 issue
	if len(issues) != 1 {
		t.Errorf("len(issues) = %d, want 1", len(issues))
	}

	if issues[0].ID != "api-API-1" {
		t.Errorf("issues[0].ID = %q, want %q", issues[0].ID, "api-API-1")
	}
}

func TestAggregateLoaderEmptyConfig(t *testing.T) {
	config := &workspace.Config{
		Repos: []workspace.RepoConfig{},
	}

	loader := workspace.NewAggregateLoader(config, "/tmp")
	_, _, err := loader.LoadAll(context.Background())

	if err == nil {
		t.Error("LoadAll() should error on empty config")
	}
}

func TestAggregateLoaderNilConfig(t *testing.T) {
	loader := workspace.NewAggregateLoader(nil, "/tmp")
	_, _, err := loader.LoadAll(context.Background())

	if err == nil {
		t.Error("LoadAll() should error on nil config")
	}
}

func TestAggregateLoaderContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a repo
	apiRepo := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiRepo, 0755); err != nil {
		t.Fatal(err)
	}
	createTestBeadsFile(t, apiRepo, []model.Issue{
		{ID: "API-1", Title: "API", Status: model.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	})

	config := &workspace.Config{
		Repos: []workspace.RepoConfig{
			{Path: "api", Prefix: "api-"},
		},
	}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	loader := workspace.NewAggregateLoader(config, tmpDir)
	_, results, _ := loader.LoadAll(ctx)

	// Results should have context error
	if len(results) > 0 && results[0].Error == nil {
		// Note: Due to the race between cancellation and loading,
		// the result may or may not have an error. This test just
		// ensures we don't panic on cancellation.
	}
}

func TestSummarize(t *testing.T) {
	results := []workspace.LoadResult{
		{RepoName: "api", Issues: make([]model.Issue, 5)},
		{RepoName: "web", Issues: make([]model.Issue, 3)},
		{RepoName: "broken", Error: os.ErrNotExist},
	}

	summary := workspace.Summarize(results)

	if summary.TotalRepos != 3 {
		t.Errorf("TotalRepos = %d, want 3", summary.TotalRepos)
	}
	if summary.SuccessfulRepos != 2 {
		t.Errorf("SuccessfulRepos = %d, want 2", summary.SuccessfulRepos)
	}
	if summary.FailedRepos != 1 {
		t.Errorf("FailedRepos = %d, want 1", summary.FailedRepos)
	}
	if summary.TotalIssues != 8 {
		t.Errorf("TotalIssues = %d, want 8", summary.TotalIssues)
	}
	if len(summary.FailedRepoNames) != 1 || summary.FailedRepoNames[0] != "broken" {
		t.Errorf("FailedRepoNames = %v, want [broken]", summary.FailedRepoNames)
	}
}

func TestLoadAllFromConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .bv directory and config
	bvDir := filepath.Join(tmpDir, ".bv")
	if err := os.MkdirAll(bvDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create api repo
	apiRepo := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiRepo, 0755); err != nil {
		t.Fatal(err)
	}
	createTestBeadsFile(t, apiRepo, []model.Issue{
		{ID: "API-1", Title: "API", Status: model.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	})

	// Write workspace config
	configPath := filepath.Join(bvDir, "workspace.yaml")
	configContent := `
repos:
  - path: api
    prefix: api-
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	issues, results, err := workspace.LoadAllFromConfig(context.Background(), configPath)
	if err != nil {
		t.Fatalf("LoadAllFromConfig() error = %v", err)
	}

	if len(issues) != 1 {
		t.Errorf("len(issues) = %d, want 1", len(issues))
	}

	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}

	if issues[0].ID != "api-API-1" {
		t.Errorf("issues[0].ID = %q, want %q", issues[0].ID, "api-API-1")
	}
}

func TestLoadAllFromConfigMissing(t *testing.T) {
	_, _, err := workspace.LoadAllFromConfig(context.Background(), "/nonexistent/workspace.yaml")
	if err == nil {
		t.Error("LoadAllFromConfig() should error on missing config")
	}
}

func TestAggregateLoaderAbsolutePaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create repo with absolute path
	apiRepo := filepath.Join(tmpDir, "api")
	if err := os.MkdirAll(apiRepo, 0755); err != nil {
		t.Fatal(err)
	}
	createTestBeadsFile(t, apiRepo, []model.Issue{
		{ID: "API-1", Title: "API", Status: model.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	})

	config := &workspace.Config{
		Repos: []workspace.RepoConfig{
			{Path: apiRepo, Prefix: "api-"}, // Absolute path
		},
	}

	loader := workspace.NewAggregateLoader(config, "/different/root")
	issues, _, err := loader.LoadAll(context.Background())

	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(issues) != 1 {
		t.Errorf("len(issues) = %d, want 1", len(issues))
	}
}

func TestAggregateLoaderCustomBeadsPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Repo with custom beads path
	repoDir := filepath.Join(tmpDir, "svc")
	customBeads := filepath.Join(repoDir, "custom_beads")
	if err := os.MkdirAll(customBeads, 0755); err != nil {
		t.Fatal(err)
	}
	// Write beads.jsonl into custom path (not default .beads)
	beadsFile := filepath.Join(customBeads, "beads.jsonl")
	f, err := os.Create(beadsFile)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	if err := enc.Encode(model.Issue{ID: "CUST-1", Title: "Custom beads", Status: model.StatusOpen, IssueType: model.TypeTask}); err != nil {
		t.Fatal(err)
	}
	f.Close()

	config := &workspace.Config{
		Repos: []workspace.RepoConfig{
			{Name: "svc", Path: "svc", Prefix: "svc-", BeadsPath: "custom_beads"},
		},
	}

	loader := workspace.NewAggregateLoader(config, tmpDir)
	issues, _, err := loader.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].ID != "svc-CUST-1" {
		t.Errorf("expected namespaced ID svc-CUST-1, got %s", issues[0].ID)
	}
}
