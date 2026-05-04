# BQL — Beads Query Language

A small composable query language for filtering and ordering issues from inside `bt`. BQL lets you ask "show me open P0/P1 bugs touched in the last week, sorted by priority" without leaving the TUI or shelling around `bd list` flags.

This is a user reference. For the parser implementation, see [`pkg/bql/`](../pkg/bql/).

## Quick reference

```
status = open and priority <= P1                                      filter
type in (bug, feature) and assignee = sms                             set membership
title ~ auth and not blocked = true                                   substring + boolean
updated_at > -7d order by priority asc                                date + sort
type = epic expand down depth 3                                       graph traversal
```

| Operator | Means | Example |
|---|---|---|
| `=`  | equals (case-insensitive for strings) | `status = open` |
| `!=` | not equals | `status != closed` |
| `<` `>` `<=` `>=` | comparison (priorities and dates) | `priority < P2` |
| `~`  | substring (case-insensitive) | `title ~ auth` |
| `!~` | does not contain | `title !~ test` |
| `in (a, b, ...)` | any of | `type in (bug, task)` |
| `not in (a, b, ...)` | none of | `label not in (deferred, wontfix)` |
| `and` `or` `not` | boolean composition | `not blocked = true` |
| `( ... )` | grouping | `(type = bug or type = feature) and priority <= P1` |
| `order by <field> [asc|desc]` | sort | `order by priority asc, updated_at desc` |
| `expand <up|down|all> [depth N|*]` | walk dependencies | `expand down depth 3` |

Keywords are case-insensitive (`AND`, `and`, `And` all work). Field names and string values are matched case-insensitively. By convention this doc uses lowercase.

## Field reference

