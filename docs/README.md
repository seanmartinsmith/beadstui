# docs/

Project documentation. This README is the canonical map of where things go.

If you're an agent or first-time contributor: read this first, then go to whichever subdirectory your work belongs in. Several subdirs have their own deeper README (`archive/`, `audits/`) — go read those when you're working there.

## Layout

| Folder | Purpose | Lifecycle |
|---|---|---|
| `adr/` | Architecture Decision Records — non-reversible decisions affecting multiple work streams. Read [`adr/README.md`](adr/README.md) for the index and reading order. | Permanent. ADRs are never deleted, only marked Superseded. |
| `audits/` | Point-in-time state assessments. Read [`audits/README.md`](audits/README.md) for the bucket layout. | Append-only. Don't edit old audits; run a new one if the data has shifted. |
| `brainstorms/` | Pre-decision exploration — open questions, options considered, alternatives. Output of brainstorming sessions before a plan or ADR is written. | Append-only. Brainstorms freeze at the moment they happen. |
| `design/` | Engineering reference and bead-specific design docs. | Mixed: evergreen guides evolve; dated bead designs freeze. |
| `plans/` | Implementation plans for in-flight or upcoming work. | When a plan is fully executed, it moves to `archive/plans/`. |
| `specs/` | Stable artifact descriptions — what something *is*, evolving in place as the artifact evolves. | Living docs. |
| `screenshots/` | Image assets referenced by the README and other docs. | As needed. |
| `archive/` | Historical artifacts kept for reference — executed plans, retired audits, bv-era documents. Read [`archive/README.md`](archive/README.md). | Read-only. Period accuracy is the value here. |
| `robot/` | `bt robot <subcmd>` API reference — one section per subcommand, output shapes, flags, usage examples. Read [`robot/README.md`](robot/README.md). | Living reference; update when subcommands are added or their output shapes change. |

## Naming conventions

| Subdir | File-naming pattern | Notes |
|---|---|---|
| `adr/` | `<NNN>-<slug>.md` (e.g. `002-stabilize-and-ship.md`) | Numbered. Slug describes the decision, not the topic. |
| `audits/` | Theme buckets + dated filenames inside. See [`audits/README.md`](audits/README.md). | Investigations under `audits/investigations/<YYYY-MM-DD>-<slug>/`. |
| `brainstorms/` | `<YYYY-MM-DD>-<slug>.md` | **Don't add a `-brainstorm` suffix** — the directory name carries the artifact type. |
| `design/` | Two shapes coexist: `<YYYY-MM-DD>-<bead-id>-<slug>.md` for bead-specific designs, `<lowercase-slug>.md` (no date) for evergreen guides. | Dated when a design is tied to a specific bead and frozen at that point; un-dated when the doc is a living reference (e.g., `testing.md`, `performance.md`). |
| `plans/` | `<YYYY-MM-DD>-<slug>.md` | **Don't add a `-plan` suffix** — the directory name carries the artifact type. Sub-flavor suffixes (`-proposal`, `-arc`, `-dispatch`) are fine when the plan isn't a standard implementation plan. |
| `specs/` | `<YYYY-MM-DD>-<slug>.md` | Specs evolve in place — date is when the spec was first written, content updates without renaming. |

## Decision tree: where does my doc go?

```
Is it a non-reversible decision affecting multiple work streams?
  → docs/adr/  (or, if smaller and local, `bd create --type decision`)

Did I just take a snapshot of how something looks right now?
  → docs/audits/  (pick the right bucket — see audits/README.md)

Did I work through options, alternatives, and questions before committing to an approach?
  → docs/brainstorms/

Am I describing a pattern, technique, or shipped behavior other people need to understand?
  → docs/design/  (un-dated lowercase-slug for evergreen, dated for bead-specific)

Am I writing the steps to ship something specific?
  → docs/plans/

Am I describing what a stable artifact is and how it evolves?
  → docs/specs/

Did the work finish and the doc is now historical reference?
  → docs/archive/  (and update any references)
```

## Conventions

- **Dates are when the doc was written**, not when it was filed. If you're writing a brainstorm today about a plan that's coming next week, the date is today's.
- **No emojis in doc filenames or headers** unless the user explicitly asks.
- **Don't proliferate naming variants.** If a doc could go in two places, pick one and add a one-line pointer in the other if discoverability matters. We avoid copies.
- **References across docs use full paths** (`docs/audits/architecture/2026-04-25-data-source-architecture-survey.md`) — ergonomic for grep, robust to dir reshuffles. Relative paths only when the file is referencing a sibling and the relative form is shorter and clearer.
- **When you move a doc, sweep references.** A simple PowerShell or grep pass against tracked `.md` files catches them. Validator script idea: anything matching `docs/(plans|audits|design|archive|brainstorms|specs|adr|screenshots)/[A-Za-z0-9_./\-]+\.md` should resolve to a real file.
- **`AGENTS.md` at repo root** has the *Docs Structure Conventions* table as a quick reference. This README is the deeper version; AGENTS.md is the one-screen scan.

## Inbox / sort discipline

`archive/inbox/` and `audits/inbox/` are holding areas for new artifacts when their final bucket isn't obvious. Drain them when 3+ items accumulate or before session handoff. Don't let the inbox become a permanent home — that's how the original audit-cleanup sessions started.

## When in doubt

Open a brainstorm, file a bead, or ask the user. Filing in the wrong place is recoverable; filing nothing is not.
