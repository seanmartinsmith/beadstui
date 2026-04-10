package export

import (
	"os"
	"testing"
)

// Note: Interactive form tests have been removed since huh forms
// cannot be easily tested with stdin injection. The wizard's interactive
// behavior is tested via E2E tests instead.

func TestWizard_PerformDeploy_Local(t *testing.T) {
	wizard := NewWizard("/tmp/test")
	wizard.config.DeployTarget = "local"
	wizard.bundlePath = "/tmp/bundle"

	result, err := wizard.PerformDeploy()
	if err != nil {
		t.Fatalf("PerformDeploy returned error: %v", err)
	}
	if result == nil {
		t.Fatal("PerformDeploy returned nil result")
	}
	if result.DeployTarget != "local" {
		t.Fatalf("Expected DeployTarget %q, got %q", "local", result.DeployTarget)
	}
	if result.BundlePath != "/tmp/bundle" {
		t.Fatalf("Expected BundlePath %q, got %q", "/tmp/bundle", result.BundlePath)
	}
}

func TestWizard_collectTargetConfig_NoTarget(t *testing.T) {
	wizard := NewWizard("/tmp/test")
	// Empty deploy target should return nil error
	wizard.config.DeployTarget = ""
	err := wizard.collectTargetConfig()
	if err != nil {
		t.Fatalf("collectTargetConfig returned error: %v", err)
	}
}

func TestWizard_SuggestProjectName(t *testing.T) {
	// Only test cases that don't depend on cwd
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "/path/to/my_project/bv-pages", "my-project-pages"},
		{"with hyphens", "/path/to/my-project/bv-pages", "my-project-pages"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SuggestProjectName(tt.input)
			if result != tt.expected {
				t.Errorf("SuggestProjectName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestWizard_printBanner_NoPanic(t *testing.T) {
	wizard := NewWizard("/tmp/test")
	// Redirect stdout to discard
	old := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = old }()

	// Should not panic
	wizard.printBanner()
}
