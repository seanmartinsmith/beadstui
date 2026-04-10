package ui

import (
	"image/color"
	"os"

	"github.com/charmbracelet/colorprofile"
	"charm.land/lipgloss/v2"
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
func ThemeBg(hex string) color.Color {
	if TermProfile < colorprofile.TrueColor {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(hex)
}

// ThemeFg returns the given hex color for ANSI256+ terminals and a safe
// ANSI white (color 7) for 16-color or lower terminals.
func ThemeFg(hex string) color.Color {
	if TermProfile < colorprofile.ANSI256 {
		return lipgloss.ANSIColor(7)
	}
	return lipgloss.Color(hex)
}

type Theme struct {
	// Colors
	Primary   AdaptiveColor
	Secondary AdaptiveColor
	Subtext   AdaptiveColor

	// Accents (map to Color* globals)
	Info    AdaptiveColor
	Success AdaptiveColor
	Warning AdaptiveColor
	Danger  AdaptiveColor

	// Status
	Open       AdaptiveColor
	InProgress AdaptiveColor
	Blocked    AdaptiveColor
	Deferred   AdaptiveColor
	Pinned     AdaptiveColor
	Hooked     AdaptiveColor
	Closed     AdaptiveColor
	Tombstone  AdaptiveColor
	Review     AdaptiveColor

	// Types
	Bug     AdaptiveColor
	Feature AdaptiveColor
	Task    AdaptiveColor
	Epic    AdaptiveColor
	Chore   AdaptiveColor

	// UI Elements
	Border    AdaptiveColor
	Highlight AdaptiveColor
	Muted     AdaptiveColor

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

// DefaultTheme returns the Tomorrow Night theme (adaptive).
func DefaultTheme() Theme {
	t := Theme{
		// Tomorrow Night palette + matcha-dark-sea teal accent
		Primary:   AdaptiveColor{Light: "#3e999f", Dark: "#8abeb7"}, // Teal
		Secondary: AdaptiveColor{Light: "#8e908c", Dark: "#969896"}, // Comment gray
		Subtext:   AdaptiveColor{Light: "#8e908c", Dark: "#b4b7b4"}, // Lighter muted

		Info:    AdaptiveColor{Light: "#4271ae", Dark: "#81a2be"}, // Blue
		Success: AdaptiveColor{Light: "#718c00", Dark: "#b5bd68"}, // Green
		Warning: AdaptiveColor{Light: "#f5871f", Dark: "#de935f"}, // Orange
		Danger:  AdaptiveColor{Light: "#c82829", Dark: "#cc6666"}, // Red

		Open:       AdaptiveColor{Light: "#718c00", Dark: "#b5bd68"}, // Green
		InProgress: AdaptiveColor{Light: "#4271ae", Dark: "#81a2be"}, // Blue
		Blocked:    AdaptiveColor{Light: "#c82829", Dark: "#cc6666"}, // Red
		Deferred:   AdaptiveColor{Light: "#f5871f", Dark: "#de935f"}, // Orange
		Pinned:     AdaptiveColor{Light: "#4271ae", Dark: "#7aa6da"}, // Blue variant
		Hooked:     AdaptiveColor{Light: "#3e999f", Dark: "#8abeb7"}, // Teal
		Closed:     AdaptiveColor{Light: "#8e908c", Dark: "#969896"}, // Gray
		Tombstone:  AdaptiveColor{Light: "#c5c8c6", Dark: "#373b41"}, // Muted
		Review:     AdaptiveColor{Light: "#8959a8", Dark: "#b294bb"}, // Purple

		Bug:     AdaptiveColor{Light: "#c82829", Dark: "#cc6666"}, // Red
		Feature: AdaptiveColor{Light: "#f5871f", Dark: "#de935f"}, // Orange
		Epic:    AdaptiveColor{Light: "#8959a8", Dark: "#b294bb"}, // Purple
		Task:    AdaptiveColor{Light: "#eab700", Dark: "#f0c674"}, // Yellow
		Chore:   AdaptiveColor{Light: "#4271ae", Dark: "#81a2be"}, // Blue

		Border:    AdaptiveColor{Light: "#d6d6d6", Dark: "#373b41"},
		Highlight: AdaptiveColor{Light: "#d6d6d6", Dark: "#373b41"},
		Muted:     AdaptiveColor{Light: "#8e908c", Dark: "#969896"},
	}

	t.Base = lipgloss.NewStyle().Foreground(AdaptiveColor{Light: "#4d4d4c", Dark: "#c5c8c6"})

	t.Selected = lipgloss.NewStyle().
		Background(t.Highlight).
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(t.Primary).
		PaddingLeft(1).
		Bold(true)

	t.Header = lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(AdaptiveColor{Light: "#ffffff", Dark: "#1d1f21"}).
		Bold(true).
		Padding(0, 1)

	// Pre-computed delegate styles (bv-o4cj optimization)
	t.MutedText = lipgloss.NewStyle().Foreground(ColorMuted)
	t.InfoText = lipgloss.NewStyle().Foreground(ColorInfo)
	t.InfoBold = lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
	t.SecondaryText = lipgloss.NewStyle().Foreground(t.Secondary)
	t.PrimaryBold = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	t.PriorityUpArrow = lipgloss.NewStyle().Foreground(ThemeFg("#cc6666")).Bold(true)
	t.PriorityDownArrow = lipgloss.NewStyle().Foreground(ThemeFg("#8abeb7")).Bold(true)
	t.TriageStar = lipgloss.NewStyle().Foreground(ThemeFg("#f0c674"))
	t.TriageUnblocks = lipgloss.NewStyle().Foreground(ThemeFg("#b5bd68"))
	t.TriageUnblocksAlt = lipgloss.NewStyle().Foreground(ThemeFg("#969896"))

	return t
}

func (t Theme) GetStatusColor(s string) AdaptiveColor {
	switch s {
	case "open":
		return t.Open
	case "in_progress":
		return t.InProgress
	case "blocked":
		return t.Blocked
	case "deferred":
		return t.Deferred
	case "pinned":
		return t.Pinned
	case "hooked":
		return t.Hooked
	case "closed":
		return t.Closed
	case "tombstone":
		return t.Tombstone
	default:
		return t.Subtext
	}
}

func (t Theme) GetTypeIcon(typ string) (string, AdaptiveColor) {
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
