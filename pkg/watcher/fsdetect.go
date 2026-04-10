package watcher

import (
	"os"
	"path/filepath"
)

// FilesystemType is a best-effort classification of a filesystem for watcher reliability.
// The primary goal is to detect common remote/network filesystems where fsnotify may not
// reliably deliver change events and proactively switch to polling mode.
type FilesystemType int

const (
	FSTypeUnknown FilesystemType = iota
	FSTypeLocal
	FSTypeNFS
	FSTypeSMB
	FSTypeSSHFS
	FSTypeFUSE
)

func (t FilesystemType) String() string {
	switch t {
	case FSTypeLocal:
		return "local"
	case FSTypeNFS:
		return "nfs"
	case FSTypeSMB:
		return "smb"
	case FSTypeSSHFS:
		return "sshfs"
	case FSTypeFUSE:
		return "fuse"
	default:
		return "unknown"
	}
}

func isRemoteFilesystem(t FilesystemType) bool {
	switch t {
	case FSTypeNFS, FSTypeSMB, FSTypeSSHFS, FSTypeFUSE:
		return true
	default:
		return false
	}
}

var detectFilesystemTypeFunc = detectFilesystemType

// DetectFilesystemType best-effort detects the filesystem type for the given path.
// If the path is a file, the containing directory is used.
func DetectFilesystemType(path string) FilesystemType {
	if path == "" {
		return FSTypeUnknown
	}

	// Statfs on the containing directory is generally more robust for our purposes,
	// and also works when the target file doesn't exist yet.
	target := path
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			target = filepath.Dir(path)
		}
	} else {
		target = filepath.Dir(path)
		if target == "." || target == "" {
			target = path
		}
	}

	return detectFilesystemTypeFunc(target)
}
