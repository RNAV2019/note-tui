package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/RNAV2019/note/internal/notes"
)

type actionKind int

const (
	actNewNotebook actionKind = iota
	actNewTag
	actPickNoteTag // first step of the two-step new-note flow
	actNewNote
	actRename
	actDelete
	actMoveNote
	actOpenDisplay // open the note matching a "notebook/tag/name" string
)

// actionMsg carries a resolved overlay choice back into Update, so the
// overlay callbacks stay pure and all mutation happens in one place.
type actionMsg struct {
	kind  actionKind
	value string
}

func act(kind actionKind) func(string) tea.Cmd {
	return func(value string) tea.Cmd {
		return func() tea.Msg { return actionMsg{kind: kind, value: value} }
	}
}

// ---------------------------------------------------------------- triggers

func (m Model) startNew() (tea.Model, tea.Cmd) {
	nb := m.currentNotebook()

	switch m.focus {
	case paneNotebooks:
		m.ov = newPrompt("New notebook", "", act(actNewNotebook))

	case paneTags:
		if nb == "" {
			m.setStatus("", fmt.Errorf("create a notebook first"))
			break
		}
		m.ov = newPrompt("New tag in "+nb, "", act(actNewTag))

	case paneNotes:
		if nb == "" {
			m.setStatus("", fmt.Errorf("create a notebook first"))
			break
		}
		if len(m.tags) == 0 {
			m.setStatus("", fmt.Errorf("create a tag first — press n in the Tags pane"))
			break
		}
		// When a concrete tag is already selected, skip straight to the title;
		// only the ambiguous "all" view has to ask which tag.
		if tag := m.selectedTag(); tag != "" {
			m.pendingTag = tag
			m.ov = newPrompt("New note in "+nb+"/"+tag, "", act(actNewNote))
			break
		}
		m.ov = newList("New note · pick a tag", m.tags, act(actPickNoteTag))
	}
	return m, nil
}

func (m Model) startRename() (tea.Model, tea.Cmd) {
	switch m.focus {
	case paneNotebooks:
		nb := m.currentNotebook()
		if nb == "" {
			return m, nil
		}
		m.ov = newPrompt("Rename notebook", nb, act(actRename))

	case paneTags:
		tag := m.selectedTag()
		if tag == "" {
			m.setStatus("", fmt.Errorf(`select a tag to rename ("all" is not a tag)`))
			return m, nil
		}
		m.ov = newPrompt("Rename tag", tag, act(actRename))

	case paneNotes:
		n, ok := m.currentNote()
		if !ok {
			return m, nil
		}
		m.ov = newPrompt("Rename note", n.Name, act(actRename))
	}
	return m, nil
}

func (m Model) startDelete() (tea.Model, tea.Cmd) {
	confirm := func() tea.Cmd { return act(actDelete)("") }

	switch m.focus {
	case paneNotebooks:
		nb := m.currentNotebook()
		if nb == "" {
			return m, nil
		}
		m.ov = newConfirm(fmt.Sprintf("Delete notebook %q and all %d notes in it?",
			nb, m.notebookCount(nb)), confirm)

	case paneTags:
		tag := m.selectedTag()
		if tag == "" {
			m.setStatus("", fmt.Errorf(`select a tag to delete ("all" is not a tag)`))
			return m, nil
		}
		m.ov = newConfirm(fmt.Sprintf("Delete tag %q and all %d notes in it?",
			tag, m.tagCount(m.currentNotebook(), tag)), confirm)

	case paneNotes:
		n, ok := m.currentNote()
		if !ok {
			return m, nil
		}
		m.ov = newConfirm(fmt.Sprintf("Delete note %q?", n.Display()), confirm)
	}
	return m, nil
}

func (m Model) startMove() (tea.Model, tea.Cmd) {
	n, ok := m.currentNote()
	if !ok {
		return m, nil
	}
	dests, err := m.destinations()
	if err != nil {
		m.setStatus("", err)
		return m, nil
	}
	// Drop the note's current home; moving there is a no-op.
	here := n.Notebook + "/" + n.Tag
	filtered := make([]string, 0, len(dests))
	for _, d := range dests {
		if d != here {
			filtered = append(filtered, d)
		}
	}
	if len(filtered) == 0 {
		m.setStatus("", fmt.Errorf("no other tag to move %q into", n.Name))
		return m, nil
	}
	m.ov = newList("Move "+n.Name+" to", filtered, act(actMoveNote))
	return m, nil
}

// destinations lists every "notebook/tag" pair a note could live in.
func (m Model) destinations() ([]string, error) {
	var out []string
	for _, nb := range m.notebooks {
		tags, err := m.store.Tags(nb)
		if err != nil {
			return nil, err
		}
		for _, t := range tags {
			out = append(out, nb+"/"+t)
		}
	}
	return out, nil
}

func (m Model) startSearch() Model {
	items := make([]string, len(m.all))
	for i, n := range m.all {
		items[i] = n.Display()
	}
	m.ov = newList("Search notes", items, act(actOpenDisplay))
	return m
}

// ---------------------------------------------------------------- apply

