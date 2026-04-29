# Audit Report: Build, Config & WASM

**Team**: 8b
**Scope**: bv-graph-wasm/, pkg/export/wasm_scorer/, go.mod, go.sum, .goreleaser.yaml, Makefile, .github/workflows/, flake.nix, install scripts, root-level files, scripts/
**Lines scanned**: ~10,200 (WASM Rust: ~7,500; workflows: ~670; root configs/scripts: ~2,000+)

## Architecture Summary

The build and release pipeline has three layers: a Go build system (go.mod, Makefile, CI), a Nix flake for reproducible builds, and a GoReleaser-driven release pipeline that publishes to GitHub Releases, Homebrew tap, and Scoop bucket. CI runs on push/PR to main with build, unit tests, E2E tests, per-package coverage enforcement, benchmarks, and Codecov upload. Separate workflows handle nightly fuzz testing (7 targets in pkg/loader), weekly Nix flake updates, auto-generated release notes, and ACFS checksum notifications.

There are two separate Rust/WASM modules in the repo that serve different purposes. The root-level `bv-graph-wasm/` (6,983 LOC Rust + 510 test LOC) is a comprehensive graph algorithm library (pagerank, betweenness, cycles, HITS, etc.) built with wasm-pack for browser use. It is a standalone Rust crate with its own Cargo.toml, Makefile, and Cargo.lock - completely decoupled from the Go build. The second WASM module `pkg/export/wasm_scorer/` (169 LOC) is a lightweight hybrid scorer for the static viewer export, built via `scripts/build_hybrid_wasm.sh`. Both are referenced by the embedded viewer assets in `pkg/export/viewer_assets/`.

The Go dependency set includes 22 direct dependencies spanning TUI (Charm Bracelet suite), data (MySQL driver, SQLite, JSONL), graph analysis (gonum), file watching (fsnotify), structured output (toon-go, goccy/go-json), clipboard, and property-based testing (rapid). The vendor directory is committed. Several dependencies are heavy (gonum pulls in BLAS/LAPACK/matrix libraries) but are actively used by pkg/analysis.

## Feature Inventory

| Feature | Location | LOC | Dolt-Compatible | Tested | Functional | Notes |
|---------|----------|-----|-----------------|--------|------------|-------|
| Go module definition | go.mod | 77 | N/A | N/A | Yes | go 1.25 with toolchain 1.25.5 |
| GoReleaser config | .goreleaser.yaml | 76 | N/A | N/A | Yes | Builds linux/darwin/windows amd64+arm64 (no win/arm64); Homebrew tap + Scoop bucket |
| CI pipeline | .github/workflows/ci.yml | 127 | N/A | N/A | Yes | Build + test + coverage thresholds + benchmarks + Codecov |
| Release pipeline | .github/workflows/release.yml | 32 | N/A | N/A | Yes | Tag-triggered GoReleaser |
| Nightly fuzz testing | .github/workflows/fuzz.yml | 156 | N/A | N/A | Yes | 7 fuzz targets in pkg/loader, 10min each, crasher detection |
| Nix flake build | flake.nix + flake.lock | 140 | N/A | N/A | Yes | Reproducible build with devShell; version hardcoded as "0.14.4" |
| Flake auto-update | .github/workflows/flake-update.yml | 76 | N/A | N/A | Yes | Weekly nixpkgs update PRs |
| Release notes auto-gen | .github/workflows/release-notes.yml | 67 | N/A | N/A | Yes | Idempotent, skips if body > 50 chars |
| ACFS notify (dispatch) | .github/workflows/acfs-checksums-dispatch.yml | 31 | N/A | N/A | Yes | Triggers on install.sh push + releases |
| ACFS notify (installer) | .github/workflows/notify-acfs.yml | 47 | N/A | N/A | Partial | Triggers on install.sh push only; overlaps with acfs-checksums-dispatch |
| Install script (bash) | install.sh | 607 | N/A | N/A | Yes | Binary download with Go source fallback; macOS Go auto-install |
| Install script (PS) | install.ps1 | 151 | N/A | N/A | Yes | Source-only build on Windows |
| Makefile | Makefile | 21 | N/A | N/A | Broken | Still references `bv` binary and `cmd/bv` path |
| bv-graph-wasm | bv-graph-wasm/ | 7,493 | N/A | Yes (Rust) | Unknown | Standalone Rust WASM crate; not built by Go CI |
| wasm_scorer (hybrid) | pkg/export/wasm_scorer/ | 169+test | N/A | Yes (Rust) | Unknown | Lightweight scorer for viewer export |
| Viewer assets | pkg/export/viewer_assets/ | ~12 files + vendor | N/A | Partial | Yes | Embedded in binary via go:embed; includes compiled WASM binaries |
| Version package | pkg/version/version.go | 58 | N/A | N/A | Yes | Three-tier: ldflags > build info > "v0.0.1" fallback |
| Codecov config | codecov.yml | 35 | N/A | N/A | Yes | 85% project target, pkg/ only |
| Coverage script | scripts/coverage.sh | 219 | N/A | N/A | Yes | Local coverage with per-package thresholds |
| Benchmark scripts | scripts/benchmark*.sh | ~180 | N/A | N/A | Yes | Full + quick + compare modes |
| WASM build script | scripts/build_hybrid_wasm.sh | 23 | N/A | N/A | Yes | Builds wasm_scorer to viewer_assets/wasm |
| E2E helper scripts | scripts/e2e_*.sh, scripts/test_*.sh | ~550 | N/A | N/A | Unknown | Shell-based E2E tests for specific features |
| Screenshots | screenshots/ | 4 webp + converter | N/A | N/A | N/A | README images |
| .gitattributes | .gitattributes | 3 | N/A | N/A | Yes | Beads JSONL merge driver |
| .ubsignore | .ubsignore | 7 | N/A | N/A | Yes | Excludes large non-code dirs |
| AGENTS.md | AGENTS.md | ~870 lines | N/A | N/A | Yes | AI agent guide for codebase |
| SKILL.md | SKILL.md | ~250 lines | N/A | N/A | Yes | Agent CLI guide for robot mode |
| README.md | README.md | ~5,000+ lines | N/A | N/A | Partial | Large; may have stale content from fork |
| LICENSE | LICENSE | 75 | N/A | N/A | Yes | MIT + OpenAI/Anthropic rider; dual copyright |

