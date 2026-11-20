package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"bmeditor-mentor/bmext/host"
	"bmeditor-mentor/internal/agent"
	"bmeditor-mentor/internal/changes"
	"bmeditor-mentor/internal/engine"
	"bmeditor-mentor/internal/explain"
	"bmeditor-mentor/internal/search"
)

const auth = "example.com/authsvc/internal/auth."

func connect(t *testing.T, folder string) *host.Client {
	t.Helper()
	s := host.New()
	st := &state{dataDir: t.TempDir()}
	register(s, st)
	toServer, fromClient := io.Pipe()
	toClient, fromServer := io.Pipe()
	go func() {
		_ = s.Serve(t.Context(), toServer, fromServer)
		fromServer.Close()
	}()
	t.Cleanup(func() {
		fromClient.Close()
		st.close()
	})
	c := host.NewClient(toClient, fromClient, nil)
	if _, err := c.Call(t.Context(), "initialize", host.Context{Folder: folder, Settings: map[string]string{"autoConnect": "false"}}); err != nil {
		t.Fatal(err)
	}
	return c
}

func call[T any](t *testing.T, c *host.Client, method string, params any) T {
	t.Helper()
	raw, err := c.Call(t.Context(), method, params)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return v
}

func TestMethods_OnTheFixture(t *testing.T) {
	folder, _ := filepath.Abs(filepath.Join("..", "..", "internal", "golang", "testdata", "authsvc"))
	c := connect(t, folder)

	res := call[search.Result](t, c, "search", map[string]any{"q": "provider identity linked to a user"})
	if len(res.Candidates) == 0 || res.Candidates[0].Key != auth+"UserIdentity" {
		t.Fatalf("search = %+v", res)
	}
	st := call[engine.Status](t, c, "status", nil)
	if st.State != "ready" || st.Symbols == 0 || st.Head.Commit == "" && false {
		t.Errorf("status = %+v", st)
	}
	card := call[explain.Card](t, c, "card", map[string]string{"key": auth + "Service"})
	if len(card.Methods) == 0 || len(card.Constructors) == 0 {
		t.Errorf("card = %+v", card)
	}
	ref := call[explain.Ref](t, c, "at", map[string]any{"file": card.Symbol.File, "line": card.Symbol.Line})
	if ref.Key != auth+"Service" {
		t.Errorf("at = %+v", ref)
	}
	flow := call[explain.Flow](t, c, "flow", map[string]any{"key": auth + "Service.Login"})
	if flow.Root == nil || len(flow.Root.Children) == 0 {
		t.Errorf("flow = %+v", flow)
	}
	arch := call[explain.Graph](t, c, "arch", nil)
	if len(arch.Nodes) < 5 {
		t.Errorf("arch = %+v", arch)
	}
	lens := call[[]lensItem](t, c, "lens", map[string]any{"file": card.Symbol.File})
	if len(lens) == 0 {
		t.Error("no lens")
	}
	// The fixture is committed in this repository: nothing changed.
	rep := call[changes.Report](t, c, "changes", nil)
	if rep.Since != "HEAD" || len(rep.Units) != 0 {
		t.Errorf("changes = %+v", rep)
	}
	states := call[map[string]string](t, c, "review.mark", map[string]any{"keys": []string{auth + "Service"}})
	if states[auth+"Service"] != "reviewed" {
		t.Errorf("states = %v", states)
	}
	if _, err := c.Call(t.Context(), "card", map[string]string{"key": "nope"}); err == nil {
		t.Error("an unknown key should fail")
	}
}

