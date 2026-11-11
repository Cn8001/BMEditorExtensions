//go:build !windows

package provider

import "os/exec"

func hide(cmd *exec.Cmd) { parentDeath(cmd) }

func bind(*exec.Cmd) {}
