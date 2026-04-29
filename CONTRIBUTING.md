# Contributing to bt

Thanks for being here. bt is a small project, but it's open to outside contributions and aims to make them easy to land.

## Welcome (the stance)

**AI-assisted contributions are explicitly welcome** - whether you're using a coding agent, an IDE copilot, or a fully autonomous workflow. The maintainer (Sean Martin Smith) optimizes for getting good PRs across the line, not for filtering them out. This project takes the "vibe maintainer" posture described by Steve Yegge: rather than reject PRs that need work, the maintainer will fix-merge / merge-fix / cherry-pick / split-merge them with attribution to you.

There's no "human-only" sneak rule here. If your PR is good, it ships - regardless of who or what wrote it.

**Disclosure norm**: if your PR is fully agent-generated, mention it in the description. It doesn't gate acceptance; it helps reviewers calibrate. The PR template has an optional checkbox for this.

## Local development

You'll need:

- **Go 1.25+** - verify with `go version`
- **`bd` (beads CLI)** - install from https://github.com/gastownhall/beads, verify with `bd version`
- **A working `.beads/` directory** in the project - bt clones come with one; `bd init` creates one if you're starting from a fresh repo

Optional:

- Set `BD_ACTOR=<your-handle>` in your shell profile so beads attributes your work correctly (per AGENTS.md convention)

A full install + Dolt setup walkthrough is tracked in **bt-lims** and will land before or with v0.1.0.

## Build, test, vet

```bash
go build ./cmd/bt/      # build the binary
go test ./...           # run all tests
go test ./... -race     # with race detector
go vet ./...            # static analysis
go install ./cmd/bt/    # install bt to your $GOPATH/bin
```

For test patterns, fixtures, golden files, e2e structure, and coverage thresholds, see [`docs/design/testing.md`](docs/design/testing.md).

## PR hygiene rules

These are lightweight, not strictly enforced (yet), but they make the difference between a PR that lands fast and one that needs back-and-forth.

- **One concern per PR.** Split unrelated changes into separate PRs.
- **No drafts.** Mark a PR ready-for-review or don't open it. Drafts get closed.
- **Rebase on `main` before submitting.** Don't open against a stale fork; the project moves quickly and merge commits from drift create churn.
- **Minimal file changes.** Don't sweep formatter / lint / unrelated cleanup into a feature PR. If you spot drive-by issues, mention them - file a bead, don't bundle the fix.
- **No cross-project pollution.** bt is a *consumer* of beads/Dolt; bt must not leak into beads, and beads concepts shouldn't be reimplemented inside bt. If your change blurs the line, flag it before opening the PR.
- **Plugins / extensions / robot subcommands over core.** If a feature can ship as configuration, an extension, or a `bt robot <subcmd>` rather than core code, prefer that path.
- **Tests for code paths.** New code paths get tests; e2e changes get e2e tests; doc-only changes don't need tests.

## Commit format

```
type(scope): description (bt-xxx)
```

- `type` - `feat`, `fix`, `docs`, `chore`, `test`, `refactor`, `perf`
- `scope` - maps to `area:*` labels in [`.beads/conventions/labels.md`](.beads/conventions/labels.md): `cli`, `tui`, `data`, `bql`, `analysis`, `search`, `export`, `correlation`, `infra`, `wasm`, `docs`, `tests`
- `(bt-xxx)` - bead reference when the commit addresses a tracked issue

## Issue tracking via beads

bt tracks issues in [beads](https://github.com/gastownhall/beads), a git-native issue tracker backed by Dolt. The `.beads/` directory at repo root is the database.

Useful commands once you have `bd` installed:

```bash
bd ready                       # find unblocked work to claim
bd show bt-xxxx                # read a specific bead
bd search "query"              # full-text search
bd list --type=decision        # see prior architecture decisions
```

For new feature requests, **prefer opening a bead first** (or commenting on an existing one) before opening a PR. This lets the maintainer flag scope or design concerns before code exists, which saves your time.

Conventions for creating quality beads live at [`.beads/conventions/reference.md`](.beads/conventions/reference.md) and [`.beads/conventions/labels.md`](.beads/conventions/labels.md).

## What happens after you open a PR

When a PR lands, the maintainer evaluates it against the following decision tree (adapted from Steve Yegge's vibe-maintainer post):

| Outcome | Meaning |
|---|---|
| **Easy-win** | Small, well-tested, no controversy → merge as-is |
| **Merge-fix** | Mergeable now, tiny follow-up fix needed → merge + maintainer pushes the fix to main |
| **Fix-merge** | Good idea, broken execution → maintainer pulls locally, fixes, pushes with **your attribution preserved** in the commit |
| **Cherry-pick** | PR has multiple changes, only some are wanted → maintainer cherry-picks the wanted parts with attribution; closes PR with explanation |
| **Split-merge** | Multi-concern PR that should have been multiple PRs → maintainer splits + commits each with attribution |
| **Reimplement** | Right problem, wrong design → maintainer designs differently, closes PR with thanks + pointer to the new commit |
| **Reject** | Out of scope, too niche, or doesn't pay tech-debt weight → close with a polite explanation |
| **Request changes** | *Last resort* - only used when none of the above fit |

In all merge-derived paths (fix-merge / cherry-pick / split-merge), **attribution stays with you** in the commit message. Your name on the work, even if the maintainer rewrote some of it.

## Where to ask

- Open a [GitHub Discussion](https://github.com/seanmartinsmith/beadstui/discussions) (or Issue if Discussions isn't enabled yet)
- Dolt Discord (link will be pinned in the repo description after v0.1.0)

## License

bt is MIT licensed with an OpenAI/Anthropic Rider. By contributing, you agree your contribution is licensed under the same terms.

---

**Reference**: the maintainer posture for this repo is documented in `bd decision list` (search for "OSS-maintainer posture"). The decision is grounded in Steve Yegge's "Vibe Maintainer" post: https://steve-yegge.medium.com/vibe-maintainer-a2273a841040
