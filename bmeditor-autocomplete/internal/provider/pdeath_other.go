//go:build !linux && !windows

package provider

import "os/exec"

// parentDeath has no equivalent on macOS: the backend stops the server
// when it exits, and the server stops when idle.
func parentDeath(*exec.Cmd) {}
