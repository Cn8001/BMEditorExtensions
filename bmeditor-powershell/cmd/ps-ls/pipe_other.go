//go:build !windows

package main

import (
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// dialPipe connects to the Unix socket .NET makes for a named pipe.
func dialPipe(name string) (io.ReadWriteCloser, error) {
	if !strings.Contains(name, "/") {
		name = filepath.Join(os.TempDir(), "CoreFxPipe_"+name)
	}
	return net.Dial("unix", name)
}

func hide(*exec.Cmd) {}
