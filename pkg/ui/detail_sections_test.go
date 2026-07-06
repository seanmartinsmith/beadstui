package ui

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/viewport"
	"charm.land/glamour/v2"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// TestSpliceSections_Empty verifies that an empty section list passes the
// rendered output through unchanged.
func TestSpliceSections_Empty(t *testing.T) {
	rendered := "  hello\n  world\n"
	got := spliceSections(rendered, nil)
	if got != rendered {
		t.Fatalf("nil sections should pass rendered through unchanged\ngot:  %q\nwant: %q", got, rendered)
	}
	got = spliceSections(rendered, []renderSection{
		{kind: "md", content: "ignored"},
	})
	if got != rendered {
		t.Fatalf("md-only sections should pass rendered through unchanged\ngot:  %q\nwant: %q", got, rendered)
	}
}

// TestSpliceSections_NoMarker confirms a missing placeholder is a no-op
// rather than a panic — safe for prefix renders that don't include a
// given section.
func TestSpliceSections_NoMarker(t *testing.T) {
	rendered := "  some content\n  more content\n"
	sections := []renderSection{
		{kind: "ansi", content: "TREE_CONTENT", placeholder: "BTXSECTION001"},
	}
	got := spliceSections(rendered, sections)
	if got != rendered {
		t.Fatalf("missing marker should leave rendered unchanged\ngot:  %q\nwant: %q", got, rendered)
	}
}

// TestSpliceSections_ReplacesWholeLine confirms the entire line containing
// the placeholder (including any Glamour-applied indentation or paragraph
// SGR around it) is removed and replaced with the ANSI content. This is
// the core invariant that makes lipgloss-styled output drop in cleanly
// without surrounding chroma-corrupted artifacts.
func TestSpliceSections_ReplacesWholeLine(t *testing.T) {
	placeholder := sectionPlaceholder(1)
	rendered := "before\n  \x1b[38;5;79m  " + placeholder + "  \x1b[0m\nafter\n"
	content := "TREE_LINE_1\nTREE_LINE_2"
	sections := []renderSection{
		{kind: "ansi", content: content, placeholder: placeholder},
	}
	got := spliceSections(rendered, sections)
	want := "before\nTREE_LINE_1\nTREE_LINE_2\nafter\n"
	if got != want {
		t.Fatalf("splice did not replace whole line\ngot:  %q\nwant: %q", got, want)
	}
}

// TestSpliceSections_MarkerOnLastLine handles a placeholder on the final
// line of the rendered output (no trailing newline).
func TestSpliceSections_MarkerOnLastLine(t *testing.T) {
	placeholder := sectionPlaceholder(1)
	rendered := "header\n" + placeholder
	sections := []renderSection{
		{kind: "ansi", content: "TREE", placeholder: placeholder},
	}
	got := spliceSections(rendered, sections)
	want := "header\nTREE"
	if got != want {
		t.Fatalf("trailing marker not replaced\ngot:  %q\nwant: %q", got, want)
	}
}

// TestSpliceSections_MultiplePlaceholders verifies that two ANSI sections
// with distinct placeholders each splice cleanly without leakage. This is
// the multi-section invariant — placeholders are independent; ordering in
// the slice doesn't affect correctness.
func TestSpliceSections_MultiplePlaceholders(t *testing.T) {
	p1 := sectionPlaceholder(1)
	p2 := sectionPlaceholder(2)
	rendered := "head\n" + p1 + "\nbetween\n" + p2 + "\ntail\n"
	sections := []renderSection{
		{kind: "ansi", content: "AAA1\nAAA2", placeholder: p1},
		{kind: "md", content: "between\n"},
		{kind: "ansi", content: "BBB1\nBBB2", placeholder: p2},
	}
	got := spliceSections(rendered, sections)
	want := "head\nAAA1\nAAA2\nbetween\nBBB1\nBBB2\ntail\n"
	if got != want {
		t.Fatalf("multi-placeholder splice mismatch\ngot:  %q\nwant: %q", got, want)
	}
	if strings.Contains(got, p1) || strings.Contains(got, p2) {
		t.Errorf("placeholders still present after multi-splice:\n%s", got)
	}
}

