package engine

import (
	"os/exec"
	"strconv"
	"syscall"
)

const createNoWindow = 0x08000000

// group keeps a console window from opening for the game's programs.
func group(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// kill ends the game's whole tree: the window player ascdii play starts
// outlives a killed parent on Windows.
func kill(cmd *exec.Cmd) {
	tk := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	tk.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	if tk.Run() != nil {
		_ = cmd.Process.Kill()
	}
}
