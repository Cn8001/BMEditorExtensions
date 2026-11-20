// Package agent pauses a coding agent, as the editor runs it, at the logical
// boundaries of its work, so the developer understands each step before
// the next begins. The agent works on its own, in auto mode or any other:
// it proposes a plan in steps, which the developer approves in Mentor;
// it does one step; at its end it opens a checkpoint and waits, while
// Mentor shows the developer what the step changed, where it sits, how it
// flows, why the agent says it did it and what it can break. Approving
// lets the agent go on by itself.
//
// The agent's hooks make the pause hold: while the developer has not
// answered, the agent changes nothing, and it cannot end its turn with a
// step that has no checkpoint. Without the hooks the agent may still
// follow the protocol through Mentor's MCP tools, but nothing makes it.
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"bmeditor-mentor/internal/changes"
	"bmeditor-mentor/internal/gitx"
	"bmeditor-mentor/internal/index"
)

// Policy is what the developer set.
type Policy struct {
	RequirePlan bool `json:"requirePlan"` // changing code waits for an approved plan
	MaxFiles    int  `json:"maxFiles"`    // files a step may change before it must end at a checkpoint; 0 for no limit
	Off         bool `json:"off"`         // the developer turned Mentor's watch over the agent off: it lets everything through
}

// DefaultPolicy is the policy when the developer has set nothing.
func DefaultPolicy() Policy { return Policy{RequirePlan: true, MaxFiles: 12} }

// Step is a part of the plan: one behavior the developer can understand on
// its own, and what the agent says of it before it starts.
type Step struct {
	N        int      `json:"n"`
	Title    string   `json:"title"`
	Why      string   `json:"why,omitempty"`
	Files    []string `json:"files"`
	Packages []string `json:"packages"`
	Status   string   `json:"status"` // todo, doing, done
	// What the agent will do, the code it will use, what it will make, what
	// it will leave alone, and when it will stop.
	Does     string   `json:"does,omitempty"`
	Uses     []string `json:"uses"`
	Creates  []string `json:"creates"`
	Avoid    []string `json:"avoid"`
	DoneWhen string   `json:"doneWhen,omitempty"`
	// Unknown are names in Uses the code does not have; Exists, names in
	// Creates it already has.
	Unknown []string `json:"unknown"`
	Exists  []string `json:"exists"`
	// Budget is how many files outside tests the step is expected to change,
	// from what it said it touches; past it by more than a little, it ends.
	Budget int `json:"budget"`
}

// Plan is the agent's proposal and the developer's answer.
type Plan struct {
	Task    string    `json:"task"`
	Steps   []Step    `json:"steps"`
	Status  string    `json:"status"` // proposed, approved, changes_requested, done
	Note    string    `json:"note,omitempty"`
	Created time.Time `json:"created"`
}

// Checkpoint is the end of a step, as the developer needs it: what was
// made, where it fits in the project, what to know of it, and whether the
// tests pass; the whole of what changed is there too, for who wants it.
type Checkpoint struct {
	ID      int    `json:"id"`
	Step    int    `json:"step"`
	Made    string `json:"made,omitempty"` // what was made, in a sentence
	Know    string `json:"know,omitempty"` // what the developer needs to know of it
	Fits    string `json:"fits,omitempty"` // where it sits in the architecture, as the agent tells it
	Summary string `json:"summary"`
	Why     string `json:"why,omitempty"`
	// Unknown are names the agent used that the code does not have.
	Unknown []string        `json:"unknown"`
	Tests   *TestRun        `json:"tests"`
	Avoided []string        `json:"avoided"` // what the step said it would leave alone and changed
	Report  *changes.Report `json:"report"`
	Status  string          `json:"status"` // waiting, approved, changes_requested, skipped
	Note    string          `json:"note,omitempty"`
	Created time.Time       `json:"created"`
}

// Event is one thing the agent did, or was held from doing.
type Event struct {
	At     time.Time `json:"at"`
	Tool   string    `json:"tool"`
	What   string    `json:"what"`
	Result string    `json:"result"` // done, held
	Reason string    `json:"reason,omitempty"`
}

