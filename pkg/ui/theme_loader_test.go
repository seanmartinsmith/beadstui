package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
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
	ApplyThemeToGlobals(tf)

	if ColorPrimary.Dark != "#ff0000" {
		t.Errorf("expected dark #ff0000, got %s", ColorPrimary.Dark)
	}
	if ColorPrimary.Light != "#00ff00" {
		t.Errorf("expected light #00ff00, got %s", ColorPrimary.Light)
	}
}

func TestApplyThemeToGlobals_Nil(t *testing.T) {
	// Should not panic
	ApplyThemeToGlobals(nil)
}

func TestApplyThemeToThemeStruct(t *testing.T) {
	r := lipgloss.NewRenderer(nil)
	theme := DefaultTheme(r)

	tf := &ThemeFile{
		Colors: ThemeColors{
			Primary: &AdaptiveHex{Dark: "#aabbcc"},
		},
	}
	ApplyThemeToThemeStruct(&theme, tf)

	if theme.Primary.Dark != "#aabbcc" {
		t.Errorf("expected primary dark #aabbcc, got %s", theme.Primary.Dark)
	}
	// Light should be preserved from default
	if theme.Primary.Light != "#3e999f" {
		t.Errorf("expected primary light #3e999f (unchanged), got %s", theme.Primary.Light)
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

func TestAdaptiveHex_ToAdaptiveColor(t *testing.T) {
	fallback := lipgloss.AdaptiveColor{Light: "#aaa", Dark: "#bbb"}

	// Full override
	hex := AdaptiveHex{Dark: "#111", Light: "#222"}
	result := hex.toAdaptiveColor(fallback)
	if result.Dark != "#111" || result.Light != "#222" {
		t.Errorf("full override failed: got %v", result)
	}

	// Partial override (dark only)
	hex = AdaptiveHex{Dark: "#333"}
	result = hex.toAdaptiveColor(fallback)
	if result.Dark != "#333" || result.Light != "#aaa" {
		t.Errorf("partial override failed: got %v", result)
	}

	// Empty (no override)
	hex = AdaptiveHex{}
	result = hex.toAdaptiveColor(fallback)
	if result != fallback {
		t.Errorf("empty should return fallback: got %v", result)
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
