package notes

import (
	"fmt"
	"os/exec"
	"strings"
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

func (s *Store) hasRemote() bool {
	out, err := s.git("remote")
	return err == nil && strings.TrimSpace(out) != ""
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
	if !s.hasRemote() {
		return false, nil
	}
	if _, err := s.git("push"); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) Sync() error {
	if !s.hasRemote() {
		return fmt.Errorf("no git remote configured in %s — add one with: git -C %s remote add origin <url>", s.Root, s.Root)
	}
	_, err := s.git("pull", "--rebase")
	return err
}
