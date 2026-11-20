package intel

import (
	"fmt"
	"sort"

	"bmeditor-mentor/internal/explain"
	"bmeditor-mentor/internal/index"
)

// Feature is a part of the project, such as authentication, followed end
// to end through the code: where a request comes in, what runs, in order,
// where it reads and writes its data, how it fails, the tests that reach
// it and where to stop in the debugger. Every item is a place in the code.
type Feature struct {
	// Entry is where it starts: a route, or a main.
	Entry  *Entry     `json:"entry,omitempty"`
	Others []Entry    `json:"others"` // other ways in
	Flow   []FlowStep `json:"flow"`   // what runs from Entry, in the order of the code
	// Data are the steps that read or write what the feature keeps.
	Data []FlowStep `json:"data"`
	// Errors are the steps that can fail, and how often they return an error.
	Errors      []FlowStep    `json:"errors"`
	Tests       []explain.Ref `json:"tests"`
	Breakpoints []explain.Ref `json:"breakpoints"`
	Note        string        `json:"note,omitempty"`
}

// Entry is a way into a feature.
type Entry struct {
	Kind  string      `json:"kind"` // route or main
	Label string      `json:"label"`
	Ref   explain.Ref `json:"ref"` // the handler or main
	File  string      `json:"file"`
	Line  int         `json:"line"`
}

// FlowStep is one function on the feature's path.
type FlowStep struct {
	explain.Ref
	Depth      int    `json:"depth"`
	Layer      string `json:"layer,omitempty"`
	ErrReturns int    `json:"errReturns,omitempty"`
	Branches   int    `json:"branches,omitempty"`
	Dynamic    bool   `json:"dynamic,omitempty"` // reached through an interface: one of several may run
	Seed       bool   `json:"seed,omitempty"`    // one the question named
}

