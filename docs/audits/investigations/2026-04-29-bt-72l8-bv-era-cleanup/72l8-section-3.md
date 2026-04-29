# 72l8 §3: Build + packaging

Audit date: 2026-04-29
Methodology: read-only `git grep` + full reads of suspect files. No edits, no deletions.

## What was scanned

Root build/packaging artifacts:
- `install.sh` (bash, *nix)
- `install.ps1` (PowerShell, Windows)
- `flake.nix`
- `flake.lock`
- `scripts/**` (13 entries: 11 .sh + 1 .js + 1 .go)

Skipped per scope:
- `.goreleaser.yaml` (already audited via bt-brid, closed 2026-04-23)
- `Makefile` — does not exist
- `Dockerfile` — does not exist

## Findings per file

### install.sh

**Status: STALE / known Jeffrey-era leftover.**

Identity-clean on the surface (REPO_OWNER="seanmartinsmith", BIN_NAME="bt", repo URL points at `seanmartinsmith/beadstui`, no `beads_viewer` / `Dicklesworthstone` / `s070681` / `Jeffrey` strings, no `BV_` env vars). Search for those tokens returned zero hits.

However, the script as-is is **functionally redundant**:

- `bt-brid` already stripped Homebrew tap publishing from `.goreleaser.yaml` (closed 2026-04-23). install.sh still tells users `brew install seanmartinsmith/tap/bt` on lines 588 and 600 — a tap that doesn't exist. **This is a user-facing lie.**
- `try_binary_install()` fetches from GitHub releases, which goreleaser still produces. That path works.
- `try_go_install()` clones the repo and runs `go build`. `go install github.com/seanmartinsmith/beadstui/cmd/bt@latest` accomplishes the same thing in one line. The shell script is a 600-line replacement for a documented one-liner.
- It carries macOS-pkg Go bootstrap logic, Homebrew Go install/upgrade flow, sudo prompts, Python release-asset parsing — substantial surface area for a script whose entire job is "download a binary or `go install`."
- `install.ps1` (Windows counterpart) is much simpler and just does `go install`.

**Recommendation: DELETE install.sh entirely.** Replace with a one-line README instruction: `go install github.com/seanmartinsmith/beadstui/cmd/bt@latest`. Reasoning:

1. Pre-built binaries are still produced by goreleaser; users who want them can grab them from the GitHub Releases page directly (no install script needed for a `tar -xzf` + `mv`).
2. `go install` works for everyone with Go 1.25+ (which is required to build from source anyway, since CGO_ENABLED=0 is already the default).
3. install.ps1 already takes this approach on Windows; `*nix` should mirror it.
4. The `brew install seanmartinsmith/tap/bt` lines are actively misleading users today.
5. Maintaining 600 lines of bash for an install path you don't actually need is a perpetual stale-reference risk (the brew tap reference is exhibit A — it's been broken since bt-brid closed six days ago).

Per project rule (no file deletion without explicit permission), filing a P2 bead recommending deletion. Do not delete inline.

### flake.nix

**Status: CLEAN.**

- `homepage = "https://github.com/seanmartinsmith/beadstui"` — correct
- `pname = "bt"`, `version = "0.0.1"`, ldflag points at `github.com/seanmartinsmith/beadstui/pkg/version.version` — correct
- `description = "bt - Terminal UI for the Beads issue tracker"` — correct
- `mainProgram = "bt"` — correct
- No `beads_viewer` / `Dicklesworthstone` / `s070681` / `Jeffrey` strings
- `vendorHash = null` is a correctness issue (Nix builds will fail to fetch deps deterministically without a real hash) but **out of scope for this audit** — flag only as a side note.

### flake.lock

**Status: CLEAN.**

Only references `nixpkgs` (NixOS/nixpkgs, unstable channel) and `flake-utils` (numtide/flake-utils). No fork/upstream references. Hashes pin to legitimate upstream nixpkgs and flake-utils repos.

### Makefile

