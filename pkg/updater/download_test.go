package updater

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadFile_SizeVerified_Success(t *testing.T) {
	body := []byte("hello")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "5")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	dest := filepath.Join(t.TempDir(), "file.bin")
	if err := downloadFile(srv.URL, dest, 5); err != nil {
		t.Fatalf("downloadFile: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("dest content=%q; want %q", string(got), "hello")
	}
}

func TestDownloadFile_SizeMismatch_Header(t *testing.T) {
	body := []byte("abcd")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	dest := filepath.Join(t.TempDir(), "file.bin")
	if err := downloadFile(srv.URL, dest, 5); err == nil {
		t.Fatalf("expected size mismatch error")
	}
}

func TestDownloadFile_SizeMismatch_WrittenBytes(t *testing.T) {
	// Force chunked transfer so ContentLength is not available to the client;
	// then rely on the post-download byte-count check.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		_, _ = w.Write([]byte("abcd"))
	}))
	t.Cleanup(srv.Close)

	dest := filepath.Join(t.TempDir(), "file.bin")
	if err := downloadFile(srv.URL, dest, 5); err == nil {
		t.Fatalf("expected downloaded size mismatch error")
	}
}
