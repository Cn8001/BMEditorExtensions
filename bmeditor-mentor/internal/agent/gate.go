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
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"bmeditor-mentor/internal/changes"
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

// Step is a part of the plan.
type Step struct {
	N        int      `json:"n"`
	Title    string   `json:"title"`
	Why      string   `json:"why,omitempty"`
	Files    []string `json:"files"`
	Packages []string `json:"packages"`
	Status   string   `json:"status"` // todo, doing, done
}

// Plan is the agent's proposal and the developer's answer.
type Plan struct {
	Task    string    `json:"task"`
	Steps   []Step    `json:"steps"`
	Status  string    `json:"status"` // proposed, approved, changes_requested, done
	Note    string    `json:"note,omitempty"`
	Created time.Time `json:"created"`
}

// Checkpoint is the end of a step: what the agent says it did and why,
// beside what the code shows it did.
type Checkpoint struct {
	ID      int             `json:"id"`
	Step    int             `json:"step"`
	Summary string          `json:"summary"`
	Why     string          `json:"why,omitempty"`
	Unknown []string        `json:"unknown"` // names the agent used that the code does not have
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
	Level      string      `json:"level"`
	Hooks      bool        `json:"hooks"`
	MCP        bool        `json:"mcp"`
	LastSeen   time.Time   `json:"lastSeen,omitzero"`
	Plan       *Plan       `json:"plan"`
	Step       int         `json:"step"` // 1-based; 0 when no step is under way
	Checkpoint *Checkpoint `json:"checkpoint"`
	Waiting    string      `json:"waiting,omitempty"` // plan or checkpoint: the agent waits for the developer
	Files      []string    `json:"files"`             // what the step changed so far
	Events     []Event     `json:"events"`
	Policy     Policy      `json:"policy"`
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
	lastSeen time.Time
	plan     *Plan
	step     int
	base     *changes.Baseline // the code when the step began
	cp       *Checkpoint
	nextID   int
	files    map[string]bool
	touched  map[string]bool // every file the agent changed, from the folder, until it no longer differs from HEAD
	events   []Event
	reply    string        // the developer's last answer, as the agent reads it
	changed  chan struct{} // closed and replaced on every change, for Wait

	// Persist keeps the gate's state, so a plan under way outlives a
	// restart of the backend.
	Persist func([]byte)
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
func (g *Gate) SetConnection(hooks, mcp bool) {
	g.mu.Lock()
	g.hooks, g.mcp = hooks, mcp
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
	st := State{Hooks: g.hooks, MCP: g.mcp, LastSeen: g.lastSeen, Plan: g.plan, Step: g.step, Checkpoint: g.cp,
		Files: []string{}, Events: append([]Event{}, g.events...), Policy: g.policy}
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
	g.mu.Lock()
	g.lastSeen = time.Now()
	g.hooks = true // a hook that calls shows they are there
	var d Decision
	switch {
	case g.policy.Off:
		d = Decision{Allow: true}
	case call.Event == "PreToolUse":
		d = g.preToolUse(call)
	case call.Event == "Stop":
		d = g.stop(call)
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

func (g *Gate) preToolUse(call HookCall) Decision {
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
		cmd, _ := call.Input["command"].(string)
		if readOnly(cmd) {
			return Decision{Allow: true}
		}
		what = firstLine(cmd, 120)
	default:
		return Decision{Allow: true}
	}
	if reason := g.held(); reason != "" {
		g.log(call.Tool, what, "held", reason)
		return Decision{Reason: "Mentor: " + reason}
	}
	if editTools[call.Tool] {
		// A step that grows past its size ends: a smaller one is easier
		// to understand.
		if n := len(g.files); g.step > 0 && g.policy.MaxFiles > 0 && !g.files[what] && n >= g.policy.MaxFiles {
			reason := fmt.Sprintf("Step %d has changed %d files, as many as one step may. End it here: call mentor_checkpoint with what you did and why, then mentor_wait, and go on in the next step.", g.step, n)
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

func readOnly(cmd string) bool {
	for _, part := range strings.FieldsFunc(cmd, func(r rune) bool { return r == ';' || r == '&' || r == '|' || r == '\n' }) {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		if !readOnlyBash.MatchString(part) || strings.ContainsAny(part, ">") {
			return false
		}
	}
	return true
}

// The MCP tools: what the agent says.

// PlanStep is a step as the agent proposes it.
type PlanStep struct {
	Title    string   `json:"title"`
	Why      string   `json:"why"`
	Files    []string `json:"files"`
	Packages []string `json:"packages"`
}

// Propose takes the agent's plan, which waits for the developer. One
// proposed while another runs replaces it, once approved.
func (g *Gate) Propose(task string, steps []PlanStep) string {
	if strings.TrimSpace(task) == "" || len(steps) == 0 {
		return "A plan needs the task and at least one step."
	}
	g.mu.Lock()
	p := &Plan{Task: task, Status: "proposed", Created: time.Now()}
	for i, s := range steps {
		p.Steps = append(p.Steps, Step{N: i + 1, Title: s.Title, Why: s.Why, Files: clean(s.Files), Packages: clean(s.Packages), Status: "todo"})
	}
	g.plan, g.step, g.cp, g.reply = p, 0, nil, ""
	g.files = map[string]bool{}
	g.log("mentor_plan", task, "done", "")
	g.mu.Unlock()
	g.notify()
	return "The plan is with the developer. Call mentor_wait for their answer, and change nothing until then."
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
func (g *Gate) OpenCheckpoint(ctx context.Context, summary, why string) string {
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
	g.cp = &Checkpoint{ID: g.nextID, Step: g.step, Summary: summary, Why: why, Status: "waiting", Unknown: []string{}, Created: time.Now()}
	g.reply = ""
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
	unknown := unknownNames(ix, cp.Summary+"\n"+cp.Why)
	g.mu.Lock()
	if g.cp == cp {
		cp.Report, cp.Unknown = rep, unknown
	}
	g.mu.Unlock()
	g.notify()
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
		g.mu.Unlock()
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
	if len(st.Files)+len(st.Packages) > 0 {
		s += "It touches " + strings.Join(append(append([]string{}, st.Files...), st.Packages...), ", ") + ".\n"
	}
	return s + "When it is done, call mentor_checkpoint with what you did and why, then mentor_wait."
}

// next says what the agent does now.
func (g *Gate) next() string {
	st := g.currentStep()
	if st == nil {
		return "All steps are done."
	}
	s := fmt.Sprintf("Do step %d: %s.", st.N, st.Title)
	if len(st.Files)+len(st.Packages) > 0 {
		s += " It touches " + strings.Join(append(append([]string{}, st.Files...), st.Packages...), ", ") + "."
	}
	return s + " When it is done, call mentor_checkpoint with what you did and why, then mentor_wait."
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
	Events  []Event           `json:"events"`
	Reply   string            `json:"reply"`
}

// Marshal is the gate's state to keep.
func (g *Gate) Marshal() ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return json.Marshal(saved{Plan: g.plan, Step: g.step, Base: g.base, CP: g.cp, NextID: g.nextID, Files: g.files, Touched: g.touched, Events: g.events, Reply: g.reply})
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
	return nil
}
