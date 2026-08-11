package ui

// Why hex literals appear in this file (and theme.go / styles.go):
//
// pkg/ui/defaults/theme.yaml is the source-of-truth for color tokens at
// runtime. The Go default tables below (globalColorDefaults and
// themeColorDefaults) intentionally MIRROR that YAML so the loader can
// fall back per-key when a user's overlay omits a value, and so the app
// still renders themed correctly when the embedded YAML cannot be loaded
// (embed failures, future build configurations that strip embeds, tests
// that bypass the loader).
//
// If you change a default color, change it in BOTH the YAML AND the
// Go-fallback tables. Do not dedupe these by removing the Go side
// without first proving the embedded YAML is loadable in every supported
// build configuration. See bt-pxbc audit (docs/audits/architecture/
// 2026-05-03-theme-system.md) for the full layer hierarchy.

import (
	"embed"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"gopkg.in/yaml.v3"
)

//go:embed defaults/theme.yaml
var defaultThemeFS embed.FS

// AdaptiveHex holds dark/light hex color strings from YAML.
type AdaptiveHex struct {
	Dark  string `yaml:"dark"`
	Light string `yaml:"light"`
}

// toColor resolves this AdaptiveHex to a single color.Color based on
// the current dark/light mode. Missing values fall back to the provided
// light/dark defaults.
func (a AdaptiveHex) toColor(fallbackLight, fallbackDark string) color.Color {
	light := fallbackLight
	dark := fallbackDark
	if a.Light != "" {
		light = a.Light
	}
	if a.Dark != "" {
		dark = a.Dark
	}
	return resolveColor(light, dark)
}

// ThemeColors is the YAML-serializable color config.
type ThemeColors struct {
	// Base
	Bg          *AdaptiveHex `yaml:"bg"`
	BgDark      *AdaptiveHex `yaml:"bg_dark"`
	BgSubtle    *AdaptiveHex `yaml:"bg_subtle"`
	BgHighlight *AdaptiveHex `yaml:"bg_highlight"`
	Text        *AdaptiveHex `yaml:"text"`
	Subtext     *AdaptiveHex `yaml:"subtext"`
	Muted       *AdaptiveHex `yaml:"muted"`

	// Accents
	Primary   *AdaptiveHex `yaml:"primary"`
	Secondary *AdaptiveHex `yaml:"secondary"`
	Info      *AdaptiveHex `yaml:"info"`
	Success   *AdaptiveHex `yaml:"success"`
	Warning   *AdaptiveHex `yaml:"warning"`
	Danger    *AdaptiveHex `yaml:"danger"`

	// New tokens
	TextSecondary *AdaptiveHex `yaml:"text_secondary"`
	BgContrast    *AdaptiveHex `yaml:"bg_contrast"`

	// Status
	Status   map[string]*AdaptiveHex `yaml:"status"`
	StatusBg map[string]*AdaptiveHex `yaml:"status_bg"`

	// Priority
	Priority   map[string]*AdaptiveHex `yaml:"priority"`
	PriorityBg map[string]*AdaptiveHex `yaml:"priority_bg"`

	// Text attributes (bt-sk00v).
	//
	// Status is a categorical variable, but a color ramp is a quantitative
	// channel. Encoding one on the other is why a monochrome palette reads as
	// seven shades of the same thing: the states differ by degree when they
	// need to differ in kind. Hue hides the error because hue is itself
	// categorical; strip it and the mistake surfaces.
	//
	// Attributes are the categorical channel a palette with no hue still has,
	// and the one monochrome terminals used before color existed. Values are
	// TextAttr names; an absent or empty entry renders plain, so every existing
	// theme is unaffected until it opts in.
	StatusAttr   map[string]string `yaml:"status_attr"`
	PriorityAttr map[string]string `yaml:"priority_attr"`

	// Type
	Type map[string]*AdaptiveHex `yaml:"type"`

	// UI chrome
	Border    *AdaptiveHex `yaml:"border"`
	Highlight *AdaptiveHex `yaml:"highlight"`
}

// ThemeFile is the top-level YAML structure.
type ThemeFile struct {
	// Theme names a vendored btop palette to use as the base (bt-o6xx1).
	// Empty keeps bt's own embedded default. Any colors: block in the same
	// file still overrides the named theme, so a user can pick a palette and
	// tweak two tokens without restating the rest.
	Theme string `yaml:"theme"`
	// Mono marks a palette that carries no chroma at all. The btop adapter
	// detects it (bt-zelhm); a bt-native theme declares it. When set, bt never
	// synthesizes a hue the palette does not already have, and separates every
	// role by lightness instead.
	Mono   bool        `yaml:"mono"`
	Colors ThemeColors `yaml:"colors"`
}

