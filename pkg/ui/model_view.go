package ui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

// renderHistoryDoltOnly is the calm empty-state shown when enterHistoryView's
// cheap git-log gate detects that no .beads/*.jsonl file has ever been
// tracked in this repo's git history. bt's commit correlator (pkg/correlation)
// still uses the JSONL file as its witness; on Dolt-only repos that yields
// either an empty pane or a confusing red error. Phase 1 of bt-ydjw replaces
// that with this message; phase 2 wires the Dolt extractor (bt-08sh) so the
// pane actually renders events. Style mirrors renderHistoryLoadingScreen.
func (m Model) renderHistoryDoltOnly(width, height int) string {
	titleStyle := lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	hintStyle := lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)

	lines := []string{
		titleStyle.Render("No commit history yet"),
		"",
		bodyStyle.Render("This repo's beads live in Dolt, not in .beads/*.jsonl."),
		bodyStyle.Render("The history view's git-based correlator is being migrated"),
		bodyStyle.Render("to read events from Dolt directly (bt-08sh)."),
		"",
		hintStyle.Render("Press h or Esc to close"),
	}

	content := lipgloss.JoinVertical(lipgloss.Center, lines...)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// renderHistoryLoadingScreen mirrors renderLoadingScreen but for the History
// view's async dispatch path (bt-uizm). The user pressed `h`, the view
// transitioned immediately, and the report is being assembled off the event
// loop; this is what fills the screen until HistoryLoadedMsg arrives. Renders
// at full m.height-1 dims so any subsequent partial frame fully covers it -
// the same anti-leak constraint TestHistoryViewTransitionNoLeakage encodes.
func (m Model) renderHistoryLoadingScreen() string {
	frame := workerSpinnerFrames[m.data.workerSpinnerIdx%len(workerSpinnerFrames)]

	spinnerStyle := lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
	titleStyle := lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	subStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	lines := []string{
		spinnerStyle.Render(frame),
		"",
		titleStyle.Render("Loading history..."),
		"",
		subStyle.Render("Press h or Esc to cancel"),
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
		// Handled as overlay after background renders (below) so the
		// backdrop dims behind the panel like the other modals (bt-yly4).
	case ModalAgentPrompt:
		// Handled as overlay after background renders (below)
	case ModalCassSession:
		// Handled as overlay after background renders (below)
	case ModalUpdate:
		// Handled as overlay after background renders (below)
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
	case ModalEpicCard:
		// Handled as overlay after background renders (below) so the overview/
		// list dims behind the focus card (bt-gfxhz.3).
	case ModalTimeTravelInput:
		// Handled as overlay after background renders (below) — bt-rhfo /
		// bt-vklk Phase 1: dimmed backdrop, rounded border, title-in-border.
	case ModalBQLQuery:
		body = m.bqlQuery.View()
	case ModalRecipePicker:
		// Handled as overlay after background renders (below) — bt-vklk
		// Phase 1: dimmed backdrop, rounded border, title-in-border.
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
			// Body surfaces use bodyWidth so the shortcuts sidebar (when
			// visible) gets reserved space rather than overflowing (bt-lin9).
			bodyW := m.bodyWidth()
			// Route by ViewMode enum
			switch m.mode {
			case ViewInsights, ViewAttention:
				m.insightsPanel.SetSize(bodyW, m.height-1)
				body = m.insightsPanel.View()
			case ViewFlowMatrix:
				m.flowMatrix.SetSize(bodyW, m.height-1)
				body = m.flowMatrix.View()
			case ViewTree:
				m.tree.SetSize(bodyW, m.height-1)
				body = m.tree.View()
			case ViewGraph:
				body = m.graphView.View(bodyW, m.height-1)
			case ViewBoard:
				body = m.board.View(bodyW, m.height-1)
			case ViewActionable:
				m.actionableView.SetSize(bodyW, m.height-2)
				body = m.actionableView.Render()
			case ViewHistory:
				switch {
				case m.historyDoltOnly:
					body = m.renderHistoryDoltOnly(bodyW, m.height-1)
				case m.historyLoading:
					body = m.renderHistoryLoadingScreen()
				default:
					m.historyView.SetSize(bodyW, m.height-1)
					body = m.historyView.View()
				}
			case ViewEpics:
				body = m.epicsViewText
			case ViewLabelDashboard:
				m.labelDashboard.SetSize(bodyW, m.height-1)
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
		// Feed the sidebar the active view's / modal's FullHelp() groups so it
		// consumes the same key.Map source as the L1 footer and ? overlay
		// (bt-ift6.10).
		m.shortcutsSidebar.SetBindings(m.sidebarHelpGroups())
		m.shortcutsSidebar.SetSize(m.shortcutsSidebar.Width(), m.height-2)
		sidebar := m.shortcutsSidebar.View()
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, sidebar)
	}

	// Overlay modals that float on top of the background. All modal overlays
	// use OverlayCenterDimBackdrop so the surrounding cells visually recede,
	// matching the alerts/notifications pop-up aesthetic introduced by bt-v8he
	// and unified across all modals by bt-o1hs. The non-dim OverlayCenter is
	// reserved for non-modal overlays (debug, transient hints).
	if m.activeModal == ModalRepoPicker {
		body = OverlayCenterDimBackdrop(body, m.repoPicker.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalLabelPicker {
		body = OverlayCenterDimBackdrop(body, m.labelPicker.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalRecipePicker {
		body = OverlayCenterDimBackdrop(body, m.recipePicker.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalTimeTravelInput {
		body = OverlayCenterDimBackdrop(body, m.renderTimeTravelPrompt(), m.width, m.height-1)
	}
	if m.activeModal == ModalAgentPrompt {
		body = OverlayCenterDimBackdrop(body, m.agentPromptModal.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalCassSession {
		body = OverlayCenterDimBackdrop(body, m.cassModal.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalUpdate {
		body = OverlayCenterDimBackdrop(body, m.updateModal.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalAlerts {
		// The alerts/notifications modal renders at a content-comfortable
		// width and relies on the dimmed surrounding cells for occlusion —
		// preserves the pop-up aesthetic while still preventing detail-pane
		// bleed-through from competing with the modal for attention (bt-v8he).
		body = OverlayCenterDimBackdrop(body, m.renderAlertsPanel(), m.width, m.height-1)
	}
	if m.activeModal == ModalQuitConfirm {
		// Dim the background behind the destructive-confirm modal so it
		// reads as an overlay rather than a mode switch (bt-yly4).
		body = OverlayCenterDimBackdrop(body, m.renderQuitConfirm(), m.width, m.height-1)
	}
	if m.activeModal == ModalEpicCard {
		// Dim the overview/list behind the tier-2 epic focus card (bt-gfxhz.3).
		body = OverlayCenterDimBackdrop(body, m.renderEpicCard(), m.width, m.height-1)
	}

	footer := m.renderFooter()

	// Pin the footer to the final row at every height. The body region owns
	// exactly m.height-1 rows and the footer owns the last. Without this,
	// under-filling views (detail viewport, actionable plan) leave their footer
	// floating mid-screen with blank rows below it, and views that over-fill by
	// a row (graph / insights panels) push the footer past the bottom where the
	// MaxHeight below clips it away entirely. Clamping makes the footer's
	// bottom-row guarantee structural — the vertical analogue of the never-wrap
	// width guarantee (bt-yyked).
	bodyRows := m.height - 1
	if bodyRows < 1 {
		bodyRows = 1
	}
	body = lipgloss.NewStyle().Height(bodyRows).MaxHeight(bodyRows).Render(body)

	// Ensure the final output fits exactly in the terminal height
	// This prevents the header from being pushed off the top
	finalStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		MaxHeight(m.height)

	rendered := finalStyle.Render(lipgloss.JoinVertical(lipgloss.Left, body, footer))
	if m.showDebugDims {
		rendered = spliceDebugDims(rendered, m.width, m.height)
	}
	v := tea.NewView(rendered)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// spliceDebugDims overlays a high-contrast [WxH] chip in the top-right of
// the first rendered row. Used by the undocumented ctrl+p toggle for
// dimension-sensitive bug repros - lets the user screenshot with exact
// terminal cell counts baked in. ANSI-aware: slices the first line's
// rightmost chipWidth cells off and replaces them with the chip.
func spliceDebugDims(s string, width, height int) string {
	chipText := fmt.Sprintf(" %dx%d ", width, height)
	chip := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#ffff00")).
		Bold(true).
		Render(chipText)
	chipW := ansi.StringWidth(chip)
	if chipW >= width {
		return s
	}

	parts := strings.SplitN(s, "\n", 2)
	first := parts[0]
	rest := ""
	if len(parts) == 2 {
		rest = "\n" + parts[1]
	}
	left := ansi.Truncate(first, width-chipW, "")
	return left + chip + rest
}

// renderQuitConfirm returns the quit-confirm modal panel. The caller
// composites it via OverlayCenterDimBackdrop in View() so the backdrop
// dims uniformly with the other modals (bt-yly4).
func (m Model) renderQuitConfirm() string {
	t := m.theme

	textStyle := lipgloss.NewStyle().
		Foreground(t.Base.GetForeground())

	keyStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	// Two centered body lines: key hint + cancel hint.
	lineQuit := keyStyle.Render("esc") + textStyle.Render(" / ") + keyStyle.Render("y") + textStyle.Render(" to quit")
	lineCancel := textStyle.Render("press any other key to cancel")

	// Width sized to fit the longer body line plus breathing room. Title
	// "Quit?" is short enough that body width drives the panel width.
	innerW := lipgloss.Width(lineCancel)
	if w := lipgloss.Width(lineQuit); w > innerW {
		innerW = w
	}
	// Side padding inside the borders so text doesn't kiss the rules.
	const sidePad = 4
	panelWidth := innerW + 2 + sidePad // 2 for borders, sidePad for inner spacing

	pad := strings.Repeat(" ", sidePad/2)
	body := pad + centerLine(lineQuit, innerW) + pad + "\n" +
		pad + centerLine(lineCancel, innerW) + pad

	// Surround body with one blank line above and below for breathing.
	content := "\n" + body + "\n"

	return RenderTitledPanel(content, PanelOpts{
		Title:       "Quit?",
		Width:       panelWidth,
		CenterTitle: true,
		BorderColor: t.Blocked,
		TitleColor:  t.Blocked,
		Focused:     true,
	})
}

// centerLine pads s with spaces on both sides so its visible width
// equals width. If s is already at-or-over width, returns s unchanged.
func centerLine(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	gap := width - w
	left := gap / 2
	right := gap - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
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

	// bodyWidth reserves space for the shortcuts sidebar when visible (bt-lin9).
	bodyW := m.bodyWidth()

	// Render column header. Clip to width; lipgloss Style.Width sets background
	// fill but does NOT truncate, so at narrow widths the literal text would
	// wrap to a second row (bt-i138).
	headerWidth := bodyW - 2
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
		Width(bodyW - 2)

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
	searchRow := m.renderSearchRow(bodyW - 2)
	parts := []string{searchRow, headerLine, listView, pageLine}
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Force exact height to prevent overflow
	return lipgloss.NewStyle().
		Width(bodyW).
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Render(content)
}

func (m Model) renderSplitView() string {
	t := m.theme

	// m.list.Width() is the inner width (set in Update)
	listInnerWidth := m.list.Width()
	// Reserve exactly 1 row for the footer, matching handleWindowSize's
	// bodyHeight (m.height-1) which sizes the list/viewport content. Using
	// m.height-2 here left the panel frame one row shorter than the content was
	// sized for, so finalStyle.Height(m.height) padded the deficit *below* the
	// footer — the bottom gap. Aligning to m.height-1 pins the footer flush to
	// the window's bottom edge.
	panelHeight := m.height - 1

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

// helpOverlayColumns returns the task-panel column count for the ? overlay at the
// given width (bt-dx7k responsive levers): 4 wide, 2 medium, 1 narrow.
func helpOverlayColumns(width int) int {
	switch {
	case width >= 120:
		return 4
	case width >= 80:
		return 2
	default:
		return 1
	}
}

// helpOverlayPanels builds the global task-group panels (essentials-first) plus
// the status-glyph legend, in display order, for the ? overlay (bt-dx7k).
func (m Model) helpOverlayPanels() []string {
	t := m.theme

	// Tomorrow Night gradient for help overlay sections.
	// Maps to semantic theme tokens so YAML retones propagate (bt-pxbc).
	colors := []color.Color{
		ColorPrimary,  // Teal
		ColorInfo,     // Blue
		ColorSuccess,  // Green
		ColorWarning,  // Orange
		ColorTypeEpic, // Purple
		ColorTypeTask, // Yellow
	}

	// helpRow is one panel line: left text (desc/meaning) + right token
	// (key/glyph, right-aligned). Keybind panels source these from the active
	// key.Maps' FullHelp() so the overlay cannot drift from the ; sidebar /
	// footer (bt-ift6.11); the status legend supplies its own literal rows.
	type helpRow struct{ left, right string }

	// renderRowsPanel renders grouped rows as an auto-sized titled panel.
	// Flipped layout: left text on the left, right token right-aligned (bt-dx7k);
	// groups are separated by a blank line. Returns "" for an empty panel.
	renderRowsPanel := func(title, icon string, colorIdx int, groups [][]helpRow) string {
		panelColor := colors[colorIdx%len(colors)]

		keyStyle := lipgloss.NewStyle().
			Foreground(panelColor).
			Bold(true)

		descStyle := lipgloss.NewStyle().
			Foreground(t.Base.GetForeground())

		// Find widest left and right token across all rows for alignment.
		maxLeft := 0
		maxRight := 0
		for _, g := range groups {
			for _, r := range g {
				if w := lipgloss.Width(r.left); w > maxLeft {
					maxLeft = w
				}
				if w := lipgloss.Width(r.right); w > maxRight {
					maxRight = w
				}
			}
		}

		// Inner content width: left pad + left + gap + right + right pad
		innerWidth := 1 + maxLeft + 2 + maxRight + 1

		var lines []string
		firstGroup := true
		for _, g := range groups {
			if len(g) == 0 {
				continue
			}
			if !firstGroup {
				lines = append(lines, "") // blank line between groups
			}
			firstGroup = false
			for _, r := range g {
				left := descStyle.Render(r.left)
				right := keyStyle.Render(r.right)
				leftPad := maxLeft - lipgloss.Width(r.left)
				// Left text left-aligned, right token right-aligned.
				line := " " + left + strings.Repeat(" ", leftPad+2) + right
				// Pad to full inner width for consistent panel sizing.
				lineWidth := lipgloss.Width(line)
				if lineWidth < innerWidth {
					line += strings.Repeat(" ", innerWidth-lineWidth)
				}
				lines = append(lines, line)
			}
		}
		if len(lines) == 0 {
			return ""
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

	// bindingGroups converts a key.Map's FullHelp() into helpRow groups: enabled
	// bindings with non-empty help, desc on the left and key on the right. The
	// single binding source means the ? overlay text always matches dispatch.
	bindingGroups := func(groups [][]key.Binding) [][]helpRow {
		out := make([][]helpRow, 0, len(groups))
		for _, g := range groups {
			rows := make([]helpRow, 0, len(g))
			for _, b := range g {
				if !b.Enabled() {
					continue
				}
				h := b.Help()
				if h.Key == "" {
					continue
				}
				rows = append(rows, helpRow{left: h.Desc, right: h.Key})
			}
			if len(rows) > 0 {
				out = append(out, rows)
			}
		}
		return out
	}

	// Global map -> essentials-first, task-headed panels (bt-dx7k). FullHelp()
	// returns 4 fixed groups: [0]Help&Chrome [1]Views [2]Workspace [3]Actions.
	// Display order puts the most-used (view switching) first so the top of the
	// overlay is useful before any scroll. Headers label the existing groups;
	// the grouping is not restructured. Header strings are a dogfood tuning point.
	g := m.keys.Global.FullHelp()
	taskOrder := []struct {
		title    string
		colorIdx int
		bindings [][]key.Binding
	}{
		{"SWITCH VIEWS", 0, [][]key.Binding{g[1]}},
		{"DO THINGS", 1, [][]key.Binding{g[3]}},
		{"WORKSPACE", 2, [][]key.Binding{g[2]}},
		{"CHROME", 3, [][]key.Binding{g[0]}},
	}
	var panels []string
	for _, tg := range taskOrder {
		if p := renderRowsPanel(tg.title, "", tg.colorIdx, bindingGroups(tg.bindings)); p != "" {
			panels = append(panels, p)
		}
	}

	// Status-glyph legend: the one panel with no key.Map source (it explains the
	// footer status indicators, not key bindings), so it keeps literal rows.
	statusLegend := [][]helpRow{{
		{left: "Phase 2 metrics computing", right: "◌ metrics"},
		{left: "Snapshot getting stale", right: "⚠ age"},
		{left: "Snapshot is stale", right: "⚠ STALE"},
		{left: "Background worker errors", right: "✗ bg"},
		{left: "Worker self-healed", right: "↻ recov"},
		{left: "Worker unresponsive", right: "⚠ dead"},
		{left: "Live reload uses polling", right: "polling"},
	}}
	if p := renderRowsPanel("Status Indicators", "🩺", 2, statusLegend); p != "" {
		panels = append(panels, p)
	}

	return panels
}

// helpOverlayBodyLines lays the task panels into the responsive grid and returns
// the flat terminal lines (pre-scroll), so the caller can window them (bt-dx7k).
func (m Model) helpOverlayBodyLines() []string {
	panels := m.helpOverlayPanels()
	if len(panels) == 0 {
		return nil
	}
	cols := helpOverlayColumns(m.width)
	gap := strings.Repeat(" ", 3)
	var rows []string
	for i := 0; i < len(panels); i += cols {
		end := i + cols
		if end > len(panels) {
			end = len(panels)
		}
		row := panels[i]
		for _, p := range panels[i+1 : end] {
			row = lipgloss.JoinHorizontal(lipgloss.Top, row, gap, p)
		}
		rows = append(rows, row)
	}
	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return strings.Split(body, "\n")
}

// helpOverlayChrome is the fixed rows the ? overlay reserves outside the
// scrollable body: title + subtitle (2), a blank spacer (1), the footer (1).
const helpOverlayChrome = 4

// helpOverlayAvailBody returns the scrollable body height for the ? overlay.
func (m Model) helpOverlayAvailBody() int {
	avail := m.height - 1 - helpOverlayChrome
	if avail < 1 {
		avail = 1
	}
	return avail
}

// helpScrollMax is the maximum helpScroll offset for the current dimensions.
func (m Model) helpScrollMax() int {
	max := len(m.helpOverlayBodyLines()) - m.helpOverlayAvailBody()
	if max < 0 {
		max = 0
	}
	return max
}

func (m *Model) renderHelpOverlay() string {
	t := m.theme
	bodyLines := m.helpOverlayBodyLines()

	avail := m.helpOverlayAvailBody()
	maxScroll := len(bodyLines) - avail
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := m.helpScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	end := scroll + avail
	if end > len(bodyLines) {
		end = len(bodyLines)
	}
	window := bodyLines
	if len(bodyLines) > 0 {
		window = bodyLines[scroll:end]
	}
	body := strings.Join(window, "\n")

	titleStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	subtitleStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)
	header := lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render("Global Shortcuts"),
		subtitleStyle.Render("; for this screen  -  ? or Esc to close"),
	)

	// Footer scroll indicator when content overflows (Task 4 enriches the footer).
	footer := ""
	if maxScroll > 0 {
		pct := scroll * 100 / maxScroll
		footer = subtitleStyle.Render(fmt.Sprintf("j/k scroll  %d%%", pct))
	}

	fullContent := lipgloss.JoinVertical(lipgloss.Center, header, "", body, footer)
	// Top-align vertically: centering clipped oversized content (the bt-dx7k bug).
	return lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Top, fullContent)
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

// renderTimeTravelPrompt renders the time-travel revision input panel.
// Composited into the view via OverlayCenterDimBackdrop (bt-rhfo / bt-vklk
// Phase 1) so callers do not center it again. Returns the rendered titled
// panel; backdrop dimming and centering are the compositor's job.
//
// Padding is applied via manual leading-space prefixes (matching the alerts
// pattern). Wrapping the block in lipgloss.NewStyle().Padding(...).Render(...)
// did not consistently pad multi-line styled content to a uniform width when
// each line had its own SGR scope, producing rows that ended early and left
// the right border misaligned (bt-rhfo dogfood).
func (m Model) renderTimeTravelPrompt() string {
	t := m.theme

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

	const promptWidth = 64
	const leadPad = "   " // 3 spaces of left padding (matches old Padding(1,3))

	contentLines := []string{
		"", // top breathing room
		leadPad + subtitleStyle.Render("Compare current state with a historical revision"),
		"",
		leadPad + m.timeTravelInput.View(),
		"",
		leadPad + exampleStyle.Render("Examples: HEAD~5, main, v1.0.0, 2024-01-01, abc123"),
		"",
		leadPad + textStyle.Render("Press ") + keyStyle.Render("Enter") +
			textStyle.Render(" to compare, ") + keyStyle.Render("Esc") +
			textStyle.Render(" to cancel"),
		"", // bottom breathing room
	}

	// Plain-text title (no emoji): VS16 emoji presentation makes runewidth
	// undercount titles like "⏱️  Time-Travel Mode" by one cell vs. what the
	// terminal actually renders, so the top border ends up one cell wider
	// than the content rows. Other modals (Alerts!, Notifications, Select
	// Recipe) all use plain titles — this matches them.
	return RenderTitledPanel(strings.Join(contentLines, "\n"), PanelOpts{
		Title:   "Time-Travel Mode",
		Width:   promptWidth,
		Focused: true,
	})
}
