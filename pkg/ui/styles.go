package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ══════════════════════════════════════════════════════════════════════════════
// DESIGN TOKENS - Consistent spacing, colors, and visual language
// ══════════════════════════════════════════════════════════════════════════════

// Spacing constants for consistent layout (in characters)
const (
	SpaceXS = 1
	SpaceSM = 2
	SpaceMD = 3
	SpaceLG = 4
	SpaceXL = 6
)

// ══════════════════════════════════════════════════════════════════════════════
// COLOR PALETTE - Adaptive colors for light and dark terminals
// Light mode colors tuned for WCAG AA compliance (contrast ratio >= 4.5:1)
// ══════════════════════════════════════════════════════════════════════════════

var (
	// Base colors - Tomorrow Night palette
	ColorBg          = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#1d1f21"}
	ColorBgDark      = lipgloss.AdaptiveColor{Light: "#f0f0f0", Dark: "#191b1d"}
	ColorBgSubtle    = lipgloss.AdaptiveColor{Light: "#efefef", Dark: "#282a2e"}
	ColorBgHighlight = lipgloss.AdaptiveColor{Light: "#d6d6d6", Dark: "#373b41"}
	ColorText        = lipgloss.AdaptiveColor{Light: "#4d4d4c", Dark: "#c5c8c6"}
	ColorSubtext     = lipgloss.AdaptiveColor{Light: "#8e908c", Dark: "#b4b7b4"}
	ColorMuted       = lipgloss.AdaptiveColor{Light: "#8e908c", Dark: "#969896"}

	// Primary accent - teal (matcha-dark-sea)
	ColorPrimary   = lipgloss.AdaptiveColor{Light: "#3e999f", Dark: "#8abeb7"}
	ColorSecondary = lipgloss.AdaptiveColor{Light: "#8e908c", Dark: "#969896"}
	ColorInfo      = lipgloss.AdaptiveColor{Light: "#4271ae", Dark: "#81a2be"}
	ColorSuccess   = lipgloss.AdaptiveColor{Light: "#718c00", Dark: "#b5bd68"}
	ColorWarning   = lipgloss.AdaptiveColor{Light: "#f5871f", Dark: "#de935f"}
	ColorDanger    = lipgloss.AdaptiveColor{Light: "#c82829", Dark: "#cc6666"}

	// Semantic tokens for inline hex replacement
	ColorTextSecondary = lipgloss.AdaptiveColor{Light: "#333333", Dark: "#e8e8e8"}
	ColorBgContrast    = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#1d1f21"}

	// Status colors
	ColorStatusOpen       = lipgloss.AdaptiveColor{Light: "#718c00", Dark: "#b5bd68"}
	ColorStatusInProgress = lipgloss.AdaptiveColor{Light: "#4271ae", Dark: "#81a2be"}
	ColorStatusBlocked    = lipgloss.AdaptiveColor{Light: "#c82829", Dark: "#cc6666"}
	ColorStatusDeferred   = lipgloss.AdaptiveColor{Light: "#f5871f", Dark: "#de935f"} // Orange - on ice
	ColorStatusPinned     = lipgloss.AdaptiveColor{Light: "#4271ae", Dark: "#7aa6da"} // Blue - persistent
	ColorStatusHooked     = lipgloss.AdaptiveColor{Light: "#3e999f", Dark: "#8abeb7"} // Teal - agent-attached
	ColorStatusReview     = lipgloss.AdaptiveColor{Light: "#8959a8", Dark: "#b294bb"} // Purple - awaiting review
	ColorStatusClosed     = lipgloss.AdaptiveColor{Light: "#8e908c", Dark: "#969896"}
	ColorStatusTombstone  = lipgloss.AdaptiveColor{Light: "#c5c8c6", Dark: "#373b41"} // Muted gray - deleted

	// Status background colors (for badges) - subtle tinted backgrounds
	ColorStatusOpenBg       = lipgloss.AdaptiveColor{Light: "#e8f0e0", Dark: "#252e1e"}
	ColorStatusInProgressBg = lipgloss.AdaptiveColor{Light: "#dce8f0", Dark: "#1e2530"}
	ColorStatusBlockedBg    = lipgloss.AdaptiveColor{Light: "#f0dce0", Dark: "#2e1e1e"}
	ColorStatusDeferredBg   = lipgloss.AdaptiveColor{Light: "#f0e4d8", Dark: "#2e251e"} // Orange bg
	ColorStatusPinnedBg     = lipgloss.AdaptiveColor{Light: "#dce4f0", Dark: "#1e2230"} // Blue bg
	ColorStatusHookedBg     = lipgloss.AdaptiveColor{Light: "#dce8e8", Dark: "#1e2a2a"} // Teal bg
	ColorStatusReviewBg     = lipgloss.AdaptiveColor{Light: "#e4dce8", Dark: "#261e2e"} // Purple bg
	ColorStatusClosedBg     = lipgloss.AdaptiveColor{Light: "#e0e0e0", Dark: "#252527"}
	ColorStatusTombstoneBg  = lipgloss.AdaptiveColor{Light: "#d6d6d6", Dark: "#1d1f21"} // Dark bg

	// Priority colors
	ColorPrioCritical = lipgloss.AdaptiveColor{Light: "#c82829", Dark: "#cc6666"}
	ColorPrioHigh     = lipgloss.AdaptiveColor{Light: "#f5871f", Dark: "#de935f"}
	ColorPrioMedium   = lipgloss.AdaptiveColor{Light: "#eab700", Dark: "#f0c674"}
	ColorPrioLow      = lipgloss.AdaptiveColor{Light: "#718c00", Dark: "#b5bd68"}

	// Priority background colors
	ColorPrioCriticalBg = lipgloss.AdaptiveColor{Light: "#f0dce0", Dark: "#2e1e1e"}
	ColorPrioHighBg     = lipgloss.AdaptiveColor{Light: "#f0e4d8", Dark: "#2e251e"}
	ColorPrioMediumBg   = lipgloss.AdaptiveColor{Light: "#f0ecd8", Dark: "#2e2e1e"}
	ColorPrioLowBg      = lipgloss.AdaptiveColor{Light: "#e8f0e0", Dark: "#252e1e"}

	// Type colors
	ColorTypeBug     = lipgloss.AdaptiveColor{Light: "#c82829", Dark: "#cc6666"}
	ColorTypeFeature = lipgloss.AdaptiveColor{Light: "#f5871f", Dark: "#de935f"}
	ColorTypeTask    = lipgloss.AdaptiveColor{Light: "#eab700", Dark: "#f0c674"}
	ColorTypeEpic    = lipgloss.AdaptiveColor{Light: "#8959a8", Dark: "#b294bb"}
	ColorTypeChore   = lipgloss.AdaptiveColor{Light: "#4271ae", Dark: "#81a2be"}

	// UI chrome
	ColorBorder    = lipgloss.AdaptiveColor{Light: "#d6d6d6", Dark: "#373b41"}
	ColorHighlight = lipgloss.AdaptiveColor{Light: "#d6d6d6", Dark: "#373b41"}
)

