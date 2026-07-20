// memories.go implements the Memories master/detail view (bt-2ea7t.4,
// implements bt-wxrr5): a read-only surface over bd's `bd remember` memories,
// aggregated across every reachable bd-managed source (design spec
// docs/design/2026-07-15-cross-project-read-layer-and-memories.md S4.2).
//
// Memories carry no status/priority/deps/graph (design spec S4.2 "Not
// beads"), so this rides its own model rather than the issue-list machinery.
// Bodies are 162-514-char single-paragraph prose with no metadata column
// (spec S4.2), which is why the shape is master/detail (key list left,
// full-body reading pane right) rather than a flat one-line list.
package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/seanmartinsmith/beadstui/internal/source"
)

// memoriesRowKind distinguishes a group-header row from a selectable memory
// row in the master pane's flattened display list.
type memoriesRowKind int

const (
	memoriesRowGroup memoriesRowKind = iota
	memoriesRowMemory
)

// memoriesRow is one row in the master pane's flattened, rendered list:
// either a group header (origin display name) or a selectable memory.
type memoriesRow struct {
	kind   memoriesRowKind
	header string
	memory source.Memory
}

// MemoriesModel renders the memories master/detail view: a left pane
// listing memory keys grouped by origin project, and a right reading pane
// with the selected memory's full body.
type MemoriesModel struct {
	aggregate source.MemoriesAggregate

	// rows is the flattened, already-filtered-and-grouped display list:
	// group headers interleaved with the memory rows they contain. Rebuilt
	// by rebuildRows whenever the aggregate, search query, or origin filter
	// changes.
	rows   []memoriesRow
	cursor int // index into rows; always rests on a memoriesRowMemory row, or -1 when rows is empty

	scrollOffset int // first visible index into rows (master pane)

	detail        viewport.Model
	detailFocused bool // single-pane: which pane is shown; split-pane: which pane j/k drives + border focus

	searchInput  textinput.Model
	searchActive bool

	// originFilter narrows displayed memories to these Origin.Scope values.
	// nil/empty means "show all origins" (bt-2ea7t.4 ships search-only
	// narrowing; the repo_picker-based origin filter described in spec S6 is
	// tracked as a follow-up - see bt-2ea7t.4 close notes).
	originFilter map[string]bool

	width, height int
	theme         Theme
}

// NewMemoriesModel constructs an empty Memories view. Call SetAggregate once
// the async load (LoadMemoriesCmd) completes.
func NewMemoriesModel(theme Theme) MemoriesModel {
	ti := textinput.New()
	ti.Placeholder = "Search keys and bodies..."
	ti.CharLimit = 200
	ti.SetWidth(40)

	dvp := viewport.New(viewport.WithWidth(40), viewport.WithHeight(10))

	return MemoriesModel{
		searchInput: ti,
		detail:      dvp,
		theme:       theme,
		cursor:      -1,
	}
}

// SetSize updates the view's outer dimensions (outer box, footer row
// already excluded by the caller - mirrors every other full-screen view).
func (m *MemoriesModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetAggregate installs a freshly loaded aggregate and rebuilds the display
// rows (grouping plus any active search/origin filter).
func (m *MemoriesModel) SetAggregate(agg source.MemoriesAggregate) {
	m.aggregate = agg
	m.rebuildRows()
}

// hasAnyMemories reports whether ANY source contributed a memory, ignoring
// the active search/origin filter. This is the "genuinely empty" signal
// (design spec S8: "the view shows an empty state only when ALL sources are
// empty") as distinct from "filtered to zero", which stays in-chrome with a
// "No matches" master pane (mirrors HistoryModel's hasAnyHistoryData split).
func (m MemoriesModel) hasAnyMemories() bool {
	return len(m.aggregate.Memories) > 0
}

// isSplitWidth reports whether the view has room for a real master/detail
// split, mirroring the outer Model's SplitViewThreshold convention
// (bt-9a3wv): width > 100 splits, at-or-below collapses to a single pane so
// the view stays usable on the user's scrunched 14-30 row terminals.
func (m MemoriesModel) isSplitWidth() bool {
	return m.width > SplitViewThreshold
}

// --- filtering / grouping ---

func memoryMatchesQuery(mem source.Memory, query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(mem.Key), q) || strings.Contains(strings.ToLower(mem.Body), q)
}

