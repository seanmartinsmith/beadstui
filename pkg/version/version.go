package version

import (
	"runtime/debug"
	"strings"
)

// version is set at build time by GoReleaser or manual ldflags:
//
//	go build -ldflags "-X github.com/seanmartinsmith/beadstui/pkg/version.version=v1.2.3"
//
// It starts empty so init() can distinguish "ldflags set it" from "no injection".
var version string

// fallback is the hardcoded version kept in sync with the latest release tag.
// Used only when both ldflags and debug.ReadBuildInfo fail to provide a version.
// Bump this constant whenever cutting a new release, alongside the git tag.
const fallback = "v0.1.2"

// Version is the resolved application version, populated by init().
var Version string

func init() {
	switch {
	case version != "":
		// 1. Build-time ldflags injection (GoReleaser, Nix, manual).
		Version = version
	case versionFromBuildInfo() != "":
		// 2. Module version from "go install ...@vX.Y.Z".
		Version = versionFromBuildInfo()
	default:
		// 3. Local development build: ldflags missing AND build info shows
		// "(devel)" / pseudo-version / dirty tree. Append "-dev" so IsDevBuild
		// reports true and the startup update check is skipped entirely. This
		// is what keeps a drifted `fallback` from surfacing a false "update
		// available" nag: even if `fallback` lags the latest release tag, a
		// dev build never auto-checks, so it can't be told it's behind.
		Version = fallback + "-dev"
	}
}

// IsDevBuild reports whether the running binary is a local development build
// rather than an injected release. Dev builds carry a dev-like prerelease
// suffix (e.g. "-dev", "-dirty", "-snapshot"); they are updated by rebuilding
// from source, not `bt update`, so the startup update check skips them. This
// decouples the update prompt from `fallback` drift (see init()).
func IsDevBuild() bool {
	idx := strings.Index(Version, "-")
	if idx == -1 {
		return false
	}
	label := strings.ToLower(Version[idx+1:])
	for _, marker := range []string{"dev", "dirty", "nightly", "local", "snapshot", "git"} {
		if strings.Contains(label, marker) {
			return true
		}
	}
	return false
}

// versionFromBuildInfo extracts the module version stamped by the Go toolchain
// when the binary is built via "go install ...@vX.Y.Z". Returns empty string
// for local development builds (which produce "(devel)" or pseudo-versions).
func versionFromBuildInfo() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return ""
	}
	// Filter out pseudo-versions (e.g., v0.14.5-0.20260212...-abcdef123456)
	// and dirty builds. These come from local "go build" or "go run", not
	// from "go install ...@vX.Y.Z" which produces clean semver tags.
	if strings.Contains(v, "-0.") || strings.HasSuffix(v, "+dirty") {
		return ""
	}
	if v[0] != 'v' {
		v = "v" + v
	}
	return v
}
