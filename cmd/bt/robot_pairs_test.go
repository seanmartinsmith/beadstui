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

// pairsFixtureBeads is a multi-prefix issue set exercising the pair
// projection end-to-end: two paired sets (suffixes "abc" and "zsy8"), a
// 3-way pair, a drifted pair, and isolated issues that should NOT surface
// as pairs (same-prefix anomaly, no matching suffix).
//
// The cross-project listing (bt/bd/cass) lives in a single JSONL file on
// purpose: binary contract tests for --global can't go through real Dolt
// discovery (BT_TEST_MODE=1 is mandatory). The pair projection itself only
// cares about IDs and distinct prefixes, which this fixture provides
// directly.
const pairsFixtureBeads = `{"id":"bt-abc","title":"3-way shared work","status":"open","priority":1,"issue_type":"task","source_repo":"bt","created_at":"2026-04-16T10:00:00Z","updated_at":"2026-04-16T10:00:00Z"}
{"id":"bd-abc","title":"3-way shared work","status":"open","priority":1,"issue_type":"task","source_repo":"bd","created_at":"2026-04-15T10:00:00Z","updated_at":"2026-04-15T10:00:00Z"}
{"id":"cass-abc","title":"3-way shared work","status":"open","priority":1,"issue_type":"task","source_repo":"cass","created_at":"2026-04-17T10:00:00Z","updated_at":"2026-04-17T10:00:00Z"}
{"id":"bt-zsy8","title":"Cross-project shared work","status":"open","priority":0,"issue_type":"task","source_repo":"bt","created_at":"2026-04-15T10:00:00Z","updated_at":"2026-04-15T10:00:00Z"}
{"id":"bd-zsy8","title":"Cross-project shared work but different","status":"closed","priority":2,"issue_type":"task","source_repo":"bd","created_at":"2026-04-16T10:00:00Z","updated_at":"2026-04-20T10:00:00Z","closed_at":"2026-04-20T10:00:00Z"}
{"id":"bt-solo","title":"no mirror","status":"open","priority":2,"issue_type":"task","source_repo":"bt","created_at":"2026-04-10T10:00:00Z","updated_at":"2026-04-10T10:00:00Z"}
`

// setupPairsFixture writes a multi-prefix beads.jsonl under t.TempDir() and
// returns the project directory. Kept separate from setupListFixture because
// the list fixture is single-prefix (inappropriate for pair detection) and
// folding cross-prefix data into it would push every list test to think
// about pair setup.
func setupPairsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(pairsFixtureBeads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}
	return dir
}

// pairsFixtureIssues is the Go-native equivalent of pairsFixtureBeads for
// in-process tests that exercise pairsOutput() without going through the
// binary. Keeping both surfaces in sync manually: any change to the JSONL
// fixture must be mirrored here.
func pairsFixtureIssues() []model.Issue {
	return []model.Issue{
		mkPairIssue("bt-abc", "3-way shared work", model.StatusOpen, 1, "bt", "2026-04-16T10:00:00Z"),
		mkPairIssue("bd-abc", "3-way shared work", model.StatusOpen, 1, "bd", "2026-04-15T10:00:00Z"),
		mkPairIssue("cass-abc", "3-way shared work", model.StatusOpen, 1, "cass", "2026-04-17T10:00:00Z"),
		mkPairIssue("bt-zsy8", "Cross-project shared work", model.StatusOpen, 0, "bt", "2026-04-15T10:00:00Z"),
		mkPairIssue("bd-zsy8", "Cross-project shared work but different", model.StatusClosed, 2, "bd", "2026-04-16T10:00:00Z"),
		mkPairIssue("bt-solo", "no mirror", model.StatusOpen, 2, "bt", "2026-04-10T10:00:00Z"),
	}
}

func mkPairIssue(id, title string, status model.Status, priority int, source, createdAt string) model.Issue {
	t, _ := time.Parse(time.RFC3339, createdAt)
	return model.Issue{
		ID: id, Title: title, Status: status, Priority: priority, SourceRepo: source,
		CreatedAt: t, UpdatedAt: t,
	}
}

// TestRobotPairs_RequiresGlobal — without --global, the subcommand exits
// non-zero with a specific error message. BT_TEST_MODE=1 is safe here
// because the error fires before any data-loading branch.
func TestRobotPairs_RequiresGlobal(t *testing.T) {
	dir := setupPairsFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "pairs")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit without --global; got success\nout=%s", out)
	}
	wantMsg := "bt robot pairs requires --global"
	if !strings.Contains(string(out), wantMsg) {
		t.Errorf("stderr missing %q; got:\n%s", wantMsg, out)
	}
}

