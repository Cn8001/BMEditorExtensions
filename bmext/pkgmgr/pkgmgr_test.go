package pkgmgr

import (
	"archive/zip"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Cn8001/BMEditorExtensions/bmext"
)

func writeExtension(t *testing.T, dir, name, version string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name": "` + name + `", "version": "` + version + `", "languages": [{"id": "c", "label": "C", "extensions": [".c"], "server": {"command": ["clangd"]}}]}`
	if err := os.WriteFile(filepath.Join(dir, bmext.ManifestFile), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInstall_CopiesAFolderAndRecordsItsSource(t *testing.T) {
	// Arrange
	src := filepath.Join(t.TempDir(), "c-ext")
	writeExtension(t, src, "c", "1.0.0")
	m := &Manager{Dir: t.TempDir()}

	// Act
	ins, err := m.Install(t.Context(), src, nil)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if ins.Manifest.Name != "c" || ins.Source.Spec != src || ins.Dir != filepath.Join(m.Dir, "c") {
		t.Fatalf("installed = %+v", ins)
	}
	list, broken, _ := m.List()
	if len(list) != 1 || len(broken) != 0 {
		t.Fatalf("list = %+v, broken = %+v", list, broken)
	}
}

func TestInstall_ClonesAGitRepositoryAtARef(t *testing.T) {
	// Arrange
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	writeExtension(t, repo, "c", "2.0.0")
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "."},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "first"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "-c", "tag.gpgSign=false", "tag", "-a", "v2.0.0", "-m", "v2.0.0"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	m := &Manager{Dir: t.TempDir()}

	// Act
	ins, err := m.Install(t.Context(), "file://"+repo+"#v2.0.0", nil)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if ins.Manifest.Version != "2.0.0" || ins.Source.Commit == "" {
		t.Fatalf("installed = %+v", ins)
	}
	if _, err := os.Stat(filepath.Join(ins.Dir, ".git")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf(".git was copied: %v", err)
	}
}

func TestInstall_ReplacesTheOlderVersion(t *testing.T) {
	// Arrange
	src := filepath.Join(t.TempDir(), "c-ext")
	writeExtension(t, src, "c", "1.0.0")
	m := &Manager{Dir: t.TempDir()}
	if _, err := m.Install(t.Context(), src, nil); err != nil {
		t.Fatal(err)
	}
	writeExtension(t, src, "c", "1.1.0")

	// Act
	ins, err := m.Update(t.Context(), "c", nil)

	// Assert
	if err != nil || ins.Manifest.Version != "1.1.0" {
		t.Fatalf("installed = %+v, %v", ins, err)
	}
}

func TestInstall_RefusesAnArchiveThatEscapesItsFolder(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "evil.zip")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("../outside.txt")
	w.Write([]byte("x"))
	zw.Close()
	f.Close()
	m := &Manager{Dir: t.TempDir()}

	// Act
	_, err := m.Install(t.Context(), path, nil)

	// Assert
	if !errors.Is(err, ErrBadSource) {
		t.Fatalf("err = %v, want ErrBadSource", err)
	}
}

func TestClassify_ReadsEveryKindOfGitAddress(t *testing.T) {
	cases := map[string][2]string{
		"git@gitlab.com:me/ext.git":            {"git@gitlab.com:me/ext.git", ""},
		"https://github.com/me/ext#v1.2.0":     {"https://github.com/me/ext", "v1.2.0"},
		"git.example.org/team/tools/ext":       {"https://git.example.org/team/tools/ext", ""},
		"ssh://git@git.example.org:2222/x.git": {"ssh://git@git.example.org:2222/x.git", ""},
	}
	for spec, want := range cases {
		// Act
		kind, target, ref := classify(spec)

		// Assert
		if kind != "git" || target != want[0] || ref != want[1] {
			t.Errorf("%s: got %s %s %s", spec, kind, target, ref)
		}
	}
}

func TestRead_RefusesAPathOutsideTheExtension(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, bmext.ManifestFile), []byte(`{"name":"x","version":"1","backend":{"command":"../../bin/sh"}}`), 0o644)

	// Act
	_, err := bmext.Read(dir)

	// Assert
	if !errors.Is(err, bmext.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestRead_AcceptsTheCExample(t *testing.T) {
	// Act
	m, err := bmext.Read(filepath.Join("..", "examples", "c"))

	// Assert
	if err != nil || m.Languages[0].Server.Command[0] != "clangd" || m.Debuggers[0].Command[0] != "gdb" {
		t.Fatalf("manifest = %+v, %v", m, err)
	}
}

func TestInstallClone_InstallsWhatACallerCloned(t *testing.T) {
	// Arrange
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	writeExtension(t, repo, "c", "3.0.0")
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "."},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "first"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	m := &Manager{Dir: t.TempDir()}
	spec := "file://" + repo
	dir, err := m.CloneDir()
	if err != nil {
		t.Fatal(err)
	}
	argv, err := m.CloneCommand(spec, dir)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("%v: %v\n%s", argv, err, out)
	}

	// Act
	ins, err := m.InstallClone(t.Context(), dir, spec, nil)

	// Assert
	if err != nil || ins.Manifest.Version != "3.0.0" || ins.Source.Spec != spec || ins.Source.Commit == "" {
		t.Fatalf("installed %+v, %v", ins, err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clone folder left behind: %v", err)
	}
}

func TestRemoveClone_LeavesOtherFoldersAlone(t *testing.T) {
	// Arrange
	m := &Manager{Dir: t.TempDir()}
	other := filepath.Join(m.Dir, "c")
	os.MkdirAll(other, 0o755)

	// Act
	err := m.RemoveClone(other)

	// Assert
	if !errors.Is(err, ErrBadSource) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatal("the folder was removed")
	}
}
