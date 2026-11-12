package bmext

import (
	"os"
	"path/filepath"
	"runtime"
)

// DataEnv is set by the editor for every program it starts, so an
// extension keeps its downloads in DataDir without guessing the editor's.
const DataEnv = "BMEDITOR_DATA"

// DataDir is the one folder the editor keeps everything but settings in:
// installed extensions, their downloads, caches. It is local, never roaming:
// %LOCALAPPDATA%\bmeditor, ~/Library/Application Support/bmeditor,
// $XDG_DATA_HOME/bmeditor (~/.local/share/bmeditor).
func DataDir() (string, error) {
	if d := os.Getenv(DataEnv); d != "" {
		return d, nil
	}
	var base string
	var err error
	switch runtime.GOOS {
	case "windows":
		base, err = os.UserCacheDir()
	case "darwin":
		base, err = os.UserConfigDir()
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" || !filepath.IsAbs(base) {
			var home string
			home, err = os.UserHomeDir()
			base = filepath.Join(home, ".local", "share")
		}
	}
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "bmeditor"), nil
}

// ExtensionsDir is where extensions are installed, one folder each.
func ExtensionsDir() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "extensions"), nil
}

// ExtensionData is the folder an extension's own downloads and logs go in.
func ExtensionData(name string) (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "extension-data", name), nil
}
