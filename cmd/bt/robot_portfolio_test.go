package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/model"
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

// TestGroupIssuesByProject_ExcludesAtlasNamespace guards bt-z1pzj: under
// --global, issues whose SourceRepo is the beads_global cross-cutting
// namespace (either raw-name spelling) must be dropped entirely rather than
// grouped into a "beads_global" (or aliased) portfolio record - it is a
// namespace, not a project, so its health/velocity numbers would be bogus.
// Ordinary projects, including one that merely contains "global" as a
// substring, must still appear.
func TestGroupIssuesByProject_ExcludesAtlasNamespace(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-1", SourceRepo: "beadstui"},
		{ID: "global-1", SourceRepo: "beads_global"},
		{ID: "global-2", SourceRepo: "BEADS_GLOBAL"}, // mixed-case, still excluded
		{ID: "unknown-1", SourceRepo: ""},
		{ID: "gz-1", SourceRepo: "globalize"}, // substring collision, must NOT be excluded
	}

	groups := groupIssuesByProject(issues, true, "")

	if _, ok := groups["beads_global"]; ok {
		t.Errorf("groups contains excluded beads_global key: %+v", groups)
	}
	if _, ok := groups["BEADS_GLOBAL"]; ok {
		t.Errorf("groups contains excluded BEADS_GLOBAL key: %+v", groups)
	}
	if got := groups["beadstui"]; len(got) != 1 {
		t.Errorf("groups[beadstui] = %d issues, want 1", len(got))
	}
	if got := groups["unknown"]; len(got) != 1 {
		t.Errorf("groups[unknown] = %d issues, want 1", len(got))
	}
	if got := groups["globalize"]; len(got) != 1 {
		t.Errorf("groups[globalize] = %d issues, want 1 (substring collision must not exclude)", len(got))
	}

	total := 0
	for _, v := range groups {
		total += len(v)
	}
	if want := len(issues) - 2; total != want {
		t.Errorf("total grouped issues = %d, want %d (2 beads_global issues excluded)", total, want)
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
