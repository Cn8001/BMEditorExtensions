// index.Package golang is the Go source of the index: it reads a repository's
// modules with go/packages and go/types, so every symbol and relation is
// what the type checker saw.
package golang

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/tools/go/packages"

	"bmeditor-mentor/internal/index"
)

// Options say what to read and what to leave out.
type Options struct {
	BuildTags string   // as for go build -tags
	Excludes  []string // globs on paths relative to the root, such as "gen/**" or "*_mock.go"
}

// Modules finds the go.mod files under root, outer ones first, leaving out
// vendor, testdata, node_modules and hidden folders, as go list does.
func Modules(root string) []index.Module {
	var mods []index.Module
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "vendor" || name == "testdata" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "go.mod" {
			if mp := modulePath(path); mp != "" {
				mods = append(mods, index.Module{Path: mp, Dir: filepath.Dir(path)})
			}
		}
		return nil
	})
	sort.Slice(mods, func(i, j int) bool { return len(mods[i].Dir) < len(mods[j].Dir) })
	return mods
}

func modulePath(gomod string) string {
	b, err := os.ReadFile(gomod)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "module"); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

// Load reads the packages of mod given as patterns relative to its folder
// ("./..." for all of them), test files included.
func Load(ctx context.Context, mod index.Module, all []index.Module, patterns []string, root string, opt Options) ([]*index.Package, error) {
	cfg := &packages.Config{
		Context: ctx,
		Dir:     mod.Dir,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedTypes |
			packages.NeedTypesInfo | packages.NeedSyntax | packages.NeedModule,
		Tests: true,
		// Each module is read on its own, with the versions its go.mod
		// names, whether or not a go.work joins them.
		Env: append(os.Environ(), "GOWORK=off"),
	}
	if t := strings.TrimSpace(opt.BuildTags); t != "" {
		cfg.BuildFlags = []string{"-tags=" + t}
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	// The test variant of a package ("p [p.test]") holds its files and its
	// tests; read it first so each file is read once, with its tests'
	// view of the types.
	sort.SliceStable(pkgs, func(i, j int) bool {
		return strings.Contains(pkgs[i].ID, " [") && !strings.Contains(pkgs[j].ID, " [")
	})
	x := &extractor{root: root, mod: mod, all: all, opt: opt, seen: map[string]bool{}, out: map[string]*index.Package{}}
	for _, p := range pkgs {
		if strings.HasSuffix(p.ID, ".test") || p.Types == nil {
			continue // the generated main of a test binary
		}
		x.pkg(p)
	}
	var out []*index.Package
	for _, p := range x.out {
		sort.Strings(p.Files)
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

type extractor struct {
	root string
	mod  index.Module
	all  []index.Module // every module of the repository: relations between them count
	opt  Options
	seen map[string]bool
	out  map[string]*index.Package
}

func (x *extractor) excluded(file string) bool {
	rel, err := filepath.Rel(x.root, file)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	for _, g := range x.opt.Excludes {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if prefix, ok := strings.CutSuffix(g, "/**"); ok {
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return true
			}
			continue
		}
		if ok, _ := filepath.Match(g, rel); ok {
			return true
		}
		if ok, _ := filepath.Match(g, filepath.Base(rel)); ok && !strings.Contains(g, "/") {
			return true
		}
	}
	return false
}

func (x *extractor) pkg(p *packages.Package) {
	out := x.out[p.PkgPath]
	if out == nil {
		out = &index.Package{Path: p.PkgPath, Language: "go", Name: p.Name, Module: x.mod.Path}
		x.out[p.PkgPath] = out
	}
	for imp := range p.Imports {
		if !contains(out.Imports, imp) {
			out.Imports = append(out.Imports, imp)
		}
	}
	sort.Strings(out.Imports)
	for _, e := range p.Errors {
		if !contains(out.Errors, e.Error()) && len(out.Errors) < 20 {
			out.Errors = append(out.Errors, e.Error())
		}
	}
	for _, f := range p.Syntax {
		name := p.Fset.Position(f.FileStart).Filename
		if name == "" || x.seen[name] || x.excluded(name) {
			continue
		}
		x.seen[name] = true
		out.Files = append(out.Files, name)
		if out.Dir == "" {
			out.Dir = filepath.Dir(name)
		}
		if st, err := os.Stat(name); err == nil && st.ModTime().After(out.Stamp) {
			out.Stamp = st.ModTime()
		}
		if f.Doc != nil && (out.Doc == "" || filepath.Base(name) == "doc.go") {
			out.Doc = strings.TrimSpace(f.Doc.Text())
		}
		src, _ := os.ReadFile(name)
		fx := &fileX{x: x, p: p, out: out, f: f, file: name, src: src,
			test: strings.HasSuffix(name, "_test.go"), generated: ast.IsGenerated(f)}
		fx.decls()
	}
}

// fileX reads one file's declarations and the relations inside them.
type fileX struct {
	x         *extractor
	p         *packages.Package
	out       *index.Package
	f         *ast.File
	file      string
	src       []byte
	test      bool
	generated bool
}

func (fx *fileX) pos(p token.Pos) token.Position { return fx.p.Fset.Position(p) }

func (fx *fileX) text(from, to token.Pos) []byte {
	a, b := fx.pos(from).Offset, fx.pos(to).Offset
	if a < 0 || b > len(fx.src) || a > b {
		return nil
	}
	return fx.src[a:b]
}

func hashOf(b []byte) string {
	sum := sha256.Sum256([]byte(strings.Join(strings.Fields(string(b)), " ")))
	return hex.EncodeToString(sum[:8])
}

// qual writes types of other packages with the package's name, as code
// does: docker.Container, not its whole import path.
func (fx *fileX) qual() types.Qualifier {
	return func(p *types.Package) string {
		if p == fx.p.Types {
			return ""
		}
		return p.Name()
	}
}

func (fx *fileX) symbol(obj types.Object, kind index.Kind, recv string, node ast.Node, doc *ast.CommentGroup) *index.Symbol {
	pos := fx.pos(obj.Pos())
	end := fx.pos(node.End())
	start := fx.pos(node.Pos())
	s := &index.Symbol{
		Key: index.KeyJoin(fx.p.PkgPath, recv, obj.Name()), Name: obj.Name(), Kind: kind,
		Pkg: fx.p.PkgPath, PkgName: fx.p.Name, Recv: recv,
		File: fx.file, Line: pos.Line, Col: pos.Column, EndLine: end.Line, Start: start.Line,
		Hash: hashOf(fx.text(node.Pos(), node.End())), Exported: obj.Exported(),
		Test: fx.test, Generated: fx.generated,
	}
	if doc != nil {
		s.Doc = strings.TrimSpace(doc.Text())
	}
	fx.out.Symbols = append(fx.out.Symbols, s)
	return s
}

func (fx *fileX) decls() {
	for _, d := range fx.f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			fx.funcDecl(d)
		case *ast.GenDecl:
			fx.genDecl(d)
		}
	}
}

