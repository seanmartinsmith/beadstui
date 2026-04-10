//go:build linux

package watcher

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	nfsSuperMagic  int64 = 0x6969
	cifsSuperMagic int64 = 0xFF534D42
	fuseSuperMagic int64 = 0x65735546
)

func detectFilesystemType(path string) FilesystemType {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return FSTypeUnknown
	}

	switch int64(stat.Type) {
	case nfsSuperMagic:
		return FSTypeNFS
	case cifsSuperMagic:
		return FSTypeSMB
	case fuseSuperMagic:
		if isLinuxSSHFS(path) {
			return FSTypeSSHFS
		}
		return FSTypeFUSE
	default:
		return FSTypeLocal
	}
}

func isLinuxSSHFS(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	contents, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		// Fall back to generic FUSE.
		return false
	}

	// Find the most specific mountpoint containing absPath and inspect fstype.
	bestMount := ""
	bestFSType := ""
	lines := bytes.Split(contents, []byte{'\n'})
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		parts := bytes.SplitN(line, []byte(" - "), 2)
		if len(parts) != 2 {
			continue
		}

		// mountinfo fields before " - ":
		// id parent major:minor root mount_point options optional_fields...
		fields := bytes.Fields(parts[0])
		if len(fields) < 5 {
			continue
		}

		mountPoint := unescapeMountField(string(fields[4]))
		if mountPoint == "" || mountPoint == "/" {
			// Root is fine but likely not the best match unless nothing else.
		}

		if !pathWithinMount(absPath, mountPoint) {
			continue
		}

		afterFields := bytes.Fields(parts[1])
		if len(afterFields) < 1 {
			continue
		}
		fsType := string(afterFields[0])
		if len(mountPoint) > len(bestMount) {
			bestMount = mountPoint
			bestFSType = fsType
		}
	}

	if bestFSType == "" {
		return false
	}

	// Common sshfs types: "fuse.sshfs" (mountinfo) and sometimes "sshfs".
	return strings.Contains(bestFSType, "sshfs")
}

func pathWithinMount(path string, mountPoint string) bool {
	if mountPoint == "" {
		return false
	}
	if mountPoint == "/" {
		return strings.HasPrefix(path, "/")
	}
	if path == mountPoint {
		return true
	}
	mountWithSep := mountPoint
	if !strings.HasSuffix(mountWithSep, string(os.PathSeparator)) {
		mountWithSep += string(os.PathSeparator)
	}
	return strings.HasPrefix(path, mountWithSep)
}

func unescapeMountField(s string) string {
	// /proc mount escapes: \040 (space), \011 (tab), \012 (newline), \134 (backslash)
	// We only implement the common escapes we might encounter in mountpoints.
	s = strings.ReplaceAll(s, `\040`, " ")
	s = strings.ReplaceAll(s, `\011`, "\t")
	s = strings.ReplaceAll(s, `\012`, "\n")
	s = strings.ReplaceAll(s, `\134`, `\`)
	return s
}
