// Package doltctl manages the Dolt server lifecycle for bt.
// It detects running servers, starts them via `bd dolt start`, and
// stops them on exit only if bt was the one that started them.
package doltctl

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
)

// ServerState tracks whether bt started the Dolt server and owns its lifecycle.
// Guarded by mu since the poll loop goroutine and main goroutine both access it.
type ServerState struct {
	mu          sync.Mutex
	Port        int
	StartedByBT bool
	ServerPID   int
	BeadsDir    string

	// stopFunc is injectable for testing. When nil, the real bd dolt stop is used.
	stopFunc func() error
}

// LookPathFunc is the signature for exec.LookPath, injectable for testing.
type LookPathFunc func(name string) (string, error)

// bdStartOutputRe matches "Dolt server started (PID XXXXX, port YYYYY)"
var bdStartOutputRe = regexp.MustCompile(`Dolt server started \(PID (\d+), port (\d+)\)`)

// parseBdDoltStartOutput extracts PID and port from bd dolt start output.
func parseBdDoltStartOutput(output string) (pid int, port int, err error) {
	matches := bdStartOutputRe.FindStringSubmatch(output)
	if matches == nil {
		return 0, 0, fmt.Errorf("cannot parse bd dolt start output: %q", output)
	}
	pid, _ = strconv.Atoi(matches[1])
	port, _ = strconv.Atoi(matches[2])
	return pid, port, nil
}

// readPIDFile reads the Dolt server PID from .beads/dolt-server.pid.
func readPIDFile(beadsDir string) (int, error) {
	data, err := os.ReadFile(filepath.Join(beadsDir, "dolt-server.pid"))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// readPortFile reads the Dolt server port from .beads/dolt-server.port.
func readPortFile(beadsDir string) (int, error) {
	data, err := os.ReadFile(filepath.Join(beadsDir, "dolt-server.port"))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// EnsureServer detects or starts a Dolt server.
// lookPath is injected for testing (pass exec.LookPath in production).
func EnsureServer(beadsDir string, lookPath LookPathFunc) (*ServerState, error) {
	// 0. Check bd is available
	if _, err := lookPath("bd"); err != nil {
		return nil, fmt.Errorf("bd CLI not found - install beads first")
	}

	// 1. Resolve port via ReadDoltConfig (single source of truth)
	cfg, ok := datasource.ReadDoltConfig(beadsDir)
	if !ok {
		return nil, fmt.Errorf("no Dolt configuration found in %s", beadsDir)
	}

	// 2. TCP dial to see if server is already running
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err == nil {
		conn.Close()
		// Server is running - attach without owning it
		return &ServerState{
			Port:        cfg.Port,
			StartedByBT: false,
			BeadsDir:    beadsDir,
		}, nil
	}

	// 3. Server not running - start it
	fmt.Fprintln(os.Stderr, "Starting Dolt server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bd", "dolt", "start")
	cmd.Dir = filepath.Dir(beadsDir) // run in project root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("bd dolt start failed: %w\nOutput: %s", err, string(out))
	}

	// 4. Parse output for PID and port
	pid, port, parseErr := parseBdDoltStartOutput(string(out))
	if parseErr != nil {
		// Fallback: read from files
		log.Printf("WARN: could not parse bd dolt start output, falling back to files: %v", parseErr)
		filePID, pidErr := readPIDFile(beadsDir)
		filePort, portErr := readPortFile(beadsDir)
		if pidErr != nil || portErr != nil {
			return nil, fmt.Errorf("bd dolt start succeeded but cannot determine PID/port: parse=%v pid=%v port=%v", parseErr, pidErr, portErr)
		}
		pid = filePID
		port = filePort
	}

	// 5. Wait for server to be ready (retry TCP dial up to 10s)
	readyAddr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", readyAddr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return &ServerState{
				Port:        port,
				StartedByBT: true,
				ServerPID:   pid,
				BeadsDir:    beadsDir,
			}, nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("bd dolt start succeeded (PID %d, port %d) but server not reachable after 10s", pid, port)
}

// StopIfOwned stops the Dolt server only if bt started it and PID still matches.
func (s *ServerState) StopIfOwned() error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.StartedByBT {
		return nil
	}

	// Check PID file - if gone or changed, someone else took over
	currentPID, err := readPIDFile(s.BeadsDir)
	if err != nil {
		// PID file gone - server already stopped or taken over
		return nil
	}
	if currentPID != s.ServerPID {
		// Different PID - another process restarted the server
		return nil
	}

	// PID matches - we own this server, stop it
	if s.stopFunc != nil {
		return s.stopFunc()
	}
	return runBdDoltStop()
}

// runBdDoltStop calls `bd dolt stop` with a 5-second timeout.
func runBdDoltStop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bd", "dolt", "stop")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd dolt stop failed: %w\nOutput: %s", err, string(out))
	}
	return nil
}

// UpdateAfterReconnect updates ServerState after a successful auto-reconnect.
// Called from the poll loop when EnsureServer creates a new server after failure.
func (s *ServerState) UpdateAfterReconnect(newState *ServerState) {
	if s == nil || newState == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Port = newState.Port
	s.StartedByBT = newState.StartedByBT
	s.ServerPID = newState.ServerPID
}
