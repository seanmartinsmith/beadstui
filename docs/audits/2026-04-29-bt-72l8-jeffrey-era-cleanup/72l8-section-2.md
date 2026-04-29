# 72l8 §2: Documentation + attribution

## What was scanned

- `README.md`
- `CHANGELOG.md` (EXEMPT per task — historical record)
- `AGENTS.md`
- `CLAUDE.md`
- `LICENSE`
- `docs/**/*.md` (full tree)
- `docs/adr/*` (EXEMPT per task — historical record)
- `.beads/conventions/*.md`
- `.beads/conventions/.onboard-state.yaml`

Patterns scanned: `beads_viewer`, `bv-` ref-shape, `Dicklesworthstone`, `Jeffrey` (any casing), `s070681`, `BV_` env prefix, inline URLs to upstream Jeffrey repos.

## Findings

Severity legend: `informational` | `needs-fix` | `leave-as-historical` | `intentional`

### LICENSE / README / AGENTS / CLAUDE

- `LICENSE` — Jeffrey copyright + MIT + Anthropic/OpenAI rider intact. **intentional, correct.**
- `README.md:15,88,90,92,98,103` — multiple Jeffrey/beads_viewer/Dicklesworthstone references in "fork of" prose, full Origin story section, copyright line, and Acknowledgments bullet. **intentional** — user authored README this way as the public attribution surface (consistent with LICENSE preserving Jeffrey's copyright). Tension with the bt-72l8 description sentence "everywhere else should read as beadstui/seanmartinsmith"; flag for parent synthesis but do NOT rewrite without explicit user direction.
- `AGENTS.md:37` — `**Project**: beadstui (fork of beads_viewer, retargeted to upstream beads/Dolt)`. **informational** — accurate factual descriptor for agents reading project context. Could be reworded to "fork of an upstream Charm-stack TUI" but the current phrasing aids agent comprehension. Recommend leave-as-is.
- `CLAUDE.md` — clean, no matches.

### .beads/conventions/

- `.beads/conventions/labels.md:22` — `area:wasm` row description: `bv-graph-wasm/ - Rust WASM graph visualization`. **needs-fix** — `bv-graph-wasm/` is the actual on-disk path of the Rust crate (also referenced in `docs/audit/team-8b-build-config.md:126` as having a stale upstream URL in `Cargo.toml`). This is a path leak rather than attribution leak; the directory itself should probably be renamed but that's out of §2 scope. Rewording to a path-agnostic description is the lighter-weight fix.
- `.beads/conventions/labels.md:80` — example `bd create` description string says `"README still has Jeffrey-era prose, needs full rewrite"`. **needs-fix** — agent-facing example documentation; replace with a generic example that doesn't reference Jeffrey by name.
- `.beads/conventions/.onboard-state.yaml:20` — same `bv-graph-wasm/` reference. Generated file ("do not edit manually"); will regenerate when re-running the onboard skill. **informational**.

### docs/ (non-ADR, non-archive)

- `docs/UPGRADE_LOG.md:3` — header: `**Project:** beads_viewer`. **needs-fix** — user-facing-ish dep upgrade log, easy single-line edit to `beadstui` or `bt`.
- `docs/semantic-search-embedding.md` — entire doc references `bv-9gf` issue id throughout (lines 1, 3, 76, 85), `BV_SEMANTIC_*` env vars (lines 86-88), and `bv` binary name (line 12: "Keep the `bv` binary small"). **needs-fix** — this is a live design doc, not historical. The bead id should be re-resolved (likely `bt-9gf`), env vars should be `BT_SEMANTIC_*`, binary name should be `bt`. Highest-impact single-doc fix in §2.
- `docs/accessor_pattern.md:3` — `**Task Reference:** bv-4jfr`. **needs-fix** — single-line edit to `bt-4jfr` (or look up the post-rename id).
- `docs/audit/cli-ergonomics-audit.md` — extensive `bv-` references (lines 325-329, 333, 400-401, 405, 409, etc.), but each one is **describing pre-existing stale references in source code that the audit is calling out**. Meta: this audit doc itself contains the strings, but quoting them as findings to fix. **leave-as-historical** — rewriting these would corrupt the audit's own claims. Whether the *source code* it references still has those strings is §1's problem, not §2's.
- `docs/audit/2026-04-20-bt-beads-schema-drift.md:23` — passing reference `from beads_viewer`. **informational** — audit doc, narrowly historical context.
- `docs/audit/beads-ecosystem-audit.md:494,497,508` — narrative explanation of feature provenance ("This was Jeffrey's feature for beads_viewer", "Jeffrey created [TOON]"). **informational** — audit document, factual provenance claims, leaving alone.
- `docs/audit/team-2-cli.md:249,303` — `Dicklesworthstone/toon-go` is the actual Go module import path for the TOON encoder dep — that's an upstream-author-owned package bt depends on. **intentional** — accurate dependency reference, not attribution drift.
- `docs/audit/team-8b-build-config.md:69,84,126` — same `Dicklesworthstone/toon-go` dep references PLUS a stale `Cargo.toml` URL at `bv-graph-wasm/Cargo.toml` line 7 pointing to old upstream. **needs-fix (cargo.toml only)** — the audit doc itself can stay, but `bv-graph-wasm/Cargo.toml`'s `repository` field should be updated. Out of §2 scope (this is build/packaging = §3) but flagging here.
- `docs/audit/team-6-agents.md:82` — narrative reference "growth/adoption feature from the original beads_viewer". **informational**.
- `docs/drafts/README-draft.md:15,155,160` — draft README mirrors live README's Jeffrey attribution. **informational** — draft scratch, not user-facing. Will get superseded if/when README rewrites land.
- `docs/design/2026-04-20-bt-mhwy-1-compact-output.md:312` — passing mention of `BV_*` env in fork-history context. **informational**.
- `docs/brainstorms/2026-03-12-post-takeover-roadmap.md:46,129` — brainstorm doc; references "beads_viewer issues" and "Jeffrey's GitHub issues". **leave-as-historical** — brainstorming archive, not user-facing.
- `docs/brainstorms/2026-03-16-dolt-lifecycle-adaptation.md:16` — narrative scaffold reference. **leave-as-historical**.
- `docs/brainstorms/2026-04-09-product-vision-brainstorm.md:103` — brainstorm narrative. **leave-as-historical**.
- `docs/archive/plans/2026-03-05-beads-migration-design.md:8,11,17,18,65` — explicit migration-plan doc that *names* the rename. **leave-as-historical** — the plan describes the rename event itself; rewriting would erase the plan's own context.
- `docs/archive/plans/2026-03-07-beads-migration-session2.md:3,32` — same migration plan, session 2. **leave-as-historical**.
- `docs/archive/plans/2026-03-12-codebase-audit-plan.md:180` — references stale-bv-naming as a checklist item. **leave-as-historical**.
- `docs/archive/plans/2026-03-26-bd-bt-audit-supplement.md:128-129` — narrative "Most were built by Jeffrey Emmanuel for beads_viewer". **leave-as-historical** — supplemental audit analysis. (Note: typo `Emmanuel` vs `Emanuel`.)
- `docs/archive/plans/2026-04-09-refactor-charm-v2-cobra-migration-plan.md:35,53` — "inherited a monolithic architecture from beads_viewer", "Jeffrey's design decisions". **leave-as-historical** — refactor plan needs the historical framing to motivate the work.
- `docs/plans/2026-04-27-bt-cluster-reorg-proposal.md:127`, `docs/plans/2026-04-27-bangout-arc.md:84` — both reference bt-72l8 itself ("Jeffrey-era leftovers epic"). **leave-as-historical** — meta references to this audit.
- `docs/audit/2026-04-27-bt-cluster-map.md:42` — meta reference to this audit. **leave-as-historical**.
- `docs/audit/triage/infra.md:10` — meta reference to this audit. **leave-as-historical**.

### docs/archive/ (everything is archival by directory convention)

- `docs/archive/README.md:3,5,6` — explicit "Historical development artifacts from the beads_viewer era (pre-fork)" header + `jeffrey-era/` subdir reference. **intentional** — user explicitly archived these with the `jeffrey-era/` naming during the takeover (see ADR-001:114). Leave.
- `docs/archive/bv-era/**` (incl. `AGENT_FRIENDLINESS_REPORT.md`, `optimization/*`, etc.) — all pre-fork artifacts under the archive convention. Saturated with `BV_*`, `bv` binary, `beads_viewer`, etc. **intentional / leave-as-historical** — archive dir is the canonical home for this content. (Reorganized 2026-04-29 under bt-llgj: jeffrey-era/ and optimization-research/ merged into bv-era/, loose root files filed under bv-era/.)

### Severity breakdown

- **needs-fix**: 4 distinct surfaces
  1. `.beads/conventions/labels.md:22` (area:wasm description path leak) and :80 (Jeffrey example string)
  2. `docs/UPGRADE_LOG.md:3` (Project header)
  3. `docs/semantic-search-embedding.md` (live design doc, multi-line, `bv-9gf` ids + `BV_SEMANTIC_*` envs + `bv` binary mention)
  4. `docs/accessor_pattern.md:3` (Task Reference id)
- **informational** (leave alone, parent may synthesize): ~10 references in audits/brainstorms/drafts.
- **intentional / leave-as-historical**: README attribution surfaces, LICENSE, archive directory, ADRs, migration plans, dep references to upstream-author-owned modules (`Dicklesworthstone/toon-go`).

## LICENSE verification

**Status: correct as-is.**

`LICENSE` (lines 1-4):
```
MIT License (with OpenAI/Anthropic Rider)

Copyright (c) 2026 Jeffrey Emanuel
Copyright (c) 2026 Sean Martin Smith
```

- Jeffrey Emanuel copyright preserved: ✓
- Sean Martin Smith copyright added: ✓
- MIT permission grant: ✓ (lines 6-12)
- OpenAI/Anthropic rider with Jeffrey as enforcement party (line 28: "express prior written permission of Jeffrey Emanuel"; line 53: "Jeffrey Emanuel may seek injunctive..."): ✓
- Standard MIT warranty disclaimer: ✓ (lines 68-74)

No discrepancies. Aligns with ADR-001 §"License" and user's stated intent that LICENSE preserves Jeffrey's authorship.

## Remediation beads filed

None filed during this audit pass. Per task instructions, remediation beads should be filed only "if a doc has a stale ref that genuinely needs rewriting" — and the four `needs-fix` items are small enough that a single follow-up bead covering all of §2's `needs-fix` set is more efficient than four micro-beads. Recommend the parent (bt-72l8 main thread) decides on bead granularity after seeing all 9 sections' findings.

If a single follow-up bead is desired, suggested shape:
- title: `Docs: clean up stale bv- references in live (non-archive) docs`
- type: task, priority: P3, labels: `area:docs`
- description scope: bullet the four needs-fix surfaces above, "Relates: bt-72l8 §2"

## Notes for parent synthesis

1. **README is the load-bearing attribution decision.** The README's prominent Jeffrey attribution conflicts with the literal reading of bt-72l8's description ("everywhere else should read as beadstui/seanmartinsmith") but aligns with the LICENSE rider's spirit and the user's stated intent in MEMORY.md ("Jeffrey's copyright kept"). Parent should treat the README as deliberate authorial choice, not drift. If it should change, that's a product decision, not a hygiene fix.

2. **`docs/semantic-search-embedding.md` is the highest-value single fix in §2.** It's a live design doc with stale binary name, env var prefix, and bead ids — all of which would mislead a future reader/agent. Worth a one-shot edit pass.

3. **`bv-graph-wasm/` directory name** keeps cropping up (labels.md, onboard-state.yaml, audit team-8b). It's the on-disk crate name. Renaming the directory is a §3 (build) concern but worth flagging since it surfaces in §2 docs.

4. **`Dicklesworthstone/toon-go` references** are the upstream module path for an actual dep — those are correct and shouldn't be touched. Anyone scanning for `Dicklesworthstone` should know to skip these.

5. **No `s070681` references found in user-facing docs** — only one mention in `docs/adr/001` (EXEMPT, historical record of git history rewrite). Clean.

6. **No `BV_` env prefix references in live (non-archive, non-historical) docs** except `docs/semantic-search-embedding.md` (covered as needs-fix #3) and one passing mention in `docs/design/2026-04-20-bt-mhwy-1-compact-output.md` (in fork-history context, leave-as-is).

7. **AGENTS.md line 37** ("fork of beads_viewer") is the only live agent-context doc with a Jeffrey-era reference. Reads as accurate framing rather than drift; leaving it is defensible.
