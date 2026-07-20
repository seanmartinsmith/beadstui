package source

import "github.com/seanmartinsmith/beadstui/pkg/model"

// Origin tags a record with where it came from. It is a purely ADDITIVE
// superset of today's issue.SourceRepo (a bare Dolt database name): Scope
// carries exactly that db-name value, and SourceRepo() returns it, so existing
// SourceRepo consumers keep working unchanged while new views read the richer
// Origin (spec §4.1). Nothing here is wired into model.Issue - the superset is
// assembled at the payload layer, keeping pkg/model a leaf with no regression.
type Origin struct {
	// SourceKind classifies the store shape / read path.
	SourceKind SourceKind
	// Scope is the source's db (or Gas City rig) name - the same value that
	// issue.SourceRepo holds today. This is the separation key aggregation
	// groups by.
	Scope string
	// Prefix is the source's identity for bead-ID namespacing. At the
	// resolver layer this is the db/rig name (== Scope); true per-bead prefix
	// resolution (e.g. db "marketplace" -> prefix "mkt") is deferred to the
	// adapter layer (bt-2ea7t.2), which reads actual beads.
	Prefix string
	// DisplayName is the human-facing label (e.g. "atlas" for beads_global, a
	// city name for a gascity rig), for grouping headers in views.
	DisplayName string
}

// SourceRepo returns the value existing SourceRepo consumers expect - the
// source's db name. This is the bridge that makes Origin a lossless superset:
// model.RepoKey(model.Issue{SourceRepo: o.SourceRepo()}) equals the key an
// unadorned SourceRepo would produce.
func (o Origin) SourceRepo() string { return o.Scope }

// Label returns a guaranteed non-empty, human-facing label for the origin:
// DisplayName when set, else Scope, else a fixed placeholder. Every Origin
// built by DetectPath/DetectSharedDB (via newOrigin) already has a non-empty
// DisplayName - dbName is never "" (readBeadsMeta defaults an unset
// dolt_database to "beads") and model.DisplayRepoName only maps a non-empty
// key onto another non-empty string - so the Scope/placeholder tiers exist
// as a defensive backstop for any Origin NOT built through that path (a
// hand-constructed literal, or a future adapter that forgets to set
// DisplayName), rather than a case reachable through today's detectors.
//
// View code (pkg/ui/memories.go group headers, unavailable-source notes)
// must call this instead of reading DisplayName directly (bt-2ea7t.6): a
// blank group header ("" + " (N)") is silently unreadable, so the guarantee
// belongs here, next to Origin, rather than duplicated ad hoc per call site.
func (o Origin) Label() string {
	if o.DisplayName != "" {
		return o.DisplayName
	}
	if o.Scope != "" {
		return o.Scope
	}
	return "unknown source"
}

// Excluded reports whether this origin must be kept out of bd-centric
// aggregation. In v1 that is exactly the Gas City sources (detect-and-exclude,
// spec §4.3); they are remembered for the later gc lens, not discarded.
func (o Origin) Excluded() bool { return o.SourceKind.IsGasCity() }

// newOrigin builds an Origin for a bd source identified by its db name. Prefix
// defaults to the db name (see Origin.Prefix); DisplayName applies bt's
// existing atlas alias for beads_global via model.DisplayRepoName.
func newOrigin(kind SourceKind, dbName string) Origin {
	return Origin{
		SourceKind:  kind,
		Scope:       dbName,
		Prefix:      dbName,
		DisplayName: model.DisplayRepoName(dbName),
	}
}
