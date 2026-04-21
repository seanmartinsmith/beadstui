package view

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// labeledCorpus is the on-disk shape of pkg/view/testdata/corpus/labeled_corpus.json.
// It is the test-time ground truth for the v2 FPR gate: issues feed the v2
// readers; expected_* entries carry per-record truth labels a human assigned
// after reading both sides of every candidate pair/ref.
type labeledCorpus struct {
	Description   string          `json:"description"`
	Issues        []model.Issue   `json:"issues"`
	ExpectedPairs []expectedPair  `json:"expected_pairs"`
	ExpectedRefs  []expectedRef   `json:"expected_refs"`
}

type expectedPair struct {
	Suffix      string   `json:"suffix"`
	Members     []string `json:"members"`
	Intentional *bool    `json:"intentional"` // pointer so we can detect "missing truth" vs "false"
	Reason      string   `json:"reason"`
}

type expectedRef struct {
	Source      string `json:"source"`
	Target      string `json:"target"`
	Intentional *bool  `json:"intentional"`
	Reason      string `json:"reason"`
}

const (
	labeledCorpusPath = "testdata/corpus/labeled_corpus.json"

	// Hard gates (plan Success Metrics):
	fprGatePairMax      = 0.10 // <10% total pair FPR on v2
	fprGateRefsVerbMax  = 0.05 // ≤5% broken-flag FPR on refs under verb mode
	fprGateMinPairCands = 10   // N≥10 candidate pair records required

	// Memory gate (plan line 341): corpus load delta must stay <10 MiB.
	fprGateMemDeltaMax = 10 * 1024 * 1024
)

// TestFPRGate_LoadCorpus asserts the labeled corpus parses cleanly, has the
// minimum candidate-pair count the FPR thresholds require, and every expected
// record carries an explicit `intentional` boolean. Any parse error, missing
// truth field, or N<10 candidate pairs fails with a message including the file
// path so the failure points at the corpus, not the reader.
func TestFPRGate_LoadCorpus(t *testing.T) {
	corpus, memDelta := loadLabeledCorpus(t)

	if memDelta > fprGateMemDeltaMax {
		t.Errorf("corpus load exceeded memory budget: delta=%d bytes (cap=%d bytes). "+
			"Split the corpus or trim heavyweight fields.",
			memDelta, fprGateMemDeltaMax)
	}

	for i, p := range corpus.ExpectedPairs {
		if p.Intentional == nil {
			t.Errorf("%s: expected_pairs[%d] (suffix=%q) missing \"intentional\" field; "+
				"label required for FPR scoring", labeledCorpusPath, i, p.Suffix)
		}
	}
	for i, r := range corpus.ExpectedRefs {
		if r.Intentional == nil {
			t.Errorf("%s: expected_refs[%d] (%s -> %s) missing \"intentional\" field",
				labeledCorpusPath, i, r.Source, r.Target)
		}
	}

	if got := len(corpus.ExpectedPairs); got < fprGateMinPairCands {
		t.Errorf("%s has only %d candidate pairs; FPR gate requires N>=%d "+
			"(single mislabel otherwise exceeds threshold)",
			labeledCorpusPath, got, fprGateMinPairCands)
	}
}

// TestFPRGate_Pairs runs ComputePairRecordsV2 against the labeled corpus and
// asserts pair FPR stays under fprGatePairMax. FPR here is "emitted records
// where truth says not intentional / total emitted records" per the plan.
// v2 is expected to emit only the 5 intentional cross-project pairs; the 6
// suffix-collision candidates have no cross-prefix dep edge and should be
// invisible to v2.
func TestFPRGate_Pairs(t *testing.T) {
	corpus, _ := loadLabeledCorpus(t)

	truth := make(map[string]bool, len(corpus.ExpectedPairs)) // suffix -> intentional
	for _, p := range corpus.ExpectedPairs {
		if p.Intentional != nil {
			truth[p.Suffix] = *p.Intentional
		}
	}

	records := ComputePairRecordsV2(corpus.Issues)
	if len(records) == 0 {
		t.Fatalf("ComputePairRecordsV2 emitted no records against labeled corpus; "+
			"expected at least 5 intentional pairs from %s", labeledCorpusPath)
	}

	var falsePositives []string
	var unlabeled []string
	for _, r := range records {
		intentional, ok := truth[r.Suffix]
		if !ok {
			unlabeled = append(unlabeled, r.Suffix)
			continue
		}
		if !intentional {
			falsePositives = append(falsePositives, r.Suffix)
		}
	}

	fpr := float64(len(falsePositives)) / float64(len(records))
	t.Logf("pair.v2 FPR: %.2f%% (%d/%d emitted); intentional emissions: %d",
		fpr*100, len(falsePositives), len(records), len(records)-len(falsePositives))
	if len(unlabeled) > 0 {
		t.Logf("pair.v2 emitted %d unlabeled suffixes (excluded from FPR): %v",
			len(unlabeled), unlabeled)
	}
	if fpr >= fprGatePairMax {
		t.Errorf("pair.v2 FPR %.2f%% exceeds cap %.2f%% (false positives: %v)",
			fpr*100, fprGatePairMax*100, falsePositives)
	}
}

