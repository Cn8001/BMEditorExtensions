// Package pkgmgr installs, updates and removes bmeditor extensions. An
// extension comes from a folder, a .zip (a path or an http(s) URL), or a git
// repository on any server: GitHub, GitLab or another, over https or ssh,
// with #ref for a branch or tag. Each install keeps where it came from, so
// Update fetches it again from there.
package pkgmgr

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Cn8001/BMEditorExtensions/bmext"
)

// SourceFile, in an installed extension's folder, records where it came from.
const SourceFile = ".bmext-source.json"

var (
	ErrNotInstalled = errors.New("extension is not installed")
	ErrBadSource    = errors.New("not a folder, .zip or git repository")
	ErrGitMissing   = errors.New("git is not installed")
	ErrFetch        = errors.New("could not fetch the extension")
	ErrNotBuilt     = errors.New("extension is not built")
)

type Source struct {
	Spec      string    `json:"spec"` // as given to Install
	Commit    string    `json:"commit,omitempty"`
	Installed time.Time `json:"installed"`
}

type Installed struct {
	Manifest bmext.Manifest `json:"manifest"`
	Dir      string         `json:"dir"`
	Source   Source         `json:"source"`
}

// Broken is a folder in the extensions folder that is not a usable
// extension, listed so it can be removed rather than silently skipped.
type Broken struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

type Manager struct {
	Dir  string // where extensions are installed, one folder each
	Git  string // the git program; "git" when empty
	HTTP *http.Client
	Now  func() time.Time
	// Release, when set, is called with an extension's name just before its
	// installed folder is replaced or removed, to stop the programs of it
	// that run (its backend, its language servers): Windows does not let a
	// folder go while a program in it runs.
	Release func(name string)
}

// Log receives the progress of an install, a line at a time.
type Log func(line string)

func (m *Manager) git() string {
	if m.Git == "" {
		return "git"
	}
	return m.Git
}

func (m *Manager) now() time.Time {
	if m.Now == nil {
		return time.Now()
	}
	return m.Now()
}

// List reads every installed extension, by name.
func (m *Manager) List() ([]Installed, []Broken, error) {
	entries, err := os.ReadDir(m.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read extensions: %w", err)
	}
	var out []Installed
	var broken []Broken
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		ins, err := m.read(e.Name())
		if err != nil {
			broken = append(broken, Broken{Name: e.Name(), Error: err.Error()})
			continue
		}
		out = append(out, ins)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Manifest.Name < out[j].Manifest.Name })
	return out, broken, nil
}

func (m *Manager) read(name string) (Installed, error) {
	dir := filepath.Join(m.Dir, name)
	man, err := bmext.Read(dir)
	if err != nil {
		return Installed{}, err
	}
	if man.Name != name {
		return Installed{}, fmt.Errorf("%w: folder %s holds extension %s", bmext.ErrInvalid, name, man.Name)
	}
	ins := Installed{Manifest: man, Dir: dir}
	if raw, err := os.ReadFile(filepath.Join(dir, SourceFile)); err == nil {
		_ = json.Unmarshal(raw, &ins.Source)
	}
	return ins, nil
}

// Install fetches spec, builds it when its manifest says how, and puts it in
// place of any installed extension of the same name. Nothing changes until
// the new one is complete and its manifest is valid.
func (m *Manager) Install(ctx context.Context, spec string, log Log) (Installed, error) {
	if log == nil {
		log = func(string) {}
	}
	spec = strings.TrimSpace(spec)
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return Installed{}, fmt.Errorf("create extensions folder: %w", err)
	}
	stage, err := os.MkdirTemp(m.Dir, ".staging-")
	if err != nil {
		return Installed{}, fmt.Errorf("stage install: %w", err)
	}
	defer os.RemoveAll(stage)

	out := filepath.Join(stage, "out")
	src := Source{Spec: spec, Installed: m.now().UTC()}
	switch kind, target, ref := classify(spec); kind {
	case "folder":
		from, err := builtFolder(target)
		if err != nil {
			return Installed{}, err
		}
		log("Copying " + from)
		if err := copyTree(from, out); err != nil {
			return Installed{}, err
		}
	case "zip":
		if err := m.unzip(ctx, target, stage, out, log); err != nil {
			return Installed{}, err
		}
	case "git":
		commit, err := m.clone(ctx, target, ref, stage, out, log)
		if err != nil {
			return Installed{}, err
		}
		src.Commit = commit
	default:
		return Installed{}, fmt.Errorf("%q: %w", spec, ErrBadSource)
	}

	return m.place(stage, out, src, log)
}

