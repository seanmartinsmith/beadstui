---
title: "feat: BQL Import - Composable Structured Search"
type: feat
status: active
date: 2026-04-01
origin: docs/brainstorms/2026-04-01-bql-import-brainstorm.md
---

# feat: BQL Import - Composable Structured Search

## Overview

Vendor the BQL (Beads Query Language) parser from zjrosen/perles (MIT), write a bt-native in-memory executor, and wire it into the TUI via a `:` modal. This gives bt composable structured search (`status:open priority<2 label:bug`) while preserving existing fuzzy/semantic search unchanged.

## Orientation

Read these before implementing:

- **Brainstorm**: `docs/brainstorms/2026-04-01-bql-import-brainstorm.md` - all design decisions
- **Issue model**: `pkg/model/types.go` - the Issue struct BQL filters against
- **Filter system**: `pkg/ui/model_filter.go` - `matchesCurrentFilter()` and `applyFilter()`
- **Modal pattern**: `pkg/ui/label_picker.go` - the pattern to follow for the BQL modal
- **Focus enum**: `pkg/ui/model.go:49-74` - focus state management
- **Key dispatch**: `pkg/ui/model_keys.go` - where `:` handler goes
- **View rendering**: `pkg/ui/model_view.go:66-71` - where modal renders

## Key Decisions (from brainstorm)

1. **`:` keybind** opens BQL modal, coexists with `/` fuzzy/semantic
2. **BQL replaces global filters** - status keys (`o`/`c`/`r`/`a`) become BQL shortcuts in a future sprint (deferred - this sprint ships BQL as a parallel filter path; status keys continue working as-is)
3. **In-memory executor now**, SQL executor interface for future global beads
4. **Copy parser layer from perles** (~1,500 LOC), rewrite executor (~300 LOC)
5. **Skip syntax highlighting** for this sprint (styles.go, syntax_adapter.go)

## Implementation Phases

### Phase 1: Copy and Adapt BQL Parser (~1,500 LOC)

Create `pkg/bql/` package. Copy these files from perles's BQL package, adapting as noted:

#### 1.1 Copy as-is (minimal changes)

**`pkg/bql/ast.go`** (~129 LOC)
- Copy perles's `ast.go`
- Change package declaration to `package bql`
- No other changes expected (pure Go, stdlib only)

**`pkg/bql/token.go`** (~168 LOC)
- Copy perles's `token.go`
- Change package declaration
- No other changes expected

**`pkg/bql/lexer.go`** (~189 LOC)
- Copy perles's `lexer.go`
- Change package declaration
- No other changes expected (uses only `fmt`, `strings`, `unicode`)

#### 1.2 Adapt parser (swap logger)

**`pkg/bql/parser.go`** (~453 LOC)
- Copy perles's `parser.go`
- Change package declaration
- Replace `internal/log` import with `log/slog` (Go stdlib structured logger)
- Find ~6 call sites where perles's logger is used (likely error/warning logging during parse)
- Replace with `slog.Warn()` or `slog.Debug()` calls
- No other perles imports expected in this file

#### 1.3 Adapt validator (check field names)

**`pkg/bql/validator.go`** (~240 LOC)
- Copy perles's `validator.go`
- Change package declaration
- Review the valid field list - perles validates against its own Issue fields
- Update to match bt's `model.Issue` fields:
  - `id`, `title`, `description`, `status`, `priority`, `type` (issue_type), `assignee`
  - `created_at`, `updated_at`, `due_date`, `closed_at`
  - `label` (singular, matches against Labels array)
  - `source_repo` (for workspace mode)
  - `blocked` (computed: has open blocking dependencies)
- Remove any perles-specific fields that don't exist in bt's model

#### 1.4 Adapt SQL builder (strip SQLite, keep for future)

**`pkg/bql/sql.go`** (~377 LOC)
- Copy perles's `sql.go`
- Change package declaration
- Replace `appbeads.SQLDialect` with a local `Dialect` type (just a string: `"mysql"`)
- Strip all SQLite dialect branches (bt is Dolt-only, Dolt speaks MySQL)
- This file is NOT used in the sprint's in-memory executor but is needed for the future SQL executor
- Mark with a comment: `// Used by DoltExecutor (not yet implemented). See brainstorm for design.`

