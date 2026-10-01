package config

import (
	"os"
	"path/filepath"
	"strings"
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
	if cfg.Preview != defaults().Preview {
		t.Errorf("Preview = %q", cfg.Preview)
	}
	if cfg.PreviewURL != "http://127.0.0.1:23635" {
		t.Errorf("PreviewURL = %q", cfg.PreviewURL)
	}
	if cfg.PreviewProfile != filepath.Join(home, ".cache", "note", "preview-profile") {
		t.Errorf("PreviewProfile = %q", cfg.PreviewProfile)
	}
}

// The preview window is only closable because it runs in a profile of its
// own, so the default command must actually ask for one; and it is a single
// app window only while extensions stay out of that fresh profile.
func TestDefaultPreviewCommandOwnsItsBrowserInstance(t *testing.T) {
	preview := defaults().Preview
	for _, want := range []string{"--app={url}", "--user-data-dir={profile}", "--disable-extensions"} {
		if !strings.Contains(preview, want) {
			t.Errorf("default preview command %q is missing %q", preview, want)
		}
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
	if cfg.Preview != defaults().Preview {
		t.Errorf("Preview should keep default, got %q", cfg.Preview)
	}
}

func TestPreviewProfileTildeExpansion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(path, []byte("preview_profile = \"~/myprofile\"\n"), 0o644)
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	if want := filepath.Join(home, "myprofile"); cfg.PreviewProfile != want {
		t.Errorf("PreviewProfile = %q, want %q", cfg.PreviewProfile, want)
	}
}

func TestLoadRejectsBadTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(path, []byte("notes_dir = [broken"), 0o644)
	if _, err := LoadFrom(path); err == nil {
		t.Fatal("expected parse error")
	}
}
