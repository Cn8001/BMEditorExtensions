// Command ascdii-host is the ASCDII extension's backend: it reads the open
// game project, checks its Lua, and runs the engine's ascdii program to
// play the game, open it in the engine's editor, or build it.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"bmeditor-ascdii/bmext/host"
	"bmeditor-ascdii/internal/engine"
	"bmeditor-ascdii/internal/game"
)

type backend struct {
	s     *host.Server
	game  engine.Runner
	build engine.Runner
}

func (b *backend) dir() string {
	c := b.s.Context()
	if c.Project.Dir != "" {
		return c.Project.Dir
	}
	return c.Folder
}

type status struct {
	Project *game.Project `json:"project"`
	Engine  string        `json:"engine"`
	Version string        `json:"version,omitempty"`
	Missing string        `json:"missing,omitempty"` // why the engine was not found
	Running string        `json:"running"`
}

func (b *backend) status(ctx context.Context) status {
	st := status{Running: b.game.Running()}
	if p, err := game.Find(ctx, b.dir()); err == nil {
		st.Project = &p
	}
	bin, err := engine.Locate(b.s.Context().Setting("engine"))
	if err != nil {
		st.Missing = err.Error()
		return st
	}
	st.Engine = bin
	vctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	st.Version, _ = engine.Version(vctx, bin)
	return st
}

func (b *backend) project(ctx context.Context) (game.Project, string, error) {
	p, err := game.Find(ctx, b.dir())
	if err != nil {
		return p, "", &host.Error{Code: "no-project", Message: err.Error()}
	}
	bin, err := engine.Locate(b.s.Context().Setting("engine"))
	if err != nil {
		return p, "", &host.Error{Code: "no-engine", Message: err.Error()}
	}
	return p, bin, nil
}

// start runs ascdii for what, passing its output to the window and its
// script errors to Problems.
func (b *backend) start(r *engine.Runner, what string, p game.Project, bin string, args []string) error {
	line := func(l engine.Line) {
		_ = b.s.Notify("ascdii/output", l)
		if pr, ok := game.LogProblem(l.Text); ok {
			pr.File = filepath.Join(p.Dir, filepath.FromSlash(pr.File))
			_ = b.s.Notify("ascdii/problem", pr)
		}
	}
	done := func(err error) {
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		_ = b.s.Notify("ascdii/exit", map[string]string{"what": what, "error": msg})
	}
	if err := r.Start(bin, p.Dir, what, args, line, done); err != nil {
		if errors.Is(err, engine.ErrRunning) {
			return &host.Error{Code: "running", Message: err.Error()}
		}
		return err
	}
	_ = b.s.Notify("ascdii/started", map[string]string{"what": what})
	return nil
}

// watch tells the window when the project's assets change, so its views
// follow what the engine's editor or a tool writes too.
func (b *backend) watch(ctx context.Context) {
	last := ""
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		p, err := game.Find(ctx, b.dir())
		if err != nil {
			last = ""
			continue
		}
		sig := signature(p.Dir)
		if last != "" && sig != last {
			_ = b.s.Notify("ascdii/changed", nil)
		}
		last = sig
	}
}

// signature sums the names, sizes and times of the project's assets.
func signature(dir string) string {
	var sb strings.Builder
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && game.SkipDir(rel, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if _, ok := game.Classify(rel); !ok {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			sb.WriteString(rel + "|" + strconv.FormatInt(fi.Size(), 10) + "|" + strconv.FormatInt(fi.ModTime().UnixNano(), 10) + "\n")
		}
		return nil
	})
	return sb.String()
}

func main() {
	s := host.New()
	b := &backend{s: s}
	s.Handle("status", func(ctx context.Context, _ json.RawMessage) (any, error) { return b.status(ctx), nil })
	s.Handle("index", func(ctx context.Context, _ json.RawMessage) (any, error) {
		p, err := game.Find(ctx, b.dir())
		if err != nil {
			return nil, &host.Error{Code: "no-project", Message: err.Error()}
		}
		return game.Read(ctx, p.Dir)
	})
	s.Handle("check", func(_ context.Context, raw json.RawMessage) (any, error) {
		var p struct {
			File string `json:"file"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return game.Check(p.File, p.Text), nil
	})
	s.Handle("play", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var o struct {
			Target string `json:"target"` // window or editor
		}
		_ = json.Unmarshal(raw, &o)
		p, bin, err := b.project(ctx)
		if err != nil {
			return nil, err
		}
		if o.Target == "editor" {
			return nil, b.start(&b.game, "ASCDII Editor", p, bin, []string{"run", p.Dir})
		}
		return nil, b.start(&b.game, p.Manifest.Name, p, bin, []string{"play", p.Dir, "--window"})
	})
	s.Handle("stop", func(context.Context, json.RawMessage) (any, error) {
		b.game.Stop()
		return nil, nil
	})
	s.Handle("build", func(ctx context.Context, _ json.RawMessage) (any, error) {
		p, bin, err := b.project(ctx)
		if err != nil {
			return nil, err
		}
		return nil, b.start(&b.build, "build", p, bin, []string{"build", p.Dir})
	})
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); b.watch(ctx) }()
	err := s.Serve(ctx, os.Stdin, os.Stdout)
	// The editor closed us: the game goes too.
	b.game.Stop()
	b.build.Stop()
	cancel()
	wg.Wait()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
