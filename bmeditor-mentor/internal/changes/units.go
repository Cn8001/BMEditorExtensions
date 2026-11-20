package changes

import (
	"sort"
	"strconv"
	"strings"

	"bmeditor-mentor/internal/explain"
	"bmeditor-mentor/internal/gitx"
	"bmeditor-mentor/internal/index"
)

// Unit is changes that belong together: declarations of one package
// that use each other, a type with its methods and fields, and the tests
// of them.
type Unit struct {
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	Pkg      string          `json:"pkg"`
	PkgName  string          `json:"pkgName"`
	Layer    string          `json:"layer,omitempty"` // guessed from the package's name
	Changes  []Change        `json:"changes"`
	Files    []string        `json:"files"`
	PlugsIn  []explain.Ref   `json:"plugsIn"`  // code that did not change and uses what did: where it is wired in
	ReliesOn []explain.Ref   `json:"reliesOn"` // code that did not change and what changed uses
	Tests    []explain.Ref   `json:"tests"`    // tests that reach it
	Routes   []index.Route   `json:"routes"`
	Impact   explain.Impact  `json:"impact"`
	Risks    []string        `json:"risks"`
	Lines    [2]int          `json:"lines"` // added and deleted, from git, for its files
	Flows    []string        `json:"flows"` // changed functions worth tracing: entry points first
	Stats    map[string]int  `json:"stats"` // added, modified, removed
	files    map[string]bool `json:"-"`
}

// Report is everything that changed since a baseline.
type Report struct {
	Since   string            `json:"since"`
	Head    gitx.Head         `json:"head"`
	Units   []Unit            `json:"units"`
	Other   []gitx.FileChange `json:"other"` // files that are not Go code
	Totals  map[string]int    `json:"totals"`
	Lines   [2]int            `json:"lines"`
	Warning string            `json:"warning,omitempty"` // such as an index older than the files
}

// Build groups changes into units and says, for each, how it fits in.
// files are git's view of the changed files, for line counts and the
// files that are not Go; none when the baseline is a snapshot.
func Build(ix *index.Index, base Baseline, list []Change, files []gitx.FileChange) *Report {
	rep := &Report{Since: base.Label, Head: base.Head, Units: []Unit{}, Other: []gitx.FileChange{}, Totals: map[string]int{"added": 0, "modified": 0, "removed": 0}}
	lines := map[string][2]int{}
	for _, f := range files {
		lines[f.Path] = [2]int{f.Added, f.Deleted}
		rep.Lines[0] += f.Added
		rep.Lines[1] += f.Deleted
		if !strings.HasSuffix(f.Path, ".go") {
			rep.Other = append(rep.Other, f)
		}
	}
	changed := map[string]*Change{}
	for i := range list {
		changed[list[i].Key] = &list[i]
		rep.Totals[list[i].Change]++
	}
	// node is what a change groups by: a method or field by its type.
	node := func(c *Change) string {
		if (c.Kind == string(index.KindMethod) || c.Kind == string(index.KindField)) && c.Recv != "" {
			return index.KeyJoin(c.Pkg, "", c.Recv)
		}
		return c.Key
	}
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		if parent[x] == "" || parent[x] == x {
			parent[x] = x
			return x
		}
		parent[x] = find(parent[x])
		return parent[x]
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	nodes := map[string]bool{}
	pkgOf := map[string]string{}
	for _, c := range changed {
		n := node(c)
		nodes[n] = true
		find(n)
		pkgOf[n] = strings.TrimSuffix(c.Pkg, "_test")
	}
	// Changes of one package that use each other change together; a test
	// joins what it exercises.
	for _, c := range changed {
		a := node(c)
		for _, r := range append(append([]index.Relation{}, ix.Out[c.Key]...), ix.In[c.Key]...) {
			other := r.To
			if other == c.Key {
				other = r.From
			}
			oc := changed[other]
			if oc == nil {
				continue
			}
			b := node(oc)
			if pkgOf[a] == pkgOf[b] || c.Test || oc.Test {
				union(a, b)
			}
		}
	}
	groups := map[string][]*Change{}
	for _, c := range changed {
		r := find(node(c))
		groups[r] = append(groups[r], c)
	}
	for _, g := range groups {
		rep.Units = append(rep.Units, unit(ix, g, changed, lines))
	}
	sort.Slice(rep.Units, func(i, j int) bool {
		a, b := rep.Units[i], rep.Units[j]
		if (a.Layer == "") != (b.Layer == "") {
			return a.Layer != ""
		}
		if a.Pkg != b.Pkg {
			return a.Pkg < b.Pkg
		}
		return a.Title < b.Title
	})
	for i := range rep.Units {
		rep.Units[i].ID = strconv.Itoa(i + 1)
	}
	return rep
}

