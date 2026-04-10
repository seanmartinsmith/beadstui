//go:build darwin

package watcher

import (
	"bytes"

	"golang.org/x/sys/unix"
)

func detectFilesystemType(path string) FilesystemType {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return FSTypeUnknown
	}

	// On macOS, Statfs_t exposes the filesystem name directly.
	fsType := string(bytes.TrimRight(stat.Fstypename[:], "\x00"))
	switch fsType {
	case "nfs":
		return FSTypeNFS
	case "smbfs", "cifs":
		return FSTypeSMB
	case "osxfuse", "macfuse", "fusefs":
		return FSTypeFUSE
	default:
		return FSTypeLocal
	}
}
