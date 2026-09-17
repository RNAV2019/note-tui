package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/RNAV2019/note-tui/internal/config"
	"github.com/RNAV2019/note-tui/internal/notes"
)

// seedTime is the modification time of the first seeded note; each later one
// is a day newer, so "newest first" is deterministic.
var seedTime = time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)

// newTestModel builds a model over a temp note tree:
//
//	scratch/misc/idea
//	uni/algos/{b-trees,hashing}
//	uni/networks/tcp
//
// The sidebar starts on scratch; the notes pane has focus.
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
	for i, s := range seed {
		if err := store.CreateTag(s.nb, s.tag); err != nil && !strings.Contains(err.Error(), "exists") {
			t.Fatal(err)
		}
		path, err := store.CreateNote(s.nb, s.tag, s.name)
		if err != nil {
			t.Fatal(err)
		}
		at := seedTime.AddDate(0, 0, i)
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	m, err := New(store, config.Config{Editor: "true", Preview: ""}, "test")
	if err != nil {
		t.Fatal(err)
	}
	// A fixed clock keeps the rendered "3d ago" ages stable.
	nowFunc = func() time.Time { return seedTime.AddDate(0, 0, 7) }
	t.Cleanup(func() { nowFunc = time.Now })
	return m
}

// key builds a synthetic key press. Bubble Tea never runs in these tests.
func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "up", "down":
		code := tea.KeyUp
		if s == "down" {
			code = tea.KeyDown
		}
		return tea.KeyPressMsg{Code: code}
	}
	if rest, ok := strings.CutPrefix(s, "ctrl+"); ok {
		return tea.KeyPressMsg{Code: []rune(rest)[0], Mod: tea.ModCtrl}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// press feeds keys through Update and returns the resulting model.
func press(t *testing.T, m Model, ks ...string) Model {
	t.Helper()
	for _, k := range ks {
		next, _ := m.Update(key(k))
		got, ok := next.(Model)
		if !ok {
			t.Fatalf("Update returned %T, want Model", next)
		}
		m = got
	}
	return m
}

// typeText presses each rune of s in turn, for filling in prompts.
func typeText(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		m = press(t, m, string(r))
	}
	return m
}

func sized(t *testing.T, m Model, w, h int) Model {
	t.Helper()
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(Model)
}

// ---------------------------------------------------------------- state

func TestInitialStateLoadsTree(t *testing.T) {
	m := newTestModel(t)
	if len(m.all) != 4 {
		t.Errorf("all = %d notes, want 4", len(m.all))
	}
	if len(m.notebooks) != 2 {
		t.Errorf("notebooks = %v, want 2", m.notebooks)
	}
	if m.focus != paneNotes {
		t.Errorf("focus = %v, want the notes pane", m.focus)
	}
	if m.currentNotebook() != "scratch" {
		t.Errorf("currentNotebook = %q, want scratch", m.currentNotebook())
	}
	if m.selectedTag() != "" {
		t.Errorf("selectedTag = %q, want the all tab", m.selectedTag())
	}
	if len(m.visible) != 1 {
		t.Errorf("visible = %d notes, want 1", len(m.visible))
	}
}

func newEmptyStore(t *testing.T) *notes.Store {
	t.Helper()
	store := notes.NewStore(t.TempDir())
	if err := store.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	return store
}

