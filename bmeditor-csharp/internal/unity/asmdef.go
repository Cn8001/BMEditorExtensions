package unity

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Assembly is an assembly definition (.asmdef).
type Assembly struct {
	Name               string          `json:"name"`
	Path               string          `json:"path"`
	GUID               string          `json:"guid,omitempty"`
	RootNamespace      string          `json:"rootNamespace,omitempty"`
	References         []Ref           `json:"references"`
	IncludePlatforms   []string        `json:"includePlatforms,omitempty"`
	ExcludePlatforms   []string        `json:"excludePlatforms,omitempty"`
	DefineConstraints  []string        `json:"defineConstraints,omitempty"`
	VersionDefines     []VersionDefine `json:"versionDefines,omitempty"`
	AutoReferenced     bool            `json:"autoReferenced"`
	OverrideReferences bool            `json:"overrideReferences,omitempty"`
	Precompiled        []string        `json:"precompiledReferences,omitempty"`
	OptionalUnity      []string        `json:"optionalUnityReferences,omitempty"`
	AllowUnsafeCode    bool            `json:"allowUnsafeCode,omitempty"`
	NoEngineReferences bool            `json:"noEngineReferences,omitempty"`
	Editor             bool            `json:"editor,omitempty"` // only for the Editor platform
	Test               bool            `json:"test,omitempty"`
	External           bool            `json:"external,omitempty"` // from Library/PackageCache, read-only
	Error              string          `json:"error,omitempty"`
}

// Ref is a reference of an assembly: by name, or "GUID:<guid>" of the
// referenced .asmdef's .meta.
type Ref struct {
	Raw    string `json:"raw"`
	Name   string `json:"name,omitempty"` // the referenced assembly, when found
	Status string `json:"status"`         // ok, external, unresolved
}

type VersionDefine struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Define     string `json:"define"`
}

// Asmref adds its folder to another assembly.
type Asmref struct {
	Path      string `json:"path"`
	Reference string `json:"reference"`
	Target    string `json:"target,omitempty"` // the assembly's name, when found
}

// Assemblies is every assembly definition under a folder and how they
// reference each other.
type Assemblies struct {
	Assemblies []Assembly `json:"assemblies"`
	Asmrefs    []Asmref   `json:"asmrefs"`
	Edges      [][2]int   `json:"edges"`  // from, to: indexes in Assemblies
	Cycles     [][]string `json:"cycles"` // names of assemblies that reference each other in a loop
}

type asmdefFile struct {
	Name                  string          `json:"name"`
	RootNamespace         string          `json:"rootNamespace"`
	References            []string        `json:"references"`
	IncludePlatforms      []string        `json:"includePlatforms"`
	ExcludePlatforms      []string        `json:"excludePlatforms"`
	AllowUnsafeCode       bool            `json:"allowUnsafeCode"`
	OverrideReferences    bool            `json:"overrideReferences"`
	PrecompiledReferences []string        `json:"precompiledReferences"`
	AutoReferenced        *bool           `json:"autoReferenced"`
	DefineConstraints     []string        `json:"defineConstraints"`
	VersionDefines        []VersionDefine `json:"versionDefines"`
	NoEngineReferences    bool            `json:"noEngineReferences"`
	OptionalUnity         []string        `json:"optionalUnityReferences"`
	Reference             string          `json:"reference"` // .asmref
}

// ScanAssemblies reads the assembly definitions under root. In a Unity
// project those of Library/PackageCache are read too, to resolve references
// to registry packages, and marked External.
func ScanAssemblies(root string) Assemblies {
	out := Assemblies{Assemblies: []Assembly{}, Asmrefs: []Asmref{}, Edges: [][2]int{}, Cycles: [][]string{}}
	visit := func(external bool) func(string, fs.DirEntry) {
		return func(path string, d fs.DirEntry) {
			switch strings.ToLower(filepath.Ext(path)) {
			case ".asmdef":
				a := readAssembly(path)
				a.External = external
				out.Assemblies = append(out.Assemblies, a)
			case ".asmref":
				var f asmdefFile
				raw, err := os.ReadFile(path)
				if err == nil && json.Unmarshal(trimBOM(raw), &f) == nil && !external {
					out.Asmrefs = append(out.Asmrefs, Asmref{Path: path, Reference: f.Reference})
				}
			}
		}
	}
	_ = walk(root, 0, func(path string, d fs.DirEntry) {
		if !hiddenFromUnity(path, root) {
			visit(false)(path, d)
		}
	})
	if cache := filepath.Join(root, "Library", "PackageCache"); isDir(cache) {
		_ = filepath.WalkDir(cache, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				visit(true)(path, d)
			}
			return nil
		})
	}
	sort.SliceStable(out.Assemblies, func(i, j int) bool {
		if out.Assemblies[i].External != out.Assemblies[j].External {
			return !out.Assemblies[i].External
		}
		return strings.ToLower(out.Assemblies[i].Name) < strings.ToLower(out.Assemblies[j].Name)
	})
	out.resolve()
	return out
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// hiddenFromUnity says whether Unity leaves path out: something in a
// folder whose name starts with a dot or ends with ~ (Samples~).
func hiddenFromUnity(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, p := range parts[:len(parts)-1] {
		if ignoredName(p) {
			return true
		}
	}
	return false
}

