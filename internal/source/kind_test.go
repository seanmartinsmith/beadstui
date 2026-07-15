package source

import "testing"

// TestSourceKindStringValues pins the wire strings for the SourceKind enum
// (design spec §4.1). These strings are the stable identity used by later
// origin-tagged payloads (bt-2ea7t.2/.3), so they must not drift.
func TestSourceKindStringValues(t *testing.T) {
	cases := []struct {
		kind SourceKind
		want string
	}{
		{SourceKindBDEmbedded, "bd-embedded"},
		{SourceKindBDServer, "bd-server"},
		{SourceKindBDShared, "bd-shared"},
		{SourceKindBeadsGlobal, "beads-global"},
		{SourceKindGasCity, "gascity"},
	}
	for _, tc := range cases {
		if string(tc.kind) != tc.want {
			t.Errorf("SourceKind = %q, want %q", string(tc.kind), tc.want)
		}
	}
}

// TestSourceKindIsBD verifies the four bd-managed kinds report IsBD, and the
// Gas City kind reports IsGasCity. The payload layer (spec §4.1) uses these to
// decide which sources a bd-only payload (memories) may read.
func TestSourceKindIsBD(t *testing.T) {
	bd := []SourceKind{
		SourceKindBDEmbedded, SourceKindBDServer,
		SourceKindBDShared, SourceKindBeadsGlobal,
	}
	for _, k := range bd {
		if !k.IsBD() {
			t.Errorf("%s.IsBD() = false, want true", k)
		}
		if k.IsGasCity() {
			t.Errorf("%s.IsGasCity() = true, want false", k)
		}
	}
	if SourceKindGasCity.IsBD() {
		t.Error("gascity.IsBD() = true, want false")
	}
	if !SourceKindGasCity.IsGasCity() {
		t.Error("gascity.IsGasCity() = false, want true")
	}
}