func newModelOver(t *testing.T, store *notes.Store) Model {
	t.Helper()
	m, err := New(store, config.Config{Editor: "true"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestEmptyTreeFocusesTheSidebar(t *testing.T) {
	m := newModelOver(t, newEmptyStore(t))
	if m.focus != paneSidebar {
		t.Errorf("focus = %v, want the sidebar when there is nothing to list", m.focus)
	}
}

func TestNotesAreNewestFirst(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "tab", "j") // sidebar, then uni
	var got []string
	for _, n := range m.visible {
		got = append(got, n.Name)
	}
	want := []string{"tcp", "hashing", "b-trees"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("visible = %v, want %v", got, want)
	}
}

func TestTabSwitchesBetweenTheTwoPanes(t *testing.T) {
	m := newTestModel(t)
	for _, want := range []pane{paneSidebar, paneNotes, paneSidebar} {
		m = press(t, m, "tab")
		if m.focus != want {
			t.Fatalf("after tab focus = %v, want %v", m.focus, want)
		}
	}
	m = press(t, m, "shift+tab")
	if m.focus != paneNotes {
		t.Errorf("after shift+tab focus = %v, want the notes pane", m.focus)
	}
}

func TestNarrowTerminalPinsFocusToTheNotes(t *testing.T) {
	m := newTestModel(t)
	m = sized(t, m, 60, 20) // too narrow for a sidebar
	m = press(t, m, "tab", "h")
	if m.focus != paneNotes {
		t.Errorf("focus = %v, want the notes pane when the sidebar is hidden", m.focus)
	}
}

func TestMovingTheNotebookCursorReloadsTags(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "tab") // sidebar
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
	if m.tagCur != 0 {
		t.Errorf("tagCur = %d, want the all tab after switching notebook", m.tagCur)
	}
}

func TestTagTabsFilterNotes(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "tab", "j", "tab") // uni, back to the notes pane
	m = press(t, m, "]")
	if m.selectedTag() != "algos" || len(m.visible) != 2 {
		t.Errorf("] gave tag %q with %d notes, want algos with 2", m.selectedTag(), len(m.visible))
	}
	m = press(t, m, "]")
	if m.selectedTag() != "networks" || len(m.visible) != 1 {
		t.Errorf("] gave tag %q with %d notes, want networks with 1", m.selectedTag(), len(m.visible))
	}
	m = press(t, m, "]") // wraps back to "all"
	if m.selectedTag() != "" {
		t.Errorf("] from the last tag gave %q, want the all tab", m.selectedTag())
	}
	m = press(t, m, "[")
	if m.selectedTag() != "networks" {
		t.Errorf("[ gave %q, want networks", m.selectedTag())
	}
	m = press(t, m, "2")
	if m.selectedTag() != "algos" {
		t.Errorf("2 jumped to %q, want the first tag", m.selectedTag())
	}
	m = press(t, m, "9") // out of range: nothing moves
	if m.selectedTag() != "algos" {
		t.Errorf("9 jumped to %q, want to stay on algos", m.selectedTag())
	}
}

func TestCursorsClampAtListEnds(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "tab")         // sidebar
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

func TestEscNeverQuits(t *testing.T) {
	m := newTestModel(t)
	next, cmd := m.Update(key("esc"))
	if cmd != nil {
		t.Error("esc produced a command; it must never quit")
	}
	if _, ok := next.(Model); !ok {
		t.Fatal("esc did not return a model")
	}
}

func TestEnterFocusesTheNotesFromTheSidebar(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "tab", "enter")
	if m.focus != paneNotes {
		t.Errorf("focus = %v, want the notes pane", m.focus)
	}
}

func TestSelectingANoteMovesThePreview(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "tab", "j", "tab") // uni, notes pane
	first := m.prev.path
	m = press(t, m, "j")
	if m.prev.path == first || m.prev.path == "" {
		t.Errorf("preview stayed on %q after moving the cursor", m.prev.path)
	}
	n, _ := m.currentNote()
	if m.prev.path != n.Path {
		t.Errorf("preview = %q, want the selected note %q", m.prev.path, n.Path)
	}
}

// ---------------------------------------------------------------- overlays

func TestFinderOpensAndEscapes(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "/")
	if m.ov.kind != ovFinder {
		t.Fatalf("kind = %v, want the finder", m.ov.kind)
	}
	if len(m.ov.candidates) != 4 {
		t.Errorf("candidates = %d, want every note", len(m.ov.candidates))
	}
	m = press(t, m, "esc")
	if m.ov.active() {
		t.Error("esc did not close the finder")
	}
}

