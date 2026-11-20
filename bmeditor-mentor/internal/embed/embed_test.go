package embed

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllama_Embed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Model string
			Input []string
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if r.URL.Path != "/api/embed" || in.Model != "m" {
			http.Error(w, "bad", 400)
			return
		}
		out := make([][]float32, len(in.Input))
		for i := range out {
			out[i] = []float32{float32(i), 1}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": out})
	}))
	defer srv.Close()
	o := &Ollama{URL: srv.URL, Name: "m"}
	vecs, err := o.Embed(t.Context(), []string{"a", "b"})
	if err != nil || len(vecs) != 2 || vecs[1][0] != 1 {
		t.Fatalf("vecs = %v, %v", vecs, err)
	}
}

func TestCosine(t *testing.T) {
	if c := Cosine([]float32{1, 0}, []float32{1, 0}); c < 0.999 {
		t.Errorf("same = %v", c)
	}
	if c := Cosine([]float32{1, 0}, []float32{0, 1}); c != 0 {
		t.Errorf("orthogonal = %v", c)
	}
	if c := Cosine([]float32{1}, []float32{1, 0}); c != 0 {
		t.Errorf("lengths differ = %v", c)
	}
}
