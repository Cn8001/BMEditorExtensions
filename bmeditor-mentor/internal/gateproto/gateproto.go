// Package gateproto is what mentor-gate, the program the agent runs,
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
const Instructions = `Mentor lets the developer follow your work step by step, in the editor, and keep the project in their head. Work on your own, but in steps they can place in their map of the project:
- Before changing code, call mentor_plan with the task and its steps. A step is one behavior the developer can understand on its own (storing users, issuing tokens), not one function and not a whole feature: usually 2 to 4 files besides tests. For each step say what it does, the existing code it uses, what it creates, what it will not touch, and when it is done. Then call mentor_wait until they answer.
- Do one step, and keep to what it said: Mentor holds an edit outside it, and ends a step that grows well past the files it named.
- When it is done, call mentor_checkpoint: what you made, in a sentence; where it sits in the architecture, in your words (its layer, what calls it, what it calls, why there); and the one thing the developer needs to know of it (the rule it follows, where to start to change it), naming types and functions in backticks. The developer keeps the shape of the project in mind from this, not from reading every line. Then mentor_wait. Mentor shows where it fits in the project and runs its tests; when they approve, mentor_wait tells you to go on.
- If mentor_wait says they have not answered yet, call it again. Change nothing while you wait.
- If mentor_wait gives you the developer's question, answer it with mentor_answer, then mentor_wait again: they are learning the code from you.
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
		Description: "Propose your plan before changing code: the task, and its steps in order, each one behavior the developer can understand on its own, usually 2 to 4 files besides tests. For each say what it does, what existing code it uses, what it creates, what it will not touch and when it is done. Then call mentor_wait.",
		InputSchema: object(map[string]any{
			"task": str("What you were asked to do, in a sentence."),
			"steps": map[string]any{"type": "array", "description": "The steps, in order.",
				"items": object(map[string]any{
					"title":    str("The step, in a few words: User repository."),
					"does":     str("What it will do, as a behavior: store users and look them up by ID."),
					"uses":     strs("Existing types and functions it builds on, by name: User, Repository."),
					"creates":  strs("Types and functions it will add, by name: UserRepository."),
					"avoid":    strs("What it will not touch: files, folders (internal/store/), globs (*.sql), or types by name (SteamProvider)."),
					"doneWhen": str("When it stops: users are stored and found, with tests."),
					"why":      str("Why it is needed, or why this way."),
					"files":    strs("Files it changes or adds, from the project root; its budget comes from them."),
					"packages": strs("Folders it changes, from the project root."),
				}, "title", "does", "doneWhen")},
		}, "task", "steps"),
	},
	{
		Name:        "mentor_checkpoint",
		Description: "End the current step. Say what you made, in a sentence, and the one thing the developer needs to know of it, naming types and functions in backticks. Mentor shows where it fits in the project and runs its tests. Then call mentor_wait.",
		InputSchema: object(map[string]any{
			"made":    str("What the step made, in a sentence: a `UserRepository` that stores users and finds them by ID."),
			"fits":    str("Where it sits in the architecture, in your words: its layer, what calls it, what it calls, and why there rather than elsewhere."),
			"know":    str("The one thing the developer needs to know of it: the rule it follows, or where to start to change it."),
			"summary": str("How it works now, if more than that is worth saying."),
			"why":     str("Why this way; what you chose and what you left out."),
		}, "made", "fits", "know"),
	},
	{
		Name:        "mentor_wait",
		Description: "Wait for the developer's answer to your plan or checkpoint, and read what to do next. It returns after a while if they have not answered yet; then call it again.",
		InputSchema: object(map[string]any{}),
	},
	{
		Name:        "mentor_answer",
		Description: "Answer the developer's question about your plan or checkpoint, which mentor_wait gave you: in plain words, from what the code is, naming types and functions in backticks. Change nothing. Then call mentor_wait.",
		InputSchema: object(map[string]any{
			"answer": str("Your answer to the developer's question."),
		}, "answer"),
	},
	{
		Name:        "mentor_status",
		Description: "Where the plan is, and what to do now.",
		InputSchema: object(map[string]any{}),
	},
}

// HookOutput is what the gate prints for the agent: nothing to let the
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
