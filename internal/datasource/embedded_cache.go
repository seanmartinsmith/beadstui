package datasource

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// Parsed-snapshot cache for the embedded (in-process Dolt) read path
// (bt-xdah0). Every embedded robot call otherwise pays a full `bd export`
// (~0.4s per 1300 issues) even when nothing changed. Dolt rewrites its
// storage manifest file on every commit (see EmbeddedManifestPath), so its
// content hash is a precise "did anything change" key: unchanged hash means
// the parsed issue snapshot from the last export is still exactly correct,
// so the export can be skipped entirely.
//
// Boundary (bt-p34aw ruling, 2026-07-06): this cache is a derived, read-side
// artifact under bt's OWN .bt/ directory -- never under .beads/. Precedent:
// .bt/semantic (the .bvvi vector index) and .bt/baseline.json already live
// there. Nothing here attaches to or persists inside the embedded Dolt data
// dir, so bt-qrt2u's never-persist boundary (no server owning embedded data)
// is untouched.
//
// Format: a small fixed header (magic + version + reserved), mirroring the
// .bvvi precedent in pkg/search/vector_index.go, so a future schema change
// invalidates cleanly on version mismatch rather than corrupting a decode.
// The payload itself is gob-encoded (encoding/gob, stdlib): model.Issue has
// ~30 fields including pointers, slices of pointers, and a map of
// json.RawMessage, and gob round-trips that struct graph exactly with no
// per-field wire code, unlike the hand-rolled binary layout .bvvi uses for
// the much simpler fixed-shape VectorEntry. JSON was the other option
// considered; gob was chosen for smaller/faster encode-decode on an
// internal-only cache nobody ever reads by hand.
//
// Concurrency: writes go through a unique temp file (os.CreateTemp) and an
// atomic rename, matching pkg/search/vector_index.go's Save. A reader either
// sees the old complete file, or the new complete file -- never a torn one.
// Concurrent cache misses race to write the same content-addressed data, so
// the worst case is a redundant export, never corruption.
const (
	embeddedSnapshotMagic   = "BTSC" // "BT Snapshot Cache"
	embeddedSnapshotVersion = uint16(1)
)

// embeddedCacheKey identifies exactly one cached snapshot: the embedded
// project's Dolt database name (defense-in-depth against path collisions)
// plus the manifest content hash (the actual invalidation key).
type embeddedCacheKey struct {
	dbName       string
	manifestHash [32]byte
}

// computeEmbeddedCacheKey derives the cache key for the embedded project
// rooted at beadsDir's parent. Returns ok=false when there is nothing stable
// to key on (not embedded, or the manifest doesn't exist yet -- e.g. a brand
// new project before its first commit) -- callers should skip caching
// entirely in that case rather than keying on a placeholder.
func computeEmbeddedCacheKey(beadsDir string) (embeddedCacheKey, bool) {
	dbName, ok := ReadEmbeddedConfig(beadsDir)
	if !ok {
		return embeddedCacheKey{}, false
	}
	manifestPath := EmbeddedManifestPath(beadsDir)
	if manifestPath == "" {
		return embeddedCacheKey{}, false
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return embeddedCacheKey{}, false
	}
	return embeddedCacheKey{dbName: dbName, manifestHash: sha256.Sum256(data)}, true
}

// embeddedSnapshotCachePath returns the per-DB cache file path under the
// project's own .bt/ directory. repoRoot is the project root (parent of
// .beads), matching EmbeddedReader.repoRoot / DataSource.Path.
func embeddedSnapshotCachePath(repoRoot, dbName string) string {
	return filepath.Join(repoRoot, ".bt", "snapshot-cache", "embedded-"+sanitizeCacheDBName(dbName)+".snap")
}

// sanitizeCacheDBName restricts a Dolt database name to filesystem-safe
// characters before it becomes part of a cache file path. Dolt database
// names are ordinarily already alphanumeric/underscore, but this is cheap
// defense against an unusual name producing a path traversal or invalid
// filename.
func sanitizeCacheDBName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "default"
	}
	return b.String()
}

