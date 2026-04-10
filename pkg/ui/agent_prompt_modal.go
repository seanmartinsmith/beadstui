package ui

import (
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/agents"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// AgentPromptResult represents the user's choice on the AGENTS.md prompt.
type AgentPromptResult int

const (
	AgentPromptPending AgentPromptResult = iota
	AgentPromptAccept
	AgentPromptDecline
	AgentPromptNeverAsk
)

// AgentPromptModal is a modal dialog for the AGENTS.md prompt.
type AgentPromptModal struct {
	selection int    // 0=yes, 1=no, 2=never
	filePath  string // Which file we're offering to modify
	fileType  string // AGENTS.md or CLAUDE.md
	result    AgentPromptResult
	theme     Theme
	width     int
	height    int
}

// NewAgentPromptModal creates a new AGENTS.md prompt modal.
func NewAgentPromptModal(filePath, fileType string, theme Theme) AgentPromptModal {
	return AgentPromptModal{
		selection: 0, // Default to "Yes"
		filePath:  filePath,
		fileType:  fileType,
		result:    AgentPromptPending,
		theme:     theme,
		width:     60,
		height:    20,
	}
}

// Update handles input for the modal.
func (m AgentPromptModal) Update(msg tea.Msg) (AgentPromptModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "left", "h", "shift+tab":
			m.selection--
			if m.selection < 0 {
				m.selection = 2
			}
		case "right", "l", "tab":
			m.selection++
			if m.selection > 2 {
				m.selection = 0
			}
		case "enter", "space":
			switch m.selection {
			case 0:
				m.result = AgentPromptAccept
			case 1:
				m.result = AgentPromptDecline
			case 2:
				m.result = AgentPromptNeverAsk
			}
		case "y", "Y":
			m.result = AgentPromptAccept
		case "n", "N":
			m.result = AgentPromptDecline
		case "d", "D":
			m.result = AgentPromptNeverAsk
		case "esc", "q":
			m.result = AgentPromptDecline
		}
	}
	return m, nil
}

// View renders the modal.
func (m AgentPromptModal) View() string {

	// Modal container style
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.theme.Primary).
		Padding(1, 2).
		Width(m.width)

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary).
		MarginBottom(1)

	// Body text
	bodyStyle := lipgloss.NewStyle().
		Foreground(ColorText)

	// Preview box
	previewBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.theme.Border).
		Padding(0, 1).
		Width(m.width - 8).
		MaxHeight(8)

	previewHeaderStyle := lipgloss.NewStyle().
		Foreground(m.theme.Subtext).
		Italic(true)

	// Buttons
	buttonBase := lipgloss.NewStyle().
		Padding(0, 2).
		MarginRight(1)

	selectedButton := buttonBase.
		Background(m.theme.Primary).
		Foreground(ColorBgContrast).
		Bold(true)

	unselectedButton := buttonBase.
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.theme.Border)

	muteButton := buttonBase.
		Foreground(m.theme.Subtext)

	// Build content
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("📝 Enhance AI Agent Integration?"))
	b.WriteString("\n\n")

	// Body
	b.WriteString(bodyStyle.Render("We found " + m.fileType + " in this project but it"))
	b.WriteString("\n")
	b.WriteString(bodyStyle.Render("doesn't include beadstui instructions."))
	b.WriteString("\n\n")
	b.WriteString(bodyStyle.Render("Adding these helps AI coding agents understand"))
	b.WriteString("\n")
	b.WriteString(bodyStyle.Render("how to use your issue tracking workflow."))
	b.WriteString("\n\n")

	// Preview
	b.WriteString(previewHeaderStyle.Render("Preview of content to add:"))
	b.WriteString("\n")

	preview := getBlurbPreview()
	b.WriteString(previewBoxStyle.Render(preview))
	b.WriteString("\n\n")

	// Buttons
	var buttons []string

	// Yes button
	yesLabel := "Yes, add it"
	if m.selection == 0 {
		buttons = append(buttons, selectedButton.Render(yesLabel))
	} else {
		buttons = append(buttons, unselectedButton.Render(yesLabel))
	}

	// No button
	noLabel := "No thanks"
	if m.selection == 1 {
		buttons = append(buttons, selectedButton.Render(noLabel))
	} else {
		buttons = append(buttons, unselectedButton.Render(noLabel))
	}

	// Never button
	neverLabel := "Don't ask again"
	if m.selection == 2 {
		buttons = append(buttons, selectedButton.Render(neverLabel))
	} else {
		buttons = append(buttons, muteButton.Render(neverLabel))
	}

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, buttons...))

	// Footer hint
	hintStyle := lipgloss.NewStyle().
		Foreground(m.theme.Subtext).
		Italic(true).
		MarginTop(1)
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("← → to select • Enter to confirm • Esc to cancel"))

	return modalStyle.Render(b.String())
}

// Result returns the user's choice, or AgentPromptPending if still deciding.
func (m AgentPromptModal) Result() AgentPromptResult {
	return m.result
}

// FilePath returns the path of the file to modify.
func (m AgentPromptModal) FilePath() string {
	return m.filePath
}

// SetSize sets the modal dimensions.
func (m *AgentPromptModal) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// getBlurbPreview returns a truncated preview of the blurb content.
func getBlurbPreview() string {
	// Get first few lines of the blurb content (skip marker)
	lines := strings.Split(agents.AgentBlurb, "\n")

	var preview []string
	lineCount := 0
	for _, line := range lines {
		// Skip marker lines
		if strings.HasPrefix(line, "<!--") {
			continue
		}
		// Skip empty lines at start
		if lineCount == 0 && strings.TrimSpace(line) == "" {
			continue
		}
		// Skip horizontal rules
		if strings.TrimSpace(line) == "---" {
			continue
		}
		preview = append(preview, line)
		lineCount++
		if lineCount >= 6 {
			break
		}
	}

	return strings.Join(preview, "\n") + "\n..."
}

// CenterModal returns the modal view centered in the given dimensions.
func (m AgentPromptModal) CenterModal(termWidth, termHeight int) string {
	modal := m.View()

	// Get actual rendered dimensions
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)

	// Calculate padding
	padTop := (termHeight - modalHeight) / 2
	padLeft := (termWidth - modalWidth) / 2

	if padTop < 0 {
		padTop = 0
	}
	if padLeft < 0 {
		padLeft = 0
	}


	// Create centered version
	centered := lipgloss.NewStyle().
		MarginTop(padTop).
		MarginLeft(padLeft).
		Render(modal)

	return centered
}
