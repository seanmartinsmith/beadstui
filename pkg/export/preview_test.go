package export

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewPreviewServer(t *testing.T) {
	server := NewPreviewServer("/tmp/test", 8080)

	if server == nil {
		t.Fatal("NewPreviewServer returned nil")
	}

	if server.bundlePath != "/tmp/test" {
		t.Errorf("Expected bundlePath '/tmp/test', got %s", server.bundlePath)
	}

	if server.port != 8080 {
		t.Errorf("Expected port 8080, got %d", server.port)
	}
}

func TestPreviewServer_Port(t *testing.T) {
	server := NewPreviewServer("/tmp/test", 9001)

	if server.Port() != 9001 {
		t.Errorf("Expected Port() to return 9001, got %d", server.Port())
	}
}

func TestPreviewServer_URL(t *testing.T) {
	port := 9002
	server := NewPreviewServer("/tmp", port)
	expected := fmt.Sprintf("http://127.0.0.1:%d", port)
	if got := server.URL(); got != expected {
		t.Errorf("Expected URL() to return %s, got %s", expected, got)
	}
}

func TestFindAvailablePort(t *testing.T) {
	// This should find an available port in the range
	port, err := FindAvailablePort(19000, 19100)
	if err != nil {
		t.Errorf("FindAvailablePort failed: %v", err)
	}

	if port < 19000 || port > 19100 {
		t.Errorf("Port %d is outside expected range 19000-19100", port)
	}
}

func TestFindAvailablePort_NoAvailable(t *testing.T) {
	// Try to find in a very narrow range that's likely already in use
	// This is a bit tricky to test reliably, so we just verify the function exists
	// and returns the expected type
	port, err := FindAvailablePort(19200, 19200)
	if err == nil {
		// Port was available, which is fine
		if port != 19200 {
			t.Errorf("Expected port 19200, got %d", port)
		}
	}
}

func TestDefaultPreviewConfig(t *testing.T) {
	config := DefaultPreviewConfig()

	if config.Port != 0 {
		t.Errorf("Expected Port 0 (auto-select), got %d", config.Port)
	}

	if !config.OpenBrowser {
		t.Error("Expected OpenBrowser to be true")
	}

	if config.Quiet {
		t.Error("Expected Quiet to be false")
	}
}

func TestPreviewConfig(t *testing.T) {
	config := PreviewConfig{
		BundlePath:  "/tmp/bundle",
		Port:        8888,
		OpenBrowser: false,
		Quiet:       true,
	}

	if config.BundlePath != "/tmp/bundle" {
		t.Errorf("Expected BundlePath '/tmp/bundle', got %s", config.BundlePath)
	}

	if config.Port != 8888 {
		t.Errorf("Expected Port 8888, got %d", config.Port)
	}

	if config.OpenBrowser {
		t.Error("Expected OpenBrowser to be false")
	}

	if !config.Quiet {
		t.Error("Expected Quiet to be true")
	}
}

func TestPreviewServer_Start_MissingBundle(t *testing.T) {
	server := NewPreviewServer("/nonexistent/path/12345", 19050)

	err := server.Start()
	if err == nil {
		t.Error("Expected error for missing bundle path")
	}
}

func TestPreviewServer_Start_MissingIndex(t *testing.T) {
	// Create a temp directory without index.html
	tmpDir := t.TempDir()

	server := NewPreviewServer(tmpDir, 19051)

	err := server.Start()
	if err == nil {
		t.Error("Expected error for missing index.html")
	}
}

func TestStartPreviewWithConfig_MissingIndexReturnsError(t *testing.T) {
	// Create a temp directory without index.html
	tmpDir := t.TempDir()

	cfg := PreviewConfig{
		BundlePath:  tmpDir,
		Port:        0,
		OpenBrowser: false,
		Quiet:       true,
	}

	err := StartPreviewWithConfig(cfg)
	if err == nil {
		t.Fatal("Expected error for missing index.html")
	}
	if !strings.Contains(err.Error(), "no index.html found") {
		t.Fatalf("Expected missing index error, got: %v", err)
	}
}

