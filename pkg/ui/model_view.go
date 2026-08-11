package ui

import (
	"fmt"
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

// renderMemoriesLoadingScreen mirrors renderHistoryLoadingScreen for the
// Memories view's async dispatch (bt-2ea7t.4): the user pressed the Memories
// key, the view transitioned immediately, and discovery+aggregation is
// running off the event loop; this fills the screen until
// MemoriesLoadedMsg arrives.
func (m Model) renderMemoriesLoadingScreen() string {
	frame := workerSpinnerFrames[m.data.workerSpinnerIdx%len(workerSpinnerFrames)]

	spinnerStyle := lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
	titleStyle := lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	subStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	lines := []string{
		spinnerStyle.Render(frame),
		"",
		titleStyle.Render("Loading memories..."),
		"",
		subStyle.Render("Press u or Esc to cancel"),
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
	case ModalClaimConfirm:
		// Handled as overlay after background renders (below) — dimmed
		// backdrop like the other confirm modals (bt-oiaj.10).
	case ModalFieldSelect:
		// Handled as overlay after background renders (below) — dimmed
		// backdrop like the other modals (bt-oiaj.5).
	case ModalFieldPicker:
		// Handled as overlay after background renders (below)
	case ModalFieldInput:
		// Handled as overlay after background renders (below)
	case ModalLongformEdit:
		// Handled as overlay after background renders (below) — dimmed
		// backdrop like the other field-edit modals (bt-oiaj.6).
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
	case ModalSettings:
		// Handled as overlay after background renders (below). The dimmed
		// backdrop matters more here than elsewhere: cycling a palette
		// repaints the UI behind the screen, and that repaint IS the preview.
	case ModalRepoPicker:
		// Handled as overlay after background renders (below)
	case ModalLabelPicker:
		// Handled as overlay after background renders (below)
	case ModalHelp:
		// Handled as overlay after background renders (below) so the help card
		// floats over the dimmed current view like the other modals, instead of
		// replacing it with its own full-screen page (bt-dx7k.1 dogfood).
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
			case ViewMemories:
				if m.memoriesLoading {
					body = m.renderMemoriesLoadingScreen()
				} else {
					m.memories.SetSize(bodyW, m.height-1)
					body = m.memories.View()
				}
			default: // ViewList
				// An on-demand fullscreen pane (bt-530vn) takes priority over
				// the width-driven split/single-pane layout at any width;
				// exiting it (same key again, or Esc/q) falls back to
				// whichever of the branches below applied before toggling.
				switch m.fullscreen {
				case fullscreenIssues:
					body = m.renderFullscreenIssues()
				case fullscreenDetails:
					body = m.renderFullscreenDetails()
				default:
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
	}

	// Add shortcuts sidebar if enabled (bv-3qi5)
	if m.showShortcutsSidebar {
		// Feed the sidebar the active view's / modal's FullHelp() groups so it
		// consumes the same key.Map source as the L1 footer and ? overlay
		// (bt-ift6.10).
		m.shortcutsSidebar.SetBindings(m.sidebarHelpGroups())
		// Match the body height convention every view above sizes to
		// (m.height-1, reserving 1 row for the footer) rather than m.height-2.
		// The mismatch left the sidebar's panel box 1 row short of the body
		// it's joined against (lipgloss.JoinHorizontal(Top, ...) then pads the
		// shorter column with a blank row rather than growing the box), so the
		// sidebar's bottom border sat one row above the true bottom edge on
		// board/history/tree/flow-matrix (bt-xavk.2).
		m.shortcutsSidebar.SetSize(m.shortcutsSidebar.Width(), m.height-1)
		sidebar := m.shortcutsSidebar.View()
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, sidebar)
	}

	// Overlay modals that float on top of the background. All modal overlays
	// use OverlayCenterDimBackdrop so the surrounding cells visually recede,
	// matching the alerts/notifications pop-up aesthetic introduced by bt-v8he
	// and unified across all modals by bt-o1hs. The non-dim OverlayCenter is
	// reserved for non-modal overlays (debug, transient hints).
	if m.activeModal == ModalHelp {
		// The ? help card floats over the dimmed current view (bt-dx7k.1 dogfood),
		// matching the pop-up aesthetic of every other modal. renderHelpOverlay
		// returns the bare card (mini or full sheet); this centers + dims.
		body = OverlayCenterDimBackdrop(body, m.renderHelpOverlay(), m.width, m.height-1)
	}
	if m.activeModal == ModalRepoPicker {
		body = OverlayCenterDimBackdrop(body, m.repoPicker.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalLabelPicker {
		body = OverlayCenterDimBackdrop(body, m.labelPicker.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalRecipePicker {
		body = OverlayCenterDimBackdrop(body, m.recipePicker.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalSettings {
		body = OverlayCenterDimBackdrop(body, m.settingsModal.View(), m.width, m.height-1)
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
	if m.activeModal == ModalClaimConfirm {
		// Dim the background behind the write-confirm modal, matching the
		// other confirm modals (bt-oiaj.10).
		body = OverlayCenterDimBackdrop(body, m.renderClaimConfirm(), m.width, m.height-1)
	}
	if m.activeModal == ModalEpicCard {
		// Dim the overview/list behind the tier-2 epic focus card (bt-gfxhz.3).
		body = OverlayCenterDimBackdrop(body, m.renderEpicCard(), m.width, m.height-1)
	}
	if m.activeModal == ModalFieldSelect {
		// Dim the background behind the field-select hub (bt-oiaj.5).
		body = OverlayCenterDimBackdrop(body, m.fieldSelect.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalFieldPicker {
		body = OverlayCenterDimBackdrop(body, m.fieldPicker.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalFieldInput {
		body = OverlayCenterDimBackdrop(body, m.fieldInput.View(), m.width, m.height-1)
	}
	if m.activeModal == ModalLongformEdit {
		body = OverlayCenterDimBackdrop(body, m.longformEdit.View(), m.width, m.height-1)
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
	// MaxWidth enforces the horizontal half of the same guarantee. A body line
	// wider than the terminal — e.g. a panel that overshoots its width budget
	// by a cell (bt-2f9ff: the insights 2-column layout emits a 121-cell line at
	// width 120) — would otherwise WRAP under finalStyle.Width below, adding a
	// row that shoves the JoinVertical past MaxHeight(m.height) and clips the
	// footer off the bottom entirely. Truncating here keeps the body at exactly
	// bodyRows rows so the footer's last-row guarantee survives an over-wide
	// view. This is the never-wrap width guarantee the comment above names.
	body = lipgloss.NewStyle().Height(bodyRows).MaxHeight(bodyRows).MaxWidth(m.width).Render(body)

	// Floating notification bubble (bt-kuvzj): yazi-style overlay anchored to
	// the bottom-right of the body region, positioned independently of the
	// footer so it never competes with footer content for width — replaces
	// the statusline-embedded toast (see toast_bubble.go, panel.go's
	// OverlayBottomRight). Composited after the body's row/width clamp so its
	// dimensions match exactly what OverlayBottomRight expects.
	if bubble := m.renderToastBubble(m.width); bubble != "" {
		body = OverlayBottomRight(body, bubble, m.width, bodyRows, toastBubbleMarginRight, toastBubbleMarginBottom)
	}

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
//     (with our own prompt + cursor shown); count of running
//     matches on the right.
//   - FilterApplied: committed query + match count on the right (the original
//     "search pill" behavior, bt-031h).
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

	headerText := issueListColumnHeader(m.workspaceMode)
	if listInnerWidth > 0 && len(headerText) > listInnerWidth {
		headerText = headerText[:listInnerWidth]
	}
	return headerStyle.Render(headerText)
}

func issueListColumnHeader(workspaceMode bool) string {
	if workspaceMode {
		return "  REPO TYPE PRI STATUS      ID    TITLE"
	}
	return "  TYPE PRI STATUS      ID    TITLE"
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

	headerText := issueListColumnHeader(m.workspaceMode)
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

// issuesPaneBadge and detailsPaneBadge are the small btop-style superscript-
// digit badges rendered on each ViewList pane's border/title (bt-530vn),
// making the "2"/"3" fullscreen toggle (keys.ListNormalKeys.
// PaneFullscreenIssues/PaneFullscreenDetails) discoverable whether or not the
// pane is currently maximized. Plain text, not styled substrings: PanelOpts.
// Title/RightLabel width math (RenderTitledPanel) measures runes, and an
// embedded ANSI style would throw that off. The panel's existing
// focus-driven title/border coloring already applies uniformly, so the
// badge needs no color of its own (no hardcoded colors).
const (
	issuesPaneBadge  = "²Issues"
	detailsPaneBadge = "³Details"
)

// renderIssuesPanel renders the issues/list pane as a titled panel at the
// given outer width. Shared by the side-by-side split view (renderSplitView)
// and the on-demand issues-fullscreen view (renderFullscreenIssues,
// bt-530vn) so both stay pixel-identical in everything but width.
func (m Model) renderIssuesPanel(outerWidth, panelHeight int, focused bool) string {
	t := m.theme

	// m.list.Width() is the inner width (set by applyListDetailSizing).
	listInnerWidth := m.list.Width()
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

	// The badge is rendered as the right-side label (bt-fxbl precedent) —
	// moves the title from top-left to top-right so the panel chrome doesn't
	// compete visually with the column header right below it. PanelOpts.
	// RightLabel + empty Title achieves this without growing the PanelOpts API.
	return RenderTitledPanel(listContent, PanelOpts{
		RightLabel: issuesPaneBadge,
		Width:      outerWidth,
		Height:     panelHeight,
		Focused:    focused,
	})
}

// renderDetailsPanel renders the details pane as a titled panel at the given
// outer width. Shared by renderSplitView and renderFullscreenDetails
// (bt-530vn).
func (m Model) renderDetailsPanel(outerWidth, panelHeight int, focused bool) string {
	return RenderTitledPanel(m.viewport.View(), PanelOpts{
		Title:   detailsPaneBadge,
		Width:   outerWidth,
		Height:  panelHeight,
		Focused: focused,
	})
}

func (m Model) renderSplitView() string {
	// Reserve exactly 1 row for the footer, matching handleWindowSize's
	// bodyHeight (m.height-1) which sizes the list/viewport content. Using
	// m.height-2 here left the panel frame one row shorter than the content was
	// sized for, so finalStyle.Height(m.height) padded the deficit *below* the
	// footer — the bottom gap. Aligning to m.height-1 pins the footer flush to
	// the window's bottom edge.
	panelHeight := m.height - 1

	// Titled panel dimensions: outer width includes the 2 border chars
	listOuterWidth := m.list.Width() + 4 // content + padding + borders
	detailOuterWidth := m.viewport.Width() + 4

	listView := m.renderIssuesPanel(listOuterWidth, panelHeight, m.focused == focusList)
	detailView := m.renderDetailsPanel(detailOuterWidth, panelHeight, m.focused == focusDetail)

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

// renderFullscreenIssues renders the issues pane alone at full body width -
// the on-demand fullscreen toggle (bt-530vn), reachable at any terminal
// width via the "2" key (keys.ListNormalKeys.PaneFullscreenIssues). Distinct
// from the width-driven single-pane auto-collapse (bt-9a3wv): this is a
// deliberate override that composes with either isSplitView state.
func (m Model) renderFullscreenIssues() string {
	outerWidth := m.list.Width() + 4
	panelHeight := m.height - 1
	return m.renderIssuesPanel(outerWidth, panelHeight, true)
}

// renderFullscreenDetails mirrors renderFullscreenIssues for the details pane
// ("3" key, keys.ListNormalKeys.PaneFullscreenDetails).
func (m Model) renderFullscreenDetails() string {
	outerWidth := m.viewport.Width() + 4
	panelHeight := m.height - 1
	return m.renderDetailsPanel(outerWidth, panelHeight, true)
}

// helpRow is one display row in the ? overlay: key token on the left,
// description on the right. Sourced from the Global key.Map's FullHelp() via
// helpGlobalGroups so the overlay text cannot drift from the ; sidebar / footer
// (bt-ift6.11). The status legend supplies its own literal rows.
type helpRow struct{ left, right string }

// helpGroup is a titled section of helpRows rendered in the ? overlay.
type helpGroup struct {
	title string
	rows  []helpRow
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

// helpGlobalGroups projects m.keys.Global.FullHelp() into task-headed helpGroup
// slices for the single-box ? overlay renderer (bt-dx7k.1). Key on the left,
// desc on the right (yazi render_col orientation). FullHelp() returns 4 fixed
// groups: [0]Help&Chrome [1]Views [2]Workspace [3]Actions. Display order puts
// the most-used (view switching) first; the STATUS glyph legend is appended as
// a literal group (no key.Map source).
func (m Model) helpGlobalGroups() []helpGroup {
	g := m.keys.Global.FullHelp()

	// project converts a binding slice-of-slices into flat helpRows: enabled
	// bindings with non-empty help only. Key on left, desc on right.
	project := func(bindings [][]key.Binding) []helpRow {
		var rows []helpRow
		for _, group := range bindings {
			for _, b := range group {
				if !b.Enabled() {
					continue
				}
				h := b.Help()
				if h.Key == "" {
					continue
				}
				rows = append(rows, helpRow{left: h.Key, right: h.Desc})
			}
		}
		return rows
	}

	taskOrder := []struct {
		title    string
		bindings [][]key.Binding
	}{
		{"SWITCH VIEWS", [][]key.Binding{g[1]}},
		{"DO THINGS", [][]key.Binding{g[3]}},
		{"WORKSPACE", [][]key.Binding{g[2]}},
		{"CHROME", [][]key.Binding{g[0]}},
	}

	var groups []helpGroup
	for _, tg := range taskOrder {
		rows := project(tg.bindings)
		if len(rows) > 0 {
			groups = append(groups, helpGroup{title: tg.title, rows: rows})
		}
	}

	// STATUS legend: explains footer status glyphs; no key.Map source.
	// Orientation matches the keybind rows: glyph/phrase on left, meaning on right.
	groups = append(groups, helpGroup{
		title: "STATUS",
		rows: []helpRow{
			{left: "◌ metrics", right: "Phase 2 metrics computing"},
			{left: activeGlyphs.Warning + " age", right: "Snapshot getting stale"},
			{left: activeGlyphs.Warning + " STALE", right: "Snapshot is stale"},
			{left: activeGlyphs.Cross + " bg", right: "Background worker errors"},
			{left: "↻ recov", right: "Worker self-healed"},
			{left: activeGlyphs.Warning + " dead", right: "Worker unresponsive"},
			{left: "polling", right: "Live reload uses polling"},
		},
	})

	return groups
}

// renderHelpGroupColumn renders a stack of helpGroups as terminal lines for one
// column of the ? overlay (bt-dx7k.1). Models yazi's render_col: each group
// opens with an inline-divider header (─── TITLE ───) that fills colWidth visible
// cells, followed by right-justified key + muted desc rows. Groups are separated
// by a blank line. keyW is the right-justify width for the key token column.
func renderHelpGroupColumn(groups []helpGroup, colWidth, keyW int, t Theme) []string {
	headerStyle := lipgloss.NewStyle().Foreground(t.Secondary)
	keyStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground()).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(t.Muted)

	var lines []string
	for i, g := range groups {
		if i > 0 {
			lines = append(lines, "") // blank line between groups
		}

		// Inline divider: ─── TITLE ──────────────────
		label := " " + g.title + " "
		labelW := lipgloss.Width(label)
		lpad := 3
		rpad := colWidth - lpad - labelW
		if rpad < 0 {
			rpad = 0
		}
		div := strings.Repeat("─", lpad) + label + strings.Repeat("─", rpad)
		lines = append(lines, headerStyle.Render(div))

		// Rows: rjust(key, keyW) + "  " + desc (yazi row shape)
		for _, r := range g.rows {
			kw := lipgloss.Width(r.left)
			pad := keyW - kw
			if pad < 0 {
				pad = 0
			}
			row := keyStyle.Render(strings.Repeat(" ", pad)+r.left) +
				descStyle.Render("  "+r.right)
			lines = append(lines, row)
		}
	}
	return lines
}

// helpOverlayBodyLines distributes the global task groups across the responsive
// column grid and returns flat terminal lines (pre-scroll) for the single-box
// ? overlay (bt-dx7k.1). It picks the FEWEST columns whose rendered height fits
// the available body: fewer columns means a taller, wider layout that fills the
// vertical space we have and stops truncating long descriptions (bt-dx7k.1
// dogfood). helpOverlayColumns(width) is the width-imposed upper bound; if no
// column count fits, it returns the widest (shortest) layout so the tier
// selector in renderHelpOverlay falls back to the mini.
// The same flat-lines contract the scroll window consumes.
func (m Model) helpOverlayBodyLines() []string {
	groups := m.helpGlobalGroups()
	if len(groups) == 0 {
		return nil
	}

	maxCols := helpOverlayColumns(m.width)
	if maxCols > len(groups) {
		maxCols = len(groups)
	}
	// Floor at 2 columns when the width supports them: a single column reads fine
	// but leaves a wide (e.g. maximized) window looking sparse. 2 columns are
	// always shorter than 1, so they fit whenever 1 would -- this floor never
	// forces the mini, it only rebalances wide-and-tall windows (bt-dx7k.1 dogfood).
	minCols := 1
	if maxCols >= 2 {
		minCols = 2
	}
	avail := m.helpOverlayAvailBody()
	var lines []string
	for c := minCols; c <= maxCols; c++ {
		lines = m.helpOverlayBodyLinesForCols(groups, c)
		if len(lines) <= avail {
			return lines
		}
	}
	return lines
}

// helpOverlayBodyLinesForCols renders the global task groups into exactly n
// columns and returns the flat terminal lines (pre-scroll). Column widths are
// sized to fit within the terminal. helpOverlayBodyLines selects n by trying the
// fewest columns that fit the height (bt-dx7k.1).
func (m Model) helpOverlayBodyLinesForCols(groups []helpGroup, n int) []string {
	t := m.theme
	if n < 1 {
		n = 1
	}

	// Contiguous integer-partitioned chunks: i-th chunk = groups[i*total/n : (i+1)*total/n].
	total := len(groups)
	chunks := make([][]helpGroup, 0, n)
	for i := 0; i < n; i++ {
		start := i * total / n
		end := (i + 1) * total / n
		if end > start {
			chunks = append(chunks, groups[start:end])
		}
	}
	n = len(chunks) // actual non-empty column count

	// Global keyW: max key visible width across all groups (uniform across cols).
	keyW := 3
	for _, g := range groups {
		for _, r := range g.rows {
			if w := lipgloss.Width(r.left); w > keyW {
				keyW = w
			}
		}
	}

	// Natural per-chunk colWidth: max of content width and header minimum.
	naturalColWidths := make([]int, n)
	for i, chunk := range chunks {
		maxDescW := 0
		maxTitleW := 0
		for _, g := range chunk {
			if w := lipgloss.Width(g.title); w > maxTitleW {
				maxTitleW = w
			}
			for _, r := range g.rows {
				if w := lipgloss.Width(r.right); w > maxDescW {
					maxDescW = w
				}
			}
		}
		contentW := keyW + 2 + maxDescW
		headerMinW := 3 + 1 + maxTitleW + 1 + 1 // "─── TITLE ─" minimum
		if headerMinW > contentW {
			contentW = headerMinW
		}
		naturalColWidths[i] = contentW
	}

	// Fit columns to the available terminal inner width (terminal - 2 for box borders).
	// sepW is the visible width of each column separator " │ " (3 cells).
	const sepW = 3
	availInner := m.width - 2
	if availInner < 10 {
		availInner = 10
	}
	totalNatural := 0
	for _, w := range naturalColWidths {
		totalNatural += w
	}
	totalNatural += (n - 1) * sepW
	colWidths := naturalColWidths
	if totalNatural > availInner && n > 0 {
		// Distribute available space evenly, capped at each column's natural width.
		colContent := availInner - (n-1)*sepW
		if colContent < n {
			colContent = n
		}
		colWidths = make([]int, n)
		for i := range colWidths {
			cw := colContent / n
			if cw > naturalColWidths[i] {
				cw = naturalColWidths[i]
			}
			if cw < 1 {
				cw = 1
			}
			colWidths[i] = cw
		}
	}

	// Render each column at its computed colWidth.
	cols := make([][]string, n)
	for i, chunk := range chunks {
		cols[i] = renderHelpGroupColumn(chunk, colWidths[i], keyW, t)
	}

	// Measure each column's actual max visible width (post-ANSI rendering).
	colActualWidths := make([]int, n)
	for i, col := range cols {
		for _, line := range col {
			if w := lipgloss.Width(line); w > colActualWidths[i] {
				colActualWidths[i] = w
			}
		}
	}

	// Max column height.
	maxH := 0
	for _, col := range cols {
		if len(col) > maxH {
			maxH = len(col)
		}
	}

	// Styled column separator (dim, secondary color).
	sepStyle := lipgloss.NewStyle().Foreground(t.Secondary)
	sep := sepStyle.Render(" │ ")

	// Build body lines: pad each column's line to its actual width, join with sep.
	var bodyLines []string
	for row := 0; row < maxH; row++ {
		var parts []string
		for i, col := range cols {
			var line string
			if row < len(col) {
				line = col[row]
			}
			w := lipgloss.Width(line)
			if w < colActualWidths[i] {
				line += strings.Repeat(" ", colActualWidths[i]-w)
			}
			parts = append(parts, line)
		}
		if n > 1 {
			bodyLines = append(bodyLines, strings.Join(parts, sep))
		} else {
			bodyLines = append(bodyLines, parts[0])
		}
	}
	return bodyLines
}

// helpOverlayChrome is the fixed rows the ? overlay reserves outside the
// scrollable body: top border (1), interior footer line (1), bottom border (1).
const helpOverlayChrome = 3

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

// helpMiniRows returns the curated mini-card projection of GlobalKeys for the
// compact help tier (bt-dx7k.1 Task 2). Key and desc are sourced from each
// binding's Help() — no literal strings, so the mini cannot drift from the Map.
// Disabled bindings are skipped. Order is chosen so a 2-col layout reads
// board/graph, insights/search, labels/<projects>, help/quit.
//
// Task 3 will insert ProjectsOrWisps between LabelPicker and Help when the
// scope is multi-project, completing the labels/<projects> display row.
func (m Model) helpMiniRows() []helpRow {
	g := m.keys.Global
	var rows []helpRow
	add := func(b key.Binding) {
		if !b.Enabled() {
			return
		}
		h := b.Help()
		if h.Key == "" {
			return
		}
		rows = append(rows, helpRow{left: h.Key, right: h.Desc})
	}
	add(g.Board)
	add(g.Graph)
	add(g.Insights)
	add(g.SearchBounce)
	add(g.LabelPicker)
	if m.workspaceMode {
		add(g.ProjectsOrWisps)
	}
	add(g.Help)
	add(g.Back)
	return rows
}

// renderHelpMini renders the compact non-scrolling mini help card for short
// terminals (bt-dx7k.1 Task 2). Lays helpMiniRows into a fixed 2-column grid
// using the Task-1 key/desc styles (bold key, muted desc). A centered nudge
// line communicates the tier relationship; the footer close hint matches the
// full sheet. Wrapped in ONE RenderTitledPanel.
func (m Model) renderHelpMini() string {
	t := m.theme
	rows := m.helpMiniRows()
	if len(rows) == 0 {
		return ""
	}

	// Reuse Task-1 row styles: bold bright key, muted desc, dim secondary for chrome.
	keyStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground()).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(t.Muted)
	dimStyle := lipgloss.NewStyle().Foreground(t.Secondary)

	// keyW: max visible width of any key token (uniform right-justification, yazi rjust).
	keyW := 1
	for _, r := range rows {
		if w := lipgloss.Width(r.left); w > keyW {
			keyW = w
		}
	}

	// leftColW: content width of the left column, sized to the widest left-side desc
	// (even-indexed rows) so right-column cells always start at the same offset.
	leftDescW := 0
	for i, r := range rows {
		if i%2 == 0 {
			if w := lipgloss.Width(r.right); w > leftDescW {
				leftDescW = w
			}
		}
	}
	leftColW := keyW + 2 + leftDescW // key + "  " + desc

	const colGap = "  " // 2-space gap between columns

	// Build display rows: pair rows[2i] (left) with rows[2i+1] (right, optional).
	numDisplay := (len(rows) + 1) / 2
	var lines []string
	for i := 0; i < numDisplay; i++ {
		lr := rows[2*i]
		kpad := keyW - lipgloss.Width(lr.left)
		if kpad < 0 {
			kpad = 0
		}
		leftCell := keyStyle.Render(strings.Repeat(" ", kpad)+lr.left) +
			descStyle.Render("  "+lr.right)
		// Pad left cell to leftColW so the right column aligns vertically.
		if w := lipgloss.Width(leftCell); w < leftColW {
			leftCell += strings.Repeat(" ", leftColW-w)
		}

		if 2*i+1 < len(rows) {
			rr := rows[2*i+1]
			kpad2 := keyW - lipgloss.Width(rr.left)
			if kpad2 < 0 {
				kpad2 = 0
			}
			rightCell := keyStyle.Render(strings.Repeat(" ", kpad2)+rr.left) +
				descStyle.Render("  "+rr.right)
			lines = append(lines, leftCell+colGap+rightCell)
		} else {
			lines = append(lines, leftCell)
		}
	}

	// Nudge line: communicates the tier relationship (grow terminal → full sheet).
	nudge := dimStyle.Render("↓ expand   ·   ; per-view")

	// Footer: last interior line, centered. CAPITAL "Esc" matches the full sheet
	// footer and the existing TestHelpOverlayScroll smoke-string expectation.
	footer := dimStyle.Render("Esc ── q ── close")

	// Box inner width: driven by widest grid line, nudge, or footer.
	innerWidth := lipgloss.Width(nudge)
	for _, l := range lines {
		if w := lipgloss.Width(l); w > innerWidth {
			innerWidth = w
		}
	}
	if fw := lipgloss.Width(footer); fw > innerWidth {
		innerWidth = fw
	}
	if innerWidth < 10 {
		innerWidth = 10
	}
	if cap := m.width - 2; cap > 10 && innerWidth > cap {
		innerWidth = cap
	}

	// Assemble interior: grid lines + centered nudge + centered footer.
	allLines := make([]string, 0, len(lines)+2)
	allLines = append(allLines, lines...)
	allLines = append(allLines, centerLine(nudge, innerWidth))
	allLines = append(allLines, centerLine(footer, innerWidth))
	interior := strings.Join(allLines, "\n")

	boxed := RenderTitledPanel(interior, PanelOpts{
		Title:       "shortcuts",
		Width:       innerWidth + 2,
		CenterTitle: true,
		BorderColor: t.Secondary,
		TitleColor:  t.Secondary,
	})

	return boxed
}

// renderHelpOverlay renders the ? help card (mini or full sheet) as a single
// rounded box (bt-dx7k.1). The box interior holds the windowed body lines (from
// helpOverlayBodyLines, panned by helpScroll) plus a centered dim footer line as
// the last interior row. The outer border carries the "shortcuts" title. ONE
// RenderTitledPanel wraps the modal; View() composites the returned card centered
// over the dimmed background via OverlayCenterDimBackdrop (it does not Place here).
func (m *Model) renderHelpOverlay() string {
	t := m.theme
	bodyLines := m.helpOverlayBodyLines()

	// Mini tier: when the full body overflows the available height, render the
	// compact non-scrolling mini card instead. Mirrors yazi's (area.h-2) < POPUP_H
	// selector — size-driven, no new keypress or toggle.
	if len(bodyLines) > m.helpOverlayAvailBody() {
		return m.renderHelpMini()
	}

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
	var window []string
	if len(bodyLines) > 0 {
		window = bodyLines[scroll:end]
	}

	// Footer: cross-ref + close hint. Must contain ";" and "Esc"; must NOT
	// contain "shortcuts" (FOOTER WORDING CAVEAT, bt-dx7k.1).
	footerStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)
	footerText := "; per-view  -  Esc / q to close"
	if maxScroll > 0 {
		pct := scroll * 100 / maxScroll
		footerText = fmt.Sprintf("; per-view  -  j/k scroll %d%%  -  Esc / q close", pct)
	}
	footer := footerStyle.Render(footerText)

	// Box inner width: max visible width across windowed body lines and footer,
	// capped at m.width - 2 so the box never overflows the terminal.
	innerWidth := lipgloss.Width(footer)
	for _, line := range window {
		if w := lipgloss.Width(line); w > innerWidth {
			innerWidth = w
		}
	}
	if innerWidth < 10 {
		innerWidth = 10
	}
	maxInner := m.width - 2
	if maxInner < 10 {
		maxInner = 10
	}
	if innerWidth > maxInner {
		innerWidth = maxInner
	}
	boxWidth := innerWidth + 2 // +2 for left/right border characters

	// Interior: windowed body + a centered footer line, joined for RenderTitledPanel.
	// Centering matches the mini's footer/nudge so the close hint sits mid-box,
	// not flush-left (bt-dx7k.1 dogfood). Build a fresh slice so appending the
	// footer never aliases the bodyLines backing array.
	allLines := make([]string, 0, len(window)+1)
	allLines = append(allLines, window...)
	allLines = append(allLines, centerLine(footer, innerWidth))
	interior := strings.Join(allLines, "\n")

	boxed := RenderTitledPanel(interior, PanelOpts{
		Title:       "shortcuts",
		Width:       boxWidth,
		CenterTitle: true,
		BorderColor: t.Secondary,
		TitleColor:  t.Secondary,
	})

	// Return the bare card; View() composites it centered over the dimmed
	// background via OverlayCenterDimBackdrop so the help floats over the current
	// view like the other modals rather than replacing it (bt-dx7k.1 dogfood).
	return boxed
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
	sb.WriteString(labelStyle.Render("Critical Path"))
	if r.CriticalPath.HasCycle {
		sb.WriteString(valStyle.Render(" " + activeGlyphs.Warning + "  (cycle detected - path unreliable)"))
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
	sb.WriteString(labelStyle.Render(activeGlyphs.BarChart + " PageRank (Top Issues)"))
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
	// undercount titles like "Time-Travel Mode" by one cell vs. what the
	// terminal actually renders, so the top border ends up one cell wider
	// than the content rows. Other modals (Alerts!, Notifications, Select
	// Recipe) all use plain titles — this matches them.
	return RenderTitledPanel(strings.Join(contentLines, "\n"), PanelOpts{
		Title:   "Time-Travel Mode",
		Width:   promptWidth,
		Focused: true,
	})
}
