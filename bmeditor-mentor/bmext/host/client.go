package host

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// ErrClosed is a call to a backend that has exited.
var ErrClosed = errors.New("extension backend is not running")

// Client is the editor's side: it calls a backend's methods and hears its
// notifications.
type Client struct {
	w        io.Writer
	onNotify func(method string, params json.RawMessage)

	mu      sync.Mutex
	next    int
	pending map[int]chan reply
	writeMu sync.Mutex
	done    chan struct{}
	err     error
}

type reply struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
		Data    *Error `json:"data"`
	} `json:"error"`
}

func NewClient(r io.Reader, w io.Writer, onNotify func(method string, params json.RawMessage)) *Client {
	c := &Client{w: w, onNotify: onNotify, pending: map[int]chan reply{}, done: make(chan struct{})}
	go c.read(bufio.NewReader(r))
	return c
}

// Done is closed when the backend's output ends.
func (c *Client) Done() <-chan struct{} { return c.done }

func (c *Client) read(r *bufio.Reader) {
	defer func() {
		c.mu.Lock()
		for id, ch := range c.pending {
			close(ch)
			delete(c.pending, id)
		}
		c.mu.Unlock()
		close(c.done)
	}()
	for {
		body, err := readFrame(r)
		if err != nil {
			c.err = err
			return
		}
		var m struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			reply
		}
		if json.Unmarshal(body, &m) != nil {
			continue
		}
		if m.Method != "" {
			if m.ID == nil && c.onNotify != nil {
				c.onNotify(m.Method, m.Params)
			}
			continue
		}
		if m.ID == nil {
			continue
		}
		c.mu.Lock()
		ch := c.pending[*m.ID]
		delete(c.pending, *m.ID)
		c.mu.Unlock()
		if ch != nil {
			ch <- m.reply
		}
	}
}

// Call sends a request and waits for its answer. When ctx ends first, the
// backend is told to cancel it.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	ch := make(chan reply, 1)
	c.mu.Lock()
	select {
	case <-c.done:
		c.mu.Unlock()
		return nil, ErrClosed
	default:
	}
	c.next++
	id := c.next
	c.pending[id] = ch
	c.mu.Unlock()
	if err := c.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrClosed, err)
	}
	select {
	case r, ok := <-ch:
		if !ok {
			return nil, ErrClosed
		}
		if r.Error != nil {
			if r.Error.Data != nil && r.Error.Data.Code != "" {
				return nil, r.Error.Data
			}
			return nil, &Error{Code: "FAILED", Message: r.Error.Message}
		}
		return r.Result, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		_ = c.Notify("$/cancelRequest", map[string]any{"id": id})
		return nil, ctx.Err()
	}
}

func (c *Client) Notify(method string, params any) error {
	return c.send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (c *Client) send(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.w.Write(body)
	return err
}
