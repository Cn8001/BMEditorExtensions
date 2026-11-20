package explain

import (
	"path/filepath"
	"sort"
	"strings"

	"bmeditor-mentor/internal/index"
)

// Node is a box in the architecture: a package, or a symbol in one.
type Node struct {
	ID            string   `json:"id"`
	Label         string   `json:"label"`
	Kind          string   `json:"kind"` // package, or a symbol's kind
	Doc           string   `json:"doc,omitempty"`
	Layer         string   `json:"layer,omitempty"`
	LayerInferred bool     `json:"layerInferred,omitempty"` // from the package's name, not from the code
	Language      string   `json:"language,omitempty"`
	Files         int      `json:"files,omitempty"`
	Symbols       int      `json:"symbols,omitempty"`
	Entry         []string `json:"entry,omitempty"` // main, routes
	File          string   `json:"file,omitempty"`
	Line          int      `json:"line,omitempty"`
	External      bool     `json:"external,omitempty"` // outside the focus, shown for its edges
	Errors        int      `json:"errors,omitempty"`
}

// Edge is a dependency between two nodes.
type Edge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Kind      string `json:"kind"`
	Count     int    `json:"count"` // how many relations it stands for
	Certainty string `json:"certainty"`
}

// Graph is one level of the architecture.
type Graph struct {
	Level string        `json:"level"` // packages, or package
	Focus string        `json:"focus,omitempty"`
	Nodes []Node        `json:"nodes"`
	Edges []Edge        `json:"edges"`
	Note  string        `json:"note"`
	Route []index.Route `json:"routes"`
}

// layers name the parts most Go services have, by the words their
// packages are named with. It is a guess, and is shown as one.
var layers = []struct {
	name  string
	words []string
}{
	{"Entry", []string{"cmd", "main"}},
	{"API", []string{"api", "http", "handler", "handlers", "server", "web", "rest", "grpc", "transport", "routes", "router", "controller", "controllers", "graphql"}},
	{"Service", []string{"service", "services", "usecase", "usecases", "app", "application", "domain", "core", "logic", "business"}},
	{"Repository", []string{"repo", "repository", "repositories", "store", "storage", "db", "database", "dao", "persistence", "sql", "postgres", "pg", "mysql", "mongo", "redis", "cache"}},
	{"Providers", []string{"provider", "providers", "client", "clients", "adapter", "adapters", "integration", "integrations", "gateway", "external", "oauth"}},
	{"Model", []string{"model", "models", "entity", "entities", "types", "schema", "dto"}},
	{"Config", []string{"config", "configuration", "settings", "env"}},
	{"Shared", []string{"util", "utils", "common", "shared", "lib", "helpers", "support", "x"}},
}

// LayerOf guesses a package's layer from the words of its path, the last
// one that names a layer winning (internal/auth/store is Repository).
func LayerOf(pkgPath, name string) string {
	if name == "main" {
		return "Entry"
	}
	parts := strings.Split(pkgPath, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		for _, w := range index.Split(parts[i]) {
			for _, l := range layers {
				for _, lw := range l.words {
					if w == lw {
						return l.name
					}
				}
			}
		}
	}
	return ""
}

// basePath folds an external test package into its package.
func basePath(p string) string { return strings.TrimSuffix(p, "_test") }

// Packages is the architecture at the level of packages: each package
// with its guessed layer, and the imports between them, which the
// compiler checked.
func Packages(ix *index.Index) *Graph {
	g := &Graph{Level: "packages", Nodes: []Node{}, Edges: []Edge{}, Route: []index.Route{},
		Note: "Edges are imports the compiler checked. Layers are guessed from package names."}
	nodes := map[string]*Node{}
	routes := map[string]int{}
	for _, r := range ix.Routes {
		if s := ix.Symbols[r.Handler]; s != nil {
			routes[basePath(s.Pkg)]++
		}
		g.Route = append(g.Route, r)
	}
	for path, p := range ix.Packages {
		id := basePath(path)
		n := nodes[id]
		if n == nil {
			n = &Node{ID: id, Label: label(ix, id), Kind: "package", Language: p.Language}
			nodes[id] = n
		}
		if p.Doc != "" && n.Doc == "" {
			n.Doc = index.FirstSentence(p.Doc)
		}
		n.Files += len(p.Files)
		n.Errors += len(p.Errors)
		for _, s := range p.Symbols {
			if s.Kind != index.KindField && !s.Test {
				n.Symbols++
			}
		}
		if p.Name == "main" && !contains(n.Entry, "main") {
			n.Entry = append(n.Entry, "main")
		}
		if l := LayerOf(id, p.Name); l != "" {
			n.Layer, n.LayerInferred = l, true
		}
	}
	for id, n := range nodes {
		if c := routes[id]; c > 0 {
			n.Entry = append(n.Entry, plural(c, "route"))
		}
	}
	counts := relationCounts(ix, func(s *index.Symbol) string { return basePath(s.Pkg) })
	edges := map[[2]string]*Edge{}
	for path, p := range ix.Packages {
		from := basePath(path)
		for _, imp := range p.Imports {
			if nodes[imp] == nil || imp == from {
				continue
			}
			k := [2]string{from, imp}
			if edges[k] == nil {
				edges[k] = &Edge{From: from, To: imp, Kind: "imports", Certainty: index.Verified, Count: counts[k]}
			}
		}
	}
	for _, n := range nodes {
		g.Nodes = append(g.Nodes, *n)
	}
	for _, e := range edges {
		g.Edges = append(g.Edges, *e)
	}
	sortGraph(g)
	return g
}

