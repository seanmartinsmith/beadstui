package analysis

import (
	"log/slog"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// externalPrefix is the sentinel at the start of cross-project dep references
// stored in Issue.Dependencies[].DependsOnID. Resolver input shape is
// "external:<project>:<identifier>" where <project> is the issue ID prefix
// (e.g. "bt" from "bt-mhwy.5") and <identifier> is the suffix.
const externalPrefix = "external:"

// ResolveExternalDeps returns a copy of issues with external:<project>:<id>
// dependencies rewritten to point at the canonical issue ID resolved against
// the input slice. Unresolved external refs are dropped from the returned
// issues' Dependencies and logged at debug. The input slice and its issues
// are not mutated. Safe for nil and empty inputs. Idempotent: calling twice
// produces the same result as calling once.
//
// Intended to be called on the full global issue set immediately before
// constructing an Analyzer. Runs in O(n + d) over issues and total deps.
func ResolveExternalDeps(issues []model.Issue) []model.Issue {
	if len(issues) == 0 {
		return issues
	}

	// byIDPrefix: ID prefix -> suffix -> canonical full ID. One map instead
	// of two so misses cost a single lookup.
	byIDPrefix := make(map[string]map[string]string, 8)
	for _, issue := range issues {
		prefix, suffix, ok := SplitID(issue.ID)
		if !ok {
			continue
		}
		bucket, exists := byIDPrefix[prefix]
		if !exists {
			bucket = make(map[string]string)
			byIDPrefix[prefix] = bucket
		}
		bucket[suffix] = issue.ID
	}

	out := make([]model.Issue, len(issues))
	for i, issue := range issues {
		out[i] = issue
		if len(issue.Dependencies) == 0 {
			continue
		}

		// Hot path: no external deps. Leave Dependencies pointing at the
		// caller's slice — resolver must not mutate input, and a shared
		// reference to an unmodified slice is a read-only alias.
		if !hasExternalDep(issue.Dependencies) {
			continue
		}

		rewritten := make([]*model.Dependency, 0, len(issue.Dependencies))
		var unresolved []string
		for _, dep := range issue.Dependencies {
			if dep == nil {
				continue
			}
			if !strings.HasPrefix(dep.DependsOnID, externalPrefix) {
				rewritten = append(rewritten, dep)
				continue
			}
			project, suffix, ok := parseExternalRef(dep.DependsOnID)
			if !ok {
				unresolved = append(unresolved, dep.DependsOnID)
				continue
			}
			canonical, found := byIDPrefix[project][suffix]
			if !found {
				unresolved = append(unresolved, dep.DependsOnID)
				continue
			}
			resolved := *dep
			resolved.DependsOnID = canonical
			rewritten = append(rewritten, &resolved)
		}
		out[i].Dependencies = rewritten

		if len(unresolved) > 0 {
			slog.Debug("external dep resolution: dropped refs",
				"issue", issue.ID,
				"count", len(unresolved),
				"refs", unresolved,
			)
		}
	}

	return out
}

// SplitID parses "bt-mhwy.5" into ("bt", "mhwy.5", true). Returns false when
// the ID has no hyphen or has an empty prefix/suffix. Exported because pair
// detection in pkg/view is a second legitimate consumer of the same primitive
// and a duplicated parse would drift silently across packages.
func SplitID(id string) (prefix, suffix string, ok bool) {
	idx := strings.IndexByte(id, '-')
	if idx <= 0 || idx == len(id)-1 {
		return "", "", false
	}
	return id[:idx], id[idx+1:], true
}

// parseExternalRef parses "external:<project>:<suffix>" into its parts.
// Returns false for any other shape: missing/extra colons, empty segments.
func parseExternalRef(ref string) (project, suffix string, ok bool) {
	rest := strings.TrimPrefix(ref, externalPrefix)
	if len(rest) == len(ref) {
		return "", "", false
	}
	colon := strings.IndexByte(rest, ':')
	if colon <= 0 || colon == len(rest)-1 {
		return "", "", false
	}
	project = rest[:colon]
	suffix = rest[colon+1:]
	if strings.ContainsRune(project, ':') || strings.ContainsRune(suffix, ':') {
		return "", "", false
	}
	return project, suffix, true
}

// hasExternalDep reports whether any dep in the slice uses the external:
// prefix. Lets the resolver skip the allocation path for issues with no
// cross-project refs.
func hasExternalDep(deps []*model.Dependency) bool {
	for _, dep := range deps {
		if dep == nil {
			continue
		}
		if strings.HasPrefix(dep.DependsOnID, externalPrefix) {
			return true
		}
	}
	return false
}
