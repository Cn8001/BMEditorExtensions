// Package intel answers a task about the code, such as "add a new
// authentication provider", with what the index knows for certain: the
// interface to implement, the types that already do and how they differ,
// how it is wired in from the entry point down to the store, what to change
// and where, the tests that cover it and where to stop in the debugger.
// Every item is a place in the code. Nothing is guessed but the choice of
// the extension point among the search's candidates, which says why.
package intel

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"bmeditor-mentor/internal/explain"
	"bmeditor-mentor/internal/index"
)

// Answer is what the code says about a task.
type Answer struct {
	Query string `json:"query"`
	// Point is the extension point the task goes through; nil when the
	// candidates have no interface with implementations.
	Point *Point `json:"point,omitempty"`
	// Feature follows the part of the project the task is about, end to
	// end; nil when no way in reaches it.
	Feature *Feature      `json:"feature,omitempty"`
	Related []explain.Ref `json:"related"` // the best of the search's candidates
	Note    string        `json:"note,omitempty"`
}

// Point is an interface the task extends, and everything around it.
type Point struct {
	Interface explain.Ref   `json:"interface"`
	Why       string        `json:"why"` // why this one
	Doc       string        `json:"doc,omitempty"`
	Methods   []explain.Ref `json:"methods"` // what an implementation must have
	Impls     []Impl        `json:"impls"`
	Fakes     []explain.Ref `json:"fakes"` // test types that implement it
	// Holders are the fields and functions that keep or take it.
	Holders []explain.Ref `json:"holders"`
	// Consumers call its methods; the site is the call.
	Consumers []explain.Ref `json:"consumers"`
	// Made are where implementations are built, outside tests; the site is
	// the literal, Via the type.
	Made []explain.Ref `json:"made"`
	// Upstream reaches the consumers: the routes and functions a request
	// comes through. Downstream is what the consumers call next.
	Routes      []index.Route `json:"routes"`
	Upstream    []explain.Ref `json:"upstream"`
	Downstream  []explain.Ref `json:"downstream"`
	Steps       []Step        `json:"steps"`
	Tests       []explain.Ref `json:"tests"`
	Breakpoints []explain.Ref `json:"breakpoints"`
}

// Impl is a type that implements the point's interface.
type Impl struct {
	explain.Ref
	Doc      string   `json:"doc,omitempty"`
	Template bool     `json:"template,omitempty"` // the one to start a new one from
	Fields   []string `json:"fields"`
	Notes    []string `json:"notes"` // how it differs: what its methods do that others' do not
	Tests    int      `json:"tests"`
	Made     int      `json:"made"`
}

// Step is one thing to do, at a place when there is one.
type Step struct {
	Text string       `json:"text"`
	At   *explain.Ref `json:"at,omitempty"`
}

// Task answers q from the index and the search's candidates for it, best
// first.
func Task(ix *index.Index, q string, candidates []string) *Answer {
	a := &Answer{Query: q, Related: []explain.Ref{}}
	for i, k := range candidates {
		if s := ix.Symbols[k]; s != nil && i < 6 {
			a.Related = append(a.Related, explain.RefOf(s))
		}
	}
	a.Feature = FeatureOf(ix, candidates)
	iface, why := pick(ix, candidates)
	if iface == nil {
		if a.Feature == nil {
			a.Note = "Nothing reaches this from a way in, and no interface fits it; the code below matches it best."
		}
		return a
	}
	a.Point = point(ix, iface, why)
	return a
}

