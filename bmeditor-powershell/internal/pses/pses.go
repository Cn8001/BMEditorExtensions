// Package pses finds, installs and starts PowerShell Editor Services, the
// language server and debug adapter of PowerShell (the one VS Code's
// PowerShell extension runs).
package pses

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// DownloadURL is the latest release's bundle: PowerShellEditorServices,
// PSScriptAnalyzer and PSReadLine, side by side.
const DownloadURL = "https://github.com/PowerShell/PowerShellEditorServices/releases/latest/download/PowerShellEditorServices.zip"

// Home is the extension's own folder in the user's cache, where Install
// puts the bundle and the logs go.
func Home() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "bmeditor-powershell"), nil
}

// Bundle is an installed bundle: the folder that holds
// PowerShellEditorServices/Start-EditorServices.ps1.
type Bundle struct {
	Dir     string `json:"dir"`
	Version string `json:"version,omitempty"`
	From    string `json:"from"` // install, env, vscode
}

func (b Bundle) Script() string {
	return filepath.Join(b.Dir, "PowerShellEditorServices", "Start-EditorServices.ps1")
}

// Find looks for a bundle: BMEDITOR_PSES, the one Install wrote, then the
// newest VS Code PowerShell extension's.
func Find() (Bundle, bool) {
	if d := os.Getenv("BMEDITOR_PSES"); d != "" {
		if b, ok := bundleIn(d, "env"); ok {
			return b, true
		}
	}
	if home, err := Home(); err == nil {
		if b, ok := bundleIn(filepath.Join(home, "PowerShellEditorServices"), "install"); ok {
			return b, true
		}
	}
	userHome, _ := os.UserHomeDir()
	for _, dir := range []string{".vscode", ".vscode-insiders", ".cursor", ".vscode-oss"} {
		exts, _ := filepath.Glob(filepath.Join(userHome, dir, "extensions", "ms-vscode.powershell-*"))
		sort.Sort(sort.Reverse(sort.StringSlice(exts)))
		for _, e := range exts {
			if b, ok := bundleIn(filepath.Join(e, "modules"), "vscode"); ok {
				return b, true
			}
		}
	}
	return Bundle{}, false
}

// bundleIn finds a bundle at dir or one folder below it (a zip unpacked
// into a folder of its own).
func bundleIn(dir, from string) (Bundle, bool) {
	cands := []string{dir}
	sub, _ := os.ReadDir(dir)
	for _, s := range sub {
		if s.IsDir() {
			cands = append(cands, filepath.Join(dir, s.Name()))
		}
	}
	for _, c := range cands {
		b := Bundle{Dir: c, From: from}
		if isFile(b.Script()) {
			b.Version = moduleVersion(filepath.Join(c, "PowerShellEditorServices", "PowerShellEditorServices.psd1"))
			return b, true
		}
	}
	return Bundle{}, false
}

var versionLine = regexp.MustCompile(`(?m)^\s*ModuleVersion\s*=\s*['"]([^'"]+)['"]`)

func moduleVersion(psd1 string) string {
	raw, err := os.ReadFile(psd1)
	if err != nil {
		return ""
	}
	if m := versionLine.FindSubmatch(raw); m != nil {
		return string(m[1])
	}
	return ""
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// Shell is the PowerShell to run Editor Services in: BMEDITOR_POWERSHELL,
// else PowerShell 7 (pwsh), else Windows PowerShell.
func Shell() (string, error) {
	if p := os.Getenv("BMEDITOR_POWERSHELL"); p != "" {
		return exec.LookPath(p)
	}
	if p, err := exec.LookPath("pwsh"); err == nil {
		return p, nil
	}
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("powershell"); err == nil {
			return p, nil
		}
	}
	return "", errors.New("PowerShell is not installed: install PowerShell 7 (pwsh)")
}

// Mode is what Editor Services serves.
type Mode struct {
	Debug    bool   // the debug adapter alone, not the language server
	Pipe     string // the debug adapter's named pipe; "" for stdio
	Session  string // the file it writes its session details to
	LogDir   string
	HostName string
}

// Command is the command line that starts Editor Services from bundle b.
func Command(shell string, b Bundle, m Mode) *exec.Cmd {
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	script := fmt.Sprintf("& %s -BundledModulesPath %s -LogPath %s -SessionDetailsPath %s -FeatureFlags @() -AdditionalModules @() -HostName %s -HostProfileId 'BMEditor' -HostVersion '1.0.0'",
		q(b.Script()), q(b.Dir), q(filepath.Join(m.LogDir, "EditorServices.log")), q(m.Session), q(m.HostName))
	switch {
	case m.Debug && m.Pipe != "":
		script += " -DebugServiceOnly -DebugServicePipeName " + q(m.Pipe)
	case m.Debug:
		script += " -DebugServiceOnly -Stdio"
	default:
		script += " -Stdio"
	}
	args := []string{"-NoLogo", "-NoProfile", "-NonInteractive"}
	if runtime.GOOS == "windows" {
		args = append(args, "-ExecutionPolicy", "Bypass")
	}
	args = append(args, "-Command", script)
	return exec.Command(shell, args...)
}

// Install downloads the latest bundle into Home, replacing the one there.
// progress gets each step as a line.
func Install(ctx context.Context, progress func(string)) (Bundle, error) {
	home, err := Home()
	if err != nil {
		return Bundle{}, err
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return Bundle{}, err
	}
	progress("Downloading " + DownloadURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DownloadURL, nil)
	if err != nil {
		return Bundle{}, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Bundle{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Bundle{}, fmt.Errorf("download: %s", res.Status)
	}
	tmp, err := os.CreateTemp(home, "download-*.zip")
	if err != nil {
		return Bundle{}, err
	}
	defer os.Remove(tmp.Name())
	n, err := io.Copy(tmp, res.Body)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return Bundle{}, err
	}
	progress(fmt.Sprintf("Downloaded %.1f MB, unpacking", float64(n)/1e6))
	part := filepath.Join(home, fmt.Sprintf("PowerShellEditorServices.part-%d", os.Getpid()))
	_ = os.RemoveAll(part)
	defer os.RemoveAll(part)
	if err := unzip(tmp.Name(), part); err != nil {
		return Bundle{}, err
	}
	if _, ok := bundleIn(part, "install"); !ok {
		return Bundle{}, errors.New("the download has no PowerShellEditorServices/Start-EditorServices.ps1")
	}
	dest := filepath.Join(home, "PowerShellEditorServices")
	old := dest + ".old"
	_ = os.RemoveAll(old)
	if _, err := os.Stat(dest); err == nil {
		// A running Editor Services keeps its files open: move it aside.
		if err := os.Rename(dest, old); err != nil {
			return Bundle{}, fmt.Errorf("the installed Editor Services is in use; close PowerShell files and debugging, then install again: %w", err)
		}
	}
	if err := os.Rename(part, dest); err != nil {
		return Bundle{}, err
	}
	_ = os.RemoveAll(old)
	b, _ := bundleIn(dest, "install")
	progress("Installed PowerShell Editor Services " + b.Version + " in " + b.Dir)
	return b, nil
}

func unzip(path, dest string) error {
	z, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, f := range z.File {
		name := filepath.FromSlash(strings.ReplaceAll(f.Name, `\`, "/"))
		to := filepath.Join(dest, name)
		if !strings.HasPrefix(to, filepath.Clean(dest)+string(filepath.Separator)) {
			return fmt.Errorf("unsafe path in the zip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(to, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		if err := writeFile(f, to); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(f *zip.File, to string) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, r)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}
