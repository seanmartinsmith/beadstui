package search

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"

	"github.com/Dicklesworthstone/beads_viewer/pkg/util/topk"
)

const (
	vectorIndexMagic   = "BVVI"
	vectorIndexVersion = uint16(1)
)

type ContentHash [32]byte

func ComputeContentHash(text string) ContentHash {
	return sha256.Sum256([]byte(text))
}

func (h ContentHash) Hex() string {
	return hex.EncodeToString(h[:])
}

func ParseContentHashHex(s string) (ContentHash, error) {
	var out ContentHash
	if len(s) != 64 {
		return out, fmt.Errorf("invalid content hash length: %d", len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return out, fmt.Errorf("decode content hash: %w", err)
	}
	copy(out[:], b)
	return out, nil
}

type VectorEntry struct {
	ContentHash ContentHash
	Vector      []float32
}

type VectorIndex struct {
	Dim int

	mu       sync.RWMutex
	entries  map[string]VectorEntry
	idsCache []string
	idsDirty bool
}

func NewVectorIndex(dim int) *VectorIndex {
	if dim <= 0 {
		dim = DefaultEmbeddingDim
	}
	return &VectorIndex{
		Dim:      dim,
		entries:  make(map[string]VectorEntry),
		idsDirty: true,
	}
}

func LoadVectorIndex(path string) (*VectorIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	r := bufio.NewReader(f)

	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if string(magic[:]) != vectorIndexMagic {
		return nil, fmt.Errorf("invalid magic %q", string(magic[:]))
	}

	var version uint16
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	if version != vectorIndexVersion {
		return nil, fmt.Errorf("unsupported version %d", version)
	}

	// Reserved uint16
	var _reserved uint16
	if err := binary.Read(r, binary.LittleEndian, &_reserved); err != nil {
		return nil, fmt.Errorf("read reserved: %w", err)
	}

	var dimU32 uint32
	var count uint32
	if err := binary.Read(r, binary.LittleEndian, &dimU32); err != nil {
		return nil, fmt.Errorf("read dim: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return nil, fmt.Errorf("read count: %w", err)
	}
	if dimU32 == 0 {
		return nil, fmt.Errorf("invalid dim 0")
	}

	idx := NewVectorIndex(int(dimU32))
	for i := uint32(0); i < count; i++ {
		var idLen uint16
		if err := binary.Read(r, binary.LittleEndian, &idLen); err != nil {
			return nil, fmt.Errorf("read id len: %w", err)
		}
		if idLen == 0 {
			return nil, fmt.Errorf("empty issue id")
		}

		idBytes := make([]byte, idLen)
		if _, err := io.ReadFull(r, idBytes); err != nil {
			return nil, fmt.Errorf("read id: %w", err)
		}
		issueID := string(idBytes)

		var ch ContentHash
		if _, err := io.ReadFull(r, ch[:]); err != nil {
			return nil, fmt.Errorf("read content hash: %w", err)
		}

		vec := make([]float32, idx.Dim)
		for j := 0; j < idx.Dim; j++ {
			var bits uint32
			if err := binary.Read(r, binary.LittleEndian, &bits); err != nil {
				return nil, fmt.Errorf("read vector: %w", err)
			}
			vec[j] = math.Float32frombits(bits)
		}

		if err := idx.Upsert(issueID, ch, vec); err != nil {
			return nil, err
		}
	}

	return idx, nil
}

func (idx *VectorIndex) Save(path string) error {
	// Acquire sorted IDs before locking to avoid deadlock (sortedIDs needs Write lock if dirty)
	ids := idx.sortedIDs()

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "bvvi-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	w := bufio.NewWriter(tmp)

	if _, err := w.WriteString(vectorIndexMagic); err != nil {
		return fmt.Errorf("write magic: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, vectorIndexVersion); err != nil {
		return fmt.Errorf("write version: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(0)); err != nil {
		return fmt.Errorf("write reserved: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(idx.Dim)); err != nil {
		return fmt.Errorf("write dim: %w", err)
	}

	if err := binary.Write(w, binary.LittleEndian, uint32(len(ids))); err != nil {
		return fmt.Errorf("write count: %w", err)
	}

	for _, issueID := range ids {
		entry, ok := idx.entries[issueID]
		if !ok {
			continue
		}
		if len(issueID) > math.MaxUint16 {
			return fmt.Errorf("issue id too long: %d", len(issueID))
		}

		if err := binary.Write(w, binary.LittleEndian, uint16(len(issueID))); err != nil {
			return fmt.Errorf("write id len: %w", err)
		}
		if _, err := w.WriteString(issueID); err != nil {
			return fmt.Errorf("write id: %w", err)
		}
		if _, err := w.Write(entry.ContentHash[:]); err != nil {
			return fmt.Errorf("write content hash: %w", err)
		}
		if len(entry.Vector) != idx.Dim {
			return fmt.Errorf("vector dim mismatch for %s: %d != %d", issueID, len(entry.Vector), idx.Dim)
		}
		for _, v := range entry.Vector {
			if err := binary.Write(w, binary.LittleEndian, math.Float32bits(v)); err != nil {
				return fmt.Errorf("write vector: %w", err)
			}
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		// os.Rename doesn't replace existing files on Windows. Since the index is deterministic
		// and can be rebuilt, fall back to removing the destination and retrying.
		if runtime.GOOS == "windows" {
			if _, statErr := os.Stat(path); statErr == nil {
				if rmErr := os.Remove(path); rmErr != nil {
					return fmt.Errorf("remove existing index: %w", rmErr)
				}
				if err2 := os.Rename(tmpPath, path); err2 == nil {
					return nil
				} else {
					return fmt.Errorf("rename: %w", err2)
				}
			}
		}
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func (idx *VectorIndex) Upsert(issueID string, hash ContentHash, vec []float32) error {
	if issueID == "" {
		return fmt.Errorf("issue id cannot be empty")
	}
	if len(vec) != idx.Dim {
		return fmt.Errorf("vector dim mismatch: %d != %d", len(vec), idx.Dim)
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	_, exists := idx.entries[issueID]
	cp := make([]float32, len(vec))
	copy(cp, vec)
	idx.entries[issueID] = VectorEntry{
		ContentHash: hash,
		Vector:      cp,
	}
	if !exists {
		idx.idsDirty = true
	}
	return nil
}

func (idx *VectorIndex) Remove(issueID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, ok := idx.entries[issueID]; !ok {
		return
	}
	delete(idx.entries, issueID)
	idx.idsDirty = true
}

func (idx *VectorIndex) Get(issueID string) (VectorEntry, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	e, ok := idx.entries[issueID]
	return e, ok
}

func (idx *VectorIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}

func (idx *VectorIndex) sortedIDs() []string {
	idx.mu.RLock()
	if !idx.idsDirty && idx.idsCache != nil {
		defer idx.mu.RUnlock()
		return idx.idsCache
	}
	idx.mu.RUnlock()

	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Double check after acquiring write lock
	if !idx.idsDirty && idx.idsCache != nil {
		return idx.idsCache
	}

	ids := make([]string, 0, len(idx.entries))
	for id := range idx.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	idx.idsCache = ids
	idx.idsDirty = false
	return ids
}

type SearchResult struct {
	IssueID string  `json:"issue_id"`
	Score   float64 `json:"score"`
}

func (idx *VectorIndex) SearchTopK(query []float32, k int) ([]SearchResult, error) {
	if k <= 0 {
		return nil, nil
	}
	if len(query) != idx.Dim {
		return nil, fmt.Errorf("query dim mismatch: %d != %d", len(query), idx.Dim)
	}

	// sortedIDs now handles its own locking safely
	ids := idx.sortedIDs()

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Use heap-based top-K collector: O(n log k) vs O(nk) for linear insert
	collector := topk.New[SearchResult](k, func(a, b SearchResult) bool {
		return a.IssueID < b.IssueID // Deterministic tie-breaking: smaller ID wins
	})

	for _, issueID := range ids {
		entry, ok := idx.entries[issueID]
		if !ok {
			// This can happen if the issue was removed concurrently between sortedIDs() and RLock()
			continue
		}
		score := dotFloat32(query, entry.Vector)
		collector.Add(SearchResult{IssueID: issueID, Score: score}, score)
	}

	return collector.Results(), nil
}

func dotFloat32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var sum float64
	for i := range a {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}
