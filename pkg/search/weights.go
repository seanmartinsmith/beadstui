package search

import (
	"fmt"
	"log"
	"math"
)

const weightSumTolerance = 0.001

// Weights defines the relative importance of each ranking factor.
// All weights should sum to 1.0 for normalized scoring.
//
// Normalization reference:
//   - StatusWeight: {open: 1.0, in_progress: 0.8, blocked: 0.5, closed: 0.1}
//   - PriorityWeight: P0=1.0, P1=0.8, P2=0.6, P3=0.4, P4=0.2
//   - RecencyWeight: exp(-days_since_update / 30)
//   - ImpactWeight: blocker_count / max_blocker_count (or 0.5 if max=0)
type Weights struct {
	TextRelevance float64 `json:"text"`     // Core search match quality
	PageRank      float64 `json:"pagerank"` // Graph centrality importance
	Status        float64 `json:"status"`   // Actionability (open > closed)
	Impact        float64 `json:"impact"`   // Blocker count normalized
	Priority      float64 `json:"priority"` // User-assigned priority
	Recency       float64 `json:"recency"`  // Temporal decay
}

// Validate checks that weights are valid (non-negative, sum to ~1.0).
// It logs a warning when text relevance is very low.
func (w Weights) Validate() error {
	if w.TextRelevance < 0 || w.PageRank < 0 || w.Status < 0 ||
		w.Impact < 0 || w.Priority < 0 || w.Recency < 0 {
		return fmt.Errorf("weights must be non-negative")
	}

	if w.TextRelevance < 0.1 {
		log.Printf("WARNING: text weight %.2f is very low; results may not match query", w.TextRelevance)
	}

	sum := w.sum()
	if math.Abs(sum-1.0) > weightSumTolerance {
		return fmt.Errorf("weights must sum to 1.0, got %.3f", sum)
	}

	return nil
}

// Normalize scales weights to sum to 1.0.
func (w Weights) Normalize() Weights {
	sum := w.sum()
	if sum == 0 {
		return w
	}

	return Weights{
		TextRelevance: w.TextRelevance / sum,
		PageRank:      w.PageRank / sum,
		Status:        w.Status / sum,
		Impact:        w.Impact / sum,
		Priority:      w.Priority / sum,
		Recency:       w.Recency / sum,
	}
}

func (w Weights) sum() float64 {
	return w.TextRelevance + w.PageRank + w.Status + w.Impact + w.Priority + w.Recency
}