// State is what the panel shows.
type State struct {
	// Level is how far the pause holds: enforced with the hooks,
	// cooperative with the MCP tools only, observe with neither.
	Level string `json:"level"`
	Hooks bool   `json:"hooks"`
	MCP   bool   `json:"mcp"`
	// Problem is why connecting the agent did not work, with what to run
	// by hand.
	Problem    string      `json:"problem,omitempty"`
	LastSeen   time.Time   `json:"lastSeen,omitzero"`
	Plan       *Plan       `json:"plan"`
	Step       int         `json:"step"` // 1-based; 0 when no step is under way
	Checkpoint *Checkpoint `json:"checkpoint"`
	Waiting    string      `json:"waiting,omitempty"` // plan or checkpoint: the agent waits for the developer
	Files      []string    `json:"files"`             // what the step changed so far
	Events     []Event     `json:"events"`
	Policy     Policy      `json:"policy"`
	// Talk is the developer's questions about what waits, the plan or the
	// checkpoint, and the agent's answers.
	Talk []Talk `json:"talk"`
}

// Code is what the gate needs of the index.
type Code interface {
	Peek() *index.Index
}

// Gate is one folder's checkpoints.
type Gate struct {
	Root string
	code Code
	tell func(State)

	mu       sync.Mutex
	policy   Policy
	hooks    bool
	mcp      bool
	problem  string
	lastSeen time.Time
	plan     *Plan
	step     int
	base     *changes.Baseline // the code when the step began
	cp       *Checkpoint
	nextID   int
	files    map[string]bool
	touched  map[string]bool              // every file the agent changed, from the folder, until it no longer differs from HEAD
	before   map[string]map[string]string // by tool use: the files that differed from HEAD before a command ran
	events   []Event
	reply    string        // the developer's last answer, as the agent reads it
	talk     []Talk        // questions and answers about what waits now
	changed  chan struct{} // closed and replaced on every change, for Wait

	// Persist keeps the gate's state, so a plan under way outlives a
	// restart of the backend.
	Persist func([]byte)

	// RunTests runs the tests of the folders a step changed, from Root; nil
	// runs go test.
	RunTests func(ctx context.Context, root string, dirs []string) TestRun

	// Dirty answers the files of the folder that differ from HEAD, each
	// with a hash of what it holds now ("" when it is gone): a command's
	// changes are what differs after it and did not before. Nil reads git.
	Dirty func(ctx context.Context) map[string]string
}

// New makes a folder's gate.
func New(root string, code Code, policy Policy, tell func(State)) *Gate {
	if tell == nil {
		tell = func(State) {}
	}
	return &Gate{Root: root, code: code, tell: tell, policy: policy, files: map[string]bool{}, touched: map[string]bool{}, changed: make(chan struct{})}
}

// SetPolicy takes the developer's new settings.
func (g *Gate) SetPolicy(p Policy) {
	g.mu.Lock()
	g.policy = p
	g.mu.Unlock()
}

// SetConnection says what is set up in the agent.
func (g *Gate) SetConnection(hooks, mcp bool, problem string) {
	g.mu.Lock()
	g.hooks, g.mcp, g.problem = hooks, mcp, problem
	g.mu.Unlock()
	g.notify()
}

// notify tells the panel, wakes whoever waits and keeps the state. Call it
// without the lock.
func (g *Gate) notify() {
	g.mu.Lock()
	close(g.changed)
	g.changed = make(chan struct{})
	st := g.state()
	persist := g.Persist
	g.mu.Unlock()
	g.tell(st)
	if persist != nil {
		if b, err := g.Marshal(); err == nil {
			persist(b)
		}
	}
}

// State is the gate now.
func (g *Gate) State() State {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state()
}

func (g *Gate) state() State {
	st := State{Hooks: g.hooks, MCP: g.mcp, Problem: g.problem, LastSeen: g.lastSeen, Plan: g.plan, Step: g.step, Checkpoint: g.cp,
		Files: []string{}, Events: append([]Event{}, g.events...), Policy: g.policy, Talk: append([]Talk{}, g.talk...)}
	for f := range g.files {
		st.Files = append(st.Files, f)
	}
	switch {
	case g.policy.Off:
		st.Level = "off"
	case g.hooks:
		st.Level = "enforced"
	case g.mcp:
		st.Level = "cooperative"
	default:
		st.Level = "observe"
	}
	switch {
	case g.plan != nil && g.plan.Status == "proposed":
		st.Waiting = "plan"
	case g.cp != nil && g.cp.Status == "waiting":
		st.Waiting = "checkpoint"
	}
	return st
}