// colorDefaults holds the light/dark hex defaults for a color token.
// Used by the loader to know what fallback values to use when a YAML
// override only specifies one side.
type colorDefaults struct {
	light, dark string
}

// globalColorDefaults maps from the color variable pointer to its
// light/dark defaults. Populated by resolveColorsWithDefaults.
var globalColorDefaults map[*color.Color]colorDefaults

func init() {
	// Build the defaults map so the theme loader can use fallbacks
	globalColorDefaults = map[*color.Color]colorDefaults{
		&ColorBg:          {"#ffffff", "#1d1f21"},
		&ColorBgDark:      {"#f0f0f0", "#191b1d"},
		&ColorBgSubtle:    {"#efefef", "#282a2e"},
		&ColorBgHighlight: {"#d6d6d6", "#373b41"},
		&ColorText:        {"#4d4d4c", "#c5c8c6"},
		&ColorSubtext:     {"#8e908c", "#b4b7b4"},
		&ColorMuted:       {"#8e908c", "#969896"},

		&ColorPrimary:   {"#3e999f", "#8abeb7"},
		&ColorSecondary: {"#8e908c", "#969896"},
		&ColorInfo:      {"#4271ae", "#81a2be"},
		&ColorSuccess:   {"#718c00", "#b5bd68"},
		&ColorWarning:   {"#f5871f", "#de935f"},
		&ColorDanger:    {"#c82829", "#cc6666"},

		&ColorTextSecondary: {"#333333", "#e8e8e8"},
		&ColorBgContrast:    {"#ffffff", "#1d1f21"},

		&ColorStatusOpen:       {"#718c00", "#b5bd68"},
		&ColorStatusInProgress: {"#4271ae", "#81a2be"},
		&ColorStatusBlocked:    {"#c82829", "#cc6666"},
		&ColorStatusDeferred:   {"#f5871f", "#de935f"},
		&ColorStatusPinned:     {"#4271ae", "#7aa6da"},
		&ColorStatusHooked:     {"#3e999f", "#8abeb7"},
		&ColorStatusReview:     {"#8959a8", "#b294bb"},
		&ColorStatusClosed:     {"#8e908c", "#969896"},
		&ColorStatusTombstone:  {"#c5c8c6", "#373b41"},

		&ColorStatusOpenBg:       {"#e8f0e0", "#252e1e"},
		&ColorStatusInProgressBg: {"#dce8f0", "#1e2530"},
		&ColorStatusBlockedBg:    {"#f0dce0", "#2e1e1e"},
		&ColorStatusDeferredBg:   {"#f0e4d8", "#2e251e"},
		&ColorStatusPinnedBg:     {"#dce4f0", "#1e2230"},
		&ColorStatusHookedBg:     {"#dce8e8", "#1e2a2a"},
		&ColorStatusReviewBg:     {"#e4dce8", "#261e2e"},
		&ColorStatusClosedBg:     {"#e0e0e0", "#252527"},
		&ColorStatusTombstoneBg:  {"#d6d6d6", "#1d1f21"},

		&ColorPrioCritical:   {"#c82829", "#cc6666"},
		&ColorPrioHigh:       {"#f5871f", "#de935f"},
		&ColorPrioMedium:     {"#eab700", "#f0c674"},
		&ColorPrioLow:        {"#718c00", "#b5bd68"},
		&ColorPrioCriticalBg: {"#f0dce0", "#2e1e1e"},
		&ColorPrioHighBg:     {"#f0e4d8", "#2e251e"},
		&ColorPrioMediumBg:   {"#f0ecd8", "#2e2e1e"},
		&ColorPrioLowBg:      {"#e8f0e0", "#252e1e"},

		&ColorTypeBug:     {"#c82829", "#cc6666"},
		&ColorTypeFeature: {"#f5871f", "#de935f"},
		&ColorTypeTask:    {"#eab700", "#f0c674"},
		&ColorTypeEpic:    {"#8959a8", "#b294bb"},
		&ColorTypeChore:   {"#4271ae", "#81a2be"},

		&ColorBorder:    {"#d6d6d6", "#373b41"},
		&ColorHighlight: {"#d6d6d6", "#373b41"},
	}
}

