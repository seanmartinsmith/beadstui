package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// NewUpdateModal tests
// ============================================================================

func TestNewUpdateModal_InitialState(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "https://github.com/example/release", theme)

	if m.state != UpdateStateConfirm {
		t.Errorf("expected initial state UpdateStateConfirm, got %v", m.state)
	}
	if m.newVersion != "v1.0.0" {
		t.Errorf("expected newVersion v1.0.0, got %s", m.newVersion)
	}
	if m.confirmFocus != 0 {
		t.Errorf("expected confirmFocus 0 (Update button), got %d", m.confirmFocus)
	}
	if !m.IsConfirming() {
		t.Error("expected IsConfirming() to return true")
	}
	if m.IsComplete() {
		t.Error("expected IsComplete() to return false")
	}
	if m.IsInProgress() {
		t.Error("expected IsInProgress() to return false")
	}
}

// ============================================================================
// State helper tests
// ============================================================================

func TestUpdateModal_IsConfirming(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)

	if !m.IsConfirming() {
		t.Error("expected IsConfirming() true in confirm state")
	}

	m.state = UpdateStateDownloading
	if m.IsConfirming() {
		t.Error("expected IsConfirming() false in downloading state")
	}
}

func TestUpdateModal_IsCancelled(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)

	// Initially focused on Update button
	if m.IsCancelled() {
		t.Error("expected IsCancelled() false when focused on Update")
	}

	// Move focus to Cancel
	m.confirmFocus = 1
	if !m.IsCancelled() {
		t.Error("expected IsCancelled() true when focused on Cancel")
	}

	// Not cancelled if not in confirm state
	m.state = UpdateStateDownloading
	if m.IsCancelled() {
		t.Error("expected IsCancelled() false when not in confirm state")
	}
}

func TestUpdateModal_IsComplete(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)

	if m.IsComplete() {
		t.Error("expected IsComplete() false in confirm state")
	}

	m.state = UpdateStateSuccess
	if !m.IsComplete() {
		t.Error("expected IsComplete() true in success state")
	}

	m.state = UpdateStateError
	if !m.IsComplete() {
		t.Error("expected IsComplete() true in error state")
	}
}

func TestUpdateModal_IsInProgress(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)

	if m.IsInProgress() {
		t.Error("expected IsInProgress() false in confirm state")
	}

	for _, state := range []UpdateState{UpdateStateDownloading, UpdateStateVerifying, UpdateStateInstalling} {
		m.state = state
		if !m.IsInProgress() {
			t.Errorf("expected IsInProgress() true for state %v", state)
		}
	}
}

// ============================================================================
// Update (key handling) tests
// ============================================================================

func TestUpdateModal_Update_NavigationKeys(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))

	tests := []struct {
		name          string
		key           string
		expectedFocus int
	}{
		{"left moves to Update", "left", 0},
		{"h moves to Update", "h", 0},
		{"right moves to Cancel", "right", 1},
		{"l moves to Cancel", "l", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewUpdateModal("v1.0.0", "", theme)
			// Start with focus in middle state
			m.confirmFocus = 0
			if tt.expectedFocus == 0 {
				m.confirmFocus = 1 // Start on Cancel to test moving to Update
			}

			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			if tt.key == "left" {
				msg = tea.KeyMsg{Type: tea.KeyLeft}
			} else if tt.key == "right" {
				msg = tea.KeyMsg{Type: tea.KeyRight}
			}

			updated, _ := m.Update(msg)
			if updated.confirmFocus != tt.expectedFocus {
				t.Errorf("expected confirmFocus %d, got %d", tt.expectedFocus, updated.confirmFocus)
			}
		})
	}
}

func TestUpdateModal_Update_TabCyclesFocus(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)

	// Start on Update (0)
	if m.confirmFocus != 0 {
		t.Fatal("expected initial focus on Update")
	}

	// Tab to Cancel (1)
	msg := tea.KeyMsg{Type: tea.KeyTab}
	m, _ = m.Update(msg)
	if m.confirmFocus != 1 {
		t.Errorf("expected focus 1 after first tab, got %d", m.confirmFocus)
	}

	// Tab back to Update (0)
	m, _ = m.Update(msg)
	if m.confirmFocus != 0 {
		t.Errorf("expected focus 0 after second tab, got %d", m.confirmFocus)
	}
}

func TestUpdateModal_Update_QuickConfirmY(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
	updated, cmd := m.Update(msg)

	if updated.state != UpdateStateDownloading {
		t.Errorf("expected state Downloading after Y, got %v", updated.state)
	}
	if cmd == nil {
		t.Error("expected command to be returned for update")
	}
	if updated.startTime.IsZero() {
		t.Error("expected startTime to be set")
	}
}

func TestUpdateModal_Update_QuickConfirmUpperY(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")}
	updated, cmd := m.Update(msg)

	if updated.state != UpdateStateDownloading {
		t.Errorf("expected state Downloading after Y, got %v", updated.state)
	}
	if cmd == nil {
		t.Error("expected command to be returned")
	}
}