func (g *Gate) log(tool, what, result, reason string) {
	g.events = append(g.events, Event{At: time.Now(), Tool: tool, What: what, Result: result, Reason: reason})
	if len(g.events) > 200 {
		g.events = g.events[len(g.events)-200:]
	}
}

func (g *Gate) currentStep() *Step {
	if g.plan == nil || g.plan.Status != "approved" || g.step < 1 || g.step > len(g.plan.Steps) {
		return nil
	}
	return &g.plan.Steps[g.step-1]
}

// The hooks.

// HookCall is what the agent's hook sends.
type HookCall struct {
	Event      string         `json:"hook_event_name"`
	Tool       string         `json:"tool_name"`
	Input      map[string]any `json:"tool_input"`
	StopActive bool           `json:"stop_hook_active"`
	ToolUseID  string         `json:"tool_use_id"`
}

// Decision is the gate's answer: let the agent go on, or hold it with a
// reason it reads.
type Decision struct {
	Allow  bool
	Reason string
}

var editTools = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}

// Hook answers one of the agent's hooks.
func (g *Gate) Hook(call HookCall) Decision {
	// A shell command's changes cannot be read from its input: the files
	// that differ from HEAD are noted before it runs and after.
	var dirty map[string]string
	if call.Tool == "Bash" && (call.Event == "PostToolUse" || call.Event == "PreToolUse" && !readOnly(commandOf(call))) && !g.off() {
		dirty = g.dirty()
	}
	g.mu.Lock()
	g.lastSeen = time.Now()
	g.hooks = true // a hook that calls shows they are there
	var d Decision
	switch {
	case g.policy.Off:
		d = Decision{Allow: true}
	case call.Event == "PreToolUse":
		d = g.preToolUse(call, dirty)
	case call.Event == "Stop":
		d = g.stop(call)
	case call.Event == "PostToolUse":
		g.ran(call, dirty)
		d = Decision{Allow: true}
	default:
		d = Decision{Allow: true}
	}
	g.mu.Unlock()
	g.notify()
	return d
}

// held is why the agent may not change anything now, if it may not: its
// plan or a checkpoint waits for the developer, or it has no plan.
func (g *Gate) held() string {
	switch {
	case g.plan != nil && g.plan.Status == "proposed":
		return "Your plan waits for the developer in Mentor. Call mentor_wait, again until they answer."
	case g.cp != nil && g.cp.Status == "waiting":
		return fmt.Sprintf("Checkpoint %d waits for the developer in Mentor. Call mentor_wait, again until they answer.", g.cp.ID)
	case g.plan != nil && g.plan.Status == "changes_requested":
		return "The developer asked for changes to your plan: " + g.plan.Note + " Propose it again with mentor_plan."
	case g.policy.RequirePlan && (g.plan == nil || g.plan.Status == "done"):
		return "Before changing code, propose your plan: call mentor_plan with the task and its steps, each a logical part the developer can review on its own, then mentor_wait."
	}
	return ""
}

func (g *Gate) preToolUse(call HookCall, dirty map[string]string) Decision {
	var what string
	switch {
	case editTools[call.Tool]:
		path := pathOf(call.Input)
		if path == "" {
			return Decision{Allow: true}
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(g.Root, path)
		}
		if !inside(g.Root, path) {
			return Decision{Allow: true}
		}
		what = g.rel(path)
	case call.Tool == "Bash":
		cmd := commandOf(call)
		if readOnly(cmd) {
			return Decision{Allow: true}
		}
		what = firstLine(cmd, 120)
		if dirty != nil {
			defer func() {
				if len(g.before) > 32 {
					g.before = nil // answers that never came
				}
				if g.before == nil {
					g.before = map[string]map[string]string{}
				}
				g.before[call.ToolUseID] = dirty
			}()
		}
	default:
		return Decision{Allow: true}
	}
	if reason := g.held(); reason != "" {
		g.log(call.Tool, what, "held", reason)
		return Decision{Reason: "Mentor: " + reason}
	}
	if editTools[call.Tool] {
		if reason := g.outOfStep(what); reason != "" {
			g.log(call.Tool, what, "held", reason)
			return Decision{Reason: "Mentor: " + reason}
		}
		if g.step > 0 {
			g.files[what] = true
		}
		g.touched[what] = true
	}
	g.log(call.Tool, what, "done", "")
	return Decision{Allow: true}
}

