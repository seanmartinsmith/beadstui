package ui

import (
	"fmt"
	"image/color"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// FormatTimeRel returns a relative time string (e.g., "2h ago", "3d ago")
func FormatTimeRel(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	d := time.Since(t)
	if d < 0 {
		// Future timestamps treated as now
		return "now"
	}
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(d.Hours()/(24*7)))
	default:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	}
}

// FormatTimeAbs returns an absolute timestamp string (e.g., "2026-03-11 14:30").
func FormatTimeAbs(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Local().Format("2006-01-02 15:04")
}

// staleDays returns the stale threshold in days from BT_STALE_DAYS env var, defaulting to 14.
func staleDays() int {
	if s := os.Getenv("BT_STALE_DAYS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 14
}

// isOverdue returns true if the issue has a past due date and is not closed.
func isOverdue(issue *model.Issue) bool {
	return issue.DueDate != nil && issue.DueDate.Before(time.Now()) && !issue.Status.IsClosed()
}

// isStale returns true if the issue hasn't been updated in staleDays() and is open/in_progress.
func isStale(issue *model.Issue) bool {
	if !issue.Status.IsOpen() {
		return false
	}
	threshold := time.Duration(staleDays()) * 24 * time.Hour
	return time.Since(issue.UpdatedAt) > threshold
}

// gateAwaitFromBlockers checks if any blocker of the given issue is a gate-type issue,
// and returns its AwaitType. Gate issues block other issues via dependencies; the gate's
// own AwaitType tells us what kind of gate it is (human, timer, gh:run, etc.).
func gateAwaitFromBlockers(issue model.Issue, issueMap map[string]*model.Issue) string {
	for _, dep := range issue.Dependencies {
		if dep == nil || !dep.Type.IsBlocking() {
			continue
		}
		if blocker, ok := issueMap[dep.DependsOnID]; ok && blocker != nil {
			if blocker.IssueType == "gate" && blocker.AwaitType != nil && *blocker.AwaitType != "" {
				return *blocker.AwaitType
			}
		}
	}
	return ""
}

// epicProgress counts children of an epic issue by scanning all issues for parent-child deps.
// Returns (closed, total). If the issue is not an epic or has no children, returns (0, 0).
func epicProgress(epicID string, allIssues []model.Issue) (done, total int) {
	for i := range allIssues {
		for _, dep := range allIssues[i].Dependencies {
			if dep != nil && dep.Type == model.DepParentChild && dep.DependsOnID == epicID {
				total++
				if allIssues[i].Status.IsClosed() {
					done++
				}
				break // only count once per child
			}
		}
	}
	return
}

// epicChildrenSorted returns the parent's children in natural-numeric order
// (.0 → .13, with .9 before .10), so the detail-pane Epic Progress block
// reads top-to-bottom in suffix order rather than the Dolt load order
// (which is created_at DESC and renders as .13 → .0). bt-u05bo.
func epicChildrenSorted(epicID string, allIssues []model.Issue) []*model.Issue {
	var children []*model.Issue
	for i := range allIssues {
		for _, dep := range allIssues[i].Dependencies {
			if dep != nil && dep.Type == model.DepParentChild && dep.DependsOnID == epicID {
				children = append(children, &allIssues[i])
				break
			}
		}
	}
	sort.Slice(children, func(i, j int) bool {
		return naturalIDKey(children[i].ID) < naturalIDKey(children[j].ID)
	})
	return children
}

// naturalIDKey returns a sort key that compares dot-separated IDs naturally:
// numeric segments are zero-padded to a fixed width so .9 sorts before .10
// and .1.2 sorts before .1.10. Non-numeric segments are compared lexically.
// Pure stdlib; no regex.
func naturalIDKey(id string) string {
	const padWidth = 12
	var sb strings.Builder
	for _, seg := range strings.Split(id, ".") {
		if n, err := strconv.Atoi(seg); err == nil && n >= 0 {
			sb.WriteString(fmt.Sprintf("%0*d", padWidth, n))
		} else {
			sb.WriteString(seg)
		}
		sb.WriteByte('.')
	}
	return sb.String()
}

// statusGlyph returns a Unicode geometric shape representing the issue
// status — used in the detail-pane Epic Progress list. Shapes (not emojis)
// keep the btop/lazygit aesthetic: visual distinction without color
// dependency, and they survive glamour's markdown rendering as plain text.
func statusGlyph(s model.Status) string {
	switch s {
	case model.StatusClosed, model.StatusTombstone:
		return "✓"
	case model.StatusInProgress:
		return "◐"
	case model.StatusBlocked:
		return "⊘"
	case model.StatusDeferred:
		return "❄"
	case model.StatusPinned:
		return "◆"
	case model.StatusReview:
		return "◇"
	case model.StatusHooked:
		return "◈"
	default: // open
		return "○"
	}
}

// StateDimension represents a parsed dimension:value label.
type StateDimension struct {
	Dimension string
	Value     string
}

// nonStatePrefixes are label prefixes that look like dimension:value but are not state dimensions.
var nonStatePrefixes = map[string]bool{
	"export":   true,
	"provides": true,
	"external": true,
	"stream":   true,
}

// parseStateDimensions extracts dimension:value pairs from labels, excluding known non-state prefixes.
func parseStateDimensions(labels []string) []StateDimension {
	var dims []StateDimension
	for _, label := range labels {
		idx := strings.Index(label, ":")
		if idx <= 0 || idx >= len(label)-1 {
			continue // no colon, or colon at start/end
		}
		dim := label[:idx]
		val := label[idx+1:]
		if nonStatePrefixes[dim] {
			continue
		}
		dims = append(dims, StateDimension{Dimension: dim, Value: val})
	}
	return dims
}

// Capability represents a cross-project capability label (bt-t0z6).
type Capability struct {
	Project       string // source project (from SourceRepo)
	TargetProject string // for external: the project being depended on
	Type          string // "export", "provides", "external"
	Capability    string // the capability name
	IssueID       string // which issue has this label
}

// parseCapabilities extracts capability labels from an issue's labels.
// Recognized patterns:
//   - export:<name>     - this project produces capability <name>
//   - provides:<name>   - this project fulfills capability <name>
//   - external:<project>:<capability> - consumes <capability> from <project>
func parseCapabilities(issue model.Issue) []Capability {
	var caps []Capability
	for _, label := range issue.Labels {
		idx := strings.Index(label, ":")
		if idx <= 0 || idx >= len(label)-1 {
			continue
		}
		prefix := label[:idx]
		rest := label[idx+1:]

		switch prefix {
		case "export", "provides":
			caps = append(caps, Capability{
				Project:    issue.SourceRepo,
				Type:       prefix,
				Capability: rest,
				IssueID:    issue.ID,
			})
		case "external":
			// external:<project>:<capability>
			idx2 := strings.Index(rest, ":")
			if idx2 <= 0 || idx2 >= len(rest)-1 {
				continue
			}
			caps = append(caps, Capability{
				Project:       issue.SourceRepo,
				TargetProject: rest[:idx2],
				Type:          "external",
				Capability:    rest[idx2+1:],
				IssueID:       issue.ID,
			})
		}
	}
	return caps
}

// CapabilityEdge represents a dependency from one project to another.
type CapabilityEdge struct {
	FromProject string // consumer
	ToProject   string // producer
	Capability  string // what's being consumed
	Resolved    bool   // whether a matching export exists
}

// aggregateCapabilities scans all issues and builds a capability graph.
func aggregateCapabilities(issues []model.Issue) (exports map[string][]Capability, consumes map[string][]Capability, edges []CapabilityEdge) {
	exports = make(map[string][]Capability)  // project -> exports
	consumes = make(map[string][]Capability) // project -> external deps

	// Collect all capabilities
	allExports := make(map[string]map[string]bool) // capability -> set of projects that export it
	for _, issue := range issues {
		for _, cap := range parseCapabilities(issue) {
			switch cap.Type {
			case "export", "provides":
				exports[cap.Project] = append(exports[cap.Project], cap)
				if allExports[cap.Capability] == nil {
					allExports[cap.Capability] = make(map[string]bool)
				}
				allExports[cap.Capability][cap.Project] = true
			case "external":
				consumes[cap.Project] = append(consumes[cap.Project], cap)
			}
		}
	}

	// Build edges from external deps
	for project, caps := range consumes {
		for _, cap := range caps {
			resolved := false
			if producers, ok := allExports[cap.Capability]; ok && len(producers) > 0 {
				resolved = true
			}
			edges = append(edges, CapabilityEdge{
				FromProject: project,
				ToProject:   cap.TargetProject,
				Capability:  cap.Capability,
				Resolved:    resolved,
			})
		}
	}

	return exports, consumes, edges
}

// formatNanoseconds converts nanoseconds to a human-readable duration string.
func formatNanoseconds(ns int64) string {
	d := time.Duration(ns)
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%.0fd", d.Hours()/24)
	case d >= time.Hour:
		return fmt.Sprintf("%.0fh", d.Hours())
	case d >= time.Minute:
		return fmt.Sprintf("%.0fm", d.Minutes())
	default:
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
}

// truncateRunesHelper truncates a string to max visual width (cells), adding suffix if needed.
// ANSI-aware: SGR escape sequences in `s` are not counted toward width and
// are preserved across the truncation cut. Plain-text input behaves
// identically to the previous runewidth-based implementation because ansi.*
// functions agree with runewidth.* on cell counting for unstyled text.
//
// The switch from runewidth.* to ansi.* closes the bt-l22b class of bug
// where SGR bytes were counted as visible chars, over-truncating styled
// rows and leaving them with visible width below the target. Downstream
// compositors then saw inconsistent row widths and drifted modal borders.
func truncateRunesHelper(s string, maxWidth int, suffix string) string {
	if maxWidth <= 0 {
		return ""
	}

	width := ansi.StringWidth(s)
	if width <= maxWidth {
		return s
	}

	suffixWidth := ansi.StringWidth(suffix)
	if suffixWidth > maxWidth {
		// Even suffix is too wide, truncate suffix
		return ansi.Truncate(suffix, maxWidth, "")
	}

	targetWidth := maxWidth - suffixWidth
	return ansi.Truncate(s, targetWidth, "") + suffix
}

// padRight pads string s with spaces on the right to reach visual width.
// Uses go-runewidth to handle wide characters (emojis, CJK) correctly,
// consistent with truncateRunesHelper which also uses visual width.
func padRight(s string, width int) string {
	visualWidth := runewidth.StringWidth(s)
	if visualWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visualWidth)
}

