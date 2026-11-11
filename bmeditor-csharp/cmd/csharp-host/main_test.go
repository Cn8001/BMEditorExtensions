package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"bmeditor-csharp/bmext/host"
	"bmeditor-csharp/internal/dotnet"
	"bmeditor-csharp/internal/unity"
)

func connect(t *testing.T, folder string) *host.Client {
	t.Helper()
	s := host.New()
	register(s, &state{})
	toServer, fromClient := io.Pipe()
	toClient, fromServer := io.Pipe()
	go func() {
		_ = s.Serve(t.Context(), toServer, fromServer)
		fromServer.Close()
	}()
	t.Cleanup(func() { fromClient.Close() })
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

func TestMethods_OnASolutionFolder(t *testing.T) {
	folder, _ := filepath.Abs(filepath.Join("..", "..", "internal", "dotnet", "testdata", "unity"))
	c := connect(t, folder)

	ws := call[struct {
		Workspace unity.Workspace
		Solutions []string
	}](t, c, "workspace", nil)
	// The solution first, then the projects at the root.
	if ws.Workspace.Kind != "folder" || len(ws.Solutions) != 3 || filepath.Base(ws.Solutions[0]) != "unity.sln" {
		t.Errorf("workspace = %+v", ws)
	}
	sln := call[dotnet.Solution](t, c, "solution", nil)
	if sln.Name != "unity" || len(sln.Items) != 2 {
		t.Errorf("solution = %+v", sln)
	}
	if _, err := c.Call(t.Context(), "guid.find", map[string]string{"guid": "nope"}); err == nil {
		t.Error("a bad GUID should be refused")
	}
	kinds := call[[]map[string]string](t, c, "script.kinds", nil)
	if len(kinds) != len(unity.ScriptKinds) {
		t.Errorf("kinds = %v", kinds)
	}
}

func TestMethods_NeedAFolder(t *testing.T) {
	c := connect(t, "")
	if _, err := c.Call(t.Context(), "assemblies", nil); err == nil {
		t.Error("without a folder the methods should fail")
	}
}

// TestRealFolder runs the read-only methods on CSHARP_REAL, a real Unity
// project or a folder of packages, and logs what they find.
func TestRealFolder(t *testing.T) {
	folder := os.Getenv("CSHARP_REAL")
	if folder == "" {
		t.Skip("CSHARP_REAL is not set")
	}
	c := connect(t, folder)
	ws := call[struct {
		Workspace unity.Workspace
		Solutions []string
	}](t, c, "workspace", nil)
	t.Logf("kind %s, Unity %s, %d packages, solutions %v", ws.Workspace.Kind, ws.Workspace.UnityVersion, len(ws.Workspace.Packages), ws.Solutions)

	as := call[unity.Assemblies](t, c, "assemblies", nil)
	own, unresolved, broken := 0, 0, 0
	for _, a := range as.Assemblies {
		if a.External {
			continue
		}
		own++
		if a.Error != "" {
			broken++
			t.Logf("broken %s: %s", a.Path, a.Error)
		}
		for _, r := range a.References {
			if r.Status == "unresolved" {
				unresolved++
				if unresolved <= 10 {
					t.Logf("unresolved %s → %s", a.Name, r.Raw)
				}
			}
		}
	}
	t.Logf("%d assemblies here (%d external), %d edges, %d cycles %v, %d unresolved references, %d broken",
		own, len(as.Assemblies)-own, len(as.Edges), len(as.Cycles), as.Cycles, unresolved, broken)

	meta := call[struct {
		Roots  []string
		Report unity.MetaReport
	}](t, c, "meta.check", nil)
	r := meta.Report
	t.Logf("meta: %d checked in %v: %d missing, %d orphans, %d duplicate GUIDs, %d likely renames",
		r.Checked, meta.Roots, len(r.Missing), len(r.Orphans), len(r.Duplicates), len(r.Renames))
	for i, m := range r.Missing {
		if i == 5 {
			break
		}
		t.Logf("  missing %s", m)
	}
	for i, p := range r.Renames {
		if i == 5 {
			break
		}
		t.Logf("  %s %s → %s", p.Reason, p.Meta, p.To)
	}

	if len(ws.Solutions) > 0 {
		sln := call[dotnet.Solution](t, c, "solution", map[string]string{"path": ws.Solutions[0]})
		projects, files, failed := 0, 0, 0
		var count func(items []*dotnet.Item)
		count = func(items []*dotnet.Item) {
			for _, it := range items {
				if it.Kind == "project" {
					projects++
					if it.Project != nil {
						files += len(it.Project.Files)
					} else {
						failed++
						t.Logf("project %s: %s", it.Name, it.Error)
					}
				}
				count(it.Children)
			}
		}
		count(sln.Items)
		t.Logf("solution %s: %d projects, %d files, %d unreadable", sln.Name, projects, files, failed)
	}
}
