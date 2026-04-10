package ui

import (
	"embed"
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

// toAdaptiveColor converts to AdaptiveColor.
// Missing values inherit from the fallback.
func (a AdaptiveHex) toAdaptiveColor(fallback AdaptiveColor) AdaptiveColor {
	result := fallback
	if a.Dark != "" {
		result.Dark = a.Dark
	}
	if a.Light != "" {
		result.Light = a.Light
	}
	return result
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

// ApplyThemeToThemeStruct updates a Theme struct's color fields from the
// loaded YAML config. Call after ApplyThemeToGlobals.
func ApplyThemeToThemeStruct(t *Theme, tf *ThemeFile) {
	if tf == nil || t == nil {
		return
	}
	c := &tf.Colors

	applyIf(c.Primary, &t.Primary)
	applyIf(c.Secondary, &t.Secondary)
	applyIf(c.Subtext, &t.Subtext)

	applyMapKey(c.Status, "open", &t.Open)
	applyMapKey(c.Status, "in_progress", &t.InProgress)
	applyMapKey(c.Status, "blocked", &t.Blocked)
	applyMapKey(c.Status, "deferred", &t.Deferred)
	applyMapKey(c.Status, "pinned", &t.Pinned)
	applyMapKey(c.Status, "hooked", &t.Hooked)
	applyMapKey(c.Status, "closed", &t.Closed)
	applyMapKey(c.Status, "tombstone", &t.Tombstone)
	applyMapKey(c.Status, "review", &t.Review)

	applyMapKey(c.Type, "bug", &t.Bug)
	applyMapKey(c.Type, "feature", &t.Feature)
	applyMapKey(c.Type, "task", &t.Task)
	applyMapKey(c.Type, "epic", &t.Epic)
	applyMapKey(c.Type, "chore", &t.Chore)

	applyIf(c.Border, &t.Border)
	applyIf(c.Highlight, &t.Highlight)
	applyIf(c.Muted, &t.Muted)

	applyIf(c.Info, &t.Info)
	applyIf(c.Success, &t.Success)
	applyIf(c.Warning, &t.Warning)
	applyIf(c.Danger, &t.Danger)

	// Rebuild pre-computed styles
	t.MutedText = lipgloss.NewStyle().Foreground(ColorMuted)
	t.InfoText = lipgloss.NewStyle().Foreground(ColorInfo)
	t.InfoBold = lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
	t.SecondaryText = lipgloss.NewStyle().Foreground(t.Secondary)
	t.PrimaryBold = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	t.PriorityUpArrow = lipgloss.NewStyle().Foreground(ThemeFg(ColorDanger.Dark)).Bold(true)
	t.PriorityDownArrow = lipgloss.NewStyle().Foreground(ThemeFg(ColorSuccess.Dark)).Bold(true)
	t.TriageStar = lipgloss.NewStyle().Foreground(ThemeFg("#f0c674"))
	t.TriageUnblocks = lipgloss.NewStyle().Foreground(ThemeFg(ColorSuccess.Dark))
	t.TriageUnblocksAlt = lipgloss.NewStyle().Foreground(ThemeFg(ColorSecondary.Dark))
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

func applyIf(hex *AdaptiveHex, target *AdaptiveColor) {
	if hex == nil {
		return
	}
	*target = hex.toAdaptiveColor(*target)
}

func applyMapKey(m map[string]*AdaptiveHex, key string, target *AdaptiveColor) {
	if m == nil {
		return
	}
	if hex, ok := m[key]; ok && hex != nil {
		*target = hex.toAdaptiveColor(*target)
	}
}
