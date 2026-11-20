// Package gateproto is what mentor-gate, the program Claude Code runs,
// and the extension's backend agree on: where the backend listens for a
// folder, the MCP tools, and what a hook answers. It is small, so the gate
// stays small and quick to start.
package gateproto

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Registry is where the backend of a folder listens.
type Registry struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
	Root  string `json:"root"`
	PID   int    `json:"pid"`
}

// Hash names a folder's registry file.
func Hash(root string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(root)))
	return hex.EncodeToString(sum[:])[:16]
}

// RegistryPath is the file in dir that says where the backend of root
// listens.
func RegistryPath(dir, root string) string { return filepath.Join(dir, Hash(root)+".json") }

// Write keeps r for root in dir, readable by this user only.
func Write(dir string, r Registry) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, _ := json.Marshal(r)
	tmp := RegistryPath(dir, r.Root) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, RegistryPath(dir, r.Root))
}

// ErrNotFound says no backend watches the folder or any above it.
var ErrNotFound = errors.New("mentor is not running for this folder")

// Find is the registry of the nearest folder at or above dir that has one:
// an agent working in a subfolder is still in the project.
func Find(regDir, dir string) (Registry, error) {
	dir = filepath.Clean(dir)
	for {
		b, err := os.ReadFile(RegistryPath(regDir, dir))
		if err == nil {
			var r Registry
			if json.Unmarshal(b, &r) == nil && r.Port > 0 {
				return r, nil
			}
		}
		up := filepath.Dir(dir)
		if up == dir {
			return Registry{}, ErrNotFound
		}
		dir = up
	}
}

// Decision is the backend's answer to a hook.
type Decision struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason,omitempty"`
}

// ToolCall is an MCP tool call passed on to the backend.
type ToolCall struct {
	Root string          `json:"root"`
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

// ToolResult is the text the agent reads back.
type ToolResult struct {
	Text    string `json:"text"`
	IsError bool   `json:"isError,omitempty"`
}

// Instructions are what the agent is told about Mentor when it connects.
const Instructions = `Mentor lets the developer follow your work step by step, in the editor. Work on your own, but in steps they can review:
- Before changing code, call mentor_plan with the task and its steps, each a logical part of the work (a data model, a service, wiring it in, its tests) with the files or packages it touches. Then call mentor_wait until they answer.
- Do one step. When it is done, call mentor_checkpoint with what you did and why, naming the types and functions in backticks, then mentor_wait. The developer reads it beside the code that changed; when they approve, mentor_wait tells you to go on.
- If mentor_wait says they have not answered yet, call it again. Change nothing while you wait.
- When Mentor holds an edit, read why and do what it says.`

// Tool is an MCP tool's definition.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

func strs(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}

func object(props map[string]any, required ...string) map[string]any {
	o := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		o["required"] = required
	}
	return o
}

// Tools are the MCP tools the agent has.
var Tools = []Tool{
	{
		Name:        "mentor_plan",
		Description: "Propose your plan before changing code: the task, and its steps in order, each a logical part the developer can review on its own, with the files and packages (folders from the project root) it touches. Then call mentor_wait.",
		InputSchema: object(map[string]any{
			"task": str("What you were asked to do, in a sentence."),
			"steps": map[string]any{"type": "array", "description": "The steps, in order.",
				"items": object(map[string]any{
					"title":    str("What the step does."),
					"why":      str("Why it is needed, or why this way."),
					"files":    strs("Files it changes or adds, from the project root."),
					"packages": strs("Folders it changes, from the project root."),
				}, "title")},
		}, "task", "steps"),
	},
	{
		Name:        "mentor_checkpoint",
		Description: "End the current step: what you did and how it works now, and why this way, naming the types and functions in backticks. The developer reviews it beside the code that changed. Then call mentor_wait.",
		InputSchema: object(map[string]any{
			"summary": str("What the step changed, and how it works now."),
			"why":     str("Why this way; what you chose and what you left out."),
		}, "summary"),
	},
	{
		Name:        "mentor_wait",
		Description: "Wait for the developer's answer to your plan or checkpoint, and read what to do next. It returns after a while if they have not answered yet; then call it again.",
		InputSchema: object(map[string]any{}),
	},
	{
		Name:        "mentor_status",
		Description: "Where the plan is, and what to do now.",
		InputSchema: object(map[string]any{}),
	},
}

// HookOutput is what the gate prints for Claude Code: nothing to let the
// agent go on as it would anyway, or what holds it, with the reason.
func HookOutput(event string, d Decision) []byte {
	if d.Allow {
		return nil
	}
	var v any
	if event == "Stop" || event == "SubagentStop" {
		v = map[string]any{"decision": "block", "reason": d.Reason}
	} else {
		v = map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": d.Reason}}
	}
	b, _ := json.Marshal(v)
	return b
}
