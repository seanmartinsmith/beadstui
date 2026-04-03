# BQL Gap Analysis

**Date**: 2026-04-03
**Scope**: Full audit of BQL implementation vs planned features
**Sources**: pkg/bql/, pkg/ui/bql_modal.go, pkg/ui/model_filter.go, cmd/bt/main.go, perles brainstorm

---

## 1. Current Capability Inventory

### Parser (pkg/bql/)

The parser is a hand-rolled recursive descent parser vendored from zjrosen/perles (MIT). Zero external dependencies beyond stdlib.

**Files**: ast.go (132 LOC), token.go (170 LOC), lexer.go (192 LOC), parser.go (464 LOC), validator.go (226 LOC), sql.go (278 LOC), executor.go (25 LOC), memory_executor.go (523 LOC)

**Comparison operators**: `=`, `!=`, `<`, `>`, `<=`, `>=`, `~` (contains), `!~` (not contains)

**Set operators**: `IN (values)`, `NOT IN (values)`

**Boolean logic**: `AND`, `OR`, `NOT`, parentheses for grouping

**Value types**:
- Strings: unquoted identifiers, `"double quoted"`, `'single quoted'`
- Integers: bare numbers
- Booleans: `true`, `false`
- Priorities: `P0`-`P4` (case-insensitive)
- Dates: `today`, `yesterday`, `-7d`, `-24h`, `-3m`

**Clauses**:
- `ORDER BY field ASC|DESC [, field2 ...]` - multi-field sorting
- `EXPAND up|down|all [DEPTH n|*]` - dependency graph traversal with depth limits

**Validated fields** (16 total):
| Field | Type | Notes |
|-------|------|-------|
| id | String | |
| title | String | |
| description | String | |
| design | String | |
| notes | String | |
| status | Enum | No value validation (see Bugs) |
| priority | Priority | P0-P4 or 0-4 |
| type | Enum | Validated: bug, feature, task, epic, chore |
| assignee | String | |
| label | String | Array membership (special handling in executor) |
| source_repo | String | |
| blocked | Bool | Computed field - walks dependency graph |
| created_at | Date | |
| updated_at | Date | |
| due_date | Date | |
| closed_at | Date | |

### Memory Executor (pkg/bql/memory_executor.go)

Walks `model.Issue` structs in memory. Supports:

- All comparison operators with type-appropriate semantics
- Case-insensitive string equality and contains
- Label array membership (special path: `label = "bug"` checks if "bug" is in Labels slice)
- Blocked computed field (walks Dependencies against IssueMap)
- Date resolution: `today`, `yesterday`, relative offsets (`-7d`, `-24h`, `-3m`)
- ORDER BY: multi-field stable sort with generic field value comparison
- EXPAND: BFS dependency graph traversal with depth limits, reverse-dep index for "down" direction
- Short-circuit evaluation for AND/OR

### SQL Builder (pkg/bql/sql.go)

MySQL/Dolt dialect SQL generator. Not used at runtime (MemoryExecutor handles everything). Present for future DoltExecutor.

- Generates WHERE clause + ORDER BY from AST
- Parameterized queries (prevents SQL injection)
- Special handling for: label (subquery against labels table), blocked (subquery against dependencies), assignee (COALESCE for nulls), priority (int comparison), dates (CURDATE/DATE_SUB)
- Field-to-column mapping (e.g., `type` -> `i.issue_type`, `due_date` -> `i.due_at`)

### TUI Integration (pkg/ui/bql_modal.go + model_filter.go)

- `:` keybind opens BQL query modal from any view
- Modal: text input with placeholder, inline parse errors, session-scoped history (up/down arrows), 20-entry cap with deduplication
- Enter: parse + validate + apply via `applyBQL()` dedicated path
- Empty enter: clears BQL filter, resets to "all"
- Esc: cancels without applying
- Error display: parse/validation errors shown inline, modal stays open
- `applyBQL()` is a separate path from `applyFilter()`/`applyRecipe()`, handles set-level operations (ORDER BY, EXPAND)
- BQL results propagate to list, board, and graph views
- Footer badge shows "BQL: <query>" (truncated to 30 chars)
- Workspace pre-filter applied before BQL execution
- BQL re-applied after Dolt data refresh
- Status keys (`o`, `c`, `r`, `a`) clear activeBQLExpr before applying their filter

