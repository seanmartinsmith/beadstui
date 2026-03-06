package ui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderSparkline creates a textual bar chart of value (0.0 - 1.0)
func RenderSparkline(val float64, width int) string {
	if width <= 0 {
		return ""
	}

	chars := []string{" ", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

	if math.IsNaN(val) {
		val = 0
	}
	if val < 0 {
		val = 0
	}
	if val > 1 {
		val = 1
	}

	// Calculate fullness
	fullChars := int(val * float64(width))
	remainder := (val * float64(width)) - float64(fullChars)

	var sb strings.Builder
	for i := 0; i < fullChars; i++ {
		sb.WriteString("█")
	}

	if fullChars < width {
		idx := int(remainder * float64(len(chars)))
		// Ensure non-zero values are visible
		if idx == 0 && remainder > 0 {
			idx = 1
		}
		if idx >= len(chars) {
			idx = len(chars) - 1
		}
		if idx > 0 {
			sb.WriteString(chars[idx])
		} else {
			sb.WriteString(" ")
		}
	}

	// Pad
	padding := width - fullChars - 1
	if padding > 0 {
		sb.WriteString(strings.Repeat(" ", padding))
	}

	return sb.String()
}

// GetHeatmapColor returns a color based on score (0-1)
func GetHeatmapColor(score float64, t Theme) lipgloss.TerminalColor {
	if score > 0.8 {
		return t.Primary // Peak/High
	} else if score > 0.5 {
		return t.Feature // Mid-High
	} else if score > 0.2 {
		return t.InProgress // Low-Mid
	}
	return t.Secondary // Low
}

// HeatmapGradientColors defines the color gradient for enhanced heatmap (bv-t4yg)
// Ordered from cold (low count) to hot (high count).
// Uses ThemeFg so 16-color terminals fall back to ANSI white.
var HeatmapGradientColors []lipgloss.TerminalColor

func init() {
	HeatmapGradientColors = []lipgloss.TerminalColor{
		ThemeFg("#1d1f21"), // 0: bg - empty
		ThemeFg("#282a2e"), // 1: subtle - very few
		ThemeFg("#373b41"), // 2: highlight - few
		ThemeFg("#81a2be"), // 3: blue - some
		ThemeFg("#8abeb7"), // 4: teal - moderate
		ThemeFg("#f0c674"), // 5: yellow - above average
		ThemeFg("#de935f"), // 6: orange - many
		ThemeFg("#cc6666"), // 7: red - hot
	}
}

// GetHeatGradientColor returns an interpolated color for heatmap intensity (0-1) (bv-t4yg)
func GetHeatGradientColor(intensity float64, t Theme) lipgloss.TerminalColor {
	if intensity <= 0 {
		return HeatmapGradientColors[0]
	}
	if intensity >= 1 {
		return HeatmapGradientColors[len(HeatmapGradientColors)-1]
	}

	// Map intensity to gradient index
	idx := int(intensity * float64(len(HeatmapGradientColors)-1))
	if idx >= len(HeatmapGradientColors)-1 {
		idx = len(HeatmapGradientColors) - 2
	}

	return HeatmapGradientColors[idx+1] // +1 because 0 is for empty cells
}

// GetHeatGradientColorBg returns a background-friendly color for heatmap cell (bv-t4yg)
// Returns both the background color and appropriate foreground for contrast.
// On 16-color terminals, backgrounds are transparent and foreground uses ANSI-safe colors.
func GetHeatGradientColorBg(intensity float64) (bg lipgloss.TerminalColor, fg lipgloss.TerminalColor) {
	if intensity <= 0 {
		return ThemeBg("#1d1f21"), ThemeFg("#969896") // Bg, muted fg
	}

	switch {
	case intensity >= 0.8:
		return ThemeBg("#cc6666"), ThemeFg("#1d1f21") // Red, dark text
	case intensity >= 0.6:
		return ThemeBg("#de935f"), ThemeFg("#1d1f21") // Orange, dark text
	case intensity >= 0.4:
		return ThemeBg("#f0c674"), ThemeFg("#1d1f21") // Yellow, dark text
	case intensity >= 0.2:
		return ThemeBg("#81a2be"), ThemeFg("#1d1f21") // Blue, dark text
	default:
		return ThemeBg("#282a2e"), ThemeFg("#81a2be") // Subtle, blue text
	}
}

// RepoColors maps repo prefixes to distinctive colors for visual differentiation
// These colors are designed to be visible on both light and dark backgrounds
var RepoColors = []lipgloss.AdaptiveColor{
	{Light: "#c82829", Dark: "#cc6666"}, // Red
	{Light: "#3e999f", Dark: "#8abeb7"}, // Teal
	{Light: "#4271ae", Dark: "#81a2be"}, // Blue
	{Light: "#718c00", Dark: "#b5bd68"}, // Green
	{Light: "#8959a8", Dark: "#b294bb"}, // Purple
	{Light: "#eab700", Dark: "#f0c674"}, // Yellow
	{Light: "#f5871f", Dark: "#de935f"}, // Orange
	{Light: "#4271ae", Dark: "#7aa6da"}, // Light blue
}

// GetRepoColor returns a consistent color for a repo prefix based on hash
func GetRepoColor(prefix string) lipgloss.AdaptiveColor {
	if prefix == "" {
		return ColorMuted
	}
	// Simple hash based on prefix characters
	hash := 0
	for _, c := range prefix {
		hash = (hash*31 + int(c)) % len(RepoColors)
	}
	if hash < 0 {
		hash = -hash
	}
	return RepoColors[hash%len(RepoColors)]
}

// RenderRepoBadge creates a compact colored badge for a repository prefix
// Example: "api" -> "[API]" with distinctive color
func RenderRepoBadge(prefix string) string {
	if prefix == "" {
		return ""
	}
	// Uppercase and limit to 4 chars for compactness
	display := strings.ToUpper(prefix)
	if len(display) > 4 {
		display = display[:4]
	}

	color := GetRepoColor(prefix)
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Render("[" + display + "]")
}
