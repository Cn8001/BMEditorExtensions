// Package search answers "which type or function should I use for this?"
// from the index: full-text search over the words in names, fields, doc
// comments and signatures finds candidates, and what the code shows about
// them (who uses them, what fields they have) ranks them and says how
// look-alikes differ. Embeddings, when on, add candidates by meaning.
package search

import (
	"context"
	"math"
	"path"
	"sort"
	"strings"

	"bmeditor-mentor/internal/embed"
	"bmeditor-mentor/internal/index"
	"bmeditor-mentor/internal/store"
)

// Docs are the search documents for every symbol of ix.
func Docs(ix *index.Index) []store.Doc {
	docs := make([]store.Doc, 0, len(ix.Symbols))
	for _, s := range ix.Symbols {
		docs = append(docs, Doc(ix, s))
	}
	return docs
}

// Doc is one symbol's search document, every column split into words.
func Doc(ix *index.Index, s *index.Symbol) store.Doc {
	var fields []string
	for _, f := range s.Fields {
		fields = append(fields, index.Split(f.Name)...)
		fields = append(fields, index.Split(f.Type)...)
	}
	// An interface's methods and a type's methods say what it is for.
	for _, r := range ix.In[s.Key] {
		if r.Kind == index.RelMethodOf || (r.Kind == index.RelFieldOf && len(s.Fields) == 0) {
			if m := ix.Symbols[r.From]; m != nil {
				fields = append(fields, index.Split(m.Name)...)
			}
		}
	}
	seen := map[string]bool{}
	var context []string
	for _, r := range ix.In[s.Key] {
		if r.Kind != index.RelCalls && r.Kind != index.RelCallsDynamic && r.Kind != index.RelReferences && r.Kind != index.RelConstructs {
			continue
		}
		if u := ix.Symbols[r.From]; u != nil && !seen[u.Key] && len(seen) < 30 {
			seen[u.Key] = true
			context = append(context, index.Split(u.Name)...)
		}
	}
	name := index.Split(s.Name)
	if s.Recv != "" {
		name = append(name, index.Split(s.Recv)...)
	}
	return store.Doc{
		Key:     s.Key,
		Name:    strings.Join(name, " "),
		Fields:  strings.Join(fields, " "),
		Doc:     s.Doc,
		Sig:     strings.Join(index.Split(s.Sig), " "),
		Pkg:     strings.Join(append([]string{s.PkgName}, index.Split(s.Pkg)...), " "),
		Context: strings.Join(context, " "),
	}
}

// Options shape a query.
type Options struct {
	Kinds []string // symbol kinds to answer with; all but fields when empty
	Limit int      // 12 when zero
	// QueryVec and Vectors, when given, add candidates by meaning.
	QueryVec []float32
	Vectors  map[string]store.Vector
}

// Result is a query's answer.
type Result struct {
	Query      string      `json:"query"`
	Words      []string    `json:"words"`      // what it looked for
	Expanded   []string    `json:"expanded"`   // related words it also looked for
	Semantic   bool        `json:"semantic"`   // embeddings took part
	Candidates []Candidate `json:"candidates"` // best first
}

// Candidate is a symbol that may be what was asked for.
type Candidate struct {
	Key            string   `json:"key"`
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	Pkg            string   `json:"pkg"`
	PkgName        string   `json:"pkgName"`
	Recv           string   `json:"recv,omitempty"`
	File           string   `json:"file"`
	Line           int      `json:"line"`
	Col            int      `json:"col"`
	Sig            string   `json:"sig"`
	Score          float64  `json:"score"`
	Responsibility string   `json:"responsibility"`
	Derived        bool     `json:"derived"` // the responsibility is worked out from the code, not its doc comment
	Matched        []string `json:"matched"` // where the words were found
	Uses           int      `json:"uses"`
	Tests          int      `json:"tests"`
	UsedIn         []string `json:"usedIn"`  // packages that use it
	Differs        []string `json:"differs"` // how it differs from the look-alikes among the candidates
	Similarity     float64  `json:"similarity,omitempty"`
}

