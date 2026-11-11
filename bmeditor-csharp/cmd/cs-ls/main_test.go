package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// The test binary is also cs-ls itself when CS_LS_MAIN is set.
func TestMain(m *testing.M) {
	if os.Getenv("CS_LS_MAIN") == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

func TestSolution_PrefersTheFolderNamedSln(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"Other.sln", filepath.Base(root) + ".sln", "x.slnx", "A.csproj"} {
		if err := os.WriteFile(filepath.Join(root, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if sln, _ := solution(root); filepath.Base(sln) != filepath.Base(root)+".sln" {
		t.Errorf("solution = %s", sln)
	}
	only := t.TempDir()
	_ = os.WriteFile(filepath.Join(only, "A.csproj"), nil, 0o644)
	if sln, projects := solution(only); filepath.Base(sln) != "A.csproj" || len(projects) != 1 {
		t.Errorf("one project: %s %v", sln, projects)
	}
}

func TestRootOf(t *testing.T) {
	cases := map[string]string{
		`{"params":{"rootUri":"file:///home/me/proj"}}`:          "/home/me/proj",
		`{"params":{"rootUri":"file:///C:/Users/me/My%20Game"}}`: filepath.FromSlash("C:/Users/me/My Game"),
		`{"params":{"rootPath":"/x"}}`:                           "/x",
	}
	for in, want := range cases {
		if got := rootOf([]byte(in)); got != want {
			t.Errorf("rootOf(%s) = %q, want %q", in, got, want)
		}
	}
	if got := fileURI("C:/Users/me/My Game/G.sln"); got != "file:///C:/Users/me/My%20Game/G.sln" {
		t.Errorf("fileURI = %s", got)
	}
}

func frame(v any) string {
	b, _ := json.Marshal(v)
	return "Content-Length: " + strconv.Itoa(len(b)) + "\r\n\r\n" + string(b)
}

// TestRoslyn_OpensTheSolutionAfterInitialized starts cs-ls with a fake
// roslyn-language-server that records what it is sent.
func TestRoslyn_OpensTheSolutionAfterInitialized(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake server is a shell script")
	}
	bin, root := t.TempDir(), t.TempDir()
	sln := filepath.Join(root, filepath.Base(root)+".sln")
	_ = os.WriteFile(sln, nil, 0o644)
	got := filepath.Join(t.TempDir(), "got")
	cat, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("no cat")
	}
	script := "#!/bin/sh\necho \"$@\" > " + got + ".args\n" + cat + " > " + got + "\n"
	if err := os.WriteFile(filepath.Join(bin, "roslyn-language-server"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	input := frame(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]string{"rootUri": fileURI(root)}}) +
		frame(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}) +
		frame(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "shutdown"})
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "CS_LS_MAIN=1", "PATH="+bin, "BMEDITOR_CSHARP_SERVER=roslyn", "HOME="+t.TempDir())
	cmd.Stdin = strings.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("cs-ls: %v: %s", err, stderr.String())
	}
	raw, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(bytes.NewReader(raw))
	var methods []string
	for {
		body, err := readFrame(r)
		if err != nil {
			break
		}
		methods = append(methods, method(body))
		if method(body) == "solution/open" && !strings.Contains(string(body), fileURI(sln)) {
			t.Errorf("solution/open names %s, want %s", body, fileURI(sln))
		}
	}
	if want := "initialize initialized solution/open shutdown"; strings.Join(methods, " ") != want {
		t.Errorf("the server got %v, want %s", methods, want)
	}
	if args, _ := os.ReadFile(got + ".args"); !strings.Contains(string(args), "--stdio") {
		t.Errorf("args = %s", args)
	}
}

func TestNoServer_AnswersInitializeWithTheReason(t *testing.T) {
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "CS_LS_MAIN=1", "PATH="+t.TempDir(), "HOME="+t.TempDir(), "BMEDITOR_CSHARP_SERVER=")
	cmd.Stdin = strings.NewReader(frame(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}}))
	out, _ := cmd.Output()
	if !strings.Contains(string(out), "csharp-ls") || !strings.Contains(string(out), `"id":1`) {
		t.Errorf("answer = %s", out)
	}
}

func TestRelay_PutsWhatTheServerSaysOnStderr(t *testing.T) {
	// Arrange: the server tells the user its solution failed, and logs.
	from := strings.NewReader(
		frame(map[string]any{"jsonrpc": "2.0", "method": "window/showMessage", "params": map[string]any{"type": 1, "message": "Solution x.sln could not be loaded: TypeLoadException"}}) +
			frame(map[string]any{"jsonrpc": "2.0", "method": "window/logMessage", "params": map[string]any{"type": 4, "message": "chatter"}}) +
			frame(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}}))
	var to bytes.Buffer
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = w

	// Act
	relay(from, &to, servers[0])
	os.Stderr = saved
	w.Close()
	stderr, _ := io.ReadAll(r)

	// Assert: every message reaches the editor unchanged...
	got := bufio.NewReader(&to)
	n := 0
	for {
		if _, err := readFrame(got); err != nil {
			break
		}
		n++
	}
	if n != 3 {
		t.Errorf("the editor got %d messages, want 3", n)
	}
	// ...and the failure, with what to do, is in Output; the chatter not.
	if s := string(stderr); !strings.Contains(s, "csharp-ls: Solution x.sln could not be loaded") || !strings.Contains(s, "dotnet tool update -g csharp-ls") || strings.Contains(s, "chatter") {
		t.Errorf("stderr = %s", s)
	}
}
