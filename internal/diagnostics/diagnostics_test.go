package diagnostics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// TestHumanizeBytes covers the documented edge cases plus a couple of
// guards against off-by-one regressions at the unit boundaries.
func TestHumanizeBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},          // 1 KB exactly
		{1024*1024 - 1, "1024 KB"}, // 1 MB minus 1 B = 1023.999... KB; rounds to 1024 KB
		{1024 * 1024, "1.00 MB"},
		{2411724, "2.30 MB"}, // 2.30 * 1024 * 1024 = 2411724.8 (rounded down)
		{847 * 1024, "847 KB"},
		{14 * 1024 * 1024 * 1024, "14.0 GB"},
		{-5, "0 B"}, // Negative input clamped.
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			got := HumanizeBytes(tc.in)
			if got != tc.want {
				t.Errorf("HumanizeBytes(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestCommaSep covers a few representative magnitudes, including the
// 100 / 1,000 / 1,000,000 boundaries.
func TestCommaSep(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1000, "1,000"},
		{14802, "14,802"},
		{1000000, "1,000,000"},
		{-1234, "-1,234"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			got := commaSep(tc.in)
			if got != tc.want {
				t.Errorf("commaSep(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestProbeEventLog_Missing verifies the "first-run" path: when the
// JSONL file does not exist, the probe returns Exists=false and zero
// counters with no error. The path is still populated so callers can
// surface where the file would live.
func TestProbeEventLog_Missing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir) // Windows.

	stats, err := ProbeEventLog()
	if err != nil {
		t.Fatalf("ProbeEventLog: %v", err)
	}
	if stats.Exists {
		t.Errorf("Exists = true on missing file, want false")
	}
	if stats.SizeBytes != 0 || stats.EntryCount != 0 {
		t.Errorf("non-zero stats on missing file: size=%d entries=%d", stats.SizeBytes, stats.EntryCount)
	}
	if stats.Path == "" {
		t.Errorf("Path empty on missing file (should still report where the file would live)")
	}
	if !strings.HasSuffix(filepath.ToSlash(stats.Path), ".bt/events.jsonl") {
		t.Errorf("Path = %q, expected to end in .bt/events.jsonl", stats.Path)
	}
}

// TestProbeEventLog_Empty verifies the "file exists but is empty"
// path: zero entries, zero bytes (or close to it), no timestamps.
func TestProbeEventLog_Empty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	path := filepath.Join(dir, ".bt", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	stats, err := ProbeEventLog()
	if err != nil {
		t.Fatalf("ProbeEventLog: %v", err)
	}
	if !stats.Exists {
		t.Errorf("Exists = false on present file, want true")
	}
	if stats.EntryCount != 0 {
		t.Errorf("EntryCount = %d on empty file, want 0", stats.EntryCount)
	}
	if !stats.OldestAt.IsZero() || !stats.NewestAt.IsZero() {
		t.Errorf("non-zero timestamps on empty file: oldest=%v newest=%v", stats.OldestAt, stats.NewestAt)
	}
}

// TestProbeEventLog_Populated writes a handful of events through the
// canonical persist path, then verifies the diagnostic recovers count
// and the oldest/newest timestamps from the first/last lines.
func TestProbeEventLog_Populated(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	path := filepath.Join(dir, ".bt", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	rb := events.NewRingBuffer(50)
	rb.SetPersistPath(path)
	// Write three events spaced out in time. We rely on persist's
	// preserve-write-order guarantee (verified in the persist tests)
	// so the diagnostic's "first line = oldest, last line = newest"
	// assumption holds.
	rb.Append(events.Event{ID: "a", Kind: events.EventCreated, BeadID: "bt-1", Repo: "bt", Title: "alpha", At: now.Add(-2 * time.Hour)})
	rb.Append(events.Event{ID: "b", Kind: events.EventClosed, BeadID: "bt-2", Repo: "bt", Title: "beta", At: now.Add(-time.Hour)})
	rb.Append(events.Event{ID: "c", Kind: events.EventEdited, BeadID: "bt-3", Repo: "bt", Title: "gamma", At: now})

	stats, err := ProbeEventLog()
	if err != nil {
		t.Fatalf("ProbeEventLog: %v", err)
	}
	if !stats.Exists {
		t.Fatal("Exists = false, want true")
	}
	if stats.EntryCount != 3 {
		t.Errorf("EntryCount = %d, want 3", stats.EntryCount)
	}
	if stats.SizeBytes <= 0 {
		t.Errorf("SizeBytes = %d, want > 0", stats.SizeBytes)
	}
	if !stats.OldestAt.Equal(now.Add(-2 * time.Hour)) {
		t.Errorf("OldestAt = %v, want %v", stats.OldestAt, now.Add(-2*time.Hour))
	}
	if !stats.NewestAt.Equal(now) {
		t.Errorf("NewestAt = %v, want %v", stats.NewestAt, now)
	}
}

// TestProbeEventLog_CountIgnoresBlankLines guards against accumulating
// counts from leading/trailing blank lines, which would diverge from
// LoadPersisted's behavior.
func TestProbeEventLog_CountIgnoresBlankLines(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	path := filepath.Join(dir, ".bt", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	now := time.Now().UTC()
	line, _ := json.Marshal(events.Event{ID: "x", Kind: events.EventCreated, BeadID: "bt-9", At: now})
	contents := []byte("\n" + string(line) + "\n\n" + string(line) + "\n\n")
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	stats, err := ProbeEventLog()
	if err != nil {
		t.Fatalf("ProbeEventLog: %v", err)
	}
	if stats.EntryCount != 2 {
		t.Errorf("EntryCount = %d, want 2 (blanks must not count)", stats.EntryCount)
	}
}

// TestProbeCache_Missing verifies the cache probe handles missing
// .bt/semantic and .bt/baseline as zero bytes, not as errors.
func TestProbeCache_Missing(t *testing.T) {
	dir := t.TempDir()
	stats, err := ProbeCache(dir)
	if err != nil {
		t.Fatalf("ProbeCache: %v", err)
	}
	if stats.SemanticIndexBytes != 0 || stats.BaselineBytes != 0 {
		t.Errorf("non-zero bytes on missing dirs: semantic=%d baseline=%d", stats.SemanticIndexBytes, stats.BaselineBytes)
	}
	// Paths should still be populated so callers can show users where
	// these would live.
	if stats.SemanticIndexPath == "" || stats.BaselinePath == "" {
		t.Errorf("paths empty on missing cache dirs: %+v", stats)
	}
}

// TestProbeCache_Populated drops a few files into .bt/semantic and
// .bt/baseline and verifies the recursive sum.
func TestProbeCache_Populated(t *testing.T) {
	dir := t.TempDir()

	semanticDir := filepath.Join(dir, ".bt", "semantic")
	if err := os.MkdirAll(semanticDir, 0o755); err != nil {
		t.Fatalf("mkdir semantic: %v", err)
	}
	if err := os.WriteFile(filepath.Join(semanticDir, "index-a.bvvi"), make([]byte, 1500), 0o644); err != nil {
		t.Fatalf("write semantic file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(semanticDir, "index-b.bvvi"), make([]byte, 500), 0o644); err != nil {
		t.Fatalf("write semantic file: %v", err)
	}

	baselineDir := filepath.Join(dir, ".bt", "baseline")
	if err := os.MkdirAll(baselineDir, 0o755); err != nil {
		t.Fatalf("mkdir baseline: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baselineDir, "baseline.json"), make([]byte, 750), 0o644); err != nil {
		t.Fatalf("write baseline file: %v", err)
	}

	stats, err := ProbeCache(dir)
	if err != nil {
		t.Fatalf("ProbeCache: %v", err)
	}
	if stats.SemanticIndexBytes != 2000 {
		t.Errorf("SemanticIndexBytes = %d, want 2000", stats.SemanticIndexBytes)
	}
	if stats.BaselineBytes != 750 {
		t.Errorf("BaselineBytes = %d, want 750", stats.BaselineBytes)
	}
}

// TestFormatEventLogSummary covers the empty-file and populated-file
// surfaces of the human-readable one-liner.
func TestFormatEventLogSummary(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		s := FormatEventLogSummary(EventLogStats{})
		want := "events log: 0 B, 0 entries, no events recorded"
		if s != want {
			t.Errorf("got %q, want %q", s, want)
		}
	})
	t.Run("present_with_range", func(t *testing.T) {
		oldest := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
		newest := time.Date(2026, 5, 4, 14, 30, 0, 0, time.UTC)
		s := FormatEventLogSummary(EventLogStats{
			Exists:     true,
			SizeBytes:  2411724, // ~2.30 MB
			EntryCount: 14802,
			OldestAt:   oldest,
			NewestAt:   newest,
		})
		// Stable substrings — exact format is tested via the components.
		for _, sub := range []string{"2.30 MB", "14,802", "since 2026-01-15", "(last: 2026-05-04)"} {
			if !strings.Contains(s, sub) {
				t.Errorf("expected %q in %q", sub, s)
			}
		}
	})
	t.Run("present_single_day", func(t *testing.T) {
		// Same oldest/newest day -> no "(last: ...)" suffix.
		when := time.Date(2026, 5, 4, 14, 30, 0, 0, time.UTC)
		s := FormatEventLogSummary(EventLogStats{
			Exists:     true,
			SizeBytes:  500,
			EntryCount: 1,
			OldestAt:   when,
			NewestAt:   when,
		})
		if strings.Contains(s, "(last:") {
			t.Errorf("unexpected '(last:' suffix on single-event log: %q", s)
		}
	})
}

// TestProbeBinary verifies the binary probe returns non-empty values
// for every field. Exact values vary by build, so we only check the
// shape.
func TestProbeBinary(t *testing.T) {
	info := ProbeBinary()
	if info.Version == "" {
		t.Error("Version empty")
	}
	if info.GoVersion == "" {
		t.Error("GoVersion empty")
	}
	if info.OS == "" {
		t.Error("OS empty")
	}
	if info.Arch == "" {
		t.Error("Arch empty")
	}
}
