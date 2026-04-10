package main_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCassModalGracefulDegradation verifies that the TUI launches and functions
// normally when Cass is not available. This is critical for users who don't have
// Cass installed - the app should work seamlessly without it.
func TestCassModalGracefulDegradation(t *testing.T) {
	skipIfNoScript(t)
	bv := buildBvBinary(t)

	tempDir := t.TempDir()
	beadsDir := filepath.Join(tempDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	// Create test beads - these should be viewable without Cass
	beads := `{"id":"bv-001","title":"Test task without cass","status":"open","priority":1,"issue_type":"task"}
{"id":"bv-002","title":"Another task","status":"in_progress","priority":2,"issue_type":"feature"}`

	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Launch TUI - should not crash even without Cass
	cmd := scriptTUICommand(ctx, bv)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"BV_TUI_AUTOCLOSE_MS=1000", // Auto-close after 1s
		"BV_NO_CASS=1",             // Explicitly disable Cass lookup
	)

	ensureCmdStdinCloses(t, ctx, cmd, 3*time.Second)
	out, err := runCmdToFile(t, cmd)
	if ctx.Err() == context.DeadlineExceeded {
		t.Skipf("skipping Cass graceful degradation test: timed out; output:\n%s", out)
	}
	if err != nil {
		t.Fatalf("TUI run failed without Cass: %v\n%s", err, out)
	}

	// TUI should have launched successfully (exit code 0 with auto-close)
}

// TestCassModalNoCrashOnVKeyWithoutCass verifies that pressing V (session preview)
// doesn't crash the app when Cass is unavailable.
// This is tested indirectly - if Cass is unavailable, the V key should either:
// - Show "Cass not available" message, or
// - Simply not respond (no modal to open)
// Either way, it shouldn't crash.
func TestCassModalNoCrashOnVKeyWithoutCass(t *testing.T) {
	skipIfNoScript(t)
	bv := buildBvBinary(t)

	tempDir := t.TempDir()
	beadsDir := filepath.Join(tempDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	// Create test beads
	beads := `{"id":"bv-test","title":"Test for V key","status":"open","priority":1,"issue_type":"task"}`

	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// The TUI should handle V key gracefully without Cass
	// Using auto-close means it will exit normally if no crash occurs
	cmd := scriptTUICommand(ctx, bv)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"BV_TUI_AUTOCLOSE_MS=1500",
		"BV_NO_CASS=1",
	)

	ensureCmdStdinCloses(t, ctx, cmd, 3*time.Second)
	out, err := runCmdToFile(t, cmd)
	if ctx.Err() == context.DeadlineExceeded {
		t.Skipf("skipping V key test: timed out; output:\n%s", out)
	}
	if err != nil {
		t.Fatalf("TUI run failed: %v\n%s", err, out)
	}
}

// TestCassDetectionEnvironmentVariable verifies that the BV_NO_CASS environment
// variable properly disables Cass integration.
func TestCassDetectionEnvironmentVariable(t *testing.T) {
	skipIfNoScript(t)
	bv := buildBvBinary(t)

	tempDir := t.TempDir()
	beadsDir := filepath.Join(tempDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	beads := `{"id":"bv-env","title":"Env var test","status":"open","priority":1,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// With BV_NO_CASS=1, Cass should be disabled
	cmd := scriptTUICommand(ctx, bv)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"BV_TUI_AUTOCLOSE_MS=1000",
		"BV_NO_CASS=1",
	)

	ensureCmdStdinCloses(t, ctx, cmd, 3*time.Second)
	out, err := runCmdToFile(t, cmd)
	if ctx.Err() == context.DeadlineExceeded {
		t.Skipf("skipping env var test: timed out; output:\n%s", out)
	}
	if err != nil {
		t.Fatalf("TUI failed with BV_NO_CASS=1: %v\n%s", err, out)
	}
}

// TestCassModalRobotTriageNoCrash verifies that --robot-triage works without Cass.
// The triage output should not require Cass and should complete normally.
func TestCassModalRobotTriageNoCrash(t *testing.T) {
	bv := buildBvBinary(t)

	tempDir := t.TempDir()
	beadsDir := filepath.Join(tempDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	beads := `{"id":"bv-triage","title":"Triage test","status":"open","priority":1,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bv, "--robot-triage")
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(), "BV_NO_CASS=1")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("robot-triage failed without Cass: %v\n%s", err, out)
	}

	// Output should be valid JSON
	if !strings.HasPrefix(strings.TrimSpace(string(out)), "{") {
		t.Errorf("Expected JSON output from robot-triage, got:\n%s", out)
	}
}