### CLI Integration (cmd/bt/main.go)

- `--bql "query"` pre-filters issues before passing to TUI or robot commands
- `--robot-bql` outputs BQL-filtered issues as JSON (requires `--bql`)
- BQL filter applies after repo filter, before recipe/label scope/analysis
- Composes with other robot commands: `--bql "status=open" --robot-triage`

### Test Coverage

- **lexer_test.go**: 3 test functions covering basic tokens, all operators, case-insensitive keywords
- **parser_test.go**: 15 test functions covering simple comparison, priority, boolean, IN/NOT IN, binary expr, NOT, parentheses, ORDER BY, dates, quoted strings, EXPAND, error cases, complex queries
- **validator_test.go**: 8 test functions covering valid queries, invalid field, invalid operators for bool/enum, invalid type value, invalid priority, IN on bool, custom fields
- **memory_executor_test.go**: 22 test functions covering status filter, priority, type, title contains/not-contains, label equals/not-equals/contains, assignee, blocked true/false, AND/OR/NOT, parentheses, IN/NOT IN, label IN, ORDER BY asc/desc, filter+order, empty filter, empty result, expand down/up, Matches() single issue, nil IssueMap safety

---

## 2. Gap Analysis

### 2A. Planned Features - Status

| Planned Feature | Status | Notes |
|----------------|--------|-------|
| CLI flag (--bql) | **SHIPPED** | Already implemented in cmd/bt/main.go:102 |
| Robot-mode BQL output (--robot-bql) | **SHIPPED** (partial) | Works but has quality gaps (see below) |
| Status key redirect through BQL | NOT STARTED | Status keys clear BQL then apply hardcoded filter |
| Syntax highlighting in modal | NOT STARTED | Skipped per plan; perles styles.go + syntax_adapter.go not vendored |
| Recipes as BQL | NOT STARTED | Recipe system is YAML-based, separate execution path |

### 2B. Detailed Gap Assessment

#### Gap 1: --robot-bql Output Quality

**Current**: Uses raw `json.NewEncoder` directly (main.go:592), bypassing the standard `robotEncoder` system that all other robot commands use.

**Missing**:
1. No `RobotEnvelope` wrapper (no `generated_at`, `data_hash`, `output_format`, `version` fields)
2. No TOON format support (`--format toon` is silently ignored for robot-bql)
3. No `--robot-min-confidence`, `--robot-max-results`, `--robot-by-label`, `--robot-by-assignee` filter integration
4. Outputs raw `[]model.Issue` instead of a structured response with query metadata

**Effort**: Low. 1 agent, 1 sequential step. Follow the pattern in robot_output.go.

**Dependencies**: None.

#### Gap 2: Status Key Redirect Through BQL

**Current**: Pressing `o`, `c`, `r`, `a` sets `m.currentFilter` to hardcoded strings ("open", "closed", "ready", "all") and explicitly clears `activeBQLExpr = nil` before calling `applyFilter()`. The filter logic in `matchesCurrentFilter()` uses custom Go code per status, not BQL.

