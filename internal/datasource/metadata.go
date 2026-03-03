package datasource

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	json "github.com/goccy/go-json"
	"gopkg.in/yaml.v3"
)

// DoltConfig holds the connection parameters for a running Dolt SQL server.
type DoltConfig struct {
	Host     string
	Port     int
	Database string
	User     string
}

// DSN returns a go-sql-driver/mysql data source name for this config.
func (c DoltConfig) DSN() string {
	return fmt.Sprintf("%s@tcp(%s:%d)/%s?parseTime=true&timeout=2s",
		c.User, c.Host, c.Port, c.Database)
}

// doltServerConfig mirrors the listener section of Dolt's config.yaml.
type doltServerConfig struct {
	Listener struct {
		Port int `yaml:"port"`
	} `yaml:"listener"`
}

// ReadDoltConfig reads .beads/metadata.json and (optionally) .beads/dolt/config.yaml
// to build a DoltConfig. Returns the config and true if the project uses Dolt,
// or a zero value and false otherwise.
func ReadDoltConfig(beadsDir string) (DoltConfig, bool) {
	metaPath := filepath.Join(beadsDir, "metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return DoltConfig{}, false
	}

	var meta struct {
		Backend      string `json:"backend"`
		DoltMode     string `json:"dolt_mode"`
		DoltDatabase string `json:"dolt_database"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return DoltConfig{}, false
	}

	if meta.Backend != "dolt" {
		return DoltConfig{}, false
	}

	cfg := DoltConfig{
		Host:     "127.0.0.1",
		Port:     3307, // Dolt default
		Database: meta.DoltDatabase,
		User:     "root",
	}

	if cfg.Database == "" {
		cfg.Database = "beads"
	}

	// Try to read the actual port from Dolt's config.yaml
	doltCfgPath := filepath.Join(beadsDir, "dolt", "config.yaml")
	if cfgData, err := os.ReadFile(doltCfgPath); err == nil {
		var sc doltServerConfig
		if err := yaml.Unmarshal(cfgData, &sc); err == nil && sc.Listener.Port > 0 {
			cfg.Port = sc.Listener.Port
		}
	}

	// The port file is written by `bd dolt start` and reflects the actual
	// running server port. It takes priority over config.yaml, which may
	// be stale (beads v0.57.0 switched to port 3307 but doesn't always
	// update the config.yaml).
	portPath := filepath.Join(beadsDir, "dolt-server.port")
	if portData, err := os.ReadFile(portPath); err == nil {
		if p, err := strconv.Atoi(strings.TrimSpace(string(portData))); err == nil && p > 0 {
			cfg.Port = p
		}
	}

	return cfg, true
}
