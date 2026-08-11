package ui

// Persisting the picked palette (bt-4ibsq).
//
// The whole file cannot simply be reserialized from ThemeFile. A user's
// theme.yaml is hand-authored: it carries comments, key order, and -- most
// importantly -- a colors: block whose per-token overrides layer ON TOP of the
// named palette. That layering is the guarantee that picking a theme never
// discards a user's tweaks, so a save path that flattened the file would break
// the exact promise the picker is built on.
//
// So this edits the document in place through yaml.Node: find the theme: key,
// replace its value, leave every other node untouched. Comments and ordering
// survive because the nodes carrying them are never rebuilt.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// userThemePath returns the file a picked palette is written to.
//
// User scope rather than project scope (.bt/theme.yaml): a palette is a
// person's preference, not a property of the repository they happen to have
// open. Writing it per-project would make the choice silently fail to follow
// the user to the next repo, which reads as the setting not saving at all.
func userThemePath() (string, error) {
	// themeConfigHome is a test seam, mirroring tutorial_progress.go. Tests
	// cannot go through os.UserHomeDir: on Windows it ignores HOME, so a test
	// that set HOME would write into the developer's real config.
	if themeConfigHome != "" {
		return filepath.Join(themeConfigHome, ".config", "bt", "theme.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home dir: %w", err)
	}
	return filepath.Join(home, ".config", "bt", "theme.yaml"), nil
}

// themeConfigHome overrides the home directory for tests; empty uses the real
// one.
var themeConfigHome string

// ThemeEnvOverride reports the BT_THEME value if one is set.
//
// BT_THEME outranks both config files, so with it exported a saved palette
// will not take effect on the next run. The picker surfaces this rather than
// letting the setting appear silently broken.
func ThemeEnvOverride() string { return strings.TrimSpace(os.Getenv("BT_THEME")) }

// SaveSelectedTheme writes name as the theme: key in the user's theme.yaml,
// preserving everything else in the file.
func SaveSelectedTheme(name string) error {
	path, err := userThemePath()
	if err != nil {
		return err
	}

	var doc yaml.Node
	existing, readErr := os.ReadFile(path)
	switch {
	case readErr == nil && len(strings.TrimSpace(string(existing))) > 0:
		if err := yaml.Unmarshal(existing, &doc); err != nil {
			// Refuse rather than overwrite. A file that fails to parse is far
			// more likely to be a user's work-in-progress edit than something
			// bt should replace with a one-key document.
			return fmt.Errorf("parse %s: %w", path, err)
		}
	case readErr != nil && !os.IsNotExist(readErr):
		return fmt.Errorf("read %s: %w", path, err)
	}

	root := documentRoot(&doc)
	if root == nil {
		return fmt.Errorf("%s: top level is not a YAML mapping", path)
	}
	setMappingValue(root, "theme", name)

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("encode theme file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	// Write via a temp file in the same directory then rename, so an
	// interrupted write cannot leave a user with a truncated theme.yaml --
	// which, given this file can hold hand-written overrides, would lose work
	// bt did not create.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".theme-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

// documentRoot returns the mapping node at the top of the document, creating
// the scaffolding when the file was absent or empty.
func documentRoot(doc *yaml.Node) *yaml.Node {
	if doc.Kind == 0 {
		doc.Kind = yaml.DocumentNode
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	return root
}

// setMappingValue sets key to value on a mapping node, replacing the existing
// entry in place if present so its position and any comment attached to the key
// are preserved.
func setMappingValue(mapping *yaml.Node, key, value string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != key {
			continue
		}
		v := mapping.Content[i+1]
		v.Kind = yaml.ScalarNode
		v.Tag = "!!str"
		v.Value = value
		// Clear any style that would misrepresent the new value, e.g. a
		// previously-quoted or multiline scalar.
		v.Style = 0
		return
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)
}
