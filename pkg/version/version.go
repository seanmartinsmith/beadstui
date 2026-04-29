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
const fallback = "v0.1.0"

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
		// "(devel)" / pseudo-version / dirty tree. Append "-dev" so the
		// updater's compareVersions recognizes this as a dev build and
		// suppresses the update prompt — even if `fallback` drifts behind
		// the latest release tag, dev builds won't get false-positive
		// downgrade prompts.
		Version = fallback + "-dev"
	}
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
