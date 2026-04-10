package export

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewLiveReloadHub(t *testing.T) {
	dir := t.TempDir()

	hub, err := NewLiveReloadHub(dir)
	if err != nil {
		t.Fatalf("NewLiveReloadHub() error = %v", err)
	}
	defer hub.Stop()

	if hub.bundlePath != dir {
		t.Errorf("bundlePath = %q, want %q", hub.bundlePath, dir)
	}

	if hub.debounce != 200*time.Millisecond {
		t.Errorf("debounce = %v, want %v", hub.debounce, 200*time.Millisecond)
	}
}

func TestLiveReloadHub_StartStop(t *testing.T) {
	dir := t.TempDir()

	hub, err := NewLiveReloadHub(dir)
	if err != nil {
		t.Fatalf("NewLiveReloadHub() error = %v", err)
	}

	if err := hub.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Should be able to stop without error
	hub.Stop()
}

func TestLiveReloadHub_ClientCount(t *testing.T) {
	dir := t.TempDir()

	hub, err := NewLiveReloadHub(dir)
	if err != nil {
		t.Fatalf("NewLiveReloadHub() error = %v", err)
	}

	if err := hub.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer hub.Stop()

	if count := hub.ClientCount(); count != 0 {
		t.Errorf("ClientCount() = %d, want 0", count)
	}
}

func TestLiveReloadHub_SSEHandler(t *testing.T) {
	dir := t.TempDir()

	hub, err := NewLiveReloadHub(dir)
	if err != nil {
		t.Fatalf("NewLiveReloadHub() error = %v", err)
	}

	if err := hub.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer hub.Stop()

	handler := hub.SSEHandler()

	// Create a test request
	req := httptest.NewRequest("GET", "/__preview__/events", nil)
	rr := httptest.NewRecorder()

	// Run handler in goroutine (it blocks)
	done := make(chan bool)
	go func() {
		handler(rr, req)
		done <- true
	}()

	// Give it time to send the initial connected event
	time.Sleep(100 * time.Millisecond)

	// Cancel the request context (simulates client disconnect)
	// In real test, we'd need to create a cancelable context
	hub.Stop() // This should cause the handler to exit

	select {
	case <-done:
		// Handler exited
	case <-time.After(time.Second):
		t.Error("Handler did not exit after stop")
	}

	// Check response headers
	contentType := rr.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", contentType, "text/event-stream")
	}

	// Check that connected event was sent
	body := rr.Body.String()
	if !strings.Contains(body, "event: connected") {
		t.Errorf("Response should contain 'event: connected', got: %s", body)
	}
}

func TestLiveReloadMiddleware_NonHTML(t *testing.T) {
	// Test that non-HTML files pass through unchanged
	handler := liveReloadMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("console.log('test');"))
	}))

	req := httptest.NewRequest("GET", "/app.js", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, "EventSource") {
		t.Errorf("Non-HTML response should not contain injected script")
	}
}

func TestLiveReloadMiddleware_HTML(t *testing.T) {
	// Test that HTML files get the script injected
	handler := liveReloadMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body><h1>Test</h1></body></html>"))
	}))

	req := httptest.NewRequest("GET", "/index.html", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Need to flush the injecting response writer
	if f, ok := rr.Result().Body.(interface{ Close() error }); ok {
		f.Close()
	}

	body := rr.Body.String()
	if !strings.Contains(body, "EventSource") {
		t.Errorf("HTML response should contain injected script, got: %s", body)
	}
	if !strings.Contains(body, "</body>") {
		t.Errorf("HTML response should still contain </body>, got: %s", body)
	}
}

func TestLiveReloadHub_FileChangeNotification(t *testing.T) {
	dir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(dir, "test.html")
	if err := os.WriteFile(testFile, []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	hub, err := NewLiveReloadHub(dir)
	if err != nil {
		t.Fatalf("NewLiveReloadHub() error = %v", err)
	}

	if err := hub.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer hub.Stop()

	// Register a client channel manually
	clientCh := make(chan struct{}, 1)
	hub.mu.Lock()
	hub.clients[clientCh] = struct{}{}
	hub.mu.Unlock()

	// Modify the file
	time.Sleep(300 * time.Millisecond) // Wait for debounce period to pass
	if err := os.WriteFile(testFile, []byte("<html><body>updated</body></html>"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Wait for notification
	select {
	case <-clientCh:
		// Got notification - success
	case <-time.After(2 * time.Second):
		t.Error("Did not receive notification within timeout")
	}
}

func TestFindLastIndex(t *testing.T) {
	tests := []struct {
		haystack string
		needle   string
		want     int
	}{
		{"hello world", "world", 6},
		{"hello world", "hello", 0},
		{"hello hello", "hello", 6},
		{"abc", "xyz", -1},
		{"", "test", -1},
		{"test", "", -1}, // Empty needle returns -1 due to loop condition
	}

	for _, tt := range tests {
		got := findLastIndex([]byte(tt.haystack), []byte(tt.needle))
		if got != tt.want {
			t.Errorf("findLastIndex(%q, %q) = %d, want %d", tt.haystack, tt.needle, got, tt.want)
		}
	}
}