// pick chooses the interface the task most likely extends: one among the
// candidates with implementations, one a candidate type implements, or the
// owner of a candidate method, earlier candidates weighing more.
func pick(ix *index.Index, candidates []string) (*index.Symbol, string) {
	score := map[string]int{}
	reason := map[string]string{}
	add := func(key string, n int, why string) {
		s := ix.Symbols[key]
		if s == nil || s.Kind != index.KindInterface || s.Test || len(impls(ix, s)) == 0 {
			return
		}
		score[key] += n
		if reason[key] == "" {
			reason[key] = why
		}
	}
	for i, k := range candidates {
		s := ix.Symbols[k]
		if s == nil {
			continue
		}
		w := max(1, 40-6*i)
		switch s.Kind {
		case index.KindInterface:
			add(k, 2*w, "it matches the task")
		case index.KindStruct, index.KindType:
			for _, r := range ix.Out[k] {
				if r.Kind == index.RelImplements {
					add(r.To, w, s.Name+", which matches the task, implements it")
				}
			}
		case index.KindMethod:
			if o := owner(ix, s); o != nil {
				add(o.Key, w, "its method "+s.Name+" matches the task")
			}
		}
	}
	best := ""
	for k, n := range score {
		if best == "" || n > score[best] || n == score[best] && k < best {
			best = k
		}
	}
	if best == "" {
		return nil, ""
	}
	s := ix.Symbols[best]
	return s, fmt.Sprintf("%s; %d types implement it", reason[best], len(impls(ix, s)))
}

func owner(ix *index.Index, s *index.Symbol) *index.Symbol {
	for _, r := range ix.Out[s.Key] {
		if r.Kind == index.RelMethodOf || r.Kind == index.RelFieldOf {
			return ix.Symbols[r.To]
		}
	}
	return nil
}

