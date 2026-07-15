package source

import (
	"context"
	"errors"
	"testing"

	"github.com/seanmartinsmith/beadstui/internal/bdexec"
)

// recordingRunner returns a bdRunner that records the (dir, args) it was
// called with and replays a canned bdexec.Result - so the memories path is
// unit-testable without a live `bd` binary or beads project (per bt-2ea7t.2's
// TDD instructions: parse from fixture strings, no live server needed).
func recordingRunner(result bdexec.Result, gotDir *string, gotArgs *[]string) bdRunner {
	return func(_ context.Context, dir string, args ...string) bdexec.Result {
		if gotDir != nil {
			*gotDir = dir
		}
		if gotArgs != nil {
			*gotArgs = args
		}
		return result
	}
}

// --- embeddedAdapter ---

func TestEmbeddedAdapter_Origin(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBDEmbedded, Scope: "myproj", Prefix: "myproj", DisplayName: "myproj"}
	a := NewEmbeddedAdapter(origin, t.TempDir())
	if got := a.Origin(); got != origin {
		t.Errorf("Origin() = %+v, want %+v", got, origin)
	}
}

// TestEmbeddedAdapter_ListBeads_BDMissingIsUnavailable exercises the real
// datasource.EmbeddedReader path (no injection) with `bd` hidden from PATH,
// covering spec §8's "bd binary missing" case: the source degrades to
// BeadsResult.Err, Issues stays empty, and Origin is still populated -
// never a panic or an unrecoverable error.
func TestEmbeddedAdapter_ListBeads_BDMissingIsUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // hide bd - no real bd/server involved

	origin := Origin{SourceKind: SourceKindBDEmbedded, Scope: "myproj"}
	a := NewEmbeddedAdapter(origin, t.TempDir())

	got := a.ListBeads(context.Background())
	if got.Err == nil {
		t.Fatal("ListBeads Err = nil, want non-nil (bd hidden from PATH)")
	}
	if got.Origin != origin {
		t.Errorf("Origin = %+v, want %+v", got.Origin, origin)
	}
	if len(got.Issues) != 0 {
		t.Errorf("Issues = %v, want empty on failure", got.Issues)
	}
}

func TestEmbeddedAdapter_ListMemories_Success(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBDEmbedded, Scope: "myproj"}
	repoRoot := t.TempDir()
	var gotDir string
	var gotArgs []string
	a := &embeddedAdapter{
		origin:   origin,
		repoRoot: repoRoot,
		runner:   recordingRunner(bdexec.Result{Stdout: fixtureMemoriesJSON, ExitCode: 0}, &gotDir, &gotArgs),
	}

	got := a.ListMemories(context.Background())
	if got.Err != nil {
		t.Fatalf("ListMemories Err = %v, want nil", got.Err)
	}
	if len(got.Memories) != 2 {
		t.Fatalf("len(Memories) = %d, want 2", len(got.Memories))
	}
	for _, m := range got.Memories {
		if m.Origin != origin {
			t.Errorf("Memory %q Origin = %+v, want %+v", m.Key, m.Origin, origin)
		}
	}
	if gotDir != repoRoot {
		t.Errorf("runner dir = %q, want repoRoot %q", gotDir, repoRoot)
	}
	if len(gotArgs) < 2 || gotArgs[0] != "memories" || gotArgs[1] != "--json" {
		t.Errorf("runner args = %v, want to start with [memories --json]", gotArgs)
	}
	for _, a := range gotArgs {
		if a == "--global" {
			t.Errorf("embeddedAdapter must not pass --global, got args %v", gotArgs)
		}
	}
}

// TestEmbeddedAdapter_ListMemories_RunnerErrorIsUnavailable covers spec §8's
// "bd binary missing"/"unreachable source" degrade-to-unavailable contract
// at the adapter level: a runner failure surfaces as MemoriesResult.Err,
// Memories stays empty, Origin is still populated.
func TestEmbeddedAdapter_ListMemories_RunnerErrorIsUnavailable(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBDEmbedded, Scope: "myproj"}
	a := &embeddedAdapter{
		origin:   origin,
		repoRoot: t.TempDir(),
		runner:   recordingRunner(bdexec.Result{Err: errors.New("exec: \"bd\": executable file not found in $PATH")}, nil, nil),
	}

	got := a.ListMemories(context.Background())
	if got.Err == nil {
		t.Fatal("ListMemories Err = nil, want non-nil")
	}
	if got.Origin != origin {
		t.Errorf("Origin = %+v, want %+v", got.Origin, origin)
	}
	if len(got.Memories) != 0 {
		t.Errorf("Memories = %v, want empty on failure", got.Memories)
	}
}

