package ui

import (
	"fmt"
	"strings"
)

// renderSection is one fragment of the bead-detail panel. The detail panel is
// composed by accumulating these in order, concatenating the markdown of all
// "md" sections through Glamour, and then splicing each "ansi" section's
// pre-rendered content back over a unique placeholder line.
//
// The architectural reason for this two-track approach is bt-x5xc4: lipgloss-
// styled output (ANSI SGR sequences) cannot pass through Glamour because
// chroma's code-fence path strips ESC bytes, leaving the rest of every escape
// sequence as visible literal text. The fix used to be a one-off splice for
// the dependency tree; this generalises it so any future styled region plugs
// in the same way.
//
// kind = "md": content is markdown source. It is concatenated into the
// Glamour input directly. placeholder is empty.
//
// kind = "ansi": content is a pre-rendered lipgloss string (already contains
// ANSI SGR sequences). A unique placeholder line is emitted into the Glamour
// input in its position; spliceSections replaces that line with content in
// the rendered output.
type renderSection struct {
	kind        string // "md" or "ansi"
	content     string // markdown source OR pre-rendered ANSI
	placeholder string // empty for "md" sections; unique BTXSECTIONNNN for "ansi"
}

// sectionPlaceholder returns a monotonic, collision-free placeholder ID for
// the n-th ANSI section in a given render. Pure uppercase letters + digits,
// no underscores: Glamour treats underscores as word-wrap boundaries and
// emits a separate SGR block around each segment, breaking substring search
// (bt-x5xc4). The "BTXSECTION" prefix is project-scoped so collisions with
// user content are astronomically unlikely.
func sectionPlaceholder(n int) string {
	return fmt.Sprintf("BTXSECTION%03d", n)
}

// buildMarkdownSource concatenates the markdown fragments and placeholder
// lines that make up the Glamour input for a slice of sections. ANSI
// sections emit their placeholder surrounded by blank lines so Glamour
// treats them as standalone paragraphs (no inline-wrapping interference).
func buildMarkdownSource(sections []renderSection) string {
	var sb strings.Builder
	for _, s := range sections {
		switch s.kind {
		case "md":
			sb.WriteString(s.content)
		case "ansi":
			sb.WriteString(s.placeholder)
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}

// spliceSections walks the ANSI sections in `sections` and replaces each
// section's placeholder line in `rendered` with the section's pre-rendered
// content. Missing placeholders are no-ops (safe for prefix renders that
// don't include a given section). Sections are independent: order in the
// slice doesn't matter for correctness, only for the markdown-source build.
func spliceSections(rendered string, sections []renderSection) string {
	for _, s := range sections {
		if s.kind != "ansi" || s.placeholder == "" || s.content == "" {
			continue
		}
		idx := strings.Index(rendered, s.placeholder)
		if idx < 0 {
			continue
		}
		lineStart := strings.LastIndex(rendered[:idx], "\n") + 1
		lineEnd := idx + len(s.placeholder)
		if nl := strings.Index(rendered[lineEnd:], "\n"); nl >= 0 {
			lineEnd += nl
		} else {
			lineEnd = len(rendered)
		}
		rendered = rendered[:lineStart] + s.content + rendered[lineEnd:]
	}
	return rendered
}
