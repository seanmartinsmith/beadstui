package agents

import "testing"

func TestShouldSuppressTTYQueries_EnvRobot(t *testing.T) {
	if !shouldSuppressTTYQueries([]string{"bv"}, true, false) {
		t.Fatal("expected envRobot=true to suppress TTY queries")
	}
}

func TestShouldSuppressTTYQueries_EnvTest(t *testing.T) {
	if !shouldSuppressTTYQueries([]string{"bv"}, false, true) {
		t.Fatal("expected envTest=true to suppress TTY queries")
	}
}

func TestShouldSuppressTTYQueries_RobotFlag(t *testing.T) {
	if !shouldSuppressTTYQueries([]string{"bv", "--robot-triage"}, false, false) {
		t.Fatal("expected --robot-triage to suppress TTY queries")
	}
	if !shouldSuppressTTYQueries([]string{"bv", "--robot-file-beads=path/to/file.go"}, false, false) {
		t.Fatal("expected --robot-file-beads=... to suppress TTY queries")
	}
}

func TestShouldSuppressTTYQueries_HelpAndVersion(t *testing.T) {
	if !shouldSuppressTTYQueries([]string{"bv", "--help"}, false, false) {
		t.Fatal("expected --help to suppress TTY queries")
	}
	if !shouldSuppressTTYQueries([]string{"bv", "--version"}, false, false) {
		t.Fatal("expected --version to suppress TTY queries")
	}
}

func TestShouldSuppressTTYQueries_TUIInvocation(t *testing.T) {
	// Common TUI entry: no args, or args that still launch the TUI.
	if shouldSuppressTTYQueries([]string{"bv"}, false, false) {
		t.Fatal("did not expect plain TUI invocation to suppress TTY queries")
	}
	if shouldSuppressTTYQueries([]string{"bv", "--recipe", "triage"}, false, false) {
		t.Fatal("did not expect --recipe triage (TUI) to suppress TTY queries")
	}
}
