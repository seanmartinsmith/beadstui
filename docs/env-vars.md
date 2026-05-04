# Environment Variables

Single reference for every env var `bt` reads at runtime, plus the beads-side vars that materially affect bt's behavior. Defaults verified against current code.

> **Scope.** Only env vars users or agents can usefully set. Internal-only flags (test fixtures, build flags, golden-regen toggles like `GENERATE_GOLDEN`, `PERF_TEST`, `BT_TEST_STATE_DIR`, `BT_TEST_GH_PAGES_E2E`, `UPDATE_BASELINE`, `SKIP_NETWORK_TESTS`) are out of scope. So are vendored-library knobs (`TEA_DEBUG`, `FSNOTIFY_DEBUG`, `GLAMOUR_STYLE`, `EDITOR`, `VISUAL`, `TERM`, `CI`, `GITHUB_TOKEN`, `GH_TOKEN`).

## Dolt connection

Resolution order for the Dolt port: `BEADS_DOLT_SERVER_PORT` (highest, beads-native) > `BT_DOLT_PORT` (bt override) > `.beads/dolt-server.port` file > config defaults. `BT_GLOBAL_DOLT_PORT` is independent — it overrides the global-mode shared-server lookup at `~/.beads/shared-server/dolt-server.port`.

| Variable | Default | Type | Purpose | Read at | Common use |
|---|---|---|---|---|---|
| `BEADS_DOLT_SERVER_PORT` | (unset) | port | Beads-native Dolt port. Highest priority across all overrides. | `internal/datasource/metadata.go` (`buildDSNFromBeadsDir`) | Connect bt to a non-default Dolt instance configured for bd. |
| `BEADS_DOLT_SERVER_HOST` | `127.0.0.1` | string | Beads-native Dolt host. | bd reads this; bt inherits via DSN derivation. | Remote Dolt server. |
| `BEADS_DOLT_SERVER_USER` | `root` | string | Beads-native Dolt user. | bd reads this; bt inherits. | Custom Dolt auth. |
| `BT_DOLT_PORT` | (unset) | port | bt-specific Dolt port override. Overridden by `BEADS_DOLT_SERVER_PORT` when both are set. | `internal/datasource/metadata.go` (`buildDSNFromBeadsDir`) | Testing or non-standard local setups without touching the beads-native env. |
| `BT_GLOBAL_DOLT_PORT` | (unset) | port | Global-mode Dolt port. Overrides `~/.beads/shared-server/dolt-server.port`. | `internal/datasource/global_dolt.go` (`DiscoverSharedServer`) | Force `--global` mode at a specific port (e.g. forwarded port, custom shared server). |
| `BT_DOLT_POLL_INTERVAL_S` | `5` | int seconds | Base poll interval for Dolt change detection in the background worker. Exponential backoff applies on errors (capped at 2 minutes). | `pkg/ui/background_worker.go` (`startDoltPollLoop`) | Raise on slow systems, lower for live-feel during heavy editing. |
| `BEADS_DIR` | `.beads/` (or repo root) | path | Override beads-data directory auto-discovery. | `pkg/loader/loader.go` (`GetBeadsDir`), `internal/datasource/source.go` | Run bt against a non-standard beads layout. |

## Freshness / data staleness

Both vars are seconds, both gate the TUI's freshness indicator. Warn fires before stale.

| Variable | Default | Type | Purpose | Read at |
|---|---|---|---|---|
| `BT_FRESHNESS_WARN_S` | `30` | int seconds | Seconds since last successful fetch before the freshness indicator turns yellow. | `pkg/ui/model.go` (`freshnessWarnDuration`) |
| `BT_FRESHNESS_STALE_S` | `120` | int seconds | Seconds since last successful fetch before the freshness indicator turns red ("stale"). | `pkg/ui/model.go` (`freshnessStaleDuration`) |
| `BT_STALE_DAYS` | `14` | int days | Open-issue staleness threshold (in days since last update) for TUI highlighting. Distinct from data freshness above. | `pkg/ui/helpers.go` (`staleDays`) |

## Output shape and format

Most have a CLI flag that takes precedence (`--format`, `--shape`, `--compact`, `--full`, `--schema`, `--sigils`).