## Dependencies

### Direct Go Dependencies (go.mod)

| Package | Purpose | Used By | Notes |
|---------|---------|---------|-------|
| charmbracelet/bubbletea | TUI framework | pkg/ui | Core framework |
| charmbracelet/lipgloss | TUI styling | pkg/ui | Core framework |
| charmbracelet/bubbles | TUI components | pkg/ui | Core framework |
| charmbracelet/glamour | Markdown rendering | pkg/ui | Renders issue descriptions |
| charmbracelet/huh | Form/wizard TUI | pkg/export/wizard.go | Used by export wizard only |
| charmbracelet/colorprofile | Color detection | pkg/ui/theme.go | Terminal color capability |
| go-sql-driver/mysql | MySQL/Dolt driver | internal/datasource | Dolt connection |
| modernc.org/sqlite | SQLite driver (pure Go) | internal/datasource, pkg/export | Legacy backend + export |
| gonum.org/v1/gonum | Graph/math library | pkg/analysis | PageRank, betweenness, cycles, etc. |
| git.sr.ht/~sbinet/gg | 2D graphics | pkg/export/graph_snapshot.go | SVG graph rendering |
| github.com/ajstarks/svgo | SVG generation | pkg/export/graph_snapshot.go | SVG graph rendering |
| golang.org/x/image | Image processing | (indirect via gg) | Dependency of gg |
| fsnotify/fsnotify | File watching | pkg/watcher, internal/datasource | Hot reload |
| goccy/go-json | Fast JSON | cmd/bt/main.go | Robot mode output |
| Dicklesworthstone/toon-go | TOON encoding | cmd/bt/main.go | Token-optimized output for AI agents |
| atotto/clipboard | Clipboard access | pkg/ui/model.go | Copy to clipboard |
| spf13/pflag | CLI flags | cmd/bt/main.go | Flag parsing |
| mattn/go-runewidth | Unicode width | (likely indirect) | Terminal width calculations |
| golang.org/x/sync | Sync primitives | pkg/workspace/loader.go | Concurrent loading |
| golang.org/x/sys | System calls | (various) | Platform-specific ops |
| golang.org/x/term | Terminal queries | cmd/bt/main.go | Terminal size detection |
| gopkg.in/yaml.v3 | YAML parsing | (theme, config) | Theme/config files |
| pgregory.net/rapid | Property testing | pkg/testutil/proptest | Test-only dependency (in direct requires) |

