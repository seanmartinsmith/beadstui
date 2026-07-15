package source

import (
	"context"
	"errors"
)

// ErrGasCityNotImplemented is returned by gascityAdapter's read methods.
// gascity is detect-only in v1 (design spec §4.3): a source is recognized
// and excluded from bd-centric aggregation via Origin.Excluded(), never
// routed through these read methods. This adapter exists purely as a
// designed seam for the later gc lens (spec §11 "Next" stage) to implement
// into - v1 callers must gate on Excluded() before ever reaching it.
var ErrGasCityNotImplemented = errors.New("gascity adapter is detect-only in v1 (own lens ships later); not implemented")

// gascityAdapter is the not-implemented Adapter for a Gas City-managed
// source. Both read methods return ErrGasCityNotImplemented with Origin
// still populated - never a panic - so a caller that (incorrectly) invokes
// it without checking Excluded() first gets a clear, typed refusal rather
// than undefined behavior.
type gascityAdapter struct {
	origin Origin
}

// NewGasCityAdapter builds the detect-only gascity Adapter. Payload
// assembly (bt-2ea7t.3) must skip sources where Origin().Excluded() is true
// rather than ever calling ListBeads/ListMemories on this adapter -
// exclusion happens by construction (§4.3), not by these methods erroring.
func NewGasCityAdapter(origin Origin) Adapter {
	return &gascityAdapter{origin: origin}
}

func (a *gascityAdapter) Origin() Origin { return a.origin }

func (a *gascityAdapter) ListBeads(_ context.Context) BeadsResult {
	return BeadsResult{Origin: a.origin, Err: ErrGasCityNotImplemented}
}

func (a *gascityAdapter) ListMemories(_ context.Context) MemoriesResult {
	return MemoriesResult{Origin: a.origin, Err: ErrGasCityNotImplemented}
}

var _ Adapter = (*gascityAdapter)(nil)