func impls(ix *index.Index, iface *index.Symbol) []*index.Symbol {
	var out []*index.Symbol
	for _, r := range ix.In[iface.Key] {
		if r.Kind == index.RelImplements {
			if s := ix.Symbols[r.From]; s != nil {
				out = append(out, s)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func methodsOf(ix *index.Index, t *index.Symbol) map[string]*index.Symbol {
	out := map[string]*index.Symbol{}
	for _, r := range ix.In[t.Key] {
		if r.Kind == index.RelMethodOf {
			if m := ix.Symbols[r.From]; m != nil {
				out[m.Name] = m
			}
		}
	}
	return out
}

func site(s *index.Symbol, r index.Relation, via string) explain.Ref {
	ref := explain.RefOf(s)
	ref.Via, ref.Certainty, ref.SiteFile, ref.SiteLine = via, r.Certainty, r.File, r.Line
	return ref
}

func point(ix *index.Index, iface *index.Symbol, why string) *Point {
	p := &Point{Interface: explain.RefOf(iface), Why: why, Doc: index.FirstSentence(iface.Doc),
		Methods: []explain.Ref{}, Impls: []Impl{}, Fakes: []explain.Ref{}, Holders: []explain.Ref{}, Consumers: []explain.Ref{},
		Made: []explain.Ref{}, Routes: []index.Route{}, Upstream: []explain.Ref{}, Downstream: []explain.Ref{},
		Steps: []Step{}, Tests: []explain.Ref{}, Breakpoints: []explain.Ref{}}
	seen := map[string]bool{}
	once := func(kind, key string) bool {
		k := kind + "|" + key
		if seen[k] {
			return false
		}
		seen[k] = true
		return true
	}
	tests := map[string]explain.Ref{}
	addTest := func(t *index.Symbol, r index.Relation, via string) {
		if t.Test && t.Kind == index.KindFunc && strings.HasPrefix(t.Name, "Test") {
			if _, ok := tests[t.Key]; !ok {
				tests[t.Key] = site(t, r, via)
			}
		}
	}

	// What an implementation must have.
	methods := methodsOf(ix, iface)
	names := make([]string, 0, len(methods))
	for n := range methods {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return methods[names[i]].Line < methods[names[j]].Line })
	for _, n := range names {
		p.Methods = append(p.Methods, explain.RefOf(methods[n]))
	}

	// Who calls its methods, and where.
	consumers := map[string]bool{}
	for _, n := range names {
		for _, r := range ix.In[methods[n].Key] {
			if r.Kind != index.RelCalls && r.Kind != index.RelCallsDynamic {
				continue
			}
			u := ix.Symbols[r.From]
			if u == nil {
				continue
			}
			if u.Test {
				addTest(u, r, "calls "+n)
				continue
			}
			consumers[u.Key] = true
			if once("consumer", u.Key+n) {
				p.Consumers = append(p.Consumers, site(u, r, "calls "+n))
				p.Breakpoints = append(p.Breakpoints, site(u, r, "where "+u.Name+" calls "+iface.Name+"."+n))
			}
		}
	}

	// Who keeps or takes it: fields of its type, functions that name it.
	word := regexp.MustCompile(`\b` + regexp.QuoteMeta(iface.Name) + `\b`)
	for _, r := range ix.In[iface.Key] {
		if r.Kind != index.RelReferences {
			continue
		}
		u := ix.Symbols[r.From]
		if u == nil || u.Test {
			continue
		}
		switch u.Kind {
		case index.KindStruct:
			for _, fr := range ix.In[u.Key] {
				if f := ix.Symbols[fr.From]; f != nil && fr.Kind == index.RelFieldOf && word.MatchString(f.Sig) && once("holder", f.Key) {
					ref := explain.RefOf(f)
					ref.Via = "keeps it: " + f.Sig
					p.Holders = append(p.Holders, ref)
				}
			}
		case index.KindFunc, index.KindMethod:
			if r.Line == u.Line && once("holder", u.Key) {
				ref := site(u, r, "takes it")
				p.Holders = append(p.Holders, ref)
			}
		}
	}

	// The implementations, how each differs, and where they are made.
	var all []*index.Symbol
	for _, t := range impls(ix, iface) {
		if t.Test {
			p.Fakes = append(p.Fakes, explain.RefOf(t))
			continue
		}
		all = append(all, t)
	}
	madeAt := map[string]int{} // a line where implementations are made → its index in p.Made
	best, bestScore := -1, -1
	for i, t := range all {
		im := Impl{Ref: explain.RefOf(t), Doc: index.FirstSentence(t.Doc), Fields: []string{}, Notes: []string{}}
		for _, f := range t.Fields {
			im.Fields = append(im.Fields, strings.TrimSpace(f.Name+" "+f.Type))
		}
		own := methodsOf(ix, t)
		for _, n := range names {
			m := own[n]
			if m == nil {
				continue
			}
			switch {
			case m.ErrReturns > 0:
				im.Notes = append(im.Notes, fmt.Sprintf("%s can fail: %d error return(s), %d branch(es)", n, m.ErrReturns, m.Branches))
			case m.ReturnsError:
				im.Notes = append(im.Notes, n+" never returns an error")
			case m.Branches > 0:
				im.Notes = append(im.Notes, fmt.Sprintf("%s has %d branch(es)", n, m.Branches))
			}
		}
		var extra []string
		for n := range own {
			if methods[n] == nil {
				extra = append(extra, n)
			}
		}
		sort.Strings(extra)
		if len(extra) > 0 {
			im.Notes = append(im.Notes, "more methods: "+strings.Join(extra, ", "))
		}
		for _, r := range ix.In[t.Key] {
			u := ix.Symbols[r.From]
			if u == nil {
				continue
			}
			if u.Test {
				addTest(u, r, "uses "+t.Name)
				im.Tests++
				continue
			}
			if r.Kind == index.RelConstructs && (u.Recv != t.Name || u.Pkg != t.Pkg) {
				im.Made++
				at := r.File + ":" + fmt.Sprint(r.Line)
				if j, ok := madeAt[at]; ok {
					// Several made on one line, as a list passed along.
					p.Made[j].Via += " and " + t.Name
				} else {
					madeAt[at] = len(p.Made)
					p.Made = append(p.Made, site(u, r, t.Name))
				}
			}
		}
		branches := 0
		for _, m := range own {
			branches += m.Branches + m.ErrReturns
		}
		if s := 3*im.Tests + 2*im.Made + branches; s > bestScore {
			best, bestScore = i, s
		}
		p.Impls = append(p.Impls, im)
	}
	if best >= 0 {
		p.Impls[best].Template = true
	}

	// The path a request takes to the consumers, and on from them.
	for k := range consumers {
		for _, rt := range ix.Routes {
			if rt.Handler == k && once("route", rt.File+fmt.Sprint(rt.Line)) {
				p.Routes = append(p.Routes, rt)
			}
		}
		for _, r := range ix.In[k] {
			if r.Kind != index.RelCalls && r.Kind != index.RelCallsDynamic {
				continue
			}
			u := ix.Symbols[r.From]
			if u == nil {
				continue
			}
			if u.Test {
				addTest(u, r, "calls "+ix.Symbols[k].Name)
				continue
			}
			if !consumers[u.Key] && once("up", u.Key) {
				p.Upstream = append(p.Upstream, site(u, r, "calls "+ix.Symbols[k].Name))
				for _, rt := range ix.Routes {
					if rt.Handler == u.Key && once("route", rt.File+fmt.Sprint(rt.Line)) {
						p.Routes = append(p.Routes, rt)
					}
				}
			}
		}
		for _, r := range ix.Out[k] {
			if r.Kind != index.RelCalls && r.Kind != index.RelCallsDynamic {
				continue
			}
			t := ix.Symbols[r.To]
			if t == nil || t.Test || methods[t.Name] != nil && t.Recv == iface.Name || consumers[t.Key] {
				continue
			}
			if once("down", t.Key) {
				p.Downstream = append(p.Downstream, site(t, r, "called by "+ix.Symbols[k].Name))
			}
		}
	}
	sortRefs(p.Consumers)
	sortRefs(p.Upstream)
	sortRefs(p.Downstream)
	sortRefs(p.Made)
	sortRefs(p.Breakpoints)
	for _, t := range tests {
		p.Tests = append(p.Tests, t)
	}
	sortRefs(p.Tests)

	p.Steps = steps(p, iface)
	if best >= 0 {
		for _, n := range names {
			if m := methodsOf(ix, all[best])[n]; m != nil {
				p.Breakpoints = append(p.Breakpoints, site(m, index.Relation{File: m.File, Line: m.Line}, "in "+all[best].Name+"."+n+", the one a new type starts from"))
			}
		}
	}
	return p
}

// steps are what adding an implementation takes, in order.
func steps(p *Point, iface *index.Symbol) []Step {
	var out []Step
	sigs := make([]string, 0, len(p.Methods))
	for _, m := range p.Methods {
		sigs = append(sigs, method(m.Sig))
	}
	var tmpl *Impl
	for i := range p.Impls {
		if p.Impls[i].Template {
			tmpl = &p.Impls[i]
		}
	}
	text := fmt.Sprintf("Add a type that implements %s: %s.", iface.Name, strings.Join(sigs, "; "))
	if tmpl != nil {
		ref := tmpl.Ref
		out = append(out, Step{Text: text + fmt.Sprintf(" Start from %s, beside it in %s.", tmpl.Name, filepath.Base(tmpl.File)), At: &ref})
	} else {
		ref := p.Interface
		out = append(out, Step{Text: text, At: &ref})
	}
	for i := range p.Made {
		m := p.Made[i]
		out = append(out, Step{Text: fmt.Sprintf("Make it beside %s, in %s, so it is used.", m.Via, m.Name), At: &m})
	}
	for i := range p.Holders {
		h := p.Holders[i]
		if strings.HasPrefix(h.Via, "keeps it") {
			out = append(out, Step{Text: fmt.Sprintf("It joins %s.%s, which keeps them (%s).", h.Recv, h.Name, strings.TrimPrefix(h.Via, "keeps it: ")), At: &h})
		}
	}
	if tmpl != nil {
		for i := range p.Tests {
			t := p.Tests[i]
			if t.Via == "uses "+tmpl.Name {
				out = append(out, Step{Text: fmt.Sprintf("Test it as %s tests %s.", t.Name, tmpl.Name), At: &t})
				break
			}
		}
	}
	return out
}

// method is a method's signature without its receiver: Exchange(ctx
// context.Context, code string) (UserIdentity, error).
func method(sig string) string {
	if i := strings.Index(sig, ")."); strings.HasPrefix(sig, "func (") && i > 0 {
		return sig[i+2:]
	}
	return strings.TrimPrefix(sig, "func ")
}

func sortRefs(refs []explain.Ref) {
	sort.SliceStable(refs, func(i, j int) bool {
		a, b := refs[i], refs[j]
		fa, fb := a.SiteFile, b.SiteFile
		if fa == "" {
			fa = a.File
		}
		if fb == "" {
			fb = b.File
		}
		if fa != fb {
			return fa < fb
		}
		return a.SiteLine < b.SiteLine
	})
}
