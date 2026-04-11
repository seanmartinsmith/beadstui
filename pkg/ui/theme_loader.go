package ui

import (
	"embed"
	"image/color"
	"os"
	"path/filepath"

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

	// Type
	Type map[string]*AdaptiveHex `yaml:"type"`

	// UI chrome
	Border    *AdaptiveHex `yaml:"border"`
	Highlight *AdaptiveHex `yaml:"highlight"`
}

// ThemeFile is the top-level YAML structure.
type ThemeFile struct {
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
func LoadTheme() *ThemeFile {
	// Layer 1: embedded defaults
	base := loadEmbeddedTheme()

	// Layer 2: user-level override (~/.config/bt/theme.yaml)
	if home, err := os.UserHomeDir(); err == nil {
		userPath := filepath.Join(home, ".config", "bt", "theme.yaml")
		if user := loadThemeFile(userPath); user != nil {
			mergeTheme(base, user)
		}
	}

	// Layer 3: project-level override (.bt/theme.yaml)
	if proj := loadThemeFile(filepath.Join(".bt", "theme.yaml")); proj != nil {
		mergeTheme(base, proj)
	}

	return base
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

	// Rebuild pre-computed styles
	t.MutedText = lipgloss.NewStyle().Foreground(ColorMuted)
	t.InfoText = lipgloss.NewStyle().Foreground(ColorInfo)
	t.InfoBold = lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
	t.SecondaryText = lipgloss.NewStyle().Foreground(t.Secondary)
	t.PrimaryBold = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	t.PriorityUpArrow = lipgloss.NewStyle().Foreground(ColorDanger).Bold(true)
	t.PriorityDownArrow = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	t.TriageStar = lipgloss.NewStyle().Foreground(ThemeFg("#f0c674"))
	t.TriageUnblocks = lipgloss.NewStyle().Foreground(ColorSuccess)
	t.TriageUnblocksAlt = lipgloss.NewStyle().Foreground(ColorSecondary)
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
