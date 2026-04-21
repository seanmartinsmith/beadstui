package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/view"
)

// refsFixtureBeads is a multi-prefix issue set exercising the refs projection
// end-to-end: one cross-project live ref, one broken prose ref, one stale,
// one orphaned_child, plus an intra-project reference that MUST NOT surface.
const refsFixtureBeads = `{"id":"bt-src","title":"source bead","description":"see bd-live for context and bt-local for intra","status":"open","priority":1,"issue_type":"task","source_repo":"bt","created_at":"2026-04-15T10:00:00Z","updated_at":"2026-04-15T10:00:00Z"}
{"id":"bt-local","title":"intra-project target","status":"open","priority":2,"issue_type":"task","source_repo":"bt","created_at":"2026-04-15T10:00:00Z","updated_at":"2026-04-15T10:00:00Z"}
{"id":"bd-live","title":"live cross-project","status":"open","priority":1,"issue_type":"task","source_repo":"bd","created_at":"2026-04-14T10:00:00Z","updated_at":"2026-04-14T10:00:00Z"}
{"id":"bt-broken","title":"references missing","description":"blocked on cass-missing to ship","status":"open","priority":2,"issue_type":"task","source_repo":"bt","created_at":"2026-04-15T10:00:00Z","updated_at":"2026-04-15T10:00:00Z"}
{"id":"bt-stale","title":"references closed","notes":"decision recorded in bd-closed","status":"open","priority":1,"issue_type":"task","source_repo":"bt","created_at":"2026-04-15T10:00:00Z","updated_at":"2026-04-15T10:00:00Z"}
{"id":"bd-closed","title":"closed upstream","status":"closed","priority":1,"issue_type":"task","source_repo":"bd","created_at":"2026-04-10T10:00:00Z","updated_at":"2026-04-18T10:00:00Z","closed_at":"2026-04-18T10:00:00Z"}
{"id":"bt-orphan-src","title":"references orphan","description":"paired with bd-orphan","status":"open","priority":2,"issue_type":"task","source_repo":"bt","created_at":"2026-04-17T10:00:00Z","updated_at":"2026-04-17T10:00:00Z"}
{"id":"bd-orphan","title":"orphaned child","status":"open","priority":2,"issue_type":"task","source_repo":"bd","dependencies":[{"issue_id":"bd-orphan","depends_on_id":"bd-parent","type":"parent-child","created_at":"2026-04-15T10:00:00Z"}],"created_at":"2026-04-15T10:00:00Z","updated_at":"2026-04-15T10:00:00Z"}
{"id":"bd-parent","title":"closed parent","status":"closed","priority":0,"issue_type":"epic","source_repo":"bd","created_at":"2026-04-10T10:00:00Z","updated_at":"2026-04-20T10:00:00Z","closed_at":"2026-04-20T10:00:00Z"}
`

// setupRefsFixture writes a multi-prefix beads.jsonl under t.TempDir() and
// returns the project directory. Mirrors setupPairsFixture because refs
// detection also needs cross-prefix data.
func setupRefsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(refsFixtureBeads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}
	return dir
}

// refsFixtureIssues is the Go-native equivalent of refsFixtureBeads for
// in-process tests that exercise refsOutput() without going through the
// binary. Any change to the JSONL fixture must be mirrored here.
func refsFixtureIssues() []model.Issue {
	mk := func(id, title, desc, notes string, status model.Status, priority int, source, createdAt string) model.Issue {
		ts, _ := time.Parse(time.RFC3339, createdAt)
		return model.Issue{
			ID: id, Title: title, Description: desc, Notes: notes,
			Status: status, Priority: priority, SourceRepo: source,
			CreatedAt: ts, UpdatedAt: ts,
		}
	}
	orphan := mk("bd-orphan", "orphaned child", "", "", model.StatusOpen, 2, "bd", "2026-04-15T10:00:00Z")
	orphan.Dependencies = []*model.Dependency{
		{IssueID: "bd-orphan", DependsOnID: "bd-parent", Type: model.DepParentChild},
	}
	return []model.Issue{
		mk("bt-src", "source bead", "see bd-live for context and bt-local for intra", "", model.StatusOpen, 1, "bt", "2026-04-15T10:00:00Z"),
		mk("bt-local", "intra-project target", "", "", model.StatusOpen, 2, "bt", "2026-04-15T10:00:00Z"),
		mk("bd-live", "live cross-project", "", "", model.StatusOpen, 1, "bd", "2026-04-14T10:00:00Z"),
		mk("bt-broken", "references missing", "blocked on cass-missing to ship", "", model.StatusOpen, 2, "bt", "2026-04-15T10:00:00Z"),
		mk("bt-stale", "references closed", "", "decision recorded in bd-closed", model.StatusOpen, 1, "bt", "2026-04-15T10:00:00Z"),
		mk("bd-closed", "closed upstream", "", "", model.StatusClosed, 1, "bd", "2026-04-10T10:00:00Z"),
		mk("bt-orphan-src", "references orphan", "paired with bd-orphan", "", model.StatusOpen, 2, "bt", "2026-04-17T10:00:00Z"),
		orphan,
		mk("bd-parent", "closed parent", "", "", model.StatusClosed, 0, "bd", "2026-04-10T10:00:00Z"),
	}
}

