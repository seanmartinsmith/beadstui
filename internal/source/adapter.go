package source

import (
	"context"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// Adapter is bt's per-source read interface (design spec §4.1): one
// implementation per SourceKind, behind a uniform surface the resolver and
// payload layer consume without caring how a given source stores its data.
// bd adapters (embedded, per-project server, shared-server, beads_global)
// wrap bt's existing internal/datasource readers for beads and add the
// memories read (`bd memories --json`) bt could not see before bt-2ea7t.
// The gascity adapter is detect-only in v1 - see gascity_adapter.go.
//
// Every read degrades a source it cannot reach to an "unavailable" result
// (a non-nil Err on BeadsResult/MemoriesResult) rather than returning a Go
// error from the method itself - a source-level failure must never fail the
// whole view (spec §8). Callers that want a hard failure on error should
// check Result.Err themselves; nothing here panics or aborts on a single
// bad source.
type Adapter interface {
	// Origin returns the Origin every record this adapter produces is
	// tagged with.
	Origin() Origin
	// ListBeads reads this source's beads, wrapping the existing
	// internal/datasource reader for the source's SourceKind.
	ListBeads(ctx context.Context) BeadsResult
	// ListMemories runs `bd memories --json` for this source and parses the
	// flat key->body map (ParseMemoriesJSON), skipping the schema_version
	// sibling.
	ListMemories(ctx context.Context) MemoriesResult
}

// BeadsResult is one source's beads-read outcome.
type BeadsResult struct {
	// Origin is the source these issues (or this failure) came from.
	Origin Origin
	// Issues is this source's beads. Empty when Err is non-nil.
	Issues []model.Issue
	// Err is non-nil when the source could not be read (server unreachable,
	// `bd export` failed, etc). A non-nil Err marks this source
	// "unavailable" (spec §8) - it is not a fatal error for the whole view.
	Err error
}

// MemoriesResult is one source's memories-read outcome. Err follows the same
// "unavailable, not fatal" contract as BeadsResult.Err.
type MemoriesResult struct {
	// Origin is the source these memories (or this failure) came from.
	Origin Origin
	// Memories is this source's memories. Empty when Err is non-nil, and
	// also legitimately empty when the source simply has none (spec §8:
	// "Empty memories: a bd source with zero memories contributes nothing").
	Memories []Memory
	// Err is non-nil when the source could not be read (bd binary missing,
	// source unreachable, unparseable output).
	Err error
}
