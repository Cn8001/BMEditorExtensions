package download

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFetch_GoesOnWhereItStoppedAndChecks(t *testing.T) {
	// Arrange: a file, half of it downloaded already.
	data := bytes.Repeat([]byte("model weights "), 10000)
	sum := sha256.Sum256(data)
	var ranges []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.Header.Get("Range"))
		http.ServeContent(w, r, "m.gguf", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "models", "m.gguf")
	_ = os.MkdirAll(filepath.Dir(dest), 0o755)
	_ = os.WriteFile(dest+".part", data[:len(data)/2], 0o644)

	// Act
	err := Fetch(context.Background(), srv.URL, dest, int64(len(data)), "sha256:"+hex.EncodeToString(sum[:]), nil)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, data) {
		t.Error("the file is not the one served")
	}
	if len(ranges) != 1 || ranges[0] != "bytes=70000-" {
		t.Errorf("ranges asked = %v", ranges)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Error("the .part should be gone")
	}
}

func TestFetch_RefusesAWrongFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("not it")) }))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "m.gguf")
	err := Fetch(context.Background(), srv.URL, dest, 6, strings.Repeat("0", 64), nil)
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("a file that failed the check should not be kept")
	}
}

func TestExtract_RefusesPathsOutside(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "evil.zip")
	f, _ := os.Create(archive)
	w := zip.NewWriter(f)
	e, _ := w.Create("../../evil.txt")
	_, _ = e.Write([]byte("x"))
	_ = w.Close()
	_ = f.Close()
	if err := Extract(archive, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Error("a path out of the folder should be refused")
	}
}
