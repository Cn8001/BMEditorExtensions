package unity

import (
	"os"
	"path/filepath"
)

// Home is the extension's folder for what it makes: the editor's data
// folder, or the user's cache under an editor before 0.1.6.
func Home() (string, error) {
	if d := os.Getenv("BMEDITOR_DATA"); d != "" {
		return filepath.Join(d, "extension-data", "csharp"), nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "bmeditor-csharp"), nil
}
