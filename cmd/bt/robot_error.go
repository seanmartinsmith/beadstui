package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// RobotError is a structured, machine-parseable error for robot-mode
// refusals (bt-s5zgk.3, per the bt-j9r2o decision). It travels inside the
// normal Go error chain - Unwrap returns the wrapped cause - so existing
// %w wrapping and errors.Is/errors.As checks keep working; only main()
// special-cases it, at the point where an error is rendered for stderr.
//
// This intentionally stays narrow: one concrete case (the --as-of robot
// refusal, cobra_robot.go's checkAsOfRobotSupport) plus the shared shape
// the next case needs, not a general error-code registry. Adding another
// structured refusal means constructing another *RobotError at the call
// site that detects the condition - no central dispatch to extend.
type RobotError struct {
	// Code is a stable, machine-matchable identifier (SCREAMING_SNAKE_CASE)
	// agents can switch on instead of pattern-matching prose.
	Code string `json:"code"`
	// Message is the same human-readable text the plain-error path would
	// have produced - kept as a field, not dropped, so agents (and humans)
	// reading the JSON still get the full explanation.
	Message string `json:"message"`
	// SupportedAlternative names the concrete action available today in
	// place of the refused request (a flag, a different mode, or "none"
	// when no alternative currently exists).
	SupportedAlternative string `json:"supported_alternative,omitempty"`
	// Doc points at further context: a bead ID or doc anchor tracking the
	// refusal or its eventual resolution.
	Doc string `json:"doc,omitempty"`

	cause error
}

// Error satisfies the error interface with the plain-text message, so any
// existing errors.Is/strings.Contains(err.Error(), ...) check keeps working
// unchanged for callers that never look for the structured form.
func (e *RobotError) Error() string { return e.Message }

// Unwrap exposes the wrapped cause for errors.Is/errors.As traversal.
func (e *RobotError) Unwrap() error { return e.cause }

// newRobotError builds a RobotError wrapping cause, deriving Message from
// cause.Error() so the human-readable text stays a single source of truth
// rather than being duplicated at each call site.
func newRobotError(code string, cause error, supportedAlternative, doc string) *RobotError {
	return &RobotError{
		Code:                 code,
		Message:              cause.Error(),
		SupportedAlternative: supportedAlternative,
		Doc:                  doc,
		cause:                cause,
	}
}

// renderCLIError formats err for the single stderr line main() prints.
// The robot I/O contract (bt-ah53) requires every non-empty stderr line to
// start with "Error:"; that invariant holds for both branches here.
//
// Errors carrying a *RobotError render as "Error: " followed by a single-
// line compact JSON envelope (code, message, supported_alternative, doc)
// so agents can strip the fixed prefix and json.Unmarshal the remainder
// instead of pattern-matching prose. Everything else keeps today's plain
// "Error: <msg>" text unchanged - this is additive, not a wire break.
func renderCLIError(err error) string {
	var re *RobotError
	if errors.As(err, &re) {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		// Refusal text routinely contains angle-bracket placeholders like
		// "<ref>"; json.Marshal's default HTML-escaping of angle brackets
		// is a browser-embedding concern that doesn't apply to a CLI's
		// stderr and only makes the envelope harder to read.
		enc.SetEscapeHTML(false)
		if marshalErr := enc.Encode(re); marshalErr == nil {
			// Encode appends a trailing newline; Fprintln in main() would
			// add a second one, so trim it here.
			return "Error: " + strings.TrimSuffix(buf.String(), "\n")
		}
		// Fall through to the plain-text form if marshaling somehow fails -
		// stderr must never be empty for a real error.
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "Error:") {
		msg = "Error: " + msg
	}
	return msg
}