// FeatureOf follows the feature the candidates are part of: from the routes
// and mains that reach them down through what runs. Nil when nothing
// reaches them from a way in.
func FeatureOf(ix *index.Index, candidates []string) *Feature {
	seeds := seedsOf(ix, candidates)
	if len(seeds) == 0 {
		return nil
	}
	// Up from the seeds, to the functions that reach them.
	reach := map[string]int{} // key → distance from a seed
	queue := []string{}
	for k := range seeds {
		reach[k] = 0
		queue = append(queue, k)
	}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		if reach[k] >= 5 {
			continue
		}
		for _, r := range ix.In[k] {
			if r.Kind != index.RelCalls && r.Kind != index.RelCallsDynamic {
				continue
			}
			u := ix.Symbols[r.From]
			if u == nil || u.Test {
				continue
			}
			if _, ok := reach[u.Key]; !ok {
				reach[u.Key] = reach[k] + 1
				queue = append(queue, u.Key)
			}
		}
	}
	var entries []Entry
	for _, rt := range ix.Routes {
		if d, ok := reach[rt.Handler]; ok && rt.Handler != "" {
			h := ix.Symbols[rt.Handler]
			entries = append(entries, Entry{Kind: "route", Label: label(rt), Ref: explain.RefOf(h), File: rt.File, Line: rt.Line})
			_ = d
		}
	}
	for k := range reach {
		if s := ix.Symbols[k]; s != nil && s.Name == "main" && s.PkgName == "main" {
			entries = append(entries, Entry{Kind: "main", Label: s.Pkg, Ref: explain.RefOf(s), File: s.File, Line: s.Line})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if (entries[i].Kind == "route") != (entries[j].Kind == "route") {
			return entries[i].Kind == "route"
		}
		return reach[entries[i].Ref.Key] < reach[entries[j].Ref.Key]
	})
	f := &Feature{Others: []Entry{}, Flow: []FlowStep{}, Data: []FlowStep{}, Errors: []FlowStep{}, Tests: []explain.Ref{}, Breakpoints: []explain.Ref{}}
	start := ""
	if len(entries) > 0 {
		f.Entry = &entries[0]
		f.Others = append(f.Others, entries[1:]...)
		start = entries[0].Ref.Key
	} else {
		// Nothing reaches it from a way in: follow it from the highest
		// function above it.
		best := -1
		for k, d := range reach {
			if s := ix.Symbols[k]; s != nil && (s.Kind == index.KindFunc || s.Kind == index.KindMethod) && (d > best || d == best && k < start) {
				best, start = d, k
			}
		}
		if start == "" {
			return nil
		}
		f.Note = "No route or main reaches it; it is followed from the highest function that calls it."
	}
	fl := explain.Trace(ix, start, 5)
	if fl == nil {
		return nil
	}
	seen := map[string]bool{}
	var walk func(s *explain.Step, depth int)
	walk = func(s *explain.Step, depth int) {
		if len(f.Flow) >= 40 || s.Cycle {
			return
		}
		sym := ix.Symbols[s.Key]
		layer := ""
		if sym != nil {
			layer = explain.LayerOf(sym.Pkg, sym.PkgName)
			if dataName(sym) {
				layer = "Repository"
			}
		}
		st := FlowStep{Ref: s.Ref, Depth: depth, Layer: layer, ErrReturns: s.ErrReturns, Branches: s.Branches,
			Dynamic: s.Certainty == index.Dynamic, Seed: seeds[s.Key]}
		f.Flow = append(f.Flow, st)
		if !seen[s.Key] {
			seen[s.Key] = true
			if layer == "Repository" {
				f.Data = append(f.Data, st)
			}
			if s.ErrReturns > 0 {
				f.Errors = append(f.Errors, st)
			}
		}
		for _, c := range s.Children {
			walk(c, depth+1)
		}
	}
	walk(fl.Root, 0)

	// Where to stop: the way in, the first step that can fail, and the
	// steps the question named.
	add := func(r explain.Ref, why string) {
		for _, b := range f.Breakpoints {
			if b.Key == r.Key {
				return
			}
		}
		r.Via = why
		f.Breakpoints = append(f.Breakpoints, r)
	}
	if len(f.Flow) > 0 {
		add(f.Flow[0].Ref, "where it starts")
	}
	if len(f.Errors) > 0 {
		add(f.Errors[0].Ref, fmt.Sprintf("the first step that can fail (%d ways)", f.Errors[0].ErrReturns))
	}
	for _, s := range f.Flow {
		if s.Seed && len(f.Breakpoints) < 5 {
			add(s.Ref, "the part you asked about")
		}
	}
	// The tests that reach it: of the seeds and of the steps on the path.
	tests := map[string]bool{}
	for _, s := range f.Flow {
		for _, r := range ix.In[s.Key] {
			t := ix.Symbols[r.From]
			if t != nil && t.Test && t.Kind == index.KindFunc && !tests[t.Key] && len(f.Tests) < 8 {
				tests[t.Key] = true
				ref := explain.RefOf(t)
				ref.Via, ref.SiteFile, ref.SiteLine = "reaches "+s.Name, r.File, r.Line
				f.Tests = append(f.Tests, ref)
			}
		}
	}
	return f
}

// seedsOf are the functions the candidates stand for: functions and
// methods themselves, and for a type the functions that use it most.
func seedsOf(ix *index.Index, candidates []string) map[string]bool {
	seeds := map[string]bool{}
	for i, k := range candidates {
		if i >= 6 {
			break
		}
		s := ix.Symbols[k]
		if s == nil || s.Test {
			continue
		}
		switch s.Kind {
		case index.KindFunc, index.KindMethod:
			if s.Recv == "" || !isInterface(ix, s) {
				seeds[k] = true
				continue
			}
			fallthrough
		default:
			// A type, or an interface's method: who calls or builds it.
			for _, r := range ix.In[k] {
				if u := ix.Symbols[r.From]; u != nil && !u.Test && (u.Kind == index.KindFunc || u.Kind == index.KindMethod) &&
					(r.Kind == index.RelCalls || r.Kind == index.RelCallsDynamic || r.Kind == index.RelConstructs || r.Kind == index.RelReferences) {
					seeds[u.Key] = true
				}
			}
		}
	}
	return seeds
}

func isInterface(ix *index.Index, m *index.Symbol) bool {
	o := owner(ix, m)
	return o != nil && o.Kind == index.KindInterface
}

// dataName says whether a method is on a type named for keeping data: a
// Repository, a Store, a DB.
func dataName(s *index.Symbol) bool {
	if s.Recv == "" {
		return false
	}
	for _, w := range index.Split(s.Recv) {
		switch w {
		case "repo", "repository", "store", "storage", "db", "database", "dao", "cache":
			return true
		}
	}
	return false
}

func label(rt index.Route) string {
	if rt.Method != "" {
		return rt.Method + " " + rt.Path
	}
	return rt.Path
}