// truncate truncates string s to maxRunes
func truncate(s string, maxRunes int) string {
	return truncateRunesHelper(s, maxRunes, "…")
}

// DependencyNode represents a visual node in the dependency tree
type DependencyNode struct {
	ID       string
	Title    string
	Status   string
	Type     string // "root", "blocks", "related", etc.
	Children []*DependencyNode
}

// BuildDependencyTree constructs a tree from dependencies for visualization.
// maxDepth limits recursion to prevent infinite loops and performance issues.
// Set maxDepth to 0 for unlimited depth (use with caution).
//
// Direction: outgoing dependencies are walked recursively (X -> things X
// depends on). In addition, immediate inverse children (issues whose
// parent_child dep points at the root or any visited node) are appended as
// leaf nodes. This surfaces hierarchy in the detail render — e.g., viewing
// an epic shows its direct children — without recursing into the inverse
// direction (which would loop back via parent_child outgoing edges and
// inflate large epics). Drilling into a child node shows that child's own
// subtree.
func BuildDependencyTree(rootID string, issueMap map[string]*model.Issue, maxDepth int) *DependencyNode {
	visited := make(map[string]bool)
	childrenOf := buildChildrenOfMap(issueMap)
	return buildTreeRecursive(rootID, issueMap, childrenOf, "root", visited, 0, maxDepth)
}

