package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// setGlyphs swaps the package-level activeGlyphs for the duration of a test and
// restores it via t.Cleanup. Rendering reads activeGlyphs at call time, so a
// swap before View()/Render() exercises the chosen tier.
func setGlyphs(t *testing.T, g GlyphSet) {
	t.Helper()
	prev := activeGlyphs
	activeGlyphs = g
	t.Cleanup(func() { activeGlyphs = prev })
}

// isEmojiRune reports whether r is in a pictographic/emoji range that breaks TUI
// layout by rendering double-width. This is the mechanical acceptance gate for
// bt-5f3bo. It deliberately EXCLUDES non-emoji marks that the nerdfont tier is
// allowed to use: geometric shapes (U+25xx), box drawing (U+25xx/U+2500s),
// braille (U+28xx), arrows (U+2190..U+21FF), the return symbol U+23CE, the
// middle dot U+00B7, bullet U+2022, ellipsis U+2026, and PUA (U+E000..U+F8FF).
func isEmojiRune(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FFFF: // supplementary pictographs / emoji
		return true
	case r >= 0x2600 && r <= 0x27BF: // Misc Symbols + Dingbats
		return true
	case r >= 0x2B00 && r <= 0x2BFF: // Misc Symbols and Arrows (⬆ ⭐ …)
		return true
	case r == 0xFE0F: // emoji variation selector
		return true
	case r == 0x2139: // ℹ information source (renders emoji with VS16)
		return true
	case r == 0x231A || r == 0x231B: // watch / hourglass
		return true
	case r >= 0x23E9 && r <= 0x23F3: // media/clock/timer emoji (⏰ ⏱ ⏳ …)
		return true
	case r >= 0x23F8 && r <= 0x23FA: // pause/stop/record emoji
		return true
	}
	return false
}

// TestNoEmojiInChrome is the acceptance gate: no emoji codepoint may remain in
// any non-test Go source under pkg/ui. Test files are excluded because fixtures
// legitimately embed emoji as width/render DATA (CJK, ballot-x, etc.).
func TestNoEmojiInChrome(t *testing.T) {
	root := "." // pkg/ui
	var hits []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		line := 1
		for _, r := range string(b) {
			if r == '\n' {
				line++
				continue
			}
			if isEmojiRune(r) {
				hits = append(hits, filepath.ToSlash(path)+": line "+strconv.Itoa(line)+" contains emoji U+"+hexUpper(r))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk pkg/ui: %v", err)
	}
	if len(hits) > 0 {
		t.Fatalf("emoji literals must not remain in pkg/ui chrome (route through activeGlyphs):\n%s",
			strings.Join(hits, "\n"))
	}
}

// TestResolveGlyphs verifies tier selection: default and unknown/empty resolve
// to nerdfont; only "ascii" (case/space-insensitive) selects the ASCII tier.
func TestResolveGlyphs(t *testing.T) {
	cases := []struct {
		env  string
		want GlyphSet
	}{
		{"", nerdfontGlyphs},
		{"nerdfont", nerdfontGlyphs},
		{"garbage", nerdfontGlyphs},
		{"ascii", asciiGlyphs},
		{"ASCII", asciiGlyphs},
		{"  ascii  ", asciiGlyphs},
	}
	for _, c := range cases {
		got := resolveGlyphs(c.env)
		if got.StOpen != c.want.StOpen || got.Bell != c.want.Bell {
			t.Errorf("resolveGlyphs(%q): got tier StOpen=%q Bell=%q, want StOpen=%q Bell=%q",
				c.env, got.StOpen, got.Bell, c.want.StOpen, c.want.Bell)
		}
	}
}

// TestAsciiTierIsPureASCII asserts every glyph in the ascii tier is < 0x80. This
// enforces the "BT_GLYPHS=ascii yields pure ASCII" acceptance criterion at the
// table level. It reflects over every string (and []string) field so new fields
// are covered automatically.
func TestAsciiTierIsPureASCII(t *testing.T) {
	v := reflect.ValueOf(asciiGlyphs)
	typ := v.Type()
	checkStr := func(field, s string) {
		for _, r := range s {
			if r >= 0x80 {
				t.Errorf("asciiGlyphs.%s = %q contains non-ASCII rune U+%s", field, s, hexUpper(r))
			}
		}
	}
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		name := typ.Field(i).Name
		switch f.Kind() {
		case reflect.String:
			checkStr(name, f.String())
		case reflect.Slice:
			for j := 0; j < f.Len(); j++ {
				checkStr(name, f.Index(j).String())
			}
		}
	}
}

