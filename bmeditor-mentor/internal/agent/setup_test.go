package agent

import (
	"encoding/json"
	"net/http"
	"strconv"

	"bmeditor-mentor/internal/gateproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHooks_AddKeepOthersAndRemove(t *testing.T) {
	// Arrange
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
	_ = os.WriteFile(settingsPath(root), []byte(`{"permissions":{"allow":["Bash(ls)"]},"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"other"}]}]}}`), 0o644)

	// Act
	if err := writeHooks(root, "/x/mentor-gate hook", true); err != nil {
		t.Fatal(err)
	}
	_ = writeHooks(root, "/x/mentor-gate hook", true) // twice: once in the file

	// Assert
	var s map[string]any
	b, _ := os.ReadFile(settingsPath(root))
	_ = json.Unmarshal(b, &s)
	pre := s["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 2 || s["permissions"] == nil || !HooksInstalled(root) {
		t.Fatalf("settings = %s", b)
	}
	if err := writeHooks(root, "", false); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(settingsPath(root))
	if strings.Contains(string(b), "mentor-gate") || !strings.Contains(string(b), "other") || strings.Contains(string(b), "Stop") {
		t.Errorf("after removing = %s", b)
	}
}

func TestServer_HookAndTool(t *testing.T) {
	g, _ := setup(t, DefaultPolicy())
	dir := t.TempDir()
	s, err := Listen(dir, func(root string) *Gate {
		if root == g.Root {
			return g
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Register(g.Root); err != nil {
		t.Fatal(err)
	}
	text, err := g.Tool(t.Context(), "mentor_plan", json.RawMessage(`{"task":"T","steps":[{"title":"S","files":["internal/auth/provider.go"]}]}`))
	if err != nil || !strings.Contains(text, "mentor_wait") || g.State().Waiting != "plan" {
		t.Errorf("plan = %q, %v", text, err)
	}
	// What mentor-gate does for a hook: find the registry, post the call.
	reg, err := gateproto.Find(dir, filepath.Join(g.Root, "internal"))
	if err != nil {
		t.Fatal(err)
	}
	body := `{"hook_event_name":"PreToolUse","tool_name":"Edit","tool_input":{"file_path":"` + filepath.Join(g.Root, "go.mod") + `"}}`
	req, _ := http.NewRequest("POST", "http://127.0.0.1:"+strconv.Itoa(reg.Port)+"/hook", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+reg.Token)
	req.Header.Set("X-Mentor-Root", reg.Root)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var d gateproto.Decision
	_ = json.NewDecoder(res.Body).Decode(&d)
	res.Body.Close()
	if d.Allow || !strings.Contains(d.Reason, "waits for the developer") {
		t.Errorf("decision = %+v", d)
	}
	req, _ = http.NewRequest("POST", "http://127.0.0.1:"+strconv.Itoa(reg.Port)+"/hook", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer wrong")
	if res, err := http.DefaultClient.Do(req); err != nil || res.StatusCode != 403 {
		t.Error("a wrong token was let in")
	}
	s.Unregister(g.Root)
	if _, err := gateproto.Find(dir, g.Root); err == nil {
		t.Error("still registered")
	}
}

func TestHooks_AnOlderSetupWithoutPostToolUseIsNotInstalled(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
	_ = os.WriteFile(settingsPath(root), []byte(`{"hooks":{"PreToolUse":[{"matcher":"Edit","hooks":[{"type":"command","command":"/x/mentor-gate hook"}]}],"Stop":[{"hooks":[{"type":"command","command":"/x/mentor-gate hook"}]}]}}`), 0o644)
	if HooksInstalled(root) {
		t.Fatal("hooks without PostToolUse count as installed")
	}
	if err := writeHooks(root, "/x/mentor-gate hook", true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(settingsPath(root))
	if !HooksInstalled(root) || !strings.Contains(string(b), `"PostToolUse"`) {
		t.Errorf("settings = %s", b)
	}
}

// TestServer_ClaimWithSeveralEditors: a backend claims a folder no running
// backend has, leaves one another editor's backend has, and takes it over
// when that one has gone.
func TestServer_ClaimWithSeveralEditors(t *testing.T) {
	dir := t.TempDir()
	s, err := Listen(dir, func(string) *Gate { return nil })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	root := t.TempDir()
	read := func() gateproto.Registry {
		b, _ := os.ReadFile(gateproto.RegistryPath(dir, root))
		var r gateproto.Registry
		_ = json.Unmarshal(b, &r)
		return r
	}

	// Another editor's backend, which runs (this test's parent), has it.
	_ = gateproto.Write(dir, gateproto.Registry{Port: 1, Root: root, PID: os.Getppid()})
	_ = s.Claim(root)
	if r := read(); r.PID != os.Getppid() {
		t.Fatalf("a running backend's folder was taken: %+v", r)
	}

	// It has gone: this one takes the folder over.
	_ = gateproto.Write(dir, gateproto.Registry{Port: 1, Root: root, PID: 1 << 30})
	_ = s.Claim(root)
	if r := read(); r.PID != os.Getpid() {
		t.Fatalf("a gone backend's folder was not taken over: %+v", r)
	}
}
