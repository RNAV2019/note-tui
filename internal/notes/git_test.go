package notes

import (
	"os/exec"
	"strings"
	"testing"
)

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
	return string(out)
}

func TestBackupCommitsChanges(t *testing.T) {
	s := newTestStore(t)
	gitOut(t, s.Root, "config", "user.email", "test@test")
	gitOut(t, s.Root, "config", "user.name", "test")
	s.CreateNotebook("uni")
	s.CreateTag("uni", "algos")
	s.CreateNote("uni", "algos", "one")

	pushed, err := s.Backup("backup: test")
	if err != nil {
		t.Fatal(err)
	}
	if pushed {
		t.Error("no remote configured, pushed should be false")
	}
	log := gitOut(t, s.Root, "log", "--oneline")
	if !strings.Contains(log, "backup: test") {
		t.Errorf("commit missing, log: %s", log)
	}
}

func TestBackupNothingToCommit(t *testing.T) {
	s := newTestStore(t)
	gitOut(t, s.Root, "config", "user.email", "test@test")
	gitOut(t, s.Root, "config", "user.name", "test")
	if _, err := s.Backup("first"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Backup("second"); err != nil {
		t.Fatalf("backup with clean tree must not error: %v", err)
	}
}

func TestSyncWithoutRemoteErrors(t *testing.T) {
	s := newTestStore(t)
	if err := s.Sync(); err == nil {
		t.Error("sync without remote should return an error")
	}
}
