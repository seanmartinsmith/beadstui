package ui

// Tests for the btop theme adapter (bt-o6xx1).
//
// The structural assertions here are the real contract: every vendored theme
// must resolve to a COMPLETE bt token set with no unset field and no token
// silently defaulted to black by a missing upstream key. Contrast is reported
// rather than asserted -- a low-contrast upstream theme is upstream's editorial
// choice, not a bug in this adapter, and failing the build on it would mean
// deleting themes the user deliberately asked to ship.

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var hexRe = regexp.MustCompile(`^#[0-9a-f]{6}$`)

func TestParseBtopColor(t *testing.T) {
	tests := []struct {
		in     string
		wantOK bool
		want   string
	}{
		{"#1d1f21", true, "#1d1f21"},
		{"#C34043", true, "#c34043"},
		{"#ff", true, "#ffffff"}, // 2-char greyscale shorthand
		{"#00", true, "#000000"}, // 2-char greyscale shorthand
		{"255 255 255", true, "#ffffff"},
		{"0 128 255", true, "#0080ff"},
		{"", false, ""}, // legal upstream: transparent
		{"   ", false, ""},
		{"#12345", false, ""}, // wrong length
		{"#zzzzzz", false, ""},
		{"300 0 0", false, ""}, // out of range
		{"1 2", false, ""},     // wrong arity
	}
	for _, tc := range tests {
		got, ok := parseBtopColor(tc.in)
		if ok != tc.wantOK {
			t.Errorf("parseBtopColor(%q) ok = %v, want %v", tc.in, ok, tc.wantOK)
			continue
		}
		if ok && got.hex() != tc.want {
			t.Errorf("parseBtopColor(%q) = %s, want %s", tc.in, got.hex(), tc.want)
		}
	}
}

func TestBtopThemeCorpusPresent(t *testing.T) {
	names := BtopThemeNames()
	if len(names) < 40 {
		t.Fatalf("expected the full vendored btop corpus, got %d themes", len(names))
	}
	// tomorrow-night is bt's historical palette source; matcha-dark-sea is the
	// accent source. Both must survive vendoring or the default look changes.
	for _, want := range []string{"tomorrow-night", "matcha-dark-sea"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("vendored corpus is missing %q", want)
		}
	}
}

// collectHexes walks a ThemeColors via reflection and returns every resolved
// token as label -> hex. Reflection is used deliberately: a hand-written list
// would silently skip any token added to ThemeColors later, which is exactly
// the failure this test exists to catch.
func collectHexes(t *testing.T, c ThemeColors) map[string]string {
	t.Helper()
	out := map[string]string{}
	v := reflect.ValueOf(c)
	rt := v.Type()
	for i := 0; i < v.NumField(); i++ {
		name := rt.Field(i).Name
		f := v.Field(i)
		switch f.Kind() {
		case reflect.Ptr:
			if f.IsNil() {
				continue
			}
			ah := f.Interface().(*AdaptiveHex)
			out[name+".dark"] = ah.Dark
			out[name+".light"] = ah.Light
		case reflect.Map:
			for _, k := range f.MapKeys() {
				mv := f.MapIndex(k)
				if mv.IsNil() {
					continue
				}
				ah := mv.Interface().(*AdaptiveHex)
				out[fmt.Sprintf("%s[%s].dark", name, k.String())] = ah.Dark
				out[fmt.Sprintf("%s[%s].light", name, k.String())] = ah.Light
			}
		}
	}
	return out
}

func TestBtopThemesResolveCompletely(t *testing.T) {
	// Every field of ThemeColors must be populated by the adapter. Anything
	// left nil would silently fall through to Tomorrow Night, mixing two
	// palettes on screen.
	wantFields := map[string]bool{}
	rt := reflect.TypeOf(ThemeColors{})
	for i := 0; i < rt.NumField(); i++ {
		wantFields[rt.Field(i).Name] = true
	}

	for _, name := range BtopThemeNames() {
		tf, err := LoadBtopTheme(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}

		v := reflect.ValueOf(tf.Colors)
		for i := 0; i < v.NumField(); i++ {
			fname := rt.Field(i).Name
			f := v.Field(i)
			if (f.Kind() == reflect.Ptr || f.Kind() == reflect.Map) && f.IsNil() {
				t.Errorf("%s: token group %s is unset", name, fname)
			}
		}

		for label, hex := range collectHexes(t, tf.Colors) {
			if !hexRe.MatchString(hex) {
				t.Errorf("%s: %s = %q is not a valid hex color", name, label, hex)
			}
		}
	}
}

