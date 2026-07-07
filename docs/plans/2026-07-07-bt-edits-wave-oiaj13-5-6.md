# Writable-TUI edits wave: bt-oiaj.13 → bt-oiaj.5 → bt-oiaj.6

<!-- Related: bt-oiaj.13, bt-oiaj.5, bt-oiaj.6, bt-88qn, bt-2pk38, bt-55n3s, bt-tkhq -->

**Plan type**: implementation. **Target executor**: one Sonnet session, slices
sequential A→B→C on one branch (`feat/bt-edits-wave`), one commit per slice
(each mergeable alone), draft PR at the end. **Authored**: 2026-07-06
(session pc:bt:5f472206, Fable dispatcher) from two recon briefs over the full
bead corpus + docs + code. Every design fork below is RESOLVED by citation to a
ratified decision — execute, don't re-litigate. If the codebase contradicts an
anchor in this plan, PAUSE AND FLAG (comment on the owning bead), don't improvise.

## Orientation (read these first, in order)

1. `docs/design/write-routing.md` — Consumers contract (binding on every write).
2. `docs/design/tui-bead-edit-patterns.md` — Pattern C canon + list.Model keymap gotchas.
3. `docs/design/tui-modal-compositing.md` — the 3-step modal recipe (recited below; follow it literally).
4. `pkg/ui/claim.go` — the reference write implementation you are generalizing.
5. `bd show bt-88qn bt-oiaj.13 bt-oiaj.5 bt-oiaj.6 bt-55n3s` (read all comments).

## Prior-art ledger (what binds, and what is DEAD TEXT)

**Binding:**
- **bt-88qn (LOCKED 2026-05-06)**: "A+C hybrid" — Pattern C modal picker
  (`OverlayCenterDimBackdrop`) is the canonical edit path for all fields;
  delegate cycle keys are an optional later layer. Pattern D (row-enters-edit-mode)
  explicitly rejected.
- **bt-tkhq ratified table (2026-05-19, lives as comments on bt-oiaj/bt-tkhq — the
  doc path its close reason cites does NOT exist; the comments are the source)**:
  `e` reserved globally for edit (colliding bindings migrate); `E` = escalate to
  $EDITOR (within edit modals); dirty-guard = Variant A (3s Esc-Esc window); no
  mid-flight bd cancellation; `q` intercepted by open modals, Ctrl+C never;
  Esc precedence chain: open modal → filter → nested view → quit.
- **bt-2pk38 / write-routing.md Consumers**: every mutation calls
  `bdroute.Resolve(issue)` pre-flight; non-nil error = refusal toast via
  `setFailure`, ZERO bd invocations. Copy the `confirmClaim` block
  (pkg/ui/claim.go:114) verbatim as the pattern.
- **bt-oiaj.13 model**: pending overlay on bd exit 0 → reload settles → timeout or
  mismatch = discrepancy annunciator, never silent stale state.
- **bt-55n3s claim matrix (empirical, 2026-07-06)**: claim outcomes by
  (status, assignee) vs actor; exact-string actor match; closed/blocked not
  claimable; TUI actor is currently bare `pc`.

**Dead text — do NOT implement, regardless of what bead bodies say:**
- `internal/bdcli` (named in .5/.6/umbrella) never existed and never will.
  Executor = `internal/bdexec`, routing = `internal/bdroute`. The formal dep
  edges .5→bt-oiaj.1 and .6→bt-oiaj.1 are stale (removed as part of this plan's
  bookkeeping); bt-oiaj.6's msxk edge is satisfied-by-extraction per bt-chbqq's
  re-sequencing.
