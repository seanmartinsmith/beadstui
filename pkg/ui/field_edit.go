package ui

// field_edit.go implements the first field edits (bt-oiaj.5): a Pattern-C
// modal picker for status/priority/title/assignee, built on the generic
// pending/settled write machinery in claim.go (bt-oiaj.13). The global `e`
// binding (ListNormalKeys.FieldEdit) opens a small field-select modal;
// picking a row opens either an enum picker (status/priority) or a
// textinput modal (title/assignee); the sub-modal's Enter IS the commit
// (resolved fork #5 — field edits get no second confirm, unlike claim's
// k9s-style confirm). Esc from a sub-modal steps back one level to the
// field-select hub; Esc from the hub cancels the whole flow. Long-form
// fields (description, design, acceptance, notes, comments) are bt-oiaj.6
// (Slice C) — fieldEditEntries below is the seam Slice C appends five more
// rows to. See docs/plans/2026-07-07-bt-edits-wave-oiaj13-5-6.md.

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// fieldEditEntry describes one row in the field-select modal. Field must
// match a case in fieldValue() (claim.go) and the argv-building switches in
// handleFieldPickerKeys/handleFieldInputKeys below. Slice C (bt-oiaj.6)
// appends five more entries here (description, design, comment, notes,
// acceptance — keys d/g/c/n/A per the plan's keybind table); this slice
// ships only the first four.
type fieldEditEntry struct {
	Field string
	Label string
	Key   string // accelerator shown in the row; kept in sync by hand with
	// the matching keys.FieldSelectKeys binding (same convention as every
	// other picker's static key hints in this package).
}

var fieldEditEntries = []fieldEditEntry{
	{Field: "status", Label: "Status", Key: "s"},
	{Field: "priority", Label: "Priority", Key: "p"},
	{Field: "title", Label: "Title", Key: "t"},
	{Field: "assignee", Label: "Assignee", Key: "a"},
	// Long-form fields (bt-oiaj.6, Slice C) - route to the textarea modal
	// (longform_edit.go) instead of an enum picker or textinput.
	{Field: "description", Label: "Description", Key: "d"},
	{Field: "design", Label: "Design", Key: "g"},
	{Field: "comment", Label: "Add Comment", Key: "c"},
	{Field: "notes", Label: "Append Notes", Key: "n"},
	{Field: "acceptance", Label: "Acceptance Criteria", Key: "A"},
}

// renderFieldModalLines composes lines into a titled panel using the same
// centering/padding shape as renderClaimConfirm (claim.go) — shared here so
// the field-select, field-picker, and field-input modals don't each
// reimplement the same box math.
func renderFieldModalLines(title string, lines []string, theme Theme, maxInner int) string {
	innerW := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > innerW {
			innerW = w
		}
	}
	if innerW > maxInner {
		innerW = maxInner
	}

	const sidePad = 4
	panelWidth := innerW + 2 + sidePad
	pad := strings.Repeat(" ", sidePad/2)
	var body strings.Builder
	for i, l := range lines {
		if i > 0 {
			body.WriteString("\n")
		}
		body.WriteString(pad + centerLine(l, innerW) + pad)
	}
	content := "\n" + body.String() + "\n"

	return RenderTitledPanel(content, PanelOpts{
		Title:       title,
		Width:       panelWidth,
		CenterTitle: true,
		BorderColor: theme.Primary,
		TitleColor:  theme.Primary,
		Focused:     true,
	})
}

// ---------------------------------------------------------------------------
// Field-select modal (Pattern C hub — bt-88qn).
// ---------------------------------------------------------------------------

// FieldSelectModal is the small picker listing the editable fields. Enter (or
// an accelerator key) opens the matching enum picker or textinput modal.
type FieldSelectModal struct {
	cursor        int
	theme         Theme
	width, height int
}

// NewFieldSelectModal creates a fresh field-select modal, cursor at the top.
func NewFieldSelectModal(theme Theme) FieldSelectModal {
	return FieldSelectModal{theme: theme}
}

// SetSize updates the modal's layout budget.
func (m *FieldSelectModal) SetSize(w, h int) { m.width, m.height = w, h }

