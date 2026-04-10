package search

import (
	"context"
	"testing"
)

// =============================================================================
// Embedder Interface Contract Tests
// =============================================================================

// TestEmbedderInterface verifies that HashEmbedder properly implements Embedder.
func TestEmbedderInterface(t *testing.T) {
	// Compile-time check that HashEmbedder implements Embedder
	var _ Embedder = (*HashEmbedder)(nil)

	// Create via factory to ensure interface is satisfied at runtime
	e, err := NewEmbedderFromConfig(EmbeddingConfig{Provider: ProviderHash, Dim: 128})
	if err != nil {
		t.Fatalf("NewEmbedderFromConfig() error = %v", err)
	}

	// Test Provider() method
	if e.Provider() != ProviderHash {
		t.Errorf("Provider() = %q, want %q", e.Provider(), ProviderHash)
	}

	// Test Dim() method
	if e.Dim() != 128 {
		t.Errorf("Dim() = %d, want 128", e.Dim())
	}

	// Test Embed() method
	ctx := context.Background()
	vectors, err := e.Embed(ctx, []string{"test text"})
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if len(vectors) != 1 {
		t.Errorf("Embed() returned %d vectors, want 1", len(vectors))
	}
	if len(vectors[0]) != 128 {
		t.Errorf("Embed()[0] has dimension %d, want 128", len(vectors[0]))
	}
}

// TestEmbedderInterfaceContract tests the behavioral contract of the Embedder interface.
func TestEmbedderInterfaceContract(t *testing.T) {
	e := NewHashEmbedder(64)

	t.Run("Dim matches configured dimension", func(t *testing.T) {
		if e.Dim() != 64 {
			t.Errorf("Dim() = %d, want 64", e.Dim())
		}
	})

	t.Run("Provider returns correct identifier", func(t *testing.T) {
		if e.Provider() != ProviderHash {
			t.Errorf("Provider() = %q, want %q", e.Provider(), ProviderHash)
		}
	})

	t.Run("Embed returns vectors matching Dim", func(t *testing.T) {
		ctx := context.Background()
		texts := []string{"hello", "world", "test"}
		vectors, err := e.Embed(ctx, texts)
		if err != nil {
			t.Fatalf("Embed() error = %v", err)
		}
		if len(vectors) != len(texts) {
			t.Errorf("Embed() returned %d vectors, want %d", len(vectors), len(texts))
		}
		for i, v := range vectors {
			if len(v) != 64 {
				t.Errorf("vectors[%d] has dimension %d, want 64", i, len(v))
			}
		}
	})

	t.Run("Embed with empty input returns empty slice", func(t *testing.T) {
		ctx := context.Background()
		vectors, err := e.Embed(ctx, []string{})
		if err != nil {
			t.Fatalf("Embed() error = %v", err)
		}
		if len(vectors) != 0 {
			t.Errorf("Embed() returned %d vectors, want 0", len(vectors))
		}
	})

	t.Run("Embed with nil input returns empty slice", func(t *testing.T) {
		ctx := context.Background()
		vectors, err := e.Embed(ctx, nil)
		if err != nil {
			t.Fatalf("Embed() error = %v", err)
		}
		if len(vectors) != 0 {
			t.Errorf("Embed() returned %d vectors, want 0", len(vectors))
		}
	})
}

// =============================================================================
// EmbeddingConfig Tests
// =============================================================================

func TestEmbeddingConfig_Fields(t *testing.T) {
	cfg := EmbeddingConfig{
		Provider: ProviderOpenAI,
		Model:    "text-embedding-3-small",
		Dim:      1536,
	}

	if cfg.Provider != ProviderOpenAI {
		t.Errorf("Provider = %q, want %q", cfg.Provider, ProviderOpenAI)
	}
	if cfg.Model != "text-embedding-3-small" {
		t.Errorf("Model = %q, want %q", cfg.Model, "text-embedding-3-small")
	}
	if cfg.Dim != 1536 {
		t.Errorf("Dim = %d, want 1536", cfg.Dim)
	}
}

func TestEmbeddingConfig_ZeroValue(t *testing.T) {
	var cfg EmbeddingConfig

	// Zero value should be empty/zero
	if cfg.Provider != "" {
		t.Errorf("zero value Provider = %q, want empty", cfg.Provider)
	}
	if cfg.Model != "" {
		t.Errorf("zero value Model = %q, want empty", cfg.Model)
	}
	if cfg.Dim != 0 {
		t.Errorf("zero value Dim = %d, want 0", cfg.Dim)
	}

	// Normalized should set default dim
	normalized := cfg.Normalized()
	if normalized.Dim != DefaultEmbeddingDim {
		t.Errorf("Normalized().Dim = %d, want %d", normalized.Dim, DefaultEmbeddingDim)
	}
}

func TestEmbeddingConfig_Normalized_DoesNotMutate(t *testing.T) {
	cfg := EmbeddingConfig{Provider: ProviderHash, Model: "test", Dim: 0}
	_ = cfg.Normalized()

	// Original should be unchanged
	if cfg.Dim != 0 {
		t.Errorf("original Dim changed to %d, want 0", cfg.Dim)
	}
}

func TestEmbeddingConfig_Normalized_PreservesOtherFields(t *testing.T) {
	cfg := EmbeddingConfig{
		Provider: ProviderOpenAI,
		Model:    "custom-model",
		Dim:      0,
	}
	normalized := cfg.Normalized()

	if normalized.Provider != ProviderOpenAI {
		t.Errorf("Normalized() changed Provider to %q", normalized.Provider)
	}
	if normalized.Model != "custom-model" {
		t.Errorf("Normalized() changed Model to %q", normalized.Model)
	}
}

// =============================================================================
// Provider Type Tests
// =============================================================================

func TestProvider_StringConversion(t *testing.T) {
	tests := []struct {
		provider Provider
		expected string
	}{
		{ProviderHash, "hash"},
		{ProviderPythonSentenceTransformers, "python-sentence-transformers"},
		{ProviderOpenAI, "openai"},
		{Provider("custom"), "custom"},
		{Provider(""), ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			if string(tt.provider) != tt.expected {
				t.Errorf("string(Provider) = %q, want %q", string(tt.provider), tt.expected)
			}
		})
	}
}

func TestProvider_Comparison(t *testing.T) {
	// Providers should be comparable
	p1 := ProviderHash
	p2 := Provider("hash")
	p3 := ProviderOpenAI

	if p1 != p2 {
		t.Error("ProviderHash should equal Provider(\"hash\")")
	}
	if p1 == p3 {
		t.Error("ProviderHash should not equal ProviderOpenAI")
	}
}

// =============================================================================
// Constants Validation
// =============================================================================

func TestDefaultDimValue(t *testing.T) {
	// 384 is the standard dimension for sentence-transformers models like all-MiniLM-L6-v2
	if DefaultEmbeddingDim != 384 {
		t.Errorf("DefaultEmbeddingDim = %d, expected 384 for sentence-transformers compatibility", DefaultEmbeddingDim)
	}
}

func TestEnvVarNaming(t *testing.T) {
	// All env vars should have BV_ prefix
	envVars := []string{EnvSemanticEmbedder, EnvSemanticModel, EnvSemanticDim}
	for _, env := range envVars {
		if len(env) < 3 || env[:3] != "BV_" {
			t.Errorf("Environment variable %q should have BV_ prefix", env)
		}
	}
}
