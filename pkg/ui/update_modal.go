package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/updater"
	"github.com/Dicklesworthstone/beads_viewer/pkg/version"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateState represents the current state of the update process
type UpdateState int

const (
	UpdateStateIdle UpdateState = iota
	UpdateStateConfirm
	UpdateStateDownloading
	UpdateStateVerifying
	UpdateStateInstalling
	UpdateStateSuccess
	UpdateStateError
)

// UpdateProgress represents progress during download
type UpdateProgress struct {
	BytesDownloaded int64
	TotalBytes      int64
	Stage           string
}

// UpdateProgressMsg is sent during the update process
type UpdateProgressMsg struct {
	Progress UpdateProgress
}

// UpdateCompleteMsg is sent when the update completes
type UpdateCompleteMsg struct {
	Success     bool
	Message     string
	NewVersion  string
	BackupPath  string
	RequireRoot bool
}

// UpdateModal displays the update confirmation and progress.
type UpdateModal struct {
	currentVersion string
	newVersion     string
	releaseURL     string
	state          UpdateState
	progress       UpdateProgress
	errorMessage   string
	successMessage string
	backupPath     string
	theme          Theme
	width          int
	height         int
	startTime      time.Time
	confirmFocus   int // 0 = Update, 1 = Cancel
}

// NewUpdateModal creates a new update modal.
func NewUpdateModal(newVersion, releaseURL string, theme Theme) UpdateModal {
	return UpdateModal{
		currentVersion: version.Version,
		newVersion:     newVersion,
		releaseURL:     releaseURL,
		state:          UpdateStateConfirm,
		theme:          theme,
		width:          60,
		height:         20,
		confirmFocus:   0, // Default to "Update" button
	}
}

// PerformUpdateCmd returns a command that performs the update in the background.
func PerformUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		release, err := updater.GetLatestRelease()
		if err != nil {
			return UpdateCompleteMsg{
				Success: false,
				Message: fmt.Sprintf("Failed to fetch release info: %v", err),
			}
		}

		result, err := updater.PerformUpdate(release, true) // Skip confirm since TUI already confirmed
		if err != nil {
			msg := fmt.Sprintf("Update failed: %v", err)
			requireRoot := false
			if result != nil {
				if result.RequireRoot {
					requireRoot = true
					msg = "Update requires elevated permissions. Run: sudo bv --update"
				}
			}
			return UpdateCompleteMsg{
				Success:     false,
				Message:     msg,
				RequireRoot: requireRoot,
			}
		}

		return UpdateCompleteMsg{
			Success:    result.Success,
			Message:    result.Message,
			NewVersion: result.NewVersion,
			BackupPath: result.BackupPath,
		}
	}
}

// Update handles input for the modal.
func (m UpdateModal) Update(msg tea.Msg) (UpdateModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case UpdateStateConfirm:
			switch msg.String() {
			case "left", "h":
				m.confirmFocus = 0
			case "right", "l":
				m.confirmFocus = 1
			case "tab":
				m.confirmFocus = (m.confirmFocus + 1) % 2
			case "enter":
				if m.confirmFocus == 0 {
					// User confirmed update
					m.state = UpdateStateDownloading
					m.startTime = time.Now()
					return m, PerformUpdateCmd()
				}
				// Cancel - will be handled by parent
				return m, nil
			case "y", "Y":
				// Quick confirm
				m.state = UpdateStateDownloading
				m.startTime = time.Now()
				return m, PerformUpdateCmd()
			case "n", "N":
				// Quick cancel - will be handled by parent
				return m, nil
			}

		case UpdateStateSuccess, UpdateStateError:
			// Any key to dismiss
			switch msg.String() {
			case "enter", "esc", "q":
				// Will be handled by parent to close modal
				return m, nil
			}
		}

	case UpdateProgressMsg:
		m.progress = msg.Progress
		switch m.progress.Stage {
		case "downloading":
			m.state = UpdateStateDownloading
		case "verifying":
			m.state = UpdateStateVerifying
		case "installing":
			m.state = UpdateStateInstalling
		}

	case UpdateCompleteMsg:
		if msg.Success {
			m.state = UpdateStateSuccess
			m.successMessage = msg.Message
			m.backupPath = msg.BackupPath
		} else {
			m.state = UpdateStateError
			m.errorMessage = msg.Message
		}
	}

	return m, nil
}