// MoveUp moves the cursor up, wrapping to the bottom.
func (m *FieldSelectModal) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	} else {
		m.cursor = len(fieldEditEntries) - 1
	}
}

// MoveDown moves the cursor down, wrapping to the top.
func (m *FieldSelectModal) MoveDown() {
	if m.cursor < len(fieldEditEntries)-1 {
		m.cursor++
	} else {
		m.cursor = 0
	}
}

// SelectedField returns the field name under the cursor.
func (m *FieldSelectModal) SelectedField() string {
	return fieldEditEntries[m.cursor].Field
}

// View renders the field-select modal. Bare content only — no centering, no
// overlay (tui-modal-compositing.md step 1); Model.View() composites it via
// OverlayCenterDimBackdrop at the bottom of its switch.
func (m FieldSelectModal) View() string {
	t := m.theme

	maxInner := m.width - 8
	if maxInner < 20 {
		maxInner = 20
	}

	textStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground())
	cursorLabelStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	cursorStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)

	var lines []string
	for i, e := range fieldEditEntries {
		cursor := "  "
		labelStyle := textStyle
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
			labelStyle = cursorLabelStyle
		}
		lines = append(lines, cursor+keyStyle.Render(e.Key)+"  "+labelStyle.Render(e.Label))
	}
	lines = append(lines, "")
	lines = append(lines, hintStyle.Render("j/k move  enter select  esc cancel"))

	return renderFieldModalLines("Edit field", lines, t, maxInner)
}

// ---------------------------------------------------------------------------
// Enum picker (status, priority).
// ---------------------------------------------------------------------------

// fieldPickerOption is one selectable value in an enum picker. Value is the
// canonical wire/settle-target form (resolved fork #8: priority is numeric
// "0".."4" on the wire, matching fieldValue()'s strconv.Itoa — NOT
// "P0".."P4"). Label is what's rendered.
type fieldPickerOption struct {
	Value string
	Label string
}

// FieldPickerModal is the enum picker for status/priority. current is the
// bead's value at open time, kept for the "current value highlighted"
// indicator — distinct from the cursor, which the user can move away from.
type FieldPickerModal struct {
	field         string
	title         string
	options       []fieldPickerOption
	cursor        int
	current       string
	theme         Theme
	width, height int
}

// newFieldPickerModal builds a picker with the cursor starting on current's
// option (falls back to index 0 if current isn't among options).
func newFieldPickerModal(field, title string, options []fieldPickerOption, current string, theme Theme) FieldPickerModal {
	cursor := 0
	for i, o := range options {
		if o.Value == current {
			cursor = i
			break
		}
	}
	return FieldPickerModal{field: field, title: title, options: options, cursor: cursor, current: current, theme: theme}
}

// statusPickerOptions returns the workflow model.Status values — everything
// except the destructive transitions closed AND tombstone (resolved fork #7,
// extended per the controller decision on the Slice B tombstone flag):
// destructive transitions need a reason-bearing form modal (tkhq's
// destructive-action pattern) — bt-oiaj.2 scope, not this slice. With no
// second confirm on picker Enter (fork #5), a stray Enter must not be able
// to soft-delete a bead. deferred/pinned/hooked/review stay — reversible
// workflow states. Order matches the declaration order in pkg/model/types.go.
func statusPickerOptions() []fieldPickerOption {
	statuses := []model.Status{
		model.StatusOpen, model.StatusInProgress, model.StatusBlocked,
		model.StatusDeferred, model.StatusPinned, model.StatusHooked,
		model.StatusReview,
	}
	opts := make([]fieldPickerOption, 0, len(statuses))
	for _, s := range statuses {
		opts = append(opts, fieldPickerOption{Value: string(s), Label: statusDisplayLabel(s)})
	}
	return opts
}

