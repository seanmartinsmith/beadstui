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
