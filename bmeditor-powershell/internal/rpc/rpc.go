// Package rpc reads and writes the frames the Language Server Protocol and
// the Debug Adapter Protocol share: a Content-Length header, a blank line,
// then the JSON.
package rpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// Read reads one frame's JSON.
func Read(r *bufio.Reader) ([]byte, error) {
	n := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if n < 0 {
				continue // stray blank line between frames
			}
			break
		}
		if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "Content-Length") {
			if n, err = strconv.Atoi(strings.TrimSpace(v)); err != nil {
				return nil, fmt.Errorf("bad Content-Length %q", v)
			}
		}
	}
	body := make([]byte, n)
	_, err := io.ReadFull(r, body)
	return body, err
}

// Writer writes whole frames, one at a time from any goroutine.
type Writer struct {
	mu sync.Mutex
	w  io.Writer
}

func NewWriter(w io.Writer) *Writer { return &Writer{w: w} }

// Raw writes JSON as it is.
func (w *Writer) Raw(body []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := fmt.Fprintf(w.w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err := w.w.Write(body)
	return err
}

// JSON writes v as JSON.
func (w *Writer) JSON(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return w.Raw(body)
}

// Message is a JSON-RPC message (LSP), with the fields a server looks at.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
