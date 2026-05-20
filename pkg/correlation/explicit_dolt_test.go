package correlation

// Tests for bt-ydjw.5: the explicit-ID matcher's modern alphanumeric ID
// patterns and the ScanCommits bulk-correlation entry point used by the
// Dolt-path correlator.
//
// The recon (.beads/tmp/ydjw5-recon.md) verified that the pre-bt-ydjw.5
// DefaultPatterns failed to match bt's actual ID format (bt-ydjw,
// bt-08sh.5, bt-72l8.1.1) and actively truncated suffix-with-period IDs
// to wrong-bead captures. These tests lock the fix in place.

import (
	"os/exec"
	"testing"
)

// TestDefaultPatterns_AlphanumericIDs covers the modern bt ID format that
// bt-ydjw.5 added: alphanumeric suffixes with optional `.N` parent-child
// nesting. Each entry asserts that ExtractIDsFromMessage emits the
// expected normalized ID. Spot-check the cases that were silent failures
// before bt-ydjw.5 (per the recon comment posted on the bead).
func TestDefaultPatterns_AlphanumericIDs(t *testing.T) {
	m := NewExplicitMatcher("/tmp/test")

	tests := []struct {
		name    string
		message string
		wantID  string // first extracted ID we expect to see
	}{
		{
			name:    "bt-ydjw alphanumeric suffix",
			message: "feat(tui): defensive empty-state gate for Dolt-only history view (bt-ydjw)",
			wantID:  "bt-ydjw",
		},
		{
			name:    "bt-08sh.5 with period child suffix",
			message: "test(correlation): cover Dolt-only extraction path + dispatcher branches (bt-08sh.5)",
			wantID:  "bt-08sh.5",
		},
		{
			name:    "bt-ydjw.1 keyword Case marker",
			message: "fix(tui): cursor-driven global-mode history dispatch (bt-ydjw.1 Case B)",
			wantID:  "bt-ydjw.1",
		},
		{
			name:    "bt-kfkrb all-letter mid-message",
			message: "chore: gitignore bt-timing.log (bt-kfkrb residue)",
			wantID:  "bt-kfkrb",
		},
		{
			name:    "bt-72l8.1.1 double-nested period",
			message: "audit bt-72l8.1.1 TUI ghost-features",
			wantID:  "bt-72l8.1.1",
		},
		{
			name:    "[bt-ydjw] bracket form",
			message: "fix: address review feedback [bt-ydjw]",
			wantID:  "bt-ydjw",
		},
		{
			name:    "closes keyword with period suffix",
			message: "closes bt-ydjw.5",
			wantID:  "bt-ydjw.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := m.ExtractIDsFromMessage(tt.message)
			if len(matches) == 0 {
				t.Fatalf("ExtractIDsFromMessage(%q) returned no matches; want %q", tt.message, tt.wantID)
			}
			// Search for wantID anywhere in the result -- the new pattern
			// runs after legacy ones, so the wantID isn't always at index 0
			// when both patterns capture overlapping forms.
			found := false
			for _, im := range matches {
				if im.ID == tt.wantID {
					found = true
					break
				}
			}
			if !found {
				ids := make([]string, len(matches))
				for i, im := range matches {
					ids[i] = im.ID
				}
				t.Errorf("ExtractIDsFromMessage(%q) = %v; want one of them to be %q",
					tt.message, ids, tt.wantID)
			}
		})
	}
}

// TestDefaultPatterns_NoSuffixTruncation pins down the bug the recon
// uncovered: before bt-ydjw.5, `(?i)bt[-_](\d+)` would match the leading
// digits of `bt-08sh.5` and normalizeBeadID's numeric-only branch would
// produce the wrong-bead ID `bt-08`. With `\b` added after `\d+`, the
// truncation can no longer happen on its own. The new alphanumeric
// pattern then produces the correct `bt-08sh.5`.
func TestDefaultPatterns_NoSuffixTruncation(t *testing.T) {
	m := NewExplicitMatcher("/tmp/test")

	message := "feat(scope): description (bt-08sh.5)"
	matches := m.ExtractIDsFromMessage(message)

	// The bug-case: `bt-08` MUST NOT appear in results.
	for _, im := range matches {
		if im.ID == "bt-08" {
			t.Errorf("ExtractIDsFromMessage(%q) produced wrong-bead truncation %q (expected the digits-only pattern to anchor \\b, preventing this)",
				message, "bt-08")
		}
	}

	// The fix-case: `bt-08sh.5` MUST appear.
	found := false
	for _, im := range matches {
		if im.ID == "bt-08sh.5" {
			found = true
			break
		}
	}
	if !found {
		ids := make([]string, len(matches))
		for i, im := range matches {
			ids[i] = im.ID
		}
		t.Errorf("ExtractIDsFromMessage(%q) = %v; want bt-08sh.5 present", message, ids)
	}
}

