package dotnet

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Project is what the Solution Explorer shows of a .csproj.
type Project struct {
	Path            string      `json:"path"`
	Name            string      `json:"name"`
	SDK             string      `json:"sdk,omitempty"` // Microsoft.NET.Sdk, for an SDK-style project
	TargetFramework string      `json:"targetFramework,omitempty"`
	AssemblyName    string      `json:"assemblyName,omitempty"`
	RootNamespace   string      `json:"rootNamespace,omitempty"`
	OutputType      string      `json:"outputType,omitempty"`
	Files           []File      `json:"files"`
	ProjectRefs     []string    `json:"projectReferences,omitempty"` // project files
	PackageRefs     []Reference `json:"packageReferences,omitempty"`
	References      []Reference `json:"references,omitempty"` // assemblies, with their HintPath
	Analyzers       []string    `json:"analyzers,omitempty"`
}

// File is a file of a project. Rel is relative to the project's folder,
// slash-separated, and may start with ../ for a linked file.
type File struct {
	Rel  string `json:"rel"`
	Path string `json:"path"`
	Kind string `json:"kind"` // compile, none, content
}

type Reference struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Path    string `json:"path,omitempty"`
}

type msbuild struct {
	Sdk     string `xml:"Sdk,attr"`
	Imports []struct {
		Sdk string `xml:"Sdk,attr"`
	} `xml:"Import"`
	Sdks []struct {
		Name string `xml:"Name,attr"`
	} `xml:"Sdk"`
	PropertyGroups []struct {
		Props []element `xml:",any"`
	} `xml:"PropertyGroup"`
	ItemGroups []struct {
		Items []item `xml:",any"`
	} `xml:"ItemGroup"`
}

type element struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type item struct {
	XMLName  xml.Name
	Include  string    `xml:"Include,attr"`
	Remove   string    `xml:"Remove,attr"`
	Version  string    `xml:"Version,attr"`
	Children []element `xml:",any"`
}

func (it item) child(name string) string {
	for _, c := range it.Children {
		if strings.EqualFold(c.XMLName.Local, name) {
			return strings.TrimSpace(c.Value)
		}
	}
	return ""
}

// ParseProject reads a .csproj: Unity's (every file listed) as well as an
// SDK-style one (every .cs under its folder unless removed).
func ParseProject(path string) (*Project, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc msbuild
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	dir := filepath.Dir(path)
	p := &Project{Path: path, Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), SDK: doc.Sdk}
	for _, imp := range doc.Imports {
		if p.SDK == "" && imp.Sdk != "" {
			p.SDK = imp.Sdk
		}
	}
	for _, s := range doc.Sdks {
		if p.SDK == "" {
			p.SDK = s.Name
		}
	}
	props := map[string]string{}
	for _, g := range doc.PropertyGroups {
		for _, e := range g.Props {
			props[strings.ToLower(e.XMLName.Local)] = strings.TrimSpace(e.Value)
		}
	}
	p.TargetFramework = first(props["targetframework"], props["targetframeworks"], props["targetframeworkversion"])
	p.AssemblyName = props["assemblyname"]
	p.RootNamespace = props["rootnamespace"]
	p.OutputType = props["outputtype"]

	abs := func(include string) string {
		return filepath.Clean(filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(include, `\`, "/"))))
	}
	files := map[string]File{}
	add := func(path, kind string) {
		if _, ok := files[path]; ok {
			return
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			rel = path
		}
		files[path] = File{Rel: filepath.ToSlash(rel), Path: path, Kind: kind}
	}
	var removes []string
	for _, g := range doc.ItemGroups {
		for _, it := range g.Items {
			include := strings.TrimSpace(it.Include)
			switch strings.ToLower(it.XMLName.Local) {
			case "compile", "none", "content":
				kind := strings.ToLower(it.XMLName.Local)
				if it.Remove != "" && kind == "compile" {
					removes = append(removes, strings.Split(it.Remove, ";")...)
				}
				for _, inc := range strings.Split(include, ";") {
					inc = strings.TrimSpace(inc)
					if inc == "" || strings.Contains(inc, "$(") {
						continue
					}
					if strings.ContainsAny(inc, "*?") {
						for _, f := range glob(dir, inc) {
							add(f, kind)
						}
					} else {
						add(abs(inc), kind)
					}
				}
			case "projectreference":
				if include != "" {
					p.ProjectRefs = append(p.ProjectRefs, abs(include))
				}
			case "packagereference":
				if include != "" {
					p.PackageRefs = append(p.PackageRefs, Reference{Name: include, Version: first(it.Version, it.child("Version"))})
				}
			case "reference":
				if include != "" {
					name, _, _ := strings.Cut(include, ",")
					ref := Reference{Name: strings.TrimSpace(name)}
					if h := it.child("HintPath"); h != "" {
						ref.Path = abs(h)
					}
					p.References = append(p.References, ref)
				}
			case "analyzer":
				if include != "" {
					p.Analyzers = append(p.Analyzers, abs(include))
				}
			}
		}
	}
	if p.SDK != "" && !strings.EqualFold(props["enabledefaultcompileitems"], "false") &&
		!strings.EqualFold(props["enabledefaultitems"], "false") {
		for _, f := range defaultCompile(dir) {
			add(f, "compile")
		}
	}
	for _, r := range removes {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		re := globRegexp(r)
		for path, f := range files {
			if f.Kind == "compile" && re.MatchString(f.Rel) {
				delete(files, path)
			}
		}
	}
	for _, f := range files {
		p.Files = append(p.Files, f)
	}
	sort.Slice(p.Files, func(i, j int) bool { return strings.ToLower(p.Files[i].Rel) < strings.ToLower(p.Files[j].Rel) })
	return p, nil
}

func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// defaultCompile is what an SDK-style project compiles by default: every
// .cs under its folder, leaving out bin, obj, hidden folders, and the
// folders of other projects.
func defaultCompile(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == dir {
				return nil
			}
			name := strings.ToLower(d.Name())
			if name == "bin" || name == "obj" || strings.HasPrefix(name, ".") || name == "node_modules" {
				return filepath.SkipDir
			}
			if other, _ := filepath.Glob(filepath.Join(path, "*.csproj")); len(other) > 0 {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".cs") {
			out = append(out, path)
		}
		return nil
	})
	return out
}

// glob finds the files an MSBuild wildcard names, relative to dir.
func glob(dir, pattern string) []string {
	pattern = strings.ReplaceAll(pattern, `\`, "/")
	re := globRegexp(pattern)
	root := dir
	// Walk from the pattern's fixed leading folders only.
	if i := strings.IndexAny(pattern, "*?"); i > 0 {
		if j := strings.LastIndex(pattern[:i], "/"); j > 0 {
			root = filepath.Join(dir, filepath.FromSlash(pattern[:j]))
		}
	}
	var out []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err == nil && re.MatchString(filepath.ToSlash(rel)) {
			out = append(out, path)
		}
		return nil
	})
	return out
}

// globRegexp turns an MSBuild wildcard into a regexp over slash paths:
// ** is any folders, * any name part, ? one character.
func globRegexp(pattern string) *regexp.Regexp {
	pattern = strings.TrimPrefix(strings.ReplaceAll(pattern, `\`, "/"), "./")
	var b strings.Builder
	b.WriteString("(?i)^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c == '*' && i+1 < len(pattern) && pattern[i+1] == '*':
			i++
			if i+1 < len(pattern) && pattern[i+1] == '/' {
				i++
				b.WriteString("(?:.*/)?")
			} else {
				b.WriteString(".*")
			}
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}
