// Package hexfile reads a file a page at a time and writes changed bytes
// back in place, for the hex editor. Every path is opened through an
// os.Root of the open folder, so neither ".." nor a symlink reaches out.
package hexfile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MaxRead bounds one Read: the editor asks for the page on screen.
const MaxRead = 1 << 20

// Page is part of a file. Data travels as base64, as JSON carries bytes.
type Page struct {
	Data []byte `json:"data"`
	Size int64  `json:"size"` // of the whole file
}

// Change is a run of bytes written over the file from Offset.
type Change struct {
	Offset int64  `json:"offset"`
	Data   []byte `json:"data"`
}

func open(root, path string, flag int) (*os.File, error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil, fmt.Errorf("%s is outside the open folder", path)
	}
	return r.OpenFile(rel, flag, 0)
}

// Read reads up to n bytes of path from offset.
func Read(root, path string, offset int64, n int) (Page, error) {
	f, err := open(root, path, os.O_RDONLY)
	if err != nil {
		return Page{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Page{}, err
	}
	out := Page{Data: []byte{}, Size: info.Size()}
	if offset < 0 || offset >= info.Size() || n <= 0 {
		return out, nil
	}
	buf := make([]byte, min(int64(min(n, MaxRead)), info.Size()-offset))
	got, err := f.ReadAt(buf, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return Page{}, err
	}
	out.Data = buf[:got]
	return out, nil
}

// Write writes each change over path, in place: the file keeps its size,
// as an edit in a hex editor overwrites bytes rather than inserting them.
// A change past the end is refused, before anything is written.
func Write(root, path string, changes []Change) error {
	f, err := open(root, path, os.O_RDWR)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	for _, c := range changes {
		if c.Offset < 0 || c.Offset+int64(len(c.Data)) > info.Size() {
			return fmt.Errorf("a change at %d runs past the end of the file (%d bytes)", c.Offset, info.Size())
		}
	}
	for _, c := range changes {
		if _, err := f.WriteAt(c.Data, c.Offset); err != nil {
			return err
		}
	}
	return f.Close()
}
