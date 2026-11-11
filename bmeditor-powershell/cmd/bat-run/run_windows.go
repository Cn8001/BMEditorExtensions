package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// command runs a batch file as cmd would, with the console's code page
// set to UTF-8 so that what it prints reads right, in a console of its own
// that is not shown.
func command(program string, args []string) (*exec.Cmd, error) {
	quote := func(s string) string {
		if s == "" || strings.ContainsAny(s, " \t&()[]{}^=;!'+,`~") {
			return `"` + s + `"`
		}
		return s
	}
	line := `chcp 65001 >nul & ` + quote(program)
	for _, a := range args {
		line += " " + quote(a)
	}
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = `C:\Windows\System32\cmd.exe`
	}
	cmd := exec.Command(comspec)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       comspec + ` /d /s /c "` + line + `"`,
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	return cmd, nil
}

// kill ends cmd and everything it started.
func kill(cmd *exec.Cmd) {
	k := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	k.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if k.Run() != nil {
		_ = cmd.Process.Kill()
	}
}
