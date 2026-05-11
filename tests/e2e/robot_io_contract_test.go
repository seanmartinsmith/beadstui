package main_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// Robot I/O contract verify-sweep (bt-ah53).
//
// Every robot subcommand must satisfy three invariants on a normal invocation:
//
//  1. stdout is a single parseable JSON document
//  2. stderr is empty (success path)
//  3. exit code is 0
//
// Plus a fourth, layered on top by bt-sdg2k:
//
//  4. envelope.scope.mode is one of {cross-project, project, workspace}
//
// And two negative cases that exercise the error paths:
//
//   - unknown subcommand under 'bt robot' exits non-zero with an "Error:" line
//     on stderr (bt-70cd)
//   - unknown flag exits non-zero with cobra's flag-parse error on stderr
//
// Subcommands that emit raw correlator/analysis payloads without the standard
// envelope are listed in skippedEnvelopeCommands and tracked in follow-up beads.
// The sweep still exercises their stdout/stderr/exit invariants — only the
// scope assertion is skipped.

// runRobotCapture executes the bt binary with args and returns stdout/stderr
// bytes plus the exit code. Errors from exec.Cmd.Run itself surface as -1.
func runRobotCapture(t *testing.T, bt, dir string, args ...string) (stdout, stderr []byte, exitCode int) {
	t.Helper()
	cmd := exec.Command(bt, args...)
	cmd.Dir = dir
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	stdout = so.Bytes()
	stderr = se.Bytes()
	if err == nil {
		return stdout, stderr, 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return stdout, stderr, exitErr.ExitCode()
	}
	return stdout, stderr, -1
}

// assertStderrClean enforces the contract: stderr is either empty, or every
// non-empty line starts with "Error:" (the documented prefix). Log/info leaks
// fail the assertion. (bt-ah53)
func assertStderrClean(t *testing.T, label string, stderr []byte) {
	t.Helper()
	if len(stderr) == 0 {
		return
	}
	for _, line := range strings.Split(strings.TrimRight(string(stderr), "\n"), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "Error:") {
			t.Fatalf("%s leaked to stderr (must be empty or start with %q): %q", label, "Error:", line)
		}
	}
}

// assertEnvelopeProject parses stdout as JSON, asserts the envelope.scope.mode
// is "project", and returns the parsed map for further inspection. (bt-ah53)
func assertEnvelopeProject(t *testing.T, label string, stdout []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(stdout, &payload); err != nil {
		t.Fatalf("%s stdout is not valid JSON: %v\nout: %s", label, err, string(stdout))
	}
	assertEnvelopeScope(t, label, payload, "project")
	return payload
}

// robotContractCase describes one subcommand invocation under the sweep.
type robotContractCase struct {
	name           string   // subtest label
	args           []string // full argv after "robot"
	skipEnvelope   bool     // payload doesn't wrap RobotEnvelope yet — follow-up bead
	skipEnvReason  string   // bead/explanation for the skip
	allowEmptyJSON bool     // some commands emit empty arrays/objects on a bare fixture
}