// statusDisplayLabel humanizes a snake_case model.Status for the picker row
// ("in_progress" -> "In Progress").
func statusDisplayLabel(s model.Status) string {
	words := strings.Split(string(s), "_")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// priorityPickerOptions returns P0-P4. Value is the numeric wire form (fork
// #8: `-p <n>` on the wire; fieldValue's settle-compare is also numeric);
// Label is the P-prefixed display form.
func priorityPickerOptions() []fieldPickerOption {
	opts := make([]fieldPickerOption, 0, 5)
	for p := 0; p <= 4; p++ {
		opts = append(opts, fieldPickerOption{Value: strconv.Itoa(p), Label: fmt.Sprintf("P%d", p)})
	}
	return opts
}

// NewStatusPickerModal builds the status picker, cursor on the bead's
// current status.
func NewStatusPickerModal(current model.Status, theme Theme) FieldPickerModal {
	return newFieldPickerModal("status", "Status", statusPickerOptions(), string(current), theme)
}

// NewPriorityPickerModal builds the priority picker, cursor on the bead's
// current priority.
func NewPriorityPickerModal(current int, theme Theme) FieldPickerModal {
	return newFieldPickerModal("priority", "Priority", priorityPickerOptions(), strconv.Itoa(current), theme)
}

// SetSize updates the modal's layout budget.
func (m *FieldPickerModal) SetSize(w, h int) { m.width, m.height = w, h }

// MoveUp moves the cursor up, wrapping to the bottom.
func (m *FieldPickerModal) MoveUp() {
	if len(m.options) == 0 {
		return
	}
	if m.cursor > 0 {
		m.cursor--
	} else {
		m.cursor = len(m.options) - 1
	}
}

// MoveDown moves the cursor down, wrapping to the top.
func (m *FieldPickerModal) MoveDown() {
	if len(m.options) == 0 {
		return
	}
	if m.cursor < len(m.options)-1 {
		m.cursor++
	} else {
		m.cursor = 0
	}
}

// Selected returns the option under the cursor.
func (m *FieldPickerModal) Selected() fieldPickerOption {
	return m.options[m.cursor]
}

// View renders the enum picker. Bare content only (tui-modal-compositing.md
// step 1) — composited via OverlayCenterDimBackdrop in Model.View().
func (m FieldPickerModal) View() string {
	t := m.theme

	maxInner := m.width - 8
	if maxInner < 20 {
		maxInner = 20
	}

	textStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground())
	cursorLabelStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	currentStyle := lipgloss.NewStyle().Foreground(t.Secondary)
	hintStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)

	var lines []string
	for i, o := range m.options {
		cursor := "  "
		labelStyle := textStyle
		if i == m.cursor {
			cursor = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("> ")
			labelStyle = cursorLabelStyle
		}
		marker := "  "
		if o.Value == m.current {
			marker = currentStyle.Render("* ")
		}
		lines = append(lines, cursor+marker+labelStyle.Render(o.Label))
	}
	lines = append(lines, "")
	lines = append(lines, hintStyle.Render("j/k move  enter commit  esc back  * current"))

	return renderFieldModalLines(m.title, lines, t, maxInner)
}

// ---------------------------------------------------------------------------
// Textinput sub-modal (title, assignee).
// ---------------------------------------------------------------------------

// FieldInputModal is the textinput modal for free-text fields (title,
// assignee). Modeled on bql_modal.go, simplified — a single-shot edit has no
// query history to navigate.
type FieldInputModal struct {
	field         string // "title" or "assignee" — matches fieldValue()'s switch
	label         string // "Title" or "Assignee" — display
	input         textinput.Model
	theme         Theme
	width, height int
	err           string
}

// NewFieldInputModal creates a textinput modal prefilled with current,
// cursor at the end. field/label drive the argv branch
// (handleFieldInputKeys) and the empty-title client-side refusal (title
// only).
func NewFieldInputModal(field, label, current string, theme Theme) FieldInputModal {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.SetWidth(40)
	ti.SetValue(current)
	ti.CursorEnd()
	return FieldInputModal{field: field, label: label, input: ti, theme: theme}
}

// SetSize updates the modal's layout budget.
func (m *FieldInputModal) SetSize(w, h int) { m.width, m.height = w, h }

// Focus activates the textinput's cursor/blink.
func (m *FieldInputModal) Focus() tea.Cmd { return m.input.Focus() }

