package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"

	"charm.land/lipgloss/v2"
)

// GraphModel represents the dependency graph view with visual ASCII art visualization
type GraphModel struct {
	issues       []model.Issue
	issueMap     map[string]*model.Issue
	insights     *analysis.Insights
	selectedIdx  int
	scrollOffset int
	width        int
	height       int
	theme        Theme

	// Precomputed graph relationships
	blockers   map[string][]string // What each issue depends on (blocks this issue)
	dependents map[string][]string // What depends on each issue (this issue blocks)

	// Flat list for navigation
	sortedIDs []string

	// Precomputed rankings for all metrics (id -> rank, 1-indexed)
	rankPageRank     map[string]int
	rankBetweenness  map[string]int
	rankEigenvector  map[string]int
	rankHubs         map[string]int
	rankAuthorities  map[string]int
	rankCriticalPath map[string]int
	rankInDegree     map[string]int
	rankOutDegree    map[string]int
}

// NewGraphModel creates a new graph view from issues
func NewGraphModel(issues []model.Issue, insights *analysis.Insights, theme Theme) GraphModel {
	g := GraphModel{
		issues:   issues,
		insights: insights,
		theme:    theme,
	}
	g.rebuildGraph()
	return g
}

// SetSnapshot updates the graph data from a pre-built DataSnapshot (bv-za8z).
// This avoids rebuilding blockers/dependents and metric ranks on the UI thread.
func (g *GraphModel) SetSnapshot(snapshot *DataSnapshot) {
	if snapshot == nil {
		return
	}

	// Capture current selection
	var selectedID string
	if len(g.sortedIDs) > 0 && g.selectedIdx >= 0 && g.selectedIdx < len(g.sortedIDs) {
		selectedID = g.sortedIDs[g.selectedIdx]
	}

	g.issues = snapshot.Issues
	g.issueMap = snapshot.IssueMap
	g.insights = &snapshot.Insights

	if g.issueMap == nil {
		g.issueMap = make(map[string]*model.Issue, len(g.issues))
		for i := range g.issues {
			g.issueMap[g.issues[i].ID] = &g.issues[i]
		}
	}

	if snapshot.GraphLayout != nil && len(snapshot.GraphLayout.SortedIDs) > 0 {
		g.blockers = snapshot.GraphLayout.Blockers
		g.dependents = snapshot.GraphLayout.Dependents
		g.sortedIDs = snapshot.GraphLayout.SortedIDs

		g.rankPageRank = snapshot.GraphLayout.RankPageRank
		g.rankBetweenness = snapshot.GraphLayout.RankBetweenness
		g.rankEigenvector = snapshot.GraphLayout.RankEigenvector
		g.rankHubs = snapshot.GraphLayout.RankHubs
		g.rankAuthorities = snapshot.GraphLayout.RankAuthorities
		g.rankCriticalPath = snapshot.GraphLayout.RankCriticalPath
		g.rankInDegree = snapshot.GraphLayout.RankInDegree
		g.rankOutDegree = snapshot.GraphLayout.RankOutDegree
	} else {
		g.rebuildGraph()
	}

	// Restore selection
	if selectedID != "" {
		found := false
		for i, id := range g.sortedIDs {
			if id == selectedID {
				g.selectedIdx = i
				found = true
				break
			}
		}
		if !found && g.selectedIdx >= len(g.sortedIDs) {
			g.selectedIdx = 0
		}
	}
	if g.selectedIdx >= len(g.sortedIDs) {
		g.selectedIdx = 0
	}
}

// SetIssues updates the graph data preserving the selected issue if possible
func (g *GraphModel) SetIssues(issues []model.Issue, insights *analysis.Insights) {
	// Capture current selection
	var selectedID string
	if len(g.sortedIDs) > 0 && g.selectedIdx >= 0 && g.selectedIdx < len(g.sortedIDs) {
		selectedID = g.sortedIDs[g.selectedIdx]
	}

	g.issues = issues
	g.insights = insights
	g.rebuildGraph()

	// Restore selection
	if selectedID != "" {
		// Try to find the previously selected ID in the new list
		found := false
		for i, id := range g.sortedIDs {
			if id == selectedID {
				g.selectedIdx = i
				found = true
				break
			}
		}
		// If not found (e.g. filter changed or issue deleted), selectedIdx
		// was reset to 0 or clamped in rebuildGraph, which is acceptable behavior.
		if !found {
			// Ensure we don't end up out of bounds if sortedIDs shrank
			if g.selectedIdx >= len(g.sortedIDs) {
				g.selectedIdx = 0
			}
		}
	}
}