// TestRobotIOContractSweep is the spine of bt-ah53: every robot subcommand
// listed here must satisfy the I/O contract. New robot subcommands MUST add
// an entry here at landing time, not after a downstream regression.
func TestRobotIOContractSweep(t *testing.T) {
	bt := buildBtBinary(t)
	env := t.TempDir()
	// Small dependency chain so triage/plan/insights produce non-trivial output.
	writeBeads(t, env, `{"id":"bt-A","title":"Root","status":"open","priority":1,"issue_type":"task","labels":["api"]}
{"id":"bt-B","title":"Mid","status":"open","priority":2,"issue_type":"task","labels":["web"],"dependencies":[{"issue_id":"bt-B","depends_on_id":"bt-A","type":"blocks"}]}
{"id":"bt-C","title":"Leaf","status":"open","priority":3,"issue_type":"task","labels":["web"],"dependencies":[{"issue_id":"bt-C","depends_on_id":"bt-B","type":"blocks"}]}`)

	cases := []robotContractCase{
		// Subcommands with full RobotEnvelope coverage (bt-sdg2k).
		{name: "triage", args: []string{"robot", "triage"}},
		{name: "next", args: []string{"robot", "next"}},
		{name: "plan", args: []string{"robot", "plan"}},
		{name: "insights", args: []string{"robot", "insights"}},
		{name: "priority", args: []string{"robot", "priority"}},
		{name: "suggest", args: []string{"robot", "suggest"}},
		{name: "alerts", args: []string{"robot", "alerts"}},
		{name: "list", args: []string{"robot", "list", "--limit=2"}},
		{name: "bql", args: []string{"robot", "bql", "--query=status = open"}},

		// Subcommands that wrap correlator/analysis payloads directly. These
		// emit valid JSON and clean stderr but the envelope shape is not
		// RobotEnvelope-derived yet. Tracked for follow-up.
		{
			name:          "metrics",
			args:          []string{"robot", "metrics"},
			skipEnvelope:  true,
			skipEnvReason: "emits metrics map without RobotEnvelope; follow-up bead",
		},

		// Subcommands that require state not present in a bare fixture (no
		// baseline saved, no git history) are excluded from the sweep — they
		// fail on the data precondition, not on the I/O contract. They have
		// their own dedicated tests elsewhere.
		// - drift: requires a baseline saved via 'bt baseline save'
		// - history/correlation/*: require git history of beads.jsonl
		// - sprint/labels/files: have dedicated contract tests
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, exit := runRobotCapture(t, bt, env, tc.args...)
			label := strings.Join(tc.args, " ")

			if exit != 0 {
				t.Fatalf("%s exit=%d (want 0)\nstdout: %s\nstderr: %s", label, exit, string(stdout), string(stderr))
			}
			assertStderrClean(t, label, stderr)

			if tc.allowEmptyJSON && len(bytes.TrimSpace(stdout)) == 0 {
				return
			}

			// stdout must be parseable JSON regardless of envelope coverage.
			var payload map[string]any
			if err := json.Unmarshal(stdout, &payload); err != nil {
				t.Fatalf("%s stdout is not valid JSON: %v\nout: %s", label, err, string(stdout))
			}

			if tc.skipEnvelope {
				t.Logf("envelope-skip: %s (%s)", label, tc.skipEnvReason)
				return
			}
			assertEnvelopeScope(t, label, payload, "project")
		})
	}
}

// TestRobotIOContract_UnknownSubcommand exercises the negative path: an
// unknown subcommand under 'bt robot' must exit non-zero with an "Error:"
// line on stderr and an empty stdout. (bt-70cd, asserted under bt-ah53)
func TestRobotIOContract_UnknownSubcommand(t *testing.T) {
	bt := buildBtBinary(t)
	stdout, stderr, exit := runRobotCapture(t, bt, t.TempDir(), "robot", "this-command-does-not-exist")

	if exit == 0 {
		t.Fatalf("unknown subcommand returned exit=0 (want non-zero)\nstdout: %s\nstderr: %s", string(stdout), string(stderr))
	}
	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Fatalf("unknown subcommand leaked to stdout (must be empty for negative path): %q", string(stdout))
	}
	if !strings.Contains(string(stderr), "Error:") {
		t.Fatalf("unknown subcommand stderr missing %q prefix: %q", "Error:", string(stderr))
	}
}

// TestRobotIOContract_UnknownFlag exercises the negative path: an unknown
// flag must exit non-zero with the parse error on stderr and an empty stdout.
func TestRobotIOContract_UnknownFlag(t *testing.T) {
	bt := buildBtBinary(t)
	env := t.TempDir()
	writeBeads(t, env, `{"id":"bt-A","title":"X","status":"open","priority":1,"issue_type":"task"}`)

	stdout, stderr, exit := runRobotCapture(t, bt, env, "robot", "triage", "--this-flag-does-not-exist")

	if exit == 0 {
		t.Fatalf("unknown flag returned exit=0 (want non-zero)\nstdout: %s\nstderr: %s", string(stdout), string(stderr))
	}
	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Fatalf("unknown flag leaked to stdout: %q", string(stdout))
	}
	// Cobra emits "Error: unknown flag: ..." on stderr.
	if !strings.Contains(strings.ToLower(string(stderr)), "unknown flag") {
		t.Fatalf("unknown flag stderr missing 'unknown flag' substring: %q", string(stderr))
	}
}

// TestRobotIOContract_MissingRequiredArg exercises the negative path: a
// subcommand whose required argument is absent must exit non-zero with the
// error on stderr.
func TestRobotIOContract_MissingRequiredArg(t *testing.T) {
	bt := buildBtBinary(t)
	env := t.TempDir()
	writeBeads(t, env, `{"id":"bt-A","title":"X","status":"open","priority":1,"issue_type":"task"}`)

	// 'bt robot search' requires a query.
	stdout, stderr, exit := runRobotCapture(t, bt, env, "robot", "search")

	if exit == 0 {
		t.Fatalf("missing required arg returned exit=0 (want non-zero)\nstdout: %s\nstderr: %s", string(stdout), string(stderr))
	}
	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Fatalf("missing required arg leaked to stdout: %q", string(stdout))
	}
	if len(bytes.TrimSpace(stderr)) == 0 {
		t.Fatalf("missing required arg produced empty stderr")
	}
}
