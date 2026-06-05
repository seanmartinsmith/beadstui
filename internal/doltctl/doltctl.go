// Package doltctl manages the Dolt server lifecycle for bt.
// It detects running servers, starts them via `bd dolt start`, and
// stops them on exit only if bt was the one that started them.
package doltctl

import (
	"context"
	"database/sql"
	"fmt"
	"io"
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

	_ "github.com/go-sql-driver/mysql"

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

	// Embedded is true when bt started its own `dolt sql-server` against a
	// beads embedded-mode data dir (no bd-managed server exists). Such a
	// server is owned by bt directly and stopped by killing the child
	// process rather than via `bd dolt stop`.
	Embedded bool
	// cmd / waitCh track the embedded child process for lifecycle control.
	cmd    *exec.Cmd
	waitCh chan error

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
	// 1. Resolve config via ReadDoltConfig (single source of truth)
	cfg, ok := datasource.ReadDoltConfig(beadsDir)
	if !ok {
		return nil, fmt.Errorf("no Dolt configuration found in %s", beadsDir)
	}

	// Embedded mode: bd runs Dolt in-process with no server, so `bd dolt
	// start` is unavailable. bt can only read over the MySQL protocol, so
	// it starts its own transient sql-server against the embedded data dir.
	if cfg.Mode == "embedded" {
		return ensureEmbeddedServer(beadsDir, cfg, lookPath)
	}

	// 0. Check bd is available (server mode delegates startup to bd)
	if _, err := lookPath("bd"); err != nil {
		return nil, fmt.Errorf("bd CLI not found - install beads first")
	}

	// 2. TCP dial to see if server is already running
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
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

// ensureEmbeddedServer starts a bt-owned `dolt sql-server` against a beads
// embedded-mode data directory. beads embedded mode runs Dolt in-process per
// command with no server; bt can only read over the MySQL protocol, so it
// hosts its own transient server for the lifetime of the bt process.
//
// The chosen port is exported via BEADS_DOLT_SERVER_PORT so the subsequent
// data-load (and any reconnect) resolves to this server. The same port is
// reused across reconnects to keep already-resolved DSNs valid.
//
// Note: while this server is running it holds the Dolt repository lock, so
// concurrent `bd` commands in the same project may fail until bt exits.
func ensureEmbeddedServer(beadsDir string, cfg datasource.DoltConfig, lookPath LookPathFunc) (*ServerState, error) {
	doltBin, err := lookPath("dolt")
	if err != nil {
		return nil, fmt.Errorf("dolt CLI not found - required to read beads embedded-mode data")
	}

	dataDir, err := filepath.Abs(cfg.EmbeddedDataDir)
	if err != nil {
		dataDir = cfg.EmbeddedDataDir
	}
	if fi, statErr := os.Stat(dataDir); statErr != nil || !fi.IsDir() {
		return nil, fmt.Errorf("embedded Dolt data dir not found: %s", dataDir)
	}

	// Reuse the port bt already exported (reconnect path) so previously
	// resolved DSNs stay valid; otherwise grab a free ephemeral port.
	port := cfg.Port
	if !cfg.PortFromEnv {
		port, err = freePort()
		if err != nil {
			return nil, fmt.Errorf("could not allocate port for embedded Dolt server: %w", err)
		}
	}

	fmt.Fprintln(os.Stderr, "Starting Dolt server for embedded beads data...")
	fmt.Fprintln(os.Stderr, "Note: while bt is open, concurrent `bd` commands in this project may fail (Dolt repository lock).")

	cmd := exec.Command(doltBin, "sql-server",
		"--data-dir", dataDir,
		"-H", "127.0.0.1",
		"-P", strconv.Itoa(port),
		"--loglevel=warning",
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	setDeathSignal(cmd) // kill the server if bt exits without graceful cleanup
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to launch dolt sql-server: %w", err)
	}

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	// Wait for SQL readiness (TCP can open before the DB is queryable).
	probe := datasource.DoltConfig{
		Host: "127.0.0.1", Port: port, Database: cfg.Database, User: cfg.User,
	}
	if err := waitForSQLReady(probe.DSN(), waitCh, 15*time.Second); err != nil {
		_ = cmd.Process.Kill()
		<-waitCh
		return nil, fmt.Errorf("embedded Dolt server did not become ready: %w", err)
	}

	// Export the port so the data-load and any reconnect resolve here.
	_ = os.Setenv("BEADS_DOLT_SERVER_PORT", strconv.Itoa(port))

	return &ServerState{
		Port:        port,
		StartedByBT: true,
		ServerPID:   cmd.Process.Pid,
		BeadsDir:    beadsDir,
		Embedded:    true,
		cmd:         cmd,
		waitCh:      waitCh,
	}, nil
}

// freePort asks the OS for an unused TCP port on the loopback interface.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// waitForSQLReady pings the DSN until it succeeds, the server process exits,
// or the deadline elapses.
func waitForSQLReady(dsn string, waitCh <-chan error, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-waitCh:
			return fmt.Errorf("server exited: %v", err)
		default:
		}
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			pingErr := db.Ping()
			_ = db.Close()
			if pingErr == nil {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timed out after %s", timeout)
}

// StopIfOwned stops the Dolt server only if bt started it and PID still matches.
// Returns true if the server was actually stopped, false if skipped.
func (s *ServerState) StopIfOwned() (bool, error) {
	if s == nil {
		return false, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.StartedByBT {
		return false, nil
	}

	// Embedded server: bt owns the child process directly. Terminate it
	// gracefully, then force-kill if it doesn't exit promptly, and reap it.
	if s.Embedded {
		if s.cmd == nil || s.cmd.Process == nil {
			return false, nil
		}
		_ = s.cmd.Process.Signal(os.Interrupt)
		select {
		case <-s.waitCh:
		case <-time.After(3 * time.Second):
			_ = s.cmd.Process.Kill()
			<-s.waitCh
		}
		return true, nil
	}

	// Check PID file - if gone or changed, someone else took over
	currentPID, err := readPIDFile(s.BeadsDir)
	if err != nil {
		// PID file gone - server already stopped or taken over
		return false, nil
	}
	if currentPID != s.ServerPID {
		// Different PID - another process restarted the server
		return false, nil
	}

	// PID matches - we own this server, stop it
	if s.stopFunc != nil {
		return true, s.stopFunc()
	}
	return true, runBdDoltStop()
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
	// Preserve embedded child handles so shutdown can kill the reconnected
	// process; otherwise the new server would be orphaned.
	s.Embedded = newState.Embedded
	s.cmd = newState.cmd
	s.waitCh = newState.waitCh
	if newState.BeadsDir != "" {
		s.BeadsDir = newState.BeadsDir
	}
}
