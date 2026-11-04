package host

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"
)

func connect(t *testing.T, s *Server, onNotify func(string, json.RawMessage)) *Client {
	t.Helper()
	toServer, fromClient := io.Pipe()
	toClient, fromServer := io.Pipe()
	go func() {
		_ = s.Serve(t.Context(), toServer, fromServer)
		fromServer.Close()
	}()
	t.Cleanup(func() { fromClient.Close() })
	return NewClient(toClient, fromClient, onNotify)
}

func TestCall_AnswersWithTheProjectSettingFirst(t *testing.T) {
	// Arrange
	s := New()
	s.Handle("schema", func(ctx context.Context, _ json.RawMessage) (any, error) {
		return s.Context().Setting("schemaPath"), nil
	})
	c := connect(t, s, nil)
	init := Context{Settings: map[string]string{"schemaPath": "global.json"}, Project: Project{Settings: map[string]string{"schemaPath": "project.json"}}}
	if _, err := c.Call(t.Context(), "initialize", init); err != nil {
		t.Fatal(err)
	}

	// Act
	raw, err := c.Call(t.Context(), "schema", nil)

	// Assert
	var got string
	if err != nil || json.Unmarshal(raw, &got) != nil || got != "project.json" {
		t.Fatalf("got %s, %v", raw, err)
	}
}

func TestCall_CarriesTheErrorCodeAndPosition(t *testing.T) {
	// Arrange
	s := New()
	s.Handle("check", func(context.Context, json.RawMessage) (any, error) {
		return nil, &Error{Code: "UNKNOWN_COLUMN", Message: "no column nope", Line: 3, Col: 7}
	})
	c := connect(t, s, nil)

	// Act
	_, err := c.Call(t.Context(), "check", nil)

	// Assert
	var e *Error
	if !errors.As(err, &e) || e.Code != "UNKNOWN_COLUMN" || e.Line != 3 || e.Col != 7 {
		t.Fatalf("err = %#v", err)
	}
}

func TestCall_CancelsTheBackendsWork(t *testing.T) {
	// Arrange
	s := New()
	stopped := make(chan struct{})
	s.Handle("slow", func(ctx context.Context, _ json.RawMessage) (any, error) {
		<-ctx.Done()
		close(stopped)
		return nil, ctx.Err()
	})
	c := connect(t, s, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	// Act
	_, err := c.Call(ctx, "slow", nil)

	// Assert
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("the handler was not cancelled")
	}
}

func TestNotify_ReachesTheEditor(t *testing.T) {
	// Arrange
	got := make(chan string, 1)
	s := New()
	s.Handle("ping", func(context.Context, json.RawMessage) (any, error) {
		return nil, s.Notify("pong", map[string]int{"n": 1})
	})
	c := connect(t, s, func(method string, _ json.RawMessage) { got <- method })

	// Act
	_, err := c.Call(t.Context(), "ping", nil)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if m := <-got; m != "pong" {
		t.Fatalf("notified %q", m)
	}
}
