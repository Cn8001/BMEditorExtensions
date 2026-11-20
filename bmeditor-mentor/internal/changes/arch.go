package changes

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"bmeditor-mentor/internal/explain"
	"bmeditor-mentor/internal/gitx"
	"bmeditor-mentor/internal/index"
)

// What changes changed in the project's shape, rather than in its lines:
// the packages they touch and the new ones, the parts they add, and the
// dependencies between packages that came or went, with the ones that cut
// across the layers said so. The developer keeps the whole in view from
// this, without reading every line.

// ArchChange is the project's shape as the changes leave it.
type ArchChange struct {
	Packages []PkgChange  `json:"packages"`
	Edges    []EdgeChange `json:"edges"`
}

// PkgChange is one package the changes touch.
type PkgChange struct {
	Path     string        `json:"path"`
	Name     string        `json:"name"`
	Layer    string        `json:"layer,omitempty"` // guessed from its path
	New      bool          `json:"new,omitempty"`   // it was not there before
	Added    int           `json:"added"`
	Modified int           `json:"modified"`
	Removed  int           `json:"removed"`
	Parts    []explain.Ref `json:"parts"` // new types, interfaces and exported functions
}

// EdgeChange is a dependency between two packages of the project that the
// changes added or took away.
type EdgeChange struct {
	From      string `json:"from"`
	To        string `json:"to"`
	FromName  string `json:"fromName"`
	ToName    string `json:"toName"`
	FromLayer string `json:"fromLayer,omitempty"`
	ToLayer   string `json:"toLayer,omitempty"`
	Added     bool   `json:"added"`
	Warn      string `json:"warn,omitempty"` // how it cuts across the layers, if it does
	File      string `json:"file,omitempty"` // where the import is written
	Line      int    `json:"line,omitempty"`
}

// rank orders the layers from the way in down to the store; the others
// (models, config, shared) any layer may use.
var rank = map[string]int{"Entry": 0, "API": 1, "Service": 2, "Providers": 3, "Repository": 4}

// Before says what a file imported before the changes, and whether it was
// there.
type Before func(path string) (imports []string, existed bool)

// HeadBefore is before as HEAD has it.
func HeadBefore(ctx context.Context, ix *index.Index) Before {
	return func(path string) ([]string, bool) {
		src, err := gitx.Show(ctx, ix.Root, "HEAD", path)
		if err != nil {
			return nil, false
		}
		return importsOf(src), true
	}
}

// Before is before as the baseline has it: what a step began with.
func (b Baseline) Before() Before {
	return func(path string) ([]string, bool) {
		fb, ok := b.Files[path]
		return fb.Imports, ok
	}
}

