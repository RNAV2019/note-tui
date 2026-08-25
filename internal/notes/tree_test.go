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

func TestDeleteNote(t *testing.T) {
	s := newTestStore(t)
	s.CreateNotebook("uni")
	s.CreateTag("uni", "algos")
	s.CreateTag("uni", "networks")
	s.CreateNote("uni", "algos", "one")
	s.CreateNote("uni", "algos", "two")
	s.CreateNote("uni", "networks", "three")
	all, _ := s.AllNotes()
	if len(all) != 3 {
		t.Fatalf("AllNotes() = %d notes, want 3", len(all))
	}
	if err := s.DeleteNote(all[0]); err != nil {
		t.Fatal(err)
	}
	if all, _ := s.AllNotes(); len(all) != 2 {
		t.Errorf("after delete = %d notes, want 2", len(all))
	}
}

func TestAllNotesPopulatesModTime(t *testing.T) {
	s := newTestStore(t)
	s.CreateNotebook("uni")
	s.CreateTag("uni", "algos")
	s.CreateNote("uni", "algos", "one")
	all, _ := s.AllNotes()
	if all[0].ModTime.IsZero() {
		t.Error("ModTime not populated")
	}
}

// seedNote returns a store with uni/algos/b-trees.typ plus an empty uni/networks.
func seedNote(t *testing.T) (*Store, Note) {
	t.Helper()
	s := newTestStore(t)
	s.CreateNotebook("uni")
	s.CreateTag("uni", "algos")
	s.CreateTag("uni", "networks")
	if _, err := s.CreateNote("uni", "algos", "B-Trees"); err != nil {
		t.Fatal(err)
	}
	all, _ := s.AllNotes()
	return s, all[0]
}

func TestRenameNote(t *testing.T) {
	tests := []struct {
		name    string
		newName string
		want    string
		wantErr bool
	}{
		{"slugifies", "Hash Tables", "hash-tables", false},
		{"same name is a no-op", "b-trees", "b-trees", false},
		{"unsluggable name", "!!!", "b-trees", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, n := seedNote(t)
			got, err := s.RenameNote(n, tt.newName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got.Name != tt.want {
				t.Errorf("Name = %q, want %q", got.Name, tt.want)
			}
			if _, err := os.Stat(got.Path); err != nil {
				t.Errorf("returned Path does not exist: %v", err)
			}
		})
	}
}

func TestRenameNoteRejectsCollision(t *testing.T) {
	s, n := seedNote(t)
	s.CreateNote("uni", "algos", "hash-tables")
	if _, err := s.RenameNote(n, "hash-tables"); err == nil {
		t.Fatal("rename onto existing note should error")
	}
	if _, err := os.Stat(n.Path); err != nil {
		t.Error("original note lost after failed rename")
	}
}

func TestMoveNote(t *testing.T) {
	s, n := seedNote(t)
	s.CreateNotebook("scratch")
	s.CreateTag("scratch", "misc")

	// Retag within the same notebook.
	moved, err := s.MoveNote(n, "uni", "networks")
	if err != nil {
		t.Fatal(err)
	}
	if moved.Tag != "networks" {
		t.Errorf("Tag = %q, want networks", moved.Tag)
	}
	if _, err := os.Stat(n.Path); !os.IsNotExist(err) {
		t.Error("note left behind at old path")
	}

	// Across notebooks.
	moved, err = s.MoveNote(moved, "scratch", "misc")
	if err != nil {
		t.Fatal(err)
	}
	if moved.Notebook != "scratch" || moved.Tag != "misc" {
		t.Errorf("moved = %+v", moved)
	}
	if _, err := os.Stat(moved.Path); err != nil {
		t.Errorf("moved note missing: %v", err)
	}
}

func TestMoveNoteErrors(t *testing.T) {
	s, n := seedNote(t)
	if _, err := s.MoveNote(n, "uni", "nonexistent"); err == nil {
		t.Error("move to missing tag should error")
	}
	s.CreateNote("uni", "networks", "b-trees")
	if _, err := s.MoveNote(n, "uni", "networks"); err == nil {
		t.Error("move onto existing note should error")
	}
	if _, err := os.Stat(n.Path); err != nil {
		t.Error("original note lost after failed move")
	}
}

func TestRenameNotebookAndTag(t *testing.T) {
	s, _ := seedNote(t)

	slug, err := s.RenameTag("uni", "algos", "Data Structures")
	if err != nil {
		t.Fatal(err)
	}
	if slug != "data-structures" {
		t.Fatalf("tag slug = %q", slug)
	}

	slug, err = s.RenameNotebook("uni", "Year 1")
	if err != nil {
		t.Fatal(err)
	}
	if slug != "year-1" {
		t.Fatalf("notebook slug = %q", slug)
	}

	// The note must have travelled with both directories.
	all, _ := s.AllNotes()
	if len(all) != 1 {
		t.Fatalf("AllNotes() = %v", all)
	}
	if got := all[0].Display(); got != "year-1/data-structures/b-trees" {
		t.Errorf("Display() = %q", got)
	}
}

func TestRenameDirErrors(t *testing.T) {
	s, _ := seedNote(t)
	if _, err := s.RenameNotebook("uni", "!!!"); err == nil {
		t.Error("unsluggable notebook rename should error")
	}
	if _, err := s.RenameNotebook("nonexistent", "other"); err == nil {
		t.Error("renaming a missing notebook should error")
	}
	s.CreateNotebook("scratch")
	if _, err := s.RenameNotebook("uni", "scratch"); err == nil {
		t.Error("rename onto existing notebook should error")
	}
	if _, err := s.RenameTag("uni", "algos", "networks"); err == nil {
		t.Error("rename onto existing tag should error")
	}
}

func TestInvalidNames(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateNotebook("!!!"); err == nil {
		t.Error("unsluggable notebook name should error")
	}
	s.CreateNotebook("uni")
	if err := s.CreateTag("uni", "!!!"); err == nil {
		t.Error("unsluggable tag name should error")
	}
	if _, err := s.CreateNote("uni", "algos", "!!!"); err == nil {
		t.Error("unsluggable note name should error")
	}
}

func TestCreateTagRequiresNotebook(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateTag("nonexistent", "algos"); err == nil {
		t.Error("tag in missing notebook should error, not create phantom notebook")
	}
	if nbs, _ := s.Notebooks(); len(nbs) != 0 {
		t.Errorf("phantom notebook created: %v", nbs)
	}
}
