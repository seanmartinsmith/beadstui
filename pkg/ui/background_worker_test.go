package ui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Dicklesworthstone/beads_viewer/pkg/recipe"
)

func TestBackgroundWorker_NewWithoutPath(t *testing.T) {
	cfg := WorkerConfig{
		BeadsPath: "",
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if worker.State() != WorkerIdle {
		t.Errorf("Expected idle state, got %v", worker.State())
	}

	if worker.GetSnapshot() != nil {
		t.Error("Expected nil snapshot initially")
	}
}

func TestBackgroundWorker_NewWithoutPath_EnvDefaults(t *testing.T) {
	t.Setenv("BV_DEBOUNCE_MS", "123")
	t.Setenv("BV_CHANNEL_BUFFER", "3")
	t.Setenv("BV_HEARTBEAT_INTERVAL_S", "9")
	t.Setenv("BV_WATCHDOG_INTERVAL_S", "11")

	worker, err := NewBackgroundWorker(WorkerConfig{BeadsPath: ""})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if worker.debounceDelay != 123*time.Millisecond {
		t.Errorf("debounceDelay=%v, want %v", worker.debounceDelay, 123*time.Millisecond)
	}
	if cap(worker.msgCh) != 3 {
		t.Errorf("cap(msgCh)=%d, want %d", cap(worker.msgCh), 3)
	}
	if worker.heartbeatInterval != 9*time.Second {
		t.Errorf("heartbeatInterval=%v, want %v", worker.heartbeatInterval, 9*time.Second)
	}
	if worker.watchdogInterval != 11*time.Second {
		t.Errorf("watchdogInterval=%v, want %v", worker.watchdogInterval, 11*time.Second)
	}
}

func TestEnvMaxLineSizeBytes(t *testing.T) {
	t.Setenv("BV_MAX_LINE_SIZE_MB", "12")
	if got := envMaxLineSizeBytes(); got != 12*1024*1024 {
		t.Errorf("envMaxLineSizeBytes()=%d, want %d", got, 12*1024*1024)
	}

	t.Setenv("BV_MAX_LINE_SIZE_MB", "-1")
	if got := envMaxLineSizeBytes(); got != 0 {
		t.Errorf("envMaxLineSizeBytes() with invalid env=%d, want %d", got, 0)
	}
}

func TestBackgroundWorker_NewWithPath(t *testing.T) {
	// Create a temporary beads file
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	// Write a valid beads file
	content := `{"id":"test-1","title":"Test Issue","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if worker.State() != WorkerIdle {
		t.Errorf("Expected idle state, got %v", worker.State())
	}
}

func TestBackgroundWorker_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}

	if err := worker.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop should be idempotent
	worker.Stop()
	worker.Stop() // Should not panic

	if worker.State() != WorkerStopped {
		t.Errorf("Expected stopped state, got %v", worker.State())
	}
}

func TestBackgroundWorker_TriggerRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// Trigger refresh and wait for processing
	worker.TriggerRefresh()

	// Wait for processing to complete
	time.Sleep(200 * time.Millisecond)

	snapshot := worker.GetSnapshot()
	if snapshot == nil {
		t.Fatal("Expected snapshot after refresh")
	}

	if len(snapshot.Issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(snapshot.Issues))
	}
}

func TestBackgroundWorker_WatcherChanged(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	ch := worker.WatcherChanged()
	if ch == nil {
		t.Error("WatcherChanged should return non-nil channel")
	}
}

func TestBackgroundWorker_WatcherChangedNil(t *testing.T) {
	// Worker without path should have nil watcher
	cfg := WorkerConfig{
		BeadsPath: "",
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if worker.WatcherChanged() != nil {
		t.Error("WatcherChanged should return nil when no watcher")
	}
}

func TestWorkerState_String(t *testing.T) {
	tests := []struct {
		state    WorkerState
		expected string
	}{
		{WorkerIdle, "0"},
		{WorkerProcessing, "1"},
		{WorkerStopped, "2"},
	}

	for _, tt := range tests {
		// Just verify the states have distinct values
		if int(tt.state) < 0 || int(tt.state) > 2 {
			t.Errorf("Unexpected state value: %v", tt.state)
		}
	}
}

func TestBackgroundWorker_ContentHashDedup(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test Issue","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// First refresh should build snapshot and set hash
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot1 := worker.GetSnapshot()
	if snapshot1 == nil {
		t.Fatal("Expected snapshot after first refresh")
	}

	hash1 := worker.LastHash()
	if hash1 == "" {
		t.Error("Expected non-empty hash after first refresh")
	}

	// Second refresh with same content should be deduped (snapshot unchanged)
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot2 := worker.GetSnapshot()
	hash2 := worker.LastHash()

	// Hash should be the same
	if hash1 != hash2 {
		t.Errorf("Hash changed unexpectedly: %s -> %s", hash1, hash2)
	}

	// Snapshot pointer should be unchanged (deduped)
	if snapshot1 != snapshot2 {
		t.Error("Snapshot pointer changed when content was unchanged - dedup failed")
	}
}

func TestBackgroundWorker_ContentHashChanges(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content1 := `{"id":"test-1","title":"Test Issue","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// First refresh
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot1 := worker.GetSnapshot()
	if snapshot1 == nil {
		t.Fatal("Expected snapshot after first refresh")
	}
	hash1 := worker.LastHash()

	// Modify the file content
	content2 := `{"id":"test-1","title":"Updated Title","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to write modified file: %v", err)
	}

	// Second refresh with different content should rebuild
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot2 := worker.GetSnapshot()
	if snapshot2 == nil {
		t.Fatal("Expected snapshot after second refresh")
	}
	hash2 := worker.LastHash()

	// Hash should be different
	if hash1 == hash2 {
		t.Error("Hash should have changed when content changed")
	}

	// Snapshot should be different
	if snapshot1 == snapshot2 {
		t.Error("Snapshot pointer should have changed when content changed")
	}

	// New snapshot should have updated title
	if snapshot2.Issues[0].Title != "Updated Title" {
		t.Errorf("Expected updated title, got %q", snapshot2.Issues[0].Title)
	}
}

func TestBackgroundWorker_MetricsSnapshot(t *testing.T) {
	t.Setenv("BV_WORKER_METRICS", "1")

	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")
	content := strings.Join([]string{
		`{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}`,
		`{"id":"test-2","title":"Test 2","status":"open","priority":2,"issue_type":"feature"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	worker.TriggerRefresh()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if worker.GetSnapshot() != nil && worker.Metrics().SnapshotVersion > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if worker.GetSnapshot() == nil {
		t.Fatal("Expected snapshot after refresh")
	}

	metrics := worker.Metrics()
	if metrics.ProcessingCount == 0 {
		t.Fatalf("expected ProcessingCount > 0, got %d", metrics.ProcessingCount)
	}
	if metrics.SnapshotVersion == 0 {
		t.Fatalf("expected SnapshotVersion > 0")
	}
	if metrics.LastSnapshotReadyAt.IsZero() {
		t.Fatal("expected LastSnapshotReadyAt to be set")
	}
	if metrics.SnapshotSizeBytes <= 0 {
		t.Fatalf("expected SnapshotSizeBytes > 0, got %d", metrics.SnapshotSizeBytes)
	}
}

func TestBackgroundWorker_IncrementalListMetrics(t *testing.T) {
	t.Setenv("BV_WORKER_METRICS", "1")

	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	var builder strings.Builder
	for i := 0; i < 10; i++ {
		builder.WriteString(fmt.Sprintf(
			`{"id":"issue-%d","title":"Issue %d","status":"open","priority":%d,"issue_type":"task"}`+"\n",
			i, i, i,
		))
	}
	if err := os.WriteFile(beadsPath, []byte(builder.String()), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	worker.TriggerRefresh()
	waitForSnapshotVersion(t, worker, 1)

	if snap := worker.GetSnapshot(); snap == nil {
		t.Fatal("Expected snapshot after first refresh")
	} else if snap.IncrementalListUsed {
		t.Fatalf("expected first snapshot to be full rebuild")
	}

	updated := builder.String()
	updated = strings.Replace(updated, `"title":"Issue 0"`, `"title":"Issue 0 updated"`, 1)
	if err := os.WriteFile(beadsPath, []byte(updated), 0644); err != nil {
		t.Fatalf("Failed to write modified file: %v", err)
	}

	worker.TriggerRefresh()
	waitForSnapshotVersion(t, worker, 2)

	snap2 := worker.GetSnapshot()
	if snap2 == nil {
		t.Fatal("Expected snapshot after second refresh")
	}
	if !snap2.IncrementalListUsed {
		t.Fatalf("expected incremental list path on second snapshot")
	}

	metrics := worker.Metrics()
	if metrics.IncrementalListCount == 0 {
		t.Fatalf("expected IncrementalListCount > 0, got %d", metrics.IncrementalListCount)
	}
	if metrics.FullListCount == 0 {
		t.Fatalf("expected FullListCount > 0, got %d", metrics.FullListCount)
	}
	if metrics.IncrementalListRatio <= 0 {
		t.Fatalf("expected IncrementalListRatio > 0, got %f", metrics.IncrementalListRatio)
	}
}

func TestBackgroundWorker_LargeDatasetWarning(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	const issueCount = 5000
	f, err := os.Create(beadsPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	writer := bufio.NewWriter(f)
	for i := 0; i < issueCount; i++ {
		line := fmt.Sprintf(`{"id":"issue-%d","title":"Issue %d","status":"open","priority":1,"issue_type":"task"}`+"\n", i, i)
		if _, err := writer.WriteString(line); err != nil {
			_ = f.Close()
			t.Fatalf("Failed to write test file: %v", err)
		}
	}
	if err := writer.Flush(); err != nil {
		_ = f.Close()
		t.Fatalf("Failed to flush test file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Failed to close test file: %v", err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	worker.TriggerRefresh()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if worker.GetSnapshot() != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	snapshot := worker.GetSnapshot()
	if snapshot == nil {
		t.Fatal("Expected snapshot after refresh")
	}
	if snapshot.DatasetTier != datasetTierLarge {
		t.Fatalf("expected datasetTierLarge, got %v", snapshot.DatasetTier)
	}
	if snapshot.SourceIssueCountHint != issueCount {
		t.Fatalf("expected SourceIssueCountHint=%d, got %d", issueCount, snapshot.SourceIssueCountHint)
	}
	if snapshot.LoadedOpenOnly {
		t.Fatalf("expected LoadedOpenOnly=false for large tier")
	}
	if snapshot.TruncatedCount != 0 {
		t.Fatalf("expected TruncatedCount=0, got %d", snapshot.TruncatedCount)
	}
	if snapshot.LargeDatasetWarning == "" {
		t.Fatal("expected LargeDatasetWarning to be populated")
	}
}

func TestBackgroundWorker_HugeDatasetOpenOnly(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	const issueCount = 20000
	f, err := os.Create(beadsPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	writer := bufio.NewWriter(f)
	openCount := 0
	for i := 0; i < issueCount; i++ {
		status := "open"
		if i%2 == 0 {
			status = "closed"
		} else {
			openCount++
		}
		line := fmt.Sprintf(`{"id":"issue-%d","title":"Issue %d","status":"%s","priority":1,"issue_type":"task"}`+"\n", i, i, status)
		if _, err := writer.WriteString(line); err != nil {
			_ = f.Close()
			t.Fatalf("Failed to write test file: %v", err)
		}
	}
	if err := writer.Flush(); err != nil {
		_ = f.Close()
		t.Fatalf("Failed to flush test file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Failed to close test file: %v", err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	worker.TriggerRefresh()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if worker.GetSnapshot() != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	snapshot := worker.GetSnapshot()
	if snapshot == nil {
		t.Fatal("Expected snapshot after refresh")
	}
	if snapshot.DatasetTier != datasetTierHuge {
		t.Fatalf("expected datasetTierHuge, got %v", snapshot.DatasetTier)
	}
	if snapshot.SourceIssueCountHint != issueCount {
		t.Fatalf("expected SourceIssueCountHint=%d, got %d", issueCount, snapshot.SourceIssueCountHint)
	}
	if !snapshot.LoadedOpenOnly {
		t.Fatalf("expected LoadedOpenOnly=true for huge tier")
	}
	if len(snapshot.Issues) != openCount {
		t.Fatalf("expected %d open issues, got %d", openCount, len(snapshot.Issues))
	}
	expectedTruncated := issueCount - openCount
	if snapshot.TruncatedCount != expectedTruncated {
		t.Fatalf("expected TruncatedCount=%d, got %d", expectedTruncated, snapshot.TruncatedCount)
	}
	if !strings.Contains(snapshot.LargeDatasetWarning, "open-only") {
		t.Fatalf("expected LargeDatasetWarning to mention open-only, got %q", snapshot.LargeDatasetWarning)
	}
}

func TestBackgroundWorker_ResetHash(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// First refresh
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot1 := worker.GetSnapshot()
	hash1 := worker.LastHash()
	if hash1 == "" {
		t.Error("Expected non-empty hash")
	}

	// Reset hash
	worker.ResetHash()
	if worker.LastHash() != "" {
		t.Error("Expected empty hash after reset")
	}

	// Refresh should rebuild even though content unchanged
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot2 := worker.GetSnapshot()
	hash2 := worker.LastHash()

	// Hash should be repopulated
	if hash2 == "" {
		t.Error("Expected hash to be set after refresh")
	}

	// Should have rebuilt (new snapshot pointer)
	if snapshot1 == snapshot2 {
		t.Error("Expected new snapshot after hash reset")
	}
}

func TestBackgroundWorker_ForceRefreshBypassesDedup(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// Build initial snapshot and set hash.
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot1 := worker.GetSnapshot()
	if snapshot1 == nil {
		t.Fatal("Expected snapshot after initial refresh")
	}

	// Second refresh with same content should be deduped.
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)
	if worker.GetSnapshot() != snapshot1 {
		t.Fatal("Expected snapshot pointer to be unchanged after dedup")
	}

	// Force refresh should rebuild even though content is unchanged.
	worker.ForceRefresh()
	time.Sleep(200 * time.Millisecond)
	if worker.GetSnapshot() == snapshot1 {
		t.Fatal("Expected new snapshot after ForceRefresh")
	}
}

func TestBackgroundWorker_SetRecipe_RebuildsOnRecipeChangeWithSameName(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	waitForSnapshot := func(prev *DataSnapshot) *DataSnapshot {
		deadline := time.Now().Add(750 * time.Millisecond)
		for time.Now().Before(deadline) {
			snap := worker.GetSnapshot()
			if snap != nil && snap != prev {
				return snap
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for snapshot change (prev=%p)", prev)
		return nil
	}

	worker.TriggerRefresh()
	snap1 := waitForSnapshot(nil)

	r1 := &recipe.Recipe{
		Name: "demo",
		Filters: recipe.FilterConfig{
			Status: []string{"open"},
		},
	}
	worker.SetRecipe(r1)
	snap2 := waitForSnapshot(snap1)
	if snap2.RecipeName != "demo" {
		t.Fatalf("expected RecipeName demo, got %q", snap2.RecipeName)
	}
	if snap2.RecipeHash != recipeFingerprint(r1) {
		t.Fatalf("expected RecipeHash %q, got %q", recipeFingerprint(r1), snap2.RecipeHash)
	}

	// Same name, different filter: should still trigger a rebuild (bv-4ilb).
	r2 := &recipe.Recipe{
		Name: "demo",
		Filters: recipe.FilterConfig{
			Status: []string{"closed"},
		},
	}
	worker.SetRecipe(r2)
	snap3 := waitForSnapshot(snap2)
	if snap3.RecipeHash != recipeFingerprint(r2) {
		t.Fatalf("expected RecipeHash %q, got %q", recipeFingerprint(r2), snap3.RecipeHash)
	}
}

func TestBackgroundWorker_SnapshotHasDataHash(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot := worker.GetSnapshot()
	if snapshot == nil {
		t.Fatal("Expected snapshot")
	}

	// Snapshot should have DataHash populated
	if snapshot.DataHash == "" {
		t.Error("Expected DataHash to be set in snapshot")
	}

	// DataHash should match LastHash
	if snapshot.DataHash != worker.LastHash() {
		t.Errorf("DataHash mismatch: snapshot=%s, worker=%s", snapshot.DataHash, worker.LastHash())
	}
}

func TestWorkerError_String(t *testing.T) {
	err := WorkerError{
		Phase:   "load",
		Cause:   os.ErrNotExist,
		Time:    time.Now(),
		Retries: 3,
	}

	s := err.Error()
	if s == "" {
		t.Error("Error() should return non-empty string")
	}

	if !strings.Contains(s, "load") {
		t.Errorf("Error() should contain phase 'load': %s", s)
	}

	if !strings.Contains(s, "3") {
		t.Errorf("Error() should contain retry count: %s", s)
	}

	// Test Unwrap
	if err.Unwrap() != os.ErrNotExist {
		t.Error("Unwrap() should return underlying error")
	}
}

func TestBackgroundWorker_LoadError(t *testing.T) {
	// Create a worker pointing to non-existent file
	cfg := WorkerConfig{
		BeadsPath:     "/nonexistent/path/beads.jsonl",
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		// Watcher creation might fail for non-existent path, which is fine
		t.Skipf("Skipping test - watcher creation failed: %v", err)
	}
	defer worker.Stop()

	// Trigger refresh
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	// Should have no snapshot (load failed)
	if worker.GetSnapshot() != nil {
		t.Error("Expected nil snapshot when file doesn't exist")
	}

	// Should have recorded error
	lastErr := worker.LastError()
	if lastErr == nil {
		t.Error("Expected error to be recorded")
	} else {
		if lastErr.Phase != "load" {
			t.Errorf("Expected phase 'load', got %q", lastErr.Phase)
		}
	}
}

func TestBackgroundWorker_ErrorRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	// Start with no file
	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// First refresh should fail (no file)
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	if worker.GetSnapshot() != nil {
		t.Error("Expected nil snapshot when file doesn't exist")
	}

	// Now create the file
	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Reset hash to force reload
	worker.ResetHash()

	// Second refresh should succeed
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot := worker.GetSnapshot()
	if snapshot == nil {
		t.Fatal("Expected snapshot after file created")
	}

	// Error should be cleared
	if worker.LastError() != nil {
		t.Error("Expected error to be cleared on success")
	}
}

func TestBackgroundWorker_SafeCompute(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// Test that safeCompute catches panics
	err2 := worker.safeCompute("test", func() error {
		panic("intentional panic for testing")
	})

	if err2 == nil {
		t.Error("safeCompute should catch panics")
	}

	if err2.Phase != "test" {
		t.Errorf("Expected phase 'test', got %q", err2.Phase)
	}

	// Verify worker still functional after panic
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	if worker.GetSnapshot() == nil {
		t.Error("Worker should still be functional after panic recovery")
	}
}

func TestHashPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "short string (empty hash)",
			input:    "empty",
			expected: "empty",
		},
		{
			name:     "exactly 16 chars",
			input:    "1234567890123456",
			expected: "1234567890123456",
		},
		{
			name:     "longer than 16 chars",
			input:    "8b423072ec4730921a2b3c4d5e6f7890",
			expected: "8b423072ec473092",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashPrefix(tt.input)
			if result != tt.expected {
				t.Errorf("hashPrefix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBackgroundWorker_StartAfterStop(t *testing.T) {
	// Test that Start() returns error after Stop() has been called
	cfg := WorkerConfig{
		BeadsPath: "", // No watcher needed for this test
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}

	// Start and stop the worker
	if err := worker.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	worker.Stop()

	// Attempting to start again should fail
	err = worker.Start()
	if err == nil {
		t.Error("Start() after Stop() should return an error")
	}

	// Verify the worker is stopped
	if worker.State() != WorkerStopped {
		t.Errorf("Expected WorkerStopped state, got %v", worker.State())
	}
}

func TestBackgroundWorker_ConcurrentTrigger(t *testing.T) {
	// Test that concurrent TriggerRefresh calls don't cause duplicate processing
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Fire multiple TriggerRefresh calls concurrently
	// The fix ensures only one process() runs at a time, others mark dirty
	for i := 0; i < 5; i++ {
		go worker.TriggerRefresh()
	}

	// Wait for processing to complete
	time.Sleep(400 * time.Millisecond)

	// Worker should still be in idle state (not stuck in processing)
	if worker.State() != WorkerIdle {
		t.Errorf("Expected idle state after concurrent triggers, got %v", worker.State())
	}

	// Should have a valid snapshot
	if worker.GetSnapshot() == nil {
		t.Error("Expected snapshot after concurrent triggers")
	}
}

func TestBackgroundWorker_Phase2Async(t *testing.T) {
	// Test that Phase 2 analysis runs asynchronously (bv-e3ub)
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	// Create a file with some dependencies to make Phase 2 analysis non-trivial
	content := `{"id":"test-1","title":"Root","status":"open","priority":1,"issue_type":"task"}
{"id":"test-2","title":"Child","status":"open","priority":2,"issue_type":"task","dependencies":[{"depends_on":"test-1","type":"blocks"}]}
`
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// Trigger refresh and wait for snapshot
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot := worker.GetSnapshot()
	if snapshot == nil {
		t.Fatal("Expected snapshot after refresh")
	}

	// Snapshot should exist with analysis
	if snapshot.Analysis == nil {
		t.Fatal("Expected Analysis in snapshot")
	}

	// Wait for Phase 2 to complete using the GraphStats API
	snapshot.Analysis.WaitForPhase2()

	// After waiting, Phase 2 should be ready
	if !snapshot.Analysis.IsPhase2Ready() {
		t.Error("Phase 2 should be ready after WaitForPhase2()")
	}
}

func TestBackgroundWorker_Phase2UpdateMsgDelivered(t *testing.T) {
	// Verify that the worker emits Phase2UpdateMsg asynchronously with a matching hash (bv-j97z).
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	// Small dependency graph; Phase 2 should typically run async.
	content := `{"id":"root","title":"Root","status":"open","priority":1,"issue_type":"task"}
{"id":"child","title":"Child","status":"open","priority":2,"issue_type":"task","dependencies":[{"depends_on_id":"root","type":"blocks"}]}
`
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 10 * time.Millisecond,
		MessageBuffer: 16,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	start := time.Now()
	t.Logf("[%s] EVENT=trigger_refresh", start.UTC().Format(time.RFC3339Nano))
	worker.TriggerRefresh()

	var snapshot *DataSnapshot
	var phase2Hash string
	var snapshotAt time.Time
	var phase2At time.Time

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()

	for snapshot == nil || phase2Hash == "" {
		select {
		case <-deadline.C:
			t.Fatalf("timed out waiting for SnapshotReadyMsg and Phase2UpdateMsg (snapshot=%v, phase2Hash=%q)", snapshot != nil, phase2Hash)
		case msg := <-worker.Messages():
			switch m := msg.(type) {
			case SnapshotReadyMsg:
				if m.Snapshot == nil {
					continue
				}
				snapshot = m.Snapshot
				snapshotAt = time.Now()
				t.Logf("[%s] EVENT=snapshot_ready elapsed_ms=%.3f issues=%d hash=%s",
					snapshotAt.UTC().Format(time.RFC3339Nano),
					float64(snapshotAt.Sub(start).Microseconds())/1000.0,
					len(snapshot.Issues),
					hashPrefix(snapshot.DataHash),
				)

				// If Phase 2 already completed by the time we received the snapshot,
				// the update message may be suppressed (no work to signal).
				if snapshot.Analysis != nil && snapshot.Analysis.IsPhase2Ready() {
					t.Skip("phase 2 completed before snapshot delivery; Phase2UpdateMsg may not be emitted for this dataset")
				}

			case Phase2UpdateMsg:
				phase2Hash = m.DataHash
				phase2At = time.Now()
				t.Logf("[%s] EVENT=phase2_update elapsed_ms=%.3f hash=%s",
					phase2At.UTC().Format(time.RFC3339Nano),
					float64(phase2At.Sub(start).Microseconds())/1000.0,
					hashPrefix(phase2Hash),
				)
			}
		}
	}

	if snapshot.DataHash == "" {
		t.Fatal("expected snapshot DataHash to be set")
	}
	if phase2Hash != snapshot.DataHash {
		t.Fatalf("phase2 hash mismatch: got %s, want %s", phase2Hash, snapshot.DataHash)
	}
}

func TestBackgroundWorker_Phase2NoSendAfterStop(t *testing.T) {
	// Test that runPhase2Analysis doesn't send if worker is stopped (bv-e3ub)
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}

	// Trigger refresh
	worker.TriggerRefresh()

	// Stop immediately (before Phase 2 can complete)
	worker.Stop()

	// Worker should be stopped
	if worker.State() != WorkerStopped {
		t.Errorf("Expected stopped state, got %v", worker.State())
	}

	// The test passes if we reach here without panicking
	// (runPhase2Analysis should gracefully handle stopped worker)
}

func TestDataSnapshot_GetGraphStats(t *testing.T) {
	// Test GetGraphStats helper method (bv-e3ub)

	// Test nil snapshot
	var nilSnapshot *DataSnapshot
	if nilSnapshot.GetGraphStats() != nil {
		t.Error("GetGraphStats on nil snapshot should return nil")
	}

	// Test snapshot with nil Analysis
	emptySnapshot := &DataSnapshot{}
	if emptySnapshot.GetGraphStats() != nil {
		t.Error("GetGraphStats with nil Analysis should return nil")
	}
}

func waitForBackgroundWorkerMsg(t *testing.T, worker *BackgroundWorker, timeout time.Duration, predicate func(tea.Msg) bool) tea.Msg {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case msg := <-worker.Messages():
			if predicate(msg) {
				return msg
			}
		case <-timer.C:
			t.Fatalf("timeout waiting for BackgroundWorker message (%v)", timeout)
		}
	}
}

func waitForSnapshotVersion(t *testing.T, worker *BackgroundWorker, minVersion uint64) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if worker.Metrics().SnapshotVersion >= minVersion {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for snapshot version %d (got %d)", minVersion, worker.Metrics().SnapshotVersion)
}

func TestBackgroundWorker_MalformedJSON_WarnsAndContinues(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"ok-1","title":"Ok 1","status":"open","priority":1,"issue_type":"task"}
not json
{"id":"bad-only-id"}
{"id":"ok-2","title":"Ok 2","status":"open","priority":2,"issue_type":"task"}
`
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	worker.TriggerRefresh()

	msg := waitForBackgroundWorkerMsg(t, worker, 2*time.Second, func(m tea.Msg) bool {
		_, ok := m.(SnapshotReadyMsg)
		return ok
	})

	ready := msg.(SnapshotReadyMsg)
	if ready.Snapshot == nil {
		t.Fatal("Expected non-nil snapshot")
	}
	if got, want := len(ready.Snapshot.Issues), 2; got != want {
		t.Fatalf("Expected %d issues, got %d", want, got)
	}
	if ready.Snapshot.LoadWarningCount == 0 {
		t.Error("Expected LoadWarningCount > 0 for malformed/invalid lines")
	}
	if worker.LastError() != nil {
		t.Errorf("Expected LastError to be nil for parse warnings, got: %v", worker.LastError())
	}
}

func TestBackgroundWorker_PreservesSnapshotOnPermissionErrorAndRecovers(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content1 := `{"id":"test-1","title":"Initial","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// Build initial snapshot.
	worker.TriggerRefresh()
	msg1 := waitForBackgroundWorkerMsg(t, worker, 2*time.Second, func(m tea.Msg) bool {
		_, ok := m.(SnapshotReadyMsg)
		return ok
	})
	snapshot1 := msg1.(SnapshotReadyMsg).Snapshot
	if snapshot1 == nil {
		t.Fatal("Expected initial snapshot")
	}

	// Make file unreadable.
	if err := os.Chmod(beadsPath, 0000); err != nil {
		t.Skipf("chmod not supported: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(beadsPath, 0644)
	})

	worker.TriggerRefresh()
	msgErr := waitForBackgroundWorkerMsg(t, worker, 2*time.Second, func(m tea.Msg) bool {
		_, ok := m.(SnapshotErrorMsg)
		return ok
	})
	errMsg := msgErr.(SnapshotErrorMsg)
	if errMsg.Err == nil {
		t.Fatal("Expected SnapshotErrorMsg to contain error")
	}
	if !errMsg.Recoverable {
		t.Error("Expected Recoverable=true for permission errors")
	}

	// Snapshot must be preserved after an error.
	if worker.GetSnapshot() != snapshot1 {
		t.Fatal("Expected previous snapshot to be preserved on load error")
	}
	if worker.LastError() == nil {
		t.Fatal("Expected LastError to be set after load error")
	}

	// Restore permissions and write new content to force a successful rebuild.
	if err := os.Chmod(beadsPath, 0644); err != nil {
		t.Fatalf("Failed to restore file permissions: %v", err)
	}

	content2 := `{"id":"test-1","title":"Recovered","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to write recovered file: %v", err)
	}
	worker.ResetHash()

	worker.TriggerRefresh()
	msg2 := waitForBackgroundWorkerMsg(t, worker, 2*time.Second, func(m tea.Msg) bool {
		_, ok := m.(SnapshotReadyMsg)
		return ok
	})
	snapshot2 := msg2.(SnapshotReadyMsg).Snapshot
	if snapshot2 == nil {
		t.Fatal("Expected snapshot after recovery")
	}
	if snapshot2 == snapshot1 {
		t.Fatal("Expected new snapshot pointer after recovery rebuild")
	}
	if got, want := snapshot2.Issues[0].Title, "Recovered"; got != want {
		t.Fatalf("Expected updated title %q, got %q", want, got)
	}
	if worker.LastError() != nil {
		t.Fatalf("Expected LastError to be cleared after recovery, got: %v", worker.LastError())
	}
}

func TestBackgroundWorker_HeartbeatUpdatesHealth(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath:         beadsPath,
		DebounceDelay:     10 * time.Millisecond,
		HeartbeatInterval: 10 * time.Millisecond,
		HeartbeatTimeout:  200 * time.Millisecond,
		WatchdogInterval:  time.Hour, // keep deterministic in tests
		ProcessingTimeout: time.Hour,
		MaxRecoveries:     3,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	h1 := worker.Health()
	if !h1.Started || !h1.Alive || h1.LastHeartbeat.IsZero() {
		t.Fatalf("expected started+alive health, got: %+v", h1)
	}

	time.Sleep(30 * time.Millisecond)
	h2 := worker.Health()
	if !h2.LastHeartbeat.After(h1.LastHeartbeat) {
		t.Fatalf("expected heartbeat to advance: %v -> %v", h1.LastHeartbeat, h2.LastHeartbeat)
	}
}

func TestBackgroundWorker_CheckHealth_TriggersRecoveryOnMissedHeartbeat(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath:         beadsPath,
		DebounceDelay:     10 * time.Millisecond,
		HeartbeatInterval: time.Hour, // suppress updates so we can force "missed"
		HeartbeatTimeout:  10 * time.Millisecond,
		WatchdogInterval:  time.Hour, // keep deterministic in tests
		ProcessingTimeout: time.Hour,
		MaxRecoveries:     3,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	worker.mu.Lock()
	worker.lastHeartbeat = time.Now().Add(-time.Second)
	worker.mu.Unlock()

	worker.checkHealth(time.Now())

	if got := worker.Health().RecoveryCount; got < 1 {
		t.Fatalf("expected recoveryCount to increment, got %d", got)
	}
	if worker.State() == WorkerStopped {
		t.Fatal("expected worker to remain running after recovery attempt")
	}
}

func TestBackgroundWorker_MaybeIdleGC_TriggersAfterThreshold(t *testing.T) {
	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath: "",
		IdleGC: &IdleGCConfig{
			Enabled:     true,
			Threshold:   5 * time.Second,
			CheckEvery:  time.Hour, // avoid nondeterministic ticker behavior in unit tests
			MinInterval: 30 * time.Second,
			GCPercent:   200,
		},
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	gcCalls := 0
	worker.idleGCFunc = func() { gcCalls++ }

	now := time.Now()
	worker.recordActivityAt(now.Add(-10 * time.Second))

	worker.maybeIdleGC(now)

	if gcCalls != 1 {
		t.Fatalf("expected idle GC to run once, ran %d times", gcCalls)
	}
	if got := worker.Health().IdleGCCount; got != 1 {
		t.Fatalf("expected IdleGCCount=1, got %d", got)
	}

	// Enforce min-interval gating.
	worker.maybeIdleGC(now.Add(1 * time.Second))
	if gcCalls != 1 {
		t.Fatalf("expected idle GC to be gated by MinInterval, ran %d times", gcCalls)
	}
}

func TestBackgroundWorker_MaybeIdleGC_DoesNotRunWhenProcessing(t *testing.T) {
	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath: "",
		IdleGC: &IdleGCConfig{
			Enabled:     true,
			Threshold:   5 * time.Second,
			CheckEvery:  time.Hour,
			MinInterval: 30 * time.Second,
			GCPercent:   200,
		},
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	gcCalls := 0
	worker.idleGCFunc = func() { gcCalls++ }

	now := time.Now()
	worker.recordActivityAt(now.Add(-10 * time.Second))

	worker.mu.Lock()
	worker.state = WorkerProcessing
	worker.mu.Unlock()

	worker.maybeIdleGC(now)
	if gcCalls != 0 {
		t.Fatalf("expected idle GC to not run during processing, ran %d times", gcCalls)
	}
	if got := worker.Health().IdleGCCount; got != 0 {
		t.Fatalf("expected IdleGCCount=0, got %d", got)
	}
}

func TestBackgroundWorker_AttemptRecovery_GivesUpAfterMaxRecoveries(t *testing.T) {
	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath:         "",
		MaxRecoveries:     1,
		HeartbeatInterval: time.Hour,
		WatchdogInterval:  time.Hour,
		HeartbeatTimeout:  10 * time.Millisecond,
		ProcessingTimeout: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	worker.attemptRecovery("test-1")

	worker.attemptRecovery("test-2")
	_ = waitForBackgroundWorkerMsg(t, worker, 2*time.Second, func(m tea.Msg) bool {
		msg, ok := m.(SnapshotErrorMsg)
		return ok && !msg.Recoverable
	}).(SnapshotErrorMsg)

	if worker.State() != WorkerStopped {
		t.Fatalf("expected worker to be stopped after giving up, got state=%v", worker.State())
	}
}

func TestStress_SustainedWrites(t *testing.T) {
	if os.Getenv("PERF_TEST") != "1" {
		t.Skip("set PERF_TEST=1 to run 10+ minute stress tests")
	}
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	const issueCount = 200
	if err := writeStressIssuesFile(beadsPath, issueCount, 0, "init"); err != nil {
		t.Fatalf("failed to write initial beads file: %v", err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
		MessageBuffer: 16,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var snapshotCount atomic.Int64
	var errorCount atomic.Int64
	go countWorkerMessages(worker, &snapshotCount, &errorCount)

	var initialMem runtime.MemStats
	initialGoros := runtime.NumGoroutine()
	runtime.ReadMemStats(&initialMem)
	initialFDs, fdOK := procFDCount()

	duration := requireTestDurationOrSkip(t, 10*time.Minute, 30*time.Second)
	end := time.Now().Add(duration)
	writeInterval := 100 * time.Millisecond

	// Ensure the worker processes at least one file-change event before we start the long loop.
	if err := writeStressIssuesFile(beadsPath, issueCount, 0, "warmup"); err != nil {
		t.Fatalf("failed to write warmup beads file: %v", err)
	}
	waitForAtomicAtLeast(t, 10*time.Second, &snapshotCount, 1)

	writeCount := 0
	for now := time.Now(); now.Before(end); now = time.Now() {
		// Rewrite with stable issue count (stress file watching + parsing + analysis,
		// without unbounded memory growth from an ever-expanding dataset).
		changeIndex := writeCount % issueCount
		if err := writeStressIssuesFile(beadsPath, issueCount, changeIndex, fmt.Sprintf("tick-%d", writeCount)); err != nil {
			t.Fatalf("failed to write beads file: %v", err)
		}
		writeCount++

		// Sample every minute.
		if writeCount%600 == 0 {
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			goros := runtime.NumGoroutine()
			if fdOK {
				fds, _ := procFDCount()
				t.Logf("Minute %d: heap=%dMB goros=%d fds=%d writes=%d", writeCount/600, mem.Alloc/1024/1024, goros, fds, writeCount)
			} else {
				t.Logf("Minute %d: heap=%dMB goros=%d writes=%d", writeCount/600, mem.Alloc/1024/1024, goros, writeCount)
			}
		}

		time.Sleep(writeInterval)
	}

	worker.Stop()

	// Final checks.
	runtime.GC()
	time.Sleep(1 * time.Second)

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	finalGoros := runtime.NumGoroutine()
	finalFDs := 0
	if fdOK {
		finalFDs, _ = procFDCount()
	}

	memDelta := int64(finalMem.Alloc) - int64(initialMem.Alloc)
	goroDelta := finalGoros - initialGoros
	fdDelta := finalFDs - initialFDs

	t.Logf("Final: heap=%dMB (delta=%dMB) goros=%d (delta=%d) fds=%d (delta=%d) writes=%d",
		finalMem.Alloc/1024/1024, memDelta/1024/1024,
		finalGoros, goroDelta,
		finalFDs, fdDelta,
		writeCount,
	)

	if got := snapshotCount.Load(); got < 1 {
		t.Fatalf("expected at least one SnapshotReadyMsg, got %d", got)
	}
	if got := errorCount.Load(); got != 0 {
		t.Fatalf("expected no SnapshotErrorMsg, got %d", got)
	}
	if goroDelta > 10 {
		t.Fatalf("goroutine leak: delta=%d (want <= 10)", goroDelta)
	}
	if memDelta > 100*1024*1024 {
		t.Fatalf("memory growth too high: delta=%dMB (want <= 100MB)", memDelta/1024/1024)
	}
	if fdOK && fdDelta > 10 {
		t.Fatalf("file descriptor leak: delta=%d (want <= 10)", fdDelta)
	}
}

func TestStress_BurstWrites(t *testing.T) {
	if os.Getenv("PERF_TEST") != "1" {
		t.Skip("set PERF_TEST=1 to run 10+ minute stress tests")
	}
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	const issueCount = 200
	if err := writeStressIssuesFile(beadsPath, issueCount, 0, "init"); err != nil {
		t.Fatalf("failed to write initial beads file: %v", err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
		MessageBuffer: 16,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var snapshotCount atomic.Int64
	var errorCount atomic.Int64
	go countWorkerMessages(worker, &snapshotCount, &errorCount)

	var initialMem runtime.MemStats
	initialGoros := runtime.NumGoroutine()
	runtime.ReadMemStats(&initialMem)
	initialFDs, fdOK := procFDCount()

	duration := requireTestDurationOrSkip(t, 5*time.Minute, 30*time.Second)
	end := time.Now().Add(duration)

	writeCount := 0
	for time.Now().Before(end) {
		// Burst of 10 quick writes (agent completing task).
		for i := 0; i < 10; i++ {
			changeIndex := writeCount % issueCount
			if err := writeStressIssuesFile(beadsPath, issueCount, changeIndex, fmt.Sprintf("burst-%d", writeCount)); err != nil {
				t.Fatalf("failed to write beads file: %v", err)
			}
			writeCount++
			time.Sleep(10 * time.Millisecond)
		}

		// Quiet period (agent thinking).
		time.Sleep(2 * time.Second)
	}

	worker.Stop()
	runtime.GC()
	time.Sleep(1 * time.Second)

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	finalGoros := runtime.NumGoroutine()
	finalFDs := 0
	if fdOK {
		finalFDs, _ = procFDCount()
	}

	memDelta := int64(finalMem.Alloc) - int64(initialMem.Alloc)
	goroDelta := finalGoros - initialGoros
	fdDelta := finalFDs - initialFDs

	t.Logf("Final: heap=%dMB (delta=%dMB) goros=%d (delta=%d) fds=%d (delta=%d) writes=%d snapshots=%d errors=%d",
		finalMem.Alloc/1024/1024, memDelta/1024/1024,
		finalGoros, goroDelta,
		finalFDs, fdDelta,
		writeCount,
		snapshotCount.Load(),
		errorCount.Load(),
	)

	if got := snapshotCount.Load(); got < 1 {
		t.Fatalf("expected at least one SnapshotReadyMsg, got %d", got)
	}
	if got := errorCount.Load(); got != 0 {
		t.Fatalf("expected no SnapshotErrorMsg, got %d", got)
	}
	if goroDelta > 10 {
		t.Fatalf("goroutine leak: delta=%d (want <= 10)", goroDelta)
	}
	if memDelta > 100*1024*1024 {
		t.Fatalf("memory growth too high: delta=%dMB (want <= 100MB)", memDelta/1024/1024)
	}
	if fdOK && fdDelta > 10 {
		t.Fatalf("file descriptor leak: delta=%d (want <= 10)", fdDelta)
	}
}

func TestStress_MemoryPressure(t *testing.T) {
	if os.Getenv("PERF_TEST") != "1" {
		t.Skip("set PERF_TEST=1 to run 10+ minute stress tests")
	}
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	// Simulate constrained memory environment.
	oldLimit := debug.SetMemoryLimit(256 * 1024 * 1024) // 256MB
	t.Cleanup(func() {
		debug.SetMemoryLimit(oldLimit)
	})

	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	const issueCount = 2000
	if err := writeStressIssuesFile(beadsPath, issueCount, 0, "init"); err != nil {
		t.Fatalf("failed to write initial beads file: %v", err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
		MessageBuffer: 16,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	worker.TriggerRefresh()
	timeout := clampToDeadline(t, 60*time.Second, 30*time.Second)
	_ = waitForBackgroundWorkerMsg(t, worker, timeout, func(m tea.Msg) bool {
		msg, ok := m.(SnapshotReadyMsg)
		return ok && msg.Snapshot != nil
	})
}

func countWorkerMessages(worker *BackgroundWorker, snapshotCount, errorCount *atomic.Int64) {
	if worker == nil {
		return
	}
	for {
		select {
		case <-worker.Done():
			return
		case msg := <-worker.Messages():
			switch msg.(type) {
			case SnapshotReadyMsg:
				snapshotCount.Add(1)
			case SnapshotErrorMsg:
				errorCount.Add(1)
			}
		}
	}
}

func waitForAtomicAtLeast(t *testing.T, timeout time.Duration, counter *atomic.Int64, min int64) {
	t.Helper()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for {
		if counter.Load() >= min {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("timed out waiting for counter >= %d (got %d)", min, counter.Load())
		case <-tick.C:
		}
	}
}

func requireTestDurationOrSkip(t *testing.T, desired, safetyWindow time.Duration) time.Duration {
	t.Helper()
	if deadline, ok := t.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < desired+safetyWindow {
			t.Skipf("need >= %s remaining before test deadline (have %s); run with -timeout >= %s", desired+safetyWindow, remaining, desired+safetyWindow)
		}
	}
	return desired
}

func clampToDeadline(t *testing.T, desired, safetyWindow time.Duration) time.Duration {
	t.Helper()
	if deadline, ok := t.Deadline(); ok {
		remaining := time.Until(deadline) - safetyWindow
		if remaining <= 0 {
			t.Skip("insufficient time before test deadline; increase -timeout")
		}
		if remaining < desired {
			return remaining
		}
	}
	return desired
}

func procFDCount() (int, bool) {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return 0, false
	}
	return len(entries), true
}

func writeStressIssuesFile(path string, issueCount int, mutateIndex int, mutateSuffix string) error {
	if issueCount <= 0 {
		return fmt.Errorf("invalid issueCount: %d", issueCount)
	}
	if mutateIndex < 0 || mutateIndex >= issueCount {
		return fmt.Errorf("invalid mutateIndex: %d", mutateIndex)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := 0; i < issueCount; i++ {
		title := fmt.Sprintf("Stress Issue %d", i)
		if i == mutateIndex {
			title = fmt.Sprintf("%s (%s)", title, mutateSuffix)
		}

		// Keep the payload small and stable (stress parsing/analysis without inflating memory).
		// created_at / updated_at are optional (zero values are accepted), but including updated_at
		// forces content to change while remaining valid JSON.
		line := fmt.Sprintf(
			`{"id":"stress-%d","title":%q,"status":"open","priority":1,"issue_type":"task","updated_at":%q}`+"\n",
			i, title, now,
		)
		if _, err := f.WriteString(line); err != nil {
			return err
		}
	}

	return f.Sync()
}
