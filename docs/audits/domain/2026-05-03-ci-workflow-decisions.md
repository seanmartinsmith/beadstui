# CI Workflow Decision Audit (2026-05-03)

> Recommendation document for two disabled CI workflows. Awaiting sign-off before any destructive or activating action.

## Background

Two workflows inherited from the Jeffrey Emanuel fork (Dicklesworthstone/beads_viewer) were temporarily disabled during the Stream 9 release-engineering audit on 2026-04-23 (commit `78204e5a`). The disable was intentional and scoped: `schedule:` triggers were commented out, `workflow_dispatch:` was preserved for manual exercise during the audit. The decision bead is bt-2q7b; the parent epic covering Jeffrey-era leftovers broadly is bt-72l8.

This document evaluates each workflow against current project state and recommends a disposition.

---

## Workflow 1: fuzz.yml

### What it does

Nightly fuzz testing of `pkg/loader/` JSONL parser. Seven fuzz targets run in sequence, each for 10 minutes (70 min total compute per night, 2-hour hard timeout):

1. `FuzzParseIssues` - full JSONL parsing pipeline via `loader.ParseIssuesWithOptions`
2. `FuzzUnmarshalIssue` - JSON deserialization into `model.Issue`
3. `FuzzValidate` - issue validation logic across field combinations
4. `FuzzValidateTimestamps` - timestamp ordering edge cases
5. `FuzzDependencyParsing` - dependency array handling
6. `FuzzCommentParsing` - comment array handling
7. `FuzzLargeLine` - buffer overflow protection for lines exceeding the 10MB scan buffer

Trigger: `schedule: 0 3 * * *` (nightly 3am UTC, currently commented out) + `workflow_dispatch` (preserved).
Outputs: fuzz result artifacts (30-day retention), crasher artifacts (90-day retention) when panics are detected.

### Current state

Schedule is commented out via `# schedule:` header comment added in `78204e5a`. The workflow file is otherwise intact and functional. `workflow_dispatch` remains active - it can be triggered manually via `gh workflow run` or the GitHub Actions UI.

### Relevance to current bt

`pkg/loader/` is still present and actively used. It is NOT dead code.

Evidence:
- `internal/datasource/load.go` calls `loader.GetBeadsDir`, `loader.FindJSONLPath`, `loader.LoadIssues`, `loader.LoadIssuesFromFile` as fallback paths when Dolt is unavailable or not declared as required.
- `pkg/ui/`, `cmd/bt/`, `pkg/workspace/`, `pkg/search/`, `pkg/analysis/`, and `tests/e2e/` all import `pkg/loader` (23 files across the codebase).
- The `internal/datasource` smart-loader preferentially uses Dolt when metadata declares it (via `isDoltRequired`), but falls through to JSONL for installations that do not have Dolt configured.

The fuzz test file (`pkg/loader/fuzz_test.go`) is complete and well-seeded. It was introduced by Jeffrey in commit `3b0ed003` (January 2026) and hardened with panic recovery in `05b0915f` (January 2026) after a real `goccy/go-json` panic was discovered. That panic recovery commit is direct evidence the fuzz tests have produced actionable findings.

One open bead is related: **bt-x2ap (P3)** - a security finding that `fuzz.yml` interpolates `fuzz_time` directly into `run:` shell blocks (script injection anti-pattern). This is a fix prerequisite if the workflow is re-enabled with write access concerns in mind.

### Recommendation: MODIFY then RE-ENABLE

Rationale:

- `pkg/loader/` is a live, actively-called code path - not a dead relic.
- The fuzz suite has real history: at least one confirmed panic discovery from `goccy/go-json` on malformed input (commit `05b0915f`). This is exactly what fuzz testing is for.
- 70 min/night is non-trivial but not excessive for a solo pre-alpha project where the parser is the input boundary for JSONL-backed installs and export consumers.
- The workflow itself is well-structured: artifacts, crasher retention, per-target logging, report generation.
- One modification is required before re-enabling: fix the `fuzz_time` expression interpolation identified in bt-x2ap. This is a script injection risk even at low threat level (write-access-only trigger).

Optional: reduce `fuzz_time` from `10m` to `3m` per target (21 min/night total) as a pre-alpha cost compromise. This is a judgment call; the bead description flagged this option.

### If modify: change summary

