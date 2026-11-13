package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Cn8001/BMEditorExtensions/bmext"
	"github.com/Cn8001/BMEditorExtensions/bmext/pkgmgr"
)

// The systems an extension's programs are built for.
var targets = []string{"linux-amd64", "linux-arm64", "windows-amd64", "darwin-amd64", "darwin-arm64"}

// cmdCheck installs the extension the way the editor installs it from its
// git repository, from a clone of only its own files, into a throwaway
// extensions folder, and checks that what was installed has every file its
// manifest names. That covers the zip as well: it is the same output. A
// copy of the SDK's host package must be this bmx's; older frontend types
// are only noted.
func cmdCheck(args []string) error {
	if err := flags("check", args, func(*flag.FlagSet) {}); err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	if _, err := bmext.Read(dir); err != nil {
		return err
	}
	var problems []string
	// Older types are the extension's to update when it needs something
	// new, so a change to them does not touch every extension; the host
	// package is code the editor talks to, and must be this one.
	for _, d := range sdkDrift(dir) {
		if strings.HasPrefix(d, "bmext/host/") {
			problems = append(problems, d)
		} else {
			fmt.Println("! " + d)
		}
	}

	tmp, err := os.MkdirTemp("", "bmx-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	repo := filepath.Join(tmp, "repo")
	if err := standaloneRepo(dir, repo); err != nil {
		return err
	}
	fmt.Println("Installing from a clean clone of the extension's own files:")
	mgr := &pkgmgr.Manager{Dir: filepath.Join(tmp, "extensions"), AllSystems: true}
	ins, err := mgr.Install(context.Background(), fileURL(repo), func(line string) { fmt.Println("  " + line) })
	if err != nil {
		problems = append(problems, "it does not install from its own repository: "+err.Error())
	} else {
		problems = append(problems, missing(ins.Dir, ins.Manifest)...)
	}

	if len(problems) > 0 {
		fmt.Println()
		for _, p := range problems {
			fmt.Println("✗", p)
		}
		return fmt.Errorf("%d problem(s)", len(problems))
	}
	fmt.Println("\n✓ installs on its own, and has every file its manifest names")
	return nil
}

// sdkDrift lists the files of the extension's SDK copy that differ from
// this bmx's SDK.
func sdkDrift(dir string) []string {
	host := wantsHost(dir)
	var out []string
	_ = fs.WalkDir(bmext.SDK, ".", func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		rel, ok := sdkPath(p, host)
		if !ok {
			return nil
		}
		want, _ := bmext.SDK.ReadFile(p)
		have, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		switch {
		case errors.Is(err, fs.ErrNotExist) && strings.HasPrefix(rel, "bmext/host/"):
			// A Go backend may not use the SDK: nothing to keep up to date.
		case err != nil:
			out = append(out, rel+" is missing; bmx init writes it")
		case !bytes.Equal(have, want):
			out = append(out, rel+" is not this bmx's SDK; bmx init updates it")
		}
		return nil
	})
	return out
}

// standaloneRepo makes repo a git repository of dir's files alone, as the
// extension's own repository would have them: those git tracks, and new
// ones it does not ignore.
func standaloneRepo(dir, repo string) error {
	ls, err := exec.Command("git", "-C", dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return fmt.Errorf("check needs the extension in a git repository: %w", err)
	}
	for rel := range strings.SplitSeq(strings.TrimSuffix(string(ls), "\x00"), "\x00") {
		src := filepath.Join(dir, filepath.FromSlash(rel))
		info, err := os.Lstat(src)
		if err != nil || !info.Mode().IsRegular() { // deleted, or a link the clone would not follow out
			continue
		}
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		dst := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, info.Mode().Perm()); err != nil {
			return err
		}
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		// A throwaway commit: not signed (gpg would wait for a passphrase), no hooks.
		{"-c", "user.name=bmx", "-c", "user.email=bmx@localhost", "-c", "commit.gpgsign=false",
			"commit", "-q", "--no-verify", "-m", "bmx check"},
	} {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %w: %s", args[0], err, out)
		}
	}
	return nil
}

func fileURL(p string) string {
	p = filepath.ToSlash(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // C:/... on Windows
	}
	return "file://" + p
}

// missing lists the files the manifest names that the installed extension
// does not have. A program it carries, ./bin/name, must be there for every
// system, as bin/name-GOOS-GOARCH[.exe], or once as bin/name for all.
func missing(root string, m bmext.Manifest) []string {
	var out []string
	file := func(what, p string) {
		if p == "" {
			return
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil {
			out = append(out, fmt.Sprintf("%s %s is not in the installed extension", what, p))
		}
	}
	program := func(what, cmd string) {
		p, ok := strings.CutPrefix(cmd, "./")
		if !ok && what != "backend" {
			return // on PATH
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err == nil {
			return
		}
		var lacking []string
		for _, t := range targets {
			name := p + "-" + t
			if strings.HasPrefix(t, "windows") {
				name += ".exe"
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
				lacking = append(lacking, t)
			}
		}
		if len(lacking) > 0 {
			out = append(out, fmt.Sprintf("%s %s is missing for %s", what, p, strings.Join(lacking, ", ")))
		}
	}

	file("icon", m.Icon)
	for _, l := range m.Languages {
		file("language "+l.ID+" icon", l.Icon)
		if l.Server != nil {
			program("language "+l.ID+" server", l.Server.Command[0])
		}
	}
	for _, k := range m.ProjectKinds {
		file("project kind "+k.Kind+" icon", k.Icon)
		file("project kind "+k.Kind+" template", k.Template)
	}
	for _, d := range m.Debuggers {
		program("debugger "+d.ID, d.Command[0])
	}
	if m.Frontend != nil {
		file("frontend script", m.Frontend.Script)
		file("frontend style", m.Frontend.Style)
	}
	if m.Backend != nil {
		program("backend", m.Backend.Command)
	}
	return out
}
