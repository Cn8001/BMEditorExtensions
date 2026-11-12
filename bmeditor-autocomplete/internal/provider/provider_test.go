package provider

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"bmeditor-autocomplete/internal/fim"
)

func cache(t *testing.T) string {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("HOME", dir)
	t.Setenv("BMEDITOR_DATA", dir)
	home, _ := Home()
	return home
}

func TestNew_ResolvesModels(t *testing.T) {
	cache(t)
	for _, c := range []struct {
		model, wantFile string
		wantURL         bool
	}{
		{"", "qwen2.5-coder-0.5b-q8_0.gguf", true},
		{"1.5B", "qwen2.5-coder-1.5b-q8_0.gguf", true},
		{"https://example.com/m/my-model.Q4.gguf?download=1", "my-model.Q4.gguf", true},
		{filepath.Join("C:", "models", "mine.gguf"), "mine.gguf", false},
	} {
		p, err := New(Config{Provider: "local", LocalModel: c.model})
		if err != nil {
			t.Fatalf("%q: %v", c.model, err)
		}
		l := p.(*local)
		if filepath.Base(l.path) != c.wantFile || (l.model.URL != "") != c.wantURL {
			t.Errorf("%q: path %s, url %q", c.model, l.path, l.model.URL)
		}
	}
	if p, err := New(Config{Provider: "local", LocalModel: "custom", ModelFile: "/m/own.gguf"}); err != nil || p.(*local).path != "/m/own.gguf" {
		t.Errorf("custom: %v", err)
	}
	if _, err := New(Config{Provider: "local", LocalModel: "custom"}); err == nil {
		t.Error("custom without a file should be refused")
	}
	if _, err := New(Config{Provider: "local", LocalModel: "7b"}); err == nil {
		t.Error("an unknown model should be refused")
	}
	if _, err := New(Config{Provider: "off"}); err != ErrOff {
		t.Errorf("off: %v", err)
	}
}

func TestInfill_AsksLlamaServer(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/infill" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"content":"return a + b"}`))
	}))
	defer srv.Close()

	out, err := infill(context.Background(), srv.URL, fim.Request{Prefix: "func add(a, b int) int {\n\t", Suffix: "\n}", Extra: []fim.Snippet{{Path: "x.go", Text: "package x"}}})

	if err != nil || out != "return a + b" {
		t.Fatalf("infill = %q, %v", out, err)
	}
	if got["input_prefix"] != "func add(a, b int) int {\n\t" || got["input_suffix"] != "\n}" || len(got["input_extra"].([]any)) != 1 {
		t.Errorf("request = %v", got)
	}
}

func TestOllama_PrepareAndComplete(t *testing.T) {
	var gen map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_, _ = w.Write([]byte(`{"models":[{"name":"qwen2.5-coder:1.5b"}]}`))
		case "/api/generate":
			_ = json.NewDecoder(r.Body).Decode(&gen)
			_, _ = w.Write([]byte(`{"response":"print(x)"}`))
		}
	}))
	defer srv.Close()
	p, _ := New(Config{Provider: "ollama", OllamaURL: srv.URL + "/"})

	if err := p.Prepare(context.Background(), func(string) {}); err != nil {
		t.Fatal(err)
	}
	out, err := p.Complete(context.Background(), fim.Request{Language: "python", Path: "a.py", Prefix: "x = 1\n", Suffix: "\n", Extra: []fim.Snippet{{Path: "b.py", Text: "y = 2"}}})

	if err != nil || out != "print(x)" {
		t.Fatalf("complete = %q, %v", out, err)
	}
	if !strings.HasPrefix(gen["prompt"].(string), "# File: b.py\ny = 2\n# File: a.py\nx = 1\n") || gen["suffix"] != "\n" {
		t.Errorf("generate = %v", gen)
	}
	missing, _ := New(Config{Provider: "ollama", OllamaURL: srv.URL, OllamaModel: "starcoder2:3b"})
	if err := missing.Prepare(context.Background(), func(string) {}); err == nil || !strings.Contains(err.Error(), "ollama pull starcoder2:3b") {
		t.Errorf("a model Ollama lacks: %v", err)
	}
}