// --- serverAdapter ---

func TestServerAdapter_Origin(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBDServer, Scope: "myproj"}
	a := NewServerAdapter(origin, "root@tcp(127.0.0.1:1)/x", t.TempDir())
	if got := a.Origin(); got != origin {
		t.Errorf("Origin() = %+v, want %+v", got, origin)
	}
}

// TestServerAdapter_ListBeads_UnreachableIsUnavailable dials a DSN with no
// listener (port 1 on loopback) - a fast, deterministic "connection refused"
// with no live Dolt server involved - covering spec §8's "unreachable
// per-project server" case.
func TestServerAdapter_ListBeads_UnreachableIsUnavailable(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBDServer, Scope: "myproj"}
	a := NewServerAdapter(origin, "root@tcp(127.0.0.1:1)/nonexistent?timeout=500ms", t.TempDir())

	got := a.ListBeads(context.Background())
	if got.Err == nil {
		t.Fatal("ListBeads Err = nil, want non-nil (no listener on port 1)")
	}
	if got.Origin != origin {
		t.Errorf("Origin = %+v, want %+v", got.Origin, origin)
	}
	if len(got.Issues) != 0 {
		t.Errorf("Issues = %v, want empty on failure", got.Issues)
	}
}

func TestServerAdapter_ListMemories_Success(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBDServer, Scope: "myproj"}
	repoRoot := t.TempDir()
	var gotDir string
	var gotArgs []string
	a := &serverAdapter{
		origin:   origin,
		dsn:      "root@tcp(127.0.0.1:1)/x",
		repoRoot: repoRoot,
		runner:   recordingRunner(bdexec.Result{Stdout: fixtureMemoriesJSON, ExitCode: 0}, &gotDir, &gotArgs),
	}

	got := a.ListMemories(context.Background())
	if got.Err != nil {
		t.Fatalf("ListMemories Err = %v, want nil", got.Err)
	}
	if len(got.Memories) != 2 {
		t.Fatalf("len(Memories) = %d, want 2", len(got.Memories))
	}
	if gotDir != repoRoot {
		t.Errorf("runner dir = %q, want repoRoot %q", gotDir, repoRoot)
	}
}

// --- sharedDBAdapter (bd-shared and beads-global) ---

func TestNewSharedAdapter_Origin(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBDShared, Scope: "otherproj"}
	a := NewSharedAdapter(origin, "root@tcp(127.0.0.1:1)/?x", t.TempDir())
	if got := a.Origin(); got != origin {
		t.Errorf("Origin() = %+v, want %+v", got, origin)
	}
}

func TestNewGlobalAdapter_Origin(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBeadsGlobal, Scope: "beads_global", DisplayName: "atlas"}
	a := NewGlobalAdapter(origin, "root@tcp(127.0.0.1:1)/?x", t.TempDir())
	if got := a.Origin(); got != origin {
		t.Errorf("Origin() = %+v, want %+v", got, origin)
	}
}

// TestSharedDBAdapter_ListBeads_UnreachableIsUnavailable covers both
// bd-shared and beads-global through the same underlying adapter (they read
// beads identically - one database on the shared Dolt server, scoped via
// GlobalDoltReader's RepoFilter; see bd_adapter.go doc comment).
func TestSharedDBAdapter_ListBeads_UnreachableIsUnavailable(t *testing.T) {
	cases := []struct {
		name string
		a    Adapter
	}{
		{"shared", NewSharedAdapter(Origin{SourceKind: SourceKindBDShared, Scope: "otherproj"}, "root@tcp(127.0.0.1:1)/?timeout=500ms", t.TempDir())},
		{"global", NewGlobalAdapter(Origin{SourceKind: SourceKindBeadsGlobal, Scope: "beads_global"}, "root@tcp(127.0.0.1:1)/?timeout=500ms", t.TempDir())},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.a.ListBeads(context.Background())
			if got.Err == nil {
				t.Fatal("ListBeads Err = nil, want non-nil (no listener on port 1)")
			}
			if len(got.Issues) != 0 {
				t.Errorf("Issues = %v, want empty on failure", got.Issues)
			}
		})
	}
}

