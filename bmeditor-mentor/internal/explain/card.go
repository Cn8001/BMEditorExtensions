// Package explain answers questions about the code from the index alone:
// a symbol's card, the architecture, the path a request takes. Every item
// it gives is a place in the code, and says how sure it is.
package explain

import (
	"bufio"
	"os"
	"sort"
	"strings"

	"bmeditor-mentor/internal/index"
	"bmeditor-mentor/internal/search"
)

// Ref is a symbol another answer points at, with where.
type Ref struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Recv      string `json:"recv,omitempty"`
	Pkg       string `json:"pkg"`
	PkgName   string `json:"pkgName"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Col       int    `json:"col"`
	Sig       string `json:"sig,omitempty"`
	Test      bool   `json:"test,omitempty"`
	Via       string `json:"via,omitempty"`       // the relation, such as calls
	Certainty string `json:"certainty,omitempty"` // verified, dynamic or inferred
	// SiteFile and SiteLine are where the relation was seen, such as the
	// line of the call, when it is not the symbol's own line.
	SiteFile string `json:"siteFile,omitempty"`
	SiteLine int    `json:"siteLine,omitempty"`
}

// RefOf makes a Ref of s.
func RefOf(s *index.Symbol) Ref {
	return Ref{Key: s.Key, Name: s.Name, Kind: string(s.Kind), Recv: s.Recv, Pkg: s.Pkg, PkgName: s.PkgName,
		File: s.File, Line: s.Line, Col: s.Col, Sig: s.Sig, Test: s.Test}
}

func refVia(s *index.Symbol, r index.Relation) Ref {
	ref := RefOf(s)
	ref.Via, ref.Certainty, ref.SiteFile, ref.SiteLine = string(r.Kind), r.Certainty, r.File, r.Line
	return ref
}

// Snippet is a few lines of real code around a use.
type Snippet struct {
	File  string `json:"file"`
	Line  int    `json:"line"`  // the line of the use
	First int    `json:"first"` // the first line of Text
	Text  string `json:"text"`
	In    string `json:"in"` // the name of the function it is in
}

// Group is refs from one package.
type Group struct {
	Pkg  string `json:"pkg"`
	Refs []Ref  `json:"refs"`
}

// Impact is how far a change to a symbol can reach: everything that
// uses it, and what uses that, through calls and references.
type Impact struct {
	Symbols  int      `json:"symbols"`
	Packages []string `json:"packages"`
	Tests    int      `json:"tests"`
	Capped   bool     `json:"capped"` // it stopped counting at its limit
}

// Card is everything the index knows about one symbol.
type Card struct {
	Symbol         *index.Symbol `json:"symbol"`
	Language       string        `json:"language"`
	Responsibility string        `json:"responsibility"`
	Derived        bool          `json:"derived"`
	PackageDoc     string        `json:"packageDoc,omitempty"`
	Owner          *Ref          `json:"owner,omitempty"` // a method's or field's type
	Fields         []Ref         `json:"fields"`
	Methods        []Ref         `json:"methods"`
	Implements     []Ref         `json:"implements"`
	ImplementedBy  []Ref         `json:"implementedBy"`
	Embeds         []Ref         `json:"embeds"`
	EmbeddedBy     []Ref         `json:"embeddedBy"`
	Constructors   []Ref         `json:"constructors"`
	Callers        []Ref         `json:"callers"`
	Callees        []Ref         `json:"callees"`
	UsedBy         []Group       `json:"usedBy"`
	Usages         []Snippet     `json:"usages"`
	Tests          []Ref         `json:"tests"`
	IndirectTests  []Ref         `json:"indirectTests"`
	Routes         []index.Route `json:"routes"`
	Impact         Impact        `json:"impact"`
}

// CardOf is key's card, nil when the index has no such symbol.
func CardOf(ix *index.Index, key string) *Card {
	s := ix.Symbols[key]
	if s == nil {
		return nil
	}
	c := &Card{Symbol: s, Fields: []Ref{}, Methods: []Ref{}, Implements: []Ref{}, ImplementedBy: []Ref{}, Embeds: []Ref{},
		EmbeddedBy: []Ref{}, Constructors: []Ref{}, Callers: []Ref{}, Callees: []Ref{}, UsedBy: []Group{}, Usages: []Snippet{},
		Tests: []Ref{}, IndirectTests: []Ref{}, Routes: []index.Route{}}
	c.Responsibility, c.Derived = search.Responsibility(ix, s)
	if p := ix.Packages[s.Pkg]; p != nil {
		c.PackageDoc = index.FirstSentence(p.Doc)
		c.Language = p.Language
	}
	seen := map[string]bool{}
	once := func(kind, key string) bool {
		k := kind + "|" + key
		if seen[k] {
			return false
		}
		seen[k] = true
		return true
	}
	for _, r := range ix.Out[key] {
		t := ix.Symbols[r.To]
		if t == nil {
			continue
		}
		switch r.Kind {
		case index.RelMethodOf, index.RelFieldOf:
			ref := RefOf(t)
			c.Owner = &ref
		case index.RelImplements:
			if once("impl", t.Key) {
				c.Implements = append(c.Implements, refVia(t, r))
			}
		case index.RelEmbeds:
			if once("emb", t.Key) {
				c.Embeds = append(c.Embeds, refVia(t, r))
			}
		case index.RelCalls, index.RelCallsDynamic:
			if once("callee", t.Key) {
				c.Callees = append(c.Callees, refVia(t, r))
			}
		}
	}
	users := map[string][]Ref{}
	var sites []index.Relation
	for _, r := range uses(ix, s) {
		u := ix.Symbols[r.From]
		if u == nil {
			continue
		}
		switch r.Kind {
		case index.RelFieldOf:
			c.Fields = append(c.Fields, RefOf(u))
			continue
		case index.RelMethodOf:
			c.Methods = append(c.Methods, RefOf(u))
			continue
		case index.RelImplements:
			if once("implby", u.Key) {
				c.ImplementedBy = append(c.ImplementedBy, refVia(u, r))
			}
			continue
		case index.RelEmbeds:
			if once("embby", u.Key) {
				c.EmbeddedBy = append(c.EmbeddedBy, refVia(u, r))
			}
			continue
		case index.RelReturns, index.RelConstructs:
			if (s.Kind == index.KindStruct || s.Kind == index.KindType || s.Kind == index.KindInterface) && !u.Test && once("ctor", u.Key) {
				c.Constructors = append(c.Constructors, refVia(u, r))
			}
		case index.RelCalls, index.RelCallsDynamic:
			if r.To == s.Key && once("caller", u.Key) {
				c.Callers = append(c.Callers, refVia(u, r))
			}
		}
		if u.Test {
			if once("test", u.Key) {
				c.Tests = append(c.Tests, refVia(u, r))
			}
			continue
		}
		if once("user", u.Key) {
			users[u.Pkg] = append(users[u.Pkg], refVia(u, r))
			sites = append(sites, r)
		}
	}
	// A method that implements an interface's method is also called
	// through it, by whoever calls the interface.
	if s.Kind == index.KindMethod && s.Recv != "" {
		owner := index.KeyJoin(s.Pkg, "", s.Recv)
		for _, r := range ix.Out[owner] {
			if r.Kind != index.RelImplements {
				continue
			}
			im := ix.Symbols[r.To+"."+s.Name]
			if im == nil {
				continue
			}
			for _, cr := range ix.In[im.Key] {
				if cr.Kind != index.RelCalls && cr.Kind != index.RelCallsDynamic {
					continue
				}
				if u := ix.Symbols[cr.From]; u != nil && once("caller", u.Key) {
					ref := refVia(u, cr)
					ref.Certainty = index.Dynamic
					ref.Via = "through " + im.Recv + "." + im.Name
					c.Callers = append(c.Callers, ref)
				}
			}
		}
	}
	pkgs := make([]string, 0, len(users))
	for p := range users {
		pkgs = append(pkgs, p)
	}
	sort.Slice(pkgs, func(i, j int) bool {
		if (pkgs[i] == s.Pkg) != (pkgs[j] == s.Pkg) {
			return pkgs[i] != s.Pkg // other packages first: they show how it is used from outside
		}
		return pkgs[i] < pkgs[j]
	})
	for _, p := range pkgs {
		c.UsedBy = append(c.UsedBy, Group{Pkg: p, Refs: users[p]})
	}
	c.Usages = snippets(ix, s, sites, 3)
	// Tests of what uses it reach it too.
	for _, g := range c.UsedBy {
		for _, u := range g.Refs {
			for _, r := range ix.In[u.Key] {
				if t := ix.Symbols[r.From]; t != nil && t.Test && once("test", t.Key) {
					ref := refVia(t, r)
					ref.Via = "through " + u.Name
					c.IndirectTests = append(c.IndirectTests, ref)
				}
			}
		}
	}
	for _, rt := range ix.Routes {
		if rt.Handler == key {
			c.Routes = append(c.Routes, rt)
		}
	}
	c.Impact = ImpactOf(ix, key, 400)
	sortRefs(c.Callers)
	sortRefs(c.Callees)
	sortRefs(c.Constructors)
	return c
}

// uses are the relations into s, and for a type the relations into its
// fields and methods: a struct is used through them.
func uses(ix *index.Index, s *index.Symbol) []index.Relation {
	out := append([]index.Relation{}, ix.In[s.Key]...)
	if s.Kind == index.KindStruct || s.Kind == index.KindType || s.Kind == index.KindInterface {
		for _, r := range ix.In[s.Key] {
			if r.Kind != index.RelFieldOf && r.Kind != index.RelMethodOf {
				continue
			}
			for _, rr := range ix.In[r.From] {
				if rr.Kind == index.RelCalls || rr.Kind == index.RelCallsDynamic || rr.Kind == index.RelReferences {
					if rr.From != s.Key {
						out = append(out, rr)
					}
				}
			}
		}
	}
	return out
}

// ImpactOf counts what a change to key can reach, up to limit symbols.
func ImpactOf(ix *index.Index, key string, limit int) Impact {
	return ImpactOfAll(ix, []string{key}, limit)
}

// ImpactOfAll counts what a change to all of keys can reach together:
// everything that uses them, and what uses that, through calls and
// references, up to limit symbols. The keys themselves do not count.
func ImpactOfAll(ix *index.Index, keys []string, limit int) Impact {
	seen := map[string]bool{}
	var queue []string
	for _, key := range keys {
		seen[key] = true
	}
	for _, key := range keys {
		queue = append(queue, key)
		if s := ix.Symbols[key]; s != nil {
			for _, r := range uses(ix, s) {
				if !seen[r.From] && r.Kind != index.RelFieldOf && r.Kind != index.RelMethodOf {
					seen[r.From] = true
					queue = append(queue, r.From)
				}
			}
		}
	}
	pkgs := map[string]bool{}
	imp := Impact{Packages: []string{}}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		for _, r := range ix.In[k] {
			switch r.Kind {
			case index.RelCalls, index.RelCallsDynamic, index.RelReferences, index.RelConstructs:
			default:
				continue
			}
			if seen[r.From] {
				continue
			}
			if len(seen) >= limit {
				imp.Capped = true
				break
			}
			seen[r.From] = true
			queue = append(queue, r.From)
		}
	}
	for _, key := range keys {
		delete(seen, key)
	}
	for k := range seen {
		u := ix.Symbols[k]
		if u == nil {
			continue
		}
		if u.Test {
			imp.Tests++
			continue
		}
		imp.Symbols++
		pkgs[u.PkgName] = true
	}
	for p := range pkgs {
		imp.Packages = append(imp.Packages, p)
	}
	sort.Strings(imp.Packages)
	return imp
}

func snippets(ix *index.Index, s *index.Symbol, sites []index.Relation, n int) []Snippet {
	out := []Snippet{}
	files := map[string][]string{}
	for _, r := range sites {
		if len(out) == n {
			break
		}
		if r.File == "" || r.Line == 0 {
			continue
		}
		lines, ok := files[r.File]
		if !ok {
			lines = ReadLines(r.File)
			files[r.File] = lines
		}
		if r.Line > len(lines) {
			continue
		}
		first := max(1, r.Line-2)
		last := min(len(lines), r.Line+2)
		in := ""
		if u := ix.Symbols[r.From]; u != nil {
			in = u.Name
			if u.Recv != "" {
				in = u.Recv + "." + u.Name
			}
		}
		out = append(out, Snippet{File: r.File, Line: r.Line, First: first, Text: strings.Join(lines[first-1:last], "\n"), In: in})
	}
	return out
}

// ReadLines reads a file's lines; none when it cannot.
func ReadLines(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4<<20)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

func sortRefs(refs []Ref) {
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Test != refs[j].Test {
			return !refs[i].Test
		}
		if refs[i].File != refs[j].File {
			return refs[i].File < refs[j].File
		}
		return refs[i].Line < refs[j].Line
	})
}
