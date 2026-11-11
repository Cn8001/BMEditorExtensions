// Command autocomplete-host is the Autocomplete extension's backend: it
// keeps the local model the settings choose (llama.cpp's, downloaded on
// request, or Ollama's), gets it ready, and asks it for the text at the
// cursor.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"

	"bmeditor-autocomplete/bmext/host"
	"bmeditor-autocomplete/internal/fim"
	"bmeditor-autocomplete/internal/provider"
)

type backend struct {
	s *host.Server

	mu        sync.Mutex
	key       string
	p         provider.Provider
	preparing context.CancelFunc
}

func (b *backend) config() provider.Config {
	c := b.s.Context()
	threads, _ := strconv.Atoi(c.Setting("threads"))
	return provider.Config{
		Provider:    c.Setting("provider"),
		LocalModel:  c.Setting("localModel"),
		ModelFile:   c.Setting("modelFile"),
		Threads:     threads,
		GPU:         c.Setting("gpu") != "false",
		OllamaURL:   c.Setting("ollamaUrl"),
		OllamaModel: c.Setting("ollamaModel"),
	}
}

// current is the provider the settings name, made again when they change.
func (b *backend) current() (provider.Provider, error) {
	c := b.config()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.p != nil && b.key == c.Key() {
		return b.p, nil
	}
	if b.p != nil {
		b.p.Close()
		b.p = nil
	}
	p, err := provider.New(c)
	if err != nil {
		return nil, err
	}
	b.p, b.key = p, c.Key()
	return p, nil
}

type status struct {
	Provider string           `json:"provider"`
	Ready    bool             `json:"ready"`
	Why      string           `json:"why,omitempty"`
	Busy     bool             `json:"busy"` // getting ready
	Models   []provider.Model `json:"models"`
}

func (b *backend) status() status {
	c := b.config()
	st := status{Provider: c.Provider, Models: provider.Here()}
	if st.Provider == "" {
		st.Provider = "local"
	}
	b.mu.Lock()
	st.Busy = b.preparing != nil
	b.mu.Unlock()
	p, err := b.current()
	if err != nil {
		st.Why = err.Error()
		return st
	}
	st.Ready, st.Why = p.Ready()
	return st
}

// prepare gets the provider ready in the background (a download can take
// minutes): each step goes to the frontend as autocomplete/progress, the
// end as autocomplete/status.
func (b *backend) prepare() error {
	p, err := b.current()
	if err != nil {
		return err
	}
	b.mu.Lock()
	if b.preparing != nil {
		b.mu.Unlock()
		return &host.Error{Code: "busy", Message: "the model is being got ready already"}
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.preparing = cancel
	b.mu.Unlock()
	go func() {
		err := p.Prepare(ctx, func(line string) { _ = b.s.Notify("autocomplete/progress", map[string]string{"line": line}) })
		b.mu.Lock()
		b.preparing = nil
		b.mu.Unlock()
		cancel()
		st := b.status()
		if err != nil {
			st.Why = err.Error()
			_ = b.s.Notify("autocomplete/progress", map[string]string{"line": err.Error(), "stream": "stderr"})
		}
		_ = b.s.Notify("autocomplete/status", st)
	}()
	return nil
}

func (b *backend) cancelPrepare() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.preparing != nil {
		b.preparing()
	}
}

func (b *backend) complete(ctx context.Context, r fim.Request) (string, error) {
	p, err := b.current()
	if err != nil {
		return "", &host.Error{Code: "off", Message: err.Error()}
	}
	if ok, why := p.Ready(); !ok {
		return "", &host.Error{Code: "not-ready", Message: why}
	}
	b.mu.Lock()
	busy := b.preparing != nil
	b.mu.Unlock()
	if busy {
		return "", &host.Error{Code: "not-ready", Message: "the model is being got ready"}
	}
	r = fim.Window(r)
	out, err := p.Complete(ctx, r)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return "", err
		}
		return "", &host.Error{Code: "failed", Message: err.Error()}
	}
	return fim.Clean(out, r), nil
}

func main() {
	s := host.New()
	b := &backend{s: s}
	s.Handle("status", func(context.Context, json.RawMessage) (any, error) { return b.status(), nil })
	s.Handle("prepare", func(context.Context, json.RawMessage) (any, error) { return nil, b.prepare() })
	s.Handle("cancel", func(context.Context, json.RawMessage) (any, error) {
		b.cancelPrepare()
		return nil, nil
	})
	s.Handle("complete", func(ctx context.Context, params json.RawMessage) (any, error) {
		var r fim.Request
		if err := json.Unmarshal(params, &r); err != nil {
			return nil, err
		}
		text, err := b.complete(ctx, r)
		if err != nil {
			return nil, err
		}
		return map[string]string{"text": text}, nil
	})
	// remove stops the model and deletes what was downloaded.
	s.Handle("remove", func(context.Context, json.RawMessage) (any, error) {
		b.cancelPrepare()
		b.mu.Lock()
		if b.p != nil {
			b.p.Close()
			b.p, b.key = nil, ""
		}
		b.mu.Unlock()
		return nil, provider.RemoveDownloads()
	})
	err := s.Serve(context.Background(), os.Stdin, os.Stdout)
	// The editor closed us: llama-server goes too.
	b.mu.Lock()
	if b.p != nil {
		b.p.Close()
	}
	b.mu.Unlock()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
