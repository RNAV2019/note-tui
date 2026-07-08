package cli

import (
	"fmt"

	"github.com/RNAV2019/note/internal/ui"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a note and open it with live preview",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, store, err := open()
		if err != nil {
			return err
		}
		nb, ok, err := pickNotebook(store, "Which notebook?")
		if err != nil || !ok {
			return err
		}
		tag, ok, err := pickTag(store, nb)
		if err != nil || !ok {
			return err
		}
		title, ok, err := ui.RunPrompt("Note title", "e.g. B-Trees")
		if err != nil || !ok {
			return err
		}
		path, err := store.CreateNote(nb, tag, title)
		if err != nil {
			return err
		}
		fmt.Println(ui.Success("created " + path))
		return openNote(cfg, path)
	},
}

func init() { rootCmd.AddCommand(newCmd) }
