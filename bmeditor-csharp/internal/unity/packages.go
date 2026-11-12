package unity

import (
	"archive/tar"
	"compress/gzip"
	"debug/pe"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// An assembly of a folder often references one that is not there: the
// Test Framework's UnityEngine.TestRunner and NUnit, which every package
// with tests uses, or an assembly of a package it depends on. Unity takes
// them from its packages; so does externals, from the project's
// Library/PackageCache or from the packages the Unity Editor ships with.

type externals struct {
	w      Workspace
	u      *Install
	byName map[string]Assembly // assemblies of packages, by name
	pkgs   []string            // the editor's package folders read
	tried  map[string]bool     // package names looked for in the editor
	queue  []string            // package names to look for
	dlls   map[string]string   // lower-case file name → a .dll
	local  []string            // the folder's own .dll files
}

func newExternals(w Workspace, u *Install, as Assemblies, local []string) *externals {
	e := &externals{w: w, u: u, byName: map[string]Assembly{}, tried: map[string]bool{}, local: local}
	for _, a := range as.Assemblies {
		if a.External && a.Error == "" {
			if _, ok := e.byName[a.Name]; !ok {
				e.byName[a.Name] = a
			}
		}
	}
	// The packages the folder depends on, and the Test Framework, which
	// Unity adds for tests of its own accord.
	manifests := []string{}
	for _, p := range w.Packages {
		manifests = append(manifests, p.Manifest)
	}
	if w.Kind == "unity" {
		manifests = append(manifests, filepath.Join(w.Root, "Packages", "manifest.json"))
	}
	for _, m := range manifests {
		e.queue = append(e.queue, dependencies(m)...)
	}
	e.queue = append(e.queue, "com.unity.test-framework", "com.unity.ext.nunit")
	return e
}

// dependencies are the packages a package.json or manifest.json names.
func dependencies(manifest string) []string {
	raw, err := os.ReadFile(manifest)
	if err != nil {
		return nil
	}
	var m struct{ Dependencies map[string]string }
	if json.Unmarshal(trimBOM(raw), &m) != nil {
		return nil
	}
	out := make([]string, 0, len(m.Dependencies))
	for n := range m.Dependencies {
		// Modules are in the editor's own assemblies already.
		if !strings.HasPrefix(n, "com.unity.modules.") && !strings.HasPrefix(n, "com.unity.feature.") {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// assembly finds an assembly by name in the packages, reading the
// editor's ones, and those they depend on, as long as it is not found.
func (e *externals) assembly(name string) (Assembly, bool) {
	for {
		if a, ok := e.byName[name]; ok {
			return a, true
		}
		if !e.next() {
			return Assembly{}, false
		}
	}
}

// next reads one more of the editor's packages; false when none is left.
func (e *externals) next() bool {
	for len(e.queue) > 0 {
		n := e.queue[0]
		e.queue = e.queue[1:]
		if e.tried[n] {
			continue
		}
		e.tried[n] = true
		dir, ok := editorPackage(e.u, n)
		if !ok {
			continue
		}
		e.pkgs = append(e.pkgs, dir)
		e.dlls = nil
		for _, a := range ScanAssemblies(dir).Assemblies {
			if _, seen := e.byName[a.Name]; !seen && a.Error == "" {
				a.External = true
				e.byName[a.Name] = a
			}
		}
		e.queue = append(e.queue, dependencies(filepath.Join(dir, "package.json"))...)
		return true
	}
	return false
}

// dll finds a precompiled assembly by file name (nunit.framework.dll): in
// the folder, the packages, or the editor's Managed folder (Mono.Cecil).
func (e *externals) dll(file string) string {
	key := strings.ToLower(file)
	for {
		if e.dlls == nil {
			e.indexDLLs()
		}
		if p, ok := e.dlls[key]; ok {
			return p
		}
		if !e.next() {
			return ""
		}
	}
}

func (e *externals) indexDLLs() {
	e.dlls = map[string]string{}
	add := func(p string) {
		k := strings.ToLower(filepath.Base(p))
		if _, ok := e.dlls[k]; !ok {
			e.dlls[k] = p
		}
	}
	for _, p := range e.local {
		add(p)
	}
	roots := append([]string{}, e.pkgs...)
	if cache := filepath.Join(e.w.Root, "Library", "PackageCache"); isDir(cache) {
		roots = append(roots, cache)
	}
	for _, r := range roots {
		_ = filepath.WalkDir(r, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path != r && ignoredName(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".dll") {
				add(path)
			}
			return nil
		})
	}
	if e.u != nil {
		managed, _ := filepath.Glob(filepath.Join(e.u.Data, "Managed", "*.dll"))
		for _, p := range managed {
			add(p)
		}
	}
}

// editorPackage is the folder of a package the editor ships: one of
// Resources/PackageManager/BuiltInPackages (Unity 6), or a .tgz of
// Resources/PackageManager/Editor (2022 and before) unpacked, once, into
// the user's cache.
func editorPackage(u *Install, name string) (string, bool) {
	if u == nil {
		return "", false
	}
	pm := filepath.Join(u.Data, "Resources", "PackageManager")
	if dir := filepath.Join(pm, "BuiltInPackages", name); isFile(filepath.Join(dir, "package.json")) {
		return dir, true
	}
	tgzs, _ := filepath.Glob(filepath.Join(pm, "Editor", name+"-*.tgz"))
	best := ""
	for _, t := range tgzs {
		v := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(t), name+"-"), ".tgz")
		if v == "" || v[0] < '0' || v[0] > '9' { // com.unity.test-framework.performance-3.5.0
			continue
		}
		if best == "" || compareVersions(v, strings.TrimSuffix(strings.TrimPrefix(filepath.Base(best), name+"-"), ".tgz")) > 0 {
			best = t
		}
	}
	if best == "" {
		return "", false
	}
	home, err := Home()
	if err != nil {
		return "", false
	}
	dir := filepath.Join(home, "packages", strings.TrimSuffix(filepath.Base(best), ".tgz"))
	if isFile(filepath.Join(dir, "package.json")) {
		return dir, true
	}
	if err := unpack(best, dir); err != nil {
		return "", false
	}
	return dir, true
}

// unpack writes what a language server needs of a package .tgz (its
// scripts, assembly definitions, assemblies and package.json) into dir.
// It unpacks into a folder of its own first, so that two programs doing it
// at once do not see half a package.
func unpack(tgz, dir string) error {
	f, err := os.Open(tgz)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.part-%d", dir, os.Getpid())
	_ = os.RemoveAll(tmp)
	defer os.RemoveAll(tmp)
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.ToSlash(filepath.Clean(h.Name))
		rel, ok := strings.CutPrefix(name, "package/")
		if !ok || !wanted(rel) {
			continue
		}
		to := filepath.Join(tmp, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		out, err := os.Create(to)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, tr)
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp, dir); err != nil && !isFile(filepath.Join(dir, "package.json")) {
		return err
	}
	return nil
}