func TestDownloadEngine_FromTheRelease(t *testing.T) {
	// Arrange: the release's file, a zip holding llama-server.
	home := cache(t)
	server := "llama-server"
	if runtime.GOOS == "windows" {
		server += ".exe"
	}
	name := "llama-" + llamaBuild + "-bin-test.zip"
	zipPath := filepath.Join(t.TempDir(), name)
	f, _ := os.Create(zipPath)
	w := zip.NewWriter(f)
	e, _ := w.Create("build/bin/" + server)
	_, _ = e.Write([]byte("binary"))
	_ = w.Close()
	_ = f.Close()
	raw, _ := os.ReadFile(zipPath)
	sum := sha256.Sum256(raw)
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		_, _ = w.Write(raw)
	}))
	defer srv.Close()
	oldReleases, oldEngine := releases, engineFor
	releases = srv.URL + "/download/"
	engineFor = func(bool) (engine, error) {
		return engine{Name: name, Size: int64(len(raw)), SHA: hex.EncodeToString(sum[:])}, nil
	}
	defer func() { releases, engineFor = oldReleases, oldEngine }()
	p, _ := newLocal(Config{})

	// Act
	err := p.downloadEngine(context.Background(), func(string) {})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0] != "/download/"+name {
		t.Errorf("asked for %v: only the file, no API", asked)
	}
	if got := p.server(); got != filepath.Join(home, "llama.cpp", llamaBuild, "build", "bin", server) {
		t.Errorf("server = %s", got)
	}
	if ok, why := p.Ready(); ok || !strings.Contains(why, "not downloaded") {
		t.Errorf("without the model: %v %s", ok, why)
	}
}

func TestEngines_EverySystemHasOne(t *testing.T) {
	for _, m := range []map[string]engine{engines, gpuEngines} {
		for sys, e := range m {
			if e.Size <= 0 || len(e.SHA) != 64 {
				t.Errorf("%s: %+v", sys, e)
			}
		}
	}
}

func TestEngineDir_GPUBuildApart(t *testing.T) {
	// Arrange
	home := cache(t)

	// Act
	cpu, _ := newLocal(Config{})
	gpu, _ := newLocal(Config{GPU: true})

	// Assert
	if cpu.engineDir() != filepath.Join(home, "llama.cpp", llamaBuild) || gpu.engineDir() != filepath.Join(home, "llama.cpp", llamaBuild+"-gpu") {
		t.Errorf("cpu %s, gpu %s", cpu.engineDir(), gpu.engineDir())
	}
}

func TestUnused_ListsOtherModelsAndOldBuilds(t *testing.T) {
	// Arrange
	home := cache(t)
	for _, p := range []string{
		filepath.Join("models", "qwen2.5-coder-0.5b-q8_0.gguf"),
		filepath.Join("models", "qwen2.5-coder-3b-q8_0.gguf"),
		filepath.Join("llama.cpp", llamaBuild, "llama-server"),
		filepath.Join("llama.cpp", llamaBuild+"-gpu", "llama-server"),
		filepath.Join("llama.cpp", "b1000", "llama-server"),
	} {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(home, p)), 0o755)
		_ = os.WriteFile(filepath.Join(home, p), []byte("12345"), 0o644)
	}
	c := Config{Provider: "local", LocalModel: "0.5b"}

	// Act
	paths, size := Unused(c)
	err := RemoveUnused(c)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || size != 10 {
		t.Fatalf("unused = %v (%d bytes), want the 3b model and b1000", paths, size)
	}
	for _, p := range []string{filepath.Join("models", "qwen2.5-coder-0.5b-q8_0.gguf"), filepath.Join("llama.cpp", llamaBuild), filepath.Join("llama.cpp", llamaBuild+"-gpu")} {
		if _, err := os.Stat(filepath.Join(home, p)); err != nil {
			t.Errorf("%s should stay: %v", p, err)
		}
	}
}
