package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
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
// COLOR PALETTE - Resolved colors for the current light/dark background
// WCAG AA compliance (contrast ratio >= 4.5:1) for light mode colors
// ══════════════════════════════════════════════════════════════════════════════

var (
	// Base colors - Tomorrow Night palette
	ColorBg          color.Color
	ColorBgDark      color.Color
	ColorBgSubtle    color.Color
	ColorBgHighlight color.Color
	ColorText        color.Color
	ColorSubtext     color.Color
	ColorMuted       color.Color

	// Primary accent - teal (matcha-dark-sea)
	ColorPrimary   color.Color
	ColorSecondary color.Color
	ColorInfo      color.Color
	ColorSuccess   color.Color
	ColorWarning   color.Color
	ColorDanger    color.Color

	// Semantic tokens for inline hex replacement
	ColorTextSecondary color.Color
	ColorBgContrast    color.Color

	// Status colors
	ColorStatusOpen       color.Color
	ColorStatusInProgress color.Color
	ColorStatusBlocked    color.Color
	ColorStatusDeferred   color.Color
	ColorStatusPinned     color.Color
	ColorStatusHooked     color.Color
	ColorStatusReview     color.Color
	ColorStatusClosed     color.Color
	ColorStatusTombstone  color.Color

	// Status background colors (for badges) - subtle tinted backgrounds
	ColorStatusOpenBg       color.Color
	ColorStatusInProgressBg color.Color
	ColorStatusBlockedBg    color.Color
	ColorStatusDeferredBg   color.Color
	ColorStatusPinnedBg     color.Color
	ColorStatusHookedBg     color.Color
	ColorStatusReviewBg     color.Color
	ColorStatusClosedBg     color.Color
	ColorStatusTombstoneBg  color.Color

	// Priority colors
	ColorPrioCritical color.Color
	ColorPrioHigh     color.Color
	ColorPrioMedium   color.Color
	ColorPrioLow      color.Color

	// Priority background colors
	ColorPrioCriticalBg color.Color
	ColorPrioHighBg     color.Color
	ColorPrioMediumBg   color.Color
	ColorPrioLowBg      color.Color

	// Type colors
	ColorTypeBug     color.Color
	ColorTypeFeature color.Color
	ColorTypeTask    color.Color
	ColorTypeEpic    color.Color
	ColorTypeChore   color.Color

	// Gate colors (blocking coordination indicators)
	ColorGateHuman color.Color
	ColorGateTimer color.Color
	ColorGateCI    color.Color
	ColorGateOther color.Color

	ColorGateHumanBg color.Color
	ColorGateTimerBg color.Color
	ColorGateCIBg    color.Color
	ColorGateOtherBg color.Color

	// Human advisory flag (non-blocking label)
	ColorHumanAdvisory   color.Color
	ColorHumanAdvisoryBg color.Color

	// Stale/overdue indicators
	ColorOverdue   color.Color
	ColorOverdueBg color.Color
	ColorStale     color.Color

	// UI chrome
	ColorBorder    color.Color
	ColorHighlight color.Color
)

