// Command vim-host is the Vim extension's backend. The window cannot read
// files, so it asks vim-host for the user's vimrc and applies it itself.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/Cn8001/BMEditorExtensions/bmext/host"
)

func main() {
	s := host.New()
	s.Handle("vimrc", func(ctx context.Context, _ json.RawMessage) (any, error) {
		c := s.Context()
		// A relative path in Project Settings is the project's file.
		base := c.Project.Dir
		if base == "" {
			base = c.Folder
		}
		home, _ := os.UserHomeDir() // without one only a named, absolute vimrc is found
		f, err := Read(ctx, Where{Setting: c.Setting("vimrc"), Base: base, Home: home})
		if errors.Is(err, ErrNotFound) {
			return nil, &host.Error{Code: "VIMRC_NOT_FOUND", Message: err.Error()}
		}
		return f, err
	})
	host.Run(s)
}