// outOfStep is why an edit of rel does not belong to the step under way:
// a file the step said it would leave alone, or one more than it can take.
// A smaller step is easier to understand.
func (g *Gate) outOfStep(rel string) string {
	st := g.currentStep()
	if st == nil || g.files[rel] {
		return ""
	}
	if a := avoided(rel, st.Avoid); a != "" {
		return fmt.Sprintf("Step %d said it would leave %s alone, and %s is there. If the step needs it after all, end the step here: call mentor_checkpoint and say why, then mentor_wait.", g.step, a, rel)
	}
	n := 0
	for f := range g.files {
		if !isTestFile(f) {
			n++
		}
	}
	if !isTestFile(rel) {
		n++
	}
	if overBudget(n, st.Budget) {
		return fmt.Sprintf("Step %d would change %d files outside tests; it planned about %d. It has grown past one behavior: end it here with mentor_checkpoint, then mentor_wait, and go on in a step of its own.", g.step, n, st.Budget)
	}
	if g.policy.MaxFiles > 0 && len(g.files) >= g.policy.MaxFiles {
		return fmt.Sprintf("Step %d has changed %d files, as many as one step may. End it here: call mentor_checkpoint, then mentor_wait, and go on in the next step.", g.step, len(g.files))
	}
	return ""
}

// ran notes what a shell command changed: the files that differ from HEAD
// now and did not, or held something else, before it ran.
func (g *Gate) ran(call HookCall, dirty map[string]string) {
	before, ok := g.before[call.ToolUseID]
	delete(g.before, call.ToolUseID)
	if !ok || before == nil || dirty == nil {
		return
	}
	for f, h := range dirty {
		if b, was := before[f]; was && b == h {
			continue
		}
		g.touched[f] = true
		if g.step > 0 {
			g.files[f] = true
		}
	}
}

func (g *Gate) off() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.policy.Off
}

func commandOf(call HookCall) string {
	cmd, _ := call.Input["command"].(string)
	return cmd
}

// dirty is Dirty, or else what git says differs from HEAD.
func (g *Gate) dirty() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if g.Dirty != nil {
		return g.Dirty(ctx)
	}
	files, err := gitx.Changes(ctx, g.Root)
	if err != nil {
		return nil
	}
	out := map[string]string{}
	for _, f := range files {
		h := ""
		if b, err := os.ReadFile(f.Path); err == nil {
			sum := sha256.Sum256(b)
			h = hex.EncodeToString(sum[:12])
		}
		out[g.rel(f.Path)] = h
	}
	return out
}

// stop keeps the agent from ending its turn in the middle of a step that
// changed code: it opens the checkpoint first. A turn already continued
// once by this ends, so the agent cannot loop.
func (g *Gate) stop(call HookCall) Decision {
	if call.StopActive || g.currentStep() == nil || len(g.files) == 0 || g.cp != nil && g.cp.Status == "waiting" {
		return Decision{Allow: true}
	}
	reason := fmt.Sprintf("Step %d changed %d file(s) and has no checkpoint. Call mentor_checkpoint with what you did and why, then mentor_wait.", g.step, len(g.files))
	g.log("Stop", "end of turn", "held", reason)
	return Decision{Reason: "Mentor: " + reason}
}

// readOnlyBash are commands that look and do not change.
var readOnlyBash = regexp.MustCompile(`^\s*(ls|cat|head|tail|wc|grep|rg|find|fd|tree|pwd|echo|which|file|stat|du|df|diff|sort|uniq|jq|go\s+(build|vet|test|list|doc|env|version)|gofmt\s+-l|git\s+(status|diff|log|show|blame|branch|rev-parse|ls-files)|make\s+(test|check|lint))\b`)

// quiet are redirections that write nothing: errors to nowhere, one stream
// into the other.
var quiet = regexp.MustCompile(`\d?>\s*/dev/null|\d?>&\d`)

// writesFiles are find's actions that change or run things.
var writesFiles = regexp.MustCompile(`(^|\s)-(delete|exec|execdir|ok|okdir|fprint\w*)\b`)

// readOnly says whether a shell command only looks: each command of its
// chain (split on ; & | and new lines outside quotes) is one that looks,
// or cd, and nothing is written to a file or run from a substitution.
func readOnly(cmd string) bool {
	parts, ok := splitShell(quiet.ReplaceAllString(cmd, " "))
	if !ok {
		return false
	}
	for _, part := range parts {
		if part = strings.TrimSpace(part); part == "" || part == "cd" || strings.HasPrefix(part, "cd ") {
			continue
		}
		if !readOnlyBash.MatchString(part) || (strings.HasPrefix(part, "find") && writesFiles.MatchString(part)) {
			return false
		}
	}
	return true
}

