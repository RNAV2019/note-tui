package notes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Store struct {
	Root string
}

type Note struct {
	Notebook string
	Tag      string
	Name     string
	Path     string
}

func (n Note) Display() string {
	return n.Notebook + "/" + n.Tag + "/" + n.Name
}

func NewStore(root string) *Store { return &Store{Root: root} }

// Bootstrap creates the notes root, a git repo, and the default template.
// Idempotent; never overwrites an existing template.
func (s *Store) Bootstrap() error {
	if err := os.MkdirAll(s.Root, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(s.Root, ".git")); os.IsNotExist(err) {
		cmd := exec.Command("git", "init")
		cmd.Dir = s.Root
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git init: %s: %w", strings.TrimSpace(string(out)), err)
		}
	}
	tmpl := filepath.Join(s.Root, ".template.typ")
	if _, err := os.Stat(tmpl); os.IsNotExist(err) {
		return os.WriteFile(tmpl, []byte(DefaultTemplate), 0o644)
	}
	return nil
}

func (s *Store) listDirs(parent string) ([]string, error) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func (s *Store) Notebooks() ([]string, error) { return s.listDirs(s.Root) }

func (s *Store) Tags(notebook string) ([]string, error) {
	return s.listDirs(filepath.Join(s.Root, notebook))
}

func slugOrErr(kind, name string) (string, error) {
	slug := Slugify(name)
	if slug == "" {
		return "", fmt.Errorf("invalid %s name %q: no usable characters", kind, name)
	}
	return slug, nil
}

func (s *Store) CreateNotebook(name string) error {
	slug, err := slugOrErr("notebook", name)
	if err != nil {
		return err
	}
	dir := filepath.Join(s.Root, slug)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("notebook %q already exists", slug)
	}
	return os.MkdirAll(dir, 0o755)
}

func (s *Store) CreateTag(notebook, name string) error {
	slug, err := slugOrErr("tag", name)
	if err != nil {
		return err
	}
	dir := filepath.Join(s.Root, notebook, slug)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("tag %q already exists in %q", slug, notebook)
	}
	return os.MkdirAll(dir, 0o755)
}

func (s *Store) DeleteNotebook(notebook string) error {
	return os.RemoveAll(filepath.Join(s.Root, notebook))
}

func (s *Store) DeleteTag(notebook, tag string) error {
	return os.RemoveAll(filepath.Join(s.Root, notebook, tag))
}

// CreateNote renders the template into a new .typ file and returns its path.
func (s *Store) CreateNote(notebook, tag, title string) (string, error) {
	slug, err := slugOrErr("note", title)
	if err != nil {
		return "", err
	}
	path := filepath.Join(s.Root, notebook, tag, slug+".typ")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("note %q already exists", slug)
	}
	tmpl := DefaultTemplate
	if data, err := os.ReadFile(filepath.Join(s.Root, ".template.typ")); err == nil {
		tmpl = string(data)
	}
	content := RenderTemplate(tmpl, title, tag, time.Now())
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) DeleteNote(n Note) error { return os.Remove(n.Path) }

func (s *Store) AllNotes() ([]Note, error) {
	var out []Note
	notebooks, err := s.Notebooks()
	if err != nil {
		return nil, err
	}
	for _, nb := range notebooks {
		tags, err := s.Tags(nb)
		if err != nil {
			return nil, err
		}
		for _, tag := range tags {
			entries, err := os.ReadDir(filepath.Join(s.Root, nb, tag))
			if err != nil {
				return nil, err
			}
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".typ") {
					continue
				}
				name := strings.TrimSuffix(e.Name(), ".typ")
				out = append(out, Note{
					Notebook: nb,
					Tag:      tag,
					Name:     name,
					Path:     filepath.Join(s.Root, nb, tag, e.Name()),
				})
			}
		}
	}
	return out, nil
}

// NoteCount counts notes in a notebook, or in one tag if tag != "".
func (s *Store) NoteCount(notebook, tag string) (int, error) {
	all, err := s.AllNotes()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, note := range all {
		if note.Notebook == notebook && (tag == "" || note.Tag == tag) {
			n++
		}
	}
	return n, nil
}
