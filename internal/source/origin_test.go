package source

import (
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// TestOriginSourceRepoIsLosslessSuperset is the no-regression contract (spec
// §4.1, §9): Origin is a purely ADDITIVE superset of today's SourceRepo. Its
// Scope carries the same db-name value SourceRepo does, so routing it back
// through an existing SourceRepo consumer (model.RepoKey) yields the identical
// key an unadorned SourceRepo would. New views read the richer Origin; old
// consumers keep reading SourceRepo unchanged.
func TestOriginSourceRepoIsLosslessSuperset(t *testing.T) {
	o := Origin{
		SourceKind:  SourceKindBDShared,
		Scope:       "bt",
		Prefix:      "bt",
		DisplayName: "bt",
	}
	if got := o.SourceRepo(); got != "bt" {
		t.Errorf("Origin.SourceRepo() = %q, want %q", got, "bt")
	}
	viaOrigin := model.RepoKey(model.Issue{SourceRepo: o.SourceRepo()})
	viaRaw := model.RepoKey(model.Issue{SourceRepo: "bt"})
	if viaOrigin != viaRaw {
		t.Errorf("RepoKey(via Origin)=%q, RepoKey(via raw SourceRepo)=%q; superset must be lossless", viaOrigin, viaRaw)
	}
}

// TestOriginExcluded checks the aggregation gate: gascity origins are excluded
// from bd views (spec §4.3 detect-and-exclude); bd origins are not.
func TestOriginExcluded(t *testing.T) {
	if !(Origin{SourceKind: SourceKindGasCity}).Excluded() {
		t.Error("gascity Origin.Excluded() = false, want true")
	}
	if (Origin{SourceKind: SourceKindBDEmbedded}).Excluded() {
		t.Error("bd-embedded Origin.Excluded() = true, want false")
	}
}
