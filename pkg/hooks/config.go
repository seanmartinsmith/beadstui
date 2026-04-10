// Package hooks provides a hook system for bv export automation.
// Hooks are configured via .bv/hooks.yaml and run at specific points
// in the export pipeline (pre-export, post-export).
package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// HookPhase represents when a hook runs
type HookPhase string

const (
	// PreExport runs before export generation. Failure cancels export.
	PreExport HookPhase = "pre-export"
	// PostExport runs after export is written. Failure is logged but doesn't break export.
	PostExport HookPhase = "post-export"
)

// Hook defines a single hook configuration
type Hook struct {
	Name    string            `yaml:"name" json:"name"`                             // Human-readable name
	Command string            `yaml:"command" json:"command"`                       // Shell command to run
	Timeout time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"`   // Execution timeout (default: 30s)
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`           // Additional environment variables
	OnError string            `yaml:"on_error,omitempty" json:"on_error,omitempty"` // "fail" (default for pre) or "continue" (default for post)
}

// Config holds all hook configurations
type Config struct {
	Hooks HooksByPhase `yaml:"hooks" json:"hooks"`
}

// HooksByPhase organizes hooks by their execution phase
type HooksByPhase struct {
	PreExport  []Hook `yaml:"pre-export,omitempty" json:"pre-export,omitempty"`
	PostExport []Hook `yaml:"post-export,omitempty" json:"post-export,omitempty"`
}

// ExportContext contains information passed to hooks via environment variables
type ExportContext struct {
	ExportPath   string    // BV_EXPORT_PATH: Output file path
	ExportFormat string    // BV_EXPORT_FORMAT: 'markdown' or 'json'
	IssueCount   int       // BV_ISSUE_COUNT: Number of issues exported
	Timestamp    time.Time // BV_TIMESTAMP: Export timestamp (RFC3339)
}

// ToEnv converts export context to environment variables
func (c ExportContext) ToEnv() []string {
	return []string{
		fmt.Sprintf("BV_EXPORT_PATH=%s", c.ExportPath),
		fmt.Sprintf("BV_EXPORT_FORMAT=%s", c.ExportFormat),
		fmt.Sprintf("BV_ISSUE_COUNT=%d", c.IssueCount),
		fmt.Sprintf("BV_TIMESTAMP=%s", c.Timestamp.Format(time.RFC3339)),
	}
}

// DefaultTimeout is the default hook execution timeout
const DefaultTimeout = 30 * time.Second

// Loader loads hook configuration from .bv/hooks.yaml
type Loader struct {
	projectDir string
	config     *Config
	warnings   []string
}

// LoaderOption configures the loader
type LoaderOption func(*Loader)

// WithProjectDir sets the project directory (default: current directory)
func WithProjectDir(dir string) LoaderOption {
	return func(l *Loader) {
		l.projectDir = dir
	}
}

// NewLoader creates a new hook loader with options
func NewLoader(opts ...LoaderOption) *Loader {
	l := &Loader{}

	for _, opt := range opts {
		opt(l)
	}

	if l.projectDir == "" {
		l.projectDir, _ = os.Getwd()
	}

	return l
}

// Load loads hook configuration from .bv/hooks.yaml
func (l *Loader) Load() error {
	configPath := filepath.Join(l.projectDir, ".bv", "hooks.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file means no hooks - this is OK
			l.config = &Config{}
			return nil
		}
		return fmt.Errorf("reading hooks config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parsing %s: %w", configPath, err)
	}

	// Apply defaults and validate
	l.normalizeConfig(&config)

	l.config = &config
	return nil
}

// normalizeConfig applies defaults and validates hooks
func (l *Loader) normalizeConfig(config *Config) {
	config.Hooks.PreExport, l.warnings = normalizeHooks(config.Hooks.PreExport, PreExport, l.warnings)
	config.Hooks.PostExport, l.warnings = normalizeHooks(config.Hooks.PostExport, PostExport, l.warnings)
}

// normalizeHooks applies defaults, drops empty commands, and accumulates warnings.
func normalizeHooks(hooks []Hook, phase HookPhase, warnings []string) ([]Hook, []string) {
	var out []Hook
	for i := range hooks {
		hook := hooks[i]
		if strings.TrimSpace(hook.Command) == "" {
			warnings = append(warnings, fmt.Sprintf("%s hook %d has empty command; skipping", phase, i+1))
			continue
		}
		if hook.Timeout == 0 {
			hook.Timeout = DefaultTimeout
		}
		if hook.OnError == "" {
			if phase == PreExport {
				hook.OnError = "fail" // pre-export failures cancel export by default
			} else {
				hook.OnError = "continue" // post-export failures don't break export by default
			}
		}
		if hook.Name == "" {
			hook.Name = fmt.Sprintf("%s-%d", phase, i+1)
		}
		out = append(out, hook)
	}
	return out, warnings
}

// Config returns the loaded configuration (or empty if not loaded)
func (l *Loader) Config() *Config {
	if l.config == nil {
		return &Config{}
	}
	return l.config
}

// HasHooks returns true if any hooks are configured
func (l *Loader) HasHooks() bool {
	if l.config == nil {
		return false
	}
	return len(l.config.Hooks.PreExport) > 0 || len(l.config.Hooks.PostExport) > 0
}

// GetHooks returns hooks for a specific phase
func (l *Loader) GetHooks(phase HookPhase) []Hook {
	if l.config == nil {
		return nil
	}

	switch phase {
	case PreExport:
		return l.config.Hooks.PreExport
	case PostExport:
		return l.config.Hooks.PostExport
	default:
		return nil
	}
}

// Warnings returns any warnings from loading
func (l *Loader) Warnings() []string {
	return l.warnings
}

// LoadDefault creates a loader and loads with default settings
func LoadDefault() (*Loader, error) {
	loader := NewLoader()
	if err := loader.Load(); err != nil {
		return nil, err
	}
	return loader, nil
}

// UnmarshalYAML implements custom YAML unmarshalling for Duration
func (h *Hook) UnmarshalYAML(node *yaml.Node) error {
	// WARNING: This struct must match Hook definition exactly, except for Timeout which is string.
	// If you add a field to Hook, you MUST add it here too.
	type hookDTO struct {
		Name    string            `yaml:"name"`
		Command string            `yaml:"command"`
		Timeout string            `yaml:"timeout,omitempty"`
		Env     map[string]string `yaml:"env,omitempty"`
		OnError string            `yaml:"on_error,omitempty"`
	}

	var dto hookDTO
	if err := node.Decode(&dto); err != nil {
		return err
	}

	h.Name = dto.Name
	h.Command = dto.Command
	h.Env = dto.Env
	h.OnError = dto.OnError

	// Parse timeout
	if dto.Timeout != "" {
		d, err := time.ParseDuration(dto.Timeout)
		if err == nil {
			h.Timeout = d
		} else {
			// Fallback: try numeric value (assumed seconds)
			// This handles cases like "timeout: 30" which YAML decodes as string "30"
			// but time.ParseDuration rejects (missing unit).
			var seconds float64
			if _, scanErr := fmt.Sscanf(dto.Timeout, "%f", &seconds); scanErr == nil {
				h.Timeout = time.Duration(seconds * float64(time.Second))
			} else {
				return fmt.Errorf("invalid timeout %q: %w", dto.Timeout, err)
			}
		}
	}

	return nil
}
