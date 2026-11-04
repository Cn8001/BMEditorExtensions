//go:build !windows

package pkgmgr

import "os/exec"

func hide(cmd *exec.Cmd) *exec.Cmd { return cmd }
