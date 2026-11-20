package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"bmeditor-mentor/internal/index"
)

// Decl is a declaration read from source alone, without types: enough
// to tell what an older version of a file declared, and whether each
// declaration differs from now.
type Decl struct {
	Key  string     `json:"key"`
	Name string     `json:"name"`
	Recv string     `json:"recv,omitempty"`
	Kind index.Kind `json:"kind"`
	Line int        `json:"line"`
	Hash string     `json:"hash"`
	// Shape is what callers depend on: a function's parameters and
	// results, a field's type, a type's definition head. A change to it
	// can break code that uses the declaration.
	Shape string `json:"shape"`
}

// Decls reads src, a file of package pkgPath, into its declarations, with
// the keys and hashes the loader gives the same code.
func Decls(pkgPath string, src []byte) ([]Decl, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil && f == nil {
		return nil, err
	}
	text := func(n ast.Node) []byte {
		a, b := fset.Position(n.Pos()).Offset, fset.Position(n.End()).Offset
		if a < 0 || b > len(src) || a > b {
			return nil
		}
		return src[a:b]
	}
	var out []Decl
	add := func(recv, name string, kind index.Kind, at token.Pos, node ast.Node, shape ast.Node) {
		if name == "_" || name == "" {
			return
		}
		sh := ""
		if shape != nil {
			sh = strings.Join(strings.Fields(string(text(shape))), " ")
		}
		out = append(out, Decl{Key: index.KeyJoin(pkgPath, recv, name), Name: name, Recv: recv, Kind: kind, Line: fset.Position(at).Line, Hash: hashOf(text(node)), Shape: sh})
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil && len(d.Recv.List) == 1 {
				add(embeddedName(d.Recv.List[0].Type), d.Name.Name, index.KindMethod, d.Name.Pos(), d, d.Type)
			} else if d.Recv == nil {
				add("", d.Name.Name, index.KindFunc, d.Name.Pos(), d, d.Type)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				var node ast.Node = spec
				if len(d.Specs) == 1 {
					node = d
				}
				switch sp := spec.(type) {
				case *ast.TypeSpec:
					kind := index.KindType
					switch t := sp.Type.(type) {
					case *ast.StructType:
						kind = index.KindStruct
						for _, fl := range t.Fields.List {
							for _, n := range fl.Names {
								add(sp.Name.Name, n.Name, index.KindField, n.Pos(), fl, fl.Type)
							}
						}
					case *ast.InterfaceType:
						kind = index.KindInterface
						for _, m := range t.Methods.List {
							for _, n := range m.Names {
								add(sp.Name.Name, n.Name, index.KindMethod, n.Pos(), m, m.Type)
							}
						}
					}
					// A struct's or interface's shape is its members', each
					// their own; another type's is what it is defined as.
					var shape ast.Node
					if kind == index.KindType {
						shape = sp.Type
					}
					add("", sp.Name.Name, kind, sp.Name.Pos(), node, shape)
				case *ast.ValueSpec:
					kind := index.KindVar
					if d.Tok == token.CONST {
						kind = index.KindConst
					}
					for _, n := range sp.Names {
						var shape ast.Node
						if sp.Type != nil {
							shape = sp.Type
						}
						add("", n.Name, kind, n.Pos(), node, shape)
					}
				}
			}
		}
	}
	return out, nil
}