func (g *GraphModel) rebuildGraph() {
	size := len(g.issues)
	g.issueMap = make(map[string]*model.Issue, size)
	g.blockers = make(map[string][]string, size)
	g.dependents = make(map[string][]string, size)
	g.sortedIDs = make([]string, 0, size)

	for i := range g.issues {
		issue := &g.issues[i]
		g.issueMap[issue.ID] = issue
		g.sortedIDs = append(g.sortedIDs, issue.ID)
	}

	// Build relationships
	for _, issue := range g.issues {
		for _, dep := range issue.Dependencies {
			if dep != nil && dep.Type.IsGraphEdge() {
				g.blockers[issue.ID] = append(g.blockers[issue.ID], dep.DependsOnID)
				g.dependents[dep.DependsOnID] = append(g.dependents[dep.DependsOnID], issue.ID)
			}
		}
	}

	// Compute rankings for all metrics
	g.computeRankings()

	// Sort by critical path score if available, else by ID
	if g.insights != nil && g.insights.Stats != nil {
		sort.Slice(g.sortedIDs, func(i, j int) bool {
			scoreI := g.insights.Stats.GetCriticalPathScore(g.sortedIDs[i])
			scoreJ := g.insights.Stats.GetCriticalPathScore(g.sortedIDs[j])
			if scoreI != scoreJ {
				return scoreI > scoreJ
			}
			return g.sortedIDs[i] < g.sortedIDs[j]
		})
	} else {
		sort.Strings(g.sortedIDs)
	}

	if g.selectedIdx >= len(g.sortedIDs) {
		g.selectedIdx = 0
	}
}

// computeRankings precomputes rankings for all metrics
func (g *GraphModel) computeRankings() {
	g.rankPageRank = nil
	g.rankBetweenness = nil
	g.rankEigenvector = nil
	g.rankHubs = nil
	g.rankAuthorities = nil
	g.rankCriticalPath = nil
	g.rankInDegree = nil
	g.rankOutDegree = nil

	if g.insights == nil || g.insights.Stats == nil {
		return
	}

	stats := g.insights.Stats

	// Reuse precomputed ranks from analysis (computed in Phase 1/2).
	g.rankPageRank = stats.PageRankRank()
	g.rankBetweenness = stats.BetweennessRank()
	g.rankEigenvector = stats.EigenvectorRank()
	g.rankHubs = stats.HubsRank()
	g.rankAuthorities = stats.AuthoritiesRank()
	g.rankCriticalPath = stats.CriticalPathRank()
	g.rankInDegree = stats.InDegreeRank()
	g.rankOutDegree = stats.OutDegreeRank()
}

// Navigation
func (g *GraphModel) MoveUp() {
	if g.selectedIdx > 0 {
		g.selectedIdx--

	}
}

func (g *GraphModel) MoveDown() {
	if g.selectedIdx < len(g.sortedIDs)-1 {
		g.selectedIdx++

	}
}

func (g *GraphModel) MoveLeft()  { g.MoveUp() }
func (g *GraphModel) MoveRight() { g.MoveDown() }

func (g *GraphModel) PageUp() {
	g.selectedIdx -= 10
	if g.selectedIdx < 0 {
		g.selectedIdx = 0
	}
}

func (g *GraphModel) PageDown() {
	if len(g.sortedIDs) == 0 {
		return
	}
	g.selectedIdx += 10
	if g.selectedIdx >= len(g.sortedIDs) {
		g.selectedIdx = len(g.sortedIDs) - 1
	}
}

func (g *GraphModel) SelectedIssue() *model.Issue {
	if len(g.sortedIDs) == 0 {
		return nil
	}
	id := g.sortedIDs[g.selectedIdx]
	return g.issueMap[id]
}

// SelectByID selects an issue by its ID (bv-xf4p)
func (g *GraphModel) SelectByID(id string) bool {
	for i, sortedID := range g.sortedIDs {
		if sortedID == id {
			g.selectedIdx = i
	
			return true
		}
	}
	return false
}

