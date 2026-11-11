package main

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// dialPipe opens a named pipe for overlapped I/O, so that a read waiting
// for the adapter does not hold up a write to it.
func dialPipe(name string) (io.ReadWriteCloser, error) {
	if !strings.HasPrefix(name, `\\`) {
		name = `\\.\pipe\` + name
	}
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	return os.NewFile(uintptr(h), name), nil
}

// hide keeps PowerShell from opening a console window.
func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
