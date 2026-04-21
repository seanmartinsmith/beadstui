package view

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// projectionFixture is the JSON-on-disk shape used by the CompactIssue golden
// harness. Each fixture lists the source issues fed into CompactAll; the
// golden file records the expected projection output.
type projectionFixture struct {
	Description string        `json:"description"`
	Issues      []model.Issue `json:"issues"`
}

// portfolioFixture is the fixture shape for PortfolioRecord golden tests. It
// carries richer inputs than projectionFixture because ComputePortfolioRecord
// depends on project scope, cross-project allIssues, a PageRank map, and a
// deterministic `now`.
type portfolioFixture struct {
	Description   string             `json:"description"`
	Project       string             `json:"project"`
	Now           time.Time          `json:"now"`
	ProjectIssues []model.Issue      `json:"project_issues"`
	AllIssues     []model.Issue      `json:"all_issues,omitempty"`
	PageRank      map[string]float64 `json:"pagerank,omitempty"`
}

const portfolioFixturePrefix = "portfolio_"

// pairFixturePrefix identifies pair projection fixtures. Pair fixtures reuse
// the projectionFixture shape (issue slice in, projection out) because
// ComputePairRecords takes the same single input as CompactAll.
const pairFixturePrefix = "pair_"

// pairV2FixturePrefix identifies the v2 subset of pair fixtures. v2 fixtures
// share the projectionFixture shape but feed a different reader
// (ComputePairRecordsV2), so they get their own golden harness and are
// excluded from the v1 harness matcher.
const pairV2FixturePrefix = "pair_v2_"

// refFixturePrefix identifies ref projection fixtures. Ref fixtures share
// the projectionFixture shape because ComputeRefRecords also takes a single
// []model.Issue input.
const refFixturePrefix = "ref_"

// TestCompactIssueGolden runs every non-portfolio fixture through CompactAll
// and asserts the JSON-serialized output matches the committed golden file at
// testdata/golden/<name>.json.
//
// To regenerate golden files when the projection intentionally changes:
//
//	GENERATE_GOLDEN=1 go test ./pkg/view/ -run TestCompactIssueGolden
//
// Always bump CompactIssueSchemaV1 when the intended change is NOT additive.
func TestCompactIssueGolden(t *testing.T) {
	const fixturesDir = "testdata/fixtures"
	const goldenDir = "testdata/golden"

	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	regenerate := os.Getenv("GENERATE_GOLDEN") == "1"

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if strings.HasPrefix(entry.Name(), portfolioFixturePrefix) {
			continue
		}
		if strings.HasPrefix(entry.Name(), pairFixturePrefix) {
			continue
		}
		if strings.HasPrefix(entry.Name(), refFixturePrefix) {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")

		t.Run(name, func(t *testing.T) {
			fixturePath := filepath.Join(fixturesDir, entry.Name())
			goldenPath := filepath.Join(goldenDir, entry.Name())

			raw, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fx projectionFixture
			if err := json.Unmarshal(raw, &fx); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}

			got := CompactAll(fx.Issues)
			gotJSON, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatalf("marshal projection: %v", err)
			}
			// Trailing newline to keep golden files POSIX-clean.
			gotJSON = append(gotJSON, '\n')

			if regenerate {
				if err := os.MkdirAll(goldenDir, 0o755); err != nil {
					t.Fatalf("mkdir golden dir: %v", err)
				}
				if err := os.WriteFile(goldenPath, gotJSON, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("Regenerated %s", goldenPath)
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (regenerate with GENERATE_GOLDEN=1): %v", err)
			}
			if !bytes.Equal(gotJSON, want) {
				t.Errorf("compact projection drift for %s\n--- got ---\n%s\n--- want ---\n%s",
					name, string(gotJSON), string(want))
			}
		})
	}
}