// View renders the modal.
func (m UpdateModal) View() string {
	r := m.theme.Renderer

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

	// Version styles
	currentVersionStyle := r.NewStyle().
		Foreground(m.theme.Subtext)

	newVersionStyle := r.NewStyle().
		Bold(true).
		Foreground(ColorStatusOpen)

	// Button styles
	buttonStyle := r.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border)

	selectedButtonStyle := r.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		Background(m.theme.Primary).
		Foreground(ColorBg).
		Bold(true)

	successStyle := r.NewStyle().
		Foreground(ColorStatusOpen).
		Bold(true)

	errorStyle := r.NewStyle().
		Foreground(ColorStatusBlocked).
		Bold(true)

	subtextStyle := r.NewStyle().
		Foreground(m.theme.Subtext).
		Italic(true)

	var b strings.Builder

	switch m.state {
	case UpdateStateConfirm:
		b.WriteString(headerStyle.Render("Update Available"))
		b.WriteString("\n\n")

		b.WriteString("Current version: ")
		b.WriteString(currentVersionStyle.Render(m.currentVersion))
		b.WriteString("\n")

		b.WriteString("New version:     ")
		b.WriteString(newVersionStyle.Render(m.newVersion))
		b.WriteString("\n\n")

		b.WriteString("Would you like to update now?\n\n")

		// Buttons
		var updateBtn, cancelBtn string
		if m.confirmFocus == 0 {
			updateBtn = selectedButtonStyle.Render(" Update ")
			cancelBtn = buttonStyle.Render(" Cancel ")
		} else {
			updateBtn = buttonStyle.Render(" Update ")
			cancelBtn = selectedButtonStyle.Render(" Cancel ")
		}
		b.WriteString("    ")
		b.WriteString(updateBtn)
		b.WriteString("  ")
		b.WriteString(cancelBtn)
		b.WriteString("\n\n")

		b.WriteString(subtextStyle.Render("[Y] Update   [N] Cancel   [Enter] Select"))

	case UpdateStateDownloading:
		b.WriteString(headerStyle.Render("Updating..."))
		b.WriteString("\n\n")
		b.WriteString(m.renderSpinner())
		b.WriteString(" Downloading ")
		b.WriteString(newVersionStyle.Render(m.newVersion))
		b.WriteString("...\n\n")
		b.WriteString(m.renderProgressBar())
		b.WriteString("\n\n")
		elapsed := time.Since(m.startTime).Round(time.Second)
		b.WriteString(subtextStyle.Render(fmt.Sprintf("Elapsed: %s", elapsed)))

	case UpdateStateVerifying:
		b.WriteString(headerStyle.Render("Updating..."))
		b.WriteString("\n\n")
		b.WriteString(m.renderSpinner())
		b.WriteString(" Verifying checksum...\n")

	case UpdateStateInstalling:
		b.WriteString(headerStyle.Render("Updating..."))
		b.WriteString("\n\n")
		b.WriteString(m.renderSpinner())
		b.WriteString(" Installing new version...\n")

	case UpdateStateSuccess:
		b.WriteString(successStyle.Render("Update Complete!"))
		b.WriteString("\n\n")
		b.WriteString(m.successMessage)
		b.WriteString("\n\n")
		if m.backupPath != "" {
			b.WriteString(subtextStyle.Render(fmt.Sprintf("Backup: %s", m.backupPath)))
			b.WriteString("\n")
			b.WriteString(subtextStyle.Render("Run 'bv --rollback' to restore if needed"))
			b.WriteString("\n\n")
		}
		b.WriteString(successStyle.Render("Restart bv to use the new version."))
		b.WriteString("\n\n")
		b.WriteString(subtextStyle.Render("[Enter] Close"))

	case UpdateStateError:
		b.WriteString(errorStyle.Render("Update Failed"))
		b.WriteString("\n\n")
		b.WriteString(m.errorMessage)
		b.WriteString("\n\n")
		b.WriteString(subtextStyle.Render("[Enter] Close"))
	}

	return modalStyle.Render(b.String())
}

// renderSpinner returns an animated spinner character
func (m UpdateModal) renderSpinner() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	idx := int(time.Since(m.startTime).Milliseconds()/100) % len(frames)
	return frames[idx]
}

// renderProgressBar renders a progress bar for downloads
func (m UpdateModal) renderProgressBar() string {
	if m.progress.TotalBytes == 0 {
		// Indeterminate progress
		return "[                    ]"
	}

	percent := float64(m.progress.BytesDownloaded) / float64(m.progress.TotalBytes)
	width := 20
	filled := int(percent * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s] %.0f%%", bar, percent*100)
}

// SetSize sets the modal dimensions based on terminal size.
func (m *UpdateModal) SetSize(width, height int) {
	maxWidth := width - 10
	if maxWidth < 50 {
		maxWidth = 50
	}
	if maxWidth > 70 {
		maxWidth = 70
	}
	m.width = maxWidth
	m.height = height
}

// IsConfirming returns true if the modal is in confirm state
func (m UpdateModal) IsConfirming() bool {
	return m.state == UpdateStateConfirm
}

// IsCancelled returns true if user selected Cancel
func (m UpdateModal) IsCancelled() bool {
	return m.state == UpdateStateConfirm && m.confirmFocus == 1
}

// IsComplete returns true if the update is done (success or error)
func (m UpdateModal) IsComplete() bool {
	return m.state == UpdateStateSuccess || m.state == UpdateStateError
}

// IsInProgress returns true if the update is in progress
func (m UpdateModal) IsInProgress() bool {
	return m.state == UpdateStateDownloading ||
		m.state == UpdateStateVerifying ||
		m.state == UpdateStateInstalling
}

// CenterModal returns the modal view centered in the given dimensions.
func (m UpdateModal) CenterModal(termWidth, termHeight int) string {
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
