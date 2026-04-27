# Triage: export

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action |
|----|----|----|----|----|
| bt-gk9y | Security: add timeouts to PreviewServer http.Server | GREEN | Pure http.Server hardening in pkg/export/preview.go; no storage-layer assumptions, fully Dolt-agnostic. | None. |
| bt-n7i5 | Investigate x/export keybind - unclear UX, dumps all beads to project root | GREEN | TUI/UX investigation about keybind scope and output path; framing is storage-agnostic. | None. |

## Bucket totals
- GREEN: 2
- YELLOW: 0
- RED: 0

## Notes / cross-bucket observations

- Neither bead in this bucket touches the static-site export pipeline at `cobra_export.go:319` where the known JSONL pin lives. bt-gk9y is about the preview HTTP server lifecycle (timeouts), and bt-n7i5 is about the TUI 'x' keybind UX (confirmation, scope, output path). Both are orthogonal to the source-of-truth question.
- If/when a bead lands that touches the static-site export's data-load path, that's the one to scrutinize against the JSONL pin. None such here.
- bt-gk9y has a parent bt-6cdi (not in this bucket) — classification is consistent regardless of parent since the work is self-contained server hardening.
