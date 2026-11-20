// Command mentor-host is the Mentor extension's backend. The window cannot
// read files, so the index of the code, search, symbol cards, the
// architecture and flows all come from here, as does everything that
// watches and gates a coding agent.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"bmeditor-mentor/bmext/host"
	"bmeditor-mentor/internal/agent"
	"bmeditor-mentor/internal/changes"
	"bmeditor-mentor/internal/engine"
	"bmeditor-mentor/internal/explain"
	"bmeditor-mentor/internal/gitx"
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

// state keeps the engine and the agent gate of the folder open now.
// Another folder closes them and opens its own.
type state struct {
	dataDir string

	mu     sync.Mutex
	eng    *engine.Engine
	gate   *agent.Gate
	server *agent.Server
	linked string // "on" or "off": what the settings last said of connecting the agent to this folder
}

func (st *state) close() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.closeFolder()
	if st.server != nil {
		st.server.Close()
		st.server = nil
	}
}

func (st *state) closeFolder() {
	if st.eng != nil {
		if st.server != nil {
			st.server.Unregister(st.eng.Root)
		}
		st.eng.Close()
		st.eng, st.gate = nil, nil
	}
}

func (st *state) gateDir() string { return filepath.Join(st.dataDir, "gate") }

// gateFor is the gate of root, for the server; nil when another folder is
// open.
func (st *state) gateFor(root string) *agent.Gate {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.eng != nil && st.gate != nil && filepath.Clean(root) == st.eng.Root {
		return st.gate
	}
	return nil
}

