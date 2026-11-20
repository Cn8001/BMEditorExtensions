package search

import (
	"path/filepath"
	"testing"

	"bmeditor-mentor/internal/golang"
	"bmeditor-mentor/internal/index"
	"bmeditor-mentor/internal/store"
)

const auth = "example.com/authsvc/internal/auth."

func setup(t *testing.T) (*index.Index, *store.DB) {
	t.Helper()
	root, _ := filepath.Abs(filepath.Join("..", "golang", "testdata", "authsvc"))
	b := &golang.Builder{Root: root}
	if err := b.Full(t.Context()); err != nil {
		t.Fatal(err)
	}
	ix := index.Merge(root, b)
	db, err := store.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ReplaceDocs(t.Context(), Docs(ix)); err != nil {
		t.Fatal(err)
	}
	return ix, db
}

func TestQuery_RanksTheIdentityStructForAProviderQuestion(t *testing.T) {
	// Arrange
	ix, db := setup(t)

	// Act
	res, err := Query(t.Context(), ix, db, "Which struct should I use to persist the provider identity linked to a user?", Options{})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) == 0 || res.Candidates[0].Key != auth+"UserIdentity" {
		for _, c := range res.Candidates {
			t.Logf("%5.2f %s %v", c.Score, c.Key, c.Matched)
		}
		t.Fatalf("first = %+v", res.Candidates)
	}
	top := res.Candidates[0]
	if top.Derived || top.Uses == 0 || len(top.Matched) == 0 {
		t.Errorf("top = %+v", top)
	}
}

func TestQuery_SessionForActiveLogin(t *testing.T) {
	ix, db := setup(t)
	res, _ := Query(t.Context(), ix, db, "where is the active login kept after sign in", Options{Kinds: []string{"struct"}})
	if len(res.Candidates) == 0 || res.Candidates[0].Key != auth+"Session" {
		for _, c := range res.Candidates {
			t.Logf("%5.2f %s %v", c.Score, c.Key, c.Matched)
		}
		t.Fatal("Session should come first")
	}
}

func TestQuery_SaysHowLookAlikesDiffer(t *testing.T) {
	ix, db := setup(t)
	res, _ := Query(t.Context(), ix, db, "user identity session struct", Options{})
	var ui *Candidate
	for i := range res.Candidates {
		if res.Candidates[i].Key == auth+"UserIdentity" {
			ui = &res.Candidates[i]
		}
	}
	if ui == nil || len(ui.Differs) == 0 {
		t.Fatalf("UserIdentity = %+v", ui)
	}
}

func TestQuery_LeavesOutTestsUnlessAsked(t *testing.T) {
	ix, db := setup(t)
	res, _ := Query(t.Context(), ix, db, "login links identity", Options{})
	if len(res.Candidates) > 0 && res.Candidates[0].Key == auth+"TestLogin_LinksIdentity" {
		t.Error("a test came first")
	}
}

func TestQuery_SemanticAddsCandidates(t *testing.T) {
	ix, db := setup(t)
	vecs := map[string]store.Vector{auth + "Session": {Vec: []float32{1, 0}}, auth + "UserIdentity": {Vec: []float32{0, 1}}}
	res, _ := Query(t.Context(), ix, db, "zzzz unrelated words", Options{QueryVec: []float32{1, 0.1}, Vectors: vecs})
	if !res.Semantic || len(res.Candidates) == 0 || res.Candidates[0].Key != auth+"Session" {
		t.Errorf("res = %+v", res)
	}
}

func TestWords(t *testing.T) {
	words, kinds := Words("Which struct should I use to store user login information?")
	if len(kinds) == 0 || kinds[0] != "struct" {
		t.Errorf("kinds = %v", kinds)
	}
	want := []string{"store", "user", "login", "information"}
	if len(words) != len(want) {
		t.Fatalf("words = %v", words)
	}
	for i := range want {
		if words[i] != want[i] {
			t.Errorf("words = %v", words)
		}
	}
}
