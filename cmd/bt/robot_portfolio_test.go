package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

// runPortfolioFixture invokes the built bt binary in the list fixture project.
// The list fixture provides a realistic mix of open/closed/blocked beads
// that exercises every portfolio field.
func runPortfolioFixture(t *testing.T, args ...string) []byte {
	t.Helper()
	dir := setupListFixture(t)
	exe := buildTestBinary(t)
	cmd := exec.Command(exe, append([]string{"robot", "portfolio"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bt robot portfolio %v failed: %v\nout=%s", args, err, string(out))
	}
	return out
}

// TestRobotPortfolio_BasicEnvelope — verifies envelope shape, required
// per-record fields, and that `projects` is a non-empty array.
func TestRobotPortfolio_BasicEnvelope(t *testing.T) {
	out := runPortfolioFixture(t)

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v\nraw=%s", err, out)
	}
	for _, key := range []string{"generated_at", "data_hash", "version", "schema", "projects"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("envelope missing %q", key)
		}
	}
	projects, ok := payload["projects"].([]any)
	if !ok {
		t.Fatalf("projects is not an array: %T", payload["projects"])
	}
	if len(projects) == 0 {
		t.Fatalf("expected at least one project record; got 0")
	}
	required := []string{"project", "counts", "priority", "velocity", "health_score"}
	for i, raw := range projects {
		rec, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("projects[%d] not an object: %T", i, raw)
		}
		for _, key := range required {
			if _, ok := rec[key]; !ok {
				t.Errorf("projects[%d] missing %q", i, key)
			}
		}
	}
}

// TestRobotPortfolio_SchemaIsPortfolioV1 — envelope.schema always
// "portfolio.v1" regardless of --shape. Portfolio is compact-by-construction
// so there is no full-mode alternate.
func TestRobotPortfolio_SchemaIsPortfolioV1(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"--compact"},
		{"--full"},
		{"--shape=compact"},
		{"--shape=full"},
	} {
		name := "default"
		if len(args) > 0 {
			name = args[0]
		}
		t.Run(name, func(t *testing.T) {
			out := runPortfolioFixture(t, args...)
			var payload map[string]any
			if err := json.Unmarshal(out, &payload); err != nil {
				t.Fatalf("parse: %v", err)
			}
			schema, _ := payload["schema"].(string)
			if schema != "portfolio.v1" {
				t.Errorf("schema = %q, want portfolio.v1 (args=%v)", schema, args)
			}
		})
	}
}

// TestRobotPortfolio_ShapeFlagNoop — --shape=compact and --shape=full produce
// identical wire bytes. Any drift means an accidental projection was
// introduced and the --shape no-op contract broke.
//
// `generated_at` is stripped before comparison because it's a wall-clock
// timestamp set on each invocation.
func TestRobotPortfolio_ShapeFlagNoop(t *testing.T) {
	compact := runPortfolioFixture(t, "--shape=compact")
	full := runPortfolioFixture(t, "--shape=full")

	compact = stripGeneratedAt(t, compact)
	full = stripGeneratedAt(t, full)

	if !bytes.Equal(compact, full) {
		t.Errorf("--shape=compact and --shape=full bytes differ\n--- compact ---\n%s\n--- full ---\n%s",
			compact, full)
	}
}

// TestRobotPortfolio_SingleProjectMode — without --global, exactly one
// record is returned so callers can trust len(projects) == 1.
func TestRobotPortfolio_SingleProjectMode(t *testing.T) {
	out := runPortfolioFixture(t)
	var payload struct {
		Projects []map[string]any `json:"projects"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(payload.Projects) != 1 {
		t.Errorf("len(projects) = %d, want 1", len(payload.Projects))
	}
}

// TestRobotPortfolio_ProjectsSortedByName — agents depend on deterministic
// ordering for scanning and diffing across runs.
func TestRobotPortfolio_ProjectsSortedByName(t *testing.T) {
	out := runPortfolioFixture(t)
	var payload struct {
		Projects []struct {
			Project string `json:"project"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for i := 1; i < len(payload.Projects); i++ {
		if payload.Projects[i-1].Project > payload.Projects[i].Project {
			t.Errorf("projects not sorted: %q > %q at index %d",
				payload.Projects[i-1].Project, payload.Projects[i].Project, i)
		}
	}
}

// stripGeneratedAt rewrites the `generated_at` value to an empty string so
// two invocations can be compared byte-for-byte.
func stripGeneratedAt(t *testing.T, raw []byte) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	m["generated_at"] = ""
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("remarshal: %v", err)
	}
	return out
}
