//go:build windows

package watcher

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func detectFilesystemType(path string) FilesystemType {
	p := filepath.Clean(path)
	vol := filepath.VolumeName(p)
	if vol == "" {
		return FSTypeUnknown
	}

	root := vol
	if strings.HasSuffix(vol, ":") {
		root = vol + `\`
	} else if !strings.HasSuffix(vol, `\`) {
		root = vol + `\`
	}

	ptr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return FSTypeUnknown
	}

	switch windows.GetDriveType(ptr) {
	case windows.DRIVE_REMOTE:
		return FSTypeSMB
	case windows.DRIVE_UNKNOWN, windows.DRIVE_NO_ROOT_DIR:
		return FSTypeUnknown
	default:
		return FSTypeLocal
	}
}
