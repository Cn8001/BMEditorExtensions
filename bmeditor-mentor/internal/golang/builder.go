package golang

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bmeditor-mentor/internal/index"
)

// Builder is the Go source of the index. It keeps the packages read so
// far, and reads again only those whose folders changed and the packages
// that import them.
type Builder struct {
	Root string
	Opt  Options

	mods   []index.Module
	pkgs   map[string]*index.Package
	stamps map[string]time.Time // folder → stamp when last read
	errs   []string
}

var _ index.Source = (*Builder)(nil)

func (b *Builder) Language() string                    { return "go" }
func (b *Builder) Packages() map[string]*index.Package { return b.pkgs }
func (b *Builder) Modules() []index.Module             { return b.mods }
func (b *Builder) Errors() []string                    { return b.errs }

// Full reads every module under the root.
func (b *Builder) Full(ctx context.Context) error {
	b.mods = Modules(b.Root)
	b.pkgs = map[string]*index.Package{}
	b.errs = nil
	b.stamps = b.Dirs()
	var firstErr error
	for _, m := range b.mods {
		pkgs, err := Load(ctx, m, b.mods, []string{"./..."}, b.Root, b.Opt)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, p := range pkgs {
			b.pkgs[p.Path] = p
		}
	}
	if firstErr != nil {
		b.errs = append(b.errs, firstErr.Error())
	}
	return nil
}

// Dirs are the folders holding .go files, with their stamps.
func (b *Builder) Dirs() map[string]time.Time {
	out := map[string]time.Time{}
	for _, m := range b.mods {
		_ = filepath.WalkDir(m.Dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			name := d.Name()
			if path != m.Dir && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "vendor" || name == "testdata" || name == "node_modules") {
				return filepath.SkipDir
			}
			if path != m.Dir {
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return filepath.SkipDir // its own module, walked on its own
				}
			}
			if t := Stamp(path); !t.IsZero() {
				out[path] = t
			}
			return nil
		})
	}
	return out
}

// Changed are the folders whose .go files changed, appeared or went
// since they were read, and whether a go.mod changed, which needs a full
// read.
func (b *Builder) Changed() (dirs []string, full bool) {
	if b.stamps == nil {
		return nil, true
	}
	if len(Modules(b.Root)) != len(b.mods) {
		return nil, true
	}
	now := b.Dirs()
	for d, t := range now {
		if old, ok := b.stamps[d]; !ok || !old.Equal(t) {
			dirs = append(dirs, d)
		}
	}
	for d := range b.stamps {
		if _, ok := now[d]; !ok {
			dirs = append(dirs, d)
		}
	}
	sort.Strings(dirs)
	return dirs, false
}

// Update reads dirs again, with the packages that import theirs, since
// their calls into them may have changed.
func (b *Builder) Update(ctx context.Context, dirs []string) error {
	if b.pkgs == nil {
		return b.Full(ctx)
	}
	want := map[string]bool{}
	changedPaths := map[string]bool{}
	for _, d := range dirs {
		want[d] = true
		for _, p := range b.pkgs {
			if p.Dir == d {
				changedPaths[strings.TrimSuffix(p.Path, "_test")] = true
			}
		}
	}
	for _, p := range b.pkgs {
		for _, imp := range p.Imports {
			if changedPaths[imp] {
				want[p.Dir] = true
			}
		}
	}
	stamps := map[string]time.Time{}
	for d := range want {
		stamps[d] = Stamp(d)
	}
	for _, m := range b.mods {
		var patterns []string
		for d := range want {
			if owner(b.mods, d).Dir != m.Dir {
				continue
			}
			rel, err := filepath.Rel(m.Dir, d)
			if err != nil {
				continue
			}
			if _, err := os.Stat(d); err != nil {
				continue // the folder is gone: its packages go below
			}
			patterns = append(patterns, "./"+filepath.ToSlash(rel))
		}
		for path, p := range b.pkgs {
			if want[p.Dir] && owner(b.mods, p.Dir).Dir == m.Dir {
				delete(b.pkgs, path)
			}
		}
		if len(patterns) == 0 {
			continue
		}
		sort.Strings(patterns)
		pkgs, err := Load(ctx, m, b.mods, patterns, b.Root, b.Opt)
		if err != nil {
			return err
		}
		for _, p := range pkgs {
			b.pkgs[p.Path] = p
		}
	}
	for d, t := range stamps {
		if t.IsZero() {
			delete(b.stamps, d)
		} else {
			b.stamps[d] = t
		}
	}
	return nil
}

// owner is the innermost module holding dir.
func owner(mods []index.Module, dir string) index.Module {
	var best index.Module
	for _, m := range mods {
		if (dir == m.Dir || strings.HasPrefix(dir, m.Dir+string(filepath.Separator))) && len(m.Dir) > len(best.Dir) {
			best = m
		}
	}
	return best
}
