package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func repo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nfunc A() {}\n"), 0o644)
	run("add", ".")
	run("-c", "commit.gpgsign=false", "commit", "-q", "-m", "first")
	return dir
}

func TestHeadAndChanges(t *testing.T) {
	// Arrange
	dir := repo(t)
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nfunc A() { B() }\n\nfunc B() {}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.go"), []byte("package a\n"), 0o644)

	// Act
	h := HeadOf(t.Context(), dir)
	ch, err := Changes(t.Context(), dir)

	// Assert
	if h.Branch != "main" || len(h.Commit) != 40 {
		t.Errorf("head = %+v", h)
	}
	if err != nil || len(ch) != 2 {
		t.Fatalf("changes = %+v, %v", ch, err)
	}
	churn := Churn(t.Context(), dir, 24*time.Hour)
	if churn[filepath.Join(Top(t.Context(), dir), "a.go")] != 3 {
		t.Errorf("churn = %v", churn)
	}
}

func TestHead_OutsideARepository(t *testing.T) {
	if h := HeadOf(t.Context(), t.TempDir()); h.Commit != "" {
		t.Errorf("head = %+v", h)
	}
}
