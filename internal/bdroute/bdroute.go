// Package bdroute resolves the filesystem directory (or --global routing) a
// bt-initiated bd write should target, given the launch mode bt booted into.
//
// The route table is built ONCE at launch (cmd/bt/root.go, one constructor
// per mode) and consulted at WRITE time by Resolve. A non-nil error from
// Resolve is a pre-flight refusal: the caller must not invoke bd at all in
// that case.
//
// This replaces routing bd writes through pkg/projects (the user-global
// prefix->path registry keyed by bead-ID prefix). That registry is a
// launch-stamped CACHE for the read-only History view, and any git repo path
// happens to pass its (path-exists + .git-present) validation - consulting it
// for writes let a stale/misleading entry silently redirect a claim into the
// wrong checkout (bt-scc35, bt-2pk38). The three modes here use trust models
// suited to what's actually knowable at each layer:
//
//   - single-project: the launch directory IS the target. No registry, no
//     identity check - you are standing in the only project bt knows about.
//   - workspace (.bt/workspace.yaml): the user hand-wrote the prefix->path
//     mapping. It is authoritative; a wrong path is a config error that bd's
//     own error surfaces, not something bdroute second-guesses.
//   - global (shared Dolt server): the mapping (dbname->path) is a CACHE
//     (settings.json project_paths, auto-stamped on single-project boots).
//     Caches can go stale, so Resolve re-reads the candidate directory's own
//     .beads/metadata.json at write time and requires dolt_database to match
//     the issue's SourceRepo (and dolt_mode to not be embedded) before
//     trusting the path.
package bdroute

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/seanmartinsmith/beadstui/internal/settings"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/workspace"
)

// BeadsGlobalDB is the project-agnostic shared-server database name (beads
// v1.0.1, upstream PR #3270; `bd --global`). Routing a write there requires
// `bd --global` dispatch (WriteTarget.Global) rather than a checkout
// directory, which is a designed follow-up (bt-scc35) - Resolve refuses it
// for now with an actionable message rather than guessing a checkout.
const BeadsGlobalDB = "beads_global"

// WriteTarget is where a bd write should run: either a project checkout
// directory (Dir, used as cmd.Dir) or the project-agnostic shared registry
// (Global, routed via `bd --global` - no checkout directory needed).
type WriteTarget struct {
	Dir    string
	Global bool
}

type routeMode int

const (
	modeSingleProject routeMode = iota
	modeWorkspace
	modeGlobal
)

// Table is a launch-time-built lookup from issue identity to write target.
// Built once per bt launch (see the constructors below) and consulted at
// write time by Resolve. The zero value is not directly usable - always
// construct via SingleProject, FromWorkspace, or FromSettings. A nil *Table
// is handled defensively by Resolve (refusal, not a panic).
type Table struct {
	mode routeMode

	// modeSingleProject: the one directory every claim targets.
	singleDir string

	// modeWorkspace: bead-ID prefix -> repo AbsPath (workspace.LoadResult).
	workspacePaths map[string]string

	// modeGlobal: dolt_database name -> checkout path, sourced from
	// settings.json's project_paths and filtered to the databases this
	// launch actually discovered.
	globalPaths map[string]string
}

// SingleProject builds the route table for a plain, non-workspace,
// non-global launch: every claim resolves to workDir (the project bt was
// launched inside), full stop. No registry lookup, no per-issue branching.
func SingleProject(workDir string) *Table {
	return &Table{mode: modeSingleProject, singleDir: workDir}
}

// FromWorkspace builds the route table for a `.bt/workspace.yaml` launch,
// keyed by each repo's config-declared Prefix - the same prefix
// workspace.AggregateLoader stamps onto every issue ID it namespaces
// (QualifyID), so a loaded issue's ID prefix is exactly the lookup key.
// Results with no Prefix or no AbsPath are skipped: they didn't load cleanly
// enough to route a write to.
func FromWorkspace(results []workspace.LoadResult) *Table {
	paths := make(map[string]string, len(results))
	for _, r := range results {
		if r.Prefix == "" || r.AbsPath == "" {
			continue
		}
		paths[r.Prefix] = r.AbsPath
	}
	return &Table{mode: modeWorkspace, workspacePaths: paths}
}

// FromSettings builds the route table for global (shared Dolt server) mode.
// dbNames is the set of database names this launch actually discovered (e.g.
// the distinct issue.SourceRepo values loaded from the shared server); only
// those are copied out of g.ProjectPaths, so a stale settings.json entry for
// a database this launch never saw can't leak into the table. g may be nil
// (settings load failure) - the table is then built with no mappings, and
// every Resolve call against it refuses with the "no known checkout path"
// error.
func FromSettings(g *settings.Global, dbNames []string) *Table {
	paths := make(map[string]string, len(dbNames))
	if g != nil {
		for _, name := range dbNames {
			if p, ok := g.ProjectPaths[name]; ok && p != "" {
				paths[name] = p
			}
		}
	}
	return &Table{mode: modeGlobal, globalPaths: paths}
}