func (fx *fileX) funcDecl(d *ast.FuncDecl) {
	obj, _ := fx.p.TypesInfo.Defs[d.Name].(*types.Func)
	if obj == nil {
		return
	}
	sig := obj.Type().(*types.Signature)
	kind, recv := index.KindFunc, ""
	if sig.Recv() != nil {
		kind, recv = index.KindMethod, namedName(sig.Recv().Type())
		if recv == "" {
			return
		}
	}
	s := fx.symbol(obj, kind, recv, d, d.Doc)
	s.Sig = types.ObjectString(obj, fx.qual())
	if res := sig.Results(); res.Len() > 0 && isError(res.At(res.Len()-1).Type()) {
		s.ReturnsError = true
	}
	if recv != "" {
		fx.rel(s.Key, index.KeyJoin(fx.p.PkgPath, "", recv), index.RelMethodOf, d.Name.Pos(), index.Verified)
	}
	// What it returns that is declared here: a constructor, most often.
	for i := 0; i < sig.Results().Len(); i++ {
		if k := fx.typeKey(sig.Results().At(i).Type()); k != "" {
			fx.rel(s.Key, k, index.RelReturns, d.Name.Pos(), index.Verified)
		}
	}
	if d.Type != nil {
		fx.uses(s.Key, d.Type)
	}
	if d.Body != nil {
		fx.body(s, d.Body)
	}
}

