package gateproto

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFind_PassesOverABackendThatHasGone: a registry left by a backend that
// was killed is not answered; the folder above's, or none, is.
func TestFind_PassesOverABackendThatHasGone(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "proj")
	sub := filepath.Join(root, "sub")
	_ = Write(dir, Registry{Port: 1, Root: dir, PID: os.Getpid()})
	_ = Write(dir, Registry{Port: 2, Root: sub, PID: 1 << 30}) // no such process

	r, err := Find(dir, sub)
	if err != nil || r.Port != 1 {
		t.Fatalf("find = %+v, %v: the gone one is passed over", r, err)
	}
	if !Alive(os.Getpid()) || Alive(1<<30) || Alive(0) {
		t.Error("Alive is wrong about this process or a missing one")
	}
}
