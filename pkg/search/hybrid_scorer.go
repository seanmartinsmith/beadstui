package search

// HybridScorer computes hybrid search scores combining text relevance with graph metrics.
type HybridScorer interface {
	// Score computes the hybrid score for an issue given its text score and metrics.
	// Returns the final score and component breakdown.
	Score(issueID string, textScore float64) (HybridScore, error)

	// Configure sets the weights for hybrid scoring.
	Configure(weights Weights) error

	// GetWeights returns the current weight configuration.
	GetWeights() Weights
}

// HybridScore contains the final score and component breakdown for transparency.
type HybridScore struct {
	IssueID         string             `json:"issue_id"`
	FinalScore      float64            `json:"score"`
	TextScore       float64            `json:"text_score"`
	ComponentScores map[string]float64 `json:"component_scores,omitempty"`
}
