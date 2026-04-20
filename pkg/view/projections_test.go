package view

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// projectionFixture is the JSON-on-disk shape used by the golden harness.
// Each fixture lists the source issues fed into a projection constructor;
// the golden file records the expected projection output.
type projectionFixture struct {
	Description string        `json:"description"`
	Issues      []model.Issue `json:"issues"`
}

// TestCompactIssueGolden runs every fixture in testdata/fixtures through
// CompactAll and asserts the JSON-serialized output matches the committed
// golden file at testdata/golden/<name>.json.
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
