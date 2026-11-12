package pkgmgr

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// zipOf is an extension's zip holding files besides its manifest.
func zipOf(t *testing.T, name, version string, files ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(path, body string) {
		w, err := zw.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	add("bmext.json", `{"name": "`+name+`", "version": "`+version+`"}`)
	for _, f := range files {
		add(f, "x")
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// releases serves zips by path with an ETag each, as GitHub does, and
// counts the bodies it sends.
type releases struct {
	mu    sync.Mutex
	zips  map[string][]byte
	etags map[string]string
	sent  int
}

func (r *releases) set(path string, body []byte, etag string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.zips[path], r.etags[path] = body, etag
}

func (r *releases) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	body, ok := r.zips[req.URL.Path]
	if !ok {
		http.NotFound(w, req)
		return
	}
	w.Header().Set("ETag", r.etags[req.URL.Path])
	if req.Header.Get("If-None-Match") == r.etags[req.URL.Path] {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	r.sent++
	_, _ = w.Write(body)
}

func serve(t *testing.T) (*releases, string) {
	t.Helper()
	r := &releases{zips: map[string][]byte{}, etags: map[string]string{}}
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return r, srv.URL
}

func TestInstall_DownloadsTheZipForThisSystemWhenThereIsOne(t *testing.T) {
	// Arrange
	r, base := serve(t)
	r.set("/c.zip", zipOf(t, "c", "1.0.0", "all.txt"), `"a"`)
	r.set("/c-"+runtime.GOOS+"-"+runtime.GOARCH+".zip", zipOf(t, "c", "1.0.0", "mine.txt"), `"b"`)
	m := &Manager{Dir: t.TempDir()}

	// Act
	ins, err := m.Install(t.Context(), base+"/c.zip", nil)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ins.Dir, "mine.txt")); err != nil {
		t.Error("the zip for this system should be installed")
	}
	if ins.Source.Spec != base+"/c.zip" {
		t.Errorf("source = %q, want the URL given, which Update follows", ins.Source.Spec)
	}
}

func TestInstall_LeavesOutOtherSystemsPrograms(t *testing.T) {
	// Arrange
	other := "windows"
	if runtime.GOOS == "windows" {
		other = "linux"
	}
	src := filepath.Join(t.TempDir(), "c-ext")
	writeExtension(t, src, "c", "1.0.0")
	mine := "host-" + runtime.GOOS + "-" + runtime.GOARCH
	for _, f := range []string{mine, "host-" + other + "-amd64", "host-" + other + "-amd64.exe", "readme-linux.txt"} {
		if err := os.MkdirAll(filepath.Join(src, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, "bin", f), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	m := &Manager{Dir: t.TempDir()}

	// Act
	ins, err := m.Install(t.Context(), src, nil)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(ins.Dir, "bin"))
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 2 || !contains(names, mine) || !contains(names, "readme-linux.txt") {
		t.Fatalf("bin = %v, want only %s and readme-linux.txt", names, mine)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