var stop = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a an and are as at be by can could do does for from get how i if in into is it its
		me my need needs of on or our should show so that the their them then there these this to use used using want
		we what when where which who why will with would you your find give tell me thing things one some about want`) {
		stop[w] = true
	}
}

// kindWords are words that ask for a kind of symbol rather than name it.
var kindWords = map[string][]string{
	"struct": {"struct"}, "structs": {"struct"}, "type": {"struct", "type", "interface"}, "types": {"struct", "type", "interface"},
	"interface": {"interface"}, "interfaces": {"interface"},
	"function": {"func", "method"}, "functions": {"func", "method"}, "func": {"func", "method"}, "method": {"method"}, "methods": {"method"},
	"constant": {"const"}, "variable": {"var"},
}

// related are words a codebase often uses for the same idea. They widen
// the search; matching them counts for less.
var related = map[string][]string{
	"login":    {"auth", "signin", "session", "credential", "authenticate"},
	"signin":   {"login", "auth", "session"},
	"auth":     {"login", "authenticate", "authorization", "token", "session"},
	"user":     {"account", "member", "profile"},
	"account":  {"user", "profile"},
	"identity": {"provider", "oauth", "account"},
	"store":    {"repository", "repo", "persist", "save", "db"},
	"persist":  {"save", "store", "repository", "repo", "insert"},
	"save":     {"store", "persist", "insert", "write"},
	"database": {"db", "sql", "store", "repository"},
	"db":       {"database", "sql", "store"},
	"delete":   {"remove", "drop"},
	"remove":   {"delete"},
	"create":   {"new", "make", "insert", "add"},
	"config":   {"settings", "options", "conf"},
	"settings": {"config", "options"},
	"error":    {"err", "failure"},
	"request":  {"handler", "http", "route"},
	"route":    {"handler", "http", "endpoint"},
	"endpoint": {"handler", "route", "http"},
	"http":     {"handler", "request", "route", "server"},
	"log":      {"logger", "logging"},
	"cache":    {"memo", "lru"},
	"token":    {"jwt", "session", "secret"},
	"password": {"hash", "credential", "secret"},
	"message":  {"event", "notification"},
	"queue":    {"job", "worker", "task"},
	"job":      {"task", "worker", "queue"},
	"test":     {"mock", "fake", "fixture"},
}

// Words are the words of a query worth looking for, and the kinds it asks
// for by name.
func Words(q string) (words []string, kinds []string) {
	seen := map[string]bool{}
	for _, raw := range strings.FieldsFunc(q, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_')
	}) {
		for _, w := range index.Split(raw) {
			if k, ok := kindWords[w]; ok {
				kinds = append(kinds, k...)
				continue
			}
			if stop[w] || len(w) < 2 || seen[w] {
				continue
			}
			seen[w] = true
			words = append(words, w)
		}
	}
	return words, kinds
}

// stem folds a word's ending, for comparing what was asked with names.
func stem(w string) string {
	w = strings.ToLower(w)
	switch {
	case len(w) > 5 && strings.HasSuffix(w, "ies"):
		return w[:len(w)-3] + "y"
	case len(w) > 5 && strings.HasSuffix(w, "ing"):
		return w[:len(w)-3]
	case len(w) > 4 && strings.HasSuffix(w, "ed"):
		return w[:len(w)-2]
	case len(w) > 4 && strings.HasSuffix(w, "es") && !strings.HasSuffix(w, "ses"):
		return w[:len(w)-2]
	case len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss"):
		return w[:len(w)-1]
	}
	return w
}

// same says whether two words are the same idea: equal stems, or one the
// start of the other when both are long enough (auth, authenticate).
func same(a, b string) bool {
	a, b = stem(a), stem(b)
	if a == b {
		return true
	}
	if len(a) >= 4 && len(b) >= 4 {
		return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
	}
	return false
}

func anySame(word string, list []string) bool {
	for _, w := range list {
		if same(word, w) {
			return true
		}
	}
	return false
}

// Query answers q from ix, with the store's full-text index.
func Query(ctx context.Context, ix *index.Index, db *store.DB, q string, opt Options) (*Result, error) {
	words, kindHint := Words(q)
	res := &Result{Query: q, Words: words}
	if len(words) == 0 {
		return res, nil
	}
	var expanded []string
	for _, w := range words {
		for _, r := range related[stem(w)] {
			if !anySame(r, words) && !anySame(r, expanded) {
				expanded = append(expanded, r)
			}
		}
	}
	res.Expanded = expanded
	limit := opt.Limit
	if limit <= 0 {
		limit = 12
	}
	hits, err := db.Match(ctx, append(append([]string{}, words...), expanded...), 300)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, k := range opt.Kinds {
		allowed[k] = true
	}
	ok := func(s *index.Symbol) bool {
		if len(allowed) > 0 {
			return allowed[string(s.Kind)]
		}
		return s.Kind != index.KindField
	}
	maxScore := 0.0
	for _, h := range hits {
		maxScore = math.Max(maxScore, h.Score)
	}
	scores := map[string]float64{}
	for _, h := range hits {
		if s := ix.Symbols[h.Key]; s != nil && ok(s) && maxScore > 0 {
			scores[h.Key] = 4 * h.Score / maxScore
		}
	}
	sims := map[string]float64{}
	if len(opt.QueryVec) > 0 && len(opt.Vectors) > 0 {
		res.Semantic = true
		type kv struct {
			k string
			v float64
		}
		var all []kv
		for k, v := range opt.Vectors {
			if s := ix.Symbols[k]; s != nil && ok(s) {
				all = append(all, kv{k, embed.Cosine(opt.QueryVec, v.Vec)})
			}
		}
		sort.Slice(all, func(i, j int) bool { return all[i].v > all[j].v })
		for i, e := range all {
			if i < 40 {
				sims[e.k] = e.v
				if _, ok := scores[e.k]; !ok {
					scores[e.k] = 0
				}
			} else if _, ok := scores[e.k]; ok {
				sims[e.k] = e.v
			}
		}
	}
	wantKinds := map[string]bool{}
	for _, k := range kindHint {
		wantKinds[k] = true
	}
	testAsked := anySame("test", words)
	var cands []Candidate
	for key, base := range scores {
		s := ix.Symbols[key]
		c := candidate(ix, s, words)
		score := base
		nameWords := index.Split(s.Name)
		var fieldWords, docWords []string
		for _, f := range s.Fields {
			fieldWords = append(fieldWords, index.Split(f.Name)...)
		}
		for _, w := range strings.Fields(strings.ToLower(s.Doc)) {
			docWords = append(docWords, strings.Trim(w, ".,;:()\"'`"))
		}
		var inName, inFields, inDoc float64
		for _, w := range words {
			if anySame(w, nameWords) {
				inName++
			}
			if anySame(w, fieldWords) {
				inFields++
			}
			if anySame(w, docWords) {
				inDoc++
			}
		}
		n := float64(len(words))
		score += 3*inName/n + 1.2*inFields/n + 1.2*inDoc/n
		if len(wantKinds) > 0 && wantKinds[string(s.Kind)] {
			score += 1.5
		}
		score += 0.35 * math.Log1p(float64(c.Uses))
		if s.Exported {
			score += 0.3
		}
		if s.Test && !testAsked {
			score -= 2.5
		}
		if s.Generated {
			score -= 0.5
		}
		if sim, ok := sims[key]; ok {
			c.Similarity = sim
			score += 4 * math.Max(0, sim)
		}
		c.Score = math.Round(score*100) / 100
		cands = append(cands, c)
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].Score != cands[j].Score {
			return cands[i].Score > cands[j].Score
		}
		return cands[i].Key < cands[j].Key
	})
	if len(cands) > limit {
		cands = cands[:limit]
	}
	differences(ix, cands)
	res.Candidates = cands
	return res, nil
}