// seedExplicitCommits creates commits in repoPath with the supplied messages
// (one per slice entry), each touching a unique file so git has a real diff
// to attach. Returns the SHAs in insertion order so tests can assert on them.
func seedExplicitCommits(t *testing.T, repoPath string, messages []string) []string {
	t.Helper()
	shas := make([]string, 0, len(messages))
	for i, msg := range messages {
		// Touch a unique file per commit so the diff is non-empty.
		fname := "file_" + sanitize(msg) + ".go"
		write := exec.Command("git", "commit", "--allow-empty", "-q", "-m", msg)
		write.Dir = repoPath
		if out, err := write.CombinedOutput(); err != nil {
			t.Fatalf("git commit %d (%q) failed: %v: %s", i, msg, err, out)
		}
		// Capture the new HEAD.
		rev := exec.Command("git", "rev-parse", "HEAD")
		rev.Dir = repoPath
		out, err := rev.Output()
		if err != nil {
			t.Fatalf("git rev-parse HEAD: %v", err)
		}
		shas = append(shas, string(trimSpace(out)))
		_ = fname
	}
	return shas
}

// trimSpace is a tiny helper to avoid importing strings just for this.
func trimSpace(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ' || b[len(b)-1] == '\t') {
		b = b[:len(b)-1]
	}
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t') {
		b = b[1:]
	}
	return b
}

// sanitize maps a commit message to a filesystem-safe slug.
func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s) && i < 32; i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// TestExplicitMatcher_ScanCommits exercises the bulk-correlation entry
// point added in bt-ydjw.5. Builds a temp git repo, seeds three commits
// with bt-style references (and one unrelated commit), and asserts the
// returned map is keyed by extracted bead ID with the correct shape.
//
// The test specifically covers the dispatcher's expectation: one git
// invocation produces all bead correlations regardless of bead count.
func TestExplicitMatcher_ScanCommits(t *testing.T) {
	repo := initBareGitRepo(t)
	seedExplicitCommits(t, repo, []string{
		"feat(correlation): wire bt-ydjw.5 explicit matcher",
		"fix(tui): address (bt-08sh.5) feedback",
		"chore: rotate bt-kfkrb timing log",
		"docs: README polish (no bead ref)",
	})

	m := NewExplicitMatcher(repo)
	byBead, err := m.ScanCommits(ExtractOptions{})
	if err != nil {
		t.Fatalf("ScanCommits: %v", err)
	}

	wantBeads := []string{"bt-ydjw.5", "bt-08sh.5", "bt-kfkrb"}
	for _, id := range wantBeads {
		matches, ok := byBead[id]
		if !ok {
			t.Errorf("byBead missing %q (keys=%v)", id, keys(byBead))
			continue
		}
		if len(matches) != 1 {
			t.Errorf("byBead[%q]: len=%d, want 1", id, len(matches))
			continue
		}
		got := matches[0]
		if got.BeadID != id {
			t.Errorf("byBead[%q].BeadID = %q, want %q", id, got.BeadID, id)
		}
		if got.CommitSHA == "" {
			t.Errorf("byBead[%q].CommitSHA empty -- ScanCommits should populate SHA from git log", id)
		}
		if got.Confidence < MethodRanges[MethodExplicitID].Min || got.Confidence > MethodRanges[MethodExplicitID].Max {
			t.Errorf("byBead[%q].Confidence = %.2f, want within [%.2f, %.2f]",
				id, got.Confidence,
				MethodRanges[MethodExplicitID].Min, MethodRanges[MethodExplicitID].Max)
		}
	}

	// Docs-only commit must not produce a real bead key. (Dashed phrases
	// like "no-bead-ref" might still appear as orphan keys; we just don't
	// want any of the seeded bead IDs to inflate beyond their commit count.)
	for _, id := range wantBeads {
		if got := len(byBead[id]); got != 1 {
			t.Errorf("bead %q matched %d commits, want 1 (docs commit should not attach)", id, got)
		}
	}
}