// buildChildrenOfMap returns parentID -> sorted slice of child IDs, derived
// from parent_child dependency edges stored on the children. Sorted with
// naturalIDKey so .9 renders before .10 in the detail dep tree, matching
// the Epic Progress block (bt-cuyiz, sort fix bt-u05bo).
func buildChildrenOfMap(issueMap map[string]*model.Issue) map[string][]string {
	out := make(map[string][]string)
	for id, issue := range issueMap {
		if issue == nil {
			continue
		}
		for _, dep := range issue.Dependencies {
			if dep != nil && dep.Type == model.DepParentChild {
				out[dep.DependsOnID] = append(out[dep.DependsOnID], id)
			}
		}
	}
	for k := range out {
		sort.Slice(out[k], func(i, j int) bool {
			return naturalIDKey(out[k][i]) < naturalIDKey(out[k][j])
		})
	}
	return out
}

func buildTreeRecursive(id string, issueMap map[string]*model.Issue, childrenOf map[string][]string, depType string, visited map[string]bool, depth, maxDepth int) *DependencyNode {
	// Check depth limit (0 = unlimited)
	if maxDepth > 0 && depth > maxDepth {
		return nil
	}

	// Cycle detection
	if visited[id] {
		return &DependencyNode{
			ID:     id,
			Title:  "(cycle)",
			Status: "?",
			Type:   depType,
		}
	}

	issue, exists := issueMap[id]
	if !exists {
		return &DependencyNode{
			ID:     id,
			Title:  "(not found)",
			Status: "?",
			Type:   depType,
		}
	}

	visited[id] = true
	defer func() { visited[id] = false }() // Allow revisiting in different branches

	node := &DependencyNode{
		ID:     issue.ID,
		Title:  issue.Title,
		Status: string(issue.Status),
		Type:   depType,
	}

	// Outgoing edges: things this issue depends on.
	for _, dep := range issue.Dependencies {
		childNode := buildTreeRecursive(dep.DependsOnID, issueMap, childrenOf, string(dep.Type), visited, depth+1, maxDepth)
		if childNode != nil {
			node.Children = append(node.Children, childNode)
		}
	}

	// Inverse children: appended as leaves (no recursion) so an epic with N
	// children renders as a flat list of N rows under the parent without
	// looping back through their parent_child edges. (bt-cuyiz)
	for _, childID := range childrenOf[id] {
		if visited[childID] {
			continue
		}
		child, ok := issueMap[childID]
		if !ok || child == nil {
			continue
		}
		node.Children = append(node.Children, &DependencyNode{
			ID:     child.ID,
			Title:  child.Title,
			Status: string(child.Status),
			Type:   "child",
		})
	}

	return node
}

