package golang

import (
	"bmeditor-mentor/internal/index"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const mod = "example.com/authsvc"

func fixture(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", "authsvc"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// copyTree copies the fixture where a test may change it.
func copyTree(t *testing.T, from string) string {
	t.Helper()
	to := t.TempDir()
	err := filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		dst := filepath.Join(to, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return to
}

func build(t *testing.T, root string) (*Builder, *index.Index) {
	t.Helper()
	b := &Builder{Root: root}
	if err := b.Full(t.Context()); err != nil {
		t.Fatal(err)
	}
	return b, index.Merge(root, b)
}

func hasRel(ix *index.Index, from, to string, kind index.RelKind) bool {
	for _, r := range ix.Out[from] {
		if r.To == to && r.Kind == kind {
			return true
		}
	}
	return false
}

func TestFull_ReadsSymbolsWithTheirShape(t *testing.T) {
	// Arrange
	root := fixture(t)

	// Act
	_, ix := build(t, root)

	// Assert
	ui := ix.Symbols[mod+"/internal/auth.UserIdentity"]
	if ui == nil || ui.Kind != index.KindStruct || len(ui.Fields) != 4 {
		t.Fatalf("UserIdentity = %+v", ui)
	}
	if index.FirstSentence(ui.Doc) != "UserIdentity links a user to their account at an outside identity provider, such as Steam or Google." {
		t.Errorf("doc = %q", ui.Doc)
	}
	login := ix.Symbols[mod+"/internal/auth.Service.Login"]
	if login == nil || login.Kind != index.KindMethod || !login.ReturnsError || login.ErrReturns != 4 || login.Branches < 4 {
		t.Errorf("Login = %+v", login)
	}
	if f := ix.Symbols[mod+"/internal/auth.UserIdentity.ProviderUserID"]; f == nil || f.Kind != index.KindField {
		t.Errorf("field = %+v", f)
	}
	if tst := ix.Symbols[mod+"/internal/auth.TestLogin_LinksIdentity"]; tst == nil || !tst.Test {
		t.Errorf("test = %+v", tst)
	}
	if len(ix.Errors) != 0 {
		t.Errorf("errors = %v", ix.Errors)
	}
}

func TestFull_ReadsRelations(t *testing.T) {
	_, ix := build(t, fixture(t))
	auth := mod + "/internal/auth."
	cases := []struct {
		from, to string
		kind     index.RelKind
	}{
		{mod + "/internal/api.LoginHandler.ServeLogin", auth + "Service.Login", index.RelCalls},
		{auth + "Service.Login", auth + "Provider.Exchange", index.RelCallsDynamic},
		{auth + "Service.Login", auth + "Repository.FindIdentity", index.RelCallsDynamic},
		{auth + "Service.Login", auth + "newSession", index.RelCalls},
		{auth + "NewService", auth + "Service", index.RelReturns},
		{auth + "newSession", auth + "Session", index.RelConstructs},
		{auth + "SteamProvider", auth + "Provider", index.RelImplements},
		{auth + "GoogleProvider", auth + "Provider", index.RelImplements},
		// Across packages read separately, with a named type in the signature.
		{mod + "/internal/store.PGStore", auth + "Repository", index.RelImplements},
		{mod + "/internal/store.PGStore.FindIdentity", auth + "UserIdentity.ProviderUserID", index.RelReferences},
		{auth + "TestLogin_LinksIdentity", auth + "NewService", index.RelCalls},
		{auth + "Service.Login", auth + "UserIdentity.ProviderUserID", index.RelReferences},
	}
	for _, c := range cases {
		if !hasRel(ix, c.from, c.to, c.kind) {
			t.Errorf("missing %s -%s-> %s", c.from, c.kind, c.to)
		}
	}
	if hasRel(ix, auth+"Session", auth+"Provider", index.RelImplements) {
		t.Error("Session does not implement Provider")
	}
	for _, r := range ix.Out[auth+"Service.Login"] {
		if r.Kind == index.RelCallsDynamic && r.Certainty != index.Dynamic {
			t.Errorf("a call through an interface is %s", r.Certainty)
		}
	}
}

func TestFull_FindsTheRouteAndItsHandler(t *testing.T) {
	_, ix := build(t, fixture(t))
	if len(ix.Routes) != 1 {
		t.Fatalf("routes = %+v", ix.Routes)
	}
	r := ix.Routes[0]
	if r.Method != "POST" || r.Path != "/login" || r.Handler != mod+"/internal/api.LoginHandler.ServeLogin" {
		t.Errorf("route = %+v", r)
	}
}

func TestAt_FindsTheInnermostDeclaration(t *testing.T) {
	_, ix := build(t, fixture(t))
	file := filepath.Join(fixture(t), "internal", "auth", "service.go")
	if s := ix.At(file, 26); s == nil || s.Name != "Login" {
		t.Errorf("At(26) = %+v", s)
	}
	if s := ix.At(file, 10); s == nil || s.Name != "repo" {
		t.Errorf("At(10) = %+v", s)
	}
}

func TestUpdate_ReadsAChangedPackageAndItsImporters(t *testing.T) {
	// Arrange
	root := copyTree(t, fixture(t))
	b, before := build(t, root)
	file := filepath.Join(root, "internal", "auth", "session.go")
	src, _ := os.ReadFile(file)
	src = append(src, []byte("\n// Revoke ends the session now.\nfunc (s *Session) Revoke() { s.ExpiresAt = time.Time{} }\n")...)
	if err := os.WriteFile(file, src, 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(file, later, later)

	// Act
	dirs, full := b.Changed()
	err := b.Update(t.Context(), dirs)
	after := index.Merge(root, b)

	// Assert
	if err != nil || full || len(dirs) != 1 {
		t.Fatalf("changed = %v %v, err = %v", dirs, full, err)
	}
	if after.Symbols[mod+"/internal/auth.Session.Revoke"] == nil {
		t.Error("the new method is missing")
	}
	if before.Symbols[mod+"/internal/auth.Session.Revoke"] != nil {
		t.Error("the old index changed")
	}
	if !hasRel(after, mod+"/internal/api.LoginHandler.ServeLogin", mod+"/internal/auth.Service.Login", index.RelCalls) {
		t.Error("the importer's call was lost")
	}
	if dirs, _ := b.Changed(); len(dirs) != 0 {
		t.Errorf("still changed: %v", dirs)
	}
}

func TestHash_IgnoresSpacing(t *testing.T) {
	if hashOf([]byte("func f() {\n\treturn 1\n}")) != hashOf([]byte("func f() { return 1 }")) {
		t.Error("spacing changed the hash")
	}
	if hashOf([]byte("func f() { return 1 }")) == hashOf([]byte("func f() { return 2 }")) {
		t.Error("a change to the code kept the hash")
	}
}
