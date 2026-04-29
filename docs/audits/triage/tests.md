# Triage: tests

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action |
|----|----|----|----|----|
| bt-5dvl | Fix test suite issues from audit (P1-P3) | GREEN | Windows path overflow + stale name strings + cosmetic ID refs; storage-agnostic. | None. |
| bt-7q26 | pkg/export coverage threshold drift: 68.87% vs 69% | GREEN | Coverage threshold tuning in CI; nothing data-layer about it. | None. |
| bt-ckin | e2e alert-severity drift: TestRobotAlerts_* fail post-bt-46p6.6 | GREEN | Severity rule recalibration; fixture priority fix, not a pre-Dolt invariant. | None. |
| bt-kvk0 | Dolt e2e test infrastructure: test against real Dolt, not just JSONL fixtures | GREEN | Bead's whole purpose is to ADD Dolt-path coverage; explicitly aligned with current architecture. | None. |
| bt-qp1j | Audit test-suite runtime performance + coverage gaps | GREEN | Perf/coverage audit framing; storage-agnostic across pkg/ui, loader, export, hooks. | None. |
| bt-tjq0 | go test output has poor progress visibility | GREEN | Streaming test output UX bug; unrelated to backend. | None. |

## Bucket totals
- GREEN: 6
- YELLOW: 0
- RED: 0

## Notes / cross-bucket observations
- All beads in this bucket are test-infrastructure / test-correctness work that is storage-agnostic. None bake pre-Dolt invariants into assertions; bt-kvk0 is in fact pro-Dolt infrastructure.
- bt-5dvl mentions JSONL only via the audit report it references, not as a test invariant — fine as-is.
- bt-qp1j and bt-tjq0 are partially overlapping (perf + visibility) but both are valid; not a triage concern here.
- bt-ckin's reference to "JSONL fixtures" is implicit only via the existing e2e harness; once bt-kvk0 lands, future tests will use Dolt — but that's evolution, not a stale assumption in this bead.
