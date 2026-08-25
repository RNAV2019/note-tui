package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/RNAV2019/note/internal/config"
	"github.com/RNAV2019/note/internal/notes"
)

// newTestModel builds a model over a temp note tree:
//
//	scratch/misc/idea
//	uni/algos/{b-trees,hashing}
//	uni/networks/tcp
func newTestModel(t *testing.T) Model {
	t.Helper()
	store := notes.NewStore(t.TempDir())
	if err := store.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	for _, nb := range []string{"scratch", "uni"} {
		if err := store.CreateNotebook(nb); err != nil {
			t.Fatal(err)
		}
	}
	seed := []struct{ nb, tag, name string }{
		{"scratch", "misc", "idea"},
		{"uni", "algos", "b-trees"},
		{"uni", "algos", "hashing"},
		{"uni", "networks", "tcp"},
	}
	for _, s := range seed {
		store.CreateTag(s.nb, s.tag)
		if _, err := store.CreateNote(s.nb, s.tag, s.name); err != nil {
			t.Fatal(err)
		}
	}
	m, err := New(store, config.Config{Editor: "true", Preview: ""}, "test")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// key builds a synthetic key press. Bubble Tea never runs in these tests.
func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		r := []rune(s)[0]
		return tea.KeyPressMsg{Code: r, Text: string(r)}
	}
}

// press feeds keys through Update and returns the resulting model.
func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		next, _ := m.Update(key(k))
		got, ok := next.(Model)
		if !ok {
			t.Fatalf("Update returned %T, want Model", next)
		}
		m = got
	}
	return m
}

func TestInitialStateLoadsTree(t *testing.T) {
	m := newTestModel(t)
	if len(m.all) != 4 {
		t.Errorf("all = %d notes, want 4", len(m.all))
	}
	if len(m.notebooks) != 2 {
		t.Errorf("notebooks = %v, want 2", m.notebooks)
	}
	// The first notebook is selected with the "all" tag row, so its whole
	// contents are visible.
	if m.currentNotebook() != "scratch" {
		t.Errorf("currentNotebook = %q, want scratch", m.currentNotebook())
	}
	if m.selectedTag() != "" {
		t.Errorf("selectedTag = %q, want the all row", m.selectedTag())
	}
	if len(m.visible) != 1 {
		t.Errorf("visible = %d notes, want 1", len(m.visible))
	}
}

func TestTabCyclesPanes(t *testing.T) {
	m := newTestModel(t)
	if m.focus != paneNotebooks {
		t.Fatalf("focus = %v, want paneNotebooks", m.focus)
	}
	for _, want := range []pane{paneTags, paneNotes, paneNotebooks} {
		m = press(t, m, "tab")
		if m.focus != want {
			t.Fatalf("after tab focus = %v, want %v", m.focus, want)
		}
	}
	// shift+tab is a distinct key string, so exercise the backwards path too.
	m, _ = func() (Model, tea.Cmd) {
		next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
		return next.(Model), cmd
	}()
	if m.focus != paneNotes {
		t.Errorf("after shift+tab focus = %v, want paneNotes", m.focus)
	}
}

func TestMovingNotebookCursorReloadsTags(t *testing.T) {
	m := newTestModel(t)
	if got := len(m.tags); got != 1 {
		t.Fatalf("scratch tags = %d, want 1", got)
	}
	m = press(t, m, "j") // scratch -> uni
	if m.currentNotebook() != "uni" {
		t.Fatalf("currentNotebook = %q, want uni", m.currentNotebook())
	}
	if len(m.tags) != 2 {
		t.Errorf("uni tags = %v, want algos and networks", m.tags)
	}
	if len(m.visible) != 3 {
		t.Errorf("visible = %d, want all 3 uni notes", len(m.visible))
	}
}

func TestSelectingATagFiltersNotes(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "j")        // uni
	m = press(t, m, "tab", "j") // tags pane, "all" -> algos
	if m.selectedTag() != "algos" {
		t.Fatalf("selectedTag = %q, want algos", m.selectedTag())
	}
	if len(m.visible) != 2 {
		t.Errorf("visible = %d, want 2 algos notes", len(m.visible))
	}
	m = press(t, m, "j") // algos -> networks
	if m.selectedTag() != "networks" || len(m.visible) != 1 {
		t.Errorf("tag = %q with %d notes, want networks with 1", m.selectedTag(), len(m.visible))
	}
}

func TestCursorsClampAtListEnds(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "k", "k", "k") // already at the top
	if m.nbCur != 0 {
		t.Errorf("nbCur = %d, want 0", m.nbCur)
	}
	m = press(t, m, "G")
	if m.nbCur != len(m.notebooks)-1 {
		t.Errorf("nbCur = %d, want %d", m.nbCur, len(m.notebooks)-1)
	}
	m = press(t, m, "j", "j", "j")
	if m.nbCur != len(m.notebooks)-1 {
		t.Errorf("nbCur overran to %d", m.nbCur)
	}
	m = press(t, m, "g")
	if m.nbCur != 0 {
		t.Errorf("nbCur = %d after g, want 0", m.nbCur)
	}
}

func TestEnterFocusesNotesFromSidebar(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "enter")
	if m.focus != paneNotes {
		t.Errorf("focus = %v, want paneNotes", m.focus)
	}
}

