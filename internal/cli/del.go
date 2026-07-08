package cli

import (
	"fmt"

	"github.com/RNAV2019/note/internal/ui"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a note",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, store, err := open()
		if err != nil {
			return err
		}
		n, ok, err := pickNote(store, "Delete which note?")
		if err != nil || !ok {
			return err
		}
		confirmed, err := ui.RunConfirm(fmt.Sprintf("Delete note %q? This cannot be undone.", n.Display()))
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println(ui.DimStyle.Render("cancelled"))
			return nil
		}
		if err := store.DeleteNote(n); err != nil {
			return err
		}
		fmt.Println(ui.Success("deleted " + n.Display()))
		return nil
	},
}

func init() { rootCmd.AddCommand(deleteCmd) }
