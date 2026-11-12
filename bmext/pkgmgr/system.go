package pkgmgr

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

var systemSuffix = regexp.MustCompile(`-(linux|darwin|windows|freebsd|openbsd|netbsd)-(amd64|arm64|386|arm|riscv64)(\.exe)?$`)

// SystemOf is the system a program is built for, when its name says one
// (bin/x-linux-amd64, bin/x-windows-amd64.exe).
func SystemOf(name string) (goos, goarch string, ok bool) {
	m := systemSuffix.FindStringSubmatch(name)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// ForSystem says whether a file named name is of use on goos/goarch: a
// program built for another system is not.
func ForSystem(name, goos, goarch string) bool {
	g, a, ok := SystemOf(name)
	return !ok || (g == goos && a == goarch)
}

// prune removes from dir the programs built for other systems, which an
// extension built for every system carries.
func prune(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || ForSystem(d.Name(), runtime.GOOS, runtime.GOARCH) {
			return nil
		}
		return os.Remove(path)
	})
}

// systemURL is the zip at url made for this system alone, as bmx pack
// writes it beside the one for every system: web.zip, web-linux-amd64.zip.
func systemURL(url string) string {
	return url[:len(url)-len(".zip")] + "-" + runtime.GOOS + "-" + runtime.GOARCH + ".zip"
}