// ══════════════════════════════════════════════════════════════════════════════
// PANEL STYLES - For split view layouts
// ══════════════════════════════════════════════════════════════════════════════

var (
	// PanelStyle is the default style for unfocused panels
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorBgHighlight)

	// FocusedPanelStyle is the style for focused panels
	FocusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(ColorPrimary)
)

// ══════════════════════════════════════════════════════════════════════════════
// BADGE RENDERING - Polished, consistent badge styles
// ══════════════════════════════════════════════════════════════════════════════

// RenderPriorityBadge returns a styled priority badge
// Priority values: 0=Critical, 1=High, 2=Medium, 3=Low, 4=Backlog
func RenderPriorityBadge(priority int) string {
	var fg, bg lipgloss.AdaptiveColor
	var label string

	switch priority {
	case 0:
		fg, bg, label = ColorPrioCritical, ColorPrioCriticalBg, "P0"
	case 1:
		fg, bg, label = ColorPrioHigh, ColorPrioHighBg, "P1"
	case 2:
		fg, bg, label = ColorPrioMedium, ColorPrioMediumBg, "P2"
	case 3:
		fg, bg, label = ColorPrioLow, ColorPrioLowBg, "P3"
	case 4:
		fg, bg, label = ColorMuted, ColorBgSubtle, "P4"
	default:
		fg, bg, label = ColorMuted, ColorBgSubtle, "P?"
	}

	return lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Bold(true).
		Padding(0, 0).
		Render(label)
}

