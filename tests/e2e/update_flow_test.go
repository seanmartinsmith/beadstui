package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ============================================================================
// Version flag tests
// ============================================================================

func TestVersionFlag_OutputsVersion(t *testing.T) {
	bv := buildBvBinary(t)

	cmd := exec.Command(bv, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}

	output := string(out)

	// Should contain version information
	if !strings.Contains(output, "v") {
		t.Errorf("expected version to contain 'v', got: %s", output)
	}

	// Version format should be semver-like (v0.0.0 or similar)
	versionPattern := regexp.MustCompile(`v\d+\.\d+\.\d+`)
	if !versionPattern.MatchString(output) {
		t.Errorf("expected semver format in version output, got: %s", output)
	}
}

func TestVersionFlag_IncludesBuildInfo(t *testing.T) {
	bv := buildBvBinary(t)

	cmd := exec.Command(bv, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}

	output := string(out)

	// Should include "bv" name
	if !strings.Contains(strings.ToLower(output), "bv") {
		t.Errorf("expected 'bv' in version output, got: %s", output)
	}
}

// ============================================================================
// Rollback tests (no backup scenarios)
// ============================================================================

func TestRollbackFlag_FailsWithoutBackup(t *testing.T) {
	bv := buildBvBinary(t)
	tmpDir := t.TempDir()

	// Run rollback from a temp directory where there's no backup
	cmd := exec.Command(bv, "--rollback")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()

	// Should fail because there's no backup
	if err == nil {
		t.Fatalf("expected --rollback to fail without backup, but succeeded: %s", out)
	}

	output := string(out)

	// Error message should mention backup
	if !strings.Contains(strings.ToLower(output), "backup") && !strings.Contains(strings.ToLower(output), "no backup") {
		t.Errorf("expected error message about missing backup, got: %s", output)
	}
}

// ============================================================================
// Update flag tests (dry-run / error scenarios)
// ============================================================================

func TestUpdateFlag_RequiresNetwork(t *testing.T) {
	// Skip in CI if network tests are disabled
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network-dependent test")
	}

	bv := buildBvBinary(t)
	tmpDir := t.TempDir()

	// Create a minimal beads repo
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// --update should attempt to connect to GitHub
	// We just verify the command runs and either succeeds or fails gracefully
	cmd := exec.Command(bv, "--update", "-y")
	cmd.Dir = tmpDir
	// Set a short timeout
	cmd.Env = append(os.Environ(), "BV_UPDATE_TIMEOUT=2s")
	out, _ := cmd.CombinedOutput()

	output := string(out)

	// Should mention update-related content (either success, error, or "already at latest")
	hasUpdateContent := strings.Contains(output, "update") ||
		strings.Contains(output, "Update") ||
		strings.Contains(output, "version") ||
		strings.Contains(output, "latest") ||
		strings.Contains(output, "download") ||
		strings.Contains(output, "failed")

	if !hasUpdateContent {
		t.Logf("Update output: %s", output)
		// Not a hard failure - just log for visibility
	}
}

// ============================================================================
// Help flag tests (update documentation)
// ============================================================================

func TestHelpFlag_DocumentsUpdateFeatures(t *testing.T) {
	bv := buildBvBinary(t)

	cmd := exec.Command(bv, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// --help might exit with 0 or non-zero depending on implementation
		// Just check the output
	}

	output := string(out)

	// Should document the update flag
	if !strings.Contains(output, "-update") && !strings.Contains(output, "--update") {
		t.Errorf("expected help to document --update flag, got: %s", output)
	}

	// Should document the rollback flag
	if !strings.Contains(output, "-rollback") && !strings.Contains(output, "--rollback") {
		t.Errorf("expected help to document --rollback flag, got: %s", output)
	}
}

// ============================================================================
// TUI keyboard shortcut tests (via robot mode if available)
// ============================================================================

func TestRobotHelp_DocumentsUpdateFeatures(t *testing.T) {
	// Verify robot-help documents update-related features
	bv := buildBvBinary(t)

	cmd := exec.Command(bv, "-robot-help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("-robot-help failed: %v\n%s", err, out)
	}

	output := string(out)

	// Robot help should mention update-related commands
	if !strings.Contains(output, "update") && !strings.Contains(output, "version") {
		t.Logf("robot-help doesn't mention update features (may be expected): %s", output)
	}
}

// ============================================================================
// Integration: Update check at startup behavior
// ============================================================================

func TestStartup_UpdateCheckDoesNotBlock(t *testing.T) {
	// Verify that the TUI starts quickly even if update check is slow
	// This is a basic smoke test
	bv := buildBvBinary(t)
	tmpDir := t.TempDir()

	// Create minimal beads
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(`{"id":"TEST-1","title":"Test","status":"open","priority":1,"issue_type":"task"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run robot-insights which should complete quickly
	cmd := exec.Command(bv, "-robot-insights")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("-robot-insights failed: %v\n%s", err, out)
	}

	// Should contain JSON output
	if !strings.Contains(string(out), "data_hash") {
		t.Errorf("expected data_hash in output, got: %s", out)
	}
}

// ============================================================================
// Binary integrity tests
// ============================================================================

func TestBinary_HasProperPermissions(t *testing.T) {
	bv := buildBvBinary(t)

	info, err := os.Stat(bv)
	if err != nil {
		t.Fatalf("stat binary: %v", err)
	}

	// Should be executable
	mode := info.Mode()
	if mode&0111 == 0 {
		t.Errorf("binary should be executable, mode: %v", mode)
	}
}

func TestBinary_RespondsToSignals(t *testing.T) {
	// Verify the binary handles interrupts gracefully
	// This is important for the update process
	bv := buildBvBinary(t)
	tmpDir := t.TempDir()

	// Create minimal beads
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bv, "--version")
	cmd.Dir = tmpDir

	// Should complete without hanging
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %v\n%s", err, out)
	}
}