// rebuildRows recomputes m.rows from m.aggregate.Memories, applying the
// active search query and origin filter, grouped by Origin.Label() (sorted)
// then Key (sorted) within each group. The previously-selected memory (by
// key+scope) is preserved if it survives the rebuild; otherwise the cursor
// lands on the first memory row.
func (m *MemoriesModel) rebuildRows() {
	prevKey, prevScope := "", ""
	if mem, ok := m.selectedMemory(); ok {
		prevKey, prevScope = mem.Key, mem.Origin.Scope
	}

	query := strings.TrimSpace(m.searchInput.Value())

	groups := map[string][]source.Memory{}
	var groupOrder []string
	for _, mem := range m.aggregate.Memories {
		if len(m.originFilter) > 0 && !m.originFilter[mem.Origin.Scope] {
			continue
		}
		if !memoryMatchesQuery(mem, query) {
			continue
		}
		label := mem.Origin.Label()
		if _, ok := groups[label]; !ok {
			groupOrder = append(groupOrder, label)
		}
		groups[label] = append(groups[label], mem)
	}
	sort.Strings(groupOrder)

	var rows []memoriesRow
	for _, label := range groupOrder {
		mems := groups[label]
		sort.Slice(mems, func(i, j int) bool { return mems[i].Key < mems[j].Key })
		rows = append(rows, memoriesRow{kind: memoriesRowGroup, header: fmt.Sprintf("%s (%d)", label, len(mems))})
		for _, mem := range mems {
			rows = append(rows, memoriesRow{kind: memoriesRowMemory, memory: mem})
		}
	}
	m.rows = rows

	m.cursor = -1
	for i, r := range rows {
		if r.kind != memoriesRowMemory {
			continue
		}
		if m.cursor < 0 {
			m.cursor = i // first memory row as fallback
		}
		if prevKey != "" && r.memory.Key == prevKey && r.memory.Origin.Scope == prevScope {
			m.cursor = i
			break
		}
	}
	m.scrollOffset = 0
	m.detail.GotoTop()
}

// selectedMemory returns the memory under the cursor, if any.
func (m MemoriesModel) selectedMemory() (source.Memory, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) || m.rows[m.cursor].kind != memoriesRowMemory {
		return source.Memory{}, false
	}
	return m.rows[m.cursor].memory, true
}

// OriginScopes returns the sorted, deduplicated set of Origin.Scope values
// across every loaded memory - the candidate set for origin filtering.
func (m MemoriesModel) OriginScopes() []string {
	seen := map[string]bool{}
	var scopes []string
	for _, mem := range m.aggregate.Memories {
		if !seen[mem.Origin.Scope] {
			seen[mem.Origin.Scope] = true
			scopes = append(scopes, mem.Origin.Scope)
		}
	}
	sort.Strings(scopes)
	return scopes
}

// --- navigation ---

func (m *MemoriesModel) MoveDown() {
	for i := m.cursor + 1; i < len(m.rows); i++ {
		if m.rows[i].kind == memoriesRowMemory {
			m.cursor = i
			m.ensureVisible()
			m.detail.GotoTop()
			return
		}
	}
}

func (m *MemoriesModel) MoveUp() {
	for i := m.cursor - 1; i >= 0; i-- {
		if m.rows[i].kind == memoriesRowMemory {
			m.cursor = i
			m.ensureVisible()
			m.detail.GotoTop()
			return
		}
	}
}

func (m *MemoriesModel) ToggleDetailFocus() {
	m.detailFocused = !m.detailFocused
}

// FocusDetail switches the single-pane view to the detail pane (Enter on a
// selected memory) and is a no-op with nothing selected.
func (m *MemoriesModel) FocusDetail() {
	if _, ok := m.selectedMemory(); ok {
		m.detailFocused = true
	}
}

func (m *MemoriesModel) ScrollDetailUp()   { m.detail.ScrollUp(1) }
func (m *MemoriesModel) ScrollDetailDown() { m.detail.ScrollDown(1) }

func (m *MemoriesModel) ensureVisible() {
	budget := m.masterListBudget()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+budget {
		m.scrollOffset = m.cursor - budget + 1
	}
}

// --- search ---

func (m *MemoriesModel) StartSearch() {
	m.searchActive = true
	m.searchInput.Focus()
}

func (m *MemoriesModel) CancelSearch() {
	m.searchActive = false
	m.searchInput.Blur()
	m.searchInput.SetValue("")
	m.rebuildRows()
}

// ConfirmSearch blurs the search input but keeps the query/filter applied.
func (m *MemoriesModel) ConfirmSearch() {
	m.searchActive = false
	m.searchInput.Blur()
}

func (m MemoriesModel) IsSearchActive() bool { return m.searchActive }