// TestSpliceSections_EndToEnd_GlamourPlusSplice verifies the full pipeline:
// the placeholder survives a real Glamour render, spliceSections locates it
// in the rendered output, and the ANSI content replaces it without leaking
// any chroma-mangled bracket sequences. Smoke test that protects against
// the original bt-x5xc4 bug regressing on the generalised primitive.
func TestSpliceSections_EndToEnd_GlamourPlusSplice(t *testing.T) {
	placeholder := sectionPlaceholder(1)
	source := "### Heading\n\n" +
		"Some prose before the tree.\n\n" +
		placeholder + "\n\n" +
		"Some prose after the tree.\n"

	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	rendered, err := r.Render(source)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	// The placeholder ASCII text must survive Glamour (no word-wrap, no
	// transformation — it has no spaces and is short enough to fit).
	if !strings.Contains(rendered, placeholder) {
		t.Fatalf("placeholder did not survive Glamour render; cannot splice:\n%s", rendered)
	}

	content := "\x1b[32mROW_1\x1b[0m\n\x1b[32mROW_2\x1b[0m"
	sections := []renderSection{
		{kind: "ansi", content: content, placeholder: placeholder},
	}
	final := spliceSections(rendered, sections)

	if strings.Contains(final, placeholder) {
		t.Errorf("placeholder still present after splice:\n%s", final)
	}
	if !strings.Contains(final, "ROW_1") || !strings.Contains(final, "ROW_2") {
		t.Errorf("section content missing from spliced output:\n%s", final)
	}
	// The bug we're protecting against: chroma stripping ESC bytes inside a
	// code fence. Because we splice past Glamour, the raw ESC byte (0x1B)
	// from the section's SGR must reach the final output intact.
	if !strings.Contains(final, "\x1b[32m") {
		t.Errorf("ESC-prefixed SGR sequences from section did not survive splice (bt-x5xc4 regression):\n%q", final)
	}
}

// TestUpdateViewportContent_PropertyBlock_NoFence verifies that the migrated
// property block (Author/Assignee/Created/Updated/Labels rows) is no longer
// emitted as a triple-backtick code fence. The fenced-code path used chroma
// and was the largest remaining bt-x5xc4-class trap before this migration.
// The rendered viewport content should contain the row labels but neither
// a fence delimiter nor the raw ANSI-escape literal text chroma leaks when
// it strips ESC bytes from styled content inside a fence.
func TestUpdateViewportContent_PropertyBlock_NoFence(t *testing.T) {
	issues := []model.Issue{{
		ID:        "bt-pb",
		Title:     "Property block test",
		Status:    model.StatusOpen,
		Author:    "sms",
		Assignee:  "sms",
		CreatedAt: time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 15, 13, 0, 0, 0, time.UTC),
		Labels:    []string{"a", "b"},
	}}
	m := NewModel(issues, nil, "", nil, nil)
	m.width = 120
	m.height = 40
	m.mode = ViewList
	m.ready = true
	m.list.Select(0)
	m.viewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))

	m.updateViewportContent()
	content := m.viewport.View()

	if strings.Contains(content, "```") {
		t.Errorf("rendered output contains triple-backtick fence; property block should now be lipgloss-styled:\n%s", content)
	}
	// Row labels must still appear (they're now lipgloss-styled, not fenced).
	for _, label := range []string{"Author", "Assignee", "Created", "Updated", "Labels"} {
		if !strings.Contains(content, label) {
			t.Errorf("rendered output missing property-block label %q:\n%s", label, content)
		}
	}
	// Defense in depth against the chroma ESC-strip artifact: the literal
	// bracket-sequence text (without the leading ESC byte) is the visible
	// symptom of bt-x5xc4. If we see "[38;2;" without an ESC byte before
	// it, something stripped the escape.
	if idx := strings.Index(content, "[38;2;"); idx > 0 {
		if content[idx-1] != 0x1b {
			t.Errorf("ESC-stripped SGR literal at offset %d (bt-x5xc4 regression):\n%s", idx, content)
		}
	}
}
