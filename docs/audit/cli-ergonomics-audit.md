# CLI Ergonomics Audit (bt-pfic)

**Date**: 2026-04-03
**Scope**: Full CLI surface of the `bt` binary - flags, error messages, robot-mode output, agent-friendliness
**Binary**: `bt` (beadstui)
**Entry point**: `cmd/bt/main.go`

---

## 1. CLI Surface Inventory

### Flag Count

**97 flags** defined via `pflag` in `cmd/bt/main.go:46-209`. No subcommands (flat flag namespace, not cobra).

### Flag Categories

#### Core / Meta (6 flags)
| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--help` | | bool | false | Show help |
| `--version` | | bool | false | Show version |
| `--cpu-profile` | | string | "" | Write CPU profile to file |
| `--format` | | string | "" | Output format: json or toon |
| `--stats` | | bool | false | TOON token estimates on stderr |
| `--robot-help` | | bool | false | Show AI agent help |

#### Robot-Mode Commands (30+ flags)
| Flag | Type | Needs Issue Data | Description |
|------|------|-----------------|-------------|
| `--robot-triage` | bool | yes | Unified triage mega-command |
| `--robot-next` | bool | yes | Single top pick |
| `--robot-plan` | bool | yes | Execution plan |
| `--robot-insights` | bool | yes | Graph metrics |
| `--robot-priority` | bool | yes | Priority recommendations |
| `--robot-triage-by-track` | bool | yes | Triage grouped by track |
| `--robot-triage-by-label` | bool | yes | Triage grouped by label |
| `--robot-diff` | bool | yes | Diff as JSON (requires `--diff-since`) |
| `--robot-recipes` | bool | no | List recipes as JSON |
| `--robot-label-health` | bool | yes | Per-label health |
| `--robot-label-flow` | bool | yes | Cross-label dependency flow |
| `--robot-label-attention` | bool | yes | Attention-ranked labels |
| `--robot-alerts` | bool | yes | Drift + proactive alerts |
| `--robot-metrics` | bool | yes | Perf metrics |
| `--robot-schema` | bool | no | JSON Schema for all outputs |
| `--robot-docs` | string | no | Machine-readable docs by topic |
| `--robot-suggest` | bool | yes | Smart suggestions |
| `--robot-graph` | bool | yes | Dependency graph export |
| `--robot-search` | bool | yes | Semantic search (requires `--search`) |
| `--robot-bql` | bool | yes | BQL-filtered issues (requires `--bql`) |
| `--robot-drift` | bool | yes | Drift check as JSON |
| `--robot-history` | bool | yes | Bead-to-commit correlations |
| `--robot-orphans` | bool | yes | Orphan commit candidates |
| `--robot-file-beads` | string | yes | Beads that touched a file |
| `--robot-file-hotspots` | bool | yes | Files touched by most beads |
| `--robot-file-relations` | string | yes | Co-change analysis |
| `--robot-related` | string | yes | Related beads |
| `--robot-blocker-chain` | string | yes | Blocker chain for issue |
| `--robot-impact-network` | string | yes | Impact network graph |
| `--robot-causality` | string | yes | Causal chain analysis |
| `--robot-impact` | string | yes | File impact analysis |
| `--robot-sprint-list` | bool | yes | List sprints |
| `--robot-sprint-show` | string | yes | Sprint details |
| `--robot-forecast` | string | yes | ETA forecast |
| `--robot-capacity` | bool | yes | Capacity simulation |
| `--robot-burndown` | string | yes | Sprint burndown |
| `--robot-correlation-stats` | bool | yes | Correlation feedback stats |
| `--robot-explain-correlation` | string | yes | Explain commit-bead link |
| `--robot-confirm-correlation` | string | yes | Confirm a correlation |
| `--robot-reject-correlation` | string | yes | Reject a correlation |

#### Robot-Mode Filters (8 flags)
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--robot-min-confidence` | float | 0.0 | Min confidence filter |
| `--robot-max-results` | int | 0 | Limit count |
| `--robot-by-label` | string | "" | Filter by label |
| `--robot-by-assignee` | string | "" | Filter by assignee |
| `--label` | string | "" | Scope to label subgraph |
| `--severity` | string | "" | Alert severity filter |
| `--alert-type` | string | "" | Alert type filter |
| `--alert-label` | string | "" | Alert label filter |

