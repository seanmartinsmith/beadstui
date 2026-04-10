package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// Race Condition Tests for bv-kozq
// Tests thread safety and concurrent access patterns.
// Run with: go test -race ./tests/e2e -run TestRace

// =============================================================================
// 1. Concurrent Robot Command Execution
// =============================================================================

// TestRace_ConcurrentRobotCommands tests that multiple robot commands can run
// concurrently without race conditions.
func TestRace_ConcurrentRobotCommands(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Create test data
	beadsDir := filepath.Join(env, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads dir: %v", err)
	}

	// Create a graph with dependencies
	issues := `{"id":"root","title":"Root","status":"open","priority":1}
{"id":"mid-1","title":"Mid 1","status":"open","priority":2,"dependencies":[{"depends_on_id":"root","type":"blocks"}]}
{"id":"mid-2","title":"Mid 2","status":"open","priority":2,"dependencies":[{"depends_on_id":"root","type":"blocks"}]}
{"id":"leaf-1","title":"Leaf 1","status":"open","priority":3,"dependencies":[{"depends_on_id":"mid-1","type":"blocks"}]}
{"id":"leaf-2","title":"Leaf 2","status":"open","priority":3,"dependencies":[{"depends_on_id":"mid-2","type":"blocks"}]}`

	issuesPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(issuesPath, []byte(issues), 0644); err != nil {
		t.Fatalf("failed to write issues.jsonl: %v", err)
	}

	// Run multiple robot commands concurrently
	commands := [][]string{
		{"--robot-triage"},
		{"--robot-next"},
		{"--robot-graph", "--graph-format", "json"},
		{"--robot-plan"},
		{"--robot-priority"},
	}

	var wg sync.WaitGroup
	errors := make(chan error, len(commands)*3)

	// Run each command 3 times concurrently
	for i := 0; i < 3; i++ {
		for _, args := range commands {
			wg.Add(1)
			go func(cmdArgs []string) {
				defer wg.Done()
				cmd := exec.Command(bv, cmdArgs...)
				cmd.Dir = env
				if err := cmd.Run(); err != nil {
					errors <- err
				}
			}(args)
		}
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errCount int
	for err := range errors {
		t.Logf("concurrent command error: %v", err)
		errCount++
	}

	if errCount > 0 {
		t.Errorf("had %d errors during concurrent execution", errCount)
	}
}

// TestRace_ConcurrentTriageRequests simulates multiple agents requesting triage
// simultaneously.
func TestRace_ConcurrentTriageRequests(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Create test data with more issues for better concurrency stress
	beadsDir := filepath.Join(env, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads dir: %v", err)
	}

	var issueLines []byte
	for i := 0; i < 50; i++ {
		line := []byte(`{"id":"issue-` + string(rune('A'+i%26)) + string(rune('0'+i/26)) + `","title":"Issue ` + string(rune('A'+i)) + `","status":"open","priority":` + string(rune('0'+i%5)) + `}` + "\n")
		issueLines = append(issueLines, line...)
	}

	issuesPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(issuesPath, issueLines, 0644); err != nil {
		t.Fatalf("failed to write issues.jsonl: %v", err)
	}

	// Simulate 10 concurrent agent requests
	const numAgents = 10
	var wg sync.WaitGroup
	results := make(chan string, numAgents)
	errors := make(chan error, numAgents)

	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(bv, "--robot-triage")
			cmd.Dir = env
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			if err := cmd.Run(); err != nil {
				errors <- err
				return
			}
			results <- stdout.String()
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	// Verify all requests succeeded
	var resultCount int
	for range results {
		resultCount++
	}

	var errCount int
	for err := range errors {
		t.Logf("concurrent triage error: %v", err)
		errCount++
	}

	if errCount > 0 {
		t.Errorf("had %d errors during concurrent triage requests", errCount)
	}

	if resultCount != numAgents {
		t.Errorf("expected %d results, got %d", numAgents, resultCount)
	}
}

// =============================================================================
// 2. Data Consistency Under Concurrent Access
// =============================================================================

// TestRace_DataConsistency verifies that concurrent reads return consistent data.
func TestRace_DataConsistency(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Create deterministic test data
	beadsDir := filepath.Join(env, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads dir: %v", err)
	}

	issues := `{"id":"A","title":"Task A","status":"open","priority":1}
{"id":"B","title":"Task B","status":"open","priority":1,"dependencies":[{"depends_on_id":"A","type":"blocks"}]}
{"id":"C","title":"Task C","status":"open","priority":1,"dependencies":[{"depends_on_id":"B","type":"blocks"}]}`

	issuesPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(issuesPath, []byte(issues), 0644); err != nil {
		t.Fatalf("failed to write issues.jsonl: %v", err)
	}

	// Run multiple reads and verify consistency
	const numReads = 5
	var wg sync.WaitGroup
	results := make([]string, numReads)

	for i := 0; i < numReads; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cmd := exec.Command(bv, "--robot-next")
			cmd.Dir = env
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			if err := cmd.Run(); err != nil {
				t.Errorf("read %d failed: %v", idx, err)
				return
			}
			results[idx] = stdout.String()
		}(i)
	}

	wg.Wait()

	// All results should be identical (deterministic)
	if len(results) > 0 && results[0] != "" {
		for i := 1; i < numReads; i++ {
			if results[i] != results[0] {
				t.Errorf("inconsistent results: read 0 != read %d", i)
			}
		}
	}
}