// wanted says whether a file of a package, by its path in it, is one a
// language server reads: not in a folder Unity leaves out (Samples~,
// .Runtime), and a script, an assembly or its definition.
func wanted(rel string) bool {
	if strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
		return false
	}
	parts := strings.Split(rel, "/")
	for _, p := range parts[:len(parts)-1] {
		if ignoredName(p) {
			return false
		}
	}
	if rel == "package.json" {
		return true
	}
	l := strings.ToLower(rel)
	for _, ext := range []string{".cs", ".asmdef", ".asmref", ".dll", ".asmdef.meta"} {
		if strings.HasSuffix(l, ext) {
			return true
		}
	}
	return false
}

// autoReferenced says whether Unity gives a .dll of the folder to every
// assembly that does not override its references: a .NET assembly (not a
// native plugin) whose .meta does not ask for it to be referenced
// explicitly.
func autoReferenced(dll string) bool {
	if meta, err := os.ReadFile(dll + ".meta"); err == nil && strings.Contains(string(meta), "isExplicitlyReferenced: 1") {
		return false
	}
	f, err := pe.Open(dll)
	if err != nil {
		return false
	}
	defer f.Close()
	const clr = 14 // IMAGE_DIRECTORY_ENTRY_COM_DESCRIPTOR
	switch h := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		return h.NumberOfRvaAndSizes > clr && h.DataDirectory[clr].VirtualAddress != 0
	case *pe.OptionalHeader64:
		return h.NumberOfRvaAndSizes > clr && h.DataDirectory[clr].VirtualAddress != 0
	}
	return false
}

// scripts are the scripts of an assembly outside the folder: those under
// its .asmdef, but not under another one's.
func scripts(a Assembly) []string {
	root := filepath.Dir(a.Path)
	var out []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			if ignoredName(d.Name()) {
				return filepath.SkipDir
			}
			if m, _ := filepath.Glob(filepath.Join(path, "*.asmdef")); len(m) > 0 {
				return filepath.SkipDir
			}
			if m, _ := filepath.Glob(filepath.Join(path, "*.asmref")); len(m) > 0 {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".cs") {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out
}
