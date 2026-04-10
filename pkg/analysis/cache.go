package analysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

const (
	robotAnalysisDiskCacheVersion      = 1
	robotAnalysisDiskCacheFileName     = "analysis_cache.json"
	robotAnalysisDiskCacheDirName      = "bv"
	robotAnalysisDiskCacheMaxEntries   = 10
	robotAnalysisDiskCacheMaxAge       = 24 * time.Hour
	robotAnalysisDiskCacheMaxEntrySize = 10 << 20 // 10MB
)

// Cache holds cached analysis results keyed by data hash.
// Thread-safe for concurrent access.
type Cache struct {
	mu         sync.RWMutex
	dataHash   string
	stats      *GraphStats
	computedAt time.Time
	ttl        time.Duration
}

// DefaultCacheTTL is the default time-to-live for cached results.
const DefaultCacheTTL = 5 * time.Minute

// globalCache is the package-level cache instance.
var globalCache = &Cache{
	ttl: DefaultCacheTTL,
}

// GetGlobalCache returns the global cache instance.
func GetGlobalCache() *Cache {
	return globalCache
}

// NewCache creates a new cache with the specified TTL.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		ttl: ttl,
	}
}

// Get retrieves cached stats if the data hash matches and TTL hasn't expired.
// Returns (stats, true) on cache hit, (nil, false) on cache miss.
func (c *Cache) Get(issues []model.Issue) (*GraphStats, bool) {
	// Compute hash outside the lock (expensive operation)
	hash := ComputeDataHash(issues)
	return c.GetByHash(hash)
}

// GetByHash retrieves cached stats if the hash matches and TTL hasn't expired.
// This is more efficient when the hash has already been computed.
func (c *Cache) GetByHash(hash string) (*GraphStats, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.stats == nil {
		return nil, false
	}

	if hash == c.dataHash && time.Since(c.computedAt) < c.ttl {
		return c.stats, true
	}
	return nil, false
}

// Set stores analysis results in the cache.
func (c *Cache) Set(issues []model.Issue, stats *GraphStats) {
	// Compute hash outside the lock (expensive operation)
	hash := ComputeDataHash(issues)
	c.SetByHash(hash, stats)
}

// SetByHash stores analysis results with a pre-computed hash.
func (c *Cache) SetByHash(hash string, stats *GraphStats) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dataHash = hash
	c.stats = stats
	c.computedAt = time.Now()
}

// Invalidate clears the cache.
func (c *Cache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dataHash = ""
	c.stats = nil
	c.computedAt = time.Time{}
}

// SetTTL updates the cache TTL.
func (c *Cache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ttl = ttl
}

// Hash returns the current data hash, or empty string if no cached data.
func (c *Cache) Hash() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dataHash
}

// Stats returns cache statistics for debugging.
func (c *Cache) Stats() (hash string, age time.Duration, hasData bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.stats == nil {
		return "", 0, false
	}
	return c.dataHash, time.Since(c.computedAt), true
}

