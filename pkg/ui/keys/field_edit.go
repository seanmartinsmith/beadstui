package keys

import "charm.land/bubbles/v2/key"

// FieldSelectKeys are the bindings for the field-select modal (bt-oiaj.5):
// the Pattern-C hub opened by the global 'e' binding (ListNormalKeys.FieldEdit)
// that leads to either an enum picker (status/priority) or a textinput modal
// (title/assignee). handleFieldSelectKeys (pkg/ui/field_edit.go) dispatches
// against these via key.Matches.
//
// Up/Down field names and Help.Key strings match ListNormalKeys / EpicCardKeys
// for the universal-nav consistency test (TestUniversalNav_ConsistentAcrossViews).
// Cancel (not Esc) avoids that check, mirroring every other picker's field
// name (RecipePickerKeys.Cancel, RepoPickerKeys.Cancel).
//
// Accelerators (Status/Priority/Title/Assignee/Description/Design/Comment/
// Notes/Acceptance) can't collide with anything - the modal intercepts every
// key while open (bt-oiaj plan resolved fork #4). Slice C (bt-oiaj.6) added
// the five long-form accelerators (d/g/c/n/A) that open the textarea modal
// (pkg/ui/longform_edit.go) instead of the enum-picker/textinput sub-modals.
type FieldSelectKeys struct {
	// Nav
	Up   key.Binding
	Down key.Binding
	Open key.Binding // enter: edit the field under the cursor

	// Accelerators
	Status   key.Binding
	Priority key.Binding
	Title    key.Binding
	Assignee key.Binding

	// Long-form accelerators (bt-oiaj.6, Slice C) - open the textarea modal
	// (pkg/ui/longform_edit.go) rather than an enum picker or textinput.
	Description key.Binding
	Design      key.Binding
	Comment     key.Binding
	Notes       key.Binding
	Acceptance  key.Binding // uppercase A - lowercase a is Assignee

	// Exit
	Cancel key.Binding
}

// NewFieldSelectKeys returns the default field-select modal keymap.
func NewFieldSelectKeys() FieldSelectKeys {
	return FieldSelectKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Open: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "edit selected field"),
		),
		Status: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "edit status"),
		),
		Priority: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "edit priority"),
		),
		Title: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "edit title"),
		),
		Assignee: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "edit assignee"),
		),
		Description: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "edit description"),
		),
		Design: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "edit design"),
		),
		Comment: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "add comment"),
		),
		Notes: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "append notes"),
		),
		Acceptance: key.NewBinding(
			key.WithKeys("A"),
			key.WithHelp("A", "edit acceptance criteria"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel edit"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
func (k FieldSelectKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Open, k.Cancel}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Navigate / Accelerators / Long-form / Exit.
func (k FieldSelectKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Open},
		{k.Status, k.Priority, k.Title, k.Assignee},
		{k.Description, k.Design, k.Comment, k.Notes, k.Acceptance},
		{k.Cancel},
	}
}

// FieldPickerKeys are the bindings for the enum-picker sub-modal
// (status/priority). handleFieldPickerKeys dispatches against these via
// key.Matches.
//
// Up/Down match the universal-nav consistency test. Cancel returns to the
// field-select hub rather than closing the whole flow (bt-oiaj.5's
// nested-modal Esc convention: one Esc steps back one level). Apply commits
// directly - resolved fork #5: the picker's Enter IS the commit, no second
// confirm (unlike claim, which keeps its k9s-style confirm).
type FieldPickerKeys struct {
	Up     key.Binding
	Down   key.Binding
	Apply  key.Binding
	Cancel key.Binding
}

// NewFieldPickerKeys returns the default enum-picker keymap.
func NewFieldPickerKeys() FieldPickerKeys {
	return FieldPickerKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Apply: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "commit value"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back to field select"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
func (k FieldPickerKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Apply, k.Cancel}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
func (k FieldPickerKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Apply, k.Cancel},
	}
}

// FieldInputKeys are the bindings for the textinput sub-modal
// (title/assignee). Letter keys are NOT matched here; the textinput
// component owns them via the default branch in handleFieldInputKeys, same
// convention as BQLQueryKeys / TimeTravelInputKeys.
type FieldInputKeys struct {
	Apply  key.Binding
	Cancel key.Binding
}

// NewFieldInputKeys returns the default textinput sub-modal keymap.
func NewFieldInputKeys() FieldInputKeys {
	return FieldInputKeys{
		Apply: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "commit value"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back to field select"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
func (k FieldInputKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Apply, k.Cancel}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
func (k FieldInputKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Apply, k.Cancel},
	}
}

// LongformEditKeys are the bindings for the textarea modal (bt-oiaj.6, Slice
// C): description/design/comment/append-notes/acceptance. Unlike
// FieldPickerKeys/FieldInputKeys this modal has no picker cursor - Up/Down
// arrow-key navigation is owned entirely by the textarea component itself,
// so no Up/Down fields are declared here (nothing to route around
// TestUniversalNav_ConsistentAcrossViews for). handleLongformEditKeys
// (pkg/ui/longform_edit.go) forwards any unmatched key to
// textarea.Model.Update, the same convention as FieldInputKeys' default
// branch forwarding to textinput.Model.Update.
//
// Commit and Escalate are new binding names with no universal-nav
// counterpart. Cancel (not Esc) mirrors FieldPickerKeys/FieldInputKeys'
// established dodge of TestUniversalNav_ConsistentAcrossViews (see their
// comments) - Cancel is dirty-guard gated here (tkhq #3 Variant A), unlike
// the immediate step-back the other two sub-modals use.
type LongformEditKeys struct {
	// Commit submits the buffer (ctrl+s, not enter - enter is the textarea's
	// own insert-newline binding, so multiline editing needs a key Enter
	// doesn't already own; tkhq Q5 ratified this as part of the hybrid model).
	Commit key.Binding

	// Escalate hands the current buffer to $EDITOR/$VISUAL via tea.ExecProcess
	// (tkhq Q5). Bound to uppercase E ONLY inside this modal - the global `E`
	// binding elsewhere in bt stays Epics (list.go); this binding intercepts
	// before the textarea would otherwise insert the literal character.
	Escalate key.Binding

	// Cancel is dirty-guard gated (handleLongformEscape, longform_edit.go):
	// on a clean buffer it steps back to the field-select hub immediately,
	// matching FieldPickerKeys/FieldInputKeys; on a dirty buffer the first
	// press arms a 3s discard window (tkhq #3 Variant A) and the second
	// press within it discards and steps back.
	Cancel key.Binding
}

// NewLongformEditKeys returns the default textarea-modal keymap.
func NewLongformEditKeys() LongformEditKeys {
	return LongformEditKeys{
		Commit: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "commit edit"),
		),
		Escalate: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "escalate to $EDITOR"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back / discard (esc-esc if unsaved)"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
func (k LongformEditKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Commit, k.Escalate, k.Cancel}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
func (k LongformEditKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Commit, k.Escalate, k.Cancel},
	}
}
