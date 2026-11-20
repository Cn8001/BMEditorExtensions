package index

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Index is everything read from a repository at one moment. It is not
// changed once built: a new one replaces it, so readers need no lock.
type Index struct {
	Root     string
	Modules  []Module
	Packages map[string]*Package // by import path
	Symbols  map[string]*Symbol  // by key
	Out      map[string][]Relation
	In       map[string][]Relation
	Routes   []Route
	ByFile   map[string][]*Symbol // in order of line
	Built    time.Time
	Errors   []string
}

// Assemble joins packages into an index: it keeps the relations whose
// both ends are here and works out which types implement which
// interfaces.
func Assemble(root string, mods []Module, pkgs map[string]*Package) *Index {
	ix := &Index{
		Root: root, Modules: mods, Packages: pkgs,
		Symbols: map[string]*Symbol{}, Out: map[string][]Relation{}, In: map[string][]Relation{},
		ByFile: map[string][]*Symbol{}, Built: time.Now(),
	}
	paths := make([]string, 0, len(pkgs))
	for p := range pkgs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, path := range paths {
		p := pkgs[path]
		for _, s := range p.Symbols {
			ix.Symbols[s.Key] = s
			ix.ByFile[s.File] = append(ix.ByFile[s.File], s)
		}
		ix.Routes = append(ix.Routes, p.Routes...)
		for _, e := range p.Errors {
			ix.Errors = append(ix.Errors, path+": "+e)
		}
	}
	for _, list := range ix.ByFile {
		sort.Slice(list, func(i, j int) bool { return list[i].Line < list[j].Line })
	}
	type dedupe struct {
		from, to string
		kind     RelKind
		file     string
		line     int
	}
	seen := map[dedupe]bool{}
	add := func(r Relation) {
		if ix.Symbols[r.From] == nil || ix.Symbols[r.To] == nil || r.From == r.To {
			return
		}
		d := dedupe{r.From, r.To, r.Kind, r.File, r.Line}
		if seen[d] {
			return
		}
		seen[d] = true
		ix.Out[r.From] = append(ix.Out[r.From], r)
		ix.In[r.To] = append(ix.In[r.To], r)
	}
	for _, path := range paths {
		for _, r := range pkgs[path].Relations {
			add(r)
		}
	}
	for _, r := range ix.implements() {
		add(r)
	}
	return ix
}

// implements compares method sets by name and signature, written with
// full import paths, so it holds across packages read separately.
func (ix *Index) implements() []Relation {
	var ifaces, types []*Symbol
	for _, s := range ix.Symbols {
		switch {
		case s.Kind == KindInterface && len(s.IfaceMethods) > 0:
			ifaces = append(ifaces, s)
		case s.Kind != KindInterface && len(s.MethodSet) > 0:
			types = append(types, s)
		}
	}
	var out []Relation
	for _, t := range types {
		set := make(map[string]bool, len(t.MethodSet))
		for _, m := range t.MethodSet {
			set[m] = true
		}
		for _, i := range ifaces {
			ok := true
			for _, m := range i.IfaceMethods {
				if !set[m] {
					ok = false
					break
				}
			}
			if ok {
				out = append(out, Relation{From: t.Key, To: i.Key, Kind: RelImplements, File: t.File, Line: t.Line, Col: t.Col, Certainty: Verified})
			}
		}
	}
	return out
}

// At is the innermost symbol whose declaration holds line in file:
// a method rather than its type, a field rather than its struct.
func (ix *Index) At(file string, line int) *Symbol {
	var best *Symbol
	for _, s := range ix.ByFile[file] {
		if line >= s.Start && line <= s.EndLine {
			if best == nil || s.EndLine-s.Start < best.EndLine-best.Start {
				best = s
			}
		}
	}
	return best
}

// Implementations are the types that implement interface key.
func (ix *Index) Implementations(key string) []string {
	var out []string
	for _, r := range ix.In[key] {
		if r.Kind == RelImplements {
			out = append(out, r.From)
		}
	}
	sort.Strings(out)
	return out
}

// Source reads one language's code. It keeps what it read, and reads
// again only the folders that changed.
type Source interface {
	Language() string
	Full(ctx context.Context) error
	Changed() (dirs []string, full bool)
	Update(ctx context.Context, dirs []string) error
	Packages() map[string]*Package
	Modules() []Module
	Errors() []string // what stopped it reading, such as no go.mod
}

// Merge assembles the packages of every source into one index.
func Merge(root string, sources ...Source) *Index {
	pkgs := map[string]*Package{}
	var mods []Module
	var errs []string
	for _, src := range sources {
		for k, p := range src.Packages() {
			if p.Language == "" {
				p.Language = src.Language()
			}
			pkgs[k] = p
		}
		mods = append(mods, src.Modules()...)
		errs = append(errs, src.Errors()...)
	}
	ix := Assemble(root, mods, pkgs)
	ix.Errors = append(errs, ix.Errors...)
	return ix
}

// KeyJoin makes a symbol's key from its package, owner and name.
func KeyJoin(pkg, recv, name string) string {
	if recv != "" {
		return pkg + "." + recv + "." + name
	}
	return pkg + "." + name
}

// PackageOf is the package path in a key: everything before the first
// dot after the last slash.
func PackageOf(key string) string {
	slash := strings.LastIndex(key, "/")
	dot := strings.Index(key[slash+1:], ".")
	if dot < 0 {
		return key
	}
	return key[:slash+1+dot]
}

// Split breaks an identifier into its words: "UserIdentityID" is user,
// identity, id; "provider_name" is provider, name.
func Split(s string) []string {
	var words []string
	var cur []rune
	rs := []rune(s)
	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	for i, r := range rs {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
		case unicode.IsUpper(r):
			// A new word starts at an upper-case letter after a lower one,
			// or before a lower one at the end of an acronym (HTTPServer).
			if len(cur) > 0 && (unicode.IsLower(rs[i-1]) || unicode.IsDigit(rs[i-1]) || (i+1 < len(rs) && unicode.IsLower(rs[i+1]))) {
				flush()
			}
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return words
}

// FirstSentence is a doc comment's first sentence, which says what the
// thing is.
func FirstSentence(doc string) string {
	doc = strings.Join(strings.Fields(doc), " ")
	for i := 0; i+1 < len(doc); i++ {
		if doc[i] == '.' && doc[i+1] == ' ' {
			return doc[:i+1]
		}
	}
	return doc
}