// place records where out came from and puts it in place of any installed
// extension of the same name, out's manifest being valid.
func (m *Manager) place(stage, out string, src Source, log Log) (Installed, error) {
	man, err := bmext.Read(out)
	if err != nil {
		return Installed{}, err
	}
	raw, _ := json.MarshalIndent(src, "", "  ")
	if err := os.WriteFile(filepath.Join(out, SourceFile), raw, 0o644); err != nil {
		return Installed{}, fmt.Errorf("record source: %w", err)
	}
	final := filepath.Join(m.Dir, man.Name)
	old := filepath.Join(stage, "previous")
	if _, err := os.Stat(final); err == nil {
		m.release(man.Name)
		if err := retry(func() error { return os.Rename(final, old) }); err != nil {
			return Installed{}, fmt.Errorf("replace %s: %w", man.Name, err)
		}
	}
	if err := retry(func() error { return os.Rename(out, final) }); err != nil {
		_ = os.Rename(old, final)
		return Installed{}, fmt.Errorf("install %s: %w", man.Name, err)
	}
	log(fmt.Sprintf("Installed %s %s", man.Title(), man.Version))
	return m.read(man.Name)
}

// GitSource says whether spec is a git repository, and which one and ref.
func GitSource(spec string) (url, ref string, ok bool) {
	kind, target, ref := classify(strings.TrimSpace(spec))
	return target, ref, kind == "git"
}

// CloneDir makes an empty folder to clone into, inside the extensions
// folder so the install can move from it; InstallClone or RemoveClone
// removes it.
func (m *Manager) CloneDir() (string, error) {
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return "", fmt.Errorf("create extensions folder: %w", err)
	}
	return os.MkdirTemp(m.Dir, ".clone-")
}

// CloneCommand is git clone of spec into dir, for a caller that runs git
// where a person can answer its questions (a terminal).
func (m *Manager) CloneCommand(spec, dir string) ([]string, error) {
	url, ref, ok := GitSource(spec)
	if !ok {
		return nil, fmt.Errorf("%q: %w", spec, ErrBadSource)
	}
	args := []string{m.git(), "clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	return append(args, "--", url, filepath.Join(dir, "repo")), nil
}

// RemoveClone drops a CloneDir folder; anything else is left alone.
func (m *Manager) RemoveClone(dir string) error {
	if !strings.HasPrefix(filepath.Base(dir), ".clone-") || filepath.Dir(filepath.Clean(dir)) != filepath.Clean(m.Dir) {
		return fmt.Errorf("%s: %w", dir, ErrBadSource)
	}
	return os.RemoveAll(dir)
}

// InstallClone installs from a repository a caller cloned into dir with
// CloneCommand: it builds it when its manifest says how, records spec as
// its source, and removes dir.
func (m *Manager) InstallClone(ctx context.Context, dir, spec string, log Log) (Installed, error) {
	if log == nil {
		log = func(string) {}
	}
	defer m.RemoveClone(dir)
	repo := filepath.Join(dir, "repo")
	out := filepath.Join(dir, "out")
	src := Source{Spec: strings.TrimSpace(spec), Installed: m.now().UTC()}
	if b, err := hide(exec.CommandContext(ctx, m.git(), "-C", repo, "rev-parse", "HEAD")).Output(); err == nil {
		src.Commit = strings.TrimSpace(string(b))
	}
	if err := m.build(ctx, repo, out, log); err != nil {
		return Installed{}, err
	}
	return m.place(dir, out, src, log)
}

// Update installs an extension again from where it first came from.
func (m *Manager) Update(ctx context.Context, name string, log Log) (Installed, error) {
	ins, err := m.read(name)
	if err != nil {
		return Installed{}, fmt.Errorf("%s: %w", name, ErrNotInstalled)
	}
	if ins.Source.Spec == "" {
		return Installed{}, fmt.Errorf("%s: no recorded source to update from: %w", name, ErrBadSource)
	}
	return m.Install(ctx, ins.Source.Spec, log)
}

func (m *Manager) Remove(name string) error {
	if !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(name) {
		return fmt.Errorf("%q: %w", name, ErrNotInstalled)
	}
	dir := filepath.Join(m.Dir, name)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("%s: %w", name, ErrNotInstalled)
	}
	m.release(name)
	if err := retry(func() error { return os.RemoveAll(dir) }); err != nil {
		return fmt.Errorf("remove %s: %w", name, err)
	}
	return nil
}