### Dependency Concerns

- **gonum** is the heaviest dependency, pulling in BLAS, LAPACK, and full matrix libraries via vendor/. It is actively used by pkg/analysis (6 non-vendor Go files import it). This is justified for graph algorithms.
- **pgregory.net/rapid** is a test-only dependency listed in direct `require` block rather than a test-only section. Go modules don't distinguish test deps in go.mod, but this is worth noting.
- **modernc.org/sqlite** is used for legacy SQLite backend and export. Since upstream beads removed SQLite (v0.56.1), the datasource usage may be removable, but export still needs it.
- **toon-go** is from Dicklesworthstone (original upstream author). One import in main.go for robot mode TOON encoding.
- **golang.org/x/sync** is imported by only 1 file (pkg/workspace/loader.go).

### External Dependencies

- **Depends on**: Go 1.25+ toolchain, Rust/wasm-pack (WASM builds only), Nix (optional), Dolt (runtime)
- **Depended on by**: Nothing external (end-user binary)

## Dead Code Candidates

1. **Root Makefile** (`Makefile:1-21`): Still references `bv` binary name and `cmd/bv` path. Does not build anything useful. The comment says "bv Makefile" and targets build `go build -o bv ./cmd/bv` which no longer exists. **Stale/broken.**

2. **`bv-graph-wasm/`** (entire directory, 7,493 LOC Rust): This standalone WASM crate is referenced in exactly two places:
   - `pkg/export/viewer_assets/graph-demo.html` line 596: `import('../../../bv-graph-wasm/pkg/bv_graph_wasm.js')` - a relative path that only works during local development
   - `pkg/export/viewer_assets/vendor/bv_graph.js` and `bv_graph_bg.wasm` - pre-compiled artifacts already committed to viewer_assets

   The crate is NOT built by CI, NOT referenced by any Go code, and its algorithms are re-implemented in Go in `pkg/analysis/`. The pre-compiled WASM artifacts in `viewer_assets/vendor/` are the only connection to the running binary. The source is essentially dead weight - the compiled artifacts are what matter, and those are already committed. **Strong dead code candidate.**

3. **`bv_test`** (50MB, root): A Linux ELF binary sitting in the repo root. Named `bv_test` (old `bv` prefix, no file extension). At 50MB it is not tracked by `.gitignore` patterns (`*.test` wouldn't match, `*.exe` wouldn't match). If committed to git, this bloats the repo significantly. If not committed, it's a local build artifact that should be cleaned up and added to `.gitignore`.

4. **Duplicate ACFS workflows**: `acfs-checksums-dispatch.yml` and `notify-acfs.yml` both dispatch to the same ACFS repo (`seanmartinsmith/agentic_coding_flywheel_setup`) with `event-type: upstream-changed`. The first triggers on install.sh push + releases; the second triggers only on install.sh push. They use different payload formats and different token secret handling (first gracefully skips if token missing, second does not). **One of these should be removed.**

5. **`scripts/build_hybrid_wasm.sh`**: References `pkg/export/wasm_scorer` which exists, but the output directory `pkg/export/viewer_assets/wasm` doesn't appear to have committed WASM artifacts. The `wasm_loader.js` tries to import from `./wasm/bv_hybrid_scorer.js`. If this WASM has never been built and committed, the hybrid scorer path always falls back to JS. Possibly vestigial.

6. **Several shell scripts in `scripts/`** reference "bv" in comments:
   - `scripts/coverage.sh` line 2: "Coverage script for bv"
   - `scripts/benchmark.sh` line 2: "Benchmark script for bv graph analysis"

## Notable Findings

1. **Makefile is completely broken**: It builds `go build -o bv ./cmd/bv` but the binary is now `bt` and the entry point is `cmd/bt/`. Anyone running `make build` or `make install` gets a build error. The Makefile also enables CGO FTS5 flags, which the actual build (goreleaser, CI, Nix) does not use.

2. **Version mismatch between flake.nix and reality**: `flake.nix` hardcodes `version = "0.14.4"` while `pkg/version/version.go` has `fallback = "v0.0.1"`. The flake version appears to be a carryover from the upstream fork and has not been updated to match the project's reset to v0.0.1.