func TestFinderFiltersAcrossNotebooks(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "/")
	m = typeText(t, m, "tcp")
	if len(m.ov.matches) != 1 {
		t.Fatalf("matches = %d, want just uni/networks/tcp", len(m.ov.matches))
	}
	if got := m.ov.candidates[m.ov.selected()].Name; got != "tcp" {
		t.Errorf("selected %q, want tcp", got)
	}
	// Enter reveals the note in the main view even though it lives in a
	// different notebook from the one the sidebar was on.
	m = press(t, m, "enter")
	if m.currentNotebook() != "uni" {
		t.Errorf("currentNotebook = %q, want uni", m.currentNotebook())
	}
	if n, _ := m.currentNote(); n.Name != "tcp" {
		t.Errorf("selected note = %q, want tcp", n.Name)
	}
}

func TestFinderKeepsLettersAsText(t *testing.T) {
	m := newTestModel(t)
	before := m.noteCur
	m = press(t, m, "/", "j")
	if m.noteCur != before {
		t.Errorf("noteCur moved to %d while the finder was open", m.noteCur)
	}
	if m.ov.input.Value() != "j" {
		t.Errorf("filter value = %q, want %q", m.ov.input.Value(), "j")
	}
	m = press(t, m, "ctrl+n")
	if m.ov.cursor == 0 && len(m.ov.matches) > 1 {
		t.Error("ctrl+n did not move the selection")
	}
}

func TestFinderRenameActsOnTheHighlightedNote(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "/")
	m = typeText(t, m, "tcp")
	m = press(t, m, "ctrl+r")
	if m.ov.kind != ovPrompt || m.ov.purpose != forRenameNote {
		t.Fatalf("kind = %v purpose = %v, want a rename prompt", m.ov.kind, m.ov.purpose)
	}
	if m.ov.note.Name != "tcp" {
		t.Errorf("renaming %q, want tcp", m.ov.note.Name)
	}
}

func TestHelpTogglesOnAnyKey(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "?")
	if m.ov.kind != ovHelp {
		t.Fatalf("kind = %v, want the help screen", m.ov.kind)
	}
	m = press(t, m, "q") // any key dismisses help, including quit
	if m.ov.active() {
		t.Error("help did not dismiss")
	}
}

func TestLeaderMenuOpensTheTagSubmenu(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "space")
	if m.ov.kind != ovLeader || m.ov.sub != "" {
		t.Fatalf("kind = %v sub = %q, want the root menu", m.ov.kind, m.ov.sub)
	}
	m = press(t, m, "t")
	if m.ov.kind != ovLeader || m.ov.sub != "tag" {
		t.Fatalf("kind = %v sub = %q, want the tag submenu", m.ov.kind, m.ov.sub)
	}
	m = press(t, m, "backspace")
	if m.ov.sub != "" {
		t.Errorf("backspace left sub = %q, want the root menu", m.ov.sub)
	}
	m = press(t, m, "n")
	if m.ov.kind != ovPrompt || m.ov.purpose != forNewNote {
		t.Errorf("space n gave %v/%v, want a new-note prompt", m.ov.kind, m.ov.purpose)
	}
}

func TestLeaderTagNewOpensATagPrompt(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "space", "t", "n")
	if m.ov.kind != ovPrompt || m.ov.purpose != forNewTag {
		t.Fatalf("kind = %v purpose = %v, want a new-tag prompt", m.ov.kind, m.ov.purpose)
	}
	if m.ov.notebook != "scratch" {
		t.Errorf("notebook = %q, want scratch", m.ov.notebook)
	}
}

func TestNewNoteSkipsThePickerWhenATagTabIsActive(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "tab", "j", "tab") // uni
	m = press(t, m, "2")               // the algos tab
	m = press(t, m, "n")
	if m.ov.kind != ovPrompt || m.ov.purpose != forNewNote {
		t.Fatalf("kind = %v purpose = %v, want a title prompt straight away", m.ov.kind, m.ov.purpose)
	}
	if m.ov.tag != "algos" {
		t.Errorf("ov.tag = %q, want algos", m.ov.tag)
	}
}

