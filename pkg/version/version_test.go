package version

import (
	"strings"
	"testing"
)

// TestFallbackIsCurrentReleaseTag is a hygiene check: the hardcoded fallback
// should match the latest release tag at any given time. If this test fails
// after cutting a release, bump `fallback` to the new tag.
func TestFallbackIsCurrentReleaseTag(t *testing.T) {
	const expected = "v0.1.2"
	if fallback != expected {
		t.Fatalf("fallback = %q, expected %q (bump after each release tag)", fallback, expected)
	}
}

// TestVersionResolved verifies that init() always populates Version with a
// non-empty string regardless of build environment.
func TestVersionResolved(t *testing.T) {
	if Version == "" {
		t.Fatal("Version is empty; init() must always populate it")
	}
}

// TestDevBuildMarker verifies that locally-built binaries (the test binary
// itself qualifies — no ldflags, ReadBuildInfo returns "(devel)" or a
// pseudo-version) carry the "-dev" suffix so the updater suppresses the
// update prompt.
func TestDevBuildMarker(t *testing.T) {
	// The test binary is built without ldflags, so version != "".
	// versionFromBuildInfo() should return "" for the test binary
	// (Main.Version is "(devel)"). That triggers the fallback path.
	// If versionFromBuildInfo() does return non-empty (e.g. someone runs
	// `go install` and then runs the installed test binary), Version
	// won't carry -dev — but that's not the common path and the comparison
	// logic handles non-dev paths correctly by definition.
	if version != "" {
		t.Skip("ldflags injected version; -dev marker not applicable")
	}
	if versionFromBuildInfo() != "" {
		t.Skip("clean build info available; -dev marker not applicable")
	}
	if !strings.HasSuffix(Version, "-dev") {
		t.Fatalf("local dev build should have -dev suffix; got %q", Version)
	}
}

// TestIsDevBuild verifies dev-like prerelease suffixes are recognized and
// clean release versions are not. The startup update check relies on this to
// skip auto-checks on local builds (see CheckUpdateCmd).
func TestIsDevBuild(t *testing.T) {
	saved := Version
	defer func() { Version = saved }()

	devCases := []string{"v0.1.2-dev", "v0.1.2-dirty", "v1.0.0-snapshot", "v0.1.2-nightly", "v0.1.2-local", "v0.1.2-git.abc123"}
	for _, v := range devCases {
		Version = v
		if !IsDevBuild() {
			t.Errorf("IsDevBuild() = false for %q; want true", v)
		}
	}

	releaseCases := []string{"v0.1.2", "v1.0.0", "v0.1.2-alpha", "v0.1.2-beta", "v0.1.2-rc1"}
	for _, v := range releaseCases {
		Version = v
		if IsDevBuild() {
			t.Errorf("IsDevBuild() = true for %q; want false", v)
		}
	}
}
