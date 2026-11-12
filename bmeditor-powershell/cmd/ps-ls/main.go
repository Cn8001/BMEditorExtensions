// Command ps-ls starts PowerShell Editor Services for BMEditor: as the
// language server of PowerShell files, or with --debug as their debug
// adapter. Without Editor Services installed it answers the first request
// with how to install it, so the editor says so.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"bmeditor-powershell/internal/pses"
	"bmeditor-powershell/internal/rpc"
)

const missing = "PowerShell Editor Services is not installed: run PowerShell: Install Editor Services from the command palette (Ctrl+Shift+P)"

func main() {
	debug := len(os.Args) > 1 && os.Args[1] == "--debug"
	b, found := pses.Find()
	shell, err := pses.Shell()
	why := ""
	switch {
	case err != nil:
		why = err.Error()
	case !found:
		why = missing
	}
	if why != "" {
		fmt.Fprintln(os.Stderr, why)
		refuse(debug, why)
		os.Exit(1)
	}
	logs, err := logDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	m := pses.Mode{Debug: debug, Session: filepath.Join(logs, "session.json"), LogDir: logs, HostName: "BMEditor"}
	fmt.Fprintf(os.Stderr, "PowerShell Editor Services %s (%s) in %s\n", b.Version, b.Dir, shell)
	if debug {
		os.Exit(debugAdapter(shell, b, m))
	}
	cmd := pses.Command(shell, b, m)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Editor Services:", err)
		os.Exit(1)
	}
}

// logDir is a folder of this run's own, for Editor Services' log and the
// session details it writes.
func logDir() (string, error) {
	home, err := pses.Home()
	if err != nil {
		return "", err
	}
	logs := filepath.Join(home, "logs")
	// One folder per run: the ones of past days are no use to anyone.
	entries, _ := os.ReadDir(logs)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > 72*time.Hour {
			_ = os.RemoveAll(filepath.Join(logs, e.Name()))
		}
	}
	dir := filepath.Join(logs, strconv.Itoa(os.Getpid()))
	return dir, os.MkdirAll(dir, 0o755)
}

// refuse answers the client's first request with why nothing can serve it.
func refuse(debug bool, why string) {
	r := bufio.NewReader(os.Stdin)
	w := rpc.NewWriter(os.Stdout)
	body, err := rpc.Read(r)
	if err != nil {
		return
	}
	if debug {
		var req struct {
			Seq     int    `json:"seq"`
			Command string `json:"command"`
		}
		_ = json.Unmarshal(body, &req)
		_ = w.JSON(map[string]any{"seq": 1, "type": "response", "request_seq": req.Seq, "command": req.Command, "success": false, "message": why})
		return
	}
	var m rpc.Message
	_ = json.Unmarshal(body, &m)
	if m.ID != nil {
		_ = w.JSON(rpc.Message{JSONRPC: "2.0", ID: m.ID, Error: &rpc.Error{Code: -32603, Message: why}})
	}
}
