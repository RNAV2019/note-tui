package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	NotesDir       string `toml:"notes_dir"`
	Editor         string `toml:"editor"`
	Preview        string `toml:"preview"`
	PreviewURL     string `toml:"preview_url"`
	PreviewProfile string `toml:"preview_profile"`
}

func defaults() Config {
	home, _ := os.UserHomeDir()
	return Config{
		NotesDir: filepath.Join(home, "Documents", "notes"),
		Editor:   "hx",
		// --user-data-dir is what makes this window ours to close: without
		// it a running browser adopts the window and this process exits.
		// {profile} expands to a directory used by this session alone, so
		// two open notes cannot adopt each other's window either.
		// --class gives the window manager something specific to target.
		Preview: "helium --app={url} --class=note-preview --user-data-dir={profile} " +
			"--no-first-run --no-default-browser-check",
		// Must match --data-plane-host in the editor's tinymist preview
		// config; tinymist serves the preview page from that same address.
		PreviewURL:     "http://127.0.0.1:23635",
		PreviewProfile: filepath.Join(home, ".cache", "note", "preview-profile"),
	}
}

// Load reads ~/.config/note/config.toml, falling back to defaults.
func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("cannot determine home directory: %w", err)
	}
	return LoadFrom(filepath.Join(home, ".config", "note", "config.toml"))
}

func LoadFrom(path string) (Config, error) {
	cfg := defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, nil // absent config is fine
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	cfg.NotesDir = expandTilde(cfg.NotesDir)
	cfg.PreviewProfile = expandTilde(cfg.PreviewProfile)
	return cfg, nil
}

func expandTilde(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
	}
	return p
}
