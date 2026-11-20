package intel

import (
	"path/filepath"
	"strings"
	"testing"

	"bmeditor-mentor/internal/golang"
	"bmeditor-mentor/internal/index"
)

const auth = "example.com/authsvc/internal/auth."

func load(t *testing.T) *index.Index {
	t.Helper()
	root, _ := filepath.Abs(filepath.Join("..", "golang", "testdata", "authsvc"))
	b := &golang.Builder{Root: root}
	if err := b.Full(t.Context()); err != nil {
		t.Fatal(err)
	}
	return index.Merge(root, b)
}

func names(refs []string) string { return strings.Join(refs, " ") }

func TestTask_AddAnAuthenticationProvider(t *testing.T) {
	// Arrange: what the search answers for the task, a provider type first.
	ix := load(t)
	candidates := []string{auth + "SteamProvider", auth + "Service.Login", auth + "Provider", auth + "UserIdentity"}

	// Act
	a := Task(ix, "add a new authentication provider", candidates)

	// Assert
	p := a.Point
	if p == nil || p.Interface.Key != auth+"Provider" {
		t.Fatalf("point = %+v", p)
	}
	var methods, impls, made, holders, consumers, down, tests, routes []string
	for _, m := range p.Methods {
		methods = append(methods, m.Name)
	}
	for _, im := range p.Impls {
		impls = append(impls, im.Name+map[bool]string{true: "*"}[im.Template])
		t.Logf("impl %s fields=%v notes=%v tests=%d made=%d", im.Name, im.Fields, im.Notes, im.Tests, im.Made)
	}
	for _, r := range p.Made {
		made = append(made, r.Name+"→"+r.Via)
	}
	for _, r := range p.Holders {
		holders = append(holders, r.Name)
	}
	for _, r := range p.Consumers {
		consumers = append(consumers, r.Name+":"+r.Via)
	}
	for _, r := range p.Downstream {
		down = append(down, r.Name)
	}
	for _, r := range p.Tests {
		tests = append(tests, r.Name)
	}
	for _, r := range p.Routes {
		routes = append(routes, r.Method+" "+r.Path)
	}
	for _, s := range p.Steps {
		t.Logf("step: %s", s.Text)
	}
	for _, b := range p.Breakpoints {
		t.Logf("breakpoint %s:%d %s", filepath.Base(b.SiteFile), b.SiteLine, b.Via)
	}
	t.Logf("why=%s methods=%v impls=%v made=%v holders=%v consumers=%v up=%d routes=%v down=%v tests=%v", p.Why, methods, impls, made, holders, consumers, len(p.Upstream), routes, down, tests)

	if names(methods) != "Name Exchange" {
		t.Errorf("methods = %v", methods)
	}
	if names(impls) != "GoogleProvider SteamProvider*" {
		t.Errorf("impls = %v (the template marked *)", impls)
	}
	if len(made) != 1 || !strings.Contains(names(made), "GoogleProvider and SteamProvider") {
		t.Errorf("made = %v", made)
	}
	if !strings.Contains(names(holders), "providers") {
		t.Errorf("holders = %v", holders)
	}
	if !strings.Contains(names(consumers), "Login:calls Exchange") {
		t.Errorf("consumers = %v", consumers)
	}
	if !strings.Contains(names(routes), "/login") {
		t.Errorf("routes = %v", routes)
	}
	if !strings.Contains(names(down), "FindIdentity") {
		t.Errorf("downstream = %v", down)
	}
	if len(tests) == 0 || len(p.Steps) < 3 || len(p.Breakpoints) == 0 {
		t.Errorf("tests = %v, steps = %d, breakpoints = %d", tests, len(p.Steps), len(p.Breakpoints))
	}
	if !strings.HasPrefix(p.Steps[0].Text, "Add a type that implements Provider: Name() string; Exchange(") {
		t.Errorf("first step = %q", p.Steps[0].Text)
	}
}

func TestTask_WithoutAnInterface(t *testing.T) {
	ix := load(t)
	a := Task(ix, "session", []string{auth + "Session"})
	if a.Point != nil || len(a.Related) != 1 || a.Feature == nil {
		t.Errorf("answer = %+v", a)
	}
	if a := Task(ix, "nothing", []string{"nope"}); a.Point != nil || a.Feature != nil || a.Note == "" {
		t.Errorf("nothing = %+v", a)
	}
}

// TestFeature_Authentication follows sign-in end to end: the route it comes
// in through, what runs in order, where it keeps its data, how it fails,
// its tests and where to stop.
func TestFeature_Authentication(t *testing.T) {
	ix := load(t)
	f := FeatureOf(ix, []string{auth + "Service.Login", auth + "UserIdentity"})
	if f == nil || f.Entry == nil || f.Entry.Label != "POST /login" {
		t.Fatalf("feature = %+v", f)
	}
	var flow, data, errs []string
	for _, s := range f.Flow {
		mark := ""
		if s.Dynamic {
			mark = "~"
		}
		flow = append(flow, strings.Repeat(" ", s.Depth)+s.Name+mark+"["+s.Layer+"]")
	}
	for _, s := range f.Data {
		data = append(data, s.Recv+"."+s.Name)
	}
	for _, s := range f.Errors {
		errs = append(errs, s.Name)
	}
	t.Logf("flow:\n%s", strings.Join(flow, "\n"))
	t.Logf("data=%v errors=%v tests=%d breakpoints=%d", data, errs, len(f.Tests), len(f.Breakpoints))
	all := strings.Join(flow, " ")
	for _, want := range []string{"ServeLogin", "Login", "Exchange~", "FindIdentity"} {
		if !strings.Contains(all, want) {
			t.Errorf("the flow lacks %s", want)
		}
	}
	if !strings.Contains(strings.Join(data, " "), "FindIdentity") || !strings.Contains(strings.Join(errs, " "), "Login") {
		t.Errorf("data = %v, errors = %v", data, errs)
	}
	if len(f.Tests) == 0 || len(f.Breakpoints) < 2 {
		t.Errorf("tests = %d, breakpoints = %d", len(f.Tests), len(f.Breakpoints))
	}
}