// TestPairsOutput_BasicEnvelope — required envelope fields and pairs array
// shape. Uses pairsOutput() directly because --global binary tests can't
// run through Dolt discovery in test mode.
func TestPairsOutput_BasicEnvelope(t *testing.T) {
	raw, err := json.Marshal(pairsOutput(pairsFixtureIssues(), "test-hash"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse: %v\nraw=%s", err, raw)
	}
	for _, key := range []string{"generated_at", "data_hash", "version", "schema", "pairs"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("envelope missing %q", key)
		}
	}
	pairs, ok := payload["pairs"].([]any)
	if !ok {
		t.Fatalf("pairs is not an array: %T", payload["pairs"])
	}
	if len(pairs) != 2 {
		t.Errorf("len(pairs) = %d, want 2", len(pairs))
	}
	required := []string{"suffix", "canonical", "mirrors"}
	for i, raw := range pairs {
		rec, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("pairs[%d] not an object: %T", i, raw)
		}
		for _, key := range required {
			if _, ok := rec[key]; !ok {
				t.Errorf("pairs[%d] missing %q", i, key)
			}
		}
	}
}

// TestPairsOutput_SchemaIsPairV1 — envelope.schema is always "pair.v1".
// Pair output is compact-by-construction; no shape switching.
func TestPairsOutput_SchemaIsPairV1(t *testing.T) {
	raw, err := json.Marshal(pairsOutput(pairsFixtureIssues(), "test-hash"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	schema, _ := payload["schema"].(string)
	if schema != "pair.v1" {
		t.Errorf("schema = %q, want pair.v1", schema)
	}
}

// TestPairsOutput_DriftDetection — the fixture's zsy8 pair drifts across
// status, priority, closed_open, and title. Asserts the full fixed order.
func TestPairsOutput_DriftDetection(t *testing.T) {
	raw, _ := json.Marshal(pairsOutput(pairsFixtureIssues(), "test-hash"))
	var payload struct {
		Pairs []view.PairRecord `json:"pairs"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}

	var zsy8 *view.PairRecord
	for i := range payload.Pairs {
		if payload.Pairs[i].Suffix == "zsy8" {
			zsy8 = &payload.Pairs[i]
			break
		}
	}
	if zsy8 == nil {
		t.Fatalf("zsy8 pair missing")
	}
	want := []string{"status", "priority", "closed_open", "title"}
	if !sameStringSlice(zsy8.Drift, want) {
		t.Errorf("zsy8 drift = %v, want %v", zsy8.Drift, want)
	}
}

// TestPairsOutput_EmptyReturnsArray — zero pairs = `[]` wire output, never
// `null`. Agents scan length without a null check.
func TestPairsOutput_EmptyReturnsArray(t *testing.T) {
	raw, err := json.Marshal(pairsOutput(nil, "test-hash"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"pairs":[]`)) {
		t.Errorf("expected empty array for pairs; got %s", raw)
	}
	// Sanity: single-project set with no cross-prefix also emits [].
	onlyBt := []model.Issue{
		mkPairIssue("bt-a", "x", model.StatusOpen, 1, "bt", "2026-04-15T10:00:00Z"),
		mkPairIssue("bt-b", "y", model.StatusOpen, 1, "bt", "2026-04-15T10:00:00Z"),
	}
	raw, _ = json.Marshal(pairsOutput(onlyBt, "test-hash"))
	if !bytes.Contains(raw, []byte(`"pairs":[]`)) {
		t.Errorf("single-project set should emit empty array; got %s", raw)
	}
}

// TestPairsOutput_PairsSortedBySuffix — deterministic ordering across runs.
func TestPairsOutput_PairsSortedBySuffix(t *testing.T) {
	raw, _ := json.Marshal(pairsOutput(pairsFixtureIssues(), "test-hash"))
	var payload struct {
		Pairs []struct {
			Suffix string `json:"suffix"`
		} `json:"pairs"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for i := 1; i < len(payload.Pairs); i++ {
		if payload.Pairs[i-1].Suffix > payload.Pairs[i].Suffix {
			t.Errorf("pairs not sorted: %q > %q at index %d",
				payload.Pairs[i-1].Suffix, payload.Pairs[i].Suffix, i)
		}
	}
}

// TestPairsOutput_CanonicalFirstCreated — the abc 3-way pair's canonical is
// the earliest CreatedAt (bd-abc), mirrors sorted by prefix (bt-abc before
// cass-abc).
func TestPairsOutput_CanonicalFirstCreated(t *testing.T) {
	raw, _ := json.Marshal(pairsOutput(pairsFixtureIssues(), "test-hash"))
	var payload struct {
		Pairs []view.PairRecord `json:"pairs"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	var abc *view.PairRecord
	for i := range payload.Pairs {
		if payload.Pairs[i].Suffix == "abc" {
			abc = &payload.Pairs[i]
			break
		}
	}
	if abc == nil {
		t.Fatalf("abc pair missing")
	}
	if abc.Canonical.ID != "bd-abc" {
		t.Errorf("canonical = %q, want bd-abc", abc.Canonical.ID)
	}
	if len(abc.Mirrors) != 2 {
		t.Fatalf("len(mirrors) = %d, want 2", len(abc.Mirrors))
	}
	if abc.Mirrors[0].ID != "bt-abc" || abc.Mirrors[1].ID != "cass-abc" {
		t.Errorf("mirror order = [%s, %s], want [bt-abc, cass-abc]",
			abc.Mirrors[0].ID, abc.Mirrors[1].ID)
	}
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
