// Package diagnostics provides read-only probes for `bt status` and
// `bt robot health` — disk usage, on-disk file inventories, and binary
// metadata. Every probe handles "missing file/dir" as zero values rather
// than as an error, so first-run installs render cleanly.
//
// This package MUST stay free of TUI dependencies (charm/bubbletea) so
// the human and robot surfaces can both consume the same probes without
// pulling renderer state into the data path.
package diagnostics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
	"github.com/seanmartinsmith/beadstui/pkg/version"
)

// EventLogStats summarizes the on-disk events.jsonl file. Path is always
// populated (even when the file is missing) so callers can show users
// where the log lives. SizeBytes/EntryCount are 0 when the file does
// not exist; OldestAt/NewestAt are zero values in that case.
type EventLogStats struct {
	Path       string    `json:"path"`
	Exists     bool      `json:"exists"`
	SizeBytes  int64     `json:"size_bytes"`
	EntryCount int       `json:"entry_count"`
	OldestAt   time.Time `json:"oldest_at,omitempty"`
	NewestAt   time.Time `json:"newest_at,omitempty"`
}

// CacheStats summarizes bt's runtime cache directories under .bt/. Each
// field is the cumulative size of the named directory (recursive); 0
// when the dir does not exist. Paths echo the absolute path probed,
// useful when the user wants to inspect the cache manually.
type CacheStats struct {
	SemanticIndexPath  string `json:"semantic_index_path"`
	SemanticIndexBytes int64  `json:"semantic_index_bytes"`
	BaselinePath       string `json:"baseline_path"`
	BaselineBytes      int64  `json:"baseline_bytes"`
}

// BinaryInfo describes the running bt binary.
type BinaryInfo struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// ProbeBinary returns metadata about the running bt binary. Cheap; never
// errors.
func ProbeBinary() BinaryInfo {
	return BinaryInfo{
		Version:   version.Version,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// ProbeEventLog returns size + entry count + oldest/newest timestamps
// for ~/.bt/events.jsonl. Missing file is not an error — the returned
// stats carry Exists=false with zero counters.
//
// Entry counting uses a streaming line scan (NOT events.LoadPersisted)
// to keep memory bounded on large logs: only the first and last non-
// empty lines are JSON-decoded for timestamps.
func ProbeEventLog() (EventLogStats, error) {
	path, err := events.DefaultPersistPath()
	if err != nil {
		return EventLogStats{}, fmt.Errorf("resolve events log path: %w", err)
	}
	stats := EventLogStats{Path: path}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil
		}
		return stats, fmt.Errorf("stat events log: %w", err)
	}
	stats.Exists = true
	stats.SizeBytes = info.Size()

	count, oldest, newest, err := scanEventLog(path)
	if err != nil {
		return stats, err
	}
	stats.EntryCount = count
	stats.OldestAt = oldest
	stats.NewestAt = newest
	return stats, nil
}

// scanEventLog streams the events JSONL once, counting non-empty lines
// and decoding only the first and last non-empty line for the At
// timestamp. Corrupt timestamp lines fall back to zero values rather
// than failing the whole probe — the diagnostic's job is to inform,
// not to validate.
func scanEventLog(path string) (count int, oldest, newest time.Time, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, time.Time{}, time.Time{}, fmt.Errorf("open events log: %w", err)
	}
	defer f.Close()

	var firstLine, lastLine []byte

	scanner := bufio.NewScanner(f)
	// Match the persist reader's buffer (1 MiB) so unusually long
	// comment-summary lines don't truncate the count or surface a
	// scanner error here when the persist path tolerates them.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		count++
		if firstLine == nil {
			firstLine = append([]byte(nil), line...)
		}
		// Always overwrite lastLine — cheap, and we only need the final value.
		lastLine = append(lastLine[:0], line...)
	}
	if scanErr := scanner.Err(); scanErr != nil {
		// Prefer returning what we have over erroring out — this is a
		// diagnostic, not a load.
		return count, time.Time{}, time.Time{}, fmt.Errorf("scan events log: %w", scanErr)
	}

	if firstLine != nil {
		if t, ok := decodeAt(firstLine); ok {
			oldest = t
		}
	}
	if lastLine != nil {
		if t, ok := decodeAt(lastLine); ok {
			newest = t
		}
	}
	return count, oldest, newest, nil
}

// decodeAt extracts the At field from a single events.jsonl line. The
// per-line struct mirrors the on-wire shape but only carries the
// timestamp, so the decoder is cheap and tolerant of unknown fields.
func decodeAt(line []byte) (time.Time, bool) {
	var rec struct {
		At time.Time `json:"At"`
	}
	if err := json.Unmarshal(line, &rec); err != nil {
		return time.Time{}, false
	}
	if rec.At.IsZero() {
		return time.Time{}, false
	}
	return rec.At, true
}

