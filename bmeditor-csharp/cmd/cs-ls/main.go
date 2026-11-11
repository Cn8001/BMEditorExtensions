// Command cs-ls starts a C# language server for BMEditor on the open
// folder's solution. Which server is installed differs per machine, and
// each is told about the solution its own way, so cs-ls picks one and
// tells it; every other message passes through as it is.
//
// The server is BMEDITOR_CSHARP_SERVER (csharp-ls, roslyn or omnisharp)
// when set, else the first one installed of csharp-ls, roslyn-language-server
// and OmniSharp.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"bmeditor-csharp/internal/dotnet"
	"bmeditor-csharp/internal/unity"
)

const install = "install the .NET 10 SDK, then dotnet tool install -g csharp-ls"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "cs-ls:", err)
		os.Exit(1)
	}
}

// server is a language server cs-ls can start.
type server struct {
	id      string
	program string
	args    func(sln string) []string
	// open, when set, is the notification that opens the solution, sent
	// after the editor's initialized.
	open func(sln string, projects []string) (method string, params any)
}

var servers = []server{
	{
		id: "csharp-ls", program: "csharp-ls",
		args: func(sln string) []string {
			if sln != "" {
				return []string{"--solution", sln}
			}
			return nil
		},
	},
	{
		id: "roslyn", program: "roslyn-language-server",
		args: func(string) []string {
			return []string{"--stdio", "--logLevel", "Information", "--extensionLogDirectory", filepath.Join(os.TempDir(), "bmeditor-roslyn")}
		},
		open: func(sln string, projects []string) (string, any) {
			if strings.EqualFold(filepath.Ext(sln), ".sln") || strings.EqualFold(filepath.Ext(sln), ".slnx") {
				return "solution/open", map[string]string{"solution": fileURI(sln)}
			}
			uris := []string{}
			for _, p := range projects {
				uris = append(uris, fileURI(p))
			}
			return "project/open", map[string]any{"projects": uris}
		},
	},
	{
		id: "omnisharp", program: "OmniSharp",
		args: func(sln string) []string {
			if sln != "" {
				return []string{"-lsp", "-s", sln}
			}
			return []string{"-lsp"}
		},
	},
}

func run() error {
	r := bufio.NewReader(os.Stdin)
	first, err := readFrame(r)
	if err != nil {
		return err
	}
	root := rootOf(first)
	sln, projects := solution(root)
	if sln == "" && len(projects) == 0 && root != "" {
		// No solution: a UPM package on its own, or a Unity project whose
		// solution Unity has not written. Write the projects Unity would.
		if g, err := generate(root); err != nil {
			fmt.Fprintln(os.Stderr, "cs-ls: could not write projects:", err)
		} else if g != "" {
			sln = g
		}
	}

	srv, path, err := pick(os.Getenv("BMEDITOR_CSHARP_SERVER"))
	if err != nil {
		// Answer initialize with the reason, which the editor shows.
		_ = refuse(first, err.Error())
		return err
	}
	cmd := exec.Command(path, srv.args(sln)...)
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		_ = refuse(first, fmt.Sprintf("start %s: %v", srv.program, err))
		return err
	}
	fmt.Fprintf(os.Stderr, "cs-ls: %s on %s\n", path, orNone(sln))
	relayed := make(chan struct{})
	go func() {
		relay(out, os.Stdout, srv)
		close(relayed)
	}()
	if err := writeFrame(in, first); err != nil {
		return err
	}
	go func() {
		defer in.Close()
		if srv.open == nil || (sln == "" && len(projects) == 0) {
			_, _ = io.Copy(in, r)
			return
		}
		// Pass messages one by one until initialized, then open the
		// solution, then pass the rest through.
		for {
			body, err := readFrame(r)
			if err != nil {
				return
			}
			if writeFrame(in, body) != nil {
				return
			}
			if method(body) == "initialized" {
				m, p := srv.open(sln, projects)
				note, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": m, "params": p})
				if writeFrame(in, note) != nil {
					return
				}
				break
			}
		}
		_, _ = io.Copy(in, r)
	}()
	<-relayed
	return cmd.Wait()
}

// relay passes the server's messages to the editor as they are, and puts
// what it says to the user (window/showMessage) and its warnings and
// errors (window/logMessage) on stderr too, which the editor shows in
// Output: a solution that does not load is otherwise easy to miss.
func relay(from io.Reader, to io.Writer, srv server) {
	r := bufio.NewReaderSize(from, 64<<10)
	hinted := false
	for {
		body, err := readFrame(r)
		if err != nil {
			return
		}
		if writeFrame(to, body) != nil {
			return
		}
		var m struct {
			Method string `json:"method"`
			Params struct {
				Type    int    `json:"type"`
				Message string `json:"message"`
			} `json:"params"`
		}
		if json.Unmarshal(body, &m) != nil || (m.Method != "window/showMessage" && m.Method != "window/logMessage") {
			continue
		}
		if m.Method == "window/logMessage" && m.Params.Type > 2 {
			continue
		}
		fmt.Fprintf(os.Stderr, "%s: %s\n", srv.id, m.Params.Message)
		if !hinted && strings.Contains(m.Params.Message, "could not be loaded") {
			hinted = true
			fmt.Fprintln(os.Stderr, "cs-ls: the solution did not load, so there is no completion beyond words. "+
				"A csharp-ls built for another .NET SDK fails like this (a TypeLoadException in Microsoft.Build): "+
				"install the newest .NET SDK and run dotnet tool update -g csharp-ls.")
		}
	}
}