func (g *GraphModel) TotalCount() int {
	return len(g.sortedIDs)
}

// View renders the visual graph view
func (g *GraphModel) View(width, height int) string {
	g.width = width
	g.height = height
	t := g.theme

	if len(g.sortedIDs) == 0 {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(t.Secondary).
			Render("No issues to display")
	}

	selectedID := g.sortedIDs[g.selectedIdx]
	selectedIssue := g.issueMap[selectedID]
	if selectedIssue == nil {
		return "Error: selected issue not found"
	}

	// Layout: Left panel (node list) | Right panel (visual graph + metrics)
	listWidth := 28
	if width < 120 {
		listWidth = 24
	}
	if width < 80 {
		// Narrow: just show visual graph
		return g.renderVisualGraph(selectedID, selectedIssue, width, height, t)
	}

	detailWidth := width - listWidth - 3

	// Left: scrollable list of all nodes
	listView := g.renderNodeList(listWidth, height-2, t)

	// Right: visual graph + metrics
	graphView := g.renderVisualGraph(selectedID, selectedIssue, detailWidth, height-2, t)

	// Combine with separator
	sepHeight := height - 2
	if sepHeight < 1 {
		sepHeight = 1
	}
	separator := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Render(strings.Repeat("│\n", sepHeight))

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, separator, graphView)
}

// renderNodeList renders the left panel with all nodes
func (g *GraphModel) renderNodeList(width, height int, t Theme) string {
	var lines []string

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Width(width)
	lines = append(lines, headerStyle.Render(fmt.Sprintf("📊 Nodes (%d)", len(g.sortedIDs))))
	lines = append(lines, strings.Repeat("─", width))

	visibleItems := height - 4
	if visibleItems < 1 {
		visibleItems = 1
	}

	startIdx := g.scrollOffset
	if g.selectedIdx < startIdx {
		startIdx = g.selectedIdx
	} else if g.selectedIdx >= startIdx+visibleItems {
		startIdx = g.selectedIdx - visibleItems + 1
	}
	g.scrollOffset = startIdx

	endIdx := startIdx + visibleItems
	if endIdx > len(g.sortedIDs) {
		endIdx = len(g.sortedIDs)
	}

	for i := startIdx; i < endIdx; i++ {
		id := g.sortedIDs[i]
		issue := g.issueMap[id]
		if issue == nil {
			continue
		}

		isSelected := i == g.selectedIdx
		statusIcon := getStatusIcon(issue.Status)
		maxIDLen := width - 4
		displayID := smartTruncateID(id, maxIDLen)
		line := fmt.Sprintf("%s %s", statusIcon, displayID)

		var style lipgloss.Style
		if isSelected {
			style = lipgloss.NewStyle().
				Bold(true).
				Foreground(t.Primary).
				Background(t.Highlight).
				Width(width)
		} else {
			style = lipgloss.NewStyle().
				Foreground(t.GetStatusColor(string(issue.Status))).
				Width(width)
		}
		lines = append(lines, style.Render(line))
	}

	if len(g.sortedIDs) > visibleItems {
		scrollInfo := fmt.Sprintf("(%d-%d of %d)", startIdx+1, endIdx, len(g.sortedIDs))
		scrollStyle := lipgloss.NewStyle().
			Foreground(t.Secondary).
			Italic(true).
			Width(width).
			Align(lipgloss.Center)
		lines = append(lines, scrollStyle.Render(scrollInfo))
	}

	return strings.Join(lines, "\n")
}

