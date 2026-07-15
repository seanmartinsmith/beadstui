package source

import (
	"context"
	"errors"
	"testing"
)

// TestGasCityAdapter_DetectOnly guards the v1 contract (spec §4.3): gascity
// has a designed Adapter seam but no real implementation - both read methods
// return ErrGasCityNotImplemented with the Origin still populated (never a
// panic). Callers must gate on Origin().Excluded() rather than ever reaching
// these methods; this test exists so a future gc-lens implementer has a
// pinned "here's the seam, here's what it does today" reference.
func TestGasCityAdapter_DetectOnly(t *testing.T) {
	origin := Origin{SourceKind: SourceKindGasCity, Scope: "rigA", DisplayName: "mycity"}
	a := NewGasCityAdapter(origin)

	if got := a.Origin(); got != origin {
		t.Errorf("Origin() = %+v, want %+v", got, origin)
	}
	if !a.Origin().Excluded() {
		t.Fatal("gascity Origin().Excluded() = false, want true (the real aggregation gate)")
	}

	beads := a.ListBeads(context.Background())
	if !errors.Is(beads.Err, ErrGasCityNotImplemented) {
		t.Errorf("ListBeads Err = %v, want ErrGasCityNotImplemented", beads.Err)
	}
	if beads.Origin != origin {
		t.Errorf("ListBeads Origin = %+v, want %+v", beads.Origin, origin)
	}
	if len(beads.Issues) != 0 {
		t.Errorf("ListBeads Issues = %v, want empty", beads.Issues)
	}

	memories := a.ListMemories(context.Background())
	if !errors.Is(memories.Err, ErrGasCityNotImplemented) {
		t.Errorf("ListMemories Err = %v, want ErrGasCityNotImplemented", memories.Err)
	}
	if memories.Origin != origin {
		t.Errorf("ListMemories Origin = %+v, want %+v", memories.Origin, origin)
	}
	if len(memories.Memories) != 0 {
		t.Errorf("ListMemories Memories = %v, want empty", memories.Memories)
	}
}