func importsOf(src []byte) []string {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ImportsOnly)
	if err != nil || f == nil {
		return nil
	}
	var out []string
	for _, im := range f.Imports {
		if p, err := strconv.Unquote(im.Path.Value); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// Arch works out the change in shape from the files that changed, with
// their declarations' changes, as ByFile or StepFiles give them, and what
// the files imported before.
func Arch(ctx context.Context, ix *index.Index, files []FileChanges, prev Before) *ArchChange {
	a := &ArchChange{Packages: []PkgChange{}, Edges: []EdgeChange{}}
	internal := func(p string) bool {
		if ix.Packages[p] != nil {
			return true
		}
		for _, m := range ix.Modules {
			if p == m.Path || strings.HasPrefix(p, m.Path+"/") {
				return true
			}
		}
		return false
	}
	type pkgFiles struct {
		touched map[string]FileChanges
		allNew  bool
	}
	pkgs := map[string]*pkgFiles{}
	var order []string
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f.Path)
		if err != nil {
			src, _ = gitx.Show(ctx, ix.Root, "HEAD", f.Path)
		}
		p := pkgPathFor(ix, f.Path, src)
		if p == "" {
			continue
		}
		pf := pkgs[p]
		if pf == nil {
			pf = &pkgFiles{touched: map[string]FileChanges{}, allNew: true}
			pkgs[p] = pf
			order = append(order, p)
		}
		pf.touched[f.Path] = f
		if f.Status != "A" {
			pf.allNew = false
		}
	}
	sort.Strings(order)
	hasService := false
	for p, pk := range ix.Packages {
		if explain.LayerOf(p, pk.Name) == "Service" {
			hasService = true
			break
		}
	}
	for _, p := range order {
		pf := pkgs[p]
		name := p[strings.LastIndex(p, "/")+1:]
		if pk := ix.Packages[p]; pk != nil {
			name = pk.Name
		}
		pc := PkgChange{Path: p, Name: name, Layer: explain.LayerOf(p, name), New: pf.allNew && ix.Packages[p] == nil || pf.allNew && onlyTouched(ix.Packages[p], pf.touched), Parts: []explain.Ref{}}
		for _, f := range pf.touched {
			for _, c := range f.Changes {
				switch c.Change {
				case "added":
					pc.Added++
					if len(pc.Parts) < 6 && !c.Test && (c.Kind == string(index.KindStruct) || c.Kind == string(index.KindInterface) || c.Kind == string(index.KindType) ||
						c.Kind == string(index.KindFunc) && isExported(c.Name)) {
						pc.Parts = append(pc.Parts, c.Ref)
					}
				case "modified":
					pc.Modified++
				case "removed":
					pc.Removed++
				}
			}
		}
		sort.Slice(pc.Parts, func(i, j int) bool { return pc.Parts[i].Line < pc.Parts[j].Line })
		a.Packages = append(a.Packages, pc)

		// The package's imports of the project's packages, now and before:
		// the files it did not touch are as they were.
		now, was := map[string]place{}, map[string]place{}
		dirFiles := map[string]bool{}
		if pk := ix.Packages[p]; pk != nil {
			for _, f := range pk.Files {
				dirFiles[f] = true
			}
		}
		for f := range pf.touched {
			dirFiles[f] = true
		}
		for f := range dirFiles {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			cur, err := os.ReadFile(f)
			if err == nil {
				addImports(now, f, cur, internal)
			}
			tf, touched := pf.touched[f]
			switch {
			case !touched:
				if err == nil {
					addImports(was, f, cur, internal)
				}
			case tf.Status != "A":
				if old, ok := prev(f); ok {
					for _, imp := range old {
						if _, seen := was[imp]; !seen && internal(imp) {
							was[imp] = place{file: f}
						}
					}
				}
			}
		}
		for to, at := range now {
			if _, ok := was[to]; !ok && to != p {
				a.Edges = append(a.Edges, edge(ix, p, name, to, true, at, hasService))
			}
		}
		for to, at := range was {
			if _, ok := now[to]; !ok && to != p {
				a.Edges = append(a.Edges, edge(ix, p, name, to, false, at, hasService))
			}
		}
	}
	sort.Slice(a.Edges, func(i, j int) bool {
		x, y := a.Edges[i], a.Edges[j]
		if (x.Warn == "") != (y.Warn == "") {
			return x.Warn != ""
		}
		if x.From != y.From {
			return x.From < y.From
		}
		return x.To < y.To
	})
	return a
}

type place struct {
	file string
	line int
}

func addImports(into map[string]place, file string, src []byte, internal func(string) bool) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ImportsOnly)
	if err != nil || f == nil {
		return
	}
	for _, im := range f.Imports {
		p, err := strconv.Unquote(im.Path.Value)
		if err != nil || !internal(p) {
			continue
		}
		if _, ok := into[p]; !ok {
			into[p] = place{file: file, line: fset.Position(im.Pos()).Line}
		}
	}
}

func edge(ix *index.Index, from, fromName, to string, added bool, at place, hasService bool) EdgeChange {
	toName := to[strings.LastIndex(to, "/")+1:]
	if pk := ix.Packages[to]; pk != nil {
		toName = pk.Name
	}
	e := EdgeChange{From: from, To: to, FromName: fromName, ToName: toName, FromLayer: explain.LayerOf(from, fromName), ToLayer: explain.LayerOf(to, toName),
		Added: added, File: at.file, Line: at.line}
	if !added {
		return e
	}
	rf, okf := rank[e.FromLayer]
	rt, okt := rank[e.ToLayer]
	switch {
	case okf && okt && rf > rt && e.FromLayer != "Entry":
		e.Warn = e.FromLayer + " now depends on " + e.ToLayer + ", a layer above it"
	case e.FromLayer == "API" && e.ToLayer == "Repository" && hasService:
		e.Warn = "API reaches the store directly, past the Service layer"
	}
	return e
}

// onlyTouched says whether every Go file the index has of p is one the
// changes added: a package the changes made, which the index has read.
func onlyTouched(p *index.Package, touched map[string]FileChanges) bool {
	if p == nil {
		return true
	}
	for _, f := range p.Files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		if t, ok := touched[filepath.Clean(f)]; !ok || t.Status != "A" {
			return false
		}
	}
	return true
}

func isExported(name string) bool { return name != "" && name[0] >= 'A' && name[0] <= 'Z' }
