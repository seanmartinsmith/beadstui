// Package projects provides a user-global registry that maps a beads
// project prefix (e.g. "bt", "bd") to the absolute filesystem path of
// the project's git working tree.
//
// The registry is stored at ~/.bt/projects.json and is auto-stamped
// when bt launches inside a project's git tree (see cmd/bt/root.go).
// Consumers - primarily the History view - look up the path for a
// bead's prefix to drive `git log` against the right repo regardless
// of bt's launch mode (cwd / --workspace / --global).
//
// Machine-local cache. Contains absolute filesystem paths; do not sync
// across machines (the paths are meaningless on a different host). The
// file is purely a cache: deleting it has no consequence beyond
// requiring a relaunch from each project to re-stamp.
//
// Concurrency model: Save uses temp-file-plus-rename so a concurrent
// reader never sees a partial file. Lost-update races between two bt
// sessions stamping different prefixes simultaneously are accepted as
// v1 behavior - the next launch re-stamps and self-heals. The race
// window is small (ms-scale read-modify-write); concurrent stamps to
// the same prefix from different paths are vanishingly rare in
// practice.
package projects

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/debug"
)

// Entry records a single project's last-known path, the time it was
// last stamped, and (for monorepos) the relative path from Path to
// the directory containing .beads/.
//
// Subdir is empty when .beads/ is at the git toplevel (the
// overwhelming majority case and the only one v1 stamping produces).
// When non-empty, it is a forward-slash-normalized relative path; a
// consumer running per-project history scopes git log with `-- <Subdir>`.
type Entry struct {
	Path     string    `json:"path"`
	Subdir   string    `json:"subdir,omitempty"`
	LastSeen time.Time `json:"last_seen"`
}

// Registry maps prefix to Entry. The zero value (nil map) is a usable
// empty registry for read paths; Stamp lazily allocates on first
// insert.
type Registry map[string]Entry

// Load reads the registry from disk. Returns an empty Registry (not an
// error) when the file does not exist - first-launch is a normal state.
// Returns an empty Registry (with debug log) when the file is corrupt
// JSON: a corrupt file is treated as an empty cache that the next
// Stamp+Save will rewrite cleanly.
//
// A read error other than ENOENT (e.g. permission denied) is returned
// to the caller so the launch path can decide whether to continue.
func Load(path string) (Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Registry{}, nil
		}
		return Registry{}, fmt.Errorf("read projects registry: %w", err)
	}
	var r Registry
	if err := json.Unmarshal(data, &r); err != nil {
		debug.Log("projects: corrupt registry at %s, treating as empty: %v", path, err)
		return Registry{}, nil
	}
	if r == nil {
		r = Registry{}
	}
	return r, nil
}

// Save writes the registry to disk via temp-file-plus-rename. Creates
// the parent directory if it does not exist. The on-disk JSON is
// pretty-printed (2-space indent) so a curious user can read the file
// directly.
//
// Atomicity guarantee: on POSIX, rename is atomic on the same
// filesystem; on Windows, os.Rename also performs an atomic
// MoveFileExW. A concurrent reader will see either the old contents
// or the new contents - never a partial write. Lost-update races
// between two writers are NOT prevented by this layer (see package
// doc).
func Save(path string, r Registry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir projects dir: %w", err)
	}
	if r == nil {
		r = Registry{}
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal projects registry: %w", err)
	}
	// Append a trailing newline for POSIX-friendly text files.
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp registry: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if anything below fails before the rename.
	cleanup := func() {
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp registry: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("sync temp registry: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp registry: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename temp registry: %w", err)
	}
	return nil
}

// Stamp records (or overwrites) a (prefix, absPath, subdir) entry
// with `now` as last_seen and returns the updated Registry. Pure:
// callers pass the registry they loaded and write back the result.
//
// Subdir is empty for the git-toplevel case (always, in v1 stamping).
// See Entry.Subdir for the future monorepo case.
//
// The registry is allocated lazily if r is nil so the cwd-mode caller
// can do `r := Stamp(nil, prefix, path, "", time.Now())` without a
// preceding Load.
func Stamp(r Registry, prefix, absPath, subdir string, now time.Time) Registry {
	if r == nil {
		r = Registry{}
	}
	r[prefix] = Entry{
		Path:     absPath,
		Subdir:   subdir,
		LastSeen: now,
	}
	return r
}

// Lookup returns the Entry for prefix and whether it is currently
// valid. "Valid" means the recorded path exists AND contains a `.git`
// entry (file or directory - `.git` is a file inside a worktree, a
// directory at the toplevel).
//
// Stale entries (path missing, or no longer a git repo) return
// (entry, false) so the caller can render a specific empty-state
// message that names the stale path. Missing entries return
// (Entry{}, false).
func Lookup(r Registry, prefix string) (Entry, bool) {
	entry, ok := r[prefix]
	if !ok {
		return Entry{}, false
	}
	if !pathLooksLikeGitRepo(entry.Path) {
		return entry, false
	}
	return entry, true
}

// LookupAndValidate is the consumer's one-liner: load the default
// registry, look up prefix, return the absolute path if valid.
// Returns ("", false) on any failure mode (missing file, corrupt
// file, missing prefix, stale path). Errors are debug-logged but
// never surfaced - the History view's empty-state message is the
// user-visible feedback channel.
func LookupAndValidate(prefix string) (string, bool) {
	path, err := resolvedPath()
	if err != nil {
		debug.Log("projects: cannot resolve registry path: %v", err)
		return "", false
	}
	r, err := Load(path)
	if err != nil {
		debug.Log("projects: load failed for lookup of %q: %v", prefix, err)
		return "", false
	}
	entry, ok := Lookup(r, prefix)
	if !ok {
		return "", false
	}
	return entry.Path, true
}

// pathLooksLikeGitRepo returns true if path contains a `.git` entry.
// Used by Lookup to decide whether a registry entry is fresh enough
// to drive `git log`.
//
// The empty-string guard is kept because filepath.Join("", ".git")
// resolves to ".git" (cwd-relative), which would falsely match the
// current directory's repo. The first os.Stat(path) check is
// intentionally omitted: if path does not exist, the os.Stat on
// path/.git also fails, making the first check redundant.
func pathLooksLikeGitRepo(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		return false
	}
	return true
}
