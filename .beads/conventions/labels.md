# Label Taxonomy

Use structured labels to categorize work. **Do not invent new labels** without
updating this file.

## Area

Exactly one `area:*` label is required per issue. Area labels describe ownership
of the change.

| Label | Use for |
|---|---|
| `area:cli` | cmd/bt/ - entry point, flags, cobra commands |
| `area:tui` | pkg/ui/ - Bubble Tea model, views, styling, key handlers |
| `area:bql` | pkg/bql/ - query language parser, executor, SQL builder |
| `area:analysis` | pkg/analysis/ - graph metrics, triage, planning |
| `area:data` | internal/datasource/, internal/dolt/, pkg/loader/ - data loading |
| `area:search` | pkg/search/ - FTS5 hybrid search, ranking |
| `area:export` | pkg/export/ - static site, HTML bundle, browser open |
| `area:correlation` | pkg/correlation/ - bead-to-commit correlation |
| `area:infra` | pkg/updater, watcher, hooks, drift, agents, cass, version, instance |
| `area:wasm` | bv-graph-wasm/ - Rust WASM graph visualization |
| `area:docs` | docs/, README, ADRs, AGENTS.md, CHANGELOG |
| `area:tests` | tests/e2e/, test infrastructure, coverage |

## Platform

| Label | Use for |
|---|---|
| `platform:windows` | Windows-specific behavior |
| `platform:macos` | macOS-specific behavior |
| `platform:linux` | Linux-specific behavior |

## Concern (cross-cutting)

| Label | Use for |
|---|---|
| `performance` | Speed, memory, optimization |
| `security` | Auth, vulnerabilities, input sanitization |
| `ux` | User experience, error messages, output quality |
| `tests` | Test failures, coverage gaps |
| `accessibility` | Accessibility improvements |

## Workflow

| Label | Use for |
|---|---|
| `workflow:investigate` | Unknown root cause; must close with root cause |
| `workflow:brainstorm` | Ideas, exploration, not yet implementation |
| `workflow:collaborative` | Needs human input or multi-agent coordination |

## Rules

- Labels must be assigned at creation time.
- Area labels describe ownership of the change, not symptoms.
- `workflow:investigate` must close with root cause + resolution or follow-up.
- Combine for specificity: `area:tui,platform:windows,ux`

## Examples

```bash
# Bug in the TUI on Windows
bd create --title="Status bar renders broken on Windows Terminal" \
  --type=bug --priority=2 --labels="area:tui,platform:windows,ux" \
  --description="Status bar JSON dump when Dolt disconnects"

# Security fix in BQL
bd create --title="Parameterize dateToSQL before DoltExecutor activation" \
  --type=bug --priority=2 --labels="area:bql,security" \
  --description="fmt.Sprintf interpolation in sql.go:219-229 is a SQL injection vector"

# Performance investigation in graph analysis
bd create --title="PageRank computation slow on large graphs" \
  --type=task --priority=3 --labels="area:analysis,performance,workflow:investigate" \
  --description="500+ node graphs take >2s for Phase 2 metrics"

# Documentation update
bd create --title="Rewrite README prose for public release" \
  --type=task --priority=2 --labels="area:docs" \
  --description="README still has Jeffrey-era prose, needs full rewrite"
```
