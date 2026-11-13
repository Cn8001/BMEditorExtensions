package engine

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLocate_TakesTheProgramOrItsFolder(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	bin := filepath.Join(dir, exe("ascdii"))
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Act
	fromFolder, err1 := Locate(dir)
	fromFile, err2 := Locate(bin)
	_, missing := Locate(filepath.Join(dir, "nope"))

	// Assert
	if err1 != nil || err2 != nil || fromFolder != bin || fromFile != bin {
		t.Fatalf("got %q %v, %q %v", fromFolder, err1, fromFile, err2)
	}
	if !errors.Is(missing, ErrNotFound) {
		t.Errorf("a missing program: %v", missing)
	}
}

func TestRunner_StopEndsWhatTheGameStarted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	// Arrange: a game that starts a child, as ascdii play starts the window player.
	var r Runner
	lines := make(chan Line, 8)
	done := make(chan error, 1)
	err := r.Start("/bin/sh", t.TempDir(), "game", []string{"-c", "echo ready; sleep 60 & wait"}, func(l Line) { lines <- l }, func(err error) { done <- err })
	if err != nil {
		t.Fatal(err)
	}
	if l := <-lines; l.Text != "ready" || l.Stream != "stdout" {
		t.Fatalf("first line %+v", l)
	}
	again := r.Start("/bin/sh", t.TempDir(), "other", []string{"-c", "true"}, func(Line) {}, func(error) {})

	// Act
	r.Stop()

	// Assert
	if !errors.Is(again, ErrRunning) {
		t.Errorf("a second game started: %v", again)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the game still runs: its child kept it open")
	}
	if r.Running() != "" {
		t.Errorf("running %q after it ended", r.Running())
	}
}
