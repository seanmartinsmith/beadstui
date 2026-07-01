package datasource

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// These tests spawn the real `bd` CLI against a throwaway embedded project.
// They are gated behind BT_EMBEDDED_INTEGRATION=1 so the default `go test`
// suite stays fast and subprocess-free (and CI without bd is unaffected).
// Run them with:
//
//	BT_EMBEDDED_INTEGRATION=1 go test ./internal/datasource/ -run Embedded_ -v
func requireEmbeddedIntegration(t *testing.T) string {
	t.Helper()
	if os.Getenv("BT_EMBEDDED_INTEGRATION") != "1" {
		t.Skip("set BT_EMBEDDED_INTEGRATION=1 to run bd-spawning embedded integration tests")
	}
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("bd CLI not on PATH")
	}
	return bdPath
}

// runBd runs a bd subcommand in dir, non-interactively, with a timeout, and
// fails the test on error. Returns combined output.
func runBd(t *testing.T, bdPath, dir string, timeout time.Duration, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bdPath, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BD_NON_INTERACTIVE=1")
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("bd %v timed out after %s in %s\noutput: %s", args, timeout, dir, out)
	}
	if err != nil {
		t.Fatalf("bd %v failed: %v\noutput: %s", args, err, out)
	}
	return string(out)
}

// initEmbeddedProject creates a fresh embedded-mode beads project in a temp
// dir and returns its repo root. It asserts the project resolved to embedded
// mode (guards against a stray shared-server auto-attach in the test env).
func initEmbeddedProject(t *testing.T, bdPath string) string {
	t.Helper()
	root := t.TempDir()
	runBd(t, bdPath, root, 30*time.Second, "init")
	if _, ok := ReadEmbeddedConfig(filepath.Join(root, ".beads")); !ok {
		t.Fatalf("bd init did not produce an embedded project in %s", root)
	}
	return root
}