func (fx *fileX) genDecl(d *ast.GenDecl) {
	for _, spec := range d.Specs {
		switch sp := spec.(type) {
		case *ast.TypeSpec:
			fx.typeSpec(d, sp)
		case *ast.ValueSpec:
			for _, n := range sp.Names {
				obj := fx.p.TypesInfo.Defs[n]
				if obj == nil || n.Name == "_" {
					continue
				}
				kind := index.KindVar
				if _, ok := obj.(*types.Const); ok {
					kind = index.KindConst
				}
				doc := sp.Doc
				if doc == nil && len(d.Specs) == 1 {
					doc = d.Doc
				}
				var node ast.Node = sp
				if len(d.Specs) == 1 {
					node = d
				}
				s := fx.symbol(obj, kind, "", node, doc)
				s.Sig = types.ObjectString(obj, fx.qual())
				if sp.Type != nil {
					fx.uses(s.Key, sp.Type)
				}
				for _, v := range sp.Values {
					fx.body(s, v)
				}
			}
		}
	}
}

func (fx *fileX) typeSpec(d *ast.GenDecl, sp *ast.TypeSpec) {
	obj, _ := fx.p.TypesInfo.Defs[sp.Name].(*types.TypeName)
	if obj == nil {
		return
	}
	kind := index.KindType
	switch obj.Type().Underlying().(type) {
	case *types.Struct:
		kind = index.KindStruct
	case *types.Interface:
		kind = index.KindInterface
	}
	doc := sp.Doc
	if doc == nil && len(d.Specs) == 1 {
		doc = d.Doc
	}
	var node ast.Node = sp
	if len(d.Specs) == 1 {
		node = d
	}
	s := fx.symbol(obj, kind, "", node, doc)
	switch kind {
	case index.KindStruct:
		s.Sig = "type " + obj.Name() + " struct"
	case index.KindInterface:
		s.Sig = "type " + obj.Name() + " interface"
	default:
		s.Sig = "type " + obj.Name() + " " + types.TypeString(obj.Type().Underlying(), fx.qual())
	}
	named, _ := obj.Type().(*types.Named)
	if named != nil && named.TypeParams().Len() == 0 {
		if it, ok := named.Underlying().(*types.Interface); ok {
			for i := 0; i < it.NumMethods(); i++ {
				m := it.Method(i)
				s.IfaceMethods = append(s.IfaceMethods, m.Name()+types.TypeString(m.Type(), fullQual))
			}
		} else {
			ms := types.NewMethodSet(types.NewPointer(named))
			for i := 0; i < ms.Len(); i++ {
				m := ms.At(i).Obj()
				s.MethodSet = append(s.MethodSet, m.Name()+types.TypeString(m.Type(), fullQual))
			}
		}
	}
	if st, ok := sp.Type.(*ast.StructType); ok {
		fx.structFields(s, st)
		return
	}
	if it, ok := sp.Type.(*ast.InterfaceType); ok {
		fx.ifaceMethods(s, it)
		return
	}
	fx.uses(s.Key, sp.Type)
}

func (fx *fileX) structFields(s *index.Symbol, st *ast.StructType) {
	for _, f := range st.Fields.List {
		typ := types.ExprString(f.Type)
		tag := ""
		if f.Tag != nil {
			tag, _ = strconv.Unquote(f.Tag.Value)
		}
		fx.uses(s.Key, f.Type)
		if len(f.Names) == 0 {
			name := embeddedName(f.Type)
			s.Fields = append(s.Fields, index.Field{Name: name, Type: typ, Embedded: true, Tag: tag})
			if k := fx.typeKey(fx.p.TypesInfo.TypeOf(f.Type)); k != "" {
				fx.rel(s.Key, k, index.RelEmbeds, f.Type.Pos(), index.Verified)
			}
			continue
		}
		for _, n := range f.Names {
			s.Fields = append(s.Fields, index.Field{Name: n.Name, Type: typ, Tag: tag})
			obj := fx.p.TypesInfo.Defs[n]
			if obj == nil {
				continue
			}
			var doc *ast.CommentGroup = f.Doc
			if doc == nil {
				doc = f.Comment
			}
			fs := fx.symbol(obj, index.KindField, s.Name, f, doc)
			fs.Sig = n.Name + " " + typ
			fx.rel(fs.Key, s.Key, index.RelFieldOf, n.Pos(), index.Verified)
		}
	}
}

