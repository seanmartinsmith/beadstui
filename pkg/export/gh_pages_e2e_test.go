package export

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDeployToGitHubPages_E2E_Success verifies the full orchestration flow
// by mocking both 'gh' and 'git' executables and asserting on the command sequence.
//
// IMPORTANT: This test is SKIPPED by default to prevent accidental deployment
// to real GitHub Pages. If the mock scripts fail to intercept real commands,
// this could deploy to an actual repository.
//
// To run this test explicitly, set BV_TEST_GH_PAGES_E2E=1
func TestDeployToGitHubPages_E2E_Success(t *testing.T) {
	if os.Getenv("BV_TEST_GH_PAGES_E2E") != "1" {
		t.Skip("Skipping GitHub Pages E2E test (set BV_TEST_GH_PAGES_E2E=1 to run)")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell script stubs not supported on windows in this test")
	}

	binDir := t.TempDir()
	bundleDir := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "command_log.txt")

	// Create dummy bundle content
	if err := os.WriteFile(filepath.Join(bundleDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Mock 'git'
	// We want to capture calls but not actually do git things, as InitAndPush
	// might fail if 'git init' "succeeds" (exit 0) but doesn't actually create .git dir
	// if subsequent commands expect it.
	// Actually, InitAndPush just runs exec.Command. If our mock returns 0, it proceeds.
	// It doesn't inspect the .git directory itself.
	gitScript := fmt.Sprintf(`#!/bin/sh
echo "git $*" >> "%s"
exit 0
`, logFile)
	writeExecutable(t, binDir, "git", gitScript)

	// 2. Mock 'gh'
	// Needs to handle auth status, repo view (fail then success), repo create, api calls.
	// Use a state file to toggle repo view behavior.
	stateFile := filepath.Join(t.TempDir(), "repo_created")

	ghScript := fmt.Sprintf(`#!/bin/sh
echo "gh $*" >> "%s"

case "$1" in
  auth)
    if [ "$2" = "status" ]; then
      echo "Logged in to github.com account TestUser (GitHub)"
      exit 0
    fi
    ;;
  repo)
    if [ "$2" = "view" ]; then
      if [ -f "%s" ]; then
        # Repo created - return info (simulate -q output)
        echo "TestUser/my-site"
        exit 0
      else
        # Repo not found
        exit 1
      fi
    elif [ "$2" = "create" ]; then
      # Create repo state
      touch "%s"
      # Output full repo name
      echo "TestUser/$3"
      exit 0
    fi
    ;;
  api)
    # Handle pages creation/check
    if echo "$*" | grep -q "repos/.*/pages"; then
       # Return mock URL (simulate -q output)
       echo "https://testuser.github.io/my-site/"
       exit 0
    elif echo "$*" | grep -q "repos/.*/contents"; then
       # RepoHasContent check - return 404/empty to simulate empty repo
       exit 1
    fi
    ;;
esac
exit 0
`, logFile, stateFile, stateFile)
	writeExecutable(t, binDir, "gh", ghScript)

	// Update PATH
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s%c%s", binDir, os.PathListSeparator, origPath))

	// 3. Execute
	config := GitHubDeployConfig{
		RepoName:         "my-site",
		Description:      "Test Site",
		Private:          false,
		BundlePath:       bundleDir,
		SkipConfirmation: true, // Bypass interactive prompts
		ForceOverwrite:   false,
	}

	result, err := DeployToGitHubPages(config)
	if err != nil {
		t.Fatalf("DeployToGitHubPages failed: %v", err)
	}

	// 4. Assert Results
	if result.RepoFullName != "TestUser/my-site" {
		t.Errorf("Expected RepoFullName 'TestUser/my-site', got %q", result.RepoFullName)
	}
	if result.PagesURL != "https://testuser.github.io/my-site/" {
		t.Errorf("Expected PagesURL 'https://testuser.github.io/my-site/', got %q", result.PagesURL)
	}

	// 5. Assert Command Log
	logBytes, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	logContent := string(logBytes)

	expectedCommands := []string{
		"gh auth status",        // Check auth
		"git config user.name",  // Check git config
		"git config user.email", // Check git config
		"gh repo view my-site",  // Check existence (mocks fail)
		"gh repo create my-site --public --description Test Site --clone=false", // Create
		"gh api repos/TestUser/my-site/contents -q length",                      // Check content
		"git init",  // Init
		"git add .", // Add
		"git commit -m Deploy static site via bv --pages",               // Commit
		"git branch -M main",                                            // Branch
		"git remote add origin https://github.com/TestUser/my-site.git", // Remote
		"git push -u origin main",                                       // Push
		"gh api repos/TestUser/my-site/pages -X POST",                   // Enable pages
	}

	for _, cmd := range expectedCommands {
		if !strings.Contains(logContent, cmd) {
			t.Errorf("Command log missing expected command: %q\nLog Content:\n%s", cmd, logContent)
		}
	}
}