func TestSearchOverlayOpensAndEscapes(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "/")
	if !m.ov.active() || m.ov.kind != overlayList {
		t.Fatalf("expected a list overlay, got kind %v active=%v", m.ov.kind, m.ov.active())
	}
	if len(m.ov.items) != 4 {
		t.Errorf("search items = %d, want every note", len(m.ov.items))
	}
	m = press(t, m, "esc")
	if m.ov.active() {
		t.Error("esc did not close the overlay")
	}
}

func TestOverlaySwallowsNavigationKeys(t *testing.T) {
	m := newTestModel(t)
	before := m.nbCur
	m = press(t, m, "/", "j") // "j" must type into the filter, not move the sidebar
	if m.nbCur != before {
		t.Errorf("nbCur moved to %d while an overlay was open", m.nbCur)
	}
	if m.ov.input.Value() != "j" {
		t.Errorf("filter value = %q, want %q", m.ov.input.Value(), "j")
	}
}

func TestHelpOverlayToggles(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "?")
	if m.ov.kind != overlayHelp {
		t.Fatalf("kind = %v, want overlayHelp", m.ov.kind)
	}
	m = press(t, m, "q") // any key dismisses help
	if m.ov.active() {
		t.Error("help overlay did not dismiss")
	}
}

func TestNewNoteSkipsTagPromptWhenTagSelected(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "j", "tab", "j") // uni, then the algos tag
	m = press(t, m, "tab", "n")      // notes pane, new note
	if m.ov.kind != overlayPrompt {
		t.Errorf("kind = %v, want a title prompt straight away", m.ov.kind)
	}
	if m.pendingTag != "algos" {
		t.Errorf("pendingTag = %q, want algos", m.pendingTag)
	}
}

func TestNewNoteAsksForTagFromTheAllView(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "j") // uni, tag row still "all"
	m = press(t, m, "tab", "tab", "n")
	if m.ov.kind != overlayList {
		t.Errorf("kind = %v, want a tag picker", m.ov.kind)
	}
}

func TestRenameAndDeleteRejectTheAllRow(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "tab") // tags pane, "all" selected
	m = press(t, m, "r")
	if m.ov.active() {
		t.Error("rename should refuse the synthetic all row")
	}
	if !m.statusErr {
		t.Error("expected an error status explaining why")
	}
	m = press(t, m, "d")
	if m.ov.active() {
		t.Error("delete should refuse the synthetic all row")
	}
}

func TestDeleteNoteFlow(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "tab", "tab") // notes pane, scratch/misc/idea
	m = press(t, m, "d")
	if m.ov.kind != overlayConfirm {
		t.Fatalf("kind = %v, want a confirmation", m.ov.kind)
	}

	// Confirming emits an action, which Update then applies.
	next, cmd := m.Update(key("y"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("confirming produced no command")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)

	if len(m.all) != 3 {
		t.Errorf("all = %d notes, want 3 after deleting one", len(m.all))
	}
	if m.statusErr {
		t.Errorf("unexpected error status: %s", m.status)
	}
}

func TestRenameNotebookFlow(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "r")
	if m.ov.kind != overlayPrompt {
		t.Fatalf("kind = %v, want a prompt", m.ov.kind)
	}
	if m.ov.input.Value() != "scratch" {
		t.Errorf("prompt prefilled with %q, want scratch", m.ov.input.Value())
	}

	m.ov.input.SetValue("Year 1")
	next, cmd := m.Update(key("enter"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)

	if m.currentNotebook() != "year-1" {
		t.Errorf("currentNotebook = %q, want the renamed and slugified year-1", m.currentNotebook())
	}
}

// The preview window can only open if the configured URL reaches the session.
func TestOpenNotePassesPreviewConfigThrough(t *testing.T) {
	m := newTestModel(t)
	want := config.Config{
		Editor:         "hx",
		Preview:        "helium --app={url} --user-data-dir={profile}",
		PreviewURL:     "http://127.0.0.1:23635",
		PreviewProfile: "/tmp/note-preview-profile",
	}
	m.cfg = want
	m = press(t, m, "tab", "tab") // notes pane

	n, ok := m.currentNote()
	if !ok {
		t.Fatal("no note selected")
	}
	sess := m.newEditSession(n)
	if sess.file != n.Path {
		t.Errorf("file = %q, want %q", sess.file, n.Path)
	}
	got := sess.opts
	if got.Editor != want.Editor || got.Preview != want.Preview ||
		got.PreviewURL != want.PreviewURL || got.PreviewProfile != want.PreviewProfile {
		t.Errorf("opts = %+v, want the configured values %+v", got, want)
	}
}

func TestViewRendersAtManySizes(t *testing.T) {
	m := newTestModel(t)
	for _, size := range [][2]int{{100, 30}, {80, 24}, {60, 20}, {40, 10}, {20, 6}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		got := next.(Model)
		if v := got.View(); v.Content == "" {
			t.Errorf("empty view at %dx%d", size[0], size[1])
		}
		// And with an overlay open, which takes a different render path.
		withOverlay := press(t, got, "?")
		if v := withOverlay.View(); v.Content == "" {
			t.Errorf("empty overlay view at %dx%d", size[0], size[1])
		}
	}
}