func readAssembly(path string) Assembly {
	a := Assembly{Path: path, Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), References: []Ref{}, AutoReferenced: true}
	a.GUID = MetaGUID(path + ".meta")
	raw, err := os.ReadFile(path)
	if err != nil {
		a.Error = err.Error()
		return a
	}
	var f asmdefFile
	if err := json.Unmarshal(trimBOM(raw), &f); err != nil {
		a.Error = err.Error()
		return a
	}
	if f.Name != "" {
		a.Name = f.Name
	}
	a.RootNamespace = f.RootNamespace
	for _, r := range f.References {
		a.References = append(a.References, Ref{Raw: r})
	}
	a.IncludePlatforms, a.ExcludePlatforms = f.IncludePlatforms, f.ExcludePlatforms
	a.DefineConstraints, a.VersionDefines = f.DefineConstraints, f.VersionDefines
	if f.AutoReferenced != nil {
		a.AutoReferenced = *f.AutoReferenced
	}
	a.OverrideReferences, a.Precompiled, a.OptionalUnity = f.OverrideReferences, f.PrecompiledReferences, f.OptionalUnity
	a.AllowUnsafeCode, a.NoEngineReferences = f.AllowUnsafeCode, f.NoEngineReferences
	a.Editor = len(f.IncludePlatforms) == 1 && f.IncludePlatforms[0] == "Editor"
	for _, r := range append(append([]string{}, f.References...), f.OptionalUnity...) {
		if r == "TestAssemblies" || strings.HasSuffix(r, "TestRunner") || r == "GUID:27619889b8ba8c24980f49ee34dbb44a" || r == "GUID:0acc523941302664db1f4e527237feb3" {
			a.Test = true
		}
	}
	for _, d := range f.DefineConstraints {
		if d == "UNITY_INCLUDE_TESTS" {
			a.Test = true
		}
	}
	return a
}

// resolve finds what each reference names, the edges between the
// assemblies here, and the loops among them.
func (as *Assemblies) resolve() {
	byName, byGUID := map[string]int{}, map[string]int{}
	for i, a := range as.Assemblies {
		if _, ok := byName[a.Name]; !ok {
			byName[a.Name] = i
		}
		if a.GUID != "" {
			byGUID[strings.ToLower(a.GUID)] = i
		}
	}
	find := func(raw string) (int, bool) {
		if g, ok := strings.CutPrefix(raw, "GUID:"); ok {
			i, ok := byGUID[strings.ToLower(strings.TrimSpace(g))]
			return i, ok
		}
		i, ok := byName[raw]
		return i, ok
	}
	for i := range as.Assemblies {
		a := &as.Assemblies[i]
		for j := range a.References {
			r := &a.References[j]
			if k, ok := find(r.Raw); ok {
				r.Name = as.Assemblies[k].Name
				r.Status = "ok"
				if as.Assemblies[k].External {
					r.Status = "external"
				}
				if !a.External && !as.Assemblies[k].External {
					as.Edges = append(as.Edges, [2]int{i, k})
				}
				continue
			}
			r.Status = "unresolved"
			// Unity's own assemblies are not .asmdef files anywhere here.
			if n := r.Raw; strings.HasPrefix(n, "Unity") && !strings.HasPrefix(n, "GUID:") {
				r.Status = "external"
				r.Name = n
			}
		}
	}
	for i := range as.Asmrefs {
		if k, ok := find(as.Asmrefs[i].Reference); ok {
			as.Asmrefs[i].Target = as.Assemblies[k].Name
		}
	}
	as.Cycles = cycles(len(as.Assemblies), as.Edges, func(i int) string { return as.Assemblies[i].Name })
}

// cycles finds the strongly connected components of more than one node, or
// one that references itself (Tarjan).
func cycles(n int, edges [][2]int, name func(int) string) [][]string {
	adj := make([][]int, n)
	self := make([]bool, n)
	for _, e := range edges {
		adj[e[0]] = append(adj[e[0]], e[1])
		if e[0] == e[1] {
			self[e[0]] = true
		}
	}
	index, low := make([]int, n), make([]int, n)
	on := make([]bool, n)
	for i := range index {
		index[i] = -1
	}
	var stack []int
	next := 0
	out := [][]string{}
	var strong func(v int)
	strong = func(v int) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		on[v] = true
		for _, w := range adj[v] {
			if index[w] < 0 {
				strong(w)
				low[v] = min(low[v], low[w])
			} else if on[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] == index[v] {
			var comp []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				on[w] = false
				comp = append(comp, name(w))
				if w == v {
					break
				}
			}
			if len(comp) > 1 || self[v] {
				sort.Strings(comp)
				out = append(out, comp)
			}
		}
	}
	for v := 0; v < n; v++ {
		if index[v] < 0 {
			strong(v)
		}
	}
	return out
}
