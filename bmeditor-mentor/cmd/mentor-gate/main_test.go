package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"bmeditor-mentor/internal/gateproto"
)

// backend is a stand-in for the extension's backend that denies edits.
func backend(t *testing.T, root string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "no", 403)
			return
		}
		switch r.URL.Path {
		case "/hook":
			_ = json.NewEncoder(w).Encode(gateproto.Decision{Reason: "propose a plan first"})
		case "/tool":
			var c gateproto.ToolCall
			_ = json.NewDecoder(r.Body).Decode(&c)
			_ = json.NewEncoder(w).Encode(gateproto.ToolResult{Text: c.Tool + " in " + c.Root})
		}
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	dir := t.TempDir()
	t.Setenv("MENTOR_GATE_DIR", dir)
	if err := gateproto.Write(dir, gateproto.Registry{Port: port, Token: "tok", Root: root}); err != nil {
		t.Fatal(err)
	}
}

func TestHook_DeniesWithTheReason(t *testing.T) {
	root := t.TempDir()
	backend(t, root)
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	out := hook(strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"Edit","cwd":"` + root + `/sub","tool_input":{}}`))
	var v struct {
		H struct {
			Decision string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out, &v); err != nil || v.H.Decision != "deny" || !strings.Contains(v.H.Reason, "propose a plan") {
		t.Errorf("out = %s", out)
	}
}

func TestHook_LetsAllThroughWithoutTheEditor(t *testing.T) {
	t.Setenv("MENTOR_GATE_DIR", t.TempDir())
	if out := hook(strings.NewReader(`{"hook_event_name":"PreToolUse","cwd":"/nowhere"}`)); out != nil {
		t.Errorf("out = %s", out)
	}
}

func TestMCP_ListsAndCallsTools(t *testing.T) {
	root := t.TempDir()
	backend(t, root)
	t.Setenv("CLAUDE_PROJECT_DIR", root)
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"mentor_status","arguments":{}}}`,
	}, "\n")
	var out strings.Builder
	serveMCP(strings.NewReader(in), &out)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %v", lines)
	}
	if !strings.Contains(lines[0], `"instructions"`) || !strings.Contains(lines[1], "mentor_checkpoint") || !strings.Contains(lines[2], "mentor_status in "+root) {
		t.Errorf("lines = %v", lines)
	}
}
