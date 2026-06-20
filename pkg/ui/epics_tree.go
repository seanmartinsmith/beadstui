package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// epicsBarWidth is the braille progress bar cell count. Fixed so the row layout
// is stable across epics; the title column absorbs the remaining width.
const epicsBarWidth = 10

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
	counts    epicCounts  // rollup: header (lane) or epic (own children)
	lastKid   []bool      // per-level "is last child" flags -> connector glyphs
	expanded  bool        // header/epic: is it expanded?
	hasKids   bool        // epic/header: does it have something to expand?
	laneEpics int         // rowProjectHeader: count of root epics in the lane
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
			kind:      rowProjectHeader,
			depth:     0,
			project:   lane.prefix,
			counts:    lane.counts,
			expanded:  hExp,
			hasKids:   len(lane.epics) > 0,
			laneEpics: len(lane.epics),
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

// cursorRow returns the row under the cursor, or (zero, false) when empty.
func (e *EpicsTreeModel) cursorRow() (epicTreeRow, bool) {
	if e.cursor < 0 || e.cursor >= len(e.flatRows) {
		return epicTreeRow{}, false
	}
	return e.flatRows[e.cursor], true
}

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

// SetSize sets the viewport dimensions and keeps the cursor visible.
func (e *EpicsTreeModel) SetSize(w, h int) {
	e.width = w
	e.height = h
	e.ensureCursorVisible()
}

// SetTheme sets the palette used by the row renderers.
func (e *EpicsTreeModel) SetTheme(t Theme) { e.theme = t }

// SetContext sets the scope/mode labels shown in the header line.
func (e *EpicsTreeModel) SetContext(scope, mode string) {
	e.scopeLabel = scope
	e.modeLabel = mode
}

// epicCount is the number of root epics across all lanes (the header's "N epics").
func (e *EpicsTreeModel) epicCount() int {
	n := 0
	for i := range e.lanes {
		n += len(e.lanes[i].epics)
	}
	return n
}

// bodyHeight is the rows available for the windowed tree body (header + footer
// each take one line).
func (e *EpicsTreeModel) bodyHeight() int {
	h := e.height - 2
	if h < 1 {
		h = 1
	}
	return h
}

// visibleCount is the content rows shown at once. When the list overflows the
// body, two lines are reserved for the up/down "N more" indicators so the body
// never exceeds bodyHeight.
func (e *EpicsTreeModel) visibleCount() int {
	h := e.bodyHeight()
	if len(e.flatRows) > h {
		h -= 2
		if h < 1 {
			h = 1
		}
	}
	return h
}

// ensureCursorVisible scrolls the viewport just enough to keep the cursor on
// screen (cursor-at-edge scrolling, mirroring tree.go).
func (e *EpicsTreeModel) ensureCursorVisible() {
	n := len(e.flatRows)
	if n == 0 {
		e.offset = 0
		return
	}
	vis := e.visibleCount()
	if e.cursor < e.offset {
		e.offset = e.cursor
	}
	if e.cursor >= e.offset+vis {
		e.offset = e.cursor - vis + 1
	}
	maxOff := n - vis
	if maxOff < 0 {
		maxOff = 0
	}
	if e.offset > maxOff {
		e.offset = maxOff
	}
	if e.offset < 0 {
		e.offset = 0
	}
}

