# Theme System Audit (pkg/ui)

Date: 2026-05-03
Bead: bt-pxbc
Scope: every site in `pkg/ui/*.go` (non-test) that produces a hex color value.

> Note on path: bt-pxbc requested `docs/audit/theme-system-audit.md`. Project
> convention (AGENTS.md / docs/audits/README.md) places audits at
> `docs/audits/<bucket>/<YYYY-MM-DD>-<slug>.md`. Filed in `architecture/`
> because this is a state snapshot of how the theme stack is wired today.

## Layer hierarchy

The theme stack has three intentional layers:

1. **YAML defaults** — `pkg/ui/defaults/theme.yaml`. The source-of-truth a
   user edits to retune the UI. Embedded into the binary via
   `//go:embed` in `theme_loader.go`.
2. **Go-fallback constants** — `theme.go:DefaultTheme()`, `styles.go:resolveColors()`,
   `theme_loader.go:globalColorDefaults` and `theme_loader.go:themeColorDefaults`.
   Mirror the YAML defaults so the UI still themes correctly if the YAML
   layer is unreadable (embed failure, future build configurations that
   strip embeds, tests that bypass loader). Documented inline; see Step 3.
3. **Hardcoded hex literals in render code** — bypass both layers. These
   are the bug. Editing the YAML doesn't propagate; user-picked accents
   end up half-applied. Step 2 of bt-pxbc redirects these.

## Site classification

Counts: 211 hex literals across 17 non-test files in `pkg/ui/`. Of those:

- **YAML-driven via Go-fallback**: ~150 sites (theme.go + styles.go + theme_loader.go default tables, two hex values per `resolveColor` call). KEEP.
- **Hardcoded — replaced**: 35 hex literals across 14 lines in 7 files (8 if you count the theme_loader.go alignment edit which removed no literals but fixed a divergent token reference). See "Replacements" table.
- **Hardcoded — skipped (curated palette / scope-limited)**: 26 hex literals across 4 files. See "Skipped" table.

### Replaced (Step 2)

| Site | Was | Becomes | Notes |
|---|---|---|---|
| `theme.go:167` | `ThemeFg("#cc6666")` | `ColorDanger` | Matches `theme_loader.go:378` parallel path. Brings divergent paths into alignment. |
| `theme.go:168` | `ThemeFg("#8abeb7")` | `ColorPrimary` | Same as above; `theme_loader.go:379` already uses `ColorSuccess` here, but per bead spec this is `t.PriorityDownArrow` and the intent is the primary accent (teal), not green. Replaced both `theme.go` and `theme_loader.go` with `ColorPrimary` to match. |
| `theme.go:170` | `ThemeFg("#b5bd68")` | `ColorSuccess` | Already aligned with loader path. |
| `theme.go:171` | `ThemeFg("#969896")` | `ColorSecondary` | Already aligned with loader path. |
| `model_footer.go:734` | `ThemeBg("#8abeb7")` | `ColorPrimary` | Workspace badge background. Was dark-mode only; now retones in both modes. |
| `model_view.go:435-440` | `resolveColor(...)` x6 | `ColorPrimary, ColorInfo, ColorSuccess, ColorWarning, ColorTypeEpic, ColorTypeTask` | Help-overlay panel gradient. Hex values are exact dupes of the corresponding semantic tokens. |
| `tutorial_content.go:22-27` | `resolveColor(...)` x6 | `ColorStatusOpen, ColorStatusInProgress, ColorStatusBlocked, ColorStatusClosed, ColorPrimary, ColorTypeFeature` | Tutorial flow-diagram colors; tracking semantic tokens means tutorial recolors with the user's theme. |
| `bql_modal.go:143` | `lipgloss.Color("#ff5555")` | `ColorDanger` | BQL error text. `#ff5555` was an off-palette red close to but not exactly Danger; aligning to Danger. |
| `bql_modal.go:149` | `lipgloss.Color("#666666")` | `ColorMuted` | BQL hint text. `#666666` was off-palette gray. |
| `velocity_comparison.go:271,273,277,288` | `ThemeFg("#b5bd68"/#cc6666/#de935f/#81a2be")` | `ColorSuccess, ColorDanger, ColorWarning, ColorInfo` | Trend indicator colors. Was dark-side only; now respects light mode. |

