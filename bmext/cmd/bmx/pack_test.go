package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/Cn8001/BMEditorExtensions/bmext/pkgmgr"
)

func TestPack_ZipsEachSystemsProgramsApart(t *testing.T) {
	// Arrange
	root := t.TempDir()
	for _, f := range []string{"bmext.json", "bin/ls-linux-amd64", "bin/ls-windows-amd64.exe", "bin/host-linux-amd64"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dst := filepath.Join(t.TempDir(), "linux.zip")

	// Act
	systems, err := systemsIn(root)
	if err == nil {
		err = zipDir(root, dst, func(name string) bool { return pkgmgr.ForSystem(name, "linux", "amd64") })
	}

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"linux-amd64", "windows-amd64"}; !reflect.DeepEqual(systems, want) {
		t.Errorf("systems = %v, want %v", systems, want)
	}
	zr, err := zip.OpenReader(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	if want := []string{"bin/host-linux-amd64", "bin/ls-linux-amd64", "bmext.json"}; !reflect.DeepEqual(names, want) {
		t.Errorf("zip = %v, want %v", names, want)
	}
}
