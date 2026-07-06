package datasource

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

const (
	// embeddedExportTimeout bounds the whole load (across retries). Measured
	// spawn+dump is ~0.4s on a ~1300-issue project (bt-ij71a grounding); 30s
	// is generous headroom for very large embedded DBs while failing fast on
	// a genuine hang.
	embeddedExportTimeout = 30 * time.Second
	// embeddedExportMaxRetries bounds retries of a FAILED export. On Windows a
	// `bd export` that opens the embedded Dolt dir exactly while a concurrent
	// `bd create` commit swaps storage files can fail transiently ("a non
	// close operation has been requested of a file object with a delete
	// pending"). Dolt commit windows are brief, so a short backoff clears it.
	embeddedExportMaxRetries = 3
	// embeddedExportRetryBackoff is the pause between export attempts.
	embeddedExportRetryBackoff = 150 * time.Millisecond
)

// exportRunner runs `bd export` in dir and returns (stdout, stderr, err).
// Injectable so retry behavior is unit-testable without spawning bd.
type exportRunner func(ctx context.Context, bdPath, dir string) (stdout, stderr []byte, err error)

func defaultExportRunner(ctx context.Context, bdPath, dir string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, bdPath, "export")
	cmd.Dir = dir
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	return out.Bytes(), errBuf.Bytes(), err
}

// EmbeddedReader loads issues from an embedded (in-process Dolt) beads project
// by shelling `bd export` and parsing the JSONL from stdout into memory.
//
// Unlike the server-mode DoltReader it opens no SQL connection and holds no
// lock, so a concurrent `bd` command (e.g. `bd create`) is never blocked -
// the core reason bt reads embedded projects this way rather than hosting its
// own dolt sql-server (bt-qrt2u). The `bd export` stream itself is never
// written to disk: it's consumed straight from the subprocess pipe. (A
// separate, read-side parsed-snapshot CACHE under bt's own .bt/ directory may
// be written afterward purely as a latency optimization - see LoadIssues and
// embedded_cache.go, bt-xdah0. That cache never touches .beads/, so it does
// not change the never-persist boundary this comment describes.)
type EmbeddedReader struct {
	repoRoot string
	// lookPath resolves the bd binary; injectable for tests. nil => exec.LookPath.
	lookPath func(string) (string, error)
	// timeout bounds the whole load; zero => embeddedExportTimeout.
	timeout time.Duration
	// maxRetries bounds retries of a failed export; zero => embeddedExportMaxRetries.
	maxRetries int
	// retryBackoff is the pause between attempts; zero => embeddedExportRetryBackoff.
	retryBackoff time.Duration
	// runExport is injectable for tests; nil => defaultExportRunner.
	runExport exportRunner
	// sleep is injectable for tests; nil => time.Sleep.
	sleep func(time.Duration)
}

// NewEmbeddedReader builds a reader for an embedded-Dolt DataSource. The
// source's Path is the repo root (parent of .beads) where `bd export` runs.
func NewEmbeddedReader(source DataSource) (*EmbeddedReader, error) {
	if source.Type != SourceTypeEmbeddedDolt {
		return nil, fmt.Errorf("NewEmbeddedReader: unexpected source type %q", source.Type)
	}
	if source.Path == "" {
		return nil, fmt.Errorf("NewEmbeddedReader: empty repo root")
	}
	return &EmbeddedReader{repoRoot: source.Path, lookPath: exec.LookPath}, nil
}

// LoadIssues runs `bd export` in the project directory and parses the JSONL
// stdout into []model.Issue, retrying briefly on transient failures.
//
// Plain `bd export` (NOT --all) matches the server-mode SQL reader for
// fidelity (bt-ij71a). The DoltReader runs `SELECT ... FROM issues WHERE
// status != 'tombstone'`, and `bd export` default emits exactly the regular
// issues-table rows. Verified empirically: bd's infra/memory/template/gate
// records live in SEPARATE tables (wisps et al.), not `issues`, so
// `SELECT FROM issues` excludes them and `--all` would over-include memories
// the server never renders. (`--include-infra` added zero rows on a
// 1314-issue project - there are no infra beads in the issues table.)
//
// Before paying for the export, checks the manifest-hash snapshot cache
// under .bt/ (bt-xdah0): if the Dolt storage manifest's content hash matches
// the hash stored alongside the last cached snapshot, nothing has changed
// since that export and the cached issues are returned as-is, skipping the
// subprocess entirely. Caching is skipped (never errors) when the project's
// embedded config or manifest can't be read - e.g. non-embedded sources, or
// a brand new project with no manifest yet.
func (r *EmbeddedReader) LoadIssues() ([]model.Issue, error) {
	lookPath := r.lookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	bdPath, err := lookPath("bd")
	if err != nil {
		return nil, fmt.Errorf("embedded-mode read requires the `bd` CLI on PATH: %w", err)
	}

	beadsDir := filepath.Join(r.repoRoot, ".beads")
	cacheKey, cacheEnabled := computeEmbeddedCacheKey(beadsDir)
	var cachePath string
	if cacheEnabled {
		cachePath = embeddedSnapshotCachePath(r.repoRoot, cacheKey.dbName)
		if issues, hit := readEmbeddedSnapshot(cachePath, cacheKey); hit {
			return issues, nil
		}
	}

	timeout := r.timeout
	if timeout <= 0 {
		timeout = embeddedExportTimeout
	}
	maxRetries := r.maxRetries
	if maxRetries <= 0 {
		maxRetries = embeddedExportMaxRetries
	}
	backoff := r.retryBackoff
	if backoff <= 0 {
		backoff = embeddedExportRetryBackoff
	}
	runExport := r.runExport
	if runExport == nil {
		runExport = defaultExportRunner
	}
	sleep := r.sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			sleep(backoff)
		}
		stdout, stderr, runErr := runExport(ctx, bdPath, r.repoRoot)
		if runErr == nil {
			issues, perr := loader.ParseIssues(bytes.NewReader(stdout))
			if perr != nil {
				return nil, fmt.Errorf("parsing `bd export` output: %w", perr)
			}
			if cacheEnabled {
				// Best-effort: a cache write failure never fails the load
				// (the cache is a derived speedup, not a system of record).
				// The next call simply re-exports.
				_ = writeEmbeddedSnapshot(cachePath, cacheKey, issues)
			}
			return issues, nil
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("`bd export` timed out after %s in %s", timeout, r.repoRoot)
		}
		if msg := strings.TrimSpace(string(stderr)); msg != "" {
			lastErr = fmt.Errorf("`bd export` failed in %s: %w (%s)", r.repoRoot, runErr, msg)
		} else {
			lastErr = fmt.Errorf("`bd export` failed in %s: %w", r.repoRoot, runErr)
		}
	}
	return nil, fmt.Errorf("`bd export` failed after %d attempts: %w", maxRetries+1, lastErr)
}
