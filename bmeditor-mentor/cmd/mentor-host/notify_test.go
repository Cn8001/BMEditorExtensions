package main

import (
	"encoding/json"
	"io"
	"path/filepath"
	"testing"
	"time"

	"bmeditor-mentor/bmext/host"
	"bmeditor-mentor/internal/agent"
)

// TestAgentNotifies_WhenThePlanComes: the panel hears of a plan the agent
// proposes through the gate, without asking.
func TestAgentNotifies_WhenThePlanComes(t *testing.T) {
	folder, _ := filepath.Abs(filepath.Join("..", "..", "internal", "golang", "testdata", "authsvc"))
	s := host.New()
	st := &state{dataDir: t.TempDir(), manual: true}
	register(s, st)
	toServer, fromClient := io.Pipe()
	toClient, fromServer := io.Pipe()
	go func() { _ = s.Serve(t.Context(), toServer, fromServer); fromServer.Close() }()
	t.Cleanup(func() { fromClient.Close(); st.close() })
	got := make(chan agent.State, 16)
	c := host.NewClient(toClient, fromClient, func(method string, params json.RawMessage) {
		if method == "mentor/agent" {
			var as agent.State
			_ = json.Unmarshal(params, &as)
			got <- as
		}
	})
	if _, err := c.Call(t.Context(), "initialize", host.Context{Folder: folder}); err != nil {
		t.Fatal(err)
	}
	_ = call[agent.State](t, c, "agent.state", nil)

	st.gateFor(folder).Propose("T", []agent.PlanStep{{Title: "S"}})

	deadline := time.After(3 * time.Second)
	for {
		select {
		case as := <-got:
			if as.Waiting == "plan" {
				return
			}
		case <-deadline:
			t.Fatal("no mentor/agent notification with the plan")
		}
	}
}
