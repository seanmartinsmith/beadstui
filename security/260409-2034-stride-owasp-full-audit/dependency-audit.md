# Dependency Audit - beadstui (bt)

**Date:** 2026-04-09
**Tool:** govulncheck (golang.org/x/vuln) + manual review
**Go Version:** 1.25 (toolchain go1.25.5)

## govulncheck Results

```
=== Symbol Results ===
No vulnerabilities found.
Your code is affected by 0 vulnerabilities.

=== Package Results (imported but not called) ===
GO-2026-4503: filippo.io/edwards25519@v1.1.0
  Fixed in: v1.1.1
  Impact: MultiScalarMult invalid results
  bt exposure: None - go-sql-driver/mysql only uses ScalarBaseMult

=== Module Results (required but not imported) ===
GO-2026-4815: golang.org/x/image@v0.35.0
  Fixed in: v0.38.0
  Impact: TIFF decoder OOM from malicious IFD offset
  bt exposure: None - only imports font/basicfont
```

## Direct Dependencies (33 total)

| Dependency | Version | Security Role | Known CVEs | Maintained | Risk |
|---|---|---|---|---|---|
| filippo.io/edwards25519 | v1.1.0 | Crypto (indirect via mysql) | GO-2026-4503 (not called) | Active | LOW |
| git.sr.ht/~sbinet/gg | v0.7.0 | 2D graphics | None | Active | LOW |
| github.com/Dicklesworthstone/toon-go | v0.0.0-20260124 | Serialization | None | Nascent (2 stars) | MONITOR |
| github.com/ajstarks/svgo | v0.0.0-20211024 | SVG generation | None | Low activity | LOW |
| github.com/atotto/clipboard | v0.1.4 | System clipboard | None | Inactive | LOW |
| github.com/charmbracelet/bubbles | v0.21.1-pre | TUI widgets | None | Very active | LOW |
| github.com/charmbracelet/bubbletea | v1.3.10 | TUI framework | None | Very active | LOW |
| github.com/charmbracelet/colorprofile | v0.4.1 | Terminal colors | None | Active | LOW |
| github.com/charmbracelet/glamour | v0.10.0 | Markdown render | None | Active | LOW |
| github.com/charmbracelet/huh | v0.8.0 | Forms | None | Active | LOW |
| github.com/charmbracelet/lipgloss | v1.1.1-pre | Styling | None | Very active | LOW |
| github.com/fsnotify/fsnotify | v1.9.0 | File watching | None | Active | LOW |
| github.com/go-sql-driver/mysql | v1.9.3 | Database driver | None | Active | LOW |
| github.com/goccy/go-json | v0.10.5 | JSON parsing | None | Active | LOW |
| github.com/mattn/go-runewidth | v0.0.19 | Text width | None | Active | LOW |
| github.com/spf13/pflag | v1.0.10 | CLI flags | None | Active | LOW |
| golang.org/x/image | v0.35.0 | Font rendering | GO-2026-4815 (not used) | Active | LOW |
| golang.org/x/sync | v0.19.0 | Concurrency | None | Active | LOW |
| golang.org/x/sys | v0.40.0 | OS interface | None | Active | LOW |
| golang.org/x/term | v0.39.0 | Terminal ops | None | Active | LOW |
| gonum.org/v1/gonum | v0.17.0 | Numerical | None | Active | LOW |
| gopkg.in/yaml.v3 | v3.0.1 | YAML parsing | CVE-2022-28948 (fixed) | Stable | LOW |
| modernc.org/sqlite | v1.44.2 | SQLite (cgo-free) | None (embeds 3.51.2) | Active | LOW |
| pgregory.net/rapid | v1.2.0 | Property testing | None | Active | LOW |

## Key Security-Sensitive Indirect Dependencies

| Dependency | Version | Role | Status |
|---|---|---|---|
| golang.org/x/net | v0.49.0 | HTML parsing (via glamour) | CVE-2025-22872, CVE-2025-58190 - both FIXED |
| github.com/microcosm-cc/bluemonday | v1.0.27 | HTML sanitizer (via glamour) | Clean |
| github.com/dlclark/regexp2 | v1.11.5 | Regex (via chroma) | Inherently ReDoS-capable but no CVE |
| github.com/yuin/goldmark | v1.7.16 | Markdown parser | Clean |

## Typosquatting Analysis

No typosquatting risks detected. All dependencies verified against known package registries.

## Recommended Updates

1. **Required:** `go get filippo.io/edwards25519@v1.1.1` (clear the advisory)
2. **Recommended:** `go get golang.org/x/image@v0.38.0` (clear the advisory)
3. **Monitor:** toon-go - nascent dependency, low usage in bt
