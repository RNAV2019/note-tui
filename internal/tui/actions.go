package tui

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/RNAV2019/note-tui/internal/notes"
)

// nowFunc is the clock, swappable in tests.
var nowFunc = time.Now

// ---------------------------------------------------------------- triggers

func (m *Model) startNewNotebook() {
	m.ov = newPromptOverlay(forNewNotebook, "")
}

func (m *Model) startNewTag() {
	nb := m.currentNotebook()
	if nb == "" {
		m.setStatus(statusErr, "create a notebook first — press space N")
		return
	}
	m.ov = newPromptOverlay(forNewTag, "")
	m.ov.notebook = nb
}

func (m *Model) startNewNote() {
	nb := m.currentNotebook()
	switch {
	case nb == "":
		m.setStatus(statusErr, "create a notebook first — press n in the sidebar")
		return
	case len(m.tags) == 0:
		m.setStatus(statusErr, "create a tag first — press space t n")
		return
	}
	// With a tag tab active, skip straight to the title; only the ambiguous
	// "all" tab has to ask which tag, and then only if there is a choice.
	tag := m.selectedTag()
	if tag == "" && len(m.tags) == 1 {
		tag = m.tags[0]
	}
	if tag != "" {
		m.pendingTag = tag
		m.ov = newPromptOverlay(forNewNote, "")
		m.ov.notebook, m.ov.tag = nb, tag
		return
	}
	m.ov = m.tagPicker(forPickNewNoteTag)
}

// tagPicker lists the current notebook's tags with their note counts.
func (m Model) tagPicker(p purpose) overlay {
	nb := m.currentNotebook()
	items := make([]pickItem, len(m.tags))
	for i, t := range m.tags {
		items[i] = pickItem{value: t, count: m.countNotes(nb, t)}
	}
	o := newPickerOverlay(p, items, m.tags)
	o.notebook = nb
	return o
}

func (m *Model) startRename() {
	if m.focus == paneSidebar {
		nb := m.currentNotebook()
		if nb == "" {
			return
		}
		m.ov = newPromptOverlay(forRenameNotebook, nb)
		m.ov.notebook = nb
		return
	}
	if n, ok := m.currentNote(); ok {
		m.startRenameNote(n)
	}
}

func (m *Model) startRenameNote(n notes.Note) {
	m.ov = newPromptOverlay(forRenameNote, n.Name)
	m.ov.note = n
}

func (m *Model) startRenameTag() {
	if m.currentNotebook() == "" || len(m.tags) == 0 {
		m.setStatus(statusErr, "no tags to rename — press space t n to add one")
		return
	}
	tag := m.selectedTag()
	if tag == "" {
		m.ov = m.tagPicker(forPickRenameTag)
		return
	}
	m.ov = newPromptOverlay(forRenameTag, tag)
	m.ov.notebook, m.ov.tag = m.currentNotebook(), tag
}

func (m *Model) startDelete() {
	if m.focus == paneSidebar {
		nb := m.currentNotebook()
		if nb == "" {
			return
		}
		m.ov = overlay{kind: ovConfirm, purpose: forDeleteNotebook, notebook: nb}
		return
	}
	if n, ok := m.currentNote(); ok {
		m.startDeleteNote(n)
	}
}

func (m *Model) startDeleteNote(n notes.Note) {
	m.ov = overlay{kind: ovConfirm, purpose: forDeleteNote, note: n}
}

func (m *Model) startDeleteTag() {
	if m.currentNotebook() == "" || len(m.tags) == 0 {
		m.setStatus(statusErr, "no tags to delete")
		return
	}
	tag := m.selectedTag()
	if tag == "" {
		m.ov = m.tagPicker(forPickDeleteTag)
		return
	}
	m.ov = overlay{kind: ovConfirm, purpose: forDeleteTag, notebook: m.currentNotebook(), tag: tag}
}

func (m *Model) startMove(n notes.Note) {
	var items []pickItem
	var labels []string
	for _, nb := range m.notebooks {
		tags, err := m.store.Tags(nb)
		if err != nil {
			m.setError(err)
			return
		}
		for _, t := range tags {
			if nb == n.Notebook && t == n.Tag {
				continue // moving a note to where it already is does nothing
			}
			dest := nb + "/" + t
			items = append(items, pickItem{value: dest, count: m.countNotes(nb, t)})
			labels = append(labels, dest)
		}
	}
	if len(items) == 0 {
		m.setStatus(statusErr, "no other tag to move "+n.Name+" into")
		return
	}
	m.ov = newPickerOverlay(forMoveNote, items, labels)
	m.ov.note = n
}

