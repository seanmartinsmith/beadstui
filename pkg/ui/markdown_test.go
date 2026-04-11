package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestNewMarkdownRenderer(t *testing.T) {
	mr := NewMarkdownRenderer(80)
	if mr == nil {
		t.Fatal("NewMarkdownRenderer returned nil")
	}
	if mr.width != 80 {
		t.Errorf("expected width 80, got %d", mr.width)
	}
	if mr.useTheme {
		t.Error("expected useTheme to be false for NewMarkdownRenderer")
	}
	if mr.theme != nil {
		t.Error("expected theme to be nil for NewMarkdownRenderer")
	}
}

func TestNewMarkdownRendererWithTheme(t *testing.T) {
	theme := DefaultTheme()
	mr := NewMarkdownRendererWithTheme(80, theme)
	if mr == nil {
		t.Fatal("NewMarkdownRendererWithTheme returned nil")
	}
	if mr.width != 80 {
		t.Errorf("expected width 80, got %d", mr.width)
	}
	if !mr.useTheme {
		t.Error("expected useTheme to be true for NewMarkdownRendererWithTheme")
	}
	if mr.theme == nil {
		t.Error("expected theme to be stored")
	}
}

func TestMarkdownRenderer_Render(t *testing.T) {
	mr := NewMarkdownRenderer(80)
	result, err := mr.Render("# Hello\n\nWorld")
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	// Should contain "Hello" somewhere in the rendered output
	if !strings.Contains(result, "Hello") {
		t.Errorf("expected result to contain 'Hello', got: %s", result)
	}
}

func TestMarkdownRenderer_RenderNilRenderer(t *testing.T) {
	mr := &MarkdownRenderer{
		renderer: nil,
		width:    80,
	}
	result, err := mr.Render("# Test")
	if err != nil {
		t.Fatalf("Render with nil renderer should not error: %v", err)
	}
	if result != "# Test" {
		t.Errorf("expected raw markdown when renderer is nil, got: %s", result)
	}
}

func TestMarkdownRenderer_SetWidth(t *testing.T) {
	mr := NewMarkdownRenderer(80)
	originalRenderer := mr.renderer

	// Same width should not recreate renderer
	mr.SetWidth(80)
	if mr.renderer != originalRenderer {
		t.Error("SetWidth with same width should not recreate renderer")
	}

	// Invalid width should not change anything
	mr.SetWidth(0)
	if mr.width != 80 {
		t.Error("SetWidth with 0 should not change width")
	}
	mr.SetWidth(-1)
	if mr.width != 80 {
		t.Error("SetWidth with negative should not change width")
	}

	// Different width should update
	mr.SetWidth(100)
	if mr.width != 100 {
		t.Errorf("expected width 100, got %d", mr.width)
	}
}

func TestMarkdownRenderer_SetWidthPreservesTheme(t *testing.T) {
	theme := DefaultTheme()
	mr := NewMarkdownRendererWithTheme(80, theme)

	if !mr.useTheme {
		t.Fatal("expected useTheme to be true")
	}

	// SetWidth should preserve theme
	mr.SetWidth(100)
	if mr.width != 100 {
		t.Errorf("expected width 100, got %d", mr.width)
	}
	if !mr.useTheme {
		t.Error("SetWidth should preserve useTheme flag")
	}
	if mr.theme == nil {
		t.Error("SetWidth should preserve theme")
	}
}

func TestMarkdownRenderer_SetWidthWithTheme(t *testing.T) {
	mr := NewMarkdownRenderer(80)

	if mr.useTheme {
		t.Fatal("expected useTheme to be false initially")
	}

	theme := DefaultTheme()
	mr.SetWidthWithTheme(100, theme)

	if mr.width != 100 {
		t.Errorf("expected width 100, got %d", mr.width)
	}
	if !mr.useTheme {
		t.Error("SetWidthWithTheme should set useTheme to true")
	}
	if mr.theme == nil {
		t.Error("SetWidthWithTheme should store theme")
	}
}

func TestMarkdownRenderer_SetWidthWithThemeSameWidth(t *testing.T) {
	// SetWidthWithTheme should allow updating theme even with same width
	theme := DefaultTheme()
	mr := NewMarkdownRendererWithTheme(80, theme)

	originalRenderer := mr.renderer

	// Same width but (conceptually) different theme should recreate renderer
	mr.SetWidthWithTheme(80, theme)

	// Renderer should be recreated (different instance)
	if mr.renderer == originalRenderer {
		t.Error("SetWidthWithTheme with same width should still recreate renderer")
	}
	if mr.width != 80 {
		t.Errorf("expected width 80, got %d", mr.width)
	}
}

func TestMarkdownRenderer_SetWidthWithThemeInvalidWidth(t *testing.T) {
	mr := NewMarkdownRenderer(80)
	originalRenderer := mr.renderer

	mr.SetWidthWithTheme(0, DefaultTheme())
	if mr.width != 80 {
		t.Error("SetWidthWithTheme with width 0 should not change width")
	}
	if mr.renderer != originalRenderer {
		t.Error("SetWidthWithTheme with width 0 should not change renderer")
	}

	mr.SetWidthWithTheme(-1, DefaultTheme())
	if mr.width != 80 {
		t.Error("SetWidthWithTheme with negative width should not change width")
	}
}

func TestMarkdownRenderer_IsDarkMode(t *testing.T) {
	mr := NewMarkdownRenderer(80)
	// Just verify it returns a boolean without panicking
	_ = mr.IsDarkMode()
}

func TestExtractHex(t *testing.T) {
	white := lipgloss.Color("#ffffff")
	black := lipgloss.Color("#000000")

	whiteHex := extractHex(white, false)
	if whiteHex != "#ffffff" {
		t.Errorf("expected #ffffff, got %s", whiteHex)
	}

	blackHex := extractHex(black, true)
	if blackHex != "#000000" {
		t.Errorf("expected #000000, got %s", blackHex)
	}
}

func TestBuildStyleFromTheme(t *testing.T) {
	theme := DefaultTheme()

	// Test dark mode
	darkConfig := buildStyleFromTheme(theme, true)
	if darkConfig.Document.Color == nil {
		t.Error("expected Document.Color to be set")
	}
	if *darkConfig.Document.Color != "#c5c8c6" {
		t.Errorf("expected dark mode doc color #c5c8c6, got %s", *darkConfig.Document.Color)
	}
	// Dark mode background should be nil (transparent) to avoid Solarized/16-color
	// terminal issues where hex colors get downconverted to wrong ANSI slots (#101)
	if darkConfig.Document.BackgroundColor != nil {
		t.Errorf("expected dark mode BackgroundColor to be nil (transparent), got %v", *darkConfig.Document.BackgroundColor)
	}

	// Test light mode
	lightConfig := buildStyleFromTheme(theme, false)
	if *lightConfig.Document.Color != "#4d4d4c" {
		t.Errorf("expected light mode doc color #4d4d4c, got %s", *lightConfig.Document.Color)
	}
	// Light mode should have nil background (use terminal default)
	if lightConfig.Document.BackgroundColor != nil {
		t.Errorf("expected light mode BackgroundColor to be nil, got %v", lightConfig.Document.BackgroundColor)
	}
}