### Skipped (curated palette — flagged for follow-up)

| Site | Rationale |
|---|---|
| `visuals.go:82-89` (`HeatmapGradientColors`) | 8-step ordered gradient picked for perceptual progression (bg → subtle → highlight → blue → teal → yellow → orange → red). Replacing with semantic tokens would change ordering and break the gradient meaning. The palette is conceptually a separate token family (`heatmap.0` through `heatmap.7`) that should arguably exist in YAML — not in scope for this bead per AMBIGUOUS-CASE rule. |
| `visuals.go:116-129` (`GetHeatGradientColorBg`) | Same palette concept as above but for bg/fg pairs. Curated for legibility (dark text on light bg, etc.). Same follow-up. |
| `visuals.go:136-143` (`repoColorPairs`) | Per-repo distinguishing palette. Hash-keyed, ordering matters for stable repo-to-color mapping. Some entries duplicate semantic tokens (Red, Teal, Blue, Green, etc.) but the mapping is by index, not by semantic role. Same "should be its own token family" follow-up. |
| `board.go:1297` (`resolveColor("#e4dce8", "#2a1e30")`) | Search-match background. Light side `#e4dce8` matches `ColorStatusReviewBg` exactly; dark side `#2a1e30` is darker than `ColorStatusReviewBg` (`#261e2e`). Either an intentional emphasis variant or a drift. Skipping per AMBIGUOUS rule. |
| `markdown.go:189,191` (`docFg = "#c5c8c6"/"#4d4d4c"`) | Glamour markdown doc text. Function takes `isDark` as a parameter (to render in either mode regardless of current terminal), so needs both light AND dark hex available. `ColorText` is already resolved to one side, and the YAML schema has no per-side accessor. Treated as Go-fallback; documented inline to keep in sync with theme.yaml `text:`. Follow-up: a `themeYamlValue("text", "light"/"dark")` accessor would eliminate the duplication. |
| `markdown.go:447` (`return "#000000"`) | Nil-color safety fallback inside `extractHex`. Not a theme color; a guard against returning an empty string when given a nil `color.Color`. KEEP as defensive Go-fallback. |
| `theme.go:169` / `theme_loader.go:380` (`TriageStar = ThemeFg("#f0c674")`) | No `ColorTriageStar` token; closest semantic is yellow (`ColorPrioMedium` = `#eab700`/`#f0c674`). Light-side values differ. SKIP — adding a `TriageStar` semantic token is a separate decision (would need YAML schema bump, out of scope per bead "DO NOT add new theme tokens"). |

### Go-fallback (KEEP, documented)

The following are intentional duplication to keep the app themable when the
embedded YAML can't be loaded. NOT a bug. See header comments added in
Step 3 to `theme.go` and `theme_loader.go`.

- `theme.go:115-145` — `DefaultTheme()` body
- `theme.go:156` — `Header` background hex pair
- `styles.go:121-196` — `resolveColors()` body
- `theme_loader.go:95-149` — `globalColorDefaults` map
- `theme_loader.go:288-311` — `themeColorDefaults` map

## Open questions / follow-ups (not in scope for bt-pxbc)

1. **Heatmap palette as theme tokens** — visuals.go has two heatmap palettes
   and a repo-distinguishing palette that all bypass YAML. Adding
   `heatmap`, `heatmap_bg`, and `repos` token families to the YAML schema
   would let users retone these. ~3 new map types in `ThemeColors`.
2. **`TriageStar` semantic token** — `t.TriageStar` is the only Theme
   field with no Color* counterpart and no YAML key. Either promote it
   (add `triage_star` to YAML) or alias it to `ColorPrioMedium`.
3. **board.go:1297 search-match bg** — confirm `#2a1e30` vs `#261e2e` is
   intentional drift, then either align to `ColorStatusReviewBg` or
   introduce a `ColorSearchMatchBg` token.
4. **`markdown.go:447` nil fallback** — could return `lipgloss.NoColor{}`
   or `ColorText` instead of `"#000000"`. Low impact; leave for now.
