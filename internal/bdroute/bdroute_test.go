// Unit tests for the launch-time write-routing table (bt-scc35 / decision
// bt-2pk38). Two clusters:
//
//  1. Resolve() behavior per mode: workspace mapping hit, unmapped source,
//     global identity mismatch, missing metadata, embedded-mode checkout,
//     beads_global refusal, and single-project passthrough.
//  2. Pollution immunity: the regression test for the bug this bead fixes -
//     a poisoned pkg/projects registry (BT_PROJECTS_REGISTRY_PATH) must have
//     zero effect on Resolve, because the write path never reads it.
package bdroute

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanmartinsmith/beadstui/internal/settings"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/workspace"
)

// isolateBeadsDirEnv guarantees loader.GetBeadsDir's BEADS_DIR override does
// not leak from the developer's shell into these tests - every fixture here
// relies on GetBeadsDir resolving ".beads" directly under the directory it
// is given.
func isolateBeadsDirEnv(t *testing.T) {
	t.Helper()
	t.Setenv(loader.BeadsDirEnvVar, "")
}

// writeMetadata writes a minimal .beads/metadata.json under dir with the
// given dolt_database and dolt_mode.
func writeMetadata(t *testing.T, dir, dbName, doltMode string) {
	t.Helper()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("writeMetadata: mkdir .beads: %v", err)
	}
	meta := ProjectMetadata{DoltDatabase: dbName, DoltMode: doltMode, ProjectID: "test"}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("writeMetadata: marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), data, 0o644); err != nil {
		t.Fatalf("writeMetadata: write metadata.json: %v", err)
	}
}

func TestResolve_SingleProject_Passthrough(t *testing.T) {
	isolateBeadsDirEnv(t)
	tbl := SingleProject(`C:\projects\bt`)

	target, err := tbl.Resolve(model.Issue{ID: "anything-1"})
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if target.Dir != `C:\projects\bt` {
		t.Errorf("target.Dir = %q, want %q", target.Dir, `C:\projects\bt`)
	}
	if target.Global {
		t.Errorf("target.Global = true, want false")
	}

	// A completely different issue ID/prefix must resolve identically - single
	// project mode never branches on issue identity.
	target2, err := tbl.Resolve(model.Issue{ID: "zz-999", SourceRepo: "zz"})
	if err != nil {
		t.Fatalf("Resolve() second call error = %v, want nil", err)
	}
	if target2.Dir != target.Dir {
		t.Errorf("single-project target changed across issues: %q vs %q", target2.Dir, target.Dir)
	}
}

func TestResolve_Workspace_MappingHit(t *testing.T) {
	isolateBeadsDirEnv(t)
	dir := t.TempDir()
	tbl := FromWorkspace([]workspace.LoadResult{
		{Prefix: "bt", AbsPath: dir},
		{Prefix: "bd", AbsPath: t.TempDir()},
	})

	target, err := tbl.Resolve(model.Issue{ID: "bt-42"})
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if target.Dir != dir {
		t.Errorf("target.Dir = %q, want %q", target.Dir, dir)
	}
	// Workspace mode does NOT identity-check metadata.json - no .beads/ was
	// ever created under dir, and Resolve must still succeed (config is
	// authoritative; see package doc).
}

func TestResolve_Workspace_UnmappedSource(t *testing.T) {
	isolateBeadsDirEnv(t)
	tbl := FromWorkspace([]workspace.LoadResult{
		{Prefix: "bt", AbsPath: t.TempDir()},
	})

	_, err := tbl.Resolve(model.Issue{ID: "zz-1"})
	if err == nil {
		t.Fatal("Resolve() error = nil, want a refusal for an unmapped prefix")
	}
	msg := err.Error()
	if !containsAll(msg, "zz", "workspace.yaml") {
		t.Errorf("refusal message = %q, want it to name the project prefix and the remedy (.bt/workspace.yaml)", msg)
	}
}