#### Search Flags (6 flags)
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--search` | string | "" | Semantic search query |
| `--search-limit` | int | 10 | Max results |
| `--search-mode` | string | "" | text or hybrid |
| `--search-preset` | string | "" | Hybrid preset name |
| `--search-weights` | string | "" | Hybrid weights JSON |
| `--bql` | string | "" | BQL query |

#### History/Diff Flags (8 flags)
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--diff-since` | string | "" | Historical reference point |
| `--as-of` | string | "" | Point-in-time view |
| `--bead-history` | string | "" | History for specific bead |
| `--history-since` | string | "" | Limit to recent commits |
| `--history-limit` | int | 500 | Max commits |
| `--min-confidence` | float | 0.0 | Correlation confidence filter |
| `--correlation-by` | string | "" | Feedback agent identifier |
| `--correlation-reason` | string | "" | Feedback reason |

#### Graph Export Flags (7 flags)
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--graph-format` | string | "json" | json, dot, mermaid |
| `--graph-root` | string | "" | Subgraph root issue |
| `--graph-depth` | int | 0 | Subgraph depth |
| `--export-graph` | string | "" | Export as image/HTML |
| `--graph-preset` | string | "compact" | Layout preset |
| `--graph-title` | string | "" | Graph title |
| `--robot-graph` | bool | false | (listed in robot section) |

#### Pages/Export Flags (9 flags)
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--export-md` | string | "" | Export markdown report |
| `--export-pages` | string | "" | Export static site |
| `--preview-pages` | string | "" | Preview static site |
| `--pages` | bool | false | Pages deployment wizard |
| `--pages-title` | string | "" | Custom site title |
| `--pages-include-closed` | bool | true | Include closed issues |
| `--pages-include-history` | bool | true | Include git history |
| `--no-live-reload` | bool | false | Disable live-reload |
| `--watch-export` | bool | false | Watch mode for export |

#### Update/Maintenance Flags (7 flags)
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--update` | bool | false | Update bt |
| `--check-update` | bool | false | Check for update |
| `--rollback` | bool | false | Rollback to previous |
| `--yes` | bool | false | Skip prompts |
| `--no-hooks` | bool | false | Skip hooks |
| `--force-full-analysis` | bool | false | Force all metrics |
| `--profile-startup` | bool | false | Startup timing |

#### Baseline/Drift Flags (4 flags)
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--save-baseline` | string | "" | Save baseline |
| `--baseline-info` | bool | false | Show baseline |
| `--check-drift` | bool | false | Check drift |
| `--robot-drift` | bool | false | Drift as JSON |

#### AGENTS.md Management (6 flags)
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--agents-add` | bool | false | Add blurb |
| `--agents-remove` | bool | false | Remove blurb |
| `--agents-update` | bool | false | Update blurb |
| `--agents-check` | bool | false | Check status |
| `--agents-dry-run` | bool | false | Dry run |
| `--agents-force` | bool | false | Skip prompts |

#### Remaining Flags
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--recipe` / `-r` | string | "" | Apply named recipe |
| `--workspace` | string | "" | Workspace config file |
| `--repo` | string | "" | Filter by repo prefix |
| `--background-mode` | bool | false | Background snapshot loading |
| `--no-background-mode` | bool | false | Disable background mode |
| `--debug-render` | string | "" | Render view to file |
| `--debug-width` | int | 180 | Debug render width |
| `--debug-height` | int | 50 | Debug render height |
| `--profile-json` | bool | false | Profile in JSON |
| `--emit-script` | bool | false | Emit shell script |
| `--script-limit` | int | 5 | Script item count |
| `--script-format` | string | "bash" | Script format |
| `--feedback-accept` | string | "" | Accept feedback |
| `--feedback-ignore` | string | "" | Ignore feedback |
| `--feedback-reset` | bool | false | Reset feedback |
| `--feedback-show` | bool | false | Show feedback |
| `--priority-brief` | string | "" | Export priority brief |
| `--agent-brief` | string | "" | Export agent brief bundle |
| `--suggest-type` | string | "" | Suggestion type filter |
| `--suggest-confidence` | float | 0.0 | Suggestion confidence |
| `--suggest-bead` | string | "" | Suggestion bead filter |
| `--attention-limit` | int | 5 | Label attention limit |
| `--orphans-min-score` | int | 30 | Orphan suspicion score |
| `--robot-file-beads` | string | "" | File-bead index |
| `--file-beads-limit` | int | 20 | File-bead result limit |
| `--robot-file-hotspots` | bool | false | File hotspots |
| `--hotspots-limit` | int | 10 | Hotspot limit |
| `--relations-threshold` | float | 0.5 | Co-change threshold |
| `--relations-limit` | int | 10 | Related files limit |
| `--related-min-relevance` | int | 20 | Related work relevance |
| `--related-max-results` | int | 10 | Related work max |
| `--related-include-closed` | bool | false | Include closed |
| `--network-depth` | int | 2 | Impact network depth |
| `--forecast-agents` | int | 1 | Forecast agent count |
| `--forecast-label` | string | "" | Forecast label |
| `--forecast-sprint` | string | "" | Forecast sprint |
| `--agents` | int | 1 | Capacity agents |
| `--capacity-label` | string | "" | Capacity label |
| `--schema-command` | string | "" | Specific schema |

