package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s := NewStore(t.TempDir())
	if err := s.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestBootstrapCreatesRepoAndTemplate(t *testing.T) {
	s := newTestStore(t)
	if _, err := os.Stat(filepath.Join(s.Root, ".git")); err != nil {
		t.Error("expected git repo at root")
	}
	data, err := os.ReadFile(filepath.Join(s.Root, ".template.typ"))
	if err != nil || len(data) == 0 {
		t.Error("expected non-empty .template.typ")
	}
	// Bootstrap must be idempotent and must not overwrite an edited template.
	os.WriteFile(filepath.Join(s.Root, ".template.typ"), []byte("custom"), 0o644)
	if err := s.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(s.Root, ".template.typ"))
	if string(data) != "custom" {
		t.Error("Bootstrap overwrote user template")
	}
}

func TestNotebookAndTagLifecycle(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateNotebook("University Y2"); err != nil {
		t.Fatal(err)
	}
	nbs, _ := s.Notebooks()
	if len(nbs) != 1 || nbs[0] != "university-y2" {
		t.Fatalf("Notebooks() = %v", nbs)
	}
	if err := s.CreateNotebook("University Y2"); err == nil {
		t.Error("duplicate notebook should error")
	}
	if err := s.CreateTag("university-y2", "Algorithms"); err != nil {
		t.Fatal(err)
	}
	tags, _ := s.Tags("university-y2")
	if len(tags) != 1 || tags[0] != "algorithms" {
		t.Fatalf("Tags() = %v", tags)
	}
	if err := s.DeleteTag("university-y2", "algorithms"); err != nil {
		t.Fatal(err)
	}
	if tags, _ := s.Tags("university-y2"); len(tags) != 0 {
		t.Error("tag not deleted")
	}
	if err := s.DeleteNotebook("university-y2"); err != nil {
		t.Fatal(err)
	}
	if nbs, _ := s.Notebooks(); len(nbs) != 0 {
		t.Error("notebook not deleted")
	}
}

func TestCreateAndListNotes(t *testing.T) {
	s := newTestStore(t)
	s.CreateNotebook("uni")
	s.CreateTag("uni", "algos")
	path, err := s.CreateNote("uni", "algos", "B-Trees")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "b-trees.typ" {
		t.Errorf("path = %q", path)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "B-Trees") || !strings.Contains(string(data), "algos") {
		t.Error("note content missing rendered title/tag")
	}
	if _, err := s.CreateNote("uni", "algos", "B-Trees"); err == nil {
		t.Error("duplicate note should error")
	}
	all, _ := s.AllNotes()
	if len(all) != 1 {
		t.Fatalf("AllNotes() = %v", all)
	}
	n := all[0]
	if n.Notebook != "uni" || n.Tag != "algos" || n.Name != "b-trees" || n.Path != path {
		t.Errorf("note = %+v", n)
	}
	if got := n.Display(); got != "uni/algos/b-trees" {
		t.Errorf("Display() = %q", got)
	}
}

func TestNoteCountAndDeleteNote(t *testing.T) {
	s := newTestStore(t)
	s.CreateNotebook("uni")
	s.CreateTag("uni", "algos")
	s.CreateTag("uni", "networks")
	s.CreateNote("uni", "algos", "one")
	s.CreateNote("uni", "algos", "two")
	s.CreateNote("uni", "networks", "three")
	if n, _ := s.NoteCount("uni", ""); n != 3 {
		t.Errorf("notebook count = %d", n)
	}
	if n, _ := s.NoteCount("uni", "algos"); n != 2 {
		t.Errorf("tag count = %d", n)
	}
	all, _ := s.AllNotes()
	if err := s.DeleteNote(all[0]); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.NoteCount("uni", ""); n != 2 {
		t.Error("note not deleted")
	}
}

func TestInvalidNames(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateNotebook("!!!"); err == nil {
		t.Error("unsluggable notebook name should error")
	}
}
