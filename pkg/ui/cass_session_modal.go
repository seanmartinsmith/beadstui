package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/cass"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CassSessionModal displays correlated cass sessions for a bead.
// It shows session previews with agent name, timestamp, match reason, and snippet.
type CassSessionModal struct {
	beadID     string              // The bead this modal is showing sessions for
	sessions   []cass.ScoredResult // Correlated sessions to display
	strategy   cass.CorrelationStrategy
	keywords   []string // Keywords used for correlation (for display)
	selected   int      // Currently selected session (for keyboard nav)
	searchCmd  string   // Command to run for more results
	theme      Theme
	width      int
	height     int
	copied     bool      // Flash feedback for clipboard copy
	copiedAt   time.Time // When copy happened
	maxDisplay int       // Max sessions to show (rest are summarized)
}

// NewCassSessionModal creates a modal from correlation results.
func NewCassSessionModal(beadID string, result cass.CorrelationResult, theme Theme) CassSessionModal {
	searchCmd := fmt.Sprintf("cass search %q", beadID)
	if len(result.Keywords) > 0 {
		searchCmd = fmt.Sprintf("cass search %q", strings.Join(result.Keywords, " "))
	}

	return CassSessionModal{
		beadID:     beadID,
		sessions:   result.TopSessions,
		strategy:   result.Strategy,
		keywords:   result.Keywords,
		selected:   0,
		searchCmd:  searchCmd,
		theme:      theme,
		width:      70,
		height:     25,
		maxDisplay: 3,
	}
}

// Update handles input for the modal.
func (m CassSessionModal) Update(msg tea.Msg) (CassSessionModal, tea.Cmd) {
	// Calculate the number of sessions actually displayed (capped by maxDisplay)
	displayCount := len(m.sessions)
	if displayCount > m.maxDisplay {
		displayCount = m.maxDisplay
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if displayCount > 1 && m.selected < displayCount-1 {
				m.selected++
			}
		case "k", "up":
			if m.selected > 0 {
				m.selected--
			}
		case "y":
			// Copy search command to clipboard
			if err := copyToClipboard(m.searchCmd); err == nil {
				m.copied = true
				m.copiedAt = time.Now()
			}
		}
	}
	return m, nil
}

// View renders the modal.
func (m CassSessionModal) View() string {
	r := m.theme.Renderer

	// Check if copy flash should be shown (within 2 seconds of copy)
	showCopied := m.copied && time.Since(m.copiedAt) <= 2*time.Second

	// Modal container style
	modalStyle := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		Padding(1, 2).
		Width(m.width)

	// Header style
	headerStyle := r.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary)

	beadIDStyle := r.NewStyle().
		Foreground(m.theme.Subtext)

	// Session card styles
	sessionHeaderStyle := r.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#F8F8F2"})

	selectedSessionStyle := r.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary)

	matchReasonStyle := r.NewStyle().
		Foreground(m.theme.Subtext).
		Italic(true)

	snippetBoxStyle := r.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.theme.Border).
		Padding(0, 1).
		Width(m.width - 10)

	footerStyle := r.NewStyle().
		Foreground(m.theme.Subtext).
		Italic(true)

	// Build content
	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render("📎 Related Coding Sessions"))
	b.WriteString("  ")
	b.WriteString(beadIDStyle.Render(m.beadID))
	b.WriteString("\n\n")

	// Sessions
	if len(m.sessions) == 0 {
		b.WriteString(matchReasonStyle.Render("No correlated sessions found."))
		b.WriteString("\n\n")
	} else {
		displayCount := len(m.sessions)
		if displayCount > m.maxDisplay {
			displayCount = m.maxDisplay
		}

		for i := 0; i < displayCount; i++ {
			session := m.sessions[i]

			// Session number with selection indicator
			numPrefix := fmt.Sprintf("[%d] ", i+1)
			if i == m.selected {
				b.WriteString(selectedSessionStyle.Render(numPrefix))
			} else {
				b.WriteString(sessionHeaderStyle.Render(numPrefix))
			}

			// Agent and timestamp
			agentStr := session.Agent
			if agentStr == "" {
				agentStr = "Unknown"
			}
			timeStr := formatRelativeTime(session.Timestamp)

			sessionInfo := fmt.Sprintf("%s • %s", agentStr, timeStr)
			if i == m.selected {
				b.WriteString(selectedSessionStyle.Render(sessionInfo))
			} else {
				b.WriteString(sessionHeaderStyle.Render(sessionInfo))
			}
			b.WriteString("\n")

			// Match reason
			matchReason := m.formatMatchReason(session)
			b.WriteString("    ")
			b.WriteString(matchReasonStyle.Render(matchReason))
			b.WriteString("\n")

			// Snippet box
			snippet := m.formatSnippet(session.Snippet)
			b.WriteString(snippetBoxStyle.Render(snippet))
			b.WriteString("\n\n")
		}

		// Show count of additional sessions
		if len(m.sessions) > m.maxDisplay {
			extra := len(m.sessions) - m.maxDisplay
			moreText := fmt.Sprintf("(%d more session", extra)
			if extra > 1 {
				moreText += "s"
			}
			moreText += fmt.Sprintf(" - run: %s)", m.searchCmd)
			b.WriteString(matchReasonStyle.Render(moreText))
			b.WriteString("\n\n")
		}
	}

	// Footer with keybindings
	footerText := "[j/k] Navigate    [y] Copy search cmd    [V/Esc] Close"
	if showCopied {
		footerText = "[j/k] Navigate    ✓ Copied!              [V/Esc] Close"
	}
	b.WriteString(footerStyle.Render(footerText))

	return modalStyle.Render(b.String())
}

