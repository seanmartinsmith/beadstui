# Dependency Audit

**Tool:** govulncheck (Go's official vuln scanner)
**Date:** 2026-04-27
**Result:** 0 exploitable, 4 advisory-only

```
=== Symbol Results ===

No vulnerabilities found.

Your code is affected by 0 vulnerabilities.
This scan also found 2 vulnerabilities in packages you import and 2
vulnerabilities in modules you require, but your code doesn't appear to call
these vulnerabilities.
```

## Advisory-Only (Imported / Required, Not Called)

| ID | Module | Found | Fix | Why bt is unaffected |
|----|--------|-------|-----|----------------------|
| GO-2026-4503 | `filippo.io/edwards25519` | v1.1.0 | v1.1.1 | `MultiScalarMult` invalid result. bt's chain (via `go-sql-driver/mysql`) only calls `ScalarBaseMult`. |
| GO-2026-4815 | `golang.org/x/image/tiff` | v0.35.0 | v0.38.0 | OOM from malicious TIFF IFD offset. bt imports `font/basicfont` only; never the TIFF decoder. |
| GO-2026-4961 | `golang.org/x/image/webp` | v0.35.0 | v0.39.0 | Panic decoding large WEBP on 32-bit platforms. Same — TIFF/WEBP not imported. |
| GO-2026-4962 | `golang.org/x/image/font/sfnt` | v0.35.0 | v0.39.0 | Excessive allocation decoding malicious SFNT. bt's `font/basicfont` use does not exercise this. |

## Recommended Hygiene Bumps

```bash
go get golang.org/x/image@latest      # v0.35.0 → v0.39.0
go get filippo.io/edwards25519@latest # v1.1.0 → v1.1.1
go mod tidy
go vet ./...
go test ./...
```

These bumps don't fix any exploitable issue, but eliminate the noise from `govulncheck` and keep bt visually clean to security tooling.

## Direct Dependencies (33 modules)

```
charm.land/bubbles/v2 v2.1.0
charm.land/bubbletea/v2 v2.0.2
charm.land/glamour/v2 v2.0.0
charm.land/huh/v2 v2.0.3
charm.land/lipgloss/v2 v2.0.2
git.sr.ht/~sbinet/gg v0.7.0
github.com/Dicklesworthstone/toon-go v0.0.0-20260124164058-e044b09590e8
github.com/ajstarks/svgo v0.0.0-20211024235047-1546f124cd8b
github.com/atotto/clipboard v0.1.4
github.com/charmbracelet/colorprofile v0.4.2
github.com/fsnotify/fsnotify v1.9.0
github.com/go-sql-driver/mysql v1.9.3
github.com/goccy/go-json v0.10.5
github.com/mattn/go-runewidth v0.0.21
github.com/spf13/cobra v1.10.2
github.com/spf13/pflag v1.0.10
golang.org/x/image v0.35.0          ← bump recommended
golang.org/x/sync v0.19.0
golang.org/x/sys v0.42.0
golang.org/x/term v0.39.0
gonum.org/v1/gonum v0.17.0
gopkg.in/yaml.v3 v3.0.1
modernc.org/sqlite v1.44.2
pgregory.net/rapid v1.2.0
filippo.io/edwards25519 v1.1.0      ← bump recommended (indirect via go-sql-driver)
```

All other indirects are within their respective modules' patch lines. No `replace` directives or local module pins.

## Supply Chain Posture

- **Module pinning:** All deps pinned to specific versions in `go.mod`. `go.sum` integrity-checks every download.
- **Build pipeline:** GoReleaser pinned to `~> v2.15` (minor-line). Action versions pinned by major (`@v4`/`@v6`).
- **Hash verification on releases:** Self-updater verifies SHA256 of downloaded archives against `_checksums.txt` from the same release. Authorization header stripped on cross-host redirects.
- **No vendored dependencies in the build path.** (`vendor/` exists for offline builds; module cache is canonical.)
- **One supply-chain weakness flagged:** GitHub Actions script injection in `fuzz.yml` (Low; mitigated by write-access requirement) — see Finding 5.