| Field | Type | Operators | Notes |
|---|---|---|---|
| `id` | string | `=` `!=` `~` `!~` `in` | Bead ID, e.g. `bt-001`. Unquoted hyphenated IDs are fine. |
| `title` | string | `=` `!=` `~` `!~` `in` | |
| `description` | string | `=` `!=` `~` `!~` `in` | Body text. |
| `design` | string | `=` `!=` `~` `!~` `in` | Design field. |
| `notes` | string | `=` `!=` `~` `!~` `in` | Notes field. |
| `status` | enum | `=` `!=` `in` | `open`, `in_progress`, `blocked`, `deferred`, `pinned`, `hooked`, `review`, `closed`, `tombstone` |
| `priority` | priority | `=` `!=` `<` `>` `<=` `>=` `in` | `P0`–`P4` (or `0`–`4`). Lower number = higher priority, so `priority < P2` means P0 or P1. |
| `type` | enum | `=` `!=` `in` | `bug`, `feature`, `task`, `epic`, `chore` |
| `assignee` | string | `=` `!=` `~` `!~` `in` | |
| `label` | string (multi-valued) | `=` `!=` `~` `!~` `in` `not in` | Membership test against the issue's label set. `label = bug` matches if any label equals `bug`. |
| `source_repo` | string | `=` `!=` `~` `!~` `in` | Source project (cross-project hub mode). |
| `blocked` | bool | `=` `!=` | Computed: true iff the issue has at least one open blocking dependency. |
| `created_at` | date | `=` `!=` `<` `>` `<=` `>=` | See [Dates](#dates). |
| `updated_at` | date | `=` `!=` `<` `>` `<=` `>=` | |
| `due_date` | date | `=` `!=` `<` `>` `<=` `>=` | |
| `closed_at` | date | `=` `!=` `<` `>` `<=` `>=` | |

Unknown fields produce a validation error listing the valid set.

### Strings — `=` vs `~`

- `=` and `!=` are full-string, case-insensitive match.
  `assignee = sms` matches "sms", "SMS", "Sms".
- `~` and `!~` are substring (case-insensitive). Despite the name, `~` is **not** a regex.
  `title ~ auth` matches "Auth bug", "reauthorize", "lauth".

Quoting:

- Bare identifiers can include letters, digits, and `_ - : . / @ # +`. So `id = bt-001`, `assignee = sms`, `label = area:docs` all work unquoted.
- Use double or single quotes for values containing spaces or other characters: `title = "hello world"`, `assignee = 'first last'`.

### Priority

`P0` is the highest priority, `P4` is the lowest. The numeric form (`0`–`4`) is also accepted: `priority = 1` is the same as `priority = P1`. Comparison is on the numeric level, so `priority <= P1` matches P0 and P1.

### Booleans

Only `blocked` is a boolean field, and only `=` / `!=` are valid. `blocked` is computed at query time from the dependency graph — true iff the issue has at least one blocking dependency that is itself open (not closed or tombstoned). It requires the executor to have access to the full issue map; in the TUI and robot mode this is always available.

## Dates

| Form | Meaning |
|---|---|
| `today` | midnight today, in local time |
| `yesterday` | midnight yesterday |
| `-Nd` | N days ago (e.g. `-7d`) |
| `-Nh` | N hours ago (e.g. `-24h`) |
| `-Nm` | N months ago (e.g. `-3m`) |
| `YYYY-MM-DD` | absolute ISO date (e.g. `2026-01-15`) |

Notes:

- For `=` and `!=`, comparisons are date-only. `created_at = 2026-03-01` matches any time on that day, not just midnight.
- For `<` `>` `<=` `>=`, the full timestamp is used. `updated_at > -24h` is strictly within the last 24 hours.
- Date offsets are always **negative** (in the past). There is no `+Nd`.
- `-Nm` is months, not minutes. Minutes are not supported. Hours use `-Nh`.

## Set ops

```
type in (bug, feature, task)
label not in (wontfix, duplicate)
priority in (P0, P1)
```

`in` and `not in` are valid for string, enum, and priority fields. They are not valid for booleans or dates (use comparisons instead).

For `label`, `in` tests "any label in the issue matches any value in the list".

## Boolean composition and precedence

```
status = open and priority <= P1
type = bug or type = feature
not blocked = true
status = open and not (label = wontfix or label = deferred)
```

Precedence, tightest first:

1. `not`
2. `and`
3. `or`

So `a or b and c` parses as `a or (b and c)`. Use parentheses when in doubt.

The parser caps nesting at depth 100; in practice you'll never hit this.

## ORDER BY

```
order by priority asc
order by priority asc, updated_at desc
status = open order by priority asc
order by created_at         (defaults to asc)
```

- One or more fields, comma-separated.
- Each field optionally followed by `asc` (default) or `desc`.
- `order by` can appear with or without a filter — `order by priority` alone returns every issue, sorted.
- Sort fields must be in the field reference above.

## EXPAND

EXPAND walks the dependency graph after filtering, adding related issues to the result.

```
type = epic expand down                  children/dependents, 1 hop
type = epic expand down depth 3          three hops down
id = bt-004 expand up                    blockers, 1 hop
id = bt-004 expand up depth 2            two hops of blockers
expand all depth *                       full connected component, both directions
```

| Direction | Meaning |
|---|---|
| `up` | follow blocking dependencies — what this issue depends on (parents/blockers) |
| `down` | follow reverse blocking dependencies — what depends on this issue (children/blocked-by) |
| `all` | both directions |

Depth:

- Default is `1` (one hop).
- `depth N` for an explicit number, where `1 <= N <= 10`.
- `depth *` for unlimited (capped internally at 100 hops as a safety net; in practice you'll see the whole connected component).

EXPAND only follows **blocking** dependencies. Other relationship kinds (parent/child, related-to) are not traversed.

## Recipes

```
# Open P0/P1, sorted by priority, freshest first
status = open and priority <= P1 order by priority asc, updated_at desc
```

```
# My active work
assignee = sms and status in (open, in_progress) order by priority asc
```

```
# Recently touched bugs that aren't blocked
type = bug and updated_at > -14d and not blocked = true order by priority asc
```

```
# Anything closed today
status = closed and closed_at = today
```

```
# Stale open work — not touched in three months
status = open and updated_at < -3m order by updated_at asc
```

```
# Docs work in flight
label ~ docs and status != closed
```

```
# Investigate a blocking cascade — start at one bead, walk down to everything it blocks
id = bt-001 expand down depth *
```

```
# What's blocking this work — walk up the chain
id = bt-004 expand up depth *
```

```
# Open work in a specific epic and its descendants
id = bt-mhcv expand down order by priority asc
```

```
# Cross-project: open P1 issues in the cass project
source_repo = cass and status = open and priority <= P1
```

## Where it works

### TUI prompt

Press `:` from the list view to open the BQL query bar. Type a query, press Enter.

- Empty query clears the active BQL filter.
- Up/down arrows cycle query history.
- Parse and validation errors are shown inline; the modal stays open so you can fix the query.
- BQL coexists with status/priority filters and the search bar; opening BQL clears the conflicting filter state.

### Robot mode

Two ways to drive BQL from `bt robot`:

**`--bql` flag (filter pre-pass)** — accepted by every robot subcommand that goes through `robotPreRun`. The query is parsed and applied before the subcommand's analysis runs:

```bash
bt robot triage   --bql 'priority <= P1 and status = open'
bt robot list     --bql 'type = bug and updated_at > -7d'
bt robot insights --bql 'label = backend'
bt robot plan     --bql 'assignee = sms'
```

**`bt robot bql` subcommand (raw filter)** — emits the filtered issue set as JSON, with `--limit` and `--offset` for pagination. Use this when you just want a query result, not a derived view:

```bash
bt robot bql --query 'status = open and priority <= P1'
bt robot bql --query 'type = bug' --limit 25 --offset 0
bt robot bql 'assignee = sms and updated_at > -7d'   # query as positional arg
```

The response envelope includes `count` (returned), `total_count` (matched before paging), and echoes the BQL string under `query.bql`.

### Bare `bt`

The same `--bql` flag is available at the top-level when launching the TUI:

```bash
bt --bql 'status = open and priority <= P2'
```

The query is applied as the initial filter when the TUI opens.

## Errors

Common parse and validation messages and what they mean:

| Message | Cause |
|---|---|
| `expected field name at position N, got "X"` | Missing or misspelled field at start of an expression. |
| `expected operator at position N, got "X"` | Field with no operator after it (`type task`). |
| `expected value at position N` | Operator with no right-hand side (`type =`). |
| `expected ')' at position N` | Unbalanced parens. |
| `unknown field: "X" (valid: ...)` | Field not in the field reference. |
| `operator "X" is not valid for ... field "Y"` | Wrong operator for type — e.g. `>` on an enum, `~` on a date. |
| `invalid value "X" for field "Y" (valid: ...)` | Enum value not in the allowed set (`type = bog`). |
| `field "Y" requires a date value (today, yesterday, -Nd, or ISO date)` | Bare string where a date was expected. |
| `depth must be at least 1` / `depth cannot exceed 10` | EXPAND `depth N` outside `[1, 10]`. Use `*` for unlimited. |

## See also

- [`pkg/bql/`](../pkg/bql/) — parser, validator, and in-memory executor
- [`docs/robot/README.md`](robot/README.md) — robot subcommand reference, including `--bql` semantics per subcommand
- [`pkg/bql/LICENSE`](../pkg/bql/LICENSE) — the parser is adapted from [Perles](https://github.com/zjrosen/perles) by Zach Rosen, MIT-licensed