// =============================================================================
// 3. Concurrent Analysis and Graph Commands
// =============================================================================

// TestRace_ConcurrentAnalysisAndGraph tests running analysis while getting graph data.
func TestRace_ConcurrentAnalysisAndGraph(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Create test data
	beadsDir := filepath.Join(env, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads dir: %v", err)
	}

	issues := `{"id":"test-1","title":"Test 1","status":"open","priority":1}
{"id":"test-2","title":"Test 2","status":"open","priority":2,"dependencies":[{"depends_on_id":"test-1","type":"blocks"}]}`

	issuesPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(issuesPath, []byte(issues), 0644); err != nil {
		t.Fatalf("failed to write issues.jsonl: %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Run analysis commands
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(bv, "--robot-triage")
			cmd.Dir = env
			if err := cmd.Run(); err != nil {
				errors <- err
			}
		}()
	}

	// Run graph commands concurrently with different formats
	formats := []string{"json", "dot", "mermaid"}
	for _, fmt := range formats {
		wg.Add(1)
		go func(format string) {
			defer wg.Done()
			cmd := exec.Command(bv, "--robot-graph", "--graph-format", format)
			cmd.Dir = env
			if err := cmd.Run(); err != nil {
				errors <- err
			}
		}(fmt)
	}

	wg.Wait()
	close(errors)

	var errCount int
	for err := range errors {
		t.Logf("concurrent analysis/graph error: %v", err)
		errCount++
	}

	if errCount > 0 {
		t.Errorf("had %d errors during concurrent analysis and graph", errCount)
	}
}

// =============================================================================
// 4. Rapid Sequential Commands
// =============================================================================

// TestRace_RapidSequentialCommands tests rapid sequential command execution.
func TestRace_RapidSequentialCommands(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Create test data
	beadsDir := filepath.Join(env, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads dir: %v", err)
	}

	issues := `{"id":"rapid-1","title":"Rapid Test","status":"open","priority":1}`
	issuesPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(issuesPath, []byte(issues), 0644); err != nil {
		t.Fatalf("failed to write issues.jsonl: %v", err)
	}

	// Run commands in rapid succession (no delay between them)
	commands := [][]string{
		{"--robot-triage"},
		{"--robot-next"},
		{"--robot-plan"},
		{"--robot-priority"},
		{"--robot-triage"},
		{"--robot-graph", "--graph-format", "json"},
		{"--robot-next"},
	}

	for i, args := range commands {
		cmd := exec.Command(bv, args...)
		cmd.Dir = env
		if err := cmd.Run(); err != nil {
			t.Errorf("command %d (%v) failed: %v", i, args, err)
		}
	}
}

// =============================================================================
// 5. Concurrent Different Output Formats
// =============================================================================

