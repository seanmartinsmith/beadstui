// Package correlation provides feedback storage for correlation auditing.
package correlation

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// FeedbackFileName is the default name for the feedback storage file
	FeedbackFileName = "correlation_feedback.jsonl"
)

// FeedbackStore manages storage and retrieval of correlation feedback
type FeedbackStore struct {
	beadsDir string
	mu       sync.RWMutex
	cache    map[feedbackKey]CorrelationFeedback
}

type feedbackKey struct {
	commitSHA string
	beadID    string
}

// NewFeedbackStore creates a new feedback store for the given beads directory
func NewFeedbackStore(beadsDir string) *FeedbackStore {
	return &FeedbackStore{
		beadsDir: beadsDir,
		cache:    make(map[feedbackKey]CorrelationFeedback),
	}
}

// feedbackPath returns the full path to the feedback file
func (fs *FeedbackStore) feedbackPath() string {
	return filepath.Join(fs.beadsDir, FeedbackFileName)
}

// Load reads existing feedback from the JSONL file
func (fs *FeedbackStore) Load() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.feedbackPath()
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		// No feedback file yet, that's fine
		return nil
	}
	if err != nil {
		return fmt.Errorf("opening feedback file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var fb CorrelationFeedback
		if err := json.Unmarshal(line, &fb); err != nil {
			// Log warning but continue with other lines
			continue
		}

		key := feedbackKey{commitSHA: fb.CommitSHA, beadID: fb.BeadID}
		fs.cache[key] = fb
	}

	return scanner.Err()
}

// Save stores a feedback entry, appending to the JSONL file
func (fs *FeedbackStore) Save(fb CorrelationFeedback) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Ensure the beads directory exists
	if err := os.MkdirAll(fs.beadsDir, 0755); err != nil {
		return fmt.Errorf("creating beads directory: %w", err)
	}

	// Open file in append mode
	path := fs.feedbackPath()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening feedback file: %w", err)
	}
	defer file.Close()

	// Marshal to JSON
	data, err := json.Marshal(fb)
	if err != nil {
		return fmt.Errorf("marshaling feedback: %w", err)
	}

	// Write with newline
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("writing feedback: %w", err)
	}

	// Update cache
	key := feedbackKey{commitSHA: fb.CommitSHA, beadID: fb.BeadID}
	fs.cache[key] = fb

	return nil
}

// Get retrieves feedback for a specific commit-bead pair
func (fs *FeedbackStore) Get(commitSHA, beadID string) (CorrelationFeedback, bool) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	key := feedbackKey{commitSHA: commitSHA, beadID: beadID}
	fb, ok := fs.cache[key]
	return fb, ok
}

// GetAll returns all feedback entries
func (fs *FeedbackStore) GetAll() []CorrelationFeedback {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	result := make([]CorrelationFeedback, 0, len(fs.cache))
	for _, fb := range fs.cache {
		result = append(result, fb)
	}
	return result
}

// GetStats calculates aggregate statistics about the feedback
func (fs *FeedbackStore) GetStats() FeedbackStats {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	stats := FeedbackStats{}
	var confirmSum, rejectSum float64

	for _, fb := range fs.cache {
		stats.TotalFeedback++
		switch fb.Type {
		case FeedbackConfirm:
			stats.Confirmed++
			confirmSum += fb.OriginalConf
		case FeedbackReject:
			stats.Rejected++
			rejectSum += fb.OriginalConf
		case FeedbackIgnore:
			stats.Ignored++
		}
	}

	// Calculate accuracy rate (confirmed / total decisions)
	decisions := stats.Confirmed + stats.Rejected
	if decisions > 0 {
		stats.AccuracyRate = float64(stats.Confirmed) / float64(decisions)
	}

	// Calculate average confidences
	if stats.Confirmed > 0 {
		stats.AvgConfirmConf = confirmSum / float64(stats.Confirmed)
	}
	if stats.Rejected > 0 {
		stats.AvgRejectConf = rejectSum / float64(stats.Rejected)
	}

	return stats
}

// Confirm records a confirmation that the correlation is correct
func (fs *FeedbackStore) Confirm(commitSHA, beadID, feedbackBy string, originalConf float64, reason string) error {
	fb := CorrelationFeedback{
		CommitSHA:    commitSHA,
		BeadID:       beadID,
		FeedbackAt:   time.Now().UTC(),
		FeedbackBy:   feedbackBy,
		Type:         FeedbackConfirm,
		Reason:       reason,
		OriginalConf: originalConf,
	}
	return fs.Save(fb)
}

// Reject records a rejection that the correlation is incorrect
func (fs *FeedbackStore) Reject(commitSHA, beadID, feedbackBy string, originalConf float64, reason string) error {
	fb := CorrelationFeedback{
		CommitSHA:    commitSHA,
		BeadID:       beadID,
		FeedbackAt:   time.Now().UTC(),
		FeedbackBy:   feedbackBy,
		Type:         FeedbackReject,
		Reason:       reason,
		OriginalConf: originalConf,
	}
	return fs.Save(fb)
}

// Ignore records that this correlation should be excluded from training
func (fs *FeedbackStore) Ignore(commitSHA, beadID, feedbackBy string, originalConf float64, reason string) error {
	fb := CorrelationFeedback{
		CommitSHA:    commitSHA,
		BeadID:       beadID,
		FeedbackAt:   time.Now().UTC(),
		FeedbackBy:   feedbackBy,
		Type:         FeedbackIgnore,
		Reason:       reason,
		OriginalConf: originalConf,
	}
	return fs.Save(fb)
}

// HasFeedback returns true if there is existing feedback for this correlation
func (fs *FeedbackStore) HasFeedback(commitSHA, beadID string) bool {
	_, ok := fs.Get(commitSHA, beadID)
	return ok
}

// GetByBead returns all feedback entries for a specific bead
func (fs *FeedbackStore) GetByBead(beadID string) []CorrelationFeedback {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var result []CorrelationFeedback
	for _, fb := range fs.cache {
		if fb.BeadID == beadID {
			result = append(result, fb)
		}
	}
	return result
}

// GetByCommit returns all feedback entries for a specific commit
func (fs *FeedbackStore) GetByCommit(commitSHA string) []CorrelationFeedback {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var result []CorrelationFeedback
	for _, fb := range fs.cache {
		if fb.CommitSHA == commitSHA {
			result = append(result, fb)
		}
	}
	return result
}
