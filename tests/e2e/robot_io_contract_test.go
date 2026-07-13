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
		// Subcommands with full RobotEnvelope coverage (bt-sdg2k, bt-govlj).
		{name: "triage", args: []string{"robot", "triage"}},
		{name: "next", args: []string{"robot", "next"}},
		{name: "plan", args: []string{"robot", "plan"}},
		{name: "insights", args: []string{"robot", "insights"}},
		{name: "priority", args: []string{"robot", "priority"}},
		{name: "suggest", args: []string{"robot", "suggest"}},
		{name: "alerts", args: []string{"robot", "alerts"}},
		{name: "list", args: []string{"robot", "list", "--limit=2"}},
		{name: "bql", args: []string{"robot", "bql", "--query=status = open"}},
		{name: "metrics", args: []string{"robot", "metrics"}},
		{name: "labels-health", args: []string{"robot", "labels", "health"}},
		{name: "labels-flow", args: []string{"robot", "labels", "flow"}},
		{name: "labels-attention", args: []string{"robot", "labels", "attention"}},

		// Subcommands that require state not present in a bare fixture (no
		// baseline saved, no git history of .beads/beads.jsonl) are excluded
		// from the sweep — they fail on the data precondition, not on the
		// I/O contract. They have their own dedicated tests where state
		// fixtures are set up.
		// - drift: requires a baseline saved via 'bt baseline save'
		// - history/correlation/* / files: require git history of beads.jsonl
		// - sprint/burndown/forecast/capacity: require sprint definitions
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

// TestRobotIOContract_StructuredErrorEnvelope exercises the structured-error
// negative path added by bt-s5zgk.3: a robot-mode refusal must still satisfy
// the base contract (non-zero exit, empty stdout, "Error:"-prefixed stderr)
// while additionally carrying a machine-parseable JSON envelope - code,
// message, supported_alternative, doc - on that same stderr line. The
// --as-of robot refusal (bt-mjsr9) is the first case; this fixture uses the
// JSONL-fallback source (no Dolt server, no embedded config), which takes
// the general "robot mode" branch of checkAsOfRobotSupport. The
// embedded-specific branch (AS_OF_NOT_SUPPORTED_EMBEDDED) has its own
// bd-spawning acceptance test gated behind BT_EMBEDDED_INTEGRATION=1
// (cmd/bt/robot_asof_embedded_test.go), since it requires a real embedded
// bd project rather than a bare JSONL fixture.
func TestRobotIOContract_StructuredErrorEnvelope(t *testing.T) {
	bt := buildBtBinary(t)
	env := t.TempDir()
	writeBeads(t, env, `{"id":"bt-A","title":"X","status":"open","priority":1,"issue_type":"task"}`)

	stdout, stderr, exit := runRobotCapture(t, bt, env, "robot", "triage", "--as-of", "HEAD~1")

	if exit == 0 {
		t.Fatalf("--as-of robot refusal returned exit=0 (want non-zero)\nstdout: %s\nstderr: %s", string(stdout), string(stderr))
	}
	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Fatalf("--as-of robot refusal leaked to stdout (must be empty on the error path): %q", string(stdout))
	}

	line := strings.TrimSpace(string(stderr))
	const prefix = "Error: "
	if !strings.HasPrefix(line, prefix) {
		t.Fatalf("stderr does not start with %q: %q", prefix, line)
	}

	var envelope struct {
		Code                 string `json:"code"`
		Message              string `json:"message"`
		SupportedAlternative string `json:"supported_alternative"`
		Doc                  string `json:"doc"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, prefix)), &envelope); err != nil {
		t.Fatalf("stderr error line is not valid JSON after stripping %q: %v\nline: %s", prefix, err, line)
	}
	if envelope.Code != "AS_OF_NOT_SUPPORTED_ROBOT" {
		t.Errorf("envelope.code = %q, want AS_OF_NOT_SUPPORTED_ROBOT", envelope.Code)
	}
	if envelope.Message == "" {
		t.Errorf("envelope.message is empty, want the human-readable refusal text")
	}
	if envelope.SupportedAlternative == "" {
		t.Errorf("envelope.supported_alternative is empty, want a concrete alternative (e.g. the interactive TUI)")
	}
	if envelope.Doc == "" {
		t.Errorf("envelope.doc is empty, want a bead/doc pointer")
	}
}