func unit(ix *index.Index, g []*Change, changed map[string]*Change, lines map[string][2]int) Unit {
	sort.Slice(g, func(i, j int) bool {
		if g[i].Test != g[j].Test {
			return !g[i].Test
		}
		if g[i].File != g[j].File {
			return g[i].File < g[j].File
		}
		return g[i].Line < g[j].Line
	})
	u := Unit{Changes: []Change{}, Files: []string{}, PlugsIn: []explain.Ref{}, ReliesOn: []explain.Ref{}, Tests: []explain.Ref{},
		Routes: []index.Route{}, Risks: []string{}, Flows: []string{}, Stats: map[string]int{}, files: map[string]bool{}}
	for _, c := range g {
		u.Changes = append(u.Changes, *c)
		u.Stats[c.Change]++
		if !u.files[c.File] {
			u.files[c.File] = true
			u.Files = append(u.Files, c.File)
		}
	}
	sort.Strings(u.Files)
	for _, f := range u.Files {
		u.Lines[0] += lines[f][0]
		u.Lines[1] += lines[f][1]
	}
	lead := g[0]
	u.Pkg, u.PkgName = strings.TrimSuffix(lead.Pkg, "_test"), lead.PkgName
	if p := ix.Packages[u.Pkg]; p != nil {
		u.PkgName = p.Name
		u.Layer = explain.LayerOf(u.Pkg, p.Name)
	}
	u.Title = title(g)

	seenIn, seenOut, seenTest := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var keys []string
	var untested []string
	for _, c := range g {
		if c.Change == "removed" {
			continue
		}
		keys = append(keys, c.Key)
		reached := false
		for _, r := range ix.In[c.Key] {
			u2 := ix.Symbols[r.From]
			if u2 == nil || r.Kind == index.RelMethodOf || r.Kind == index.RelFieldOf {
				continue
			}
			if u2.Test {
				reached = true
				if !seenTest[u2.Key] {
					seenTest[u2.Key] = true
					u.Tests = append(u.Tests, explain.RefOf(u2))
				}
				continue
			}
			if changed[u2.Key] != nil || seenIn[u2.Key] || r.Kind == index.RelImplements {
				continue
			}
			seenIn[u2.Key] = true
			ref := explain.RefOf(u2)
			ref.Via, ref.Certainty, ref.SiteFile, ref.SiteLine = string(r.Kind), r.Certainty, r.File, r.Line
			u.PlugsIn = append(u.PlugsIn, ref)
		}
		for _, r := range ix.Out[c.Key] {
			t := ix.Symbols[r.To]
			if t == nil || changed[t.Key] != nil || seenOut[t.Key] || r.Kind == index.RelMethodOf || r.Kind == index.RelFieldOf {
				continue
			}
			if r.Kind != index.RelCalls && r.Kind != index.RelCallsDynamic && r.Kind != index.RelConstructs && r.Kind != index.RelImplements {
				continue
			}
			seenOut[t.Key] = true
			ref := explain.RefOf(t)
			ref.Via, ref.Certainty, ref.SiteFile, ref.SiteLine = string(r.Kind), r.Certainty, r.File, r.Line
			u.ReliesOn = append(u.ReliesOn, ref)
		}
		// Tests one call away reach it too.
		if !reached && !c.Test {
			for _, r := range ix.In[c.Key] {
				for _, rr := range ix.In[r.From] {
					if t := ix.Symbols[rr.From]; t != nil && t.Test {
						reached = true
						if !seenTest[t.Key] {
							seenTest[t.Key] = true
							u.Tests = append(u.Tests, explain.RefOf(t))
						}
					}
				}
			}
		}
		if !reached && !c.Test && (c.Kind == string(index.KindFunc) || c.Kind == string(index.KindMethod)) {
			if s := ix.Symbols[c.Key]; s != nil && s.Recv != "" && ix.Symbols[index.KeyJoin(s.Pkg, "", s.Recv)] != nil &&
				ix.Symbols[index.KeyJoin(s.Pkg, "", s.Recv)].Kind == index.KindInterface {
				continue // an interface's method has no body to test
			}
			untested = append(untested, display(c.Ref))
		}
		if s := ix.Symbols[c.Key]; s != nil && !s.Test && (s.Kind == index.KindFunc || s.Kind == index.KindMethod) {
			u.Flows = append(u.Flows, c.Key)
		}
	}
	for _, rt := range ix.Routes {
		if changed[rt.Handler] != nil {
			u.Routes = append(u.Routes, rt)
			continue
		}
		for _, r := range ix.Out[rt.Handler] {
			if changed[r.To] != nil && (r.Kind == index.RelCalls || r.Kind == index.RelCallsDynamic) {
				u.Routes = append(u.Routes, rt)
				break
			}
		}
	}
	u.Impact = explain.ImpactOfAll(ix, keys, 500)
	sortRefs(u.PlugsIn)
	sortRefs(u.ReliesOn)
	sortRefs(u.Tests)
	u.Risks = risks(ix, g, untested)
	// Entry points first: what nothing else changed calls, then the rest.
	sort.SliceStable(u.Flows, func(i, j int) bool {
		return len(callersIn(ix, u.Flows[i], changed)) < len(callersIn(ix, u.Flows[j], changed))
	})
	if len(u.Flows) > 5 {
		u.Flows = u.Flows[:5]
	}
	return u
}