// UpdateSearchInput forwards msg to the search textinput and re-filters when
// the query text changes.
func (m *MemoriesModel) UpdateSearchInput(msg tea.Msg) {
	old := m.searchInput.Value()
	m.searchInput, _ = m.searchInput.Update(msg)
	if m.searchInput.Value() != old {
		m.rebuildRows()
	}
}

// --- layout helpers ---

// pluralSuffix returns "" for n==1, "s" otherwise.
func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// renderNotes renders the gc-hidden note (design spec S4.3/S8) and an
// unavailable-sources note above the master/detail panels. Returns "" when
// neither applies.
func (m MemoriesModel) renderNotes() string {
	var lines []string
	if n := len(m.aggregate.Excluded); n > 0 {
		noun := "source"
		if n != 1 {
			noun = "sources"
		}
		style := m.theme.Base.Foreground(m.theme.Muted).Italic(true)
		lines = append(lines, style.Render(fmt.Sprintf("%d Gas City %s hidden (own lens, coming later)", n, noun)))
	}
	if n := len(m.aggregate.Unavailable); n > 0 {
		style := m.theme.Base.Foreground(m.theme.Warning)
		shown := n
		if shown > 3 {
			shown = 3
		}
		names := make([]string, 0, shown)
		for i := 0; i < shown; i++ {
			u := m.aggregate.Unavailable[i]
			names = append(names, u.Origin.Label())
		}
		suffix := ""
		if n > shown {
			suffix = fmt.Sprintf(" +%d more", n-shown)
		}
		lines = append(lines, style.Render(fmt.Sprintf("%d source%s unavailable: %s%s", n, pluralSuffix(n), strings.Join(names, ", "), suffix)))
	}
	if len(lines) == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m MemoriesModel) noteHeight() int {
	if n := m.renderNotes(); n != "" {
		return lipgloss.Height(n)
	}
	return 0
}

// panelHeight is the outer box height (borders included) available to the
// master/detail panels once the notes strip is reserved.
func (m MemoriesModel) panelHeight() int {
	h := m.height - m.noteHeight()
	if h < 3 {
		h = 3
	}
	return h
}

func (m MemoriesModel) masterContentHeight() int {
	h := m.panelHeight() - 2 // borders
	if h < 1 {
		h = 1
	}
	return h
}

// masterListBudget is the row budget for the scrollable rows list within the
// master pane, after reserving a line for the search input when active.
func (m MemoriesModel) masterListBudget() int {
	h := m.masterContentHeight()
	if m.searchActive {
		h--
	}
	if h < 1 {
		h = 1
	}
	return h
}

func (m MemoriesModel) memoryRowCount() int {
	n := 0
	for _, r := range m.rows {
		if r.kind == memoriesRowMemory {
			n++
		}
	}
	return n
}

// VisibleProjectCount returns the number of distinct Origin.Scope values
// among the memories currently visible in m.rows (post search-filter) — the
// lens-scoped project count feeding the footer's memories CenterOverride
// (bt-p8y2f: "N memories · M projects"). Mirrors memoryRowCount's rows-based
// scoping rather than OriginScopes' unfiltered aggregate.Memories scoping.
func (m MemoriesModel) VisibleProjectCount() int {
	seen := map[string]bool{}
	for _, r := range m.rows {
		if r.kind == memoriesRowMemory {
			seen[r.memory.Origin.Scope] = true
		}
	}
	return len(seen)
}

// --- rendering ---

// renderEmptyState is the full-screen empty state shown only when every
// attempted source had zero memories (design spec S8).
func (m MemoriesModel) renderEmptyState() string {
	titleStyle := m.theme.Base.Bold(true)
	bodyStyle := m.theme.Base.Foreground(m.theme.Muted)

	lines := []string{
		titleStyle.Render("No memories found"),
		"",
		bodyStyle.Render("No bd source in scope has any `bd remember` entries yet."),
	}
	if n := len(m.aggregate.Excluded); n > 0 {
		noun := "source"
		if n != 1 {
			noun = "sources"
		}
		lines = append(lines, "", bodyStyle.Italic(true).Render(fmt.Sprintf("%d Gas City %s hidden (own lens, coming later)", n, noun)))
	}
	if n := len(m.aggregate.Unavailable); n > 0 {
		lines = append(lines, "", m.theme.Base.Foreground(m.theme.Warning).Render(fmt.Sprintf("%d source%s unavailable", n, pluralSuffix(n))))
	}

	content := lipgloss.JoinVertical(lipgloss.Center, lines...)
	w, h := m.width, m.height
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}

