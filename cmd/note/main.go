package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/RNAV2019/note-tui/internal/config"
	"github.com/RNAV2019/note-tui/internal/notes"
	"github.com/RNAV2019/note-tui/internal/tui"
)

// version is injected at build time with -X main.version=...
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print the version and exit")
	flag.BoolVar(&showVersion, "v", false, "print the version and exit")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: note [--version]\n\n"+
			"Run with no arguments to open the notes TUI.\n")
	}
	flag.Parse()

	if showVersion {
		fmt.Println("note " + version)
		return nil
	}
	if flag.NArg() > 0 {
		flag.Usage()
		return fmt.Errorf("unexpected argument %q", flag.Arg(0))
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store := notes.NewStore(cfg.NotesDir)
	if err := store.Bootstrap(); err != nil {
		return fmt.Errorf("preparing %s: %w", cfg.NotesDir, err)
	}
	return tui.Run(store, cfg, version)
}
