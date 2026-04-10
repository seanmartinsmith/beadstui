package export

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()

	orig := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	if _, err := w.Write([]byte(input)); err != nil {
		_ = w.Close()
		_ = r.Close()
		t.Fatalf("Write: %v", err)
	}
	_ = w.Close()
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = orig
		_ = r.Close()
	})

	fn()
}

func TestConfirmPrompt_ReadsInput(t *testing.T) {
	withStdin(t, "y\n", func() {
		if !confirmPrompt("Proceed?") {
			t.Fatal("Expected confirmPrompt to return true for 'y'")
		}
	})

	withStdin(t, "no\n", func() {
		if confirmPrompt("Proceed?") {
			t.Fatal("Expected confirmPrompt to return false for 'no'")
		}
	})
}

func TestCloudflareConfirmPrompt_ReadsInput(t *testing.T) {
	withStdin(t, "yes\n", func() {
		if !cloudflareConfirmPrompt("Proceed?") {
			t.Fatal("Expected cloudflareConfirmPrompt to return true for 'yes'")
		}
	})
}

func TestGetGitHubPagesURL_FallbackWhenGHMissing(t *testing.T) {
	t.Setenv("PATH", "")

	url, err := getGitHubPagesURL("alice/repo")
	if err != nil {
		t.Fatalf("getGitHubPagesURL returned error: %v", err)
	}
	if url != "https://alice.github.io/repo/" {
		t.Fatalf("Expected fallback pages URL %q, got %q", "https://alice.github.io/repo/", url)
	}
}

func TestCheckGitHubPagesStatus_GHMissingReturnsDisabled(t *testing.T) {
	t.Setenv("PATH", "")

	status, err := CheckGitHubPagesStatus("alice/repo")
	if err != nil {
		t.Fatalf("CheckGitHubPagesStatus returned error: %v", err)
	}
	if status.Enabled {
		t.Fatalf("Expected Pages status to be disabled when gh missing, got %+v", status)
	}
}

func TestOpenInBrowser_NoCommandReturnsError(t *testing.T) {
	// First verify that BV_NO_BROWSER suppresses browser opening
	t.Setenv("BV_NO_BROWSER", "1")
	if err := OpenInBrowser("https://example.com"); err != nil {
		t.Fatalf("Expected OpenInBrowser to return nil with BV_NO_BROWSER set, got: %v", err)
	}

	// Now test error when command is missing (with browser suppression off)
	t.Setenv("BV_NO_BROWSER", "")
	t.Setenv("BV_TEST_MODE", "")
	t.Setenv("PATH", "")

	if err := OpenInBrowser("https://example.com"); err == nil {
		t.Fatal("Expected OpenInBrowser to return error when platform command is missing")
	}
}

func TestOpenCloudflareInBrowser_NoCommandReturnsError(t *testing.T) {
	// First verify that BV_NO_BROWSER suppresses browser opening
	t.Setenv("BV_NO_BROWSER", "1")
	if err := OpenCloudflareInBrowser("my-project"); err != nil {
		t.Fatalf("Expected OpenCloudflareInBrowser to return nil with BV_NO_BROWSER set, got: %v", err)
	}

	// Now test error when command is missing (with browser suppression off)
	t.Setenv("BV_NO_BROWSER", "")
	t.Setenv("BV_TEST_MODE", "")
	t.Setenv("PATH", "")

	if err := OpenCloudflareInBrowser("my-project"); err == nil {
		t.Fatal("Expected OpenCloudflareInBrowser to return error when platform command is missing")
	}
}

func TestDeployToGitHubPages_NoGHReturnsError(t *testing.T) {
	t.Setenv("PATH", "")

	// Bundle path is irrelevant for this early return.
	_, err := DeployToGitHubPages(GitHubDeployConfig{
		RepoName:   "repo",
		BundlePath: filepath.Join(t.TempDir(), "bundle"),
	})
	if err == nil {
		t.Fatal("Expected DeployToGitHubPages to return error when gh is missing")
	}
}

func TestDeployToCloudflarePages_NoNPMOrWranglerReturnsError(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := DeployToCloudflarePages(CloudflareDeployConfig{
		ProjectName: "proj",
		BundlePath:  filepath.Join(t.TempDir(), "bundle"),
	})
	if err == nil {
		t.Fatal("Expected DeployToCloudflarePages to return error when npm/wrangler are missing")
	}
}

func TestCheckWranglerStatus_NoToolsInstalled(t *testing.T) {
	t.Setenv("PATH", "")

	status, err := CheckWranglerStatus()
	if err != nil {
		t.Fatalf("CheckWranglerStatus returned error: %v", err)
	}
	if status.Installed || status.Authenticated || status.NPMInstalled {
		t.Fatalf("Expected no tools installed/authenticated, got %+v", status)
	}
}

