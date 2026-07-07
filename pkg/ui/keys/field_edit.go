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
// Accelerators (Status/Priority/Title/Assignee) can't collide with anything -
// the modal intercepts every key while open (bt-oiaj plan resolved fork #4).
// Slice C (bt-oiaj.6) adds five more accelerators here (description, design,
// comment, notes, acceptance - keys d/g/c/n/A per the keybind table in
// docs/plans/2026-07-07-bt-edits-wave-oiaj13-5-6.md); this slice wires only
// the first four rows.
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
// Columns: Navigate / Accelerators / Exit.
func (k FieldSelectKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Open},
		{k.Status, k.Priority, k.Title, k.Assignee},
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