// TestRobotRefs_RequiresGlobal — without --global, the subcommand exits
// non-zero with the expected error. BT_TEST_MODE=1 is safe here because the
// error fires before any data-loading branch.
func TestRobotRefs_RequiresGlobal(t *testing.T) {
	dir := setupRefsFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "refs")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit without --global; got success\nout=%s", out)
	}
	wantMsg := "bt robot refs requires --global"
	if !strings.Contains(string(out), wantMsg) {
		t.Errorf("stderr missing %q; got:\n%s", wantMsg, out)
	}
}

// TestRefsOutput_BasicEnvelope — required envelope fields and refs array
// shape. Uses refsOutput() directly because --global binary tests can't run
// through Dolt discovery in test mode.
func TestRefsOutput_BasicEnvelope(t *testing.T) {
	raw, err := json.Marshal(refsOutput(refsFixtureIssues(), "test-hash"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse: %v\nraw=%s", err, raw)
	}
	for _, key := range []string{"generated_at", "data_hash", "version", "schema", "refs"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("envelope missing %q", key)
		}
	}
	refs, ok := payload["refs"].([]any)
	if !ok {
		t.Fatalf("refs is not an array: %T", payload["refs"])
	}
	if len(refs) == 0 {
		t.Fatalf("fixture should produce >0 refs; got empty")
	}
	required := []string{"source", "target", "location", "flags"}
	for i, raw := range refs {
		rec, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("refs[%d] not an object: %T", i, raw)
		}
		for _, key := range required {
			if _, ok := rec[key]; !ok {
				t.Errorf("refs[%d] missing %q", i, key)
			}
		}
	}
}

