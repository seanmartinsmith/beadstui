// Package toon provides Go bindings for TOON (Token-Optimized Object Notation).
//
// TOON is a format designed to minimize token usage when passing structured data
// to/from LLM coding agents while remaining human-readable.
//
// This library wraps the tru CLI binary. Ensure tru is installed and in PATH,
// or set TOON_TRU_BIN environment variable to specify the binary location.
//
// Basic usage:
//
//	// Encode Go data to TOON
//	data := map[string]any{"name": "Alice", "age": 30}
//	toon, err := toon.Encode(data)
//
//	// Decode TOON back to Go data
//	var result map[string]any
//	err := toon.Decode(toonString, &result)
//
//	// Check if tru is available
//	if toon.Available() {
//	    // TOON operations will work
//	}
package toon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Format represents detected data format
type Format string

const (
	FormatJSON    Format = "json"
	FormatTOON    Format = "toon"
	FormatUnknown Format = "unknown"
)

// Error codes following TOON error spec
const (
	ErrCodeEncodeFailed = 10 // Encoding failed
	ErrCodeDecodeFailed = 11 // Decoding failed
	ErrCodeTruNotFound  = 13 // tru binary not available
)

// ToonError represents an error from TOON operations
type ToonError struct {
	Code    int    // Error code
	Message string // Human-readable message
	Cause   error  // Underlying error, if any
}