#### 1.5 Port parser tests

**`pkg/bql/lexer_test.go`**, **`pkg/bql/parser_test.go`**, **`pkg/bql/validator_test.go`**
- Copy perles's test files for lexer, parser, validator
- Change package declaration
- Update field name assertions in validator tests to match bt's field list
- These should be ~2,000 LOC of portable tests
- Skip executor tests (we're writing our own executor)

**Verification**: `go test ./pkg/bql/...` passes.

---

### Phase 2: Write bt-native MemoryExecutor (~400 LOC)

#### 2.1 Define executor interface

**`pkg/bql/executor.go`**

```go
// BQLExecutor evaluates a parsed BQL expression against issues.
type BQLExecutor interface {
    Execute(expr *ast.Expr, issues []model.Issue, opts ExecuteOpts) []model.Issue
}

type ExecuteOpts struct {
    IssueMap map[string]model.Issue   // For dependency traversal (EXPAND) and blocked checks
    Analysis AnalysisAccessor          // Optional: for computed fields (pagerank, impact)
}

// AnalysisAccessor abstracts access to graph analysis scores.
// Keeps pkg/bql independent of pkg/analysis.
type AnalysisAccessor interface {
    GetPageRankScore(id string) float64
    GetCriticalPathScore(id string) float64
}
```

#### 2.2 Implement MemoryExecutor

**`pkg/bql/memory_executor.go`** (~300 LOC)

Walk the AST and evaluate each node against `model.Issue` fields in memory:

- **Comparison nodes** (`status = "open"`, `priority < 2`): match against Issue struct fields
- **Contains operator** (`title ~ "auth"`): substring match on string fields
- **IN operator** (`status IN ("open", "in_progress")`): set membership
- **Label matching** (`label = "bug"`): check if label exists in `Issue.Labels` slice
- **Date comparisons** (`created_at > -7d`): parse date literals, compare against time.Time fields
- **Priority shorthand** (`P0`, `P1`): convert to numeric comparison
- **Boolean logic** (`AND`, `OR`, `NOT`): recursive evaluation
- **Blocked computed field** (`blocked = true`): walk `Issue.Dependencies` against `IssueMap`
- **ORDER BY**: sort results by specified field and direction
- **EXPAND** (dependency graph traversal): walk dependency chains using `IssueMap`, respect DEPTH limit

Field accessor helper function maps BQL field names to Issue struct fields:

```go
func fieldValue(issue model.Issue, field string) (any, error) {
    switch field {
    case "id":         return issue.ID, nil
    case "title":      return issue.Title, nil
    case "status":     return string(issue.Status), nil
    case "priority":   return issue.Priority, nil
    case "type":       return string(issue.IssueType), nil
    case "assignee":   return issue.Assignee, nil
    case "created_at": return issue.CreatedAt, nil
    case "updated_at": return issue.UpdatedAt, nil
    // ... etc
    }
}
```

#### 2.3 Write executor tests

**`pkg/bql/memory_executor_test.go`** (~500 LOC)

Test against a fixture set of `model.Issue` structs:

- Simple field comparisons (status, priority, type)
- String contains/not-contains
- Date comparisons with relative literals
- Label array membership
- Boolean AND/OR/NOT combinations
- Nested parenthesized expressions
- Blocked computed field with dependency fixtures
- ORDER BY sorting
- EXPAND dependency traversal with depth limits
- Edge cases: empty issues list, unknown fields, nil pointer fields (DueDate, ClosedAt)

**Verification**: `go test ./pkg/bql/...` passes including executor tests.

---

### Phase 3: Build BQL Modal in TUI

#### 3.1 Create BQL query modal

**`pkg/ui/bql_modal.go`** (~200 LOC)

Follow the label picker pattern (`label_picker.go`):

```go
type BQLQueryModal struct {
    input    textinput.Model  // Charm bubbles text input
    width    int
    height   int
    theme    Theme
    err      string           // Parse error message (shown inline)
    history  []string         // Recent queries (session-scoped)
    histIdx  int              // History navigation index
}

func NewBQLQueryModal(theme Theme) BQLQueryModal { ... }
func (m *BQLQueryModal) SetSize(w, h int)        { ... }
func (m *BQLQueryModal) Value() string            { ... }
func (m *BQLQueryModal) SetError(msg string)      { ... }
func (m BQLQueryModal) View() string              { ... }
func (m BQLQueryModal) Update(msg tea.Msg) (BQLQueryModal, tea.Cmd) { ... }
```

View renders:
- Title: "BQL Query" in a `RenderTitledPanel`
- Text input field with the query
- Parse error (if any) below the input in warning color
- Help hint: `enter: apply | esc: cancel | up/down: history`

#### 3.2 Wire into model state

**`pkg/ui/model.go`** - add to Model struct (near line 427, alongside other modal fields):

```go
showBQLQuery bool
bqlQuery     BQLQueryModal
bqlEngine    *bql.MemoryExecutor
```

Add to focus enum (near line 49):

```go
focusBQLQuery
```

Initialize in model constructor:

```go
m.bqlQuery = NewBQLQueryModal(theme)
m.bqlEngine = bql.NewMemoryExecutor()
```

#### 3.3 Wire `:` keybind

**`pkg/ui/model.go`** - add to the **global key dispatch** switch block (~line 3146, alongside `'` recipe picker), NOT `handleListKeys()`. The recipe picker and repo picker are both in this global dispatch, which means `:` works from list view, board view, and any other view that doesn't intercept it first.

```go
case ":":
    m.bqlQuery.SetSize(m.width, m.height-1)
    m.bqlQuery.SetError("")
    m.showBQLQuery = true
    m.focused = focusBQLQuery
    return m, m.bqlQuery.input.Focus()
```

Add new handler function:

```go
func (m Model) handleBQLQueryKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
    switch msg.String() {
    case "enter":
        query := m.bqlQuery.Value()
        if query == "" {
            // Empty query = clear BQL filter, show all
            m.activeBQLExpr = nil
            m.currentFilter = "all"
            m.applyFilter()
        } else {
            // Parse and validate
            expr, err := bql.Parse(query)
            if err != nil {
                m.bqlQuery.SetError(err.Error())
                return m, nil // Stay in modal, show error
            }
            // Clear stale filter state from other filter types
            m.setActiveRecipe(nil)
            m.list.ResetFilter()
            // Store parsed expression, use dedicated BQL path
            m.activeBQLExpr = expr
            m.applyBQL(expr, query)
            // Add to history
            m.bqlQuery.AddToHistory(query)
        }
        m.showBQLQuery = false
        m.focused = focusList
        m.setTransientStatus("BQL: " + query)
        return m, nil
    case "esc":
        m.showBQLQuery = false
        m.focused = focusList
        return m, nil
    case "up":
        // Navigate query history
        m.bqlQuery.HistoryPrev()
        return m, nil
    case "down":
        m.bqlQuery.HistoryNext()
        return m, nil
    default:
        var cmd tea.Cmd
        m.bqlQuery, cmd = m.bqlQuery.Update(msg)
        return m, cmd
    }
}
```

#### 3.4 Wire into key dispatch

**`pkg/ui/model.go`** - in the main Update() function's modal overlay section (lines 2459-2567, alongside repo picker and recipe picker checks), add BQL modal dispatch:

```go
if m.showBQLQuery {
    m, cmd = m.handleBQLQueryKeys(msg)
    return m, cmd
}
```

**Important**: This goes in the early modal dispatch section (before line 2567), NOT near line 3264 which is the focus-based key dispatch for list view. Modal overlays intercept keys before focus handlers.

Also add `focusBQLQuery` case to three mouse scroll handlers in model.go (lines ~3198, ~3306, ~3335) that have exhaustive focus switches - return no-op for BQL focus since the modal doesn't scroll.

#### 3.5 Wire into filter system - dedicated BQL path

**Critical**: BQL does NOT go through `matchesCurrentFilter()`. BQL has set-level operations (ORDER BY, EXPAND) that can't work per-issue. Instead, BQL gets its own execution path parallel to `applyRecipe()`.

**`pkg/ui/model_filter.go`** - add `applyBQL()` function (~100 LOC), following the `applyRecipe()` pattern (line 346):

```go
func (m *Model) applyBQL(expr *ast.Expr, queryStr string) {
    // Workspace pre-filter (same as applyRecipe lines 357-363)
    issues := m.issues
    if m.workspaceMode && m.activeRepos != nil {
        prefiltered := make([]model.Issue, 0, len(issues))
        for _, issue := range issues {
            repoKey := strings.ToLower(ExtractRepoPrefix(issue.ID))
            if repoKey == "" || m.activeRepos[repoKey] {
                prefiltered = append(prefiltered, issue)
            }
        }
        issues = prefiltered
    }

    opts := bql.ExecuteOpts{
        IssueMap: m.issueMap,
        Analysis: m.analysis, // For computed fields: pagerank, impact
    }

    // Execute BQL - handles filtering, ORDER BY, and EXPAND as set-level ops
    filtered := m.bqlEngine.Execute(expr, issues, opts)

    // Build list items from filtered results (same pattern as applyRecipe lines 419-439)
    var filteredItems []list.Item
    for _, issue := range filtered {
        item := IssueItem{
            Issue:      issue,
            GraphScore: m.analysis.GetPageRankScore(issue.ID),
            Impact:     m.analysis.GetCriticalPathScore(issue.ID),
            DiffStatus: m.getDiffStatus(issue.ID),
            RepoPrefix: ExtractRepoPrefix(issue.ID),
        }
        item.TriageScore = m.triageScores[issue.ID]
        if reasons, exists := m.triageReasons[issue.ID]; exists {
            item.TriageReason = reasons.Primary
            item.TriageReasons = reasons.All
        }
        item.IsQuickWin = m.quickWinSet[issue.ID]
        item.IsBlocker = m.blockerSet[issue.ID]
        item.UnblocksCount = len(m.unblocksMap[issue.ID])
        filteredItems = append(filteredItems, item)
    }

    // NOTE: Do NOT call sortFilteredItems() here.
    // BQL ORDER BY already sorted the results inside Execute().
    // If no ORDER BY in the query, results keep default order.

    m.list.SetItems(filteredItems)
    m.updateSemanticIDs(filteredItems)
    m.currentFilter = "bql:" + queryStr

    // Update board and graph (same as applyRecipe lines 558-563)
    m.board.SetIssues(filtered)
    filterIns := m.analysis.GenerateInsights(len(filtered))
    m.graphView.SetIssues(filtered, &filterIns)

    if len(filteredItems) > 0 && m.list.Index() >= len(filteredItems) {
        m.list.Select(0)
    }
    m.updateViewportContent()
}
```

**`pkg/ui/model_filter.go`** - extend `filteredIssuesForActiveView()` (line 106) to route BQL:

```go
func (m *Model) filteredIssuesForActiveView() []model.Issue {
    // BQL filter active? Use BQL executor (set-level operations)
    bqlFilterActive := m.activeBQLExpr != nil && strings.HasPrefix(m.currentFilter, "bql:")
    if bqlFilterActive {
        // Apply workspace pre-filter inline (same logic as applyBQL)
        issues := m.issues
        if m.workspaceMode && m.activeRepos != nil {
            prefiltered := make([]model.Issue, 0, len(issues))
            for _, issue := range issues {
                repoKey := strings.ToLower(ExtractRepoPrefix(issue.ID))
                if repoKey == "" || m.activeRepos[repoKey] {
                    prefiltered = append(prefiltered, issue)
                }
            }
            issues = prefiltered
        }
        opts := bql.ExecuteOpts{IssueMap: m.issueMap, Analysis: m.analysis}
        return m.bqlEngine.Execute(m.activeBQLExpr, issues, opts)
    }

    // Recipe filter active? (existing path, line 108)
    recipeFilterActive := m.activeRecipe != nil && strings.HasPrefix(m.currentFilter, "recipe:")
    // ... rest unchanged
}
```

This ensures `refreshBoardAndGraphForCurrentFilter()` (line 132) also works with BQL since it calls `filteredIssuesForActiveView()`.

**`pkg/ui/model_filter.go`** - extend `clearAllFilters()` (line 47) to clear BQL state:

```go
func (m *Model) clearAllFilters() {
    m.currentFilter = "all"
    m.setActiveRecipe(nil)
    m.activeBQLExpr = nil    // Clear BQL state
    m.list.ResetFilter()
    m.applyFilter()
}
```

#### 3.6 Handle Dolt refresh re-application

When issues refresh from Dolt polling, the existing code path at model.go ~line 2137-2151 calls `refreshBoardAndGraphForCurrentFilter()` which routes through `filteredIssuesForActiveView()`. The BQL routing branch added in 3.5 handles this automatically.

However, there's also an explicit recipe re-application at ~line 2148:
```go
if m.activeRecipe != nil {
    m.applyRecipe(m.activeRecipe)
}
```

Add a parallel BQL re-application after this block:
```go
if m.activeBQLExpr != nil && strings.HasPrefix(m.currentFilter, "bql:") {
    queryStr := strings.TrimPrefix(m.currentFilter, "bql:")
    m.applyBQL(m.activeBQLExpr, queryStr)
}
```

This ensures BQL filters persist through data refreshes.

#### 3.7 Wire into View rendering

**`pkg/ui/model_view.go`** - add modal render (near line 66, alongside other modal checks):

```go
} else if m.showBQLQuery {
    body = m.bqlQuery.View()
```

#### 3.8 Update status bar

**`pkg/ui/model_footer.go`** - the filter badge is a switch statement (lines 81-102). Add a BQL case before the `default` block, following the recipe pattern:

```go
// Add before the default case (around line 94)
case strings.HasPrefix(m.currentFilter, "bql:"):
    filterTxt = "BQL: " + strings.TrimPrefix(m.currentFilter, "bql:")
    filterIcon = "🔍"
```

Note: the footer switch uses `case` with expressions, not just constants - recipe prefix already uses `strings.HasPrefix` pattern in the default block, so follow that convention.

#### 3.9 Write modal tests

**`pkg/ui/bql_modal_test.go`** (~100 LOC)

- Test modal opens/closes on `:` / `esc`
- Test enter with valid query sets `m.currentFilter`
- Test enter with invalid query shows error, doesn't close modal
- Test empty enter clears filter to "all"

**Verification**: `go test ./pkg/ui/...` passes, `go build ./cmd/bt/` compiles.

---

### Phase 4: Integration Verification

#### 4.1 End-to-end manual test

Build and run bt against a project with beads:

```bash
go build ./cmd/bt/ && ./bt
```

Test:
- Press `:`, type `status:open`, press enter - should filter to open issues
- Press `:`, type `priority<2`, press enter - should show P0 and P1 only
- Press `:`, type `status:open AND priority<2 AND label:bug`, press enter - combined filter
- Press `:`, type invalid query, press enter - should show error, modal stays open
- Press `:`, press enter with empty input - should clear filter to "all"
- Press `o` then `:` - BQL should replace the status filter
- Verify board view and graph view also reflect BQL filter

#### 4.2 Run full test suite

```bash
go test ./...
```

Ensure no regressions. Existing filter tests should still pass since BQL is additive.

---

## Acceptance Criteria

- [ ] `pkg/bql/` package exists with parser, lexer, AST, tokens, validator from perles
- [ ] `pkg/bql/LICENSE` includes perles MIT copyright; copied files have origin comment headers
- [ ] `go test ./pkg/bql/...` passes (ported parser/lexer/validator tests + new executor tests)
- [ ] `:` keybind opens BQL query modal from list view
- [ ] Valid BQL queries filter the issue list via dedicated `applyBQL()` path
- [ ] BQL filter results propagate to list, board, AND graph views
- [ ] BQL ORDER BY controls result ordering (overrides sortMode)
- [ ] Invalid BQL queries show inline parse error without closing the modal
- [ ] Empty BQL query clears filter to "all" and clears activeBQLExpr
- [ ] BQL filter state shows in the status bar filter badge
- [ ] `BQLExecutor` interface exists with `AnalysisAccessor` for future SQL executor
- [ ] `sql.go` is present (Dolt-only, stripped of SQLite) for future use
- [ ] `go test ./...` passes with no regressions
- [ ] `go build ./cmd/bt/` compiles cleanly

## Files Created

| File | LOC (est) | Purpose |
|------|-----------|---------|
| `pkg/bql/ast.go` | 129 | AST node types (from perles) |
| `pkg/bql/token.go` | 168 | Token types (from perles) |
| `pkg/bql/lexer.go` | 189 | Lexer (from perles) |
| `pkg/bql/parser.go` | 453 | Parser (from perles, swap logger) |
| `pkg/bql/validator.go` | 240 | Field validation (adapted for bt fields) |
| `pkg/bql/sql.go` | 300 | SQL builder (stripped SQLite, future use) |
| `pkg/bql/executor.go` | 80 | BQLExecutor interface + AnalysisAccessor |
| `pkg/bql/memory_executor.go` | 300 | In-memory executor |
| `pkg/bql/LICENSE` | ~20 | MIT license from perles |
| `pkg/bql/lexer_test.go` | ~400 | Lexer tests (from perles) |
| `pkg/bql/parser_test.go` | ~800 | Parser tests (from perles) |
| `pkg/bql/validator_test.go` | ~400 | Validator tests (adapted) |
| `pkg/bql/memory_executor_test.go` | ~500 | Executor tests (new) |
| `pkg/ui/bql_modal.go` | 200 | BQL query input modal |
| `pkg/ui/bql_modal_test.go` | 100 | Modal tests |

## Files Modified

| File | Change |
|------|--------|
| `pkg/ui/model.go` | Add focusBQLQuery enum (~line 73), showBQLQuery/bqlQuery/bqlEngine/activeBQLExpr fields (~line 427), init in constructor, `:` keybind in global dispatch (~line 3146), modal dispatch in Update() (~line 2549-2567), BQL re-application in Dolt refresh (~line 2148), focusBQLQuery case in 3 mouse scroll handlers (~lines 3198, 3306, 3335) |
| `pkg/ui/model_keys.go` | Add handleBQLQueryKeys() function |
| `pkg/ui/model_filter.go` | Add `applyBQL()` function (~100 LOC with workspace pre-filter), add BQL routing in `filteredIssuesForActiveView()`, add `m.activeBQLExpr = nil` to `clearAllFilters()` |
| `pkg/ui/model_view.go` | Add showBQLQuery render branch |
| `pkg/ui/model_footer.go` | Add BQL filter badge display |

## Risks and Caveats

1. **Perles source not directly verified.** The BQL file inventory (names, LOC, imports) comes from a research agent that fetched the repo. The actual package name, export patterns, and `Parse()` entry point need verification during Phase 1. If the structure differs significantly from what's documented, adapt - the parser architecture (hand-rolled recursive descent) is confirmed.

2. **MIT attribution required.** Add a `pkg/bql/LICENSE` file with perles's MIT copyright notice, plus a comment header in copied files noting the origin: `// Adapted from github.com/zjrosen/perles (MIT License)`.

3. **EXPAND complexity.** Dependency graph traversal with depth limits is the most complex executor feature. If it proves too complex for the sprint, defer EXPAND to a follow-up and ship the core filter/sort operations first. The parser will still parse EXPAND syntax - just return an "EXPAND not yet supported" error from the executor.

## Dependencies

- No new external dependencies. BQL parser is pure Go (stdlib only).
- `charmbracelet/bubbles` textinput already available (used by time travel input at model.go:449).

## Sources

- **Origin brainstorm:** [docs/brainstorms/2026-04-01-bql-import-brainstorm.md](docs/brainstorms/2026-04-01-bql-import-brainstorm.md) - activation keybind, scope, executor strategy, copy list, competitive context, global beads future
- **Perles BQL source:** github.com/zjrosen/perles (MIT license) - parser, lexer, AST, validator, SQL builder
- **bt compatibility surface:** [docs/brainstorms/2026-03-26-bd-bt-compatibility-surface.md](docs/brainstorms/2026-03-26-bd-bt-compatibility-surface.md) - bd CLI commands, schema contract
