// pkg/projects/path.go
// Resolves the canonical user-global path for the projects registry.
//
// Mirrors pkg/ui/events.DefaultPersistPath: ~/.bt/<file>, parent
// directory created lazily by the writer rather than here.
package projects

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultRegistryPath returns ~/.bt/projects.json. Returns ("", err) when
// the user home directory cannot be resolved.
//
// The directory is NOT created here - Save creates it on first write.
func DefaultRegistryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".bt", "projects.json"), nil
}

// resolvedPath honors the BT_PROJECTS_REGISTRY_PATH env override (used
// by tests) and falls back to DefaultRegistryPath. Returns either
// (path, nil) on success or ("", err) when the home directory cannot
// be resolved.
func resolvedPath() (string, error) {
	if override := os.Getenv("BT_PROJECTS_REGISTRY_PATH"); override != "" {
		return override, nil
	}
	return DefaultRegistryPath()
}
