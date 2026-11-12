package bmext

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The extensions in this repository keep copies of the SDK, and bmx check
// fails on a stale one, at publish. Caught here first, when the SDK
// changes: run bmx init in each extension to update its copy.
func TestSDKCopiesInThisRepository(t *testing.T) {
	// The extensions git has: not a folder of one's own beside them.
	out, err := exec.Command("git", "-C", "..", "ls-files", "*/bmext.json").Output()
	if err != nil {
		t.Skip("not in the extensions repository's git checkout")
	}
	var exts []string
	for _, f := range strings.Fields(string(out)) {
		if strings.Count(f, "/") == 1 && !strings.HasPrefix(f, "bmext/") { // an extension at the top, not bmx's template
			exts = append(exts, filepath.Join("..", filepath.FromSlash(f)))
		}
	}
	_ = fs.WalkDir(SDK, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		want, _ := SDK.ReadFile(path)
		// frontend/x lands as bmext/x; host/x as bmext/host/x.
		rel := strings.TrimPrefix(path, "frontend/")
		for _, m := range exts {
			ext := filepath.Dir(m)
			copyPath := filepath.Join(ext, "bmext", filepath.FromSlash(rel))
			got, err := os.ReadFile(copyPath)
			if os.IsNotExist(err) && strings.HasPrefix(path, "host/") {
				continue // an extension without a backend has no host
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s is not the SDK's %s: run bmx init in %s", copyPath, path, filepath.Base(ext))
			}
		}
		return nil
	})
}
