package ui

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/projects"
)

// TestMain prevents tests from accidentally opening a browser, and guards
// against silently writing to the user's real ~/.bt/projects.json
// (bt-fg7wa). history_test.go calls projects.Save/Stamp directly to seed
// fixtures and already isolates BT_PROJECTS_REGISTRY_PATH per-test via
// t.Setenv; this TestMain is the structural backstop for the whole
// package - it snapshots the real registry before the suite runs and
// fails loudly if a future test forgets to isolate and the real file
// changes.
func TestMain(m *testing.M) {
	// Prevent any test from accidentally opening a browser
	os.Setenv("BT_NO_BROWSER", "1")
	os.Setenv("BT_TEST_MODE", "1")

	realPath, pathErr := projects.DefaultRegistryPath()

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
			fmt.Fprintf(os.Stderr, "pkg/ui TestMain: failed to create isolated registry temp file: %v\n", err)
			os.Exit(1)
		}
		tmpPath = tmp.Name()
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
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
			fmt.Fprintf(os.Stderr, "FATAL: pkg/ui tests modified the real registry at %s (bt-fg7wa regression guard)\n", realPath)
			if code == 0 {
				code = 1
			}
		}
	}

	os.Exit(code)
}
