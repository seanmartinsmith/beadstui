// Package source is bt's cross-project read layer: it classifies candidate
// bead sources into a SourceKind, tags every record with a rich Origin, and
// selects a read adapter. It is new construction (bt has no unified scope
// object today - launch mode is implicit flag precedence and SourceRepo is a
// bare db name), but it wraps bt's existing readers rather than replacing them.
//
// This file (bt-2ea7t.1) is the foundation: the SourceKind enum and the
// detector's classification primitives. Adapters (bt-2ea7t.2), the memories
// payload (bt-2ea7t.3), and the view (bt-2ea7t.4) ride on top. See
// docs/design/2026-07-15-cross-project-read-layer-and-memories.md (§4.1, §4.3).
package source

// SourceKind classifies a bead source by its store shape and read path
// (design spec §3 state space, §4.1). Four kinds are bd-managed (bt reads
// them today); gascity is Gas City-managed and, in v1, detect-and-excluded
// from bd views rather than read (§4.3).
type SourceKind string

const (
	// SourceKindBDEmbedded is an embedded (in-process) Dolt project - the
	// beads v1.0 default (dolt_mode:embedded, no server), read by shelling
	// `bd export` (never attaching a server, which would deadlock bd - bt-qrt2u).
	SourceKindBDEmbedded SourceKind = "bd-embedded"
	// SourceKindBDServer is a per-project Dolt SQL server (dolt_mode:server).
	SourceKindBDServer SourceKind = "bd-server"
	// SourceKindBDShared is one database on the shared Dolt server that hosts
	// many project databases (~/.beads/shared-server :3308).
	SourceKindBDShared SourceKind = "bd-shared"
	// SourceKindBeadsGlobal is the github-synced cross-cutting global DB
	// (upstream `bd --global`, db name "beads_global", displayed "atlas").
	SourceKindBeadsGlobal SourceKind = "beads-global"
	// SourceKindGasCity is a Gas City-managed source (per-city Dolt server,
	// one database per scope). bt has no memories to read here and, in v1,
	// detects-and-excludes it from bd views (§4.3). Its own lens comes later.
	SourceKindGasCity SourceKind = "gascity"
)

// IsBD reports whether the kind is bd-managed (embedded, per-project server,
// shared-server, or beads_global) - i.e. a source bt reads directly. The
// payload layer uses this to decide which sources a bd-only payload may read.
func (k SourceKind) IsBD() bool {
	switch k {
	case SourceKindBDEmbedded, SourceKindBDServer, SourceKindBDShared, SourceKindBeadsGlobal:
		return true
	default:
		return false
	}
}

// IsGasCity reports whether the kind is Gas City-managed (detect-and-excluded
// from bd views in v1).
func (k SourceKind) IsGasCity() bool {
	return k == SourceKindGasCity
}
