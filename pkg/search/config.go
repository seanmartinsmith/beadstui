package search

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// EmbeddingConfigFromEnv reads semantic embedding configuration from environment variables.
//
// Supported variables:
//   - BV_SEMANTIC_EMBEDDER: embedding provider (default: "hash")
//   - BV_SEMANTIC_MODEL: model identifier (provider-specific, optional)
//   - BV_SEMANTIC_DIM: embedding dimension (default: DefaultEmbeddingDim)
func EmbeddingConfigFromEnv() EmbeddingConfig {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv(EnvSemanticEmbedder)))
	cfg := EmbeddingConfig{
		Provider: Provider(provider),
		Model:    strings.TrimSpace(os.Getenv(EnvSemanticModel)),
	}
	if dimStr := os.Getenv(EnvSemanticDim); dimStr != "" {
		if dim, err := strconv.Atoi(dimStr); err == nil {
			cfg.Dim = dim
		}
	}
	if cfg.Provider == "" {
		cfg.Provider = ProviderHash
	}
	return cfg.Normalized()
}

// NewEmbedderFromConfig constructs an Embedder for the given configuration.
func NewEmbedderFromConfig(cfg EmbeddingConfig) (Embedder, error) {
	cfg = cfg.Normalized()
	switch cfg.Provider {
	case "", ProviderHash:
		return NewHashEmbedder(cfg.Dim), nil
	case ProviderPythonSentenceTransformers:
		return nil, fmt.Errorf("semantic embedder %q not implemented (mvp placeholder); set %s=%q for deterministic fallback", cfg.Provider, EnvSemanticEmbedder, ProviderHash)
	case ProviderOpenAI:
		return nil, fmt.Errorf("semantic embedder %q not implemented (placeholder); set %s=%q for deterministic fallback", cfg.Provider, EnvSemanticEmbedder, ProviderHash)
	default:
		return nil, fmt.Errorf("unknown semantic embedder %q; expected %q", cfg.Provider, ProviderHash)
	}
}

// SearchMode defines the search ranking mode.
type SearchMode string

const (
	SearchModeText   SearchMode = "text"
	SearchModeHybrid SearchMode = "hybrid"
)

const (
	EnvSearchMode    = "BV_SEARCH_MODE"
	EnvSearchPreset  = "BV_SEARCH_PRESET"
	EnvSearchWeights = "BV_SEARCH_WEIGHTS"
)

// SearchConfig captures hybrid search configuration from env or flags.
type SearchConfig struct {
	Mode       SearchMode
	Preset     PresetName
	Weights    Weights
	HasWeights bool
}

// SearchConfigFromEnv reads hybrid search configuration from environment variables.
// Defaults: mode=text, preset=default.
func SearchConfigFromEnv() (SearchConfig, error) {
	cfg := SearchConfig{
		Mode:   SearchModeText,
		Preset: PresetDefault,
	}

	if mode := strings.TrimSpace(os.Getenv(EnvSearchMode)); mode != "" {
		switch SearchMode(strings.ToLower(mode)) {
		case SearchModeText, SearchModeHybrid:
			cfg.Mode = SearchMode(strings.ToLower(mode))
		default:
			return SearchConfig{}, fmt.Errorf("invalid %s: %q (expected text|hybrid)", EnvSearchMode, mode)
		}
	}

	if preset := strings.TrimSpace(os.Getenv(EnvSearchPreset)); preset != "" {
		name := PresetName(strings.ToLower(preset))
		if _, err := GetPreset(name); err != nil {
			return SearchConfig{}, err
		}
		cfg.Preset = name
	}

	if raw := strings.TrimSpace(os.Getenv(EnvSearchWeights)); raw != "" {
		weights, err := ParseWeightsJSON(raw)
		if err != nil {
			return SearchConfig{}, err
		}
		cfg.Weights = weights
		cfg.HasWeights = true
	}

	return cfg, nil
}

// ParseWeightsJSON parses a JSON string into Weights, enforcing required keys.
func ParseWeightsJSON(raw string) (Weights, error) {
	var payload map[string]float64
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return Weights{}, fmt.Errorf("invalid weights JSON: %w", err)
	}

	required := []string{"text", "pagerank", "status", "impact", "priority", "recency"}
	for _, key := range required {
		if _, ok := payload[key]; !ok {
			return Weights{}, fmt.Errorf("weights JSON missing %q", key)
		}
	}
	for key := range payload {
		if !isWeightKey(key) {
			return Weights{}, fmt.Errorf("weights JSON has unknown key %q", key)
		}
	}

	weights := Weights{
		TextRelevance: payload["text"],
		PageRank:      payload["pagerank"],
		Status:        payload["status"],
		Impact:        payload["impact"],
		Priority:      payload["priority"],
		Recency:       payload["recency"],
	}

	if err := weights.Validate(); err != nil {
		return Weights{}, err
	}

	return weights, nil
}

func isWeightKey(key string) bool {
	switch key {
	case "text", "pagerank", "status", "impact", "priority", "recency":
		return true
	default:
		return false
	}
}