func TestUpdateModal_Update_QuickCancelN(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	updated, cmd := m.Update(msg)

	// State should remain confirm, cmd should be nil (parent handles close)
	if updated.state != UpdateStateConfirm {
		t.Errorf("expected state to remain Confirm, got %v", updated.state)
	}
	if cmd != nil {
		t.Error("expected nil command for cancel")
	}
}

func TestUpdateModal_Update_EnterConfirmsUpdate(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)
	m.confirmFocus = 0 // Focus on Update

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, cmd := m.Update(msg)

	if updated.state != UpdateStateDownloading {
		t.Errorf("expected state Downloading, got %v", updated.state)
	}
	if cmd == nil {
		t.Error("expected command to be returned")
	}
}

func TestUpdateModal_Update_EnterCancels(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)
	m.confirmFocus = 1 // Focus on Cancel

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, cmd := m.Update(msg)

	// Parent handles the close, state remains confirm
	if updated.state != UpdateStateConfirm {
		t.Errorf("expected state Confirm, got %v", updated.state)
	}
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestUpdateModal_Update_IgnoresKeysWhenInProgress(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)
	m.state = UpdateStateDownloading

	// Try to cancel - should be ignored
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	updated, _ := m.Update(msg)

	if updated.state != UpdateStateDownloading {
		t.Errorf("expected state to remain Downloading, got %v", updated.state)
	}
}

func TestUpdateModal_Update_DismissOnComplete(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))

	for _, state := range []UpdateState{UpdateStateSuccess, UpdateStateError} {
		t.Run(state.String(), func(t *testing.T) {
			m := NewUpdateModal("v1.0.0", "", theme)
			m.state = state

			msg := tea.KeyMsg{Type: tea.KeyEnter}
			updated, cmd := m.Update(msg)

			// State remains, parent handles dismiss
			if updated.state != state {
				t.Errorf("expected state %v, got %v", state, updated.state)
			}
			if cmd != nil {
				t.Error("expected nil command")
			}
		})
	}
}

// ============================================================================
// Message handling tests
// ============================================================================

func TestUpdateModal_Update_ProgressMessage(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)
	m.state = UpdateStateDownloading

	msg := UpdateProgressMsg{
		Progress: UpdateProgress{
			BytesDownloaded: 500,
			TotalBytes:      1000,
			Stage:           "downloading",
		},
	}

	updated, _ := m.Update(msg)

	if updated.progress.BytesDownloaded != 500 {
		t.Errorf("expected BytesDownloaded 500, got %d", updated.progress.BytesDownloaded)
	}
	if updated.progress.TotalBytes != 1000 {
		t.Errorf("expected TotalBytes 1000, got %d", updated.progress.TotalBytes)
	}
}

func TestUpdateModal_Update_ProgressStageTransitions(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))

	tests := []struct {
		stage    string
		expected UpdateState
	}{
		{"downloading", UpdateStateDownloading},
		{"verifying", UpdateStateVerifying},
		{"installing", UpdateStateInstalling},
	}

	for _, tt := range tests {
		t.Run(tt.stage, func(t *testing.T) {
			m := NewUpdateModal("v1.0.0", "", theme)
			m.state = UpdateStateDownloading

			msg := UpdateProgressMsg{
				Progress: UpdateProgress{Stage: tt.stage},
			}

			updated, _ := m.Update(msg)
			if updated.state != tt.expected {
				t.Errorf("expected state %v, got %v", tt.expected, updated.state)
			}
		})
	}
}

func TestUpdateModal_Update_CompleteMessageSuccess(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)
	m.state = UpdateStateDownloading

	msg := UpdateCompleteMsg{
		Success:    true,
		Message:    "Updated successfully",
		NewVersion: "v1.0.0",
		BackupPath: "/tmp/bv.backup",
	}

	updated, _ := m.Update(msg)

	if updated.state != UpdateStateSuccess {
		t.Errorf("expected state Success, got %v", updated.state)
	}
	if updated.successMessage != "Updated successfully" {
		t.Errorf("expected successMessage, got %s", updated.successMessage)
	}
	if updated.backupPath != "/tmp/bv.backup" {
		t.Errorf("expected backupPath, got %s", updated.backupPath)
	}
}

func TestUpdateModal_Update_CompleteMessageError(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)
	m.state = UpdateStateDownloading

	msg := UpdateCompleteMsg{
		Success: false,
		Message: "Download failed",
	}

	updated, _ := m.Update(msg)

	if updated.state != UpdateStateError {
		t.Errorf("expected state Error, got %v", updated.state)
	}
	if updated.errorMessage != "Download failed" {
		t.Errorf("expected errorMessage, got %s", updated.errorMessage)
	}
}

// ============================================================================
// View rendering tests
// ============================================================================

