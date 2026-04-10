package debug

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

func TestEnabled(t *testing.T) {
	// Save original state
	originalEnabled := enabled
	originalLogger := logger

	defer func() {
		enabled = originalEnabled
		logger = originalLogger
	}()

	// Test enabled
	SetEnabled(true)
	if !Enabled() {
		t.Error("Enabled() = false; want true")
	}

	// Test disabled
	SetEnabled(false)
	if Enabled() {
		t.Error("Enabled() = true; want false")
	}
}

func TestLog(t *testing.T) {
	// Save original state
	originalEnabled := enabled
	originalLogger := logger

	defer func() {
		enabled = originalEnabled
		logger = originalLogger
	}()

	// Capture output
	var buf bytes.Buffer
	enabled = true
	logger = log.New(&buf, "[TEST] ", 0)

	Log("test message %d", 42)

	output := buf.String()
	if !strings.Contains(output, "test message 42") {
		t.Errorf("Log output = %q; want to contain 'test message 42'", output)
	}
}

func TestLog_Disabled(t *testing.T) {
	// Save original state
	originalEnabled := enabled
	originalLogger := logger

	defer func() {
		enabled = originalEnabled
		logger = originalLogger
	}()

	// Capture output
	var buf bytes.Buffer
	enabled = false
	logger = log.New(&buf, "[TEST] ", 0)

	Log("should not appear")

	output := buf.String()
	if output != "" {
		t.Errorf("Log when disabled produced output: %q", output)
	}
}

func TestLogTiming(t *testing.T) {
	// Save original state
	originalEnabled := enabled
	originalLogger := logger

	defer func() {
		enabled = originalEnabled
		logger = originalLogger
	}()

	// Capture output
	var buf bytes.Buffer
	enabled = true
	logger = log.New(&buf, "[TEST] ", 0)

	LogTiming("operation", 123*time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, "operation took") {
		t.Errorf("LogTiming output = %q; want to contain 'operation took'", output)
	}
}

func TestLogIf(t *testing.T) {
	// Save original state
	originalEnabled := enabled
	originalLogger := logger

	defer func() {
		enabled = originalEnabled
		logger = originalLogger
	}()

	// Capture output
	var buf bytes.Buffer
	enabled = true
	logger = log.New(&buf, "[TEST] ", 0)

	LogIf(false, "should not appear")
	if buf.Len() > 0 {
		t.Errorf("LogIf(false) produced output: %q", buf.String())
	}

	LogIf(true, "should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Errorf("LogIf(true) output = %q; want to contain 'should appear'", buf.String())
	}
}

func TestLogEnterExit(t *testing.T) {
	// Save original state
	originalEnabled := enabled
	originalLogger := logger

	defer func() {
		enabled = originalEnabled
		logger = originalLogger
	}()

	// Capture output
	var buf bytes.Buffer
	enabled = true
	logger = log.New(&buf, "[TEST] ", 0)

	func() {
		defer LogEnterExit("testFunc")()
		time.Sleep(1 * time.Millisecond)
	}()

	output := buf.String()
	if !strings.Contains(output, "-> testFunc") {
		t.Errorf("LogEnterExit output = %q; want to contain '-> testFunc'", output)
	}
	if !strings.Contains(output, "<- testFunc") {
		t.Errorf("LogEnterExit output = %q; want to contain '<- testFunc'", output)
	}
}

func TestDump(t *testing.T) {
	// Save original state
	originalEnabled := enabled
	originalLogger := logger

	defer func() {
		enabled = originalEnabled
		logger = originalLogger
	}()

	// Capture output
	var buf bytes.Buffer
	enabled = true
	logger = log.New(&buf, "[TEST] ", 0)

	type testStruct struct {
		A int
		B string
	}
	Dump("myVar", testStruct{A: 1, B: "hello"})

	output := buf.String()
	if !strings.Contains(output, "myVar") {
		t.Errorf("Dump output = %q; want to contain 'myVar'", output)
	}
	if !strings.Contains(output, "testStruct") {
		t.Errorf("Dump output = %q; want to contain 'testStruct'", output)
	}
}

func TestSection(t *testing.T) {
	// Save original state
	originalEnabled := enabled
	originalLogger := logger

	defer func() {
		enabled = originalEnabled
		logger = originalLogger
	}()

	// Capture output
	var buf bytes.Buffer
	enabled = true
	logger = log.New(&buf, "[TEST] ", 0)

	Section("Test Section")

	output := buf.String()
	if !strings.Contains(output, "=== Test Section ===") {
		t.Errorf("Section output = %q; want to contain '=== Test Section ==='", output)
	}
}

func TestCheckpoint(t *testing.T) {
	// Save original state
	originalEnabled := enabled
	originalLogger := logger
	originalCounter := checkpointCounter

	defer func() {
		enabled = originalEnabled
		logger = originalLogger
		checkpointCounter = originalCounter
	}()

	// Capture output
	var buf bytes.Buffer
	enabled = true
	logger = log.New(&buf, "[TEST] ", 0)
	checkpointCounter = 0

	Checkpoint("first")
	Checkpoint("second")

	output := buf.String()
	if !strings.Contains(output, "[1] first") {
		t.Errorf("Checkpoint output = %q; want to contain '[1] first'", output)
	}
	if !strings.Contains(output, "[2] second") {
		t.Errorf("Checkpoint output = %q; want to contain '[2] second'", output)
	}
}

func TestResetCheckpoints(t *testing.T) {
	checkpointCounter = 10
	ResetCheckpoints()
	if checkpointCounter != 0 {
		t.Errorf("checkpointCounter after reset = %d; want 0", checkpointCounter)
	}
}

func TestAssert_Pass(t *testing.T) {
	// Save original state
	originalEnabled := enabled

	defer func() {
		enabled = originalEnabled
	}()

	enabled = true

	// Should not panic
	Assert(true, "this should pass")
}

func TestAssert_Fail(t *testing.T) {
	// Save original state
	originalEnabled := enabled
	originalLogger := logger

	defer func() {
		enabled = originalEnabled
		logger = originalLogger
	}()

	// Capture output
	var buf bytes.Buffer
	enabled = true
	logger = log.New(&buf, "[TEST] ", 0)

	// Should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Assert(false) did not panic")
		}
	}()

	Assert(false, "this should fail")
}

func TestAssertNoError_Pass(t *testing.T) {
	// Save original state
	originalEnabled := enabled

	defer func() {
		enabled = originalEnabled
	}()

	enabled = true

	// Should not panic
	AssertNoError(nil, "context")
}

func TestLogFunc(t *testing.T) {
	// Save original state
	originalEnabled := enabled
	originalLogger := logger

	defer func() {
		enabled = originalEnabled
		logger = originalLogger
	}()

	// Capture output
	var buf bytes.Buffer
	enabled = true
	logger = log.New(&buf, "[TEST] ", 0)

	LogFunc("done")()

	output := buf.String()
	if !strings.Contains(output, "done") {
		t.Errorf("LogFunc output = %q; want to contain 'done'", output)
	}
}

func TestLogFunc_Disabled(t *testing.T) {
	// Save original state
	originalEnabled := enabled

	defer func() {
		enabled = originalEnabled
	}()

	enabled = false

	// Should return a no-op function
	fn := LogFunc("should not appear")
	fn() // Should not panic
}