// TestRefsOutput_SchemaIsRefV1 — envelope.schema is always "ref.v1".
func TestRefsOutput_SchemaIsRefV1(t *testing.T) {
	raw, err := json.Marshal(refsOutput(refsFixtureIssues(), "test-hash"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	schema, _ := payload["schema"].(string)
	if schema != "ref.v1" {
		t.Errorf("schema = %q, want ref.v1", schema)
	}
}

// TestRefsOutput_CrossProjectOnly — fixture includes an intra-project ref
// ("bt-local" inside bt-src's description). It MUST NOT surface in v1
// because refs is cross-project only.
func TestRefsOutput_CrossProjectOnly(t *testing.T) {
	raw, _ := json.Marshal(refsOutput(refsFixtureIssues(), "test-hash"))
	var payload struct {
		Refs []view.RefRecord `json:"refs"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, r := range payload.Refs {
		if r.Target == "bt-local" {
			t.Errorf("intra-project ref leaked into output: %+v", r)
		}
	}
}

// TestRefsOutput_FlagOrder — every record's flags respect the fixed output
// order: broken, stale, orphaned_child, cross_project.
func TestRefsOutput_FlagOrder(t *testing.T) {
	raw, _ := json.Marshal(refsOutput(refsFixtureIssues(), "test-hash"))
	var payload struct {
		Refs []view.RefRecord `json:"refs"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	rank := map[string]int{"broken": 0, "stale": 1, "orphaned_child": 2, "cross_project": 3}
	for _, r := range payload.Refs {
		if len(r.Flags) == 0 {
			t.Errorf("empty flags on %+v", r)
			continue
		}
		if r.Flags[len(r.Flags)-1] != "cross_project" {
			t.Errorf("cross_project must trail; got %v on %+v", r.Flags, r)
		}
		for i := 1; i < len(r.Flags); i++ {
			if rank[r.Flags[i-1]] >= rank[r.Flags[i]] {
				t.Errorf("flag order violation on %+v: %v", r, r.Flags)
			}
		}
	}
}

// TestRefsOutput_EmptyReturnsArray — zero refs = `[]` wire output, never
// `null`. Agents scan length without a null check.
func TestRefsOutput_EmptyReturnsArray(t *testing.T) {
	raw, err := json.Marshal(refsOutput(nil, "test-hash"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"refs":[]`)) {
		t.Errorf("expected empty array for refs; got %s", raw)
	}
	// Sanity: single-project set with no cross-prefix also emits [].
	onlyBt := []model.Issue{
		{ID: "bt-a", Title: "x", Status: model.StatusOpen, Description: "see bt-b"},
		{ID: "bt-b", Title: "y", Status: model.StatusOpen},
	}
	raw, _ = json.Marshal(refsOutput(onlyBt, "test-hash"))
	if !bytes.Contains(raw, []byte(`"refs":[]`)) {
		t.Errorf("single-project set should emit empty array; got %s", raw)
	}
}

// TestRefsOutput_SortedOutput — deterministic ordering by
// (Source, Target, Location).
func TestRefsOutput_SortedOutput(t *testing.T) {
	raw, _ := json.Marshal(refsOutput(refsFixtureIssues(), "test-hash"))
	var payload struct {
		Refs []view.RefRecord `json:"refs"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for i := 1; i < len(payload.Refs); i++ {
		prev, cur := payload.Refs[i-1], payload.Refs[i]
		if prev.Source > cur.Source {
			t.Errorf("source not sorted at %d: %q > %q", i, prev.Source, cur.Source)
		}
		if prev.Source == cur.Source && prev.Target > cur.Target {
			t.Errorf("target not sorted at %d: %q > %q", i, prev.Target, cur.Target)
		}
		if prev.Source == cur.Source && prev.Target == cur.Target && prev.Location > cur.Location {
			t.Errorf("location not sorted at %d", i)
		}
	}
}

// TestRefsValidate covers the pure flag-validation helper for
// `bt robot refs`. Binary tests (below) confirm the validator hook
// fires before robotPreRun so BT_TEST_MODE=1 doesn't mask errors.
func TestRefsValidate(t *testing.T) {
	cases := []struct {
		name       string
		flagSchema string
		flagSigils string
		envSchema  string
		envSigils  string
		wantSchema string
		wantSigils string
		wantErr    string
	}{
		{"defaults resolve cleanly", "", "", "", "", robotSchemaV1, robotSigilVerb, ""},
		{"v1 + no sigils ok", "v1", "", "", "", robotSchemaV1, robotSigilVerb, ""},
		{"v2 + strict ok", "v2", "strict", "", "", robotSchemaV2, robotSigilStrict, ""},
		{"v2 env + verb sigils flag ok", "", "verb", "v2", "", robotSchemaV2, robotSigilVerb, ""},
		{"v1 + explicit sigils errors", "v1", "strict", "", "", "", "", "--sigils requires --schema=v2"},
		{"v1 default + env sigils errors", "", "", "", "strict", "", "", "--sigils requires --schema=v2"},
		{"invalid sigils errors", "v2", "lax", "", "", "", "", `invalid --sigils "lax"`},
		{"invalid schema errors before sigils check", "bogus", "strict", "", "", "", "", `invalid --schema "bogus"`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BT_OUTPUT_SCHEMA", tc.envSchema)
			t.Setenv("BT_SIGIL_MODE", tc.envSigils)
			schema, sigils, err := refsValidate(tc.flagSchema, tc.flagSigils)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("want error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want contains %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if schema != tc.wantSchema {
				t.Errorf("schema = %q, want %q", schema, tc.wantSchema)
			}
			if sigils != tc.wantSigils {
				t.Errorf("sigils = %q, want %q", sigils, tc.wantSigils)
			}
		})
	}
}

// TestRobotRefs_SchemaInvalid — an unknown --schema value exits 1
// with stderr listing valid values.
func TestRobotRefs_SchemaInvalid(t *testing.T) {
	dir := setupRefsFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "refs", "--global", "--schema=bogus")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for --schema=bogus; got success\nout=%s", out)
	}
	if !strings.Contains(string(out), "expected v1|v2") {
		t.Errorf("stderr missing valid-values hint; got:\n%s", out)
	}
}

// TestRobotRefs_SigilsInvalid — an unknown --sigils value exits 1
// with stderr listing valid values, even before the schema conflict
// check runs.
func TestRobotRefs_SigilsInvalid(t *testing.T) {
	dir := setupRefsFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "refs", "--global", "--sigils=lax")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for --sigils=lax; got success\nout=%s", out)
	}
	if !strings.Contains(string(out), "expected strict|verb|permissive") {
		t.Errorf("stderr missing valid-values hint; got:\n%s", out)
	}
}

// TestRobotRefs_SigilsRequiresV2 — explicit --sigils with --schema=v1
// (the Phase 1 default) errors with the resolution message instructing
// the user to set --schema=v2 or omit --sigils.
func TestRobotRefs_SigilsRequiresV2(t *testing.T) {
	dir := setupRefsFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "refs", "--global", "--schema=v1", "--sigils=strict")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for --schema=v1 --sigils=strict; got success\nout=%s", out)
	}
	if !strings.Contains(string(out), "--sigils requires --schema=v2") {
		t.Errorf("stderr missing resolution message; got:\n%s", out)
	}
	if !strings.Contains(string(out), "Run with --schema=v2") {
		t.Errorf("stderr missing remediation hint; got:\n%s", out)
	}
}

// TestRobotRefs_EnvSigilsRequiresV2 — BT_SIGIL_MODE set in env paired
// with --schema=v1 triggers the same conflict error. The default
// sigils mode (no env, no flag) does NOT trigger.
func TestRobotRefs_EnvSigilsRequiresV2(t *testing.T) {
	dir := setupRefsFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "refs", "--global", "--schema=v1")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1", "BT_SIGIL_MODE=strict")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit with BT_SIGIL_MODE + --schema=v1; got success\nout=%s", out)
	}
	if !strings.Contains(string(out), "--sigils requires --schema=v2") {
		t.Errorf("stderr missing conflict message; got:\n%s", out)
	}
}
