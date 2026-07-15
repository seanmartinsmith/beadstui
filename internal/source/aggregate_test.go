package source

import (
	"context"
	"errors"
	"testing"
)

// fakeAdapter is a canned Adapter for aggregation tests (bt-2ea7t.3): it
// replays a fixed MemoriesResult and records whether ListMemories was
// invoked, so a test can assert a gascity (or otherwise unsupported)
// source's memories read is never called - the skip-by-construction
// guarantee spec §4.1/§10 requires. No live bd/Dolt server is involved
// anywhere in this file; every Adapter here is a fixture.
type fakeAdapter struct {
	origin         Origin
	memories       MemoriesResult
	memoriesCalled bool
}

func (f *fakeAdapter) Origin() Origin { return f.origin }

func (f *fakeAdapter) ListBeads(_ context.Context) BeadsResult {
	return BeadsResult{Origin: f.origin}
}

func (f *fakeAdapter) ListMemories(_ context.Context) MemoriesResult {
	f.memoriesCalled = true
	return f.memories
}

var _ Adapter = (*fakeAdapter)(nil)

// bdOrigin and gasCityOrigin build minimal fixture Origins for the tests
// below - real field values don't matter beyond SourceKind (which drives
// Payload.Supports / Excluded()) and Scope (used to tell records apart).
func bdOrigin(kind SourceKind, scope string) Origin {
	return Origin{SourceKind: kind, Scope: scope, Prefix: scope, DisplayName: scope}
}

func gasCityOrigin(scope string) Origin {
	return Origin{SourceKind: SourceKindGasCity, Scope: scope, Prefix: scope, DisplayName: scope}
}

// --- AggregateMemories ---

// TestAggregateMemories_MultiSourceOriginTagged is the primary acceptance
// test: aggregating across more than one bd source assembles a single
// []Memory slice with each record's Origin intact.
func TestAggregateMemories_MultiSourceOriginTagged(t *testing.T) {
	originA := bdOrigin(SourceKindBDEmbedded, "projA")
	originB := bdOrigin(SourceKindBeadsGlobal, "beads_global")

	a := &fakeAdapter{origin: originA, memories: MemoriesResult{Origin: originA, Memories: []Memory{
		{Key: "k1", Body: "b1", Origin: originA},
	}}}
	b := &fakeAdapter{origin: originB, memories: MemoriesResult{Origin: originB, Memories: []Memory{
		{Key: "k2", Body: "b2", Origin: originB},
		{Key: "k3", Body: "b3", Origin: originB},
	}}}

	agg := AggregateMemories(context.Background(), []Adapter{a, b}, MemoriesPayload{})

	if len(agg.Memories) != 3 {
		t.Fatalf("len(Memories) = %d, want 3", len(agg.Memories))
	}
	if len(agg.Unavailable) != 0 {
		t.Errorf("Unavailable = %+v, want empty", agg.Unavailable)
	}
	if len(agg.Excluded) != 0 {
		t.Errorf("Excluded = %+v, want empty", agg.Excluded)
	}

	var gotA, gotB int
	for _, m := range agg.Memories {
		switch m.Origin {
		case originA:
			gotA++
		case originB:
			gotB++
		default:
			t.Errorf("memory %q has unexpected Origin %+v", m.Key, m.Origin)
		}
	}
	if gotA != 1 || gotB != 2 {
		t.Errorf("gotA=%d gotB=%d, want 1 and 2", gotA, gotB)
	}
}