func TestNewNoteAsksForATagFromTheAllTab(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "tab", "j", "tab") // uni, all tab
	m = press(t, m, "n")
	if m.ov.kind != ovPicker || m.ov.purpose != forPickNewNoteTag {
		t.Fatalf("kind = %v purpose = %v, want a tag picker", m.ov.kind, m.ov.purpose)
	}
	if len(m.ov.items) != 2 {
		t.Errorf("picker items = %d, want both uni tags", len(m.ov.items))
	}
	// Choosing a tag leads to the title prompt, and esc steps back.
	m = press(t, m, "enter")
	if m.ov.kind != ovPrompt || !m.ov.back {
		t.Fatalf("kind = %v back = %v, want a title prompt that can step back", m.ov.kind, m.ov.back)
	}
	m = press(t, m, "esc")
	if m.ov.kind != ovPicker {
		t.Errorf("esc gave %v, want the tag picker again", m.ov.kind)
	}
}

func TestNewNoteNeedsATagFirst(t *testing.T) {
	store := notes.NewStore(t.TempDir())
	if err := store.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateNotebook("empty"); err != nil {
		t.Fatal(err)
	}
	m, err := New(store, config.Config{Editor: "true"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	m.focus = paneNotes
	m = press(t, m, "n")
	if m.ov.active() {
		t.Error("a note was offered with no tag to put it in")
	}
	if m.statusLevel != statusErr || !strings.Contains(m.status, "tag") {
		t.Errorf("status = %q (level %v), want an explanation about tags", m.status, m.statusLevel)
	}
}

func TestDeleteNoteNeedsY(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "d")
	if m.ov.kind != ovConfirm || m.ov.purpose != forDeleteNote {
		t.Fatalf("kind = %v purpose = %v, want a confirmation", m.ov.kind, m.ov.purpose)
	}
	// Enter must not delete: it is too easy to hit by reflex.
	after := press(t, m, "enter")
	if !after.ov.active() || len(after.all) != 4 {
		t.Error("enter resolved the delete confirmation")
	}
	after = press(t, m, "n")
	if after.ov.active() || len(after.all) != 4 {
		t.Error("n did not cancel cleanly")
	}
	after = press(t, m, "y")
	if after.ov.active() {
		t.Error("y left the confirmation open")
	}
	if len(after.all) != 3 {
		t.Errorf("all = %d notes, want 3 after deleting one", len(after.all))
	}
	if after.statusLevel == statusErr {
		t.Errorf("unexpected error status: %s", after.status)
	}
}

func TestRenameNotebookFlow(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "tab", "r")
	if m.ov.kind != ovPrompt || m.ov.purpose != forRenameNotebook {
		t.Fatalf("kind = %v purpose = %v, want a rename prompt", m.ov.kind, m.ov.purpose)
	}
	if m.ov.input.Value() != "scratch" {
		t.Errorf("prompt prefilled with %q, want scratch", m.ov.input.Value())
	}
	m.ov.input.SetValue("Year 1")
	m = press(t, m, "enter")
	if m.currentNotebook() != "year-1" {
		t.Errorf("currentNotebook = %q, want the slugified year-1", m.currentNotebook())
	}
	if m.statusLevel != statusOK {
		t.Errorf("status = %q, want a confirmation", m.status)
	}
}

func TestEmptyPromptDoesNothing(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "tab", "n") // new notebook
	m.ov.input.SetValue("   ")
	m = press(t, m, "enter")
	if m.ov.active() {
		t.Error("prompt stayed open")
	}
	if len(m.notebooks) != 2 {
		t.Errorf("notebooks = %v, want no new one", m.notebooks)
	}
}

func TestMoveNoteListsOtherTagsOnly(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "tab", "j", "tab") // uni, all tab
	m = press(t, m, "2")               // algos
	m = press(t, m, "m")
	if m.ov.kind != ovPicker || m.ov.purpose != forMoveNote {
		t.Fatalf("kind = %v purpose = %v, want a move picker", m.ov.kind, m.ov.purpose)
	}
	for _, it := range m.ov.items {
		if it.value == "uni/algos" {
			t.Errorf("picker offered the note's own tag %q", it.value)
		}
	}
	m = typeText(t, m, "networks")
	m = press(t, m, "enter")
	n, ok := m.currentNote()
	if !ok || n.Tag != "networks" {
		t.Errorf("note landed in %q, want networks", n.Tag)
	}
	if m.statusLevel != statusOK {
		t.Errorf("status = %q, want a confirmation", m.status)
	}
}

// ---------------------------------------------------------------- editor

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
