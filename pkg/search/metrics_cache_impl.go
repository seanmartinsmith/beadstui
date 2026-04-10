package search

import (
	"context"
	"fmt"
	"sync"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

const defaultPageRank = 0.5

// metricsCache is the default MetricsCache implementation.
type metricsCache struct {
	mu              sync.RWMutex
	metrics         map[string]IssueMetrics
	dataHash        string
	maxBlockerCount int
	loader          MetricsLoader
}

// NewMetricsCache creates a MetricsCache backed by the provided loader.
func NewMetricsCache(loader MetricsLoader) MetricsCache {
	return &metricsCache{
		metrics: make(map[string]IssueMetrics),
		loader:  loader,
	}
}

// AnalyzerMetricsLoader loads metrics from the analysis engine.
type AnalyzerMetricsLoader struct {
	issues []model.Issue
	cache  *analysis.Cache
	config *analysis.AnalysisConfig
}

// NewAnalyzerMetricsLoader creates a loader that derives metrics from issues.
func NewAnalyzerMetricsLoader(issues []model.Issue) *AnalyzerMetricsLoader {
	return &AnalyzerMetricsLoader{issues: issues}
}

// WithCache configures a custom analysis cache for this loader.
func (l *AnalyzerMetricsLoader) WithCache(cache *analysis.Cache) *AnalyzerMetricsLoader {
	l.cache = cache
	return l
}

// WithConfig overrides the analysis configuration.
func (l *AnalyzerMetricsLoader) WithConfig(config *analysis.AnalysisConfig) *AnalyzerMetricsLoader {
	l.config = config
	return l
}

// LoadMetrics computes metrics for all issues using the analysis engine.
func (l *AnalyzerMetricsLoader) LoadMetrics() (map[string]IssueMetrics, error) {
	if len(l.issues) == 0 {
		return map[string]IssueMetrics{}, nil
	}

	cached := analysis.NewCachedAnalyzer(l.issues, l.cache)
	if l.config != nil {
		cached.SetConfig(l.config)
	}

	stats := cached.AnalyzeAsync(context.Background())
	stats.WaitForPhase2()

	pageRank := stats.PageRank()
	metrics := make(map[string]IssueMetrics, len(l.issues))

	for _, issue := range l.issues {
		pr, ok := pageRank[issue.ID]
		if !ok {
			pr = defaultPageRank
		}
		metrics[issue.ID] = IssueMetrics{
			IssueID:      issue.ID,
			PageRank:     pr,
			Status:       string(issue.Status),
			Priority:     issue.Priority,
			BlockerCount: stats.InDegree[issue.ID],
			UpdatedAt:    issue.UpdatedAt,
		}
	}

	return metrics, nil
}

// ComputeDataHash returns the data hash for the loader's issue set.
func (l *AnalyzerMetricsLoader) ComputeDataHash() (string, error) {
	return analysis.ComputeDataHash(l.issues), nil
}

// Get returns metrics for an issue, computing/loading if needed.
func (c *metricsCache) Get(issueID string) (IssueMetrics, bool) {
	if issueID == "" {
		return defaultIssueMetrics(issueID), false
	}

	if err := c.ensureFresh(); err != nil {
		return defaultIssueMetrics(issueID), false
	}

	c.mu.RLock()
	metric, ok := c.metrics[issueID]
	c.mu.RUnlock()
	if !ok {
		return defaultIssueMetrics(issueID), false
	}
	return metric, true
}

// GetBatch returns metrics for multiple issues efficiently.
func (c *metricsCache) GetBatch(issueIDs []string) map[string]IssueMetrics {
	results := make(map[string]IssueMetrics, len(issueIDs))
	if len(issueIDs) == 0 {
		return results
	}

	if err := c.ensureFresh(); err != nil {
		for _, id := range issueIDs {
			results[id] = defaultIssueMetrics(id)
		}
		return results
	}

	c.mu.RLock()
	for _, id := range issueIDs {
		metric, ok := c.metrics[id]
		if !ok {
			metric = defaultIssueMetrics(id)
		}
		results[id] = metric
	}
	c.mu.RUnlock()

	return results
}

// Refresh recomputes the cache from source data.
func (c *metricsCache) Refresh() error {
	if c.loader == nil {
		return fmt.Errorf("metrics loader is nil")
	}

	metrics, err := c.loader.LoadMetrics()
	if err != nil {
		return err
	}

	hash, err := c.loader.ComputeDataHash()
	if err != nil {
		return err
	}

	copied := make(map[string]IssueMetrics, len(metrics))
	maxBlocker := 0
	for id, metric := range metrics {
		copied[id] = metric
		if metric.BlockerCount > maxBlocker {
			maxBlocker = metric.BlockerCount
		}
	}

	c.mu.Lock()
	c.metrics = copied
	c.dataHash = hash
	c.maxBlockerCount = maxBlocker
	c.mu.Unlock()

	return nil
}

// DataHash returns the hash of source data for cache validation.
func (c *metricsCache) DataHash() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dataHash
}

// MaxBlockerCount returns the maximum blocker count for normalization.
func (c *metricsCache) MaxBlockerCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.maxBlockerCount
}

func (c *metricsCache) ensureFresh() error {
	if c.loader == nil {
		return fmt.Errorf("metrics loader is nil")
	}

	hash, err := c.loader.ComputeDataHash()
	if err != nil {
		return err
	}

	c.mu.RLock()
	isFresh := c.metrics != nil && c.dataHash != "" && c.dataHash == hash
	c.mu.RUnlock()

	if isFresh {
		return nil
	}

	return c.Refresh()
}

func defaultIssueMetrics(issueID string) IssueMetrics {
	return IssueMetrics{
		IssueID:      issueID,
		PageRank:     defaultPageRank,
		Priority:     2,
		BlockerCount: 0,
	}
}
