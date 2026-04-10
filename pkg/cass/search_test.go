package cass

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSearchOptions_Defaults(t *testing.T) {
	s := NewSearcher(NewDetector())
	args := s.buildArgs(SearchOptions{Query: "test"})

	// Should include defaults
	expected := []string{"search", "test", "--robot", "--limit", "10", "--fields", "minimal"}
	if len(args) != len(expected) {
		t.Errorf("buildArgs() returned %d args, want %d", len(args), len(expected))
	}
	for i, want := range expected {
		if i < len(args) && args[i] != want {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want)
		}
	}
}

func TestSearchOptions_AllFields(t *testing.T) {
	s := NewSearcher(NewDetector())
	args := s.buildArgs(SearchOptions{
		Query:     "my query",
		Limit:     20,
		Days:      30,
		Workspace: "/home/user/project",
		Fields:    "summary",
	})

	// Check key arguments are present
	argSet := make(map[string]bool)
	for i := 0; i < len(args); i++ {
		argSet[args[i]] = true
	}

	if !argSet["--robot"] {
		t.Error("missing --robot flag")
	}
	if !argSet["my query"] {
		t.Error("missing query")
	}

	// Check limit value
	for i, arg := range args {
		if arg == "--limit" && i+1 < len(args) {
			if args[i+1] != "20" {
				t.Errorf("--limit value = %q, want \"20\"", args[i+1])
			}
		}
		if arg == "--days" && i+1 < len(args) {
			if args[i+1] != "30" {
				t.Errorf("--days value = %q, want \"30\"", args[i+1])
			}
		}
		if arg == "--workspace" && i+1 < len(args) {
			if args[i+1] != "/home/user/project" {
				t.Errorf("--workspace value = %q, want \"/home/user/project\"", args[i+1])
			}
		}
		if arg == "--fields" && i+1 < len(args) {
			if args[i+1] != "summary" {
				t.Errorf("--fields value = %q, want \"summary\"", args[i+1])
			}
		}
	}
}

func TestSearcher_CassNotHealthy(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "", errors.New("not found")
	}

	s := NewSearcher(d)
	resp := s.Search(context.Background(), SearchOptions{Query: "test"})

	if len(resp.Results) != 0 {
		t.Errorf("Results = %d items, want 0", len(resp.Results))
	}
	if resp.Meta.Error == "" {
		t.Error("Meta.Error should be set when cass not available")
	}
}

func TestSearcher_SuccessfulSearch(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil // Healthy
	}
	d.Check() // Prime the cache

	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		resp := SearchResponse{
			Results: []SearchResult{
				{
					SourcePath: "/sessions/session1.json",
					LineNumber: 42,
					Agent:      "claude",
					Title:      "Refactoring discussion",
					Score:      0.95,
					Snippet:    "Let's refactor the auth module...",
					MatchType:  "exact",
				},
				{
					SourcePath: "/sessions/session2.json",
					LineNumber: 100,
					Agent:      "cursor",
					Title:      "Bug fix review",
					Score:      0.8,
					Snippet:    "The bug was in the auth code...",
					MatchType:  "fuzzy",
				},
			},
			Meta: SearchMeta{
				Total:     2,
				ElapsedMs: 50,
			},
		}
		return json.Marshal(resp)
	}

	resp := s.Search(context.Background(), SearchOptions{Query: "auth"})

	if len(resp.Results) != 2 {
		t.Errorf("Results = %d items, want 2", len(resp.Results))
	}
	if resp.Results[0].Agent != "claude" {
		t.Errorf("Results[0].Agent = %q, want \"claude\"", resp.Results[0].Agent)
	}
	if resp.Results[0].Score != 0.95 {
		t.Errorf("Results[0].Score = %f, want 0.95", resp.Results[0].Score)
	}
}

func TestSearcher_EmptyResults(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		resp := SearchResponse{
			Results: []SearchResult{},
			Meta:    SearchMeta{Total: 0},
		}
		return json.Marshal(resp)
	}

	resp := s.Search(context.Background(), SearchOptions{Query: "nonexistent"})

	if resp.Results == nil {
		t.Error("Results should never be nil")
	}
	if len(resp.Results) != 0 {
		t.Errorf("Results = %d items, want 0", len(resp.Results))
	}
}

func TestSearcher_CommandError(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("command failed")
	}

	resp := s.Search(context.Background(), SearchOptions{Query: "test"})

	if len(resp.Results) != 0 {
		t.Errorf("Results = %d items, want 0", len(resp.Results))
	}
	if resp.Meta.Error == "" {
		t.Error("Meta.Error should be set on command failure")
	}
}

func TestSearcher_MalformedJSON(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(`{invalid json`), nil
	}

	resp := s.Search(context.Background(), SearchOptions{Query: "test"})

	if len(resp.Results) != 0 {
		t.Errorf("Results = %d items, want 0", len(resp.Results))
	}
	if resp.Meta.Error == "" {
		t.Error("Meta.Error should be set on parse failure")
	}
}

func TestSearcher_PartialJSON(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcher(d)
	// Return JSON with only results array (no meta)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(`{"results":[{"source_path":"/test","score":0.5}]}`), nil
	}

	resp := s.Search(context.Background(), SearchOptions{Query: "test"})

	if len(resp.Results) != 1 {
		t.Errorf("Results = %d items, want 1", len(resp.Results))
	}
	if resp.Results[0].SourcePath != "/test" {
		t.Errorf("Results[0].SourcePath = %q, want \"/test\"", resp.Results[0].SourcePath)
	}
}

