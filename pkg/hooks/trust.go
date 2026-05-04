// Trust gate for .bt/hooks.yaml execution.
//
// Hook commands flow through `sh -c` / `cmd /C`, which makes them a
// git-config-style RCE vector when a malicious project ships a hostile
// hooks.yaml. To prevent execution-on-clone, bt requires that each hooks.yaml
// be explicitly trusted by absolute path AND content hash before any hook will
// run. Trust binds the tuple (absolute path, sha256(content)). Editing the
// file or moving the project resets trust.
//
// The trust DB lives at ~/.bt/hook-trust.json with mode 0600 on POSIX and
// the default user-profile ACL on Windows.
//
// Known limitation: trust covers hooks.yaml content only. If a hook command
// references an external script (e.g., command: "./scripts/build.sh"), the
// script's contents are NOT covered by the hash and can be replaced after
// trust is granted. Reviewing referenced scripts is the user's responsibility.
// See SECURITY.md.
package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// trustDBVersion is the on-disk schema version. Bump when the schema changes.
const trustDBVersion = 1

// TrustDB is the on-disk schema for ~/.bt/hook-trust.json.
type TrustDB struct {
	Version int                      `json:"version"`
	Trusted map[string]TrustedRecord `json:"trusted"`
}

// TrustedRecord is one trust-grant entry. The map key is the absolute path of
// the hooks.yaml file. SHA256 is hex-encoded sha256 of the file contents at
// the moment trust was granted.
type TrustedRecord struct {
	SHA256    string    `json:"sha256"`
	TrustedAt time.Time `json:"trusted_at"`
}

// UntrustedHooksError is returned when execution is gated on trust and trust
// is missing or stale. Callers (cmd/bt) catch this type and exit 78.
type UntrustedHooksError struct {
	Path string // absolute path to the offending hooks.yaml
	Hash string // current sha256 of the file contents (hex)
}

func (e *UntrustedHooksError) Error() string {
	return fmt.Sprintf("refused to run untrusted hooks at %s\nsha256: %s\nto trust: bt hooks trust %s\nto bypass once: pass --allow-hooks", e.Path, e.Hash, e.Path)
}

// trustDBPath returns the canonical path to the trust DB file. It is a
// package-level var so tests can swap it for a temp path.
var trustDBPath = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home dir: %w", err)
	}
	return filepath.Join(home, ".bt", "hook-trust.json"), nil
}

// TrustDBPath returns the current trust DB location. Exposed for diagnostics.
func TrustDBPath() (string, error) {
	return trustDBPath()
}

// HashHooksFile computes the hex sha256 of the hooks.yaml at path.
func HashHooksFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// LoadTrustDB reads the trust DB from disk. Returns an empty (but valid) DB
// if the file does not exist; only IO/parse failures produce an error.
func LoadTrustDB() (*TrustDB, error) {
	path, err := trustDBPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &TrustDB{Version: trustDBVersion, Trusted: map[string]TrustedRecord{}}, nil
		}
		return nil, fmt.Errorf("read trust db: %w", err)
	}
	var db TrustDB
	if err := json.Unmarshal(data, &db); err != nil {
		return nil, fmt.Errorf("parse trust db: %w", err)
	}
	if db.Trusted == nil {
		db.Trusted = map[string]TrustedRecord{}
	}
	return &db, nil
}

// SaveTrustDB writes the trust DB to disk with mode 0600 on POSIX. The parent
// directory is created with mode 0700 if missing.
func SaveTrustDB(db *TrustDB) error {
	if db == nil {
		return fmt.Errorf("save trust db: nil db")
	}
	if db.Version == 0 {
		db.Version = trustDBVersion
	}
	if db.Trusted == nil {
		db.Trusted = map[string]TrustedRecord{}
	}
	path, err := trustDBPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create trust db dir: %w", err)
	}
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return fmt.Errorf("encode trust db: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write trust db: %w", err)
	}
	return nil
}

// IsTrusted reports whether absPath is registered in the trust DB AND the
// current hash matches the recorded hash. Returns false (no error) when the
// path is unknown or the hash differs.
func IsTrusted(absPath, currentHash string) (bool, error) {
	db, err := LoadTrustDB()
	if err != nil {
		return false, err
	}
	rec, ok := db.Trusted[absPath]
	if !ok {
		return false, nil
	}
	return rec.SHA256 == currentHash, nil
}

// RegisterTrust records or updates a trust grant for absPath at the given
// hash. Overwrites any prior record for the same path.
func RegisterTrust(absPath, hash string) error {
	db, err := LoadTrustDB()
	if err != nil {
		return err
	}
	if db.Trusted == nil {
		db.Trusted = map[string]TrustedRecord{}
	}
	db.Trusted[absPath] = TrustedRecord{
		SHA256:    hash,
		TrustedAt: time.Now().UTC(),
	}
	return SaveTrustDB(db)
}
