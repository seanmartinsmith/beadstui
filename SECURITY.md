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

## Hook execution trust model

bt's hook system (`.bt/hooks.yaml`) executes shell commands declared by the
project being processed. To prevent RCE via cloned repositories (the
git-config-style attack vector), bt refuses to execute hooks unless the
hooks.yaml file is registered in the user's trust database.

**Trust binding:** `(absolute path of hooks.yaml, SHA256 of hooks.yaml contents)`.
Editing hooks.yaml resets trust. Moving the project to a new path resets trust.

**Trust DB location:** `~/.bt/hook-trust.json`, mode 0600 on POSIX. On Windows
the file inherits the user-profile ACL.

**Default behavior:** export commands (`bt export md`, `bt export pages`,
`bt --export-md`) refuse to execute hooks unless the hooks.yaml file is trusted.
Refusal exits with code 78 (config error) and prints a remediation message to
stderr pointing at `bt hooks trust <path>`.

**Bypass:** `--allow-hooks` flag on export commands skips the trust check.
Use only in trusted CI environments where the hooks.yaml is part of the
controlled pipeline. There is no environment-variable equivalent by design;
opting out of the safe default must be explicit at the call site.

**Granting trust:** `bt hooks trust [path]`. Inspect first with
`bt hooks list [path]`. Both default to `./.bt/hooks.yaml` when no path is
given.

**Pre-run announcement:** before each hook executes, bt prints
`bt: would run hook '<phase>': <command>` to stderr regardless of trust state.
This is defense-in-depth observability so you can see what is about to run
even when the file is trusted.

**Known gap:** trust covers `hooks.yaml` content only. If a hook command
references an external script (e.g., `command: "./scripts/build.sh"`), that
script's contents are NOT covered by the hash. The script can be replaced
post-trust. A future phase may extend the model to cover transitively
referenced scripts; for now, treat hooks.yaml as the trust unit and review
external scripts manually.

**Out of scope for current implementation:**
- Sigstore/cosign signing of hooks.yaml (future hardening).
- Per-hook execution timeouts (separate finding tracked elsewhere).
- A `bt hooks distrust` subcommand. Until added, edit `~/.bt/hook-trust.json`
  directly to remove an entry.