func TestUpdateModal_View_ConfirmState(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v2.0.0", "", theme)
	m.SetSize(80, 24)

	view := m.View()

	if !strings.Contains(view, "Update Available") {
		t.Error("expected 'Update Available' header in view")
	}
	if !strings.Contains(view, "v2.0.0") {
		t.Error("expected new version in view")
	}
	if !strings.Contains(view, "Update") {
		t.Error("expected Update button in view")
	}
	if !strings.Contains(view, "Cancel") {
		t.Error("expected Cancel button in view")
	}
}

func TestUpdateModal_View_DownloadingState(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v2.0.0", "", theme)
	m.state = UpdateStateDownloading
	m.startTime = time.Now()
	m.SetSize(80, 24)

	view := m.View()

	if !strings.Contains(view, "Updating") {
		t.Error("expected 'Updating' in view")
	}
	if !strings.Contains(view, "Downloading") {
		t.Error("expected 'Downloading' in view")
	}
}

func TestUpdateModal_View_SuccessState(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v2.0.0", "", theme)
	m.state = UpdateStateSuccess
	m.successMessage = "Update complete"
	m.backupPath = "/tmp/backup"
	m.SetSize(80, 24)

	view := m.View()

	if !strings.Contains(view, "Update Complete") {
		t.Error("expected 'Update Complete' in view")
	}
	if !strings.Contains(view, "Update complete") {
		t.Error("expected success message in view")
	}
	if !strings.Contains(view, "/tmp/backup") {
		t.Error("expected backup path in view")
	}
	if !strings.Contains(view, "rollback") {
		t.Error("expected rollback hint in view")
	}
}

func TestUpdateModal_View_ErrorState(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v2.0.0", "", theme)
	m.state = UpdateStateError
	m.errorMessage = "Network error"
	m.SetSize(80, 24)

	view := m.View()

	if !strings.Contains(view, "Update Failed") {
		t.Error("expected 'Update Failed' in view")
	}
	if !strings.Contains(view, "Network error") {
		t.Error("expected error message in view")
	}
}

// ============================================================================
// SetSize tests
// ============================================================================

func TestUpdateModal_SetSize(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)

	m.SetSize(100, 40)
	if m.width != 70 { // Capped at 70
		t.Errorf("expected width 70 (max), got %d", m.width)
	}

	m.SetSize(40, 20)
	if m.width != 50 { // Min 50
		t.Errorf("expected width 50 (min), got %d", m.width)
	}

	m.SetSize(65, 30)
	if m.width != 55 { // 65-10 = 55
		t.Errorf("expected width 55, got %d", m.width)
	}
}

// ============================================================================
// CenterModal tests
// ============================================================================

func TestUpdateModal_CenterModal(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)
	m.SetSize(80, 24)

	centered := m.CenterModal(100, 50)

	// Just verify it doesn't panic and returns non-empty content
	if centered == "" {
		t.Error("expected non-empty centered modal")
	}
}

// ============================================================================
// Spinner and progress bar tests
// ============================================================================

func TestUpdateModal_RenderSpinner(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)
	m.startTime = time.Now()

	spinner := m.renderSpinner()

	// Should return a non-empty spinner character
	if spinner == "" {
		t.Error("expected non-empty spinner")
	}
}

func TestUpdateModal_RenderProgressBar_Indeterminate(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)
	m.progress.TotalBytes = 0

	bar := m.renderProgressBar()

	if !strings.Contains(bar, "[") || !strings.Contains(bar, "]") {
		t.Errorf("expected brackets in progress bar, got %s", bar)
	}
}

func TestUpdateModal_RenderProgressBar_WithProgress(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)
	m.progress.BytesDownloaded = 500
	m.progress.TotalBytes = 1000

	bar := m.renderProgressBar()

	if !strings.Contains(bar, "50%") {
		t.Errorf("expected 50%% in progress bar, got %s", bar)
	}
	if !strings.Contains(bar, "█") {
		t.Errorf("expected filled blocks in progress bar, got %s", bar)
	}
}

func TestUpdateModal_RenderProgressBar_Complete(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	m := NewUpdateModal("v1.0.0", "", theme)
	m.progress.BytesDownloaded = 1000
	m.progress.TotalBytes = 1000

	bar := m.renderProgressBar()

	if !strings.Contains(bar, "100%") {
		t.Errorf("expected 100%% in progress bar, got %s", bar)
	}
}

// ============================================================================
// UpdateState String helper (for test output)
// ============================================================================

func (s UpdateState) String() string {
	switch s {
	case UpdateStateIdle:
		return "Idle"
	case UpdateStateConfirm:
		return "Confirm"
	case UpdateStateDownloading:
		return "Downloading"
	case UpdateStateVerifying:
		return "Verifying"
	case UpdateStateInstalling:
		return "Installing"
	case UpdateStateSuccess:
		return "Success"
	case UpdateStateError:
		return "Error"
	default:
		return "Unknown"
	}
}
