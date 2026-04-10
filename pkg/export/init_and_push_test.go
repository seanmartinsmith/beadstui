package export

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInitAndPush_UsesForceFallbackOnPushError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stubs not supported on windows in this test")
	}

	binDir := t.TempDir()

	ghScript := `#!/bin/sh
set -eu
if [ "${1-}" = "api" ]; then
  # RepoHasContent calls: gh api repos/<repo>/contents -q length
  echo "1"
  exit 0
fi
exit 0
`
	gitScript := `#!/bin/sh
set -eu
cmd="${1-}"
shift || true

case "$cmd" in
  remote)
    sub="${1-}"
    shift || true
    case "$sub" in
      get-url)
        # Pretend there is no existing origin.
        exit 1
        ;;
      remove)
        exit 0
        ;;
      add)
        exit 0
        ;;
    esac
    ;;
  init)
    exit 0
    ;;
  add)
    exit 0
    ;;
  commit)
    echo "nothing to commit"
    exit 1
    ;;
  branch)
    echo "already"
    exit 1
    ;;
  push)
    # First push uses --force-with-lease when overwriting.
    if echo "$*" | grep -q -- "--force-with-lease"; then
      echo "cannot be resolved"
      exit 1
    fi
    # Second push retries with --force.
    exit 0
    ;;
esac

exit 0
`

	writeExecutable(t, binDir, "gh", ghScript)
	writeExecutable(t, binDir, "git", gitScript)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s%c%s", binDir, os.PathListSeparator, origPath))

	bundleDir := t.TempDir()
	// Ensure the directory contains at least one file for realism.
	if err := os.WriteFile(filepath.Join(bundleDir, "index.html"), []byte("<!doctype html>"), 0644); err != nil {
		t.Fatalf("WriteFile index.html: %v", err)
	}

	if err := InitAndPush(bundleDir, "alice/repo", true); err != nil {
		t.Fatalf("InitAndPush returned error: %v", err)
	}
}

func TestInitAndPush_RequiresForceOverwriteWhenRepoHasContent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stubs not supported on windows in this test")
	}

	binDir := t.TempDir()

	ghScript := `#!/bin/sh
set -eu
if [ "${1-}" = "api" ]; then
  echo "1"
  exit 0
fi
exit 0
`
	writeExecutable(t, binDir, "gh", ghScript)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s%c%s", binDir, os.PathListSeparator, origPath))

	bundleDir := t.TempDir()
	if err := InitAndPush(bundleDir, "alice/repo", false); err == nil {
		t.Fatal("Expected InitAndPush to return error when repo has content and ForceOverwrite=false")
	} else if !strings.Contains(strings.ToLower(err.Error()), "forceoverwrite") {
		t.Fatalf("Unexpected InitAndPush error: %v", err)
	}
}
