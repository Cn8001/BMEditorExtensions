package agent

import (
	"context"
	"os"
	"os/exec"
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
	g := New(root, c, p, nil)
	g.RunTests = func(context.Context, string, []string) TestRun { return TestRun{State: "passed", Failed: []string{}} }
	return g, c
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
	g.OpenCheckpoint(ctx, "", "", "", "Added `DiscordProvider` with `DiscordProvider.Exchange`, like `TwitchProvider`.", "Same shape as SteamProvider.")
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
	g.OpenCheckpoint(ctx, "", "", "", "Added a test too.", "")
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
	g.OpenCheckpoint(ctx, "", "", "", "Wired it in.", "")
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

func TestReadOnly(t *testing.T) {
	cases := map[string]bool{
		`ls -la`:                                true,
		`grep -n '"create"\|NewWindow' a.go`:    true, // a | inside quotes
		`grep -rn "x|y" . 2>/dev/null`:          true,
		`go test ./... 2>&1`:                    true,
		`cd /tmp/x && git status --short`:       true,
		"git diff HEAD | head -20":              true,
		`grep a b.go > out.txt`:                 false,
		`echo hi >> notes`:                      false,
		`ls; rm -rf build`:                      false,
		`cd x && rm a.go`:                       false,
		`cat $(rm a.go)`:                        false,
		"echo `touch x`":                        false,
		`find . -name '*.tmp' -delete`:          false,
		`find . -exec rm {} \;`:                 false,
		`find . -name '*.go'`:                   true,
		`sed -i 's/a/b/' x.go`:                  false,
		`grep -e '>' x.go`:                      true, // > inside quotes
		`W=1 grep x y`:                          false,
		`sed -n '1,80p' gate.go`:                true,
		`sed -e 's/a/b/' x.go`:                  true,
		`sed -ni 's/a/b/p' x.go`:                false,
		`sed --in-place 's/a/b/' x.go`:          false,
		`sed -n 's/a/b/w out.txt' x.go`:         false,
		`awk '{print $1}' x.go`:                 true,
		`awk '{print > "out"}' x.go`:            false,
		`awk 'BEGIN{system("rm x")}'`:           false,
		`awk '{print | "sh"}' x`:                false,
		"cmp -s a b && echo same":               true,
		`git grep -n Provider`:                  true,
		`ps -eo pid,args`:                       true,
		"grep -c x a.go | cut -d: -f1 | tr a b": true,
	}
	for cmd, want := range cases {
		if got := readOnly(cmd); got != want {
			t.Errorf("readOnly(%q) = %v, want %v", cmd, got, want)
		}
	}
}

// TestGate_AStepSaysWhatItWillDo: before a step, the agent says what it
// will do, use, make and leave alone, and when it stops; names it uses that
// the code lacks, and ones it makes that the code has, are flagged.
func TestGate_AStepSaysWhatItWillDo(t *testing.T) {
	g, _ := setup(t, DefaultPolicy())
	g.Propose("Tokens", []PlanStep{{
		Title: "Token issuing", Does: "JWT token issuing", Uses: []string{"UserIdentity", "Repository", "Ghost"},
		Creates: []string{"TokenService", "Session"}, Avoid: []string{"internal/store", "SteamProvider"},
		DoneWhen: "tokens are issued and tested", Files: []string{"internal/auth/token.go", "internal/auth/token_test.go"},
	}})
	st := g.State().Plan.Steps[0]
	if st.Does != "JWT token issuing" || st.DoneWhen == "" || len(st.Avoid) != 2 {
		t.Errorf("step = %+v", st)
	}
	if strings.Join(st.Unknown, ",") != "Ghost" || strings.Join(st.Exists, ",") != "Session" {
		t.Errorf("unknown = %v, exists = %v", st.Unknown, st.Exists)
	}
	if st.Budget != 2 {
		t.Errorf("budget = %d: a test file does not count, and two at least", st.Budget)
	}
}

// TestGate_TheStepStaysWhatItSaid: an edit where the step said it would not
// go, or past its budget, ends the step; tests do not count.
func TestGate_TheStepStaysWhatItSaid(t *testing.T) {
	p := DefaultPolicy()
	p.MaxFiles = 0
	g, _ := setup(t, p)
	g.Propose("Tokens", []PlanStep{{Title: "Tokens", Files: []string{"internal/auth/token.go"}, Avoid: []string{"internal/store/", "*.sql"}}})
	_ = g.AnswerPlan(t.Context(), true, "")
	if d := g.Hook(edit(g, "internal/store/pg.go")); d.Allow || !strings.Contains(d.Reason, "leave internal/store/ alone") {
		t.Fatalf("an avoided folder: %+v", d)
	}
	if d := g.Hook(edit(g, "db/001_init.sql")); d.Allow {
		t.Fatal("an avoided glob was let through")
	}
	// Budget 2, so up to 4 files outside tests; tests are free.
	for _, f := range []string{"internal/auth/token.go", "internal/auth/a.go", "internal/auth/b.go", "internal/auth/c.go", "internal/auth/a_test.go", "internal/auth/b_test.go"} {
		if d := g.Hook(edit(g, f)); !d.Allow {
			t.Fatalf("%s: %+v", f, d)
		}
	}
	if d := g.Hook(edit(g, "internal/auth/d.go")); d.Allow || !strings.Contains(d.Reason, "planned about 2") {
		t.Fatalf("past the budget: %+v", d)
	}
	if d := g.Hook(edit(g, "internal/auth/a.go")); !d.Allow {
		t.Error("a file the step already changed should stay editable")
	}
}

// TestGate_TheCheckpointPlacesTheNewPart: the checkpoint keeps what the
// agent says it made, where it fits and what to know, runs the tests of what
// changed, and flags what the step said it would leave alone and changed.
func TestGate_TheCheckpointPlacesTheNewPart(t *testing.T) {
	g, c := setup(t, DefaultPolicy())
	ctx := t.Context()
	ran := make(chan []string, 1)
	g.RunTests = func(_ context.Context, _ string, dirs []string) TestRun {
		ran <- dirs
		return TestRun{State: "failed", Packages: dirs, Failed: []string{"TestLogin_LinksIdentity"}}
	}
	g.Propose("Discord", []PlanStep{{Title: "Add DiscordProvider", Files: []string{"internal/auth/provider.go"}, Avoid: []string{"SteamProvider"}}})
	_ = g.AnswerPlan(ctx, true, "")
	g.Hook(edit(g, "internal/auth/provider.go"))
	write(t, g, "internal/auth/provider.go", "\ntype DiscordProvider struct{ Repo Repository }\n\nfunc (DiscordProvider) Name() string { return \"discord\" }\n\nfunc (p DiscordProvider) Exchange(ctx context.Context, code string) (UserIdentity, error) {\n\t_, _ = p.Repo.FindIdentity(ctx, \"discord\", code)\n\treturn UserIdentity{}, nil\n}\n\nfunc (SteamProvider) Extra() {}\n")
	c.reread(t)

	g.OpenCheckpoint(ctx, "A `DiscordProvider` that signs users in with Discord.", "Providers are kept in `Service`; a new one is made in main.", "Beside `SteamProvider`, called by `Service.Login`.", "", "")

	cp := g.State().Checkpoint
	if cp.Made == "" || cp.Know == "" || cp.Fits == "" {
		t.Fatalf("checkpoint = %+v", cp)
	}
	if strings.Join(cp.Avoided, ",") != "SteamProvider" {
		t.Errorf("avoided = %v", cp.Avoided)
	}
	select {
	case dirs := <-ran:
		if strings.Join(dirs, ",") != "internal/auth" {
			t.Errorf("tests ran for %v", dirs)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the tests did not run")
	}
	deadline := time.Now().Add(2 * time.Second)
	for g.State().Checkpoint.Tests.State != "failed" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if tr := g.State().Checkpoint.Tests; tr.State != "failed" || tr.Failed[0] != "TestLogin_LinksIdentity" {
		t.Errorf("tests = %+v", tr)
	}
}

func TestAvoided(t *testing.T) {
	cases := map[string]string{
		"internal/store/pg.go|internal/store":     "internal/store",
		"internal/store/pg.go|./internal/store/":  "./internal/store/",
		"migrations/001.sql|*.sql":                "*.sql",
		"internal/auth/provider.go|provider.go":   "provider.go",
		"internal/auth/provider.go|SteamProvider": "",
		"internal/storex/a.go|internal/store":     "",
	}
	for in, want := range cases {
		rel, a, _ := strings.Cut(in, "|")
		if got := avoided(rel, []string{a}); got != want {
			t.Errorf("avoided(%q, %q) = %q, want %q", rel, a, got, want)
		}
	}
}

func TestRunGoTests(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test")
	}
	g, _ := setup(t, DefaultPolicy())
	if run := RunGoTests(t.Context(), g.Root, []string{"internal/auth"}); run.State != "passed" {
		t.Fatalf("run = %+v", run)
	}
	write(t, g, "internal/auth/broken_test.go", "package auth\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) { t.Fatal(\"no\") }\n")
	run := RunGoTests(t.Context(), g.Root, []string{"internal/auth"})
	if run.State != "failed" || strings.Join(run.Failed, ",") != "TestBroken" || run.Output == "" {
		t.Errorf("run = %+v", run)
	}
	if run := RunGoTests(t.Context(), g.Root, nil); run.State != "none" {
		t.Errorf("no folders: %+v", run)
	}
}

// TestGate_TheDeveloperAsks: at the plan or a checkpoint the developer asks
// a question; mentor_wait gives it to the agent, which answers without
// changing anything, and what waits keeps waiting.
func TestGate_TheDeveloperAsks(t *testing.T) {
	g, _ := setup(t, DefaultPolicy())
	ctx := t.Context()
	if err := g.Ask("why?"); err == nil {
		t.Error("a question with nothing waiting")
	}
	g.Propose("Tokens", []PlanStep{{Title: "Tokens"}})
	got := make(chan string, 1)
	go func() { got <- g.Wait(ctx, 5*time.Second) }()
	time.Sleep(20 * time.Millisecond)
	if err := g.Ask("Why a new TokenService rather than Service?"); err != nil {
		t.Fatal(err)
	}
	if reply := <-got; !strings.Contains(reply, "Why a new TokenService") || !strings.Contains(reply, "mentor_answer") {
		t.Fatalf("wait = %q", reply)
	}
	if err := g.Ask("And?"); err == nil {
		t.Error("a second question before the first is answered")
	}
	if d := g.Hook(edit(g, "internal/auth/provider.go")); d.Allow {
		t.Error("an edit while the plan waits, after a question")
	}
	if r := g.Answer("Service signs in; tokens are their own concern."); !strings.Contains(r, "mentor_wait") {
		t.Fatalf("answer = %q", r)
	}
	st := g.State()
	if st.Waiting != "plan" || len(st.Talk) != 1 || st.Talk[0].A == "" {
		t.Fatalf("state = %+v", st.Talk)
	}
	if reply := g.Wait(ctx, 30*time.Millisecond); !strings.Contains(reply, "not answered") {
		t.Errorf("after the answer the agent waits again: %q", reply)
	}
	_ = g.AnswerPlan(ctx, true, "")
	if g.State().Talk == nil || len(g.State().Talk) != 1 {
		t.Error("the talk of the plan should stay until the next checkpoint")
	}
}

// TestGate_AMilestoneShowsItsOwnChanges: the shape at a milestone is what
// that step did, since it began, not all the agent did since HEAD.
func TestGate_AMilestoneShowsItsOwnChanges(t *testing.T) {
	g, c := setup(t, DefaultPolicy())
	ctx := t.Context()
	g.Propose("Discord and a cache", []PlanStep{{Title: "Discord", Files: []string{"internal/auth/provider.go"}}, {Title: "Cache", Files: []string{"internal/store/cache.go"}}})
	_ = g.AnswerPlan(ctx, true, "")

	g.Hook(edit(g, "internal/auth/provider.go"))
	write(t, g, "internal/auth/provider.go", "\ntype DiscordProvider struct{}\n")
	c.reread(t)
	g.OpenCheckpoint(ctx, "DiscordProvider.", "", "", "", "")
	if _, err := g.AnswerCheckpoint(ctx, "approve", ""); err != nil {
		t.Fatal(err)
	}

	g.Hook(HookCall{Event: "PreToolUse", Tool: "Write", Input: map[string]any{"file_path": filepath.Join(g.Root, "internal/store/cache.go")}})
	if err := os.WriteFile(filepath.Join(g.Root, "internal/store/cache.go"), []byte("package store\n\nimport _ \"example.com/authsvc/internal/api\"\n\n// Cache keeps answers.\ntype Cache struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.reread(t)
	g.OpenCheckpoint(ctx, "A Cache.", "", "", "", "")

	a := g.State().Checkpoint.Arch
	if a == nil {
		t.Fatal("no shape at the milestone")
	}
	var pkgs, parts, edges []string
	for _, p := range a.Packages {
		pkgs = append(pkgs, p.Name)
		for _, r := range p.Parts {
			parts = append(parts, r.Name)
		}
	}
	for _, e := range a.Edges {
		edges = append(edges, e.FromName+"→"+e.ToName+":"+e.Warn)
	}
	if strings.Join(pkgs, ",") != "store" || strings.Join(parts, ",") != "Cache" {
		t.Errorf("packages %v, parts %v: only step 2's", pkgs, parts)
	}
	if strings.Join(edges, ",") != "store→api:Repository now depends on API, a layer above it" {
		t.Errorf("edges = %v", edges)
	}
}

// TestGate_NamesOutsideGoAreKnownByTheirFiles: a Vue component's name and a
// TypeScript function the step changed are not flagged as missing, as the
// Go index cannot know them; a name found nowhere still is.
func TestGate_NamesOutsideGoAreKnownByTheirFiles(t *testing.T) {
	g, _ := setup(t, DefaultPolicy())
	ctx := t.Context()
	init := exec.Command("git", "init", "-q")
	init.Dir = g.Root
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	_ = os.MkdirAll(filepath.Join(g.Root, "web"), 0o755)
	_ = os.WriteFile(filepath.Join(g.Root, "web", "ArchView.vue"), []byte("<template><div/></template>\n"), 0o644)

	g.Propose("Shape", []PlanStep{{Title: "Shape", Files: []string{"web/api.ts"}, Uses: []string{"ArchView", "Nowhere"}}})
	if got := strings.Join(g.State().Plan.Steps[0].Unknown, ","); got != "Nowhere" {
		t.Errorf("plan unknown = %q", got)
	}
	_ = g.AnswerPlan(ctx, true, "")
	g.Hook(HookCall{Event: "PreToolUse", Tool: "Write", Input: map[string]any{"file_path": filepath.Join(g.Root, "web", "api.ts")}})
	_ = os.WriteFile(filepath.Join(g.Root, "web", "api.ts"), []byte("export async function reveal(g: Go) {}\n"), 0o644)
	g.OpenCheckpoint(ctx, "`ArchView` now opens through `reveal`, like `Bogus`.", "", "", "", "")
	if got := strings.Join(g.State().Checkpoint.Unknown, ","); got != "Bogus" {
		t.Errorf("checkpoint unknown = %q", got)
	}
}
