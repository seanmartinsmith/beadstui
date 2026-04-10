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
		ThemeFg("#1a1a2e"), // 0: dark blue/gray - empty
		ThemeFg("#16213e"), // 1: navy - very few
		ThemeFg("#0f4c75"), // 2: blue - few
		ThemeFg("#3282b8"), // 3: light blue - some
		ThemeFg("#bbe1fa"), // 4: pale blue - moderate (transition)
		ThemeFg("#f7dc6f"), // 5: gold - above average
		ThemeFg("#e94560"), // 6: coral - many
		ThemeFg("#ff2e63"), // 7: hot pink/red - hot
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
		return ThemeBg("#1a1a2e"), ThemeFg("#6272a4") // Dark bg, muted fg
	}

	// Select background color based on intensity
	switch {
	case intensity >= 0.8:
		return ThemeBg("#ff2e63"), ThemeFg("#ffffff") // Hot pink, white text
	case intensity >= 0.6:
		return ThemeBg("#e94560"), ThemeFg("#ffffff") // Coral, white text
	case intensity >= 0.4:
		return ThemeBg("#f7dc6f"), ThemeFg("#1a1a2e") // Gold, dark text
	case intensity >= 0.2:
		return ThemeBg("#3282b8"), ThemeFg("#ffffff") // Blue, white text
	default:
		return ThemeBg("#16213e"), ThemeFg("#bbe1fa") // Navy, light text
	}
}

// RepoColors maps repo prefixes to distinctive colors for visual differentiation
// These colors are designed to be visible on both light and dark backgrounds
var RepoColors = []lipgloss.AdaptiveColor{
	{Light: "#CC5555", Dark: "#FF6B6B"}, // Coral red
	{Light: "#3BA89E", Dark: "#4ECDC4"}, // Teal
	{Light: "#3891A6", Dark: "#45B7D1"}, // Sky blue
	{Light: "#6B9E87", Dark: "#96CEB4"}, // Sage green
	{Light: "#AA7AAA", Dark: "#DDA0DD"}, // Plum
	{Light: "#C4A93D", Dark: "#F7DC6F"}, // Gold
	{Light: "#9370A8", Dark: "#BB8FCE"}, // Lavender
	{Light: "#5A9BC2", Dark: "#85C1E9"}, // Light blue
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
