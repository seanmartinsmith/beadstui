package analysis

import (
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestCacheSetTTLAndHash(t *testing.T) {
	issues := []model.Issue{{ID: "C1", Title: "Cache"}}
	c := NewCache(10 * time.Second)
	stats := &GraphStats{NodeCount: 1}
	c.Set(issues, stats)
	if c.Hash() == "" {
		t.Fatalf("expected hash after Set")
	}

	// Override TTL and ensure GetByHash respects expiry
	c.SetTTL(-1 * time.Second)
	if got, ok := c.Get(issues); got != nil || ok {
		t.Fatalf("expected cache miss after expired TTL")
	}
}
