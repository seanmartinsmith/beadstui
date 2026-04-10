package ui

import (
	"os"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
)

// TermProfile holds the detected terminal color profile. Computed once at
// package init so every style helper can branch without re-detecting.
var TermProfile colorprofile.Profile

func init() {
	TermProfile = colorprofile.Detect(os.Stdout, os.Environ())
}

// ThemeBg returns the given hex color for TrueColor terminals and
// lipgloss.NoColor{} otherwise, so 16/256-color terminals use the
// terminal's own background instead of a down-converted approximation
// that may clash with palettes like Solarized.
func ThemeBg(hex string) lipgloss.TerminalColor {
	if TermProfile < colorprofile.TrueColor {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(hex)
}

// ThemeFg returns the given hex color for ANSI256+ terminals and a safe
// ANSI white (color 7) for 16-color or lower terminals.
func ThemeFg(hex string) lipgloss.TerminalColor {
	if TermProfile < colorprofile.ANSI256 {
		return lipgloss.ANSIColor(7)
	}
	return lipgloss.Color(hex)
}

type Theme struct {
	Renderer *lipgloss.Renderer

	// Colors
	Primary   lipgloss.AdaptiveColor
	Secondary lipgloss.AdaptiveColor
	Subtext   lipgloss.AdaptiveColor

	// Status
	Open       lipgloss.AdaptiveColor
	InProgress lipgloss.AdaptiveColor
	Blocked    lipgloss.AdaptiveColor
	Deferred   lipgloss.AdaptiveColor
	Pinned     lipgloss.AdaptiveColor
	Hooked     lipgloss.AdaptiveColor
	Closed     lipgloss.AdaptiveColor
	Tombstone  lipgloss.AdaptiveColor

	// Types
	Bug     lipgloss.AdaptiveColor
	Feature lipgloss.AdaptiveColor
	Task    lipgloss.AdaptiveColor
	Epic    lipgloss.AdaptiveColor
	Chore   lipgloss.AdaptiveColor

	// UI Elements
	Border    lipgloss.AdaptiveColor
	Highlight lipgloss.AdaptiveColor
	Muted     lipgloss.AdaptiveColor

	// Styles
	Base     lipgloss.Style
	Selected lipgloss.Style
	Column   lipgloss.Style
	Header   lipgloss.Style

	// Pre-computed delegate styles (bv-o4cj optimization)
	// These are created once at startup instead of per-frame
	MutedText         lipgloss.Style // Age, muted info
	InfoText          lipgloss.Style // Comments
	InfoBold          lipgloss.Style // Search scores
	SecondaryText     lipgloss.Style // ID, assignee
	PrimaryBold       lipgloss.Style // Selection indicator
	PriorityUpArrow   lipgloss.Style // Priority hint ↑
	PriorityDownArrow lipgloss.Style // Priority hint ↓
	TriageStar        lipgloss.Style // Top pick ⭐
	TriageUnblocks    lipgloss.Style // Unblocks indicator 🔓
	TriageUnblocksAlt lipgloss.Style // Secondary unblocks ↪
}

// DefaultTheme returns the standard Dracula-inspired theme (adaptive)
func DefaultTheme(r *lipgloss.Renderer) Theme {
	t := Theme{
		Renderer: r,

		// Dracula / Light Mode equivalent
		// Light mode colors improved for WCAG AA compliance (bv-3fcg)
		Primary:   lipgloss.AdaptiveColor{Light: "#6B47D9", Dark: "#BD93F9"}, // Purple (darker for contrast)
		Secondary: lipgloss.AdaptiveColor{Light: "#555555", Dark: "#6272A4"}, // Gray
		Subtext:   lipgloss.AdaptiveColor{Light: "#666666", Dark: "#BFBFBF"}, // Dim (was #999999, now ~6:1)

		Open:       lipgloss.AdaptiveColor{Light: "#007700", Dark: "#50FA7B"}, // Green (was #00A800, now ~4.6:1)
		InProgress: lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"}, // Cyan (darker for contrast)
		Blocked:    lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#FF5555"}, // Red (slightly adjusted)
		Deferred:   lipgloss.AdaptiveColor{Light: "#B06800", Dark: "#FFB86C"}, // Orange - on ice
		Pinned:     lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#6699FF"}, // Blue - persistent
		Hooked:     lipgloss.AdaptiveColor{Light: "#008080", Dark: "#00CED1"}, // Teal - agent-attached
		Closed:     lipgloss.AdaptiveColor{Light: "#555555", Dark: "#6272A4"}, // Gray
		Tombstone:  lipgloss.AdaptiveColor{Light: "#888888", Dark: "#44475A"}, // Muted gray - deleted

		Bug:     lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#FF5555"}, // Red
		Feature: lipgloss.AdaptiveColor{Light: "#B06800", Dark: "#FFB86C"}, // Orange (darker for contrast)
		Epic:    lipgloss.AdaptiveColor{Light: "#6B47D9", Dark: "#BD93F9"}, // Purple (darker)
		Task:    lipgloss.AdaptiveColor{Light: "#808000", Dark: "#F1FA8C"}, // Yellow/olive (darker for contrast)
		Chore:   lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"}, // Cyan (darker)

		Border:    lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#44475A"}, // Border (was #DDDDDD)
		Highlight: lipgloss.AdaptiveColor{Light: "#E0E0E0", Dark: "#44475A"}, // Slightly darker
		Muted:     lipgloss.AdaptiveColor{Light: "#555555", Dark: "#6272A4"}, // Dimmed text (was #888888, now ~7:1)
	}

	t.Base = r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#F8F8F2"})

	t.Selected = r.NewStyle().
		Background(t.Highlight).
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(t.Primary).
		PaddingLeft(1).
		Bold(true)

	t.Header = r.NewStyle().
		Background(t.Primary).
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#282A36"}).
		Bold(true).
		Padding(0, 1)

	// Pre-computed delegate styles (bv-o4cj optimization)
	// Reduces ~16 NewStyle() allocations per visible item per frame
	t.MutedText = r.NewStyle().Foreground(ColorMuted)
	t.InfoText = r.NewStyle().Foreground(ColorInfo)
	t.InfoBold = r.NewStyle().Foreground(ColorInfo).Bold(true)
	t.SecondaryText = r.NewStyle().Foreground(t.Secondary)
	t.PrimaryBold = r.NewStyle().Foreground(t.Primary).Bold(true)
	t.PriorityUpArrow = r.NewStyle().Foreground(ThemeFg("#FF6B6B")).Bold(true)
	t.PriorityDownArrow = r.NewStyle().Foreground(ThemeFg("#4ECDC4")).Bold(true)
	t.TriageStar = r.NewStyle().Foreground(ThemeFg("#FFD700"))
	t.TriageUnblocks = r.NewStyle().Foreground(ThemeFg("#50FA7B"))
	t.TriageUnblocksAlt = r.NewStyle().Foreground(ThemeFg("#6272A4"))

	return t
}

func (t Theme) GetStatusColor(s string) lipgloss.AdaptiveColor {
	switch s {
	case "open":
		return t.Open
	case "in_progress":
		return t.InProgress
	case "blocked":
		return t.Blocked
	case "closed":
		return t.Closed
	default:
		return t.Subtext
	}
}

func (t Theme) GetTypeIcon(typ string) (string, lipgloss.AdaptiveColor) {
	switch typ {
	case "bug":
		return "🐛", t.Bug
	case "feature":
		return "✨", t.Feature
	case "task":
		return "📋", t.Task
	case "epic":
		// Use 🚀 instead of 🏔️ - the snow-capped mountain has a variation selector
		// (U+FE0F) that causes inconsistent width calculations across terminals
		return "🚀", t.Epic
	case "chore":
		return "🧹", t.Chore
	default:
		return "•", t.Subtext
	}
}
