// bd_adapter.go implements the Adapter interface (adapter.go) for the four
// bd-managed SourceKinds (design spec §4.1, §7): bd-embedded, bd-server,
// bd-shared, and beads-global. Each wraps an existing internal/datasource
// reader for beads rather than duplicating its logic, and adds the memories
// read (`bd memories --json` via internal/bdexec) that bt could not see
// before bt-2ea7t.
package source

import (
	"context"
	"fmt"
	"strings"

	"github.com/seanmartinsmith/beadstui/internal/bdexec"
	"github.com/seanmartinsmith/beadstui/internal/datasource"
)

// bdRunner matches bdexec.Run's signature and is injectable per-adapter so
// the memories read is unit-testable without a live `bd` binary or beads
// project (bt-2ea7t.2 TDD instructions: fixture strings, no live server). A
// nil runner defaults to bdexec.Run - see runMemories.
type bdRunner func(ctx context.Context, dir string, args ...string) bdexec.Result

// runMemories is the single memories-read implementation shared by every bd
// adapter below: it shells `bd memories --json` (never a direct SQL read -
// memories has no stable schema bt should couple to; §4.2) and parses the
// flat map via ParseMemoriesJSON.
//
// dir sets the subprocess's working directory (bdexec.Run's cmd.Dir), the
// same mechanism datasource.EmbeddedReader already uses to target `bd
// export` at a specific project - no `-C` flag needed. global adds
// --global, routing bd to the beads_global database regardless of dir's own
// project identity (see sharedDBAdapter's doc comment for the caveat this
// still requires dir to resolve to *some* beads project).
//
// A runner failure (non-zero exit, spawn failure, timeout - anything
// bdexec.Result.Err covers) or an unparseable payload both degrade this
// source to MemoriesResult.Err - never a panic, never propagated as a fatal
// error to the caller (spec §8).
func runMemories(ctx context.Context, origin Origin, dir string, global bool, runner bdRunner) MemoriesResult {
	if runner == nil {
		runner = bdexec.Run
	}
	args := []string{"memories", "--json"}
	if global {
		args = append(args, "--global")
	}

	res := runner(ctx, dir, args...)
	if res.Err != nil {
		return MemoriesResult{Origin: origin, Err: fmt.Errorf("%s: %w%s", res.Argv(), res.Err, stderrSuffix(res.Stderr))}
	}

	memories, err := ParseMemoriesJSON([]byte(res.Stdout), origin)
	if err != nil {
		return MemoriesResult{Origin: origin, Err: fmt.Errorf("%s: %w", res.Argv(), err)}
	}
	return MemoriesResult{Origin: origin, Memories: memories}
}

// stderrSuffix formats a non-empty stderr capture as " (<stderr>)" for an
// error message, mirroring datasource.EmbeddedReader.LoadIssues' pattern for
// surfacing a failed subprocess's stderr.
func stderrSuffix(stderr string) string {
	if s := strings.TrimSpace(stderr); s != "" {
		return " (" + s + ")"
	}
	return ""
}

// --- bd-embedded ---

// embeddedAdapter reads a bd-embedded source (spec §3 state-space row 1):
// beads via `bd export` (datasource.EmbeddedReader - bt-qrt2u: never attach
// a server, which would deadlock the concurrent bd CLI), memories via `bd
// memories --json` run in the same directory.
type embeddedAdapter struct {
	origin   Origin
	repoRoot string // directory containing .beads/ - bd's cwd for both reads
	runner   bdRunner
}

// NewEmbeddedAdapter builds an Adapter for a bd-embedded project rooted at
// repoRoot (the directory containing .beads/, matching
// datasource.DataSource.Path for SourceTypeEmbeddedDolt).
func NewEmbeddedAdapter(origin Origin, repoRoot string) Adapter {
	return &embeddedAdapter{origin: origin, repoRoot: repoRoot}
}

func (a *embeddedAdapter) Origin() Origin { return a.origin }

func (a *embeddedAdapter) ListBeads(_ context.Context) BeadsResult {
	reader, err := datasource.NewEmbeddedReader(datasource.DataSource{
		Type: datasource.SourceTypeEmbeddedDolt,
		Path: a.repoRoot,
	})
	if err != nil {
		return BeadsResult{Origin: a.origin, Err: err}
	}
	issues, err := reader.LoadIssues()
	if err != nil {
		return BeadsResult{Origin: a.origin, Err: err}
	}
	return BeadsResult{Origin: a.origin, Issues: issues}
}

func (a *embeddedAdapter) ListMemories(ctx context.Context) MemoriesResult {
	return runMemories(ctx, a.origin, a.repoRoot, false, a.runner)
}

// --- bd-server ---

// serverAdapter reads a bd-server source (spec §3 row 2): beads via a direct
// Dolt MySQL connection (datasource.DoltReader), memories via `bd memories
// --json` run in the project's own checkout directory.
type serverAdapter struct {
	origin   Origin
	dsn      string // Dolt MySQL DSN, pinned to this project's database
	repoRoot string // project checkout dir - bd's cwd for the memories read
	runner   bdRunner
}

