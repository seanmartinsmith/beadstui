# Keybindings Audit

> Status: initial survey 2026-04-23, dogfooding session
> Feeds: bt-gf3d (keybinding consistency overhaul), bt-tkhq (research conventions), bt-xavk (help system redesign)
> Scope: main issues list view + global dispatcher + help/shortcuts surfaces. Per-view key maps documented but not exhaustively verified against visible help copy.

This document is a living survey. Update when keys are added, moved, or retired.

## Methodology

Grep-based scan of:

- `pkg/ui/model_keys.go` — per-view key handlers (board, graph, tree, actionable, history, flow matrix, recipe picker, repo picker, BQL query, label picker, insights, main list, time-travel input, help)
- `pkg/ui/model_update_input.go` — top-level `handleKeyPress` dispatcher (global shortcuts, view switches, modal keys)
- `pkg/ui/model_view.go:492-560` — `?` help-overlay content (source of truth for the help modal)
- `pkg/ui/shortcuts_sidebar.go:89-191` — `;` shortcuts-sidebar content

Dispatch order: when a key arrives, `handleKeyPress` runs first (top-level globals + view switches). Unhandled keys fall through to the per-view handler (e.g., `handleListKeys`). This means per-view handlers can only "catch" keys the global dispatcher doesn't claim.

## Key surface — main list + globals

Legend: **G** = caught at global dispatcher; **L** = caught at list handler only; **M** = modal-specific only.

### Letter keys (lowercase)

| Key | Binding | Layer | Notes |
|---|---|---|---|
| `a` | Toggle actionable view | G | Also "all filter" in list (`handleListKeys:976`) — unreachable because global grabs it first |
| `b` | Toggle kanban board | G | |
| `c` | Closed filter | G | Also in alerts modal (severity cycle) |
| `d` | — | free at list | Used in board (`toggle expand`) |
| `e` | — | free at list | Used in board (`toggle empty cols`), insights (`explanations`) |
| `f` | Toggle flow matrix | G | |
| `g` | Toggle graph view | G | Also vim `gg` top-of-list combo in board |
| `h` | Toggle history view | G | Also list handler has `h` case — duplicate, unreachable via list |
| `i` | Toggle insights | G | |
| `j` / `k` | Move down / up | G→L | Universal |
| `l` | Open label picker | G | Filter-by-label in help copy |
| `m` | — | free at list | Used in insights (heatmap) |
| `n` / `N` | — | free at list | Used in board (search match next/prev) |
| `o` | Open filter | G | Also in alerts modal |
| `p` | Toggle priority hints | G | Help overlay puts under Actions; shortcuts bar puts under Views — drift |
| `q` | Back / quit | G | |
| `r` | Ready filter | G | Also in alerts modal |
| `s` | Cycle sort (list) | L | Different meaning per view: board=swimlane cycle, graph=swarm toggle, alerts=severity cycle |
| `t` | Time-travel prompt | L | |
| `u` | — | free at list | |
| `v` | — | free at list | Used in history (git/bead mode) |
| `w` | Project picker (workspace) / wisp toggle (single-repo) | G | Overloaded by mode |
| `x` | Export to markdown | G | List handler also has `x` via insights (calc proof) in that view |
| `y` | Copy ID | G→L | |
| `z` | — | free at list | Free everywhere scanned |

### Letter keys (uppercase)

| Key | Binding | Layer | Notes |
|---|---|---|---|
| `A`–`D` | — | free | |
| `E` | Toggle hierarchical tree view | G | |
| `F` | — | free | Used as `f/F` in insights (filter) |
| `G` | End/last item | G→L | Also vim-end navigation |
| `H` | Hybrid ranking | G | Also in board (jump to first column) |
| `I` | — | free | |
| `J` / `K` | — | free at list | Used in history view (detail scroll) |
| `L` | — | free at list | Footer shows `L:labels` but no handler — **bug or stale footer copy** |
| `M`–`Q` | — | free | |
| `R` | — | free | `Ctrl+R` is force refresh (different binding) |
| `S` | Apply triage recipe | L | Scheduled to move (bt-ktcr) — wants to become reverse-cycle sort |
| `T` | Quick time-travel (HEAD~5) | L | |
| `U` | Self-update | L | |
| `V` | Cass sessions correlator | L | Present in `;` shortcuts bar but **missing from `?` help overlay** |
| `W` | Toggle project scope (home ↔ all) | G | Help inverts `w` and `W` descriptions — **drift** |
| `X`–`Z` | — | free | |