### Short Flags

Only **one** short flag exists: `-r` for `--recipe`. No other shortcuts defined.

### Environment Variables

Documented in `--robot-docs env`:
- `BT_OUTPUT_FORMAT` - output format
- `TOON_DEFAULT_FORMAT` - fallback format
- `TOON_STATS` - show token estimates
- `TOON_KEY_FOLDING` - TOON key folding mode
- `TOON_INDENT` - TOON indent level
- `BT_PRETTY_JSON` - indented JSON
- `BT_ROBOT` - force robot mode
- `BT_SEARCH_MODE` - search mode
- `BT_SEARCH_PRESET` - hybrid preset

**Undocumented** in `--robot-docs env` but used in code:
- `BT_DOLT_PORT` - Dolt port override (`internal/datasource/metadata.go:91`)
- `BEADS_DOLT_SERVER_PORT` - upstream beads Dolt port (`internal/datasource/metadata.go:97`)
- `BEADS_DIR` - beads directory override (`internal/datasource/source.go:106`)
- `BT_BACKGROUND_MODE` - background snapshot loading (`pkg/ui/model.go:829`)
- `BT_TUI_AUTOCLOSE_MS` - auto-quit for testing (`cmd/bt/main.go:1485`)
- `BT_INSIGHTS_MAP_LIMIT` - cap insight maps (`cmd/bt/robot_analysis.go:96`)
- `BT_METRICS` - enable/disable metrics (`pkg/metrics/timing.go:27`)
- `BT_DEBUG` - debug logging (`pkg/debug/debug.go:36`)
- `BT_WORKER_LOG_LEVEL` - worker log level (`pkg/ui/background_worker.go:308`)
- `BT_WORKER_TRACE` - worker trace file (`pkg/ui/background_worker.go:310`)
- `BT_NO_BROWSER` - suppress browser open (`pkg/export/github.go:598`)
- `BT_TEST_MODE` - test mode (`pkg/agents/tty_guard.go:22`)
- `BT_CACHE_DIR` - analysis cache dir (`pkg/analysis/cache.go:721`)
- `BT_BUILD_HYBRID_WASM` - WASM build flag (`cmd/bt/pages.go:685`)
- `BT_NO_SAVED_CONFIG` - skip saved config (`pkg/export/wizard.go:208`)
- `BT_DOLT_POLL_INTERVAL_S` - poll interval (per MEMORY.md)
- `BT_FRESHNESS_STALE_S` - stale threshold (per MEMORY.md)
- `BT_FRESHNESS_WARN_S` - warn threshold (per MEMORY.md)

---

## 2. Issues Found

### Critical (breaks agent workflows)

#### C1: `--robot-search` output skips standard envelope

**File**: `cmd/bt/search_output.go:22-38`

`robotSearchOutput` builds its own `generated_at` and `data_hash` fields but does NOT include `output_format` or `version` - fields present on `RobotEnvelope` and documented as part of the standard robot output contract.

Compare to `robot_alerts.go:109` and `robot_sprint.go:45` which embed `RobotEnvelope` properly.

An agent parsing robot output expecting `version` and `output_format` on every response will get nulls from `--robot-search`.

