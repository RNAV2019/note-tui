package cli

import (
	"fmt"

	"github.com/RNAV2019/note/internal/ui"
	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{Use: "tag", Short: "Manage tags (module folders) inside a notebook"}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tags in a notebook",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, store, err := open()
		if err != nil {
			return err
		}
		nb, ok, err := pickNotebook(store, "Which notebook?")
		if err != nil || !ok {
			return err
		}
		tags, err := store.Tags(nb)
		if err != nil {
			return err
		}
		if len(tags) == 0 {
			fmt.Println(ui.DimStyle.Render("no tags in " + nb))
			return nil
		}
		for _, tag := range tags {
			n, _ := store.NoteCount(nb, tag)
			fmt.Printf("%s %s\n", ui.TitleStyle.Render(tag), ui.DimStyle.Render(fmt.Sprintf("(%d notes)", n)))
		}
		return nil
	},
}

var tagNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a tag in a notebook",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, store, err := open()
		if err != nil {
			return err
		}
		nb, ok, err := pickNotebook(store, "Add a tag to which notebook?")
		if err != nil || !ok {
			return err
		}
		name, ok, err := ui.RunPrompt("New tag name", "e.g. algorithms")
		if err != nil || !ok {
			return err
		}
		if err := store.CreateTag(nb, name); err != nil {
			return err
		}
		fmt.Println(ui.Success(fmt.Sprintf("created tag %s in %s", notesSlug(name), nb)))
		return nil
	},
}

var tagDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a tag and all its notes",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, store, err := open()
		if err != nil {
			return err
		}
		nb, ok, err := pickNotebook(store, "Delete a tag from which notebook?")
		if err != nil || !ok {
			return err
		}
		tags, err := store.Tags(nb)
		if err != nil {
			return err
		}
		if len(tags) == 0 {
			fmt.Println(ui.Fail("no tags in " + nb))
			return nil
		}
		tag, ok, err := runSelect("Delete which tag?", tags)
		if err != nil || !ok {
			return err
		}
		count, _ := store.NoteCount(nb, tag)
		msg := fmt.Sprintf("Delete tag %q from %q and its %d note(s)? This cannot be undone.", tag, nb, count)
		confirmed, err := ui.RunConfirm(msg)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println(ui.DimStyle.Render("cancelled"))
			return nil
		}
		if err := store.DeleteTag(nb, tag); err != nil {
			return err
		}
		fmt.Println(ui.Success(fmt.Sprintf("deleted tag %s from %s", tag, nb)))
		return nil
	},
}

func init() {
	tagCmd.AddCommand(tagListCmd, tagNewCmd, tagDeleteCmd)
	rootCmd.AddCommand(tagCmd)
}
