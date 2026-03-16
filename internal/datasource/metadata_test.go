package datasource

import (
	"os"
	"path/filepath"
	"testing"
)

// setupDoltMetadata creates a minimal .beads directory with metadata.json declaring backend=dolt.
func setupDoltMetadata(t *testing.T) string {
	t.Helper()
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}
	meta := `{"backend":"dolt","dolt_database":"beads"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), []byte(meta), 0644); err != nil {
		t.Fatal(err)
	}
	return beadsDir
}

func TestReadDoltConfig_DefaultPort(t *testing.T) {
	beadsDir := setupDoltMetadata(t)

	cfg, ok := ReadDoltConfig(beadsDir)
	if !ok {
		t.Fatal("expected ok=true for Dolt backend")
	}
	if cfg.Port != 3307 {
		t.Errorf("expected default port 3307, got %d", cfg.Port)
	}
}

func TestReadDoltConfig_PortFileOverridesConfig(t *testing.T) {
	beadsDir := setupDoltMetadata(t)

	// Write a config.yaml with port 13307
	doltDir := filepath.Join(beadsDir, "dolt")
	if err := os.MkdirAll(doltDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(doltDir, "config.yaml"), []byte("listener:\n  port: 13307\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a port file with 55555
	if err := os.WriteFile(filepath.Join(beadsDir, "dolt-server.port"), []byte("55555"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, ok := ReadDoltConfig(beadsDir)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if cfg.Port != 55555 {
		t.Errorf("expected port file to win (55555), got %d", cfg.Port)
	}
}

func TestReadDoltConfig_EnvVarOverridesAll(t *testing.T) {
	beadsDir := setupDoltMetadata(t)

	// Write port file to prove env var wins over it
	if err := os.WriteFile(filepath.Join(beadsDir, "dolt-server.port"), []byte("55555"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("BEADS_DOLT_SERVER_PORT", "9999")

	cfg, ok := ReadDoltConfig(beadsDir)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if cfg.Port != 9999 {
		t.Errorf("expected BEADS_DOLT_SERVER_PORT to win (9999), got %d", cfg.Port)
	}
}

func TestReadDoltConfig_BTEnvVarOverridesPortFile(t *testing.T) {
	beadsDir := setupDoltMetadata(t)

	// Write port file
	if err := os.WriteFile(filepath.Join(beadsDir, "dolt-server.port"), []byte("55555"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("BT_DOLT_PORT", "7777")

	cfg, ok := ReadDoltConfig(beadsDir)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if cfg.Port != 7777 {
		t.Errorf("expected BT_DOLT_PORT to win (7777), got %d", cfg.Port)
	}
}

func TestReadDoltConfig_BeadsEnvTrumpsBTEnv(t *testing.T) {
	beadsDir := setupDoltMetadata(t)

	t.Setenv("BEADS_DOLT_SERVER_PORT", "9999")
	t.Setenv("BT_DOLT_PORT", "7777")

	cfg, ok := ReadDoltConfig(beadsDir)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if cfg.Port != 9999 {
		t.Errorf("expected BEADS_DOLT_SERVER_PORT (9999) to beat BT_DOLT_PORT (7777), got %d", cfg.Port)
	}
}

func TestReadDoltConfig_NonDoltBackend(t *testing.T) {
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}
	meta := `{"backend":"jsonl"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), []byte(meta), 0644); err != nil {
		t.Fatal(err)
	}

	_, ok := ReadDoltConfig(beadsDir)
	if ok {
		t.Fatal("expected ok=false for non-dolt backend")
	}
}
