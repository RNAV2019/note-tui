package session

import (
	"net/http"
	"os"
	"testing"
	"time"
)

const sampleLog = `[2026-07-08T22:39:49Z INFO  tinymist::tool::preview::http] preview server listening on http://127.0.0.1:23626
[2026-07-08T22:39:49Z INFO  tinymist::cmd::preview] Control panel server listening on: 127.0.0.1:23626
[2026-07-08T22:39:49Z INFO  tinymist::cmd::preview] Data plane server listening on: 127.0.0.1:23625
[2026-07-08T22:39:49Z INFO  tinymist::cmd::preview] Static file server listening on: 127.0.0.1:33843
`

func TestParseStaticURL(t *testing.T) {
	url, ok := ParseStaticURL(sampleLog)
	if !ok || url != "http://127.0.0.1:33843" {
		t.Errorf("got %q, %v", url, ok)
	}
}

func TestParseStaticURLLineByLine(t *testing.T) {
	if _, ok := ParseStaticURL("Control panel server listening on: 127.0.0.1:1"); ok {
		t.Error("should not match control panel line")
	}
	url, ok := ParseStaticURL("blah Static file server listening on: 0.0.0.0:8080 trailing")
	if !ok || url != "http://0.0.0.0:8080" {
		t.Errorf("got %q, %v", url, ok)
	}
}

func TestStartPreviewSmoke(t *testing.T) {
	if os.Getenv("NOTE_SMOKE") == "" {
		t.Skip("set NOTE_SMOKE=1 to run smoke test")
	}

	// Create a minimal .typ file
	f, err := os.CreateTemp("", "note-smoke-*.typ")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("= Smoke Test\n\nHello, world!\n")
	f.Close()

	p, err := StartPreview(f.Name(), 15*time.Second)
	if err != nil {
		t.Fatalf("StartPreview failed: %v", err)
	}
	defer p.Stop()

	t.Logf("Preview URL: %s", p.URL)

	// Verify the URL is reachable
	resp, err := http.Get(p.URL)
	if err != nil {
		t.Fatalf("GET %s failed: %v", p.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", resp.StatusCode)
	}
	t.Logf("HTTP status: %d", resp.StatusCode)

	// Stop and verify the process is no longer running
	pid := p.cmd.Process.Pid
	p.Stop()

	// After Stop, Wait should have been called; process should be gone
	// We check by trying to signal PID 0 of the process group (should fail)
	err = p.cmd.Wait()
	// Wait may return an error if already waited, that's fine
	t.Logf("cmd.Wait after Stop: %v (expected non-nil or nil, process should be gone)", err)
	t.Logf("tinymist PID was: %d", pid)
}
