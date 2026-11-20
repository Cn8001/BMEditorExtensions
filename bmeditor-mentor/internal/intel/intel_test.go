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
	if a.Point != nil || len(a.Related) != 1 || a.Note == "" {
		t.Errorf("answer = %+v", a)
	}
}
