package ui

import (
	"bytes"
	"image/color"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/exp/teatest/v2"
)

func TestDefaultTheme(t *testing.T) {
	theme := DefaultTheme()

	// Check a few known colors are set (not nil)
	if theme.Primary == nil {
		t.Error("DefaultTheme Primary color is nil")
	}
	if theme.Open == nil {
		t.Error("DefaultTheme Open color is nil")
	}
}

func TestGetStatusColor(t *testing.T) {
	theme := DefaultTheme()

	tests := []struct {
		status string
		want   color.Color
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
	theme := DefaultTheme()

	tests := []struct {
		typ      string
		wantIcon string
		wantCol  color.Color
	}{
		{"bug", activeGlyphs.TypeBug, theme.Bug},
		{"feature", activeGlyphs.TypeFeature, theme.Feature},
		{"task", activeGlyphs.TypeTask, theme.Task},
		{"epic", activeGlyphs.TypeEpic, theme.Epic},
		{"chore", activeGlyphs.TypeChore, theme.Chore},
		{"unknown", activeGlyphs.Bullet, theme.Subtext},
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

// -- Color profile detection tests (bd-2rih) --

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

// -- Live theme swap (bt-1n0b1) --

// restoreThemeGlobals puts the mutable theme globals back after a test that
// repaints them. Color*, PanelStyle/FocusedPanelStyle and isDarkBackground are
// package-level and shared with every other test in this package, so leaving a
// foreign palette applied would silently repaint later golden tests -- the
// mutable-global coupling bt-zq6z identified.
//
// The restore reproduces NewModel's own startup sequence rather than snapshotting
// all ~60 vars, which lands on the same well-defined state NewModel leaves.
func restoreThemeGlobals(t *testing.T) {
	t.Helper()
	savedDark := isDarkBackground
	t.Cleanup(func() {
		isDarkBackground = savedDark
		resolveColors()
		ApplyThemeToGlobals(LoadTheme())
	})
}

// TestThemeSwapMidSession_NoRace repaints the entire palette from inside Update,
// mid-session, while the real Bubble Tea event loop is running and async cmds
// are in flight. It is the empirical half of bt-1n0b1.
//
// Why this is the right shape: Bubble Tea calls Update and View sequentially on
// one goroutine (vendor/charm.land/bubbletea/v2/tea.go:853 and :869 are
// consecutive statements in eventLoop), so a swap performed in Update cannot
// race the renderer by construction. What CAN race it is a tea.Cmd, because
// every cmd runs on its own goroutine (tea.go:702-714), or a worker goroutine.
// Those are what this test puts under the race detector.
//
// The swap path exercised here (tea.BackgroundColorMsg, model.go:1892) is the
// one already shipping, and it is the same sequence a theme picker keypress
// would run: resolveColors -> DefaultTheme -> LoadTheme -> ApplyThemeToGlobals
// -> ApplyThemeToThemeStruct.
//
// Run with: go test ./pkg/ui -race -run TestThemeSwapMidSession
func TestThemeSwapMidSession_NoRace(t *testing.T) {
	restoreThemeGlobals(t)

	// Two vendored btop palettes. They must resolve to different primaries,
	// otherwise "the swap took effect" would not be observable and this test
	// would pass vacuously.
	const themeA, themeB = "dracula", "ayu"

	isDarkBackground = true
	primaryOf := func(name string) color.Color {
		t.Setenv("BT_THEME", name)
		ApplyThemeToGlobals(LoadTheme())
		return ColorPrimary
	}
	wantA, wantB := primaryOf(themeA), primaryOf(themeB)
	if wantA == wantB {
		t.Fatalf("fixture palettes %q and %q both resolve primary to %v; pick two that differ",
			themeA, themeB, wantA)
	}

	t.Setenv("BT_THEME", themeA)
	tm := teatest.NewTestModel(t, NewModel(claimTestIssues(), nil, "", nil, nil),
		teatest.WithInitialTermSize(120, 32))

	// Swap only once the session is genuinely running, so the repaints land
	// among live cmds rather than during startup.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("zz-"))
	}, teatest.WithDuration(8*time.Second))

	// Storm phase: alternate palettes while driving input, so View runs and new
	// cmds spawn between repaints. BT_THEME is read by LoadTheme on each swap,
	// so every iteration rewrites all ~60 Color* vars to a different palette.
	// No value assertions here -- tm.Send is async, so which palette is live at
	// any instant is deliberately nondeterministic. The race detector is the
	// assertion.
	for i := range 10 {
		name := themeA
		if i%2 == 1 {
			name = themeB
		}
		t.Setenv("BT_THEME", name)
		tm.Send(tea.BackgroundColorMsg{Color: lipgloss.Color("#1d1f21")})
		tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
		tm.Send(tea.BackgroundColorMsg{Color: lipgloss.Color("#ffffff")})
		tm.Send(tea.KeyPressMsg{Code: tea.KeyUp})
	}

	tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(15*time.Second))

	// Settle phase, on the test goroutine once the loop is done, so the
	// assertions are deterministic. Reading the globals here is itself part of
	// the check: a leaked cmd goroutine still touching them would race this.
	t.Setenv("BT_THEME", themeB)
	isDarkBackground = true
	resolveColors()
	ApplyThemeToGlobals(LoadTheme())

	if ColorPrimary != wantB {
		t.Errorf("after swap to %q, ColorPrimary = %v, want %v", themeB, ColorPrimary, wantB)
	}
	// The package-level styles derived from the tokens must be rebuilt by the
	// swap, not snapshotted at init -- a stale style here is a correctness bug
	// distinct from any race.
	if got := FocusedPanelStyle.GetBorderTopForeground(); got != ColorPrimary {
		t.Errorf("FocusedPanelStyle border = %v, want ColorPrimary %v (style not rebuilt on swap)", got, ColorPrimary)
	}
	if got := PanelStyle.GetBorderTopForeground(); got != ColorBgHighlight {
		t.Errorf("PanelStyle border = %v, want ColorBgHighlight %v (style not rebuilt on swap)", got, ColorBgHighlight)
	}
}
