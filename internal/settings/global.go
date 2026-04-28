// Package settings owns bt's persistent user configuration. The global
// scope (this file) lives at ~/.bt/settings.json and contains values that
// aren't tied to any single beads project — most importantly the cold-boot
// anchor (bt-mxz9 Phase 2). Per-project settings live alongside each
// project's .bt/ directory and are not in scope here.
//
// Format is JSON. Writes go through a tempfile + rename to avoid partial
// writes if bt is killed mid-write. The settings file is single-author
// (bt itself), so no inter-process locking is needed.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AnchorEnvVar is the override for the persisted anchor_project field.
// Set to a project path to force bt to use that path as the cold-boot
// anchor regardless of what's in settings.json. Never persisted, never
// auto-modified.
const AnchorEnvVar = "BT_ANCHOR_PROJECT"

// Global is the shape of ~/.bt/settings.json. New fields go here without
// migration — JSON unmarshalling tolerates missing keys, marshalling
// preserves field order, and the file is small enough that re-encoding
// the whole struct on each Save is fine.
type Global struct {
	// AnchorProject is the absolute path to a beads project that bt can
	// fall back on for cold-boot operations (e.g. starting the shared
	// Dolt server from a non-workspace cwd). Auto-managed by bt's
	// successful-boot hook with latest-cwd-wins semantics.
	AnchorProject string `json:"anchor_project,omitempty"`
}

// DefaultPath returns the canonical location of the global settings
// file: ~/.bt/settings.json. Returns ("", err) when the user home
// directory cannot be resolved.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".bt", "settings.json"), nil
}

// Load reads the global settings file at the default path. A missing
// file is not an error — Load returns a zero-value Global so callers
// can treat first-run identically to subsequent runs. Corrupt JSON IS
// surfaced, since silently wiping a malformed file would erase the
// user's anchor on the first cold-boot after the corruption.
func Load() (*Global, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads settings from an explicit path. Exported for tests and
// for callers that want to read settings under a non-default root.
func LoadFrom(path string) (*Global, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Global{}, nil
		}
		return nil, fmt.Errorf("read settings %s: %w", path, err)
	}
	var g Global
	if len(data) == 0 {
		return &g, nil
	}
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("parse settings %s: %w", path, err)
	}
	return &g, nil
}

// Save writes the receiver to the default path using atomic-replace
// (tempfile + os.Rename in the same directory). Creates ~/.bt/ if it
// doesn't exist.
func (g *Global) Save() error {
	path, err := DefaultPath()
	if err != nil {
		return err
	}
	return g.SaveTo(path)
}

// SaveTo writes to an explicit path. Exported for tests.
func (g *Global) SaveTo(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir settings dir: %w", err)
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	// Tempfile in the same directory so os.Rename is atomic on the same
	// filesystem. Pattern includes a leading dot so a partial write left
	// behind by a hard kill doesn't visually pollute ls output.
	f, err := os.CreateTemp(filepath.Dir(path), ".settings.json.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp settings: %w", err)
	}
	tmpPath := f.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := f.Write(data); err != nil {
		f.Close()
		cleanup()
		return fmt.Errorf("write temp settings: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp settings: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("rename temp settings: %w", err)
	}
	return nil
}

// Anchor returns the cold-boot anchor with env-var precedence: BT_ANCHOR_PROJECT
// trumps the persisted field, which trumps "no anchor available". Returns the
// empty string when no anchor is set anywhere.
func (g *Global) Anchor() string {
	if v := strings.TrimSpace(os.Getenv(AnchorEnvVar)); v != "" {
		return v
	}
	if g == nil {
		return ""
	}
	return strings.TrimSpace(g.AnchorProject)
}

// AnchorFromEnv reports whether the anchor came from the env override
// rather than the persisted file. Used by the cold-boot path to decide
// whether to clear a persisted anchor on "anchor invalid" errors —
// env-supplied anchors are never auto-modified.
func AnchorFromEnv() bool {
	return strings.TrimSpace(os.Getenv(AnchorEnvVar)) != ""
}
