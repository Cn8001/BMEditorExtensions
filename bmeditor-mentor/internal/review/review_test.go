package review

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"bmeditor-mentor/internal/golang"
	"bmeditor-mentor/internal/index"
	"bmeditor-mentor/internal/store"
)

const key = "example.com/authsvc/internal/auth.Session.Expired"

func TestMark_GoesStaleWhenTheCodeChanges(t *testing.T) {
	// Arrange
	root := t.TempDir()
	from, _ := filepath.Abs(filepath.Join("..", "golang", "testdata", "authsvc"))
	_ = os.CopyFS(root, os.DirFS(from))
	b := &golang.Builder{Root: root}
	if err := b.Full(t.Context()); err != nil {
		t.Fatal(err)
	}
	ix := index.Merge(root, b)
	db, _ := store.Open(filepath.Join(t.TempDir(), "m.db"))
	defer db.Close()
	bk, _ := Open(t.Context(), db)
	if bk.StateOf(ix, key) != Unseen {
		t.Fatal("not looked at yet")
	}
	_ = bk.Mark(t.Context(), ix, []string{key}, "")
	file := filepath.Join(root, "internal", "auth", "session.go")
	src, _ := os.ReadFile(file)
	src = append(src[:len(src)-len("func (s *Session) Expired(now time.Time) bool { return now.After(s.ExpiresAt) }\n")],
		[]byte("func (s *Session) Expired(now time.Time) bool { return !now.Before(s.ExpiresAt) }\n")...)
	_ = os.WriteFile(file, src, 0o644)
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(file, later, later)

	// Act
	dirs, _ := b.Changed()
	_ = b.Update(t.Context(), dirs)
	after := index.Merge(root, b)

	// Assert
	if bk.StateOf(ix, key) != Reviewed {
		t.Error("the old index should still say reviewed")
	}
	if bk.StateOf(after, key) != Stale || len(bk.StaleKeys(after)) != 1 {
		t.Errorf("after the change = %s", bk.StateOf(after, key))
	}
	again, _ := Open(t.Context(), db)
	if again.StateOf(after, key) != Stale {
		t.Error("the mark was not kept")
	}
}
