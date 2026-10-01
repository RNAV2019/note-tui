package session

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// previewTimeout is how long to wait for the editor's tinymist LSP to
	// bring up its background preview before giving up on the preview window.
	previewTimeout = 15 * time.Second
	// shutdownGrace is how long the preview window gets to exit cleanly
	// before it is killed outright.
	shutdownGrace = 3 * time.Second
)

// IO is the terminal the editor inherits. Use StdIO for a plain process; a
// TUI passes the streams its runtime hands back when it releases the terminal.
type IO struct {
	In       io.Reader
	Out, Err io.Writer
}

// StdIO is the process's own standard streams.
func StdIO() IO { return IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr} }

// Options is how an edit session is launched.
type Options struct {
	Editor  string // command run in the foreground; gets the file as its last argument
	Preview string // preview window command template, with {url} and {profile}
	// PreviewURL is where tinymist serves the preview. It must match
	// --data-plane-host in the editor's language server config.
	PreviewURL string
	// PreviewProfile is the directory this tool keeps browser profiles in.
	// Each session gets a fresh profile underneath it: Chromium-based
	// browsers hand a new window to whichever instance already holds the
	// profile and exit immediately, which would leave us with no process to
	// close when the editor quits. See sessionProfile.
	PreviewProfile string
	// Backdrop is the CSS colour painted behind the preview's pages, and
	// over the whole window while the page loads. Empty keeps tinymist's
	// gray.
	Backdrop string
}

// Open runs the full edit session for a note file.
//
// The preview is owned by the editor's tinymist language server, which spawns
// it in the background (see the Helix setup in the README). That means the
// preview only exists once the editor is running, so this starts the editor
// first, waits for the preview server to listen on PreviewURL, and only then
// opens the preview window beside it. Preview failures warn but never block
// editing, and the window is closed when the editor exits.
func Open(file string, opts Options, tty IO, warn func(string)) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Buffered and always closed, so the receive below cannot deadlock.
	browsers := make(chan *preview, 1)
	go func() {
		defer close(browsers)
		if err := waitForPreview(ctx, opts.PreviewURL); err != nil {
			// A cancelled wait just means the editor exited first.
			if ctx.Err() == nil {
				warn(fmt.Sprintf("live preview unavailable: %v", err))
			}
			return
		}
		// The window opens the preview through a proxy that restyles it; if
		// that can't start, the plain preview is still worth having.
		pageURL := opts.PreviewURL
		px, err := startProxy(opts.PreviewURL, opts.Backdrop)
		if err == nil {
			pageURL = px.url
		}
		browser, err := startBrowser(opts.Preview, pageURL, opts.PreviewProfile)
		if err != nil {
			px.close()
			warn(err.Error())
			return
		}
		if browser == nil {
			px.close()
			return
		}
		browser.proxy = px
		browsers <- browser
	}()

	ed := exec.Command(opts.Editor, file)
	ed.Stdin, ed.Stdout, ed.Stderr = tty.In, tty.Out, tty.Err
	edErr := ed.Run()
	if edErr != nil {
		edErr = fmt.Errorf("editor %q: %w", opts.Editor, edErr)
	}
	cancel()

	stopBrowser(<-browsers)
	return edErr
}

// waitForPreview polls previewURL until something accepts a TCP connection,
// which is the signal that tinymist's preview server is serving.
func waitForPreview(ctx context.Context, previewURL string) error {
	addr, err := hostPort(previewURL)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, previewTimeout)
	defer cancel()

	var dialer net.Dialer
	for {
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("nothing listening on %s after %s", addr, previewTimeout)
		case <-time.After(150 * time.Millisecond):
		}
	}
}

// hostPort extracts a dialable "host:port" from a preview URL, defaulting to
// the scheme's usual port when none is given.
func hostPort(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid preview URL %q: %w", raw, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("preview URL %q has no host", raw)
	}
	if u.Port() != "" {
		return u.Host, nil
	}
	if u.Scheme == "https" {
		return net.JoinHostPort(u.Hostname(), "443"), nil
	}
	return net.JoinHostPort(u.Hostname(), "80"), nil
}

// preview is a running preview window: the browser's process group plus the
// throwaway profile directory that guarantees the group is ours to kill.
type preview struct {
	cmd     *exec.Cmd
	profile string // session-owned; removed once the window is gone
	proxy   *proxy // what the window is pointed at; closed with it
}

// sessionProfile creates a fresh profile directory under base for one edit
// session, first sweeping any left behind by sessions that died.
//
// A per-session directory is what makes the window closable. Chromium-based
// browsers keep one process per profile: launched against a profile another
// instance already holds, the new process hands its window to that instance
// and exits immediately, leaving us holding a dead pid and a window nothing
// can close. A profile of its own means every launch is its own process.
func sessionProfile(base string) (string, error) {
	if base == "" {
		return "", nil
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", fmt.Errorf("preparing preview profile %s: %w", base, err)
	}
	sweepProfiles(base)
	dir, err := os.MkdirTemp(base, "session-")
	if err != nil {
		return "", fmt.Errorf("preparing preview profile in %s: %w", base, err)
	}
	return dir, nil
}

// sweepProfiles deletes session profiles whose browser is no longer running.
// Chromium records the owning pid in the profile's SingletonLock symlink
// ("hostname-pid"), so a lock naming a dead process marks a leftover. A
// profile is several hundred megabytes, so they cannot be left to pile up.
func sweepProfiles(base string) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "session-") {
			continue
		}
		dir := filepath.Join(base, e.Name())
		if pid, ok := lockedBy(dir); ok && syscall.Kill(pid, 0) == nil {
			continue // still in use by a live session
		}
		os.RemoveAll(dir)
	}
}

// lockedBy reads the pid out of a profile's SingletonLock symlink.
func lockedBy(profile string) (int, bool) {
	target, err := os.Readlink(filepath.Join(profile, "SingletonLock"))
	if err != nil {
		return 0, false
	}
	// The target is "hostname-pid", and a hostname may itself contain "-".
	pid, err := strconv.Atoi(target[strings.LastIndex(target, "-")+1:])
	if err != nil {
		return 0, false
	}
	return pid, true
}

// startBrowser runs the preview command template with {url} and {profile}
// substituted, in its own process group so the whole browser can be closed
// as a unit.
func startBrowser(template, previewURL, profileBase string) (*preview, error) {
	profile, err := sessionProfile(profileBase)
	if err != nil {
		return nil, err
	}
	expanded := strings.NewReplacer("{url}", previewURL, "{profile}", profile).Replace(template)
	parts := strings.Fields(expanded)
	if len(parts) == 0 {
		os.RemoveAll(profile)
		return nil, nil
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		os.RemoveAll(profile)
		return nil, fmt.Errorf("could not open preview window (%s): %v", parts[0], err)
	}
	return &preview{cmd: cmd, profile: profile}, nil
}

// stopBrowser closes the preview window, giving the browser a moment to shut
// down cleanly before killing its process group, then discards its profile.
func stopBrowser(p *preview) {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	defer func() {
		p.proxy.close()
		if p.profile != "" {
			os.RemoveAll(p.profile)
		}
	}()
	cmd := p.cmd
	pid := cmd.Process.Pid
	syscall.Kill(-pid, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(shutdownGrace):
		syscall.Kill(-pid, syscall.SIGKILL)
		<-done
	}
}