func TestSearcher_Timeout(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcherWithOptions(d, WithSearchTimeout(50*time.Millisecond))
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		// Simulate slow command
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return []byte(`{"results":[]}`), nil
		}
	}

	start := time.Now()
	resp := s.Search(context.Background(), SearchOptions{Query: "test"})
	elapsed := time.Since(start)

	if len(resp.Results) != 0 {
		t.Errorf("Results = %d items, want 0", len(resp.Results))
	}
	if elapsed > 150*time.Millisecond {
		t.Errorf("Search took %v, should have timed out around 50ms", elapsed)
	}
}

func TestSearcher_CustomTimeout(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcherWithOptions(d, WithSearchTimeout(200*time.Millisecond))
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		// Simulate command that takes 100ms but respects context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return []byte(`{"results":[{"source_path":"/test"}]}`), nil
		}
	}

	resp := s.Search(context.Background(), SearchOptions{
		Query:   "test",
		Timeout: 30 * time.Millisecond, // Override with shorter timeout
	})

	// Should timeout due to custom timeout being shorter than command time
	if resp.Meta.Error == "" {
		t.Error("Expected timeout error with custom short timeout")
	}
}

func TestSearcher_ConcurrencyLimit(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	var concurrent int32
	var maxConcurrent int32
	var failureCount int32

	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		current := atomic.AddInt32(&concurrent, 1)
		defer atomic.AddInt32(&concurrent, -1)

		// Track max concurrent
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
				break
			}
		}

		time.Sleep(20 * time.Millisecond)
		return []byte(`{"results":[]}`), nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := s.Search(context.Background(), SearchOptions{Query: "test"})
			if resp.Results == nil {
				atomic.AddInt32(&failureCount, 1)
			}
		}()
	}
	wg.Wait()

	if failures := atomic.LoadInt32(&failureCount); failures > 0 {
		t.Errorf("%d goroutines failed, want 0", failures)
	}

	if maxConcurrent > MaxConcurrentSearches {
		t.Errorf("maxConcurrent = %d, want <= %d", maxConcurrent, MaxConcurrentSearches)
	}
}

func TestSearcher_ContextCancellation(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	resp := s.Search(ctx, SearchOptions{Query: "test"})

	if len(resp.Results) != 0 {
		t.Errorf("Results = %d items, want 0", len(resp.Results))
	}
}

func TestSearcher_SearchWithQuery(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	var capturedQuery string
	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if len(args) >= 2 {
			capturedQuery = args[1]
		}
		return []byte(`{"results":[]}`), nil
	}

	s.SearchWithQuery(context.Background(), "my simple query")

	if capturedQuery != "my simple query" {
		t.Errorf("capturedQuery = %q, want \"my simple query\"", capturedQuery)
	}
}

func TestSearcher_SearchInWorkspace(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	var capturedArgs []string
	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(`{"results":[]}`), nil
	}

	s.SearchInWorkspace(context.Background(), "query", "/my/workspace")

	hasWorkspace := false
	for i, arg := range capturedArgs {
		if arg == "--workspace" && i+1 < len(capturedArgs) {
			if capturedArgs[i+1] == "/my/workspace" {
				hasWorkspace = true
			}
		}
	}
	if !hasWorkspace {
		t.Error("--workspace flag not set correctly")
	}
}

func TestSearcher_EmptyOutput(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte{}, nil // Empty output
	}

	resp := s.Search(context.Background(), SearchOptions{Query: "test"})

	if resp.Results == nil {
		t.Error("Results should never be nil")
	}
	if len(resp.Results) != 0 {
		t.Errorf("Results = %d items, want 0", len(resp.Results))
	}
}

func TestSearcher_NullResults(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(`{"results":null}`), nil
	}

	resp := s.Search(context.Background(), SearchOptions{Query: "test"})

	if resp.Results == nil {
		t.Error("Results should never be nil, even when JSON has null")
	}
}

func TestLimitedWriter(t *testing.T) {
	var buf bytes.Buffer
	lw := &limitedWriter{w: &buf, limit: 10}

	n, err := lw.Write([]byte("hello"))
	if err != nil {
		t.Errorf("Write error: %v", err)
	}
	if n != 5 {
		t.Errorf("Write returned %d, want 5", n)
	}

	// Should truncate
	n, err = lw.Write([]byte("world!"))
	if err != nil {
		t.Errorf("Write error: %v", err)
	}
	// Returns full length even though truncated
	if n != 6 {
		t.Errorf("Write returned %d, want 6", n)
	}

	if buf.String() != "helloworld" {
		t.Errorf("buf = %q, want \"helloworld\"", buf.String())
	}
}

func TestLimitedWriter_ExactLimit(t *testing.T) {
	var buf bytes.Buffer
	lw := &limitedWriter{w: &buf, limit: 5}

	n, err := lw.Write([]byte("hello"))
	if err != nil {
		t.Errorf("Write error: %v", err)
	}
	if n != 5 {
		t.Errorf("Write returned %d, want 5", n)
	}

	// Further writes should be silently discarded but return original length
	n, err = lw.Write([]byte("more"))
	if err != nil {
		t.Errorf("Write error: %v", err)
	}
	if n != 4 {
		t.Errorf("Write returned %d, want 4 (original length)", n)
	}

	if buf.String() != "hello" {
		t.Errorf("buf = %q, want \"hello\"", buf.String())
	}
}

// BenchmarkSearcher_Search benchmarks the search operation with mocked cass.
func BenchmarkSearcher_Search(b *testing.B) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(`{"results":[{"source_path":"/test","score":0.5}]}`), nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Search(context.Background(), SearchOptions{Query: "test"})
	}
}
