package session

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var staticURLRe = regexp.MustCompile(`Static file server listening on: (\S+)`)

// ParseStaticURL extracts the preview URL from tinymist log output,
// which may be a single line or a multi-line blob; the first
// "Static file server listening on:" match wins.
func ParseStaticURL(s string) (string, bool) {
	m := staticURLRe.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	// tinymist logs a bare host:port today; tolerate a full URL if a
	// future version starts logging one.
	if strings.HasPrefix(m[1], "http://") || strings.HasPrefix(m[1], "https://") {
		return m[1], true
	}
	return "http://" + m[1], true
}

type Preview struct {
	cmd *exec.Cmd
	URL string
}

// StartPreview launches `tinymist preview` on a random port and waits
// (up to timeout) for the static file server URL to appear in its logs.
func StartPreview(file string, timeout time.Duration) (*Preview, error) {
	cmd := exec.Command("tinymist", "preview", file, "--no-open", "--host", "127.0.0.1:0")
	// New process group so we can kill tinymist and any children together.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting tinymist: %w", err)
	}

	urls := make(chan string, 1)
	scan := func(r io.Reader) {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if url, ok := ParseStaticURL(sc.Text()); ok {
				select {
				case urls <- url:
				default:
				}
			}
		}
	}
	go scan(stderr)
	go scan(stdout)

	select {
	case url := <-urls:
		return &Preview{cmd: cmd, URL: url}, nil
	case <-time.After(timeout):
		killGroup(cmd)
		return nil, fmt.Errorf("tinymist did not report a preview URL within %s", timeout)
	}
}

func (p *Preview) Stop() {
	if p != nil && p.cmd != nil {
		killGroup(p.cmd)
	}
}

// killGroup terminates cmd's whole process group.
// cmd must have been started with Setpgid: true.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	cmd.Wait()
}
