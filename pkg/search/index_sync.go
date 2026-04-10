package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultIndexPath returns the default semantic index path under the given project directory.
// The filename is keyed by provider+dim to avoid mixing incompatible embeddings.
func DefaultIndexPath(projectDir string, cfg EmbeddingConfig) string {
	cfg = cfg.Normalized()
	provider := cfg.Provider
	if provider == "" {
		provider = ProviderHash
	}
	safeProvider := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(string(provider))
	return filepath.Join(projectDir, ".bv", "semantic", fmt.Sprintf("index-%s-%d.bvvi", safeProvider, cfg.Dim))
}

type IndexSyncStats struct {
	Total    int `json:"total"`
	Added    int `json:"added"`
	Updated  int `json:"updated"`
	Removed  int `json:"removed"`
	Skipped  int `json:"skipped"`
	Embedded int `json:"embedded"`
}

func (s IndexSyncStats) Changed() bool {
	return s.Added+s.Updated+s.Removed > 0
}

// LoadOrNewVectorIndex loads an existing vector index if present, otherwise creates a new one.
// If loading fails due to corruption, it backs up the corrupt file and returns a new empty index.
func LoadOrNewVectorIndex(path string, dim int) (*VectorIndex, bool, error) {
	idx, err := LoadVectorIndex(path)
	if err == nil {
		return idx, true, nil
	}

	if os.IsNotExist(err) {
		return NewVectorIndex(dim), false, nil
	}

	// File exists but failed to load - likely corrupt
	// Attempt to back it up
	backupPath := path + ".corrupt-" + fmt.Sprintf("%d", time.Now().Unix())
	if renameErr := os.Rename(path, backupPath); renameErr == nil {
		// Successfully backed up, return new index
		// Note: We return error as nil here because we recovered, but the caller might want to know?
		// We'll return false for loaded, and nil for error, effectively resetting.
		// Ideally we'd log this, but this function is low-level.
		// Let's rely on the fact that 'loaded=false' implies we started fresh.
		return NewVectorIndex(dim), false, nil
	}

	// If rename failed (e.g. permissions), we return the original error
	return nil, false, fmt.Errorf("load vector index (and backup failed): %w", err)
}

// SyncVectorIndex updates idx to match docs using embedder, incrementally embedding only changed items.
//
// This is intended for offline, deterministic embedding providers. Callers should persist idx
// with (*VectorIndex).Save when desired.
func SyncVectorIndex(ctx context.Context, idx *VectorIndex, embedder Embedder, docs map[string]string, batchSize int) (IndexSyncStats, error) {
	var stats IndexSyncStats
	if idx == nil {
		return stats, fmt.Errorf("index cannot be nil")
	}
	if embedder == nil {
		return stats, fmt.Errorf("embedder cannot be nil")
	}
	if idx.Dim != embedder.Dim() {
		return stats, fmt.Errorf("index dim %d does not match embedder dim %d", idx.Dim, embedder.Dim())
	}
	if batchSize <= 0 {
		batchSize = 32
	}

	stats.Total = len(docs)

	// Remove stale IDs.
	docIDs := make(map[string]struct{}, len(docs))
	for id := range docs {
		docIDs[id] = struct{}{}
	}

	// Use sortedIDs to safely iterate over keys without holding lock or racing
	existingIDs := idx.sortedIDs()
	for _, issueID := range existingIDs {
		if _, ok := docIDs[issueID]; !ok {
			idx.Remove(issueID)
			stats.Removed++
		}
	}

	// Determine which docs need embedding (deterministic order).
	ids := make([]string, 0, len(docs))
	for id := range docs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	toEmbedIDs := make([]string, 0)
	toEmbedTexts := make([]string, 0)
	toEmbedHashes := make([]ContentHash, 0)

	for _, id := range ids {
		text := docs[id]
		ch := ComputeContentHash(text)
		existing, ok := idx.Get(id)
		if ok && existing.ContentHash == ch {
			stats.Skipped++
			continue
		}
		if ok {
			stats.Updated++
		} else {
			stats.Added++
		}
		toEmbedIDs = append(toEmbedIDs, id)
		toEmbedTexts = append(toEmbedTexts, text)
		toEmbedHashes = append(toEmbedHashes, ch)
	}

	// Embed in batches.
	for start := 0; start < len(toEmbedTexts); start += batchSize {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		end := start + batchSize
		if end > len(toEmbedTexts) {
			end = len(toEmbedTexts)
		}
		vecs, err := embedder.Embed(ctx, toEmbedTexts[start:end])
		if err != nil {
			return stats, err
		}
		if len(vecs) != end-start {
			return stats, fmt.Errorf("embedder returned %d vectors for %d texts", len(vecs), end-start)
		}
		for i, vec := range vecs {
			if err := idx.Upsert(toEmbedIDs[start+i], toEmbedHashes[start+i], vec); err != nil {
				return stats, err
			}
			stats.Embedded++
		}
	}

	return stats, nil
}