// TestNerdfontTierIsSingleWidth guards the layout invariant that no nerdfont
// glyph is double-width (the whole reason emoji were deleted). PUA icons and the
// geometric/braille marks are all single cell.
func TestNerdfontTierIsSingleWidth(t *testing.T) {
	v := reflect.ValueOf(nerdfontGlyphs)
	typ := v.Type()
	check := func(field, s string) {
		if s == "" {
			return
		}
		if w := ansi.StringWidth(s); w > 1 {
			t.Errorf("nerdfontGlyphs.%s = %q has display width %d (must be single-width)", field, s, w)
		}
	}
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		name := typ.Field(i).Name
		switch f.Kind() {
		case reflect.String:
			check(name, f.String())
		case reflect.Slice:
			for j := 0; j < f.Len(); j++ {
				check(name, f.Index(j).String())
			}
		}
	}
}

// footerTierFixture is a representative FooterData with every chrome slot
// populated via the glyph table, so the tier tests exercise real footer glyphs
// rather than hard-coded literals. Hints use ASCII keys (the keys subpackage is
// out of scope for this bead; see glyphs.go notes).
func footerTierFixture(width int) FooterData {
	return FooterData{
		Width:         width,
		FilterText:    "OPEN",
		FilterIcon:    activeGlyphs.FilterOpen,
		HintText:      "l:labels",
		TotalItems:    169,
		CountOpen:     120,
		CountReady:    14,
		CountBlocked:  9,
		CountClosed:   26,
		SearchMode:    "fuzzy",
		SortLabel:     "updated",
		BellCount:     4,
		AlertCount:    3,
		CriticalCount: 1,
		WarningCount:  2,
		Hints: []FooterHint{
			{Key: "j/k", Desc: "nav"}, {Key: "o", Desc: "issues"}, {Key: "?", Desc: "help"},
		},
	}
}

// TestFooterSingleRowBothTiers renders the footer at the design widths in both
// glyph tiers and asserts it stays a single row within the terminal width (no
// wrap, no mid-token overflow).
func TestFooterSingleRowBothTiers(t *testing.T) {
	widths := []int{50, 70, 100, 130, 160}
	for _, tier := range []struct {
		name string
		g    GlyphSet
	}{{"nerdfont", nerdfontGlyphs}, {"ascii", asciiGlyphs}} {
		t.Run(tier.name, func(t *testing.T) {
			setGlyphs(t, tier.g)
			for _, w := range widths {
				fd := footerTierFixture(w)
				out := fd.Render()
				if strings.Contains(out, "\n") {
					t.Errorf("width %d tier %s: footer wrapped to >1 row:\n%q", w, tier.name, out)
				}
				if got := ansi.StringWidth(out); got > w {
					t.Errorf("width %d tier %s: footer display width %d exceeds terminal width", w, tier.name, got)
				}
			}
		})
	}
}

// TestFooterAsciiTierIsPureASCII asserts the rendered footer contains no
// non-ASCII rune when BT_GLYPHS=ascii, at every design width — the acceptance
// criterion applied to the footer, the fully-chrome design centerpiece.
func TestFooterAsciiTierIsPureASCII(t *testing.T) {
	setGlyphs(t, asciiGlyphs)
	for _, w := range []int{50, 70, 100, 130, 160} {
		fd := footerTierFixture(w)
		out := ansi.Strip(fd.Render())
		for _, r := range out {
			if r >= 0x80 {
				t.Errorf("width %d: ascii-tier footer contains non-ASCII rune U+%s in:\n%q", w, hexUpper(r), out)
				break
			}
		}
	}
}

// TestRenderHarnessBothTiers drives the real Model through the design widths in
// both tiers and asserts the layout invariant: the view is exactly `height`
// rows and no row exceeds the terminal width (the footer never wraps or steals a
// content line). Mirrors the render harness scenarios (bt-5f3bo Testing).
func TestRenderHarnessBothTiers(t *testing.T) {
	widths := []int{50, 70, 100, 130, 160}
	const height = 24
	for _, tier := range []struct {
		name string
		g    GlyphSet
	}{{"nerdfont", nerdfontGlyphs}, {"ascii", asciiGlyphs}} {
		t.Run(tier.name, func(t *testing.T) {
			setGlyphs(t, tier.g)
			for _, w := range widths {
				issues := harnessIssues()
				m := NewModel(issues, nil, "", nil, nil)
				nm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: height})
				m = nm.(Model)
				plain := ansi.Strip(m.View().Content)
				lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
				if len(lines) > height {
					t.Errorf("width %d tier %s: view has %d rows > height %d (footer or content wrapped)",
						w, tier.name, len(lines), height)
				}
				for i, ln := range lines {
					if lw := ansi.StringWidth(ln); lw > w {
						t.Errorf("width %d tier %s: row %d width %d exceeds terminal width %d: %q",
							w, tier.name, i, lw, w, ln)
					}
				}
			}
		})
	}
}

// --- tiny local helpers (avoid strconv import churn in this test file) ---

func hexUpper(r rune) string {
	const digits = "0123456789ABCDEF"
	if r == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for r > 0 {
		i--
		b[i] = digits[r&0xF]
		r >>= 4
	}
	return string(b[i:])
}
