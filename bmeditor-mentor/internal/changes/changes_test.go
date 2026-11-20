package changes

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bmeditor-mentor/internal/golang"
	"bmeditor-mentor/internal/index"
)

const auth = "example.com/authsvc/internal/auth."

// repo is the fixture in a git repository of its own, committed.
func repo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	from, _ := filepath.Abs(filepath.Join("..", "golang", "testdata", "authsvc"))
	if err := os.CopyFS(root, os.DirFS(from)); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "commit.gpgsign=false", "commit", "-q", "-m", "base"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	return root
}

func index_(t *testing.T, root string) *index.Index {
	t.Helper()
	b := &golang.Builder{Root: root}
	if err := b.Full(t.Context()); err != nil {
		t.Fatal(err)
	}
	return index.Merge(root, b)
}

func edit(t *testing.T, root, rel, from, to string) {
	t.Helper()
	path := filepath.Join(root, rel)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if from != "" && !strings.Contains(string(b), from) {
		t.Fatalf("%s has no %q", rel, from)
	}
	s := string(b)
	if from == "" {
		s += to
	} else {
		s = strings.Replace(s, from, to, 1)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// agentWork is what an agent might do to add a sign-in provider: a new
// type, wired in at main, a field more on Session, a branch in Login, and
// a method gone.
func agentWork(t *testing.T, root string) {
	edit(t, root, "internal/auth/provider.go", "", `
// DiscordProvider signs users in with Discord.
type DiscordProvider struct{ ClientID string }

func (DiscordProvider) Name() string { return "discord" }

func (p DiscordProvider) Exchange(ctx context.Context, code string) (UserIdentity, error) {
	return UserIdentity{Provider: "discord", ProviderUserID: code}, nil
}
`)
	edit(t, root, "cmd/server/main.go", "auth.GoogleProvider{})", "auth.GoogleProvider{}, auth.DiscordProvider{})")
	edit(t, root, "internal/auth/session.go", "	RefreshToken string\n", "	RefreshToken string\n	Scope        string\n")
	edit(t, root, "internal/auth/session.go", "// Expired says whether the session can no longer be used.\nfunc (s *Session) Expired(now time.Time) bool { return now.After(s.ExpiresAt) }\n", "")
	edit(t, root, "internal/auth/service.go", "	if found == nil {", "	if found == nil && code != \"\" {")
}

func byKey(rep *Report) map[string]Change {
	out := map[string]Change{}
	for _, u := range rep.Units {
		for _, c := range u.Changes {
			out[c.Key] = c
		}
	}
	return out
}

func unitOf(rep *Report, key string) *Unit {
	for i, u := range rep.Units {
		for _, c := range u.Changes {
			if c.Key == key {
				return &rep.Units[i]
			}
		}
	}
	return nil
}

func TestFromHead_SaysWhatTheAgentDid(t *testing.T) {
	// Arrange
	root := repo(t)
	agentWork(t, root)
	ix := index_(t, root)

	// Act
	base, files, err := FromHead(t.Context(), ix)
	if err != nil {
		t.Fatal(err)
	}
	rep := Build(ix, base, Diff(ix, base), files)

	// Assert
	got := byKey(rep)
	want := map[string]string{
		auth + "DiscordProvider":              "added",
		auth + "DiscordProvider.Exchange":     "added",
		auth + "Session.Scope":                "added",
		auth + "Session":                      "modified",
		auth + "Session.Expired":              "removed",
		auth + "Service.Login":                "modified",
		"example.com/authsvc/cmd/server.main": "modified",
	}
	for k, v := range want {
		if got[k].Change != v {
			t.Errorf("%s: %q, want %q", k, got[k].Change, v)
		}
	}
	if c, ok := got[auth+"SteamProvider"]; ok {
		t.Errorf("SteamProvider did not change: %+v", c)
	}
	if got[auth+"Service.Login"].ShapeChanged {
		t.Error("Login's signature did not change")
	}

	// The new provider is wired in at main and implements Provider.
	u := unitOf(rep, auth+"DiscordProvider")
	if u == nil {
		t.Fatal("no unit for DiscordProvider")
	}
	relies := ""
	for _, r := range u.ReliesOn {
		relies += r.Key + " "
	}
	if !strings.Contains(relies, auth+"Provider") {
		t.Errorf("relies on = %s", relies)
	}
	removed := false
	for _, un := range rep.Units {
		for _, r := range un.Risks {
			if r == "Removes Session.Expired." {
				removed = true
			}
		}
	}
	if !removed {
		t.Errorf("risks = %+v", rep.Units)
	}
	login := unitOf(rep, auth+"Service.Login")
	if login == nil || len(login.Tests) == 0 || len(login.PlugsIn) == 0 || len(login.Routes) != 1 {
		t.Errorf("Login's unit = %+v", login)
	}
	if rep.Lines[0] == 0 || rep.Totals["added"] < 4 {
		t.Errorf("totals = %v, lines = %v", rep.Totals, rep.Lines)
	}
}

func TestSnapshot_SeesOnlyWhatChangedSince(t *testing.T) {
	// Arrange
	root := repo(t)
	edit(t, root, "internal/users/model.go", "", "\n// Rename changes a user's name.\nfunc (u *User) Rename(name string) { u.Name = name }\n")
	ix := index_(t, root)
	base := Snapshot(t.Context(), ix, "step 1")

	// Act
	edit(t, root, "internal/auth/service.go", "	if found == nil {", "	if found == nil && code != \"\" {")
	later := time.Now().Add(time.Second)
	_ = os.Chtimes(filepath.Join(root, "internal/auth/service.go"), later, later)
	list := Diff(index_(t, root), base)

	// Assert
	if len(list) != 1 || list[0].Key != auth+"Service.Login" || list[0].Change != "modified" {
		t.Errorf("changes = %+v", list)
	}
}

func TestDiff_SignatureChange(t *testing.T) {
	root := repo(t)
	edit(t, root, "internal/auth/service.go", "func NewService(repo Repository, providers ...Provider) *Service {", "func NewService(repo Repository, strict bool, providers ...Provider) *Service {")
	edit(t, root, "cmd/server/main.go", "auth.NewService(db, ", "auth.NewService(db, true, ")
	edit(t, root, "internal/auth/service_test.go", "NewService(repo, ", "NewService(repo, false, ")
	ix := index_(t, root)
	base, files, _ := FromHead(t.Context(), ix)
	rep := Build(ix, base, Diff(ix, base), files)
	c := byKey(rep)[auth+"NewService"]
	if !c.ShapeChanged || !strings.Contains(c.Shape, "strict bool") {
		t.Fatalf("NewService = %+v", c)
	}
	found := false
	for _, u := range rep.Units {
		for _, r := range u.Risks {
			if strings.HasPrefix(r, "Changes the signature of NewService, which 1 other package uses") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("units = %+v", rep.Units)
	}
}
