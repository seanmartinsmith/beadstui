# Wave 1 Regression Hunt - Findings

> Confirmed bugs with full evidence. Eliminated hypotheses live in `eliminated.md`.

Session: 2026-05-03 14:30
Scope: pkg/ui/helpers.go, pkg/ui/helpers_test.go, pkg/analysis/sigils_test.go, tests/e2e/robot_matrix_test.go, scripts/, docs/robot/, docs/audits/domain/, AGENTS.md, README.md, docs/README.md
Iterations: 5 of 15 (truncated - user pivoted to fix phase after iter 5)

---

## FINDING #1 (LOW): README.md feature bullet points to AGENTS.md as the full API

**Location:** `README.md:84`

**Symptom:** The Robot Mode bullet in the features list says: "See [AGENTS.md](AGENTS.md) for the full API." But the canonical full API reference is now `docs/robot/README.md` (created by bt-iigg). AGENTS.md only has a quick-ref table that itself points to docs/robot/README.md.

**Root cause:** bt-iigg updated the dedicated "Robot mode" section (README.md:121-135) but missed line 84 in the features list.

**Reproduction:** Read README.md as a new user; follow the link in the features bullet; land at AGENTS.md; have to do a second hop to find the actual reference doc.

**Suggested fix:** Change `See [AGENTS.md](AGENTS.md) for the full API.` to `See [docs/robot/README.md](docs/robot/README.md) for the full API.`

---

## FINDING #2 (MEDIUM): Broken doc link in docs/robot/README.md

**Location:** `docs/robot/README.md:465`

**Symptom:** The `bt robot bql` section has a Note: "BQL syntax is documented at `docs/design/bql-reference.md`." But that file does not exist - `ls docs/design/bql*` returns "No such file or directory."

**Root cause:** bt-iigg referenced a future doc that hasn't been written yet. The BQL reference doc is filed as bt-01pk (P3 OPEN) and remains unwritten.

**Reproduction:** Click or follow the link from a markdown viewer - 404. Or `cat docs/design/bql-reference.md` from terminal - file not found.

**Suggested fix:** Either remove the broken link, or replace with: `BQL syntax is documented in code; a dedicated reference is tracked in bt-01pk.` Or wait for bt-01pk to ship and revisit. Pragmatic call: replace with bt-01pk pointer now (no 404 in published docs), update again when the actual reference doc lands.

---

## Eliminated hypotheses (5 iterations done)

See `eliminated.md` for full list. Summary:

1. graph.go duplicate `getStatusIcon` - intentional design (different emoji set for compact node rendering); explicit comment says so.
2. tests/e2e/robot_matrix_test.go BV refs - false positive in broad-grep result; file is actually clean.
3. scripts/* BV refs after bt-k6n8 cleanup - false positive; scope-grep shows zero hits.
4. AGENTS.md Robot Mode section drift - clean; iigg added "Full reference" pointer correctly.

---
