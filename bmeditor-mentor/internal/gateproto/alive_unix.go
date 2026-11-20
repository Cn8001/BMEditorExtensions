//go:build !windows

package gateproto

import (
	"errors"
	"syscall"
)

// Alive says whether the process pid runs: a backend that wrote a registry
// and was killed leaves the file behind.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
