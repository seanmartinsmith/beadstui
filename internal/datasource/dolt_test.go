package datasource

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadDoltConfig_ValidMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := `{"backend":"dolt","dolt_mode":"server","dolt_database":"beads_dotfiles"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "metadata.json"), []byte(metadata), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, ok := ReadDoltConfig(tmpDir)
	if !ok {
		t.Fatal("Expected ReadDoltConfig to return true for dolt backend")
	}

	if cfg.Host != "127.0.0.1" {
		t.Errorf("Expected host 127.0.0.1, got %s", cfg.Host)
	}
	if cfg.Port != 3307 {
		t.Errorf("Expected default port 3307, got %d", cfg.Port)
	}
	if cfg.Database != "beads_dotfiles" {
		t.Errorf("Expected database beads_dotfiles, got %s", cfg.Database)
	}
	if cfg.User != "root" {
		t.Errorf("Expected user root, got %s", cfg.User)
	}
}

func TestReadDoltConfig_CustomPort(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := `{"backend":"dolt","dolt_database":"mydb"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "metadata.json"), []byte(metadata), 0644); err != nil {
		t.Fatal(err)
	}

	doltDir := filepath.Join(tmpDir, "dolt")
	if err := os.MkdirAll(doltDir, 0755); err != nil {
		t.Fatal(err)
	}

	doltCfg := "listener:\n  port: 3309\n"
	if err := os.WriteFile(filepath.Join(doltDir, "config.yaml"), []byte(doltCfg), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, ok := ReadDoltConfig(tmpDir)
	if !ok {
		t.Fatal("Expected ReadDoltConfig to return true")
	}
	if cfg.Port != 3309 {
		t.Errorf("Expected custom port 3309, got %d", cfg.Port)
	}
	if cfg.Database != "mydb" {
		t.Errorf("Expected database mydb, got %s", cfg.Database)
	}
}

func TestReadDoltConfig_PortFileOverridesConfigYaml(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := `{"backend":"dolt","dolt_database":"bv"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "metadata.json"), []byte(metadata), 0644); err != nil {
		t.Fatal(err)
	}

	// config.yaml says 13729 (stale hash-derived port)
	doltDir := filepath.Join(tmpDir, "dolt")
	if err := os.MkdirAll(doltDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(doltDir, "config.yaml"), []byte("listener:\n  port: 13729\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// dolt-server.port says 3307 (actual running server)
	if err := os.WriteFile(filepath.Join(tmpDir, "dolt-server.port"), []byte("3307\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, ok := ReadDoltConfig(tmpDir)
	if !ok {
		t.Fatal("Expected ReadDoltConfig to return true")
	}
	if cfg.Port != 3307 {
		t.Errorf("Port file should override config.yaml: want 3307, got %d", cfg.Port)
	}
}

func TestReadDoltConfig_DefaultDatabase(t *testing.T) {
	tmpDir := t.TempDir()

	// No dolt_database field - should default to "beads"
	metadata := `{"backend":"dolt","dolt_mode":"server"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "metadata.json"), []byte(metadata), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, ok := ReadDoltConfig(tmpDir)
	if !ok {
		t.Fatal("Expected ReadDoltConfig to return true")
	}
	if cfg.Database != "beads" {
		t.Errorf("Expected default database 'beads', got %s", cfg.Database)
	}
}

func TestReadDoltConfig_NotDoltBackend(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := `{"database":"beads.db","backend":"sqlite"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "metadata.json"), []byte(metadata), 0644); err != nil {
		t.Fatal(err)
	}

	_, ok := ReadDoltConfig(tmpDir)
	if ok {
		t.Error("Expected ReadDoltConfig to return false for non-dolt backend")
	}
}

func TestReadDoltConfig_NoMetadataFile(t *testing.T) {
	tmpDir := t.TempDir()

	_, ok := ReadDoltConfig(tmpDir)
	if ok {
		t.Error("Expected ReadDoltConfig to return false when metadata.json is missing")
	}
}

func TestReadDoltConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "metadata.json"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, ok := ReadDoltConfig(tmpDir)
	if ok {
		t.Error("Expected ReadDoltConfig to return false for invalid JSON")
	}
}

func TestDoltConfig_DSN(t *testing.T) {
	cfg := DoltConfig{
		Host:     "127.0.0.1",
		Port:     3307,
		Database: "beads_dotfiles",
		User:     "root",
	}

	expected := "root@tcp(127.0.0.1:3307)/beads_dotfiles?parseTime=true&timeout=2s"
	if got := cfg.DSN(); got != expected {
		t.Errorf("DSN mismatch:\n  got:  %s\n  want: %s", got, expected)
	}
}

func TestDoltConfig_DSN_CustomValues(t *testing.T) {
	cfg := DoltConfig{
		Host:     "192.168.1.100",
		Port:     3309,
		Database: "myproject",
		User:     "admin",
	}

	expected := "admin@tcp(192.168.1.100:3309)/myproject?parseTime=true&timeout=2s"
	if got := cfg.DSN(); got != expected {
		t.Errorf("DSN mismatch:\n  got:  %s\n  want: %s", got, expected)
	}
}

func TestDiscoverDoltSources_NoDoltConfig(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	sources, err := discoverDoltSources(beadsDir, DiscoveryOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("Expected 0 sources for non-dolt project, got %d", len(sources))
	}
}

func TestSourceTypeDolt_Priority(t *testing.T) {
	if PriorityDolt <= PrioritySQLite {
		t.Errorf("Dolt priority (%d) should be higher than SQLite (%d)", PriorityDolt, PrioritySQLite)
	}
}

func TestSelectBestSource_DoltWinsTiebreak(t *testing.T) {
	now := time.Now()
	sources := []DataSource{
		{
			Type:     SourceTypeSQLite,
			Path:     "/test/beads.db",
			Priority: PrioritySQLite,
			ModTime:  now,
			Valid:    true,
		},
		{
			Type:     SourceTypeDolt,
			Path:     "root@tcp(127.0.0.1:3307)/beads?parseTime=true&timeout=2s",
			Priority: PriorityDolt,
			ModTime:  now, // Same time
			Valid:    true,
		},
	}

	selected, err := SelectBestSource(sources)
	if err != nil {
		t.Fatalf("Selection failed: %v", err)
	}

	if selected.Type != SourceTypeDolt {
		t.Errorf("Expected Dolt (highest priority) to win tiebreak, got %s", selected.Type)
	}
}

func TestBuildSelectionReason_Dolt(t *testing.T) {
	now := time.Now()
	selected := DataSource{
		Type:     SourceTypeDolt,
		Path:     "root@tcp(127.0.0.1:3307)/beads?parseTime=true&timeout=2s",
		Priority: PriorityDolt,
		ModTime:  now,
		Valid:    true,
	}
	candidates := []DataSource{
		selected,
		{
			Type:     SourceTypeSQLite,
			Path:     "/test/beads.db",
			Priority: PrioritySQLite,
			ModTime:  now.Add(-1 * time.Hour),
			Valid:    true,
		},
	}

	reason := buildSelectionReason(selected, candidates, DefaultSelectionOptions())
	if reason != "freshest modification time" && reason != "highest priority (110)" && reason != "Dolt is most authoritative" {
		t.Errorf("Unexpected reason: %s", reason)
	}
}