// splitShell splits a command line into its commands at ; & | and new
// lines outside quotes. It answers false for what it cannot tell is
// harmless: a redirection into a file, or a command substitution.
func splitShell(cmd string) ([]string, bool) {
	var parts []string
	var cur strings.Builder
	var quote rune // ' or ", while inside one
	escaped := false
	for i, r := range cmd {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
		case quote == '\'':
			if r == '\'' {
				quote = 0
			}
		case r == '`' || r == '$' && i+1 < len(cmd) && cmd[i+1] == '(':
			return nil, false
		case quote == '"':
			if r == '"' {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '>':
			return nil, false
		case r == ';' || r == '&' || r == '|' || r == '\n':
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	return append(parts, cur.String()), true
}

// The MCP tools: what the agent says.

// PlanStep is a step as the agent proposes it.
type PlanStep struct {
	Title    string   `json:"title"`
	Why      string   `json:"why"`
	Files    []string `json:"files"`
	Packages []string `json:"packages"`
	Does     string   `json:"does"`
	Uses     []string `json:"uses"`
	Creates  []string `json:"creates"`
	Avoid    []string `json:"avoid"`
	DoneWhen string   `json:"doneWhen"`
}

// Propose takes the agent's plan, which waits for the developer. One
// proposed while another runs replaces it, once approved.
func (g *Gate) Propose(task string, steps []PlanStep) string {
	if strings.TrimSpace(task) == "" || len(steps) == 0 {
		return "A plan needs the task and at least one step."
	}
	ix := g.code.Peek()
	g.mu.Lock()
	p := &Plan{Task: task, Status: "proposed", Created: time.Now()}
	for i, s := range steps {
		st := Step{N: i + 1, Title: s.Title, Why: s.Why, Files: clean(s.Files), Packages: clean(s.Packages), Status: "todo",
			Does: strings.TrimSpace(s.Does), Uses: trimmed(s.Uses), Creates: trimmed(s.Creates), Avoid: trimmed(s.Avoid), DoneWhen: strings.TrimSpace(s.DoneWhen)}
		st.Unknown, st.Exists = namesCheck(ix, st.Uses, st.Creates)
		st.Budget = budgetOf(st)
		p.Steps = append(p.Steps, st)
	}
	g.plan, g.step, g.cp, g.reply, g.talk = p, 0, nil, "", nil
	g.files = map[string]bool{}
	g.log("mentor_plan", task, "done", "")
	g.mu.Unlock()
	g.notify()
	return "The plan is with the developer. Call mentor_wait for their answer, and change nothing until then."
}

func trimmed(list []string) []string {
	out := []string{}
	for _, s := range list {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func clean(list []string) []string {
	out := []string{}
	for _, s := range list {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, strings.TrimPrefix(filepath.ToSlash(s), "./"))
		}
	}
	return out
}

// OpenCheckpoint ends the step: what the agent says it did and why, beside
// what the code shows changed since the step began.
func (g *Gate) OpenCheckpoint(ctx context.Context, made, know, fits, summary, why string) string {
	g.mu.Lock()
	if g.currentStep() == nil {
		g.mu.Unlock()
		return "No step is under way: propose a plan with mentor_plan first."
	}
	if g.cp != nil && g.cp.Status == "waiting" {
		id := g.cp.ID
		g.mu.Unlock()
		return fmt.Sprintf("Checkpoint %d already waits for the developer. Call mentor_wait.", id)
	}
	g.nextID++
	if strings.TrimSpace(made) == "" && strings.TrimSpace(summary) == "" {
		g.nextID--
		g.mu.Unlock()
		return "Say what the step made, in a sentence (made), and what the developer needs to know of it (know)."
	}
	g.cp = &Checkpoint{ID: g.nextID, Step: g.step, Made: strings.TrimSpace(made), Know: strings.TrimSpace(know), Fits: strings.TrimSpace(fits), Summary: summary, Why: why,
		Status: "waiting", Unknown: []string{}, Avoided: []string{}, Created: time.Now()}
	g.reply = ""
	g.talk = nil
	g.log("mentor_checkpoint", fmt.Sprintf("step %d", g.step), "done", "")
	id := g.cp.ID
	g.mu.Unlock()
	g.Refresh(ctx)
	return fmt.Sprintf("Checkpoint %d is with the developer. Call mentor_wait, and change nothing until they answer.", id)
}

// Refresh works out again what the waiting checkpoint's step changed, from
// the code now: the index follows edits a moment after they land.
func (g *Gate) Refresh(ctx context.Context) {
	ix := g.code.Peek()
	g.mu.Lock()
	cp, base := g.cp, g.base
	g.mu.Unlock()
	if cp == nil || cp.Status != "waiting" || base == nil || ix == nil {
		return
	}
	rep := changes.Build(ix, *base, changes.Diff(ix, *base), nil)
	rep.Since = fmt.Sprintf("step %d began", cp.Step)
	unknown := unknownNames(ix, cp.Made+"\n"+cp.Know+"\n"+cp.Fits+"\n"+cp.Summary+"\n"+cp.Why)
	g.mu.Lock()
	var avoid []string
	if st := g.currentStep(); st != nil {
		avoid = st.Avoid
	}
	start := false
	if g.cp == cp {
		cp.Report, cp.Unknown, cp.Avoided = rep, unknown, avoidedNames(avoid, rep)
		// The tests of what the step changed run once, Mentor's own way:
		// the developer does not take the agent's word for it.
		if cp.Tests == nil {
			cp.Tests = &TestRun{State: "running", Packages: goDirs(g.Root, g.files), Failed: []string{}}
			start = true
		}
	}
	dirs := []string(nil)
	if cp.Tests != nil {
		dirs = cp.Tests.Packages
	}
	run := g.RunTests
	g.mu.Unlock()
	g.notify()
	if start {
		if run == nil {
			run = RunGoTests
		}
		go func() {
			res := run(context.Background(), g.Root, dirs)
			g.mu.Lock()
			if g.cp == cp {
				cp.Tests = &res
			}
			g.mu.Unlock()
			g.notify()
		}()
	}
}

// Wait blocks until nothing waits for the developer, or until timeout, and
// says what to do next.
func (g *Gate) Wait(ctx context.Context, timeout time.Duration) string {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		g.mu.Lock()
		waiting := g.state().Waiting
		ch, reply := g.changed, g.reply
		g.lastSeen = time.Now()
		q := g.question()
		g.mu.Unlock()
		if q != "" {
			return "The developer asks about your " + waiting + ": " + q + "\nAnswer it with mentor_answer, in plain words, from what the code is; change nothing. Then call mentor_wait."
		}
		if waiting == "" {
			if reply == "" {
				reply = g.Status()
			}
			return reply
		}
		select {
		case <-ch:
		case <-deadline.C:
			return "The developer has not answered yet. Call mentor_wait again, and change nothing until then."
		case <-ctx.Done():
			return "The developer has not answered yet. Call mentor_wait again."
		}
	}
}

// Talk is one question of the developer's, and the agent's answer.
type Talk struct {
	Q  string    `json:"q"`
	A  string    `json:"a,omitempty"`
	At time.Time `json:"at"`
	// Sent says the agent has been given the question.
	Sent bool `json:"sent,omitempty"`
}

// question is the developer's question the agent has not been given yet,
// which it now is. Call it with the lock held.
func (g *Gate) question() string {
	if n := len(g.talk); n > 0 && !g.talk[n-1].Sent && g.talk[n-1].A == "" {
		g.talk[n-1].Sent = true
		return g.talk[n-1].Q
	}
	return ""
}

// Ask puts the developer's question about what waits, the plan or the
// checkpoint, to the agent, which answers it without changing anything;
// what waits keeps waiting.
func (g *Gate) Ask(q string) error {
	q = strings.TrimSpace(q)
	g.mu.Lock()
	if q == "" || g.state().Waiting == "" {
		g.mu.Unlock()
		return fmt.Errorf("nothing waits for an answer to ask about")
	}
	if n := len(g.talk); n > 0 && g.talk[n-1].A == "" {
		g.mu.Unlock()
		return fmt.Errorf("the agent has not answered the last question yet")
	}
	g.talk = append(g.talk, Talk{Q: q, At: time.Now()})
	g.log("Mentor", "a question", "asked", q)
	g.mu.Unlock()
	g.notify()
	return nil
}

// Answer is the agent's answer to the developer's question.
func (g *Gate) Answer(a string) string {
	a = strings.TrimSpace(a)
	g.mu.Lock()
	n := len(g.talk)
	if n == 0 || g.talk[n-1].A != "" {
		g.mu.Unlock()
		return "No question waits for an answer. Call mentor_wait."
	}
	if a == "" {
		g.mu.Unlock()
		return "The answer is empty: answer the developer's question, then call mentor_wait."
	}
	g.talk[n-1].A = a
	g.talk[n-1].Sent = true
	g.log("mentor_answer", firstLine(g.talk[n-1].Q, 80), "done", "")
	g.mu.Unlock()
	g.notify()
	return "The developer has your answer. Call mentor_wait, and change nothing until they answer."
}

// Status says where the plan is, for the agent.
func (g *Gate) Status() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if r := g.held(); r != "" {
		return r
	}
	st := g.currentStep()
	if st == nil {
		return "No plan is under way."
	}
	s := fmt.Sprintf("Plan: %s\nStep %d of %d: %s\n", g.plan.Task, st.N, len(g.plan.Steps), st.Title)
	return s + said(st) + "When it is done, call mentor_checkpoint with what you made and what the developer needs to know, then mentor_wait."
}

