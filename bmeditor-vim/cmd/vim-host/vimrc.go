package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound is a vimrc the user named that is not there. One BMEditor
// looks for on its own may be missing; that is no error.
var ErrNotFound = errors.New("vimrc not found")

// File is the vimrc the window applies; Path is empty when there is none.
type File struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

// Where says where to look: the setting as the project or the editor has
// it, the folder a relative setting is in, and the user's home.
type Where struct {
	Setting string
	Base    string
	Home    string
}

// defaults are where Vim itself looks, so the vimrc a Vim user already has
// works without a setting; _vimrc is its name on Windows.
var defaults = []string{".vimrc", "_vimrc", filepath.Join(".vim", "vimrc")}

// candidates are the files to try in order, and whether the user named it.
func candidates(w Where) ([]string, bool) {
	s := strings.TrimSpace(w.Setting)
	if s == "" {
		if w.Home == "" {
			return nil, false
		}
		paths := make([]string, 0, len(defaults))
		for _, d := range defaults {
			paths = append(paths, filepath.Join(w.Home, d))
		}
		return paths, false
	}
	if s == "~" || strings.HasPrefix(s, "~/") || strings.HasPrefix(s, `~\`) {
		s = filepath.Join(w.Home, s[1:])
	} else if !filepath.IsAbs(s) {
		base := w.Base
		if base == "" {
			base = w.Home
		}
		s = filepath.Join(base, s)
	}
	return []string{s}, true
}

// Read returns the first vimrc there is, or an empty File when the user
// named none and none of Vim's own places has one.
func Read(ctx context.Context, w Where) (File, error) {
	paths, named := candidates(w)
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return File{}, fmt.Errorf("read vimrc: %w", err)
		}
		b, err := os.ReadFile(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return File{}, fmt.Errorf("read vimrc %s: %w", p, err)
		}
		return File{Path: p, Text: string(b)}, nil
	}
	if named {
		return File{}, fmt.Errorf("%s: %w", paths[0], ErrNotFound)
	}
	return File{}, nil
}
