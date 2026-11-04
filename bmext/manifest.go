// Package bmext is the format of a bmeditor extension: a folder with a
// bmext.json manifest at its root. Most of what an extension adds is
// declared there (languages, their language servers, settings, project
// templates); code is optional, a frontend script for the editor's window
// and a backend program the editor talks to over stdin and stdout (see
// package host). Package pkgmgr installs extensions from folders, archives
// and git repositories.
package bmext

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ManifestFile is the manifest's name at the root of an extension.
const ManifestFile = "bmext.json"

var ErrInvalid = errors.New("extension manifest is not valid")

type Manifest struct {
	// Name is the extension's id and its folder's name once installed:
	// lower-case letters, digits and dashes.
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"` // an image in the extension

	Languages    []Language    `json:"languages,omitempty"`
	Settings     []Setting     `json:"settings,omitempty"`
	ProjectKinds []ProjectKind `json:"projectKinds,omitempty"`
	Debuggers    []Debugger    `json:"debuggers,omitempty"`
	Frontend     *Frontend     `json:"frontend,omitempty"`
	Backend      *Backend      `json:"backend,omitempty"`
	// Build makes the installable extension from its source, for an install
	// from a git repository. Without it the repository is the extension.
	Build *Build `json:"build,omitempty"`
}

// Language is a language the editor learns: which files are in it, how it
// looks, and the language server that understands it.
type Language struct {
	ID         string   `json:"id"`
	Label      string   `json:"label"`
	Extensions []string `json:"extensions,omitempty"` // with the dot: ".c"
	FileNames  []string `json:"fileNames,omitempty"`  // whole names: "CMakeLists.txt"
	Icon       string   `json:"icon,omitempty"`       // an image in the extension, for its files
	Codicon    string   `json:"codicon,omitempty"`    // or a codicon name
	Color      string   `json:"color,omitempty"`
	// LineComment and BlockComment drive Toggle Comment.
	LineComment  string    `json:"lineComment,omitempty"`
	BlockComment [2]string `json:"blockComment,omitzero"`
	// Grammar names a highlighting grammar the editor carries: c, cpp,
	// csharp, java, kotlin, rust, python, javascript, typescript, sql, lua,
	// ruby, swift, shell, yaml, toml. A frontend script can bring its own.
	Grammar string  `json:"grammar,omitempty"`
	Server  *Server `json:"server,omitempty"`
}

// Server is a language server the editor starts for the language's files:
// a program on PATH, or one the extension carries, written ./bin/name and
// found like a backend (./bin/name-GOOS-GOARCH first).
type Server struct {
	Command []string       `json:"command"`           // program and arguments: ["clangd"] or ["./bin/my-lsp"]
	Install string         `json:"install,omitempty"` // shown when the program is missing
	Options map[string]any `json:"options,omitempty"` // initializationOptions
}

// Debugger is a debug adapter (Debug Adapter Protocol) for languages: the
// editor's F5 starts it for their files. Command is a program on PATH or
// ./bin/name in the extension, as for language servers. Transport stdio
// (the default) talks over stdin and stdout; tcp runs Command with {port}
// in its arguments replaced by a free port and connects there.
type Debugger struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Languages []string `json:"languages"`
	Command   []string `json:"command"`
	Transport string   `json:"transport,omitempty"`
	Install   string   `json:"install,omitempty"`
	// Prepare runs before each session, in the launch's cwd or else the
	// file's folder, to build what is debugged: ["make"]. The ${...} names
	// of Launch are filled in.
	Prepare []string `json:"prepare,omitempty"`
	// Cleanup is files removed when the session ends, ${...} filled in:
	// what Prepare built, such as ${fileDirname}/${fileBasenameNoExtension}.
	// Only a file Prepare made is removed; one there before it stays.
	Cleanup []string `json:"cleanup,omitempty"`
	// Launch is the launch request's arguments. ${workspaceFolder}, ${file},
	// ${fileDirname}, ${fileBasename} and ${fileBasenameNoExtension} are
	// filled in; the project's run arguments and environment are added.
	Launch map[string]any `json:"launch,omitempty"`
}

// Setting is one value the extension reads. Global settings are in the
// editor's Settings, project ones in Project Settings, "both" in each with
// the project's winning.
type Setting struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`  // string (default), secret, boolean, path
	Scope       string `json:"scope,omitempty"` // global (default), project, both
	Default     string `json:"default,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}

// ProjectKind is a kind of project New Project offers.
type ProjectKind struct {
	Kind   string   `json:"kind"`
	Label  string   `json:"label"`
	Title  string   `json:"title"`
	Text   string   `json:"text,omitempty"`
	Icon   string   `json:"icon,omitempty"`
	Folder string   `json:"folder,omitempty"` // the name a new project starts with
	Detect []string `json:"detect,omitempty"` // globs that make a plain folder this kind
	// Template is a folder in the extension copied into a new project.
	// A file's text may hold {{name}}, the project's name.
	Template string            `json:"template,omitempty"`
	Open     []string          `json:"open,omitempty"`     // files a new project opens on
	Settings map[string]string `json:"settings,omitempty"` // project settings it starts with
}

