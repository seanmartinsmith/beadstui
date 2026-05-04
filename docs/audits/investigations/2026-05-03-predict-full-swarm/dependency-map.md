---
commit_hash: 5b3767830036f23ad425f1223edd62fb1982cf3b
---

## External dependencies (go.mod direct)

| Module | Version | Risk profile |
|---|---|---|
| charm.land/bubbletea/v2 | v2.0.2 | TUI runtime — major-version stable; pinned to 2.0.x |
| charm.land/bubbles/v2 | v2.1.0 | TUI components |
| charm.land/lipgloss/v2 | v2.0.2 | Style engine |
| charm.land/glamour/v2 | v2.0.0 | Markdown rendering |
| charm.land/huh/v2 | v2.0.3 | Forms |
| github.com/charmbracelet/colorprofile | v0.4.2 | Terminal color detection |
| github.com/spf13/cobra | v1.10.2 | CLI framework |
| github.com/spf13/pflag | v1.0.10 | Flag parsing |
| github.com/go-sql-driver/mysql | v1.9.3 | Dolt protocol (MySQL-compatible) |
| modernc.org/sqlite | v1.44.2 | Pure-Go SQLite — used by export AND runtime SQLite reader |
| github.com/fsnotify/fsnotify | v1.9.0 | File watcher |
| github.com/atotto/clipboard | v0.1.4 | Clipboard integration |
| github.com/goccy/go-json | v0.10.5 | Faster JSON encoding |
| github.com/mattn/go-runewidth | v0.0.21 | Terminal width math |
| github.com/Dicklesworthstone/toon-go | v0.0.0-20260124... | TOON output format (compact JSON-ish) |
| gonum.org/v1/gonum | v0.17.0 | Graph algorithms |
| github.com/ajstarks/svgo | (old) | SVG export |
| git.sr.ht/~sbinet/gg | v0.7.0 | 2D graphics for charts |
| golang.org/x/image | v0.35.0 | Image processing |
| golang.org/x/sync | v0.19.0 | errgroup, semaphore |
| golang.org/x/term | v0.39.0 | Terminal queries |
| golang.org/x/sys | v0.42.0 | OS primitives |
| pgregory.net/rapid | v1.2.0 | Property-based testing |
| gopkg.in/yaml.v3 | v3.0.1 | YAML parsing (config, settings) |

