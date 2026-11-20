// Command mentor-host is the Mentor extension's backend. The window cannot
// read files, so the index of the code, search, symbol cards, the
// architecture and flows all come from here, as does everything that
// watches and gates a coding agent.
package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"

	"bmeditor-mentor/bmext/host"
	"bmeditor-mentor/internal/changes"
	"bmeditor-mentor/internal/engine"
	"bmeditor-mentor/internal/explain"
	"bmeditor-mentor/internal/index"
	"bmeditor-mentor/internal/search"
)

func main() {
	s := host.New()
	st := &state{dataDir: engine.DataDir()}
	register(s, st)
	host.Run(s)
	st.close()
}

// state keeps the engine of the folder open now. Another folder closes it
// and opens its own.
type state struct {
	dataDir string

	mu  sync.Mutex
	eng *engine.Engine
}

func (st *state) close() {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.eng != nil {
		st.eng.Close()
		st.eng = nil
	}
}

// root is the folder the extension works in: the open folder, else the
// project's.
func root(s *host.Server) (string, error) {
	c := s.Context()
	r := c.Folder
	if r == "" {
		r = c.Project.Dir
	}
	if r == "" {
		return "", &host.Error{Code: "NO_FOLDER", Message: "Open a folder first."}
	}
	return filepath.Clean(r), nil
}

// engineFor is the engine of root, opened when it is not yet, with the
// settings of the call.
func (st *state) engineFor(s *host.Server, r string) (*engine.Engine, error) {
	set := engine.SettingsOf(s.Context().Setting)
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.eng != nil && st.eng.Root == r {
		st.eng.Apply(set)
		return st.eng, nil
	}
	if st.eng != nil {
		st.eng.Close()
		st.eng = nil
	}
	e, err := engine.Open(r, st.dataDir, set, func(method string, params any) { _ = s.Notify(method, params) })
	if err != nil {
		return nil, &host.Error{Code: "STORE", Message: err.Error()}
	}
	st.eng = e
	return e, nil
}

func params[T any](raw json.RawMessage) (T, error) {
	var v T
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &v); err != nil {
			return v, &host.Error{Code: "BAD_PARAMS", Message: err.Error()}
		}
	}
	return v, nil
}

// handler wraps a method that needs the folder's engine.
func handler[T any](s *host.Server, st *state, fn func(ctx context.Context, e *engine.Engine, p T) (any, error)) host.Handler {
	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		r, err := root(s)
		if err != nil {
			return nil, err
		}
		p, err := params[T](raw)
		if err != nil {
			return nil, err
		}
		e, err := st.engineFor(s, r)
		if err != nil {
			return nil, err
		}
		return fn(ctx, e, p)
	}
}

// indexed wraps a method that needs the index, waiting for the first.
func indexed[T any](s *host.Server, st *state, fn func(ctx context.Context, e *engine.Engine, ix *index.Index, p T) (any, error)) host.Handler {
	return handler(s, st, func(ctx context.Context, e *engine.Engine, p T) (any, error) {
		ix, err := e.Index(ctx)
		if err != nil {
			return nil, &host.Error{Code: "NO_INDEX", Message: err.Error()}
		}
		return fn(ctx, e, ix, p)
	})
}

type none struct{}

type position struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

func notFound(what string) error {
	return &host.Error{Code: "NOT_FOUND", Message: what}
}