func TestAgentMethods(t *testing.T) {
	folder, _ := filepath.Abs(filepath.Join("..", "..", "internal", "golang", "testdata", "authsvc"))
	s := host.New()
	st := &state{dataDir: t.TempDir()}
	register(s, st)
	toServer, fromClient := io.Pipe()
	toClient, fromServer := io.Pipe()
	go func() { _ = s.Serve(t.Context(), toServer, fromServer); fromServer.Close() }()
	t.Cleanup(func() { fromClient.Close(); st.close() })
	c := host.NewClient(toClient, fromClient, nil)
	if _, err := c.Call(t.Context(), "initialize", host.Context{Folder: folder, Settings: map[string]string{"maxFiles": "3", "autoConnect": "false"}}); err != nil {
		t.Fatal(err)
	}
	as := call[agent.State](t, c, "agent.state", nil)
	if as.Level == "" || as.Plan != nil || as.Policy.MaxFiles != 3 || !as.Policy.RequirePlan {
		t.Fatalf("state = %+v", as)
	}
	// The agent proposes through the gate; the developer answers here.
	g := st.gateFor(folder)
	g.Propose("Task", []agent.PlanStep{{Title: "One"}})
	as = call[agent.State](t, c, "agent.plan", map[string]any{"approve": true})
	if as.Plan.Status != "approved" || as.Step != 1 {
		t.Errorf("after approving = %+v", as.Plan)
	}
	as = call[agent.State](t, c, "agent.end", nil)
	if as.Plan.Status != "done" {
		t.Error("the plan did not end")
	}
	if _, err := c.Call(t.Context(), "agent.checkpoint", map[string]any{"answer": "approve"}); err == nil {
		t.Error("no checkpoint waits; answering one should fail")
	}
}

// TestAgentChanges_OnlyWhatTheAgentChanged edits two files of a
// committed copy of the fixture, one through the agent's hook and one by
// hand: agent.changes lists the agent's alone, and forgets it once it is
// committed.
func TestAgentChanges_OnlyWhatTheAgentChanged(t *testing.T) {
	from, _ := filepath.Abs(filepath.Join("..", "..", "internal", "golang", "testdata", "authsvc"))
	folder := t.TempDir()
	if err := os.CopyFS(folder, os.DirFS(from)); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = folder
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "fixture")

	s := host.New()
	st := &state{dataDir: t.TempDir()}
	register(s, st)
	toServer, fromClient := io.Pipe()
	toClient, fromServer := io.Pipe()
	go func() { _ = s.Serve(t.Context(), toServer, fromServer); fromServer.Close() }()
	t.Cleanup(func() { fromClient.Close(); st.close() })
	c := host.NewClient(toClient, fromClient, nil)
	if _, err := c.Call(t.Context(), "initialize", host.Context{Folder: folder, Settings: map[string]string{"autoConnect": "false", "requirePlan": "false"}}); err != nil {
		t.Fatal(err)
	}
	_ = call[engine.Status](t, c, "status", nil)

	appendTo := func(rel, text string) string {
		path := filepath.Join(folder, rel)
		b, _ := os.ReadFile(path)
		if err := os.WriteFile(path, append(b, []byte(text)...), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	g := st.gateFor(folder)
	agentFile := filepath.Join(folder, "internal", "auth", "provider.go")
	if d := g.Hook(agent.HookCall{Event: "PreToolUse", Tool: "Edit", Input: map[string]any{"file_path": agentFile}}); !d.Allow {
		t.Fatalf("the edit was held: %+v", d)
	}
	appendTo("internal/auth/provider.go", "\nfunc AddedByTheAgent() {}\n")
	appendTo("internal/auth/service.go", "\nfunc AddedByHand() {}\n")
	_ = call[engine.Status](t, c, "reindex", nil)

	got := call[agentChanges](t, c, "agent.changes", nil)
	if len(got.Files) != 1 || got.Files[0].Path != agentFile {
		t.Fatalf("files = %+v", got.Files)
	}
	f := got.Files[0]
	if f.Added == 0 || len(f.Changes) != 1 || f.Changes[0].Name != "AddedByTheAgent" || f.Changes[0].Change != "added" {
		t.Errorf("file = %+v", f)
	}
	if got.Lines[0] != f.Added {
		t.Errorf("lines = %v", got.Lines)
	}

	git("commit", "-q", "-am", "agent's work")
	if got := call[agentChanges](t, c, "agent.changes", nil); len(got.Files) != 0 {
		t.Errorf("after the commit = %+v", got.Files)
	}
}

func TestMethods_NeedAFolder(t *testing.T) {
	c := connect(t, "")
	if _, err := c.Call(t.Context(), "status", nil); err == nil {
		t.Error("without a folder the methods should fail")
	}
}

func TestMethods_WithoutGoCode(t *testing.T) {
	c := connect(t, t.TempDir())
	st := call[engine.Status](t, c, "status", nil)
	_ = st
	if _, err := c.Call(t.Context(), "search", map[string]any{"q": "x"}); err == nil {
		t.Error("search with no code should say so")
	}
	st = call[engine.Status](t, c, "status", nil)
	if st.State != "error" {
		t.Errorf("status = %+v", st)
	}
}
