// Package export provides data export functionality for bv.
//
// This file implements a local preview server for static site bundles.
// It serves files with no-cache headers and auto-opens the browser.
package export

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// PreviewServer serves a static site bundle locally for previewing.
type PreviewServer struct {
	bundlePath    string
	port          int
	server        *http.Server
	liveReloadHub *LiveReloadHub
}

// NewPreviewServer creates a new preview server for the given bundle.
func NewPreviewServer(bundlePath string, port int) *PreviewServer {
	return &PreviewServer{
		bundlePath: bundlePath,
		port:       port,
	}
}

// Start starts the preview server and blocks until stopped.
func (p *PreviewServer) Start() error {
	// Verify bundle path exists
	if _, err := os.Stat(p.bundlePath); os.IsNotExist(err) {
		return fmt.Errorf("bundle path does not exist: %s", p.bundlePath)
	}

	// Check for index.html
	indexPath := filepath.Join(p.bundlePath, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return fmt.Errorf("no index.html found in bundle: %s", p.bundlePath)
	}

	mux := http.NewServeMux()

	// Static file server with no-cache middleware
	fs := http.FileServer(http.Dir(p.bundlePath))
	mux.Handle("/", noCacheMiddleware(fs))

	// Status endpoint
	mux.HandleFunc("/__preview__/status", p.statusHandler)

	p.server = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", p.port),
		Handler: mux,
	}

	// Open browser after short delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		url := fmt.Sprintf("http://127.0.0.1:%d", p.port)
		if err := OpenInBrowser(url); err != nil {
			fmt.Printf("Could not open browser: %v\n", err)
			fmt.Printf("Open %s in your browser\n", url)
		}
	}()

	fmt.Printf("\nPreview server running at http://127.0.0.1:%d\n", p.port)
	fmt.Printf("Serving: %s\n", p.bundlePath)
	fmt.Println("\nPress Ctrl+C to stop")

	return p.server.ListenAndServe()
}

// StartWithGracefulShutdown starts the server with signal handling for clean shutdown.
func (p *PreviewServer) StartWithGracefulShutdown() error {
	// Channel to receive OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	// Channel to receive server errors
	errChan := make(chan error, 1)

	// Start server in goroutine
	go func() {
		if err := p.Start(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for either signal or error
	select {
	case <-stop:
		fmt.Println("\nShutting down preview server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return p.server.Shutdown(ctx)
	case err := <-errChan:
		return err
	}
}

// Stop gracefully stops the preview server.
func (p *PreviewServer) Stop() error {
	if p.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.server.Shutdown(ctx)
}

// Port returns the port the server is running on.
func (p *PreviewServer) Port() int {
	return p.port
}

// URL returns the full URL of the preview server.
func (p *PreviewServer) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", p.port)
}

// statusHandler returns the preview server status as JSON.
func (p *PreviewServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	// Check if bundle is valid
	indexPath := filepath.Join(p.bundlePath, "index.html")
	hasIndex := true
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		hasIndex = false
	}

	// Get bundle info
	var fileCount int
	filepath.Walk(p.bundlePath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fileCount++
		}
		return nil
	})

	type statusResponse struct {
		Status     string `json:"status"`
		Port       int    `json:"port"`
		BundlePath string `json:"bundle_path"`
		HasIndex   bool   `json:"has_index"`
		FileCount  int    `json:"file_count"`
	}

	resp := statusResponse{
		Status:     "running",
		Port:       p.port,
		BundlePath: strings.ToValidUTF8(p.bundlePath, "?"),
		HasIndex:   hasIndex,
		FileCount:  fileCount,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, fmt.Sprintf("encode preview status: %v", err), http.StatusInternalServerError)
		return
	}
}

// noCacheMiddleware adds headers to prevent browser caching.
func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set no-cache headers
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		next.ServeHTTP(w, r)
	})
}

// FindAvailablePort finds an available port in the given range.
func FindAvailablePort(start, end int) (int, error) {
	for port := start; port <= end; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port in range %d-%d", start, end)
}

// DefaultPreviewPort is the default port for the preview server.
const DefaultPreviewPort = 9000

// PreviewPortRange defines the range of ports to try if default is unavailable.
const PreviewPortRangeStart = 9000
const PreviewPortRangeEnd = 9100

