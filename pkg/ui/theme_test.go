package ui

import (
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
)

func TestDefaultTheme(t *testing.T) {
	renderer := lipgloss.NewRenderer(nil)
	theme := DefaultTheme(renderer)

	if theme.Renderer != renderer {
		t.Error("DefaultTheme renderer mismatch")
	}
	// Check a few known colors are set (not zero value)
	if isColorEmpty(theme.Primary) {
		t.Error("DefaultTheme Primary color is empty")
	}
	if isColorEmpty(theme.Open) {
		t.Error("DefaultTheme Open color is empty")
	}
}

func isColorEmpty(c lipgloss.AdaptiveColor) bool {
	return c.Light == "" && c.Dark == ""
}

func TestGetStatusColor(t *testing.T) {
	renderer := lipgloss.NewRenderer(nil)
	theme := DefaultTheme(renderer)

	tests := []struct {
		status string
		want   lipgloss.AdaptiveColor
	}{
		{"open", theme.Open},
		{"in_progress", theme.InProgress},
		{"blocked", theme.Blocked},
		{"closed", theme.Closed},
		{"unknown", theme.Subtext},
		{"", theme.Subtext},
	}

	for _, tt := range tests {
		got := theme.GetStatusColor(tt.status)
		if got != tt.want {
			t.Errorf("GetStatusColor(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestGetTypeIcon(t *testing.T) {
	renderer := lipgloss.NewRenderer(nil)
	theme := DefaultTheme(renderer)

	tests := []struct {
		typ      string
		wantIcon string
		wantCol  lipgloss.AdaptiveColor
	}{
		{"bug", "🐛", theme.Bug},
		{"feature", "✨", theme.Feature},
		{"task", "📋", theme.Task},
		{"epic", "🚀", theme.Epic}, // Changed from 🏔️ - variation selector caused width issues
		{"chore", "🧹", theme.Chore},
		{"unknown", "•", theme.Subtext},
	}

	for _, tt := range tests {
		icon, col := theme.GetTypeIcon(tt.typ)
		if icon != tt.wantIcon {
			t.Errorf("GetTypeIcon(%q) icon = %q, want %q", tt.typ, icon, tt.wantIcon)
		}
		if col != tt.wantCol {
			t.Errorf("GetTypeIcon(%q) color = %v, want %v", tt.typ, col, tt.wantCol)
		}
	}
}

// ── Color profile detection tests (bd-2rih) ─────────────────────────────

func TestColorProfile_Detection(t *testing.T) {
	// TermProfile is set at init(); just verify it's a valid value
	valid := map[colorprofile.Profile]bool{
		colorprofile.Unknown:   true,
		colorprofile.NoTTY:     true,
		colorprofile.ASCII:     true,
		colorprofile.ANSI:      true,
		colorprofile.ANSI256:   true,
		colorprofile.TrueColor: true,
	}
	if !valid[TermProfile] {
		t.Errorf("TermProfile has unexpected value: %d", TermProfile)
	}
}

func TestThemeBg_TrueColor(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.TrueColor

	got := ThemeBg("#282A36")
	if _, ok := got.(lipgloss.NoColor); ok {
		t.Error("ThemeBg should return hex color in TrueColor mode, got NoColor")
	}
}

func TestThemeBg_ANSI(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.ANSI

	got := ThemeBg("#282A36")
	if _, ok := got.(lipgloss.NoColor); !ok {
		t.Errorf("ThemeBg should return NoColor in ANSI mode, got %T", got)
	}
}

func TestThemeBg_ANSI256(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.ANSI256

	got := ThemeBg("#282A36")
	if _, ok := got.(lipgloss.NoColor); !ok {
		t.Errorf("ThemeBg should return NoColor in ANSI256 mode (only TrueColor gets hex bg), got %T", got)
	}
}

func TestThemeFg_TrueColor(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.TrueColor

	got := ThemeFg("#FF6B6B")
	if _, ok := got.(lipgloss.ANSIColor); ok {
		t.Error("ThemeFg should return hex color in TrueColor mode, got ANSIColor")
	}
}

func TestThemeFg_ANSI256(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.ANSI256

	got := ThemeFg("#FF6B6B")
	if _, ok := got.(lipgloss.ANSIColor); ok {
		t.Error("ThemeFg should return hex color in ANSI256 mode, got ANSIColor")
	}
}

func TestThemeFg_ANSI(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.ANSI

	got := ThemeFg("#FF6B6B")
	ansiColor, ok := got.(lipgloss.ANSIColor)
	if !ok {
		t.Errorf("ThemeFg should return ANSIColor in ANSI mode, got %T", got)
	} else if ansiColor != 7 {
		t.Errorf("ThemeFg should return ANSI white (7) in ANSI mode, got %d", ansiColor)
	}
}

func TestThemeFg_NoTTY(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.NoTTY

	got := ThemeFg("#FF6B6B")
	if _, ok := got.(lipgloss.ANSIColor); !ok {
		t.Errorf("ThemeFg should return ANSIColor in NoTTY mode, got %T", got)
	}
}
