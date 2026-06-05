//go:build !linux

package doltctl

import "os/exec"

// setDeathSignal is a no-op on platforms without a parent-death signal
// (macOS, Windows). On those systems the embedded dolt sql-server is stopped
// via StopIfOwned on the TUI shutdown path; one-shot/robot commands rely on
// the OS reclaiming the short-lived child when bt exits.
func setDeathSignal(cmd *exec.Cmd) {}
