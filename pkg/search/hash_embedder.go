package search

import (
	"context"
	"math"
	"unicode"
)

// HashEmbedder is a dependency-free embedding fallback.
// It uses feature hashing over tokens into a fixed-size dense vector.
type HashEmbedder struct {
	dim int
}

func NewHashEmbedder(dim int) *HashEmbedder {
	if dim <= 0 {
		dim = DefaultEmbeddingDim
	}
	return &HashEmbedder{dim: dim}
}

func (*HashEmbedder) Provider() Provider { return ProviderHash }
func (h *HashEmbedder) Dim() int         { return h.dim }

func (h *HashEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		vec := make([]float32, h.dim)
		hashEmbedInto(vec, text)
		normalizeL2(vec)
		out[i] = vec
	}
	return out, nil
}

func hashEmbedInto(vec []float32, text string) {
	if len(vec) == 0 {
		return
	}
	dim := uint64(len(vec))

	tokenStart := -1
	for idx, r := range text {
		isTokenChar := unicode.IsLetter(r) || unicode.IsDigit(r)
		if isTokenChar {
			if tokenStart == -1 {
				tokenStart = idx
			}
			continue
		}
		if tokenStart != -1 {
			addHashedToken(vec, dim, text[tokenStart:idx])
			tokenStart = -1
		}
	}
	if tokenStart != -1 {
		addHashedToken(vec, dim, text[tokenStart:])
	}
}

func addHashedToken(vec []float32, dim uint64, token string) {
	// FNV-1a 64-bit over UTF-8 bytes.
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	var h uint64 = offset64
	for i := 0; i < len(token); i++ {
		b := token[i]
		// Lowercase ASCII fast-path; unicode case folding isn't worth it for a fallback embedder.
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		h ^= uint64(b)
		h *= prime64
	}

	idx := int(h % dim)
	sign := float32(1.0)
	if (h>>63)&1 == 1 {
		sign = -1.0
	}
	vec[idx] += sign
}

func normalizeL2(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	if sum == 0 {
		return
	}
	scale := float32(1.0 / math.Sqrt(sum))
	for i := range vec {
		vec[i] *= scale
	}
}