func (fx *fileX) ifaceMethods(s *index.Symbol, it *ast.InterfaceType) {
	for _, m := range it.Methods.List {
		if len(m.Names) == 0 {
			fx.uses(s.Key, m.Type)
			if k := fx.typeKey(fx.p.TypesInfo.TypeOf(m.Type)); k != "" {
				fx.rel(s.Key, k, index.RelEmbeds, m.Type.Pos(), index.Verified)
			}
			continue
		}
		for _, n := range m.Names {
			obj := fx.p.TypesInfo.Defs[n]
			if obj == nil {
				continue
			}
			doc := m.Doc
			if doc == nil {
				doc = m.Comment
			}
			ms := fx.symbol(obj, index.KindMethod, s.Name, m, doc)
			ms.Sig = types.ObjectString(obj, fx.qual())
			if sig, ok := obj.Type().(*types.Signature); ok && sig.Results().Len() > 0 && isError(sig.Results().At(sig.Results().Len()-1).Type()) {
				ms.ReturnsError = true
			}
			fx.rel(ms.Key, s.Key, index.RelMethodOf, n.Pos(), index.Verified)
			fx.uses(ms.Key, m.Type)
		}
	}
}

// body reads the relations inside a function body or an initializer, and
// counts its branches and the errors it returns.
func (fx *fileX) body(s *index.Symbol, n ast.Node) {
	handled := map[*ast.Ident]bool{}
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			s.Branches++
		case *ast.ReturnStmt:
			if s.ReturnsError && len(n.Results) > 0 {
				last := n.Results[len(n.Results)-1]
				if id, ok := last.(*ast.Ident); !ok || id.Name != "nil" {
					s.ErrReturns++
				}
			}
		case *ast.CallExpr:
			fx.call(s.Key, n, handled)
		case *ast.CompositeLit:
			if k := fx.typeKey(fx.p.TypesInfo.TypeOf(n)); k != "" {
				fx.rel(s.Key, k, index.RelConstructs, n.Pos(), index.Verified)
				owner := strings.TrimPrefix(k, index.PackageOf(k)+".")
				for _, e := range n.Elts {
					kv, ok := e.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if id, ok := kv.Key.(*ast.Ident); ok {
						if v, ok := fx.p.TypesInfo.Uses[id].(*types.Var); ok && v.IsField() && v.Pkg() != nil {
							fx.rel(s.Key, index.KeyJoin(v.Pkg().Path(), owner, v.Name()), index.RelReferences, id.Pos(), index.Verified)
							handled[id] = true
						}
					}
				}
			}
		case *ast.SelectorExpr:
			if sel := fx.p.TypesInfo.Selections[n]; sel != nil && sel.Kind() == types.FieldVal {
				if k := fieldKey(sel); k != "" {
					fx.rel(s.Key, k, index.RelReferences, n.Sel.Pos(), index.Verified)
				}
				handled[n.Sel] = true
			}
		case *ast.Ident:
			if !handled[n] {
				fx.ref(s.Key, n)
			}
		}
		return true
	})
}

// call records what a call reaches, and the route it registers when it
// looks like mux.HandleFunc("/path", handler).
func (fx *fileX) call(from string, c *ast.CallExpr, handled map[*ast.Ident]bool) {
	var id *ast.Ident
	var recvExpr ast.Expr
	switch f := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id, recvExpr = f.Sel, f.X
	case *ast.IndexExpr: // a generic function instantiated: F[T](x)
		if i, ok := f.X.(*ast.Ident); ok {
			id = i
		}
	}
	if id == nil {
		return
	}
	fx.route(from, c, id.Name)
	fn, ok := fx.p.TypesInfo.Uses[id].(*types.Func)
	if !ok {
		return
	}
	handled[id] = true
	fn = fn.Origin()
	key := funcKey(fn)
	if key == "" || !fx.inModule(fn.Pkg()) {
		return
	}
	certainty, kind := index.Verified, index.RelCalls
	if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil && recvExpr != nil {
		if types.IsInterface(sig.Recv().Type()) {
			certainty, kind = index.Dynamic, index.RelCallsDynamic
		}
	}
	fx.rel(from, key, kind, id.Pos(), certainty)
}

