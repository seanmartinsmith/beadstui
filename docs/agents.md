# bt for Agents

<!-- Related: bt-5l3zq, bt-sdg2k, bt-ah53 -->

> **bt is the agent-consumption layer for bead data.** The surfaces below are stable APIs for agent consumers. Read this page first if you're an agent or AI tool that needs to query, watch, or correlate bead state across one or more beads projects.

This page is the canonical agent entry point. It frames the existing bt surfaces (`bt tail`, `bt robot activity`, `bt robot search`, `bt robot list`, `bt robot bql`) as a coherent toolkit — not a feature catalog. If your reach-for is "I need to ask the bead graph about X," start here.

bd remains the authority for *writing* bead state (claim, close, update, comment). bt is the authority for *reading* it.

## Use-case → surface

| Use case | Surface |
|---|---|
| "Has a bead been filed for X?" (project scope) | `bd search "X"` |
| "Has a bead been filed for X?" (cross-project) | `bt robot search "X" --source <prefix>` |
| "What changed today / this week?" | `bt robot activity --today` / `--this-week` |
| "Tell me when bead `bt-X` changes." | `bt tail --bead bt-X --robot-format jsonl` |
| "Watch a dynamic set of beads as they evolve." | `bt tail --bql "<query>" --robot-format jsonl` |
| "Filter beads by structured criteria right now." | `bt robot list --bql "<query>"` or `bt robot bql --query "<query>"` |
| "Take a snapshot at a point in time." | any subcommand + `--as-of <ref>` |
| "Find cross-project paired beads or refs." | `bt robot pairs --global` / `bt robot refs --global` |
| "Long-running follow with replay on connect." | `bt tail --since <ref>` (exit with `--idle-exit <duration>` when needed) |
| "Triage: what should I work on next?" | `bt robot triage` or `bt robot next` |
| "What's the dependency graph around this bead?" | `bt robot impact-network <id>` |

If your use case isn't on this table, the answer is probably `bt robot --help`. If it still isn't, file a bead — the missing surface is the bug.

## Reading the envelope

Every `bt robot` subcommand wraps its payload in the same envelope:

```jsonc
{
  "generated_at": "2026-05-11T19:08:53Z",   // RFC3339 timestamp
  "data_hash": "1fde1a72427292ab",          // payload fingerprint (cache key)
  "output_format": "json",                  // or "toon"
  "version": "0.0.1",                       // bt binary version
  "schema": "compact.v1",                   // present only for non-default projections
  "scope": {                                // bt-sdg2k: what these counts cover
    "mode": "cross-project",                // or "project" or "workspace"
    "databases": ["bd", "bt", "..."],       // present only when mode=cross-project
    "project_filter": "bt",                 // present when --source / --repo applied
    "workspace": "...",                     // present when --workspace applied
    "as_of": "<sha>"                        // present when --as-of applied
  },
  // ... subcommand-specific payload ...
}
```

**Read `.scope.mode` first.** It's the authoritative answer to "are these counts cross-project or project-local?" — agents that infer scope from cwd or invocation flags will be wrong some of the time. Agents that read `.scope` will be right every time.

**Use `.data_hash` as a cache key.** Re-running the same subcommand with the same inputs against the same dataset produces the same `data_hash`. Identical hash = identical payload, by construction.

## I/O contract

Every `bt robot` subcommand guarantees:

- **stdout**: a single JSON object (or TOON document with `--format=toon`). No log lines, banners, or status text. Safe to pipe into `jq`.
- **stderr**: empty on the success path. Errors are prefixed `Error:`.
- **exit code**: `0` on success, non-zero on any failure.

Enforced by `tests/e2e/robot_io_contract_test.go`. See [robot/README.md](robot/README.md) for the per-subcommand reference.

## Wire-format stability

Output shapes follow a strict add-only rule: **fields may be added, never renamed or removed.** Wire-versioned surfaces carry a `schema` or `schema_version` marker so agents can detect breaking changes.

| Surface | Marker | Tier | Notes |
|---|---|---|---|
| `bt robot activity` | `schema: "activity.v1"` on envelope | 1 | Versioned envelope, stable contract. |
| `bt robot search` | `schema: "search.v1"` on envelope | 1 | Versioned envelope. |
| `bt robot pairs` / `refs` | `schema: "pair.v1"` / `"ref.v1"` on envelope | 1 | v1 today; v2 reader rolling out. |
| `bt robot list/triage/diff` (compact) | `schema: "compact.v1"` on envelope | 1 | Default shape. `--full` drops the schema field and emits the full payload. |
| `bt tail --robot-format jsonl/json` | `schema_version: "tail.v1"` per event | 1 | Per-event marker; the stream itself has no envelope. |
| `bt tail --robot-format human/compact` | n/a | 3 | Human-facing; agents should prefer jsonl. |
| `bt robot history` and other correlator paths | n/a | 2 | Shape-stable but no schema marker yet (tracked under bt-govlj). |

Tier 1 surfaces are the safe default for agent automation. Tier 2 surfaces work today but should not be relied on for long-lived integrations without checking back. Tier 3 surfaces are human-oriented; building on them is an explicit choice to accept future drift.

## Patterns

### Discovery and orientation

A fresh agent landing in a workspace can orient in two commands:

```sh
bt robot triage --shape compact         # what's actionable, blockers, project health
bt robot --help                         # full subcommand list with per-command help
```

Both inherit the cross-project default. `bt robot triage --source <prefix>` narrows when the agent's work is project-scoped.

### Following a single bead

```sh
bt tail --bead bt-X --robot-format jsonl --idle-exit 5m
```

Emits one JSON event per state change (status, comment, dep edit, etc.). `--idle-exit 5m` is the recommended pattern for finite agent invocations; omit for daemon-style follows.

### Watching a dynamic set

```sh
bt tail --bql "status = open and label in ('area:ux','area:tui')" --robot-format jsonl
```

The BQL set is re-evaluated on each event, so beads entering/leaving the predicate appear/disappear correctly.

### Snapshotting state

Every robot subcommand accepts `--as-of <ref>` where `<ref>` is a git revision (branch, tag, SHA, or relative ref like `HEAD~30`). The envelope echoes the resolved SHA in `scope.as_of` so the snapshot is self-documenting.

```sh
bt robot triage --as-of HEAD~7   # what triage looked like a week ago
bt robot diff --since HEAD~7     # what changed between then and now
```

## Discoverability hooks

- `bt robot --help` documents the I/O contract and references this page.
- [`docs/robot/README.md`](robot/README.md) is the per-subcommand schema reference.
- Project [`AGENTS.md`](../AGENTS.md) references this page in its "bt Robot Mode (for agents)" section.

When working in a *consumer* of bt (another beads project, dotfiles, sym, etc.), the project's own AGENTS.md should reference this file so agents reach for the right primitive first.
