package version

import (
	"strings"
	"testing"
)

// TestFallbackIsCurrentReleaseTag is a hygiene check: the hardcoded fallback
// should match the latest release tag at any given time. If this test fails
// after cutting a release, bump `fallback` to the new tag.
func TestFallbackIsCurrentReleaseTag(t *testing.T) {
	const expected = "v0.1.0"
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