// Value returns the current input text.
func (m *FieldInputModal) Value() string { return m.input.Value() }

// SetError sets the inline validation error shown below the input.
func (m *FieldInputModal) SetError(msg string) { m.err = msg }

// Update forwards msg to the textinput, clearing any validation error on
// further typing (mirrors BQLQueryModal.Update).
func (m FieldInputModal) Update(msg tea.Msg) (FieldInputModal, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.err != "" {
		m.err = ""
	}
	return m, cmd
}

// View renders the textinput modal. Bare content only
// (tui-modal-compositing.md step 1) — composited via OverlayCenterDimBackdrop
// in Model.View().
func (m FieldInputModal) View() string {
	t := m.theme

	maxInner := m.width - 8
	if maxInner < 24 {
		maxInner = 24
	}

	labelStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(t.Warning)
	hintStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)

	lines := []string{labelStyle.Render(m.label + ":"), m.input.View()}
	if m.err != "" {
		lines = append(lines, errStyle.Render(m.err))
	}
	lines = append(lines, "")
	lines = append(lines, hintStyle.Render("enter apply  esc back"))

	return renderFieldModalLines("Edit "+m.label, lines, t, maxInner)
}

// ---------------------------------------------------------------------------
// Trigger / commit (mirrors requestClaim/confirmClaim — claim.go).
// ---------------------------------------------------------------------------

// requestFieldEdit opens the field-select modal for the currently selected
// bead. Mirrors requestClaim (claim.go): same "no selection" / "write
// already pending" guards, same double-dispatch refusal (one pending write
// per issue — bt-oiaj.13 v1 simplification).
func (m *Model) requestFieldEdit() {
	sel, ok := m.list.SelectedItem().(IssueItem)
	if !ok {
		m.setNotice("No issue selected")
		return
	}
	if _, pending := m.pendingWrites[sel.Issue.ID]; pending {
		m.setNotice(fmt.Sprintf("write already pending for %s", sel.Issue.ID))
		return
	}
	m.fieldEditTargetID = sel.Issue.ID
	m.fieldSelect = NewFieldSelectModal(m.theme)
	m.fieldSelect.SetSize(m.width, m.height-1)
	m.openModal(ModalFieldSelect)
	m.focused = focusFieldSelect
}

// cancelFieldEdit fully closes the field-edit flow (Esc from the
// field-select hub) — back to the list, matching cancelClaim's shape.
func (m *Model) cancelFieldEdit() {
	m.fieldEditTargetID = ""
	m.closeModal()
	m.focused = focusList
	m.setNotice("Edit cancelled")
}

// backToFieldSelect returns from a sub-modal (enum picker or textinput) to
// the field-select hub (Esc steps back one level), keeping
// fieldEditTargetID intact so the user can pick a different field without
// restarting the flow.
func (m *Model) backToFieldSelect() {
	m.openModal(ModalFieldSelect)
	m.focused = focusFieldSelect
}

