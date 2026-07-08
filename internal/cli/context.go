package cli

import (
	"fmt"

	"github.com/RNAV2019/note/internal/config"
	"github.com/RNAV2019/note/internal/notes"
	"github.com/RNAV2019/note/internal/ui"
)

// open loads config and returns a bootstrapped store.
func open() (config.Config, *notes.Store, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("config: %w", err)
	}
	store := notes.NewStore(cfg.NotesDir)
	if err := store.Bootstrap(); err != nil {
		return config.Config{}, nil, fmt.Errorf("bootstrapping %s: %w", cfg.NotesDir, err)
	}
	return cfg, store, nil
}

// pickNotebook lets the user choose an existing notebook. With zero
// notebooks it offers to create one inline instead of erroring.
// ok=false means the user cancelled.
func pickNotebook(store *notes.Store, title string) (string, bool, error) {
	nbs, err := store.Notebooks()
	if err != nil {
		return "", false, err
	}
	if len(nbs) == 0 {
		name, ok, err := ui.RunPrompt("No notebooks yet — create one", "e.g. university")
		if err != nil || !ok {
			return "", ok, err
		}
		if err := store.CreateNotebook(name); err != nil {
			return "", false, err
		}
		return notes.Slugify(name), true, nil
	}
	return runSelect(title, nbs)
}

// runSelect shows a huh select — used for notebook/tag lists.
func runSelect(title string, items []string) (string, bool, error) {
	return ui.RunSelect(title, items)
}

// runPicker shows the custom fuzzy picker — used only for note search.
func runPicker(title string, items []string) (string, bool, error) {
	return ui.RunPicker(title, items)
}

func notesSlug(name string) string { return notes.Slugify(name) }
