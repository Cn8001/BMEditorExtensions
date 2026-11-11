// Package dotnet reads .NET solutions (.sln, .slnx) and projects (.csproj)
// into a tree for the Solution Explorer: solution folders, projects, their
// files and references.
package dotnet

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Solution is a solution file and what it holds.
type Solution struct {
	Path  string  `json:"path"`
	Name  string  `json:"name"`
	Items []*Item `json:"items"`
}

// Item is a solution folder or a project.
type Item struct {
	Kind     string   `json:"kind"` // "folder" or "project"
	Name     string   `json:"name"`
	Path     string   `json:"path,omitempty"` // a project's file
	Children []*Item  `json:"children,omitempty"`
	Project  *Project `json:"project,omitempty"`
	Error    string   `json:"error,omitempty"` // the project could not be read
	Files    []string `json:"files,omitempty"` // a folder's solution items
	guid     string
}

const folderType = "2150E333-8FDC-42A3-9474-1A3956D46DE8"

var (
	projectLine = regexp.MustCompile(`^Project\("\{([^}]+)\}"\)\s*=\s*"([^"]*)",\s*"([^"]*)",\s*"\{([^}]+)\}"`)
	nestedLine  = regexp.MustCompile(`^\{([^}]+)\}\s*=\s*\{([^}]+)\}`)
)

// Load reads a solution (.sln or .slnx) and each of its projects, or a
// single project file as a solution of one.
func Load(path string) (*Solution, error) {
	var (
		s   *Solution
		err error
	)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".sln":
		s, err = parseSln(path)
	case ".slnx":
		s, err = parseSlnx(path)
	case ".csproj":
		s = &Solution{Path: path, Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
			Items: []*Item{{Kind: "project", Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Path: path}}}
	default:
		return nil, fmt.Errorf("%s is not a solution or a C# project", filepath.Base(path))
	}
	if err != nil {
		return nil, err
	}
	walkItems(s.Items, func(it *Item) {
		if it.Kind != "project" || !strings.EqualFold(filepath.Ext(it.Path), ".csproj") {
			return
		}
		p, err := ParseProject(it.Path)
		if err != nil {
			it.Error = err.Error()
			return
		}
		it.Project = p
	})
	return s, nil
}

func walkItems(items []*Item, fn func(*Item)) {
	for _, it := range items {
		fn(it)
		walkItems(it.Children, fn)
	}
}