// window returns the [start,end) slice of flatRows currently visible.
func (e *EpicsTreeModel) window() (start, end int) {
	n := len(e.flatRows)
	vis := e.visibleCount()
	start = e.offset
	if start < 0 {
		start = 0
	}
	end = start + vis
	if end > n {
		end = n
		start = end - vis
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

// moveCursor moves the cursor by delta, clamps it, and follows the viewport.
func (e *EpicsTreeModel) moveCursor(delta int) {
	n := len(e.flatRows)
	if n == 0 {
		return
	}
	e.cursor += delta
	if e.cursor < 0 {
		e.cursor = 0
	}
	if e.cursor >= n {
		e.cursor = n - 1
	}
	e.ensureCursorVisible()
}

// View renders the full-bleed epics tree: a 1-line header, the windowed tree
// body (with up/down "N more" indicators), and a 1-line footer of key hints. It
// never calls lipgloss.Place; every line is clamped to width via MaxWidth so
// braille bars, deep prefixes, and long titles never wrap (mirrors tree.go).
func (e *EpicsTreeModel) View() string {
	t := e.theme
	muted := lipgloss.NewStyle().Foreground(t.Muted)
	title := lipgloss.NewStyle().Bold(true).Foreground(t.Primary)

	var sb strings.Builder

	// Header: EPICS · <scope> · <mode>    N epics
	head := "EPICS"
	if e.scopeLabel != "" {
		head += " · " + e.scopeLabel
	}
	if e.modeLabel != "" {
		head += " · " + e.modeLabel
	}
	header := title.Render(head) + muted.Render(fmt.Sprintf("    %d epics", e.epicCount()))
	sb.WriteString(e.clamp(header))
	sb.WriteString("\n")

	if len(e.flatRows) == 0 {
		sb.WriteString("\n")
		sb.WriteString(e.clamp(muted.Render("  No epics in scope.")))
		sb.WriteString("\n")
		sb.WriteString(e.footer())
		return sb.String()
	}

	start, end := e.window()
	if start > 0 {
		sb.WriteString(e.clamp(muted.Render(fmt.Sprintf("  ↑ %d more", start))))
		sb.WriteString("\n")
	}
	for i := start; i < end; i++ {
		sb.WriteString(e.clamp(e.renderRow(e.flatRows[i], i == e.cursor)))
		sb.WriteString("\n")
	}
	if end < len(e.flatRows) {
		sb.WriteString(e.clamp(muted.Render(fmt.Sprintf("  ↓ %d more", len(e.flatRows)-end))))
		sb.WriteString("\n")
	}

	sb.WriteString(e.footer())
	return sb.String()
}

// clamp truncates a styled line to the viewport width (never wraps).
func (e *EpicsTreeModel) clamp(line string) string {
	if e.width > 0 {
		return lipgloss.NewStyle().MaxWidth(e.width).Render(line)
	}
	return line
}

// footer renders the key-hint line.
func (e *EpicsTreeModel) footer() string {
	return lipgloss.NewStyle().Foreground(e.theme.Muted).Italic(true).Render(
		"j/k nav · →/⏎ expand · ← collapse · z collapse-all · s active/all/completed · v zoom · esc back")
}

// renderRow dispatches to the per-kind row renderer.
func (e *EpicsTreeModel) renderRow(r epicTreeRow, selected bool) string {
	switch r.kind {
	case rowProjectHeader:
		return e.renderHeaderRow(r)
	case rowEpic:
		return e.renderEpicRow(r, selected)
	default:
		return e.renderChildRow(r, selected)
	}
}

// renderHeaderRow renders a swimlane header: expand glyph, lane name, a
// width-filling rule, and the lane rollup (epic count + aggregate %).
func (e *EpicsTreeModel) renderHeaderRow(r epicTreeRow) string {
	t := e.theme
	glyph := "▾"
	if !r.expanded {
		glyph = "▸"
	}
	left := fmt.Sprintf("%s %s ", glyph, strings.ToUpper(r.project))
	pct := 0
	if r.counts.Total > 0 {
		pct = r.counts.Done * 100 / r.counts.Total
	}
	right := fmt.Sprintf(" %d epics · %d%% ", r.laneEpics, pct)

	ruleW := e.width - lipgloss.Width(left) - lipgloss.Width(right)
	if ruleW < 0 {
		ruleW = 0
	}
	rule := strings.Repeat("─", ruleW)

	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
	ruleStyle := lipgloss.NewStyle().Foreground(t.Border)
	rollStyle := lipgloss.NewStyle().Foreground(t.Muted)
	return nameStyle.Render(left) + ruleStyle.Render(rule) + rollStyle.Render(right)
}

// renderEpicRow renders an epic node: connectors, expand glyph, ID, braille
// progress bar (composition bar at top level, compact mono bar when nested),
// pct, done/total, at-risk marker, then a title truncated to the remaining
// width by plain width so styling never overflows.
func (e *EpicsTreeModel) renderEpicRow(r epicTreeRow, selected bool) string {
	t := e.theme
	prefix := buildEpicTreePrefix(r.lastKid, t)
	prefixW := 4 * len(r.lastKid)

	glyph := " "
	if r.hasKids {
		if r.expanded {
			glyph = "▾"
		} else {
			glyph = "▸"
		}
	}

	pct := 0
	if r.counts.Total > 0 {
		pct = r.counts.Done * 100 / r.counts.Total
	}
	pctStr := fmt.Sprintf("%d%%", pct)
	countStr := fmt.Sprintf("%d/%d", r.counts.Done, r.counts.Total)
	risk := ""
	if r.counts.AtRisk > 0 {
		risk = fmt.Sprintf(" ⚠%d", r.counts.AtRisk)
	}

	// Top-level epics get the full status-composition bar; nested epics get the
	// compact monochrome mini bar (composition is less central at depth).
	var bar string
	if r.depth <= 1 {
		bar = brailleCompositionBar(r.counts, epicsBarWidth, t)
	} else {
		bar = braillePlainBar(r.counts.Done, r.counts.Total, epicsBarWidth)
	}

	id := r.issue.ID
	// Fixed (non-title) plain width: prefix + glyph + sp + id + sp + bar + sp +
	// pct + sp + counts + risk + 2 (gap before title).
	fixed := prefixW + 2 + lipgloss.Width(id) + 1 + epicsBarWidth + 1 +
		lipgloss.Width(pctStr) + 1 + lipgloss.Width(countStr) + lipgloss.Width(risk) + 2
	titleBudget := e.width - fixed
	if titleBudget < 0 {
		titleBudget = 0
	}
	title := truncateString(r.issue.Title, titleBudget)

	glyphStyle := lipgloss.NewStyle().Foreground(t.Secondary)
	idStyle := lipgloss.NewStyle().Foreground(t.Secondary)
	pctStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground())
	countStyle := lipgloss.NewStyle().Foreground(t.Muted)
	riskStyle := lipgloss.NewStyle().Foreground(t.Feature)
	titleStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground())
	if selected {
		idStyle = idStyle.Bold(true)
		titleStyle = titleStyle.Bold(true)
		glyphStyle = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	}

	var sb strings.Builder
	sb.WriteString(prefix)
	sb.WriteString(glyphStyle.Render(glyph))
	sb.WriteString(" ")
	sb.WriteString(idStyle.Render(id))
	sb.WriteString(" ")
	sb.WriteString(bar)
	sb.WriteString(" ")
	sb.WriteString(pctStyle.Render(pctStr))
	sb.WriteString(" ")
	sb.WriteString(countStyle.Render(countStr))
	if risk != "" {
		sb.WriteString(riskStyle.Render(risk))
	}
	if title != "" {
		sb.WriteString("  ")
		sb.WriteString(titleStyle.Render(title))
	}
	return sb.String()
}

