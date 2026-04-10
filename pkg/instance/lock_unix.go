//go:build !windows

package instance

import (
	"os"
	"syscall"
)

// isProcessAlive checks if a process with the given PID is still running.
// On Unix systems, this uses signal 0 which checks process existence
// without actually sending a signal.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, sending signal 0 checks if process exists without actually signaling
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