// RenderDependencyTree renders a dependency tree as a formatted string.
// Uses charm.land/lipgloss/v2/tree for connectors and indentation. Per-row
// content is pre-styled with lipgloss; the resulting ANSI must NOT pass
// through Glamour's code-fence path (chroma strips ESC bytes — bt-x5xc4).
// Callers in markdown-rendering paths must splice this output past Glamour.
func RenderDependencyTree(node *DependencyNode) string {
	if node == nil {
		return "No dependency data."
	}

	t := tree.New().Root(formatTreeNode(node))
	for _, child := range node.Children {
		addTreeChild(t, child)
	}
	return "Dependency Graph:\n" + t.String()
}

// formatTreeNode formats a single tree row: unstyled type icon + lipgloss-
// styled body (ID, title, status icon, status text, dep type).
func formatTreeNode(node *DependencyNode) string {
	statusIcon := GetStatusIcon(node.Status)
	typeIcon := getDepTypeIcon(node.Type)
	title := truncateRunesHelper(node.Title, 40, "...")

	style := statusTreeStyle(node.Status)
	rowBody := style.Render(fmt.Sprintf("%s %s %s (%s) [%s]",
		node.ID,
		title,
		statusIcon,
		node.Status,
		node.Type,
	))
	return typeIcon + " " + rowBody
}

// addTreeChild appends a DependencyNode (and its descendants) to a lipgloss
// tree, preserving cycle markers and the natural-numeric ordering established
// by BuildDependencyTree.
func addTreeChild(parent *tree.Tree, node *DependencyNode) {
	if len(node.Children) == 0 {
		parent.Child(formatTreeNode(node))
		return
	}
	sub := tree.Root(formatTreeNode(node))
	for _, child := range node.Children {
		addTreeChild(sub, child)
	}
	parent.Child(sub)
}