// readEmbeddedSnapshot loads the cached issue snapshot at path, returning
// (issues, true) only when the file exists, has a matching header, and its
// stored key exactly matches want (db name AND manifest hash). Any mismatch
// or read/decode error is treated as a cache miss -- never as a hard error --
// so a corrupt, foreign, or stale cache file always falls back to a fresh
// export rather than surfacing wrong data or failing the load.
func readEmbeddedSnapshot(path string, want embeddedCacheKey) ([]model.Issue, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()

	r := bufio.NewReader(f)

	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil || string(magic[:]) != embeddedSnapshotMagic {
		return nil, false
	}

	var version uint16
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil || version != embeddedSnapshotVersion {
		return nil, false
	}

	var reserved uint16
	if err := binary.Read(r, binary.LittleEndian, &reserved); err != nil {
		return nil, false
	}

	var hash [32]byte
	if _, err := io.ReadFull(r, hash[:]); err != nil {
		return nil, false
	}

	var dbNameLen uint16
	if err := binary.Read(r, binary.LittleEndian, &dbNameLen); err != nil {
		return nil, false
	}
	dbNameBytes := make([]byte, dbNameLen)
	if _, err := io.ReadFull(r, dbNameBytes); err != nil {
		return nil, false
	}

	if hash != want.manifestHash || string(dbNameBytes) != want.dbName {
		// Stale (hash changed) or a foreign cache file for a different DB.
		// Both are ordinary cache misses.
		return nil, false
	}

	var issues []model.Issue
	if err := gob.NewDecoder(r).Decode(&issues); err != nil {
		return nil, false
	}
	return issues, true
}

// writeEmbeddedSnapshot atomically writes issues to path under key: encode
// into a unique temp file in the same directory, then rename over the
// destination. Rename is atomic on the same filesystem, so a concurrent
// reader never observes a partially-written file.
func writeEmbeddedSnapshot(path string, key embeddedCacheKey, issues []model.Issue) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating snapshot cache dir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "btsc-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp snapshot cache file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	w := bufio.NewWriter(tmp)

	if _, err := w.WriteString(embeddedSnapshotMagic); err != nil {
		return fmt.Errorf("write magic: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, embeddedSnapshotVersion); err != nil {
		return fmt.Errorf("write version: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(0)); err != nil {
		return fmt.Errorf("write reserved: %w", err)
	}
	if _, err := w.Write(key.manifestHash[:]); err != nil {
		return fmt.Errorf("write manifest hash: %w", err)
	}
	if len(key.dbName) > 0xFFFF {
		return fmt.Errorf("db name too long: %d bytes", len(key.dbName))
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(len(key.dbName))); err != nil {
		return fmt.Errorf("write db name len: %w", err)
	}
	if _, err := w.WriteString(key.dbName); err != nil {
		return fmt.Errorf("write db name: %w", err)
	}

	if err := gob.NewEncoder(w).Encode(issues); err != nil {
		return fmt.Errorf("gob-encode snapshot: %w", err)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush snapshot cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp snapshot cache file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		// os.Rename doesn't replace existing files on Windows (mirrors the
		// same fallback in pkg/search/vector_index.go's VectorIndex.Save).
		// The cache is fully derivable from a re-export, so removing the
		// stale destination and retrying is safe.
		if runtime.GOOS == "windows" {
			if _, statErr := os.Stat(path); statErr == nil {
				if rmErr := os.Remove(path); rmErr != nil {
					return fmt.Errorf("remove existing snapshot cache: %w", rmErr)
				}
				if err2 := os.Rename(tmpPath, path); err2 != nil {
					return fmt.Errorf("rename snapshot cache: %w", err2)
				}
				return nil
			}
		}
		return fmt.Errorf("rename snapshot cache: %w", err)
	}
	return nil
}
