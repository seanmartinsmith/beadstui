package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// isDarkBackground defaults to true (dark theme). Phase 2 will set this
// at runtime via tea.BackgroundColorMsg for proper light/dark detection.
// We intentionally avoid calling lipgloss.HasDarkBackground() at init time
// because it queries the terminal and can hang in non-TTY environments
// (tests, CI, pipes).
var isDarkBackground = true

// AdaptiveColor is a Phase 0 compatibility shim for lipgloss v1's
// AdaptiveColor. It holds Light and Dark hex strings and resolves
// to the appropriate one based on isDarkBackground detected at startup.
//
// Phase 2 will replace all 161 occurrences with lipgloss.LightDark() driven
// by tea.BackgroundColorMsg for runtime light/dark switching.
type AdaptiveColor struct {
	Light string
	Dark  string
}

// RGBA implements color.Color. It resolves to the Dark or Light hex color
// based on the cached background detection.
func (ac AdaptiveColor) RGBA() (uint32, uint32, uint32, uint32) {
	if isDarkBackground {
		return lipgloss.Color(ac.Dark).RGBA()
	}
	return lipgloss.Color(ac.Light).RGBA()
}

// Ensure AdaptiveColor satisfies color.Color at compile time.
var _ color.Color = AdaptiveColor{}
