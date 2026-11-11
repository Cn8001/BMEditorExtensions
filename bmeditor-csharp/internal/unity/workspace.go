// Package unity knows a Unity project's and a UPM package's files: which
// folder is what, assembly definitions, .meta files and their GUIDs,
// Unity's Editor.log, and new scripts.
package unity

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Workspace is what the open folder is.
type Workspace struct {
	Root         string    `json:"root"`
	Kind         string    `json:"kind"` // unity, upm, folder
	UnityVersion string    `json:"unityVersion,omitempty"`
	Packages     []Package `json:"packages"`
	// RootNamespace is the project's own (Project Settings, Editor), used
	// when no assembly definition names one.
	RootNamespace string `json:"rootNamespace,omitempty"`
}

// Package is a UPM package whose files are here: the folder itself, one
// embedded in Packages/, or a local one the manifest names.
type Package struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Version     string `json:"version,omitempty"`
	Dir         string `json:"dir"`
	Manifest    string `json:"manifest"` // its package.json
}

var upmName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*\.[a-z0-9._-]+$`)

// IsUPMName says whether a package.json name is a UPM one: lower case, in
// reverse-domain form, com.company.thing.
func IsUPMName(name string) bool { return upmName.MatchString(name) && len(name) <= 214 }

// Detect says what root is.
func Detect(root string) Workspace {
	w := Workspace{Root: root, Kind: "folder", Packages: []Package{}}
	if v := projectVersion(root); v != "" {
		w.Kind, w.UnityVersion = "unity", v
		w.RootNamespace = settingValue(filepath.Join(root, "ProjectSettings", "EditorSettings.asset"), "m_ProjectGenerationRootNamespace")
		entries, _ := os.ReadDir(filepath.Join(root, "Packages"))
		for _, e := range entries {
			if e.IsDir() {
				if p, ok := readPackage(filepath.Join(root, "Packages", e.Name())); ok {
					w.Packages = append(w.Packages, p)
				}
			}
		}
		w.Packages = append(w.Packages, localPackages(root)...)
	} else if p, ok := readPackage(root); ok {
		w.Kind = "upm"
		w.Packages = append(w.Packages, p)
	} else {
		// A folder of packages and assets, such as a shared plugins checkout.
		_ = walk(root, 3, func(path string, d fs.DirEntry) {
			if d.Name() == "package.json" {
				if p, ok := readPackage(filepath.Dir(path)); ok {
					w.Packages = append(w.Packages, p)
				}
			}
		})
	}
	sort.Slice(w.Packages, func(i, j int) bool { return w.Packages[i].Name < w.Packages[j].Name })
	return w
}

// projectVersion is the Unity version ProjectSettings/ProjectVersion.txt
// names, "" when root is not a Unity project.
func projectVersion(root string) string {
	return settingValue(filepath.Join(root, "ProjectSettings", "ProjectVersion.txt"), "m_EditorVersion")
}

// settingValue reads "key: value" from a Unity YAML file.
func settingValue(path, key string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), key+":"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func readPackage(dir string) (Package, bool) {
	path := filepath.Join(dir, "package.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return Package{}, false
	}
	var m struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Version     string `json:"version"`
	}
	if json.Unmarshal(trimBOM(raw), &m) != nil || !IsUPMName(m.Name) {
		return Package{}, false
	}
	return Package{Name: m.Name, DisplayName: m.DisplayName, Version: m.Version, Dir: dir, Manifest: path}, true
}

// localPackages are the "file:" dependencies of Packages/manifest.json
// that are folders outside Packages/.
func localPackages(root string) []Package {
	raw, err := os.ReadFile(filepath.Join(root, "Packages", "manifest.json"))
	if err != nil {
		return nil
	}
	var m struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if json.Unmarshal(trimBOM(raw), &m) != nil {
		return nil
	}
	var out []Package
	for _, v := range m.Dependencies {
		rel, ok := strings.CutPrefix(v, "file:")
		if !ok || strings.HasSuffix(strings.ToLower(rel), ".tgz") {
			continue
		}
		dir := rel
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, "Packages", filepath.FromSlash(rel))
		}
		if p, ok := readPackage(filepath.Clean(dir)); ok {
			out = append(out, p)
		}
	}
	return out
}

func trimBOM(b []byte) []byte {
	return []byte(strings.TrimPrefix(string(b), "\ufeff"))
}

// skipDir is a folder no search goes into: Unity's caches and output, and
// version control.
func skipDir(name string) bool {
	switch strings.ToLower(name) {
	case "library", "temp", "logs", "obj", "bin", "build", "builds", "usersettings", "node_modules", ".git", ".svn", ".vs", ".idea":
		return true
	}
	return false
}

// walk visits the files under root, depth folders deep at most (0 for no
// limit), leaving out the folders skipDir names.
func walk(root string, depth int, fn func(path string, d fs.DirEntry)) error {
	base := strings.Count(filepath.Clean(root), string(filepath.Separator))
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			if depth > 0 && strings.Count(filepath.Clean(path), string(filepath.Separator))-base >= depth {
				return filepath.SkipDir
			}
			return nil
		}
		fn(path, d)
		return nil
	})
}