// LoadTheme loads the theme by merging layers: embedded defaults, user config,
// project config. Each layer only overrides what it specifies.
// Call ApplyThemeToGlobals after to update the Color* package vars.
func LoadTheme() *ThemeFile { return loadThemeWith("") }

// LoadThemeNamed resolves the full theme stack with an explicit palette name,
// bypassing only the BT_THEME/file selection chain (bt-54c3).
//
// The picker previews through this rather than through ResolveTheme directly,
// because ResolveTheme returns the palette alone. A user with per-token tweaks
// in ~/.config/bt/theme.yaml would then see a preview that discards them and a
// different UI once the choice was committed. Every other layer stays, so what
// the picker shows is what the user gets.
func LoadThemeNamed(name string) *ThemeFile { return loadThemeWith(name) }

func loadThemeWith(override string) *ThemeFile {
	// Layer 1: embedded defaults
	base := loadEmbeddedTheme()

	// Same helper the save path uses (bt-4ibsq). Deriving this path
	// independently in each place is how bt ends up writing a theme to one
	// file and reading it back from another.
	var user *ThemeFile
	if path, err := userThemePath(); err == nil {
		user = loadThemeFile(path)
	}
	proj := loadThemeFile(filepath.Join(".bt", "theme.yaml"))

	// Layer 2: a named palette, if one is selected -- bt-native or vendored
	// btop, resolved by ResolveTheme (bt-o6xx1, bt-ba9fc). This sits UNDER the
	// hand-written overlays so picking a theme never discards the per-token
	// tweaks a user already wrote; the name is read from the same files, most
	// specific first, with the env var winning so a palette can be tried for
	// one run without editing anything.
	name := override
	if name == "" {
		name = selectedThemeName(base, user, proj)
	}
	if name != "" {
		if named, err := ResolveTheme(name); err == nil {
			mergeTheme(base, named)
		}
		// A bad name falls through to the default palette rather than
		// failing startup: a typo in a cosmetic setting must not cost the
		// user their TUI.
	}

	// Layer 3: user-level override (~/.config/bt/theme.yaml)
	if user != nil {
		mergeTheme(base, user)
	}

	// Layer 4: project-level override (.bt/theme.yaml)
	if proj != nil {
		mergeTheme(base, proj)
	}

	return base
}

// SelectedThemeName reports the palette name bt would resolve right now,
// reading the same sources in the same order as LoadTheme (bt-54c3).
//
// The settings screen opens on this so it shows what is actually rendering
// rather than the first name in the corpus. Empty means no palette is selected
// and the embedded colors: block is in force.
func SelectedThemeName() string {
	base := loadEmbeddedTheme()
	// Same helper the save path uses (bt-4ibsq). Deriving this path
	// independently in each place is how bt ends up writing a theme to one
	// file and reading it back from another.
	var user *ThemeFile
	if path, err := userThemePath(); err == nil {
		user = loadThemeFile(path)
	}
	proj := loadThemeFile(filepath.Join(".bt", "theme.yaml"))
	return selectedThemeName(base, user, proj)
}

// selectedThemeName resolves which vendored btop palette to use, most specific
// source first: BT_THEME, then the project file, then the user file, then the
// embedded default. Including the embedded default last is what lets bt ship a
// named palette rather than a hand-maintained copy of one.
func selectedThemeName(base, user, proj *ThemeFile) string {
	for _, candidate := range []string{
		os.Getenv("BT_THEME"),
		themeNameOf(proj),
		themeNameOf(user),
		themeNameOf(base),
	} {
		if n := strings.TrimSpace(candidate); n != "" {
			return n
		}
	}
	return ""
}

func themeNameOf(tf *ThemeFile) string {
	if tf == nil {
		return ""
	}
	return tf.Theme
}

// getDefaults returns the light/dark fallback for a color pointer, or
// empty strings if unknown.
func getDefaults(target *color.Color) (string, string) {
	if d, ok := globalColorDefaults[target]; ok {
		return d.light, d.dark
	}
	return "", ""
}

// applyIf resolves an AdaptiveHex into the target color.Color using the
// current isDarkBackground and the target's known defaults as fallback.
func applyIf(hex *AdaptiveHex, target *color.Color) {
	if hex == nil {
		return
	}
	light, dark := getDefaults(target)
	*target = hex.toColor(light, dark)
}

// applyMapKey resolves a map entry into the target color.Color.
func applyMapKey(m map[string]*AdaptiveHex, key string, target *color.Color) {
	if m == nil {
		return
	}
	if hex, ok := m[key]; ok && hex != nil {
		light, dark := getDefaults(target)
		*target = hex.toColor(light, dark)
	}
}

