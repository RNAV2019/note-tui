package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsWhenFileMissing(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("missing file should yield defaults, got error: %v", err)
	}
	home, _ := os.UserHomeDir()
	if cfg.NotesDir != filepath.Join(home, "Documents", "notes") {
		t.Errorf("NotesDir = %q", cfg.NotesDir)
	}
	if cfg.Editor != "hx" {
		t.Errorf("Editor = %q", cfg.Editor)
	}
	if cfg.Preview != "helium --app={url}" {
		t.Errorf("Preview = %q", cfg.Preview)
	}
}

func TestLoadOverridesAndTildeExpansion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(path, []byte("notes_dir = \"~/mynotes\"\neditor = \"vim\"\n"), 0o644)
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	if cfg.NotesDir != filepath.Join(home, "mynotes") {
		t.Errorf("NotesDir = %q, want tilde expanded", cfg.NotesDir)
	}
	if cfg.Editor != "vim" {
		t.Errorf("Editor = %q", cfg.Editor)
	}
	if cfg.Preview != "helium --app={url}" {
		t.Errorf("Preview should keep default, got %q", cfg.Preview)
	}
}

func TestLoadRejectsBadTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(path, []byte("notes_dir = [broken"), 0o644)
	if _, err := LoadFrom(path); err == nil {
		t.Fatal("expected parse error")
	}
}
