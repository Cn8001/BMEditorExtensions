//go:build !windows

package dotnet

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRunner_StreamsLinesAndRunsOneAtATime(t *testing.T) {
	// Arrange
	var r Runner
	var mu sync.Mutex
	var got []Line
	done := make(chan error, 1)
	line := func(l Line) { mu.Lock(); got = append(got, l); mu.Unlock() }

	// Act
	err := r.Start("sh", t.TempDir(), "run", []string{"-c", "echo out; echo err >&2; sleep 5"}, line, func(err error) { done <- err })
	second := r.Start("sh", t.TempDir(), "build", []string{"-c", "true"}, line, func(error) {})
	time.Sleep(200 * time.Millisecond)
	r.Stop()
	end := <-done

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(second, ErrRunning) {
		t.Errorf("second Start = %v, want ErrRunning", second)
	}
	if end == nil {
		t.Error("a stopped run ended without an error")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 {
		t.Errorf("lines = %+v, want out and err", got)
	}
	if r.Running() != "" {
		t.Errorf("Running = %q after the end", r.Running())
	}
}
