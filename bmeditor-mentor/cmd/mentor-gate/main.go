// Command mentor-gate is what Claude Code runs for Mentor: an MCP server
// with the tools through which the agent proposes its plan and opens
// checkpoints ("mentor-gate mcp"), and a hook before each edit and command
// and at the end of each turn, which holds the agent while a checkpoint
// waits ("mentor-gate hook"). It asks the extension's backend in the
// editor, found through a file beside this program. When the editor does
// not run Mentor for the folder, it lets everything through: Mentor
// watches the folder, it does not lock it.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"bmeditor-mentor/internal/gateproto"
)

const version = "0.0.1"

func main() {
	switch {
	case len(os.Args) == 2 && os.Args[1] == "hook":
		os.Stdout.Write(hook(os.Stdin))
	case len(os.Args) == 2 && os.Args[1] == "mcp":
		serveMCP(os.Stdin, os.Stdout)
	default:
		fmt.Fprintln(os.Stderr, "mentor-gate "+version+": usage: mentor-gate hook|mcp")
		os.Exit(2)
	}
}

// regDir is where the backend says where it listens: beside this program,
// or MENTOR_GATE_DIR.
func regDir() string {
	if d := os.Getenv("MENTOR_GATE_DIR"); d != "" {
		return d
	}
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return filepath.Dir(exe)
}

// project is the folder Claude Code works in.
func project(cwd string) string {
	if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" {
		return d
	}
	if cwd != "" {
		return cwd
	}
	d, _ := os.Getwd()
	return d
}

func post(reg gateproto.Registry, path string, body []byte, timeout time.Duration) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+strconv.Itoa(reg.Port)+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+reg.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mentor-Root", reg.Root)
	res, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mentor: %s", res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, 16<<20))
}

// hook answers one of Claude Code's hooks: nothing to let it go on, or a
// denial with what to do instead.
func hook(in io.Reader) []byte {
	body, err := io.ReadAll(io.LimitReader(in, 16<<20))
	if err != nil {
		return nil
	}
	var call struct {
		Event string `json:"hook_event_name"`
		Cwd   string `json:"cwd"`
	}
	_ = json.Unmarshal(body, &call)
	reg, err := gateproto.Find(regDir(), project(call.Cwd))
	if err != nil {
		return nil
	}
	out, err := post(reg, "/hook", body, 20*time.Second)
	if err != nil {
		return nil // the editor closed: let the agent go on
	}
	var d gateproto.Decision
	if json.Unmarshal(out, &d) != nil {
		return nil
	}
	return gateproto.HookOutput(call.Event, d)
}

type rpc struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// serveMCP is an MCP server over stdio, a JSON message a line, whose
// tools the backend answers.
func serveMCP(in io.Reader, out io.Writer) {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 64*1024), 16<<20)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		var m rpc
		if json.Unmarshal(sc.Bytes(), &m) != nil || len(m.ID) == 0 {
			continue // a notification, such as notifications/initialized
		}
		res := rpc{JSONRPC: "2.0", ID: m.ID}
		switch m.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if p.ProtocolVersion == "" {
				p.ProtocolVersion = "2025-06-18"
			}
			res.Result = map[string]any{
				"protocolVersion": p.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
				"serverInfo":      map[string]any{"name": "mentor", "version": version},
				"instructions":    gateproto.Instructions,
			}
		case "ping":
			res.Result = map[string]any{}
		case "tools/list":
			res.Result = map[string]any{"tools": gateproto.Tools}
		case "tools/call":
			res.Result = callTool(m.Params)
		default:
			res.Error = &rpcError{Code: -32601, Message: "no method " + m.Method}
		}
		_ = enc.Encode(res)
	}
}

func callTool(params json.RawMessage) map[string]any {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	_ = json.Unmarshal(params, &p)
	text, isErr := "", false
	reg, err := gateproto.Find(regDir(), project(""))
	if err != nil {
		text = "Mentor is not running in the editor for this folder; carry on as the developer told you."
	} else {
		body, _ := json.Marshal(gateproto.ToolCall{Root: reg.Root, Tool: p.Name, Args: p.Arguments})
		out, err := post(reg, "/tool", body, 90*time.Second)
		var r gateproto.ToolResult
		switch {
		case err != nil:
			text = "Mentor did not answer (" + err.Error() + "); carry on as the developer told you."
		case json.Unmarshal(out, &r) != nil:
			text, isErr = "Mentor's answer did not read.", true
		default:
			text, isErr = r.Text, r.IsError
		}
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isErr}
}
