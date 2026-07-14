package model

import "strings"

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

// atlasDisplayName is the display alias for beads' cross-cutting shared
// namespace (upstream `bd --global`, DB name "beads_global", PR #3270; see
// bt-z1pzj). Display-only — SourceRepo values, activeRepos filter keys, and
// internal/bdroute.BeadsGlobalDB all keep the raw "beads_global" spelling so
// filtering/write-routing logic is untouched.
const atlasDisplayName = "atlas"

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

// DisplayRepoName maps a repo key to its display label. The only alias today
// is the beads_global namespace -> "atlas" (bt-z1pzj); every other key passes
// through unchanged. Presentation-only: never use the return value as a
// filter/map key — see RepoKey and IsAtlasNamespace.
func DisplayRepoName(key string) string {
	if IsAtlasNamespace(key) {
		return atlasDisplayName
	}
	return key
}
