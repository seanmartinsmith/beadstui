package bdexec

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestResultArgv(t *testing.T) {
	r := Result{Args: []string{"bd", "update", "bt-1", "--claim"}}
	if got, want := r.Argv(), "bd update bt-1 --claim"; got != want {
		t.Errorf("Argv() = %q, want %q", got, want)
	}
}

// TestRunPopulatesArgvOnSpawnFailure proves Args is always set - even when bd
// cannot be reached - so a trace/receipt can record what was attempted. Uses a
// nonexistent working directory to force a spawn failure without depending on
// whether bd is installed.
func TestRunPopulatesArgvOnSpawnFailure(t *testing.T) {
	res := Run(context.Background(), t.TempDir()+"/does-not-exist", "update", "bt-1", "--claim")
	want := []string{"bd", "update", "bt-1", "--claim"}
	if len(res.Args) != len(want) {
		t.Fatalf("Args = %v, want %v", res.Args, want)
	}
	for i := range want {
		if res.Args[i] != want[i] {
			t.Fatalf("Args = %v, want %v", res.Args, want)
		}
	}
	if res.Err == nil {
		t.Error("expected an error running bd in a nonexistent directory")
	}
	if res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 for a non-exit failure", res.ExitCode)
	}
}

// TestRunAgainstRealBd exercises the happy path when bd is installed: a plain
// `bd --version` must exit 0 with the argv recorded. Skipped when bd is absent.
func TestRunAgainstRealBd(t *testing.T) {
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not on PATH")
	}
	res := RunWithTimeout(context.Background(), "", 15*time.Second, "--version")
	if res.Err != nil || res.ExitCode != 0 {
		t.Fatalf("bd --version: exit=%d err=%v stderr=%q", res.ExitCode, res.Err, res.Stderr)
	}
	if res.Argv() != "bd --version" {
		t.Errorf("Argv() = %q, want %q", res.Argv(), "bd --version")
	}
}
