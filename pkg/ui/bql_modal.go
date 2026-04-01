package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BQLQueryModal is the modal overlay for entering BQL queries.
type BQLQueryModal struct {
	input   textinput.Model
	width   int
	height  int
	theme   Theme
	err     string   // Parse error message (shown inline)
	history []string // Recent queries (session-scoped)
	histIdx int      // -1 = current input, 0+ = history index
	saved   string   // Saved input when navigating history
}

// NewBQLQueryModal creates a new BQL query input modal.
func NewBQLQueryModal(theme Theme) BQLQueryModal {
	ti := textinput.New()
	ti.Placeholder = "status:open priority<P2 label:bug"
	ti.CharLimit = 256
	ti.Width = 60

	return BQLQueryModal{
		input:   ti,
		theme:   theme,
		histIdx: -1,
	}
}

// SetSize updates the modal dimensions.
func (m *BQLQueryModal) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.input.Width = w - 10 // padding for panel border
	if m.input.Width < 20 {
		m.input.Width = 20
	}
}

// Value returns the current input value.
func (m *BQLQueryModal) Value() string {
	return m.input.Value()
}

// SetError sets the parse error message shown below the input.
func (m *BQLQueryModal) SetError(msg string) {
	m.err = msg
}

// AddToHistory adds a query to the history.
func (m *BQLQueryModal) AddToHistory(query string) {
	if query == "" {
		return
	}
	// Deduplicate: remove if already present
	for i, h := range m.history {
		if h == query {
			m.history = append(m.history[:i], m.history[i+1:]...)
			break
		}
	}
	// Prepend (most recent first)
	m.history = append([]string{query}, m.history...)
	// Cap at 20
	if len(m.history) > 20 {
		m.history = m.history[:20]
	}
}

// HistoryPrev navigates to the previous (older) query in history.
func (m *BQLQueryModal) HistoryPrev() {
	if len(m.history) == 0 {
		return
	}
	if m.histIdx == -1 {
		m.saved = m.input.Value()
	}
	if m.histIdx < len(m.history)-1 {
		m.histIdx++
		m.input.SetValue(m.history[m.histIdx])
		m.input.CursorEnd()
	}
}

// HistoryNext navigates to the next (newer) query in history.
func (m *BQLQueryModal) HistoryNext() {
	if m.histIdx <= 0 {
		if m.histIdx == 0 {
			m.histIdx = -1
			m.input.SetValue(m.saved)
			m.input.CursorEnd()
		}
		return
	}
	m.histIdx--
	m.input.SetValue(m.history[m.histIdx])
	m.input.CursorEnd()
}

// Reset clears the input and error state for a fresh query.
func (m *BQLQueryModal) Reset() {
	m.input.SetValue("")
	m.err = ""
	m.histIdx = -1
	m.saved = ""
}

// Focus activates the text input.
func (m *BQLQueryModal) Focus() tea.Cmd {
	return m.input.Focus()
}

// Update handles key events for the text input.
func (m BQLQueryModal) Update(msg tea.Msg) (BQLQueryModal, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	// Clear error when user types
	if m.err != "" {
		m.err = ""
	}
	return m, cmd
}

// View renders the BQL query modal.
func (m BQLQueryModal) View() string {
	var sb strings.Builder

	// Input line
	sb.WriteString(m.input.View())
	sb.WriteString("\n")

	// Error line (if any)
	if m.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555"))
		sb.WriteString(errStyle.Render(fmt.Sprintf("  %s", m.err)))
		sb.WriteString("\n")
	}

	// Help hint
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	sb.WriteString(hint.Render("  enter: apply | esc: cancel | up/down: history"))

	content := sb.String()

	// Wrap in a titled panel
	panelWidth := m.width - 4
	if panelWidth < 30 {
		panelWidth = 30
	}

	panel := RenderTitledPanel(m.theme.Renderer, content, PanelOpts{
		Title: "BQL Query",
		Width: panelWidth,
	})

	// Center the panel vertically
	topPad := (m.height - lipgloss.Height(panel)) / 3
	if topPad < 0 {
		topPad = 0
	}

	return strings.Repeat("\n", topPad) + panel
}