#### C2: `--robot-search` ignores `--format toon`

**File**: `cmd/bt/search_output.go:40-44`

`writeRobotSearchOutput` creates its own `json.NewEncoder` directly, bypassing the `newRobotEncoder()` dispatch that handles TOON format. When an agent sets `BT_OUTPUT_FORMAT=toon` globally and calls `--robot-search`, it gets JSON back instead of TOON, breaking format expectations.

#### C3: No positional arg handling at all

**File**: `cmd/bt/main.go` (entire file)

No call to `flag.Args()` or `flag.NArg()` anywhere in the codebase. Positional arguments are silently ignored. Running `bt some-random-text` just launches the TUI as if no args were given.

An agent that types `bt robot-triage` (forgetting the `--`) gets a blocking TUI instead of an error. There is no "did you mean" recovery.

### High (confusing for agents, workaround exists)

#### H1: Paired flag pattern creates redundancy and confusion

**Files**: `cmd/bt/main.go:102-105`, `cmd/bt/main.go:570-572`, `cmd/bt/main.go:647-649`

Three commands require paired flags:
- `--robot-search` requires `--search "query"` - the query goes on `--search`, not `--robot-search`
- `--robot-bql` requires `--bql "query"` - same pattern
- `--robot-diff` requires `--diff-since <ref>` - same pattern

The `--search` flag without `--robot-search` produces human-readable output. The separation exists to support human vs robot output modes. But agents will naturally try `--robot-search "query"` which silently ignores the positional arg and produces the error:

```
Error: --robot-search requires --search "query"
```

This is the "cryptic error" noted in the issue. The error message itself is actually not terrible - it tells you what to do. The problem is that the invocation pattern is surprising. Every other `--robot-*` command that takes a value uses the flag value directly (e.g., `--robot-forecast bd-123`, `--robot-blocker-chain bd-123`).

**Recommendation**: Accept `--robot-search "query"` as equivalent to `--robot-search --search "query"`. Same for `--robot-bql` and `--robot-diff`. The bool flag becomes a string flag; empty string means "not requested". Backward-compatible since `--robot-search --search "query"` still works.

#### H2: Duplicate confidence flags with different semantics

**File**: `cmd/bt/main.go:80,92,126`

Three separate confidence flags:
- `--suggest-confidence` (0.0-1.0) - for `--robot-suggest`
- `--robot-min-confidence` (0.0-1.0) - for robot output filtering
- `--min-confidence` (0.0-1.0) - for `--robot-history` correlations

An agent choosing the wrong one gets silently wrong results (default 0.0 lets everything through). The names are close enough to confuse.

#### H3: Duplicate agent-count flags

**File**: `cmd/bt/main.go:167,170`

- `--forecast-agents` (default 1) - for `--robot-forecast`
- `--agents` (default 1) - for `--robot-capacity`

Two flags that mean "number of parallel agents" with different names. The generic `--agents` should work for both, or both should use a consistent prefix.

#### H4: 18 env vars undocumented in `--robot-docs env`

**File**: `cmd/bt/robot_graph.go:218-228`

The `--robot-docs env` topic lists 9 env vars. At least 18 additional `BT_*` and `BEADS_*` env vars are used in the codebase but not surfaced in the docs. Agents relying on `--robot-docs env` as the canonical source will miss configuration options like `BT_DOLT_PORT`, `BT_INSIGHTS_MAP_LIMIT`, `BT_CACHE_DIR`, and `BT_BACKGROUND_MODE`.

### Medium (cosmetic or low-frequency friction)

#### M1: Stale `bv` / `br` references in help text

**File**: `cmd/bt/robot_help.go`

Several stale references remain from the bv->bt rename:

- Line 79, 225-226: `br show commands`, `br update commands` - should be `bd show`, `bd update`
- Lines 164-165: Example bead IDs use `bv-abc1` - should use `bt-abc1` or `bd-abc1`
- Line 201: Example uses `bv-123` - should be `bt-123` or `bd-123`
- Line 275: Recipe path `~/.config/bv/recipes.yaml` - should be `~/.config/bt/recipes.yaml`
- Lines 379, 385, 441: Internal issue refs `(bv-84)`, `(bv-122)`, `(bv-7pu)` leaked into user-facing help text
- Lines 451, 457: Example paths use `./bv-pages` - should be `./bt-pages` or a generic name

