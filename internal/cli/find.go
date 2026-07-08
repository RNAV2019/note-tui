package cli

import (
	"fmt"

	"github.com/RNAV2019/note/internal/notes"
	"github.com/RNAV2019/note/internal/ui"
	"github.com/spf13/cobra"
)

// pickNote fuzzy-picks a note across all notebooks.
func pickNote(store *notes.Store, title string) (notes.Note, bool, error) {
	all, err := store.AllNotes()
	if err != nil {
		return notes.Note{}, false, err
	}
	if len(all) == 0 {
		fmt.Println(ui.Fail("no notes yet — create one with: note new"))
		return notes.Note{}, false, nil
	}
	byDisplay := make(map[string]notes.Note, len(all))
	items := make([]string, len(all))
	for i, n := range all {
		items[i] = n.Display()
		byDisplay[n.Display()] = n
	}
	choice, ok, err := runPicker(title, items)
	if err != nil || !ok {
		return notes.Note{}, ok, err
	}
	return byDisplay[choice], true, nil
}

var findCmd = &cobra.Command{
	Use:   "find",
	Short: "Fuzzy-find a note and open it with live preview",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, store, err := open()
		if err != nil {
			return err
		}
		n, ok, err := pickNote(store, "Open which note?")
		if err != nil || !ok {
			return err
		}
		return openNote(cfg, n.Path)
	},
}

func init() { rootCmd.AddCommand(findCmd) }