// NewServerAdapter builds an Adapter for a per-project Dolt server source.
// dsn is the connection string (as datasource.DataSource.Path for
// SourceTypeDolt); repoRoot is the project's checkout directory bd shells
// out from for the memories read.
func NewServerAdapter(origin Origin, dsn, repoRoot string) Adapter {
	return &serverAdapter{origin: origin, dsn: dsn, repoRoot: repoRoot}
}

func (a *serverAdapter) Origin() Origin { return a.origin }

func (a *serverAdapter) ListBeads(_ context.Context) BeadsResult {
	reader, err := datasource.NewDoltReader(datasource.DataSource{
		Type: datasource.SourceTypeDolt,
		Path: a.dsn,
	})
	if err != nil {
		return BeadsResult{Origin: a.origin, Err: err}
	}
	defer reader.Close()

	issues, err := reader.LoadIssues()
	if err != nil {
		return BeadsResult{Origin: a.origin, Err: err}
	}
	return BeadsResult{Origin: a.origin, Issues: issues}
}

func (a *serverAdapter) ListMemories(ctx context.Context) MemoriesResult {
	return runMemories(ctx, a.origin, a.repoRoot, false, a.runner)
}

// --- bd-shared and beads-global ---

// sharedDBAdapter reads a source that is, mechanically, one database on the
// shared Dolt server (spec §3 rows 3-4): bd-shared (an ordinary project's
// own database) and beads-global (the fixed db name "beads_global") are read
// identically for beads - a datasource.GlobalDoltReader scoped to
// origin.Scope via DataSource.RepoFilter, reusing the exact enumeration/UNION
// machinery bt's global mode already relies on. They differ only in how the
// memories read targets bd:
//   - bd-shared runs `bd memories --json` with the project's own checkout
//     directory as cwd (global=false).
//   - beads-global instead passes --global so bd routes to the beads_global
//     database regardless of which project the anchor directory itself is
//     (global=true).
//
// Empirically verified (2026-07-15, live against this project's own bd):
// `bd --global memories --json` still requires repoRoot to resolve to *some*
// bd-managed directory - run from a directory with no .beads/ ancestor at
// all it fails with "no beads database found" even though --global is set.
// It just ignores that anchor directory's OWN database identity once it
// finds one. This is a load-bearing discovery beyond the bt-2ea7t.2 task
// brief's "verified facts" section, so NewGlobalAdapter takes a repoRoot
// (any known bd-managed project directory), not "" - unlike a hypothetical
// directory-independent global read.
type sharedDBAdapter struct {
	origin   Origin
	dsn      string // shared Dolt server DSN, no database selected (see globalDSN)
	repoRoot string // anchor directory for the memories shell-out
	global   bool   // true => beads-global (adds --global to `bd memories`)
	runner   bdRunner
}

// NewSharedAdapter builds an Adapter for one database on the shared Dolt
// server (bd-shared). dsn is the multi-DB DSN (datasource.NewGlobalDataSource
// / globalDSN shape, no database segment); repoRoot is that project's own
// checkout directory.
func NewSharedAdapter(origin Origin, dsn, repoRoot string) Adapter {
	return &sharedDBAdapter{origin: origin, dsn: dsn, repoRoot: repoRoot}
}

// NewGlobalAdapter builds an Adapter for the beads_global database on the
// shared Dolt server. dsn is the same multi-DB DSN shape as NewSharedAdapter;
// repoRoot is any known bd-managed project directory to anchor the `bd
// --global` memories shell-out (see sharedDBAdapter's doc comment).
func NewGlobalAdapter(origin Origin, dsn, repoRoot string) Adapter {
	return &sharedDBAdapter{origin: origin, dsn: dsn, repoRoot: repoRoot, global: true}
}

func (a *sharedDBAdapter) Origin() Origin { return a.origin }

func (a *sharedDBAdapter) ListBeads(_ context.Context) BeadsResult {
	reader, err := datasource.NewGlobalDoltReader(datasource.DataSource{
		Type:       datasource.SourceTypeDoltGlobal,
		Path:       a.dsn,
		RepoFilter: a.origin.Scope,
	})
	if err != nil {
		return BeadsResult{Origin: a.origin, Err: err}
	}
	defer reader.Close()

	issues, err := reader.LoadIssues()
	if err != nil {
		return BeadsResult{Origin: a.origin, Err: err}
	}
	return BeadsResult{Origin: a.origin, Issues: issues}
}

func (a *sharedDBAdapter) ListMemories(ctx context.Context) MemoriesResult {
	return runMemories(ctx, a.origin, a.repoRoot, a.global, a.runner)
}

// compile-time interface satisfaction checks.
var (
	_ Adapter = (*embeddedAdapter)(nil)
	_ Adapter = (*serverAdapter)(nil)
	_ Adapter = (*sharedDBAdapter)(nil)
)