// ComputeDataHash generates a deterministic hash of issue data.
// The hash includes issue IDs, content hashes, and dependency relationships.
// Issues are sorted by ID to ensure consistent hashing regardless of input order.
func ComputeDataHash(issues []model.Issue) string {
	if len(issues) == 0 {
		return "empty"
	}

	// Sort issues by ID for deterministic ordering
	sorted := make([]model.Issue, len(issues))
	copy(sorted, issues)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	h := sha256.New()
	for _, issue := range sorted {
		// Core identity
		h.Write([]byte(issue.ID))
		h.Write([]byte{0})

		// Important scalar fields
		h.Write([]byte(issue.Title))
		h.Write([]byte{0})
		h.Write([]byte(issue.Description))
		h.Write([]byte{0})
		h.Write([]byte(issue.Notes))
		h.Write([]byte{0})
		h.Write([]byte(issue.Design))
		h.Write([]byte{0})
		h.Write([]byte(issue.AcceptanceCriteria))
		h.Write([]byte{0})
		h.Write([]byte(issue.Assignee))
		h.Write([]byte{0})
		h.Write([]byte(issue.SourceRepo))
		h.Write([]byte{0})
		if issue.ExternalRef != nil {
			h.Write([]byte(*issue.ExternalRef))
		}
		h.Write([]byte{0})

		h.Write([]byte(issue.Status))
		h.Write([]byte{0})
		h.Write([]byte(issue.IssueType))
		h.Write([]byte{0})

		// Numeric fields
		h.Write([]byte(strconv.Itoa(issue.Priority)))
		h.Write([]byte{0})
		if issue.EstimatedMinutes != nil {
			h.Write([]byte(strconv.Itoa(*issue.EstimatedMinutes)))
		}
		h.Write([]byte{0})
		h.Write([]byte(issue.CreatedAt.UTC().Format(time.RFC3339Nano)))
		h.Write([]byte{0})
		h.Write([]byte(issue.UpdatedAt.UTC().Format(time.RFC3339Nano)))
		h.Write([]byte{0})
		if issue.ClosedAt != nil {
			h.Write([]byte(issue.ClosedAt.UTC().Format(time.RFC3339Nano)))
		}
		h.Write([]byte{0})

		// Labels (sorted for determinism)
		if len(issue.Labels) > 0 {
			labels := append([]string(nil), issue.Labels...)
			sort.Strings(labels)
			for _, lbl := range labels {
				h.Write([]byte(lbl))
				h.Write([]byte{0})
			}
		}
		h.Write([]byte{0})

		// Dependencies (sorted)
		if len(issue.Dependencies) > 0 {
			type depKey struct {
				dependsOn string
				depType   string
				createdAt string
				createdBy string
			}
			deps := make([]depKey, 0, len(issue.Dependencies))
			for _, dep := range issue.Dependencies {
				if dep == nil {
					continue
				}
				deps = append(deps, depKey{
					dependsOn: dep.DependsOnID,
					depType:   string(dep.Type),
					createdAt: dep.CreatedAt.UTC().Format(time.RFC3339Nano),
					createdBy: dep.CreatedBy,
				})
			}
			sort.Slice(deps, func(i, j int) bool {
				if deps[i].dependsOn != deps[j].dependsOn {
					return deps[i].dependsOn < deps[j].dependsOn
				}
				if deps[i].depType != deps[j].depType {
					return deps[i].depType < deps[j].depType
				}
				if deps[i].createdAt != deps[j].createdAt {
					return deps[i].createdAt < deps[j].createdAt
				}
				return deps[i].createdBy < deps[j].createdBy
			})
			for _, dep := range deps {
				h.Write([]byte(dep.dependsOn))
				h.Write([]byte{0})
				h.Write([]byte(dep.depType))
				h.Write([]byte{0})
				h.Write([]byte(dep.createdAt))
				h.Write([]byte{0})
				h.Write([]byte(dep.createdBy))
				h.Write([]byte{0})
			}
		}
		h.Write([]byte{0})

		// Comments (sorted)
		if len(issue.Comments) > 0 {
			type commentKey struct {
				id        string
				author    string
				text      string
				createdAt string
			}
			comments := make([]commentKey, 0, len(issue.Comments))
			for _, comment := range issue.Comments {
				if comment == nil {
					continue
				}
				comments = append(comments, commentKey{
					id:        strconv.FormatInt(comment.ID, 10),
					author:    comment.Author,
					text:      comment.Text,
					createdAt: comment.CreatedAt.UTC().Format(time.RFC3339Nano),
				})
			}
			sort.Slice(comments, func(i, j int) bool {
				if comments[i].id != comments[j].id {
					return comments[i].id < comments[j].id
				}
				if comments[i].createdAt != comments[j].createdAt {
					return comments[i].createdAt < comments[j].createdAt
				}
				if comments[i].author != comments[j].author {
					return comments[i].author < comments[j].author
				}
				return comments[i].text < comments[j].text
			})
			for _, comment := range comments {
				h.Write([]byte(comment.id))
				h.Write([]byte{0})
				h.Write([]byte(comment.author))
				h.Write([]byte{0})
				h.Write([]byte(comment.text))
				h.Write([]byte{0})
				h.Write([]byte(comment.createdAt))
				h.Write([]byte{0})
			}
		}

		h.Write([]byte{1}) // issue separator
	}

	return hex.EncodeToString(h.Sum(nil))[:16] // Use first 16 chars for brevity
}