// openFieldPickerOrInput opens the enum picker or textinput modal for field,
// reading the bead's current value from already-loaded data (zero bd spawn —
// same posture as predictClaimOutcome). The "issue not found" guard mirrors
// confirmClaim's identical guard (claim.go).
func (m Model) openFieldPickerOrInput(field string) (Model, tea.Cmd) {
	iss, ok := m.data.issueMap[m.fieldEditTargetID]
	if !ok {
		m.setFailure(fmt.Sprintf("Edit %s refused: issue not found in loaded data", m.fieldEditTargetID))
		m.fieldEditTargetID = ""
		m.closeModal()
		m.focused = focusList
		return m, nil
	}

	switch field {
	case "status":
		// Reopen fence (fork #7's second clause: "excludes closed (and
		// reopen-from-closed)"): on a closed or tombstoned bead the status
		// picker must not open at all — newFieldPickerModal's cursor falls
		// back to index 0 ("open") when the current status isn't among the
		// options, so with no second confirm on picker Enter (fork #5) a
		// stray e→s→Enter would silently commit a reopen. Destructive-state
		// transitions in EITHER direction need bt-oiaj.2's reason-bearing
		// form. The field-select hub stays open (mirrors the empty-title
		// refusal: the flow isn't broken, only this entry is fenced —
		// title/priority/assignee stay editable on closed beads).
		if iss.Status == model.StatusClosed || iss.Status == model.StatusTombstone {
			m.setNotice(fmt.Sprintf(
				"status of a %s bead needs a reason form - not yet available (bt-oiaj.2)", iss.Status))
			return m, nil
		}
		m.fieldPicker = NewStatusPickerModal(iss.Status, m.theme)
		m.fieldPicker.SetSize(m.width, m.height-1)
		m.openModal(ModalFieldPicker)
		m.focused = focusFieldPicker
	case "priority":
		m.fieldPicker = NewPriorityPickerModal(iss.Priority, m.theme)
		m.fieldPicker.SetSize(m.width, m.height-1)
		m.openModal(ModalFieldPicker)
		m.focused = focusFieldPicker
	case "title":
		m.fieldInput = NewFieldInputModal("title", "Title", iss.Title, m.theme)
		m.fieldInput.SetSize(m.width, m.height-1)
		m.openModal(ModalFieldInput)
		m.focused = focusFieldInput
		return m, m.fieldInput.Focus()
	case "assignee":
		m.fieldInput = NewFieldInputModal("assignee", "Assignee", iss.Assignee, m.theme)
		m.fieldInput.SetSize(m.width, m.height-1)
		m.openModal(ModalFieldInput)
		m.focused = focusFieldInput
		return m, m.fieldInput.Focus()
	// Long-form fields (bt-oiaj.6, Slice C): the textarea modal
	// (longform_edit.go). description/design/acceptance prefill from the
	// bead's current value (full-replace fields, same as title/assignee
	// above); comment/notes open empty - they're add-only (bd comments add
	// appends a new comment; --append-notes appends to the existing notes
	// rather than replacing them), so prefilling from the current value
	// would invite committing a duplicate of it. No terminal-status fence
	// here (unlike status above): the plan doesn't fence long-form fields on
	// closed/tombstoned beads, so they stay editable throughout.
	case "description":
		return m.openLongformEditModal("description", "Description", iss.Description)
	case "design":
		return m.openLongformEditModal("design", "Design", iss.Design)
	case "comment":
		return m.openLongformEditModal("comment", "Add Comment", "")
	case "notes":
		return m.openLongformEditModal("notes", "Append Notes", "")
	case "acceptance":
		return m.openLongformEditModal("acceptance", "Acceptance Criteria", iss.AcceptanceCriteria)
	}
	return m, nil
}

// commitFieldEdit is the shared commit path for every field edit - the
// original four single-line fields (plan step 5) AND the five long-form
// fields Slice C adds (commitLongformEdit, longform_edit.go, builds the
// argv/target tuple per field's transport and calls this verbatim): Resolve
// pre-flight, refusal = setFailure + close + zero bd spawns; success =
// register a pendingWrite and dispatch writeCmd. Copies confirmClaim's block
// shape (claim.go) verbatim. target is the canonical settle-compare string
// captured at write time (pendingWrite.Target — fork #3: field edits
// target-compare exactly, unlike claim's status/assignee heuristic; "comment"
// is the one exception, with Target always "" - see writeSettled's explicit
// third predicate case in claim.go).
func (m Model) commitFieldEdit(field, target string, args []string) (Model, tea.Cmd) {
	id := m.fieldEditTargetID
	m.fieldEditTargetID = ""
	m.closeModal()
	m.focused = focusList
	if id == "" {
		return m, nil
	}

	iss, ok := m.data.issueMap[id]
	if !ok {
		m.setFailure(fmt.Sprintf("Edit %s refused: issue not found in loaded data", id))
		return m, nil
	}

	writeTarget, err := m.routeTable.Resolve(*iss)
	if err != nil {
		m.setFailure(err.Error())
		return m, nil
	}

	if m.pendingWrites == nil {
		m.pendingWrites = make(map[string]pendingWrite)
	}
	m.pendingWrites[id] = pendingWrite{Kind: writeFieldEdit, Field: field, Target: target, StartedAt: time.Now()}
	m.updateListDelegate()
	m.setNotice(fmt.Sprintf("Updating %s (%s)...", id, field))

	cmds := []tea.Cmd{writeCmd(writeTarget, id, writeFieldEdit, field, args)}
	if !m.writeSpinnerActive {
		m.writeSpinnerActive = true
		cmds = append(cmds, writeSpinnerTickCmd())
	}
	return m, tea.Batch(cmds...)
}

