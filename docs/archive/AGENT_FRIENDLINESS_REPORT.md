# Agent-Friendliness Report: beads_viewer (bv)

**Bead ID**: bd-c5g (re-underwriting)
**Date**: 2026-01-25
**Agent**: Claude Opus 4.5

## Executive Summary

**Status: EXCEPTIONAL AGENT-FRIENDLINESS MATURITY**

bv is among the most agent-friendly tools in the suite:
- 40+ `--robot-*` flags for comprehensive structured output
- Full JSON output with usage hints and actionable commands
- Comprehensive AGENTS.md documentation (23KB)
- TOON integration planned, not yet implemented

## 1. Current State Assessment

### 1.1 Robot Mode Support

| Feature | Status | Details |
|---------|--------|---------|
| `--robot-*` flags | YES | 40+ specialized robot flags |
| JSON output | YES | All robot commands output JSON |
| `--format` flag | NO | Planned in RESEARCH_FINDINGS.md |
| `BV_OUTPUT_FORMAT` env | NO | Planned |
| `BV_PRETTY_JSON` env | YES | Enable pretty-printed JSON |
| TOON integration | NO | Planned via tru binary |

### 1.2 Robot Command Categories

| Category | Flags |
|----------|-------|
| Triage & Priority | `--robot-triage`, `--robot-next`, `--robot-priority`, `--robot-plan` |
| Graph Analysis | `--robot-insights`, `--robot-graph`, `--robot-metrics` |
| Dependency Analysis | `--robot-blocker-chain`, `--robot-impact-network`, `--robot-causality` |
| File Analysis | `--robot-file-beads`, `--robot-file-hotspots`, `--robot-file-relations`, `--robot-impact` |
| Label Management | `--robot-label-health`, `--robot-label-flow`, `--robot-label-attention` |
| Sprint/Forecast | `--robot-sprint-list`, `--robot-sprint-show`, `--robot-forecast`, `--robot-capacity`, `--robot-burndown` |
| Search & Correlation | `--robot-search`, `--robot-related`, `--robot-orphans`, `--robot-history` |
| Monitoring | `--robot-alerts`, `--robot-drift`, `--robot-diff`, `--robot-suggest` |
| Recipes & Export | `--robot-recipes`, `--agent-brief` |

### 1.3 Output Structure

All robot commands output structured JSON with:
- `generated_at`: Timestamp
- `data_hash`: Content hash for caching
- `triage`: Main data payload
- `usage_hints`: JQ commands for common extractions

### 1.4 Example Output (robot-triage)

```json
{
  "generated_at": "2026-01-25T15:46:17Z",
  "data_hash": "553adcf4b95003cb",
  "triage": {
    "meta": {
      "version": "1.0.0",
      "issue_count": 199,
      "compute_time_ms": 15
    },
    "quick_ref": {
      "open_count": 14,
      "actionable_count": 14,
      "top_picks": [...]
    },
    "recommendations": [...],
    "quick_wins": [...],
    "project_health": {...},
    "commands": {
      "claim_top": "CI=1 bd update bd-xxx --status in_progress --json",
      "show_top": "CI=1 bd show bd-xxx --json"
    }
  },
  "usage_hints": [
    "jq '.triage.quick_ref.top_picks[:3]' - Top 3 picks",
    "jq '.triage.blockers_to_clear | map(.id)' - High-impact blockers"
  ]
}
```

## 2. Documentation Assessment

### 2.1 AGENTS.md

**Status**: EXISTS and comprehensive (23KB)

Contains:
- Rule 1: Absolute file deletion protection
- Go toolchain guidelines
- Graph analysis patterns
- Performance optimization rules
- TUI vs CLI mode documentation

### 2.2 Additional Documentation

- RESEARCH_FINDINGS.md: 10KB TOON integration analysis
- README.md: Comprehensive user guide
- Robot help: `bv --robot-help`

## 3. Scorecard

| Dimension | Score (1-5) | Notes |
|-----------|-------------|-------|
| Documentation | 5 | Comprehensive AGENTS.md + robot-help |
| CLI Ergonomics | 5 | 40+ specialized robot flags |
| Robot Mode | 5 | Exceptional coverage and structure |
| Error Handling | 5 | Structured errors in JSON |
| Consistency | 5 | Unified output envelope |
| Zero-shot Usability | 5 | Usage hints embedded in output |
| **Overall** | **5.0** | Exceptional maturity |

## 4. TOON Integration Status

**Status: PLANNED, NOT YET IMPLEMENTED**

From RESEARCH_FINDINGS.md:
- Plan to use `tru` binary for encoding
- `--format` flag planned
- `BV_OUTPUT_FORMAT` env variable planned
- Expected 50-55% token savings

### Planned Integration

```go
// pkg/robot/format.go
type Format string

const (
    FormatJSON Format = "json"
    FormatTOON Format = "toon"
    FormatAuto Format = "auto"
)

func Encode(payload any, format Format, w io.Writer) error {
    switch format {
    case FormatTOON:
        return encodeToon(payload, w)
    case FormatAuto:
        // Try TOON, fallback to JSON
        ...
    default:
        return json.NewEncoder(w).Encode(payload)
    }
}
```

## 5. Recommendations

### 5.1 High Priority (P1)

None - bv is already exceptionally agent-friendly

### 5.2 Medium Priority (P2)

1. Implement TOON integration (follows RESEARCH_FINDINGS.md plan)
2. Add `--format` flag before TOON (json-only initially)

### 5.3 Low Priority (P3)

1. Add `--robot-schema` flag for JSON Schema emission
2. Document token savings metrics when TOON is implemented

## 6. Agent Usage Patterns

### Get Unified Triage
```bash
bv --robot-triage
```

### Get Top Recommendation
```bash
bv --robot-next
```

### Analyze Dependencies
```bash
bv --robot-blocker-chain bd-xxx
bv --robot-impact-network bd-xxx
```

### Monitor for Changes
```bash
bv --robot-drift
bv --robot-alerts
```

### Export Agent Brief
```bash
bv --agent-brief /path/to/output/
```

## 7. Unique Agent-Friendly Features

1. **Embedded Usage Hints**: JSON output includes JQ commands for common extractions
2. **Actionable Commands**: Output includes ready-to-run commands (claim, show, refresh)
3. **Multi-Agent Support**: `--robot-triage-by-track` for parallel agent coordination
4. **Feedback Loop**: `--feedback-accept` and `--feedback-ignore` for learning

## 8. Conclusion

bv sets the gold standard for agent-friendliness with:
- 40+ specialized robot flags covering all use cases
- Usage hints embedded directly in output
- Actionable commands ready for agent execution
- Exceptional documentation

Score: **5.0/5** - Gold standard for agent-friendliness.

---
*Generated by Claude Opus 4.5 during agent-friendly re-underwriting*
