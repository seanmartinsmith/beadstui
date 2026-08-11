package ui

// bt-native themes (bt-ba9fc).
//
// themes/btop holds the upstream corpus verbatim so that re-vendoring stays a
// copy rather than a merge; nothing in this repo may edit those files. That
// leaves bt with nowhere to author a palette of its own. Routing one through
// btop's format instead would mean expressing bt's tokens in btop's
// vocabulary -- cpu_box, temp_start, download_end -- keys that describe a
// system monitor, not an issue tracker, and which the adapter would then have
// to reverse-engineer back into bt semantics it already knows.
//
// So bt-native themes are plain ThemeFile YAML: exactly the schema a user
// already writes in ~/.config/bt/theme.yaml. Authoring one needs no new
// vocabulary and no adapter, and a user can copy a shipped theme out of this
// directory as the starting point for their own.

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed themes/bt/*.yaml
var nativeThemeFS embed.FS

const nativeThemeDir = "themes/bt"

// Source prefixes, so either corpus stays addressable by name even where both
// define the same one. Without them, shipping a bt-native "greyscale" would
// make the vendored file unreachable rather than merely shadowed.
const (
	nativeThemePrefix = "bt:"
	btopThemePrefix   = "btop:"
)

// NativeThemeNames returns the bt-authored theme names, sorted.
func NativeThemeNames() []string {
	entries, err := fs.ReadDir(nativeThemeFS, nativeThemeDir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if n := strings.TrimSuffix(e.Name(), ".yaml"); n != e.Name() {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

// LoadNativeTheme resolves a bt-authored theme by name.
func LoadNativeTheme(name string) (*ThemeFile, error) {
	data, err := nativeThemeFS.ReadFile(nativeThemeDir + "/" + name + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("read bt theme %q: %w", name, err)
	}
	var tf ThemeFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("parse bt theme %q: %w", name, err)
	}
	// A bt-native file names a palette, it does not select one. Leaving Theme
	// set would make a theme able to redirect to another theme, which the
	// layering in LoadTheme has no cycle guard for.
	tf.Theme = ""
	return &tf, nil
}

// ResolveTheme turns a theme name into a ThemeFile, searching bt-native
// palettes before the vendored btop corpus.
//
// Precedence is bt-native first. A bt-native theme deliberately sharing a
// vendored name is a replacement: it is authored directly in bt's token
// vocabulary rather than adapted out of a system monitor's, so it is the
// better answer to that name. The vendored file cannot be edited to say so,
// because it must stay byte-identical to upstream, so precedence is where the
// decision has to live. Both sources stay explicitly addressable through the
// "bt:" and "btop:" prefixes, which also let a caller pin a source and not
// care what gets added to the other later.
func ResolveTheme(name string) (*ThemeFile, error) {
	name = strings.TrimSpace(name)
	switch {
	case strings.HasPrefix(name, nativeThemePrefix):
		return LoadNativeTheme(strings.TrimPrefix(name, nativeThemePrefix))
	case strings.HasPrefix(name, btopThemePrefix):
		return LoadBtopTheme(strings.TrimPrefix(name, btopThemePrefix))
	}
	if tf, err := LoadNativeTheme(name); err == nil {
		return tf, nil
	}
	return LoadBtopTheme(name)
}

// ThemeNames returns every selectable theme name, bt-native first, then the
// vendored corpus. A vendored name shadowed by a bt-native one is returned in
// its "btop:" form, so every name this returns resolves to the palette it
// names -- a picker can list these verbatim without having to know the
// precedence rule.
func ThemeNames() []string {
	native := NativeThemeNames()
	shadowed := make(map[string]bool, len(native))
	for _, n := range native {
		shadowed[n] = true
	}
	out := append([]string{}, native...)
	for _, n := range BtopThemeNames() {
		if shadowed[n] {
			out = append(out, btopThemePrefix+n)
			continue
		}
		out = append(out, n)
	}
	return out
}
