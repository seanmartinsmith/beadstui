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

// ResolvedPath returns the registry path to read or write, honoring the
// BT_PROJECTS_REGISTRY_PATH env override and falling back to
// DefaultRegistryPath when unset.
//
// Single source of truth for the env-var name. Two callers share this
// helper:
//   - LookupAndValidate (this package) for read paths.
//   - cmd/bt/root.go::stampLaunchProjects for the write path.
//
// Renaming the env var or changing the resolution rule must happen here;
// grep for BT_PROJECTS_REGISTRY_PATH to confirm exactly one source.
//
// Returns ("", err) only when the user home directory cannot be resolved
// (and no override is set). Tests pin the override via t.Setenv to keep
// real ~/.bt/projects.json untouched.
func ResolvedPath() (string, error) {
	if override := os.Getenv("BT_PROJECTS_REGISTRY_PATH"); override != "" {
		return override, nil
	}
	return DefaultRegistryPath()
}
