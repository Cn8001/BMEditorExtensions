//go:build !windows

package gitx

import "os/exec"

func hide(*exec.Cmd) {}
