package datasource

import (
	"strconv"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// synthesizeHierarchyDeps adds parent_child Dependencies to issues whose IDs
// follow bd's implicit-hierarchy convention (`<base>.<n>` where <n> is numeric
// and <base> exists in the loaded set). bd computes hierarchy from ID
// structure at query time rather than materializing it as a column, so bt
// mirrors that computation locally so consumers (tree, detail dep render,
// graph metrics) see the same hierarchy bd does. See bt-cuyiz for the
// decision rationale.
func synthesizeHierarchyDeps(issues []model.Issue) []model.Issue {
	if len(issues) == 0 {
		return issues
	}
	idSet := make(map[string]struct{}, len(issues))
	for i := range issues {
		idSet[issues[i].ID] = struct{}{}
	}
	for i := range issues {
		parent, ok := implicitParentID(issues[i].ID)
		if !ok {
			continue
		}
		if _, exists := idSet[parent]; !exists {
			continue
		}
		if hasParentChildDep(&issues[i], parent) {
			continue
		}
		issues[i].Dependencies = append(issues[i].Dependencies, &model.Dependency{
			IssueID:     issues[i].ID,
			DependsOnID: parent,
			Type:        model.DepParentChild,
			CreatedBy:   "bt:synthesized-hierarchy",
		})
	}
	return issues
}

// implicitParentID returns the parent ID for a bead whose ID follows the
// `<base>.<n>` convention (n must be a non-negative integer). Returns
// ("", false) for IDs without a numeric dot-suffix, which deliberately
// excludes cross-prefix paired IDs (e.g., bt-zsy8 + bd-zsy8) and
// non-numeric suffixes (e.g., bt-foo.bar).
func implicitParentID(id string) (string, bool) {
	idx := strings.LastIndex(id, ".")
	if idx <= 0 || idx == len(id)-1 {
		return "", false
	}
	if _, err := strconv.Atoi(id[idx+1:]); err != nil {
		return "", false
	}
	return id[:idx], true
}

// hasParentChildDep reports whether issue already has a parent_child dep
// pointing at parentID. Defensive guard against duplicating an edge that
// was filed explicitly (e.g., via `bd create --parent`).
func hasParentChildDep(issue *model.Issue, parentID string) bool {
	for _, dep := range issue.Dependencies {
		if dep != nil && dep.DependsOnID == parentID && dep.Type == model.DepParentChild {
			return true
		}
	}
	return false
}