// policyOf reads the checkpoint settings.
func policyOf(get func(string) string) agent.Policy {
	p := agent.DefaultPolicy()
	p.RequirePlan = get("requirePlan") != "false"
	p.Off = get("watchAgent") == "false"
	if n, err := strconv.Atoi(strings.TrimSpace(get("maxFiles"))); err == nil && n >= 0 {
		p.MaxFiles = n
	}
	return p
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

// engineFor is the engine of root, opened with its gate when it is not
// yet, with the settings of the call.
func (st *state) engineFor(s *host.Server, r string) (*engine.Engine, error) {
	set := engine.SettingsOf(s.Context().Setting)
	policy := policyOf(s.Context().Setting)
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.eng != nil && st.eng.Root == r {
		st.eng.Apply(set)
		st.gate.SetPolicy(policy)
		st.link(s, st.eng)
		return st.eng, nil
	}
	st.closeFolder()
	var g *agent.Gate
	notify := func(method string, params any) {
		_ = s.Notify(method, params)
		// A checkpoint follows the code as each read of it lands.
		if es, ok := params.(engine.Status); ok && es.State == "ready" && g != nil {
			go g.Refresh(context.Background())
		}
	}
	e, err := engine.Open(r, st.dataDir, set, notify)
	if err != nil {
		return nil, &host.Error{Code: "STORE", Message: err.Error()}
	}
	g = agent.New(r, e, policy, func(as agent.State) { _ = s.Notify("mentor/agent", as) })
	if b := e.DB.Meta(context.Background(), "gate"); b != "" {
		_ = g.Unmarshal([]byte(b))
	}
	g.Persist = func(b []byte) { _ = e.DB.SetMeta(context.Background(), "gate", string(b)) }
	g.SetConnection(agent.HooksInstalled(r), e.DB.Meta(context.Background(), "mcp") == "1", "")
	if st.server == nil {
		if srv, err := agent.Listen(st.gateDir(), st.gateFor); err == nil {
			st.server = srv
		}
	}
	if st.server != nil {
		_ = st.server.Register(r)
	}
	st.eng, st.gate, st.linked = e, g, ""
	st.link(s, e)
	return e, nil
}

// link connects the agent to the folder or takes it out, as the settings
// say: connected while Mentor pauses the agent and Connect the agent is
// on. It acts when the folder opens and when the settings change; a setup
// already as the settings want is left alone. Call it with st.mu held.
func (st *state) link(s *host.Server, e *engine.Engine) {
	want := "off"
	if s.Context().Setting("watchAgent") != "false" && s.Context().Setting("autoConnect") != "false" {
		want = "on"
	}
	if want == st.linked {
		return
	}
	first := st.linked == ""
	st.linked = want
	switch linkAction(want == "on", first, agent.HooksInstalled(e.Root), e.DB.Meta(context.Background(), "mcp") == "1") {
	case "connect":
		go func() { _ = st.connect(context.Background(), e) }()
	case "disconnect":
		go st.disconnect(context.Background(), e)
	}
}

// linkAction is what link does: when the folder opens (first), only what
// is not yet as the settings want; when the settings change, always.
func linkAction(want, first, hooks, mcp bool) string {
	switch {
	case want && (!first || !hooks || !mcp):
		return "connect"
	case !want && (!first || hooks || mcp):
		return "disconnect"
	}
	return ""
}

// gateOf is the gate of the folder the call is for.
func (st *state) gateOf(e *engine.Engine) *agent.Gate {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.eng == e {
		return st.gate
	}
	return nil
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

	// agent.changes is what the agent changed that still differs from
	// HEAD, file by file: the files its edits touched, each with the
	// declarations changed in it and which the developer has reviewed.
	s.Handle("agent.changes", indexed(s, st, func(ctx context.Context, e *engine.Engine, ix *index.Index, _ none) (any, error) {
		base, files, err := changes.FromHead(ctx, ix)
		if err != nil {
			return nil, &host.Error{Code: "NO_GIT", Message: err.Error()}
		}
		rel := func(path string) string {
			r, err := filepath.Rel(e.Root, path)
			if err != nil {
				return path
			}
			return filepath.ToSlash(r)
		}
		differs := map[string]bool{}
		for _, f := range files {
			differs[rel(f.Path)] = true
		}
		touched := map[string]bool{}
		if g := st.gateOf(e); g != nil {
			for _, f := range g.Touched(func(f string) bool { return differs[f] }) {
				touched[f] = true
			}
		}
		out := agentChanges{Head: base.Head, Files: changes.ByFile(changes.Diff(ix, base), files, func(path string) bool { return touched[rel(path)] })}
		var keys []string
		for _, f := range out.Files {
			out.Lines[0] += f.Added
			out.Lines[1] += f.Deleted
			for _, c := range f.Changes {
				keys = append(keys, c.Key)
			}
		}
		states := e.Review.States(ix, keys)
		for i := range out.Files {
			for j := range out.Files[i].Changes {
				if c := &out.Files[i].Changes[j]; c.Change != "removed" {
					c.Review = states[c.Key]
				}
			}
		}
		if e.Status().State == "stale" {
			out.Warning = "Files changed since the index was read; this updates when they are read again."
		}
		return out, nil
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

	registerAgent(s, st)

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

// agentChanges is the answer of agent.changes.
type agentChanges struct {
	Head    gitx.Head             `json:"head"`
	Files   []changes.FileChanges `json:"files"`
	Lines   [2]int                `json:"lines"`
	Warning string                `json:"warning,omitempty"`
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

// registerAgent adds the methods the checkpoint card calls: the
// developer's side of the gate.
func registerAgent(s *host.Server, st *state) {
	gated := func(fn func(ctx context.Context, e *engine.Engine, g *agent.Gate, a answer) error) host.Handler {
		return handler(s, st, func(ctx context.Context, e *engine.Engine, a answer) (any, error) {
			g := st.gateOf(e)
			if g == nil {
				return nil, &host.Error{Code: "NO_GATE", Message: "The folder changed; try again."}
			}
			if err := fn(ctx, e, g, a); err != nil {
				return nil, &host.Error{Code: "AGENT", Message: err.Error()}
			}
			return g.State(), nil
		})
	}
	s.Handle("agent.state", gated(func(context.Context, *engine.Engine, *agent.Gate, answer) error { return nil }))
	s.Handle("agent.plan", gated(func(ctx context.Context, _ *engine.Engine, g *agent.Gate, a answer) error {
		return g.AnswerPlan(ctx, a.Approve, a.Note)
	}))
	s.Handle("agent.checkpoint", gated(func(ctx context.Context, e *engine.Engine, g *agent.Gate, a answer) error {
		keys, err := g.AnswerCheckpoint(ctx, a.Answer, a.Note)
		if err == nil && len(keys) > 0 {
			if ix := e.Peek(); ix != nil {
				_ = e.Review.Mark(ctx, ix, keys, "checkpoint approved")
			}
		}
		return err
	}))
	s.Handle("agent.end", gated(func(_ context.Context, _ *engine.Engine, g *agent.Gate, _ answer) error {
		g.EndPlan()
		return nil
	}))
}

// connect sets the agent up for the folder: Mentor's MCP server and the
// hooks that make a checkpoint hold.
func (st *state) connect(ctx context.Context, e *engine.Engine) error {
	g := st.gateOf(e)
	if g == nil {
		return errors.New("the folder changed")
	}
	gate, err := agent.InstallGate(st.gateDir())
	if err != nil {
		g.SetConnection(false, false, "Mentor could not connect the agent: "+err.Error())
		return err
	}
	res := agent.Connect(ctx, e.Root, gate)
	if res.MCP {
		_ = e.DB.SetMeta(ctx, "mcp", "1")
	}
	problem := res.Detail
	if !res.MCP && res.MCPCommand != "" {
		problem += "\n" + res.MCPCommand
	}
	g.SetConnection(res.Hooks, res.MCP || e.DB.Meta(ctx, "mcp") == "1", strings.TrimSpace(problem))
	return nil
}

// disconnect takes Mentor's MCP server and hooks out of the agent for the
// folder.
func (st *state) disconnect(ctx context.Context, e *engine.Engine) {
	err := agent.Disconnect(ctx, e.Root)
	_ = e.DB.SetMeta(ctx, "mcp", "")
	if g := st.gateOf(e); g != nil {
		problem := ""
		if err != nil {
			problem = "Mentor could not take its hooks out of " + e.Root + ": " + err.Error()
		}
		g.SetConnection(false, false, problem)
	}
}

// answer is the developer's answer from the checkpoint card.
type answer struct {
	Approve bool   `json:"approve"`
	Answer  string `json:"answer"` // approve, changes or skip
	Note    string `json:"note"`
}
