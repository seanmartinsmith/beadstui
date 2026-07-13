package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
)

// TestCheckAsOfRobotSupport is a pure table test of the robot --as-of guard.
// Robot commands never wire a historical loader, so --as-of must be refused for
// EVERY source type rather than serving current data under a historical stamp
// (bt-fyzll for embedded, generalized to server/JSONL/global by bt-mjsr9).
// Embedded sources get the specific "embedded ... no server" message; every
// other source gets the general "robot mode" refusal. bt-s5zgk.3 additionally
// requires every refusal to carry a *RobotError with a machine-parseable
// code and supported_alternative.
func TestCheckAsOfRobotSupport(t *testing.T) {
	tests := []struct {
		name        string
		asOf        string
		source      *datasource.DataSource
		wantErr     bool
		wantErrHint string // substring the error must explain
		wantCode    string // RobotError.Code, only checked when wantErr
	}{
		{
			name:   "no as-of requested, embedded source",
			asOf:   "",
			source: &datasource.DataSource{Type: datasource.SourceTypeEmbeddedDolt},
		},
		{
			name:   "no as-of requested, no source",
			asOf:   "",
			source: nil,
		},
		{
			name:        "as-of against embedded source: embedded-specific refusal",
			asOf:        "HEAD~1",
			source:      &datasource.DataSource{Type: datasource.SourceTypeEmbeddedDolt},
			wantErr:     true,
			wantErrHint: "embedded",
			wantCode:    "AS_OF_NOT_SUPPORTED_EMBEDDED",
		},
		{
			name:        "as-of against server-mode source: general robot refusal",
			asOf:        "HEAD~1",
			source:      &datasource.DataSource{Type: datasource.SourceTypeDolt},
			wantErr:     true,
			wantErrHint: "robot mode",
			wantCode:    "AS_OF_NOT_SUPPORTED_ROBOT",
		},
		{
			name:        "as-of against global source: general robot refusal",
			asOf:        "HEAD~1",
			source:      &datasource.DataSource{Type: datasource.SourceTypeDoltGlobal},
			wantErr:     true,
			wantErrHint: "robot mode",
			wantCode:    "AS_OF_NOT_SUPPORTED_ROBOT",
		},
		{
			name:        "as-of against jsonl fallback: general robot refusal",
			asOf:        "HEAD~1",
			source:      &datasource.DataSource{Type: datasource.SourceTypeJSONLFallback},
			wantErr:     true,
			wantErrHint: "robot mode",
			wantCode:    "AS_OF_NOT_SUPPORTED_ROBOT",
		},
		{
			name:        "as-of with no resolved source: general robot refusal, no panic",
			asOf:        "HEAD~1",
			source:      nil,
			wantErr:     true,
			wantErrHint: "robot mode",
			wantCode:    "AS_OF_NOT_SUPPORTED_ROBOT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkAsOfRobotSupport(tt.asOf, tt.source)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkAsOfRobotSupport(%q, %+v) error = %v, wantErr %v", tt.asOf, tt.source, err, tt.wantErr)
			}
			if err == nil {
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrHint) {
				t.Errorf("error should explain %q, got: %v", tt.wantErrHint, err)
			}

			var re *RobotError
			if !errors.As(err, &re) {
				t.Fatalf("checkAsOfRobotSupport(%q, %+v) error is not a *RobotError: %v (%T)", tt.asOf, tt.source, err, err)
			}
			if re.Code != tt.wantCode {
				t.Errorf("RobotError.Code = %q, want %q", re.Code, tt.wantCode)
			}
			if re.Message != err.Error() {
				t.Errorf("RobotError.Message = %q, want it to equal err.Error() %q", re.Message, err.Error())
			}
			if re.SupportedAlternative == "" {
				t.Errorf("RobotError.SupportedAlternative is empty, want a concrete alternative")
			}
			if re.Doc == "" {
				t.Errorf("RobotError.Doc is empty, want a bead/doc pointer")
			}
		})
	}
}

// TestRobotAsOfRefusesOnEmbeddedProject is the end-to-end acceptance test for
// bt-fyzll: `bt robot <subcmd> --as-of <ref>` against a REAL embedded-mode
// project must exit non-zero with an explanatory stderr message, and must
// NEVER print a JSON envelope (which would mean current data slipped through
// stamped with a historical as_of ref). Spawns the real `bd` CLI, so it is
// gated the same way as internal/datasource's embedded integration tests:
//
//	BT_EMBEDDED_INTEGRATION=1 go test ./cmd/bt/ -run AsOfRefusesOnEmbedded -v
func TestRobotAsOfRefusesOnEmbeddedProject(t *testing.T) {
	if os.Getenv("BT_EMBEDDED_INTEGRATION") != "1" {
		t.Skip("set BT_EMBEDDED_INTEGRATION=1 to run bd-spawning embedded integration tests")
	}
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("bd CLI not on PATH")
	}

	root := t.TempDir()
	runBd := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(bdPath, args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "BD_NON_INTERACTIVE=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("bd %v failed: %v\noutput: %s", args, err, out)
		}
		return string(out)
	}
	runBd("init")

	beadsDir := filepath.Join(root, ".beads")
	if _, ok := datasource.ReadEmbeddedConfig(beadsDir); !ok {
		t.Skipf("bd init did not produce an embedded project in %s; skipping", root)
	}
	runBd("create", "Alpha", "-t", "task", "-p", "1")

	exe := buildTestBinary(t)
	cmd := exec.Command(exe, "robot", "triage", "--as-of", "HEAD~1")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	if runErr == nil {
		t.Fatalf("expected non-zero exit for --as-of on an embedded project, got success\nstdout: %s", stdout.String())
	}

	// The whole point of bt-fyzll: never let a JSON envelope out the door
	// stamped with an as_of ref while carrying current-state data. Assert
	// stdout is empty (robot I/O contract: stdout is structured data only,
	// nothing on the error path) rather than merely "not parseable", so a
	// regression that emits a partial/valid envelope is still caught.
	if stdout.String() != "" {
		t.Errorf("expected empty stdout on refusal, got: %s", stdout.String())
		var payload map[string]any
		if json.Unmarshal([]byte(stdout.String()), &payload) == nil {
			t.Errorf("stdout parsed as JSON despite the refusal - as_of=%v data_hash=%v (silent wrong answer)", payload["as_of"], payload["data_hash"])
		}
	}
	if !strings.Contains(stderr.String(), "embedded") {
		t.Errorf("stderr should explain the embedded-mode restriction, got: %s", stderr.String())
	}

	// bt-s5zgk.3: the refusal must also carry a machine-parseable structured
	// envelope - "Error: " followed by a single-line JSON object with code,
	// message, supported_alternative, and doc.
	line := strings.TrimSpace(stderr.String())
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
	if envelope.Code != "AS_OF_NOT_SUPPORTED_EMBEDDED" {
		t.Errorf("envelope.code = %q, want AS_OF_NOT_SUPPORTED_EMBEDDED", envelope.Code)
	}
	if envelope.SupportedAlternative == "" {
		t.Errorf("envelope.supported_alternative is empty")
	}
	if envelope.Doc == "" {
		t.Errorf("envelope.doc is empty")
	}
}
