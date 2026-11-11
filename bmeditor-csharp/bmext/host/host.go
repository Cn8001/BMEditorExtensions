// Package host is the backend side of a bmeditor extension: a program the
// editor starts and talks to with JSON-RPC 2.0 over stdin and stdout, framed
// as the Language Server Protocol frames it (a Content-Length header, a
// blank line, the JSON). The extension's frontend calls its methods; the
// backend answers, and can send notifications the frontend listens to.
//
//	s := host.New()
//	s.Handle("compile", func(ctx context.Context, p json.RawMessage) (any, error) { ... })
//	host.Run(s)
//
// The editor sends "initialize" first and "context" whenever the open
// folder, the project or the extension's settings change; Context returns
// the latest. Requests run concurrently; one the editor gives up on has its
// ctx cancelled.
package host

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Context is what the editor tells the backend about where it works.
type Context struct {
	Folder   string            `json:"folder"`   // the open folder, empty when none
	Project  Project           `json:"project"`  // the open project, if any
	Settings map[string]string `json:"settings"` // the extension's global settings
}

type Project struct {
	Dir      string            `json:"dir"`
	Kind     string            `json:"kind"`
	Settings map[string]string `json:"settings"` // the extension's settings for this project
}

// Setting reads a setting as the project has it, or else as the editor's
// Settings have it.
func (c Context) Setting(key string) string {
	if v := c.Project.Settings[key]; v != "" {
		return v
	}
	return c.Settings[key]
}

// Error is an error the editor shows with its code, and with a caret at
// Line and Col (1-based) in the file the request was about, when given.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
	Col     int    `json:"col,omitempty"`
}

func (e *Error) Error() string { return e.Message }

// Handler answers one method. Its result is sent as JSON.
type Handler func(ctx context.Context, params json.RawMessage) (any, error)

type Server struct {
	handlers map[string]Handler

	mu      sync.Mutex
	context Context
	cancels map[string]context.CancelFunc
	out     io.Writer
	writeMu sync.Mutex
}

func New() *Server {
	return &Server{handlers: map[string]Handler{}, cancels: map[string]context.CancelFunc{}}
}

func (s *Server) Handle(method string, h Handler) { s.handlers[method] = h }

func (s *Server) Context() Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.context
}

// Notify sends a notification to the extension's frontend.
func (s *Server) Notify(method string, params any) error {
	return s.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

// Run serves on stdin and stdout until the editor closes them, and exits.
func Run(s *Server) {
	if err := s.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

type message struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Serve reads requests until r ends, the editor sends "exit", or ctx ends.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	s.out = w
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	defer wg.Wait()
	br := bufio.NewReader(r)
	for {
		body, err := readFrame(br)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var m message
		if err := json.Unmarshal(body, &m); err != nil {
			continue
		}
		switch m.Method {
		case "exit":
			return nil
		case "initialize", "context":
			var c Context
			_ = json.Unmarshal(m.Params, &c)
			s.mu.Lock()
			s.context = c
			s.mu.Unlock()
			if m.ID != nil {
				_ = s.reply(m.ID, map[string]any{}, nil)
			}
			continue
		case "shutdown":
			_ = s.reply(m.ID, nil, nil)
			continue
		case "$/cancelRequest":
			var p struct {
				ID json.RawMessage `json:"id"`
			}
			_ = json.Unmarshal(m.Params, &p)
			s.mu.Lock()
			if c := s.cancels[string(p.ID)]; c != nil {
				c()
			}
			s.mu.Unlock()
			continue
		}
		if m.ID == nil {
			continue // a notification nobody handles
		}
		h := s.handlers[m.Method]
		if h == nil {
			_ = s.replyError(m.ID, -32601, &Error{Code: "UNKNOWN_METHOD", Message: "no method " + m.Method})
			continue
		}
		rctx, rcancel := context.WithCancel(ctx)
		key := string(m.ID)
		s.mu.Lock()
		s.cancels[key] = rcancel
		s.mu.Unlock()
		wg.Go(func() {
			defer func() {
				s.mu.Lock()
				delete(s.cancels, key)
				s.mu.Unlock()
				rcancel()
			}()
			result, err := h(rctx, m.Params)
			_ = s.reply(m.ID, result, err)
		})
	}
}

func (s *Server) reply(id json.RawMessage, result any, err error) error {
	if err == nil {
		return s.write(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	var e *Error
	if !errors.As(err, &e) {
		code := "FAILED"
		if errors.Is(err, context.Canceled) {
			code = "CANCELLED"
		}
		e = &Error{Code: code, Message: err.Error()}
	}
	return s.replyError(id, -32000, e)
}

func (s *Server) replyError(id json.RawMessage, code int, e *Error) error {
	return s.write(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": e.Message, "data": e}})
}

func (s *Server) write(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = s.out.Write(body)
	return err
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if name, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("bad Content-Length %q: %w", value, err)
			}
			length = n
		}
	}
	if length < 0 {
		return nil, errors.New("frame without Content-Length")
	}
	body := make([]byte, length)
	_, err := io.ReadFull(r, body)
	return body, err
}
