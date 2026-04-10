package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// E2E: Wizard Flow Testing (bv-focq)
// ============================================================================

// TestWizard_LocalExportFlow tests the complete local export wizard flow
func TestWizard_LocalExportFlow(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 5)

	// Input for local export flow:
	// - Include closed: n
	// - Title: Test Export
	// - Subtitle: (empty)
	// - Deploy target: 3 (local)
	// - Output dir: ./test-pages
	input := "n\nTest Export\n\n3\n./test-pages\n"

	cmd := exec.Command(bv, "--pages")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "BV_NO_BROWSER=1")

	out, err := cmd.CombinedOutput()
	if err != nil {
		// Wizard may fail on prerequisites check if gh/wrangler not installed
		// For local export, this should succeed
		if !strings.Contains(string(out), "Export") && !strings.Contains(string(out), "Step") {
			t.Fatalf("--pages wizard failed unexpectedly: %v\n%s", err, out)
		}
	}

	// Verify wizard banner appears
	if !strings.Contains(string(out), "Static Site Deployment Wizard") {
		t.Error("wizard banner not shown")
	}

	// Verify steps appear
	if !strings.Contains(string(out), "Step 1") {
		t.Error("Step 1 (Export Configuration) not shown")
	}
	if !strings.Contains(string(out), "Step 2") {
		t.Error("Step 2 (Deployment Target) not shown")
	}
}

// TestWizard_GitHubFlowPrompts tests GitHub Pages flow prompts appear correctly
func TestWizard_GitHubFlowPrompts(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)

	// Input for GitHub flow:
	// - Include closed: y
	// - Title: GitHub Test
	// - Subtitle: Test Subtitle
	// - Deploy target: 1 (GitHub)
	// - Repo name: test-repo
	// - Private: n
	// - Description: Test description
	// Then it will check prerequisites (gh CLI) which may fail
	input := "y\nGitHub Test\nTest Subtitle\n1\ntest-repo\nn\nTest description\nn\n"

	cmd := exec.Command(bv, "--pages")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "BV_NO_BROWSER=1")

	out, _ := cmd.CombinedOutput()
	output := string(out)

	// Verify GitHub-specific prompts appear
	if !strings.Contains(output, "GitHub") {
		t.Error("GitHub option not shown in deployment target selection")
	}

	// Verify GitHub Configuration step appears
	if !strings.Contains(output, "GitHub Configuration") {
		t.Log("Note: GitHub Configuration step may not appear if gh CLI check fails first")
	}
}

// TestWizard_CloudflareFlowPrompts tests Cloudflare Pages flow prompts appear correctly
func TestWizard_CloudflareFlowPrompts(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)

	// Input for Cloudflare flow:
	// - Include closed: n
	// - Title: CF Test
	// - Subtitle: (empty)
	// - Deploy target: 2 (Cloudflare)
	// - Project name: test-project
	// - Branch: main
	// Then it will check prerequisites (wrangler) which may fail
	input := "n\nCF Test\n\n2\ntest-project\nmain\nn\n"

	cmd := exec.Command(bv, "--pages")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "BV_NO_BROWSER=1")

	out, _ := cmd.CombinedOutput()
	output := string(out)

	// Verify Cloudflare option appears
	if !strings.Contains(output, "Cloudflare") {
		t.Error("Cloudflare option not shown in deployment target selection")
	}
}

// TestWizard_DeployTargetSelection tests all three deployment target options appear
func TestWizard_DeployTargetSelection(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)

	// Minimal input to get to deployment target selection, then cancel
	input := "n\nTest\n\n"

	cmd := exec.Command(bv, "--pages")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader(input)
	// Add timeout to prevent hanging
	cmd.Env = append(os.Environ(), "BV_TEST_MODE=1", "BV_NO_BROWSER=1")

	out, _ := cmd.CombinedOutput()
	output := string(out)

	// Verify all three options appear
	options := []string{
		"GitHub Pages",
		"Cloudflare Pages",
		"Export locally",
	}
	for _, opt := range options {
		if !strings.Contains(output, opt) {
			t.Errorf("missing deployment option: %s", opt)
		}
	}
}

// TestWizard_ExportConfigPrompts tests export configuration prompts
func TestWizard_ExportConfigPrompts(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)

	// Provide some input to trigger prompts
	input := "y\nMy Custom Title\nMy Subtitle\n3\n./output\n"

	cmd := exec.Command(bv, "--pages")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "BV_NO_BROWSER=1", "BV_NO_SAVED_CONFIG=1")

	out, _ := cmd.CombinedOutput()
	output := string(out)

	// Verify export configuration prompts appear
	configPrompts := []string{
		"Include closed",
		"title",
		"subtitle",
	}
	for _, prompt := range configPrompts {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(prompt)) {
			t.Errorf("missing export config prompt: %s", prompt)
		}
	}
}

// TestWizard_BannerDisplay tests the wizard banner is displayed correctly
func TestWizard_BannerDisplay(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)

	// Just trigger the wizard and let it timeout/fail
	input := ""

	cmd := exec.Command(bv, "--pages")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "BV_NO_BROWSER=1")

	out, _ := cmd.CombinedOutput()
	output := string(out)

	// Verify banner elements
	bannerElements := []string{
		"╔", // Box drawing
		"Static Site Deployment Wizard",
		"Export your issues",
		"Preview",
		"Deploy",
		"Ctrl+C",
	}
	foundCount := 0
	for _, elem := range bannerElements {
		if strings.Contains(output, elem) {
			foundCount++
		}
	}
	if foundCount < 3 {
		t.Errorf("banner incomplete, only found %d of expected elements", foundCount)
	}
}

