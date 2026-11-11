// Command bat-run is a debug adapter that only runs: F5 on a .bat or .cmd
// file runs it with cmd, its output in the Debug Console, and Stop ends
// it and what it started. cmd has no debugger, so there are no
// breakpoints or steps.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"bmeditor-powershell/internal/rpc"
)

type request struct {
	Seq       int             `json:"seq"`
	Command   string          `json:"command"`
	Arguments json.RawMessage `json:"arguments"`
}

type launch struct {
	Program string            `json:"program"`
	Cwd     string            `json:"cwd"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

type adapter struct {
	out  *rpc.Writer
	seq  int
	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan struct{}
}

func main() {
	a := &adapter{out: rpc.NewWriter(os.Stdout)}
	r := bufio.NewReader(os.Stdin)
	for {
		body, err := rpc.Read(r)
		if err != nil {
			a.stop()
			return
		}
		var req request
		if json.Unmarshal(body, &req) != nil {
			continue
		}
		a.handle(req)
		if req.Command == "disconnect" {
			return
		}
	}
}

func (a *adapter) send(v map[string]any) {
	a.mu.Lock()
	a.seq++
	v["seq"] = a.seq
	a.mu.Unlock()
	_ = a.out.JSON(v)
}

func (a *adapter) respond(req request, body any, failure string) {
	m := map[string]any{"type": "response", "request_seq": req.Seq, "command": req.Command, "success": failure == ""}
	if failure != "" {
		m["message"] = failure
	}
	if body != nil {
		m["body"] = body
	}
	a.send(m)
}

func (a *adapter) event(name string, body any) {
	m := map[string]any{"type": "event", "event": name}
	if body != nil {
		m["body"] = body
	}
	a.send(m)
}

func (a *adapter) output(category, text string) {
	a.event("output", map[string]any{"category": category, "output": text})
}

func (a *adapter) handle(req request) {
	switch req.Command {
	case "initialize":
		a.respond(req, map[string]any{"supportsConfigurationDoneRequest": true, "supportsTerminateRequest": true}, "")
		a.event("initialized", nil)
	case "launch":
		var l launch
		_ = json.Unmarshal(req.Arguments, &l)
		if err := a.start(l); err != nil {
			a.respond(req, nil, err.Error())
			return
		}
		a.respond(req, nil, "")
	case "configurationDone", "setBreakpoints", "setExceptionBreakpoints", "setFunctionBreakpoints":
		var body any
		if req.Command == "setBreakpoints" {
			body = map[string]any{"breakpoints": []any{}}
		}
		a.respond(req, body, "")
	case "threads":
		a.respond(req, map[string]any{"threads": []any{map[string]any{"id": 1, "name": "cmd"}}}, "")
	case "terminate", "disconnect":
		a.stop()
		a.respond(req, nil, "")
	default:
		a.respond(req, nil, "cmd has no debugger: "+req.Command+" is not supported")
	}
}

func (a *adapter) start(l launch) error {
	if l.Program == "" {
		return fmt.Errorf("no batch file to run")
	}
	cwd := l.Cwd
	if cwd == "" {
		cwd = filepath.Dir(l.Program)
	}
	cmd, err := command(l.Program, l.Args)
	if err != nil {
		return err
	}
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	keys := make([]string, 0, len(l.Env))
	for k := range l.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		cmd.Env = append(cmd.Env, k+"="+l.Env[k])
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	a.cmd, a.done = cmd, make(chan struct{})
	a.event("process", map[string]any{"name": l.Program, "systemProcessId": cmd.Process.Pid, "startMethod": "launch"})
	var wg sync.WaitGroup
	wg.Add(2)
	go a.relay(stdout, "stdout", &wg)
	go a.relay(stderr, "stderr", &wg)
	go func() {
		wg.Wait()
		err := cmd.Wait()
		code := 0
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		} else if err != nil {
			code = 1
		}
		a.event("exited", map[string]any{"exitCode": code})
		a.event("terminated", nil)
		close(a.done)
	}()
	return nil
}

func (a *adapter) relay(r io.Reader, category string, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, 8192)
	pending := []byte{}
	for {
		n, err := r.Read(buf)
		pending = append(pending, buf[:n]...)
		// Hold back a character the read cut in two.
		cut := len(pending)
		if err == nil {
			for i := len(pending) - 1; i >= 0 && i >= len(pending)-3; i-- {
				if utf8.RuneStart(pending[i]) {
					if !utf8.FullRune(pending[i:]) {
						cut = i
					}
					break
				}
			}
		}
		if cut > 0 {
			a.output(category, strings.ToValidUTF8(string(pending[:cut]), "?"))
			pending = append([]byte{}, pending[cut:]...)
		}
		if err != nil {
			return
		}
	}
}

func (a *adapter) stop() {
	if a.cmd == nil || a.cmd.Process == nil {
		return
	}
	select {
	case <-a.done:
		return
	default:
	}
	kill(a.cmd)
}
