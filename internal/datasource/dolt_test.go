package datasource

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bt-edi: schema-drift tolerance. When a column on the issues table is
// absent (e.g. upstream beads drops closed_by_session for the events-table
// redesign), the single-DB scan path must NULL-substitute it instead of
// silently degrading to the 8-column fallback.
func TestBuildSingleDBIssuesQuery_NullSubstitutesMissingColumns(t *testing.T) {
	available := map[string]bool{}
	for _, c := range IssuesColumnList {
		available[c] = true
	}
	delete(available, "closed_by_session")

	query := buildSingleDBIssuesQuery(available)

	if !strings.Contains(query, "NULL AS closed_by_session") {
		t.Errorf("expected NULL substitution for missing closed_by_session column.\nquery:\n%s", query)
	}
	if strings.Contains(query, "NULL AS id") {
		t.Errorf("present columns must not be NULL-substituted.\nquery:\n%s", query)
	}
	if !strings.Contains(query, "FROM issues") {
		t.Errorf("query must read from issues table.\nquery:\n%s", query)
	}
	if !strings.Contains(query, "status != 'tombstone'") {
		t.Errorf("query must filter tombstones.\nquery:\n%s", query)
	}
}

func TestBuildSingleDBIssuesQuery_AllPresent(t *testing.T) {
	available := map[string]bool{}
	for _, c := range IssuesColumnList {
		available[c] = true
	}

	query := buildSingleDBIssuesQuery(available)

	if strings.Contains(query, "NULL AS") {
		t.Errorf("no NULL substitutions expected when all columns present.\nquery:\n%s", query)
	}
	for _, col := range IssuesColumnList {
		if !strings.Contains(query, col) {
			t.Errorf("expected column %q in SELECT list.\nquery:\n%s", col, query)
		}
	}
}

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

func TestReadDoltConfig_EmbeddedMode(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := `{"backend":"dolt","dolt_mode":"embedded","dolt_database":"beads"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "metadata.json"), []byte(metadata), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, ok := ReadDoltConfig(tmpDir)
	if !ok {
		t.Fatal("Expected ReadDoltConfig to return true for embedded dolt backend")
	}
	if cfg.Mode != "embedded" {
		t.Errorf("Expected Mode embedded, got %q", cfg.Mode)
	}
	want := filepath.Join(tmpDir, "embeddeddolt")
	if cfg.EmbeddedDataDir != want {
		t.Errorf("Expected EmbeddedDataDir %q, got %q", want, cfg.EmbeddedDataDir)
	}
	if cfg.PortFromEnv {
		t.Error("Expected PortFromEnv false when no port env var is set")
	}
}

func TestReadDoltConfig_PortFromEnv(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := `{"backend":"dolt","dolt_mode":"embedded","dolt_database":"beads"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "metadata.json"), []byte(metadata), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("BEADS_DOLT_SERVER_PORT", "45678")
	cfg, ok := ReadDoltConfig(tmpDir)
	if !ok {
		t.Fatal("Expected ReadDoltConfig to return true")
	}
	if cfg.Port != 45678 {
		t.Errorf("Expected port 45678 from env, got %d", cfg.Port)
	}
	if !cfg.PortFromEnv {
		t.Error("Expected PortFromEnv true when BEADS_DOLT_SERVER_PORT is set")
	}
}

// TestDiscoverSource_EmbeddedWithoutPortRequiresServer verifies that an
// embedded-mode project with no exported server port reports ErrDoltRequired
// (so the caller starts a bt-owned server) instead of attaching to whatever
// happens to be listening on the default port.
func TestDiscoverSource_EmbeddedWithoutPortRequiresServer(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}
	metadata := `{"backend":"dolt","dolt_mode":"embedded","dolt_database":"beads"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), []byte(metadata), 0644); err != nil {
		t.Fatal(err)
	}

	// Ensure no inherited env port makes this look like a started server.
	t.Setenv("BEADS_DOLT_SERVER_PORT", "")
	t.Setenv("BT_DOLT_PORT", "")

	_, err := DiscoverSource(DiscoveryOptions{BeadsDir: beadsDir})
	if !errors.Is(err, ErrDoltRequired) {
		t.Fatalf("Expected ErrDoltRequired for embedded mode without port, got %v", err)
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

// Note: pre-ADR-003 tests for discoverDoltSources, PriorityDolt vs JSONL,
// SelectBestSource_DoltWinsTiebreak, and BuildSelectionReason_Dolt were
// removed when their symbols were collapsed in Phase 2 (bt-okpn). The
// per-project Dolt resolution path is now exercised by TestDiscoverSource_*
// in source_test.go.
