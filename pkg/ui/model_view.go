package ui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m Model) renderLoadingScreen() string {
	frame := workerSpinnerFrames[0]
	if m.data.backgroundWorker != nil && m.data.backgroundWorker.State() == WorkerProcessing {
		frame = workerSpinnerFrames[m.data.workerSpinnerIdx%len(workerSpinnerFrames)]
	}

	spinnerStyle := lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
	titleStyle := lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	subStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	lines := []string{
		spinnerStyle.Render(frame),
		"",
		titleStyle.Render("Loading beads..."),
	}
	if m.data.beadsPath != "" {
		lines = append(lines, "", subStyle.Render(m.data.beadsPath))
	}

	content := lipgloss.JoinVertical(lipgloss.Center, lines...)
	return lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) View() tea.View {
	if !m.ready {
		return tea.NewView("Initializing...")
	}

	var body string

	// Modal overlays take highest priority - dispatch by activeModal
	switch m.activeModal {
	case ModalQuitConfirm:
		body = m.renderQuitConfirm()
	case ModalAgentPrompt:
		body = m.agentPromptModal.CenterModal(m.width, m.height-1)
	case ModalCassSession:
		body = m.cassModal.CenterModal(m.width, m.height-1)
	case ModalUpdate:
		body = m.updateModal.CenterModal(m.width, m.height-1)
	case ModalLabelHealthDetail:
		if m.labelHealthDetail != nil {
			body = m.renderLabelHealthDetail(*m.labelHealthDetail)
		}
	case ModalLabelGraphAnalysis:
		if m.labelGraphAnalysisResult != nil {
			body = m.renderLabelGraphAnalysis()
		}
	case ModalLabelDrilldown:
		if m.labelDrilldownLabel != "" {
			body = m.renderLabelDrilldown()
		}
	case ModalAlerts:
		// Handled as overlay after background renders (below)
	case ModalTimeTravelInput:
		body = m.renderTimeTravelPrompt()
	case ModalBQLQuery:
		body = m.bqlQuery.View()
	case ModalRecipePicker:
		body = m.recipePicker.View()
	case ModalRepoPicker:
		// Handled as overlay after background renders (below)
	case ModalLabelPicker:
		// Handled as overlay after background renders (below)
	case ModalHelp:
		body = m.renderHelpOverlay()
	case ModalTutorial:
		body = m.tutorialModel.View()
	case ModalNone:
		// No modal - fall through to view routing below
	}

	// If no modal rendered content, route by view mode
	if body == "" {
		if m.data.snapshotInitPending && m.data.snapshot == nil {
			body = m.renderLoadingScreen()
		} else {
			// Route by ViewMode enum
			switch m.mode {
			case ViewInsights, ViewAttention:
				m.insightsPanel.SetSize(m.width, m.height-1)
				body = m.insightsPanel.View()
			case ViewFlowMatrix:
				m.flowMatrix.SetSize(m.width, m.height-1)
				body = m.flowMatrix.View()
			case ViewTree:
				m.tree.SetSize(m.width, m.height-1)
				body = m.tree.View()
			case ViewGraph:
				body = m.graphView.View(m.width, m.height-1)
			case ViewBoard:
				body = m.board.View(m.width, m.height-1)
			case ViewActionable:
				m.actionableView.SetSize(m.width, m.height-2)
				body = m.actionableView.Render()
			case ViewHistory:
				m.historyView.SetSize(m.width, m.height-1)
				body = m.historyView.View()
			case ViewSprint:
				body = m.sprintViewText
			case ViewLabelDashboard:
				m.labelDashboard.SetSize(m.width, m.height-1)
				body = m.labelDashboard.View()
			default: // ViewList
				if m.isSplitView {
					body = m.renderSplitView()
				} else if m.showDetails {
					body = m.viewport.View()
				} else {
					body = m.renderListWithHeader()
				}
			}
		}
	}

	// Add shortcuts sidebar if enabled (bv-3qi5)
	if m.showShortcutsSidebar {
		// Update sidebar context based on current focus
		m.shortcutsSidebar.SetContext(ContextFromFocus(m.focused))
		m.shortcutsSidebar.SetSize(m.shortcutsSidebar.Width(), m.height-2)
		sidebar := m.shortcutsSidebar.View()
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, sidebar)
	}

	// Overlay modals that float on top of the background
	if m.activeModal == ModalRepoPicker {
		body = OverlayCenter(body, m.repoPicker.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalLabelPicker {
		body = OverlayCenter(body, m.labelPicker.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalAlerts {
		body = OverlayCenter(body, m.renderAlertsPanel(), m.width, m.height-1)
	}

	footer := m.renderFooter()

	// Ensure the final output fits exactly in the terminal height
	// This prevents the header from being pushed off the top
	finalStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		MaxHeight(m.height)

	v := tea.NewView(finalStyle.Render(lipgloss.JoinVertical(lipgloss.Left, body, footer)))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) renderQuitConfirm() string {
	t := m.theme

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.Blocked).
		Padding(1, 3).
		Align(lipgloss.Center)

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Blocked).
		Bold(true)

	textStyle := lipgloss.NewStyle().
		Foreground(t.Base.GetForeground())

	keyStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	content := titleStyle.Render("Quit bt?") + "\n\n" +
		textStyle.Render("Press ") + keyStyle.Render("Esc") + textStyle.Render(" or ") + keyStyle.Render("Y") + textStyle.Render(" to quit\n") +
		textStyle.Render("Press any other key to cancel")

	box := boxStyle.Render(content)

	return lipgloss.Place(
		m.width,
		m.height-1,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)
}

// renderSearchRow returns the always-present, one-line search row that lives
// directly above the list's column header (bt-fxbl). It bridges all three
// filter states with a fixed-height row so the column header position never
// shifts as the user types, commits, or clears the filter:
//
//   - Unfiltered:    discreet placeholder hint ("/  search   <count> beads")
//   - Filtering:     live FilterInput rendered via m.list.FilterInput.View()
//                    (with our own prompt + cursor shown); count of running
//                    matches on the right.
//   - FilterApplied: committed query + match count on the right (the original
//                    "search pill" behavior, bt-031h).
//
// Why we own this rather than letting Bubbles render its titleView: Bubbles'
// built-in title row sits BELOW our column header strip, and during Filtering
// it shows the FilterInput there — visibly shifting the column header by one
// row relative to FilterApplied (where the pill renders ABOVE). Suppressing
// Bubbles' titleView via SetShowFilter(false) + SetShowTitle(false) and
// rendering this row ourselves above the column header makes chrome height
// constant across states. Width is the row width to fill.
func (m Model) renderSearchRow(width int) string {
	t := m.theme
	state := m.list.FilterState()
	totalItems := len(m.list.Items())
	visibleItems := len(m.list.VisibleItems())

	hintStyle := lipgloss.NewStyle().Foreground(t.Subtext).Italic(true)
	labelStyle := lipgloss.NewStyle().Foreground(t.Muted)
	queryStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	countStyle := lipgloss.NewStyle().Foreground(t.Muted)

	var left, right string

	switch state {
	case list.Filtering:
		// Live editing: render the FilterInput directly so the cursor + typed
		// chars are visible. The prompt "Search: " is set on l.FilterInput in
		// model.go (bt-imcn). FilterInput.View() handles cursor blink for us.
		left = "  " + m.list.FilterInput.View()
		// Show running match count if we have a query.
		query := strings.TrimSpace(m.list.FilterInput.Value())
		if query != "" {
			right = countStyle.Render(fmt.Sprintf("  %d/%d matches  ", visibleItems, totalItems))
		}
	case list.FilterApplied:
		query := strings.TrimSpace(m.list.FilterInput.Value())
		if query == "" {
			// Edge: applied with empty query — fall through to placeholder.
			left = labelStyle.Render("  Search: ") + hintStyle.Render("/")
			right = countStyle.Render(fmt.Sprintf("  %d  ", totalItems))
		} else {
			left = labelStyle.Render("  Search: ") + queryStyle.Render(query)
			right = countStyle.Render(fmt.Sprintf("  %d/%d matches  ", visibleItems, totalItems))
		}
	default: // list.Unfiltered
		left = labelStyle.Render("  Search: ") + hintStyle.Render("/")
		right = countStyle.Render(fmt.Sprintf("  %d  ", totalItems))
	}

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := width - leftWidth - rightWidth

	// Overflow path: prefer keeping the typed query visible. Drop the right
	// (count) first, then if still too wide clip the left to width. Without
	// this, lipgloss wraps the row to 2 lines and breaks the 1-row chrome
	// invariant in splitViewListChromeHeight (bt-m6cd).
	if gap < 1 {
		right = ""
		rightWidth = 0
		gap = width - leftWidth
		if gap < 0 {
			return lipgloss.NewStyle().MaxWidth(width).Render(left)
		}
	}

	out := left + strings.Repeat(" ", gap) + right
	// Defensive final clip in case styled-content widths drift from our math.
	return lipgloss.NewStyle().MaxWidth(width).Render(out)
}

// splitViewHeader renders the split-view list column header ("TYPE PRI STATUS
// ID TITLE" strip). Extracted so splitViewListChromeHeight can measure the
// actual rendered height via lipgloss.Height — lipgloss Style.Width only sets
// background fill and does NOT truncate long text, so at narrow pane widths
// the literal header would wrap to a second row, putting mouse click math
// off by 1 (bt-i138, bt-ej61). Clip to fit before rendering.
func (m Model) splitViewHeader() string {
	t := m.theme
	listInnerWidth := m.list.Width()

	headerStyle := lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(ColorBgContrast).
		Bold(true).
		Width(listInnerWidth)

	headerText := "  TYPE PRI STATUS      ID                     TITLE"
	if listInnerWidth > 0 && len(headerText) > listInnerWidth {
		headerText = headerText[:listInnerWidth]
	}
	return headerStyle.Render(headerText)
}

func (m Model) renderListWithHeader() string {
	t := m.theme

	// Calculate dimensions based on actual list height set in sizing
	availableHeight := m.list.Height()
	if availableHeight == 0 {
		availableHeight = m.height - 3 // fallback
	}

	// Render column header. Clip to width; lipgloss Style.Width sets background
	// fill but does NOT truncate, so at narrow widths the literal text would
	// wrap to a second row (bt-i138).
	headerWidth := m.width - 2
	headerStyle := lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(ColorBgContrast).
		Bold(true).
		Width(headerWidth)

	headerText := "  TYPE PRI STATUS      ID                                   TITLE"
	if m.workspaceMode {
		// Account for repo badges like [API] shown in workspace mode.
		headerText = "  REPO TYPE PRI STATUS      ID                               TITLE"
	}
	if headerWidth > 0 && len(headerText) > headerWidth {
		headerText = headerText[:headerWidth]
	}
	header := headerStyle.Render(headerText)

	// Page info
	totalItems := len(m.list.Items())
	currentIdx := m.list.Index()
	itemsPerPage := availableHeight
	if itemsPerPage < 1 {
		itemsPerPage = 1
	}
	currentPage := (currentIdx / itemsPerPage) + 1
	totalPages := (totalItems + itemsPerPage - 1) / itemsPerPage
	if totalPages < 1 {
		totalPages = 1
	}
	startItem := 0
	endItem := 0
	if totalItems > 0 {
		startItem = (currentPage-1)*itemsPerPage + 1
		endItem = startItem + itemsPerPage - 1
		if endItem > totalItems {
			endItem = totalItems
		}
	}

	pageInfo := fmt.Sprintf(" Page %d of %d (items %d-%d of %d) ", currentPage, totalPages, startItem, endItem, totalItems)
	pageStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Align(lipgloss.Right).
		Width(m.width - 2)

	// Combine header with page info on the right
	headerLine := lipgloss.JoinHorizontal(lipgloss.Top,
		header,
	)

	// List view - just render it normally since bubbles handles scrolling
	listView := m.list.View()

	// Page indicator line
	pageLine := pageStyle.Render(pageInfo)

	// Combine all elements and force exact height
	// bodyHeight = m.height - 1 (1 for footer)
	bodyHeight := m.height - 1
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	// Build content with explicit height constraint.
	// Layout (top to bottom): SearchRow (1) + ColumnHeader (1) + List + PageLine (1).
	// The search row is ALWAYS rendered above the column header (bt-fxbl) so
	// the header position is stable across all FilterStates: empty placeholder
	// when Unfiltered, live FilterInput when Filtering, applied pill when
	// FilterApplied. This fixed chrome height also keeps the click row math
	// (splitViewListChromeHeight) deterministic.
	searchRow := m.renderSearchRow(m.width - 2)
	parts := []string{searchRow, headerLine, listView, pageLine}
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Force exact height to prevent overflow
	return lipgloss.NewStyle().
		Width(m.width).
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Render(content)
}

func (m Model) renderSplitView() string {
	t := m.theme

	// m.list.Width() is the inner width (set in Update)
	listInnerWidth := m.list.Width()
	panelHeight := m.height - 2 // leave room for footer

	header := m.splitViewHeader()

	// Page info for list
	totalItems := len(m.list.Items())
	currentIdx := m.list.Index()
	listHeight := m.list.Height()
	if listHeight == 0 {
		listHeight = panelHeight - 3 // fallback
	}
	if listHeight < 1 {
		listHeight = 1
	}
	currentPage := (currentIdx / listHeight) + 1
	totalPages := (totalItems + listHeight - 1) / listHeight
	if totalPages < 1 {
		totalPages = 1
	}
	startItem := 0
	endItem := 0
	if totalItems > 0 {
		startItem = (currentPage-1)*listHeight + 1
		endItem = startItem + listHeight - 1
		if endItem > totalItems {
			endItem = totalItems
		}
	}

	pageInfo := fmt.Sprintf("Page %d/%d (%d-%d of %d) ", currentPage, totalPages, startItem, endItem, totalItems)
	pageStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Width(listInnerWidth).
		Align(lipgloss.Center)

	pageLine := pageStyle.Render(pageInfo)

	// Combine search row + column header + list + page indicator. The search
	// row (bt-fxbl) is always rendered above the column header so chrome
	// height is fixed across all FilterStates. This also keeps the
	// click-row math (splitViewListChromeHeight) deterministic.
	searchRow := m.renderSearchRow(listInnerWidth)
	splitParts := []string{searchRow, header, m.list.View(), pageLine}
	listContent := lipgloss.JoinVertical(lipgloss.Left, splitParts...)

	// Titled panel dimensions: outer width includes the 2 border chars
	listOuterWidth := listInnerWidth + 4 // content + padding + borders
	detailOuterWidth := m.viewport.Width() + 4

	// "Issues" rendered as the right-side label (bt-fxbl) — moves the title
	// from top-left to top-right so the panel chrome doesn't compete visually
	// with the column header right below it. PanelOpts.RightLabel + empty
	// Title achieves this without growing the PanelOpts API.
	listView := RenderTitledPanel(listContent, PanelOpts{
		RightLabel: "Issues",
		Width:      listOuterWidth,
		Height:     panelHeight,
		Focused:    m.focused == focusList,
	})

	detailView := RenderTitledPanel(m.viewport.View(), PanelOpts{
		Title:   "Details",
		Width:   detailOuterWidth,
		Height:  panelHeight,
		Focused: m.focused == focusDetail,
	})

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

func (m *Model) renderHelpOverlay() string {
	t := m.theme

	gapWidth := 3 // gap between panels in river layout

	// Tomorrow Night gradient for help overlay sections.
	// Maps to semantic theme tokens so YAML retones propagate (bt-pxbc).
	colors := []color.Color{
		ColorPrimary,   // Teal
		ColorInfo,      // Blue
		ColorSuccess,   // Green
		ColorWarning,   // Orange
		ColorTypeEpic,  // Purple
		ColorTypeTask,  // Yellow
	}

	// Helper to render a section panel (auto-sized to content).
	// Flipped layout: description on left, key right-aligned (bt-dx7k).
	renderPanel := func(title string, icon string, colorIdx int, shortcuts []struct{ key, desc string }) string {
		panelColor := colors[colorIdx%len(colors)]

		keyStyle := lipgloss.NewStyle().
			Foreground(panelColor).
			Bold(true)

		descStyle := lipgloss.NewStyle().
			Foreground(t.Base.GetForeground())

		// Find widest key and widest desc for alignment
		maxKeyWidth := 0
		maxDescWidth := 0
		for _, s := range shortcuts {
			if w := lipgloss.Width(s.key); w > maxKeyWidth {
				maxKeyWidth = w
			}
			if w := lipgloss.Width(s.desc); w > maxDescWidth {
				maxDescWidth = w
			}
		}

		// Inner content width: left pad + desc + gap + key + right pad
		innerWidth := 1 + maxDescWidth + 2 + maxKeyWidth + 1

		var lines []string
		for _, s := range shortcuts {
			desc := descStyle.Render(s.desc)
			key := keyStyle.Render(s.key)
			descPad := maxDescWidth - lipgloss.Width(s.desc)
			// Description left-aligned, key right-aligned
			line := " " + desc + strings.Repeat(" ", descPad+2) + key
			// Pad to full inner width for consistent panel sizing
			lineWidth := lipgloss.Width(line)
			if lineWidth < innerWidth {
				line += strings.Repeat(" ", innerWidth-lineWidth)
			}
			lines = append(lines, line)
		}

		// Panel width: inner content + border (2) + right pad (1)
		panelWidth := innerWidth + 3
		titleWidth := lipgloss.Width(icon+" "+title) + 6 // title + border decorations
		if titleWidth > panelWidth {
			panelWidth = titleWidth
		}

		content := lipgloss.JoinVertical(lipgloss.Left, lines...)
		return RenderTitledPanel(content, PanelOpts{
			Title:       icon + " " + title,
			Width:       panelWidth,
			CenterTitle: true,
			BorderColor: panelColor,
			TitleColor:  panelColor,
		})
	}

	// Define all sections
	navSection := []struct{ key, desc string }{
		{"j / ↓", "Move down"},
		{"k / ↑", "Move up"},
		{"G/end", "Go to last"},
		{"Ctrl+d", "Page down"},
		{"Ctrl+u", "Page up"},
		{"Tab", "Switch focus"},
		{"< / >", "Resize list pane"},
		{"Enter", "View details"},
		{"Esc", "Back / close"},
	}

	viewsSection := []struct{ key, desc string }{
		{"b", "Kanban board"},
		{"g", "Graph view"},
		{"i", "Insights"},
		{"h", "History view"},
		{"a", "Actionable"},
		{"f", "Flow matrix"},
		{"[", "Label dashboard"},
		{"]", "Attention view"},
	}

	globalSection := []struct{ key, desc string }{
		{"?", "This help"},
		{";", "Shortcuts bar"},
		{"!", "Alerts panel"},
		{"'", "Recipes"},
		{"w", "Project picker"},
		{"W", "Toggle project scope"},
		{"q", "Back / Quit"},
		{"Ctrl+c", "Force quit"},
	}

	filterSection := []struct{ key, desc string }{
		{"/", "Fuzzy search"},
		{"Ctrl+S", "Semantic search"},
		{"H", "Hybrid ranking"},
		{"Alt+H", "Hybrid preset"},
		{"o", "Open issues"},
		{"c", "Closed issues"},
		{"r", "Ready (unblocked)"},
		{"l", "Filter by label"},
		{"s", "Cycle sort"},
		{"S", "Cycle sort reverse"},
	}

	graphSection := []struct{ key, desc string }{
		{"hjkl", "Navigate nodes"},
		{"H/L", "Scroll left/right"},
		{"PgUp/Dn", "Scroll up/down"},
		{"Enter", "Jump to issue"},
	}

	insightsSection := []struct{ key, desc string }{
		{"h/l/Tab", "Switch panels"},
		{"j/k", "Navigate items"},
		{"e", "Explanations"},
		{"x", "Calc details"},
		{"m", "Toggle heatmap"},
		{"Enter", "Jump to issue"},
	}

	historySection := []struct{ key, desc string }{
		{"j/k", "Navigate beads"},
		{"J/K", "Navigate commits"},
		{"Tab", "Toggle focus"},
		{"y", "Copy SHA"},
		{"c", "Confidence filter"},
	}

	actionsSection := []struct{ key, desc string }{
		{"p", "Priority hints"},
		{"R", "Triage recipe"},
		{"Ctrl+R", "Force refresh"},
		{"F5", "Force refresh"},
		{"t", "Time-travel"},
		{"T", "Quick time-travel"},
		{"x", "Export markdown"},
		{"C", "Copy to clipboard"},
		{"O", "Open in editor"},
	}

	statusSection := []struct{ key, desc string }{
		{"◌ metrics", "Phase 2 metrics computing"},
		{"⚠ age", "Snapshot getting stale"},
		{"⚠ STALE", "Snapshot is stale"},
		{"✗ bg", "Background worker errors"},
		{"↻ recov", "Worker self-healed"},
		{"⚠ dead", "Worker unresponsive"},
		{"polling", "Live reload uses polling"},
	}

	// Build shortcut panels
	panels := []string{
		renderPanel("Navigation", "🧭", 0, navSection),
		renderPanel("Views", "👁", 1, viewsSection),
		renderPanel("Filters & Sort", "🔍", 3, filterSection),
		renderPanel("Global", "🌐", 2, globalSection),
		renderPanel("Graph View", "📊", 4, graphSection),
		renderPanel("Insights", "💡", 5, insightsSection),
		renderPanel("History", "📜", 0, historySection),
		renderPanel("Actions", "⚡", 1, actionsSection),
	}

	// River/masonry layout: greedily pack panels into rows (bt-dx7k)
	availableWidth := m.width - 4 // leave margin on sides
	gap := strings.Repeat(" ", gapWidth)

	var rows []string
	var currentRow []string
	currentRowWidth := 0

	for _, panel := range panels {
		panelWidth := lipgloss.Width(panel)
		needed := panelWidth
		if len(currentRow) > 0 {
			needed += gapWidth // account for gap before this panel
		}

		if currentRowWidth+needed > availableWidth && len(currentRow) > 0 {
			// Current row is full, flush it
			joined := currentRow[0]
			for _, p := range currentRow[1:] {
				joined = lipgloss.JoinHorizontal(lipgloss.Top, joined, gap, p)
			}
			rows = append(rows, joined)
			currentRow = []string{panel}
			currentRowWidth = panelWidth
		} else {
			currentRow = append(currentRow, panel)
			currentRowWidth += needed
		}
	}
	if len(currentRow) > 0 {
		joined := currentRow[0]
		for _, p := range currentRow[1:] {
			joined = lipgloss.JoinHorizontal(lipgloss.Top, joined, gap, p)
		}
		rows = append(rows, joined)
	}

	body := lipgloss.JoinVertical(lipgloss.Center, rows...)

	// Status indicators panel - append to river flow if room
	statusPanel := renderPanel("Status Indicators", "🩺", 2, statusSection)
	statusHeight := lipgloss.Height(statusPanel)
	bodyHeight := lipgloss.Height(body)
	// Show status panel if there's vertical room
	if bodyHeight+statusHeight+4 < m.height {
		body = lipgloss.JoinVertical(lipgloss.Center, body, statusPanel)
	}

	// Title and subtitle as plain centered text (no outer border)
	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	subtitleStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Italic(true)

	header := lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render("Keyboard Shortcuts"),
		subtitleStyle.Render("Space: Tutorial  |  ? or Esc to close"),
	)

	fullContent := lipgloss.JoinVertical(lipgloss.Center, header, "", body)

	return lipgloss.Place(
		m.width,
		m.height-1,
		lipgloss.Center,
		lipgloss.Center,
		fullContent,
	)
}

func (m Model) renderLabelHealthDetail(lh analysis.LabelHealth) string {
	t := m.theme
	innerWidth := m.width - 10
	if innerWidth < 20 {
		innerWidth = 20
	}

	// 1. Define styles first so closures can capture them
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2)

	labelStyle := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground())

	// 2. Define helper functions
	bar := func(score int) string {
		lvl := analysis.HealthLevelFromScore(score)
		fill := innerWidth * score / 100
		if fill < 0 {
			fill = 0
		}
		if fill > innerWidth {
			fill = innerWidth
		}
		filled := strings.Repeat("█", fill)
		blank := strings.Repeat("░", innerWidth-fill)
		style := t.Base
		switch lvl {
		case analysis.HealthLevelHealthy:
			style = style.Foreground(t.Open)
		case analysis.HealthLevelWarning:
			style = style.Foreground(t.Feature)
		default:
			style = style.Foreground(t.Blocked)
		}
		return style.Render(filled + blank)
	}

	flowList := func(title string, items []labelCount, arrow string) string {
		if len(items) == 0 {
			return ""
		}
		var b strings.Builder
		b.WriteString(labelStyle.Render(title))
		b.WriteString("\n")
		limit := len(items)
		if limit > 6 {
			limit = 6
		}
		for i := 0; i < limit; i++ {
			lc := items[i]
			line := fmt.Sprintf("  %s %-16s %3d", arrow, lc.Label, lc.Count)
			b.WriteString(valStyle.Render(line))
			b.WriteString("\n")
		}
		if len(items) > limit {
			b.WriteString(valStyle.Render(fmt.Sprintf("  … +%d more", len(items)-limit)))
			b.WriteString("\n")
		}
		return b.String()
	}

	// 3. Build content
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Foreground(t.Primary).Bold(true).MarginBottom(1).
		Render(fmt.Sprintf("Label Health: %s", lh.Label)))
	sb.WriteString("\n")

	sb.WriteString(labelStyle.Render("Overall: "))
	sb.WriteString(valStyle.Render(fmt.Sprintf("%d/100 (%s)", lh.Health, lh.HealthLevel)))
	sb.WriteString("\n")
	sb.WriteString(bar(lh.Health))
	sb.WriteString("\n\n")

	sb.WriteString(labelStyle.Render("Issues: "))
	sb.WriteString(valStyle.Render(fmt.Sprintf("%d total (%d open, %d blocked, %d closed)", lh.IssueCount, lh.OpenCount, lh.Blocked, lh.ClosedCount)))
	sb.WriteString("\n\n")

	sb.WriteString(labelStyle.Render("Velocity: "))
	sb.WriteString(valStyle.Render(fmt.Sprintf("%d/100 (7d=%d, 30d=%d, avg_close=%.1fd, trend=%s %.1f%%)", lh.Velocity.VelocityScore, lh.Velocity.ClosedLast7Days, lh.Velocity.ClosedLast30Days, lh.Velocity.AvgDaysToClose, lh.Velocity.TrendDirection, lh.Velocity.TrendPercent)))
	sb.WriteString("\n")
	sb.WriteString(bar(lh.Velocity.VelocityScore))
	sb.WriteString("\n\n")

	sb.WriteString(labelStyle.Render("Freshness: "))
	oldest := "n/a"
	if !lh.Freshness.OldestOpenIssue.IsZero() {
		oldest = lh.Freshness.OldestOpenIssue.Format("2006-01-02")
	}
	mostRecent := "n/a"
	if !lh.Freshness.MostRecentUpdate.IsZero() {
		mostRecent = lh.Freshness.MostRecentUpdate.Format("2006-01-02")
	}
	sb.WriteString(valStyle.Render(fmt.Sprintf("%d/100 (stale=%d, oldest_open=%s, most_recent=%s)", lh.Freshness.FreshnessScore, lh.Freshness.StaleCount, oldest, mostRecent)))
	sb.WriteString("\n")
	sb.WriteString(bar(lh.Freshness.FreshnessScore))
	sb.WriteString("\n\n")

	sb.WriteString(labelStyle.Render("Flow: "))
	sb.WriteString(valStyle.Render(fmt.Sprintf("%d/100 (in=%d from %v, out=%d to %v, external blocked=%d blocking=%d)", lh.Flow.FlowScore, lh.Flow.IncomingDeps, lh.Flow.IncomingLabels, lh.Flow.OutgoingDeps, lh.Flow.OutgoingLabels, lh.Flow.BlockedByExternal, lh.Flow.BlockingExternal)))
	sb.WriteString("\n")
	sb.WriteString(bar(lh.Flow.FlowScore))
	sb.WriteString("\n\n")

	// Cross-Label Flow Table (incoming/outgoing dependencies)
	if len(m.labelHealthDetailFlow.Incoming) > 0 || len(m.labelHealthDetailFlow.Outgoing) > 0 {
		sb.WriteString(labelStyle.Render("Cross-label deps:"))
		sb.WriteString("\n")

		if in := flowList("  Incoming", m.labelHealthDetailFlow.Incoming, "←"); in != "" {
			sb.WriteString(in)
			sb.WriteString("\n")
		}
		if out := flowList("  Outgoing", m.labelHealthDetailFlow.Outgoing, "→"); out != "" {
			sb.WriteString(out)
			sb.WriteString("\n")
		}
	}

	sb.WriteString(lipgloss.NewStyle().Foreground(t.Secondary).Italic(true).Render("Press Esc to close"))

	content := boxStyle.Render(sb.String())

	return lipgloss.Place(
		m.width,
		m.height-1,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

// renderLabelDrilldown shows a compact drilldown for the selected label
func (m Model) renderLabelDrilldown() string {
	t := m.theme

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2).
		Align(lipgloss.Left)

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(t.Base.GetForeground()).
		Bold(true)

	valStyle := lipgloss.NewStyle().
		Foreground(t.Base.GetForeground())

	// Locate cached health for this label (if available)
	var lh *analysis.LabelHealth
	for i := range m.labelHealthCache.Labels {
		if m.labelHealthCache.Labels[i].Label == m.labelDrilldownLabel {
			lh = &m.labelHealthCache.Labels[i]
			break
		}
	}

	issues := m.labelDrilldownIssues
	total := len(issues)
	open, blocked, inProgress, closed := 0, 0, 0, 0
	for _, is := range issues {
		if isClosedLikeStatus(is.Status) {
			closed++
			continue
		}
		switch is.Status {
		case model.StatusBlocked:
			blocked++
		case model.StatusInProgress:
			inProgress++
		default:
			open++
		}
	}

	// Top issues by PageRank (fallback to ID sort)
	type scored struct {
		issue model.Issue
		score float64
	}
	var scoredIssues []scored
	for _, is := range issues {
		scoredIssues = append(scoredIssues, scored{issue: is, score: m.data.analysis.GetPageRankScore(is.ID)})
	}
	sort.Slice(scoredIssues, func(i, j int) bool {
		if scoredIssues[i].score == scoredIssues[j].score {
			return scoredIssues[i].issue.ID < scoredIssues[j].issue.ID
		}
		return scoredIssues[i].score > scoredIssues[j].score
	})
	maxRows := m.height - 12
	if maxRows < 3 {
		maxRows = 3
	}
	if len(scoredIssues) > maxRows {
		scoredIssues = scoredIssues[:maxRows]
	}

	bar := func(score int) string {
		width := 20
		fill := int(float64(width) * float64(score) / 100.0)
		if fill < 0 {
			fill = 0
		}
		if fill > width {
			fill = width
		}
		filled := strings.Repeat("█", fill)
		blank := strings.Repeat("░", width-fill)
		style := t.Base
		if lh != nil {
			switch lh.HealthLevel {
			case analysis.HealthLevelHealthy:
				style = style.Foreground(t.Open)
			case analysis.HealthLevelWarning:
				style = style.Foreground(t.Feature)
			default:
				style = style.Foreground(t.Blocked)
			}
		}
		return style.Render(filled + blank)
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(fmt.Sprintf("Label Drilldown: %s", m.labelDrilldownLabel)))
	sb.WriteString("\n\n")

	if lh != nil {
		sb.WriteString(labelStyle.Render("Health: "))
		sb.WriteString(valStyle.Render(fmt.Sprintf("%d/100 (%s)", lh.Health, lh.HealthLevel)))
		sb.WriteString("\n")
		sb.WriteString(bar(lh.Health))
		sb.WriteString("\n\n")
	}

	sb.WriteString(labelStyle.Render("Issues: "))
	sb.WriteString(valStyle.Render(fmt.Sprintf("%d total (open %d, blocked %d, in-progress %d, closed %d)", total, open, blocked, inProgress, closed)))
	sb.WriteString("\n\n")

	if len(scoredIssues) > 0 {
		sb.WriteString(labelStyle.Render("Top issues by PageRank:"))
		sb.WriteString("\n")
		for _, si := range scoredIssues {
			line := fmt.Sprintf("  %s  %-10s  PR=%.3f  %s", getStatusIcon(si.issue.Status), si.issue.ID, si.score, si.issue.Title)
			sb.WriteString(valStyle.Render(line))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Cross-label flows summary
	flow := m.getCrossFlowsForLabel(m.labelDrilldownLabel)
	if len(flow.Incoming) > 0 || len(flow.Outgoing) > 0 {
		sb.WriteString(labelStyle.Render("Cross-label deps:"))
		sb.WriteString("\n")
		renderFlowList := func(title string, items []labelCount, arrow string) {
			if len(items) == 0 {
				return
			}
			sb.WriteString(valStyle.Render(title))
			sb.WriteString("\n")
			limit := len(items)
			if limit > 5 {
				limit = 5
			}
			for i := 0; i < limit; i++ {
				lc := items[i]
				line := fmt.Sprintf("  %s %-14s %3d", arrow, lc.Label, lc.Count)
				sb.WriteString(valStyle.Render(line))
				sb.WriteString("\n")
			}
			if len(items) > limit {
				sb.WriteString(valStyle.Render(fmt.Sprintf("  … +%d more", len(items)-limit)))
				sb.WriteString("\n")
			}
		}
		renderFlowList("  Incoming", flow.Incoming, "←")
		renderFlowList("  Outgoing", flow.Outgoing, "→")
		sb.WriteString("\n")
	}

	sb.WriteString(lipgloss.NewStyle().Foreground(t.Secondary).Italic(true).Render("Press Esc to close • g for graph analysis"))

	content := boxStyle.Render(sb.String())

	return lipgloss.Place(
		m.width,
		m.height-1,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

// renderLabelGraphAnalysis shows label-specific graph metrics (bv-109)
func (m Model) renderLabelGraphAnalysis() string {
	t := m.theme
	r := m.labelGraphAnalysisResult

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2).
		Align(lipgloss.Left)

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(t.Base.GetForeground()).
		Bold(true)

	valStyle := lipgloss.NewStyle().
		Foreground(t.Base.GetForeground())

	subtextStyle := lipgloss.NewStyle().
		Foreground(t.Subtext).
		Italic(true)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(fmt.Sprintf("Graph Analysis: %s", r.Label)))
	sb.WriteString("\n")
	sb.WriteString(subtextStyle.Render("PageRank & Critical Path computed on label subgraph"))
	sb.WriteString("\n\n")

	// Subgraph stats
	sb.WriteString(labelStyle.Render("Subgraph: "))
	sb.WriteString(valStyle.Render(fmt.Sprintf("%d issues (%d core, %d dependencies), %d edges",
		r.Subgraph.IssueCount, r.Subgraph.CoreCount,
		r.Subgraph.IssueCount-r.Subgraph.CoreCount, r.Subgraph.EdgeCount)))
	sb.WriteString("\n\n")

	// Critical Path section
	sb.WriteString(labelStyle.Render("🛤️  Critical Path"))
	if r.CriticalPath.HasCycle {
		sb.WriteString(valStyle.Render(" ⚠️  (cycle detected - path unreliable)"))
	}
	sb.WriteString("\n")
	if r.CriticalPath.PathLength == 0 {
		sb.WriteString(subtextStyle.Render("  No dependency chains found"))
	} else {
		sb.WriteString(valStyle.Render(fmt.Sprintf("  Length: %d issues (max height: %d)",
			r.CriticalPath.PathLength, r.CriticalPath.MaxHeight)))
		sb.WriteString("\n")

		// Show the path with titles
		maxRows := m.height - 20
		if maxRows < 3 {
			maxRows = 3
		}
		showCount := len(r.CriticalPath.Path)
		if showCount > maxRows {
			showCount = maxRows
		}

		for i := 0; i < showCount; i++ {
			issueID := r.CriticalPath.Path[i]
			title := r.CriticalPath.PathTitles[i]
			if title == "" {
				title = "(no title)"
			}
			arrow := "  →"
			if i == 0 {
				arrow = "  ●" // root
			}
			if i == len(r.CriticalPath.Path)-1 {
				arrow = "  ◆" // leaf
			}

			// Truncate title if needed
			maxTitleLen := m.width/2 - 20
			if maxTitleLen < 20 {
				maxTitleLen = 20
			}
			if len(title) > maxTitleLen {
				title = title[:maxTitleLen-1] + "…"
			}

			height := r.CriticalPath.AllHeights[issueID]
			line := fmt.Sprintf("%s %-12s [h=%d] %s", arrow, issueID, height, title)
			sb.WriteString(valStyle.Render(line))
			sb.WriteString("\n")
		}
		if len(r.CriticalPath.Path) > showCount {
			sb.WriteString(subtextStyle.Render(fmt.Sprintf("  … +%d more in path", len(r.CriticalPath.Path)-showCount)))
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")

	// PageRank section
	sb.WriteString(labelStyle.Render("📊 PageRank (Top Issues)"))
	sb.WriteString("\n")
	if len(r.PageRank.TopIssues) == 0 {
		sb.WriteString(subtextStyle.Render("  No issues to rank"))
	} else {
		maxPRRows := 8
		showPRCount := len(r.PageRank.TopIssues)
		if showPRCount > maxPRRows {
			showPRCount = maxPRRows
		}

		for i := 0; i < showPRCount; i++ {
			item := r.PageRank.TopIssues[i]
			title := ""
			statusIcon := "○"
			if iss, ok := r.Subgraph.IssueMap[item.ID]; ok {
				title = iss.Title
				statusIcon = getStatusIcon(iss.Status)
			}
			if title == "" {
				title = "(no title)"
			}

			// Truncate title if needed
			maxTitleLen := m.width/2 - 30
			if maxTitleLen < 15 {
				maxTitleLen = 15
			}
			if len(title) > maxTitleLen {
				title = title[:maxTitleLen-1] + "…"
			}

			normalized := r.PageRank.Normalized[item.ID]
			line := fmt.Sprintf("  %s %-12s PR=%.4f (%.0f%%) %s",
				statusIcon, item.ID, item.Score, normalized*100, title)
			sb.WriteString(valStyle.Render(line))
			sb.WriteString("\n")
		}
		if len(r.PageRank.TopIssues) > showPRCount {
			sb.WriteString(subtextStyle.Render(fmt.Sprintf("  … +%d more ranked", len(r.PageRank.TopIssues)-showPRCount)))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(t.Secondary).Italic(true).Render("Press Esc/q/g to close"))

	content := boxStyle.Render(sb.String())

	return lipgloss.Place(
		m.width,
		m.height-1,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

// renderTimeTravelPrompt renders the time-travel revision input overlay
func (m Model) renderTimeTravelPrompt() string {
	t := m.theme

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.Primary).
		Padding(1, 3).
		Align(lipgloss.Center)

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(t.Subtext).
		Italic(true)

	exampleStyle := lipgloss.NewStyle().
		Foreground(t.Secondary)

	keyStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	textStyle := lipgloss.NewStyle().
		Foreground(t.Base.GetForeground())

	// Build content
	content := titleStyle.Render("⏱️  Time-Travel Mode") + "\n\n" +
		subtitleStyle.Render("Compare current state with a historical revision") + "\n\n" +
		m.timeTravelInput.View() + "\n\n" +
		exampleStyle.Render("Examples: HEAD~5, main, v1.0.0, 2024-01-01, abc123") + "\n\n" +
		textStyle.Render("Press ") + keyStyle.Render("Enter") + textStyle.Render(" to compare, ") +
		keyStyle.Render("Esc") + textStyle.Render(" to cancel")

	box := boxStyle.Render(content)

	return lipgloss.Place(
		m.width,
		m.height-1,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)
}
