package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:           "note",
	Short:         "Typst lecture notes: create, find, and edit with live preview",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}