func TestResolve_Global_MappingMissing(t *testing.T) {
	isolateBeadsDirEnv(t)
	g := &settings.Global{ProjectPaths: map[string]string{}}
	tbl := FromSettings(g, []string{"myproject"})

	_, err := tbl.Resolve(model.Issue{ID: "myproject-1", SourceRepo: "myproject"})
	if err == nil {
		t.Fatal("Resolve() error = nil, want a refusal for a database with no known checkout path")
	}
	msg := err.Error()
	if !containsAll(msg, "myproject", "settings.json") {
		t.Errorf("refusal message = %q, want it to name the project and the settings.json remedy", msg)
	}
}

func TestResolve_Global_MetadataMissing(t *testing.T) {
	isolateBeadsDirEnv(t)
	// dir exists but has no .beads/ at all.
	dir := t.TempDir()
	g := &settings.Global{ProjectPaths: map[string]string{"myproject": dir}}
	tbl := FromSettings(g, []string{"myproject"})

	_, err := tbl.Resolve(model.Issue{ID: "myproject-1", SourceRepo: "myproject"})
	if err == nil {
		t.Fatal("Resolve() error = nil, want a refusal when metadata.json is missing")
	}
	msg := err.Error()
	if !containsAll(msg, "myproject", dir) {
		t.Errorf("refusal message = %q, want it to name the project and the dir", msg)
	}
}

func TestResolve_Global_IdentityMismatch(t *testing.T) {
	isolateBeadsDirEnv(t)
	dir := t.TempDir()
	// The checkout at dir is actually database "otherproject" - a stale
	// settings.json mapping claims it's "myproject".
	writeMetadata(t, dir, "otherproject", "server")
	g := &settings.Global{ProjectPaths: map[string]string{"myproject": dir}}
	tbl := FromSettings(g, []string{"myproject"})

	_, err := tbl.Resolve(model.Issue{ID: "myproject-1", SourceRepo: "myproject"})
	if err == nil {
		t.Fatal("Resolve() error = nil, want a refusal when dolt_database does not match SourceRepo")
	}
	msg := err.Error()
	if !containsAll(msg, "myproject", "otherproject", dir) {
		t.Errorf("refusal message = %q, want it to name both the expected and actual database and the dir", msg)
	}
}

func TestResolve_Global_EmbeddedRefused(t *testing.T) {
	isolateBeadsDirEnv(t)
	dir := t.TempDir()
	// Correct database identity, but the checkout uses embedded Dolt - it
	// cannot be reached via the shared server bt read this issue from.
	writeMetadata(t, dir, "myproject", "embedded")
	g := &settings.Global{ProjectPaths: map[string]string{"myproject": dir}}
	tbl := FromSettings(g, []string{"myproject"})

	_, err := tbl.Resolve(model.Issue{ID: "myproject-1", SourceRepo: "myproject"})
	if err == nil {
		t.Fatal("Resolve() error = nil, want a refusal for an embedded-mode checkout in global mode")
	}
	msg := err.Error()
	if !containsAll(msg, "myproject", "embedded") {
		t.Errorf("refusal message = %q, want it to mention embedded Dolt", msg)
	}
}

func TestResolve_Global_MappingHit(t *testing.T) {
	isolateBeadsDirEnv(t)
	dir := t.TempDir()
	writeMetadata(t, dir, "myproject", "server")
	g := &settings.Global{ProjectPaths: map[string]string{"myproject": dir}}
	tbl := FromSettings(g, []string{"myproject"})

	target, err := tbl.Resolve(model.Issue{ID: "myproject-1", SourceRepo: "myproject"})
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil for a validated mapping", err)
	}
	if target.Dir != dir {
		t.Errorf("target.Dir = %q, want %q", target.Dir, dir)
	}
	if target.Global {
		t.Errorf("target.Global = true, want false (this issue is not beads_global)")
	}
}

func TestResolve_Global_BeadsGlobalRefused(t *testing.T) {
	isolateBeadsDirEnv(t)
	// Even with a (hypothetical) valid mapping present, beads_global must
	// still refuse in this slice - the follow-up (WriteTarget.Global / `bd
	// --global` routing) is not implemented yet.
	dir := t.TempDir()
	writeMetadata(t, dir, BeadsGlobalDB, "server")
	g := &settings.Global{ProjectPaths: map[string]string{BeadsGlobalDB: dir}}
	tbl := FromSettings(g, []string{BeadsGlobalDB})

	_, err := tbl.Resolve(model.Issue{ID: "global-1", SourceRepo: BeadsGlobalDB})
	if err == nil {
		t.Fatal("Resolve() error = nil, want a refusal for beads_global")
	}
	msg := err.Error()
	if !containsAll(msg, "beads_global", "bd --global") {
		t.Errorf("refusal message = %q, want it to name beads_global and the bd --global follow-up", msg)
	}
}