func (e *ToonError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("toon error %d: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("toon error %d: %s", e.Code, e.Message)
}

func (e *ToonError) Unwrap() error {
	return e.Cause
}

// EncodeOptions configures TOON encoding behavior
type EncodeOptions struct {
	KeyFolding   string // "off" or "safe" - default "off"
	FlattenDepth int    // Max folded segment count when key folding is enabled
	Delimiter    string // Array delimiter: "," (default), "\t", "|"
	Indent       int    // Indentation spaces (default 2)
}

// DecodeOptions configures TOON decoding behavior
type DecodeOptions struct {
	ExpandPaths bool // Enable path expansion ("safe" mode)
	Strict      bool // Enable strict validation (default true)
}

// DefaultEncodeOptions returns the default encoding options
func DefaultEncodeOptions() EncodeOptions {
	return EncodeOptions{
		KeyFolding:   "off",
		FlattenDepth: 0,
		Delimiter:    ",",
		Indent:       2,
	}
}

// DefaultDecodeOptions returns the default decoding options
func DefaultDecodeOptions() DecodeOptions {
	return DecodeOptions{
		ExpandPaths: false,
		Strict:      true,
	}
}

// findTruBinary locates the tru binary
func findTruBinary() (string, error) {
	// Check environment variables first (accepts either a path OR a command name).
	for _, env := range []string{"TOON_TRU_BIN", "TOON_BIN"} {
		if raw := strings.TrimSpace(os.Getenv(env)); raw != "" {
			path, err := resolveTruCandidate(raw)
			if err == nil {
				return path, nil
			}
			return "", &ToonError{
				Code:    ErrCodeTruNotFound,
				Message: fmt.Sprintf("%s=%q does not appear to be toon_rust (expected tru)", env, raw),
				Cause:   err,
			}
		}
	}

	// Check PATH
	if path, err := exec.LookPath("tru"); err == nil {
		if isToonRustBinary(path) {
			return path, nil
		}
	}

	// Check common locations
	commonPaths := []string{
		"/usr/local/bin/tru",
		"/usr/bin/tru",
		"/data/tmp/cargo-target/release/tru",
		"/data/tmp/cargo-target/debug/tru",
	}
	for _, p := range commonPaths {
		if isToonRustBinary(p) {
			return p, nil
		}
	}

	return "", &ToonError{
		Code:    ErrCodeTruNotFound,
		Message: "tru binary not found. Install via: brew install dicklesworthstone/tap/tru OR https://github.com/Dicklesworthstone/toon_rust",
	}
}

func resolveTruCandidate(raw string) (string, error) {
	// If raw looks like a path, validate it directly.
	if strings.ContainsAny(raw, `/\`) || filepath.IsAbs(raw) || strings.HasPrefix(raw, ".") {
		if isToonRustBinary(raw) {
			return raw, nil
		}
		return "", fmt.Errorf("candidate is not a valid toon_rust tru binary: %q", raw)
	}

	// Otherwise treat it as a command name and resolve via PATH.
	path, err := exec.LookPath(raw)
	if err != nil {
		return "", err
	}
	if !isToonRustBinary(path) {
		return "", fmt.Errorf("candidate is not a valid toon_rust tru binary: %q (resolved to %q)", raw, path)
	}
	return path, nil
}

func isToonRustBinary(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	// Hard ban: Node.js `toon` CLI wrapper names.
	if base == "toon" || base == "toon.exe" {
		return false
	}

	helpOut, _ := exec.Command(path, "--help").CombinedOutput()
	helpLower := strings.ToLower(string(helpOut))
	if strings.Contains(helpLower, "reference implementation in rust") {
		return true
	}

	verOut, _ := exec.Command(path, "--version").CombinedOutput()
	verLower := strings.ToLower(strings.TrimSpace(string(verOut)))
	return strings.HasPrefix(verLower, "tru ") || strings.HasPrefix(verLower, "toon_rust ")
}

// Available returns true if the tru binary is installed and accessible
func Available() bool {
	_, err := findTruBinary()
	return err == nil
}

// TruPath returns the path to the tru binary, or an error if not found
func TruPath() (string, error) {
	return findTruBinary()
}

// Encode converts Go data to TOON format using default options
func Encode(data any) (string, error) {
	return EncodeWithOptions(data, DefaultEncodeOptions())
}

// EncodeWithOptions converts Go data to TOON format with custom options
func EncodeWithOptions(data any, opts EncodeOptions) (string, error) {
	truBin, err := findTruBinary()
	if err != nil {
		return "", err
	}

	// Marshal data to JSON first
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", &ToonError{
			Code:    ErrCodeEncodeFailed,
			Message: "failed to marshal data to JSON",
			Cause:   err,
		}
	}

	// Build command arguments
	args := []string{"--encode"}

	if opts.KeyFolding != "" && opts.KeyFolding != "off" {
		args = append(args, "--key-folding", opts.KeyFolding)
	}
	if opts.FlattenDepth > 0 {
		args = append(args, "--flatten-depth", fmt.Sprintf("%d", opts.FlattenDepth))
	}
	if opts.Delimiter != "" && opts.Delimiter != "," {
		args = append(args, "--delimiter", opts.Delimiter)
	}
	if opts.Indent != 2 && opts.Indent > 0 {
		args = append(args, "--indent", fmt.Sprintf("%d", opts.Indent))
	}

	// Execute tru
	cmd := exec.Command(truBin, args...)
	cmd.Stdin = bytes.NewReader(jsonBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", &ToonError{
			Code:    ErrCodeEncodeFailed,
			Message: "tru encode failed",
			Cause:   fmt.Errorf("%s: %w", strings.TrimSpace(stderr.String()), err),
		}
	}

	return stdout.String(), nil
}

// Decode parses TOON format and unmarshals into the provided Go value
func Decode(toonStr string, v any) error {
	return DecodeWithOptions(toonStr, DefaultDecodeOptions(), v)
}

// DecodeWithOptions parses TOON format with custom options and unmarshals into v
func DecodeWithOptions(toonStr string, opts DecodeOptions, v any) error {
	truBin, err := findTruBinary()
	if err != nil {
		return err
	}

	// Build command arguments
	args := []string{"--decode"}

	if opts.ExpandPaths {
		args = append(args, "--expand-paths", "safe")
	}
	if !opts.Strict {
		args = append(args, "--no-strict")
	}

	// Execute tru
	cmd := exec.Command(truBin, args...)
	cmd.Stdin = strings.NewReader(toonStr)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &ToonError{
			Code:    ErrCodeDecodeFailed,
			Message: "tru decode failed",
			Cause:   fmt.Errorf("%s: %w", strings.TrimSpace(stderr.String()), err),
		}
	}

	// Unmarshal JSON result into v
	if err := json.Unmarshal(stdout.Bytes(), v); err != nil {
		return &ToonError{
			Code:    ErrCodeDecodeFailed,
			Message: "failed to unmarshal decoded JSON",
			Cause:   err,
		}
	}

	return nil
}

// DecodeToValue parses TOON format and returns the result as any
func DecodeToValue(toonStr string) (any, error) {
	return DecodeToValueWithOptions(toonStr, DefaultDecodeOptions())
}

// DecodeToValueWithOptions parses TOON format with custom options
func DecodeToValueWithOptions(toonStr string, opts DecodeOptions) (any, error) {
	var result any
	if err := DecodeWithOptions(toonStr, opts, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DecodeToJSON parses TOON format and returns the JSON string
func DecodeToJSON(toonStr string) (string, error) {
	return DecodeToJSONWithOptions(toonStr, DefaultDecodeOptions())
}

// DecodeToJSONWithOptions parses TOON format with custom options and returns JSON
func DecodeToJSONWithOptions(toonStr string, opts DecodeOptions) (string, error) {
	truBin, err := findTruBinary()
	if err != nil {
		return "", err
	}

	// Build command arguments
	args := []string{"--decode"}

	if opts.ExpandPaths {
		args = append(args, "--expand-paths", "safe")
	}
	if !opts.Strict {
		args = append(args, "--no-strict")
	}

	// Execute tru
	cmd := exec.Command(truBin, args...)
	cmd.Stdin = strings.NewReader(toonStr)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", &ToonError{
			Code:    ErrCodeDecodeFailed,
			Message: "tru decode failed",
			Cause:   fmt.Errorf("%s: %w", strings.TrimSpace(stderr.String()), err),
		}
	}

	return stdout.String(), nil
}

// DetectFormat determines whether the input is JSON or TOON format
func DetectFormat(input string) Format {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return FormatUnknown
	}

	// Prefer JSON whenever the input is valid JSON (including scalars like "str", 123, true, null).
	var js json.RawMessage
	if json.Unmarshal([]byte(trimmed), &js) == nil {
		return FormatJSON
	}

	// TOON has key: value or array[N] patterns
	if strings.Contains(trimmed, ": ") || strings.Contains(trimmed, "[") && strings.Contains(trimmed, "]:") {
		return FormatTOON
	}

	return FormatTOON
}

// Convert automatically detects format and converts between JSON and TOON
// If input is JSON, returns TOON. If input is TOON, returns JSON.
func Convert(input string) (string, Format, error) {
	format := DetectFormat(input)

	switch format {
	case FormatJSON:
		var data any
		if err := json.Unmarshal([]byte(input), &data); err != nil {
			return "", format, &ToonError{
				Code:    ErrCodeEncodeFailed,
				Message: "failed to parse JSON input",
				Cause:   err,
			}
		}
		result, err := Encode(data)
		if err != nil {
			return "", format, err
		}
		return result, FormatJSON, nil

	case FormatTOON:
		result, err := DecodeToJSON(input)
		if err != nil {
			return "", format, err
		}
		return result, FormatTOON, nil

	default:
		return "", FormatUnknown, &ToonError{
			Code:    ErrCodeDecodeFailed,
			Message: "unable to detect input format",
		}
	}
}
