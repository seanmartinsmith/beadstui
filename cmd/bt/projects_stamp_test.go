package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/projects"
	"github.com/seanmartinsmith/beadstui/pkg/workspace"
)

// makeGitRepoDir creates a temp directory and runs `git init` inside it,
// returning the canonical (symlink-resolved) directory path. A real init is
// required because correlation.Toplevel shells out to
// `git rev-parse --show-toplevel`, which rejects directories with only a
// fake .git/ subdirectory.
func makeGitRepoDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("makeGitRepoDir: git init in %s: %v", dir, err)
	}
	// Resolve any symlinks/short-name forms so the path matches what
	// `git rev-parse --show-toplevel` reports back. On Windows, t.TempDir()
	// can return a short-name path while git emits the long form.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("makeGitRepoDir: EvalSymlinks: %v", err)
	}
	return filepath.Clean(resolved)
}

// writeBeadsMetadata writes a minimal .beads/metadata.json into dir with the
// given dolt_database value. Satisfies detectProjectDBAt.
func writeBeadsMetadata(t *testing.T, dir, dbName string) {
	t.Helper()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("writeBeadsMetadata: mkdir .beads: %v", err)
	}
	data, _ := json.Marshal(map[string]string{"dolt_database": dbName})
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), data, 0o644); err != nil {
		t.Fatalf("writeBeadsMetadata: write metadata.json: %v", err)
	}
}

// setIsolatedRegistry sets BT_PROJECTS_REGISTRY_PATH to a fresh path for the
// duration of the test and returns the path for inspection.
func setIsolatedRegistry(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "projects.json")
	t.Setenv("BT_PROJECTS_REGISTRY_PATH", path)
	return path
}

// inspectRegistry reads the registry at path and returns it. Missing file
// returns an empty Registry (mirrors projects.Load semantics).
func inspectRegistry(t *testing.T, path string) projects.Registry {
	t.Helper()
	r, err := projects.Load(path)
	if err != nil {
		t.Fatalf("inspectRegistry: %v", err)
	}
	return r
}

// hasGitAncestor walks upward from dir looking for a .git entry. Used to
// skip tests when t.TempDir() happens to live inside a git repo.
func hasGitAncestor(dir string) bool {
	current := dir
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}

// TestStampLaunchProjects_CwdMode verifies that a directory containing both
// .git/ and .beads/metadata.json produces exactly one registry entry mapping
// the dolt_database value (the prefix) to the git toplevel.
//
// This test exercises the cwd-stamp branch of stampLaunchProjects. That
// branch has multiple call sites in loadIssues (local-project fall-through
// AND auto-global-from-inside-project) which all pass identical args:
// (cwd, nil, false, false). Verifying the helper once is sufficient - that
// loadIssues actually invokes it from each site is verified by reading the
// code, not by a unit test that can't observe call sites.
func TestStampLaunchProjects_CwdMode(t *testing.T) {
	regPath := setIsolatedRegistry(t)

	// Create a project directory: .git/ at the root makes Toplevel() return
	// the directory itself, so the stamped path equals dir.
	dir := makeGitRepoDir(t)
	writeBeadsMetadata(t, dir, "myproject")

	stampLaunchProjects(dir, nil, false, false)

	r := inspectRegistry(t, regPath)
	if len(r) != 1 {
		t.Fatalf("expected 1 registry entry, got %d: %v", len(r), r)
	}
	entry, ok := r["myproject"]
	if !ok {
		t.Fatalf("expected entry for prefix 'myproject', got keys: %v", func() []string {
			var ks []string
			for k := range r {
				ks = append(ks, k)
			}
			return ks
		}())
	}
	if entry.Path != dir {
		t.Errorf("entry.Path = %q, want %q", entry.Path, dir)
	}
}

