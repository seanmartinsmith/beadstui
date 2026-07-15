// aggregate.go implements multi-source aggregation (bt-2ea7t.3, design spec
// §4.1, §5): given a set of already-selected Adapters, run a Payload's read
// on every source it supports and assemble the origin-tagged results. This
// is the "payload assembler" box in the spec's architecture diagram (§4.1) -
// the seam bt-2ea7t.4's memories view builds its render directly on.
package source

import "context"

// UnavailableSource pairs an Origin with the error that made it unreadable
// (design spec §8): a per-source read failure that must never fail the
// whole aggregation - the source is reported as "unavailable" and every
// other source still contributes.
type UnavailableSource struct {
	Origin Origin
	Err    error
}

// MemoriesAggregate is the result of running the memories payload across a
// set of sources (spec §4.1 "payload assembler", §5 data flow) - the seam
// bt-2ea7t.4's master/detail view is built on. There is deliberately no Err
// field: an aggregate where every source was empty is a legitimate, fully
// successful result (spec §8), not a degraded one.
type MemoriesAggregate struct {
	// Memories is every origin-tagged Memory read from a supporting,
	// reachable source. Nil/empty means every attempted source had zero
	// memories - a normal outcome, not an error (spec §8).
	Memories []Memory
	// Unavailable lists sources the payload attempted to read but could
	// not (bd missing, server unreachable, unparseable output) - spec §8's
	// per-source "unavailable" note. Non-fatal to the rest of the view.
	Unavailable []UnavailableSource
	// Excluded lists sources that were never queried because the payload
	// does not support their kind, or their Origin is Excluded() - in v1
	// this is exactly the gascity sources, feeding spec §4.3/§8's "N Gas
	// City sources hidden" note.
	Excluded []Origin
}

// AggregateMemories runs payload's memories read across every source in
// sources whose kind the payload Supports AND whose Origin is not
// Excluded() (design spec §4.1, §5). A source failing either check is
// recorded in Excluded and its ListMemories is NEVER called - this is how a
// gascity source is skipped by construction, not by a special-case branch
// (spec §10). A source whose read then fails is recorded in Unavailable
// rather than treated as fatal to the whole aggregation (spec §8). Sources
// with zero memories simply contribute nothing to Memories; an aggregate
// where every source was empty is a legitimate, error-free result.
func AggregateMemories(ctx context.Context, sources []Adapter, payload Payload) MemoriesAggregate {
	var agg MemoriesAggregate
	for _, a := range sources {
		origin := a.Origin()
		if origin.Excluded() || !payload.Supports(origin.SourceKind) {
			agg.Excluded = append(agg.Excluded, origin)
			continue
		}

		res := a.ListMemories(ctx)
		if res.Err != nil {
			agg.Unavailable = append(agg.Unavailable, UnavailableSource{Origin: res.Origin, Err: res.Err})
			continue
		}
		agg.Memories = append(agg.Memories, res.Memories...)
	}
	return agg
}

// SelectAdapter maps an Origin's SourceKind to the matching Adapter
// constructor (spec §4.1 "adapter select"). repoRoot is the source's own
// checkout directory, used by every kind's memories shell-out except
// gascity; dsn is the Dolt MySQL DSN for kinds backed by a live connection
// (bd-server, bd-shared, beads-global) and is ignored for bd-embedded and
// gascity.
func SelectAdapter(origin Origin, repoRoot, dsn string) Adapter {
	switch origin.SourceKind {
	case SourceKindBDEmbedded:
		return NewEmbeddedAdapter(origin, repoRoot)
	case SourceKindBDServer:
		return NewServerAdapter(origin, dsn, repoRoot)
	case SourceKindBDShared:
		return NewSharedAdapter(origin, dsn, repoRoot)
	case SourceKindBeadsGlobal:
		return NewGlobalAdapter(origin, dsn, repoRoot)
	default:
		// SourceKindGasCity, and defensively any future unrecognized kind:
		// the detect-only seam (gascity_adapter.go). AggregateMemories
		// never reaches this adapter's read methods for a
		// properly-classified gascity origin - Excluded() gates it first -
		// so this fallback exists only so SelectAdapter itself never
		// panics on an unexpected kind.
		return NewGasCityAdapter(origin)
	}
}
