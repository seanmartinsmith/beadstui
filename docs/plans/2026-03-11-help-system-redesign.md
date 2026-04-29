# Help System Redesign Plan (bt-xavk)

## Context

bt has ~50 keyboard shortcuts spread across 7+ view-specific handlers. Session 11 audit found 22 undocumented shortcuts. The current help overlay (single `?` press) dumps everything into a static 4x2 grid - overwhelming for new users, insufficient for power users.

## Design: 3-Layer Progressive Disclosure

Based on Nielsen's "Recognition over Recall" and "Help and Documentation" heuristics.

### Layer 1: Contextual Status Bar Hints (always visible)

**What**: The footer already shows context-specific hints (e.g., `1-4:col  o/c/r:filter  L:labels  /:search  ?:help` in board view). This layer is already implemented but incomplete.

**Work needed**:
- Audit each view's footer hints for completeness
- Add hints for undocumented shortcuts relevant to current view
- Show the 3-5 most useful shortcuts for the active context, not all of them
- Views: board, list, graph, insights, history, tree, flow matrix, label dashboard, attention

**Key principle**: Show what's discoverable from the current state, not everything possible.

### Layer 2: Compact Cheat Sheet (`?` - single press)

**What**: Task-oriented quick reference. Groups shortcuts by what users want to DO, not by category. Fits on one screen for common terminal sizes.

**Layout**:
- Task-oriented headers: "MOVE AROUND", "FIND THINGS", "CHANGE VIEWS", "TAKE ACTION"
- Only the most common shortcuts per group (5-8 per group)
- Current view highlighted or shown first
- Footer: `?? for full reference  |  Esc to close`

**Differences from current**:
- Fewer sections (4 vs 8)
- Task-oriented naming
- Omits view-specific shortcuts (those go in Layer 3)
- Smaller footprint - should fit in 80x24

### Layer 3: Full Reference (`??` - double press, or second `?` while Layer 2 is open)

**What**: Complete reference organized by view. This is roughly what the current help overlay does, but better organized.

**Layout**:
- Tab-style navigation: General | Board | Graph | Insights | History | Tree
- Each tab shows only that view's shortcuts
- Status indicators legend at bottom
- Scrollable if content exceeds terminal height

**Implementation**: Second `?` press while help is showing transitions from Layer 2 to Layer 3.

## Undocumented Shortcuts to Document

From session 11 audit:

| Key | Function | View |
|-----|----------|------|
| `` ` `` | Toggle tutorial | Global |
| F1-F4 | View aliases | Global |
| 0 | Jump to column top | Board |
| $ | Jump to column bottom | Board |
| n/N | Search next/prev match | Board |
| v | Toggle git/bead view | History |
| E | Tree view | Global |
| V | Cass preview | Global |
| U | Self-update | Global |
| Shift+H/L | Scroll graph left/right | Graph |
| PgUp/PgDn | Scroll graph up/down | Graph |
| x | Toggle calc details | Insights |
| m | Toggle heatmap | Insights |
| e | Toggle explanations | Insights |
| y | Copy SHA | History |
| c | Confidence filter | History |
| J/K | Navigate commits | History |
| p | Priority hints | Global |
| t/T | Time travel / Quick time travel | Global |
| O | Open in editor | Global |
| C | Copy to clipboard | Global |
| x | Export markdown | Global |

## Implementation Steps

### Step 1: Refactor help state (model changes)

- Add `helpLayer int` field to Model (0=hidden, 1=cheat sheet, 2=full reference)
- Modify `?` key handler: if helpLayer==0 -> 1, if helpLayer==1 -> 2, if helpLayer==2 -> 0
- Keep `Esc` as universal close
- Deprecate `showHelp bool` in favor of `helpLayer`

### Step 2: Build Layer 2 (compact cheat sheet)

- New `renderHelpCheatSheet()` function
- 4 task-oriented groups, max 8 entries each
- Must fit in 80x24 terminal
- Simple 2-column layout (2 groups per row)

### Step 3: Refactor Layer 3 (full reference)

- Rename current `renderHelpOverlay()` to `renderHelpReference()`
- Add tab navigation (h/l or left/right to switch views)
- Track `helpRefTab int` for active tab
- Each tab renders only relevant shortcuts

### Step 4: Audit and complete Layer 1 (status bar hints)

- Review each view's `renderFooter()` hints
- Add missing shortcuts for each view context
- Prioritize by frequency of use

## Open Questions

- Should Layer 3 use actual Bubble Tea tabs component or just styled text?
- Should `F1` be an alias for `?` (common convention)?
- Should the tutorial (backtick) be mentioned in Layer 2?

## Dependencies

- bt-aog1 (responsive layout) - resolved in this session
- No blocking dependencies

## Sizing

- Step 1: Small (model refactor, ~30 lines)
- Step 2: Medium (new render function, ~100 lines)
- Step 3: Medium-Large (refactor + tabs, ~150 lines)
- Step 4: Small (hint string updates across views, ~50 lines)

Recommended: implement steps 1-2 first for immediate UX improvement, then 3-4 in a follow-up session.
