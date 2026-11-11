// Package download fetches a large file (a model, a llama.cpp build) so
// that a broken download goes on where it stopped, checks it against the
// size and SHA-256 published for it, and unpacks archives.
package download

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Progress gets the bytes done and the total (0 when unknown).
type Progress func(done, total int64)

var client = &http.Client{} // no timeout: a model is gigabytes; ctx ends it

// Retries are the waits before trying again after a network failure: a
// lookup that failed, a connection cut. Each try goes on from the bytes
// already here.
var Retries = []time.Duration{2 * time.Second, 5 * time.Second, 15 * time.Second}

// Fetch downloads url to dest. A dest.part left by an earlier try is
// continued. size and sum (hex SHA-256) are checked when given; a file
// that fails the check is removed. A network failure is tried again.
func Fetch(ctx context.Context, url, dest string, size int64, sum string, progress Progress) error {
	err := fetch(ctx, url, dest, size, sum, progress)
	for _, wait := range Retries {
		if err == nil || !transient(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		err = fetch(ctx, url, dest, size, sum, progress)
	}
	return err
}

// transient says whether an error is the network's, worth another try:
// not a refusal by the server or a file that failed its check.
func transient(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var u *neturl.Error
	var n net.Error
	return errors.As(err, &u) || errors.As(err, &n) || errors.Is(err, io.ErrUnexpectedEOF)
}

func fetch(ctx context.Context, url, dest string, size int64, sum string, progress Progress) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	part := dest + ".part"
	var have int64
	if info, err := os.Stat(part); err == nil {
		have = info.Size()
	}
	if size > 0 && have > size {
		_ = os.Remove(part)
		have = 0
	}
	if size == 0 || have < size {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		if have > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
		}
		res, err := client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		flag := os.O_CREATE | os.O_WRONLY
		switch res.StatusCode {
		case http.StatusPartialContent:
			flag |= os.O_APPEND
		case http.StatusOK:
			flag |= os.O_TRUNC
			have = 0
		default:
			return fmt.Errorf("download %s: %s", url, res.Status)
		}
		total := size
		if total == 0 && res.ContentLength > 0 {
			total = have + res.ContentLength
		}
		f, err := os.OpenFile(part, flag, 0o644)
		if err != nil {
			return err
		}
		err = copyWithProgress(f, res.Body, have, total, progress)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err // the .part stays, to go on from next time
		}
	}
	if err := check(part, size, sum); err != nil {
		_ = os.Remove(part)
		return err
	}
	return os.Rename(part, dest)
}

func copyWithProgress(w io.Writer, r io.Reader, done, total int64, progress Progress) error {
	buf := make([]byte, 1<<20)
	last := time.Time{}
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			if progress != nil && time.Since(last) > 500*time.Millisecond {
				progress(done, total)
				last = time.Now()
			}
		}
		if err == io.EOF {
			if progress != nil {
				progress(done, total)
			}
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// check compares a file with its published size and SHA-256.
func check(path string, size int64, sum string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if size > 0 && info.Size() != size {
		return fmt.Errorf("%s: %d bytes, not the %d published", filepath.Base(path), info.Size(), size)
	}
	if sum == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, strings.TrimPrefix(sum, "sha256:")) {
		return fmt.Errorf("%s: SHA-256 %s, not the %s published", filepath.Base(path), got, sum)
	}
	return nil
}

// Extract unpacks a .zip or .tar.gz into dir, refusing paths that leave it.
func Extract(archive, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	switch {
	case strings.HasSuffix(archive, ".zip"):
		return extractZip(archive, dir)
	case strings.HasSuffix(archive, ".tar.gz"), strings.HasSuffix(archive, ".tgz"):
		return extractTarGz(archive, dir)
	}
	return fmt.Errorf("not a .zip or .tar.gz: %s", archive)
}

func target(dir, name string) (string, error) {
	to := filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(name, `\`, "/")))
	if to != filepath.Clean(dir) && !strings.HasPrefix(to, filepath.Clean(dir)+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe path in the archive: %s", name)
	}
	return to, nil
}

func extractZip(archive, dir string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, f := range z.File {
		to, err := target(dir, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(to, 0o755); err != nil {
				return err
			}
			continue
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		err = writeFile(to, r, f.Mode()|0o644)
		r.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(archive, dir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		to, err := target(dir, h.Name)
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(to, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeFile(to, tr, os.FileMode(h.Mode)|0o644); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// Shared libraries are linked by version (libllama.so.0 → ...).
			if strings.Contains(h.Linkname, "..") || filepath.IsAbs(h.Linkname) {
				return fmt.Errorf("unsafe link in the archive: %s -> %s", h.Name, h.Linkname)
			}
			_ = os.Remove(to)
			if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(h.Linkname, to); err != nil {
				return err
			}
		}
	}
}

func writeFile(to string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, r)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}