// ---------------------------------------------------------------------------
// Key dispatch (mirrors handleRepoPickerKeys/handleRecipePickerKeys — each
// modal owns its keys fully; the ctrl+c-then-call wrapper lives in
// model_update_input.go, same convention as the other early-return picker
// blocks there).
// ---------------------------------------------------------------------------

// handleFieldSelectKeys dispatches keys for the field-select hub.
func (m Model) handleFieldSelectKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	kk := m.keys.FieldSelect
	switch {
	case key.Matches(msg, kk.Up):
		m.fieldSelect.MoveUp()
	case key.Matches(msg, kk.Down):
		m.fieldSelect.MoveDown()
	case key.Matches(msg, kk.Cancel):
		m.cancelFieldEdit()
	case key.Matches(msg, kk.Status):
		return m.openFieldPickerOrInput("status")
	case key.Matches(msg, kk.Priority):
		return m.openFieldPickerOrInput("priority")
	case key.Matches(msg, kk.Title):
		return m.openFieldPickerOrInput("title")
	case key.Matches(msg, kk.Assignee):
		return m.openFieldPickerOrInput("assignee")
	case key.Matches(msg, kk.Description):
		return m.openFieldPickerOrInput("description")
	case key.Matches(msg, kk.Design):
		return m.openFieldPickerOrInput("design")
	case key.Matches(msg, kk.Comment):
		return m.openFieldPickerOrInput("comment")
	case key.Matches(msg, kk.Notes):
		return m.openFieldPickerOrInput("notes")
	case key.Matches(msg, kk.Acceptance):
		return m.openFieldPickerOrInput("acceptance")
	case key.Matches(msg, kk.Open):
		return m.openFieldPickerOrInput(m.fieldSelect.SelectedField())
	}
	return m, nil
}

// handleFieldPickerKeys dispatches keys for the enum-picker sub-modal.
func (m Model) handleFieldPickerKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	kk := m.keys.FieldPicker
	switch {
	case key.Matches(msg, kk.Up):
		m.fieldPicker.MoveUp()
	case key.Matches(msg, kk.Down):
		m.fieldPicker.MoveDown()
	case key.Matches(msg, kk.Cancel):
		m.backToFieldSelect()
	case key.Matches(msg, kk.Apply):
		opt := m.fieldPicker.Selected()
		id := m.fieldEditTargetID
		field := m.fieldPicker.field
		var args []string
		switch field {
		case "status":
			args = []string{"update", id, "--status", opt.Value}
		case "priority":
			args = []string{"update", id, "-p", opt.Value}
		}
		return m.commitFieldEdit(field, opt.Value, args)
	}
	return m, nil
}

// handleFieldInputKeys dispatches keys for the textinput sub-modal.
func (m Model) handleFieldInputKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	kk := m.keys.FieldInput
	switch {
	case key.Matches(msg, kk.Cancel):
		m.backToFieldSelect()
		return m, nil
	case key.Matches(msg, kk.Apply):
		value := strings.TrimSpace(m.fieldInput.Value())
		field := m.fieldInput.field
		if field == "title" && value == "" {
			m.fieldInput.SetError("title cannot be empty")
			return m, nil
		}
		id := m.fieldEditTargetID
		var args []string
		switch field {
		case "title":
			args = []string{"update", id, "--title", value}
		case "assignee":
			args = []string{"update", id, "-a", value}
		}
		return m.commitFieldEdit(field, value, args)
	default:
		var cmd tea.Cmd
		m.fieldInput, cmd = m.fieldInput.Update(msg)
		return m, cmd
	}
}