// renderVisualGraph renders the ASCII art graph visualization with metrics
func (g *GraphModel) renderVisualGraph(id string, issue *model.Issue, width, height int, t Theme) string {
	var sections []string

	blockerIDs := g.blockers[id]
	dependentIDs := g.dependents[id]

	// ═══════════════════════════════════════════════════════════════════════
	// BLOCKERS SECTION (what this issue depends on)
	// ═══════════════════════════════════════════════════════════════════════
	if len(blockerIDs) > 0 {
		sections = append(sections, g.renderBlockersVisual(blockerIDs, width, t))
		// Connecting lines down to ego
		sections = append(sections, g.renderConnectorDown(len(blockerIDs), width, t))
	}

	// ═══════════════════════════════════════════════════════════════════════
	// EGO NODE (selected issue) - prominent center box
	// ═══════════════════════════════════════════════════════════════════════
	sections = append(sections, g.renderEgoNode(id, issue, width, t))

	// ═══════════════════════════════════════════════════════════════════════
	// DEPENDENTS SECTION (what depends on this issue)
	// ═══════════════════════════════════════════════════════════════════════
	if len(dependentIDs) > 0 {
		// Connecting lines down from ego
		sections = append(sections, g.renderConnectorDown(len(dependentIDs), width, t))
		sections = append(sections, g.renderDependentsVisual(dependentIDs, width, t))
	}

	sections = append(sections, "")

	// ═══════════════════════════════════════════════════════════════════════
	// COMPREHENSIVE METRICS PANEL - ALL 8 metrics with values AND ranks
	// ═══════════════════════════════════════════════════════════════════════
	sections = append(sections, g.renderMetricsPanel(id, width, t))

	// Navigation hint
	navStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Italic(true)
	sections = append(sections, "")
	sections = append(sections, navStyle.Render("j/k: navigate • enter: view details • g: back to list"))

	return strings.Join(sections, "\n")
}

// renderBlockersVisual renders blocker nodes as boxes
func (g *GraphModel) renderBlockersVisual(blockerIDs []string, width int, t Theme) string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Feature).
		Width(width).
		Align(lipgloss.Center)

	header := headerStyle.Render("▲ BLOCKED BY (must complete first) ▲")

	// Calculate box width based on available space and number of blockers
	maxBoxes := 5
	if len(blockerIDs) < maxBoxes {
		maxBoxes = len(blockerIDs)
	}
	if maxBoxes < 1 {
		maxBoxes = 1
	}
	boxWidth := (width - 4) / maxBoxes
	if boxWidth > 20 {
		boxWidth = 20
	}
	if boxWidth < 12 {
		boxWidth = 12
	}
	// Ensure boxWidth doesn't exceed available space (narrow terminals)
	if boxWidth > width-2 {
		boxWidth = width - 2
	}
	if boxWidth < 8 {
		boxWidth = 8
	}

	var boxes []string
	for i, bid := range blockerIDs {
		if i >= 5 {
			remaining := len(blockerIDs) - 5
			boxes = append(boxes, lipgloss.NewStyle().
				Foreground(t.Secondary).
				Italic(true).
				Render(fmt.Sprintf("+%d more", remaining)))
			break
		}
		boxes = append(boxes, g.renderNodeBox(bid, boxWidth, t, false))
	}

	boxRow := lipgloss.JoinHorizontal(lipgloss.Center, boxes...)
	centered := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(boxRow)

	return header + "\n" + centered
}

// renderDependentsVisual renders dependent nodes as boxes
func (g *GraphModel) renderDependentsVisual(dependentIDs []string, width int, t Theme) string {
	maxBoxes := 5
	if len(dependentIDs) < maxBoxes {
		maxBoxes = len(dependentIDs)
	}
	if maxBoxes < 1 {
		maxBoxes = 1
	}
	boxWidth := (width - 4) / maxBoxes
	if boxWidth > 20 {
		boxWidth = 20
	}
	if boxWidth < 12 {
		boxWidth = 12
	}
	// Ensure boxWidth doesn't exceed available space (narrow terminals)
	if boxWidth > width-2 {
		boxWidth = width - 2
	}
	if boxWidth < 8 {
		boxWidth = 8
	}

	var boxes []string
	for i, did := range dependentIDs {
		if i >= 5 {
			remaining := len(dependentIDs) - 5
			boxes = append(boxes, lipgloss.NewStyle().
				Foreground(t.Secondary).
				Italic(true).
				Render(fmt.Sprintf("+%d more", remaining)))
			break
		}
		boxes = append(boxes, g.renderNodeBox(did, boxWidth, t, false))
	}

	boxRow := lipgloss.JoinHorizontal(lipgloss.Center, boxes...)
	centered := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(boxRow)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Feature).
		Width(width).
		Align(lipgloss.Center)

	header := headerStyle.Render("▼ BLOCKS (waiting on this) ▼")

	return centered + "\n" + header
}

