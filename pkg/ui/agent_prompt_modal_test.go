package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestNewAgentPromptModal(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	modal := NewAgentPromptModal("/test/AGENTS.md", "AGENTS.md", theme)

	if modal.filePath != "/test/AGENTS.md" {
		t.Errorf("Expected filePath '/test/AGENTS.md', got %q", modal.filePath)
	}
	if modal.fileType != "AGENTS.md" {
		t.Errorf("Expected fileType 'AGENTS.md', got %q", modal.fileType)
	}
	if modal.selection != 0 {
		t.Errorf("Expected initial selection 0, got %d", modal.selection)
	}
	if modal.result != AgentPromptPending {
		t.Errorf("Expected initial result Pending, got %d", modal.result)
	}
}

func TestAgentPromptModalKeyNavigation(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	modal := NewAgentPromptModal("/test/AGENTS.md", "AGENTS.md", theme)

	// Initial selection is 0 (Yes)
	if modal.selection != 0 {
		t.Errorf("Expected initial selection 0, got %d", modal.selection)
	}

	// Press right
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if modal.selection != 1 {
		t.Errorf("Expected selection 1 after right, got %d", modal.selection)
	}

	// Press right again
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if modal.selection != 2 {
		t.Errorf("Expected selection 2 after right, got %d", modal.selection)
	}

	// Press right - should wrap to 0
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if modal.selection != 0 {
		t.Errorf("Expected selection to wrap to 0, got %d", modal.selection)
	}

	// Press left - should wrap to 2
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if modal.selection != 2 {
		t.Errorf("Expected selection to wrap to 2, got %d", modal.selection)
	}
}

func TestAgentPromptModalEnterConfirms(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}

	tests := []struct {
		name           string
		selection      int
		expectedResult AgentPromptResult
	}{
		{"yes", 0, AgentPromptAccept},
		{"no", 1, AgentPromptDecline},
		{"never", 2, AgentPromptNeverAsk},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modal := NewAgentPromptModal("/test/AGENTS.md", "AGENTS.md", theme)
			modal.selection = tt.selection

			modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if modal.result != tt.expectedResult {
				t.Errorf("Expected result %d, got %d", tt.expectedResult, modal.result)
			}
		})
	}
}

func TestAgentPromptModalShortcuts(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}

	tests := []struct {
		key            string
		expectedResult AgentPromptResult
	}{
		{"y", AgentPromptAccept},
		{"Y", AgentPromptAccept},
		{"n", AgentPromptDecline},
		{"N", AgentPromptDecline},
		{"d", AgentPromptNeverAsk},
		{"D", AgentPromptNeverAsk},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			modal := NewAgentPromptModal("/test/AGENTS.md", "AGENTS.md", theme)
			modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
			if modal.result != tt.expectedResult {
				t.Errorf("Key %q: expected result %d, got %d", tt.key, tt.expectedResult, modal.result)
			}
		})
	}
}

func TestAgentPromptModalEscDismisses(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	modal := NewAgentPromptModal("/test/AGENTS.md", "AGENTS.md", theme)

	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if modal.result != AgentPromptDecline {
		t.Errorf("Escape should decline, got %d", modal.result)
	}
}

func TestAgentPromptModalView(t *testing.T) {
	theme := Theme{
		Renderer:  lipgloss.DefaultRenderer(),
		Primary:   lipgloss.AdaptiveColor{Light: "#00ff00", Dark: "#00ff00"},
		Secondary: lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
		Subtext:   lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
		Border:    lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
	}
	modal := NewAgentPromptModal("/test/AGENTS.md", "AGENTS.md", theme)

	view := modal.View()

	// Should contain title
	if !strings.Contains(view, "Enhance AI Agent Integration") {
		t.Error("View should contain title")
	}

	// Should contain buttons
	if !strings.Contains(view, "Yes, add it") {
		t.Error("View should contain Yes button")
	}
	if !strings.Contains(view, "No thanks") {
		t.Error("View should contain No button")
	}
	if !strings.Contains(view, "Don't ask again") {
		t.Error("View should contain Never button")
	}

	// Should contain preview
	if !strings.Contains(view, "Preview") {
		t.Error("View should contain preview section")
	}

	// Should contain file type
	if !strings.Contains(view, "AGENTS.md") {
		t.Error("View should mention file type")
	}
}

func TestAgentPromptModalResult(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	modal := NewAgentPromptModal("/test/AGENTS.md", "AGENTS.md", theme)

	if modal.Result() != AgentPromptPending {
		t.Error("Initial result should be Pending")
	}

	modal.result = AgentPromptAccept
	if modal.Result() != AgentPromptAccept {
		t.Error("Result should return set value")
	}
}

func TestAgentPromptModalFilePath(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	modal := NewAgentPromptModal("/my/project/AGENTS.md", "AGENTS.md", theme)

	if modal.FilePath() != "/my/project/AGENTS.md" {
		t.Errorf("FilePath() = %q, want '/my/project/AGENTS.md'", modal.FilePath())
	}
}

func TestAgentPromptModalSetSize(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	modal := NewAgentPromptModal("/test/AGENTS.md", "AGENTS.md", theme)

	modal.SetSize(80, 30)
	if modal.width != 80 {
		t.Errorf("Expected width 80, got %d", modal.width)
	}
	if modal.height != 30 {
		t.Errorf("Expected height 30, got %d", modal.height)
	}
}

func TestGetBlurbPreview(t *testing.T) {
	preview := getBlurbPreview()

	// Should not be empty
	if preview == "" {
		t.Error("Preview should not be empty")
	}

	// Should end with ellipsis
	if !strings.HasSuffix(preview, "...") {
		t.Error("Preview should end with ellipsis")
	}

	// Should contain some key content
	if !strings.Contains(preview, "Beads") {
		t.Error("Preview should contain 'Beads'")
	}

	// Should not contain HTML comments (markers)
	if strings.Contains(preview, "<!--") {
		t.Error("Preview should not contain HTML comment markers")
	}
}

func TestCenterModal(t *testing.T) {
	theme := Theme{
		Renderer:  lipgloss.DefaultRenderer(),
		Primary:   lipgloss.AdaptiveColor{Light: "#00ff00", Dark: "#00ff00"},
		Secondary: lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
		Subtext:   lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
		Border:    lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
	}
	modal := NewAgentPromptModal("/test/AGENTS.md", "AGENTS.md", theme)

	centered := modal.CenterModal(100, 40)

	// Should not be empty
	if centered == "" {
		t.Error("Centered modal should not be empty")
	}

	// Should still contain the content
	if !strings.Contains(centered, "Enhance AI Agent") {
		t.Error("Centered modal should contain title")
	}
}
