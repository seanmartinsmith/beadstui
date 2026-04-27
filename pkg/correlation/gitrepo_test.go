package correlation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsInsideWorkTree_InRepo(t *testing.T) {
	// The package itself lives inside a git work tree, so any path under
	// the module root must be reported as inside.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	inside, err := IsInsideWorkTree(wd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inside {
		t.Fatalf("expected inside=true for %s, got false", wd)
	}
}

func TestIsInsideWorkTree_NotInRepo(t *testing.T) {
	// Create an isolated directory outside any git repo. t.TempDir() lives
	// under the OS temp dir which we trust is not a git work tree.
	tmp := t.TempDir()

	// Defensive: ensure no parent .git accidentally makes this a work tree.
	// We don't try to fight that here — if someone runs the test inside a
	// git-tracked temp, skip rather than pretend.
	if hasGitParent(tmp) {
		t.Skipf("temp dir %s is inside a git repo; skipping", tmp)
	}

	inside, err := IsInsideWorkTree(tmp)
	if err != nil {
		t.Fatalf("expected silent fallback, got error: %v", err)
	}
	if inside {
		t.Fatalf("expected inside=false for %s, got true", tmp)
	}
}

// hasGitParent walks upward looking for a .git entry. Used only by the test
// above to skip cleanly when t.TempDir() happens to live inside a repo.
func hasGitParent(dir string) bool {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}
