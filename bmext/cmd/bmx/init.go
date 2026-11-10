package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/Cn8001/BMEditorExtensions/bmext"
)

// The files of a new extension: base/ for every one, backend/ for one with
// a Go backend. NAME in a path is the extension's name, a .tmpl suffix is
// dropped, and a file named gitignore becomes .gitignore.
//
//go:embed all:template
var templates embed.FS

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type data struct {
	Name    string // the manifest's name: vim
	Title   string // Vim
	Module  string // the Go module and npm package: bmeditor-vim
	Backend bool
}

// cmdInit writes the files of a new extension that are not there yet, and
// the copy of the SDK in bmext/ always, so it also updates an existing one.
func cmdInit(args []string) error {
	var d data
	if err := flags("init", args, func(fs *flag.FlagSet) {
		fs.BoolVar(&d.Backend, "backend", false, "add a Go backend, cmd/<name>-host")
		fs.StringVar(&d.Name, "name", "", "the extension's name (default: from bmext.json or the folder)")
	}); err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	d.Module = filepath.Base(dir)
	if d.Name == "" {
		if m, err := bmext.Read(dir); err == nil {
			d.Name = m.Name
		} else {
			d.Name = strings.TrimPrefix(d.Module, "bmeditor-")
		}
	}
	if !namePattern.MatchString(d.Name) {
		return fmt.Errorf("name %q: use lower-case letters, digits and dashes (-name)", d.Name)
	}
	d.Title = strings.ToUpper(d.Name[:1]) + strings.ReplaceAll(d.Name[1:], "-", " ")

	parts := []string{"template/base"}
	if d.Backend {
		parts = append(parts, "template/backend")
	}
	for _, root := range parts {
		if err := writeTemplates(dir, root, d); err != nil {
			return err
		}
	}
	if err := writeSDK(dir); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "package-lock.json")); errors.Is(err, fs.ErrNotExist) {
		fmt.Println("\nNext: npm install, then bmx dev.")
	}
	return nil
}

func writeTemplates(dir, root string, d data) error {
	return fs.WalkDir(templates, root, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		rel := strings.ReplaceAll(strings.TrimPrefix(p, root+"/"), "NAME", d.Name)
		rel = strings.TrimSuffix(rel, ".tmpl") // Go files, which this module would otherwise build
		if path.Base(rel) == "gitignore" {
			rel = path.Join(path.Dir(rel), ".gitignore")
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if _, err := os.Stat(dst); err == nil {
			fmt.Println("keep  ", rel)
			return nil
		}
		src, err := templates.ReadFile(p)
		if err != nil {
			return err
		}
		t, err := template.New(rel).Parse(string(src))
		if err != nil {
			return err
		}
		var out bytes.Buffer
		if err := t.Execute(&out, d); err != nil {
			return err
		}
		if strings.HasSuffix(rel, ".json") && !json.Valid(out.Bytes()) {
			return fmt.Errorf("template %s is not valid JSON", rel)
		}
		fmt.Println("create", rel)
		return write(dst, out.Bytes())
	})
}

// writeSDK copies the SDK into bmext/: the frontend files always, the
// backend's when the extension has a Go backend.
func writeSDK(dir string) error {
	host := wantsHost(dir)
	return fs.WalkDir(bmext.SDK, ".", func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		rel, ok := sdkPath(p, host)
		if !ok {
			return nil
		}
		b, err := bmext.SDK.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if old, err := os.ReadFile(dst); err == nil && bytes.Equal(old, b) {
			return nil
		}
		fmt.Println("sdk   ", rel)
		return write(dst, b)
	})
}

// sdkPath is where an SDK file goes in an extension: frontend/x as bmext/x,
// host/x as bmext/host/x (only when it wants the backend SDK).
func sdkPath(p string, host bool) (string, bool) {
	if rest, ok := strings.CutPrefix(p, "frontend/"); ok {
		return "bmext/" + rest, true
	}
	if strings.HasPrefix(p, "host/") && host {
		return "bmext/" + p, true
	}
	return "", false
}

// wantsHost says whether the extension in dir keeps a copy of the backend
// SDK: it has a backend, written in Go. Go code that is not the backend
// (a language server, say) does not need it.
func wantsHost(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return false
	}
	m, err := bmext.Read(dir)
	return err == nil && m.Backend != nil
}

func write(dst string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}
