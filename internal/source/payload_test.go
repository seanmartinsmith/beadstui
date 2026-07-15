package source

import "testing"

// TestMemoriesPayload_Supports is the acceptance-criterion test for the
// memories payload's declared support set (design spec §4.1, §4.2): exactly
// the four bd-managed SourceKinds. gascity is absent from the declaration
// itself - Supports returning false for it IS the skip-by-construction
// mechanism (spec §10), not a special-case "if gascity" branch anywhere in
// the aggregator.
func TestMemoriesPayload_Supports(t *testing.T) {
	p := MemoriesPayload{}
	cases := []struct {
		kind SourceKind
		want bool
	}{
		{SourceKindBDEmbedded, true},
		{SourceKindBDServer, true},
		{SourceKindBDShared, true},
		{SourceKindBeadsGlobal, true},
		{SourceKindGasCity, false},
	}
	for _, tc := range cases {
		if got := p.Supports(tc.kind); got != tc.want {
			t.Errorf("Supports(%s) = %v, want %v", tc.kind, got, tc.want)
		}
	}
}

// TestMemoriesPayload_ImplementsPayload is a compile-time-style guard kept
// as a runtime test so a future refactor that breaks the interface fails
// loudly here rather than only at a call site.
func TestMemoriesPayload_ImplementsPayload(t *testing.T) {
	var p Payload = MemoriesPayload{}
	if p == nil {
		t.Fatal("MemoriesPayload{} must satisfy Payload")
	}
}
