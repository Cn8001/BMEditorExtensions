//go:build windows

package gateproto

import "syscall"

// Alive says whether the process pid runs: a backend that wrote a registry
// and was killed leaves the file behind.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	const queryLimited = 0x1000 // PROCESS_QUERY_LIMITED_INFORMATION
	h, err := syscall.OpenProcess(queryLimited, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == 259 // STILL_ACTIVE
}
