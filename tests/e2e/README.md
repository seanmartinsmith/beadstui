# E2E Harness

Small, bash-first helpers for scripting `bv` end-to-end checks. The goal is
repeatability and CI-friendly logs without bespoke glue per script.

## Usage

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/harness.sh"

section "build"
run build ./bv --version

section "robot-plan"
run robot_plan ./bv --robot-plan
jq_field "$BV_E2E_LOG_DIR/robot_plan.out" '.plan.tracks | length > 0'

section "robot-insights"
run robot_insights ./bv --robot-insights
jq_field "$BV_E2E_LOG_DIR/robot_insights.out" '.data_hash'
```

Environment:
- `BV_E2E_LOG_DIR` (optional) — log directory (default: `./.e2e-logs`).

Helpers:
- `run <name> <cmd...>` — timestamps, captures stdout/stderr to named files.
- `jq_field <file> <jq expr>` — minimal assertion helper (exits non-zero on failure).
- `section <title>` — log banner.
