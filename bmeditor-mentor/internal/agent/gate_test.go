package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"bmeditor-mentor/internal/golang"
	"bmeditor-mentor/internal/index"
)

type code struct {
	root string
	b    *golang.Builder
	ix   atomic.Pointer[index.Index]
}

func (c *code) Peek() *index.Index { return c.ix.Load() }

// reread reads the files again, as the engine does after edits.
func (c *code) reread(t *testing.T) {
	t.Helper()
	dirs, _ := c.b.Changed()
	if err := c.b.Update(t.Context(), dirs); err != nil {
		t.Fatal(err)
	}
	c.ix.Store(index.Merge(c.root, c.b))
}

func setup(t *testing.T, p Policy) (*Gate, *code) {
	t.Helper()
	root := t.TempDir()
	from, _ := filepath.Abs(filepath.Join("..", "golang", "testdata", "authsvc"))
	if err := os.CopyFS(root, os.DirFS(from)); err != nil {
		t.Fatal(err)
	}
	c := &code{root: root, b: &golang.Builder{Root: root}}
	if err := c.b.Full(t.Context()); err != nil {
		t.Fatal(err)
	}
	c.ix.Store(index.Merge(root, c.b))
	return New(root, c, p, nil), c
}

func edit(g *Gate, rel string) HookCall {
	return HookCall{Event: "PreToolUse", Tool: "Edit", Input: map[string]any{"file_path": filepath.Join(g.Root, rel), "old_string": "x", "new_string": "y"}}
}

func bash(cmd string) HookCall {
	return HookCall{Event: "PreToolUse", Tool: "Bash", Input: map[string]any{"command": cmd}}
}

func write(t *testing.T, g *Gate, rel, add string) {
	t.Helper()
	path := filepath.Join(g.Root, rel)
	b, _ := os.ReadFile(path)
	_ = os.WriteFile(path, append(b, []byte(add)...), 0o644)
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, later, later)
}

func TestGate_StepByStep(t *testing.T) {
	g, c := setup(t, DefaultPolicy())
	ctx := t.Context()

	// Before a plan the agent may look, not change.
	if d := g.Hook(edit(g, "internal/auth/provider.go")); d.Allow || !strings.Contains(d.Reason, "mentor_plan") {
		t.Fatalf("no plan: %+v", d)
	}
	if d := g.Hook(bash("go test ./... && git diff")); !d.Allow {
		t.Fatalf("looking: %+v", d)
	}
	if d := g.Hook(HookCall{Event: "PreToolUse", Tool: "Read", Input: map[string]any{"file_path": "x"}}); !d.Allow {
		t.Fatal("Read was held")
	}

	// It proposes; the plan waits for the developer.
	g.Propose("Add Discord sign-in", []PlanStep{
		{Title: "Add DiscordProvider", Files: []string{"internal/auth/provider.go"}},
		{Title: "Wire it in", Packages: []string{"cmd/server"}},
	})
	if g.State().Waiting != "plan" {
		t.Fatal("the plan should wait")
	}
	if d := g.Hook(edit(g, "internal/auth/provider.go")); d.Allow {
		t.Fatal("an edit before the plan was approved")
	}
	if reply := g.Wait(ctx, 30*time.Millisecond); !strings.Contains(reply, "not answered") {
		t.Fatalf("wait = %q", reply)
	}

	// The developer approves: mentor_wait, already waiting, wakes with step 1.
	got := make(chan string, 1)
	go func() { got <- g.Wait(ctx, 5*time.Second) }()
	time.Sleep(20 * time.Millisecond)
	if err := g.AnswerPlan(ctx, true, ""); err != nil {
		t.Fatal(err)
	}
	if reply := <-got; !strings.Contains(reply, "Do step 1: Add DiscordProvider") {
		t.Fatalf("reply = %q", reply)
	}

	// It does the step, on its own.
	if d := g.Hook(edit(g, "internal/auth/provider.go")); !d.Allow {
		t.Fatalf("step 1: %+v", d)
	}
	write(t, g, "internal/auth/provider.go", "\n// DiscordProvider signs users in with Discord.\ntype DiscordProvider struct{}\n\nfunc (DiscordProvider) Name() string { return \"discord\" }\n\nfunc (DiscordProvider) Exchange(ctx context.Context, code string) (UserIdentity, error) {\n\treturn UserIdentity{Provider: \"discord\", ProviderUserID: code}, nil\n}\n")
	c.reread(t)

	// It cannot end its turn in the middle of a step that changed code.
	if d := g.Hook(HookCall{Event: "Stop"}); d.Allow || !strings.Contains(d.Reason, "mentor_checkpoint") {
		t.Fatalf("stop: %+v", d)
	}
	if d := g.Hook(HookCall{Event: "Stop", StopActive: true}); !d.Allow {
		t.Fatal("a turn continued once by the stop hook must end")
	}

	// The checkpoint: its words beside what the code shows.
	g.OpenCheckpoint(ctx, "Added `DiscordProvider` with `DiscordProvider.Exchange`, like `TwitchProvider`.", "Same shape as SteamProvider.")
	st := g.State()
	if st.Waiting != "checkpoint" || st.Checkpoint.Report == nil || len(st.Checkpoint.Report.Units) == 0 {
		t.Fatalf("checkpoint = %+v", st.Checkpoint)
	}
	if len(st.Checkpoint.Unknown) != 1 || st.Checkpoint.Unknown[0] != "TwitchProvider" {
		t.Errorf("unknown = %v", st.Checkpoint.Unknown)
	}
	if d := g.Hook(edit(g, "cmd/server/main.go")); d.Allow || !strings.Contains(d.Reason, "Checkpoint") {
		t.Fatalf("an edit while the checkpoint waits: %+v", d)
	}

	// Changes asked: the same step goes on.
	if _, err := g.AnswerCheckpoint(ctx, "changes", "Add a test."); err != nil {
		t.Fatal(err)
	}
	if reply := g.Wait(ctx, time.Second); !strings.Contains(reply, "Add a test.") {
		t.Fatalf("reply = %q", reply)
	}
	if g.State().Step != 1 {
		t.Fatal("the step moved on after changes were asked")
	}
	g.OpenCheckpoint(ctx, "Added a test too.", "")
	keys, err := g.AnswerCheckpoint(ctx, "approve", "")
	if err != nil || len(keys) < 3 {
		t.Fatalf("keys = %v, %v", keys, err)
	}
	if reply := g.Wait(ctx, time.Second); !strings.Contains(reply, "Do step 2: Wire it in") {
		t.Fatalf("reply = %q", reply)
	}

	// Step 2, approved without changes: the plan is done.
	if d := g.Hook(edit(g, "cmd/server/main.go")); !d.Allow {
		t.Fatalf("step 2: %+v", d)
	}
	g.OpenCheckpoint(ctx, "Wired it in.", "")
	if _, err := g.AnswerCheckpoint(ctx, "skip", ""); err != nil {
		t.Fatal(err)
	}
	if st := g.State(); st.Plan.Status != "done" || st.Step != 0 {
		t.Errorf("plan = %+v", st.Plan)
	}
	if d := g.Hook(edit(g, "cmd/server/main.go")); d.Allow {
		t.Error("an edit after the plan was done")
	}
}