// Resolve returns the write target for issue, or a non-nil error that is a
// pre-flight refusal - callers must not invoke bd at all when Resolve
// returns an error. issue.ID drives workspace-mode lookup (prefix before the
// first hyphen); issue.SourceRepo drives global-mode lookup and identity
// verification.
func (t *Table) Resolve(issue model.Issue) (WriteTarget, error) {
	if t == nil {
		return WriteTarget{}, fmt.Errorf("claim %s: no write-routing table configured (internal error); relaunch bt", issue.ID)
	}

	switch t.mode {
	case modeSingleProject:
		return WriteTarget{Dir: t.singleDir}, nil
	case modeWorkspace:
		return t.resolveWorkspace(issue)
	case modeGlobal:
		return t.resolveGlobal(issue)
	default:
		return WriteTarget{}, fmt.Errorf("claim %s: no write-routing table configured (internal error); relaunch bt", issue.ID)
	}
}

func (t *Table) resolveWorkspace(issue model.Issue) (WriteTarget, error) {
	prefix, _, ok := strings.Cut(issue.ID, "-")
	if !ok || prefix == "" {
		return WriteTarget{}, fmt.Errorf("claim %s: cannot determine a project prefix from the issue ID", issue.ID)
	}
	dir, ok := t.workspacePaths[prefix]
	if !ok {
		return WriteTarget{}, fmt.Errorf("claim %s: no workspace mapping for project prefix %q; check repos[].prefix in .bt/workspace.yaml", issue.ID, prefix)
	}
	return WriteTarget{Dir: dir}, nil
}

func (t *Table) resolveGlobal(issue model.Issue) (WriteTarget, error) {
	if issue.SourceRepo == "" {
		return WriteTarget{}, fmt.Errorf("claim %s: issue has no source-repo metadata; cannot determine which project owns it in global mode", issue.ID)
	}
	if issue.SourceRepo == BeadsGlobalDB {
		return WriteTarget{}, fmt.Errorf("claim %s: writes to beads_global are not yet supported by bt; run `bd --global update %s --claim` directly", issue.ID, issue.ID)
	}

	dir, ok := t.globalPaths[issue.SourceRepo]
	if !ok {
		return WriteTarget{}, fmt.Errorf("claim %s: no known checkout path for project %q; cd into that project once so bt can record it (~/.bt/settings.json), then relaunch bt --global", issue.ID, issue.SourceRepo)
	}

	meta, err := ReadMetadataAt(dir)
	if err != nil {
		return WriteTarget{}, fmt.Errorf("claim %s: cannot verify project %q at %s: %w", issue.ID, issue.SourceRepo, dir, err)
	}
	if meta.DoltDatabase != issue.SourceRepo {
		return WriteTarget{}, fmt.Errorf("claim %s: project mismatch - %s is database %q, expected %q; the settings.json mapping is stale, cd into the correct project once to refresh it", issue.ID, dir, meta.DoltDatabase, issue.SourceRepo)
	}
	if meta.DoltMode == "embedded" {
		return WriteTarget{}, fmt.Errorf("claim %s: project %q at %s uses embedded Dolt, not the shared server bt read this issue from; cannot route a global-mode claim there", issue.ID, issue.SourceRepo, dir)
	}

	return WriteTarget{Dir: dir}, nil
}

// ProjectMetadata is the subset of .beads/metadata.json bdroute (and its
// callers, e.g. cmd/bt/root.go's detectProjectDBAt) need.
type ProjectMetadata struct {
	DoltDatabase string `json:"dolt_database"`
	DoltMode     string `json:"dolt_mode"`
	ProjectID    string `json:"project_id"`
}

// ReadMetadataAt reads and parses the .beads/metadata.json reachable from
// dir, via the same discovery rule as the rest of bt (loader.GetBeadsDir:
// BEADS_DIR env override, then git-worktree walk-up to the main repo's
// .beads/). Returns an error naming dir on any failure - dir unresolvable,
// metadata.json missing/unreadable, or malformed JSON - so callers can build
// an actionable refusal message.
func ReadMetadataAt(dir string) (ProjectMetadata, error) {
	beadsDir, err := loader.GetBeadsDir(dir)
	if err != nil {
		return ProjectMetadata{}, fmt.Errorf("resolve .beads dir under %s: %w", dir, err)
	}
	data, err := os.ReadFile(filepath.Join(beadsDir, "metadata.json"))
	if err != nil {
		return ProjectMetadata{}, fmt.Errorf("read metadata.json under %s: %w", dir, err)
	}
	var meta ProjectMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return ProjectMetadata{}, fmt.Errorf("parse metadata.json under %s: %w", dir, err)
	}
	return meta, nil
}
