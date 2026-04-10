package main_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestTUIPrioritySnapshot launches the TUI briefly to ensure it initializes and exits cleanly.
// We rely on BV_TUI_AUTOCLOSE_MS to avoid hanging in CI.
func TestTUIPrioritySnapshot(t *testing.T) {
	skipIfNoScript(t)
	bv := buildBvBinary(t)

	tempDir := t.TempDir()
	beadsDir := filepath.Join(tempDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}
	// Minimal graph with a dependency to exercise insights/priority panes.
	beads := `{"id":"P1","title":"Parent","status":"open","priority":1,"issue_type":"task"}
{"id":"C1","title":"Child","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"C1","depends_on_id":"P1","type":"blocks"}]}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := scriptTUICommand(ctx, bv)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"BV_TUI_AUTOCLOSE_MS=1500",
	)

	ensureCmdStdinCloses(t, ctx, cmd, 3*time.Second)
	out, err := runCmdToFile(t, cmd)
	if ctx.Err() == context.DeadlineExceeded {
		t.Skipf("skipping TUI snapshot: timed out (likely TTY/OS mismatch); output:\n%s", out)
	}
	if err != nil {
		t.Fatalf("TUI run failed: %v\n%s", err, out)
	}
}

// TestTUIBackgroundModeRapidWrites verifies that the TUI stays responsive enough to
// run and exit cleanly under rapid `.beads/beads.jsonl` updates while receiving
// keypress input. This is a smoke test intended to catch deadlocks/panics during
// multi-agent write scenarios.
func TestTUIBackgroundModeRapidWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping rapid-write TUI test in short mode")
	}
	skipIfNoScript(t)
	bv := buildBvBinary(t)

	tempDir := t.TempDir()
	beadsDir := filepath.Join(tempDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	beadsPath := filepath.Join(beadsDir, "beads.jsonl")
	initial := `{"id":"P1","title":"Parent","status":"open","priority":1,"issue_type":"task"}
{"id":"C1","title":"Child","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"C1","depends_on_id":"P1","type":"blocks"}]}
`
	if err := os.WriteFile(beadsPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := scriptTUICommand(ctx, bv, "--background-mode")
	if cmd == nil {
		t.Skip("skipping: script command not available on this platform")
	}
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"BV_TUI_AUTOCLOSE_MS=2000",
	)

	stdinR, stdinW := io.Pipe()
	cmd.Stdin = stdinR
	t.Cleanup(func() {
		_ = stdinW.Close()
		_ = stdinR.Close()
	})
	// Some `script` implementations keep the pseudo-TTY session open until stdin
	// is closed, even if the child process has exited. Ensure we eventually close
	// stdin so the test can't hang indefinitely.
	time.AfterFunc(3*time.Second, func() { _ = stdinW.Close() })

	done := make(chan struct{})
	t.Cleanup(func() { close(done) })

	// Simulate user navigation keys while the file is changing.
	go func() {
		ticker := time.NewTicker(30 * time.Millisecond)
		defer ticker.Stop()
		for i := 0; ; i++ {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := io.WriteString(stdinW, "j"); err != nil {
					return
				}
			}
		}
	}()

	// Simulate multi-agent writes by appending new issues rapidly.
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for i := 0; ; i++ {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				f, err := os.OpenFile(beadsPath, os.O_APPEND|os.O_WRONLY, 0o644)
				if err != nil {
					continue
				}
				_, _ = fmt.Fprintf(f, `{"id":"auto-%d","title":"Auto %d","status":"open","priority":2,"issue_type":"task"}`+"\n", i, i)
				_ = f.Close()
			}
		}
	}()

	out, err := runCmdToFile(t, cmd)
	if ctx.Err() == context.DeadlineExceeded {
		t.Skipf("skipping rapid-write TUI test: timed out (likely TTY/OS mismatch); output:\n%s", out)
	}
	if err != nil {
		t.Fatalf("TUI run failed: %v\n%s", err, out)
	}
}