// TestPortfolioRecordGolden runs every portfolio_*.json fixture through
// ComputePortfolioRecord. Separate from the CompactIssue harness because the
// portfolio fixture carries richer inputs than the compact one.
//
// Regenerate with:
//
//	GENERATE_GOLDEN=1 go test ./pkg/view/ -run TestPortfolioRecordGolden
func TestPortfolioRecordGolden(t *testing.T) {
	const fixturesDir = "testdata/fixtures"
	const goldenDir = "testdata/golden"

	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	regenerate := os.Getenv("GENERATE_GOLDEN") == "1"

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if !strings.HasPrefix(entry.Name(), portfolioFixturePrefix) {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")

		t.Run(name, func(t *testing.T) {
			fixturePath := filepath.Join(fixturesDir, entry.Name())
			goldenPath := filepath.Join(goldenDir, entry.Name())

			raw, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fx portfolioFixture
			if err := json.Unmarshal(raw, &fx); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if fx.AllIssues == nil {
				fx.AllIssues = fx.ProjectIssues
			}

			got := ComputePortfolioRecord(fx.Project, fx.ProjectIssues, fx.AllIssues, fx.PageRank, fx.Now)
			gotJSON, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatalf("marshal projection: %v", err)
			}
			gotJSON = append(gotJSON, '\n')

			if regenerate {
				if err := os.MkdirAll(goldenDir, 0o755); err != nil {
					t.Fatalf("mkdir golden dir: %v", err)
				}
				if err := os.WriteFile(goldenPath, gotJSON, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("Regenerated %s", goldenPath)
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (regenerate with GENERATE_GOLDEN=1): %v", err)
			}
			if !bytes.Equal(gotJSON, want) {
				t.Errorf("portfolio projection drift for %s\n--- got ---\n%s\n--- want ---\n%s",
					name, string(gotJSON), string(want))
			}
		})
	}
}

// TestPortfolioRecordSchemaFileExists — a committed JSON schema is part of
// the projection contract.
func TestPortfolioRecordSchemaFileExists(t *testing.T) {
	path := filepath.Join("schemas", "portfolio_record.v1.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("schema file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("schema file is empty: %s", path)
	}
}

// TestCompactIssueSchemaFileExists — a committed JSON schema is part of the
// projection contract.
func TestCompactIssueSchemaFileExists(t *testing.T) {
	path := filepath.Join("schemas", "compact_issue.v1.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("schema file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("schema file is empty: %s", path)
	}
}

// TestPairRecordGolden runs every pair_*.json fixture through
// ComputePairRecords and asserts the JSON-serialized output matches the
// committed golden file. Separate harness (not folded into the compact one)
// because pair records have a different projection shape even though they
// share the same fixture layout.
//
// Regenerate with:
//
//	GENERATE_GOLDEN=1 go test ./pkg/view/ -run TestPairRecordGolden
func TestPairRecordGolden(t *testing.T) {
	const fixturesDir = "testdata/fixtures"
	const goldenDir = "testdata/golden"

	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	regenerate := os.Getenv("GENERATE_GOLDEN") == "1"

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if !strings.HasPrefix(entry.Name(), pairFixturePrefix) {
			continue
		}
		// v2 fixtures share the pair_ prefix but go through the v2 harness.
		if strings.HasPrefix(entry.Name(), pairV2FixturePrefix) {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")

		t.Run(name, func(t *testing.T) {
			fixturePath := filepath.Join(fixturesDir, entry.Name())
			goldenPath := filepath.Join(goldenDir, entry.Name())

			raw, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fx projectionFixture
			if err := json.Unmarshal(raw, &fx); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}

			got := ComputePairRecords(fx.Issues)
			gotJSON, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatalf("marshal projection: %v", err)
			}
			gotJSON = append(gotJSON, '\n')

			if regenerate {
				if err := os.MkdirAll(goldenDir, 0o755); err != nil {
					t.Fatalf("mkdir golden dir: %v", err)
				}
				if err := os.WriteFile(goldenPath, gotJSON, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("Regenerated %s", goldenPath)
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (regenerate with GENERATE_GOLDEN=1): %v", err)
			}
			if !bytes.Equal(gotJSON, want) {
				t.Errorf("pair projection drift for %s\n--- got ---\n%s\n--- want ---\n%s",
					name, string(gotJSON), string(want))
			}
		})
	}
}

