# Triage: tui-2

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action |
|----|----|----|----|----|
| bt-d8ty | Scroll acceleration / speed ramp on main issues list | GREEN | Pure Bubble Tea update-loop behavior; storage-agnostic. | None. |
| bt-dcby | Global mode: features don't respect activeRepos project filter | GREEN | TUI filter wiring against `m.issues`; no JSONL/sprint/correlator assumption. | None. |
| bt-dp41 | Project Filter modal: bg bleeds through to the right on narrow layouts | GREEN | Pure lipgloss/OverlayCenter rendering bug; storage-agnostic. | None. |
| bt-dx7k | Help overlay broken/unusable at small terminal sizes | GREEN | Pure responsive-layout TUI bug. | None. |
| bt-e30n | Labels: better visual display + explore as navigation/filter mechanism | GREEN | TUI styling + filter UX; reads labels already on issues. | None. |
| bt-eiec | Board view cards need redesign - labels should do more heavy lifting | GREEN | Card rendering/labels-as-chips; storage-agnostic. | None. |
| bt-ezk8 | History view doesn't work in global mode (no git repo context) | GREEN | Acceptance is storage-agnostic ("functional or graceful indicator"); incidental "store paths in beads metadata?" is just an exploratory note, not load-bearing. | None. |
| bt-fbx6 | Recipes: no mouse support in picker modal | GREEN | Pure TUI mouse wiring. | None. |
| bt-foit | TUI: undocumented <> pane-resize keybinds + label column alignment | GREEN | Help-surface docs + list-delegate rendering fix; storage-agnostic. | None. |
| bt-gf3d | [epic] Keybinding consistency overhaul | GREEN | Pure TUI architecture / dispatch; storage-agnostic. | None. |
| bt-i20z | Alerts modal: ? key opens dedicated help modal explaining alert types | GREEN | Modal UX feature; consumes static drift alertTypeDefinitions. | None. |
| bt-if3w.1 | Extract sprint view as standalone component | YELLOW | Sprint feature itself is bt-only and pending the bt-z5jj rebuild-or-retire decision; extracting now risks rework if sprint is retired. | Append corrective comment grounding in bt-z5jj outcome; sequence after that decision. |
| bt-iu56 | Prompt management panel in bt | GREEN | Local prompt library/runner; .bt/prompts/ scope, no beads-storage tie-in. | None. |
| bt-k8rk | h/H key behavior is buggy and confusing | GREEN | Keybinding audit/fix; storage-agnostic. | None. |
| bt-km6d | Mouse support for project filter modal (ModalRepoPicker) | GREEN | Pure TUI mouse wiring on existing modal. | None. |
| bt-ks0w | Mouse click support: panel focus + issue selection | GREEN | Bubble Tea mouse click wiring; storage-agnostic. | None. |
| bt-lgbz | Card expand broken when empty columns are visible | GREEN | Pure lipgloss layout bug in board view. | None. |
| bt-lin9 | Shortcuts bar (;): layout broken, overlaps with underlying view | GREEN | Pure overlay/modal rendering fix. | None. |
| bt-lwdy | Bug: project filter (activeRepos) reset by Dolt poll refresh in global mode | GREEN | Already grounded in current Dolt poll architecture; mirrors closed bt-nzsy pattern. | None. |
| bt-mbjg | Default-hide type=gate beads from list view; surface via explicit filter | GREEN | TUI filter predicate on issue.Type; consistent with current beads schema (gate is upstream type). | None. |
| bt-menk | Project Filter modal border renders broken when Details panel visible | GREEN | Pure lipgloss border/Place rendering bug. | None. |
| bt-nb7o | Recipes: underexposed - need better discoverability and onboarding | GREEN | TUI discoverability/UX; storage-agnostic. | None. |
| bt-npnh | History view broken at smaller terminal dimensions | GREEN | Pure responsive-layout / scroll-offset TUI work in pkg/ui/history.go. | None. |
| bt-nyjj | History view: red 'git log failed' error in global mode / non-git cwd | GREEN | UX/error-framing fix; defers structural multi-repo correlator work to bt-3ltq (GREEN landmark) and bt-08sh. | None. |

## Bucket totals
- GREEN: 23
- YELLOW: 1
- RED: 0

## Notes / cross-bucket observations

- TUI bucket is overwhelmingly storage-agnostic, as expected. The only non-GREEN is **bt-if3w.1**, which is a child task of the sprint-view decomposition. Its fate is coupled to **bt-z5jj** (sprint feature decision: rebuild against Dolt or retire). If retired, this extraction work is wasted; if rebuilt, the data interface for the extracted component will need to be designed against the Dolt-era source-of-truth, not whatever the current sprint_view assumes.
- Several history-view beads (**bt-ezk8**, **bt-nyjj**, **bt-npnh**) cluster around the same gap: history view is per-cwd-git-repo only and has no global-mode awareness. They are individually GREEN (UX framing / responsive layout), but collectively they're the surface symptoms of the correlator/multi-repo gap that **bt-08sh** + **bt-3ltq** own at the data layer. Worth coordinating sequencing so the band-aids don't conflict with the structural fix.
- Modal-rendering bugs (**bt-dp41**, **bt-menk**, **bt-lin9**) all point at the same OverlayCenter vs lipgloss.Place inconsistency. They're triaged independently here, but a single fix to the modal composition path probably resolves all three.
- Mouse-support beads (**bt-fbx6**, **bt-km6d**, **bt-ks0w**) share a chrome-measurement / bubblezone design question; classified GREEN individually but the design spike is shared.