// renderMasterLines renders the master pane's content (search row when
// active, then the visible window of grouped rows).
func (m MemoriesModel) renderMasterLines() string {
	var lines []string
	if m.searchActive {
		inputStyle := m.theme.Base.Foreground(m.theme.Primary)
		lines = append(lines, inputStyle.Render("Search: ")+m.searchInput.View())
	}

	if len(m.rows) == 0 {
		lines = append(lines, m.theme.Base.Foreground(m.theme.Muted).Italic(true).Render("No matches"))
		return strings.Join(lines, "\n")
	}

	budget := m.masterListBudget()
	start := m.scrollOffset
	if start > len(m.rows) {
		start = len(m.rows)
	}
	end := start + budget
	if end > len(m.rows) {
		end = len(m.rows)
	}

	for i := start; i < end; i++ {
		row := m.rows[i]
		switch row.kind {
		case memoriesRowGroup:
			lines = append(lines, m.theme.Header.Render(row.header))
		default:
			text := "  " + row.memory.Key
			if i == m.cursor {
				lines = append(lines, m.theme.Selected.Render(text))
			} else {
				lines = append(lines, m.theme.Base.Render(text))
			}
		}
	}
	return strings.Join(lines, "\n")
}

// detailTitleText is the right panel's title-in-border text.
func (m MemoriesModel) detailTitleText() string {
	mem, ok := m.selectedMemory()
	if !ok {
		return "Detail"
	}
	return mem.Key
}

// detailContent word-wraps the selected memory's body to innerWidth and
// renders the (persisted-scroll-position) viewport. Rewrapping happens at
// render time rather than at selection time so a live terminal resize
// re-wraps correctly without needing a separate resize hook.
func (m MemoriesModel) detailContent(innerWidth, innerHeight int) string {
	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}
	mem, ok := m.selectedMemory()
	body := "Select a memory to view its contents."
	if ok {
		body = mem.Body
	}
	wrapped := lipgloss.NewStyle().Width(innerWidth).Render(body)
	m.detail.SetWidth(innerWidth)
	m.detail.SetHeight(innerHeight)
	m.detail.SetContent(wrapped)
	return m.detail.View()
}

// renderSplitPanels renders the master (left) and detail (right) panels
// side by side via lipgloss.JoinHorizontal, mirroring renderSplitView
// (model_view.go).
func (m MemoriesModel) renderSplitPanels(panelH int) string {
	leftWidth := m.width * 35 / 100
	if leftWidth < 28 {
		leftWidth = 28
	}
	if leftWidth > m.width-24 {
		leftWidth = m.width - 24
	}
	if leftWidth < 10 {
		leftWidth = 10
	}
	rightWidth := m.width - leftWidth

	left := RenderTitledPanel(m.renderMasterLines(), PanelOpts{
		Title:   fmt.Sprintf("Memories (%d)", m.memoryRowCount()),
		Width:   leftWidth,
		Height:  panelH,
		Focused: !m.detailFocused,
	})
	right := RenderTitledPanel(m.detailContent(rightWidth-4, panelH-2), PanelOpts{
		Title:   m.detailTitleText(),
		Width:   rightWidth,
		Height:  panelH,
		Focused: m.detailFocused,
	})

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// renderSinglePanel collapses to one full-width pane below
// SplitViewThreshold (small-terminal usability, bead acceptance: 14-30 row
// terminals) - master by default, detail once FocusDetail/ToggleDetailFocus
// switches over. Mirrors LabelDashboard's isSplitView collapse convention.
func (m MemoriesModel) renderSinglePanel(panelH int) string {
	if m.detailFocused {
		return RenderTitledPanel(m.detailContent(m.width-4, panelH-2), PanelOpts{
			Title:   m.detailTitleText(),
			Width:   m.width,
			Height:  panelH,
			Focused: true,
		})
	}
	return RenderTitledPanel(m.renderMasterLines(), PanelOpts{
		Title:   fmt.Sprintf("Memories (%d) - tab/enter: detail", m.memoryRowCount()),
		Width:   m.width,
		Height:  panelH,
		Focused: true,
	})
}

// View renders the full memories master/detail surface.
func (m MemoriesModel) View() string {
	if !m.hasAnyMemories() {
		return m.renderEmptyState()
	}

	notes := m.renderNotes()
	panelH := m.panelHeight()

	var body string
	if m.isSplitWidth() {
		body = m.renderSplitPanels(panelH)
	} else {
		body = m.renderSinglePanel(panelH)
	}

	if notes == "" {
		return body
	}
	return lipgloss.JoinVertical(lipgloss.Left, notes, body)
}