// TestEmbedded_LoadsRealProject exercises the full read path against real bd:
// DiscoverSource -> embedded -> EmbeddedReader (`bd export`) -> loader,
// asserting labels, dependencies, and comments survive the round trip.
func TestEmbedded_LoadsRealProject(t *testing.T) {
	bdPath := requireEmbeddedIntegration(t)
	root := initEmbeddedProject(t, bdPath)

	runBd(t, bdPath, root, 15*time.Second, "create", "Alpha", "-t", "task", "-p", "1", "--labels", "area:data")
	runBd(t, bdPath, root, 15*time.Second, "create", "Beta", "-t", "bug", "-p", "2", "--labels", "area:tui")

	// Grab the two IDs from a plain list.
	out := runBd(t, bdPath, root, 15*time.Second, "list")
	ids := parseIDs(out)
	if len(ids) < 2 {
		t.Fatalf("expected >=2 issues, parsed ids=%v from:\n%s", ids, out)
	}
	a, b := ids[0], ids[1]
	runBd(t, bdPath, root, 15*time.Second, "dep", "add", a, b)
	runBd(t, bdPath, root, 15*time.Second, "comments", "add", a, "a comment on alpha")

	src, err := DiscoverSource(DiscoveryOptions{BeadsDir: filepath.Join(root, ".beads")})
	if err != nil {
		t.Fatalf("DiscoverSource: %v", err)
	}
	if src.Type != SourceTypeEmbeddedDolt {
		t.Fatalf("src.Type = %q, want embedded", src.Type)
	}

	issues, err := LoadFromSource(src)
	if err != nil {
		t.Fatalf("LoadFromSource: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("loaded %d issues, want 2", len(issues))
	}

	byID := make(map[string]int) // id -> index
	for i, is := range issues {
		byID[is.ID] = i
	}
	ai, ok := byID[a]
	if !ok {
		t.Fatalf("issue %s not loaded", a)
	}
	alpha := issues[ai]
	if len(alpha.Labels) == 0 {
		t.Errorf("alpha lost its labels")
	}
	if len(alpha.Dependencies) == 0 {
		t.Errorf("alpha lost its dependency on %s", b)
	} else if alpha.Dependencies[0].DependsOnID != b {
		t.Errorf("alpha dep DependsOnID = %q, want %q", alpha.Dependencies[0].DependsOnID, b)
	}
	if len(alpha.Comments) == 0 {
		t.Errorf("alpha lost its comment")
	} else if alpha.Comments[0].Text != "a comment on alpha" {
		t.Errorf("alpha comment text = %q", alpha.Comments[0].Text)
	}
	// Actor: bd export emits created_by; must surface as Author.
	if alpha.Author == "" {
		t.Errorf("alpha Author empty; created_by not mapped through the embedded read path")
	}
}

// TestEmbedded_ConcurrentBdCreateSucceeds is the HARD-CONSTRAINT acceptance
// test (bt-ij71a / bt-qrt2u): while bt actively reads an embedded project,
// a concurrent `bd create` in another process MUST succeed and MUST NOT hang.
// Option B holds no lock and opens no SQL connection, so there is nothing for
// bd to contend with - this proves it empirically. Under the rejected option A
// (bt hosting a dolt sql-server on the embedded dir) bd would back off
// indefinitely and this test would time out.
func TestEmbedded_ConcurrentBdCreateSucceeds(t *testing.T) {
	bdPath := requireEmbeddedIntegration(t)
	root := initEmbeddedProject(t, bdPath)
	runBd(t, bdPath, root, 15*time.Second, "create", "seed", "-t", "task", "-p", "3")

	src, err := DiscoverSource(DiscoveryOptions{BeadsDir: filepath.Join(root, ".beads")})
	if err != nil || src.Type != SourceTypeEmbeddedDolt {
		t.Fatalf("DiscoverSource: type=%q err=%v", src.Type, err)
	}

	// Readers: 4 goroutines each running several full loads, mimicking bt
	// holding the project open with live refreshes.
	readErrs := make(chan error, 8)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if _, e := LoadFromSource(src); e != nil {
					readErrs <- e
					return
				}
			}
		}()
	}

	// While readers hammer `bd export`, run concurrent `bd create`s. Each is
	// bounded; a hang (option-A symptom) trips the deadline and fails.
	const createTimeout = 20 * time.Second
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), createTimeout)
		cmd := exec.CommandContext(ctx, bdPath, "create", fmt.Sprintf("concurrent-%d", i), "-t", "task", "-p", "3")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "BD_NON_INTERACTIVE=1")
		out, e := cmd.CombinedOutput()
		timedOut := ctx.Err() == context.DeadlineExceeded
		cancel()
		if timedOut {
			close(stop)
			wg.Wait()
			t.Fatalf("HARD CONSTRAINT VIOLATED: `bd create %d` hung (>%s) while bt was reading the embedded project.\noutput: %s", i, createTimeout, out)
		}
		if e != nil {
			close(stop)
			wg.Wait()
			t.Fatalf("bd create %d failed: %v\noutput: %s", i, e, out)
		}
	}

	close(stop)
	wg.Wait()
	select {
	case e := <-readErrs:
		t.Fatalf("reader errored during concurrent writes: %v", e)
	default:
	}

	// Sanity: a final load reflects the concurrently-created issues.
	issues, err := LoadFromSource(src)
	if err != nil {
		t.Fatalf("final load: %v", err)
	}
	if len(issues) < 4 { // seed + 3 concurrent
		t.Errorf("final load has %d issues, want >=4 (seed + 3 concurrent creates)", len(issues))
	}
}

// parseIDs extracts issue IDs from `bd list` plain output lines like
// "○ embtest-blc ● P1 Title". IDs are the second whitespace token.
func parseIDs(listOutput string) []string {
	var ids []string
	for _, line := range splitLines(listOutput) {
		fields := fieldsOf(line)
		if len(fields) >= 2 && looksLikeID(fields[1]) {
			ids = append(ids, fields[1])
		}
	}
	return ids
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func fieldsOf(line string) []string {
	var out []string
	i := 0
	for i < len(line) {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t' || line[i] == '\r') {
			i++
		}
		start := i
		for i < len(line) && line[i] != ' ' && line[i] != '\t' && line[i] != '\r' {
			i++
		}
		if start < i {
			out = append(out, line[start:i])
		}
	}
	return out
}

// looksLikeID reports whether tok matches <prefix>-<suffix> (an issue ID),
// excluding pure status glyphs.
func looksLikeID(tok string) bool {
	dash := -1
	for i := 0; i < len(tok); i++ {
		if tok[i] == '-' {
			dash = i
			break
		}
	}
	return dash > 0 && dash < len(tok)-1
}
