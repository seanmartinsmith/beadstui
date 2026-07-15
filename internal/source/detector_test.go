package source

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeBeadsMeta writes a minimal .beads/metadata.json under dir, mirroring the
// fields bt's real readers key on (backend/dolt_mode/dolt_database).
func writeBeadsMeta(t *testing.T, dir, backend, doltMode, doltDB string) {
	t.Helper()
	beads := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beads, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := fmt.Sprintf(`{"backend":%q,"dolt_mode":%q,"dolt_database":%q}`, backend, doltMode, doltDB)
	if err := os.WriteFile(filepath.Join(beads, "metadata.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- Filesystem detector (PRIMARY entry point, spec §4.3) ---

// TestDetectPath_StandaloneEmbedded: a plain embedded bd project with no
// ancestor city.toml classifies bd-embedded, Scope = its db name.
func TestDetectPath_StandaloneEmbedded(t *testing.T) {
	dir := t.TempDir()
	writeBeadsMeta(t, dir, "dolt", "embedded", "myproj")

	got, ok := DetectPath(dir)
	if !ok {
		t.Fatal("DetectPath ok=false, want true")
	}
	if got.SourceKind != SourceKindBDEmbedded {
		t.Errorf("SourceKind = %s, want bd-embedded", got.SourceKind)
	}
	if got.Scope != "myproj" {
		t.Errorf("Scope = %q, want %q", got.Scope, "myproj")
	}
}

// TestDetectPath_StandaloneServer: dolt_mode=server classifies bd-server.
func TestDetectPath_StandaloneServer(t *testing.T) {
	dir := t.TempDir()
	writeBeadsMeta(t, dir, "dolt", "server", "myproj")

	got, ok := DetectPath(dir)
	if !ok {
		t.Fatal("DetectPath ok=false, want true")
	}
	if got.SourceKind != SourceKindBDServer {
		t.Errorf("SourceKind = %s, want bd-server", got.SourceKind)
	}
}

// TestDetectPath_GasCityRigUnderCityToml is the load-bearing gc case: a rig's
// .beads/ is indistinguishable from a standalone bd project in isolation, so
// the detector must walk UP to the nearest ancestor city.toml to know it is
// gc-managed (spec §4.3 primary). The rig here even declares embedded mode -
// the ancestor city.toml must still win.
func TestDetectPath_GasCityRigUnderCityToml(t *testing.T) {
	city := t.TempDir()
	if err := os.WriteFile(filepath.Join(city, "city.toml"), []byte("name = \"mycity\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rig := filepath.Join(city, "rigA")
	writeBeadsMeta(t, rig, "dolt", "embedded", "rigdb")

	got, ok := DetectPath(rig)
	if !ok {
		t.Fatal("DetectPath ok=false, want true")
	}
	if got.SourceKind != SourceKindGasCity {
		t.Errorf("SourceKind = %s, want gascity (ancestor city.toml must win)", got.SourceKind)
	}
	if !got.Excluded() {
		t.Error("gascity origin must report Excluded()")
	}
}

// TestDetectPath_BDProjectNamedHQNotExcluded encodes the bias against
// false-positive exclusion (spec §4.3): on the filesystem path only an ancestor
// city.toml excludes. A real bd project whose db is literally named "hq" but
// has NO ancestor city.toml must NOT be hidden - a false positive (hiding a
// real project) is worse than a false negative (a gc source leaking in).
func TestDetectPath_BDProjectNamedHQNotExcluded(t *testing.T) {
	dir := t.TempDir()
	writeBeadsMeta(t, dir, "dolt", "embedded", "hq")

	got, ok := DetectPath(dir)
	if !ok {
		t.Fatal("DetectPath ok=false, want true")
	}
	if got.SourceKind == SourceKindGasCity {
		t.Error("bd project named hq with no ancestor city.toml must NOT classify gascity")
	}
	if got.SourceKind != SourceKindBDEmbedded {
		t.Errorf("SourceKind = %s, want bd-embedded", got.SourceKind)
	}
}

// TestDetectPath_BeadsGlobalByName: the db-name signal recognizes beads_global
// (overriding dolt_mode), displayed via bt's atlas alias.
func TestDetectPath_BeadsGlobalByName(t *testing.T) {
	dir := t.TempDir()
	writeBeadsMeta(t, dir, "dolt", "server", "beads_global")

	got, ok := DetectPath(dir)
	if !ok {
		t.Fatal("DetectPath ok=false, want true")
	}
	if got.SourceKind != SourceKindBeadsGlobal {
		t.Errorf("SourceKind = %s, want beads-global", got.SourceKind)
	}
	if got.DisplayName != "atlas" {
		t.Errorf("DisplayName = %q, want %q (atlas alias)", got.DisplayName, "atlas")
	}
}

// TestDetectPath_NoSource: a directory with neither city.toml nor a .beads
// metadata is not a source.
func TestDetectPath_NoSource(t *testing.T) {
	dir := t.TempDir()
	if _, ok := DetectPath(dir); ok {
		t.Error("DetectPath ok=true for an empty dir, want false")
	}
}

// --- DB-enumeration detector (DEFENSIVE guard, spec §4.3) ---

// TestDetectSharedDB covers the cheap hq-name guard and its false-positive
// bias. Only the exact db name "hq" fires (the Gas City city-HQ database by
// contract); hq-prefixed real projects and differently-cased names are NOT
// excluded, so a real bd project is never hidden by a lone weak signal.
func TestDetectSharedDB(t *testing.T) {
	cases := []struct {
		db   string
		want SourceKind
	}{
		{"hq", SourceKindGasCity},               // defensive guard fires
		{"beads_global", SourceKindBeadsGlobal}, // enumerated global DB
		{"myproject", SourceKindBDShared},       // ordinary shared db
		{"hquarters", SourceKindBDShared},       // exact-match only - not a prefix guard
		{"HQ", SourceKindBDShared},              // case-sensitive - gc names it lowercase
	}
	for _, tc := range cases {
		got := DetectSharedDB(tc.db)
		if got.SourceKind != tc.want {
			t.Errorf("DetectSharedDB(%q).SourceKind = %s, want %s", tc.db, got.SourceKind, tc.want)
		}
		if got.Scope != tc.db {
			t.Errorf("DetectSharedDB(%q).Scope = %q, want %q", tc.db, got.Scope, tc.db)
		}
	}
}

// TestDetectSharedDB_GasCityExcluded: the hq guard's classification feeds the
// aggregation gate.
func TestDetectSharedDB_GasCityExcluded(t *testing.T) {
	if !DetectSharedDB("hq").Excluded() {
		t.Error("DetectSharedDB(\"hq\").Excluded() = false, want true")
	}
	if DetectSharedDB("myproject").Excluded() {
		t.Error("DetectSharedDB(\"myproject\").Excluded() = true, want false")
	}
}
