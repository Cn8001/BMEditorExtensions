package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"bmeditor-powershell/internal/pses"
	"bmeditor-powershell/internal/rpc"
)

// debugAdapter runs Editor Services' debug adapter, which serves on a
// named pipe, and relays the editor's stdin and stdout to it. What the
// script prints reaches Editor Services' own stdout and stderr, not the
// pipe: it goes to the editor as output events.
func debugAdapter(shell string, b pses.Bundle, m pses.Mode) int {
	m.Pipe = fmt.Sprintf("BMEditor_PSES_Debug_%d", os.Getpid())
	_ = os.Remove(m.Session)
	out := rpc.NewWriter(os.Stdout)
	cmd := pses.Command(shell, b, m)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	hide(cmd)
	if err := cmd.Start(); err != nil {
		body, _ := rpc.Read(bufio.NewReader(os.Stdin))
		answerFailure(out, body, "cannot start PowerShell: "+err.Error())
		return 1
	}
	defer func() { _ = cmd.Process.Kill() }()
	var exited sync.WaitGroup
	exited.Add(2)
	go relayText(stdout, out, "stdout", &exited)
	go relayText(stderr, out, "stderr", &exited)
	died := make(chan error, 1)
	go func() {
		exited.Wait()
		died <- cmd.Wait()
	}()

	// The editor's requests, from now on: initialize comes before the pipe
	// is up, and waits for it.
	in := make(chan []byte, 64)
	go func() {
		r := bufio.NewReader(os.Stdin)
		for {
			body, err := rpc.Read(r)
			if err != nil {
				close(in)
				return
			}
			in <- body
		}
	}()

	pipe, err := waitForPipe(m.Session, died)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		answerFailure(out, <-in, err.Error())
		return 1
	}
	defer pipe.Close()
	pw := rpc.NewWriter(pipe)
	done := make(chan struct{})
	go func() {
		defer close(done)
		r := bufio.NewReader(pipe)
		for {
			body, err := rpc.Read(r)
			if err != nil {
				return
			}
			_ = out.Raw(body)
		}
	}()
	for {
		select {
		case body, ok := <-in:
			if !ok {
				return 0 // the editor ended the session
			}
			if err := pw.Raw(body); err != nil {
				return 0
			}
		case <-done:
			time.Sleep(200 * time.Millisecond) // the last output events
			return 0
		case <-died:
			return 0
		}
	}
}

// waitForPipe waits for Editor Services to write in its session details
// that it started, and connects to its debug adapter's pipe.
func waitForPipe(session string, died <-chan error) (io.ReadWriteCloser, error) {
	deadline := time.After(90 * time.Second)
	for {
		select {
		case err := <-died:
			return nil, fmt.Errorf("PowerShell Editor Services stopped before it started: %v", err)
		case <-deadline:
			return nil, errors.New("PowerShell Editor Services did not start in 90 seconds")
		case <-time.After(150 * time.Millisecond):
		}
		raw, err := os.ReadFile(session)
		if err != nil {
			continue
		}
		var s struct {
			Status               string `json:"status"`
			Reason               string `json:"reason"`
			Details              string `json:"details"`
			DebugServicePipeName string `json:"debugServicePipeName"`
		}
		if json.Unmarshal(raw, &s) != nil {
			continue // half written
		}
		if s.Status != "started" {
			return nil, fmt.Errorf("PowerShell Editor Services did not start: %s %s %s", s.Status, s.Reason, s.Details)
		}
		return dialPipe(s.DebugServicePipeName)
	}
}

// relayText sends what Editor Services prints to the editor's Debug
// Console, as output events.
func relayText(r io.Reader, out *rpc.Writer, category string, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, 8192)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			text := strings.ToValidUTF8(string(buf[:n]), "?")
			_ = out.JSON(map[string]any{"seq": 0, "type": "event", "event": "output", "body": map[string]any{"category": category, "output": text}})
		}
		if err != nil {
			return
		}
	}
}

// answerFailure answers a request (initialize) with a failure.
func answerFailure(out *rpc.Writer, body []byte, why string) {
	if body == nil {
		return
	}
	var req struct {
		Seq     int    `json:"seq"`
		Command string `json:"command"`
	}
	_ = json.Unmarshal(body, &req)
	_ = out.JSON(map[string]any{"seq": 1, "type": "response", "request_seq": req.Seq, "command": req.Command, "success": false, "message": why})
}
