// Command hex-host is the Hex extension's backend: it reads a file a page
// at a time and writes the bytes changed in it back, in place, inside the
// open folder only.
package main

import (
	"context"
	"encoding/json"

	"bmeditor-hex/bmext/host"
	"bmeditor-hex/internal/hexfile"
)

func main() {
	s := host.New()
	s.Handle("read", handler(s, func(root string, p struct {
		Path   string
		Offset int64
		Size   int
	}) (any, error) {
		return hexfile.Read(root, p.Path, p.Offset, p.Size)
	}))
	s.Handle("write", handler(s, func(root string, p struct {
		Path    string
		Changes []hexfile.Change
	}) (any, error) {
		return nil, hexfile.Write(root, p.Path, p.Changes)
	}))
	host.Run(s)
}

// handler gives fn the open folder and its parameters.
func handler[T any](s *host.Server, fn func(root string, p T) (any, error)) host.Handler {
	return func(_ context.Context, raw json.RawMessage) (any, error) {
		c := s.Context()
		root := c.Folder
		if root == "" {
			root = c.Project.Dir
		}
		if root == "" {
			return nil, &host.Error{Code: "NO_FOLDER", Message: "Open a folder first."}
		}
		var p T
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, &host.Error{Code: "BAD_PARAMS", Message: err.Error()}
		}
		out, err := fn(root, p)
		if err != nil {
			return nil, &host.Error{Code: "HEX", Message: err.Error()}
		}
		return out, nil
	}
}
