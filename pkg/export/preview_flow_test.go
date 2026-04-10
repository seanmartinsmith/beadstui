package export

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewServer_StartWithGracefulShutdown_ReturnsStartError(t *testing.T) {
	server := NewPreviewServer("/path/does/not/exist", 9010)
	if err := server.StartWithGracefulShutdown(); err == nil {
		t.Fatal("Expected StartWithGracefulShutdown to return error for missing bundle path")
	}
}

func TestStartPreview_ReturnsBundleError(t *testing.T) {
	if err := StartPreview("/path/does/not/exist"); err == nil {
		t.Fatal("Expected StartPreview to return error for missing bundle path")
	}
}

func TestStartPreviewWithConfig_PortInUseReturnsError(t *testing.T) {
	bundleDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(bundleDir, "index.html"), []byte("<!doctype html><title>ok</title>"), 0644); err != nil {
		t.Fatalf("WriteFile index.html: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	port := listener.Addr().(*net.TCPAddr).Port
	cfg := PreviewConfig{
		BundlePath:  bundleDir,
		Port:        port,
		OpenBrowser: false,
		Quiet:       true,
	}

	if err := StartPreviewWithConfig(cfg); err == nil {
		t.Fatal("Expected StartPreviewWithConfig to return error when port is already in use")
	}
}