**What "redirect through BQL" means**: Status keys would translate to BQL queries:
- `o` -> `status = open` (via BQL)
- `c` -> `status = closed` (via BQL - but note: bt's "closed" includes closed + tombstone, BQL `status = closed` is exact match)
- `r` -> `status = open and blocked = false` (or: `not blocked = true and status != closed`)
- `a` -> no filter (clear BQL)

**Challenges**:
1. bt's `matchesCurrentFilter("closed")` uses `isClosedLikeStatus()` which matches closed + tombstone. BQL's `status = closed` is an exact match. Would need either a `status ~ closed` convention or an `isClosed` computed field.
2. bt's `matchesCurrentFilter("ready")` includes custom logic: not closed-like, not blocked status, and walks dependencies. BQL's `blocked = false` covers the dependency walk, but the status check is different (currently excludes `StatusBlocked` enum value in addition to closed-like).
3. The `applyFilter()` path creates `IssueItem` structs with triage scores, graph scores, diff status, and quick-win flags. `applyBQL()` does the same. But `applyFilter()` also respects the sort mode (`m.sortMode`). BQL's ORDER BY would override this, which is correct for explicit BQL queries but might surprise users who expect `o` to preserve their sort preference.

**Effort**: Medium. 1 agent, 2 sequential steps (implement mapping + handle the closed/ready semantic gaps).

**Dependencies**: Needs a decision on how "closed" and "ready" map to BQL (exact status match vs. semantic grouping).

#### Gap 3: Syntax Highlighting in Modal

**Current**: The BQL modal uses a plain `textinput.Model` from Charm Bubbles. All text renders in the default foreground color. No token-level coloring.

**What perles has** (not vendored):
- `styles.go` (~64 LOC): defines color tokens for BQL syntax elements (keywords, operators, field names, values, strings)
- `syntax_adapter.go` (~153 LOC): tokenizes input and applies Lipgloss styles per token type

**Implementation approach**: The lexer already tokenizes input. A syntax highlighter would:
1. Run the lexer over the current input text
2. Map each token to a Lipgloss style (keywords in one color, operators in another, field names in another, values in another, errors in red)
3. Render the styled segments instead of the raw textinput view

**Challenge**: Charm's `textinput.Model` doesn't support inline styled rendering. Options:
- Replace `textinput` with a custom input widget that renders styled tokens (moderate effort, cursor management is the hard part)
- Render a styled "preview" line above/below the plain input (simpler but less polished)
- Use `textarea.Model` from Charm Bubbles which has more rendering flexibility

**Effort**: Medium-high. 1 agent, 3 sequential steps (design approach for styled input, implement highlighter, handle cursor/editing edge cases).

**Dependencies**: Decision on whether to use a custom input widget or the preview approach.

#### Gap 4: Recipes as BQL

**Current**: Recipes are YAML structs (`recipe.Recipe`) with typed filter fields:
```yaml
filters:
  status: [open, in_progress]
  priority: [0, 1]
  tags: [bug]
  actionable: true
  created_after: "14d"
sort:
  field: priority
  direction: asc
```

Recipe execution uses dedicated Go code in `applyRecipe()` (~200 LOC) with manual field matching.

**What "recipes as BQL" means**: Each recipe's YAML filters could be compiled to a BQL expression:
```
status in (open, in_progress) and priority in (P0, P1) and label = bug and blocked = false and created_at > -14d order by priority asc
```

**Challenges**:
1. **Recipe features with no BQL equivalent**:
   - `exclude_tags` (negative label filter - doable via `label not in (...)`)
   - `has_blockers` (different from `blocked` - `has_blockers` checks if issue HAS blocking deps regardless of their status)
   - `title_contains` (doable via `title ~ "..."`)
   - `id_prefix` (doable via `id ~ "bt-"`)
   - `created_before`, `updated_before`, `updated_after` (doable with BQL date operators)
   - `sort.secondary` (BQL ORDER BY supports multi-field already)
   - `sort.field = "impact"` and `sort.field = "pagerank"` (computed fields not in BQL's ValidFields)
   - `view` config (columns, max_items, group_by, truncate_title) - these aren't filters, can't be BQL
   - `export` config - not a filter concern
   - `metrics` list - not a filter concern

2. **Two-way conversion**: Converting recipe YAML to BQL is one direction. If recipes "become" BQL, users need to write BQL in recipe files, which is less structured than YAML filters.

3. **AnalysisAccessor gap**: The original plan had `AnalysisAccessor` on `ExecuteOpts` for computed fields (pagerank, impact). This was dropped from the shipped executor. Recipe sort by `impact` or `pagerank` can't be expressed in BQL currently.

**Effort**: High. 1 agent, 4+ sequential steps (design recipe-to-BQL compiler, handle computed fields, migration path for existing recipes, backward compat).

**Dependencies**: Decision on whether recipes *become* BQL (replacement) or recipes *generate* BQL (compilation). Also depends on whether computed sort fields (pagerank, impact) get added to BQL.

---

## 3. Bugs and Limitations

### Bug 1: Status Enum Not Validated

The validator treats `status` as `FieldEnum` but has no `ValidStatusValues` map. The `type` field validates against `ValidTypeValues` (bug, feature, task, epic, chore), but `status = nonexistent` passes validation silently.

**bt's valid statuses**: open, in_progress, blocked, deferred, pinned, hooked, review, closed, tombstone

**Impact**: Low at runtime (MemoryExecutor does string comparison, non-matching values just return empty results). But the validator should catch typos like `status = opne`.

**Fix**: Add a `ValidStatusValues` map to validator.go, parallel to `ValidTypeValues`.

### Bug 2: --robot-bql Bypasses Standard Robot Output Envelope

As noted in Gap 1, `--robot-bql` outputs raw JSON array instead of using `RobotEnvelope` + `robotEncoder`. This means:
- No metadata (timestamp, version, data hash)
- No TOON support
- Inconsistent with every other `--robot-*` command

### Bug 3: readySQL Option Declared But Never Used

In sql.go, `WithReadySQL` is declared as an option on `SQLBuilder` but never consumed in `buildCompare()`. The `blockedSQL` option IS used (line 256-263). The `readySQL` field is dead code.

**Impact**: None currently (SQL builder isn't used at runtime). Should be cleaned up or wired when DoltExecutor is implemented.

### Bug 4: Date Equality Comparison Is Too Strict

`compareTimes()` uses `time.Equal()` for `=` operator. Since dates are resolved to midnight (today) or exact time (relative offsets), a query like `created_at = today` will only match issues created at exactly midnight. Users likely expect `created_at = today` to mean "created today" (any time).

**Impact**: Medium - date equality queries silently return empty results. Users need to write `created_at >= today` which is unintuitive.

**Fix**: For `=` on date fields, compare date-only (truncate both values to midnight).

### Bug 5: ISO Date Parsing Not Supported in BQL

The lexer handles relative dates (`-7d`, `-24h`, `-3m`) and named dates (`today`, `yesterday`) but cannot parse ISO dates (`2026-01-15`). The `readNumber()` method stops at the first non-digit/suffix character, so `2026-01-15` would be lexed as the number `2026` followed by `-01` then `-15`.

This is documented implicitly in the validator's error message ("today, yesterday, -Nd, or ISO date") but ISO dates don't actually work.

**Impact**: Low - relative dates cover most use cases. Users who need absolute date ranges can't express them.

**Fix**: Add ISO date recognition to the lexer (quoted strings work as a workaround: `created_at > "2026-01-15"` - but this produces a ValueString, not ValueDate, so the date comparison path won't fire).

### Limitation 1: No Computed Sort Fields

BQL ORDER BY can sort by any validated field. But recipe sort supports computed fields (`impact`, `pagerank`) that come from graph analysis. BQL has no access to these because `ExecuteOpts` dropped `AnalysisAccessor` from the original plan.

### Limitation 2: No Field Autocomplete

The BQL modal has no tab-completion or field name suggestions. Users must memorize the 16 valid field names. The perles upstream may have this in its TUI (syntax_adapter.go was skipped).

### Limitation 3: EXPAND Only Follows Blocking Dependencies

`expandIssues()` and `isIssueBlocked()` both filter on `dep.Type.IsBlocking()`. Non-blocking dependency types (e.g., "related", "parent/child" if they exist) are invisible to EXPAND and blocked queries.

### Limitation 4: Session-Scoped History Only

The BQL modal's query history is in-memory only. It resets when bt exits. Persistent history (saved to `.bt/bql_history`) would improve the workflow.

---

## 4. Perles Upstream Comparison

Based on the brainstorm's inventory of perles files (session 16 research):

| Perles File | LOC | Status in bt | Notes |
|-------------|-----|-------------|-------|
| ast.go | 129 | Vendored + adapted | Adapted for bt's ExpandClause design |
| token.go | 168 | Vendored + adapted | Added EXPAND/DEPTH/STAR tokens |
| lexer.go | 189 | Vendored + adapted | Added identifier chars (`.`, `/`, `@`, `#`, `+`) for beads IDs |
| parser.go | 453 | Vendored + adapted | Swapped internal/log for log/slog |
| validator.go | 240 | Vendored + rewritten | Field list rewritten for model.Issue |
| sql.go | 377 | Vendored + adapted | Stripped SQLite dialect, Dolt-only |
| executor.go | 923 | **Skipped** - rewritten | bt's MemoryExecutor (523 LOC) is bt-native |
| styles.go | 64 | **Not vendored** | Syntax highlighting colors |
| syntax_adapter.go | 153 | **Not vendored** | Token-to-style mapping for TUI |

**Features in perles but not in bt**:
1. **Syntax highlighting** (styles.go + syntax_adapter.go) - the primary missing feature
2. **SQL executor** - perles may have a working SQL executor; bt has the SQL builder but no DoltExecutor runtime

**Features in bt but not in perles** (bt-specific):
1. **EXPAND clause** with BFS graph traversal and depth limits
2. **Workspace pre-filtering** (multi-repo)
3. **Blocked computed field** with dependency graph walking
4. **CLI composability** (`--bql` pre-filters for all robot commands)
5. **Session query history** in TUI modal

---

## 5. Recommended Priority Order

### Tier 1: Quick fixes (ship with next release)

1. **Add ValidStatusValues to validator** - 15 minutes of work, prevents typo-induced empty results
2. **Fix --robot-bql to use RobotEnvelope + robotEncoder** - follow existing pattern in robot_output.go, adds TOON support + metadata consistency

### Tier 2: High value, moderate effort

3. **Date equality semantics** (Bug 4) - change `=` on date fields to compare date-only. Small code change in `compareTimes()` and `resolveDateValue()`, high usability impact.
4. **Syntax highlighting in modal** - the single most visible UX improvement. Recommend the "styled preview line" approach first (render highlighted BQL below the plain input), upgrade to custom input widget later. Perles's styles.go is only 64 LOC - straightforward to adapt to bt's theme system.

### Tier 3: Significant features, needs design decisions first

5. **Status key redirect through BQL** - needs a decision on how "closed" (multi-status) and "ready" (computed) map to BQL. Consider adding `status_group` or `is_closed` computed fields rather than forcing the mapping onto existing BQL operators. This is the gateway to BQL becoming the single filter engine.
6. **ISO date support** - add date literal recognition to lexer. Lower priority since quoted strings + relative dates cover most cases.
7. **Persistent BQL history** - save query history to `.bt/bql_history`, load on startup. Nice quality-of-life, not blocking anything.

### Tier 4: Strategic, high effort, deferred

8. **Recipes as BQL** - this is an architecture decision, not just implementation. The recipe system works and has features BQL doesn't cover (view config, export config, computed sort fields). Recommend: add `AnalysisAccessor` to `ExecuteOpts` first (enables pagerank/impact sort in BQL), then build a recipe-to-BQL compiler as a one-way translation. Don't replace recipe YAML - let both coexist, with BQL as the runtime representation.
9. **DoltExecutor** - the SQL builder exists and generates valid MySQL/Dolt queries. Implementing DoltExecutor pushes BQL queries to the database instead of in-memory filtering. Only needed when issue counts exceed memory-comfortable levels (thousands+). Current in-memory approach is fine for beads-scale projects.

### Not recommended

- **readySQL cleanup** - dead code in sql.go, but harmless. Clean up when DoltExecutor is implemented, not before.
- **EXPAND for non-blocking deps** - beads only has blocking deps in practice. Add this when/if the schema adds other relationship types.

---

## 6. Summary

The BQL implementation is solid for a first sprint. The parser, executor, TUI modal, and CLI integration all work correctly. The biggest surprise is that `--bql` and `--robot-bql` are already shipped (the memory note that CLI flag was "NOT done" is stale - it landed in the same session).

The two most impactful next steps are:
1. **Fix the robot-bql output quality** (Tier 1 - quick fix, consistency win)
2. **Add syntax highlighting** (Tier 2 - the highest-visibility UX gap)

The "recipes as BQL" vision is the longest road and needs a design session before code. It's the right strategic direction but shouldn't block the smaller improvements.