// RenderStatusBadge returns a styled status badge
func RenderStatusBadge(status string) string {
	var fg, bg lipgloss.AdaptiveColor
	var label string

	switch status {
	case "open":
		fg, bg, label = ColorStatusOpen, ColorStatusOpenBg, "OPEN"
	case "in_progress":
		fg, bg, label = ColorStatusInProgress, ColorStatusInProgressBg, "PROG"
	case "blocked":
		fg, bg, label = ColorStatusBlocked, ColorStatusBlockedBg, "BLKD"
	case "deferred":
		fg, bg, label = ColorStatusDeferred, ColorStatusDeferredBg, "DEFR"
	case "pinned":
		fg, bg, label = ColorStatusPinned, ColorStatusPinnedBg, "PIN"
	case "hooked":
		fg, bg, label = ColorStatusHooked, ColorStatusHookedBg, "HOOK"
	case "review":
		fg, bg, label = ColorStatusReview, ColorStatusReviewBg, "REVW"
	case "closed":
		fg, bg, label = ColorStatusClosed, ColorStatusClosedBg, "DONE"
	case "tombstone":
		fg, bg, label = ColorStatusTombstone, ColorStatusTombstoneBg, "TOMB"
	default:
		fg, bg, label = ColorMuted, ColorBgSubtle, "????"
	}

	return lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Padding(0, 0).
		Render(label)
}

// ══════════════════════════════════════════════════════════════════════════════
// METRIC VISUALIZATION - Mini-bars and rank badges
// ══════════════════════════════════════════════════════════════════════════════

// RenderMiniBar renders a mini horizontal bar for a value between 0 and 1
func RenderMiniBar(value float64, width int, t Theme) string {
	if width <= 0 {
		return ""
	}
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}

	filled := int(value * float64(width))
	if filled > width {
		filled = width
	}

	// Choose color based on value
	var barColor lipgloss.AdaptiveColor
	if value >= 0.75 {
		barColor = t.Open // Green/Success
	} else if value >= 0.5 {
		barColor = t.Feature // Orange/Warning
	} else if value >= 0.25 {
		barColor = t.InProgress // Cyan/Info
	} else {
		barColor = t.Secondary // Muted
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return t.Renderer.NewStyle().Foreground(barColor).Render(bar)
}

// RenderRankBadge renders a rank badge like "#1" with color based on percentile
func RenderRankBadge(rank, total int) string {
	if total == 0 {
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("#?")
	}

	percentile := float64(rank) / float64(total)

	var color lipgloss.AdaptiveColor
	if percentile <= 0.1 {
		color = ColorSuccess // Top 10%
	} else if percentile <= 0.25 {
		color = ColorInfo // Top 25%
	} else if percentile <= 0.5 {
		color = ColorWarning // Top 50%
	} else {
		color = ColorMuted // Bottom 50%
	}

	return lipgloss.NewStyle().
		Foreground(color).
		Render(fmt.Sprintf("#%d", rank))
}

// ══════════════════════════════════════════════════════════════════════════════
// DIVIDERS AND SEPARATORS
// ══════════════════════════════════════════════════════════════════════════════

// RenderDivider renders a horizontal divider line
func RenderDivider(width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(ColorBgHighlight).
		Render(strings.Repeat("─", width))
}

// RenderSubtleDivider renders a more subtle divider using dots
func RenderSubtleDivider(width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render(strings.Repeat("·", width))
}