func TestGate_AStepThatGrowsTooBigEnds(t *testing.T) {
	p := DefaultPolicy()
	p.MaxFiles = 2
	g, _ := setup(t, p)
	ctx := t.Context()
	g.Propose("Many files", []PlanStep{{Title: "Everything"}})
	_ = g.AnswerPlan(ctx, true, "")
	for _, f := range []string{"internal/auth/provider.go", "internal/auth/session.go"} {
		if d := g.Hook(edit(g, f)); !d.Allow {
			t.Fatalf("%s: %+v", f, d)
		}
	}
	if d := g.Hook(edit(g, "internal/auth/repo.go")); d.Allow || !strings.Contains(d.Reason, "mentor_checkpoint") {
		t.Fatalf("a third file: %+v", d)
	}
	if d := g.Hook(edit(g, "internal/auth/session.go")); !d.Allow {
		t.Error("a file the step already changed should stay editable")
	}
	if d := g.Hook(bash("echo hi > note.txt")); !d.Allow {
		t.Error("a command within the step was held")
	}
}

func TestGate_WithoutRequirePlan(t *testing.T) {
	p := DefaultPolicy()
	p.RequirePlan = false
	g, _ := setup(t, p)
	if d := g.Hook(edit(g, "internal/auth/provider.go")); !d.Allow {
		t.Errorf("without a plan required: %+v", d)
	}
	if d := g.Hook(HookCall{Event: "Stop"}); !d.Allow {
		t.Error("no step: the turn may end")
	}
}

func TestGate_EndPlanAndActivity(t *testing.T) {
	g, _ := setup(t, DefaultPolicy())
	ctx := t.Context()
	g.Propose("T", []PlanStep{{Title: "S"}})
	_ = g.AnswerPlan(ctx, true, "")
	g.Hook(edit(g, "internal/auth/provider.go"))
	g.EndPlan()
	st := g.State()
	if st.Plan.Status != "done" || st.Step != 0 {
		t.Errorf("state = %+v", st)
	}
	var done, held bool
	g.Hook(edit(g, "internal/auth/provider.go"))
	for _, e := range g.State().Events {
		if e.Tool == "Edit" && e.Result == "done" {
			done = true
		}
		if e.Tool == "Edit" && e.Result == "held" {
			held = true
		}
	}
	if !done || !held {
		t.Errorf("events = %+v", g.State().Events)
	}
}