// resolveColors resolves all package-level Color* vars based on the current
// isDarkBackground state. Called at init and when tea.BackgroundColorMsg
// detects a mode change.
func resolveColors() {
	ColorBg = resolveColor("#ffffff", "#1d1f21")
	ColorBgDark = resolveColor("#f0f0f0", "#191b1d")
	ColorBgSubtle = resolveColor("#efefef", "#282a2e")
	ColorBgHighlight = resolveColor("#d6d6d6", "#373b41")
	ColorText = resolveColor("#4d4d4c", "#c5c8c6")
	ColorSubtext = resolveColor("#8e908c", "#b4b7b4")
	ColorMuted = resolveColor("#8e908c", "#969896")

	ColorPrimary = resolveColor("#3e999f", "#8abeb7")
	ColorSecondary = resolveColor("#8e908c", "#969896")
	ColorInfo = resolveColor("#4271ae", "#81a2be")
	ColorSuccess = resolveColor("#718c00", "#b5bd68")
	ColorWarning = resolveColor("#f5871f", "#de935f")
	ColorDanger = resolveColor("#c82829", "#cc6666")

	ColorTextSecondary = resolveColor("#333333", "#e8e8e8")
	ColorBgContrast = resolveColor("#ffffff", "#1d1f21")

	ColorStatusOpen = resolveColor("#718c00", "#b5bd68")
	ColorStatusInProgress = resolveColor("#4271ae", "#81a2be")
	ColorStatusBlocked = resolveColor("#c82829", "#cc6666")
	ColorStatusDeferred = resolveColor("#f5871f", "#de935f")
	ColorStatusPinned = resolveColor("#4271ae", "#7aa6da")
	ColorStatusHooked = resolveColor("#3e999f", "#8abeb7")
	ColorStatusReview = resolveColor("#8959a8", "#b294bb")
	ColorStatusClosed = resolveColor("#8e908c", "#969896")
	ColorStatusTombstone = resolveColor("#c5c8c6", "#373b41")

	ColorStatusOpenBg = resolveColor("#e8f0e0", "#252e1e")
	ColorStatusInProgressBg = resolveColor("#dce8f0", "#1e2530")
	ColorStatusBlockedBg = resolveColor("#f0dce0", "#2e1e1e")
	ColorStatusDeferredBg = resolveColor("#f0e4d8", "#2e251e")
	ColorStatusPinnedBg = resolveColor("#dce4f0", "#1e2230")
	ColorStatusHookedBg = resolveColor("#dce8e8", "#1e2a2a")
	ColorStatusReviewBg = resolveColor("#e4dce8", "#261e2e")
	ColorStatusClosedBg = resolveColor("#e0e0e0", "#252527")
	ColorStatusTombstoneBg = resolveColor("#d6d6d6", "#1d1f21")

	ColorPrioCritical = resolveColor("#c82829", "#cc6666")
	ColorPrioHigh = resolveColor("#f5871f", "#de935f")
	ColorPrioMedium = resolveColor("#eab700", "#f0c674")
	ColorPrioLow = resolveColor("#718c00", "#b5bd68")

	ColorPrioCriticalBg = resolveColor("#f0dce0", "#2e1e1e")
	ColorPrioHighBg = resolveColor("#f0e4d8", "#2e251e")
	ColorPrioMediumBg = resolveColor("#f0ecd8", "#2e2e1e")
	ColorPrioLowBg = resolveColor("#e8f0e0", "#252e1e")

	ColorTypeBug = resolveColor("#c82829", "#cc6666")
	ColorTypeFeature = resolveColor("#f5871f", "#de935f")
	ColorTypeTask = resolveColor("#eab700", "#f0c674")
	ColorTypeEpic = resolveColor("#8959a8", "#b294bb")
	ColorTypeChore = resolveColor("#4271ae", "#81a2be")

	// Gate colors: human=warm red, timer=amber, CI=blue, other=muted
	ColorGateHuman = resolveColor("#c82829", "#cc6666")
	ColorGateTimer = resolveColor("#f5871f", "#de935f")
	ColorGateCI = resolveColor("#4271ae", "#81a2be")
	ColorGateOther = resolveColor("#8959a8", "#b294bb")

	ColorGateHumanBg = resolveColor("#f0dce0", "#2e1e1e")
	ColorGateTimerBg = resolveColor("#f0e4d8", "#2e251e")
	ColorGateCIBg = resolveColor("#dce8f0", "#1e2530")
	ColorGateOtherBg = resolveColor("#e4dce8", "#261e2e")

	// Human advisory: yellow/amber to distinguish from red blocking gate
	ColorHumanAdvisory = resolveColor("#eab700", "#f0c674")
	ColorHumanAdvisoryBg = resolveColor("#f0ecd8", "#2e2e1e")

	// Stale/overdue
	ColorOverdue = resolveColor("#c82829", "#cc6666")
	ColorOverdueBg = resolveColor("#f0dce0", "#2e1e1e")
	ColorStale = resolveColor("#8e908c", "#969896") // muted/dimmed

	ColorBorder = resolveColor("#d6d6d6", "#373b41")
	ColorHighlight = resolveColor("#d6d6d6", "#373b41")

	// Rebuild panel styles with resolved colors
	PanelStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorBgHighlight)
	FocusedPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorPrimary)

	// Rebuild any other color arrays that depend on isDarkBackground
	resolveRepoColors()
}