// next says what the agent does now.
func (g *Gate) next() string {
	st := g.currentStep()
	if st == nil {
		return "All steps are done."
	}
	s := fmt.Sprintf("Do step %d: %s.\n", st.N, st.Title)
	return s + said(st) + "When it is done, call mentor_checkpoint with what you made and what the developer needs to know, then mentor_wait."
}

// said is what the agent said of a step, to hold it to.
func said(st *Step) string {
	var b strings.Builder
	if st.Does != "" {
		b.WriteString("It does: " + st.Does + "\n")
	}
	if len(st.Files)+len(st.Packages) > 0 {
		b.WriteString("It touches " + strings.Join(append(append([]string{}, st.Files...), st.Packages...), ", ") + fmt.Sprintf(" (about %d files besides tests).\n", st.Budget))
	}
	if len(st.Avoid) > 0 {
		b.WriteString("It leaves alone " + strings.Join(st.Avoid, ", ") + ".\n")
	}
	if st.DoneWhen != "" {
		b.WriteString("It is done when " + st.DoneWhen + "\n")
	}
	return b.String()
}

// What the developer answers, from the panel.

// AnswerPlan approves the plan, which starts its first step, or sends it
// back with a note.
func (g *Gate) AnswerPlan(ctx context.Context, approve bool, note string) error {
	g.mu.Lock()
	if g.plan == nil || g.plan.Status != "proposed" {
		g.mu.Unlock()
		return fmt.Errorf("no plan waits for an answer")
	}
	g.plan.Note = note
	if !approve {
		g.plan.Status = "changes_requested"
		g.reply = "The developer asked for changes to the plan: " + note + "\nPropose it again with mentor_plan."
		g.log("Mentor", "the plan", "changes requested", note)
		g.mu.Unlock()
		g.notify()
		return nil
	}
	g.plan.Status = "approved"
	g.log("Mentor", "the plan", "approved", note)
	g.mu.Unlock()
	g.startStep(ctx, 1)
	g.mu.Lock()
	g.reply = "The developer approved the plan" + noteOf(note) + ". " + g.next()
	g.mu.Unlock()
	g.notify()
	return nil
}

