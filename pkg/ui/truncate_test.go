package ui

import (
	"testing"
	"unicode/utf8"
)

func TestTruncateString_UTF8Safe(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "zero max", input: "hello", maxLen: 0, want: ""},
		{name: "fits", input: "hello", maxLen: 10, want: "hello"},
		{name: "small max no ellipsis", input: "こんにちは", maxLen: 3, want: "こんに"},
		{name: "ellipsis", input: "a🙂b🙂c", maxLen: 4, want: "a🙂b…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Fatalf("truncateString(%q, %d) = %q; want %q", tt.input, tt.maxLen, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("truncateString output is not valid UTF-8: %q", got)
			}
			if tt.maxLen >= 0 && len([]rune(got)) > tt.maxLen {
				t.Fatalf("truncateString output has %d runes; max %d", len([]rune(got)), tt.maxLen)
			}
		})
	}
}

func TestTruncateStrSprint_UTF8Safe(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "zero max", input: "hello", maxLen: 0, want: ""},
		{name: "fits", input: "hello", maxLen: 10, want: "hello"},
		{name: "small max no ellipsis", input: "🙂🙂🙂", maxLen: 2, want: "🙂🙂"},
		{name: "ellipsis", input: "a🙂b🙂c", maxLen: 4, want: "a🙂b…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStrSprint(tt.input, tt.maxLen)
			if got != tt.want {
				t.Fatalf("truncateStrSprint(%q, %d) = %q; want %q", tt.input, tt.maxLen, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("truncateStrSprint output is not valid UTF-8: %q", got)
			}
			if tt.maxLen >= 0 && len([]rune(got)) > tt.maxLen {
				t.Fatalf("truncateStrSprint output has %d runes; max %d", len([]rune(got)), tt.maxLen)
			}
		})
	}
}