### Punctuation and chords

| Key | Binding | Layer | Notes |
|---|---|---|---|
| `/` | Fuzzy search | G | Also board search |
| `?` | Help overlay | G | |
| `;` | Shortcuts sidebar | G | |
| `!` | Alerts panel | G | |
| `:` | BQL query modal | G | |
| `'` | Recipe picker | G | |
| `[` / `f3` | Label dashboard | G | **Not "Attention view"** — `;` sidebar copy is correct; mental models may still read `[` as attention |
| `]` / `f4` | Attention view | G | |
| `<` | Shrink list pane | G | **Undocumented** in both `;` and `?` — bead needed |
| `>` | Expand list pane | G | **Undocumented** in both `;` and `?` — bead needed |
| `#`, `$`, `%`, `&`, `*`, `+`, `=`, `@`, `~`, `\|`, `,`, `.`, `-` | — | all free | `$` used in board (move to last in column); otherwise free |
| `0`–`9` | Board-only column jump | — | Free on list layer |
| `Tab` | Toggle split / focus | view-dep | |
| `Esc` | Back / close | G→L | |
| `Enter` | Details / open | G→L | |
| `Space` | Open tutorial from help | M | |
| `Ctrl+R` / `F5` | Force refresh | G | |
| `Ctrl+S` | Semantic search | G | |
| `Ctrl+C` | Force quit | G | |
| `Ctrl+D` / `Ctrl+U` | Page down/up | G | |
| `Ctrl+J` / `Ctrl+K` | Scroll detail pane | G | |
| `Alt+H` | Hybrid preset | G | |

## Free keys summary (main list layer)

