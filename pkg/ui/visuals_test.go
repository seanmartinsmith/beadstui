package ui_test

import (
	"strings"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/ui"
)

func TestRenderSparkline(t *testing.T) {
	tests := []struct {
		name  string
		val   float64
		width int
	}{
		{"Zero", 0.0, 5},
		{"Full", 1.0, 5},
		{"Half", 0.5, 5},
		{"Small", 0.1, 5},
		{"AlmostFull", 0.99, 5},
		{"Overflow", 1.5, 5},
		{"Underflow", -0.5, 5},
		{"Width1", 0.5, 1},
		{"Width0", 0.5, 0}, // Edge case
		{"VerySmall", 0.01, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("RenderSparkline panicked: %v", r)
				}
			}()
			got := ui.RenderSparkline(tt.val, tt.width)
			if len([]rune(got)) != tt.width {
				if tt.width > 0 { // Allow 0 length for 0 width
					t.Errorf("RenderSparkline length mismatch. Want %d, got %d ('%s')", tt.width, len([]rune(got)), got)
				}
			}
			if strings.Count(got, "\n") > 0 {
				t.Errorf("RenderSparkline contains newlines")
			}
			// Verify visibility for non-zero small values
			if tt.name == "VerySmall" && tt.width > 0 {
				if strings.TrimSpace(got) == "" {
					t.Errorf("RenderSparkline should show visible bar for small values, got empty/spaces: '%s'", got)
				}
			}
		})
	}
}
