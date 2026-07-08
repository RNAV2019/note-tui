package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	NotesDir string `toml:"notes_dir"`
	Editor   string `toml:"editor"`
	Preview  string `toml:"preview"`
}

func defaults() Config {
	home, _ := os.UserHomeDir()
	return Config{
		NotesDir: filepath.Join(home, "Documents", "notes"),
		Editor:   "hx",
		Preview:  "helium --app={url}",
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
