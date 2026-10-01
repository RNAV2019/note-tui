package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Gated: drives a real helium window against a real tinymist preview.
func TestRealHeliumLifecycle(t *testing.T) {
	if os.Getenv("NOTE_REAL") != "1" {
		t.Skip("set NOTE_REAL=1")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "n.typ")
	os.WriteFile(file, []byte("= Hi\n"), 0o644)
	editor := filepath.Join(dir, "e")
	os.WriteFile(editor, []byte("#!/bin/sh\nsleep 8\n"), 0o755)

	profile := filepath.Join(dir, "profile")
	opts := Options{
		Editor: editor,
		Preview: "helium --app={url} --user-data-dir={profile} " +
			"--no-first-run --no-default-browser-check --disable-extensions",
		PreviewURL:     "http://127.0.0.1:23635",
		PreviewProfile: profile,
	}

	// While the "editor" runs, confirm a note-preview window exists.
	go func() {
		time.Sleep(6 * time.Second)
		out, _ := exec.Command("hyprctl", "clients", "-j").Output()
		procs, _ := exec.Command("pgrep", "-fc", "user-data-dir="+profile).Output()
		t.Logf("during edit: note-preview windows=%d helium procs=%s classes=%v", strings.Count(string(out), "__note-preview-"), strings.TrimSpace(string(procs)), strings.Count(string(out), "\"class\""))
	}()

	var warns []string
	if err := Open(file, opts, StdIO(), func(m string) { warns = append(warns, m) }); err != nil {
		t.Fatal(err)
	}
	if len(warns) > 0 {
		t.Fatalf("warnings: %v", warns)
	}

	time.Sleep(2 * time.Second)
	out, _ := exec.Command("hyprctl", "clients", "-j").Output()
	if n := strings.Count(string(out), "__note-preview-"); n != 0 {
		t.Errorf("%d note-preview windows survived after the editor exited", n)
	}
	if out, _ := exec.Command("pgrep", "-fc", "user-data-dir="+profile).Output(); strings.TrimSpace(string(out)) != "0" && strings.TrimSpace(string(out)) != "" {
		t.Errorf("helium processes survived: %s", out)
	}
}
