// Command powershell-host is the PowerShell extension's backend: it says
// whether PowerShell Editor Services is installed, and installs it.
package main

import (
	"context"
	"encoding/json"
	"sync"

	"bmeditor-powershell/bmext/host"
	"bmeditor-powershell/internal/pses"
)

type status struct {
	Installed bool        `json:"installed"`
	Bundle    pses.Bundle `json:"bundle"`
	Shell     string      `json:"shell,omitempty"`
	ShellErr  string      `json:"shellError,omitempty"`
}

func current() status {
	b, ok := pses.Find()
	st := status{Installed: ok, Bundle: b}
	if sh, err := pses.Shell(); err != nil {
		st.ShellErr = err.Error()
	} else {
		st.Shell = sh
	}
	return st
}

func main() {
	s := host.New()
	var installing sync.Mutex
	s.Handle("status", func(context.Context, json.RawMessage) (any, error) {
		return current(), nil
	})
	// install downloads Editor Services; each step goes to the frontend as
	// pses/progress, for Output.
	s.Handle("install", func(ctx context.Context, _ json.RawMessage) (any, error) {
		if !installing.TryLock() {
			return nil, &host.Error{Code: "busy", Message: "Editor Services is being installed already"}
		}
		defer installing.Unlock()
		_, err := pses.Install(ctx, func(line string) { _ = s.Notify("pses/progress", map[string]string{"line": line}) })
		if err != nil {
			return nil, err
		}
		return current(), nil
	})
	host.Run(s)
}