// Package is one package's inside: its types and functions that matter
// (exported, or used), what they use in other packages, and what uses
// them from outside.
func Package(ix *index.Index, focus string) *Graph {
	g := &Graph{Level: "package", Focus: focus, Nodes: []Node{}, Edges: []Edge{}, Route: []index.Route{},
		Note: "Edges are calls and references the compiler checked; dashed ones go through an interface."}
	in := func(s *index.Symbol) bool { return basePath(s.Pkg) == focus }
	shown := map[string]bool{}
	var members []*index.Symbol
	for _, s := range ix.Symbols {
		if !in(s) || s.Kind == index.KindField || s.Test {
			continue
		}
		members = append(members, s)
	}
	// Methods fold into their type, so the graph shows types.
	nodeOf := func(s *index.Symbol) string {
		if s.Kind == index.KindMethod && s.Recv != "" {
			return index.KeyJoin(s.Pkg, "", s.Recv)
		}
		if s.Kind == index.KindField && s.Recv != "" {
			return index.KeyJoin(s.Pkg, "", s.Recv)
		}
		return s.Key
	}
	for _, s := range members {
		if s.Kind == index.KindMethod {
			continue
		}
		if !s.Exported && len(ix.In[s.Key]) == 0 && s.Kind != index.KindFunc {
			continue
		}
		g.Nodes = append(g.Nodes, Node{ID: s.Key, Label: s.Name, Kind: string(s.Kind), Doc: index.FirstSentence(s.Doc), File: s.File, Line: s.Line})
		shown[s.Key] = true
	}
	ext := map[string]*Node{}
	edges := map[[2]string]*Edge{}
	add := func(from, to string, r index.Relation) {
		if from == to {
			return
		}
		k := [2]string{from, to}
		e := edges[k]
		if e == nil {
			e = &Edge{From: from, To: to, Kind: "uses", Certainty: index.Verified}
			edges[k] = e
		}
		e.Count++
		if r.Certainty == index.Dynamic {
			e.Certainty = index.Dynamic
		}
		if r.Kind == index.RelImplements {
			e.Kind = "implements"
		}
	}
	for _, s := range members {
		from := nodeOf(s)
		if !shown[from] {
			continue
		}
		for _, r := range ix.Out[s.Key] {
			if r.Kind == index.RelMethodOf || r.Kind == index.RelFieldOf {
				continue
			}
			t := ix.Symbols[r.To]
			if t == nil {
				continue
			}
			to := nodeOf(t)
			if !in(t) {
				pkg := basePath(t.Pkg)
				if ext[pkg] == nil {
					ext[pkg] = &Node{ID: pkg, Label: label(ix, pkg), Kind: "package", External: true}
				}
				to = pkg
			} else if !shown[to] {
				continue
			}
			add(from, to, r)
		}
		for _, r := range ix.In[s.Key] {
			u := ix.Symbols[r.From]
			if u == nil || in(u) || u.Test || r.Kind == index.RelMethodOf || r.Kind == index.RelFieldOf {
				continue
			}
			pkg := basePath(u.Pkg)
			if ext[pkg] == nil {
				ext[pkg] = &Node{ID: pkg, Label: label(ix, pkg), Kind: "package", External: true}
			}
			add(pkg, from, r)
		}
	}
	for _, n := range ext {
		g.Nodes = append(g.Nodes, *n)
	}
	for _, e := range edges {
		g.Edges = append(g.Edges, *e)
	}
	for _, r := range ix.Routes {
		if s := ix.Symbols[r.Handler]; s != nil && in(s) {
			g.Route = append(g.Route, r)
		}
	}
	sortGraph(g)
	return g
}

// relationCounts counts the relations between the groups symbols fall
// in, such as their packages.
func relationCounts(ix *index.Index, group func(*index.Symbol) string) map[[2]string]int {
	out := map[[2]string]int{}
	for from, rels := range ix.Out {
		fs := ix.Symbols[from]
		if fs == nil || fs.Test {
			continue
		}
		for _, r := range rels {
			if t := ix.Symbols[r.To]; t != nil {
				out[[2]string{group(fs), group(t)}]++
			}
		}
	}
	return out
}

// label is a package's path within its module, which is shorter.
func label(ix *index.Index, pkg string) string {
	best := ""
	for _, m := range ix.Modules {
		if (pkg == m.Path || strings.HasPrefix(pkg, m.Path+"/")) && len(m.Path) > len(best) {
			best = m.Path
		}
	}
	if best == "" {
		if p := ix.Packages[pkg]; p != nil && p.Dir != "" {
			if rel, err := filepath.Rel(ix.Root, p.Dir); err == nil {
				return filepath.ToSlash(rel)
			}
		}
		return pkg
	}
	if pkg == best {
		return filepath.Base(pkg)
	}
	return strings.TrimPrefix(pkg, best+"/")
}

func sortGraph(g *Graph) {
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].ID < g.Nodes[j].ID })
	sort.Slice(g.Edges, func(i, j int) bool {
		if g.Edges[i].From != g.Edges[j].From {
			return g.Edges[i].From < g.Edges[j].From
		}
		return g.Edges[i].To < g.Edges[j].To
	})
	sort.Slice(g.Route, func(i, j int) bool { return g.Route[i].Path < g.Route[j].Path })
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return itoa(n) + " " + word + "s"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