var routeMethods = map[string]string{
	"HandleFunc": "", "Handle": "", "Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE",
	"Patch": "PATCH", "Head": "HEAD", "Options": "OPTIONS", "Any": "", "Group": "", "Route": "",
	"GET": "GET", "POST": "POST", "PUT": "PUT", "DELETE": "DELETE", "PATCH": "PATCH",
	"HEAD": "HEAD", "OPTIONS": "OPTIONS", "Mount": "",
}

func (fx *fileX) route(from string, c *ast.CallExpr, name string) {
	method, ok := routeMethods[name]
	if !ok || len(c.Args) < 1 {
		return
	}
	lit, ok := c.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return
	}
	path, err := strconv.Unquote(lit.Value)
	if err != nil {
		return
	}
	// Go 1.22 patterns carry their method: "POST /login".
	if m, p, ok := strings.Cut(path, " "); ok && strings.HasPrefix(p, "/") && strings.ToUpper(m) == m {
		method, path = m, p
	}
	if !strings.HasPrefix(path, "/") {
		return
	}
	r := index.Route{Method: method, Path: path, Via: name, File: fx.file, Line: fx.pos(c.Pos()).Line}
	if len(c.Args) > 1 {
		r.Handler = fx.handlerKey(c.Args[len(c.Args)-1])
	}
	fx.out.Routes = append(fx.out.Routes, r)
}

func (fx *fileX) handlerKey(e ast.Expr) string {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		if fn, ok := fx.p.TypesInfo.Uses[e].(*types.Func); ok {
			return funcKey(fn.Origin())
		}
	case *ast.SelectorExpr:
		if fn, ok := fx.p.TypesInfo.Uses[e.Sel].(*types.Func); ok {
			return funcKey(fn.Origin())
		}
	case *ast.CallExpr: // http.HandlerFunc(h.Login), or a constructor of the handler
		if len(e.Args) == 1 {
			if k := fx.handlerKey(e.Args[0]); k != "" {
				return k
			}
		}
		return fx.handlerKey(e.Fun)
	case *ast.UnaryExpr:
		return fx.handlerKey(e.X)
	case *ast.CompositeLit:
		return fx.typeKey(fx.p.TypesInfo.TypeOf(e))
	}
	return ""
}

func (fx *fileX) ref(from string, id *ast.Ident) {
	obj := fx.p.TypesInfo.Uses[id]
	if obj == nil || obj.Pkg() == nil || !fx.inModule(obj.Pkg()) {
		return
	}
	var key string
	switch o := obj.(type) {
	case *types.Func:
		key = funcKey(o.Origin())
	case *types.TypeName:
		if o.IsAlias() || o.Parent() != o.Pkg().Scope() {
			return
		}
		key = index.KeyJoin(o.Pkg().Path(), "", o.Name())
	case *types.Var:
		if o.IsField() || o.Parent() != o.Pkg().Scope() {
			return // fields come from selections; locals are not symbols
		}
		key = index.KeyJoin(o.Pkg().Path(), "", o.Name())
	case *types.Const:
		if o.Parent() != o.Pkg().Scope() {
			return
		}
		key = index.KeyJoin(o.Pkg().Path(), "", o.Name())
	default:
		return
	}
	if key != "" && key != from {
		fx.rel(from, key, index.RelReferences, id.Pos(), index.Verified)
	}
}

// uses records the types an expression names: a field's type, a
// signature's parameters.
func (fx *fileX) uses(from string, e ast.Expr) {
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if tn, ok := fx.p.TypesInfo.Uses[id].(*types.TypeName); ok && tn.Pkg() != nil && fx.inModule(tn.Pkg()) && !tn.IsAlias() && tn.Parent() == tn.Pkg().Scope() {
				if k := index.KeyJoin(tn.Pkg().Path(), "", tn.Name()); k != from {
					fx.rel(from, k, index.RelReferences, id.Pos(), index.Verified)
				}
			}
		}
		return true
	})
}

