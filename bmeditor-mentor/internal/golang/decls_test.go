package golang

import (
	"os"
	"testing"

	"bmeditor-mentor/internal/index"
)

// Every declaration the loader reads hashes the same read from source
// alone: that is how an older version of a file is compared with now.
func TestDecls_MatchTheLoader(t *testing.T) {
	_, ix := build(t, fixture(t))
	checked := 0
	for file, syms := range ix.ByFile {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		pkg := syms[0].Pkg
		decls, err := Decls(pkg, src)
		if err != nil {
			t.Fatal(err)
		}
		byKey := map[string]Decl{}
		for _, d := range decls {
			byKey[d.Key] = d
		}
		for _, s := range syms {
			d, ok := byKey[s.Key]
			if !ok {
				t.Errorf("%s: not read from source", s.Key)
				continue
			}
			if d.Hash != s.Hash || d.Kind != s.Kind || d.Line != s.Line {
				t.Errorf("%s: source %+v, loader %s %s line %d", s.Key, d, s.Hash, s.Kind, s.Line)
			}
			checked++
		}
		if len(decls) != len(syms) {
			t.Errorf("%s: %d from source, %d from the loader", file, len(decls), len(syms))
		}
	}
	if checked < 40 {
		t.Errorf("only %d checked", checked)
	}
	_ = index.KindFunc
}
