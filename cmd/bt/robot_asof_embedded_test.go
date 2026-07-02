package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
)

// TestCheckAsOfEmbeddedSupport is a pure table test of the bt-fyzll guard:
// --as-of must be refused for embedded-mode Dolt sources and left alone for
// everything else (server, global, JSONL fallback, or no source resolved
// yet).
func TestCheckAsOfEmbeddedSupport(t *testing.T) {
	tests := []struct {
		name    string
		asOf    string
		source  *datasource.DataSource
		wantErr bool
	}{
		{
			name:   "no as-of requested, embedded source",
			asOf:   "",
			source: &datasource.DataSource{Type: datasource.SourceTypeEmbeddedDolt},
		},
		{
			name:    "as-of requested against embedded source",
			asOf:    "HEAD~1",
			source:  &datasource.DataSource{Type: datasource.SourceTypeEmbeddedDolt},
			wantErr: true,
		},
		{
			name:   "as-of requested against server-mode source is unaffected",
			asOf:   "HEAD~1",
			source: &datasource.DataSource{Type: datasource.SourceTypeDolt},
		},
		{
			name:   "as-of requested against global source is unaffected",
			asOf:   "HEAD~1",
			source: &datasource.DataSource{Type: datasource.SourceTypeDoltGlobal},
		},
		{
			name:   "as-of requested against jsonl fallback is unaffected",
			asOf:   "HEAD~1",
			source: &datasource.DataSource{Type: datasource.SourceTypeJSONLFallback},
		},
		{
			name: "as-of requested with no resolved source",
			asOf: "HEAD~1",
			// source intentionally nil: must not panic and must not refuse.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkAsOfEmbeddedSupport(tt.asOf, tt.source)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkAsOfEmbeddedSupport(%q, %+v) error = %v, wantErr %v", tt.asOf, tt.source, err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "embedded") {
				t.Errorf("error should explain the embedded-mode restriction, got: %v", err)
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
}
