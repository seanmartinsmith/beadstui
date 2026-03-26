# bt - Beads TUI

A terminal interface for [Beads](https://github.com/steveyegge/beads), the git-native issue tracker.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), and the rest of the [Charm](https://charm.sh) stack.

> **Status: Pre-alpha.** Actively developed, not yet released. Expect rough edges.

## What is this

Beads is a CLI issue tracker that stores issues alongside your code in git. It moved to [Dolt](https://www.dolthub.com/) as its storage backend starting around v0.56, dropping SQLite and JSONL in the process.

`bt` gives you a keyboard-driven TUI on top of that Dolt backend - board views, detail panels, dependency graphs, and insights, all in your terminal.

This is a fork of Jeffrey Emanuel's [beads_viewer](https://github.com/Dicklesworthstone/beads_viewer), which was built for `beads_rust` (a Rust fork of beads with its own storage format). The takeover retargets everything at upstream beads and its Dolt schema, so `bt` stays compatible as beads evolves.

## Install

Requires Go 1.25+ and a working [beads](https://github.com/steveyegge/beads) installation with Dolt.

```bash
go install github.com/seanmartinsmith/beadstui/cmd/bt@latest
```

Or build from source:

```bash
git clone https://github.com/seanmartinsmith/beadstui.git
cd beadstui
go build ./cmd/bt/
```

## Quick start

```bash
cd your-project       # any directory with beads initialized
bt                    # launches the TUI
```

`bt` auto-starts a Dolt server if one isn't already running (via `bd dolt start`), connects over the MySQL protocol, and polls for changes. When you exit, it shuts down the server if it started one.

## Views

- **List** - issue list with detail panel, the default view
- **Board** (`b`) - kanban-style columns by status
- **Graph** (`g`) - dependency DAG visualization
- **Insights** (`i`) - PageRank, critical path, cycle detection
- **History** (`h`) - issue timeline correlated with git commits

## Key bindings

| Key | Action |
|-----|--------|
| `j`/`k` or arrows | Navigate |
| `Enter` | Expand/collapse detail |
| `b` | Board view |
| `g` | Graph view |
| `i` | Insights |
| `h` | History |
| `l` | List view |
| `/` | Search |
| `f` | Filter by status |
| `p` | Filter by priority |
| `t` | Filter by type |
| `?` | Help |
| `q` | Quit |

## Configuration

`bt` looks for config in three places (later overrides earlier):

1. Built-in defaults
2. `~/.config/bt/theme.yaml` - user-level theme
3. `.bt/theme.yaml` - project-level theme

Environment variables for Dolt connection:

| Variable | Default | Description |
|----------|---------|-------------|
| `BEADS_DOLT_SERVER_PORT` | - | Port override (highest priority) |
| `BT_DOLT_PORT` | - | Port override |
| `BT_DOLT_POLL_INTERVAL_S` | `5` | Seconds between polls |
| `BT_FRESHNESS_STALE_S` | `120` | Seconds before data shows as stale |
| `BT_FRESHNESS_WARN_S` | `30` | Seconds before stale warning |

## Origin story

[Steve Yegge](https://github.com/steveyegge) built [beads](https://github.com/steveyegge/beads) - a git-native issue tracker in Go, originally backed by SQLite and JSONL. [Jeffrey Emanuel](https://github.com/Dicklesworthstone) built [beads_viewer](https://github.com/Dicklesworthstone/beads_viewer) as a TUI companion for it.

In early 2026, Steve began migrating beads to Dolt as its storage backend. By v0.50 (Feb 2026), SQLite and JSONL were removed entirely - Dolt became the only path. Jeffrey's tooling was built around the classic architecture, so rather than follow the migration, he forked beads into [beads_rust](https://github.com/Dicklesworthstone/beads_rust) - a Rust rewrite that freezes the SQLite + JSONL model. Steve endorsed the fork. beads_viewer pivoted to support beads_rust.

`bt` goes the other direction. I forked beads_viewer and retargeted it at upstream beads and its Dolt backend, so the TUI stays compatible as beads evolves. The Dolt integration, cross-platform test suite, theme system, and ongoing UI work is the fork. The TUI architecture, graph algorithms, and view system underneath - that's Jeffrey's foundation.

## License

MIT License with OpenAI/Anthropic Rider. See [LICENSE](LICENSE).

Copyright (c) 2026 Jeffrey Emanuel
Copyright (c) 2026 Sean Martin Smith

## Acknowledgments

- [Jeffrey Emanuel](https://github.com/Dicklesworthstone) for building beads_viewer
- [Steve Yegge](https://github.com/steveyegge) for beads
- [Charm](https://charm.sh) for the terminal UI ecosystem