// TestPairRecordV2Golden runs every pair_v2_*.json fixture through
// ComputePairRecordsV2. Shares the fixture shape with the v1 harness but
// feeds a different reader — v2 requires dep edges between members, so
// fixtures populate `dependencies` on at least one member of every pair.
//
// Regenerate with:
//
//	GENERATE_GOLDEN=1 go test ./pkg/view/ -run TestPairRecordV2Golden
func TestPairRecordV2Golden(t *testing.T) {
	const fixturesDir = "testdata/fixtures"
	const goldenDir = "testdata/golden"

	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	regenerate := os.Getenv("GENERATE_GOLDEN") == "1"

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if !strings.HasPrefix(entry.Name(), pairV2FixturePrefix) {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")

		t.Run(name, func(t *testing.T) {
			fixturePath := filepath.Join(fixturesDir, entry.Name())
			goldenPath := filepath.Join(goldenDir, entry.Name())

			raw, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fx projectionFixture
			if err := json.Unmarshal(raw, &fx); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}

			got := ComputePairRecordsV2(fx.Issues)
			gotJSON, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatalf("marshal projection: %v", err)
			}
			gotJSON = append(gotJSON, '\n')

			if regenerate {
				if err := os.MkdirAll(goldenDir, 0o755); err != nil {
					t.Fatalf("mkdir golden dir: %v", err)
				}
				if err := os.WriteFile(goldenPath, gotJSON, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("Regenerated %s", goldenPath)
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (regenerate with GENERATE_GOLDEN=1): %v", err)
			}
			if !bytes.Equal(gotJSON, want) {
				t.Errorf("pair v2 projection drift for %s\n--- got ---\n%s\n--- want ---\n%s",
					name, string(gotJSON), string(want))
			}
		})
	}
}

// TestPairRecordSchemaFileExists — a committed JSON schema is part of the
// projection contract.
func TestPairRecordSchemaFileExists(t *testing.T) {
	path := filepath.Join("schemas", "pair_record.v1.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("schema file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("schema file is empty: %s", path)
	}
}

// TestPairRecordV2SchemaFileExists — v2 has its own wire contract; the
// committed schema file is part of the projection contract. Phase 1 of
// bt-gkyn already landed this file.
func TestPairRecordV2SchemaFileExists(t *testing.T) {
	path := filepath.Join("schemas", "pair_record.v2.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("schema file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("schema file is empty: %s", path)
	}
}

// TestRefRecordGolden runs every ref_*.json fixture through
// ComputeRefRecords and asserts the JSON-serialized output matches the
// committed golden file.
//
// Regenerate with:
//
//	GENERATE_GOLDEN=1 go test ./pkg/view/ -run TestRefRecordGolden
func TestRefRecordGolden(t *testing.T) {
	const fixturesDir = "testdata/fixtures"
	const goldenDir = "testdata/golden"

	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	regenerate := os.Getenv("GENERATE_GOLDEN") == "1"

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if !strings.HasPrefix(entry.Name(), refFixturePrefix) {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")

		t.Run(name, func(t *testing.T) {
			fixturePath := filepath.Join(fixturesDir, entry.Name())
			goldenPath := filepath.Join(goldenDir, entry.Name())

			raw, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fx projectionFixture
			if err := json.Unmarshal(raw, &fx); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}

			got := ComputeRefRecords(fx.Issues)
			gotJSON, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatalf("marshal projection: %v", err)
			}
			gotJSON = append(gotJSON, '\n')

			if regenerate {
				if err := os.MkdirAll(goldenDir, 0o755); err != nil {
					t.Fatalf("mkdir golden dir: %v", err)
				}
				if err := os.WriteFile(goldenPath, gotJSON, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("Regenerated %s", goldenPath)
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (regenerate with GENERATE_GOLDEN=1): %v", err)
			}
			if !bytes.Equal(gotJSON, want) {
				t.Errorf("ref projection drift for %s\n--- got ---\n%s\n--- want ---\n%s",
					name, string(gotJSON), string(want))
			}
		})
	}
}

// TestRefRecordSchemaFileExists — a committed JSON schema is part of the
// projection contract.
func TestRefRecordSchemaFileExists(t *testing.T) {
	path := filepath.Join("schemas", "ref_record.v1.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("schema file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("schema file is empty: %s", path)
	}
}
