package ui

// btop theme adapter (bt-o6xx1).
//
// bt's default palette was originally hand-copied out of btop's
// `tomorrow-night` theme plus a `matcha-dark-sea` teal accent (see the comment
// at the top of theme.go). That copy took only flat colors and discarded the
// part of a btop theme that actually carries hierarchy: every btop theme
// defines three-stop gradients (temp_start/mid/end, cpu_*, used_*, ...) that
// describe a low -> mid -> high ramp. bt has exactly such ramps to express
// (priority P4..P0, staleness, graph heat) and was expressing them with four
// unrelated colors at identical saturation, which is why nothing read as
// hierarchy.
//
// This file ingests the upstream btop corpus (vendored under themes/btop) and
// maps it onto bt's existing token set, so bt ships the whole collection
// instead of one flattened palette. The vendored .theme files are the upstream
// originals rather than pre-converted bt YAML: keeping the mapping as code
// means a mapping fix does not require regenerating 41 files, and the rules
// below stay reviewable and testable in one place.
//
// Hex literals in this file are permitted for the same reason as theme.go /
// theme_loader.go / styles.go: they are polarity anchors for color math
// (pure black / pure white), not palette values.

import (
	"embed"
	"fmt"
	"io/fs"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed themes/btop/*.theme
var btopThemeFS embed.FS

const btopThemeDir = "themes/btop"

// btopAssignRe matches a btop theme assignment: theme[key]="value".
// Upstream files are shell-ish key/value with # comments; only this form is
// significant.
var btopAssignRe = regexp.MustCompile(`^\s*theme\[([a-z_]+)\]\s*=\s*"([^"]*)"`)

// BtopTheme is a parsed btop .theme file: its name plus the raw key -> value
// map exactly as written upstream. Values are kept as strings because btop
// permits three encodings (see parseBtopColor) and an intentionally empty
// value is meaningful.
type BtopTheme struct {
	Name string
	Keys map[string]string
}

// srgb is an sRGB triple with components in [0,1]. Sufficient for the mixing
// and hue comparisons this adapter needs; no color-appearance model is
// warranted for picking accent hues out of a 20-color palette.
type srgb struct{ r, g, b float64 }

// parseBtopColor decodes the three encodings btop documents in every theme
// header: "#RRGGBB" hex, "#BW" two-character greyscale, and "R G B" decimal
// triples in 0-255. Returns ok=false for an empty or unparseable value; an
// empty value is legal upstream and means "use the terminal default".
func parseBtopColor(s string) (srgb, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return srgb{}, false
	}

	if strings.HasPrefix(s, "#") {
		h := s[1:]
		switch len(h) {
		case 6:
			v, err := strconv.ParseUint(h, 16, 32)
			if err != nil {
				return srgb{}, false
			}
			return srgb{
				r: float64((v>>16)&0xff) / 255,
				g: float64((v>>8)&0xff) / 255,
				b: float64(v&0xff) / 255,
			}, true
		case 2:
			// "#BW": a two-character greyscale shorthand.
			v, err := strconv.ParseUint(h, 16, 32)
			if err != nil {
				return srgb{}, false
			}
			g := float64(v) / 255
			return srgb{r: g, g: g, b: g}, true
		default:
			return srgb{}, false
		}
	}

	// "R G B" decimal triple.
	fields := strings.Fields(s)
	if len(fields) != 3 {
		return srgb{}, false
	}
	var out [3]float64
	for i, f := range fields {
		v, err := strconv.Atoi(f)
		if err != nil || v < 0 || v > 255 {
			return srgb{}, false
		}
		out[i] = float64(v) / 255
	}
	return srgb{r: out[0], g: out[1], b: out[2]}, true
}

func (c srgb) hex() string {
	clamp := func(v float64) int {
		i := int(math.Round(v * 255))
		if i < 0 {
			return 0
		}
		if i > 255 {
			return 255
		}
		return i
	}
	return fmt.Sprintf("#%02x%02x%02x", clamp(c.r), clamp(c.g), clamp(c.b))
}

// luminance is the WCAG relative luminance, used for polarity detection and
// contrast ratios.
func (c srgb) luminance() float64 {
	lin := func(v float64) float64 {
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.r) + 0.7152*lin(c.g) + 0.0722*lin(c.b)
}

// contrastRatio returns the WCAG contrast ratio between two colors (1.0..21.0).
func contrastRatio(a, b srgb) float64 {
	la, lb := a.luminance(), b.luminance()
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// hueSat returns the HSV hue in degrees and saturation in [0,1]. Achromatic
// colors report saturation 0 and are excluded from accent picking.
func (c srgb) hueSat() (float64, float64) {
	maxV := math.Max(c.r, math.Max(c.g, c.b))
	minV := math.Min(c.r, math.Min(c.g, c.b))
	d := maxV - minV
	if d < 1e-9 || maxV < 1e-9 {
		return 0, 0
	}
	var h float64
	switch maxV {
	case c.r:
		h = math.Mod((c.g-c.b)/d, 6)
	case c.g:
		h = (c.b-c.r)/d + 2
	default:
		h = (c.r-c.g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, d / maxV
}

// hueDist is the shortest angular distance between two hues, in degrees.
func hueDist(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > 180 {
		d = 360 - d
	}
	return d
}

// mixColor linearly interpolates between two colors; t=0 yields a, t=1 yields b.
func mixColor(a, b srgb, t float64) srgb {
	return srgb{
		r: a.r + (b.r-a.r)*t,
		g: a.g + (b.g-a.g)*t,
		b: a.b + (b.b-a.b)*t,
	}
}

// hsvToRGB converts hue in degrees plus saturation and value in [0,1].
func hsvToRGB(h, s, v float64) srgb {
	h = math.Mod(math.Mod(h, 360)+360, 360)
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return srgb{r: r + m, g: g + m, b: b + m}
}

// mixRamp interpolates a gradient stop by rotating hue along the shorter arc
// rather than blending channels.
//
// Straight RGB interpolation between distant hues passes through grey: in
// adwaita-dark, whose temp ramp runs blue -> red, the midpoint landed on a
// muddy #7e467e at 2.5:1 contrast. Rotating the hue keeps saturation and value
// up, so every step of the priority ramp stays as legible as its endpoints.
// Achromatic endpoints fall back to channel mixing, which is correct for them.
func mixRamp(a, b srgb, t float64) srgb {
	ha, sa := a.hueSat()
	hb, sb := b.hueSat()
	if sa < 0.1 || sb < 0.1 {
		return mixColor(a, b, t)
	}
	// Rotate along the shorter arc.
	d := hb - ha
	if d > 180 {
		d -= 360
	} else if d < -180 {
		d += 360
	}
	va := math.Max(a.r, math.Max(a.g, a.b))
	vb := math.Max(b.r, math.Max(b.g, b.b))
	return hsvToRGB(ha+d*t, sa+(sb-sa)*t, va+(vb-va)*t)
}

var (
	srgbBlack = srgb{0, 0, 0}
	srgbWhite = srgb{1, 1, 1}
)

// ParseBtopTheme parses the contents of a btop .theme file. Unknown keys are
// retained; comments and blank lines are ignored. Parsing never fails -- a
// line that is not an assignment simply contributes nothing.
func ParseBtopTheme(name, content string) *BtopTheme {
	t := &BtopTheme{Name: name, Keys: make(map[string]string)}
	for _, line := range strings.Split(content, "\n") {
		if m := btopAssignRe.FindStringSubmatch(line); m != nil {
			t.Keys[m[1]] = m[2]
		}
	}
	return t
}

// BtopThemeNames returns the vendored theme names, sorted. These are the
// values accepted by LoadBtopTheme.
func BtopThemeNames() []string {
	entries, err := fs.ReadDir(btopThemeFS, btopThemeDir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if n := strings.TrimSuffix(e.Name(), ".theme"); n != e.Name() {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

// LoadBtopThemeRaw reads and parses a vendored btop theme by name.
func LoadBtopThemeRaw(name string) (*BtopTheme, error) {
	data, err := btopThemeFS.ReadFile(btopThemeDir + "/" + name + ".theme")
	if err != nil {
		return nil, fmt.Errorf("read btop theme %q: %w", name, err)
	}
	return ParseBtopTheme(name, string(data)), nil
}

// color returns the first key that parses to a color, in preference order.
func (b *BtopTheme) color(keys ...string) (srgb, bool) {
	for _, k := range keys {
		if c, ok := parseBtopColor(b.Keys[k]); ok {
			return c, true
		}
	}
	return srgb{}, false
}

// ramp resolves a btop three-stop gradient. Only the endpoints are reliable
// across the corpus: the `*_mid` stop is absent or empty in up to 9 of the 41
// upstream themes and `*_end` in up to 2, so both degrade rather than
// defaulting to black. A missing mid is interpolated, never assumed.
func (b *BtopTheme) ramp(prefix string) (start, mid, end srgb, ok bool) {
	start, ok = b.color(prefix + "_start")
	if !ok {
		return srgb{}, srgb{}, srgb{}, false
	}
	end, hasEnd := b.color(prefix + "_end")
	if !hasEnd {
		end = start
	}
	mid, hasMid := b.color(prefix + "_mid")
	if !hasMid {
		mid = mixRamp(start, end, 0.5)
	}
	return start, mid, end, true
}

// accentMinContrast is the floor for colors this adapter *derives* (info,
// review, and the type colors built from them). Upstream's own semantic picks
// -- danger from temp_end, success from temp_start, primary from hi_fg -- are
// passed through untouched even when they sit below this, because those are
// the theme author's editorial choices. The floor applies only where bt is
// making the choice, and 3.0 is the WCAG AA threshold for UI components.
const accentMinContrast = 3.0

// accentPool collects the theme's own chromatic colors, deduplicated, for
// accent derivation. Only colors saturated enough to read as a hue and legible
// enough against the background are eligible.
func (b *BtopTheme) accentPool(bg srgb) []srgb {
	seen := make(map[string]bool)
	var pool []srgb
	// Sorted for determinism: map iteration order must not change the palette.
	keys := make([]string, 0, len(b.Keys))
	for k := range b.Keys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		c, ok := parseBtopColor(b.Keys[k])
		if !ok {
			continue
		}
		_, sat := c.hueSat()
		if sat < 0.20 || contrastRatio(c, bg) < accentMinContrast {
			continue
		}
		h := c.hex()
		if seen[h] {
			continue
		}
		seen[h] = true
		pool = append(pool, c)
	}
	return pool
}

// pickDistinctHue returns the pool color whose hue is farthest from every
// already-taken hue, and whether one was available at all.
//
// This is a greedy farthest-point choice. It reports ok=false rather than
// inventing a color because most btop themes carry only four or five distinct
// chromatic hues -- tokyo-night's whole pool is red, orange, green and cyan --
// which is fewer than bt has roles to fill. Callers decide what a genuinely
// exhausted palette should degrade to; silently returning a duplicate would
// make two roles indistinguishable without anyone noticing.
func pickDistinctHue(pool []srgb, taken []srgb) (srgb, bool) {
	isTaken := func(c srgb) bool {
		for _, t := range taken {
			if t.hex() == c.hex() {
				return true
			}
		}
		return false
	}
	var best srgb
	bestScore := -1.0
	for _, c := range pool {
		if isTaken(c) {
			continue
		}
		ch, _ := c.hueSat()
		score := 360.0
		for _, t := range taken {
			th, _ := t.hueSat()
			if d := hueDist(ch, th); d < score {
				score = d
			}
		}
		if score > bestScore {
			bestScore = score
			best = c
		}
	}
	return best, bestScore >= 0
}

// distinctVariant returns a lightness variant of base that duplicates nothing
// in taken. It is the fallback for themes with no spare hue: adwaita-dark, for
// instance, carries three chromatic colors total and its hi_fg *is* the cool
// end of its own temp ramp, so deriving in-progress by hue is impossible and
// separating it by lightness is the only honest option left.
func distinctVariant(base srgb, taken []srgb, toward, away srgb) srgb {
	duplicates := func(c srgb) bool {
		for _, k := range taken {
			if k.hex() == c.hex() {
				return true
			}
		}
		return false
	}
	for _, amount := range []float64{0.35, 0.55, 0.22, 0.7} {
		for _, dir := range []srgb{toward, away} {
			if c := mixColor(base, dir, amount); !duplicates(c) {
				return c
			}
		}
	}
	return mixColor(base, toward, 0.5)
}

// mixToContrast blends fg toward bg until it hits the target contrast ratio,
// returning fg unchanged when fg is already at or below the target.
//
// A fixed blend fraction does not work here: contrast is non-linear in the
// blend, so the same 45% step lands at 5.7:1 in tomorrow-night and 2.2:1 in
// solarized_light. Targeting the ratio directly gives every theme the same
// perceived hierarchy regardless of how much headroom it has.
func mixToContrast(fg, bg srgb, target float64) srgb {
	if contrastRatio(fg, bg) <= target {
		return fg
	}
	lo, hi := 0.0, 1.0
	// Contrast decreases monotonically as t goes 0 -> 1, so bisection is safe.
	for i := 0; i < 24; i++ {
		mid := (lo + hi) / 2
		if contrastRatio(mixColor(fg, bg, mid), bg) > target {
			lo = mid
		} else {
			hi = mid
		}
	}
	return mixColor(fg, bg, (lo+hi)/2)
}

// ThemeFile converts a parsed btop theme into bt's ThemeFile.
//
// Both sides of every AdaptiveHex are filled with the same value. A btop theme
// is a single palette, not a light/dark pair, and bt resolves AdaptiveHex
// against terminal-detected polarity; filling both sides means an explicitly
// chosen theme renders as chosen regardless of what the terminal reports,
// which is the least surprising reading of "I picked gruvbox_light".
func (b *BtopTheme) ThemeFile() *ThemeFile {
	// --- polarity -------------------------------------------------------
	// main_bg is intentionally empty in 3 upstream themes (btop's
	// transparent-background convention). Fall back to selected_bg, which is
	// non-empty in all 41 and always sits within a step of the real
	// background, then push it away from the foreground so it still reads as
	// a backdrop rather than as a selection band.
	mainFg, _ := b.color("main_fg")
	bg, hasBg := b.color("main_bg")
	if !hasBg {
		if sel, ok := b.color("selected_bg"); ok {
			if mainFg.luminance() > sel.luminance() {
				bg = mixColor(sel, srgbBlack, 0.4)
			} else {
				bg = mixColor(sel, srgbWhite, 0.4)
			}
		} else {
			bg = srgbBlack
		}
	}
	isDark := bg.luminance() < 0.5
	// away is the direction "further from the text", used to deepen
	// backgrounds; toward is the direction "closer to the text".
	away, toward := srgbBlack, srgbWhite
	if !isDark {
		away, toward = srgbWhite, srgbBlack
	}

	// --- structure ------------------------------------------------------
	title, _ := b.color("title", "main_fg")
	hiFg, _ := b.color("hi_fg", "title", "main_fg")
	selBg, _ := b.color("selected_bg")
	inactive, _ := b.color("inactive_fg")

	// --- semantic ramp --------------------------------------------------
	// temp_* is the one gradient with both endpoints defined in all 41
	// themes, and its semantics (cool -> hot) match bt's priority and
	// severity ramps exactly.
	rampLow, rampMid, rampHigh, hasRamp := b.ramp("temp")
	if !hasRamp {
		rampLow, rampMid, rampHigh, _ = b.ramp("cpu")
	}
	// P1 sits between medium and critical rather than reusing either, so all
	// four priority steps stay distinguishable.
	rampHighMid := mixRamp(rampMid, rampHigh, 0.5)

	// --- neutral ramp ---------------------------------------------------
	// Computed from the theme's own text and background rather than taken
	// from proc_misc. btop's proc_misc is not reliably a dim tone: in
	// tokyo-night it is #7dcfff, a full-strength cyan, which would render
	// closed and de-emphasized rows brighter than live ones. Stepping
	// text -> bg by contrast target guarantees a monotonically receding ramp
	// in every theme, which is the value hierarchy the flat palette lacked.
	textC := contrastRatio(mainFg, bg)
	subtext := mixToContrast(mainFg, bg, math.Max(textC*0.72, math.Min(4.5, textC)))
	muted := mixToContrast(mainFg, bg, math.Max(textC*0.45, math.Min(3.2, textC)))
	// Keep upstream's inactive_fg for chrome when it is genuinely more
	// subordinate than muted; otherwise synthesize a dimmer step so borders
	// never out-shout body text.
	chrome := inactive
	if contrastRatio(chrome, bg) >= contrastRatio(muted, bg) {
		chrome = mixToContrast(mainFg, bg, math.Max(textC*0.18, 1.3))
	}

	// --- derived accents ------------------------------------------------
	// The temp ramp is upstream's own severity gradient and is never
	// substituted. Only in-progress genuinely must be separable from it: in a
	// dense list, confusing in-progress with blocked or open is the costly
	// mistake, so it takes the hue farthest from all three ramp stops.
	pool := b.accentPool(bg)
	rampStops := []srgb{rampHigh, rampLow, rampMid}
	info, ok := pickDistinctHue(pool, rampStops)
	if !ok {
		info = distinctVariant(hiFg, rampStops, toward, away)
	}
	// primary is chrome -- the selection caret, focused borders, the selected
	// title -- rather than a status, so it is allowed to share a hue with one.
	// Substituting it would stop themes looking like themselves, which is the
	// entire point of shipping the corpus.
	primary := hiFg
	// review has no upstream counterpart. Prefer a spare hue; when the theme
	// has none left, step the in-progress hue toward the foreground extreme so
	// it still reads as a distinct state instead of silently duplicating one.
	reviewTaken := append(append([]srgb{}, rampStops...), info)
	review, ok := pickDistinctHue(pool, reviewTaken)
	if !ok {
		review = distinctVariant(info, reviewTaken, toward, away)
	}

	ah := func(c srgb) *AdaptiveHex {
		h := c.hex()
		return &AdaptiveHex{Dark: h, Light: h}
	}
	// tint blends an accent most of the way into the background to produce the
	// subtle status/priority band backgrounds bt's schema expects.
	tint := func(c srgb) *AdaptiveHex { return ah(mixColor(bg, c, 0.18)) }

	return &ThemeFile{Colors: ThemeColors{
		Bg:          ah(bg),
		BgDark:      ah(mixColor(bg, away, 0.12)),
		BgSubtle:    ah(selBg),
		BgHighlight: ah(inactive),
		Text:        ah(mainFg),
		Subtext:     ah(subtext),
		Muted:       ah(muted),

		Primary:   ah(primary),
		Secondary: ah(muted),
		Info:      ah(info),
		Success:   ah(rampLow),
		Warning:   ah(rampMid),
		Danger:    ah(rampHigh),

		// Row titles are the densest reading surface in the TUI, so they take
		// the theme's title color pushed slightly toward the foreground
		// extreme for a little more separation from body text.
		TextSecondary: ah(mixColor(title, toward, 0.15)),
		BgContrast:    ah(bg),

		Status: map[string]*AdaptiveHex{
			"open":        ah(rampLow),
			"in_progress": ah(info),
			"blocked":     ah(rampHigh),
			"deferred":    ah(rampMid),
			"pinned":      ah(mixColor(info, toward, 0.2)),
			"hooked":      ah(primary),
			"review":      ah(review),
			"closed":      ah(muted),
			"tombstone":   ah(chrome),
		},
		StatusBg: map[string]*AdaptiveHex{
			"open":        tint(rampLow),
			"in_progress": tint(info),
			"blocked":     tint(rampHigh),
			"deferred":    tint(rampMid),
			"pinned":      tint(mixColor(info, toward, 0.2)),
			"hooked":      tint(primary),
			"review":      tint(review),
			"closed":      tint(muted),
			"tombstone":   ah(bg),
		},

		Priority: map[string]*AdaptiveHex{
			"critical": ah(rampHigh),
			"high":     ah(rampHighMid),
			"medium":   ah(rampMid),
			"low":      ah(rampLow),
		},
		PriorityBg: map[string]*AdaptiveHex{
			"critical": tint(rampHigh),
			"high":     tint(rampHighMid),
			"medium":   tint(rampMid),
			"low":      tint(rampLow),
		},

		Type: map[string]*AdaptiveHex{
			"bug":     ah(rampHigh),
			"feature": ah(rampMid),
			"task":    ah(primary),
			"epic":    ah(review),
			"chore":   ah(info),
		},

		Border:    ah(chrome),
		Highlight: ah(selBg),
	}}
}

// LoadBtopTheme resolves a vendored btop theme by name into a bt ThemeFile.
func LoadBtopTheme(name string) (*ThemeFile, error) {
	raw, err := LoadBtopThemeRaw(name)
	if err != nil {
		return nil, err
	}
	return raw.ThemeFile(), nil
}
