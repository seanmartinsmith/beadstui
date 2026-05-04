package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestRenderTitledPanel_Basic(t *testing.T) {
	content := "hello"

	result := RenderTitledPanel(content, PanelOpts{
		Title: "Test",
		Width: 20,
	})

	if !strings.Contains(result, "Test") {
		t.Error("panel should contain title")
	}
	if !strings.Contains(result, "╭") {
		t.Error("panel should have top-left corner")
	}
	if !strings.Contains(result, "╯") {
		t.Error("panel should have bottom-right corner")
	}
	if !strings.Contains(result, "hello") {
		t.Error("panel should contain content")
	}
}

func TestRenderTitledPanel_NoTitle(t *testing.T) {
	result := RenderTitledPanel("content", PanelOpts{
		Width: 20,
	})

	// Should have full horizontal line on top (no title text)
	if !strings.Contains(result, "╭") {
		t.Error("no-title panel should still have border")
	}
	if !strings.Contains(result, "content") {
		t.Error("should contain content")
	}
}

func TestRenderTitledPanel_Focused(t *testing.T) {

	unfocused := RenderTitledPanel("a", PanelOpts{
		Title: "Panel",
		Width: 20,
	})
	focused := RenderTitledPanel("a", PanelOpts{
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
	result := RenderTitledPanel("line1\nline2", PanelOpts{
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
	result := RenderTitledPanel("x", PanelOpts{
		Title: "This Is A Very Long Title That Should Be Truncated",
		Width: 20,
	})

	if !strings.Contains(result, "…") {
		t.Error("long title should be truncated with ellipsis")
	}
}

func TestRenderTitledPanel_MinWidth(t *testing.T) {
	// Should not panic with very small width
	result := RenderTitledPanel("x", PanelOpts{
		Title: "T",
		Width: 2,
	})
	if result == "" {
		t.Error("should produce output even with small width")
	}
}

func TestRenderTitledPanel_Variants(t *testing.T) {

	normal := RenderTitledPanel("x", PanelOpts{
		Title:   "N",
		Width:   20,
		Variant: BorderNormal,
	})
	thick := RenderTitledPanel("x", PanelOpts{
		Title:   "T",
		Width:   20,
		Variant: BorderThick,
	})
	double := RenderTitledPanel("x", PanelOpts{
		Title:   "D",
		Width:   20,
		Variant: BorderDouble,
	})

	if !strings.Contains(normal, "╭") {
		t.Error("normal variant should use ╭")
	}
	if !strings.Contains(thick, "┏") {
		t.Error("thick variant should use ┏")
	}
	if !strings.Contains(double, "╔") {
		t.Error("double variant should use ╔")
	}
}

func TestRenderTitledPanel_ColorOverrides(t *testing.T) {

	customBorder := lipgloss.Color("#ff0000")
	customTitle := lipgloss.Color("#00ff00")

	// With overrides, the panel should still render correctly regardless of Focused
	result := RenderTitledPanel("content", PanelOpts{
		Title:       "Custom",
		Width:       20,
		Focused:     false,
		BorderColor: customBorder,
		TitleColor:  customTitle,
	})

	if !strings.Contains(result, "Custom") {
		t.Error("panel with color overrides should contain title")
	}
	if !strings.Contains(result, "content") {
		t.Error("panel with color overrides should contain content")
	}
	if !strings.Contains(result, "╭") {
		t.Error("panel with color overrides should have border")
	}

	// Overrides should work with focused too
	focusedResult := RenderTitledPanel("x", PanelOpts{
		Title:       "F",
		Width:       20,
		Focused:     true,
		BorderColor: customBorder,
		TitleColor:  customTitle,
	})
	if !strings.Contains(focusedResult, "F") {
		t.Error("focused panel with overrides should contain title")
	}
}

func TestRenderTitledPanel_PartialOverrides(t *testing.T) {

	// Only override border color, let title use default
	customBorder := lipgloss.Color("#ff0000")
	result := RenderTitledPanel("x", PanelOpts{
		Title:       "Partial",
		Width:       20,
		BorderColor: customBorder,
	})
	if !strings.Contains(result, "Partial") {
		t.Error("partial override should still render title")
	}

	// Only override title color, let border use default
	customTitle := lipgloss.Color("#00ff00")
	result2 := RenderTitledPanel("x", PanelOpts{
		Title:      "Partial2",
		Width:      20,
		TitleColor: customTitle,
	})
	if !strings.Contains(result2, "Partial2") {
		t.Error("partial title override should still render title")
	}
}

func TestRenderTitledPanel_RightLabel(t *testing.T) {
	// RightLabel renders on the top border with corner stability preserved.
	result := RenderTitledPanel("body", PanelOpts{
		Title:      "Alerts!",
		RightLabel: "(219)",
		Width:      40,
	})
	if !strings.Contains(result, "Alerts!") {
		t.Errorf("right-label render should still show title; got:\n%s", result)
	}
	if !strings.Contains(result, "(219)") {
		t.Errorf("right-label should appear in output; got:\n%s", result)
	}
	// Top border row is first line; both title and label should be there.
	firstLine := strings.SplitN(result, "\n", 2)[0]
	if !strings.Contains(firstLine, "Alerts!") || !strings.Contains(firstLine, "(219)") {
		t.Errorf("title and right-label both expected on top border; got:\n%s", firstLine)
	}
	// Width of the top line equals Width (corner stability).
	if w := lipgloss.Width(firstLine); w != 40 {
		t.Errorf("top border width expected 40 (opts.Width), got %d; line=%q", w, firstLine)
	}

	// Empty RightLabel is a no-op (backwards compat).
	plain := RenderTitledPanel("body", PanelOpts{Title: "Alerts!", Width: 40})
	if strings.Contains(plain, "(") {
		t.Errorf("empty RightLabel should not introduce parens; got:\n%s", plain)
	}
}

// TestRenderTitledPanel_RightLabelOnly covers the bt-fxbl variant where the
// caller wants the label rendered ONLY on the right (no left title). Used
// by the Issues panel in renderSplitView so the title doesn't compete
// visually with the column header right below it.
func TestRenderTitledPanel_RightLabelOnly(t *testing.T) {
	result := RenderTitledPanel("body", PanelOpts{
		RightLabel: "Issues",
		Width:      40,
	})
	firstLine := strings.SplitN(result, "\n", 2)[0]
	if !strings.Contains(firstLine, "Issues") {
		t.Errorf("right-label-only panel should show label on top border; got:\n%s", firstLine)
	}
	// Width should still equal opts.Width (corner stability).
	if w := lipgloss.Width(firstLine); w != 40 {
		t.Errorf("top border width expected 40 (opts.Width), got %d; line=%q", w, firstLine)
	}
	// Sanity: the label appears in the top border (ANSI styling may split
	// "Issues" from the trailing space). Above we already asserted "Issues"
	// is present and the line width matches opts.Width.
}
