package search

import "fmt"

type hybridScorer struct {
	weights Weights
	cache   MetricsCache
}

// NewHybridScorer creates a scorer with the given weights and metrics cache.
func NewHybridScorer(weights Weights, cache MetricsCache) HybridScorer {
	normalized := weights.Normalize()
	if err := normalized.Validate(); err != nil {
		if preset, presetErr := GetPreset(PresetDefault); presetErr == nil {
			normalized = preset
		} else {
			normalized = Weights{TextRelevance: 1.0}
		}
	}
	return &hybridScorer{
		weights: normalized,
		cache:   cache,
	}
}

func (s *hybridScorer) Score(issueID string, textScore float64) (HybridScore, error) {
	if issueID == "" {
		return HybridScore{}, fmt.Errorf("issueID is required")
	}

	if s.cache == nil {
		return HybridScore{
			IssueID:    issueID,
			FinalScore: textScore,
			TextScore:  textScore,
		}, nil
	}

	metrics, found := s.cache.Get(issueID)
	if !found {
		return HybridScore{
			IssueID:    issueID,
			FinalScore: textScore,
			TextScore:  textScore,
		}, nil
	}

	// Skip normalizations for zero-weight components to save computation
	var statusScore, priorityScore, impactScore, recencyScore float64
	if s.weights.Status > 0 {
		statusScore = normalizeStatus(metrics.Status)
	}
	if s.weights.Priority > 0 {
		priorityScore = normalizePriority(metrics.Priority)
	}
	if s.weights.Impact > 0 {
		impactScore = normalizeImpact(metrics.BlockerCount, s.cache.MaxBlockerCount())
	}
	if s.weights.Recency > 0 {
		recencyScore = normalizeRecency(metrics.UpdatedAt)
	}

	final := s.weights.TextRelevance*textScore +
		s.weights.PageRank*metrics.PageRank +
		s.weights.Status*statusScore +
		s.weights.Impact*impactScore +
		s.weights.Priority*priorityScore +
		s.weights.Recency*recencyScore

	return HybridScore{
		IssueID:    issueID,
		FinalScore: final,
		TextScore:  textScore,
		ComponentScores: map[string]float64{
			"pagerank": metrics.PageRank,
			"status":   statusScore,
			"impact":   impactScore,
			"priority": priorityScore,
			"recency":  recencyScore,
		},
	}, nil
}

func (s *hybridScorer) Configure(weights Weights) error {
	if err := weights.Validate(); err != nil {
		return err
	}
	s.weights = weights
	return nil
}

func (s *hybridScorer) GetWeights() Weights {
	return s.weights
}
