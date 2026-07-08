package session

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Open runs the full edit session for a note file:
// tinymist preview in the background, the preview window (config template
// with {url}), then the editor in the foreground. Preview failures warn but
// never block editing. Everything is cleaned up when the editor exits.
func Open(file, editor, previewTemplate string, warn func(string)) error {
	preview, err := StartPreview(file, 10*time.Second)
	if err != nil {
		warn(fmt.Sprintf("live preview unavailable: %v", err))
	}

	var browser *exec.Cmd
	if preview != nil {
		parts := strings.Fields(strings.ReplaceAll(previewTemplate, "{url}", preview.URL))
		if len(parts) > 0 {
			browser = exec.Command(parts[0], parts[1:]...)
			if err := browser.Start(); err != nil {
				warn(fmt.Sprintf("could not open preview window (%s): %v", parts[0], err))
				browser = nil
			}
		}
	}

	ed := exec.Command(editor, file)
	ed.Stdin, ed.Stdout, ed.Stderr = os.Stdin, os.Stdout, os.Stderr
	edErr := ed.Run()

	preview.Stop()
	if browser != nil && browser.Process != nil {
		browser.Process.Kill()
		browser.Wait()
	}
	return edErr
}