// TestSharedDBAdapter_ListMemories_SharedUsesDirNoGlobalFlag verifies the
// bd-shared memories read shells `bd memories --json` with the project's own
// checkout directory as cwd and does NOT pass --global.
func TestSharedDBAdapter_ListMemories_SharedUsesDirNoGlobalFlag(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBDShared, Scope: "otherproj"}
	repoRoot := t.TempDir()
	var gotDir string
	var gotArgs []string
	a := &sharedDBAdapter{
		origin:   origin,
		repoRoot: repoRoot,
		global:   false,
		runner:   recordingRunner(bdexec.Result{Stdout: fixtureMemoriesJSON, ExitCode: 0}, &gotDir, &gotArgs),
	}

	got := a.ListMemories(context.Background())
	if got.Err != nil {
		t.Fatalf("ListMemories Err = %v, want nil", got.Err)
	}
	if len(got.Memories) != 2 {
		t.Fatalf("len(Memories) = %d, want 2", len(got.Memories))
	}
	if gotDir != repoRoot {
		t.Errorf("runner dir = %q, want repoRoot %q", gotDir, repoRoot)
	}
	for _, arg := range gotArgs {
		if arg == "--global" {
			t.Errorf("bd-shared adapter must not pass --global, got args %v", gotArgs)
		}
	}
}

// TestSharedDBAdapter_ListMemories_GlobalAddsFlag verifies the beads-global
// memories read passes --global (so bd routes to beads_global regardless of
// the anchor directory's own database), per the live-verified behavior that
// `bd --global memories --json` still requires an anchor directory with SOME
// resolvable .beads/ project (see bd_adapter.go doc comment).
func TestSharedDBAdapter_ListMemories_GlobalAddsFlag(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBeadsGlobal, Scope: "beads_global", DisplayName: "atlas"}
	anchor := t.TempDir()
	var gotDir string
	var gotArgs []string
	a := &sharedDBAdapter{
		origin:   origin,
		repoRoot: anchor,
		global:   true,
		runner:   recordingRunner(bdexec.Result{Stdout: `{"schema_version":1}`, ExitCode: 0}, &gotDir, &gotArgs),
	}

	got := a.ListMemories(context.Background())
	if got.Err != nil {
		t.Fatalf("ListMemories Err = %v, want nil", got.Err)
	}
	if len(got.Memories) != 0 {
		t.Errorf("Memories = %v, want empty (fixture is schema_version only)", got.Memories)
	}
	if gotDir != anchor {
		t.Errorf("runner dir = %q, want anchor %q", gotDir, anchor)
	}
	var sawGlobal bool
	for _, arg := range gotArgs {
		if arg == "--global" {
			sawGlobal = true
		}
	}
	if !sawGlobal {
		t.Errorf("beads-global adapter must pass --global, got args %v", gotArgs)
	}
}

// TestSharedDBAdapter_ListMemories_RunnerErrorIsUnavailable covers spec §8
// for both bd-shared and beads-global memories reads.
func TestSharedDBAdapter_ListMemories_RunnerErrorIsUnavailable(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBeadsGlobal, Scope: "beads_global"}
	a := &sharedDBAdapter{
		origin:   origin,
		repoRoot: t.TempDir(),
		global:   true,
		runner:   recordingRunner(bdexec.Result{Err: errors.New("no beads database found")}, nil, nil),
	}

	got := a.ListMemories(context.Background())
	if got.Err == nil {
		t.Fatal("ListMemories Err = nil, want non-nil")
	}
	if got.Origin != origin {
		t.Errorf("Origin = %+v, want %+v", got.Origin, origin)
	}
	if len(got.Memories) != 0 {
		t.Errorf("Memories = %v, want empty on failure", got.Memories)
	}
}