func callersIn(ix *index.Index, key string, changed map[string]*Change) []string {
	var out []string
	for _, r := range ix.In[key] {
		if changed[r.From] != nil && (r.Kind == index.RelCalls || r.Kind == index.RelCallsDynamic) {
			out = append(out, r.From)
		}
	}
	return out
}

// risks are what a reviewer should look at first, from what the code
// shows: a changed shape others use, an interface its implementations
// must follow, something removed, code no test reaches.
func risks(ix *index.Index, g []*Change, untested []string) []string {
	var out []string
	for _, c := range g {
		s := ix.Symbols[c.Key]
		switch c.Change {
		case "removed":
			out = append(out, "Removes "+display(c.Ref)+".")
		case "modified":
			if c.ShapeChanged && s != nil {
				pkgs := map[string]bool{}
				for _, r := range ix.In[c.Key] {
					if u := ix.Symbols[r.From]; u != nil && strings.TrimSuffix(u.Pkg, "_test") != strings.TrimSuffix(s.Pkg, "_test") {
						pkgs[u.PkgName] = true
					}
				}
				line := "Changes the signature of " + display(c.Ref)
				if len(pkgs) > 0 {
					line += ", which " + strconv.Itoa(len(pkgs)) + " other package" + plural(len(pkgs)) + " use"
					if len(pkgs) == 1 {
						line += "s"
					}
				}
				out = append(out, line+".")
			}
		}
		if s != nil && s.Kind == index.KindInterface && c.Change != "added" {
			if n := len(ix.Implementations(s.Key)); n > 0 {
				out = append(out, "Changes interface "+s.Name+": its "+strconv.Itoa(n)+" implementation"+plural(n)+" must still match.")
			}
		}
		if s != nil && s.Kind == index.KindMethod && s.Recv != "" {
			if owner := ix.Symbols[index.KeyJoin(s.Pkg, "", s.Recv)]; owner != nil && owner.Kind == index.KindInterface && c.Change != "removed" {
				if n := len(ix.Implementations(owner.Key)); n > 0 {
					out = append(out, "Changes "+display(c.Ref)+" of an interface: its "+strconv.Itoa(n)+" implementation"+plural(n)+" must still match.")
				}
			}
		}
	}
	if len(untested) > 0 {
		names := untested
		if len(names) > 4 {
			names = append(names[:4:4], "…")
		}
		out = append(out, "No test reaches "+strings.Join(names, ", ")+".")
	}
	return dedupe(out)
}

func title(g []*Change) string {
	var names []string
	seen := map[string]bool{}
	for _, c := range g {
		n := c.Name
		if c.Recv != "" {
			n = c.Recv
			if c.Kind == string(index.KindMethod) {
				n = c.Recv + "." + c.Name
			}
		}
		if c.Kind == string(index.KindField) {
			n = c.Recv
		}
		if seen[n] || (c.Test && len(g) > 1) {
			continue
		}
		seen[n] = true
		names = append(names, n)
	}
	if len(names) == 0 {
		names = append(names, display(g[0].Ref))
	}
	t := strings.Join(first(names, 3), ", ")
	if len(names) > 3 {
		t += " and " + strconv.Itoa(len(names)-3) + " more"
	}
	return t
}

func first(list []string, n int) []string {
	if len(list) > n {
		return list[:n]
	}
	return list
}

func display(r explain.Ref) string {
	if r.Recv != "" {
		return r.Recv + "." + r.Name
	}
	return r.Name
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func dedupe(list []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range list {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func sortRefs(refs []explain.Ref) {
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Pkg != refs[j].Pkg {
			return refs[i].Pkg < refs[j].Pkg
		}
		return refs[i].Name < refs[j].Name
	})
}