**Status: N/A.** No Makefile in the repo. Not a finding — bt is `go build`-driven, no make wrapper needed.

### scripts/

**Status: PERVASIVE bv-era leftovers.** Twelve scripts under `scripts/` were retained verbatim from the beads_viewer era. They reference `bv` as the binary name (not `bt`), build `./cmd/bt/` into an output named `bv`, name temp dirs and log files with `bv-*` / `bv_*` prefixes, and use `BV` in comments and headers. None reference `beads_viewer` / `Dicklesworthstone` / `s070681` / `Jeffrey` directly, so they're identity-clean — but functionally they look like another tool's scripts.

Inventory of bv-era scripts:

| Script | Issue |
|---|---|
| `capture_baseline.sh` | Header comment "for bv"; emits `=== BV Performance Baseline ===` to baseline file |
| `e2e_hybrid_search.sh` | Invokes `bv --search ...` directly (binary name wrong) |
| `e2e_web_hybrid_scoring.js` | `execFileSync('bv', args, ...)`; tmp dir prefix `bv-web-hybrid-` |
| `test_datasource_e2e.sh` | Header "bv datasource", builds `go build -o bv ./cmd/bt/`, log file `/tmp/bv_datasource_test_*.log` |
| `test_toon_e2e.sh` | "BV TOON E2E", builds `go build -o bv`, prose throughout references bv |
| `verify_isomorphic.sh` | Variable names `bv_baseline`, `bv_current` for binary paths |
| `tests/e2e/harness.sh` | Entire variable namespace `BV_E2E_*` (LOG_DIR, LOG_LEVEL, RUN_ID, TEST_NAME, PASS_COUNT, FAIL_COUNT, etc.) — adjacent to scripts/ but technically under tests/, surfaced because the same scripts source it |
| `tests/e2e/harness_test.sh` | Same `BV_E2E_*` namespace |

This is **not in scope to fix during the audit**, but it's a substantial finding. The scripts are either dead (no one runs `bv` anymore — the binary is `bt`), broken if invoked (they'd fail to find `bv` on PATH), or work by accident (when the user has a `bv` symlink lying around). Filing a P2 bead to triage script-by-script: rename to `bt`, retire as superseded by `go test ./...` / `go build`, or delete as dead weight.

Note also that `BV_E2E_*` env var prefix violates the bt project memory rule "no BV_ fallback" — env vars under that prefix are a contract surface for users running e2e harnesses. Should be `BT_E2E_*`.

### Dockerfile

**Status: N/A.** No Dockerfile in the repo. Not a finding.

## Out-of-scope notes (raised, not actioned)

- `vendor/github.com/Dicklesworthstone/toon-go/` is a third-party Go dependency owned by a GitHub user named "Dicklesworthstone" — this is **not** the same actor as Jeffrey-from-beads_viewer, just a name collision in vendored code. Out of scope for §3 (it's vendor/, not build/packaging), but worth surfacing: any `git grep Dicklesworthstone` will hit it and look like a leftover when it isn't. Audit acceptance criterion in bt-72l8 ("zero hits outside CHANGELOG / ADR-002") will need a vendor/ exclusion.
- `flake.nix` `vendorHash = null` is a Nix correctness issue, not a fork-leftover issue. Out of scope here; would belong under a "make Nix builds reproducible" bead if/when bt commits to Nix as a supported install path.

## Remediation beads filed

- **bt-d63v** — Delete install.sh (superseded by `go install`); fix stale `brew install seanmartinsmith/tap/bt` user-facing message (P2, area:infra)
- **bt-csmr** — Triage scripts/ bv-era leftovers: rename `bv` references to `bt`, fix `BV_E2E_*` env vars in tests/e2e/harness.sh, retire dead scripts (P2, area:infra)

## Notes

Read-only audit complete. No files edited. No files deleted. Two remediation beads filed; both reference "Relates: bt-72l8 §3". install.sh recommendation per the bead's special-case prompt: **delete entirely** — superseded by `go install`, and currently surfaces a broken brew-tap pointer to users.