**Truly free** (unbound at both global dispatcher and main list handler; may be used in other views but that's OK — view handlers are scoped):

- Lowercase letters: `d`, `e`, `m`, `n`, `u`, `v`, `z`
- Uppercase letters: `A`, `B`, `C` (wait — C=copy), `D`, `F`, `I`, `J`, `K`, `M`, `N`, `P`, `Q`, `R`, `X`, `Y`, `Z`
  - Correction: `C` = copy to clipboard — **not free**
  - Also-used-elsewhere-but-free-on-list: `J`/`K` (history detail scroll), `F` (insights filter)
- Punctuation: `#`, `%`, `&`, `*`, `+`, `=`, `@`, `~`, `\|`, `,`, `.`, `-`, most digits
- Chord space: plenty of `Alt+*`, `Ctrl+*` with single-letter collisions; requires per-terminal testing

## Drift between surfaces

The `;` shortcuts sidebar (`pkg/ui/shortcuts_sidebar.go`) and the `?` help overlay (`pkg/ui/model_view.go:492-560`) are **independently maintained** lists. They drift.

| Issue | Help overlay (`?`) | Shortcuts sidebar (`;`) | Code truth | Severity |
|---|---|---|---|---|
| `V` = Cass sessions | **missing** | listed under Actions | bound | high — undiscoverable from help |
| `p` = Priority hints | under Actions | under Views | Actions-ish (toggles an overlay) | medium — category split |
| `;` itself | under Global | under Views as "This sidebar" | global handler | low — naming |
| `<` / `>` = pane resize | **missing** | **missing** | bound | high — undocumented power feature |
| `w` / `W` swap | `w` = Project picker, `W` = Toggle project scope | sidebar has no Global section | code matches help | OK (but confusing: `W` is the quick shortcut, `w` is the full picker — consider renaming descriptions) |
| Footer `L:labels` | `l` = Filter by label | `l` = Label picker | `l` bound, `L` unbound on list | high — footer implies binding that doesn't exist |
| `f` = Flow matrix | under Views | **missing from sidebar Views** | bound | medium |
| `[` `]` = Label/Attention | under Views | under Views | bound | OK |
| `Ctrl+R`/`F5` = Force refresh | under Actions | **missing from sidebar** | bound | medium |
| `S` label: "Triage sort" | fine | fine | actually applies the triage **recipe** (filter + sort) | low — wording only |

**Structural recommendation**: the two surfaces should consume a shared registry. Today they're hand-maintained string tables in two files; the list structures are literally identical types (`{key, desc string}`). Merging into one source would prevent future drift.

## Cross-view conflicts (same key, different meanings)

These aren't bugs per se — per-view handlers legitimately scope keys differently — but they're cognitive load.

| Key | List | Board | Graph | Insights | History | Alerts modal |
|---|---|---|---|---|---|---|
| `s` | cycle sort | swimlane cycle | swarm toggle | — | — | severity cycle |
| `S` | triage recipe | — | — | — | — | severity cycle reverse |
| `e` | — | toggle empty cols | — | explanations | — | — |
| `d` | — | toggle expand | — | — | — | — |
| `c` | closed filter | — | — | — | confidence filter | (closed filter) |
| `h` | history view | — | left nav | switch panel | — | — |
| `l` | label picker | — | right nav | switch panel | — | — |
| `n`/`N` | — | next/prev match | — | — | — | — |
| `v` | — | — | — | — | git/bead mode | — |

**Noteworthy**: `s` does something different in every major view. Cycling is the unifying theme (sort / swimlane / swarm / severity) but the *what being cycled* is wildly different. Acceptable if documented per-view in the sidebar (today it is, for board and insights; less so elsewhere).

## Convention observations

1. **Reverse-cycle pattern is bt's own convention** — the alerts modal uses `s` forward / `S` reverse for severity (`model_update_input.go:299, 364`). The main list breaks this by using `S` for triage recipe instead. This is what bt-ktcr will fix.

2. **Uppercase as alternate** — `g`/`G`, `j`/`k` → `J`/`K` scroll, `t`/`T` quick-variant, `w`/`W`. Largely consistent. Exceptions: `H` (hybrid ranking — orthogonal to `h`), `L` (unbound but implied by footer).

3. **View-switch letters are mnemonic** — `a` actionable, `b` board, `g` graph, `h` history, `i` insights, `f` flow, `[` label dashboard (weak), `]` attention (weak). Brackets break the mnemonic pattern; `d` dashboard and `n` attention are free if we ever want to fix that.

4. **Workflow-symbol keys** — `!` alerts, `'` recipes, `;` shortcuts, `?` help, `:` BQL, `/` search. Reasonable; all punctuation keys have mnemonics rooted in chat/vim conventions.

## Open gaps flagged for bead creation

These surfaced during the audit. Not filed yet.

1. **`<` / `>` pane resize undocumented** — power feature absent from both help surfaces. Needs a bead: add to help overlay + shortcuts sidebar. Related: pane-resize widens list but **breaks label column alignment** (observed 2026-04-23 in image 4 of dogfood session) — that's a separate rendering bug.

2. **Footer shows `L:labels` with no `L` handler on main list** — footer copy drift or missing binding. Needs a bead to decide which way: make L do something (label dashboard shortcut?) or fix footer.

3. **`V` missing from `?` help overlay** — already flagged under drift table. Either add V to help or retire V and route cass-sessions through a menu.

4. **Help surfaces have no shared registry** — structural refactor candidate. Today: `shortcuts_sidebar.go` and `model_view.go` each hardcode the same type. Merging is small (< 100 LOC) and prevents future drift. Could fit under bt-xavk (help redesign).

5. **`s` means five different things across views** — not necessarily a bug, but worth a UX review: is this the price of dense single-letter bindings, or should cross-view consistency take priority? Bead candidate under bt-gf3d or bt-tkhq.

## Quick-reference table for adding a new list-level shortcut

Pick from these free-and-unambiguous keys:

| Tier | Options | Why |
|---|---|---|
| Best (semantic) | `R` (ranked/recipes), `#` (rank/priority), `N` (navigate/next), `Z` (uncommon, safe) | No current binding anywhere scanned; mnemonic attachable |
| Good | `d`, `e`, `m`, `n`, `u`, `v`, `z` | Free on list layer, may collide in other views (safe because views scope keys) |
| Chord | `Ctrl+Letter`, `Alt+Letter` | Requires terminal compat check; abundant space |
| Avoid | `A`, `B`, `D`, `F`, `I`, `J`, `K`, `P`, `Q`, `X`, `Y` | Free today, but low mnemonic value and visually looks like shouting in docs |

---

*Last updated: 2026-04-23 (initial survey).*