| Variable | Default | Type | Purpose | Read at |
|---|---|---|---|---|
| `BT_OUTPUT_FORMAT` | `json` | string (`json`/`toon`) | Default output format for `bt robot`. CLI `--format` overrides. | `cmd/bt/robot_output.go` (`resolveRobotOutputFormat`) |
| `BT_OUTPUT_SHAPE` | `compact` | string (`compact`/`full`) | Default output shape for compact-aware robot subcommands. CLI `--shape`/`--compact`/`--full` override. | `cmd/bt/robot_compact_flag.go` |
| `BT_OUTPUT_SCHEMA` | `v2` | string (`v1`/`v2`) | Default projection schema on `bt robot pairs` and `bt robot refs`. CLI `--schema` overrides. | `cmd/bt/robot_schema_flag.go` |
| `BT_SIGIL_MODE` | `strict` | string (`strict`/`verb`/`permissive`) | Default sigil-recognition mode on `bt robot refs --schema=v2`. CLI `--sigils` overrides. | `cmd/bt/robot_sigils_flag.go` |
| `BT_PRETTY_JSON` | `0` | bool (`1`) | Indent JSON output for human readability. Disabled by default for performance. | `cmd/bt/robot_output.go` (`newJSONRobotEncoder`) |
| `BT_ROBOT` | `0` | bool (`1`) | Force robot mode: clean stdout (suppresses TUI/log noise), JSON-shaped logs, robot-only code paths. Set automatically by `bt robot` subcommands. | `cmd/bt/root.go`, many call sites |
| `TOON_DEFAULT_FORMAT` | (unset) | string | Fallback format if `BT_OUTPUT_FORMAT` is unset. Read after `BT_OUTPUT_FORMAT`. | `cmd/bt/robot_output.go` |
| `TOON_STATS` | `0` | bool (`1`) | Print JSON-vs-TOON token estimates to stderr (vendored TOON encoder behavior; bt also reads it for `--stats`-style output). | `cmd/bt/root.go` |
| `TOON_KEY_FOLDING` | (encoder default) | string | TOON key folding mode passthrough. | `cmd/bt/robot_output.go` |
| `TOON_INDENT` | (encoder default) | int (`0`-`16`) | TOON indentation level. Clamped to `[0, 16]`. | `cmd/bt/robot_output.go` |

## Test / safety

Browser opening is gated by either flag — both treat any non-empty value as "on".

| Variable | Default | Type | Purpose | Read at |
|---|---|---|---|---|
| `BT_NO_BROWSER` | (unset) | bool (any non-empty) | Suppress browser-opening side effects. Set in tests, headless environments, and when running over SSH without `BROWSER` configured. | `pkg/export/github.go`, `pkg/export/cloudflare.go`, `pkg/ui/model_export.go` |
| `BT_TEST_MODE` | (unset) | bool (any non-empty) | Enable test-mode guards: fail-fast in global-mode Dolt discovery, suppress browser, suppress event persistence, suppress TTY queries, etc. Set automatically by all bt test main packages. | `internal/datasource/global_dolt.go`, `cmd/bt/root.go`, `pkg/ui/background_worker.go`, `pkg/ui/model.go`, `pkg/agents/tty_guard.go`, `pkg/export/*` |
| `BT_NO_EVENT_PERSIST` | `0` | bool (`1`) | Disable persisting bd-event records emitted from the TUI. Useful for read-only inspection or test isolation. | `pkg/ui/model.go` |
| `BT_NO_SAVED_CONFIG` | (unset) | bool (any non-empty) | Skip loading the saved export-wizard configuration. Forces fresh prompts. | `pkg/export/wizard.go` |

## Performance / robot / observability