func TestAttemptWranglerInstall_NoNPMReturnsError(t *testing.T) {
	t.Setenv("PATH", "")

	if err := AttemptWranglerInstall(); err == nil {
		t.Fatal("Expected AttemptWranglerInstall to return error when npm is missing")
	}
}

func TestAuthenticateWrangler_NoWranglerReturnsError(t *testing.T) {
	t.Setenv("PATH", "")

	if err := AuthenticateWrangler(); err == nil {
		t.Fatal("Expected AuthenticateWrangler to return error when wrangler is missing")
	}
}

func TestListCloudflareProjects_NoWranglerReturnsError(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := ListCloudflareProjects()
	if err == nil {
		t.Fatal("Expected ListCloudflareProjects to return error when wrangler is missing")
	}
}

func TestDeleteCloudflareProject_RequiresConfirmation(t *testing.T) {
	if err := DeleteCloudflareProject("proj", false); err == nil {
		t.Fatal("Expected DeleteCloudflareProject to require confirmation")
	}
}

func TestListCloudflareProjects_ParsesOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stubs not supported on windows in this test")
	}

	binDir := t.TempDir()
	wranglerScript := `#!/bin/sh
set -eu
if [ "${1-}" = "pages" ] && [ "${2-}" = "project" ] && [ "${3-}" = "list" ]; then
  echo "Name  Created"
  echo "----  -------"
  echo "proj-one  2025-01-01"
  echo "proj-two  2025-01-02"
  exit 0
fi
exit 1
`
	writeExecutable(t, binDir, "wrangler", wranglerScript)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s%c%s", binDir, os.PathListSeparator, origPath))

	projects, err := ListCloudflareProjects()
	if err != nil {
		t.Fatalf("ListCloudflareProjects returned error: %v", err)
	}
	if len(projects) != 2 || projects[0] != "proj-one" || projects[1] != "proj-two" {
		t.Fatalf("Unexpected projects: %+v", projects)
	}
}

func TestListUserRepos_GHMissingReturnsError(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := ListUserRepos(5)
	if err == nil {
		t.Fatal("Expected ListUserRepos to return error when gh is missing")
	}
}

func TestDeleteRepository_RequiresConfirmation(t *testing.T) {
	if err := DeleteRepository("alice/repo", false); err == nil {
		t.Fatal("Expected DeleteRepository to require confirmation")
	}
}

func TestShowWranglerInstallInstructions_DoesNotPanic(t *testing.T) {
	ShowWranglerInstallInstructions()
}

func TestShowInstallInstructions_DoesNotPanic(t *testing.T) {
	ShowInstallInstructions()
}

func TestAttemptGHInstall_NoBrewReturnsError(t *testing.T) {
	t.Setenv("PATH", "")
	if err := AttemptGHInstall(); err == nil {
		t.Fatal("Expected AttemptGHInstall to return error when brew is missing or unsupported")
	}
}

func TestAuthenticateGH_NoGHReturnsError(t *testing.T) {
	t.Setenv("PATH", "")
	if err := AuthenticateGH(); err == nil {
		t.Fatal("Expected AuthenticateGH to return error when gh is missing")
	}
}

func TestCreateRepository_NoGHReturnsError(t *testing.T) {
	t.Setenv("PATH", "")
	if _, err := CreateRepository("repo", false, ""); err == nil {
		t.Fatal("Expected CreateRepository to return error when gh is missing")
	}
}

func TestEnableGitHubPages_NoGHReturnsError(t *testing.T) {
	t.Setenv("PATH", "")
	if _, err := EnableGitHubPages("alice/repo"); err == nil {
		t.Fatal("Expected EnableGitHubPages to return error when gh is missing")
	}
}

func TestConfirmPrompt_DefaultNoOnEOF(t *testing.T) {
	withStdin(t, "", func() {
		if confirmPrompt("Proceed?") {
			t.Fatal("Expected confirmPrompt to return false on read error/EOF")
		}
	})
}

func TestCloudflareConfirmPrompt_DefaultNoOnEOF(t *testing.T) {
	withStdin(t, "", func() {
		if cloudflareConfirmPrompt("Proceed?") {
			t.Fatal("Expected cloudflareConfirmPrompt to return false on read error/EOF")
		}
	})
}

func TestEnableGitHubPages_AlreadyEnabledFallbackPath(t *testing.T) {
	// This test is a lightweight parser path check: if gh isn't present, we only
	// assert the error is surfaced (without sleeping).
	t.Setenv("PATH", "")
	_, err := EnableGitHubPages("alice/repo")
	if err == nil {
		t.Fatal("Expected EnableGitHubPages to return error when gh is missing")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "failed") && !strings.Contains(strings.ToLower(err.Error()), "gh") {
		t.Fatalf("Unexpected EnableGitHubPages error: %v", err)
	}
}