// parseAttrMap converts a theme's raw attribute strings into TextAttrs.
//
// Returns nil for an empty input rather than an empty map, so the common case
// -- a palette that declares no attributes at all, which is every theme
// predating bt-sk00v -- costs no allocation and reads as AttrNone on lookup.
func parseAttrMap(raw map[string]string) map[string]TextAttr {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]TextAttr, len(raw))
	for k, v := range raw {
		if a := ParseTextAttr(v); a != AttrNone {
			out[k] = a
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ApplyThemeToGlobals writes the loaded theme colors into the Color* package
// variables so all existing call sites work without changes. Call once at
// startup after LoadTheme.
func ApplyThemeToGlobals(tf *ThemeFile) {
	if tf == nil {
		return
	}
	c := &tf.Colors

	applyIf(c.Bg, &ColorBg)
	applyIf(c.BgDark, &ColorBgDark)
	applyIf(c.BgSubtle, &ColorBgSubtle)
	applyIf(c.BgHighlight, &ColorBgHighlight)
	applyIf(c.Text, &ColorText)
	applyIf(c.Subtext, &ColorSubtext)
	applyIf(c.Muted, &ColorMuted)

	applyIf(c.Primary, &ColorPrimary)
	applyIf(c.Secondary, &ColorSecondary)
	applyIf(c.Info, &ColorInfo)
	applyIf(c.Success, &ColorSuccess)
	applyIf(c.Warning, &ColorWarning)
	applyIf(c.Danger, &ColorDanger)

	applyIf(c.TextSecondary, &ColorTextSecondary)
	applyIf(c.BgContrast, &ColorBgContrast)

	// Status fg
	applyMapKey(c.Status, "open", &ColorStatusOpen)
	applyMapKey(c.Status, "in_progress", &ColorStatusInProgress)
	applyMapKey(c.Status, "blocked", &ColorStatusBlocked)
	applyMapKey(c.Status, "deferred", &ColorStatusDeferred)
	applyMapKey(c.Status, "pinned", &ColorStatusPinned)
	applyMapKey(c.Status, "hooked", &ColorStatusHooked)
	applyMapKey(c.Status, "review", &ColorStatusReview)
	applyMapKey(c.Status, "closed", &ColorStatusClosed)
	applyMapKey(c.Status, "tombstone", &ColorStatusTombstone)

	// Status bg
	applyMapKey(c.StatusBg, "open", &ColorStatusOpenBg)
	applyMapKey(c.StatusBg, "in_progress", &ColorStatusInProgressBg)
	applyMapKey(c.StatusBg, "blocked", &ColorStatusBlockedBg)
	applyMapKey(c.StatusBg, "deferred", &ColorStatusDeferredBg)
	applyMapKey(c.StatusBg, "pinned", &ColorStatusPinnedBg)
	applyMapKey(c.StatusBg, "hooked", &ColorStatusHookedBg)
	applyMapKey(c.StatusBg, "review", &ColorStatusReviewBg)
	applyMapKey(c.StatusBg, "closed", &ColorStatusClosedBg)
	applyMapKey(c.StatusBg, "tombstone", &ColorStatusTombstoneBg)

	// Priority
	applyMapKey(c.Priority, "critical", &ColorPrioCritical)
	applyMapKey(c.Priority, "high", &ColorPrioHigh)
	applyMapKey(c.Priority, "medium", &ColorPrioMedium)
	applyMapKey(c.Priority, "low", &ColorPrioLow)

	applyMapKey(c.PriorityBg, "critical", &ColorPrioCriticalBg)
	applyMapKey(c.PriorityBg, "high", &ColorPrioHighBg)
	applyMapKey(c.PriorityBg, "medium", &ColorPrioMediumBg)
	applyMapKey(c.PriorityBg, "low", &ColorPrioLowBg)

	// Text attributes (bt-sk00v). Rebuilt wholesale rather than merged, so
	// switching to a theme that declares none clears the previous theme's
	// instead of inheriting them -- an attribute is part of the palette's
	// design, not a user preference that outlives it.
	AttrStatus = parseAttrMap(c.StatusAttr)
	AttrPriority = parseAttrMap(c.PriorityAttr)

	// Type
	applyMapKey(c.Type, "bug", &ColorTypeBug)
	applyMapKey(c.Type, "feature", &ColorTypeFeature)
	applyMapKey(c.Type, "task", &ColorTypeTask)
	applyMapKey(c.Type, "epic", &ColorTypeEpic)
	applyMapKey(c.Type, "chore", &ColorTypeChore)

	// UI chrome
	applyIf(c.Border, &ColorBorder)
	applyIf(c.Highlight, &ColorHighlight)

	// Rebuild panel styles with new colors
	PanelStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorBgHighlight)
	FocusedPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorPrimary)
}

