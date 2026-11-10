package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Cn8001/BMEditorExtensions/bmext"
)

// cmdPack runs the manifest's build for every system and zips its output as
// <name>-<version>.zip, the file to publish: BMEditor installs it from a
// path or a URL without building.
func cmdPack(args []string) error {
	if err := flags("pack", args, func(*flag.FlagSet) {}); err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	m, err := bmext.Read(dir)
	if err != nil {
		return err
	}
	out := dir
	if m.Build != nil {
		if err := build(dir, m, ""); err != nil {
			return err
		}
		out = filepath.Join(dir, filepath.FromSlash(m.Build.Output))
	}
	built, err := bmext.Read(out)
	if err != nil {
		return fmt.Errorf("the build's output: %w", err)
	}
	zipPath := filepath.Join(dir, fmt.Sprintf("%s-%s.zip", built.Name, built.Version))
	if err := zipDir(out, zipPath); err != nil {
		return err
	}
	fmt.Println(filepath.Base(zipPath))
	return nil
}

// build runs the manifest's build command in dir. targets, when given,
// limits it to those systems (TARGETS, which the Makefile of bmx init
// reads); otherwise the build's own list is used, TARGETS from the
// environment left out.
func build(dir string, m bmext.Manifest, targets string) error {
	fmt.Println("build:", strings.Join(m.Build.Command, " "))
	cmd := exec.Command(m.Build.Command[0], m.Build.Command[1:]...)
	cmd.Dir = dir
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "TARGETS=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	if targets != "" {
		cmd.Env = append(cmd.Env, "TARGETS="+targets)
	}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build: %w", err)
	}
	return nil
}

// zipDir writes the files under root to a new zip at dst, executables kept
// executable, leaving out what an install leaves out (version control,
// dependencies, symbolic links) and any zip in root itself.
func zipDir(root, dst string) error {
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	zw := zip.NewWriter(f)
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !d.Type().IsRegular() || p == tmp || strings.HasSuffix(p, ".zip") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		h.Name = filepath.ToSlash(rel)
		h.Method = zip.Deflate
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	})
	if err == nil {
		err = zw.Close()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
