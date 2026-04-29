# 72l8 §6: GitHub repo surface

Audit performed 2026-04-29. Read-only. Three remediation beads filed. Repo is in good shape — no security or attribution leakage.

## Auth status

Active account: `seanmartinsmith` (confirmed via `gh auth status`). Secondary `s070681` account also keyring-resident but inactive. All commands run as `seanmartinsmith`.

## Repo metadata

- description: `graph-aware task management TUI for beads projects` ✅ clean
- homepage: empty string ⚠ (filed: bt-73a2)
- topics: null ⚠ (filed: bt-73a2)
- visibility: PUBLIC
- default branch: `main` ✅
- license (API): NOASSERTION (LICENSE file is MIT + rider — GitHub can't auto-detect the rider; this is expected and not actionable)
- archived: false
- fork: false (clean, no fork relationship advertised on GitHub)
- open_graph_image: null (default)

## Webhooks

`gh api repos/seanmartinsmith/beadstui/hooks` → `[]` ✅ none

## Deploy keys

`gh api repos/seanmartinsmith/beadstui/keys` → `[]` ✅ none

## Secrets

- Repo secrets (`gh secret list`): no output ✅ empty
- Actions secrets (`gh api .../actions/secrets`): `{"total_count":0,"secrets":[]}` ✅ empty
- HOMEBREW_TAP_GITHUB_TOKEN: ✅ confirmed gone (per bt-brid 2026-04-23)

## Collaborators

`gh api repos/seanmartinsmith/beadstui/collaborators`:
- `seanmartinsmith` only (admin/maintain/push/triage/pull) ✅ sole maintainer

## Branch protection on main

`gh api repos/seanmartinsmith/beadstui/branches/main/protection` → 404 "Branch not protected".

Solo maintainer, no protection rules. Low risk for now. Not filing a bead — pre-v0.1.0 this is fine. Worth revisiting if external contributors land or if CI gates need enforcement.

## Pinned issues / Discussions / Projects / Wiki

- pinnedIssues (GraphQL): `[]` ✅ none
- hasDiscussionsEnabled: false ✅
- hasIssuesEnabled: true ✅ (keep)
- hasWikiEnabled: **true** ⚠ (filed: bt-7r2m)
- hasProjectsEnabled: **true** ⚠ (filed: bt-7r2m)

Wiki and Projects are GitHub-default-on. Both empty. Surface area without value — this project tracks work in beads/Dolt.

## v0.0.1 release body

Tag: `v0.0.1`, published 2026-04-23 by github-actions[bot]. Five platform tarballs + checksums attached (darwin amd64/arm64, linux amd64/arm64, windows amd64). Assets clean — all named `bt_0.0.1_*`.

**Issue: release body is an auto-generated changelog of all commits, including this entry, surfaced verbatim on the public release page:**

```
454784c9fb4a012e3d93e1fdcadaab6198648234 Initial codebase from Dicklesworthstone/beads_viewer
```

This is the most visible Jeffrey-era leakage in the entire audit — it's on the v0.0.1 release page that any first-time visitor sees. The commit itself is intentional in git history (post-rewrite, marking the fork-takeover boundary), but the release notes should not surface it. Filed as **P2** (higher priority than other §6 findings) because it's the user-facing entry point.

Filed: **bt-4hq9** (P2).

## Findings

| Severity | Finding | Bead |
|---|---|---|
| P2 | v0.0.1 release body surfaces "Initial codebase from Dicklesworthstone/beads_viewer" commit on public release page | bt-4hq9 |
| P3 | Topics not set, homepage empty (discoverability) | bt-73a2 |
| P3 | Wiki + Projects enabled but unused (default-on surface area) | bt-7r2m |
| INFO | No branch protection on main | not filed (solo maintainer, pre-v0.1.0) |
| INFO | License shows NOASSERTION in GitHub API | not filed (LICENSE has MIT + rider; can't auto-detect — expected) |

## Confirmed clean

- Webhooks: 0
- Deploy keys: 0
- Repo secrets: 0
- Actions secrets: 0
- HOMEBREW_TAP_GITHUB_TOKEN: gone (verified)
- Collaborators: sms-only
- Discussions: disabled
- Pinned issues: none
- Fork relationship: not advertised (repo is standalone)
- Default branch: `main` (not `master`)
- Description: clean, beadstui-branded

## Remediation beads filed

- bt-4hq9 — v0.0.1 release body contains 'Initial codebase from Dicklesworthstone/beads_viewer' commit ref — **P2**
- bt-73a2 — Set GitHub repo topics + homepage on seanmartinsmith/beadstui — P3
- bt-7r2m — Disable unused GitHub repo features: Wiki + Projects — P3

All three reference "Relates: bt-72l8 §6".
