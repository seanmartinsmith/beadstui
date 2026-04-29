# 72l8 §5: Git metadata + history

Audit date: 2026-04-29
Scope: read-only verification of Jeffrey-era leftovers in git metadata on the local beadstui repo.

## Authors on main

`git log main --format='%an <%ae>' | sort | uniq -c`:

```
    311 seanmartinsmith <114885497+seanmartinsmith@users.noreply.github.com>
      3 sms <114885497+seanmartinsmith@users.noreply.github.com>
```

Both names route to the same noreply email (`114885497+seanmartinsmith@users.noreply.github.com`), so attribution on GitHub is uniform under the seanmartinsmith account. The `sms` short-name is cosmetic (3 commits, same identity). No Jeffrey, Dicklesworthstone, Emanuel, or s070681 authors on main. History rewrite from auto-memory still holds.

Cross-check: `git log main --format='%H %an' | grep -iE 'jeff|dicklesworthstone|s070681|emanuel'` returns zero hits.

Note: `git log --all` does surface Jeffrey Emanuel / Dicklesworthstone / s070681 authors, but those commits are reachable only from `remotes/upstream/*` refs (Dicklesworthstone/beads_viewer), which is the intentional read-only reference remote. They are not on any local branch and not on `origin/*`.

## Remotes

```
origin   git@github.com:seanmartinsmith/beadstui.git (fetch)
origin   git@github.com:seanmartinsmith/beadstui.git (push)
upstream https://github.com/Dicklesworthstone/beads_viewer.git (fetch)
upstream https://github.com/Dicklesworthstone/beads_viewer.git (push)
```

`origin` correct. `upstream` correct and intentional per ADR / scope guards (read-only reference). Push URL on `upstream` matches fetch URL — no protective change needed since pushes to Dicklesworthstone would be rejected anyway (no auth), but a defensive `git remote set-url --push upstream no_push` could be added if the maintainer wants belt-and-suspenders. Not in scope here.

## .gitignore findings

Read full file. One bv-era reference, already handled cleanly:

```
# Legacy bv config (pre-rename) - .bt/ is the active pattern above
.bv/
```

This is a deliberate compatibility comment-explained ignore for any pre-rename `.bv/` directory that might still exist on a maintainer machine. It is not a leftover bug — it's a documented retention. No remediation needed.

No other bv/Jeffrey/Dicklesworthstone references in .gitignore. All paths reference `.bt/`, `bt`, `beadstui_*`, etc.

## .gitattributes findings

```
# Use bd merge for beads JSONL files
.beads/beads.jsonl merge=beads
```

Single line, beads-aware merge driver registration. No bv-era patterns. Clean.

## Branches

Local branches:
- `main` (default)
- `fix/robot-stdout-corruption`
- `phase-0.5/test-foundation`
- `phase-0/charm-v2-mechanical`
- `worktree-agent-*` (~10 transient worktree branches)

Remote-tracking:
- `origin/HEAD -> origin/main`
- `origin/main`
- `origin/feat/robot-list`
- `upstream/HEAD -> upstream/main`
- `upstream/main`
- `upstream/master`  ← Jeffrey's old default
- `upstream/beads-sync`

No local `master` branch. The `upstream/master` ref exists because Dicklesworthstone/beads_viewer kept a `master` branch upstream — that's their repo's choice, not ours, and since `upstream` is read-only reference there is nothing to act on. Worth noting in the audit but not actionable on the beadstui side.

The `worktree-agent-*` proliferation is unrelated to the §5 audit but flagged here as a hygiene observation — those are transient and likely should get pruned periodically (separate concern, not Jeffrey-era).

## Conclusion

**Clean.** No remediation beads needed for §5.

Specifically:
- main author history: 100% seanmartinsmith identity
- remotes: correct and intentional
- .gitignore: bv-era reference is the documented compatibility line, not a bug
- .gitattributes: clean
- branches: no local `master`; the upstream `master` ref is upstream's branch, out of scope

## Remediation beads filed

None. Section is clean per the scope of bt-72l8 §5.