// TestRace_ConcurrentGraphFormats tests concurrent exports in different formats.
func TestRace_ConcurrentGraphFormats(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Create test data with dependencies for interesting graph
	beadsDir := filepath.Join(env, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads dir: %v", err)
	}

	issues := `{"id":"fmt-1","title":"Format Test 1","status":"open","priority":1}
{"id":"fmt-2","title":"Format Test 2","status":"open","priority":2,"dependencies":[{"depends_on_id":"fmt-1","type":"blocks"}]}
{"id":"fmt-3","title":"Format Test 3","status":"open","priority":2,"dependencies":[{"depends_on_id":"fmt-1","type":"blocks"}]}`

	issuesPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(issuesPath, []byte(issues), 0644); err != nil {
		t.Fatalf("failed to write issues.jsonl: %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 3)

	formats := []string{"json", "dot", "mermaid"}
	for _, format := range formats {
		wg.Add(1)
		go func(fmt string) {
			defer wg.Done()
			cmd := exec.Command(bv, "--robot-graph", "--graph-format", fmt)
			cmd.Dir = env
			if err := cmd.Run(); err != nil {
				errors <- err
			}
		}(format)
	}

	wg.Wait()
	close(errors)

	var errCount int
	for err := range errors {
		t.Logf("concurrent format error: %v", err)
		errCount++
	}

	if errCount > 0 {
		t.Errorf("had %d errors during concurrent format exports", errCount)
	}
}

// =============================================================================
// 6. High Concurrency Stress Test
// =============================================================================

// TestRace_HighConcurrencyStress runs many concurrent operations.
func TestRace_HighConcurrencyStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	bv := buildBvBinary(t)
	env := t.TempDir()

	// Create test data
	beadsDir := filepath.Join(env, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads dir: %v", err)
	}

	// Create larger dataset
	var issueLines []byte
	for i := 0; i < 100; i++ {
		var deps string
		if i > 0 {
			deps = `,"dependencies":[{"depends_on_id":"stress-` + string(rune('A'+(i-1)%26)) + string(rune('0'+(i-1)/26)) + `","type":"blocks"}]`
		}
		line := []byte(`{"id":"stress-` + string(rune('A'+i%26)) + string(rune('0'+i/26)) + `","title":"Stress Issue","status":"open","priority":` + string(rune('0'+i%5)) + deps + `}` + "\n")
		issueLines = append(issueLines, line...)
	}

	issuesPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(issuesPath, issueLines, 0644); err != nil {
		t.Fatalf("failed to write issues.jsonl: %v", err)
	}

	// Run 20 concurrent operations
	const numOps = 20
	var wg sync.WaitGroup
	errors := make(chan error, numOps)

	commands := [][]string{
		{"--robot-triage"},
		{"--robot-next"},
		{"--robot-plan"},
		{"--robot-graph", "--graph-format", "json"},
	}

	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			args := commands[idx%len(commands)]
			cmd := exec.Command(bv, args...)
			cmd.Dir = env
			if err := cmd.Run(); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	var errCount int
	for err := range errors {
		t.Logf("high concurrency error: %v", err)
		errCount++
	}

	// Allow some tolerance for high concurrency (resource contention is possible)
	if errCount > 2 {
		t.Errorf("too many errors during high concurrency: %d", errCount)
	}
}

// =============================================================================
// 7. Concurrent File Reading
// =============================================================================

// TestRace_ConcurrentFileReading tests that multiple processes can read
// the same beads files concurrently.
func TestRace_ConcurrentFileReading(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Create test data
	beadsDir := filepath.Join(env, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads dir: %v", err)
	}

	issues := `{"id":"read-1","title":"Read Test","status":"open","priority":1}`
	issuesPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(issuesPath, []byte(issues), 0644); err != nil {
		t.Fatalf("failed to write issues.jsonl: %v", err)
	}

	// Run multiple readers concurrently
	const numReaders = 10
	var wg sync.WaitGroup
	errors := make(chan error, numReaders)

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(bv, "--robot-triage")
			cmd.Dir = env
			if err := cmd.Run(); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	var errCount int
	for err := range errors {
		t.Logf("concurrent read error: %v", err)
		errCount++
	}

	if errCount > 0 {
		t.Errorf("had %d errors during concurrent file reading", errCount)
	}
}
