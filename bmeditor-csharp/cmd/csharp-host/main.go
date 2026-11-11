// Command csharp-host is the C# extension's backend: the window cannot read
// files, so the Solution Explorer, the assembly graph, the .meta tools,
// new scripts and Unity's compile errors all come from here.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bmeditor-csharp/bmext/host"
	"bmeditor-csharp/internal/dotnet"
	"bmeditor-csharp/internal/unity"
)

func main() {
	s := host.New()
	register(s, &state{})
	host.Run(s)
}

type state struct {
	mu sync.Mutex

	index     *unity.GUIDIndex
	indexRoot string
	indexAt   time.Time

	watching bool
}

// root is the folder the extension works in: the open folder, else the
// project's.
func root(s *host.Server) (string, error) {
	c := s.Context()
	r := c.Folder
	if r == "" {
		r = c.Project.Dir
	}
	if r == "" {
		return "", &host.Error{Code: "NO_FOLDER", Message: "Open a folder first."}
	}
	return r, nil
}

func params[T any](raw json.RawMessage) (T, error) {
	var v T
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &v); err != nil {
			return v, &host.Error{Code: "BAD_PARAMS", Message: err.Error()}
		}
	}
	return v, nil
}

// handler wraps a method that needs the open folder.
func handler[T any](s *host.Server, fn func(ctx context.Context, root string, p T) (any, error)) host.Handler {
	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		r, err := root(s)
		if err != nil {
			return nil, err
		}
		p, err := params[T](raw)
		if err != nil {
			return nil, err
		}
		return fn(ctx, r, p)
	}
}

type none struct{}

func register(s *host.Server, st *state) {
	s.Handle("workspace", handler(s, func(_ context.Context, root string, _ none) (any, error) {
		w := unity.Detect(root)
		sln := dotnet.Candidates(root, projectRelative(s, s.Context().Setting("solution")))
		if sln == nil {
			sln = []string{}
		}
		return map[string]any{"workspace": w, "solutions": sln}, nil
	}))

	s.Handle("solution", handler(s, func(_ context.Context, root string, p struct{ Path string }) (any, error) {
		path := p.Path
		if path == "" {
			found := dotnet.Candidates(root, projectRelative(s, s.Context().Setting("solution")))
			if len(found) == 0 {
				// What the language server works on: the projects of the
				// assembly definitions (see cs-ls).
				if s, err := generatedSolution(root); err == nil && s != nil {
					return s, nil
				}
				return nil, &host.Error{Code: "NO_SOLUTION", Message: "No solution or C# scripts in " + filepath.Base(root) + "."}
			}
			path = found[0]
		}
		return dotnet.Load(path)
	}))

	s.Handle("assemblies", handler(s, func(_ context.Context, root string, _ none) (any, error) {
		return unity.ScanAssemblies(root), nil
	}))

	s.Handle("meta.check", handler(s, func(_ context.Context, root string, _ none) (any, error) {
		roots := unity.AssetRoots(unity.Detect(root))
		if roots == nil {
			roots = []string{}
		}
		return map[string]any{"roots": roots, "report": unity.CheckMeta(roots)}, nil
	}))

	s.Handle("meta.fix", handler(s, func(_ context.Context, root string, p struct{ Fixes []unity.MetaFix }) (any, error) {
		roots := unity.AssetRoots(unity.Detect(root))
		failed := unity.ApplyMeta(roots, p.Fixes)
		st.mu.Lock()
		st.index = nil // GUIDs may have moved
		st.mu.Unlock()
		return map[string]any{"failed": failed}, nil
	}))

	s.Handle("guid.find", handler(s, func(_ context.Context, root string, p struct{ GUID string }) (any, error) {
		guid := strings.ToLower(strings.TrimSpace(p.GUID))
		if len(guid) != 32 || strings.Trim(guid, "0123456789abcdef") != "" {
			return nil, &host.Error{Code: "BAD_GUID", Message: "A GUID is 32 hex digits."}
		}
		path := st.find(root, guid, false)
		if path == "" {
			path = st.find(root, guid, true)
		}
		if path == "" {
			return nil, &host.Error{Code: "NOT_FOUND", Message: "No asset here has GUID " + guid + "."}
		}
		return map[string]string{"path": path}, nil
	}))

	s.Handle("guid.of", handler(s, func(_ context.Context, _ string, p struct{ Path string }) (any, error) {
		g := unity.MetaGUID(p.Path + ".meta")
		if g == "" {
			return nil, &host.Error{Code: "NO_META", Message: filepath.Base(p.Path) + " has no .meta with a GUID."}
		}
		return map[string]string{"guid": g}, nil
	}))

	s.Handle("script.kinds", func(context.Context, json.RawMessage) (any, error) {
		return unity.ScriptKinds, nil
	})

	s.Handle("script.new", handler(s, func(_ context.Context, root string, p struct{ Dir, Name, Kind string }) (any, error) {
		meta := s.Context().Setting("createMeta") != "false"
		path, err := unity.NewScript(unity.Detect(root), p.Dir, strings.TrimSpace(p.Name), p.Kind, meta)
		if err != nil && path == "" {
			return nil, &host.Error{Code: "NEW_SCRIPT", Message: err.Error()}
		}
		res := map[string]string{"path": path}
		if err != nil {
			res["warning"] = err.Error()
		}
		return res, nil
	}))

	s.Handle("unity.watch", handler(s, func(_ context.Context, root string, _ none) (any, error) {
		st.mu.Lock()
		start := !st.watching
		st.watching = true
		st.mu.Unlock()
		if start {
			go watch(s)
		}
		return logState(nil, logPath(s), root), nil
	}))
}