func (m *Manager) release(name string) {
	if m.Release != nil {
		m.Release(name)
	}
}

// retry runs fn again for a little while when it fails: on Windows a file
// stays in use for a moment after its program ends, and a virus scanner or
// the search indexer may hold one briefly.
func retry(fn func() error) error {
	err := fn()
	for i := 0; err != nil && i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		err = fn()
	}
	return err
}

// classify tells what kind of source spec is, and for git which repository
// and ref. host/path without a scheme, as in github.com/me/ext, is https.
// builtFolder is what to install from a folder: the folder itself, or, for
// an extension's source whose manifest has a build step, the folder the
// build writes (dist/), which must be built already. A source folder
// copied as it is would lack the programs and the script the build makes.
func builtFolder(dir string) (string, error) {
	man, err := bmext.Read(dir)
	if err != nil || man.Build == nil || man.Build.Output == "" {
		return dir, nil // a manifest's own problems are reported once installed
	}
	out := filepath.Join(dir, filepath.FromSlash(man.Build.Output))
	if _, err := bmext.Read(out); err != nil {
		return "", fmt.Errorf("%w: %s is the source of %s; build it (%s, or bmx dev) and install %s", ErrNotBuilt,
			dir, man.Title(), strings.Join(man.Build.Command, " "), out)
	}
	return out, nil
}

func classify(spec string) (kind, target, ref string) {
	if spec == "" {
		return "", "", ""
	}
	if info, err := os.Stat(spec); err == nil {
		if info.IsDir() {
			return "folder", spec, ""
		}
		if strings.HasSuffix(strings.ToLower(spec), ".zip") {
			return "zip", spec, ""
		}
		return "", "", ""
	}
	lower := strings.ToLower(spec)
	if (strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")) && strings.HasSuffix(lower, ".zip") {
		return "zip", spec, ""
	}
	target = strings.TrimPrefix(spec, "git+")
	if i := strings.LastIndex(target, "#"); i > 0 {
		target, ref = target[:i], target[i+1:]
	}
	switch {
	case strings.HasPrefix(target, "git@"), strings.HasPrefix(target, "ssh://"), strings.HasPrefix(target, "git://"),
		strings.HasPrefix(target, "http://"), strings.HasPrefix(target, "https://"), strings.HasPrefix(target, "file://"):
		return "git", target, ref
	case regexp.MustCompile(`^[a-zA-Z0-9.-]+\.[a-z]{2,}/[^\s]+$`).MatchString(target):
		return "git", "https://" + target, ref
	}
	return "", "", ""
}

