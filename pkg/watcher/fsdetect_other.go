//go:build !linux && !darwin && !windows

package watcher

func detectFilesystemType(path string) FilesystemType {
	return FSTypeUnknown
}
