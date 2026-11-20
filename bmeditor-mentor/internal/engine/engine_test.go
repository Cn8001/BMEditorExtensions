package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"bmeditor-mentor/internal/search"
)

func TestEngine_ReadsAgainWhatChanged(t *testing.T) {
	// Arrange
	root := t.TempDir()
	from, _ := filepath.Abs(filepath.Join("..", "golang", "testdata", "authsvc"))
	_ = os.CopyFS(root, os.DirFS(from))
	states := make(chan string, 64)
	e, err := Open(root, t.TempDir(), Settings{}, func(method string, p any) {
		if st, ok := p.(Status); ok {
			select {
			case states <- st.State:
			default:
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if _, err := e.Index(t.Context()); err != nil {
		t.Fatal(err)
	}

	// Act
	file := filepath.Join(root, "internal", "users", "model.go")
	src, _ := os.ReadFile(file)
	_ = os.WriteFile(file, append(src, []byte("\n// Rename changes a user's name.\nfunc (u *User) Rename(name string) { u.Name = name }\n")...), 0o644)
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(file, later, later)

	// Assert
	deadline := time.After(15 * time.Second)
	for {
		if ix := e.Peek(); ix != nil && ix.Symbols["example.com/authsvc/internal/users.User.Rename"] != nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("the change was not read")
		case <-time.After(100 * time.Millisecond):
		}
	}
	res, err := e.Search(t.Context(), "rename user", search.Options{})
	if err != nil || len(res.Candidates) < 2 || (res.Candidates[0].Name != "Rename" && res.Candidates[1].Name != "Rename") {
		t.Errorf("search = %+v, %v", res, err)
	}
}

// TestRealFolder reads MENTOR_REAL, a real repository, and logs how long
// it took and what it found.
func TestRealFolder(t *testing.T) {
	root := os.Getenv("MENTOR_REAL")
	if root == "" {
		t.Skip("MENTOR_REAL not set")
	}
	start := time.Now()
	e, err := Open(root, t.TempDir(), Settings{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if _, err := e.Index(t.Context()); err != nil {
		t.Fatal(err)
	}
	st := e.Status()
	t.Logf("%s: %d packages, %d symbols, %d files in %v; errors: %v", root, st.Packages, st.Symbols, st.Files, time.Since(start), st.Errors)
	for _, q := range []string{"start the language server", "settings of a project", "run a docker container"} {
		qs := time.Now()
		res, err := e.Search(t.Context(), q, search.Options{Limit: 5})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%q in %v:", q, time.Since(qs))
		for _, c := range res.Candidates {
			t.Logf("  %5.2f %s %s — %s", c.Score, c.Kind, c.Key, c.Responsibility)
		}
	}
}