// gitEnv never lets git wait for a password: a private repository answers
// from the credentials already set up (an ssh agent, a credential helper) or
// fails at once with git's own message.
func gitEnv() []string {
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if os.Getenv("GIT_SSH_COMMAND") == "" {
		env = append(env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
	}
	return env
}

func (m *Manager) clone(ctx context.Context, url, ref, stage, out string, log Log) (string, error) {
	if _, err := exec.LookPath(m.git()); err != nil {
		return "", ErrGitMissing
	}
	repo := filepath.Join(stage, "repo")
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, "--", url, repo)
	log("git " + strings.Join(args, " "))
	if err := run(ctx, hide(exec.CommandContext(ctx, m.git(), args...)), gitEnv(), log); err != nil {
		return "", fmt.Errorf("%w: git clone %s: %w", ErrFetch, url, err)
	}
	commit := ""
	if b, err := hide(exec.CommandContext(ctx, m.git(), "-C", repo, "rev-parse", "HEAD")).Output(); err == nil {
		commit = strings.TrimSpace(string(b))
	}
	if err := m.build(ctx, repo, out, log); err != nil {
		return "", err
	}
	return commit, nil
}

// build makes the installable extension of a cloned repository in out: its
// manifest's build command, then the folder it names, or the repository.
func (m *Manager) build(ctx context.Context, repo, out string, log Log) error {
	man, err := bmext.Read(repo)
	if err != nil {
		return err
	}
	from := repo
	if man.Build != nil {
		log("Building: " + strings.Join(man.Build.Command, " "))
		cmd := hide(exec.CommandContext(ctx, man.Build.Command[0], man.Build.Command[1:]...))
		cmd.Dir = repo
		if err := run(ctx, cmd, os.Environ(), log); err != nil {
			return fmt.Errorf("build %s: %w", man.Name, err)
		}
		if man.Build.Output != "" {
			from = filepath.Join(repo, filepath.FromSlash(man.Build.Output))
		}
	}
	return copyTree(from, out)
}

func run(ctx context.Context, cmd *exec.Cmd, env []string, log Log) error {
	cmd.Env = env
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			log(sc.Text())
		}
		// A line too long to scan: keep draining so the program never blocks.
		if sc.Err() != nil {
			_, _ = io.Copy(io.Discard, pr)
		}
	}()
	err := cmd.Run()
	pw.Close()
	<-done
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func (m *Manager) unzip(ctx context.Context, target, stage, out string, log Log) error {
	path := target
	if strings.HasPrefix(strings.ToLower(target), "http") {
		log("Downloading " + target)
		path = filepath.Join(stage, "download.zip")
		if err := m.download(ctx, target, path); err != nil {
			return err
		}
	}
	log("Unpacking " + filepath.Base(path))
	root := filepath.Join(stage, "unzipped")
	if err := extract(path, root); err != nil {
		return err
	}
	// An archive often holds one folder with the extension in it.
	from := root
	if _, err := os.Stat(filepath.Join(root, bmext.ManifestFile)); err != nil {
		if entries, _ := os.ReadDir(root); len(entries) == 1 && entries[0].IsDir() {
			from = filepath.Join(root, entries[0].Name())
		}
	}
	return os.Rename(from, out)
}

func (m *Manager) download(ctx context.Context, url, path string) error {
	client := m.HTTP
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFetch, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFetch, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s answered %s", ErrFetch, url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("%w: %w", ErrFetch, err)
	}
	return f.Close()
}

// extract unpacks a zip, refusing an entry that would land outside root.
func extract(path, root string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadSource, err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if !bmext.Inside(f.Name) {
			return fmt.Errorf("%w: %s leaves the archive", ErrBadSource, f.Name)
		}
		dst := filepath.Join(root, filepath.FromSlash(f.Name))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := writeEntry(f, dst); err != nil {
			return err
		}
	}
	return nil
}

func writeEntry(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	mode := os.FileMode(0o644)
	if f.Mode()&0o111 != 0 {
		mode = 0o755
	}
	w, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, rc); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// copyTree copies a folder, leaving out version control and dependencies,
// which an installed extension does not need.
func copyTree(from, to string) error {
	skip := map[string]bool{".git": true, "node_modules": true}
	return filepath.WalkDir(from, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		if d.IsDir() && skip[d.Name()] {
			return filepath.SkipDir
		}
		dst := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}