// TestFPRGate_Refs runs ComputeRefRecordsV2 under each --sigils mode against
// the labeled corpus. verb mode is gated hard at fprGateRefsVerbMax on the
// broken-flag subset; strict and permissive report informational FPR readouts.
//
// "Broken-flag FPR" means: of the records emitted with the "broken" flag,
// what fraction does truth say are not intentional? A broken ref is
// intentional when the author knew the target was missing (surfacing the
// decay) — a false positive is when the detector flagged a legitimate ref
// as broken because it failed to resolve.
func TestFPRGate_Refs(t *testing.T) {
	corpus, _ := loadLabeledCorpus(t)

	truth := make(map[string]bool, len(corpus.ExpectedRefs)) // "src|tgt" -> intentional
	for _, r := range corpus.ExpectedRefs {
		if r.Intentional != nil {
			truth[r.Source+"|"+r.Target] = *r.Intentional
		}
	}

	modes := []struct {
		name string
		mode analysis.SigilMode
		gate float64 // -1 = informational only
	}{
		{"strict", analysis.SigilModeStrict, -1},
		{"verb", analysis.SigilModeVerb, fprGateRefsVerbMax},
		{"permissive", analysis.SigilModePermissive, -1},
	}

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			records := ComputeRefRecordsV2(corpus.Issues, m.mode)

			totalEmitted := len(records)
			var brokenEmitted int
			var brokenFalsePositives []string
			var brokenUnlabeled []string

			for _, r := range records {
				if !containsFlag(r.Flags, "broken") {
					continue
				}
				brokenEmitted++
				key := r.Source + "|" + r.Target
				intentional, ok := truth[key]
				if !ok {
					brokenUnlabeled = append(brokenUnlabeled, key)
					continue
				}
				if !intentional {
					brokenFalsePositives = append(brokenFalsePositives, key)
				}
			}

			var fpr float64
			if brokenEmitted > 0 {
				fpr = float64(len(brokenFalsePositives)) / float64(brokenEmitted)
			}
			t.Logf("ref.v2 mode=%s: total=%d, broken=%d, broken_fpr=%.2f%% (fp=%d)",
				m.name, totalEmitted, brokenEmitted, fpr*100, len(brokenFalsePositives))
			if len(brokenUnlabeled) > 0 {
				sort.Strings(brokenUnlabeled)
				t.Logf("ref.v2 mode=%s: %d unlabeled broken records (excluded from FPR): %v",
					m.name, len(brokenUnlabeled), brokenUnlabeled)
			}

			if m.gate >= 0 && fpr > m.gate {
				t.Errorf("ref.v2 mode=%s broken FPR %.2f%% exceeds gate %.2f%% "+
					"(false positives: %v)",
					m.name, fpr*100, m.gate*100, brokenFalsePositives)
			}
		})
	}
}

// loadLabeledCorpus reads the committed corpus fixture, measures the
// Unmarshal memory delta, and returns both. Any read/parse error fails
// the test with the full file path so the cause is obvious.
func loadLabeledCorpus(t *testing.T) (labeledCorpus, uint64) {
	t.Helper()

	abs, err := filepath.Abs(labeledCorpusPath)
	if err != nil {
		t.Fatalf("resolve %s: %v", labeledCorpusPath, err)
	}

	raw, err := os.ReadFile(labeledCorpusPath)
	if err != nil {
		t.Fatalf("read %s: %v", abs, err)
	}

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	var corpus labeledCorpus
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("parse %s: %v", abs, err)
	}

	runtime.ReadMemStats(&after)
	delta := after.HeapAlloc - before.HeapAlloc
	// HeapAlloc can decrease if GC ran mid-unmarshal; treat underflow as 0.
	if after.HeapAlloc < before.HeapAlloc {
		delta = 0
	}

	if len(corpus.Issues) == 0 {
		t.Fatalf("%s contains zero issues; corpus is empty or schema drift", abs)
	}

	return corpus, delta
}

// containsFlag returns true if flags contains f. Named to avoid collision
// with the existing hasFlag helper in pair_record_test.go.
func containsFlag(flags []string, f string) bool {
	for _, x := range flags {
		if x == f {
			return true
		}
	}
	return false
}

// Ensure fmt/strings imports aren't removed by future edits that trim logs;
// both are used by the test's diagnostic messages in failure paths above.
var _ = fmt.Sprintf
var _ = strings.Contains
