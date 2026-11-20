package explain

import (
	"sort"
	"strings"

	"bmeditor-mentor/internal/index"
)

// Example is one place the project uses a symbol, as it is written there:
// the best way to learn a type or a function is often to read it used well.
type Example struct {
	Snippet
	How  string `json:"how"` // what the code there does with it: builds one, calls it, calls its Save, uses its Name
	Pkg  string `json:"pkg"` // the package of the code that uses it
	Test bool   `json:"test,omitempty"`
}

// Examples are the places that show best how key is used.
type Examples struct {
	Key   string    `json:"key"`
	Name  string    `json:"name"`
	Total int       `json:"total"` // places that use it
	Items []Example `json:"items"`
}

// ExamplesOf picks up to n places that use key, the ones that teach most
// first: code outside the symbol's own methods before code inside them,
// code before tests, a body before a signature, other packages before its
// own, and each from a different function, and from different packages
// while there are some. Nil when key is unknown.
func ExamplesOf(ix *index.Index, key string, n int) *Examples {
	s := ix.Symbols[key]
	if s == nil {
		return nil
	}
	ex := &Examples{Key: key, Name: display(s), Items: []Example{}}
	isType := s.Kind == index.KindStruct || s.Kind == index.KindType || s.Kind == index.KindInterface
	type site struct {
		r     index.Relation
		u     *index.Symbol
		how   string
		score int
	}
	var sites []site
	seenPlace := map[string]bool{}
	for _, r := range uses(ix, s) {
		switch r.Kind {
		case index.RelCalls, index.RelCallsDynamic, index.RelReferences, index.RelConstructs:
		default:
			continue
		}
		u := ix.Symbols[r.From]
		if u == nil || r.File == "" || r.Line == 0 || u.Key == key {
			continue
		}
		place := r.File + ":" + itoa(r.Line)
		if seenPlace[place] {
			continue
		}
		seenPlace[place] = true
		st := site{r: r, u: u, how: how(ix, s, r)}
		if r.File == u.File && r.Line == u.Line && (u.Kind == index.KindFunc || u.Kind == index.KindMethod) {
			// In the signature: it shows the type passed along, not used.
			st.score += 20
			st.how = "takes or returns one"
		}
		if u.Test {
			st.score += 100
		}
		if u.Recv != "" && u.Pkg == s.Pkg && u.Recv == s.Name {
			st.score += 50 // its own methods: they show what it does, not how it is used
		}
		if u.Pkg == s.Pkg {
			st.score += 10
		}
		if isType && r.Kind == index.RelReferences && r.To == s.Key {
			st.score -= 3 // building one or naming the type
		}
		sites = append(sites, st)
	}
	ex.Total = len(sites)
	sort.SliceStable(sites, func(i, j int) bool {
		if sites[i].score != sites[j].score {
			return sites[i].score < sites[j].score
		}
		if sites[i].r.File != sites[j].r.File {
			return sites[i].r.File < sites[j].r.File
		}
		return sites[i].r.Line < sites[j].r.Line
	})
	// One per function, and while it can, one per package first.
	usedFn := map[string]bool{}
	usedPkg := map[string]bool{}
	var picked []site
	for pass := 0; pass < 2 && len(picked) < n; pass++ {
		for _, st := range sites {
			if len(picked) == n {
				break
			}
			if usedFn[st.u.Key] || pass == 0 && usedPkg[st.u.Pkg] {
				continue
			}
			usedFn[st.u.Key] = true
			usedPkg[st.u.Pkg] = true
			picked = append(picked, st)
		}
	}
	files := map[string][]string{}
	for _, st := range picked {
		lines, ok := files[st.r.File]
		if !ok {
			lines = ReadLines(st.r.File)
			files[st.r.File] = lines
		}
		if st.r.Line > len(lines) {
			continue
		}
		first := max(1, st.r.Line-3)
		last := min(len(lines), st.r.Line+3)
		ex.Items = append(ex.Items, Example{
			Snippet: Snippet{File: st.r.File, Line: st.r.Line, First: first, Text: strings.Join(lines[first-1:last], "\n"), In: display(st.u)},
			How:     st.how,
			Pkg:     st.u.PkgName,
			Test:    st.u.Test,
		})
	}
	return ex
}

// how says what the code at r does with s.
func how(ix *index.Index, s *index.Symbol, r index.Relation) string {
	if r.To != s.Key {
		if t := ix.Symbols[r.To]; t != nil {
			if t.Kind == index.KindField {
				return "uses its " + t.Name
			}
			return "calls its " + t.Name
		}
	}
	switch {
	case r.Kind == index.RelConstructs:
		return "makes one"
	case r.Kind == index.RelCalls || r.Kind == index.RelCallsDynamic:
		return "calls it"
	case s.Kind == index.KindStruct:
		return "builds or names one"
	}
	return "uses it"
}

func display(s *index.Symbol) string {
	if s.Recv != "" {
		return s.Recv + "." + s.Name
	}
	return s.Name
}
