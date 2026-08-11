package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withThemeConfigHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev := themeConfigHome
	themeConfigHome = dir
	t.Cleanup(func() { themeConfigHome = prev })
	return filepath.Join(dir, ".config", "bt", "theme.yaml")
}

// TestSaveThemePreservesUserOverrides is the reason this save path edits YAML
// nodes instead of reserializing a ThemeFile. The colors: block layers ON TOP
// of the named palette, so a save that dropped it would break the guarantee
// that picking a theme never discards per-token tweaks -- and would delete
// work bt did not create.
func TestSaveThemePreservesUserOverrides(t *testing.T) {
	path := withThemeConfigHome(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := `# my theme
# hand-written, do not lose
theme: tokyo-night

colors:
  # I like this teal
  primary: { dark: "#00ffcc", light: "#008877" }
  status:
    blocked: { dark: "#ff0000", light: "#aa0000" }
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := SaveSelectedTheme("loam"); err != nil {
		t.Fatalf("save: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	got := string(out)

	if !strings.Contains(got, "theme: loam") {
		t.Errorf("theme key not updated:\n%s", got)
	}
	if strings.Contains(got, "tokyo-night") {
		t.Errorf("old theme value still present:\n%s", got)
	}
	for _, want := range []string{
		"#00ffcc",          // the user's primary override
		"#ff0000",          // a nested status override
		"I like this teal", // an inline comment
		"hand-written",     // a leading document comment
	} {
		if !strings.Contains(got, want) {
			t.Errorf("save lost %q from the user's file:\n%s", want, got)
		}
	}
}

// TestSaveThemeCreatesFile covers the first-run path: no config file yet.
func TestSaveThemeCreatesFile(t *testing.T) {
	path := withThemeConfigHome(t)

	if err := SaveSelectedTheme("greyscale"); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(out), "theme: greyscale") {
		t.Errorf("created file missing theme key:\n%s", out)
	}
}

// TestSaveThemeAppendsWhenKeyAbsent covers a config file that overrides colors
// but has never named a palette.
func TestSaveThemeAppendsWhenKeyAbsent(t *testing.T) {
	path := withThemeConfigHome(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("colors:\n  primary: { dark: \"#123456\", light: \"#123456\" }\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := SaveSelectedTheme("loam"); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, _ := os.ReadFile(path)
	got := string(out)
	if !strings.Contains(got, "theme: loam") {
		t.Errorf("theme key not appended:\n%s", got)
	}
	if !strings.Contains(got, "#123456") {
		t.Errorf("existing override lost:\n%s", got)
	}
}

// TestSaveThemeRefusesUnparseableFile pins the refusal. A file that does not
// parse is far more likely to be a user mid-edit than something bt should
// replace with a one-key document.
func TestSaveThemeRefusesUnparseableFile(t *testing.T) {
	path := withThemeConfigHome(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	broken := "colors:\n  primary: { dark: \"#fff\"\n   oops: [unclosed\n"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := SaveSelectedTheme("loam"); err == nil {
		t.Error("expected an error rather than overwriting an unparseable file")
	}
	out, _ := os.ReadFile(path)
	if string(out) != broken {
		t.Errorf("unparseable file was modified:\n%s", out)
	}
}

// TestSaveThenLoadRoundTrip is the end-to-end the user actually reported on:
// pick a theme, and it is still there next run.
func TestSaveThenLoadRoundTrip(t *testing.T) {
	restoreThemeGlobals(t)
	withThemeConfigHome(t)
	t.Setenv("BT_THEME", "")

	if err := SaveSelectedTheme("loam"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := SelectedThemeName(); got != "loam" {
		t.Errorf("after saving, SelectedThemeName() = %q, want loam", got)
	}
}