// TestAggregateMemories_GasCityNeverQueried is the acceptance-critical
// exclusion test (design spec §4.1, §10): a gascity source's ListMemories
// must never be invoked, and it must be surfaced in Excluded rather than
// silently dropped, so the view can render the "N Gas City sources hidden"
// note (spec §4.3, §8).
func TestAggregateMemories_GasCityNeverQueried(t *testing.T) {
	bdOriginV := bdOrigin(SourceKindBDServer, "projA")
	gcOrigin := gasCityOrigin("rigA")

	bd := &fakeAdapter{origin: bdOriginV, memories: MemoriesResult{Origin: bdOriginV, Memories: []Memory{
		{Key: "k1", Body: "b1", Origin: bdOriginV},
	}}}
	gc := &fakeAdapter{origin: gcOrigin, memories: MemoriesResult{Origin: gcOrigin, Memories: []Memory{
		{Key: "should-never-appear", Body: "x", Origin: gcOrigin},
	}}}

	agg := AggregateMemories(context.Background(), []Adapter{bd, gc}, MemoriesPayload{})

	if gc.memoriesCalled {
		t.Error("gascity adapter's ListMemories was called, want never called")
	}
	if len(agg.Memories) != 1 || agg.Memories[0].Key != "k1" {
		t.Fatalf("Memories = %+v, want exactly the bd source's one memory", agg.Memories)
	}
	if len(agg.Excluded) != 1 || agg.Excluded[0] != gcOrigin {
		t.Fatalf("Excluded = %+v, want [%+v]", agg.Excluded, gcOrigin)
	}
}

// TestAggregateMemories_UnavailableSourceSurfaced covers spec §8: a source
// whose read fails is reported in Unavailable, not treated as fatal to the
// whole aggregation - other sources still contribute.
func TestAggregateMemories_UnavailableSourceSurfaced(t *testing.T) {
	okOrigin := bdOrigin(SourceKindBDEmbedded, "projA")
	badOrigin := bdOrigin(SourceKindBDServer, "projB")

	ok := &fakeAdapter{origin: okOrigin, memories: MemoriesResult{Origin: okOrigin, Memories: []Memory{
		{Key: "k1", Body: "b1", Origin: okOrigin},
	}}}
	bad := &fakeAdapter{origin: badOrigin, memories: MemoriesResult{Origin: badOrigin, Err: errors.New("bd binary missing")}}

	agg := AggregateMemories(context.Background(), []Adapter{ok, bad}, MemoriesPayload{})

	if len(agg.Memories) != 1 {
		t.Fatalf("len(Memories) = %d, want 1 (only the healthy source)", len(agg.Memories))
	}
	if len(agg.Unavailable) != 1 {
		t.Fatalf("len(Unavailable) = %d, want 1", len(agg.Unavailable))
	}
	if agg.Unavailable[0].Origin != badOrigin {
		t.Errorf("Unavailable[0].Origin = %+v, want %+v", agg.Unavailable[0].Origin, badOrigin)
	}
	if agg.Unavailable[0].Err == nil {
		t.Error("Unavailable[0].Err = nil, want non-nil")
	}
}

// TestAggregateMemories_EmptySourceContributesNothing covers spec §8: a
// reachable source with zero memories contributes nothing and is reported
// as neither unavailable nor excluded - it is a normal, successful, empty
// read.
func TestAggregateMemories_EmptySourceContributesNothing(t *testing.T) {
	origin := bdOrigin(SourceKindBDShared, "projA")
	a := &fakeAdapter{origin: origin, memories: MemoriesResult{Origin: origin}}

	agg := AggregateMemories(context.Background(), []Adapter{a}, MemoriesPayload{})

	if len(agg.Memories) != 0 {
		t.Errorf("Memories = %+v, want empty", agg.Memories)
	}
	if len(agg.Unavailable) != 0 {
		t.Errorf("Unavailable = %+v, want empty (empty is not unavailable)", agg.Unavailable)
	}
	if len(agg.Excluded) != 0 {
		t.Errorf("Excluded = %+v, want empty", agg.Excluded)
	}
}

// TestAggregateMemories_AllEmptyYieldsEmptyAggregateNoError is the
// all-sources-empty acceptance case (spec §8): when every source has zero
// memories, the aggregate is legitimately empty. MemoriesAggregate carries
// no Err field at all - emptiness IS the success case, not a degraded one.
func TestAggregateMemories_AllEmptyYieldsEmptyAggregateNoError(t *testing.T) {
	o1 := bdOrigin(SourceKindBDEmbedded, "projA")
	o2 := bdOrigin(SourceKindBDShared, "projB")
	a := &fakeAdapter{origin: o1, memories: MemoriesResult{Origin: o1}}
	b := &fakeAdapter{origin: o2, memories: MemoriesResult{Origin: o2}}

	agg := AggregateMemories(context.Background(), []Adapter{a, b}, MemoriesPayload{})

	if len(agg.Memories) != 0 {
		t.Errorf("Memories = %+v, want empty", agg.Memories)
	}
	if len(agg.Unavailable) != 0 || len(agg.Excluded) != 0 {
		t.Errorf("Unavailable=%+v Excluded=%+v, want both empty", agg.Unavailable, agg.Excluded)
	}
}