func TestGate_KeepsItsStateAcrossARestart(t *testing.T) {
	g, c := setup(t, DefaultPolicy())
	var kept []byte
	g.Persist = func(b []byte) { kept = b }
	g.Propose("Task", []PlanStep{{Title: "One"}, {Title: "Two"}})
	_ = g.AnswerPlan(t.Context(), true, "fine")
	again := New(g.Root, c, DefaultPolicy(), nil)
	if err := again.Unmarshal(kept); err != nil {
		t.Fatal(err)
	}
	if st := again.State(); st.Step != 1 || st.Plan.Status != "approved" {
		t.Errorf("state = %+v", st)
	}
}

func TestGate_Off(t *testing.T) {
	p := DefaultPolicy()
	p.Off = true
	g, _ := setup(t, p)
	if d := g.Hook(edit(g, "internal/auth/provider.go")); !d.Allow {
		t.Errorf("off, an edit was held: %+v", d)
	}
	if text, _ := g.Tool(t.Context(), "mentor_plan", nil); !strings.Contains(text, "Mentor is off") {
		t.Errorf("off, mentor_plan = %q", text)
	}
	if g.State().Level != "off" {
		t.Errorf("level = %s", g.State().Level)
	}
}

func TestGate_RemembersTheFilesTheAgentChanged(t *testing.T) {
	p := DefaultPolicy()
	p.RequirePlan = false
	g, c := setup(t, p)
	var kept []byte
	g.Persist = func(b []byte) { kept = b }
	g.Hook(edit(g, "internal/auth/provider.go"))
	g.Hook(edit(g, "internal/auth/service.go"))
	g.Hook(edit(g, "../elsewhere.go")) // outside the folder
	g.Hook(bash("go test ./..."))

	all := func(string) bool { return true }
	if got := strings.Join(g.Touched(all), ","); got != "internal/auth/provider.go,internal/auth/service.go" {
		t.Fatalf("touched = %s", got)
	}
	// service.go was committed: it no longer differs, and is forgotten.
	got := g.Touched(func(f string) bool { return f != "internal/auth/service.go" })
	if strings.Join(got, ",") != "internal/auth/provider.go" {
		t.Errorf("touched = %v", got)
	}
	if again := g.Touched(all); len(again) != 1 {
		t.Errorf("a forgotten file came back: %v", again)
	}
	g.notify()
	restarted := New(g.Root, c, p, nil)
	if err := restarted.Unmarshal(kept); err != nil {
		t.Fatal(err)
	}
	if got := restarted.Touched(all); len(got) != 1 || got[0] != "internal/auth/provider.go" {
		t.Errorf("after a restart = %v", got)
	}
}

func TestGate_OffRemembersNothing(t *testing.T) {
	p := DefaultPolicy()
	p.Off = true
	g, _ := setup(t, p)
	g.Hook(edit(g, "internal/auth/provider.go"))
	if got := g.Touched(func(string) bool { return true }); len(got) != 0 {
		t.Errorf("off, touched = %v", got)
	}
}

// TestGate_ACommandsChangesAreTheAgents: what differs from HEAD after a
// shell command and did not before it, or held something else, is the
// agent's: a file it removed or moved too. A file already changed by hand
// that the command left alone is not.
func TestGate_ACommandsChangesAreTheAgents(t *testing.T) {
	p := DefaultPolicy()
	p.RequirePlan = false
	g, _ := setup(t, p)
	dirty := map[string]string{"by-hand.go": "h1"}
	g.Dirty = func(context.Context) map[string]string {
		out := map[string]string{}
		for k, v := range dirty {
			out[k] = v
		}
		return out
	}
	run := func(id, cmd string, effect func()) {
		g.Hook(HookCall{Event: "PreToolUse", Tool: "Bash", ToolUseID: id, Input: map[string]any{"command": cmd}})
		effect()
		g.Hook(HookCall{Event: "PostToolUse", Tool: "Bash", ToolUseID: id, Input: map[string]any{"command": cmd}})
	}
	run("1", "rm internal/auth/session.go", func() { dirty["internal/auth/session.go"] = "" })
	run("2", "git mv a.go b.go", func() { dirty["a.go"] = ""; dirty["b.go"] = "h2" })
	run("3", "ls", func() { dirty["by-ls.go"] = "h3" }) // only looks: not watched
	run("4", "gofmt -w by-hand.go", func() { dirty["by-hand.go"] = "h4" })

	got := strings.Join(g.Touched(func(string) bool { return true }), ",")
	if got != "a.go,b.go,by-hand.go,internal/auth/session.go" {
		t.Errorf("touched = %s", got)
	}
}
