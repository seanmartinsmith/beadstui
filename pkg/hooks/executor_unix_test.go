//go:build !windows

package hooks

import "testing"

func TestGetShellCommand_Unix(t *testing.T) {
	shell, flag := getShellCommand()
	if shell != "sh" || flag != "-c" {
		t.Fatalf("getShellCommand() = (%q, %q); want (\"sh\", \"-c\")", shell, flag)
	}
}