// formatMatchReason creates a human-readable match reason string.
func (m CassSessionModal) formatMatchReason(session cass.ScoredResult) string {
	switch session.Strategy {
	case cass.StrategyIDMention:
		return fmt.Sprintf("Matched via: bead ID mentioned (%s)", m.beadID)
	case cass.StrategyKeywords:
		if len(session.Keywords) > 0 {
			return fmt.Sprintf("Matched via: keywords %q", strings.Join(session.Keywords, ", "))
		}
		return "Matched via: keyword search"
	case cass.StrategyTimestamp:
		return "Matched via: recent activity timeframe"
	case cass.StrategyCombined:
		return "Matched via: multiple signals"
	default:
		return fmt.Sprintf("Matched via: %s", session.Strategy)
	}
}

// formatSnippet cleans and truncates a snippet for display.
func (m CassSessionModal) formatSnippet(snippet string) string {
	if snippet == "" {
		return "(no preview available)"
	}

	// Clean up the snippet
	lines := strings.Split(snippet, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Truncate long lines
		maxLineLen := m.width - 14 // Account for box padding
		if len(line) > maxLineLen {
			line = line[:maxLineLen-3] + "..."
		}
		cleaned = append(cleaned, line)
		if len(cleaned) >= 3 {
			break
		}
	}

	if len(cleaned) == 0 {
		return "(no preview available)"
	}
	return strings.Join(cleaned, "\n")
}

// formatRelativeTime formats a timestamp as a relative time string.
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown time"
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 48*time.Hour:
		return "yesterday"
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		return t.Format("Jan 2, 2006")
	}
}

// SetSize sets the modal dimensions based on terminal size.
func (m *CassSessionModal) SetSize(width, height int) {
	// Constrain width
	maxWidth := width - 10
	if maxWidth < 50 {
		maxWidth = 50
	}
	if maxWidth > 80 {
		maxWidth = 80
	}
	m.width = maxWidth
	m.height = height
}

// HasSessions returns true if there are sessions to display.
func (m CassSessionModal) HasSessions() bool {
	return len(m.sessions) > 0
}

// CenterModal returns the modal view centered in the given dimensions.
func (m CassSessionModal) CenterModal(termWidth, termHeight int) string {
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

	r := m.theme.Renderer

	// Create centered version
	centered := r.NewStyle().
		MarginTop(padTop).
		MarginLeft(padLeft).
		Render(modal)

	return centered
}

// copyToClipboard copies text to the system clipboard.
// It uses platform-specific commands and fails silently if unavailable.
func copyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, then xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("no clipboard utility found")
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := stdin.Write([]byte(text)); err != nil {
		return err
	}
	stdin.Close()

	return cmd.Wait()
}