// ---------------------------------------------------------------- apply

func (m Model) applyPrompt(o overlay) (tea.Model, tea.Cmd) {
	value := o.query()
	switch o.purpose {
	case forNewNotebook:
		if err := m.store.CreateNotebook(value); err != nil {
			m.setError(err)
			return m, nil
		}
		slug := notes.Slugify(value)
		m.reloadOrStatus()
		m.selectNotebook(slug)
		m.focus = paneSidebar
		m.setStatus(statusOK, "created notebook "+slug)

	case forNewTag:
		if err := m.store.CreateTag(o.notebook, value); err != nil {
			m.setError(err)
			return m, nil
		}
		slug := notes.Slugify(value)
		m.reloadOrStatus()
		m.selectTag(slug)
		m.setStatus(statusOK, "created tag "+slug)

	case forNewNote:
		return m.createNote(o.notebook, o.tag, value)

	case forRenameNotebook:
		slug, err := m.store.RenameNotebook(o.notebook, value)
		if err != nil {
			m.setError(err)
			return m, nil
		}
		m.reloadOrStatus()
		m.selectNotebook(slug)
		m.setStatus(statusOK, "renamed notebook to "+slug)

	case forRenameTag:
		slug, err := m.store.RenameTag(o.notebook, o.tag, value)
		if err != nil {
			m.setError(err)
			return m, nil
		}
		m.reloadOrStatus()
		m.selectTag(slug)
		m.setStatus(statusOK, "renamed tag to "+slug)

	case forRenameNote:
		renamed, err := m.store.RenameNote(o.note, value)
		if err != nil {
			m.setError(err)
			return m, nil
		}
		m.reloadOrStatus()
		m.selectNote(renamed.Name, renamed.Tag)
		m.setStatus(statusOK, "renamed note to "+renamed.Name)
	}
	return m, nil
}

func (m *Model) applyPick(o overlay, value string) {
	switch o.purpose {
	case forPickNewNoteTag:
		m.pendingTag = value
		m.ov = newPromptOverlay(forNewNote, "")
		m.ov.notebook, m.ov.tag, m.ov.back = o.notebook, value, true

	case forPickRenameTag:
		m.ov = newPromptOverlay(forRenameTag, value)
		m.ov.notebook, m.ov.tag = o.notebook, value

	case forPickDeleteTag:
		m.ov = overlay{kind: ovConfirm, purpose: forDeleteTag, notebook: o.notebook, tag: value}

	case forMoveNote:
		nb, tag, ok := strings.Cut(value, "/")
		if !ok {
			return
		}
		moved, err := m.store.MoveNote(o.note, nb, tag)
		if err != nil {
			m.setError(err)
			return
		}
		m.reloadOrStatus()
		m.revealNote(moved)
		m.setStatus(statusOK, "moved to "+moved.Display())
	}
}

func (m *Model) applyDelete(o overlay) {
	var err error
	var what string
	switch o.purpose {
	case forDeleteNotebook:
		what = "notebook " + o.notebook
		err = m.store.DeleteNotebook(o.notebook)
	case forDeleteTag:
		what = "tag " + o.tag
		err = m.store.DeleteTag(o.notebook, o.tag)
		m.tagCur = 0
	case forDeleteNote:
		what = "note " + o.note.Name
		err = m.store.DeleteNote(o.note)
	default:
		err = errors.New("nothing to delete")
	}
	if err != nil {
		m.setError(err)
		return
	}
	m.reloadOrStatus()
	if len(m.notebooks) == 0 {
		m.focus = paneSidebar
	}
	m.setStatus(statusOK, "deleted "+what)
}

func (m Model) createNote(nb, tag, title string) (tea.Model, tea.Cmd) {
	path, err := m.store.CreateNote(nb, tag, title)
	if err != nil {
		m.setError(err)
		return m, nil
	}
	m.pendingTag = ""
	m.reloadOrStatus()
	name := strings.TrimSuffix(filepath.Base(path), ".typ")
	m.selectNotebook(nb)
	m.selectTag(tag)
	m.selectNote(name, tag)
	m.focus = paneNotes
	return m.openNote(notes.Note{Notebook: nb, Tag: tag, Name: name, Path: path})
}
