//go:build windows

package instance

import (
	"golang.org/x/sys/windows"
)

// isProcessAlive checks if a process with the given PID is still running.
// On Windows, this uses the OpenProcess API to check if we can open
// a handle to the process with query information access.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	// PROCESS_QUERY_LIMITED_INFORMATION is sufficient to check if process exists
	// and works even for processes running as a different user
	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000

	handle, err := windows.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	windows.CloseHandle(handle)
	return true
}
