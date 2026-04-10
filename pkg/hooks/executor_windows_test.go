//go:build windows

package hooks

import "testing"

func TestGetShellCommand_Windows(t *testing.T) {
	shell, flag := getShellCommand()
	if shell != "cmd" || flag != "/C" {
		t.Fatalf("getShellCommand() = (%q, %q); want (\"cmd\", \"/C\")", shell, flag)
	}
}
