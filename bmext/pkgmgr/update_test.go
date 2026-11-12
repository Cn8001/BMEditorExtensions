package pkgmgr

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestUpdate_AnUnchangedZipIsNotDownloadedAgain(t *testing.T) {
	// Arrange
	r, base := serve(t)
	r.set("/c.zip", zipOf(t, "c", "1.0.0"), `"a"`)
	m := &Manager{Dir: t.TempDir()}
	if _, err := m.Install(t.Context(), base+"/c.zip", nil); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(filepath.Join(m.Dir, "c", SourceFile))

	// Act
	ins, err := m.Update(t.Context(), "c", nil)

	// Assert
	if !errors.Is(err, ErrUpToDate) || ins.Manifest.Version != "1.0.0" {
		t.Fatalf("update = %+v, %v; want ErrUpToDate", ins, err)
	}
	if r.sent != 1 {
		t.Errorf("zip sent %d times, want once", r.sent)
	}
	after, _ := os.Stat(filepath.Join(m.Dir, "c", SourceFile))
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("the installed extension should be left alone")
	}
}

func TestUpdate_TheSameVersionIsKept(t *testing.T) {
	// Arrange: the zip was uploaded again, so its ETag changed
	r, base := serve(t)
	r.set("/c.zip", zipOf(t, "c", "1.0.0"), `"a"`)
	m := &Manager{Dir: t.TempDir()}
	if _, err := m.Install(t.Context(), base+"/c.zip", nil); err != nil {
		t.Fatal(err)
	}
	r.set("/c.zip", zipOf(t, "c", "1.0.0", "new.txt"), `"b"`)

	// Act
	ins, err := m.Update(t.Context(), "c", nil)

	// Assert
	if !errors.Is(err, ErrUpToDate) {
		t.Fatalf("err = %v, want ErrUpToDate", err)
	}
	if _, err := os.Stat(filepath.Join(ins.Dir, "new.txt")); err == nil {
		t.Error("the same version should not be installed again")
	}
	if ins.Source.ETag != `"b"` {
		t.Errorf("etag = %q, want the new one kept for the next Update", ins.Source.ETag)
	}
}

func TestUpdate_InstallsANewerVersion(t *testing.T) {
	// Arrange
	r, base := serve(t)
	r.set("/c.zip", zipOf(t, "c", "1.0.0"), `"a"`)
	m := &Manager{Dir: t.TempDir()}
	if _, err := m.Install(t.Context(), base+"/c.zip", nil); err != nil {
		t.Fatal(err)
	}
	r.set("/c.zip", zipOf(t, "c", "1.1.0"), `"b"`)

	// Act
	ins, err := m.Update(t.Context(), "c", nil)

	// Assert
	if err != nil || ins.Manifest.Version != "1.1.0" {
		t.Fatalf("update = %+v, %v", ins, err)
	}
}

func TestUpdate_AGitRefAtTheSameCommitIsUpToDate(t *testing.T) {
	// Arrange
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	writeExtension(t, repo, "c", "2.0.0")
	gitIn := func(args ...string) {
		t.Helper()
		base := []string{"-C", repo, "-c", "user.email=t@example.com", "-c", "user.name=t", "-c", "commit.gpgsign=false"}
		if out, err := exec.Command("git", append(base, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitIn("init", "-q", "-b", "main")
	gitIn("add", ".")
	gitIn("commit", "-q", "-m", "first")
	m := &Manager{Dir: t.TempDir()}
	if _, err := m.Install(t.Context(), "file://"+repo, nil); err != nil {
		t.Fatal(err)
	}

	// Act
	_, sameErr := m.Update(t.Context(), "c", nil)
	gitIn("commit", "-q", "--allow-empty", "-m", "second")
	_, newErr := m.Update(t.Context(), "c", nil)

	// Assert
	if !errors.Is(sameErr, ErrUpToDate) {
		t.Errorf("same commit: err = %v, want ErrUpToDate", sameErr)
	}
	if newErr != nil {
		t.Errorf("new commit: err = %v, want it installed", newErr)
	}
}
