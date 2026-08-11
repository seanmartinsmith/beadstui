package ui

// Tests for bt-native themes and unified theme selection (bt-ba9fc).

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// deltaE76 is the euclidean CIELAB distance between two colors. Rules of
// thumb: below 2.3 is the just-noticeable difference, around 10 reads as
// clearly different, above 20 as obviously different.
//
// Used instead of a raw hue or lightness delta because neither alone is
// honest about an earth palette: loam's rust and terracotta sit 13 degrees
// apart in hue and 4.6 apart in L*, which looks alarming measured one axis at
// a time and is comfortably separable measured properly.
func deltaE76(a, b srgb) float64 {
	lab := func(c srgb) (float64, float64, float64) {
		lin := func(v float64) float64 {
			if v <= 0.04045 {
				return v / 12.92
			}
			return math.Pow((v+0.055)/1.055, 2.4)
		}
		r, g, bl := lin(c.r), lin(c.g), lin(c.b)
		x := 0.4124564*r + 0.3575761*g + 0.1804375*bl
		y := 0.2126729*r + 0.7151522*g + 0.0721750*bl
		z := 0.0193339*r + 0.1191920*g + 0.9503041*bl
		f := func(t float64) float64 {
			if t > math.Pow(6.0/29.0, 3) {
				return math.Cbrt(t)
			}
			return t/(3*math.Pow(6.0/29.0, 2)) + 4.0/29.0
		}
		fx, fy, fz := f(x/0.95047), f(y/1.0), f(z/1.08883)
		return 116*fy - 16, 500 * (fx - fy), 200 * (fy - fz)
	}
	l1, a1, b1 := lab(a)
	l2, a2, b2 := lab(b)
	return math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2))
}

func TestNativeThemesShip(t *testing.T) {
	names := NativeThemeNames()
	for _, want := range []string{"loam", "greyscale"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("bt-native corpus is missing %q (have %v)", want, names)
		}
	}
}

// TestNativeThemesResolveCompletely mirrors the vendored corpus's contract: a
// nil token group would fall through to Tomorrow Night and mix two palettes on
// screen. Reflection rather than a hand-written list, so a token added to
// ThemeColors later cannot be silently missed by the bt-native files.
func TestNativeThemesResolveCompletely(t *testing.T) {
	rt := reflect.TypeOf(ThemeColors{})
	for _, name := range NativeThemeNames() {
		tf, err := LoadNativeTheme(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		v := reflect.ValueOf(tf.Colors)
		for i := 0; i < v.NumField(); i++ {
			if f := v.Field(i); (f.Kind() == reflect.Ptr || f.Kind() == reflect.Map) && f.IsNil() {
				t.Errorf("%s: token group %s is unset", name, rt.Field(i).Name)
			}
		}
		for label, hex := range collectHexes(t, tf.Colors) {
			if !hexRe.MatchString(hex) {
				t.Errorf("%s: %s = %q is not a valid hex color", name, label, hex)
			}
		}
	}
}

// TestThemePrecedenceAndCollision is the documented collision rule. loam and
// the bt-native greyscale are shipped, and greyscale deliberately shares a
// name with a vendored file, so this exercises a real collision rather than a
// fixture.
func TestThemePrecedenceAndCollision(t *testing.T) {
	// A name only the vendored corpus has still resolves.
	if _, err := ResolveTheme("nord"); err != nil {
		t.Errorf("vendored-only name should resolve: %v", err)
	}
	// A name only bt has resolves.
	loam, err := ResolveTheme("loam")
	if err != nil {
		t.Fatalf("loam should resolve from the bt-native dir: %v", err)
	}
	native, err := LoadNativeTheme("loam")
	if err != nil {
		t.Fatal(err)
	}
	if loam.Colors.Bg.Dark != native.Colors.Bg.Dark {
		t.Error("loam did not resolve from the bt-native dir")
	}

	// The collision: bare name takes the bt-native palette.
	bare, err := ResolveTheme("greyscale")
	if err != nil {
		t.Fatal(err)
	}
	btNative, err := ResolveTheme("bt:greyscale")
	if err != nil {
		t.Fatal(err)
	}
	vendored, err := ResolveTheme("btop:greyscale")
	if err != nil {
		t.Fatal(err)
	}
	if bare.Colors.Bg.Dark != btNative.Colors.Bg.Dark {
		t.Errorf("bare %q should resolve to the bt-native palette: got bg %s, want %s",
			"greyscale", bare.Colors.Bg.Dark, btNative.Colors.Bg.Dark)
	}
	if vendored.Colors.Bg.Dark == btNative.Colors.Bg.Dark {
		t.Error("btop:greyscale and bt:greyscale resolved identically; " +
			"the prefix escape is not reaching the vendored file")
	}
	// Both remain monochrome -- the point of the collision is tuning, not one
	// being broken.
	if !vendored.Mono || !btNative.Mono {
		t.Errorf("both greyscales should be mono: vendored=%v native=%v",
			vendored.Mono, btNative.Mono)
	}

	if _, err := ResolveTheme("bt:no-such-theme"); err == nil {
		t.Error("an unknown bt-native name should error, not fall through to btop")
	}
	if _, err := ResolveTheme("no-such-theme-anywhere"); err == nil {
		t.Error("an unknown name should error")
	}
}

// TestThemeNamesAreAllResolvable is the contract a picker depends on: every
// name listed resolves to the palette it names, shadowed vendored entries
// included.
func TestThemeNamesAreAllResolvable(t *testing.T) {
	names := ThemeNames()
	if len(names) < len(BtopThemeNames())+len(NativeThemeNames()) {
		t.Errorf("ThemeNames dropped entries: %d listed, %d btop + %d native",
			len(names), len(BtopThemeNames()), len(NativeThemeNames()))
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Errorf("duplicate name %q", n)
		}
		seen[n] = true
		if _, err := ResolveTheme(n); err != nil {
			t.Errorf("listed name %q does not resolve: %v", n, err)
		}
	}
	// greyscale is shadowed, so the vendored one must be listed in prefixed form.
	if !seen["btop:greyscale"] {
		t.Error("shadowed vendored theme should be listed as btop:greyscale")
	}
	if seen["btop:nord"] {
		t.Error("unshadowed vendored themes should be listed bare, not prefixed")
	}
}