// renderChildRow renders a non-epic child: connectors, status glyph, ID, title.
// Closed children render faint so completed work recedes.
func (e *EpicsTreeModel) renderChildRow(r epicTreeRow, selected bool) string {
	t := e.theme
	prefix := buildEpicTreePrefix(r.lastKid, t)
	prefixW := 4 * len(r.lastKid)

	glyph := statusGlyph(r.issue.Status)
	statusColor := t.GetStatusColor(string(r.issue.Status))

	id := r.issue.ID
	fixed := prefixW + lipgloss.Width(glyph) + 1 + lipgloss.Width(id) + 3 // " — "
	titleBudget := e.width - fixed
	if titleBudget < 0 {
		titleBudget = 0
	}
	title := truncateString(r.issue.Title, titleBudget)

	var sb strings.Builder
	sb.WriteString(prefix)
	sb.WriteString(lipgloss.NewStyle().Foreground(statusColor).Render(glyph))
	sb.WriteString(" ")

	if isClosedLikeStatus(r.issue.Status) {
		body := id
		if title != "" {
			body += " — " + title
		}
		sb.WriteString(lipgloss.NewStyle().Faint(true).Render(body))
	} else {
		idStyle := lipgloss.NewStyle().Foreground(t.Secondary)
		titleStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground())
		if selected {
			idStyle = idStyle.Bold(true)
			titleStyle = titleStyle.Bold(true)
		}
		sb.WriteString(idStyle.Render(id))
		if title != "" {
			sb.WriteString(" — ")
			sb.WriteString(titleStyle.Render(title))
		}
	}
	return sb.String()
}

// buildEpicTreePrefix builds the connector prefix (├─ └─ │) from a node's
// per-level is-last-child flags. Mirrors tree.go buildTreePrefix.
func buildEpicTreePrefix(lastKid []bool, t Theme) string {
	if len(lastKid) == 0 {
		return ""
	}
	var sb strings.Builder
	for i := 0; i < len(lastKid)-1; i++ {
		if lastKid[i] {
			sb.WriteString("    ")
		} else {
			sb.WriteString("│   ")
		}
	}
	if lastKid[len(lastKid)-1] {
		sb.WriteString("└── ")
	} else {
		sb.WriteString("├── ")
	}
	return lipgloss.NewStyle().Foreground(t.Muted).Render(sb.String())
}
