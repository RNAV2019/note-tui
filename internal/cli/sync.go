package cli

import (
	"fmt"
	"time"

	"github.com/RNAV2019/note/internal/ui"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Commit all notes and push to the remote (if configured)",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, store, err := open()
		if err != nil {
			return err
		}
		msg := "backup: " + time.Now().Format("2006-01-02 15:04")
		pushed, err := store.Backup(msg)
		if err != nil {
			return err
		}
		if pushed {
			fmt.Println(ui.Success("backed up and pushed"))
		} else {
			fmt.Println(ui.Success("backed up locally") + ui.DimStyle.Render("  (no remote configured — not pushed)"))
		}
		return nil
	},
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Pull the latest notes from the remote",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, store, err := open()
		if err != nil {
			return err
		}
		if err := store.Sync(); err != nil {
			return err
		}
		fmt.Println(ui.Success("synced"))
		return nil
	},
}

func init() { rootCmd.AddCommand(backupCmd, syncCmd) }
