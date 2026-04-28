package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFrom_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	g, err := LoadFrom(filepath.Join(tmp, "settings.json"))
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if g == nil {
		t.Fatalf("expected non-nil zero-value Global, got nil")
	}
	if g.AnchorProject != "" {
		t.Fatalf("expected zero-value AnchorProject, got %q", g.AnchorProject)
	}
}

func TestLoadFrom_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	in := &Global{AnchorProject: "C:/Users/sms/System/tools/bt"}
	if err := in.SaveTo(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if out.AnchorProject != in.AnchorProject {
		t.Fatalf("anchor round-trip mismatch: got %q want %q", out.AnchorProject, in.AnchorProject)
	}
}

func TestLoadFrom_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("seed empty file: %v", err)
	}
	g, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load empty file: %v", err)
	}
	if g.AnchorProject != "" {
		t.Fatalf("expected empty anchor for empty file, got %q", g.AnchorProject)
	}
}

func TestLoadFrom_CorruptJSONErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o644); err != nil {
		t.Fatalf("seed corrupt file: %v", err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Fatalf("expected error parsing corrupt JSON, got nil")
	}
}

func TestSaveTo_AtomicReplaceOverExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := (&Global{AnchorProject: "/old"}).SaveTo(path); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := (&Global{AnchorProject: "/new"}).SaveTo(path); err != nil {
		t.Fatalf("overwrite save: %v", err)
	}
	out, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load after overwrite: %v", err)
	}
	if out.AnchorProject != "/new" {
		t.Fatalf("overwrite did not stick: got %q", out.AnchorProject)
	}
	// No leftover tempfiles.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Fatalf("tempfile leaked: %s", e.Name())
		}
	}
}

func TestSaveTo_CreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "subdir", "settings.json")
	if err := (&Global{AnchorProject: "/x"}).SaveTo(path); err != nil {
		t.Fatalf("save into nested dir: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file created at nested path: %v", err)
	}
}

func TestAnchor_EnvOverridesFile(t *testing.T) {
	t.Setenv(AnchorEnvVar, "/from/env")
	g := &Global{AnchorProject: "/from/file"}
	if got := g.Anchor(); got != "/from/env" {
		t.Fatalf("expected env to win: got %q", got)
	}
}

func TestAnchor_FileWhenEnvUnset(t *testing.T) {
	t.Setenv(AnchorEnvVar, "")
	g := &Global{AnchorProject: "/from/file"}
	if got := g.Anchor(); got != "/from/file" {
		t.Fatalf("expected file anchor: got %q", got)
	}
}

func TestAnchor_EmptyWhenBothUnset(t *testing.T) {
	t.Setenv(AnchorEnvVar, "")
	g := &Global{}
	if got := g.Anchor(); got != "" {
		t.Fatalf("expected empty anchor: got %q", got)
	}
}

func TestAnchor_EnvWhitespaceOnlyIgnored(t *testing.T) {
	t.Setenv(AnchorEnvVar, "   ")
	g := &Global{AnchorProject: "/from/file"}
	if got := g.Anchor(); got != "/from/file" {
		t.Fatalf("expected whitespace env to fall through to file: got %q", got)
	}
}

func TestAnchor_NilReceiverWithEnv(t *testing.T) {
	t.Setenv(AnchorEnvVar, "/from/env")
	var g *Global
	if got := g.Anchor(); got != "/from/env" {
		t.Fatalf("expected env anchor on nil receiver: got %q", got)
	}
}

func TestAnchor_NilReceiverNoEnv(t *testing.T) {
	t.Setenv(AnchorEnvVar, "")
	var g *Global
	if got := g.Anchor(); got != "" {
		t.Fatalf("expected empty anchor on nil receiver: got %q", got)
	}
}

func TestAnchorFromEnv(t *testing.T) {
	t.Setenv(AnchorEnvVar, "")
	if AnchorFromEnv() {
		t.Fatalf("expected false when env unset")
	}
	t.Setenv(AnchorEnvVar, "/x")
	if !AnchorFromEnv() {
		t.Fatalf("expected true when env set")
	}
}