// TestBtThemeEnvSelectsNative wires the whole path: BT_THEME through
// LoadTheme, which is how a user actually picks a palette.
func TestBtThemeEnvSelectsNative(t *testing.T) {
	loam, err := LoadNativeTheme("loam")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("BT_THEME", "loam")
	got := LoadTheme()
	if got.Colors.Bg.Dark != loam.Colors.Bg.Dark {
		t.Errorf("BT_THEME=loam gave bg %s, want %s", got.Colors.Bg.Dark, loam.Colors.Bg.Dark)
	}
	if got.Colors.Status["blocked"].Dark != loam.Colors.Status["blocked"].Dark {
		t.Error("BT_THEME=loam did not apply loam's status colors")
	}

	t.Setenv("BT_THEME", "btop:greyscale")
	if got := LoadTheme(); !got.Mono {
		t.Error("BT_THEME=btop:greyscale should carry Mono through LoadTheme")
	}
}

// TestNativeMonoThemesHaveNoChroma holds a declared mono: true palette to the
// same bar mono mode holds the adapter to.
func TestNativeMonoThemesHaveNoChroma(t *testing.T) {
	checked := 0
	for _, name := range NativeThemeNames() {
		tf, err := LoadNativeTheme(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !tf.Mono {
			continue
		}
		checked++
		for label, hex := range collectHexes(t, tf.Colors) {
			c, ok := parseBtopColor(hex)
			if !ok {
				continue
			}
			if _, s := c.hueSat(); s != 0 {
				t.Errorf("%s declares mono but %s = %s has saturation %.4f",
					name, label, hex, s)
			}
		}
	}
	if checked == 0 {
		t.Fatal("expected at least one bt-native mono theme (greyscale)")
	}
}

// TestNativeThemesAreLegible is the accessibility floor for palettes bt
// authors itself. Unlike the vendored corpus -- where a dim token is upstream's
// editorial choice and not bt's to override -- every one of these was chosen
// here, so there is no excuse for one landing below the UI threshold.
func TestNativeThemesAreLegible(t *testing.T) {
	// Chrome is meant to recede; everything else is meant to be read.
	chrome := map[string]bool{
		"Border": true, "Highlight": true, "Bg": true, "BgDark": true,
		"BgSubtle": true, "BgHighlight": true, "BgContrast": true,
	}
	for _, name := range NativeThemeNames() {
		tf, err := LoadNativeTheme(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		bg, _ := parseBtopColor(tf.Colors.Bg.Dark)
		for label, hex := range collectHexes(t, tf.Colors) {
			base, _, _ := strings.Cut(label, ".")
			base, _, _ = strings.Cut(base, "[")
			if chrome[base] || strings.HasSuffix(base, "Bg") ||
				strings.HasSuffix(label, ".light") ||
				label == "Status[tombstone].dark" {
				continue
			}
			c, ok := parseBtopColor(hex)
			if !ok {
				continue
			}
			if r := contrastRatio(c, bg); r < accentMinContrast {
				t.Errorf("%s: %s = %s has contrast %.2f against bg, below %.1f",
					name, label, hex, r, accentMinContrast)
			}
		}
	}
}

// TestNativeStatusesAreSeparable holds bt-authored palettes to a separation
// bar across the statuses that share the issues list.
//
// The bar differs by palette kind, because the honest bar does. A chromatic
// theme has two axes to separate on and is measured perceptually, in CIELAB
// deltaE -- loam is hue-poor by construction, since an earth palette keeps
// rust, terracotta and amber inside a 30 degree arc, so this is what stops
// that constraint from quietly collapsing two badges into one. A monochrome
// theme has only lightness, where deltaE degenerates to the L* delta and a
// deltaE-12 bar across seven statuses would demand 72 L* of a band that only
// affords about 60. Mono is therefore held to monoMinDeltaLstar, the same
// contract the adapter's ladder meets.
func TestNativeStatusesAreSeparable(t *testing.T) {
	const minDeltaE = 12.0
	statuses := []string{"open", "in_progress", "blocked", "review", "closed", "hooked", "deferred"}
	for _, name := range NativeThemeNames() {
		tf, err := LoadNativeTheme(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		bar, metric := minDeltaE, "deltaE"
		if tf.Mono {
			bar, metric = monoMinDeltaLstar, "L*"
		}
		for i := 0; i < len(statuses); i++ {
			for j := i + 1; j < len(statuses); j++ {
				a, ok1 := tf.Colors.Status[statuses[i]]
				b, ok2 := tf.Colors.Status[statuses[j]]
				if !ok1 || !ok2 {
					t.Errorf("%s: missing status %s or %s", name, statuses[i], statuses[j])
					continue
				}
				ca, _ := parseBtopColor(a.Dark)
				cb, _ := parseBtopColor(b.Dark)
				d := deltaE76(ca, cb)
				if tf.Mono {
					d = math.Abs(ca.lstar() - cb.lstar())
				}
				if d < bar {
					t.Errorf("%s: %s (%s) and %s (%s) differ by %s %.1f, want >= %.1f",
						name, statuses[i], a.Dark, statuses[j], b.Dark, metric, d, bar)
				}
			}
		}
	}
}

// TestNativeMonoClosedIsQuietest is the ordering invariant a one-axis palette
// has to get right: closed must be the dimmest status, or a finished bead
// renders louder than a live one.
func TestNativeMonoClosedIsQuietest(t *testing.T) {
	for _, name := range NativeThemeNames() {
		tf, err := LoadNativeTheme(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !tf.Mono {
			continue
		}
		bg, _ := parseBtopColor(tf.Colors.Bg.Dark)
		closed, _ := parseBtopColor(tf.Colors.Status["closed"].Dark)
		closedC := contrastRatio(closed, bg)
		for _, live := range []string{"open", "in_progress", "blocked", "review", "hooked", "deferred"} {
			c, _ := parseBtopColor(tf.Colors.Status[live].Dark)
			if r := contrastRatio(c, bg); r <= closedC {
				t.Errorf("%s: %s at %.2f:1 is no louder than closed at %.2f:1",
					name, live, r, closedC)
			}
		}
	}
}

// TestVendoredBtopDirStaysUpstream guards the re-vendoring boundary. The btop
// directory must remain a verbatim copy, so a bt-authored palette landing in
// it -- which would make the next re-vendor a merge instead of a copy -- is
// the failure this catches.
func TestVendoredBtopDirStaysUpstream(t *testing.T) {
	for _, name := range BtopThemeNames() {
		raw, err := LoadBtopThemeRaw(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		// Every vendored file must still be btop-shaped: bt's own vocabulary
		// has no main_bg/main_fg, so this fails if a bt YAML is dropped in.
		if _, ok := raw.Keys["main_fg"]; !ok {
			t.Errorf("%s: vendored file has no main_fg; is it still a btop theme?", name)
		}
	}
	// And the two corpora must not overlap on disk.
	for _, name := range NativeThemeNames() {
		if _, err := btopThemeFS.ReadFile(btopThemeDir + "/" + name + ".yaml"); err == nil {
			t.Errorf("bt-native theme %q has a file inside the vendored btop dir", name)
		}
	}
}