// renderNodeBox renders a single node as an ASCII box
func (g *GraphModel) renderNodeBox(id string, boxWidth int, t Theme, isEgo bool) string {
	issue := g.issueMap[id]

	var statusIcon, displayID, title string
	var statusColor AdaptiveColor

	if issue != nil {
		statusIcon = getStatusIcon(issue.Status)
		statusColor = t.GetStatusColor(string(issue.Status))
		displayID = smartTruncateID(id, boxWidth-4)
		if issue.Title != "" {
			title = truncateRunesHelper(issue.Title, boxWidth-4, "…")
		}
	} else {
		statusIcon = "❓"
		statusColor = t.Secondary
		displayID = smartTruncateID(id, boxWidth-4)
		title = "(not in filter)"
	}

	// Build box content
	line1 := fmt.Sprintf("%s %s", statusIcon, displayID)

	var boxStyle lipgloss.Style
	if isEgo {
		// Ego node gets double-line border and highlight
		boxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(t.Primary).
			Foreground(t.Primary).
			Bold(true).
			Width(boxWidth).
			Align(lipgloss.Center).
			Padding(0, 1)
	} else {
		boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(statusColor).
			Foreground(statusColor).
			Width(boxWidth).
			Align(lipgloss.Center).
			Padding(0, 0)
	}

	content := line1
	if title != "" && boxWidth > 14 {
		content = line1 + "\n" + title
	}

	return boxStyle.Render(content)
}

// renderEgoNode renders the selected/ego node prominently
func (g *GraphModel) renderEgoNode(id string, issue *model.Issue, width int, t Theme) string {
	statusIcon := getStatusIcon(issue.Status)
	prioIcon := getPriorityIcon(issue.Priority)
	typeIcon := getTypeIcon(issue.IssueType)

	egoWidth := width / 2
	if egoWidth > 50 {
		egoWidth = 50
	}
	if egoWidth < 30 {
		egoWidth = 30
	}
	// Don't exceed available width
	if egoWidth > width-4 {
		egoWidth = width - 4
	}
	if egoWidth < 10 {
		egoWidth = 10
	}

	icons := fmt.Sprintf("%s %s %s", statusIcon, prioIcon, typeIcon)
	displayID := smartTruncateID(id, egoWidth-4)
	title := ""
	if issue.Title != "" {
		title = truncateRunesHelper(issue.Title, egoWidth-4, "…")
	}

	content := icons + " " + displayID
	if title != "" {
		content += "\n" + title
	}

	// Add connection counts
	blockerCount := len(g.blockers[id])
	dependentCount := len(g.dependents[id])
	content += fmt.Sprintf("\n⬆%d  ⬇%d", blockerCount, dependentCount)

	egoStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(t.Primary).
		Foreground(t.Primary).
		Bold(true).
		Width(egoWidth).
		Align(lipgloss.Center).
		Padding(0, 1)

	box := egoStyle.Render(content)

	// Center the ego box
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(box)
}

// renderConnectorDown renders connector lines between sections
func (g *GraphModel) renderConnectorDown(count int, width int, t Theme) string {
	if count == 0 {
		return ""
	}

	connStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Width(width).
		Align(lipgloss.Center)

	if count == 1 {
		return connStyle.Render("│\n│\n▼")
	}

	// Multiple connections - fan pattern using proper rune slicing
	// Pattern chars: ├ ─ ┼ ─ ┼ ─ ┤ (for 3 connections)
	lines := []string{"│"}

	// Build the connector pattern properly
	var pattern strings.Builder
	pattern.WriteRune('├')
	for i := 0; i < count && i < 4; i++ {
		if i > 0 {
			pattern.WriteRune('┼')
		}
		pattern.WriteRune('─')
	}
	pattern.WriteRune('┤')
	lines = append(lines, pattern.String())
	lines = append(lines, "▼")

	return connStyle.Render(strings.Join(lines, "\n"))
}