// StartPreview is a convenience function to start a preview server with auto port selection.
func StartPreview(bundlePath string) error {
	port, err := FindAvailablePort(PreviewPortRangeStart, PreviewPortRangeEnd)
	if err != nil {
		return fmt.Errorf("could not find available port: %w", err)
	}

	server := NewPreviewServer(bundlePath, port)
	return server.StartWithGracefulShutdown()
}

// PreviewConfig configures the preview server.
type PreviewConfig struct {
	// BundlePath is the path to the static site bundle
	BundlePath string

	// Port is the port to serve on (0 for auto-select)
	Port int

	// OpenBrowser determines whether to auto-open a browser
	OpenBrowser bool

	// Quiet suppresses status messages
	Quiet bool

	// LiveReload enables live-reload via SSE when bundle files change
	LiveReload bool
}

// DefaultPreviewConfig returns sensible defaults for preview configuration.
func DefaultPreviewConfig() PreviewConfig {
	return PreviewConfig{
		Port:        0, // Auto-select
		OpenBrowser: true,
		Quiet:       false,
		LiveReload:  true, // Enable by default for immediate feedback
	}
}

// StartPreviewWithConfig starts a preview server with the given configuration.
func StartPreviewWithConfig(config PreviewConfig) error {
	// Verify bundle exists
	if config.BundlePath == "" {
		return fmt.Errorf("bundle path is required")
	}
	if _, err := os.Stat(config.BundlePath); os.IsNotExist(err) {
		return fmt.Errorf("bundle path does not exist: %s", config.BundlePath)
	}

	// Check for index.html (match PreviewServer.Start behavior)
	indexPath := filepath.Join(config.BundlePath, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return fmt.Errorf("no index.html found in bundle: %s", config.BundlePath)
	}

	// Auto-select port if needed
	port := config.Port
	if port == 0 {
		var err error
		port, err = FindAvailablePort(PreviewPortRangeStart, PreviewPortRangeEnd)
		if err != nil {
			return fmt.Errorf("could not find available port: %w", err)
		}
	}

	// Create server
	server := NewPreviewServer(config.BundlePath, port)

	// Need to initialize the server first
	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir(config.BundlePath))

	// Set up live-reload if enabled
	var liveReloadHub *LiveReloadHub
	if config.LiveReload {
		var err error
		liveReloadHub, err = NewLiveReloadHub(config.BundlePath)
		if err != nil {
			if !config.Quiet {
				fmt.Printf("Warning: Could not enable live-reload: %v\n", err)
			}
		} else {
			if err := liveReloadHub.Start(); err != nil {
				if !config.Quiet {
					fmt.Printf("Warning: Could not start live-reload: %v\n", err)
				}
				liveReloadHub = nil
			} else {
				// Add SSE endpoint for live-reload
				mux.HandleFunc("/__preview__/events", liveReloadHub.SSEHandler())
				// Wrap file server with live-reload script injection
				mux.Handle("/", liveReloadMiddleware(noCacheMiddleware(fs)))
			}
		}
	}

	// If no live-reload, just use no-cache middleware
	if liveReloadHub == nil {
		mux.Handle("/", noCacheMiddleware(fs))
	}

	mux.HandleFunc("/__preview__/status", server.statusHandler)
	server.liveReloadHub = liveReloadHub

	server.server = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	// Handle opening browser
	if config.OpenBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond)
			url := server.URL()
			if err := OpenInBrowser(url); err != nil {
				if !config.Quiet {
					fmt.Printf("Could not open browser: %v\n", err)
					fmt.Printf("Open %s in your browser\n", url)
				}
			}
		}()
	}

	// Print status message
	if !config.Quiet {
		fmt.Printf("\nPreview server running at http://127.0.0.1:%d\n", port)
		fmt.Printf("Serving: %s\n", config.BundlePath)
		if liveReloadHub != nil {
			fmt.Println("Live-reload: enabled (browser will refresh on file changes)")
		}
		fmt.Println("\nPress Ctrl+C to stop")
	}

	// Channel to receive OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	// Channel to receive server errors
	errChan := make(chan error, 1)

	// Start server in goroutine
	go func() {
		if err := server.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for either signal or error
	select {
	case <-stop:
		if !config.Quiet {
			fmt.Println("\nShutting down preview server...")
		}
		// Stop live-reload hub if running
		if liveReloadHub != nil {
			liveReloadHub.Stop()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.server.Shutdown(ctx)
	case err := <-errChan:
		// Stop live-reload hub on error
		if liveReloadHub != nil {
			liveReloadHub.Stop()
		}
		return err
	}
}