func candidate(ix *index.Index, s *index.Symbol, words []string) Candidate {
	c := Candidate{Key: s.Key, Name: s.Name, Kind: string(s.Kind), Pkg: s.Pkg, PkgName: s.PkgName, Recv: s.Recv,
		File: s.File, Line: s.Line, Col: s.Col, Sig: s.Sig}
	users := map[string]bool{}
	tests := map[string]bool{}
	pkgs := map[string]bool{}
	for _, r := range ix.In[s.Key] {
		switch r.Kind {
		case index.RelMethodOf, index.RelFieldOf, index.RelImplements, index.RelEmbeds:
			continue
		}
		u := ix.Symbols[r.From]
		if u == nil {
			continue
		}
		if u.Test {
			tests[u.Key] = true
			continue
		}
		users[u.Key] = true
		if u.Pkg != s.Pkg {
			pkgs[u.PkgName] = true
		}
	}
	// A type is used through its fields and methods too.
	for _, r := range ix.In[s.Key] {
		if r.Kind != index.RelFieldOf && r.Kind != index.RelMethodOf {
			continue
		}
		for _, rr := range ix.In[r.From] {
			if u := ix.Symbols[rr.From]; u != nil && rr.Kind != index.RelFieldOf && rr.Kind != index.RelMethodOf {
				if u.Test {
					tests[u.Key] = true
				} else if u.Key != s.Key {
					users[u.Key] = true
					if u.Pkg != s.Pkg {
						pkgs[u.PkgName] = true
					}
				}
			}
		}
	}
	c.Uses, c.Tests = len(users), len(tests)
	for p := range pkgs {
		c.UsedIn = append(c.UsedIn, p)
	}
	sort.Strings(c.UsedIn)
	c.Responsibility, c.Derived = Responsibility(ix, s)
	c.Matched = matched(s, words)
	return c
}