// renderMetricsPanel renders ALL graph metrics with polished visualization
func (g *GraphModel) renderMetricsPanel(id string, width int, t Theme) string {
	total := len(g.sortedIDs)

	// ══════════════════════════════════════════════════════════════════════════
	// POLISHED METRICS PANEL - Stripe-level visual design
	// ══════════════════════════════════════════════════════════════════════════

	// Panel header with accent background
	panelHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorText).
		Background(ColorPrimary).
		Padding(0, 2).
		Width(width - 4)

	panelTitle := panelHeaderStyle.Render("📊 GRAPH METRICS")

	if g.insights == nil || g.insights.Stats == nil {
		noDataStyle := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true).
			Padding(1, 2).
			Width(width - 4).
			Align(lipgloss.Center)
		return panelTitle + "\n" + noDataStyle.Render("No graph analysis data available")
	}

	stats := g.insights.Stats

	// Get all values and ranks (using thread-safe accessors for Phase 2 data)
	pageRank := stats.GetPageRankScore(id)
	betweenness := stats.GetBetweennessScore(id)
	eigenvector := stats.GetEigenvectorScore(id)
	hubs := stats.GetHubScore(id)
	authorities := stats.GetAuthorityScore(id)
	critPath := stats.GetCriticalPathScore(id)
	inDeg := float64(stats.InDegree[id])
	outDeg := float64(stats.OutDegree[id])

	rankPR := g.rankPageRank[id]
	rankBW := g.rankBetweenness[id]
	rankEV := g.rankEigenvector[id]
	rankHub := g.rankHubs[id]
	rankAuth := g.rankAuthorities[id]
	rankCP := g.rankCriticalPath[id]
	rankIn := g.rankInDegree[id]
	rankOut := g.rankOutDegree[id]

	// Default ranks to total if 0
	if rankPR == 0 {
		rankPR = total
	}
	if rankBW == 0 {
		rankBW = total
	}
	if rankEV == 0 {
		rankEV = total
	}
	if rankHub == 0 {
		rankHub = total
	}
	if rankAuth == 0 {
		rankAuth = total
	}
	if rankCP == 0 {
		rankCP = total
	}
	if rankIn == 0 {
		rankIn = total
	}
	if rankOut == 0 {
		rankOut = total
	}

	// Helper to render a metric row with mini-bar visualization
	renderMetricRow := func(name string, value float64, rank int, maxVal float64, isInt bool) string {
		// Name with fixed width
		nameStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Width(14)

		// Value formatting
		var valStr string
		if isInt {
			valStr = fmt.Sprintf("%d", int(value))
		} else if value >= 1.0 {
			valStr = fmt.Sprintf("%.2f", value)
		} else {
			valStr = fmt.Sprintf("%.4f", value)
		}
		valueStyle := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Width(8).Align(lipgloss.Right)

		// Mini-bar for relative importance (normalize to 0-1)
		normalized := 0.0
		if maxVal > 0 {
			normalized = value / maxVal
		}
		bar := RenderMiniBar(normalized, 6, t)

		// Rank badge
		rankBadge := RenderRankBadge(rank, total)

		return nameStyle.Render(name) + " " + valueStyle.Render(valStr) + " " + bar + " " + rankBadge
	}

	// Find max values for normalization (using thread-safe accessors)
	maxCP, maxPR, maxBW, maxEV := 0.0, 0.0, 0.0, 0.0
	maxHub, maxAuth, maxIn, maxOut := 0.0, 0.0, 0.0, 0.0
	for _, issueID := range g.sortedIDs {
		if v := stats.GetCriticalPathScore(issueID); v > maxCP {
			maxCP = v
		}
		if v := stats.GetPageRankScore(issueID); v > maxPR {
			maxPR = v
		}
		if v := stats.GetBetweennessScore(issueID); v > maxBW {
			maxBW = v
		}
		if v := stats.GetEigenvectorScore(issueID); v > maxEV {
			maxEV = v
		}
		if v := stats.GetHubScore(issueID); v > maxHub {
			maxHub = v
		}
		if v := stats.GetAuthorityScore(issueID); v > maxAuth {
			maxAuth = v
		}
		if v := float64(stats.InDegree[issueID]); v > maxIn {
			maxIn = v
		}
		if v := float64(stats.OutDegree[issueID]); v > maxOut {
			maxOut = v
		}
	}

	var rows []string
	rows = append(rows, panelTitle)
	rows = append(rows, RenderDivider(width-4))

	// Section: Importance Metrics
	sectionStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Padding(0, 1)
	rows = append(rows, sectionStyle.Render("Importance"))
	rows = append(rows, "  "+renderMetricRow("Critical Path", critPath, rankCP, maxCP, false))
	rows = append(rows, "  "+renderMetricRow("PageRank", pageRank, rankPR, maxPR, false))
	rows = append(rows, "  "+renderMetricRow("Eigenvector", eigenvector, rankEV, maxEV, false))

	rows = append(rows, "")

	// Section: Flow Metrics
	rows = append(rows, sectionStyle.Render("Flow & Connectivity"))
	rows = append(rows, "  "+renderMetricRow("Betweenness", betweenness, rankBW, maxBW, false))
	rows = append(rows, "  "+renderMetricRow("Hub Score", hubs, rankHub, maxHub, false))
	rows = append(rows, "  "+renderMetricRow("Authority", authorities, rankAuth, maxAuth, false))

	rows = append(rows, "")

	// Section: Degree
	rows = append(rows, sectionStyle.Render("Connections"))
	rows = append(rows, "  "+renderMetricRow("In-Degree", inDeg, rankIn, maxIn, true))
	rows = append(rows, "  "+renderMetricRow("Out-Degree", outDeg, rankOut, maxOut, true))

	rows = append(rows, "")

	// Legend
	legendStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Italic(true).
		Width(width - 4)

	rows = append(rows, legendStyle.Render("█ relative score │ #N rank of "+fmt.Sprintf("%d", total)+" issues"))

	return strings.Join(rows, "\n")
}