// TestBtopNoAccidentalBlack guards the specific failure mode this adapter is
// designed around: a missing upstream key silently resolving to the zero value
// of srgb, which is pure black. Foreground tokens are never legitimately
// #000000 in a dark theme, so any that appear indicate an unguarded lookup.
func TestBtopNoAccidentalBlack(t *testing.T) {
	fgTokens := []string{
		"Text.dark", "Subtext.dark", "Muted.dark", "Primary.dark",
		"Secondary.dark", "Info.dark", "Success.dark", "Warning.dark",
		"Danger.dark", "TextSecondary.dark",
	}
	for _, name := range BtopThemeNames() {
		tf, err := LoadBtopTheme(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		hexes := collectHexes(t, tf.Colors)
		bg, _ := parseBtopColor(hexes["Bg.dark"])
		if bg.luminance() >= 0.5 {
			continue // light theme: black foregrounds are legitimate
		}
		for _, tok := range fgTokens {
			if hexes[tok] == "#000000" {
				t.Errorf("%s: %s resolved to pure black on a dark background "+
					"(missing upstream key not guarded)", name, tok)
			}
		}
	}
}

// TestBtopSeverityIsSemanticNotPositional is the reason the adapter stopped
// reading btop gradients positionally.
//
// matcha-dark-sea's temp ramp runs purple -> rose -> green, so its "hot" end is
// green. Taking severity from the endpoints rendered blocked in green and open
// in purple. Severity is matched by hue instead, because red-means-blocked is
// bt's semantics rather than the theme author's.
func TestBtopSeverityIsSemanticNotPositional(t *testing.T) {
	raw, err := LoadBtopThemeRaw("matcha-dark-sea")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(raw.Keys["temp_end"]); got != "#33b165" {
		t.Skipf("matcha-dark-sea temp_end is now %q upstream; fixture no longer valid", got)
	}

	tf := raw.ThemeFile()
	blocked, _ := parseBtopColor(tf.Colors.Status["blocked"].Dark)
	open, _ := parseBtopColor(tf.Colors.Status["open"].Dark)

	bh, _ := blocked.hueSat()
	oh, _ := open.hueSat()
	// blocked must be warm, not the theme's green temp_end.
	if hueDist(bh, hueRed) > 60 {
		t.Errorf("blocked resolved to hue %.0f (%s), which is not a warm/alarm hue",
			bh, blocked.hex())
	}
	// open must be green-ish, not the theme's purple temp_start.
	if hueDist(oh, hueGreen) > 60 {
		t.Errorf("open resolved to hue %.0f (%s), which is not a green hue",
			oh, open.hex())
	}
}

// TestBtopSynthesizesMissingAlarmHue covers themes with no warm color at all:
// kyli0x's whole temp ramp is teal and gotham's hot end is white. Neither can
// express "blocked" from its own palette, and rendering blocked in teal is
// worse than a synthesized red carrying the theme's own intensity.
func TestBtopSynthesizesMissingAlarmHue(t *testing.T) {
	for _, name := range []string{"kyli0x", "gotham"} {
		tf, err := LoadBtopTheme(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		blocked, _ := parseBtopColor(tf.Colors.Status["blocked"].Dark)
		open, _ := parseBtopColor(tf.Colors.Status["open"].Dark)
		bh, bs := blocked.hueSat()
		if hueDist(bh, hueRed) > 60 {
			t.Errorf("%s: blocked resolved to hue %.0f (%s), not a warm hue",
				name, bh, blocked.hex())
		}
		if bs < 0.3 {
			t.Errorf("%s: synthesized alarm color %s is too washed out to alarm (sat %.2f)",
				name, blocked.hex(), bs)
		}
		if blocked.hex() == open.hex() {
			t.Errorf("%s: blocked and open collapsed to %s", name, blocked.hex())
		}
	}
}

// TestBtopEmptyMainBgIsNotBlack covers btop's transparent-background
// convention. These three themes ship main_bg="" deliberately; resolving that
// to black would render e.g. flat-remix on a background it was never designed
// against.
func TestBtopEmptyMainBgIsNotBlack(t *testing.T) {
	for _, name := range []string{"adapta", "flat-remix", "matcha-dark-sea"} {
		raw, err := LoadBtopThemeRaw(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := strings.TrimSpace(raw.Keys["main_bg"]); got != "" {
			t.Skipf("%s now defines main_bg=%q upstream; fixture no longer valid", name, got)
		}
		tf := raw.ThemeFile()
		if tf.Colors.Bg.Dark == "#000000" {
			t.Errorf("%s: empty main_bg resolved to pure black instead of a "+
				"theme-derived backdrop", name)
		}
		// It must still be a plausible backdrop: clearly separated from the
		// body text.
		bg, _ := parseBtopColor(tf.Colors.Bg.Dark)
		fg, _ := parseBtopColor(tf.Colors.Text.Dark)
		if r := contrastRatio(fg, bg); r < 3.0 {
			t.Errorf("%s: derived backdrop has contrast %.2f against text, want >= 3.0", name, r)
		}
	}
}

// TestBtopAccentsAreDistinct checks the derived accents actually separate.
// in_progress must not be confusable with blocked, or the list loses its most
// important distinction at a glance.
func TestBtopAccentsAreDistinct(t *testing.T) {
	for _, name := range BtopThemeNames() {
		tf, err := LoadBtopTheme(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		info, _ := parseBtopColor(tf.Colors.Info.Dark)
		danger, _ := parseBtopColor(tf.Colors.Danger.Dark)
		if info.hex() == danger.hex() {
			t.Errorf("%s: in-progress and blocked resolved to the same color %s",
				name, info.hex())
		}
	}
}

// TestBtopNeutralRampRecedes is the value-hierarchy contract. text, subtext,
// muted and border must step monotonically away from the foreground, so
// de-emphasized rows always read as quieter than live ones.
//
// Regression guard: taking Muted straight from btop's proc_misc broke this in
// tokyo-night, where proc_misc is #7dcfff -- a full-strength cyan that made
// closed rows the brightest thing on screen.
func TestBtopNeutralRampRecedes(t *testing.T) {
	for _, name := range BtopThemeNames() {
		tf, err := LoadBtopTheme(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		h := collectHexes(t, tf.Colors)
		bg, _ := parseBtopColor(h["Bg.dark"])
		get := func(tok string) float64 {
			c, _ := parseBtopColor(h[tok])
			return contrastRatio(c, bg)
		}
		text, subtext, muted, border := get("Text.dark"), get("Subtext.dark"), get("Muted.dark"), get("Border.dark")
		if !(text >= subtext && subtext >= muted && muted >= border) {
			t.Errorf("%s: neutral ramp does not recede: text %.1f, subtext %.1f, muted %.1f, border %.1f",
				name, text, subtext, muted, border)
		}
		// Muted must stay legible even while receding. A theme whose own body
		// text sits below the UI floor cannot get its muted tone above it, so
		// the floor is capped at what the theme actually affords.
		floor := 3.0
		if text < floor {
			floor = text
		}
		if muted < floor-0.05 {
			t.Errorf("%s: muted contrast %.1f is below the %.1f floor this theme affords",
				name, muted, floor)
		}
	}
}

// TestBtopStatusesAreSeparable checks the distinctions that actually cost the
// user something when they collapse.
//
// Scoped deliberately. btop themes carry only four or five distinct chromatic
// hues (tokyo-night's entire pool is red, orange, green, cyan) while bt has
// more roles than that, so demanding every pair differ would be a demand the
// corpus cannot meet. What must hold is that the three statuses sharing the
// issues list -- open, in_progress, blocked -- never render the same, plus
// review not silently duplicating in_progress.
//
// primary is excluded on purpose: it is chrome (caret, focused border) rather
// than a status, and forcing it away from the theme's own hi_fg would stop
// themes looking like themselves.
func TestBtopStatusesAreSeparable(t *testing.T) {
	for _, name := range BtopThemeNames() {
		tf, err := LoadBtopTheme(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		h := collectHexes(t, tf.Colors)

		raw, err := LoadBtopThemeRaw(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		bg, _ := parseBtopColor(h["Bg.dark"])
		// A theme with no chromatic range genuinely cannot separate statuses
		// by hue. greyscale is the honest example; it is not a defect.
		if len(raw.accentPool(bg)) < 3 {
			continue
		}

		pairs := []struct{ a, b string }{
			{"Status[open].dark", "Status[in_progress].dark"},
			{"Status[open].dark", "Status[blocked].dark"},
			{"Status[in_progress].dark", "Status[blocked].dark"},
			{"Status[in_progress].dark", "Status[review].dark"},
		}
		for _, p := range pairs {
			if h[p.a] == h[p.b] {
				t.Errorf("%s: %s and %s both resolved to %s", name, p.a, p.b, h[p.a])
			}
		}
	}
}

// TestBtopPriorityRampIsMonotonic is the hierarchy contract for the issues
// list: P0..P3 must form an actual gradient rather than four unrelated colors
// at the same weight, which was the original complaint about the flattened
// Tomorrow Night palette.
func TestBtopPriorityRampIsMonotonic(t *testing.T) {
	for _, name := range BtopThemeNames() {
		tf, err := LoadBtopTheme(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		h := collectHexes(t, tf.Colors)
		crit := h["Priority[critical].dark"]
		high := h["Priority[high].dark"]
		med := h["Priority[medium].dark"]
		low := h["Priority[low].dark"]

		raw, err := LoadBtopThemeRaw(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		// When upstream's ramp endpoints coincide the theme has no gradient to
		// offer and every step legitimately matches.
		if strings.TrimSpace(raw.Keys["temp_start"]) == strings.TrimSpace(raw.Keys["temp_end"]) {
			continue
		}
		if crit == low {
			t.Errorf("%s: P0 and P3 resolved to the same color %s", name, crit)
		}

		// Interpolated steps must not be muddier than the endpoints they sit
		// between. Regression guard: blending adwaita-dark's blue mid toward
		// its red end in RGB passed through grey and produced a #7e467e P1 at
		// 2.5:1, dimmer than either neighbour.
		bg, _ := parseBtopColor(h["Bg.dark"])
		ratio := func(hex string) float64 {
			c, _ := parseBtopColor(hex)
			return contrastRatio(c, bg)
		}
		floor := math.Min(ratio(crit), ratio(med)) * 0.8
		if got := ratio(high); got < floor {
			t.Errorf("%s: P1 contrast %.1f is muddier than its neighbours "+
				"(P0 %.1f, P2 %.1f)", name, got, ratio(crit), ratio(med))
		}
		// The interpolated middle steps must land between the endpoints, not
		// on top of one of them.
		if high == crit && med == crit {
			t.Errorf("%s: priority ramp collapsed onto critical (%s)", name, crit)
		}
	}
}

// TestBtopDeterministic guards against map-iteration order leaking into the
// palette: the accent pool is built from a Go map and must be sorted first.
func TestBtopDeterministic(t *testing.T) {
	for _, name := range BtopThemeNames() {
		first, err := LoadBtopTheme(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := collectHexes(t, first.Colors)
		for i := 0; i < 8; i++ {
			again, err := LoadBtopTheme(name)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			got := collectHexes(t, again.Colors)
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("%s: palette is not deterministic across loads", name)
			}
		}
	}
}

func TestSelectedThemeName(t *testing.T) {
	base := &ThemeFile{Theme: "matcha-dark-sea"}
	user := &ThemeFile{Theme: "nord"}
	proj := &ThemeFile{Theme: "gruvbox_dark"}

	t.Setenv("BT_THEME", "")
	if got := selectedThemeName(base, user, proj); got != "gruvbox_dark" {
		t.Errorf("project file should outrank user file, got %q", got)
	}
	if got := selectedThemeName(base, user, nil); got != "nord" {
		t.Errorf("user file should apply when no project file, got %q", got)
	}
	if got := selectedThemeName(base, nil, nil); got != "matcha-dark-sea" {
		t.Errorf("embedded default should apply when nothing else selects, got %q", got)
	}
	if got := selectedThemeName(nil, nil, nil); got != "" {
		t.Errorf("no source should select nothing, got %q", got)
	}
	// Whitespace-only is not a selection.
	if got := selectedThemeName(nil, &ThemeFile{Theme: "  "}, nil); got != "" {
		t.Errorf("blank theme should select nothing, got %q", got)
	}

	t.Setenv("BT_THEME", "tokyo-night")
	if got := selectedThemeName(base, user, proj); got != "tokyo-night" {
		t.Errorf("env should outrank every file, got %q", got)
	}
}

// TestBtopUnknownThemeIsNotFatal covers the typo path: a misspelled cosmetic
// setting must degrade to the default palette, never take the TUI down.
func TestBtopUnknownThemeIsNotFatal(t *testing.T) {
	if _, err := LoadBtopTheme("no-such-theme"); err == nil {
		t.Fatal("expected an error for an unknown theme name")
	}
	t.Setenv("BT_THEME", "no-such-theme")
	tf := LoadTheme()
	if tf == nil || tf.Colors.Text == nil {
		t.Fatal("LoadTheme must still return a usable theme for a bad name")
	}
}

// TestBtopThemeIsOverridable proves the layering: a named palette supplies the
// base, and a hand-written colors: block still wins over it, so choosing a
// theme never silently discards the tweaks someone already wrote.
func TestBtopThemeIsOverridable(t *testing.T) {
	base := loadEmbeddedTheme()
	btopTF, err := LoadBtopTheme("nord")
	if err != nil {
		t.Fatal(err)
	}
	mergeTheme(base, btopTF)
	nordText := base.Colors.Text.Dark
	if nordText == "" {
		t.Fatal("nord did not supply a text color")
	}

	overlay := &ThemeFile{Colors: ThemeColors{Text: &AdaptiveHex{Dark: "#ff00ff", Light: "#ff00ff"}}}
	mergeTheme(base, overlay)
	if base.Colors.Text.Dark != "#ff00ff" {
		t.Errorf("overlay should win over the named theme, got %s", base.Colors.Text.Dark)
	}
	// An unrelated token must still come from the named theme.
	if base.Colors.Danger.Dark != btopTF.Colors.Danger.Dark {
		t.Errorf("untouched token should stay from the named theme: got %s want %s",
			base.Colors.Danger.Dark, btopTF.Colors.Danger.Dark)
	}
}

// TestBtopSwatchDump is not a regression test -- it has no assertions. Gated
// behind BT_RENDER_DUMP like the render harness, it writes an ANSI swatch
// sheet per theme to _tmp/render/themes/ for visual review (pipe the .ansi
// into charmbracelet/freeze for a PNG), plus a contrast report naming every
// token pair that falls below WCAG AA.
func TestBtopSwatchDump(t *testing.T) {
	if os.Getenv("BT_RENDER_DUMP") == "" {
		t.Skip("set BT_RENDER_DUMP=1 to write theme swatches")
	}
	dir := filepath.Join("..", "..", "_tmp", "render", "themes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Ordered so the sheet reads as a hierarchy check, not an alphabet.
	rows := []struct{ label, token string }{
		{"text", "Text.dark"},
		{"subtext", "Subtext.dark"},
		{"muted", "Muted.dark"},
		{"border", "Border.dark"},
		{"primary", "Primary.dark"},
		{"info", "Info.dark"},
		{"success", "Success.dark"},
		{"warning", "Warning.dark"},
		{"danger", "Danger.dark"},
		{"P0 critical", "Priority[critical].dark"},
		{"P1 high", "Priority[high].dark"},
		{"P2 medium", "Priority[medium].dark"},
		{"P3 low", "Priority[low].dark"},
		{"open", "Status[open].dark"},
		{"in_progress", "Status[in_progress].dark"},
		{"blocked", "Status[blocked].dark"},
		{"review", "Status[review].dark"},
		{"closed", "Status[closed].dark"},
	}

	var report strings.Builder
	report.WriteString("Contrast of each token against its theme background (WCAG).\n")
	report.WriteString("AA body text needs 4.5; AA large/UI needs 3.0.\n\n")

	for _, name := range BtopThemeNames() {
		tf, err := LoadBtopTheme(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		hexes := collectHexes(t, tf.Colors)
		bg, _ := parseBtopColor(hexes["Bg.dark"])
		br, bgc, bb := int(bg.r*255), int(bg.g*255), int(bg.b*255)

		var sheet strings.Builder
		var low []string
		sheet.WriteString(fmt.Sprintf("\x1b[48;2;%d;%d;%dm\x1b[38;2;255;255;255m  %-28s  \x1b[0m\n",
			br, bgc, bb, name))
		for _, r := range rows {
			c, ok := parseBtopColor(hexes[r.token])
			if !ok {
				continue
			}
			cr, cg, cb := int(c.r*255), int(c.g*255), int(c.b*255)
			ratio := contrastRatio(c, bg)
			sheet.WriteString(fmt.Sprintf(
				"\x1b[48;2;%d;%d;%dm\x1b[38;2;%d;%d;%dm  %-12s %s  %4.1f:1  \x1b[0m\n",
				br, bgc, bb, cr, cg, cb, r.label, c.hex(), ratio))
			// border is chrome and is SUPPOSED to recede; holding it to a
			// text threshold would flag every well-designed theme in the
			// corpus, so it is excluded rather than reported as a defect.
			if ratio < 4.5 && r.token != "Border.dark" {
				low = append(low, fmt.Sprintf("%s %.1f", r.label, ratio))
			}
		}

		if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(sheet.String()), 0o644); err != nil {
			t.Fatalf("write swatch: %v", err)
		}
		status := "ok"
		if len(low) > 0 {
			status = "below 4.5: " + strings.Join(low, ", ")
		}
		report.WriteString(fmt.Sprintf("%-28s %s\n", name, status))
	}

	if err := os.WriteFile(filepath.Join(dir, "contrast-report.txt"), []byte(report.String()), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	t.Logf("wrote %d swatch sheets + contrast-report.txt to %s", len(BtopThemeNames()), dir)
}
