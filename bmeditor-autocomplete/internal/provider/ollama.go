package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bmeditor-autocomplete/internal/fim"
)

var httpClient = &http.Client{Timeout: 60 * time.Second}

// postJSON posts body as JSON and decodes the answer into out.
func postJSON(ctx context.Context, url string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode/100 != 2 {
		msg := strings.TrimSpace(string(data))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return fmt.Errorf("%s: %s %s", url, res.Status, msg)
	}
	return json.Unmarshal(data, out)
}

// ollama is a model Ollama serves, completed in the middle: Ollama turns
// prompt and suffix into the model's own fill-in-the-middle format.
type ollama struct {
	url, model string
}

// DefaultOllamaModel is a small code model that fills in the middle.
const DefaultOllamaModel = "qwen2.5-coder:1.5b"

func newOllama(c Config) *ollama {
	url := strings.TrimRight(c.OllamaURL, "/")
	if url == "" {
		url = "http://localhost:11434"
	}
	model := c.OllamaModel
	if model == "" {
		model = DefaultOllamaModel
	}
	return &ollama{url: url, model: model}
}

func (p *ollama) Ready() (bool, string) { return true, "" }

// Prepare checks that Ollama runs and has the model.
func (p *ollama) Prepare(ctx context.Context, progress func(string)) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.url+"/api/tags", nil)
	res, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama is not running at %s: start it, or install it from ollama.com", p.url)
	}
	defer res.Body.Close()
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	_ = json.NewDecoder(res.Body).Decode(&tags)
	for _, m := range tags.Models {
		if m.Name == p.model || strings.TrimSuffix(m.Name, ":latest") == p.model {
			progress("Ollama has " + p.model)
			return nil
		}
	}
	return fmt.Errorf("Ollama has no %s: run ollama pull %s", p.model, p.model)
}

func (p *ollama) Close() {}

func (p *ollama) Complete(ctx context.Context, r fim.Request) (string, error) {
	prefix := r.Prefix
	if len(r.Extra) > 0 {
		var b strings.Builder
		for _, e := range r.Extra {
			b.WriteString(commentLine(r.Language, "File: "+e.Path) + e.Text + "\n")
		}
		prefix = b.String() + commentLine(r.Language, "File: "+r.Path) + prefix
	}
	var out struct {
		Response string `json:"response"`
	}
	err := postJSON(ctx, p.url+"/api/generate", map[string]any{
		"model":   p.model,
		"prompt":  prefix,
		"suffix":  r.Suffix,
		"stream":  false,
		"options": map[string]any{"num_predict": 128, "temperature": 0.2},
	}, &out)
	return out.Response, err
}

// commentLine is a line naming a file, as a comment of the language; a
// code model reads it as the start of another file.
func commentLine(language, text string) string {
	switch language {
	case "python", "shell", "yaml", "toml", "ruby", "powershell", "makefile", "dockerfile", "r", "perl":
		return "# " + text + "\n"
	case "sql", "lua", "haskell":
		return "-- " + text + "\n"
	case "html", "xml", "svg", "vue", "svelte", "markdown":
		return "<!-- " + text + " -->\n"
	case "batch":
		return "rem " + text + "\n"
	}
	return "// " + text + "\n"
}