// TestStampLaunchProjects_WorkspaceMode_MultiRepo verifies that workspace mode
// stamps one entry per LoadResult that has a non-empty Prefix and a git repo
// at its AbsPath.
func TestStampLaunchProjects_WorkspaceMode_MultiRepo(t *testing.T) {
	regPath := setIsolatedRegistry(t)

	dir1 := makeGitRepoDir(t)
	dir2 := makeGitRepoDir(t)

	results := []workspace.LoadResult{
		{Prefix: "bt", AbsPath: dir1},
		{Prefix: "bd", AbsPath: dir2},
	}

	stampLaunchProjects("", results, false, false)

	r := inspectRegistry(t, regPath)
	if len(r) != 2 {
		t.Fatalf("expected 2 registry entries, got %d: %v", len(r), r)
	}
	if e, ok := r["bt"]; !ok || e.Path != dir1 {
		t.Errorf("bt entry: path=%q ok=%v, want path=%q", e.Path, ok, dir1)
	}
	if e, ok := r["bd"]; !ok || e.Path != dir2 {
		t.Errorf("bd entry: path=%q ok=%v, want path=%q", e.Path, ok, dir2)
	}
}

// TestStampLaunchProjects_GlobalMode verifies that --global launches are a
// no-op: the registry file is not created or modified.
func TestStampLaunchProjects_GlobalMode(t *testing.T) {
	regPath := setIsolatedRegistry(t)

	stampLaunchProjects("", nil, true, false)

	r := inspectRegistry(t, regPath)
	if len(r) != 0 {
		t.Errorf("expected empty registry for global mode, got %d entries: %v", len(r), r)
	}
}

// TestStampLaunchProjects_NoBeadsCwd verifies that a directory with .git/ but
// no .beads/metadata.json produces no registry entry (prefix is empty).
func TestStampLaunchProjects_NoBeadsCwd(t *testing.T) {
	regPath := setIsolatedRegistry(t)

	dir := makeGitRepoDir(t) // .git/ only; no .beads/

	stampLaunchProjects(dir, nil, false, false)

	r := inspectRegistry(t, regPath)
	if len(r) != 0 {
		t.Errorf("expected empty registry when .beads/ is absent, got %d entries: %v", len(r), r)
	}
}

// TestStampLaunchProjects_NoGitCwd verifies that a directory with
// .beads/metadata.json but outside any git repo produces no registry entry
// because Toplevel returns an empty string.
func TestStampLaunchProjects_NoGitCwd(t *testing.T) {
	regPath := setIsolatedRegistry(t)

	dir := t.TempDir()

	if hasGitAncestor(dir) {
		t.Skipf("temp dir %s is inside a git repo; skipping", dir)
	}

	writeBeadsMetadata(t, dir, "someproject")

	stampLaunchProjects(dir, nil, false, false)

	r := inspectRegistry(t, regPath)
	if len(r) != 0 {
		t.Errorf("expected empty registry when outside a git repo, got %d entries: %v", len(r), r)
	}
}

// TestStampLaunchProjects_WorkspaceFailedRepo verifies that workspace mode
// stamps only repos whose AbsPath resolves to a real git toplevel. A repo
// pointing at a non-existent directory is silently skipped.
func TestStampLaunchProjects_WorkspaceFailedRepo(t *testing.T) {
	regPath := setIsolatedRegistry(t)

	goodDir := makeGitRepoDir(t)
	badDir := filepath.Join(t.TempDir(), "does_not_exist")

	results := []workspace.LoadResult{
		{Prefix: "good", AbsPath: goodDir},
		{Prefix: "bad", AbsPath: badDir},
	}

	stampLaunchProjects("", results, false, false)

	r := inspectRegistry(t, regPath)
	if len(r) != 1 {
		t.Fatalf("expected 1 registry entry (good repo only), got %d: %v", len(r), r)
	}
	if e, ok := r["good"]; !ok || e.Path != goodDir {
		t.Errorf("good entry: path=%q ok=%v, want path=%q", e.Path, ok, goodDir)
	}
	if _, ok := r["bad"]; ok {
		t.Error("bad entry should not be in registry")
	}
}
