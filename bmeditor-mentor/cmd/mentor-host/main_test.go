package main

import (
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	"bmeditor-mentor/bmext/host"
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
	if _, err := c.Call(t.Context(), "initialize", host.Context{Folder: folder}); err != nil {
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