// IssueFingerprint represents a per-issue hash split across content and dependencies.
// It supports fast diffing between snapshots without a full rebuild.
type IssueFingerprint struct {
	ID             string
	ContentHash    string
	DependencyHash string
}

// IssueDiff captures a per-issue diff between two snapshots.
type IssueDiff struct {
	Added             []string
	Removed           []string
	Modified          []string
	ContentChanged    []string
	DependencyChanged []string
	Unchanged         []string
}

// ComputeIssueFingerprint returns the fingerprint for a single issue.
func ComputeIssueFingerprint(issue model.Issue) IssueFingerprint {
	return IssueFingerprint{
		ID:             issue.ID,
		ContentHash:    computeIssueContentHash(issue),
		DependencyHash: computeIssueDependencyHash(issue),
	}
}

// ComputeIssueDiff compares old and new issue slices and returns an IssueDiff.
func ComputeIssueDiff(oldIssues, newIssues []model.Issue) IssueDiff {
	oldFP := make(map[string]IssueFingerprint, len(oldIssues))
	for i := range oldIssues {
		fp := ComputeIssueFingerprint(oldIssues[i])
		oldFP[fp.ID] = fp
	}
	newFP := make(map[string]IssueFingerprint, len(newIssues))
	for i := range newIssues {
		fp := ComputeIssueFingerprint(newIssues[i])
		newFP[fp.ID] = fp
	}

	var diff IssueDiff
	for id, newIssue := range newFP {
		oldIssue, exists := oldFP[id]
		if !exists {
			diff.Added = append(diff.Added, id)
			continue
		}
		contentChanged := oldIssue.ContentHash != newIssue.ContentHash
		dependencyChanged := oldIssue.DependencyHash != newIssue.DependencyHash
		if contentChanged || dependencyChanged {
			diff.Modified = append(diff.Modified, id)
			if contentChanged {
				diff.ContentChanged = append(diff.ContentChanged, id)
			}
			if dependencyChanged {
				diff.DependencyChanged = append(diff.DependencyChanged, id)
			}
			continue
		}
		diff.Unchanged = append(diff.Unchanged, id)
	}

	for id := range oldFP {
		if _, exists := newFP[id]; !exists {
			diff.Removed = append(diff.Removed, id)
		}
	}

	sort.Strings(diff.Added)
	sort.Strings(diff.Removed)
	sort.Strings(diff.Modified)
	sort.Strings(diff.ContentChanged)
	sort.Strings(diff.DependencyChanged)
	sort.Strings(diff.Unchanged)
	return diff
}

func computeIssueContentHash(issue model.Issue) string {
	h := sha256.New()

	writeStringHash(h, issue.Title)
	writeStringHash(h, issue.Description)
	writeStringHash(h, issue.Design)
	writeStringHash(h, issue.AcceptanceCriteria)
	writeStringHash(h, issue.Notes)
	writeStringHash(h, issue.Assignee)
	writeStringHash(h, issue.SourceRepo)
	writeStringPtrHash(h, issue.ExternalRef)

	writeStringHash(h, string(issue.Status))
	writeStringHash(h, string(issue.IssueType))
	writeIntHash(h, issue.Priority)
	writeIntPtrHash(h, issue.EstimatedMinutes)
	writeTimeHash(h, issue.CreatedAt)
	writeTimeHash(h, issue.UpdatedAt)
	writeTimePtrHash(h, issue.DueDate)
	writeTimePtrHash(h, issue.ClosedAt)

	writeIntHash(h, issue.CompactionLevel)
	writeTimePtrHash(h, issue.CompactedAt)
	writeStringPtrHash(h, issue.CompactedAtCommit)
	writeIntHash(h, issue.OriginalSize)

	if len(issue.Labels) > 0 {
		labels := append([]string(nil), issue.Labels...)
		sort.Strings(labels)
		for _, label := range labels {
			writeStringHash(h, label)
		}
	}
	writeStringHash(h, "")

	if len(issue.Comments) > 0 {
		comments := make([]*model.Comment, 0, len(issue.Comments))
		for _, comment := range issue.Comments {
			if comment != nil {
				comments = append(comments, comment)
			}
		}
		sort.Slice(comments, func(i, j int) bool {
			if comments[i].ID != comments[j].ID {
				return comments[i].ID < comments[j].ID
			}
			return comments[i].CreatedAt.Before(comments[j].CreatedAt)
		})
		for _, comment := range comments {
			writeInt64Hash(h, comment.ID)
			writeStringHash(h, comment.IssueID)
			writeStringHash(h, comment.Author)
			writeStringHash(h, comment.Text)
			writeTimeHash(h, comment.CreatedAt)
		}
	}
	writeStringHash(h, "")

	return hex.EncodeToString(h.Sum(nil))[:16]
}

