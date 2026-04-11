package ui

import (
	"image/color"
	"os"

	"github.com/charmbracelet/colorprofile"
	"charm.land/lipgloss/v2"
)

// isDarkBackground defaults to true (dark theme). Updated at runtime via
// tea.BackgroundColorMsg for proper light/dark detection. We intentionally
// avoid calling lipgloss.HasDarkBackground() at init time because it
// queries the terminal and can hang in non-TTY environments (tests, CI, pipes).
var isDarkBackground = true

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

// resolveColor picks the dark or light hex color based on isDarkBackground
// and returns a resolved color.Color via lipgloss.Color().
func resolveColor(light, dark string) color.Color {
	if isDarkBackground {
		return lipgloss.Color(dark)
	}
	return lipgloss.Color(light)
}

type Theme struct {
	// Colors - resolved color.Color values (not adaptive)
	Primary   color.Color
	Secondary color.Color
	Subtext   color.Color

	// Accents (map to Color* globals)
	Info    color.Color
	Success color.Color
	Warning color.Color
	Danger  color.Color

	// Status
	Open       color.Color
	InProgress color.Color
	Blocked    color.Color
	Deferred   color.Color
	Pinned     color.Color
	Hooked     color.Color
	Closed     color.Color
	Tombstone  color.Color
	Review     color.Color

	// Types
	Bug     color.Color
	Feature color.Color
	Task    color.Color
	Epic    color.Color
	Chore   color.Color

	// UI Elements
	Border    color.Color
	Highlight color.Color
	Muted     color.Color

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
	PriorityUpArrow   lipgloss.Style // Priority hint up
	PriorityDownArrow lipgloss.Style // Priority hint down
	TriageStar        lipgloss.Style // Top pick star
	TriageUnblocks    lipgloss.Style // Unblocks indicator
	TriageUnblocksAlt lipgloss.Style // Secondary unblocks
}

// DefaultTheme returns the Tomorrow Night theme with colors resolved
// for the current isDarkBackground state.
func DefaultTheme() Theme {
	t := Theme{
		// Tomorrow Night palette + matcha-dark-sea teal accent
		Primary:   resolveColor("#3e999f", "#8abeb7"), // Teal
		Secondary: resolveColor("#8e908c", "#969896"), // Comment gray
		Subtext:   resolveColor("#8e908c", "#b4b7b4"), // Lighter muted

		Info:    resolveColor("#4271ae", "#81a2be"), // Blue
		Success: resolveColor("#718c00", "#b5bd68"), // Green
		Warning: resolveColor("#f5871f", "#de935f"), // Orange
		Danger:  resolveColor("#c82829", "#cc6666"), // Red

		Open:       resolveColor("#718c00", "#b5bd68"), // Green
		InProgress: resolveColor("#4271ae", "#81a2be"), // Blue
		Blocked:    resolveColor("#c82829", "#cc6666"), // Red
		Deferred:   resolveColor("#f5871f", "#de935f"), // Orange
		Pinned:     resolveColor("#4271ae", "#7aa6da"), // Blue variant
		Hooked:     resolveColor("#3e999f", "#8abeb7"), // Teal
		Closed:     resolveColor("#8e908c", "#969896"), // Gray
		Tombstone:  resolveColor("#c5c8c6", "#373b41"), // Muted
		Review:     resolveColor("#8959a8", "#b294bb"), // Purple

		Bug:     resolveColor("#c82829", "#cc6666"), // Red
		Feature: resolveColor("#f5871f", "#de935f"), // Orange
		Epic:    resolveColor("#8959a8", "#b294bb"), // Purple
		Task:    resolveColor("#eab700", "#f0c674"), // Yellow
		Chore:   resolveColor("#4271ae", "#81a2be"), // Blue

		Border:    resolveColor("#d6d6d6", "#373b41"),
		Highlight: resolveColor("#d6d6d6", "#373b41"),
		Muted:     resolveColor("#8e908c", "#969896"),
	}

	t.Base = lipgloss.NewStyle().Foreground(resolveColor("#4d4d4c", "#c5c8c6"))

	t.Selected = lipgloss.NewStyle().
		Background(t.Highlight).
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(t.Primary).
		PaddingLeft(1).
		Bold(true)

	t.Header = lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(resolveColor("#ffffff", "#1d1f21")).
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

func (t Theme) GetStatusColor(s string) color.Color {
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

func (t Theme) GetTypeIcon(typ string) (string, color.Color) {
	switch typ {
	case "bug":
		return "🐛", t.Bug
	case "feature":
		return "✨", t.Feature
	case "task":
		return "📋", t.Task
	case "epic":
		// Use rocket instead of snow-capped mountain - the mountain has a variation
		// selector (U+FE0F) that causes inconsistent width calculations across terminals
		return "🚀", t.Epic
	case "chore":
		return "🧹", t.Chore
	default:
		return "•", t.Subtext
	}
}
