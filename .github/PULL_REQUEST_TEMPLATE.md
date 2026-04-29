<!--
Thanks for opening a PR!

bt explicitly welcomes AI-assisted contributions - see CONTRIBUTING.md for the
maintainer posture and decision tree. The maintainer prefers fix-merge and
cherry-pick over request-changes, so don't worry if your PR isn't perfect; if
the idea is good, the maintainer will help land it with attribution.

Fill out the sections below. Anything marked optional can be skipped if not
relevant.
-->

## Summary

<!-- One-line description of what this PR changes. Bead ref (`bt-xxxx`) if applicable. -->

## Why

<!-- Brief motivation - what problem does this solve. Link to issue / bead /
discussion if applicable. -->

## What changed

<!-- Bullet list of changes. Files touched at a high level. -->

## Tests

<!-- Unit / e2e / manual / N/A.
For new code paths: tests added, or explicit reason why not. -->

## Hygiene checklist

- [ ] One concern per PR (no unrelated changes)
- [ ] Rebased on `main` (no stale-fork merge commits)
- [ ] Minimal file changes (no swept-in formatter / lint / cleanup)
- [ ] No cross-project pollution (bt doesn't leak into beads or vice versa)
- [ ] Tests pass locally (`go test ./...`)
- [ ] `go vet ./...` clean

## AI assistance (optional disclosure)

<!-- Informational. AI-assisted PRs are welcome; this just helps the reviewer
calibrate. Pick one if you'd like. -->

- [ ] Fully agent-generated
- [ ] Agent-generated, human-edited
- [ ] Human-written
- [ ] Other (describe)

## Notes for the reviewer

<!-- Anything tricky, non-obvious, or worth flagging. Known follow-ups can be
mentioned here - file as beads if they're real work. -->
