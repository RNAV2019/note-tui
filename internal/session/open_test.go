package session

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestHostPort(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{"explicit port", "http://127.0.0.1:23635", "127.0.0.1:23635", false},
		{"http default port", "http://localhost", "localhost:80", false},
		{"https default port", "https://example.test", "example.test:443", false},
		{"no host", "not-a-url", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hostPort(tt.url)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("hostPort(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestWaitForPreviewReturnsOnceListening(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	if err := waitForPreview(context.Background(), "http://"+ln.Addr().String()); err != nil {
		t.Errorf("waitForPreview = %v, want nil", err)
	}
}

// The preview only appears after the editor starts, so the wait must tolerate
// the port being closed for a while first.
func TestWaitForPreviewPollsUntilTheServerAppears(t *testing.T) {
	// Reserve a port, close it, then re-listen shortly after.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()

	go func() {
		time.Sleep(300 * time.Millisecond)
		if ln, err := net.Listen("tcp", addr); err == nil {
			time.Sleep(2 * time.Second)
			ln.Close()
		}
	}()

	start := time.Now()
	if err := waitForPreview(context.Background(), "http://"+addr); err != nil {
		t.Fatalf("waitForPreview = %v, want nil", err)
	}
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Errorf("returned in %s, so it cannot have waited for the listener", elapsed)
	}
}

func TestWaitForPreviewStopsWhenCancelled(t *testing.T) {
	// A port nothing is listening on.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	if err := waitForPreview(ctx, "http://"+addr); err == nil {
		t.Error("expected an error when cancelled")
	}
	// Must give up on cancellation rather than burning the full timeout.
	if elapsed := time.Since(start); elapsed > previewTimeout/2 {
		t.Errorf("took %s to notice cancellation", elapsed)
	}
}

func TestStartBrowserSubstitutesTheURL(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "args.txt")
	// A "browser" that records the arguments it was given.
	script := filepath.Join(dir, "fake-browser")
	body := "#!/bin/sh\nprintf '%s' \"$1\" > " + out + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	p, err := startBrowser(script+" --app={url}", "http://127.0.0.1:23635", "")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("startBrowser returned no command")
	}
	p.cmd.Wait()

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := "--app=http://127.0.0.1:23635"; string(got) != want {
		t.Errorf("browser got %q, want %q", got, want)
	}
}

func TestStartBrowserSubstitutesTheProfileAndCreatesIt(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "args.txt")
	base := filepath.Join(dir, "nested", "preview-profile")
	script := filepath.Join(dir, "fake-browser")
	body := "#!/bin/sh\nprintf '%s' \"$2\" > " + out + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	p, err := startBrowser(script+" --app={url} --user-data-dir={profile}", "http://x", base)
	if err != nil {
		t.Fatal(err)
	}
	p.cmd.Wait()

	got, _ := os.ReadFile(out)
	if want := "--user-data-dir=" + p.profile; string(got) != want {
		t.Errorf("browser got %q, want %q", got, want)
	}
	// The profile lives under the configured base, not at it.
	if filepath.Dir(p.profile) != base {
		t.Errorf("profile %q is not under base %q", p.profile, base)
	}
	// The browser needs the directory to exist, including any parents.
	if info, err := os.Stat(p.profile); err != nil || !info.IsDir() {
		t.Errorf("profile directory not created: %v", err)
	}
}

// The whole point of the per-session profile: two concurrent sessions must
// never share one, or the second browser hands its window to the first and
// exits, leaving a window no pid of ours can close.
func TestStartBrowserGivesEachSessionItsOwnProfile(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "profiles")
	script := filepath.Join(dir, "browser")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	a, err := startBrowser(script+" --user-data-dir={profile}", "http://x", base)
	if err != nil {
		t.Fatal(err)
	}
	defer stopBrowser(a)
	b, err := startBrowser(script+" --user-data-dir={profile}", "http://x", base)
	if err != nil {
		t.Fatal(err)
	}
	defer stopBrowser(b)

	if a.profile == b.profile {
		t.Fatalf("both sessions got profile %q", a.profile)
	}
}

// A crashed session leaves its profile behind; each is hundreds of megabytes,
// so the next start has to reclaim them without touching live ones.
func TestSessionProfileSweepsDeadProfilesOnly(t *testing.T) {
	base := t.TempDir()

	dead := filepath.Join(base, "session-dead")
	live := filepath.Join(base, "session-live")
	unlocked := filepath.Join(base, "session-nolock")
	keep := filepath.Join(base, "not-a-session")
	for _, d := range []string{dead, live, unlocked, keep} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A pid nothing can be running under, and this test's own pid.
	if err := os.Symlink("host-4194303", filepath.Join(dead, "SingletonLock")); err != nil {
		t.Fatal(err)
	}
	self := "host-" + strconv.Itoa(os.Getpid())
	if err := os.Symlink(self, filepath.Join(live, "SingletonLock")); err != nil {
		t.Fatal(err)
	}

	fresh, err := sessionProfile(base)
	if err != nil {
		t.Fatal(err)
	}

	for _, d := range []string{live, keep, fresh} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("%s was swept but should have been kept: %v", d, err)
		}
	}
	for _, d := range []string{dead, unlocked} {
		if _, err := os.Stat(d); err == nil {
			t.Errorf("%s survived the sweep", d)
		}
	}
}