// parseSln reads the classic text format: Project(...) lines, and the
// NestedProjects section that puts items in solution folders.
func parseSln(path string) (*Solution, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dir := filepath.Dir(path)
	byGUID := map[string]*Item{}
	var order []*Item
	parent := map[string]string{}
	var current *Item
	inNested, inItems := false, false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		switch {
		case projectLine.MatchString(line):
			m := projectLine.FindStringSubmatch(line)
			it := &Item{Name: m[2], guid: strings.ToUpper(m[4])}
			if strings.EqualFold(m[1], folderType) {
				it.Kind = "folder"
			} else {
				it.Kind = "project"
				it.Path = filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(m[3], `\`, "/")))
			}
			byGUID[it.guid] = it
			order = append(order, it)
			current = it
		case line == "EndProject":
			current = nil
		case strings.HasPrefix(line, "ProjectSection(SolutionItems)"):
			inItems = true
		case line == "EndProjectSection":
			inItems = false
		case inItems && current != nil:
			if k, _, ok := strings.Cut(line, "="); ok {
				current.Files = append(current.Files, filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(strings.TrimSpace(k), `\`, "/"))))
			}
		case strings.HasPrefix(line, "GlobalSection(NestedProjects)"):
			inNested = true
		case line == "EndGlobalSection":
			inNested = false
		case inNested && nestedLine.MatchString(line):
			m := nestedLine.FindStringSubmatch(line)
			parent[strings.ToUpper(m[1])] = strings.ToUpper(m[2])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	s := &Solution{Path: path, Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))}
	for _, it := range order {
		if p, ok := byGUID[parent[it.guid]]; ok && p != it {
			p.Children = append(p.Children, it)
		} else {
			s.Items = append(s.Items, it)
		}
	}
	sortItems(s.Items)
	return s, nil
}

type slnxFolder struct {
	Name     string        `xml:"Name,attr"`
	Projects []slnxProject `xml:"Project"`
	Files    []slnxProject `xml:"File"`
}

type slnxProject struct {
	Path string `xml:"Path,attr"`
}

// parseSlnx reads the XML format: <Folder Name="/a/b/"> holds Projects and
// Files; a folder's parent is the folder whose name is its name's prefix.
func parseSlnx(path string) (*Solution, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Folders  []slnxFolder  `xml:"Folder"`
		Projects []slnxProject `xml:"Project"`
	}
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	dir := filepath.Dir(path)
	abs := func(p string) string { return filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(p, `\`, "/"))) }
	project := func(p slnxProject) *Item {
		return &Item{Kind: "project", Name: strings.TrimSuffix(filepath.Base(abs(p.Path)), filepath.Ext(p.Path)), Path: abs(p.Path)}
	}
	s := &Solution{Path: path, Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))}
	folders := map[string]*Item{}
	// folder makes "/a/b/" and its parents, and returns it.
	var folder func(name string) *Item
	folder = func(name string) *Item {
		name = "/" + strings.Trim(name, "/") + "/"
		if f, ok := folders[name]; ok {
			return f
		}
		trimmed := strings.Trim(name, "/")
		f := &Item{Kind: "folder", Name: trimmed[strings.LastIndex(trimmed, "/")+1:]}
		folders[name] = f
		if i := strings.LastIndex(trimmed, "/"); i >= 0 {
			p := folder(trimmed[:i])
			p.Children = append(p.Children, f)
		} else {
			s.Items = append(s.Items, f)
		}
		return f
	}
	for _, fd := range doc.Folders {
		f := folder(fd.Name)
		for _, p := range fd.Projects {
			f.Children = append(f.Children, project(p))
		}
		for _, file := range fd.Files {
			f.Files = append(f.Files, abs(file.Path))
		}
	}
	for _, p := range doc.Projects {
		s.Items = append(s.Items, project(p))
	}
	sortItems(s.Items)
	return s, nil
}

// sortItems puts folders first, then projects, each by name.
func sortItems(items []*Item) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind == "folder"
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	for _, it := range items {
		sortItems(it.Children)
	}
}

// Find lists the solutions in dir, best first: .sln then .slnx, one named
// like the folder (as Unity names its own) before the others.
func Find(dir string) []string {
	var out []string
	for _, ext := range []string{".sln", ".slnx"} {
		matches, _ := filepath.Glob(filepath.Join(dir, "*"+ext))
		sort.SliceStable(matches, func(i, j int) bool {
			return named(matches[i], dir) && !named(matches[j], dir)
		})
		out = append(out, matches...)
	}
	return out
}

func named(path, dir string) bool {
	return strings.EqualFold(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), filepath.Base(dir))
}

// Candidates are what a folder can open, best first: the one the project
// names (preferred, relative to root or absolute), then Find's solutions,
// then the projects at root, as a solution of one each.
func Candidates(root, preferred string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		key := strings.ToLower(filepath.Clean(p))
		if !seen[key] {
			seen[key] = true
			out = append(out, p)
		}
	}
	if preferred != "" {
		if !filepath.IsAbs(preferred) {
			preferred = filepath.Join(root, filepath.FromSlash(preferred))
		}
		if info, err := os.Stat(preferred); err == nil && !info.IsDir() {
			add(filepath.Clean(preferred))
		}
	}
	for _, p := range Find(root) {
		add(p)
	}
	projects, _ := filepath.Glob(filepath.Join(root, "*.csproj"))
	sort.Strings(projects)
	for _, p := range projects {
		add(p)
	}
	return out
}

// ProjectSetting reads an extension's setting from the .bmproj file in
// root, for a program that does not get the editor's settings (the
// language server's): "csharp.solution", say.
func ProjectSetting(root, key string) string {
	files, _ := filepath.Glob(filepath.Join(root, "*.bmproj"))
	if len(files) != 1 {
		return ""
	}
	raw, err := os.ReadFile(files[0])
	if err != nil {
		return ""
	}
	var p struct {
		Settings map[string]string `json:"settings"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return ""
	}
	return p.Settings[key]
}
