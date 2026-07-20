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

// TestOriginLabel covers bt-2ea7t.6: Origin.Label() must never return "" so
// a view's group header/note can never render a blank ("" + " (N)")
// caption. DisplayName wins when set; Scope is the fallback for an Origin
// missing DisplayName; a fixed placeholder is the last resort for an Origin
// missing both (not reachable through DetectPath/DetectSharedDB today, but
// possible for a hand-built literal or a future adapter).
func TestOriginLabel(t *testing.T) {
	tests := []struct {
		name string
		o    Origin
		want string
	}{
		{"display name set", Origin{DisplayName: "atlas", Scope: "beads_global"}, "atlas"},
		{"empty display name falls back to scope", Origin{DisplayName: "", Scope: "dotfiles"}, "dotfiles"},
		{"both empty falls back to placeholder", Origin{}, "unknown source"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.o.Label(); got != tt.want {
				t.Errorf("Origin.Label() = %q, want %q", got, tt.want)
			}
		})
	}
}