func TestResolve_Global_SourceRepoEmpty(t *testing.T) {
	isolateBeadsDirEnv(t)
	tbl := FromSettings(&settings.Global{}, nil)

	_, err := tbl.Resolve(model.Issue{ID: "orphan-1"})
	if err == nil {
		t.Fatal("Resolve() error = nil, want a refusal when SourceRepo is empty in global mode")
	}
}

func TestResolve_NilTable(t *testing.T) {
	var tbl *Table
	_, err := tbl.Resolve(model.Issue{ID: "bt-1"})
	if err == nil {
		t.Fatal("Resolve() on a nil table = nil error, want a refusal")
	}
}

// TestResolve_PollutionImmunity is the regression test for the bug this bead
// fixes: resolveClaimDir used to consult pkg/projects (a launch-stamped,
// easily-poisoned cache) FIRST in every mode. A poisoned registry entry for
// a project prefix must have zero effect on bdroute's Resolve - the write
// path never reads pkg/projects at all.
func TestResolve_PollutionImmunity_Workspace(t *testing.T) {
	isolateBeadsDirEnv(t)

	// Poison the registry: "bt" maps to a directory that does not exist
	// (mirrors the reported bug - a deleted bench temp dir).
	regPath := filepath.Join(t.TempDir(), "projects.json")
	poisoned := `{"bt":{"path":"` + filepathToJSON(filepath.Join(t.TempDir(), "deleted-bench-dir")) + `","last_seen":"2026-01-01T00:00:00Z"}}`
	if err := os.WriteFile(regPath, []byte(poisoned), 0o644); err != nil {
		t.Fatalf("seed poisoned registry: %v", err)
	}
	t.Setenv("BT_PROJECTS_REGISTRY_PATH", regPath)

	// The correct mapping, from a real workspace config.
	correctDir := t.TempDir()
	tbl := FromWorkspace([]workspace.LoadResult{{Prefix: "bt", AbsPath: correctDir}})

	target, err := tbl.Resolve(model.Issue{ID: "bt-scc35"})
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if target.Dir != correctDir {
		t.Errorf("target.Dir = %q, want %q (poisoned registry must be ignored)", target.Dir, correctDir)
	}
}

func TestResolve_PollutionImmunity_Global(t *testing.T) {
	isolateBeadsDirEnv(t)

	regPath := filepath.Join(t.TempDir(), "projects.json")
	poisoned := `{"myproject":{"path":"` + filepathToJSON(filepath.Join(t.TempDir(), "deleted-bench-dir")) + `","last_seen":"2026-01-01T00:00:00Z"}}`
	if err := os.WriteFile(regPath, []byte(poisoned), 0o644); err != nil {
		t.Fatalf("seed poisoned registry: %v", err)
	}
	t.Setenv("BT_PROJECTS_REGISTRY_PATH", regPath)

	correctDir := t.TempDir()
	writeMetadata(t, correctDir, "myproject", "server")
	g := &settings.Global{ProjectPaths: map[string]string{"myproject": correctDir}}
	tbl := FromSettings(g, []string{"myproject"})

	target, err := tbl.Resolve(model.Issue{ID: "myproject-1", SourceRepo: "myproject"})
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if target.Dir != correctDir {
		t.Errorf("target.Dir = %q, want %q (poisoned registry must be ignored)", target.Dir, correctDir)
	}
}

// containsAll reports whether s contains every substring in subs.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// filepathToJSON forward-slashes a path so it can be embedded in a hand-built
// JSON string literal without escaping backslashes (Windows paths).
func filepathToJSON(p string) string {
	out := make([]rune, 0, len(p))
	for _, r := range p {
		if r == '\\' {
			out = append(out, '/')
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
