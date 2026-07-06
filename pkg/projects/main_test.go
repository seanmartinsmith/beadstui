package projects

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

// TestMain guards against tests in this package silently writing to the
// user's real ~/.bt/projects.json (bt-fg7wa). This package owns Save,
// Load, Stamp, DefaultRegistryPath, and ResolvedPath - the entire
// registry write API that cmd/bt/root.go::stampLaunchProjects depends
// on. Every existing test in this package passes an explicit t.TempDir()
// path to Save/Load rather than resolving the default path, but that is
// a convention, not a guarantee: a future test that calls Save/Load with
// a path from ResolvedPath()/DefaultRegistryPath() (or calls
// LookupAndValidate) without first isolating BT_PROJECTS_REGISTRY_PATH
// would silently touch the real file.
//
// The snapshot/diff around m.Run() is the automated acceptance check for
// bt-fg7wa: capture the real registry's bytes (if any) before the suite,
// isolate the env var for the run (unless the environment already
// overrides it), then fail loudly if the real file changed.
func TestMain(m *testing.M) {
	realPath, pathErr := DefaultRegistryPath()

	var before []byte
	beforeExisted := false
	if pathErr == nil {
		if data, err := os.ReadFile(realPath); err == nil {
			before = data
			beforeExisted = true
		}
	}

	isolatedHere := false
	var tmpPath string
	if os.Getenv("BT_PROJECTS_REGISTRY_PATH") == "" {
		tmp, err := os.CreateTemp("", "bt-projects-registry-test-*.json")
		if err != nil {
			fmt.Fprintf(os.Stderr, "pkg/projects TestMain: failed to create isolated registry temp file: %v\n", err)
			os.Exit(1)
		}
		tmpPath = tmp.Name()
		_ = tmp.Close()
		_ = os.Remove(tmpPath) // Save() recreates it fresh; Load() tolerates missing.
		os.Setenv("BT_PROJECTS_REGISTRY_PATH", tmpPath)
		isolatedHere = true
	}

	code := m.Run()

	if isolatedHere {
		_ = os.Remove(tmpPath)
	}

	if pathErr == nil {
		after, err := os.ReadFile(realPath)
		afterExisted := err == nil
		if beforeExisted != afterExisted || !bytes.Equal(before, after) {
			fmt.Fprintf(os.Stderr, "FATAL: pkg/projects tests modified the real registry at %s (bt-fg7wa regression guard)\n", realPath)
			if code == 0 {
				code = 1
			}
		}
	}

	os.Exit(code)
}
