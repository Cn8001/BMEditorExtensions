// Package embed turns text into vectors for semantic search. It is off
// unless the developer picks a provider; the only one is a local Ollama,
// so code stays on the machine.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// Provider makes embeddings.
type Provider interface {
	Model() string
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Ollama asks an Ollama server for embeddings (POST /api/embed).
type Ollama struct {
	URL    string // http://localhost:11434 when empty
	Name   string // the model, such as nomic-embed-text
	Client *http.Client
}

func (o *Ollama) Model() string { return "ollama:" + o.Name }

func (o *Ollama) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	url := strings.TrimRight(o.URL, "/")
	if url == "" {
		url = "http://localhost:11434"
	}
	c := o.Client
	if c == nil {
		c = &http.Client{Timeout: 2 * time.Minute}
	}
	body, _ := json.Marshal(map[string]any{"model": o.Name, "input": texts})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama at %s: %w", url, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: %s: %s", res.Status, strings.TrimSpace(string(b)))
	}
	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	if len(out.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama: %d embeddings for %d texts", len(out.Embeddings), len(texts))
	}
	return out.Embeddings, nil
}

// Cosine is the cosine similarity of a and b, 0 when either is empty or
// they differ in length.
func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