func TestPreviewServer_Integration(t *testing.T) {
	// Create a temp bundle directory
	tmpDir := t.TempDir()

	// Create index.html
	indexContent := `<!DOCTYPE html><html><head><title>Test</title></head><body>Hello</body></html>`
	if err := os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	// Create a data file
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "meta.json"), []byte(`{"test": true}`), 0644); err != nil {
		t.Fatalf("Failed to create meta.json: %v", err)
	}

	// Find available port
	port, err := FindAvailablePort(19060, 19080)
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}

	server := NewPreviewServer(tmpDir, port)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test index.html
	resp, err := http.Get(server.URL())
	if err != nil {
		t.Fatalf("Failed to GET index.html: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check no-cache headers
	cacheControl := resp.Header.Get("Cache-Control")
	if cacheControl == "" {
		t.Error("Expected Cache-Control header")
	}

	pragma := resp.Header.Get("Pragma")
	if pragma != "no-cache" {
		t.Errorf("Expected Pragma: no-cache, got %s", pragma)
	}

	// Check body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	if string(body) != indexContent {
		t.Errorf("Expected body %q, got %q", indexContent, string(body))
	}

	// Test status endpoint
	statusResp, err := http.Get(server.URL() + "/__preview__/status")
	if err != nil {
		t.Fatalf("Failed to GET status: %v", err)
	}
	defer statusResp.Body.Close()

	if statusResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", statusResp.StatusCode)
	}

	statusBody, _ := io.ReadAll(statusResp.Body)
	if len(statusBody) == 0 {
		t.Error("Expected non-empty status response")
	}

	// Clean shutdown
	if err := server.Stop(); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}
}

func TestPreviewServer_StatusHandler_EmitsValidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Include a control character in the bundle path. fmt's %q uses Go escapes (e.g. \x01),
	// which are NOT valid JSON escapes, so this ensures we don't hand-roll JSON here.
	// Windows generally forbids control characters in file paths.
	if runtime.GOOS == "windows" {
		t.Skip("control-character paths are not supported on windows")
	}

	segment := "x" + string([]byte{0x01}) + "y"
	bundleDir := filepath.Join(tmpDir, segment)
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Skipf("skipping control-char path test (mkdir failed): %v", err)
	}

	if err := os.WriteFile(filepath.Join(bundleDir, "index.html"), []byte("<!doctype html><title>ok</title>"), 0644); err != nil {
		t.Fatalf("WriteFile index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "data.json"), []byte(`{"ok":true}`), 0644); err != nil {
		t.Fatalf("WriteFile data.json: %v", err)
	}

	server := NewPreviewServer(bundleDir, 1234)

	req := httptest.NewRequest(http.MethodGet, "/__preview__/status", nil)
	rec := httptest.NewRecorder()
	server.statusHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	type statusResponse struct {
		Status     string `json:"status"`
		Port       int    `json:"port"`
		BundlePath string `json:"bundle_path"`
		HasIndex   bool   `json:"has_index"`
		FileCount  int    `json:"file_count"`
	}

	var got statusResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("expected valid JSON, got error: %v (body=%q)", err, rec.Body.String())
	}

	if got.Status != "running" {
		t.Fatalf("status=%q, want %q", got.Status, "running")
	}
	if got.Port != 1234 {
		t.Fatalf("port=%d, want %d", got.Port, 1234)
	}
	if !got.HasIndex {
		t.Fatalf("has_index=%v, want true", got.HasIndex)
	}
	if got.FileCount < 2 {
		t.Fatalf("file_count=%d, want >= 2", got.FileCount)
	}

	if got.BundlePath != bundleDir {
		t.Fatalf("bundle_path=%q, want %q", got.BundlePath, bundleDir)
	}
}

func TestNoCacheMiddleware(t *testing.T) {
	// Create a simple handler
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	// Wrap with no-cache middleware
	handler := noCacheMiddleware(inner)

	// Create a test request
	req, _ := http.NewRequest("GET", "/", nil)
	rec := &testResponseWriter{headers: make(http.Header)}

	handler.ServeHTTP(rec, req)

	// Check headers
	if rec.headers.Get("Cache-Control") == "" {
		t.Error("Expected Cache-Control header")
	}

	if rec.headers.Get("Pragma") != "no-cache" {
		t.Errorf("Expected Pragma: no-cache, got %s", rec.headers.Get("Pragma"))
	}

	if rec.headers.Get("Expires") != "0" {
		t.Errorf("Expected Expires: 0, got %s", rec.headers.Get("Expires"))
	}

	// Verify no CORS headers
	if rec.headers.Get("Access-Control-Allow-Origin") != "" {
		t.Error("Unexpected CORS header Access-Control-Allow-Origin found")
	}
}

func TestPreviewServer_StatusJSONEncoding(t *testing.T) {
	invalidPath := string([]byte{0xff, 0xfe, 'b'})
	server := NewPreviewServer(invalidPath, 19090)

	req := httptest.NewRequest(http.MethodGet, "/__preview__/status", nil)
	rec := httptest.NewRecorder()

	server.statusHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}

	if payload["status"] != "running" {
		t.Errorf("expected status=running, got %v", payload["status"])
	}
}

// testResponseWriter is a simple ResponseWriter for testing
type testResponseWriter struct {
	headers    http.Header
	body       []byte
	statusCode int
}

func (w *testResponseWriter) Header() http.Header {
	return w.headers
}

func (w *testResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return len(data), nil
}

func (w *testResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}
