package search

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestPresetsMatchJavaScript(t *testing.T) {
	jsPresets := loadJSPresets(t)

	goPresets := ListPresets()
	if len(jsPresets) != len(goPresets) {
		t.Fatalf("preset count mismatch: js=%d go=%d", len(jsPresets), len(goPresets))
	}

	for _, name := range goPresets {
		jsWeights, ok := jsPresets[name]
		if !ok {
			t.Fatalf("missing preset %q in JS", name)
		}
		goWeights, err := GetPreset(name)
		if err != nil {
			t.Fatalf("unexpected error for preset %q: %v", name, err)
		}
		compareWeights(t, name, goWeights, jsWeights)
	}

	for name := range jsPresets {
		found := false
		for _, goName := range goPresets {
			if name == goName {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("extra preset %q found in JS", name)
		}
	}
}

func loadJSPresets(t *testing.T) map[PresetName]Weights {
	t.Helper()

	root := projectRoot(t)
	path := filepath.Join(root, "pkg", "export", "viewer_assets", "hybrid_scorer.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read JS presets: %v", err)
	}

	block := extractPresetsBlock(t, string(data))
	entries := parsePresetEntries(t, block)
	return entries
}

func projectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// tests run from pkg/search; go up to repo root
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	return root
}

func extractPresetsBlock(t *testing.T, content string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)const\s+HYBRID_PRESETS\s*=\s*\{(.*?)\};`)
	m := re.FindStringSubmatch(content)
	if len(m) < 2 {
		t.Fatalf("failed to locate HYBRID_PRESETS block in JS")
	}
	return m[1]
}

func parsePresetEntries(t *testing.T, block string) map[PresetName]Weights {
	t.Helper()
	entryRe := regexp.MustCompile(`(?m)^\s*([A-Za-z0-9_-]+|'[^']+'|"[^"]+")\s*:\s*\{([^}]*)\}`)
	matches := entryRe.FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		t.Fatalf("no presets parsed from JS block")
	}

	out := make(map[PresetName]Weights, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		name := normalizeJSKey(match[1])
		weights := parseWeights(t, name, match[2])
		out[PresetName(name)] = weights
	}
	return out
}

func normalizeJSKey(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'`)
	return raw
}

func parseWeights(t *testing.T, presetName, body string) Weights {
	t.Helper()

	fieldRe := regexp.MustCompile(`([a-zA-Z_]+)\s*:\s*([0-9.]+)`)
	matches := fieldRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		t.Fatalf("no fields parsed for preset %q", presetName)
	}

	values := make(map[string]float64, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		val, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			t.Fatalf("parse weight %q for preset %q: %v", match[1], presetName, err)
		}
		values[match[1]] = val
	}

	required := []string{"text", "pagerank", "status", "impact", "priority", "recency"}
	for _, key := range required {
		if _, ok := values[key]; !ok {
			t.Fatalf("preset %q missing key %q in JS", presetName, key)
		}
	}

	return Weights{
		TextRelevance: values["text"],
		PageRank:      values["pagerank"],
		Status:        values["status"],
		Impact:        values["impact"],
		Priority:      values["priority"],
		Recency:       values["recency"],
	}
}

func compareWeights(t *testing.T, name PresetName, goW, jsW Weights) {
	t.Helper()

	assertClose := func(field string, got, want float64) {
		const tol = 1e-9
		if diff := got - want; diff < -tol || diff > tol {
			t.Fatalf("preset %q %s mismatch: go=%v js=%v", name, field, got, want)
		}
	}

	assertClose("text", goW.TextRelevance, jsW.TextRelevance)
	assertClose("pagerank", goW.PageRank, jsW.PageRank)
	assertClose("status", goW.Status, jsW.Status)
	assertClose("impact", goW.Impact, jsW.Impact)
	assertClose("priority", goW.Priority, jsW.Priority)
	assertClose("recency", goW.Recency, jsW.Recency)
}

func TestPresetsMatchJavaScript_ParseGuard(t *testing.T) {
	// Quick guard to ensure the parser doesn't silently accept empty content.
	_, err := regexp.Compile(`const\s+HYBRID_PRESETS`)
	if err != nil {
		t.Fatalf("regex compile: %v", err)
	}
	if len(ListPresets()) == 0 {
		t.Fatal("expected at least one Go preset")
	}
	if _, err := os.Stat(filepath.Join(projectRoot(t), "pkg", "export", "viewer_assets", "hybrid_scorer.js")); err != nil {
		t.Fatalf("expected JS presets file: %v", err)
	}
}
