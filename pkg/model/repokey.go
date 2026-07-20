package model

import (
	"strings"
	"sync"
)

// RepoKey returns the canonical workspace key for an issue.
//
// Prefers issue.SourceRepo (authoritative — set by the loader from the
// workspace DB name) over parsing the bead ID. This avoids mismatches
// where the database name differs from the ID prefix, e.g. database
// "marketplace" with IDs like "mkt-xxx".
//
// Single source of truth for repo-key derivation. pkg/ui's IssueRepoKey
// and pkg/ui/events constructors both delegate here so workspace filters
// (issue list, alerts, notifications) all look up the same key.
//
// Returns an empty string only when both SourceRepo is empty and the ID
// has no extractable prefix.
func RepoKey(issue Issue) string {
	if issue.SourceRepo != "" {
		return strings.ToLower(strings.TrimRight(issue.SourceRepo, "-:_"))
	}
	return strings.ToLower(ExtractRepoPrefix(issue.ID))
}

// ExtractRepoPrefix extracts the repository prefix from a namespaced
// issue ID. For example, "api-AUTH-123" returns "api", "web-UI-1"
// returns "web". Tries common separators (-, :, _) and only accepts a
// prefix when it's alphanumeric and at most 10 characters. Returns ""
// when no valid prefix is found.
func ExtractRepoPrefix(id string) string {
	for _, sep := range []string{"-", ":", "_"} {
		if idx := strings.Index(id, sep); idx > 0 {
			prefix := id[:idx]
			if len(prefix) <= 10 && isAlphanumericRepoPrefix(prefix) {
				return prefix
			}
		}
	}
	return ""
}

func isAlphanumericRepoPrefix(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return len(s) > 0
}

// atlasDisplayName is the DEFAULT display label for beads' cross-cutting shared
// namespace (upstream `bd --global`, DB name "beads_global", PR #3270; see
// bt-z1pzj). It is the fallback used until the real global issue prefix is
// derived from loaded data, and whenever that prefix is empty/unavailable so no
// regression is visible on this fleet (which renamed the global prefix to
// "atlas-"). Display-only - SourceRepo values, activeRepos filter keys, and
// internal/bdroute.BeadsGlobalDB all keep the raw "beads_global" spelling so
// filtering/write-routing logic is untouched.
const atlasDisplayName = "atlas"

// globalDisplayName is the live label DisplayRepoName returns for the
// beads_global namespace. It defaults to atlasDisplayName and is overwritten by
// SetGlobalDisplayName once the real global prefix is derived from loaded issues
// (bt-l76b8) - hardcoding "atlas" mislabels any fleet whose global prefix isn't
// "atlas-". Guarded because DisplayRepoName is read from the render loop while
// the setter runs from the data-load handler.
var (
	globalDisplayMu   sync.RWMutex
	globalDisplayName = atlasDisplayName
)

// SetGlobalDisplayName overrides the label DisplayRepoName returns for the
// beads_global namespace. Pass the real global issue prefix (see
// DeriveGlobalDisplayName); an empty name resets to the atlas default so an
// empty or unavailable global DB never regresses the label.
func SetGlobalDisplayName(name string) {
	globalDisplayMu.Lock()
	defer globalDisplayMu.Unlock()
	if name == "" {
		globalDisplayName = atlasDisplayName
		return
	}
	globalDisplayName = name
}

// globalDisplay returns the current beads_global display label.
func globalDisplay() string {
	globalDisplayMu.RLock()
	defer globalDisplayMu.RUnlock()
	return globalDisplayName
}

// DeriveGlobalDisplayName inspects issues for the beads_global namespace and
// returns its real ID prefix (e.g. "atlas" after `bd --global rename-prefix
// atlas-`, or "foo" for a global DB on the "foo-" prefix). The namespace is
// identified by RepoKey (its authoritative SourceRepo db-name spelling), and the
// label is the issue's ID prefix because the raw db name ("beads_global") is not
// what users see. Returns "" when no global issue is present; callers pass that
// to SetGlobalDisplayName to keep the atlas default. Early-exits on the first
// global issue.
func DeriveGlobalDisplayName(issues []Issue) string {
	for i := range issues {
		if IsAtlasNamespace(RepoKey(issues[i])) {
			if prefix := ExtractRepoPrefix(issues[i].ID); prefix != "" {
				return prefix
			}
		}
	}
	return ""
}

// IsAtlasNamespace reports whether key refers to the beads_global namespace,
// under either spelling RepoKey can produce: "beads_global" (SourceRepo,
// lowercased) or the bare ID-prefix fallback "global" (used when SourceRepo
// is empty). Callers that need to exclude the namespace from per-project
// aggregation (e.g. portfolio health, where it isn't a project and its
// counts/velocity numbers are meaningless) should check this rather than
// comparing against DisplayRepoName's output string.
func IsAtlasNamespace(key string) bool {
	switch strings.ToLower(key) {
	case "beads_global", "global":
		return true
	default:
		return false
	}
}

// DisplayRepoName maps a repo key to its display label. The only alias is the
// beads_global namespace -> the derived global prefix (defaulting to "atlas";
// bt-z1pzj, bt-l76b8); every other key passes through unchanged. Presentation-
// only: never use the return value as a filter/map key — see RepoKey and
// IsAtlasNamespace.
func DisplayRepoName(key string) string {
	if IsAtlasNamespace(key) {
		return globalDisplay()
	}
	return key
}
