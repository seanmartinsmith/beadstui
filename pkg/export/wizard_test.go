package export

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewWizard(t *testing.T) {
	wizard := NewWizard("/tmp/test")

	if wizard == nil {
		t.Fatal("NewWizard returned nil")
	}

	if wizard.beadsPath != "/tmp/test" {
		t.Errorf("Expected beadsPath '/tmp/test', got %s", wizard.beadsPath)
	}

	if wizard.config == nil {
		t.Error("Expected config to be initialized")
	}

	// Check new defaults
	if !wizard.config.IncludeClosed {
		t.Error("Expected IncludeClosed to be true by default")
	}
	if !wizard.config.IncludeHistory {
		t.Error("Expected IncludeHistory to be true by default")
	}
}

func TestWizardConfig(t *testing.T) {
	config := WizardConfig{
		IncludeClosed:   true,
		IncludeHistory:  true,
		Title:           "Test Title",
		Subtitle:        "Test Subtitle",
		DeployTarget:    "github",
		RepoName:        "test-repo",
		RepoPrivate:     true,
		RepoDescription: "Test description",
		OutputPath:      "/tmp/output",
	}

	if !config.IncludeClosed {
		t.Error("Expected IncludeClosed to be true")
	}

	if !config.IncludeHistory {
		t.Error("Expected IncludeHistory to be true")
	}

	if config.Title != "Test Title" {
		t.Errorf("Expected Title 'Test Title', got %s", config.Title)
	}

	if config.DeployTarget != "github" {
		t.Errorf("Expected DeployTarget 'github', got %s", config.DeployTarget)
	}

	if !config.RepoPrivate {
		t.Error("Expected RepoPrivate to be true")
	}
}

func TestWizardResult(t *testing.T) {
	result := WizardResult{
		BundlePath:   "/tmp/bundle",
		RepoFullName: "user/repo",
		PagesURL:     "https://user.github.io/repo/",
		DeployTarget: "github",
	}

	if result.BundlePath != "/tmp/bundle" {
		t.Errorf("Expected BundlePath '/tmp/bundle', got %s", result.BundlePath)
	}

	if result.RepoFullName != "user/repo" {
		t.Errorf("Expected RepoFullName 'user/repo', got %s", result.RepoFullName)
	}

	if result.PagesURL != "https://user.github.io/repo/" {
		t.Errorf("Expected PagesURL 'https://user.github.io/repo/', got %s", result.PagesURL)
	}
}

func TestWizard_GetConfig(t *testing.T) {
	wizard := NewWizard("/tmp/test")

	config := wizard.GetConfig()
	if config == nil {
		t.Fatal("GetConfig returned nil")
	}

	// Default values - both IncludeClosed and IncludeHistory default to true
	if !config.IncludeClosed {
		t.Error("Expected IncludeClosed to be true by default")
	}

	if !config.IncludeHistory {
		t.Error("Expected IncludeHistory to be true by default")
	}

	if config.DeployTarget != "" {
		t.Errorf("Expected empty DeployTarget by default, got %s", config.DeployTarget)
	}
}

func TestWizardConfigPath(t *testing.T) {
	path := WizardConfigPath()

	// Should return a valid path (or empty if no home dir)
	if path != "" {
		if !filepath.IsAbs(path) {
			t.Errorf("Expected absolute path, got %s", path)
		}

		// Should end with pages-wizard.json
		if filepath.Base(path) != "pages-wizard.json" {
			t.Errorf("Expected path to end with pages-wizard.json, got %s", path)
		}
	}
}

func TestSaveAndLoadWizardConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	loaded, err := LoadWizardConfig()
	if err != nil {
		t.Fatalf("LoadWizardConfig returned error: %v", err)
	}
	if loaded != nil {
		t.Fatalf("Expected nil config when file doesn't exist, got %+v", loaded)
	}

	config := &WizardConfig{
		IncludeClosed:  true,
		IncludeHistory: true,
		Title:          "Saved Title",
		DeployTarget:   "github",
		RepoName:       "saved-repo",
		RepoPrivate:    true,
	}

	if err := SaveWizardConfig(config); err != nil {
		t.Fatalf("SaveWizardConfig returned error: %v", err)
	}

	loaded, err = LoadWizardConfig()
	if err != nil {
		t.Fatalf("LoadWizardConfig after save returned error: %v", err)
	}
	if loaded == nil {
		t.Fatal("Expected loaded config, got nil")
	}
	if loaded.Title != "Saved Title" {
		t.Fatalf("Expected loaded Title %q, got %q", "Saved Title", loaded.Title)
	}
	if loaded.RepoName != "saved-repo" {
		t.Fatalf("Expected loaded RepoName %q, got %q", "saved-repo", loaded.RepoName)
	}
	if loaded.DeployTarget != "github" {
		t.Fatalf("Expected loaded DeployTarget %q, got %q", "github", loaded.DeployTarget)
	}
}

func TestLoadWizardConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configPath := WizardConfigPath()
	if configPath == "" {
		t.Skip("WizardConfigPath returned empty path")
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("{"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadWizardConfig()
	if err == nil {
		t.Fatal("Expected LoadWizardConfig to return error for invalid JSON")
	}
}

func TestWizard_PerformExport(t *testing.T) {
	wizard := NewWizard("/tmp/test")

	tmpDir := t.TempDir()
	err := wizard.PerformExport(tmpDir)
	if err != nil {
		t.Errorf("PerformExport returned unexpected error: %v", err)
	}

	if wizard.bundlePath != tmpDir {
		t.Errorf("Expected bundlePath %s, got %s", tmpDir, wizard.bundlePath)
	}
}

func TestWizard_PrintBanner(t *testing.T) {
	wizard := NewWizard("/tmp/test")

	// Just verify it doesn't panic
	wizard.printBanner()
}

func TestWizard_PrintSuccess_GitHub(t *testing.T) {
	wizard := NewWizard("/tmp/test")

	result := &WizardResult{
		BundlePath:   "/tmp/bundle",
		RepoFullName: "user/repo",
		PagesURL:     "https://user.github.io/repo/",
		DeployTarget: "github",
	}

	// Just verify it doesn't panic
	wizard.PrintSuccess(result)
}

func TestWizard_PrintSuccess_Local(t *testing.T) {
	wizard := NewWizard("/tmp/test")

	result := &WizardResult{
		BundlePath:   "/tmp/bundle",
		DeployTarget: "local",
	}

	// Just verify it doesn't panic
	wizard.PrintSuccess(result)
}

func TestWizard_PrintSuccess_Cloudflare(t *testing.T) {
	wizard := NewWizard("/tmp/test")

	result := &WizardResult{
		BundlePath:        "/tmp/bundle",
		CloudflareProject: "my-project",
		CloudflareURL:     "https://my-project.pages.dev",
		DeployTarget:      "cloudflare",
	}

	// Just verify it doesn't panic
	wizard.PrintSuccess(result)
}
