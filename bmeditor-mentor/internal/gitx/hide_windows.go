package gitx

import (
	"os/exec"
	"syscall"
)

// hide keeps git from flashing a console window.
func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