func (m Model) applyAction(msg actionMsg) (tea.Model, tea.Cmd) {
	nb := m.currentNotebook()

	switch msg.kind {
	case actNewNotebook:
		if err := m.store.CreateNotebook(msg.value); err != nil {
			m.setStatus("", err)
			return m, nil
		}
		m.reloadOrStatus()
		m.selectNotebook(notes.Slugify(msg.value))
		m.setStatus("created notebook "+notes.Slugify(msg.value), nil)

	case actNewTag:
		if err := m.store.CreateTag(nb, msg.value); err != nil {
			m.setStatus("", err)
			return m, nil
		}
		m.reloadOrStatus()
		m.selectTag(notes.Slugify(msg.value))
		m.setStatus("created tag "+notes.Slugify(msg.value), nil)

	case actPickNoteTag:
		m.pendingTag = msg.value
		m.ov = newPrompt("New note in "+nb+"/"+msg.value, "", act(actNewNote))
		return m, nil

	case actNewNote:
		return m.createNote(nb, m.pendingTag, msg.value)

	case actRename:
		return m.rename(msg.value)

	case actDelete:
		return m.deleteSelected()

	case actMoveNote:
		return m.moveNote(msg.value)

	case actOpenDisplay:
		for _, n := range m.all {
			if n.Display() == msg.value {
				m.selectNotebook(n.Notebook)
				m.selectTag(n.Tag)
				m.selectNote(n.Name)
				m.focus = paneNotes
				return m.openNote(n)
			}
		}
	}
	return m, nil
}

func (m Model) createNote(nb, tag, title string) (tea.Model, tea.Cmd) {
	path, err := m.store.CreateNote(nb, tag, title)
	if err != nil {
		m.setStatus("", err)
		return m, nil
	}
	m.pendingTag = ""
	m.reloadOrStatus()
	name := strings.TrimSuffix(filepath.Base(path), ".typ")
	m.selectTag(tag)
	m.selectNote(name)
	m.focus = paneNotes
	return m.openNote(notes.Note{Notebook: nb, Tag: tag, Name: name, Path: path})
}

func (m Model) rename(value string) (tea.Model, tea.Cmd) {
	nb := m.currentNotebook()

	switch m.focus {
	case paneNotebooks:
		slug, err := m.store.RenameNotebook(nb, value)
		if err != nil {
			m.setStatus("", err)
			return m, nil
		}
		m.reloadOrStatus()
		m.selectNotebook(slug)
		m.setStatus("renamed notebook to "+slug, nil)

	case paneTags:
		old := m.selectedTag()
		slug, err := m.store.RenameTag(nb, old, value)
		if err != nil {
			m.setStatus("", err)
			return m, nil
		}
		m.reloadOrStatus()
		m.selectTag(slug)
		m.setStatus("renamed tag to "+slug, nil)

	case paneNotes:
		n, ok := m.currentNote()
		if !ok {
			return m, nil
		}
		renamed, err := m.store.RenameNote(n, value)
		if err != nil {
			m.setStatus("", err)
			return m, nil
		}
		m.reloadOrStatus()
		m.selectNote(renamed.Name)
		m.setStatus("renamed note to "+renamed.Name, nil)
	}
	return m, nil
}

func (m Model) deleteSelected() (tea.Model, tea.Cmd) {
	nb := m.currentNotebook()

	var err error
	var what string
	switch m.focus {
	case paneNotebooks:
		what = "notebook " + nb
		err = m.store.DeleteNotebook(nb)
	case paneTags:
		tag := m.selectedTag()
		what = "tag " + tag
		err = m.store.DeleteTag(nb, tag)
		m.tagCur = 0
	case paneNotes:
		n, ok := m.currentNote()
		if !ok {
			return m, nil
		}
		what = "note " + n.Name
		err = m.store.DeleteNote(n)
	}
	if err != nil {
		m.setStatus("", err)
		return m, nil
	}
	m.reloadOrStatus()
	m.setStatus("deleted "+what, nil)
	return m, nil
}

func (m Model) moveNote(dest string) (tea.Model, tea.Cmd) {
	n, ok := m.currentNote()
	if !ok {
		return m, nil
	}
	nb, tag, found := strings.Cut(dest, "/")
	if !found {
		return m, nil
	}
	moved, err := m.store.MoveNote(n, nb, tag)
	if err != nil {
		m.setStatus("", err)
		return m, nil
	}
	m.reloadOrStatus()
	m.selectNotebook(moved.Notebook)
	m.selectTag(moved.Tag)
	m.selectNote(moved.Name)
	m.setStatus("moved to "+moved.Display(), nil)
	return m, nil
}

// ---------------------------------------------------------------- selection

func (m *Model) selectNotebook(name string) {
	for i, nb := range m.notebooks {
		if nb == name {
			m.nbCur, m.tagCur = i, 0
			m.refreshTagsOrStatus()
			return
		}
	}
}

func (m *Model) selectTag(name string) {
	for i, t := range m.tags {
		if t == name {
			m.tagCur = i + 1
			m.refreshVisible()
			return
		}
	}
}

func (m *Model) selectNote(name string) {
	for i, n := range m.visible {
		if n.Name == name {
			m.noteCur = i
			return
		}
	}
}

// backupCmd commits and pushes, reporting whether a push actually happened.
func (m Model) backupCmd() tea.Cmd {
	return func() tea.Msg {
		pushed, err := m.store.Backup("backup: " + time.Now().Format("2006-01-02 15:04"))
		if err != nil {
			return gitDoneMsg{err: err}
		}
		if pushed {
			return gitDoneMsg{text: "backed up and pushed"}
		}
		return gitDoneMsg{text: "backed up locally (no remote configured)"}
	}
}
