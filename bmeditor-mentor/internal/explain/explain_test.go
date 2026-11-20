package explain

import (
	"path/filepath"
	"strings"
	"testing"

	"bmeditor-mentor/internal/golang"
	"bmeditor-mentor/internal/index"
)

const (
	mod  = "example.com/authsvc"
	auth = mod + "/internal/auth."
)

func load(t *testing.T) *index.Index {
	t.Helper()
	root, _ := filepath.Abs(filepath.Join("..", "golang", "testdata", "authsvc"))
	b := &golang.Builder{Root: root}
	if err := b.Full(t.Context()); err != nil {
		t.Fatal(err)
	}
	return index.Merge(root, b)
}

func keys(refs []Ref) string {
	var out []string
	for _, r := range refs {
		out = append(out, r.Key)
	}
	return strings.Join(out, " ")
}

func TestCard_ForAStruct(t *testing.T) {
	// Arrange
	ix := load(t)

	// Act
	c := CardOf(ix, auth+"UserIdentity")

	// Assert
	if len(c.Fields) != 4 {
		t.Errorf("fields = %s", keys(c.Fields))
	}
	if !strings.Contains(keys(c.Constructors), auth+"SteamProvider.Exchange") {
		t.Errorf("constructors = %s", keys(c.Constructors))
	}
	if len(c.UsedBy) == 0 || c.UsedBy[0].Pkg == mod+"/internal/auth" {
		t.Errorf("used by should list other packages first: %+v", c.UsedBy)
	}
	if len(c.Usages) == 0 || c.Usages[0].Text == "" {
		t.Errorf("usages = %+v", c.Usages)
	}
	if len(c.Tests) == 0 && len(c.IndirectTests) == 0 {
		t.Error("no tests reach it")
	}
	if c.Impact.Symbols == 0 || len(c.Impact.Packages) < 2 {
		t.Errorf("impact = %+v", c.Impact)
	}
}

func TestCard_ForAnInterfaceAndItsImplementation(t *testing.T) {
	ix := load(t)
	c := CardOf(ix, auth+"Repository")
	if !strings.Contains(keys(c.ImplementedBy), mod+"/internal/store.PGStore") || len(c.Methods) != 2 {
		t.Errorf("implemented by = %s, methods = %s", keys(c.ImplementedBy), keys(c.Methods))
	}
	m := CardOf(ix, mod+"/internal/store.PGStore.FindIdentity")
	var through *Ref
	for i := range m.Callers {
		if m.Callers[i].Key == auth+"Service.Login" {
			through = &m.Callers[i]
		}
	}
	if through == nil || through.Certainty != index.Dynamic {
		t.Errorf("Login calls it through Repository: %+v", m.Callers)
	}
	if m.Owner == nil || m.Owner.Name != "PGStore" {
		t.Errorf("owner = %+v", m.Owner)
	}
}

func TestCard_ForAHandlerHasItsRoute(t *testing.T) {
	ix := load(t)
	c := CardOf(ix, mod+"/internal/api.LoginHandler.ServeLogin")
	if len(c.Routes) != 1 || c.Routes[0].Path != "/login" {
		t.Errorf("routes = %+v", c.Routes)
	}
}

func TestTrace_FollowsIntoImplementations(t *testing.T) {
	ix := load(t)
	f := Trace(ix, mod+"/internal/api.LoginHandler.ServeLogin", 4)
	if f == nil || len(f.Root.Children) != 1 || f.Root.Children[0].Key != auth+"Service.Login" {
		t.Fatalf("flow = %+v", f)
	}
	login := f.Root.Children[0]
	var exchange *Step
	for _, c := range login.Children {
		if c.Key == auth+"Provider.Exchange" {
			exchange = c
		}
	}
	if exchange == nil || exchange.Certainty != index.Dynamic || exchange.Choices != 2 || len(exchange.Children) != 2 {
		t.Fatalf("Exchange = %+v", exchange)
	}
	// In the order of the code: Exchange comes before newSession.
	if login.Children[0].Key != auth+"Provider.Exchange" || login.Children[len(login.Children)-1].Key != auth+"newSession" {
		var got []string
		for _, c := range login.Children {
			got = append(got, c.Name)
		}
		t.Errorf("order = %v", got)
	}
	for _, c := range exchange.Children {
		if strings.Contains(c.Key, "memRepo") {
			t.Error("a test double ran in the flow")
		}
	}
}

func TestPackages_GuessesLayersAndKeepsImports(t *testing.T) {
	ix := load(t)
	g := Packages(ix)
	layer := map[string]string{}
	for _, n := range g.Nodes {
		layer[n.Label] = n.Layer
	}
	want := map[string]string{"cmd/server": "Entry", "internal/api": "API", "internal/store": "Repository"}
	for k, v := range want {
		if layer[k] != v {
			t.Errorf("layer of %s = %q, want %q", k, layer[k], v)
		}
	}
	found := false
	for _, e := range g.Edges {
		if e.From == mod+"/internal/api" && e.To == mod+"/internal/auth" && e.Count > 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("edges = %+v", g.Edges)
	}
	if len(g.Route) != 1 {
		t.Errorf("routes = %+v", g.Route)
	}
}

func TestPackage_ShowsOutsideUsers(t *testing.T) {
	ix := load(t)
	g := Package(ix, mod+"/internal/auth")
	var ext, dyn bool
	for _, n := range g.Nodes {
		if n.External && n.ID == mod+"/internal/api" {
			ext = true
		}
	}
	for _, e := range g.Edges {
		if e.Certainty == index.Dynamic {
			dyn = true
		}
	}
	if !ext || !dyn {
		t.Errorf("nodes = %+v\nedges = %+v", g.Nodes, g.Edges)
	}
}

func TestEntries(t *testing.T) {
	ix := load(t)
	es := Entries(ix)
	if len(es) != 2 || es[0].Kind != "route" || es[0].Label != "POST /login" || es[1].Kind != "main" {
		t.Errorf("entries = %+v", es)
	}
}

func TestExamples_ShowHowAStructIsUsed(t *testing.T) {
	// Arrange
	ix := load(t)

	// Act
	ex := ExamplesOf(ix, auth+"UserIdentity", 3)

	// Assert
	if ex == nil || ex.Total < 3 || len(ex.Items) != 3 {
		t.Fatalf("examples = %+v", ex)
	}
	fns := map[string]bool{}
	for i, it := range ex.Items {
		t.Logf("%s in %s (%s): %s", it.How, it.In, it.Pkg, strings.Fields(strings.Split(it.Text, "\n")[it.Line-it.First])[0])
		if fns[it.In] {
			t.Errorf("two examples from %s", it.In)
		}
		fns[it.In] = true
		if it.Test {
			t.Errorf("a test was picked before code: %+v", it)
		}
		if it.Line < it.First || !strings.Contains(strings.Split(it.Text, "\n")[it.Line-it.First], "UserIdentity") && i == 0 {
			t.Errorf("the first example's line does not show it: %+v", it)
		}
	}
	// Code from another package comes first: it shows how it is used from outside.
	if ex.Items[0].Pkg == "auth" {
		t.Errorf("first example is from its own package: %+v", ex.Items[0])
	}
	if ExamplesOf(ix, "nope", 3) != nil {
		t.Error("an unknown key should have no examples")
	}
}
