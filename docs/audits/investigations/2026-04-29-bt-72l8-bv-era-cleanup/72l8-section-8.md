# 72l8 §8: Dolt records text scan

## Search method

Read-only audit, no writes. Methodology:

1. `bd search "bv-"` / `bd search "beads_viewer"` / `bd search "Dicklesworthstone"` / `bd search "Jeffrey"` / `bd search "s070681"` — bd's text search.
2. `bd list --status=closed --json --limit 0`, `--status=open`, `--status=in_progress` — full JSON dumps under `.bt/tmp/bd_{closed,open,inprogress}.json`.
3. ripgrep `bv-[a-z0-9]+` and `beads_viewer|Dicklesworthstone|s070681` against the JSON dumps for descriptions, titles, acceptance criteria, etc.
4. `bd comments <id>` looped across all 200 open + in_progress IDs, grepped for the same patterns.
5. `bd dep list` confirmed no `external:bv-*` dep strings exist (the command rejects no-arg, and a grep for `external:bv-` across all dump files returned zero).

Token shape used: `bv-[a-z0-9]+` (catches both Jeffrey-era 4-char hex IDs like `bv-156` and longer slug forms like `bv-graph-wasm`).

## Total matches

- **Open beads**: 4 description hits + 0 unique comment hits in non-audit beads
- **In-progress beads**: 0 hits
- **Closed beads**: 89 description-line hits across 101 unique `bv-*` tokens — all expected historical record per scope guard
- **Comments on open/in-progress beads**: 7 lines all within audit-context beads (`bt-72l8`, `bt-t82t`) — intentional recon notes, not stale refs

## Stale references in OPEN beads

| Bead | Field | Snippet | Classification |
|---|---|---|---|
| `bt-72l8` | description | `bv-xxxx`, "bv-era leftovers" | **Historical/intentional** — this IS the audit epic; placeholders and the phrase "bv-era" are the audit's own framing. Leave. |
| `bt-72l8.1` | description | "Jeffrey's bv-156 commit message said it explicitly: 'Full sprint CRUD requires bd CLI changes.'" | **Historical/intentional** — quoting Jeffrey's pre-rename commit by ID. Leave. |
| `bt-gkda` | description | "whatever survived the bv-graph-wasm/ → ??? rename" | **Mildly stale** — references old directory name in a Dependencies block on a current open epic. Author already flagged uncertainty (`→ ???`). Worth replacing with the actual path or removing the parenthetical when bt-gkda is picked up. Low urgency. |
| `bt-4ew7` | description | `m → heatmap toggle (bv-95)` | **Stale** — parenthetical Jeffrey-era bead ID in a key-bindings table. Source bead doesn't exist in bt; the (bv-95) provides no value to a future reader. Worth deleting the parenthetical (or replacing with a bt- equivalent if one exists, but per bt-t82t recon no `bv- → bt-` mapping survives). |

Comment-level: `bt-72l8` and `bt-t82t` comments contain bv- references but those are intentional audit/recon findings about the codebase, not stale refs in their host beads.

## Historical references in CLOSED beads

89 hits across 101 unique `bv-*` tokens (e.g., `bv-156`, `bv-graph`, `bv-95`, `bv-agent`, `bv-pages`, `bv-abc123`). All within close_reason / description text of closed beads documenting the bv→bt migration, prior commit history, or prior fork architecture. No action — explicitly preserved by bt-72l8 scope guards as historical record.

## Cross-project dep strings

Zero `external:bv-*` references found in any dependency field, dep string, or `bd dep list` output. Cross-project dep machinery has no leftover bv-prefix references.

## Recommended remediation

Two genuine stale references in active product beads warrant a small rewrite-in-place pass:

1. **bt-4ew7** description — strip the `(bv-95)` parenthetical from the key-bindings table.
2. **bt-gkda** description — Dependencies bullet for `bt-b23i` references `bv-graph-wasm/` directory; replace with the current directory name (or remove the parenthetical) when bt-gkda is next touched.

Single rollup remediation bead recommended (P3, area:data) listing these two.

`bt-72l8` and `bt-72l8.1` references are part of the audit's own historical framing and should not be rewritten.
