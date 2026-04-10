// Package debug provides conditional debug logging for bv.
//
// Debug logging is enabled by setting the BV_DEBUG environment variable:
//
//	BV_DEBUG=1 bv --robot-triage
//
// When enabled, debug messages are written to stderr with timestamps.
// When disabled (default), all debug functions are no-ops with zero overhead.
//
// Usage:
//
//	import "github.com/Dicklesworthstone/beads_viewer/pkg/debug"
//
//	func myFunc() {
//	    debug.Log("processing %d items", count)
//	    // ...
//	    debug.LogTiming("myFunc", elapsed)
//	}
package debug

import (
	"fmt"
	"log"
	"os"
	"time"
)

var (
	// enabled is true when BV_DEBUG env var is set
	enabled bool
	// logger writes to stderr with [BV_DEBUG] prefix
	logger *log.Logger
)

func init() {
	if os.Getenv("BV_DEBUG") != "" {
		enabled = true
		logger = log.New(os.Stderr, "[BV_DEBUG] ", log.Ltime|log.Lmicroseconds)
	}
}

// Enabled returns whether debug logging is enabled.
func Enabled() bool {
	return enabled
}

// SetEnabled allows programmatic control of debug logging.
// Note: This also requires initializing the logger if not already done.
func SetEnabled(e bool) {
	enabled = e
	if e && logger == nil {
		logger = log.New(os.Stderr, "[BV_DEBUG] ", log.Ltime|log.Lmicroseconds)
	}
}

// Log writes a debug message if debug logging is enabled.
// Uses printf-style formatting.
func Log(format string, args ...any) {
	if !enabled {
		return
	}
	logger.Printf(format, args...)
}

// LogTiming writes a timing message if debug logging is enabled.
func LogTiming(name string, d time.Duration) {
	if !enabled {
		return
	}
	logger.Printf("%s took %v", name, d)
}

// LogIf writes a debug message only if the condition is true.
func LogIf(cond bool, format string, args ...any) {
	if !enabled || !cond {
		return
	}
	logger.Printf(format, args...)
}

// LogFunc returns a function that logs a debug message when called.
// Useful for deferred logging:
//
//	defer debug.LogFunc("myFunc done")()
func LogFunc(msg string) func() {
	if !enabled {
		return func() {}
	}
	return func() {
		logger.Print(msg)
	}
}

// LogEnterExit logs function entry and exit with timing.
// Usage:
//
//	func myFunc() {
//	    defer debug.LogEnterExit("myFunc")()
//	    // ...
//	}
func LogEnterExit(name string) func() {
	if !enabled {
		return func() {}
	}
	logger.Printf("-> %s", name)
	start := time.Now()
	return func() {
		logger.Printf("<- %s (%v)", name, time.Since(start))
	}
}

// Trace is an alias for LogEnterExit for convenience.
var Trace = LogEnterExit

// Dump logs a value with its type for debugging complex structures.
func Dump(name string, v any) {
	if !enabled {
		return
	}
	logger.Printf("%s: %T = %+v", name, v, v)
}

// Section logs a section header for visual organization in debug output.
func Section(name string) {
	if !enabled {
		return
	}
	logger.Printf("=== %s ===", name)
}

// Checkpoint logs a numbered checkpoint for tracking progress.
var checkpointCounter int

func Checkpoint(msg string) {
	if !enabled {
		return
	}
	checkpointCounter++
	logger.Printf("[%d] %s", checkpointCounter, msg)
}

// ResetCheckpoints resets the checkpoint counter.
func ResetCheckpoints() {
	checkpointCounter = 0
}

// Assert logs a message and panics if the condition is false.
// Only active when debug is enabled.
func Assert(cond bool, msg string) {
	if !enabled {
		return
	}
	if !cond {
		logger.Printf("ASSERTION FAILED: %s", msg)
		panic(fmt.Sprintf("debug assertion failed: %s", msg))
	}
}

// AssertNoError logs and panics if err is not nil.
// Only active when debug is enabled.
func AssertNoError(err error, context string) {
	if !enabled {
		return
	}
	if err != nil {
		logger.Printf("ASSERTION FAILED: %s: %v", context, err)
		panic(fmt.Sprintf("debug assertion failed: %s: %v", context, err))
	}
}
