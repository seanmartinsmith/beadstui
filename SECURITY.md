# Security policy

## Reporting a vulnerability

Please report security issues privately via GitHub's [private vulnerability reporting](https://github.com/seanmartinsmith/beadstui/security/advisories/new). Do not file public issues for suspected vulnerabilities.

A maintainer will acknowledge the report within ~7 days. This is a small project; we don't guarantee a fix timeline, but we'll work with you on coordinated disclosure.

## What's in scope

- Code execution via crafted bead content, labels, or BQL queries fed into `bt`.
- Sandboxing of the browser-open path (`BT_NO_BROWSER` / `BT_TEST_MODE` gates).
- Secret leakage in robot-mode JSON output, logs, or debug paths.
- Arbitrary command execution via `.bt/hooks.yaml` consent flow (see `bt-m8fo`).
- Path traversal or unsafe deserialization in `pkg/loader/`, `pkg/correlation/`, or the Dolt reader path.

## What's out of scope

- Bugs in [beads](https://github.com/gastownhall/beads) itself — please report upstream.
- Bugs in [Dolt](https://github.com/dolthub/dolt) — please report at the dolt repo.
- Go stdlib CVEs that we don't directly trigger via `bt` code paths.
- UX confusion, brittle workflows, or non-security bugs — please file a regular issue or beads task.

## Disclosure preference

Coordinated disclosure with a 90-day default. Lower-severity issues can disclose sooner; higher-severity ones with active exploitation get the same 90-day window unless we agree on something different.

This policy reflects current project state (alpha, pre-1.0). Expect it to harden as bt approaches a stable release.
