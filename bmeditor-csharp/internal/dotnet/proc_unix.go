//go:build !windows

package dotnet

import (
	"os/exec"
	"syscall"
)

// group puts dotnet in a process group of its own, so Stop reaches the
// program dotnet run starts too.
func group(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func kill(cmd *exec.Cmd) { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
