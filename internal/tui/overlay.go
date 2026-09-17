package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/RNAV2019/note-tui/internal/notes"
)

type overlayKind int

const (
	ovNone    overlayKind = iota
	ovLeader              // space menu (which-key)
	ovPrompt              // single-line text entry
	ovPicker              // filtered choice
	ovFinder              // fuzzy search over every note, with preview
	ovConfirm             // y/n before deleting
	ovHelp
)

// purpose says what an overlay's answer is for.
type purpose int

const (
	forNone purpose = iota
	forNewNotebook
	forNewTag
	forNewNote
	forRenameNotebook
	forRenameTag
	forRenameNote
	forDeleteNotebook
	forDeleteTag
	forDeleteNote
	forMoveNote
	forPickNewNoteTag
	forPickRenameTag
	forPickDeleteTag
)

type pickItem struct {
	value string
	count int
}

// overlay is the modal layer: at most one is open, and it takes every key
// until it resolves. The targets are captured when it opens, so an action
// always applies to what the user was looking at.
type overlay struct {
	kind    overlayKind
	purpose purpose
	sub     string // leader submenu: "" or "tag"

	input   textinput.Model
	items   []pickItem
	labels  []string // what the filter matches against
	matches []match
	cursor  int

	notebook, tag string
	note          notes.Note
	candidates    []notes.Note // what the finder searches, newest first

	// back reopens the tag picker when esc is pressed on a new-note title
	// prompt that the picker led to.
	back bool
}

func (o overlay) active() bool { return o.kind != ovNone }

func newTextInput(initial string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 120
	ti.SetValue(initial)
	ti.CursorEnd()
	ti.Focus()
	return ti
}

func newPromptOverlay(p purpose, initial string) overlay {
	return overlay{kind: ovPrompt, purpose: p, input: newTextInput(initial)}
}

func newPickerOverlay(p purpose, items []pickItem, labels []string) overlay {
	o := overlay{kind: ovPicker, purpose: p, input: newTextInput(""), items: items, labels: labels}
	o.refilter()
	return o
}

func (o *overlay) refilter() {
	o.matches = filterItems(o.input.Value(), o.labels)
	o.cursor = 0
}

// selected returns the index of the highlighted item, or -1.
func (o overlay) selected() int {
	if o.cursor < 0 || o.cursor >= len(o.matches) {
		return -1
	}
	return o.matches[o.cursor].index
}

// updateInput feeds a message to the text input, refiltering if it changed.
func (o *overlay) updateInput(msg tea.Msg) tea.Cmd {
	before := o.input.Value()
	var cmd tea.Cmd
	o.input, cmd = o.input.Update(msg)
	if o.input.Value() != before && (o.kind == ovPicker || o.kind == ovFinder) {
		o.refilter()
	}
	return cmd
}

// moveSelection handles the list keys shared by pickers and the finder.
// Letters are text here, so only arrows and ctrl+n/p move.
func (o *overlay) moveSelection(key string) bool {
	switch {
	case keys.pickUp.matches(key):
		o.cursor = max(o.cursor-1, 0)
	case keys.pickDown.matches(key):
		o.cursor = min(o.cursor+1, max(len(o.matches)-1, 0))
	default:
		return false
	}
	return true
}

// crumb is appended to the statusline breadcrumb while the overlay is open.
func (o overlay) crumb() string {
	switch o.purpose {
	case forNewNotebook:
		return "new notebook"
	case forNewTag:
		return "new tag"
	case forNewNote, forPickNewNoteTag:
		return "new note"
	case forRenameNotebook, forRenameTag, forRenameNote, forPickRenameTag:
		return "rename"
	case forDeleteNotebook:
		return "delete notebook"
	case forDeleteTag, forPickDeleteTag:
		return "delete tag"
	case forDeleteNote:
		return "delete note"
	case forMoveNote:
		return "move"
	}
	switch o.kind {
	case ovHelp:
		return "keys"
	}
	return ""
}

func (o overlay) mode() mode {
	switch o.kind {
	case ovLeader:
		return modeLeader
	case ovFinder:
		return modeFind
	case ovPrompt, ovPicker:
		return modeInput
	case ovConfirm:
		return modeConfirm
	case ovHelp:
		return modeKeys
	}
	return modeNormal
}

// query is the text typed into the overlay, trimmed.
func (o overlay) query() string { return strings.TrimSpace(o.input.Value()) }
