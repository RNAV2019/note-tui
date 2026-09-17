package notes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (s *Store) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.Root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (s *Store) hasRemote() (bool, error) {
	out, err := s.git("remote")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// Backup stages everything, commits (skipping cleanly if nothing changed),
// and pushes when a remote exists. Returns whether a push happened.
func (s *Store) Backup(message string) (pushed bool, err error) {
	if _, err := s.git("add", "-A"); err != nil {
		return false, err
	}
	status, err := s.git("status", "--porcelain")
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(status) != "" {
		if _, err := s.git("commit", "-m", message); err != nil {
			return false, err
		}
	}
	remote, err := s.hasRemote()
	if err != nil {
		return false, err
	}
	if !remote {
		return false, nil
	}
	if _, err := s.git("push"); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) Sync() error {
	remote, err := s.hasRemote()
	if err != nil {
		return err
	}
	if !remote {
		return fmt.Errorf("no git remote configured in %s — add one with: git -C %s remote add origin <url>", s.Root, s.Root)
	}
	_, err = s.git("pull", "--rebase")
	return err
}

// RepoStatus is a snapshot of the notes repository, as shown in the TUI.
type RepoStatus struct {
	Branch        string
	Remote        string // first configured remote, "" when there is none
	Ahead, Behind int    // relative to the upstream branch, if any
	Changed       int    // files with uncommitted changes
	// Dirty holds the notebooks containing at least one changed file.
	Dirty map[string]bool
	// LastSync is when the repo last fetched from its remote; zero if never.
	LastSync time.Time
}

// Status reads the repository state without touching the network.
func (s *Store) Status() (RepoStatus, error) {
	st := RepoStatus{Dirty: map[string]bool{}}

	out, err := s.git("status", "--porcelain", "--branch", "--untracked-files=all", "-z")
	if err != nil {
		return st, err
	}
	entries := strings.Split(out, "\x00")
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if strings.HasPrefix(e, "## ") {
			st.Branch, st.Ahead, st.Behind = parseBranchLine(e[3:])
			continue
		}
		if len(e) < 4 {
			continue
		}
		code, path := e[:2], e[3:]
		if code[0] == 'R' || code[0] == 'C' {
			i++ // -z puts the rename source in its own entry
		}
		st.Changed++
		if nb, _, nested := strings.Cut(path, "/"); nested && !strings.HasPrefix(nb, ".") {
			st.Dirty[nb] = true
		}
	}

	remotes, err := s.git("remote")
	if err != nil {
		return st, err
	}
	if fields := strings.Fields(remotes); len(fields) > 0 {
		st.Remote = fields[0]
	}
	if info, err := os.Stat(filepath.Join(s.Root, ".git", "FETCH_HEAD")); err == nil {
		st.LastSync = info.ModTime()
	}
	return st, nil
}

// parseBranchLine reads git's "## main...origin/main [ahead 1, behind 2]"
// header. Before the first commit it reads "## No commits yet on main".
func parseBranchLine(line string) (branch string, ahead, behind int) {
	line = strings.TrimPrefix(line, "No commits yet on ")
	line = strings.TrimPrefix(line, "Initial commit on ")
	head, counts, _ := strings.Cut(line, " [")
	branch, _, _ = strings.Cut(head, "...")
	for _, part := range strings.Split(strings.TrimSuffix(counts, "]"), ", ") {
		var n int
		if _, err := fmt.Sscanf(part, "ahead %d", &n); err == nil {
			ahead = n
		} else if _, err := fmt.Sscanf(part, "behind %d", &n); err == nil {
			behind = n
		}
	}
	return branch, ahead, behind
}