func (fx *fileX) rel(from, to string, kind index.RelKind, at token.Pos, certainty string) {
	if from == "" || to == "" {
		return
	}
	p := fx.pos(at)
	fx.out.Relations = append(fx.out.Relations, index.Relation{From: from, To: to, Kind: kind, File: fx.file, Line: p.Line, Col: p.Column, Certainty: certainty})
}

func (fx *fileX) inModule(p *types.Package) bool {
	if p == nil {
		return false
	}
	path := strings.TrimSuffix(p.Path(), "_test")
	for _, m := range append([]index.Module{fx.x.mod}, fx.x.all...) {
		if path == m.Path || strings.HasPrefix(path, m.Path+"/") {
			return true
		}
	}
	return false
}

// typeKey is the key of a type declared at package level, through a
// pointer, a slice or a map's element: what a function returns or a
// literal builds.
func (fx *fileX) typeKey(t types.Type) string {
	for i := 0; i < 4 && t != nil; i++ {
		switch u := t.(type) {
		case *types.Pointer:
			t = u.Elem()
			continue
		case *types.Slice:
			t = u.Elem()
			continue
		case *types.Map:
			t = u.Elem()
			continue
		case *types.Alias:
			t = types.Unalias(u)
			continue
		case *types.Named:
			o := u.Origin().Obj()
			if o.Pkg() == nil || !fx.inModule(o.Pkg()) || o.Parent() != o.Pkg().Scope() {
				return ""
			}
			return index.KeyJoin(o.Pkg().Path(), "", o.Name())
		}
		return ""
	}
	return ""
}

func funcKey(fn *types.Func) string {
	if fn.Pkg() == nil {
		return ""
	}
	sig, _ := fn.Type().(*types.Signature)
	if sig != nil && sig.Recv() != nil {
		r := namedName(sig.Recv().Type())
		if r == "" {
			return ""
		}
		return index.KeyJoin(fn.Pkg().Path(), r, fn.Name())
	}
	if fn.Parent() != fn.Pkg().Scope() {
		return ""
	}
	return index.KeyJoin(fn.Pkg().Path(), "", fn.Name())
}

// fieldKey names the field a selection reaches, through embedded structs:
// the owner is the struct that declares it.
func fieldKey(sel *types.Selection) string {
	t := sel.Recv()
	idx := sel.Index()
	owner := ""
	for i, ix := range idx {
		t = deref(t)
		named, _ := t.(*types.Named)
		st, ok := t.Underlying().(*types.Struct)
		if !ok || ix >= st.NumFields() {
			return ""
		}
		if named != nil {
			owner = named.Origin().Obj().Name()
		} else {
			owner = ""
		}
		f := st.Field(ix)
		if i == len(idx)-1 {
			if owner == "" || f.Pkg() == nil {
				return ""
			}
			return index.KeyJoin(f.Pkg().Path(), owner, f.Name())
		}
		t = f.Type()
	}
	return ""
}

func deref(t types.Type) types.Type {
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		return types.Unalias(p.Elem())
	}
	return t
}

func namedName(t types.Type) string {
	t = deref(t)
	if n, ok := t.(*types.Named); ok {
		return n.Origin().Obj().Name()
	}
	return ""
}

func isError(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() == nil && n.Obj().Name() == "error"
}

func embeddedName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.StarExpr:
		return embeddedName(e.X)
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.Ident:
		return e.Name
	case *ast.IndexExpr:
		return embeddedName(e.X)
	case *ast.IndexListExpr:
		return embeddedName(e.X)
	}
	return types.ExprString(e)
}

func fullQual(p *types.Package) string { return p.Path() }

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Stamp is the newest modification time among dir's .go files and dir
// itself, which changes when a file is added or removed: whether a
// package changed since it was read. Zero when dir holds no .go file.
func Stamp(dir string) time.Time {
	var t time.Time
	entries, _ := os.ReadDir(dir)
	any := false
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			any = true
		}
	}
	if !any {
		return t
	}
	if info, err := os.Stat(dir); err == nil {
		t = info.ModTime()
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().After(t) {
			t = info.ModTime()
		}
	}
	return t
}