func noteOf(note string) string {
	if strings.TrimSpace(note) == "" {
		return ""
	}
	return ", saying: " + strings.TrimSpace(note)
}

// startStep begins step n: what it changes is compared with the code now.
func (g *Gate) startStep(ctx context.Context, n int) {
	var base *changes.Baseline
	if ix := g.code.Peek(); ix != nil {
		b := changes.Snapshot(ctx, ix, fmt.Sprintf("step %d began", n))
		base = &b
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.step, g.base = n, base
	g.files = map[string]bool{}
	if g.plan != nil && n >= 1 && n <= len(g.plan.Steps) {
		g.plan.Steps[n-1].Status = "doing"
	}
}

// AnswerCheckpoint approves the step, which starts the next; asks for
// changes within it; or skips its review deliberately. It answers the keys
// of what an approved step changed, for marking them reviewed.
func (g *Gate) AnswerCheckpoint(ctx context.Context, answer, note string) ([]string, error) {
	g.mu.Lock()
	cp := g.cp
	if cp == nil || cp.Status != "waiting" {
		g.mu.Unlock()
		return nil, fmt.Errorf("no checkpoint waits for an answer")
	}
	cp.Note = note
	if answer == "changes" {
		cp.Status = "changes_requested"
		g.reply = fmt.Sprintf("The developer asked for changes in step %d: %s\nMake them within the step, then call mentor_checkpoint again and mentor_wait.", cp.Step, note)
		g.log("Mentor", fmt.Sprintf("checkpoint %d", cp.ID), "changes requested", note)
		g.mu.Unlock()
		g.notify()
		return nil, nil
	}
	var keys []string
	if answer == "skip" {
		cp.Status = "skipped"
	} else {
		cp.Status = "approved"
		if cp.Report != nil {
			for _, u := range cp.Report.Units {
				for _, c := range u.Changes {
					if c.Change != "removed" {
						keys = append(keys, c.Key)
					}
				}
			}
		}
	}
	g.log("Mentor", fmt.Sprintf("checkpoint %d", cp.ID), cp.Status, note)
	g.plan.Steps[cp.Step-1].Status = "done"
	n := cp.Step + 1
	done := n > len(g.plan.Steps)
	if done {
		g.plan.Status = "done"
		g.step, g.base = 0, nil
		g.files = map[string]bool{}
	}
	g.mu.Unlock()
	if !done {
		g.startStep(ctx, n)
	}
	g.mu.Lock()
	if done {
		g.reply = "The developer approved the last step" + noteOf(note) + ". The plan is done."
	} else {
		g.reply = fmt.Sprintf("The developer approved step %d%s. %s", cp.Step, noteOf(note), g.next())
	}
	g.mu.Unlock()
	g.notify()
	return keys, nil
}

// EndPlan ends the plan now, as when the developer takes over.
func (g *Gate) EndPlan() {
	g.mu.Lock()
	if g.plan != nil {
		g.plan.Status = "done"
	}
	g.step, g.base, g.cp = 0, nil, nil
	g.files = map[string]bool{}
	g.reply = "The developer ended the plan. Propose a new one with mentor_plan before changing more."
	g.log("Mentor", "the plan", "ended", "")
	g.mu.Unlock()
	g.notify()
}

// Touched answers the files the agent changed, from the folder, that
// still differ from HEAD: differs says which do. The rest, committed or
// put back, are forgotten.
func (g *Gate) Touched(differs func(rel string) bool) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var list []string
	for f := range g.touched {
		if differs(f) {
			list = append(list, f)
		} else {
			delete(g.touched, f)
		}
	}
	sort.Strings(list)
	return list
}

