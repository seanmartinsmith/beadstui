package ui

import (
	"strings"
	"testing"

	"charm.land/glamour/v2"
)

// TestSpliceDepTree_EmptyTree returns the rendered output unchanged when no
// tree is being spliced (i.e. the markdown source never emitted a placeholder).
func TestSpliceDepTree_EmptyTree(t *testing.T) {
	rendered := "  hello\n  world\n"
	got := spliceDepTree(rendered, "")
	if got != rendered {
		t.Fatalf("empty tree should pass rendered through unchanged\ngot:  %q\nwant: %q", got, rendered)
	}
}

// TestSpliceDepTree_NoMarker handles the case where the placeholder is absent
// (e.g. the marker was somehow stripped) without panicking.
func TestSpliceDepTree_NoMarker(t *testing.T) {
	rendered := "  some content\n  more content\n"
	got := spliceDepTree(rendered, "TREE_CONTENT")
	if got != rendered {
		t.Fatalf("missing marker should leave rendered unchanged\ngot:  %q\nwant: %q", got, rendered)
	}
}

// TestSpliceDepTree_ReplacesWholeLine confirms the entire line containing the
// marker (including any Glamour-applied indentation or paragraph SGR around
// it) is removed and replaced with treeStr. This is the core invariant that
// makes the lipgloss-styled tree drop in cleanly without surrounding
// chroma-corrupted artifacts.
func TestSpliceDepTree_ReplacesWholeLine(t *testing.T) {
	// Simulate Glamour-rendered output: paragraph-style indented placeholder
	// surrounded by ANSI SGR sequences (typical for a paragraph block).
	rendered := "before\n  \x1b[38;5;79m  " + depTreePlaceholder + "  \x1b[0m\nafter\n"
	treeStr := "TREE_LINE_1\nTREE_LINE_2"

	got := spliceDepTree(rendered, treeStr)
	want := "before\nTREE_LINE_1\nTREE_LINE_2\nafter\n"
	if got != want {
		t.Fatalf("splice did not replace whole line\ngot:  %q\nwant: %q", got, want)
	}
}

// TestSpliceDepTree_MarkerOnLastLine handles a marker on the final line of
// the rendered output (no trailing newline).
func TestSpliceDepTree_MarkerOnLastLine(t *testing.T) {
	rendered := "header\n" + depTreePlaceholder
	treeStr := "TREE"
	got := spliceDepTree(rendered, treeStr)
	want := "header\nTREE"
	if got != want {
		t.Fatalf("trailing marker not replaced\ngot:  %q\nwant: %q", got, want)
	}
}

// TestSpliceDepTree_EndToEnd_GlamourPlusSplice verifies the full pipeline:
// the placeholder survives a real Glamour render, spliceDepTree locates it
// in the rendered output, and the tree replaces it without leaking any
// chroma-mangled bracket sequences. This is the smoke test that protects
// against the original bt-x5xc4 bug regressing.
func TestSpliceDepTree_EndToEnd_GlamourPlusSplice(t *testing.T) {
	source := "### Heading\n\n" +
		"Some prose before the tree.\n\n" +
		depTreePlaceholder + "\n\n" +
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
	if !strings.Contains(rendered, depTreePlaceholder) {
		t.Fatalf("placeholder did not survive Glamour render; cannot splice:\n%s", rendered)
	}

	treeStr := "\x1b[32mROW_1\x1b[0m\n\x1b[32mROW_2\x1b[0m"
	final := spliceDepTree(rendered, treeStr)

	if strings.Contains(final, depTreePlaceholder) {
		t.Errorf("placeholder still present after splice:\n%s", final)
	}
	if !strings.Contains(final, "ROW_1") || !strings.Contains(final, "ROW_2") {
		t.Errorf("tree content missing from spliced output:\n%s", final)
	}
	// The bug we're protecting against: chroma stripping ESC bytes inside a
	// code fence. Because we splice past Glamour, the raw ESC byte (0x1B)
	// from the tree's SGR must reach the final output intact.
	if !strings.Contains(final, "\x1b[32m") {
		t.Errorf("ESC-prefixed SGR sequences from tree did not survive splice (bt-x5xc4 regression):\n%q", final)
	}
}