// themeColorDefaults maps Theme struct field names to their light/dark hex defaults.
var themeColorDefaults = map[string]colorDefaults{
	"Primary":    {"#3e999f", "#8abeb7"},
	"Secondary":  {"#8e908c", "#969896"},
	"Subtext":    {"#8e908c", "#b4b7b4"},
	"Info":       {"#4271ae", "#81a2be"},
	"Success":    {"#718c00", "#b5bd68"},
	"Warning":    {"#f5871f", "#de935f"},
	"Danger":     {"#c82829", "#cc6666"},
	"Open":       {"#718c00", "#b5bd68"},
	"InProgress": {"#4271ae", "#81a2be"},
	"Blocked":    {"#c82829", "#cc6666"},
	"Deferred":   {"#f5871f", "#de935f"},
	"Pinned":     {"#4271ae", "#7aa6da"},
	"Hooked":     {"#3e999f", "#8abeb7"},
	"Closed":     {"#8e908c", "#969896"},
	"Tombstone":  {"#c5c8c6", "#373b41"},
	"Review":     {"#8959a8", "#b294bb"},
	"Bug":        {"#c82829", "#cc6666"},
	"Feature":    {"#f5871f", "#de935f"},
	"Task":       {"#eab700", "#f0c674"},
	"Epic":       {"#8959a8", "#b294bb"},
	"Chore":      {"#4271ae", "#81a2be"},
	"Border":     {"#d6d6d6", "#373b41"},
	"Highlight":  {"#d6d6d6", "#373b41"},
	"Muted":      {"#8e908c", "#969896"},
}

// applyThemeField resolves an AdaptiveHex into a Theme field using its
// known defaults as fallback.
func applyThemeField(hex *AdaptiveHex, target *color.Color, fieldName string) {
	if hex == nil {
		return
	}
	d := themeColorDefaults[fieldName]
	*target = hex.toColor(d.light, d.dark)
}

func applyThemeMapKey(m map[string]*AdaptiveHex, key string, target *color.Color, fieldName string) {
	if m == nil {
		return
	}
	if hex, ok := m[key]; ok && hex != nil {
		d := themeColorDefaults[fieldName]
		*target = hex.toColor(d.light, d.dark)
	}
}

// ApplyThemeToThemeStruct updates a Theme struct's color fields from the
// loaded YAML config. Call after ApplyThemeToGlobals.
func ApplyThemeToThemeStruct(t *Theme, tf *ThemeFile) {
	if tf == nil || t == nil {
		return
	}
	c := &tf.Colors

	applyThemeField(c.Primary, &t.Primary, "Primary")
	applyThemeField(c.Secondary, &t.Secondary, "Secondary")
	applyThemeField(c.Subtext, &t.Subtext, "Subtext")

	applyThemeMapKey(c.Status, "open", &t.Open, "Open")
	applyThemeMapKey(c.Status, "in_progress", &t.InProgress, "InProgress")
	applyThemeMapKey(c.Status, "blocked", &t.Blocked, "Blocked")
	applyThemeMapKey(c.Status, "deferred", &t.Deferred, "Deferred")
	applyThemeMapKey(c.Status, "pinned", &t.Pinned, "Pinned")
	applyThemeMapKey(c.Status, "hooked", &t.Hooked, "Hooked")
	applyThemeMapKey(c.Status, "closed", &t.Closed, "Closed")
	applyThemeMapKey(c.Status, "tombstone", &t.Tombstone, "Tombstone")
	applyThemeMapKey(c.Status, "review", &t.Review, "Review")

	applyThemeMapKey(c.Type, "bug", &t.Bug, "Bug")
	applyThemeMapKey(c.Type, "feature", &t.Feature, "Feature")
	applyThemeMapKey(c.Type, "task", &t.Task, "Task")
	applyThemeMapKey(c.Type, "epic", &t.Epic, "Epic")
	applyThemeMapKey(c.Type, "chore", &t.Chore, "Chore")

	applyThemeField(c.Border, &t.Border, "Border")
	applyThemeField(c.Highlight, &t.Highlight, "Highlight")
	applyThemeField(c.Muted, &t.Muted, "Muted")

	applyThemeField(c.Info, &t.Info, "Info")
	applyThemeField(c.Success, &t.Success, "Success")
	applyThemeField(c.Warning, &t.Warning, "Warning")
	applyThemeField(c.Danger, &t.Danger, "Danger")

	// Rebuild pre-computed styles from the fields just applied above, not from
	// the Color* globals. Sourcing them from globals meant a Theme built from
	// one ThemeFile could carry styles belonging to another (bt-zq6z).
	t.MutedText = lipgloss.NewStyle().Foreground(t.Muted)
	t.MutedTextItalic = lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
	t.InfoText = lipgloss.NewStyle().Foreground(t.Info)
	t.InfoBold = lipgloss.NewStyle().Foreground(t.Info).Bold(true)
	t.SecondaryText = lipgloss.NewStyle().Foreground(t.Secondary)
	t.PrimaryBold = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	t.PriorityUpArrow = lipgloss.NewStyle().Foreground(t.Danger).Bold(true)
	t.PriorityDownArrow = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	// TriageStar has no semantic Color* counterpart yet; closest is yellow
	// (ColorPrioMedium). Kept as ThemeFg literal for now — see bt-pxbc
	// audit follow-up #2 (promote to YAML token or alias).
	t.TriageStar = lipgloss.NewStyle().Foreground(ThemeFg("#f0c674"))
	t.TriageUnblocks = lipgloss.NewStyle().Foreground(t.Success)
	t.TriageUnblocksAlt = lipgloss.NewStyle().Foreground(t.Secondary)
}