func init() {
	resolveColors()
}

// ══════════════════════════════════════════════════════════════════════════════
// PANEL STYLES - For split view layouts
// ══════════════════════════════════════════════════════════════════════════════

var (
	// PanelStyle is the default style for unfocused panels
	PanelStyle lipgloss.Style

	// FocusedPanelStyle is the style for focused panels
	FocusedPanelStyle lipgloss.Style
)

// ══════════════════════════════════════════════════════════════════════════════
// BADGE RENDERING - Polished, consistent badge styles
// ══════════════════════════════════════════════════════════════════════════════

// RenderPriorityBadge returns a styled priority badge
// Priority values: 0=Critical, 1=High, 2=Medium, 3=Low, 4=Backlog
func RenderPriorityBadge(priority int) string {
	var fg, bg color.Color
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
	var fg, bg color.Color
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

// RenderGateBadge returns a styled badge for gate-blocked issues.
// awaitType values: "human", "timer", "gh:run", "gh:pr", "bead", etc.
func RenderGateBadge(awaitType string) string {
	var fg, bg color.Color
	var label string

	switch {
	case awaitType == "human":
		fg, bg, label = ColorGateHuman, ColorGateHumanBg, "👤"
	case awaitType == "timer":
		fg, bg, label = ColorGateTimer, ColorGateTimerBg, "⏱TMR"
	case awaitType == "gh:run" || awaitType == "ci":
		fg, bg, label = ColorGateCI, ColorGateCIBg, "⚙CI"
	case awaitType == "gh:pr" || awaitType == "pr":
		fg, bg, label = ColorGateCI, ColorGateCIBg, "⬡PR"
	case awaitType == "bead":
		fg, bg, label = ColorGateOther, ColorGateOtherBg, "⛓BD"
	default:
		fg, bg, label = ColorGateOther, ColorGateOtherBg, "⏸GTD"
	}

	return lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Bold(true).
		Padding(0, 0).
		Render(label)
}

// RenderHumanAdvisoryBadge returns a yellow flag badge for advisory human flags.
// Distinct from gate badge: advisory (label) = yellow, blocking (gate) = red.
func RenderHumanAdvisoryBadge() string {
	return lipgloss.NewStyle().
		Foreground(ColorHumanAdvisory).
		Background(ColorHumanAdvisoryBg).
		Bold(true).
		Padding(0, 0).
		Render("👤")
}

// RenderStateDimensionBadge returns a styled badge for a state dimension:value label.
// Known dimensions get distinct colors; others get a default muted style.
func RenderStateDimensionBadge(dim, val string) string {
	var fg color.Color
	switch dim {
	case "health":
		fg = ColorDanger
	case "patrol":
		fg = ColorSuccess
	case "mode":
		fg = ColorInfo
	default:
		fg = ColorSecondary
	}

	label := dim + ":" + val
	return lipgloss.NewStyle().
		Foreground(fg).
		Background(ColorBgSubtle).
		Padding(0, 0).
		Render(label)
}

// RenderOverdueBadge returns a red badge for overdue issues.
func RenderOverdueBadge() string {
	return lipgloss.NewStyle().
		Foreground(ColorOverdue).
		Background(ColorOverdueBg).
		Bold(true).
		Padding(0, 0).
		Render("⏰DUE")
}

// RenderStaleBadge returns a muted badge for stale issues.
func RenderStaleBadge() string {
	return lipgloss.NewStyle().
		Foreground(ColorStale).
		Padding(0, 0).
		Render("💤")
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
	var barColor color.Color
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
	return lipgloss.NewStyle().Foreground(barColor).Render(bar)
}

// RenderRankBadge renders a rank badge like "#1" with color based on percentile
func RenderRankBadge(rank, total int) string {
	if total == 0 {
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("#?")
	}

	percentile := float64(rank) / float64(total)

	var rankColor color.Color
	if percentile <= 0.1 {
		rankColor = ColorSuccess // Top 10%
	} else if percentile <= 0.25 {
		rankColor = ColorInfo // Top 25%
	} else if percentile <= 0.5 {
		rankColor = ColorWarning // Top 50%
	} else {
		rankColor = ColorMuted // Bottom 50%
	}

	return lipgloss.NewStyle().
		Foreground(rankColor).
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