// generate writes projects for root's assemblies into the user's cache and
// returns their solution, or "" when root has no C# scripts.
func generate(root string) (string, error) {
	w := unity.Detect(root)
	out, err := unity.ProjectsDir(root)
	if err != nil {
		return "", err
	}
	want := unity.WantedVersion(w)
	var u *unity.Install
	if in, ok := unity.FindUnity(unity.Installs(), want); ok {
		u = &in
	}
	g, err := unity.Projects(w, out, u)
	if err != nil || len(g.Projects) == 0 {
		return "", err
	}
	switch {
	case u == nil:
		fmt.Fprintf(os.Stderr, "cs-ls: no Unity Editor found (Unity Hub's folder, or BMEDITOR_UNITY_EDITOR): Unity's types are unknown\n")
	case want != "" && u.Version != want && !strings.HasPrefix(u.Version, want+"."):
		fmt.Fprintf(os.Stderr, "cs-ls: Unity %s is not installed; using %s\n", want, u.Version)
	}
	names := make([]string, 0, len(g.Projects))
	for _, p := range g.Projects {
		names = append(names, p.Name)
	}
	unityVersion := "no Unity"
	if u != nil {
		unityVersion = "Unity " + u.Version
	}
	fmt.Fprintf(os.Stderr, "cs-ls: no solution in %s; wrote projects for %s (%s) in %s\n", root, strings.Join(names, ", "), unityVersion, out)
	return g.Solution, nil
}

func orNone(s string) string {
	if s == "" {
		return "no solution"
	}
	return s
}

// pick finds the server to start: the one named, or the first installed.
func pick(named string) (server, string, error) {
	for _, s := range servers {
		if named != "" && !strings.EqualFold(named, s.id) && !strings.EqualFold(named, s.program) {
			continue
		}
		if p, ok := find(s.program); ok {
			return s, p, nil
		}
		if named != "" {
			return s, "", fmt.Errorf("%s is not installed", s.program)
		}
	}
	if named != "" {
		return server{}, "", fmt.Errorf("BMEDITOR_CSHARP_SERVER=%s: use csharp-ls, roslyn or omnisharp", named)
	}
	return server{}, "", errors.New("no C# language server is installed: " + install + ", then restart BMEditor")
}

// find looks for a program on PATH, then where dotnet tool install -g puts
// it, which is on PATH only for shells started after the install.
func find(program string) (string, bool) {
	if p, err := exec.LookPath(program); err == nil {
		return p, true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	p := filepath.Join(home, ".dotnet", "tools", program)
	if runtime.GOOS == "windows" {
		p += ".exe"
	}
	if _, err := os.Stat(p); err == nil {
		return p, true
	}
	return "", false
}

// solution is the solution to open in root: the one the project's
// settings name (C# › Solution, kept in the .bmproj), else a .sln named
// like the folder (Unity's), another .sln, a .slnx; with none, the
// projects in root, opened as a solution of one when there is one.
func solution(root string) (string, []string) {
	if root == "" {
		return "", nil
	}
	chosen := dotnet.ProjectSetting(root, "csharp.solution")
	found := dotnet.Candidates(root, chosen)
	if len(found) == 0 {
		return "", nil
	}
	if !strings.EqualFold(filepath.Ext(found[0]), ".csproj") {
		return found[0], nil
	}
	// Projects only: the one chosen or the only one, else all of them.
	if chosen != "" || len(found) == 1 {
		return found[0], found[:1]
	}
	return "", found
}

// refuse answers the initialize request with an error.
func refuse(first []byte, msg string) error {
	var req struct {
		ID json.RawMessage `json:"id"`
	}
	_ = json.Unmarshal(first, &req)
	if len(req.ID) == 0 {
		return nil
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32002, "message": msg}})
	return writeFrame(os.Stdout, body)
}

func method(body []byte) string {
	var m struct {
		Method string `json:"method"`
	}
	_ = json.Unmarshal(body, &m)
	return m.Method
}

// rootOf is the folder an initialize request opens, from its rootUri.
func rootOf(body []byte) string {
	var req struct {
		Params struct {
			RootURI  string `json:"rootUri"`
			RootPath string `json:"rootPath"`
		} `json:"params"`
	}
	if json.Unmarshal(body, &req) != nil {
		return ""
	}
	u, err := url.Parse(req.Params.RootURI)
	if err != nil || u.Scheme != "file" {
		return req.Params.RootPath
	}
	// file:///C:/x is C:/x on Windows; elsewhere the path is as it is.
	p := u.Path
	if len(p) > 2 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}

func fileURI(p string) string {
	p = filepath.ToSlash(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if v, ok := strings.CutPrefix(line, "Content-Length:"); ok {
			length, _ = strconv.Atoi(strings.TrimSpace(v))
		}
	}
	if length < 0 {
		return nil, errors.New("a message without Content-Length")
	}
	body := make([]byte, length)
	_, err := io.ReadFull(r, body)
	return body, err
}

func writeFrame(w io.Writer, body []byte) error {
	_, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}
