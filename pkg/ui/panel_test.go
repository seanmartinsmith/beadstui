package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderTitledPanel_Basic(t *testing.T) {
	r := lipgloss.NewRenderer(nil)
	content := "hello"

	result := RenderTitledPanel(r, content, PanelOpts{
		Title: "Test",
		Width: 20,
	})

	if !strings.Contains(result, "Test") {
		t.Error("panel should contain title")
	}
	if !strings.Contains(result, "┌") {
		t.Error("panel should have top-left corner")
	}
	if !strings.Contains(result, "┘") {
		t.Error("panel should have bottom-right corner")
	}
	if !strings.Contains(result, "hello") {
		t.Error("panel should contain content")
	}
}

func TestRenderTitledPanel_NoTitle(t *testing.T) {
	r := lipgloss.NewRenderer(nil)
	result := RenderTitledPanel(r, "content", PanelOpts{
		Width: 20,
	})

	// Should have full horizontal line on top (no title text)
	if !strings.Contains(result, "┌") {
		t.Error("no-title panel should still have border")
	}
	if !strings.Contains(result, "content") {
		t.Error("should contain content")
	}
}

func TestRenderTitledPanel_Focused(t *testing.T) {
	r := lipgloss.NewRenderer(nil)

	unfocused := RenderTitledPanel(r, "a", PanelOpts{
		Title: "Panel",
		Width: 20,
	})
	focused := RenderTitledPanel(r, "a", PanelOpts{
		Title:   "Panel",
		Width:   20,
		Focused: true,
	})

	// Both should have titles but different styling (hard to test colors in unit test)
	if !strings.Contains(unfocused, "Panel") {
		t.Error("unfocused should have title")
	}
	if !strings.Contains(focused, "Panel") {
		t.Error("focused should have title")
	}
}

func TestRenderTitledPanel_Height(t *testing.T) {
	r := lipgloss.NewRenderer(nil)
	result := RenderTitledPanel(r, "line1\nline2", PanelOpts{
		Title:  "H",
		Width:  20,
		Height: 5, // top border + 3 content lines + bottom border
	})

	lines := strings.Split(result, "\n")
	// Should have 5 lines: top border, 3 content, bottom border
	// (result ends with bottom border, no trailing newline from split)
	if len(lines) < 5 {
		t.Errorf("expected at least 5 lines for height=5, got %d", len(lines))
	}
}

func TestRenderTitledPanel_TitleTruncation(t *testing.T) {
	r := lipgloss.NewRenderer(nil)
	result := RenderTitledPanel(r, "x", PanelOpts{
		Title: "This Is A Very Long Title That Should Be Truncated",
		Width: 20,
	})

	if !strings.Contains(result, "…") {
		t.Error("long title should be truncated with ellipsis")
	}
}

func TestRenderTitledPanel_MinWidth(t *testing.T) {
	r := lipgloss.NewRenderer(nil)
	// Should not panic with very small width
	result := RenderTitledPanel(r, "x", PanelOpts{
		Title: "T",
		Width: 2,
	})
	if result == "" {
		t.Error("should produce output even with small width")
	}
}

func TestRenderTitledPanel_Variants(t *testing.T) {
	r := lipgloss.NewRenderer(nil)

	normal := RenderTitledPanel(r, "x", PanelOpts{
		Title:   "N",
		Width:   20,
		Variant: BorderNormal,
	})
	thick := RenderTitledPanel(r, "x", PanelOpts{
		Title:   "T",
		Width:   20,
		Variant: BorderThick,
	})
	double := RenderTitledPanel(r, "x", PanelOpts{
		Title:   "D",
		Width:   20,
		Variant: BorderDouble,
	})

	if !strings.Contains(normal, "┌") {
		t.Error("normal variant should use ┌")
	}
	if !strings.Contains(thick, "┏") {
		t.Error("thick variant should use ┏")
	}
	if !strings.Contains(double, "╔") {
		t.Error("double variant should use ╔")
	}
}
