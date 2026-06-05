//go:build linux

package doltctl

import (
	"os/exec"
	"syscall"
)

// setDeathSignal ties the child's lifetime to bt: when the bt process exits
// (cleanly, via os.Exit, or on a crash), the kernel sends SIGTERM to the
// embedded dolt sql-server so it can't be orphaned and hold the Dolt lock.
// This backstops the graceful StopIfOwned path used by the TUI and covers
// the robot/one-shot commands that exit via os.Exit without running it.
func setDeathSignal(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGTERM
}
