//go:build windows

package pkgmgr

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// hide stops a console window flashing up for each git or build command
// when the package manager runs inside a GUI program, which on Windows
// gives every console child a window of its own.
func hide(cmd *exec.Cmd) *exec.Cmd {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
	return cmd
}
