//go:build !windows

package main

import (
	"errors"
	"os/exec"
)

func command(string, []string) (*exec.Cmd, error) {
	return nil, errors.New("batch files run only on Windows, with cmd")
}

func kill(cmd *exec.Cmd) { _ = cmd.Process.Kill() }
