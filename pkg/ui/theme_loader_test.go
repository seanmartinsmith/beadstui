package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTheme_EmbeddedDefaults(t *testing.T) {
	tf := LoadTheme()
	if tf == nil {
		t.Fatal("LoadTheme returned nil")
	}
	// Embedded defaults should have primary teal color
	if tf.Colors.Primary == nil {
		t.Fatal("embedded theme should have primary color")
	}
	if tf.Colors.Primary.Dark != "#8abeb7" {
		t.Errorf("expected primary dark #8abeb7, got %s", tf.Colors.Primary.Dark)
	}
}

func TestApplyThemeToGlobals(t *testing.T) {
	// Save originals
	origPrimary := ColorPrimary
	defer func() { ColorPrimary = origPrimary }()

	tf := &ThemeFile{
		Colors: ThemeColors{
			Primary: &AdaptiveHex{Dark: "#ff0000", Light: "#00ff00"},
		},
	}

	// In dark mode, the resolved color should be the dark value
	origDark := isDarkBackground
	isDarkBackground = true
	defer func() { isDarkBackground = origDark }()

	ApplyThemeToGlobals(tf)

	// Verify the resolved color is correct for dark mode
	r, g, b, _ := ColorPrimary.RGBA()
	if r>>8 != 0xff || g>>8 != 0x00 || b>>8 != 0x00 {
		t.Errorf("expected dark mode color #ff0000, got #%02x%02x%02x", r>>8, g>>8, b>>8)
	}
}

func TestApplyThemeToGlobals_Nil(t *testing.T) {
	// Should not panic
	ApplyThemeToGlobals(nil)
}

func TestApplyThemeToThemeStruct(t *testing.T) {
	origDark := isDarkBackground
	isDarkBackground = true
	defer func() { isDarkBackground = origDark }()

	theme := DefaultTheme()

	tf := &ThemeFile{
		Colors: ThemeColors{
			Primary: &AdaptiveHex{Dark: "#aabbcc"},
		},
	}
	ApplyThemeToThemeStruct(&theme, tf)

	// In dark mode, should be the overridden dark value
	r, g, b, _ := theme.Primary.RGBA()
	if r>>8 != 0xaa || g>>8 != 0xbb || b>>8 != 0xcc {
		t.Errorf("expected primary #aabbcc in dark mode, got #%02x%02x%02x", r>>8, g>>8, b>>8)
	}
}

func TestAdaptiveHex_ToColor(t *testing.T) {
	// Full override
	hex := AdaptiveHex{Dark: "#111111", Light: "#222222"}
	origDark := isDarkBackground
	defer func() { isDarkBackground = origDark }()

	isDarkBackground = true
	result := hex.toColor("#aaa", "#bbb")
	r, g, b, _ := result.RGBA()
	if r>>8 != 0x11 || g>>8 != 0x11 || b>>8 != 0x11 {
		t.Errorf("dark mode full override failed: got #%02x%02x%02x", r>>8, g>>8, b>>8)
	}

	isDarkBackground = false
	result = hex.toColor("#aaa", "#bbb")
	r, g, b, _ = result.RGBA()
	if r>>8 != 0x22 || g>>8 != 0x22 || b>>8 != 0x22 {
		t.Errorf("light mode full override failed: got #%02x%02x%02x", r>>8, g>>8, b>>8)
	}

	// Partial override (dark only) - light should use fallback
	hex = AdaptiveHex{Dark: "#333333"}
	isDarkBackground = true
	result = hex.toColor("#aaaaaa", "#bbbbbb")
	r, g, b, _ = result.RGBA()
	if r>>8 != 0x33 || g>>8 != 0x33 || b>>8 != 0x33 {
		t.Errorf("partial dark override failed: got #%02x%02x%02x", r>>8, g>>8, b>>8)
	}

	isDarkBackground = false
	result = hex.toColor("#aaaaaa", "#bbbbbb")
	r, g, b, _ = result.RGBA()
	if r>>8 != 0xaa || g>>8 != 0xaa || b>>8 != 0xaa {
		t.Errorf("partial light fallback failed: got #%02x%02x%02x", r>>8, g>>8, b>>8)
	}

	// Empty (no override) - should use fallback
	hex = AdaptiveHex{}
	isDarkBackground = true
	result = hex.toColor("#aaaaaa", "#bbbbbb")
	r, g, b, _ = result.RGBA()
	if r>>8 != 0xbb || g>>8 != 0xbb || b>>8 != 0xbb {
		t.Errorf("empty should use dark fallback: got #%02x%02x%02x", r>>8, g>>8, b>>8)
	}
}

func TestMergeTheme_PartialOverride(t *testing.T) {
	base := &ThemeFile{
		Colors: ThemeColors{
			Primary: &AdaptiveHex{Dark: "#111111", Light: "#222222"},
			Info:    &AdaptiveHex{Dark: "#333333", Light: "#444444"},
		},
	}
	overlay := &ThemeFile{
		Colors: ThemeColors{
			Primary: &AdaptiveHex{Dark: "#aaaaaa"}, // Only override dark
		},
	}
	mergeTheme(base, overlay)

	if base.Colors.Primary.Dark != "#aaaaaa" {
		t.Errorf("expected dark #aaaaaa, got %s", base.Colors.Primary.Dark)
	}
	if base.Colors.Primary.Light != "#222222" {
		t.Errorf("expected light #222222 (unchanged), got %s", base.Colors.Primary.Light)
	}
	// Info should be untouched
	if base.Colors.Info.Dark != "#333333" {
		t.Errorf("expected info dark #333333 (unchanged), got %s", base.Colors.Info.Dark)
	}
}

func TestLoadThemeFile_MalformedYAML(t *testing.T) {
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.yaml")
	os.WriteFile(badFile, []byte("{{invalid yaml"), 0644)

	result := loadThemeFile(badFile)
	if result != nil {
		t.Error("malformed YAML should return nil")
	}
}

func TestLoadThemeFile_ValidPartial(t *testing.T) {
	tmpDir := t.TempDir()
	partial := filepath.Join(tmpDir, "theme.yaml")
	os.WriteFile(partial, []byte(`
colors:
  primary: { dark: "#ff79c6" }
`), 0644)

	result := loadThemeFile(partial)
	if result == nil {
		t.Fatal("valid partial YAML should not return nil")
	}
	if result.Colors.Primary == nil {
		t.Fatal("primary should be parsed")
	}
	if result.Colors.Primary.Dark != "#ff79c6" {
		t.Errorf("expected #ff79c6, got %s", result.Colors.Primary.Dark)
	}
	// Light not specified - should be empty
	if result.Colors.Primary.Light != "" {
		t.Errorf("light should be empty, got %s", result.Colors.Primary.Light)
	}
}

func TestLoadThemeFile_Nonexistent(t *testing.T) {
	result := loadThemeFile("/nonexistent/path/theme.yaml")
	if result != nil {
		t.Error("nonexistent file should return nil")
	}
}
