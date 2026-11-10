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

// cmdDev runs the manifest's build for this system only (TARGETS, which the
// Makefile of bmx init reads) and installs the result as the editor does,
// in place of the installed one. The editor uses it once its window
// reloads.
func cmdDev(args []string) error {
	extDir, err := extensionsDir()
	if err != nil {
		return err
	}
	if err := flags("dev", args, func(fs *flag.FlagSet) {
		fs.StringVar(&extDir, "dir", extDir, "the editor's extensions folder")
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
		if err := build(dir, m, runtime.GOOS+"/"+runtime.GOARCH); err != nil {
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

// extensionsDir is where the editor keeps extensions:
// ~/.config/bmeditor/extensions on Linux, %AppData%\bmeditor\extensions on
// Windows.
func extensionsDir() (string, error) {
	c, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(c, "bmeditor", "extensions"), nil
}