1. Fix bt-x2ap: replace `${{ github.event.inputs.fuzz_time }}` and `${{ steps.fuzz-config.outputs.fuzz_time }}` with env-var indirection in all `run:` blocks.
2. (Optional) Reduce default fuzz_time from `10m` to `3m` in the nightly path.
3. Uncomment `schedule:` trigger.
4. Close bt-x2ap as resolved.

---

## Workflow 2: flake-update.yml

### What it does

Weekly auto-PR bumping Nix flake inputs. Runs every Sunday at midnight UTC (`cron: "0 0 * * 0"`).

Steps:
1. Install Nix via `DeterminateSystems/nix-installer-action`
2. Configure Nix cache
3. Run `nix flake update` and detect if `flake.lock` changed
4. If changed: run `nix build .#bt --no-link` to verify the update doesn't break the build
5. If build passes: open a PR via `peter-evans/create-pull-request` bumping nixpkgs revision

Requires `contents: write` and `pull-requests: write` permissions.

### Current state

Schedule commented out via `# schedule:` header comment added in `78204e5a`. `workflow_dispatch` preserved. The workflow file is otherwise intact.

**Critical finding**: `flake.nix` and `flake.lock` were **deleted from the repository** in commit `0c75785d` (2026-04-29, authored by sms) with the explicit rationale "Nix unused." The commit message reads:

> `flake.nix + flake.lock (Nix unused)`

This was part of a root-level cleanup that also removed `.ubsignore`, `bv_test` binary, `codecov.yml`, and `install.ps1`.

There is no `flake.nix`, `flake.lock`, or `nix/` directory in the current working tree. The workflow references `nix build .#bt` and `nix flake update` against a `flake.lock` that does not exist.

If run today, the workflow would fail at `nix flake metadata` (no `flake.lock` to read) or earlier (no `flake.nix` to define the `#bt` output).

### Relevance to current bt

None. The Nix build surface was explicitly removed by the project owner on 2026-04-29, six days before this audit. The workflow is not just disabled - it is orphaned. There is nothing for it to update.

The bead description mentioned checking whether auto-PRs historically got merged or rotted. That question is moot given the Nix files were deleted. The correct read is: the maintainer already made the Nix decision by removing the files.

### Recommendation: DELETE

Rationale:

- `flake.nix` and `flake.lock` were explicitly deleted from the repo on 2026-04-29. The workflow has no target to operate on.
- Running this workflow (if re-enabled) would fail on every execution - wasting Actions compute with guaranteed failure.
- There is no signal that Nix will be restored. The deletion commit message (`Nix unused`) is unambiguous.
- Keeping a disabled, orphaned workflow file adds confusion and noise to the `.github/workflows/` directory.

### History-preservation note

The workflow was introduced in commit `1adc193f` (`ci(nix): add weekly flake input auto-update workflow`, Jeffrey era). The Nix surface that supports it was introduced in `ceb993a8` (`feat: add Nix flake for reproducible builds and development`, January 2026) and deleted in `0c75785d` (April 2026). Both commits are in git history and remain accessible via `git show <sha>` if the Nix build surface is ever restored in the future.

Adjacent cleanup to consider (separate bead if desired): verify no references to `nix build .#bt` or Nix-specific instructions remain in `README.md`, `CHANGELOG.md`, or docs. The `0c75785d` commit removed the files but did not audit documentation.

---

## Summary

| Workflow | Recommendation | Rationale (one line) |
|---|---|---|
| fuzz.yml | MODIFY then RE-ENABLE | pkg/loader is live code, fuzz tests have found real bugs, fix bt-x2ap script injection first |
| flake-update.yml | DELETE | flake.nix was deleted 2026-04-29; workflow is orphaned and would fail on every run |

---

## Related beads

- **bt-2q7b** (this bead) - decision audit for both workflows
- **bt-72l8** (P1, parent epic) - comprehensive Jeffrey-era leftovers sweep; CI workflows are a subset of Section 3 (Build + packaging)
- **bt-x2ap** (P3) - security fix for fuzz.yml expression interpolation; must be resolved before re-enabling fuzz.yml schedule
- **bt-6cdi** (P2, parent of bt-x2ap) - security audit follow-up epic

---

## Sign-off

- [ ] sms: approve
- [ ] sms: action taken (date + commit ref)
