// payload.go implements the payload abstraction (design spec §4.1): a
// payload declares which SourceKinds it may be read from, and the
// aggregator (aggregate.go) only invokes a payload's read on a source whose
// kind Supports returns true for. This is the mechanism that skips gascity
// for memories - the memories payload simply never declares gascity as a
// supported kind - rather than a special-case "if gascity" branch anywhere
// in the aggregation loop (spec §10, the design's core cleanliness claim).
package source

// Payload declares which SourceKinds a payload type may be read from. Every
// concrete payload (memories today; beads/graph later, per spec §11) is a
// stateless value implementing this - the aggregator gates every candidate
// source through Supports before ever calling a read method on its adapter.
type Payload interface {
	// Supports reports whether this payload can be read from a source of
	// the given kind.
	Supports(kind SourceKind) bool
}

// memoriesSupportedKinds is the exact set of SourceKinds the memories
// payload declares support for (design spec §4.1, §4.2): every bd-managed
// kind. gascity is deliberately absent from this set - see Payload's doc
// comment and package doc for why that absence is the skip mechanism, not a
// special case.
var memoriesSupportedKinds = map[SourceKind]bool{
	SourceKindBDEmbedded:  true,
	SourceKindBDServer:    true,
	SourceKindBDShared:    true,
	SourceKindBeadsGlobal: true,
}

// MemoriesPayload is the memories payload (spec §4.2): a stateless value
// whose Supports method is the seam AggregateMemories gates every candidate
// source through.
type MemoriesPayload struct{}

// Supports implements Payload for the memories payload: true for exactly
// the four bd-managed SourceKinds, false for gascity (and any future
// non-bd-managed kind).
func (MemoriesPayload) Supports(kind SourceKind) bool {
	return memoriesSupportedKinds[kind]
}

var _ Payload = MemoriesPayload{}