Indirect / transitive: bluemonday (HTML sanitizer — likely from glamour), goldmark (markdown — glamour), microcosm-cc, alecthomas/chroma (syntax highlighting — glamour), ultraviolet (charm internal), x/ansi (charm), x/exp/* (charm).

**Supply chain notes**:
- `Dicklesworthstone/toon-go` is from the original beads_viewer author (Jeffrey Emanuel). Pseudo-version, no SemVer. Worth flagging as a vendor risk for Supply Chain persona.
- `git.sr.ht/~sbinet/gg` is on sourcehut — alt-VCS host, not GitHub. Not necessarily a risk but unusual.
- `ajstarks/svgo` has no recent dated tag in this go.mod. Worth checking if maintained.
- `vendor/` directory exists — dependencies are vendored. Reduces supply-chain attack surface at install time but means upstream patches require explicit `go mod vendor`.

## Architectural import graph (high level)

```
cmd/bt
  ├─> pkg/ui (TUI launch from root)
  ├─> pkg/analysis (graph, triage, recommendations)
  ├─> pkg/correlation (history, diff)
  ├─> pkg/search (semantic search)
  ├─> pkg/bql (filter parsing)
  ├─> pkg/view (compact projections)
  ├─> pkg/export (static site, SQLite snapshot)
  ├─> pkg/loader (JSONL bead loading)
  ├─> internal/datasource (multi-source loader)
  ├─> internal/doltctl (Dolt server lifecycle)
  └─> pkg/agents, pkg/baseline, pkg/recipe, pkg/hooks, pkg/version, pkg/instance

pkg/ui
  ├─> pkg/analysis (results display)
  ├─> pkg/loader (data load)
  ├─> internal/datasource (multi-source)
  ├─> pkg/view (projections)
  ├─> pkg/search (search pane)
  ├─> pkg/bql (filter input)
  ├─> pkg/correlation (commit links)
  └─> pkg/watcher (live updates)

pkg/analysis
  ├─> pkg/model
  ├─> pkg/view (projections)
  ├─> pkg/correlation (commit-aware ranking)
  └─> gonum.org/v1/gonum/graph (algorithms)

internal/datasource
  ├─> pkg/loader (JSONL fallback)
  ├─> pkg/model
  ├─> github.com/go-sql-driver/mysql (Dolt)
  └─> modernc.org/sqlite (legacy SQLite reader)

pkg/correlation
  ├─> pkg/loader (git rev-parse helpers)
  ├─> pkg/model
  └─> exec.Command("git", ...) — direct shell-out
```

## Call graph — high-leverage entry points

| Entry | Eventually calls | Risk surface |
|---|---|---|
| `bt robot triage` | analysis.Triage → analysis.Phase1 + Phase2 (timeout 500ms) → recipe.Apply → view.Compact | Phase2 timeout swallows partial results — fall-through behavior worth verifying |
| `bt` (TUI) | ui.NewModel → tea.NewProgram → loops on Update + View | Concurrency: watcher goroutine, analysis goroutine, render loop |
| `bt robot bql` | bql.Parse → bql.Evaluate → datasource.Load | BQL parser is adapted from perles (MIT) — ownership/maint clarity? |
| `bt export` | export.WriteStatic → preview.Server (livereload) | livereload binds local HTTP server (default port?) — verify scope |
| `bt pages` | pages.Wizard → exec wasm-pack → upload | External binary required; fallback path if wasm-pack absent? |
| `bd dolt start` shell-out | doltctl.Start → exec.CommandContext("bd", "dolt", "start") | bd binary lookup, PATH dependency |

## Data flows

| Source | Transform | Sink | Risk |
|---|---|---|---|
| `bd dolt` MySQL protocol | go-sql-driver/mysql → datasource.Dolt → model.Issue | analysis, ui, export | Connection pool config, query timeouts not visible from grep |
| Filesystem `.beads/<project>/` | doltctl.Start → spawn dolt server → connect on ephemeral port | runtime queries | PID/port discovery: stdout-regex (`internal/doltctl/doltctl.go:39`) — fragile if bd output changes |
| JSONL `.beads/*.jsonl` | loader.LoadIssuesFromFile | model.Issue | Legacy path; bead memory says no longer system of record |
| SQLite `.bt/*.db` | sqlite reader | model.Issue | Memory says SQLite is export-only — runtime path is therefore EITHER stale OR memory is wrong |
| `git log` / `git show` | exec.Command + parse stdout | correlation.Witness | Stdout parsing — locale/format brittleness; verify `--no-pager`, `--end-of-options` usage |
| Browser open | `BT_NO_BROWSER` / `BT_TEST_MODE` gate (per CLAUDE.md) | xdg-open / start / open | Verify gate is universal, not just on some paths |
| Self-update | pkg/updater fetches from GitHub releases | install path | Signature verification? checksum? Worth verifying |

## Cross-package surfaces of interest for personas

| Surface | Files | Why interesting |
|---|---|---|
| Dolt connection lifecycle | internal/doltctl/doltctl.go, cmd/bt/root.go:1009 | Race conditions on start/stop, port collision, child process orphaning |
| Two-phase analysis timeout | pkg/analysis/graph.go (14 sync primitives) | Phase 2 has a 500ms timeout — what's the partial-result API contract? |
| BQL parser | pkg/bql/* | Adapted from perles. Trust boundary: BQL queries can come from CLI or robot mode |
| Robot output shapes | pkg/view/*, cmd/bt/robot_*.go | The bt-mhwy.1 unification produced `CompactIssue`. Verify single-source-of-truth claim |
| Git stdout parsing | pkg/loader/git.go, pkg/correlation/*.go | Brittleness; no apparent abstraction over `git` versions |
| AGENTS.md filename hardcode | pkg/agents/* (15 Go files per CLAUDE.md) | Renaming ergonomics — costly to change |
| Browser-opening | gated by `BT_NO_BROWSER` / `BT_TEST_MODE` | Must verify ALL invocations are gated |
| Vendor directory | vendor/ | All deps vendored; affects upgrade ergonomics |
