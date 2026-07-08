package cli

import (
	"fmt"

	"github.com/RNAV2019/note/internal/ui"
	"github.com/spf13/cobra"
)

var nbCmd = &cobra.Command{Use: "nb", Short: "Manage notebooks"}

var nbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List notebooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, store, err := open()
		if err != nil {
			return err
		}
		nbs, err := store.Notebooks()
		if err != nil {
			return err
		}
		if len(nbs) == 0 {
			fmt.Println(ui.DimStyle.Render("no notebooks yet"))
			return nil
		}
		for _, nb := range nbs {
			n, _ := store.NoteCount(nb, "")
			fmt.Printf("%s %s\n", ui.TitleStyle.Render(nb), ui.DimStyle.Render(fmt.Sprintf("(%d notes)", n)))
		}
		return nil
	},
}

var nbNewCmd = &cobra.Command{
	Use:   "new [name]",
	Short: "Create a notebook",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, store, err := open()
		if err != nil {
			return err
		}
		name := ""
		if len(args) == 1 {
			name = args[0]
		} else {
			var ok bool
			name, ok, err = ui.RunPrompt("New notebook name", "e.g. university")
			if err != nil || !ok {
				return err
			}
		}
		if err := store.CreateNotebook(name); err != nil {
			return err
		}
		fmt.Println(ui.Success("created notebook " + notesSlug(name)))
		return nil
	},
}

var nbDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a notebook and all its notes",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, store, err := open()
		if err != nil {
			return err
		}
		nb, ok, err := pickNotebook(store, "Delete which notebook?")
		if err != nil || !ok {
			return err
		}
		count, _ := store.NoteCount(nb, "")
		msg := fmt.Sprintf("Delete notebook %q and its %d note(s)? This cannot be undone.", nb, count)
		confirmed, err := ui.RunConfirm(msg)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println(ui.DimStyle.Render("cancelled"))
			return nil
		}
		if err := store.DeleteNotebook(nb); err != nil {
			return err
		}
		fmt.Println(ui.Success("deleted notebook " + nb))
		return nil
	},
}

func init() {
	nbCmd.AddCommand(nbListCmd, nbNewCmd, nbDeleteCmd)
	rootCmd.AddCommand(nbCmd)
}
