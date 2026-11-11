package pses

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bundle(t *testing.T, dir string) {
	t.Helper()
	p := filepath.Join(dir, "PowerShellEditorServices")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(p, "Start-EditorServices.ps1"), []byte("# start"), 0o644)
	_ = os.WriteFile(filepath.Join(p, "PowerShellEditorServices.psd1"), []byte("@{\n    ModuleVersion = '4.3.0'\n}\n"), 0o644)
}

func TestFind_TakesTheEnvironmentsThenTheInstalledOne(t *testing.T) {
	// Arrange: one installed, one named by BMEDITOR_PSES, a folder below.
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("LOCALAPPDATA", cache)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("BMEDITOR_PSES", "")
	if _, ok := Find(); ok {
		t.Fatal("found a bundle where there is none")
	}
	home, _ := Home()
	bundle(t, filepath.Join(home, "PowerShellEditorServices"))
	b, ok := Find()
	if !ok || b.From != "install" || b.Version != "4.3.0" {
		t.Fatalf("installed = %+v, %v", b, ok)
	}
	env := t.TempDir()
	bundle(t, filepath.Join(env, "unzipped"))
	t.Setenv("BMEDITOR_PSES", env)

	// Act
	b, ok = Find()

	// Assert
	if !ok || b.From != "env" || b.Dir != filepath.Join(env, "unzipped") {
		t.Errorf("with BMEDITOR_PSES = %+v, %v", b, ok)
	}
}

func TestCommand_QuotesPathsForPowerShell(t *testing.T) {
	b := Bundle{Dir: `C:\Users\o'neil\PSES`}
	cmd := Command("pwsh", b, Mode{Session: "s.json", LogDir: "logs", HostName: "BMEditor", Debug: true, Pipe: "p1"})
	script := cmd.Args[len(cmd.Args)-1]
	for _, want := range []string{`'C:\Users\o''neil\PSES'`, "-DebugServiceOnly -DebugServicePipeName 'p1'", "-SessionDetailsPath 's.json'"} {
		if !strings.Contains(script, want) {
			t.Errorf("script lacks %s:\n%s", want, script)
		}
	}
	lsp := Command("pwsh", b, Mode{Session: "s.json", LogDir: "logs"})
	if s := lsp.Args[len(lsp.Args)-1]; !strings.HasSuffix(s, " -Stdio") || strings.Contains(s, "DebugServiceOnly") {
		t.Errorf("the language server's script = %s", s)
	}
}

func TestUnzip_RefusesPathsOutside(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "evil.zip")
	f, _ := os.Create(zipPath)
	w := zip.NewWriter(f)
	e, _ := w.Create("../evil.txt")
	_, _ = e.Write([]byte("x"))
	_ = w.Close()
	_ = f.Close()
	if err := unzip(zipPath, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Error("a path out of the folder should be refused")
	}
}
