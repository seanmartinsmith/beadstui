package ui

import (
	"sort"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// EpicsTreeModel is the full-sheet, project-grouped epics tree (bt-3ftfm.1). It
// does NOT reuse the global TreeModel (which builds from every issue's edges and
// renders generic rows); it builds an epics-rooted, project-grouped tree and
// renders epic-specific rows (braille progress bars, swimlane headers). It
// mirrors tree.go's proven windowing approach (a flat visible-row list + a
// manual [start,end) clamp + cursor-follows-viewport) rather than entangling the
// shipped global tree.
//
// The Phase-1 data layer (epicsOverviewRows / EpicRow / EpicStatusMode) is
// reused unchanged for the per-epic child tallies and the status-mode listing
// decision.
type EpicsTreeModel struct {
	lanes    []epicLane    // built structure: project lanes -> root epics -> subtrees
	flatRows []epicTreeRow // flattened, expansion-aware visible rows
	expanded map[string]bool

	cursor int
	offset int
	width  int
	height int
	theme  Theme

	// Header context, set by the caller before View (Task 3).
	scopeLabel string
	modeLabel  string
}

// epicTreeRowKind tags each flattened row.
type epicTreeRowKind int

const (
	rowProjectHeader epicTreeRowKind = iota // swimlane header (collapsible)
	rowEpic                                 // an epic node (has a progress bar)
	rowChild                                // a non-epic child (status glyph + title)
)

// epicTreeRow is one rendered line in the flattened, expansion-aware list.
type epicTreeRow struct {
	kind     epicTreeRowKind
	depth    int          // indent level: project=0, epic=1, child=2, ...
	project  string       // lane key (ID prefix); set on every row
	issue    *model.Issue // nil for rowProjectHeader
	counts   epicCounts   // rollup: header (lane) or epic (own children)
	lastKid  []bool       // per-level "is last child" flags -> connector glyphs
	expanded bool         // header/epic: is it expanded?
	hasKids  bool         // epic/header: does it have something to expand?
}

// epicLane is a project swimlane: its root epics plus a rollup of their counts.
type epicLane struct {
	prefix string
	counts epicCounts // sum across the lane's root epics
	epics  []*epicTreeNode
}

// epicTreeNode is a node in the built (pre-flatten) tree. Epics carry their
// child rollup and a recursively built subtree; non-epic children are leaves.
type epicTreeNode struct {
	issue  model.Issue
	counts epicCounts
	isEpic bool
	kids   []*epicTreeNode
}

// projectPrefix is the lane key for an epic ID: the namespace prefix
// ("bt-3ftfm" -> "bt"). Falls back to the whole ID when there is no separator
// so an unprefixed epic still groups deterministically.
func projectPrefix(id string) string {
	if p := ExtractRepoPrefix(id); p != "" {
		return p
	}
	return id
}

// epicCountsFromRow lifts an EpicRow's child tally into the small epicCounts
// struct the bars and rollups consume.
func epicCountsFromRow(r EpicRow) epicCounts {
	return epicCounts{
		Done: r.Done, Total: r.Total,
		InProgress: r.InProgress, Blocked: r.Blocked, Open: r.Open, AtRisk: r.AtRisk,
	}
}

// countsFraction is a completion ratio (done/total); a childless node sorts as 0%.
func countsFraction(c epicCounts) float64 {
	if c.Total == 0 {
		return 0
	}
	return float64(c.Done) / float64(c.Total)
}

// Build (re)constructs the project-grouped epic tree from the workspace/label-
// prefiltered set. mode decides which epics are listed as roots; child tallies
// always count every child in full (the Phase-1 status-filter override). Expand
// state is preserved across rebuilds.
func (e *EpicsTreeModel) Build(scoped []model.Issue, mode EpicStatusMode, now time.Time) {
	if e.expanded == nil {
		e.expanded = map[string]bool{}
	}

	// Counts for EVERY epic (mode-independent) so nested child-epics that fall
	// outside the listing mode still render with an accurate bar. Reuses the
	// Phase-1 counting; epicsOverviewRows stays untouched.
	countsByID := make(map[string]epicCounts)
	for _, r := range epicsOverviewRows(scoped, EpicsAll, now) {
		countsByID[r.Epic.ID] = epicCountsFromRow(r)
	}

	// Which epics to list as roots (mode-filtered).
	listRows := epicsOverviewRows(scoped, mode, now)

	// Nested epic set: any epic that is a parent-child child of another epic
	// nests under its parent rather than appearing as its own lane row.
	nested := make(map[string]bool)
	for i := range scoped {
		if scoped[i].IssueType != model.TypeEpic {
			continue
		}
		for _, c := range epicChildrenSorted(scoped[i].ID, scoped) {
			if c.IssueType == model.TypeEpic {
				nested[c.ID] = true
			}
		}
	}

	// Group root epics into project lanes, building each subtree.
	laneMap := make(map[string]*epicLane)
	var laneOrder []string
	for _, r := range listRows {
		if nested[r.Epic.ID] {
			continue
		}
		prefix := projectPrefix(r.Epic.ID)
		lane, ok := laneMap[prefix]
		if !ok {
			lane = &epicLane{prefix: prefix}
			laneMap[prefix] = lane
			laneOrder = append(laneOrder, prefix)
		}
		lane.epics = append(lane.epics, e.buildNode(r.Epic, scoped, countsByID, make(map[string]bool)))
	}

	// Within each lane: sort epics by progress ascending; compute lane rollup.
	for _, lane := range laneMap {
		sort.SliceStable(lane.epics, func(i, j int) bool {
			return countsFraction(lane.epics[i].counts) < countsFraction(lane.epics[j].counts)
		})
		var roll epicCounts
		for _, ep := range lane.epics {
			roll.Done += ep.counts.Done
			roll.Total += ep.counts.Total
			roll.InProgress += ep.counts.InProgress
			roll.Blocked += ep.counts.Blocked
			roll.Open += ep.counts.Open
			roll.AtRisk += ep.counts.AtRisk
		}
		lane.counts = roll
	}

	// Lanes sort by epic count desc, then prefix asc for stability.
	sort.SliceStable(laneOrder, func(i, j int) bool {
		li, lj := laneMap[laneOrder[i]], laneMap[laneOrder[j]]
		if len(li.epics) != len(lj.epics) {
			return len(li.epics) > len(lj.epics)
		}
		return laneOrder[i] < laneOrder[j]
	})

	e.lanes = e.lanes[:0]
	for _, p := range laneOrder {
		e.lanes = append(e.lanes, *laneMap[p])
	}

	e.flatten()
}

// buildNode recursively builds an epic's subtree. Epic-typed children recurse
// (they become rowEpic with their own bars); other children are leaves. Cycle
// guarded by a path-visited set (mirrors tree.go buildNode).
func (e *EpicsTreeModel) buildNode(epic model.Issue, scoped []model.Issue, countsByID map[string]epicCounts, visited map[string]bool) *epicTreeNode {
	node := &epicTreeNode{issue: epic, isEpic: true, counts: countsByID[epic.ID]}
	if visited[epic.ID] {
		return node // cycle: stop recursing, no kids
	}
	visited[epic.ID] = true
	defer delete(visited, epic.ID)

	for _, c := range epicChildrenSorted(epic.ID, scoped) {
		if c.IssueType == model.TypeEpic {
			node.kids = append(node.kids, e.buildNode(*c, scoped, countsByID, visited))
		} else {
			node.kids = append(node.kids, &epicTreeNode{issue: *c})
		}
	}
	return node
}

// flatten walks the built lanes honoring expand state and rebuilds flatRows.
func (e *EpicsTreeModel) flatten() {
	rows := make([]epicTreeRow, 0, len(e.flatRows))
	for li := range e.lanes {
		lane := e.lanes[li]
		hExp := e.headerExpanded(lane.prefix)
		rows = append(rows, epicTreeRow{
			kind:     rowProjectHeader,
			depth:    0,
			project:  lane.prefix,
			counts:   lane.counts,
			expanded: hExp,
			hasKids:  len(lane.epics) > 0,
		})
		if !hExp {
			continue
		}
		for i, ep := range lane.epics {
			last := i == len(lane.epics)-1
			e.flattenNode(ep, lane.prefix, 1, []bool{last}, &rows)
		}
	}
	e.flatRows = rows
	if e.cursor >= len(rows) {
		e.cursor = len(rows) - 1
	}
	if e.cursor < 0 {
		e.cursor = 0
	}
}

// flattenNode emits one epic node (and, if expanded, its children) into rows.
func (e *EpicsTreeModel) flattenNode(n *epicTreeNode, project string, depth int, lastKid []bool, rows *[]epicTreeRow) {
	exp := e.epicExpanded(n.issue.ID)
	issue := n.issue
	*rows = append(*rows, epicTreeRow{
		kind:     rowEpic,
		depth:    depth,
		project:  project,
		issue:    &issue,
		counts:   n.counts,
		lastKid:  append([]bool(nil), lastKid...),
		expanded: exp,
		hasKids:  len(n.kids) > 0,
	})
	if !exp || len(n.kids) == 0 {
		return
	}
	for i, kid := range n.kids {
		last := i == len(n.kids)-1
		childLast := append(append([]bool(nil), lastKid...), last)
		if kid.isEpic {
			e.flattenNode(kid, project, depth+1, childLast, rows)
		} else {
			ki := kid.issue
			*rows = append(*rows, epicTreeRow{
				kind:    rowChild,
				depth:   depth + 1,
				project: project,
				issue:   &ki,
				lastKid: childLast,
			})
		}
	}
}

// rows returns the flattened, expansion-aware row list.
func (e *EpicsTreeModel) rows() []epicTreeRow { return e.flatRows }

// headerExpanded reports whether a project lane is expanded (default: true).
func (e *EpicsTreeModel) headerExpanded(prefix string) bool {
	if v, ok := e.expanded["proj:"+prefix]; ok {
		return v
	}
	return true
}

// epicExpanded reports whether an epic node is expanded (default: false).
func (e *EpicsTreeModel) epicExpanded(id string) bool {
	if v, ok := e.expanded[id]; ok {
		return v
	}
	return false
}

// expand marks a key expanded and re-flattens. The key is an epic ID, or
// "proj:<prefix>" for a lane header.
func (e *EpicsTreeModel) expand(key string) {
	if e.expanded == nil {
		e.expanded = map[string]bool{}
	}
	e.expanded[key] = true
	e.flatten()
}

// collapse marks a key collapsed and re-flattens.
func (e *EpicsTreeModel) collapse(key string) {
	if e.expanded == nil {
		e.expanded = map[string]bool{}
	}
	e.expanded[key] = false
	e.flatten()
}

// collapseAll resets to the lane overview: headers expanded, every epic
// collapsed (clearing the map restores both defaults).
func (e *EpicsTreeModel) collapseAll() {
	e.expanded = map[string]bool{}
	e.flatten()
}
