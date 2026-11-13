// Package game reads an ASCDII Engine project: its manifest, its assets by
// kind, which scene objects use each script, and the Lua errors in a script
// or in what the engine logs.
package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrNoProject = errors.New("no .ascdiiproj in the folder")

type Manifest struct {
	Name    string  `json:"name"`
	Version string  `json:"version"`
	Engine  string  `json:"engine"`
	Start   string  `json:"start"`
	Cols    int     `json:"cols,omitempty"`
	Rows    int     `json:"rows,omitempty"`
	FPS     int     `json:"fps,omitempty"`
	Display Display `json:"display"`
}

type Display struct {
	Mode string `json:"mode,omitempty"` // window, or the terminal when empty
}

// Project is a game folder and its manifest.
type Project struct {
	Dir      string   `json:"dir"`
	File     string   `json:"file"` // the .ascdiiproj
	Manifest Manifest `json:"manifest"`
}

// Find reads the manifest at the folder's top, where the engine keeps it.
func Find(ctx context.Context, dir string) (Project, error) {
	if err := ctx.Err(); err != nil {
		return Project{}, err
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.ascdiiproj"))
	if err != nil {
		return Project{}, fmt.Errorf("find manifest: %w", err)
	}
	if len(matches) == 0 {
		return Project{}, ErrNoProject
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		return Project{}, fmt.Errorf("read manifest: %w", err)
	}
	p := Project{Dir: dir, File: matches[0]}
	if err := json.Unmarshal(raw, &p.Manifest); err != nil {
		return Project{}, fmt.Errorf("read %s: %w", filepath.Base(matches[0]), err)
	}
	return p, nil
}
