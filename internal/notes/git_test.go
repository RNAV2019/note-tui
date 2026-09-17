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

func identify(t *testing.T, dir string) {
	t.Helper()
	gitOut(t, dir, "config", "user.email", "test@test")
	gitOut(t, dir, "config", "user.name", "test")
}

func TestStatusReportsDirtyNotebooks(t *testing.T) {
	s := newTestStore(t)
	identify(t, s.Root)
	s.CreateNotebook("uni")
	s.CreateNotebook("scratch")
	s.CreateTag("uni", "algos")
	s.CreateTag("scratch", "misc")
	s.CreateNote("uni", "algos", "one")
	s.CreateNote("uni", "algos", "two")

	st, err := s.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch == "" {
		t.Error("branch should be reported even before the first commit")
	}
	if st.Remote != "" {
		t.Errorf("remote = %q, want none", st.Remote)
	}
	// The template counts as a change but belongs to no notebook.
	if st.Changed != 3 {
		t.Errorf("changed = %d, want 3 (two notes + template)", st.Changed)
	}
	if !st.Dirty["uni"] || st.Dirty["scratch"] {
		t.Errorf("dirty = %v, want only uni", st.Dirty)
	}
	if !st.LastSync.IsZero() {
		t.Errorf("last sync = %v, want never", st.LastSync)
	}

	if _, err := s.Backup("snapshot"); err != nil {
		t.Fatal(err)
	}
	st, err = s.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Changed != 0 || len(st.Dirty) != 0 {
		t.Errorf("after backup changed = %d dirty = %v, want a clean tree", st.Changed, st.Dirty)
	}
}

func TestStatusCountsAheadAndBehind(t *testing.T) {
	remote := t.TempDir()
	gitOut(t, remote, "init", "--bare", "-b", "main")

	s := newTestStore(t)
	identify(t, s.Root)
	gitOut(t, s.Root, "checkout", "-b", "main")
	gitOut(t, s.Root, "remote", "add", "origin", remote)
	gitOut(t, s.Root, "add", "-A")
	gitOut(t, s.Root, "commit", "-m", "first")
	gitOut(t, s.Root, "push", "-u", "origin", "main")

	s.CreateNotebook("uni")
	s.CreateTag("uni", "algos")
	s.CreateNote("uni", "algos", "one")
	gitOut(t, s.Root, "add", "-A")
	gitOut(t, s.Root, "commit", "-m", "local")

	st, err := s.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch != "main" || st.Remote != "origin" {
		t.Errorf("branch/remote = %q/%q, want main/origin", st.Branch, st.Remote)
	}
	if st.Ahead != 1 || st.Behind != 0 {
		t.Errorf("ahead/behind = %d/%d, want 1/0", st.Ahead, st.Behind)
	}
}