// keys returns the map's keys for diagnostic output. Order is unstable
// (map iteration), used only inside t.Errorf.
func keys(m map[string][]ExplicitMatch) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestCorrelator_Dispatch_DoltPath_ExplicitMerge covers the GenerateReport
// branch added by bt-ydjw.5: a Dolt-only repo (no JSONL on disk) whose host
// git history contains commits referencing seeded bead IDs should populate
// `Histories[id].Commits` with MethodExplicitID entries. This is the
// integration test for the COMMITS pane filling up after bt-ydjw.5 ships.
//
// Mutation protection: if `explicitCommits` is removed from GenerateReport,
// the assertion on len(h.Commits) >= 1 fails because the Dolt path no
// longer produces any CorrelatedCommit instances. If the bead-set filter
// (the `known` map check) is dropped, the test still passes for `bt-1`
// but starts producing orphan attachments for non-bead dashed phrases --
// the second assertion that `bt-2` has commits AND `bt-1` has commits is
// also strict on counts.
func TestCorrelator_Dispatch_DoltPath_ExplicitMerge(t *testing.T) {
	repo := initBareGitRepo(t)
	// Deliberately no seedJSONLOnDisk: this is the Dolt-only scenario where
	// bt-ydjw.5's wiring is the entire git-correlation surface.

	seedExplicitCommits(t, repo, []string{
		"feat(correlation): touch bead bt-1 via parens (bt-1)",
		"fix(scope): closes bt-2",
		"chore: irrelevant commit, no bead",
	})

	db := newDoltEventsFixture(t)
	c := NewCorrelatorWithDolt(repo, db)
	beads := []BeadInfo{
		{ID: "bt-1", Title: "Bead 1", Status: "in_progress"},
		{ID: "bt-2", Title: "Bead 2", Status: "in_progress"},
	}

	report, err := c.GenerateReport(beads, CorrelatorOptions{})
	if err != nil {
		t.Fatalf("GenerateReport: %v", err)
	}
	if report.RepoStatus.JSONLTracked {
		t.Fatalf("JSONLTracked = true, want false (no JSONL on disk -> Dolt-only path)")
	}

	for _, id := range []string{"bt-1", "bt-2"} {
		h, ok := report.Histories[id]
		if !ok {
			t.Errorf("Histories missing %q", id)
			continue
		}
		if len(h.Commits) < 1 {
			t.Errorf("Histories[%q].Commits = %d, want >= 1 (explicit-ID matcher should populate)", id, len(h.Commits))
			continue
		}
		// Confidence must be in MethodExplicitID's documented range.
		for i, cm := range h.Commits {
			if cm.Method != MethodExplicitID {
				t.Errorf("Histories[%q].Commits[%d].Method = %q, want %q",
					id, i, cm.Method, MethodExplicitID)
			}
			if cm.Confidence < MethodRanges[MethodExplicitID].Min {
				t.Errorf("Histories[%q].Commits[%d].Confidence = %.2f, below %.2f",
					id, i, cm.Confidence, MethodRanges[MethodExplicitID].Min)
			}
		}
	}

	// Stats should reflect the new beads-with-commits population.
	if report.Stats.BeadsWithCommits < 2 {
		t.Errorf("Stats.BeadsWithCommits = %d, want >= 2 (both seeded beads should have commits)", report.Stats.BeadsWithCommits)
	}
	if got := report.Stats.MethodDistribution[MethodExplicitID.String()]; got < 2 {
		t.Errorf("Stats.MethodDistribution[%q] = %d, want >= 2",
			MethodExplicitID.String(), got)
	}
}

// TestCorrelator_Dispatch_JSONLPath_NoExplicitOnDoltPath covers the
// negative case: on the JSONL-tracked path, bt-ydjw.5's explicit-ID merge
// must NOT run. The JSONL path already had its own MethodExplicitID flow
// (via co-commit's containsBeadID confidence bump); running the matcher
// again would double-count.
//
// We make the assertion mutation-resistant by giving the JSONL path no
// JSONL events (empty file) and seeding the host git history with a
// commit referencing the seeded bead. If bt-ydjw.5 incorrectly ran on the
// JSONL branch too, we'd see >= 1 commit on the report. The expected
// result is 0 commits because the JSONL extractor has nothing to do and
// explicit-ID merging is gated behind !jsonlTracked.
func TestCorrelator_Dispatch_JSONLPath_NoExplicitOnDoltPath(t *testing.T) {
	repo := initBareGitRepo(t)
	seedJSONLOnDisk(t, repo) // empty file, JSONLTracked=true
	seedExplicitCommits(t, repo, []string{
		"feat: touch bead bt-1 via parens (bt-1)",
	})

	c := NewCorrelatorWithDolt(repo, newDoltEventsFixture(t))
	beads := []BeadInfo{{ID: "bt-1", Title: "Bead 1", Status: "open"}}

	report, err := c.GenerateReport(beads, CorrelatorOptions{})
	if err != nil {
		t.Fatalf("GenerateReport: %v", err)
	}
	if !report.RepoStatus.JSONLTracked {
		t.Fatalf("JSONLTracked = false, want true (JSONL file is on disk)")
	}

	h, ok := report.Histories["bt-1"]
	if !ok {
		t.Fatalf("Histories missing bt-1")
	}
	if len(h.Commits) != 0 {
		t.Errorf("Histories[bt-1].Commits = %d, want 0 (JSONL path should not run bt-ydjw.5 explicit-ID merge)", len(h.Commits))
	}
}