// statusTreeStyle returns a lipgloss style appropriate for the given bead
// status in a dependency tree row. Closed nodes are dimmed so they recede
// visually; open/in_progress/blocked nodes are styled to pop. Uses package-
// level ColorStatus* vars so light/dark adaptation is automatic.
func statusTreeStyle(status string) lipgloss.Style {
	switch status {
	case "closed", "tombstone":
		// Recede: dim so closed work doesn't compete with still-open items.
		return lipgloss.NewStyle().Foreground(ColorStatusClosed).Faint(true)
	case "open":
		// Default: visible but not emphasized.
		return lipgloss.NewStyle().Foreground(ColorStatusOpen)
	case "in_progress":
		// Accent: bright to signal active work.
		return lipgloss.NewStyle().Foreground(ColorStatusInProgress).Bold(true)
	case "blocked":
		// Warning: red foreground to demand attention.
		return lipgloss.NewStyle().Foreground(ColorStatusBlocked).Bold(true)
	case "deferred":
		// Parked: italic + dim, distinct from both closed and open.
		return lipgloss.NewStyle().Foreground(ColorStatusDeferred).Italic(true).Faint(true)
	case "pinned":
		return lipgloss.NewStyle().Foreground(ColorStatusPinned)
	case "hooked":
		return lipgloss.NewStyle().Foreground(ColorStatusHooked)
	case "review":
		return lipgloss.NewStyle().Foreground(ColorStatusReview)
	default:
		return lipgloss.NewStyle().Foreground(ColorMuted)
	}
}

func getDepTypeIcon(depType string) string {
	switch depType {
	case "root":
		return "📍"
	case "blocks":
		return "⛔"
	case "related":
		return "🔗"
	case "parent-child", "child":
		return "📦"
	case "discovered-from":
		return "🔍"
	default:
		return "•"
	}
}

// GetStatusIcon returns a colored icon for a status
func GetStatusIcon(s string) string {
	switch s {
	case "open":
		return "🟢"
	case "in_progress":
		return "🔵"
	case "blocked":
		return "🔴"
	case "closed":
		return "⚫"
	case "deferred":
		return "❄"
	default:
		return "⚪"
	}
}

// GetPriorityIcon returns the emoji for a priority level
func GetPriorityIcon(priority int) string {
	switch priority {
	case 0:
		return "🔥" // Critical
	case 1:
		return "⚡" // High
	case 2:
		return "🔹" // Medium
	case 3:
		return "☕" // Low
	case 4:
		return "💤" // Backlog
	default:
		return "  "
	}
}

// GetPriorityLabel returns a compact text label for priority (P0, P1, etc.)
func GetPriorityLabel(priority int) string {
	if priority >= 0 && priority <= 4 {
		return fmt.Sprintf("P%d", priority)
	}
	return "P?"
}

// GetAgeDays returns the number of days since the given time
func GetAgeDays(t time.Time) int {
	if t.IsZero() {
		return 0
	}
	return int(time.Since(t).Hours() / 24)
}

// GetAgeColor returns a color based on staleness:
// green (<7 days), yellow (7-30 days), red (>30 days)
func GetAgeColor(t time.Time) color.Color {
	days := GetAgeDays(t)
	switch {
	case days < 7:
		return ColorSuccess // Green - fresh
	case days < 30:
		return ColorWarning // Yellow/Orange - aging
	default:
		return ColorDanger // Red - stale
	}
}

// FormatAgeBadge returns a compact age string with timer emoji (e.g., "3d ⏱")
func FormatAgeBadge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	days := GetAgeDays(t)
	switch {
	case days == 0:
		return "<1d"
	case days < 7:
		return fmt.Sprintf("%dd", days)
	case days < 30:
		return fmt.Sprintf("%dw", days/7)
	default:
		return fmt.Sprintf("%dmo", days/30)
	}
}