// TestAggregateMemories_NoSources covers the degenerate case at the
// call-site boundary: an empty (or nil) source list still yields a clean,
// all-empty aggregate rather than a panic or an error.
func TestAggregateMemories_NoSources(t *testing.T) {
	agg := AggregateMemories(context.Background(), nil, MemoriesPayload{})
	if len(agg.Memories) != 0 || len(agg.Unavailable) != 0 || len(agg.Excluded) != 0 {
		t.Errorf("agg = %+v, want all-empty", agg)
	}
}

// --- SelectAdapter ---

// TestSelectAdapter_MapsKindToConstructor verifies the Origin.SourceKind ->
// Adapter constructor mapping (spec §4.1 "adapter select") picks the right
// concrete adapter type and preserves Origin unchanged.
func TestSelectAdapter_MapsKindToConstructor(t *testing.T) {
	repoRoot := t.TempDir()
	dsn := "root@tcp(127.0.0.1:1)/x"

	kinds := []SourceKind{
		SourceKindBDEmbedded,
		SourceKindBDServer,
		SourceKindBDShared,
		SourceKindBeadsGlobal,
		SourceKindGasCity,
	}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			origin := Origin{SourceKind: kind, Scope: "x"}
			got := SelectAdapter(origin, repoRoot, dsn)
			if got.Origin() != origin {
				t.Errorf("Origin() = %+v, want %+v", got.Origin(), origin)
			}
			switch kind {
			case SourceKindBDEmbedded:
				if _, ok := got.(*embeddedAdapter); !ok {
					t.Errorf("got %T, want *embeddedAdapter", got)
				}
			case SourceKindBDServer:
				if _, ok := got.(*serverAdapter); !ok {
					t.Errorf("got %T, want *serverAdapter", got)
				}
			case SourceKindBDShared, SourceKindBeadsGlobal:
				if _, ok := got.(*sharedDBAdapter); !ok {
					t.Errorf("got %T, want *sharedDBAdapter", got)
				}
			case SourceKindGasCity:
				if _, ok := got.(*gascityAdapter); !ok {
					t.Errorf("got %T, want *gascityAdapter", got)
				}
			}
		})
	}
}

// TestSelectAdapter_GlobalSetsGlobalFlag verifies beads-global routes
// through sharedDBAdapter with global=true (i.e. behaves like
// NewGlobalAdapter, not NewSharedAdapter) so its memories read passes
// --global (bd_adapter.go / bd_adapter_test.go's established contract).
func TestSelectAdapter_GlobalSetsGlobalFlag(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBeadsGlobal, Scope: "beads_global"}
	got := SelectAdapter(origin, t.TempDir(), "dsn")
	sd, ok := got.(*sharedDBAdapter)
	if !ok {
		t.Fatalf("got %T, want *sharedDBAdapter", got)
	}
	if !sd.global {
		t.Error("sharedDBAdapter.global = false, want true for beads-global")
	}
}

// TestSelectAdapter_SharedDoesNotSetGlobalFlag is the bd-shared counterpart:
// an ordinary shared-server database must NOT get the --global flag.
func TestSelectAdapter_SharedDoesNotSetGlobalFlag(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBDShared, Scope: "otherproj"}
	got := SelectAdapter(origin, t.TempDir(), "dsn")
	sd, ok := got.(*sharedDBAdapter)
	if !ok {
		t.Fatalf("got %T, want *sharedDBAdapter", got)
	}
	if sd.global {
		t.Error("sharedDBAdapter.global = true, want false for bd-shared")
	}
}