func computeIssueDependencyHash(issue model.Issue) string {
	if len(issue.Dependencies) == 0 {
		return "none"
	}
	type depKey struct {
		dependsOn string
		depType   string
		createdAt string
		createdBy string
	}
	deps := make([]depKey, 0, len(issue.Dependencies))
	for _, dep := range issue.Dependencies {
		if dep == nil {
			continue
		}
		deps = append(deps, depKey{
			dependsOn: dep.DependsOnID,
			depType:   string(dep.Type),
			createdAt: dep.CreatedAt.UTC().Format(time.RFC3339Nano),
			createdBy: dep.CreatedBy,
		})
	}
	sort.Slice(deps, func(i, j int) bool {
		if deps[i].dependsOn != deps[j].dependsOn {
			return deps[i].dependsOn < deps[j].dependsOn
		}
		if deps[i].depType != deps[j].depType {
			return deps[i].depType < deps[j].depType
		}
		if deps[i].createdAt != deps[j].createdAt {
			return deps[i].createdAt < deps[j].createdAt
		}
		return deps[i].createdBy < deps[j].createdBy
	})

	h := sha256.New()
	for _, dep := range deps {
		writeStringHash(h, dep.dependsOn)
		writeStringHash(h, dep.depType)
		writeStringHash(h, dep.createdAt)
		writeStringHash(h, dep.createdBy)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func writeStringHash(w io.Writer, v string) {
	if v != "" {
		_, _ = io.WriteString(w, v)
	}
	_, _ = w.Write([]byte{0})
}

func writeStringPtrHash(w io.Writer, v *string) {
	if v != nil {
		_, _ = io.WriteString(w, *v)
	}
	_, _ = w.Write([]byte{0})
}

func writeIntHash(w io.Writer, v int) {
	_, _ = io.WriteString(w, strconv.Itoa(v))
	_, _ = w.Write([]byte{0})
}

func writeIntPtrHash(w io.Writer, v *int) {
	if v != nil {
		_, _ = io.WriteString(w, strconv.Itoa(*v))
	}
	_, _ = w.Write([]byte{0})
}

func writeInt64Hash(w io.Writer, v int64) {
	_, _ = io.WriteString(w, strconv.FormatInt(v, 10))
	_, _ = w.Write([]byte{0})
}

func writeTimeHash(w io.Writer, t time.Time) {
	if !t.IsZero() {
		_, _ = io.WriteString(w, t.UTC().Format(time.RFC3339Nano))
	}
	_, _ = w.Write([]byte{0})
}

func writeTimePtrHash(w io.Writer, t *time.Time) {
	if t != nil {
		_, _ = io.WriteString(w, t.UTC().Format(time.RFC3339Nano))
	}
	_, _ = w.Write([]byte{0})
}

// ComputeConfigHash generates a deterministic hash of the analysis configuration.
func ComputeConfigHash(config *AnalysisConfig) string {
	if config == nil {
		return "dynamic"
	}
	h := sha256.New()
	// Using %#v is stable enough for configuration struct
	h.Write([]byte(fmt.Sprintf("%#v", *config)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// CachedAnalyzer wraps an Analyzer with caching support.
type CachedAnalyzer struct {
	*Analyzer
	cache      *Cache
	issues     []model.Issue
	dataHash   string // Hash of the issue data
	configHash string // Hash of the configuration
	cacheHit   bool   // Set by AnalyzeAsync to track if it was a cache hit
}

// NewCachedAnalyzer creates an analyzer that checks the cache before computing.
// The Analyzer is always created because it may be needed for GenerateRecommendations
// even on cache hit. Creating the Analyzer (graph building) is O(V+E) which is fast;
// the expensive part is the analysis itself, which we skip on cache hit.
func NewCachedAnalyzer(issues []model.Issue, cache *Cache) *CachedAnalyzer {
	if cache == nil {
		cache = globalCache
	}
	return &CachedAnalyzer{
		Analyzer:   NewAnalyzer(issues),
		cache:      cache,
		issues:     issues,
		dataHash:   ComputeDataHash(issues),
		configHash: "dynamic",
	}
}

// SetConfig updates the analyzer configuration and the configuration hash.
func (ca *CachedAnalyzer) SetConfig(config *AnalysisConfig) {
	ca.Analyzer.SetConfig(config)
	ca.configHash = ComputeConfigHash(config)
}

// AnalyzeAsync returns cached stats if available, otherwise computes and caches.
func (ca *CachedAnalyzer) AnalyzeAsync(ctx context.Context) *GraphStats {
	// Combined key: dataHash|configHash
	fullHash := ca.dataHash + "|" + ca.configHash

	// Check cache first
	if stats, ok := ca.cache.GetByHash(fullHash); ok {
		ca.cacheHit = true
		return stats
	}

	// Cache miss - compute fresh
	ca.cacheHit = false
	stats := ca.Analyzer.AnalyzeAsync(ctx)

	// Store in cache when Phase 2 completes
	go func() {
		stats.WaitForPhase2()
		ca.cache.SetByHash(fullHash, stats)
	}()

	return stats
}

// Analyze returns cached stats if available, otherwise computes synchronously.
// Note: This returns a value copy that shares map references with the original.
// This is safe because the maps are immutable after Phase 2 completion.
func (ca *CachedAnalyzer) Analyze() GraphStats {
	stats := ca.AnalyzeAsync(context.Background())
	stats.WaitForPhase2()
	return GraphStats{
		OutDegree:         stats.OutDegree,
		InDegree:          stats.InDegree,
		TopologicalOrder:  stats.TopologicalOrder,
		Density:           stats.Density,
		NodeCount:         stats.NodeCount,
		EdgeCount:         stats.EdgeCount,
		Config:            stats.Config,
		pageRank:          stats.pageRank,
		betweenness:       stats.betweenness,
		eigenvector:       stats.eigenvector,
		hubs:              stats.hubs,
		authorities:       stats.authorities,
		criticalPathScore: stats.criticalPathScore,
		cycles:            stats.cycles,
		phase2Ready:       true,
	}
}

// DataHash returns the computed hash for the analyzer's issue data.
func (ca *CachedAnalyzer) DataHash() string {
	return ca.dataHash
}

// WasCacheHit returns true if the last AnalyzeAsync call was a cache hit.
func (ca *CachedAnalyzer) WasCacheHit() bool {
	return ca.cacheHit
}

type robotAnalysisDiskCacheFile struct {
	Version int                                    `json:"version"`
	Entries map[string]robotAnalysisDiskCacheEntry `json:"entries"`
}

type robotAnalysisDiskCacheEntry struct {
	CreatedAt  time.Time           `json:"created_at"`
	AccessedAt time.Time           `json:"accessed_at"`
	DataHash   string              `json:"data_hash"`
	ConfigHash string              `json:"config_hash"`
	Result     graphStatsCacheBlob `json:"result"`
}

type graphStatsCacheBlob struct {
	OutDegree        map[string]int `json:"out_degree"`
	InDegree         map[string]int `json:"in_degree"`
	TopologicalOrder []string       `json:"topological_order"`
	Density          float64        `json:"density"`
	NodeCount        int            `json:"node_count"`
	EdgeCount        int            `json:"edge_count"`
	Config           AnalysisConfig `json:"config"`

	PageRank          map[string]float64 `json:"page_rank"`
	Betweenness       map[string]float64 `json:"betweenness"`
	Eigenvector       map[string]float64 `json:"eigenvector"`
	Hubs              map[string]float64 `json:"hubs"`
	Authorities       map[string]float64 `json:"authorities"`
	CriticalPathScore map[string]float64 `json:"critical_path_score"`
	CoreNumber        map[string]int     `json:"core_number"`
	Articulation      []string           `json:"articulation"`
	Slack             map[string]float64 `json:"slack"`
	Cycles            [][]string         `json:"cycles"`
	Status            MetricStatus       `json:"status"`
}

func (b graphStatsCacheBlob) toGraphStats() *GraphStats {
	stats := &GraphStats{
		OutDegree:        b.OutDegree,
		InDegree:         b.InDegree,
		TopologicalOrder: b.TopologicalOrder,
		Density:          b.Density,
		NodeCount:        b.NodeCount,
		EdgeCount:        b.EdgeCount,
		Config:           b.Config,

		phase2Ready: true,
		phase2Done:  make(chan struct{}),

		pageRank:          b.PageRank,
		betweenness:       b.Betweenness,
		eigenvector:       b.Eigenvector,
		hubs:              b.Hubs,
		authorities:       b.Authorities,
		criticalPathScore: b.CriticalPathScore,
		coreNumber:        b.CoreNumber,
		slack:             b.Slack,
		cycles:            b.Cycles,
		status:            b.Status,
	}

	if len(b.Articulation) > 0 {
		art := make(map[string]bool, len(b.Articulation))
		for _, id := range b.Articulation {
			art[id] = true
		}
		stats.articulation = art
	}

	// Rank maps are derived for UI optimization, so recompute rather than persist.
	stats.inDegreeRank = computeIntRanks(stats.InDegree)
	stats.outDegreeRank = computeIntRanks(stats.OutDegree)
	stats.pageRankRank = computeFloatRanks(stats.pageRank)
	stats.betweennessRank = computeFloatRanks(stats.betweenness)
	stats.eigenvectorRank = computeFloatRanks(stats.eigenvector)
	stats.hubsRank = computeFloatRanks(stats.hubs)
	stats.authoritiesRank = computeFloatRanks(stats.authorities)
	stats.criticalPathRank = computeFloatRanks(stats.criticalPathScore)

	close(stats.phase2Done)
	return stats
}

func robotDiskCacheEnabled() bool {
	return os.Getenv("BV_ROBOT") == "1"
}

func robotAnalysisDiskCachePath(create bool) (string, error) {
	base := os.Getenv("BV_CACHE_DIR")
	if base == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("getting user cache dir: %w", err)
		}
		base = filepath.Join(dir, robotAnalysisDiskCacheDirName)
	}
	if create {
		if err := os.MkdirAll(base, 0o755); err != nil {
			return "", fmt.Errorf("creating cache dir: %w", err)
		}
	}
	return filepath.Join(base, robotAnalysisDiskCacheFileName), nil
}

func readRobotDiskCacheLocked(f *os.File) robotAnalysisDiskCacheFile {
	if _, err := f.Seek(0, 0); err != nil {
		return robotAnalysisDiskCacheFile{Version: robotAnalysisDiskCacheVersion, Entries: map[string]robotAnalysisDiskCacheEntry{}}
	}
	data, err := io.ReadAll(f)
	if err != nil || len(data) == 0 {
		return robotAnalysisDiskCacheFile{Version: robotAnalysisDiskCacheVersion, Entries: map[string]robotAnalysisDiskCacheEntry{}}
	}

	var cf robotAnalysisDiskCacheFile
	if err := json.Unmarshal(data, &cf); err != nil || cf.Version != robotAnalysisDiskCacheVersion {
		return robotAnalysisDiskCacheFile{Version: robotAnalysisDiskCacheVersion, Entries: map[string]robotAnalysisDiskCacheEntry{}}
	}
	if cf.Entries == nil {
		cf.Entries = map[string]robotAnalysisDiskCacheEntry{}
	}
	return cf
}

func writeRobotDiskCacheLocked(f *os.File, cf robotAnalysisDiskCacheFile) error {
	if cf.Entries == nil {
		cf.Entries = map[string]robotAnalysisDiskCacheEntry{}
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cf); err != nil {
		return err
	}
	return f.Sync()
}

func pruneRobotDiskCacheEntries(now time.Time, entries map[string]robotAnalysisDiskCacheEntry) {
	for k, e := range entries {
		if e.CreatedAt.IsZero() || now.Sub(e.CreatedAt) > robotAnalysisDiskCacheMaxAge {
			delete(entries, k)
		}
	}
}

func evictRobotDiskCacheLRU(entries map[string]robotAnalysisDiskCacheEntry) {
	if len(entries) <= robotAnalysisDiskCacheMaxEntries {
		return
	}
	type item struct {
		key string
		t   time.Time
	}
	items := make([]item, 0, len(entries))
	for k, e := range entries {
		t := e.AccessedAt
		if t.IsZero() {
			t = e.CreatedAt
		}
		items = append(items, item{key: k, t: t})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].t.Equal(items[j].t) {
			return items[i].key < items[j].key
		}
		return items[i].t.Before(items[j].t)
	})
	for len(entries) > robotAnalysisDiskCacheMaxEntries && len(items) > 0 {
		delete(entries, items[0].key)
		items = items[1:]
	}
}

func getRobotDiskCachedStats(fullKey string) (*GraphStats, bool) {
	if !robotDiskCacheEnabled() {
		return nil, false
	}

	path, err := robotAnalysisDiskCachePath(false)
	if err != nil {
		return nil, false
	}

	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	if err := lockFile(f); err != nil {
		return nil, false
	}
	defer func() { _ = unlockFile(f) }()

	now := time.Now()
	cf := readRobotDiskCacheLocked(f)
	pruneRobotDiskCacheEntries(now, cf.Entries)

	entry, ok := cf.Entries[fullKey]
	if !ok {
		// Best-effort: persist prunes.
		_ = writeRobotDiskCacheLocked(f, cf)
		return nil, false
	}

	entry.AccessedAt = now.UTC()
	cf.Entries[fullKey] = entry
	evictRobotDiskCacheLRU(cf.Entries)
	_ = writeRobotDiskCacheLocked(f, cf)

	return entry.Result.toGraphStats(), true
}

func putRobotDiskCachedStats(fullKey, dataHash, configHash string, stats *GraphStats) {
	if !robotDiskCacheEnabled() {
		return
	}
	if stats == nil || !stats.IsPhase2Ready() {
		return
	}

	stats.mu.RLock()
	blob := graphStatsCacheBlob{
		OutDegree:        stats.OutDegree,
		InDegree:         stats.InDegree,
		TopologicalOrder: stats.TopologicalOrder,
		Density:          stats.Density,
		NodeCount:        stats.NodeCount,
		EdgeCount:        stats.EdgeCount,
		Config:           stats.Config,

		PageRank:          stats.pageRank,
		Betweenness:       stats.betweenness,
		Eigenvector:       stats.eigenvector,
		Hubs:              stats.hubs,
		Authorities:       stats.authorities,
		CriticalPathScore: stats.criticalPathScore,
		CoreNumber:        stats.coreNumber,
		Slack:             stats.slack,
		Cycles:            stats.cycles,
		Status:            stats.status,
	}
	if stats.articulation != nil {
		blob.Articulation = make([]string, 0, len(stats.articulation))
		for id := range stats.articulation {
			blob.Articulation = append(blob.Articulation, id)
		}
		sort.Strings(blob.Articulation)
	}
	stats.mu.RUnlock()

	if b, err := json.Marshal(blob); err != nil || len(b) > robotAnalysisDiskCacheMaxEntrySize {
		return
	}

	path, err := robotAnalysisDiskCachePath(true)
	if err != nil {
		return
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	if err := lockFile(f); err != nil {
		return
	}
	defer func() { _ = unlockFile(f) }()

	now := time.Now().UTC()
	cf := readRobotDiskCacheLocked(f)
	pruneRobotDiskCacheEntries(now, cf.Entries)

	if cf.Entries == nil {
		cf.Entries = map[string]robotAnalysisDiskCacheEntry{}
	}

	cf.Entries[fullKey] = robotAnalysisDiskCacheEntry{
		CreatedAt:  now,
		AccessedAt: now,
		DataHash:   dataHash,
		ConfigHash: configHash,
		Result:     blob,
	}

	evictRobotDiskCacheLRU(cf.Entries)
	_ = writeRobotDiskCacheLocked(f, cf)
}