// --- Internal helpers ---

func loadEmbeddedTheme() *ThemeFile {
	data, err := defaultThemeFS.ReadFile("defaults/theme.yaml")
	if err != nil {
		return &ThemeFile{}
	}
	var tf ThemeFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return &ThemeFile{}
	}
	return &tf
}

func loadThemeFile(path string) *ThemeFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var tf ThemeFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil
	}
	return &tf
}

// mergeTheme deep-merges overlay into base. Only non-nil fields override.
func mergeTheme(base, overlay *ThemeFile) {
	// Mono is sticky: it records that the palette underneath declared itself
	// monochrome, which stays true of that palette even if a user overlay then
	// paints a token on top of it.
	base.Mono = base.Mono || overlay.Mono

	bc := &base.Colors
	oc := &overlay.Colors

	mergeHex(&bc.Bg, oc.Bg)
	mergeHex(&bc.BgDark, oc.BgDark)
	mergeHex(&bc.BgSubtle, oc.BgSubtle)
	mergeHex(&bc.BgHighlight, oc.BgHighlight)
	mergeHex(&bc.Text, oc.Text)
	mergeHex(&bc.Subtext, oc.Subtext)
	mergeHex(&bc.Muted, oc.Muted)

	mergeHex(&bc.Primary, oc.Primary)
	mergeHex(&bc.Secondary, oc.Secondary)
	mergeHex(&bc.Info, oc.Info)
	mergeHex(&bc.Success, oc.Success)
	mergeHex(&bc.Warning, oc.Warning)
	mergeHex(&bc.Danger, oc.Danger)

	mergeHex(&bc.TextSecondary, oc.TextSecondary)
	mergeHex(&bc.BgContrast, oc.BgContrast)

	mergeHex(&bc.Border, oc.Border)
	mergeHex(&bc.Highlight, oc.Highlight)

	mergeHexMap(bc.Status, oc.Status)
	mergeHexMap(bc.StatusBg, oc.StatusBg)
	mergeHexMap(bc.Priority, oc.Priority)
	mergeHexMap(bc.PriorityBg, oc.PriorityBg)
	mergeHexMap(bc.Type, oc.Type)
}

func mergeHex(base **AdaptiveHex, overlay *AdaptiveHex) {
	if overlay == nil {
		return
	}
	if *base == nil {
		*base = overlay
		return
	}
	if overlay.Dark != "" {
		(*base).Dark = overlay.Dark
	}
	if overlay.Light != "" {
		(*base).Light = overlay.Light
	}
}

func mergeHexMap(base, overlay map[string]*AdaptiveHex) {
	for k, v := range overlay {
		if v == nil {
			continue
		}
		existing, ok := base[k]
		if !ok || existing == nil {
			base[k] = v
			continue
		}
		if v.Dark != "" {
			existing.Dark = v.Dark
		}
		if v.Light != "" {
			existing.Light = v.Light
		}
	}
}