func register(s *host.Server, st *state) {
	s.Handle("status", handler(s, st, func(_ context.Context, e *engine.Engine, _ none) (any, error) {
		return e.Status(), nil
	}))

	s.Handle("reindex", handler(s, st, func(_ context.Context, e *engine.Engine, _ none) (any, error) {
		e.Reindex()
		return e.Status(), nil
	}))

	s.Handle("search", handler(s, st, func(ctx context.Context, e *engine.Engine, p struct {
		Q     string   `json:"q"`
		Kinds []string `json:"kinds"`
		Limit int      `json:"limit"`
	}) (any, error) {
		res, err := e.Search(ctx, p.Q, search.Options{Kinds: p.Kinds, Limit: p.Limit})
		if err != nil {
			return nil, &host.Error{Code: "SEARCH", Message: err.Error()}
		}
		return res, nil
	}))

	// at is the symbol declared at a place: what Explain This explains.
	s.Handle("at", indexed(s, st, func(_ context.Context, _ *engine.Engine, ix *index.Index, p position) (any, error) {
		sym := ix.At(filepath.Clean(p.File), p.Line)
		if sym == nil {
			return nil, notFound("No declaration here the index knows. Put the cursor in a type, function or method.")
		}
		return explain.RefOf(sym), nil
	}))

	s.Handle("card", indexed(s, st, func(_ context.Context, _ *engine.Engine, ix *index.Index, p struct {
		Key string `json:"key"`
	}) (any, error) {
		c := explain.CardOf(ix, p.Key)
		if c == nil {
			return nil, notFound("The index has no " + p.Key + " now. It may have been renamed or removed.")
		}
		return c, nil
	}))

	s.Handle("arch", indexed(s, st, func(_ context.Context, _ *engine.Engine, ix *index.Index, p struct {
		Focus string `json:"focus"`
	}) (any, error) {
		if p.Focus != "" {
			return explain.Package(ix, p.Focus), nil
		}
		return explain.Packages(ix), nil
	}))

	s.Handle("flow", indexed(s, st, func(_ context.Context, _ *engine.Engine, ix *index.Index, p struct {
		Key   string `json:"key"`
		Depth int    `json:"depth"`
	}) (any, error) {
		f := explain.Trace(ix, p.Key, p.Depth)
		if f == nil {
			return nil, notFound("The index has no " + p.Key + " now.")
		}
		return f, nil
	}))

	s.Handle("entries", indexed(s, st, func(_ context.Context, _ *engine.Engine, ix *index.Index, _ none) (any, error) {
		return explain.Entries(ix), nil
	}))

	// changes is what changed since HEAD, in units, with which of the
	// declarations the developer has reviewed as they are now.
	s.Handle("changes", indexed(s, st, func(ctx context.Context, e *engine.Engine, ix *index.Index, _ none) (any, error) {
		base, files, err := changes.FromHead(ctx, ix)
		if err != nil {
			return nil, &host.Error{Code: "NO_GIT", Message: err.Error()}
		}
		rep := changes.Build(ix, base, changes.Diff(ix, base), files)
		markReviews(e, ix, rep)
		if e.Status().State == "stale" {
			rep.Warning = "Files changed since the index was read; this updates when they are read again."
		}
		return rep, nil
	}))

	s.Handle("review.mark", indexed(s, st, func(ctx context.Context, e *engine.Engine, ix *index.Index, p struct {
		Keys []string `json:"keys"`
		Note string   `json:"note"`
	}) (any, error) {
		if err := e.Review.Mark(ctx, ix, p.Keys, p.Note); err != nil {
			return nil, &host.Error{Code: "STORE", Message: err.Error()}
		}
		return e.Review.States(ix, p.Keys), nil
	}))

	s.Handle("review.unmark", indexed(s, st, func(ctx context.Context, e *engine.Engine, ix *index.Index, p struct {
		Keys []string `json:"keys"`
	}) (any, error) {
		if err := e.Review.Unmark(ctx, p.Keys); err != nil {
			return nil, &host.Error{Code: "STORE", Message: err.Error()}
		}
		return e.Review.States(ix, p.Keys), nil
	}))

	// stale are the declarations reviewed once and changed since.
	s.Handle("review.stale", indexed(s, st, func(_ context.Context, e *engine.Engine, ix *index.Index, _ none) (any, error) {
		out := []explain.Ref{}
		for _, k := range e.Review.StaleKeys(ix) {
			if sym := ix.Symbols[k]; sym != nil {
				out = append(out, explain.RefOf(sym))
			}
		}
		return out, nil
	}))

	// lens is what the editor shows above each declaration of a file.
	s.Handle("lens", handler(s, st, func(_ context.Context, e *engine.Engine, p position) (any, error) {
		ix := e.Peek()
		out := []lensItem{}
		if ix == nil {
			return out, nil
		}
		for _, sym := range ix.ByFile[filepath.Clean(p.File)] {
			if sym.Kind == index.KindField || sym.Kind == index.KindConst || sym.Kind == index.KindVar {
				continue
			}
			out = append(out, lensOf(ix, sym))
		}
		return out, nil
	}))
}

type lensItem struct {
	Key   string `json:"key"`
	Line  int    `json:"line"`
	Start int    `json:"start"`
	Uses  int    `json:"uses"`
	Tests int    `json:"tests"`
	Impls int    `json:"impls"`
}

func lensOf(ix *index.Index, s *index.Symbol) lensItem {
	users := map[string]bool{}
	tests := map[string]bool{}
	impls := 0
	for _, r := range ix.In[s.Key] {
		switch r.Kind {
		case index.RelImplements:
			impls++
			continue
		case index.RelMethodOf, index.RelFieldOf, index.RelEmbeds:
			continue
		}
		if u := ix.Symbols[r.From]; u != nil {
			if u.Test {
				tests[u.Key] = true
			} else {
				users[u.Key] = true
			}
		}
	}
	return lensItem{Key: s.Key, Line: s.Line, Start: s.Start, Uses: len(users), Tests: len(tests), Impls: impls}
}

// markReviews fills in each change's review state. A removed declaration
// has nothing left to review.
func markReviews(e *engine.Engine, ix *index.Index, rep *changes.Report) {
	var keys []string
	for _, u := range rep.Units {
		for _, c := range u.Changes {
			keys = append(keys, c.Key)
		}
	}
	states := e.Review.States(ix, keys)
	for i := range rep.Units {
		for j := range rep.Units[i].Changes {
			c := &rep.Units[i].Changes[j]
			if c.Change == "removed" {
				continue
			}
			c.Review = states[c.Key]
		}
	}
}