// A Chromium-based browser spawns a tree of helper processes. Closing the
// window has to take the whole group down, not just the process we started.
func TestStopBrowserKillsTheWholeProcessGroup(t *testing.T) {
	dir := t.TempDir()
	childPID := filepath.Join(dir, "child.pid")
	script := filepath.Join(dir, "browser")
	// Parent spawns a child (like a browser helper), then both linger.
	body := "#!/bin/sh\nsh -c 'echo $$ > " + childPID + "; sleep 60' &\nsleep 60\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	p, err := startBrowser(script, "http://x", "")
	if err != nil {
		t.Fatal(err)
	}
	// Wait for the helper to record its pid.
	var raw []byte
	for i := 0; i < 100; i++ {
		if raw, err = os.ReadFile(childPID); err == nil && len(raw) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	child, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	if child == 0 {
		t.Fatal("helper process never started")
	}

	stopBrowser(p)

	if err := syscall.Kill(p.cmd.Process.Pid, 0); err == nil {
		t.Error("browser process survived stopBrowser")
	}
	// Give the signal a moment to land on the group.
	time.Sleep(200 * time.Millisecond)
	if err := syscall.Kill(child, 0); err == nil {
		syscall.Kill(child, syscall.SIGKILL)
		t.Errorf("helper process %d survived stopBrowser", child)
	}
}

func TestStopBrowserDiscardsTheSessionProfile(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "profiles")
	script := filepath.Join(dir, "browser")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	p, err := startBrowser(script+" --user-data-dir={profile}", "http://x", base)
	if err != nil {
		t.Fatal(err)
	}
	stopBrowser(p)

	if _, err := os.Stat(p.profile); err == nil {
		t.Errorf("profile %s survived stopBrowser", p.profile)
	}
}

func TestStopBrowserToleratesNil(t *testing.T) {
	stopBrowser(nil)                        // must not panic
	stopBrowser(&preview{})                 // no command
	stopBrowser(&preview{cmd: &exec.Cmd{}}) // never started
}

// An empty template means "no preview"; it must not leave a profile behind.
func TestStartBrowserHandlesAnEmptyTemplate(t *testing.T) {
	base := t.TempDir()
	p, err := startBrowser("   ", "http://127.0.0.1:23635", base)
	if err != nil || p != nil {
		t.Errorf("startBrowser(empty) = (%v, %v), want (nil, nil)", p, err)
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("left %d profiles behind, want none", len(entries))
	}
}

func TestStartBrowserReportsAMissingCommand(t *testing.T) {
	_, err := startBrowser("definitely-not-a-real-browser-binary {url}", "http://x", "")
	if err == nil {
		t.Fatal("expected an error for a missing browser binary")
	}
	if !strings.Contains(err.Error(), "could not open preview window") {
		t.Errorf("error = %q, want it to name the failure", err)
	}
}

// The whole point of the rework: the preview server only exists once the
// editor's language server has started it, so Open must launch the editor
// first, notice the preview appearing, open the window, and close it after.
func TestOpenLaunchesThePreviewWindowAfterTheEditorStarts(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "note.typ")
	os.WriteFile(file, []byte("= Hi\n"), 0o644)

	// Reserve an address the "language server" will listen on shortly.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()

	// An "editor" that runs long enough for the preview to come up.
	editor := filepath.Join(dir, "editor")
	os.WriteFile(editor, []byte("#!/bin/sh\nsleep 1.5\n"), 0o755)

	// A "browser" that records its arguments then blocks, so we can tell
	// whether Open kills it.
	launched := filepath.Join(dir, "launched.txt")
	pidFile := filepath.Join(dir, "browser.pid")
	browser := filepath.Join(dir, "browser")
	os.WriteFile(browser, []byte(
		"#!/bin/sh\nprintf '%s' \"$1\" > "+launched+"\necho $$ > "+pidFile+"\nsleep 60\n"), 0o755)

	// The "language server" opens the preview port after the editor starts.
	go func() {
		time.Sleep(300 * time.Millisecond)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		defer ln.Close()
		time.Sleep(5 * time.Second)
	}()

	previewURL := "http://" + addr
	var warnings []string
	err = Open(file, Options{Editor: editor, Preview: browser + " --app={url}", PreviewURL: previewURL}, StdIO(),
		func(msg string) { warnings = append(warnings, msg) })
	if err != nil {
		t.Fatalf("Open = %v, want nil", err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	got, err := os.ReadFile(launched)
	if err != nil {
		t.Fatalf("preview window was never launched: %v", err)
	}
	// The window opens the restyling proxy, which must be gone with it.
	pageURL, ok := strings.CutPrefix(string(got), "--app=")
	if !ok || !strings.HasPrefix(pageURL, "http://127.0.0.1:") || !strings.HasSuffix(pageURL, pagePath) {
		t.Errorf("browser got %q, want the proxied page", got)
	}
	if addr, err := hostPort(pageURL); err == nil {
		if conn, err := net.Dial("tcp", addr); err == nil {
			conn.Close()
			t.Errorf("the preview proxy on %s outlived the session", addr)
		}
	}

	// Open returned, so the browser it started must be gone.
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err := syscall.Kill(pid, 0); err == nil {
		syscall.Kill(pid, syscall.SIGKILL)
		t.Errorf("browser pid %d still running after Open returned", pid)
	}
}

// Open must not hang when the preview never appears: the editor drives the
// session, and a missing preview only warns.
func TestOpenRunsTheEditorWithoutAPreview(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()

	dir := t.TempDir()
	file := filepath.Join(dir, "note.typ")
	os.WriteFile(file, []byte("= Hi\n"), 0o644)

	var warnings []string
	done := make(chan error, 1)
	go func() {
		done <- Open(file, Options{Editor: "true", Preview: "true {url}", PreviewURL: "http://" + addr}, StdIO(),
			func(msg string) { warnings = append(warnings, msg) })
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Open = %v, want nil", err)
		}
	case <-time.After(previewTimeout):
		t.Fatal("Open blocked on the missing preview instead of following the editor")
	}
}