3. **Install scripts specify Go 1.21 minimum**: Both `install.sh` and `install.ps1` set minimum Go version to 1.21, but `go.mod` requires Go 1.25. Users installing from source via the install scripts with Go 1.21-1.24 would get a build failure from the Go toolchain, not a friendly error from the installer.

4. **Pre-compiled WASM binaries in vendor/**: `pkg/export/viewer_assets/vendor/` contains `bv_graph.js` and `bv_graph_bg.wasm` (pre-compiled from bv-graph-wasm/) and `sql-wasm.js` + `sql-wasm.wasm` (SQLite WASM for browser). These are embedded into the Go binary via `go:embed`. The `bv_graph` prefix is a remnant of the old naming.

5. **CI tests only `pkg/...` and `cmd/bt` for coverage**: The `internal/` directory is not included in coverage collection (ci.yml line 24: `go test -v -covermode=atomic -coverprofile=coverage.out ./pkg/... ./cmd/bt`). The E2E tests run separately without coverage. `internal/datasource/` and `internal/dolt/` and `internal/doltctl/` have no coverage tracking in CI.

6. **Coverage threshold mismatch**: CI enforces `pkg/export` at 69% threshold (ci.yml line 79), but `scripts/coverage.sh` enforces it at 80% (line 33). This discrepancy means local coverage checks are stricter than CI.

7. **The `bv-graph-wasm/` crate has `repository` pointing to old upstream**: `Cargo.toml` line 7: `repository = "https://github.com/Dicklesworthstone/beads_viewer"`. This should be updated if the crate is kept.

8. **`graph-demo.html` uses relative path to WASM source**: Line 596 attempts `import('../../../bv-graph-wasm/pkg/bv_graph_wasm.js')` which only works in a development file layout, not when exported as a static site. This file appears to be a development/demo page embedded in the binary.

9. **Two separate WASM scoring pipelines**: The `bv-graph-wasm/` crate does graph algorithms (pagerank, etc.), while `pkg/export/wasm_scorer/` does hybrid scoring (weighted combination of metrics). The viewer's `wasm_loader.js` loads only the hybrid scorer, not the graph algorithms crate directly. The graph algorithms are instead available via the pre-compiled `vendor/bv_graph.js`.

10. **GoReleaser scoop bucket token reuses Homebrew token**: `.goreleaser.yaml` line 72: `token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"` is used for the Scoop bucket repo too. This works but the naming is misleading.

## Questions for Synthesis

1. **Should `bv-graph-wasm/` be removed entirely?** The compiled artifacts are already in `viewer_assets/vendor/`. The 7,500 LOC Rust source adds maintenance burden with no CI building it. Alternatively, it could be kept as a reference but excluded from the main repo.

2. **Is the `bv_test` (50MB ELF binary) committed to git?** If so, it should be removed from history (BFG or filter-branch) as it bloats the repo. If not, it should be added to `.gitignore`.

3. **Should the root `Makefile` be fixed or removed?** It's completely broken. Options: fix it to build `bt` from `cmd/bt/`, or remove it since `go build ./cmd/bt/` is simpler and the project uses goreleaser for releases.

4. **Which ACFS notification workflow should be kept?** `acfs-checksums-dispatch.yml` is more comprehensive (triggers on releases too, handles missing token gracefully). `notify-acfs.yml` appears to be an earlier version that could be removed.

5. **Should `internal/` packages be included in CI coverage?** Currently only `pkg/...` and `cmd/bt` are measured. The internal packages contain meaningful logic (datasource, dolt, doltctl).

6. **Should the install scripts' Go version minimum be updated to 1.25?** The current 1.21 minimum doesn't match `go.mod` requirements.

7. **Is `modernc.org/sqlite` still needed?** It's used for export (generating SQLite files) and for the legacy datasource. If the export-to-SQLite feature is kept, the dependency stays. If only Dolt matters going forward, it could potentially be removed.

8. **Should `flake.nix` version be synced to v0.0.1?** The current "0.14.4" is misleading and comes from the upstream fork.

9. **What is the maintenance plan for the pre-compiled WASM artifacts in `viewer_assets/vendor/`?** They have old `bv_graph` naming and there's no CI step to rebuild them. Are they expected to be updated, or is the static export feature frozen?