type Frontend struct {
	Script string `json:"script"`          // an ES module; its default export is called with the editor's API
	Style  string `json:"style,omitempty"` // CSS loaded with it
}

// Backend is a program the editor starts on first use and speaks JSON-RPC
// with over stdin and stdout (package host). Command is a path in the
// extension; the editor tries it with -GOOS-GOARCH appended first, and
// .exe on Windows, so one extension can carry a program for each system.
type Backend struct {
	Command string `json:"command"`
}

type Build struct {
	Command []string `json:"command"`          // run at the repository's root
	Output  string   `json:"output,omitempty"` // the folder it makes; default the root
}

var (
	namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	idPattern   = regexp.MustCompile(`^[a-z][a-z0-9_+-]{0,63}$`)
)

// Read loads and checks the manifest of the extension in dir.
func Read(dir string) (Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		return Manifest{}, fmt.Errorf("read %s: %w", ManifestFile, err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w: %w", ManifestFile, ErrInvalid, err)
	}
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Validate checks what the editor relies on. Paths must stay inside the
// extension, so an extension cannot point the editor at other files.
func (m Manifest) Validate() error {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
	}
	if !namePattern.MatchString(m.Name) {
		return bad("name %q must be lower-case letters, digits and dashes", m.Name)
	}
	if strings.TrimSpace(m.Version) == "" {
		return bad("version is empty")
	}
	paths := []string{m.Icon}
	for _, l := range m.Languages {
		if !idPattern.MatchString(l.ID) || l.Label == "" {
			return bad("language %q needs a lower-case id and a label", l.ID)
		}
		for _, e := range l.Extensions {
			if !strings.HasPrefix(e, ".") || len(e) < 2 {
				return bad("language %s: extension %q must start with a dot", l.ID, e)
			}
		}
		if l.Server != nil && len(l.Server.Command) == 0 {
			return bad("language %s: server has no command", l.ID)
		}
		if l.Server != nil {
			if rel, ok := strings.CutPrefix(l.Server.Command[0], "./"); ok {
				paths = append(paths, rel)
			}
		}
		paths = append(paths, l.Icon)
	}
	keys := map[string]bool{}
	for _, s := range m.Settings {
		if !idPattern.MatchString(strings.ToLower(s.Key)) || keys[s.Key] {
			return bad("setting %q needs a unique key", s.Key)
		}
		keys[s.Key] = true
		switch s.Type {
		case "", "string", "secret", "boolean", "path":
		default:
			return bad("setting %s: type %q is not string, secret, boolean or path", s.Key, s.Type)
		}
		switch s.Scope {
		case "", "global", "project", "both":
		default:
			return bad("setting %s: scope %q is not global, project or both", s.Key, s.Scope)
		}
	}
	for _, k := range m.ProjectKinds {
		if !idPattern.MatchString(k.Kind) || k.Kind == "go" || k.Kind == "empty" || k.Title == "" {
			return bad("project kind %q needs its own lower-case id and a title", k.Kind)
		}
		paths = append(paths, k.Icon, k.Template)
		paths = append(paths, k.Open...)
	}
	debuggerIDs := map[string]bool{}
	for _, d := range m.Debuggers {
		if !idPattern.MatchString(d.ID) || debuggerIDs[d.ID] || len(d.Command) == 0 || len(d.Languages) == 0 {
			return bad("debugger %q needs a unique lower-case id, a command and its languages", d.ID)
		}
		debuggerIDs[d.ID] = true
		if d.Transport != "" && d.Transport != "stdio" && d.Transport != "tcp" {
			return bad("debugger %s: transport %q is not stdio or tcp", d.ID, d.Transport)
		}
		if rel, ok := strings.CutPrefix(d.Command[0], "./"); ok {
			paths = append(paths, rel)
		}
	}
	if m.Frontend != nil {
		if m.Frontend.Script == "" {
			return bad("frontend has no script")
		}
		paths = append(paths, m.Frontend.Script, m.Frontend.Style)
	}
	if m.Backend != nil {
		if m.Backend.Command == "" {
			return bad("backend has no command")
		}
		paths = append(paths, m.Backend.Command)
	}
	if m.Build != nil {
		if len(m.Build.Command) == 0 {
			return bad("build has no command")
		}
		paths = append(paths, m.Build.Output)
	}
	for _, p := range paths {
		if p != "" && !Inside(p) {
			return bad("path %q leaves the extension", p)
		}
	}
	return nil
}

// Inside says whether a slash-separated path stays within the folder it is
// relative to.
func Inside(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || filepath.IsAbs(p) {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(p)))
	return clean != ".." && !strings.HasPrefix(clean, "../")
}

// Title is the name to show.
func (m Manifest) Title() string {
	if m.DisplayName != "" {
		return m.DisplayName
	}
	return m.Name
}
