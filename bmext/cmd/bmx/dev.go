package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Cn8001/BMEditorExtensions/bmext"
	"github.com/Cn8001/BMEditorExtensions/bmext/pkgmgr"
)

// cmdDev runs the manifest's build for one system only, this one unless
// -target names another (TARGETS, which the Makefile of bmx init reads),
// and installs the result as the editor does, in place of the installed
// one. The editor uses it once its window reloads. -target and -dir install
// for an editor on another system, such as BMEditor on Windows from WSL:
//
//	bmx dev -target windows/amd64 -dir /mnt/c/Users/me/AppData/Roaming/bmeditor/extensions
func cmdDev(args []string) error {
	extDir, err := extensionsDir()
	if err != nil {
		return err
	}
	target := runtime.GOOS + "/" + runtime.GOARCH
	if err := flags("dev", args, func(fs *flag.FlagSet) {
		fs.StringVar(&extDir, "dir", extDir, "the editor's extensions folder")
		fs.StringVar(&target, "target", target, "the system to build for, os/arch")
	}); err != nil {
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
		if err := build(dir, m, target); err != nil {
			return err
		}
		out = filepath.Join(dir, filepath.FromSlash(m.Build.Output))
	}
	mgr := &pkgmgr.Manager{Dir: extDir}
	ins, err := mgr.Install(context.Background(), out, func(line string) { fmt.Println(line) })
	if err != nil {
		return err
	}
	fmt.Printf("Installed in %s. Reload the editor's window to use it.\n", ins.Dir)
	return nil
}

// extensionsDir is where the editor keeps extensions.
func extensionsDir() (string, error) { return bmext.ExtensionsDir() }
