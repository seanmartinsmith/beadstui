# Dependency Upgrade Log

**Date:** 2026-01-18  |  **Project:** beads_viewer  |  **Language:** Go

## Summary
- **Updated:** 27 modules
- **Skipped:** 0
- **Failed:** 0
- **Needs attention:** 0

## Major Updates

| Module | Old | New |
|--------|-----|-----|
| git.sr.ht/~sbinet/gg | v0.6.0 | v0.7.0 |
| github.com/alecthomas/chroma/v2 | v2.14.0 | v2.23.0 |
| github.com/charmbracelet/colorprofile | v0.2.3 | v0.4.1 |
| github.com/charmbracelet/x/ansi | v0.10.1 | v0.11.4 |
| github.com/lucasb-eyer/go-colorful | v1.2.0 | v1.3.0 |
| github.com/mattn/go-runewidth | v0.0.16 | v0.0.19 |
| github.com/ncruces/go-strftime | v0.1.9 | v1.0.0 |
| golang.org/x/image | v0.25.0 | v0.35.0 |
| golang.org/x/sync | v0.16.0 | v0.19.0 |
| golang.org/x/term | v0.31.0 | v0.39.0 |
| gonum.org/v1/gonum | v0.16.0 | v0.17.0 |
| modernc.org/sqlite | v1.40.1 | v1.44.2 |

## Verification

- `go build ./...` - Build successful
- `go test ./...` - All tests pass (25 packages)
- `go mod vendor` - Vendor directory synced
