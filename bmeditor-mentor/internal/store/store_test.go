package store

import (
	"path/filepath"
	"testing"
	"time"
)

func open(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "sub", "mentor.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMatch_RanksNamesOverContext(t *testing.T) {
	// Arrange
	db := open(t)
	err := db.ReplaceDocs(t.Context(), []Doc{
		{Key: "a.UserIdentity", Name: "user identity", Fields: "provider provider user id", Doc: "links a user to an identity provider"},
		{Key: "a.Session", Name: "session", Fields: "token user id", Doc: "an active login"},
		{Key: "a.Other", Name: "other", Context: "identity"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Act
	hits, err := db.Match(t.Context(), []string{"identities", "provider"}, 10)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].Key != "a.UserIdentity" || hits[1].Key != "a.Other" {
		t.Errorf("hits = %+v", hits)
	}
}

func TestReplaceDocs_Replaces(t *testing.T) {
	db := open(t)
	_ = db.ReplaceDocs(t.Context(), []Doc{{Key: "a", Name: "alpha"}})
	_ = db.ReplaceDocs(t.Context(), []Doc{{Key: "b", Name: "beta"}})
	if hits, _ := db.Match(t.Context(), []string{"alpha"}, 10); len(hits) != 0 {
		t.Errorf("old docs stayed: %+v", hits)
	}
}

func TestReviews_RoundTrip(t *testing.T) {
	db := open(t)
	_ = db.SaveReviews(t.Context(), []Review{{Key: "a.F", Hash: "h1", At: time.Now()}})
	_ = db.SaveReviews(t.Context(), []Review{{Key: "a.F", Hash: "h2", Note: "ok", At: time.Now()}})
	got, err := db.Reviews(t.Context())
	if err != nil || got["a.F"].Hash != "h2" || got["a.F"].Note != "ok" {
		t.Errorf("reviews = %+v, %v", got, err)
	}
	_ = db.ForgetReviews(t.Context(), []string{"a.F"})
	if got, _ := db.Reviews(t.Context()); len(got) != 0 {
		t.Errorf("forgotten = %+v", got)
	}
}

func TestEmbeddings_RoundTrip(t *testing.T) {
	db := open(t)
	_ = db.SaveEmbeddings(t.Context(), "m", map[string]Vector{"a": {Hash: "h", Vec: []float32{0.5, -1, 2}}})
	got, _ := db.Embeddings(t.Context(), "m")
	if v := got["a"].Vec; len(v) != 3 || v[1] != -1 {
		t.Errorf("vec = %v", v)
	}
	if other, _ := db.Embeddings(t.Context(), "n"); len(other) != 0 {
		t.Error("another model's vectors came back")
	}
}

func TestReopen_KeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.db")
	db, _ := Open(path)
	_ = db.SetMeta(t.Context(), "k", "v")
	db.Close()
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if db.Meta(t.Context(), "k") != "v" {
		t.Error("meta lost")
	}
}

func TestDecisions_ByKey(t *testing.T) {
	db := open(t)
	_, _ = db.AddDecision(t.Context(), Decision{Claim: "c", Rationale: "r", Source: "agent", Status: "recorded", Keys: []string{"a.X"}})
	_, _ = db.AddDecision(t.Context(), Decision{Claim: "d", Source: "user", Status: "recorded", Keys: []string{"a.Y"}})
	got, _ := db.Decisions(t.Context(), "a.X")
	if len(got) != 1 || got[0].Claim != "c" {
		t.Errorf("decisions = %+v", got)
	}
}
