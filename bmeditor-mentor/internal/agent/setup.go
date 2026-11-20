package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// GateName is the gate program's file name on this system.
func GateName() string {
	if runtime.GOOS == "windows" {
		return "mentor-gate.exe"
	}
	return "mentor-gate"
}

// InstallGate copies the gate program shipped beside the backend into
// dir, where Claude Code runs it from: a path that stays when the
// extension updates. It answers the path.
func InstallGate(dir string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	bin := filepath.Dir(exe)
	suffix := "-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		suffix += ".exe"
	}
	var src string
	for _, name := range []string{"mentor-gate" + suffix, GateName()} {
		if _, err := os.Stat(filepath.Join(bin, name)); err == nil {
			src = filepath.Join(bin, name)
			break
		}
	}
	if src == "" {
		return "", errors.New("the gate program is missing beside " + exe)
	}
	dst := filepath.Join(dir, GateName())
	if same(src, dst) {
		return dst, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return "", err
	}
	out.Close()
	return dst, os.Rename(tmp, dst)
}

func same(a, b string) bool {
	x, err1 := os.ReadFile(a)
	y, err2 := os.ReadFile(b)
	return err1 == nil && err2 == nil && bytes.Equal(x, y)
}

// settingsPath is the project's Claude Code settings this machine keeps
// to itself: never the shared settings.json.
func settingsPath(root string) string { return filepath.Join(root, ".claude", "settings.local.json") }

func quote(path string) string {
	if strings.ContainsAny(path, " \t'\"") {
		return `"` + path + `"`
	}
	return path
}

func ours(cmd string) bool { return strings.Contains(cmd, "mentor-gate") }

// hookEvents are the agent's hooks the gate answers, with the tools each
// is for: before an edit or a command, after a command (what it changed),
// and at the end of a turn.
var hookEvents = []struct{ event, matcher string }{
	{"PreToolUse", "Edit|Write|MultiEdit|NotebookEdit|Bash"},
	{"PostToolUse", "Bash"},
	{"Stop", ""},
}

// HooksInstalled says whether the project's settings run the gate for
// each of its hooks; settings from an older Mentor lack some.
func HooksInstalled(root string) bool {
	b, err := os.ReadFile(settingsPath(root))
	if err != nil || !bytes.Contains(b, []byte("mentor-gate")) {
		return false
	}
	var settings struct {
		Hooks map[string][]any `json:"hooks"`
	}
	if json.Unmarshal(b, &settings) != nil {
		return false
	}
	for _, h := range hookEvents {
		found := false
		for _, entry := range settings.Hooks[h.event] {
			found = found || mentions(entry)
		}
		if !found {
			return false
		}
	}
	return true
}

// Setup is what Connect did, for the panel to show.
type Setup struct {
	HooksFile  string `json:"hooksFile"`
	Hooks      bool   `json:"hooks"`
	MCP        bool   `json:"mcp"`
	MCPCommand string `json:"mcpCommand"` // to run by hand when it could not
	Detail     string `json:"detail,omitempty"`
}

// Connect sets Claude Code up for root: Mentor's MCP tools, through which
// the agent proposes its plan and opens checkpoints, and the hooks that
// make a checkpoint hold, in the project's local settings. Claude Code
// started in the folder, from the editor's panel or a terminal, uses
// both, whatever its permission mode.
func Connect(ctx context.Context, root, gate string) Setup {
	s := Setup{HooksFile: settingsPath(root), MCPCommand: "claude mcp add --scope local mentor -- " + quote(gate) + " mcp"}
	if err := writeHooks(root, quote(gate)+" hook", true); err != nil {
		s.Detail = "Could not write " + s.HooksFile + ": " + err.Error()
		return s
	}
	s.Hooks = true
	_, _ = claude(ctx, root, "mcp", "remove", "--scope", "local", "mentor")
	if out, err := claude(ctx, root, "mcp", "add", "--scope", "local", "mentor", "--", gate, "mcp"); err != nil {
		s.Detail = "Could not add Mentor's MCP server (" + strings.TrimSpace(out+" "+err.Error()) + "). Run the command below in the folder."
	} else {
		s.MCP = true
	}
	return s
}

// Disconnect takes Mentor out of Claude Code for root.
func Disconnect(ctx context.Context, root string) error {
	_, _ = claude(ctx, root, "mcp", "remove", "--scope", "local", "mentor")
	return writeHooks(root, "", false)
}

// writeHooks puts the gate's hooks in the settings, or takes them out,
// keeping everything else there.
func writeHooks(root, command string, add bool) error {
	path := settingsPath(root)
	settings := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &settings); err != nil {
			return errors.New("it is not valid JSON")
		}
	} else if !add {
		return nil
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	for _, h := range hookEvents {
		event := h.event
		list, _ := hooks[event].([]any)
		var kept []any
		for _, m := range list {
			if !mentions(m) {
				kept = append(kept, m)
			}
		}
		if add {
			entry := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command, "timeout": 30}}}
			if h.matcher != "" {
				entry["matcher"] = h.matcher
			}
			kept = append(kept, entry)
		}
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(settings, "", "  ")
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// mentions says whether a hook entry runs the gate.
func mentions(entry any) bool {
	m, _ := entry.(map[string]any)
	list, _ := m["hooks"].([]any)
	for _, h := range list {
		if hm, ok := h.(map[string]any); ok {
			if cmd, _ := hm["command"].(string); ours(cmd) {
				return true
			}
		}
	}
	return false
}

// claude runs the Claude Code CLI in root: from PATH, else through the
// login shell, where the editor's Claude panel finds it.
func claude(ctx context.Context, root string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	if p, err := exec.LookPath("claude"); err == nil {
		cmd = exec.CommandContext(ctx, p, args...)
	} else if runtime.GOOS != "windows" {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		line := "claude"
		for _, a := range args {
			line += " '" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
		cmd = exec.CommandContext(ctx, shell, "-lc", line)
	} else {
		return "", errors.New("claude is not on the PATH")
	}
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return string(out), err
}