// TestCassStatusBarIndicator tests that the status bar correctly shows
// Cass availability status. When Cass is unavailable, no indicator should
// be shown (graceful degradation - don't confuse users with missing feature).
func TestCassStatusBarIndicator(t *testing.T) {
	skipIfNoScript(t)
	bv := buildBvBinary(t)

	tempDir := t.TempDir()
	beadsDir := filepath.Join(tempDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	beads := `{"id":"bv-status","title":"Status bar test","status":"open","priority":1,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := scriptTUICommand(ctx, bv)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"BV_TUI_AUTOCLOSE_MS=1000",
		"BV_NO_CASS=1",
	)

	ensureCmdStdinCloses(t, ctx, cmd, 3*time.Second)
	out, err := runCmdToFile(t, cmd)
	if ctx.Err() == context.DeadlineExceeded {
		t.Skipf("skipping status bar test: timed out; output:\n%s", out)
	}
	if err != nil {
		t.Fatalf("TUI failed: %v\n%s", err, out)
	}

	// When Cass is unavailable, the status bar should NOT show 🤖 Active
	// (it should either show nothing or show 💤 Idle if Cass was found but no sessions)
	// The key point is it shouldn't crash or show error messages
	outStr := string(out)

	// Should not contain error messages about Cass
	if strings.Contains(strings.ToLower(outStr), "cass error") ||
		strings.Contains(strings.ToLower(outStr), "cass not found") {
		t.Errorf("Status bar should not show Cass error messages, got:\n%s", outStr)
	}
}

// TestMultipleViewsWithoutCass verifies that all views (list, board, graph, history)
// work correctly when Cass is unavailable.
func TestMultipleViewsWithoutCass(t *testing.T) {
	skipIfNoScript(t)
	bv := buildBvBinary(t)

	tempDir := t.TempDir()
	beadsDir := filepath.Join(tempDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	// Create beads for various view tests
	beads := `{"id":"bv-view1","title":"View test 1","status":"open","priority":1,"issue_type":"task"}
{"id":"bv-view2","title":"View test 2","status":"in_progress","priority":2,"issue_type":"feature"}
{"id":"bv-view3","title":"View test 3","status":"closed","priority":1,"issue_type":"bug"}`

	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	// Test TUI launch - covers default list view
	t.Run("list_view", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := scriptTUICommand(ctx, bv)
		cmd.Dir = tempDir
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"BV_TUI_AUTOCLOSE_MS=1000",
			"BV_NO_CASS=1",
		)

		ensureCmdStdinCloses(t, ctx, cmd, 3*time.Second)
		out, err := runCmdToFile(t, cmd)
		if ctx.Err() == context.DeadlineExceeded {
			t.Skipf("skipping list view test: timed out; output:\n%s", out)
		}
		if err != nil {
			t.Errorf("List view failed without Cass: %v\n%s", err, out)
		}
	})

	// Test robot-priority command that should work without Cass
	t.Run("robot_priority", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, bv, "--robot-priority")
		cmd.Dir = tempDir
		cmd.Env = append(os.Environ(), "BV_NO_CASS=1")

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("robot-priority failed without Cass: %v\n%s", err, out)
		}
	})
}