// Helpers.

func (g *Gate) rel(path string) string {
	r, err := filepath.Rel(g.Root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}

func inside(root, path string) bool {
	r, err := filepath.Rel(root, path)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}

func pathOf(in map[string]any) string {
	for _, k := range []string{"file_path", "notebook_path", "path"} {
		if s, ok := in[k].(string); ok && s != "" {
			return filepath.Clean(s)
		}
	}
	return ""
}

func firstLine(s string, n int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " …"
	}
	if len(s) > n {
		s = s[:n-1] + "…"
	}
	return s
}

// Persistence.

type saved struct {
	Plan    *Plan             `json:"plan"`
	Step    int               `json:"step"`
	Base    *changes.Baseline `json:"base"`
	CP      *Checkpoint       `json:"cp"`
	NextID  int               `json:"nextId"`
	Files   map[string]bool   `json:"files"`
	Touched map[string]bool   `json:"touched"`
	Talk    []Talk            `json:"talk"`
	Events  []Event           `json:"events"`
	Reply   string            `json:"reply"`
}

// Marshal is the gate's state to keep.
func (g *Gate) Marshal() ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return json.Marshal(saved{Plan: g.plan, Step: g.step, Base: g.base, CP: g.cp, NextID: g.nextID, Files: g.files, Touched: g.touched, Talk: g.talk, Events: g.events, Reply: g.reply})
}

// Unmarshal takes back state Marshal kept.
func (g *Gate) Unmarshal(b []byte) error {
	var s saved
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.plan, g.step, g.base, g.cp, g.nextID, g.events, g.reply = s.Plan, s.Step, s.Base, s.CP, s.NextID, s.Events, s.Reply
	if s.Files != nil {
		g.files = s.Files
	}
	if s.Touched != nil {
		g.touched = s.Touched
	}
	g.talk = s.Talk
	return nil
}