| Variable | Default | Type | Purpose | Read at |
|---|---|---|---|---|
| `BT_INSIGHTS_MAP_LIMIT` | (unlimited) | int | Per-map size limit in `bt robot insights` output. Reduces payload size for very large graphs. | `cmd/bt/robot_analysis.go`, `pkg/analysis/status_fullstats_test.go` |
| `BT_TEMPORAL_CACHE_TTL` | `1h` | duration (`30m`, `2h`, ...) | Cache TTL for the temporal analysis snapshot store. | `pkg/analysis/temporal.go` (`DefaultTemporalCacheConfig`) |
| `BT_TEMPORAL_MAX_SNAPSHOTS` | `30` | int | Maximum snapshots retained by the temporal analyzer. | `pkg/analysis/temporal.go` |
| `BT_CACHE_DIR` | `<user-cache-dir>/bt-robot-analysis-cache` | path | Base directory for the robot-mode analysis cache (active when `BT_ROBOT=1`). | `pkg/analysis/cache.go` (`robotAnalysisDiskCachePath`) |
| `BT_DEBUG` | (unset) | bool (any non-empty) | Enable debug logging to stderr with `[BT_DEBUG]` prefix and microsecond timestamps. | `pkg/debug/debug.go` |
| `BT_METRICS` | `1` (enabled) | bool (`0` to disable) | Disable internal timing-metric collection. Default is enabled. | `pkg/metrics/timing.go` |
| `BT_DEBOUNCE_MS` | `200` | int milliseconds | Debounce window for filesystem-change events in the background worker. | `pkg/ui/background_worker.go` |
| `BT_CHANNEL_BUFFER` | `8` | int | Background-worker message-channel buffer size. | `pkg/ui/background_worker.go` |
| `BT_HEARTBEAT_INTERVAL_S` | `5` | int seconds | Worker heartbeat interval. | `pkg/ui/background_worker.go` |
| `BT_WATCHDOG_INTERVAL_S` | `10` | int seconds | Worker watchdog interval. | `pkg/ui/background_worker.go` |
| `BT_MAX_LINE_SIZE_MB` | (parser default) | int megabytes | Override max line size when parsing JSONL beads files. | `pkg/ui/background_worker.go` (`envMaxLineSizeBytes`) |
| `BT_WORKER_LOG_LEVEL` | `info` | string (`debug`/`info`/`warn`/`error`) | Background-worker log verbosity. | `pkg/ui/background_worker.go` |
| `BT_WORKER_TRACE` | (unset) | path | Write a background-worker trace log to this path. Empty disables. | `pkg/ui/background_worker.go` |
| `BT_WORKER_METRICS` | `0` | bool (`1`) | Emit background-worker metrics. | `pkg/ui/background_worker.go` |
| `BT_FORCE_POLLING` / `BT_FORCE_POLL` | `0` | bool (`1`/`true`/`yes`/`y`/`on`) | Force the watcher to use polling instead of native filesystem events. Useful on remote/network filesystems where inotify/ReadDirectoryChangesW is unreliable. Either name works. | `pkg/watcher/watcher.go` |
| `BT_TUI_AUTOCLOSE_MS` | (unset) | int milliseconds | Auto-close the TUI after N ms. Used by tests and demos. | `cmd/bt/root.go` |
| `BT_BACKGROUND_MODE` | (unset) | bool (`1`) | **Internal.** Set by bt itself to `1` when running in background/daemon mode. Reading it is fine; setting it manually is unsupported. | `cmd/bt/root.go`, `pkg/ui/model.go` |
| `BT_ANCHOR_PROJECT` | (unset) | path | Override the persisted `anchor_project` setting (cold-boot fallback for the shared Dolt server when cwd is not a beads project). | `internal/settings/global.go` |

## Semantic / hybrid search

The search subsystem can be reconfigured at runtime via these vars (or the corresponding flags on `bt robot search`).

| Variable | Default | Type | Purpose | Read at |
|---|---|---|---|---|
| `BT_SEARCH_MODE` | `text` | string (`text`/`hybrid`) | Search ranking mode. | `pkg/search/config.go` |
| `BT_SEARCH_PRESET` | (unset) | string | Hybrid-search preset name. CLI `--preset` overrides. | `pkg/search/config.go` |
| `BT_SEARCH_WEIGHTS` | (unset) | JSON string | Hybrid-search weights as JSON. CLI `--weights` overrides. | `pkg/search/config.go` |
| `BT_SEMANTIC_EMBEDDER` | `hash` | string | Embedding provider for semantic search. | `pkg/search/config.go`, `pkg/search/embedder.go` |
| `BT_SEMANTIC_MODEL` | (provider default) | string | Embedding model identifier (provider-specific). | `pkg/search/config.go` |
| `BT_SEMANTIC_DIM` | (provider default) | int | Embedding vector dimension. | `pkg/search/config.go` |

## Beads-side (read by `bd`, not by bt)

bt shells out to `bd` for some workflows; these env vars affect `bd`'s behavior and therefore what bt sees indirectly. Not read by bt itself.

| Variable | Default | Type | Purpose | Read at |
|---|---|---|---|---|
| `BEADS_ACTOR` | (git `user.name` -> `$USER`) | string | Audit-trail actor name on bd writes. Primary upstream env. See [beads docs](https://github.com/gastownhall/beads). | `bd` (upstream) |
| `BD_ACTOR` | (unset) | string | Deprecated fallback for actor. Still honored by bd as a fallback when `BEADS_ACTOR` is unset. New setups should use `BEADS_ACTOR`. | `bd` (upstream) |

## Build-time (informational)

Documented for completeness — these are not read at runtime.

| Variable | Default | Type | Purpose |
|---|---|---|---|
| `BT_BUILD_HYBRID_WASM` | (unset) | bool (any non-empty) | Build flag: require `wasm-pack` when building the hybrid-search WASM module. Read in `cmd/bt/pages.go` build-time path. |

## See also

- `bt robot help env` emits the same map as machine-readable JSON (the canonical in-binary source is `cmd/bt/robot_graph.go`'s `envVars` map).
- For beads-CLI env vars beyond what bt indirectly cares about, see the upstream [beads docs](https://github.com/gastownhall/beads).
