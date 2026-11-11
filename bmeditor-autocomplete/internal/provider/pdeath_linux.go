package provider

import (
	"os/exec"
	"syscall"
)

// parentDeath ends llama-server when the backend ends, however it ends.
func parentDeath(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