**File**: `SKILL.md`
- Lines 102-103: `bv --recipe` should be `bt --recipe`
- Line 139: `bv --bead bd-123` should be `bt --bead bd-123` (note: `--bead` flag doesn't exist)
- Lines 244-246: `bv --as-of` should be `bt --as-of`

#### M2: `--help` output is 130+ lines of unsorted flags

**File**: `cmd/bt/main.go:211-215,293-297`

The `--help` output dumps all 97 flags in alphabetical order via `flag.PrintDefaults()`. There is no grouping, no section headers, no usage examples. A human scrolling through 130 lines of flags cannot find what they need. An agent gets no signal about which flags are primary vs modifier.

The `--robot-help` output is much better (grouped, with examples), but it's only available as a separate flag. The standard `--help` should at minimum show a grouped summary or point to `--robot-help`.

#### M3: Label commands don't embed `RobotEnvelope`

**File**: `cmd/bt/robot_labels.go:16-33`

`runLabelHealth`, `runLabelFlow`, and `runLabelAttention` build output structs with manual `GeneratedAt` and `DataHash` fields instead of embedding `RobotEnvelope`. They lack `output_format` and `version` fields. Same issue as C1 but for label commands.

#### M4: Exit code semantics are inconsistent

**Files**: `cmd/bt/main.go` (grep for `os.Exit`)

Documented exit codes (from `--robot-docs exit-codes`):
- 0 = Success
- 1 = Error (general failure, drift critical)
- 2 = Invalid arguments or drift warning

Actual behavior:
- Exit 2 for `--format blah` (correct: invalid args)
- Exit 2 for `--robot-docs invalid` (correct: invalid args)
- Exit 1 for `--robot-search` without `--search` (should be 2: invalid args)
- Exit 1 for `--robot-bql` without `--bql` (should be 2: invalid args)
- Exit 1 for unknown recipe (should be 2: invalid args)
- Exit 1 for `--schema-command` with unknown command (should be 2: invalid args)
- pflag's own unknown-flag error uses exit 2 (correct)

The "invalid arguments" case is only exit 2 in some places. Most missing-required-companion-flag errors use exit 1, which is indistinguishable from "something crashed during analysis."

#### M5: No short flags for common robot commands

**File**: `cmd/bt/main.go` (flag definitions)

Only `-r` / `--recipe` has a short form. The most common agent commands have no shortcuts:
- `--robot-triage` could be `--rt` or `-T`
- `--robot-next` could be `-N`
- `--robot-search` could be `-S`

Minor for agents (they spell things out) but meaningful for human-in-the-loop workflows where an operator pipes bt output to jq.

#### M6: `--robot-bql` output bypasses `newRobotEncoder`

**File**: `cmd/bt/main.go:591-598`

`--robot-bql` creates its own `json.NewEncoder(os.Stdout)` and always pretty-prints. It ignores both `--format toon` and the compact-JSON default. Same class of issue as C2.

#### M7: `--background-mode` / `--no-background-mode` error uses exit 2

**File**: `cmd/bt/main.go:1389-1391`

Mutually exclusive flags exit with code 2. This is correct for "invalid args" semantics but happens late - after issue loading, recipe validation, and robot-mode checks. Should be caught in the flag validation phase before data loading begins.

### Low (polish items)

#### L1: Internal issue IDs in flag help text

**File**: `cmd/bt/main.go:63,64`

Flag descriptions contain raw internal issue IDs visible to users:
- `--robot-triage-by-track`: `"Group triage recommendations by execution track (bv-87)"`
- `--robot-triage-by-label`: `"Group triage recommendations by label (bv-87)"`

These are internal tracker references that mean nothing to end users.

#### L2: `--export-pages` example path still says `./bv-pages`

**File**: `cmd/bt/main.go:188`

The flag description: `"Export static site to directory (e.g., ./bv-pages)"` - should be `./pages` or `./bt-pages`.

#### L3: `--pages-include-closed` default discoverability

**File**: `cmd/bt/main.go:190`

Default is `true`. To turn it off you need `--pages-include-closed=false`. pflag booleans can be confusing here - `--pages-include-closed` with no value means true (already the default), and `--no-pages-include-closed` isn't defined. The flag name should be `--exclude-closed` or there should be a `--no-pages-include-closed` alias.

#### L4: `--robot-docs` returns JSON even for errors

**File**: `cmd/bt/robot_graph.go:253-256`

When you pass an invalid topic, the output is a JSON object with an `error` key, followed by exit code 2. This is actually good behavior for agents (parseable error). But it's inconsistent with other errors which write plain text to stderr and exit 1.

---

## 3. Flag Composition Analysis

### What works well

- `--label` scopes multiple robot commands: `--robot-insights`, `--robot-plan`, `--robot-priority`. Clean cross-cutting filter.
- `--as-of` works with all robot commands for historical analysis. Metadata fields `as_of` and `as_of_commit` are added to output.
- `--recipe` composes with robot commands to pre-filter issues before analysis.
- `--format` globally controls output encoding for most robot commands.
- Robot filter flags (`--robot-min-confidence`, `--robot-max-results`, `--robot-by-label`, `--robot-by-assignee`) apply across multiple commands.
- `--force-full-analysis` works with any command that runs graph analysis.

### What doesn't compose

- `--robot-search` / `--robot-bql` / `--robot-diff` each require their own companion flag instead of accepting the value directly.
- `--workspace` and `--as-of` are explicitly incompatible (warned at runtime, not at flag level).
- Alert filters (`--severity`, `--alert-type`, `--alert-label`) only apply to `--robot-alerts` - not documented as such in the flag description.
- Suggestion filters (`--suggest-type`, `--suggest-confidence`, `--suggest-bead`) only apply to `--robot-suggest`.
- Three separate confidence filters are not interchangeable (H2).

---

## 4. Error Message Quality

### Good patterns

```
Error: Unknown recipe 'nonexistent'

Available recipes:
  actionable      Issues ready to work on (no open blockers)
  ...
```
This is excellent - states the problem, shows valid options. Recipe validation at `cmd/bt/main.go:443-451`.

```
Error: --robot-search requires --search "query"
```
Clear about what's needed, though the pattern itself is the problem (H1).

```
Unknown command: badname
Available commands:
  robot-triage
  ...
```
Schema command validation at `cmd/bt/main.go:389-394` lists alternatives. Good.

### Problematic patterns

```
Error loading beads: <error>
Make sure you are in a project initialized with 'bd init'.
```
(`cmd/bt/main.go:543-544`) - The hint is correct but the first line varies wildly. When Dolt isn't running, the cascade is: detect ErrDoltRequired -> try EnsureServer -> if that fails, print a different error about server startup. The error chain is good but an agent needs to know "is this a missing-project error or a Dolt-connectivity error?" to branch correctly. Both exit with code 1.

```
Invalid --format "blah" (expected json|toon)
```
Good - states invalid value and valid options.

pflag's unknown flag error:
```
unknown flag: --nonexistent
```
Followed by the full 130-line help dump. The error message itself is fine but gets buried under the help text. An agent parsing stderr would need to handle the noise.

---

## 5. Help Text Assessment

### `--help`

Prints `Usage: bt [options]` plus a one-liner, then dumps all 97 flags alphabetically via pflag's default formatter. No grouping, no examples, no "start here" guidance.

**Rating**: Poor for humans. Marginal for agents (they can grep for flag names, but there's no semantic structure).

### `--robot-help`

Comprehensive, well-organized, with examples and jq snippets. Covers all robot commands, explains output structure, includes usage patterns.

**Rating**: Good for agents. 474 lines is a lot of context but it's well-structured.

**Gap**: No `--robot-help` equivalent for human-oriented features (TUI keybinds, export workflows, pages wizard). A human running `bt --help` gets no guidance on TUI shortcuts or export workflows.

### `--robot-docs`

Machine-readable JSON documentation with topics: guide, commands, examples, env, exit-codes, all. Excellent for agent bootstrapping.

**Rating**: Excellent design. Minor issues with incomplete env var coverage (H4).

### `--robot-schema`

JSON Schema definitions for all robot command outputs. Allows agents to validate responses.

**Rating**: Excellent - this is rare for CLI tools and genuinely useful for agent integration.

---

## 6. Exit Code Analysis

### Documented (via `--robot-docs exit-codes`)

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Error (general failure, drift critical) |
| 2 | Invalid arguments or drift warning |

### Actual usage patterns

| Situation | Actual Code | Expected Code | Correct? |
|-----------|------------|---------------|----------|
| Success | 0 | 0 | Yes |
| Unknown pflag flag | 2 | 2 | Yes |
| Invalid `--format` value | 2 | 2 | Yes |
| Invalid `--robot-docs` topic | 2 | 2 | Yes |
| Missing `--search` for `--robot-search` | 1 | 2 | **No** |
| Missing `--bql` for `--robot-bql` | 1 | 2 | **No** |
| Unknown recipe | 1 | 2 | **No** |
| Unknown `--schema-command` | 1 | 2 | **No** |
| JSON encoding error | 1 | 1 | Yes |
| Data load failure | 1 | 1 | Yes |
| Drift: critical | 1 | 1 | Yes |
| Drift: warning | 2 | 2 | Yes |
| Mutually exclusive flags | 2 | 2 | Yes |

**Verdict**: Exit code 2 for "invalid arguments" is only applied in ~4 places. Most argument validation errors use exit 1, making it impossible for agents to distinguish "bad arguments" from "runtime error." An agent that retries on exit 1 (thinking it's a transient error) will loop forever on a bad flag combination.

---

## 7. Robot Mode Output Consistency

### Envelope compliance

The `RobotEnvelope` struct defines the standard fields:
```go
type RobotEnvelope struct {
    GeneratedAt  string `json:"generated_at"`
    DataHash     string `json:"data_hash"`
    OutputFormat string `json:"output_format,omitempty"`
    Version      string `json:"version,omitempty"`
}
```

| Command | Uses RobotEnvelope? | Has output_format? | Has version? |
|---------|--------------------|--------------------|-------------|
| `--robot-triage` | Yes | Yes | Yes |
| `--robot-alerts` | Yes | Yes | Yes |
| `--robot-sprint-*` | Yes | Yes | Yes |
| `--robot-history` | Yes | Yes | Yes |
| `--robot-burndown` | Yes | Yes | Yes |
| `--robot-forecast` | Yes | Yes | Yes |
| `--robot-capacity` | Yes | Yes | Yes |
| `--robot-search` | **No** | **No** | **No** |
| `--robot-label-health` | **No** | **No** | **No** |
| `--robot-label-flow` | **No** | **No** | **No** |
| `--robot-label-attention` | **No** | **No** | **No** |
| `--robot-bql` | **No** | **No** | **No** |
| `--robot-docs` | Custom | Yes | Yes |
| `--robot-schema` | Custom | N/A | N/A |

5 robot commands bypass the standard envelope. 3 of those (label commands) have `generated_at` and `data_hash` but lack `output_format` and `version`. 2 (`--robot-search`, `--robot-bql`) have even less.

### Format dispatch compliance

| Command | Uses newRobotEncoder? | Respects --format toon? |
|---------|----------------------|------------------------|
| Most robot commands | Yes | Yes |
| `--robot-search` | **No** (uses `json.NewEncoder` directly) | **No** |
| `--robot-bql` | **No** (uses `json.NewEncoder` directly) | **No** |
| `--robot-recipes` | Yes | Yes |

---

## 8. Agent-Friendliness Score

### Scoring criteria (1-5 scale)

| Category | Score | Notes |
|----------|-------|-------|
| **Discoverability** | 4/5 | `--robot-docs all` and `--robot-schema` are excellent. `--help` is poor but agents should use `--robot-docs`. |
| **Consistency** | 3/5 | Most robot outputs share a common structure, but 5 commands skip the envelope and 2 skip format dispatch. |
| **Error handling** | 2/5 | Error messages are generally clear, but exit codes don't distinguish "bad args" from "runtime error." |
| **Composability** | 4/5 | `--label`, `--recipe`, `--as-of`, `--format` compose well. The paired-flag pattern (H1) is the main gap. |
| **Parsability** | 4/5 | JSON output is well-structured. `data_hash` enables caching. `status` field enables metric readiness checks. |

**Overall: 3.4/5** - Good foundation, needs cleanup on consistency and exit codes.

---

## 9. Recommendations (Prioritized)

### Quick fixes (low effort, high impact)

1. **Fix exit codes for argument validation** (`cmd/bt/main.go:572,649` + `cmd/bt/cli_misc.go` recipe validation): Change `os.Exit(1)` to `os.Exit(2)` for missing companion flags and invalid arg values. ~10 lines changed.

2. **Embed `RobotEnvelope` in label commands** (`cmd/bt/robot_labels.go`): Replace manual `GeneratedAt`/`DataHash` with `RobotEnvelope` embed. ~15 lines changed.

3. **Route `--robot-search` through `newRobotEncoder`** (`cmd/bt/search_output.go:40-44`): Replace `json.NewEncoder(w)` with the robot encoder dispatch. Add `RobotEnvelope` fields to `robotSearchOutput`. ~20 lines changed.

4. **Route `--robot-bql` through `newRobotEncoder`** (`cmd/bt/main.go:592-597`): Same fix as above. ~5 lines changed.

5. **Remove internal issue IDs from user-facing text** (`cmd/bt/main.go:63-64`, `cmd/bt/robot_help.go` multiple lines): Strip `(bv-87)`, `(bv-84)`, `(bv-122)`, `(bv-7pu)` from flag descriptions and help text. ~10 lines changed.

6. **Fix stale `bv`/`br` references** (`cmd/bt/robot_help.go`, `SKILL.md`): Replace `br show` -> `bd show`, `bv-abc1` -> `bt-abc1`, `~/.config/bv/` -> `~/.config/bt/`, `bv --recipe` -> `bt --recipe`, etc. ~20 lines across 2 files.

### Medium effort improvements

7. **Warn on unexpected positional args**: After `flag.Parse()`, check `flag.NArg() > 0` and print a warning:
   ```
   Warning: unexpected arguments: [robot-triage]
   Did you mean: --robot-triage?
   ```
   ~10 lines in `cmd/bt/main.go` after line 216.

8. **Accept value on `--robot-search`**: Change from `flag.Bool("robot-search", ...)` to `flag.String("robot-search", ...)`. When non-empty, treat the value as the query (equivalent to `--search <value>`). Still accept `--robot-search --search "query"` for backward compat. Same for `--robot-bql` and `--robot-diff`. ~30 lines.

9. **Improve `--help` output**: Add a brief header with grouped sections before the full flag dump:
   ```
   Usage: bt [options]

   Quick start:
     bt                         Launch TUI viewer
     bt --robot-triage          Full triage for agents (JSON)
     bt --robot-next            Top pick for agents (JSON)
     bt --search "query"        Semantic search (human-readable)
     bt --robot-help            Full robot mode documentation

   All flags:
     ...
   ```
   ~15 lines in `cmd/bt/main.go:211-215`.

10. **Document missing env vars in `--robot-docs env`** (`cmd/bt/robot_graph.go:218-228`): Add the 18 undocumented env vars. Categorize as "Core", "Dolt", "Debug", "Internal". ~30 lines.

### Larger structural improvements (future)

11. **Consolidate confidence flags**: Introduce a single `--confidence` flag that applies contextually, or at minimum rename to avoid confusion: `--min-confidence` -> `--correlation-min-confidence`.

12. **Consider cobra migration**: With 97 flags, the flat pflag namespace is straining. Cobra subcommands would allow `bt robot triage`, `bt robot search "query"`, `bt export pages ./dir`, etc. This is a significant refactor but would solve the paired-flag pattern, improve help text grouping, and enable proper positional arg handling per subcommand.

---

## 10. Files Referenced

| File | Lines | Relevance |
|------|-------|-----------|
| `cmd/bt/main.go` | 46-216 | All flag definitions |
| `cmd/bt/main.go` | 233-276 | Robot mode detection |
| `cmd/bt/main.go` | 570-572, 647-649 | Paired flag validation |
| `cmd/bt/robot_output.go` | 21-38 | RobotEnvelope definition |
| `cmd/bt/robot_output.go` | 89-98 | Encoder dispatch (json/toon) |
| `cmd/bt/search_output.go` | 22-44 | Search output (skips envelope) |
| `cmd/bt/robot_help.go` | 1-474 | Robot help text (stale refs) |
| `cmd/bt/robot_graph.go` | 218-258 | robot-docs env + exit codes |
| `cmd/bt/robot_labels.go` | 16-33 | Label health (manual envelope) |
| `SKILL.md` | 102-103, 139, 244-246 | Stale `bv` references |