// projectRelative makes a path from Project Settings absolute against the
// project's folder.
func projectRelative(s *host.Server, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	if dir := s.Context().Project.Dir; dir != "" {
		return filepath.Join(dir, filepath.FromSlash(p))
	}
	return p
}

// generatedSolution writes the projects of root's assemblies, as cs-ls
// does for the language server, and shows them as a solution: each
// assembly with its scripts under its own folder.
func generatedSolution(root string) (*dotnet.Solution, error) {
	w := unity.Detect(root)
	out, err := unity.ProjectsDir(root)
	if err != nil {
		return nil, err
	}
	var u *unity.Install
	if in, ok := unity.FindUnity(unity.Installs(), unity.WantedVersion(w)); ok {
		u = &in
	}
	g, err := unity.Projects(w, out, u)
	if err != nil || len(g.Projects) == 0 {
		return nil, err
	}
	s := &dotnet.Solution{Path: g.Solution, Name: filepath.Base(root) + " (from its assembly definitions)"}
	byName := map[string]string{}
	for _, p := range g.Projects {
		byName[p.Name] = p.Project
	}
	var packages *dotnet.Item
	for _, gp := range g.Projects {
		path := gp.Asmdef
		if path == "" {
			path = filepath.Join(gp.Dir, gp.Name+".csproj") // a default assembly has no file of its own
		}
		proj := &dotnet.Project{Path: path, Name: gp.Name, Files: []dotnet.File{}}
		if u != nil {
			proj.TargetFramework = "Unity " + u.Version
		}
		for _, f := range gp.Files {
			rel, err := filepath.Rel(gp.Dir, f)
			if err != nil {
				rel = f
			}
			proj.Files = append(proj.Files, dotnet.File{Rel: filepath.ToSlash(rel), Path: f, Kind: "compile"})
		}
		for _, n := range gp.Projects {
			proj.ProjectRefs = append(proj.ProjectRefs, byName[n])
		}
		for _, dll := range gp.DLLs {
			proj.References = append(proj.References, dotnet.Reference{Name: strings.TrimSuffix(filepath.Base(dll), ".dll"), Path: dll})
		}
		item := &dotnet.Item{Kind: "project", Name: gp.Name, Path: path, Project: proj}
		if !gp.External {
			s.Items = append(s.Items, item)
			continue
		}
		// The packages' assemblies it references go in a Packages folder.
		if packages == nil {
			packages = &dotnet.Item{Kind: "folder", Name: "Packages"}
			s.Items = append(s.Items, packages)
		}
		packages.Children = append(packages.Children, item)
	}
	return s, nil
}

// find looks a GUID up in an index of the open folder and the package
// cache, built again when old or asked for.
func (st *state) find(root, guid string, fresh bool) string {
	st.mu.Lock()
	defer st.mu.Unlock()
	if fresh || st.index == nil || st.indexRoot != root || time.Since(st.indexAt) > time.Minute {
		roots := []string{root}
		if cache := filepath.Join(root, "Library", "PackageCache"); isDir(cache) {
			roots = append(roots, cache)
		}
		st.index, st.indexRoot, st.indexAt = unity.IndexGUIDs(roots), root, time.Now()
	}
	return st.index.Find(guid)
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func logPath(s *host.Server) string {
	if p := s.Context().Setting("unityEditorLog"); p != "" {
		return p
	}
	return unity.DefaultEditorLog()
}

// logState is what the window shows of the Editor.log.
func logState(l *unity.EditorLog, path, root string) map[string]any {
	st := map[string]any{"path": path, "root": root, "exists": false, "matches": true, "project": "", "messages": []unity.Message{}}
	if _, err := os.Stat(path); err == nil {
		st["exists"] = true
	}
	if l != nil {
		st["project"], st["matches"], st["messages"] = l.Project, l.Matches(), l.Messages()
	}
	return st
}

// watch follows the Editor.log for the open folder, and tells the window
// its compile messages (unity/problems) and its new lines (unity/log). It
// starts over when the folder or the log setting changes.
func watch(s *host.Server) {
	var (
		l      *unity.EditorLog
		exists bool
	)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for range tick.C {
		r, err := root(s)
		if err != nil {
			continue
		}
		path := logPath(s)
		if l == nil || l.Path != path || !unity.SamePath(l.Root, r) {
			l = &unity.EditorLog{Path: path, Root: r}
			exists = false
			_ = s.Notify("unity/problems", logState(l, path, r))
		}
		lines, changed, err := l.Poll()
		if errors.Is(err, os.ErrNotExist) {
			if exists {
				exists = false
				_ = s.Notify("unity/problems", logState(l, path, r))
			}
			continue
		}
		if err == nil && !exists {
			exists, changed = true, true
		}
		if len(lines) > 0 {
			if len(lines) > 300 {
				lines = lines[len(lines)-300:]
			}
			_ = s.Notify("unity/log", map[string]any{"lines": lines})
		}
		if changed {
			_ = s.Notify("unity/problems", logState(l, path, r))
		}
	}
}