// ProbeCache returns the recursive size of bt's runtime cache
// directories under projectDir/.bt/. Missing directories are 0 bytes,
// not errors.
//
// projectDir should be the cwd or workspace root the caller has already
// resolved. Pass an empty string to use the cwd.
func ProbeCache(projectDir string) (CacheStats, error) {
	if projectDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return CacheStats{}, fmt.Errorf("getwd: %w", err)
		}
		projectDir = cwd
	}
	semanticPath := filepath.Join(projectDir, ".bt", "semantic")
	baselinePath := filepath.Join(projectDir, ".bt", "baseline")

	semanticBytes, err := dirSize(semanticPath)
	if err != nil {
		return CacheStats{}, fmt.Errorf("size semantic cache: %w", err)
	}
	baselineBytes, err := dirSize(baselinePath)
	if err != nil {
		return CacheStats{}, fmt.Errorf("size baseline cache: %w", err)
	}
	return CacheStats{
		SemanticIndexPath:  semanticPath,
		SemanticIndexBytes: semanticBytes,
		BaselinePath:       baselinePath,
		BaselineBytes:      baselineBytes,
	}, nil
}

// dirSize sums file sizes under root. Missing root returns 0, nil.
// Walk errors on individual entries are ignored so a single permission
// glitch doesn't break the whole probe.
func dirSize(root string) (int64, error) {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if !info.IsDir() {
		return info.Size(), nil
	}

	var total int64
	walkErr := filepath.Walk(root, func(_ string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			// Per-entry errors are non-fatal — skip and keep walking.
			if fi != nil && fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	if walkErr != nil {
		// Walk only returns errors the WalkFunc bubbled up, which we
		// already swallow above. Any error here is unexpected.
		return total, walkErr
	}
	return total, nil
}

// HumanizeBytes renders a byte count as a short, base-1024 string with
// up to one decimal: "0 B", "847 KB", "2.3 MB", "14 GB". Negative
// values are clamped to 0 (signed input is a defensive carry-over —
// os.FileInfo.Size() returns int64).
//
// Format mirrors common "du -h" output for bytes that aren't a clean
// power of two so users can sanity-check against an independent tool.
func HumanizeBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	suffixes := [...]string{"KB", "MB", "GB", "TB", "PB"}
	if exp >= len(suffixes) {
		exp = len(suffixes) - 1
	}
	val := float64(n) / float64(div)
	if val >= 100 {
		// >=100 KB/MB/GB: drop the decimal, "847 KB" not "847.0 KB".
		return fmt.Sprintf("%.0f %s", val, suffixes[exp])
	}
	if val >= 10 {
		return fmt.Sprintf("%.1f %s", val, suffixes[exp])
	}
	return fmt.Sprintf("%.2f %s", val, suffixes[exp])
}

// FormatEventLogSummary renders a one-line human summary suitable for
// `bt status`. Examples:
//
//	"events log: 2.3 MB, 14,802 entries since 2026-01-15 (last: 2026-05-04)"
//	"events log: 0 B, 0 entries, no events recorded"
func FormatEventLogSummary(s EventLogStats) string {
	if !s.Exists || s.EntryCount == 0 {
		return "events log: 0 B, 0 entries, no events recorded"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "events log: %s, %s entries", HumanizeBytes(s.SizeBytes), commaSep(s.EntryCount))
	if !s.OldestAt.IsZero() {
		fmt.Fprintf(&b, " since %s", s.OldestAt.UTC().Format("2006-01-02"))
	}
	if !s.NewestAt.IsZero() && !s.OldestAt.Equal(s.NewestAt) {
		fmt.Fprintf(&b, " (last: %s)", s.NewestAt.UTC().Format("2006-01-02"))
	}
	return b.String()
}

// commaSep renders an integer with thousands-separator commas. Used for
// entry counts where a raw 14802 reads worse than 14,802. ASCII-only by
// design (see AGENTS.md note on Windows bash + non-ASCII).
func commaSep(n int) string {
	if n < 0 {
		return "-" + commaSep(-n)
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	// Walk from the right inserting commas every 3 digits.
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
		if len(s) > pre {
			b.WriteByte(',')
		}
	}
	for i := pre; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte(',')
		}
	}
	return b.String()
}

// Pin io to ensure the import survives even if every other reference is
// stripped during edits. dirSize+ProbeEventLog use it indirectly via
// bufio/os, but a direct reference here avoids future-edit regressions.
var _ = io.EOF