// Graph-specific icon helpers. These intentionally use different emoji sets than
// the public helpers in helpers.go and theme.go. The graph panel uses a distinct
// visual language (e.g. colored circles for status, different priority/type icons)
// tuned for the compact node rendering context. If these should be unified with
// the board/tree icon sets, consolidate into the public helpers and update both.

func getStatusIcon(status model.Status) string {
	switch {
	case isClosedLikeStatus(status):
		return "✅"
	case status == model.StatusOpen:
		return "🔵"
	case status == model.StatusInProgress:
		return "🟡"
	case status == model.StatusBlocked:
		return "🔴"
	case status == model.StatusDeferred:
		return "⏸️"
	case status == model.StatusPinned:
		return "📌"
	case status == model.StatusHooked:
		return "🪝"
	default:
		return "⚪"
	}
}

func getPriorityIcon(priority int) string {
	switch priority {
	case 1:
		return "🔥"
	case 2:
		return "⚡"
	case 3:
		return "📌"
	case 4:
		return "📋"
	default:
		return "  "
	}
}

func getTypeIcon(itype model.IssueType) string {
	switch itype {
	case model.TypeBug:
		return "🐛"
	case model.TypeFeature:
		return "✨"
	case model.TypeTask:
		return "📝"
	case model.TypeEpic:
		return "🎯"
	case model.TypeChore:
		return "🔧"
	default:
		return "📄"
	}
}

func smartTruncateID(id string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	clamp := func(s string) string {
		r := []rune(s)
		if len(r) <= maxLen {
			return s
		}
		if maxLen == 1 {
			return string(r[:1])
		}
		return string(r[:maxLen-1]) + "…"
	}

	runes := []rune(id)
	if len(runes) <= maxLen {
		return id
	}

	// Split by common separators to abbreviate parts
	f := func(c rune) bool {
		return c == '_' || c == '-'
	}
	parts := strings.FieldsFunc(id, f)

	sep := "_"
	if strings.Contains(id, "-") && !strings.Contains(id, "_") {
		sep = "-"
	}

	if len(parts) > 2 {
		var abbrev strings.Builder
		runeCount := 0
		for i, part := range parts {
			partRunes := []rune(part)
			if i == len(parts)-1 {
				// Last part: keep as much as possible
				remaining := maxLen - runeCount
				if remaining > 0 {
					if len(partRunes) <= remaining {
						abbrev.WriteString(part)
					} else if remaining > 1 {
						abbrev.WriteString(string(partRunes[:remaining-1]))
						abbrev.WriteRune('…')
					} else {
						abbrev.WriteRune('…')
					}
				}
			} else {
				// Non-last parts: just first char + separator
				if len(partRunes) > 0 {
					abbrev.WriteRune(partRunes[0])
					abbrev.WriteString(sep)
					runeCount += 1 + len(sep)
				}
			}
		}
		result := abbrev.String()
		return clamp(result)
	}

	// Fallback: simple truncation
	return clamp(string(runes))
}
