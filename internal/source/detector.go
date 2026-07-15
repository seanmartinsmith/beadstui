package source

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	// cityTOMLName is the Gas City marker file at a city root. Its presence as
	// an ancestor of a .beads/ rig is the primary gc signal (spec §4.3).
	cityTOMLName = "city.toml"
	// beadsGlobalDBName is the github-synced cross-cutting global DB. Matches
	// internal/bdroute.BeadsGlobalDB and model.IsAtlasNamespace's raw spelling;
	// kept local to avoid coupling the classifier to the write-routing package.
	beadsGlobalDBName = "beads_global"
	// gasCityHQDBName is the Gas City city-HQ database name (one per city, by
	// gc contract). The defensive DB-enum guard matches it EXACTLY - not as a
	// prefix, not case-folded - so a real bd project named "hquarters" or "HQ"
	// is never hidden (spec §4.3: bias against false-positive exclusion).
	gasCityHQDBName = "hq"
)

// DetectPath classifies the bead source rooted at dir using the filesystem
// (PRIMARY) entry point - decidable by os.Stat alone, no DB connection. This
// is how bt actually meets a source: cwd inside a project, or a path stamped
// into ~/.bt/projects.json. Returns ok=false when dir is neither Gas
// City-managed nor a bd project.
//
// Order is load-bearing (spec §4.3): a gascity rig's own .beads/ is
// indistinguishable from a standalone bd project, so the ancestor city.toml
// walk-up runs FIRST and wins over the .beads/ dolt_mode classification.
func DetectPath(dir string) (Origin, bool) {
	// PRIMARY: nearest ancestor city.toml => Gas City. This is the only
	// filesystem signal that excludes; a bd project is never guessed to be gc
	// from its db name or prefix alone (bias against false-positive exclusion).
	if cityRoot, ok := findCityRoot(dir); ok {
		scope := filepath.Base(dir)
		if dbName, ok := readBeadsDBName(dir); ok {
			scope = dbName // the rig's own db name is a more specific scope
		}
		return Origin{
			SourceKind:  SourceKindGasCity,
			Scope:       scope,
			Prefix:      scope,
			DisplayName: filepath.Base(cityRoot),
		}, true
	}

	// Otherwise classify a bd source by its .beads/metadata.json.
	dbName, mode, ok := readBeadsMeta(dir)
	if !ok {
		return Origin{}, false
	}
	return newOrigin(classifyBDName(dbName, mode), dbName), true
}

// DetectSharedDB classifies an enumerated database name on a shared Dolt server
// using the DEFENSIVE (DB-enum) entry point - a cheap in-store guard with no
// filesystem context (spec §4.3). This path is near-dead in practice: bt only
// enumerates its OWN shared server, while a Gas City city runs a separate
// per-city Dolt server bt never connects to. It exists as a backstop for the
// unusual case of a gc rig deliberately `bd init --shared-server`'d onto bt's
// server. It is intentionally a bare hq-name guard, not co-equal gc.*-metadata
// machinery.
func DetectSharedDB(dbName string) Origin {
	return newOrigin(classifyDBName(dbName), dbName)
}

// classifyDBName maps an enumerated (or discovered) db name to a SourceKind
// with no filesystem context. Only the exact name "hq" trips the gc guard.
func classifyDBName(dbName string) SourceKind {
	switch dbName {
	case gasCityHQDBName:
		return SourceKindGasCity
	case beadsGlobalDBName:
		return SourceKindBeadsGlobal
	default:
		return SourceKindBDShared
	}
}

// classifyBDName maps a bd source's db name + dolt_mode to a SourceKind. The
// beads_global db name is recognized by name and overrides the mode; otherwise
// server vs embedded follows dolt_mode (embedded is the beads v1.0 default, so
// anything not explicitly "server" is treated as embedded).
func classifyBDName(dbName, doltMode string) SourceKind {
	if dbName == beadsGlobalDBName {
		return SourceKindBeadsGlobal
	}
	if doltMode == "server" {
		return SourceKindBDServer
	}
	return SourceKindBDEmbedded
}

// findCityRoot walks up from dir (inclusive) to the nearest ancestor directory
// containing a city.toml, returning that directory and true. Pure os.Stat; no
// DB access. Stops at the filesystem root.
func findCityRoot(dir string) (string, bool) {
	cur := dir
	for {
		if info, err := os.Stat(filepath.Join(cur, cityTOMLName)); err == nil && !info.IsDir() {
			return cur, true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", false // reached the root
		}
		cur = parent
	}
}

// beadsMeta is the subset of .beads/metadata.json the detector needs.
type beadsMeta struct {
	Backend      string `json:"backend"`
	DoltMode     string `json:"dolt_mode"`
	DoltDatabase string `json:"dolt_database"`
}

// readBeadsMeta reads <dir>/.beads/metadata.json and returns the Dolt database
// name and dolt_mode for a Dolt-backed project. Returns ok=false when the
// metadata is missing, unparseable, or not a Dolt backend. The db name
// defaults to "beads" when unset, matching bt's other metadata readers.
func readBeadsMeta(dir string) (dbName, doltMode string, ok bool) {
	data, err := os.ReadFile(filepath.Join(dir, ".beads", "metadata.json"))
	if err != nil {
		return "", "", false
	}
	var m beadsMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return "", "", false
	}
	if m.Backend != "dolt" {
		return "", "", false
	}
	db := m.DoltDatabase
	if db == "" {
		db = "beads"
	}
	return db, m.DoltMode, true
}

// readBeadsDBName is readBeadsMeta narrowed to just the db name, for tagging a
// gascity rig's Scope with its own database name.
func readBeadsDBName(dir string) (string, bool) {
	db, _, ok := readBeadsMeta(dir)
	return db, ok
}
