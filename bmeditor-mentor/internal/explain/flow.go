package explain

import (
	"sort"
	"strings"

	"bmeditor-mentor/internal/index"
)

// Step is one function on the path a call takes.
type Step struct {
	Ref
	ReturnsError bool    `json:"returnsError,omitempty"`
	ErrReturns   int     `json:"errReturns,omitempty"`
	Branches     int     `json:"branches,omitempty"`
	Doc          string  `json:"doc,omitempty"`
	Cycle        bool    `json:"cycle,omitempty"`     // it is already on the path above
	Truncated    bool    `json:"truncated,omitempty"` // the trace stopped here: depth or size
	Choices      int     `json:"choices,omitempty"`   // an interface call: how many implementations it may run
	Children     []*Step `json:"children"`
}

// Flow is the trace from an entry point.
type Flow struct {
	Root  *Step  `json:"root"`
	Steps int    `json:"steps"`
	Note  string `json:"note"`
}

// Trace follows calls from key, depth levels down, in the order they
// appear in each function. A call through an interface is followed into
// each implementation the index knows, marked dynamic: which one runs is
// decided at run time.
func Trace(ix *index.Index, key string, depth int) *Flow {
	s := ix.Symbols[key]
	if s == nil {
		return nil
	}
	if depth <= 0 {
		depth = 4
	}
	t := &tracer{ix: ix, limit: 250, depth: depth}
	root := t.step(s, index.Relation{Certainty: index.Verified}, 0, map[string]bool{})
	return &Flow{Root: root, Steps: t.n,
		Note: "Calls the compiler checked. Dynamic steps are calls through an interface: any implementation listed may run. Calls through function values, reflection and goroutines started elsewhere are not followed."}
}

type tracer struct {
	ix    *index.Index
	n     int
	limit int
	depth int
}

func (t *tracer) step(s *index.Symbol, via index.Relation, level int, path map[string]bool) *Step {
	t.n++
	st := &Step{Ref: refVia(s, via), ReturnsError: s.ReturnsError, ErrReturns: s.ErrReturns, Branches: s.Branches,
		Doc: index.FirstSentence(s.Doc), Children: []*Step{}}
	if via.Kind == "" {
		st.Via = ""
	}
	if path[s.Key] {
		st.Cycle = true
		return st
	}
	if level >= t.depth || t.n >= t.limit {
		st.Truncated = len(t.calls(s)) > 0
		return st
	}
	path[s.Key] = true
	defer delete(path, s.Key)
	for _, r := range t.calls(s) {
		if t.n >= t.limit {
			st.Truncated = true
			break
		}
		callee := t.ix.Symbols[r.To]
		if callee == nil {
			continue
		}
		child := t.step(callee, r, level+1, path)
		if r.Kind == index.RelCallsDynamic && callee.Recv != "" {
			impls := t.implementations(callee, s.Test)
			child.Choices = len(impls)
			for _, im := range impls {
				if t.n >= t.limit {
					child.Truncated = true
					break
				}
				ir := index.Relation{From: callee.Key, To: im.Key, Kind: index.RelCallsDynamic, File: im.File, Line: im.Line, Certainty: index.Dynamic}
				child.Children = append(child.Children, t.step(im, ir, level+1, path))
			}
		}
		st.Children = append(st.Children, child)
	}
	return st
}

// calls are s's outgoing calls, one per callee, in the order of the code.
func (t *tracer) calls(s *index.Symbol) []index.Relation {
	var out []index.Relation
	seen := map[string]bool{}
	for _, r := range t.ix.Out[s.Key] {
		if (r.Kind == index.RelCalls || r.Kind == index.RelCallsDynamic) && !seen[r.To] {
			seen[r.To] = true
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Col < out[j].Col
	})
	return out
}

// implementations are the methods that can run for a call to interface
// method m. Test doubles count only for a test.
func (t *tracer) implementations(m *index.Symbol, fromTest bool) []*index.Symbol {
	ifaceKey := index.KeyJoin(m.Pkg, "", m.Recv)
	var out []*index.Symbol
	for _, tk := range t.ix.Implementations(ifaceKey) {
		ty := t.ix.Symbols[tk]
		if ty == nil || (ty.Test && !fromTest) {
			continue
		}
		if im := t.ix.Symbols[tk+"."+m.Name]; im != nil {
			out = append(out, im)
		}
	}
	return out
}

// Entry is a place a flow starts: a route, a main function, a test.
type Entry struct {
	Kind  string `json:"kind"` // route, main, exported
	Label string `json:"label"`
	Key   string `json:"key"`
	File  string `json:"file"`
	Line  int    `json:"line"`
}

// Entries are the starting points of the repository's flows: its routes
// with handlers here, and its main functions.
func Entries(ix *index.Index) []Entry {
	out := []Entry{}
	for _, r := range ix.Routes {
		if r.Handler == "" || ix.Symbols[r.Handler] == nil {
			continue
		}
		label := r.Path
		if r.Method != "" {
			label = r.Method + " " + r.Path
		}
		out = append(out, Entry{Kind: "route", Label: label, Key: r.Handler, File: r.File, Line: r.Line})
	}
	for _, s := range ix.Symbols {
		if s.Kind == index.KindFunc && s.Name == "main" && s.PkgName == "main" {
			out = append(out, Entry{Kind: "main", Label: label(ix, s.Pkg), Key: s.Key, File: s.File, Line: s.Line})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind > out[j].Kind // routes, then main
		}
		return strings.Compare(out[i].Label, out[j].Label) < 0
	})
	return out
}