// Responsibility says in one line what s is for: its doc comment's first
// sentence, or what the code shows when it has none.
func Responsibility(ix *index.Index, s *index.Symbol) (string, bool) {
	if s.Doc != "" {
		return index.FirstSentence(s.Doc), false
	}
	switch s.Kind {
	case index.KindStruct:
		var names []string
		for i, f := range s.Fields {
			if i == 5 {
				names = append(names, "…")
				break
			}
			names = append(names, f.Name)
		}
		line := "A struct in " + s.PkgName
		if len(names) > 0 {
			line += " with " + strings.Join(names, ", ")
		}
		var makers []string
		for _, r := range ix.In[s.Key] {
			if r.Kind == index.RelReturns || r.Kind == index.RelConstructs {
				if m := ix.Symbols[r.From]; m != nil && !m.Test && !containsStr(makers, m.Name) && len(makers) < 2 {
					makers = append(makers, m.Name)
				}
			}
		}
		if len(makers) > 0 {
			line += "; made by " + strings.Join(makers, ", ")
		}
		return line + ".", true
	case index.KindInterface:
		impls := ix.Implementations(s.Key)
		line := "An interface in " + s.PkgName
		if len(impls) > 0 {
			var names []string
			for i, k := range impls {
				if i == 3 {
					names = append(names, "…")
					break
				}
				names = append(names, path.Ext("." + k)[1:])
			}
			line += ", implemented by " + strings.Join(names, ", ")
		}
		return line + ".", true
	case index.KindMethod:
		return "A method of " + s.Recv + ": " + s.Sig, true
	case index.KindFunc:
		return "A function in " + s.PkgName + ": " + s.Sig, true
	}
	return s.Sig, true
}

func matched(s *index.Symbol, words []string) []string {
	var out []string
	name := index.Split(s.Name)
	for _, w := range words {
		if anySame(w, name) {
			out = append(out, "name: "+w)
			continue
		}
		found := false
		for _, f := range s.Fields {
			if anySame(w, index.Split(f.Name)) {
				out = append(out, "field "+f.Name+": "+w)
				found = true
				break
			}
		}
		if found {
			continue
		}
		for _, d := range strings.Fields(strings.ToLower(s.Doc)) {
			if same(w, strings.Trim(d, ".,;:()\"'`")) {
				out = append(out, "doc: "+w)
				break
			}
		}
	}
	return out
}

// differences says, for each candidate, what sets it apart from the
// others of its sort: the fields only it has, the packages only it is
// used in. That is how UserIdentity and Session tell apart, though both
// carry a user's ID.
func differences(ix *index.Index, cands []Candidate) {
	for i := range cands {
		a := ix.Symbols[cands[i].Key]
		if a == nil {
			continue
		}
		var others []*index.Symbol
		for j := range cands {
			if j == i {
				continue
			}
			if b := ix.Symbols[cands[j].Key]; b != nil && group(b.Kind) == group(a.Kind) {
				others = append(others, b)
			}
		}
		if len(others) == 0 {
			continue
		}
		if len(a.Fields) > 0 {
			var only []string
			for _, f := range a.Fields {
				shared := false
				for _, b := range others {
					for _, g := range b.Fields {
						if g.Name == f.Name {
							shared = true
						}
					}
				}
				if !shared {
					only = append(only, f.Name)
				}
			}
			if len(only) > 0 && len(only) < len(a.Fields) {
				cands[i].Differs = append(cands[i].Differs, "Only one here with "+strings.Join(cap3(only), ", ")+".")
			}
			var sharedWith []string
			for _, b := range others {
				for _, f := range a.Fields {
					for _, g := range b.Fields {
						if g.Name == f.Name && g.Type == f.Type && !containsStr(sharedWith, b.Name) {
							sharedWith = append(sharedWith, b.Name)
						}
					}
				}
			}
			if len(sharedWith) > 0 {
				cands[i].Differs = append(cands[i].Differs, "Shares fields with "+strings.Join(cap3(sharedWith), ", ")+".")
			}
		}
		if a.Kind == index.KindInterface {
			if n := len(ix.Implementations(a.Key)); n > 0 {
				cands[i].Differs = append(cands[i].Differs, plural(n, "implementation")+".")
			}
		}
	}
}

func group(k index.Kind) string {
	switch k {
	case index.KindStruct, index.KindType, index.KindInterface:
		return "type"
	case index.KindFunc, index.KindMethod:
		return "func"
	}
	return string(k)
}

func cap3(list []string) []string {
	if len(list) > 3 {
		return append(list[:3:3], "…")
	}
	return list
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
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