- Digit keys `1`-`4` for priority (in .5's body) — contradicted by bt-88qn and
  colliding (`1` = Notifications, global.go:171). Never bind digits.
- writable-tui-design-surface.md Appendix B key candidates — all superseded;
  the collision table in this plan is the current truth.
- bt-oiaj.6's cp1252 rationale — REFRAMED not reversed: tempfiles stay where bd
  has file flags, for multiline/size robustness; argv is Unicode-safe from bt
  (Go argv is UTF-16 end-to-end).

## Resolved forks (decision + citation)

1. **Container for title/assignee**: textinput INSIDE a Pattern C modal
   (template: `pkg/ui/bql_modal.go`). bt-88qn's canon governs the container;
   write-routing.md's "in-TUI textinput → argv" governs widget + transport.
   No tension: modal → textinput → argv. bt-oiaj.5's "§3.1 pick once" is
   hereby picked: modal, uniformly.
2. **Write machinery generalization** (the claim slice deferred this to exactly
   now): ONE generic mechanism, built in Slice A, claim migrates onto it.
   NOT parallel per-field copies — nine fields would mean nine copies.
3. **Settle semantics**: pending entries capture `(field, targetValue)` at
   write time; settle = exact compare of that field on reload. Claim keeps its
   shipped heuristic (`status != open || assignee != ""`) as its op-specific
   predicate — bt cannot predict the actor string bd will resolve, so claim
   cannot target-compare assignee.
4. **Keybinds**: `e` opens ONE field-select modal (tkhq #1 + Pattern C
   one-paradigm + the keyspace is exhausted — see collision table). Per-field
   accelerator keys live INSIDE the modal where nothing collides. Cycle keys
   (bt-88qn's P/O layer) are DEFERRED — their letters are unresolved against
   bt-gf3d by bt-88qn's own text.
5. **Claim-confirm divergence** (tkhq said no-confirm; oiaj.10 shipped k9s
   confirm): resolved by making the confirm modal earn its keep — Slice A puts
   the bt-55n3s outcome prediction INTO the modal body. Field edits get no
   second confirm; the picker/textinput Enter IS the commit.
6. **Prediction posture**: WARN in the confirm/picker, never block; bd stays the
   source of truth (don't-trust-verify; zero risk of wrongly refusing).
7. **Status picker excludes `closed`** (and reopen-from-closed): tkhq's
   destructive-action pattern requires a reason-bearing form modal — that is
   bt-oiaj.2 scope, not .5. Picker offers bt's non-closed `model.Status` values.
8. **Priority representation**: numeric `-p 0..4` on the wire; picker displays P0–P4.
9. **bd flag reality** (verified against installed bd 1.0.5): description →
   `--body-file`; design → `--design-file`; **acceptance → inline `--acceptance`
   argv (NO --acceptance-file exists)**; notes → `--append-notes` argv; comments →
   `bd comments add <id> -f <file>` (canonical; NOT `bd comment --file`).
   Slice C amends write-routing.md's Consumers section to record this
   file-flag-where-it-exists split.
10. **Actor trap fencing**: assignee EDITS (`-a value`) are not actor-gated and
    are safe. Claim/unclaim exact-match traps belong to bt-oiaj.14 — no local
    workaround, no env mutation, no --actor composition in this wave.

## Keybind changes (Slice B; verify each with `grep -rn 'WithKeys' pkg/ui/keys/` + a raw-string grep in pkg/ui before wiring)

| Key | Now | Becomes | Basis |
|---|---|---|---|
| `e` (list/detail normal) | EpicCard (list.go:119) | **Edit field-select modal** | tkhq #1 (ratified; collisions migrate) |
| `F` | free (audited 2026-07-06) | EpicCard ("focus card") | migration target; fallbacks if collision: `B`, `D`, `M`, `Y` |
| `e` (board) | toggle empty cols | `z` (free) | tkhq #1 |
| `e` (insights) | explanations | `X` (uppercase, free) | tkhq #1 |
| `E` inside edit modals only | — | escalate to $EDITOR (Slice C) | tkhq #5; global `E` stays Epics — modal interception makes this safe |

Inside the field-select modal (no collisions possible — modal intercepts):
`s` status, `p` priority, `t` title, `a` assignee, plus Slice C adds
`d` description, `g` design, `c` comment, `n` append-notes, `A` acceptance.
j/k + Enter always work.

## Slice A — bt-oiaj.13: pending/settled generalization + outcome prediction

Files: `pkg/ui/claim.go` (rename/extend), `pkg/ui/model.go` (state fields),
`pkg/ui/model_update_data.go` (two settle call sites: :360, :452), tests.

1. Replace `pendingClaims map[string]bool` (model.go:848) with
   `pendingWrites map[string]pendingWrite`:
   ```go
   type pendingWrite struct {
       Kind      writeKind // writeClaim | writeFieldEdit
       Field     string    // "status", "priority", "title", "assignee", ... (empty for claim)
       Target    string    // canonical string form of the expected new value
       StartedAt time.Time
   }
   ```
   One pending write per issue (map keyed by ID) — a second write request on a
   row with a pending write is refused with a notice ("write already pending
   for <id>"). This is a deliberate v1 simplification; state it in the code
   comment.
2. Generalize messages: `writeResultMsg{id string, kind writeKind, field string, result bdexec.Result}`
   replaces `claimResultMsg`; `claimCmd` becomes `writeCmd(target bdroute.WriteTarget, id string, kind, field string, args []string)`
   — claim passes `["update", id, "--claim"]`, keeps its `target.Global` branch.
   Spinner state (`claimSpinnerActive`, model.go:849) is already issue-agnostic:
   rename to writeSpinner*, mechanics unchanged.
3. Settle: `settlePendingWrites` replaces `settlePendingClaims` (claim.go:226),
   called from the SAME two sites in model_update_data.go. Switch on Kind:
   claim → existing heuristic verbatim; fieldEdit → compare the named field's
   canonical string against Target.
4. Timeout annunciator: on each settle pass, entries older than **45s** emit a
   warning-severity footer toast ("write to <id> (<field>) not confirmed after
   45s — refresh or check bd") + events ring entry, and the pending marker
   clears (discrepancy is surfaced, never silent). Reuse the existing footer
   severity API next to `setFailure`; if only failure severity exists, use it
   and note that in the PR description.
5. Outcome prediction in the claim confirm modal (bt-55n3s matrix, from data bt
   already holds — NO bd spawn): closed/blocked → "not claimable: status X";
   assignee set and != "" → "assigned to <assignee> — bd will refuse unless
   that's your actor"; open+unassigned → no warning line. Warn only; `y` still
   proceeds.
6. Tests (write first): unit for pendingWrite settle per kind (target hit,
   target miss, timeout path); teatest: claim keypath still green end-to-end
   (expect mechanical renames in claim_test.go — that churn is in-scope);
   prediction lines render for a closed bead and an assigned bead;
   BT_CLAIM_INTEGRATION=1 suite still green (pkg/ui/claim_integration_test.go).

## Slice B — bt-oiaj.5: field edits (status, priority, title, assignee)

Files: new `pkg/ui/field_edit.go` (+test), `pkg/ui/model.go` (ModalType consts
at :117-136 — add `ModalFieldSelect`, `ModalFieldPicker`, `ModalFieldInput`),
`pkg/ui/model_update_input.go`, `pkg/ui/model_view.go`, `pkg/ui/keys/*`
(migrations per table above), help surface entries.

1. **Trigger**: `e` with an issue selected (list OR detail focus) reads
   `m.list.SelectedItem()` exactly like `requestClaim` (claim.go:91).
   **The detail viewport has NO addressable rows or field cursor
   (model_filter.go:946 renders one opaque string) — do not attempt viewport-row
   addressing or Tab-between-fields. This is the #1 way to wedge.**
2. **Field-select modal**: small picker (template `pkg/ui/repo_picker.go`)
   listing Status/Priority/Title/Assignee with accelerator keys. Esc closes.
3. **Enum pickers** (status, priority): repo_picker template, current value
   highlighted, Enter commits. Status options = bt's `model.Status` set minus
   closed (fork #7). Priority options P0–P4 → `-p <n>`.
4. **Textinput modals** (title, assignee): bql_modal template
   (vendored `charm.land/bubbles/v2/textinput` — confirmed vendored), prefilled
   with current value, Enter commits, empty title refused client-side.
   Argv: `["update", id, "--title", v]` / `["update", id, "-a", v]`.
5. **Commit path** (identical for all four): `bdroute.Resolve(*iss)` → on error
   `setFailure(err.Error())`, close modal, ZERO bd spawns; on success register
   `pendingWrite{writeFieldEdit, field, target}` + `writeCmd(...)`. Copy the
   confirmClaim block shape.
6. **Modal wiring — recite of tui-modal-compositing.md, follow literally**:
   (1) modal exposes bare `View()`, no centering logic inside;
   (2) the `activeModal` switch case in `model_view.go` is a FALL-THROUGH
   comment only — body renders first;
   (3) overlay block at the BOTTOM of `View()` calls
   `OverlayCenterDimBackdrop(body, m.x.View(), m.width, m.height-1)`.
   Rendering modal content inside the switch case silently breaks the dim
   backdrop — nothing but visual review catches it.
7. **Key dispatch**: insert the new modal check blocks in `handleKeyPress`'s
   ordered if-chain immediately AFTER the `ModalClaimConfirm` block
   (model_update_input.go:573-583) and BEFORE Help (:586). Placed later, digit
   and letter keys inside your pickers get swallowed by global bindings.
8. Tests: teatest keypaths per field (open → pick → pending → settle via
   injected reload); refusal keypath (unmappable bead → toast, executor stub
   asserts zero invocations); key-migration regression (F opens epic card;
   board z; insights X); render-harness scenarios for the new modals
   (`render_harness_test.go` + BT_RENDER_DUMP=1, small-terminal sizes —
   user norm is 14-30 rows).

## Slice C — bt-oiaj.6: long-form fields (description, design, acceptance, notes-append, comment)

Files: new `pkg/ui/longform_edit.go` (+test), field-select modal gains the five
entries, `internal/` untouched.

1. **Textarea modal**: vendored `charm.land/bubbles/v2/textarea` (confirmed
   vendored), prefilled from the current field value, markdown preview toggle
   DEFERRED (glamour preview is .6 polish, not v1 — note on the bead).
2. **Transport per field** (fork #9 — bd flags verified):
   description → tempfile + `--body-file`; design → tempfile + `--design-file`;
   comment → tempfile + `bd comments add <id> -f <file>` (author flag unused —
   bt-oiaj.14's seam); acceptance → inline `--acceptance` argv; notes →
   inline `--append-notes` argv. Amend write-routing.md's Consumers section
   with this split in the same commit.
3. **Tempfiles**: `.bt/tmp/edits/<session-pid>/<id>-<field>.md`, created on
   commit, swept on bt startup (add sweep to root.go startup path only if a
   trivial hook point exists — otherwise sweep lazily on first edit-modal open
   and note it). Gitignore already covers `.bt/`.
4. **Dirty-guard** (tkhq #3, Variant A): Esc on a dirty textarea arms a 3s
   window with a footer hint ("unsaved — Esc again to discard"); second Esc in
   window discards; any other key disarms. Rapid Esc-Esc works. Draft survives
   modal reopen in memory for the session; NO disk persistence (fence).
5. **$EDITOR escalation**: `E` inside the textarea modal →
   `tea.ExecProcess` on the tempfile (write current buffer first), reload
   buffer on return. Never bind global `E`. Settle prediction for comments:
   comments have no field to compare — settle on the reload that follows exit 0
   (kind writeFieldEdit, field "comment", Target "" → settle-on-next-reload;
   encode as a third tiny predicate case).
6. Tests: teatest for description edit end-to-end with executor stub asserting
   the tempfile path appears in argv; acceptance edit asserts INLINE argv (no
   tempfile); comment path asserts `comments add -f`; dirty-guard timing test
   (arm, disarm on keypress, discard on double-Esc); startup/lazy sweep test.

## Scope fences (violating any of these = stop and flag)

No optimistic UI or rollback (bt-mpt9d). No undo (bt-msxk §3.4). No
close/reopen UI, no reason forms (bt-oiaj.2's scope; status picker excludes
closed). No labels-write promotion of LabelPickerModel (follow-up bead — the
picker is read-only filter today). No type-change UI, no metadata editing
(.5 defers both). No comment edit/delete/threading (.6). No
`--allow-empty-description`. No beads_global routing workarounds (Resolve's
refusal is by design). No actor/--actor composition, no BD_ACTOR mutation
(bt-oiaj.14). No drafts-to-disk. No new write path bypassing bdexec/bdroute
(receipts .11 and --readonly .12 must keep covering everything). Don't wait on
bt-krx1/bt-evuf (layout redesigns; related, no hard edge — edits are
modal-driven, not pane-driven; krx1 owns later editable-affordance polish in
the detail pane).

## Where you WILL get stuck if you skip the pre-decisions

1. Viewport field-cursor invention → forbidden, see Slice B step 1.
2. Picking mnemonic keys ad hoc → the keyspace is exhausted
   (`e s a t c x r 1-4` all taken); use ONLY the table above.
3. Copy-pasting claim's settle heuristic for field edits → target-compare
   instead (fork #3).
4. Building nine parallel claim.go clones → the Slice A generalization is the
   decided shape.
5. Modal check block placed after global keys → keys get swallowed
   (Slice B step 7 pins the insertion point).
6. Rendering modal content in the model_view.go switch case → breaks the dim
   backdrop (Slice B step 6).
7. Hunting for `--acceptance-file` or `internal/bdcli` → neither exists.
8. Mixing `bd comment` and `bd comments add` → pinned to `comments add -f`.
9. `bd ready` may not surface .5/.6 if stale dep edges linger — the plan
   overrides them (bookkeeping removes the bt-oiaj.1 edges).
10. Windows/cp1252 fear pushing everything to tempfiles — argv is safe from bt;
    tempfiles are used exactly where bd has file flags, nowhere else.

## Session protocol

- Branch `feat/bt-edits-wave` off current main; claim bt-oiaj.13/.5/.6 as you
  start each slice; TDD within each slice; `go build ./... && go vet ./... &&
  go test ./...` green before each commit; commit format
  `feat(tui): <slice> (bt-oiaj.13)` etc.
- BT_CLAIM_INTEGRATION=1 suite must stay green after Slice A (it exercises the
  migrated machinery).
- Render-harness dumps for every new modal at 120x30 AND a scrunched size
  (~100x16) — eyeball both.
- `go install ./cmd/bt/` at the END only, after tests, per install-after-build
  convention.
- Close each bead with the structured template (reference.md) before the final
  push; open a draft PR; do not merge.
- PAUSE AND FLAG conditions: model.Status set differs from expectations; a
  pinned key collides anyway (use the listed fallbacks, note the swap);
  textarea/textinput vendoring missing; settle races between two reloads;
  anything that makes a fence look wrong. Flag = comment on the owning bead +
  PR description note, then continue with the rest if independent.
