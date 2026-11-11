// Package provider is the local model that completes code: one llama.cpp
// runs here (a code model it downloads, or a .gguf of one's own), or one
// Ollama serves. Nothing leaves the machine.
package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"bmeditor-autocomplete/internal/fim"
)

// Provider completes code at a cursor.
type Provider interface {
	// Ready says whether it can complete now, and if not why: a model not
	// downloaded, Ollama not running.
	Ready() (bool, string)
	// Prepare gets it ready: downloads what it needs, starts its server.
	// progress gets each step as a line.
	Prepare(ctx context.Context, progress func(string)) error
	// Complete returns the raw answer; the caller cleans it (fim.Clean).
	Complete(ctx context.Context, r fim.Request) (string, error)
	// Close stops what it started.
	Close()
}

// Config is the extension's settings that choose and set up a provider.
type Config struct {
	Provider    string // off, local, ollama
	LocalModel  string // 0.5b, 1.5b, 3b, custom, or a .gguf file or URL of one's own
	ModelFile   string // the .gguf file or URL when LocalModel is custom
	Threads     int    // llama.cpp's threads; 0 for its own choice
	GPU         bool   // llama.cpp's Vulkan build, with the model on the GPU
	OllamaURL   string
	OllamaModel string
}

// Names are the providers, in the order the chooser lists them.
var Names = []string{"local", "ollama", "off"}

// ErrOff is returned when completion is turned off.
var ErrOff = errors.New("inline completion is off: set Settings › Autocomplete › Model server to local or ollama")

// New makes the provider the config names.
func New(c Config) (Provider, error) {
	switch c.Provider {
	case "", "local":
		return newLocal(c)
	case "ollama":
		return newOllama(c), nil
	case "off":
		return nil, ErrOff
	}
	return nil, fmt.Errorf("unknown provider %q; one of %s", c.Provider, strings.Join(Names, ", "))
}

// Key is what makes two configs the same provider: a change of any of it
// makes a new one.
func (c Config) Key() string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%t\x00%s\x00%s", c.Provider, c.LocalModel, c.ModelFile, c.Threads, c.GPU, c.OllamaURL, c.OllamaModel)
}