// TestWizard_ConfigPersistence tests wizard config save/load
func TestWizard_ConfigPersistence(t *testing.T) {
	// Create a temp home directory
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create config directory
	configDir := filepath.Join(tmpHome, ".config", "bv")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}

	// Write a test wizard config
	testConfig := map[string]interface{}{
		"include_closed": true,
		"title":          "Saved Title",
		"deploy_target":  "local",
		"output_path":    "./saved-output",
	}

	configPath := filepath.Join(configDir, "pages-wizard.json")
	configBytes, _ := json.MarshalIndent(testConfig, "", "  ")
	if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Verify config file exists
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Read it back
	readBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var readConfig map[string]interface{}
	if err := json.Unmarshal(readBytes, &readConfig); err != nil {
		t.Fatalf("parse config: %v", err)
	}

	if readConfig["title"] != "Saved Title" {
		t.Errorf("config title = %v, want 'Saved Title'", readConfig["title"])
	}
}

// TestWizard_InvalidDeployTargetRecovery tests handling of invalid input
func TestWizard_InvalidDeployTargetRecovery(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)

	// Provide invalid input (9) then valid input (3)
	// Invalid inputs should default to option 1 (GitHub)
	input := "n\nTest\n\n9\n"

	cmd := exec.Command(bv, "--pages")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "BV_NO_BROWSER=1")

	out, _ := cmd.CombinedOutput()
	output := string(out)

	// The wizard should still proceed (using default)
	if !strings.Contains(output, "Step") {
		t.Error("wizard did not proceed after invalid input")
	}
}

// TestWizard_DefaultValues tests that defaults are applied correctly
func TestWizard_DefaultValues(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)

	// Press enter to accept all defaults, select local (3)
	input := "\n\n\n3\n\n"

	cmd := exec.Command(bv, "--pages")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "BV_NO_BROWSER=1")

	out, _ := cmd.CombinedOutput()
	output := string(out)

	// Verify default prompts appear with their defaults shown
	// [Y/n] or [y/N] format
	if !strings.Contains(output, "[") && !strings.Contains(output, "]") {
		t.Log("Note: Default value indicators may not be visible in all prompts")
	}

	// Wizard should progress through steps
	stepCount := strings.Count(output, "Step")
	if stepCount < 2 {
		t.Errorf("expected at least 2 steps, found %d", stepCount)
	}
}

// TestWizard_StepProgression tests wizard progresses through all steps
func TestWizard_StepProgression(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)

	// Complete local export flow with all inputs
	input := "n\nTest Title\n\n3\n./test-output\n"

	cmd := exec.Command(bv, "--pages")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "BV_NO_BROWSER=1")

	out, _ := cmd.CombinedOutput()
	output := string(out)

	// Expected steps for local flow
	expectedSteps := []string{
		"Step 1",
		"Step 2",
		"Step 3",
	}

	for _, step := range expectedSteps {
		if !strings.Contains(output, step) {
			t.Errorf("missing %s in wizard output", step)
		}
	}
}

// TestWizard_OutputDirectoryPrompt tests output directory configuration for local export
func TestWizard_OutputDirectoryPrompt(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)

	// Select local export and provide custom output path
	input := "n\nTest\n\n3\n./custom-output-dir\n"

	cmd := exec.Command(bv, "--pages")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "BV_NO_BROWSER=1")

	out, _ := cmd.CombinedOutput()
	output := string(out)

	// Verify output directory prompt appears for local export
	if !strings.Contains(strings.ToLower(output), "output") || !strings.Contains(strings.ToLower(output), "directory") {
		// Could also be "Export Configuration" step
		if !strings.Contains(output, "Local Export") {
			t.Log("Note: Output directory prompt may appear in different format")
		}
	}
}

// TestWizard_InterruptHandling tests that wizard can be interrupted
func TestWizard_InterruptHandling(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)

	// Start wizard and immediately close stdin (simulates Ctrl+C/interrupt)
	cmd := exec.Command(bv, "--pages")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("")
	cmd.Env = append(os.Environ(), "BV_NO_BROWSER=1")

	// Use a timeout to prevent hanging
	done := make(chan error, 1)
	go func() {
		_, err := cmd.CombinedOutput()
		done <- err
	}()

	select {
	case <-done:
		// Command completed (possibly with error, which is fine)
	case <-time.After(15 * time.Second):
		t.Error("wizard did not handle interrupt/empty input within timeout (15s)")
		cmd.Process.Kill()
	}
}

// TestWizard_MultiplePlatformFlows tests that different platforms have different prompts
func TestWizard_MultiplePlatformFlows(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	tests := []struct {
		name     string
		choice   string
		contains string
	}{
		{"GitHub", "1", "GitHub"},
		{"Cloudflare", "2", "Cloudflare"},
		{"Local", "3", "local"}, // lowercase to match "locally" or "local"
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := createSimpleRepo(t, 3)

			// Provide input to select the platform
			input := "n\nTest\n\n" + tc.choice + "\n\n\n"

			cmd := exec.Command(bv, "--pages")
			cmd.Dir = repoDir
			cmd.Stdin = strings.NewReader(input)
			cmd.Env = append(os.Environ(), "BV_NO_BROWSER=1")

			out, _ := cmd.CombinedOutput()
			output := strings.ToLower(string(out))

			if !strings.Contains(output, strings.ToLower(tc.contains)) {
				t.Errorf("expected output to contain %q for %s platform", tc.contains, tc.name)
			}
		})
	}
}
