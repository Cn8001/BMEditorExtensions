//go:build !windows

package engine

import (
	"os/exec"
	"syscall"
)

// group puts the game in a process group of its own, so Stop reaches the
// window player it starts too.
func group(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func kill(cmd *exec.Cmd) { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
