# toon-go

Go bindings for [TOON](https://github.com/Dicklesworthstone/toon_rust) (Token-Optimized Object Notation).

TOON is a format designed to minimize token usage when passing structured data to/from LLM coding agents while remaining human-readable. It typically provides 20-50% token savings compared to JSON.

## Installation

```bash
go get github.com/Dicklesworthstone/toon-go
```

### Prerequisites

This library requires the `tru` CLI binary to be installed:

```bash
# macOS
brew install dicklesworthstone/tap/tru

# One-liner install script
curl -fsSL "https://raw.githubusercontent.com/Dicklesworthstone/toon_rust/main/install.sh" | bash

# From source (requires Rust)
cargo install --git https://github.com/Dicklesworthstone/toon_rust --tag v0.1.1

# Or download from GitHub releases
# https://github.com/Dicklesworthstone/toon_rust/releases
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    "github.com/Dicklesworthstone/toon-go"
)

func main() {
    // Check if tru is available
    if !toon.Available() {
        log.Fatal("tru binary not found")
    }

    // Encode Go data to TOON
    data := map[string]any{
        "users": []any{
            map[string]any{"name": "Alice", "age": 30},
            map[string]any{"name": "Bob", "age": 25},
        },
        "count": 2,
    }

    toonStr, err := toon.Encode(data)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("TOON output:")
    fmt.Println(toonStr)

    // Decode TOON back to Go
    var result map[string]any
    if err := toon.Decode(toonStr, &result); err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Decoded: %v\n", result)
}
```

## API Reference

### Core Functions

```go
// Encode converts Go data to TOON format
func Encode(data any) (string, error)

// Decode parses TOON and unmarshals into v
func Decode(toonStr string, v any) error

// Check if tru binary is available
func Available() bool

// Get path to tru binary
func TruPath() (string, error)
```

### With Options

```go
// Encoding options
opts := toon.EncodeOptions{
    KeyFolding:   "safe",  // Enable key folding ("off" or "safe")
    FlattenDepth: 3,       // Max folding depth
    Delimiter:    ",",     // Array delimiter: ",", "\t", "|"
    Indent:       2,       // Indentation spaces
}
result, err := toon.EncodeWithOptions(data, opts)

// Decoding options
opts := toon.DecodeOptions{
    ExpandPaths: true,  // Enable path expansion
    Strict:      true,  // Strict validation
}
err := toon.DecodeWithOptions(toonStr, opts, &result)
```

### Utility Functions

```go
// Detect if input is JSON or TOON
format := toon.DetectFormat(input) // toon.FormatJSON, toon.FormatTOON, or toon.FormatUnknown

// Convert between formats automatically
result, detectedFormat, err := toon.Convert(input)

// Decode to JSON string (useful for debugging)
jsonStr, err := toon.DecodeToJSON(toonStr)

// Decode to any (when you don't know the structure)
value, err := toon.DecodeToValue(toonStr)
```

## Error Handling

All errors are wrapped in `*toon.ToonError`:

```go
result, err := toon.Encode(data)
if err != nil {
    if toonErr, ok := err.(*toon.ToonError); ok {
        switch toonErr.Code {
        case toon.ErrCodeTruNotFound:
            // tru binary not installed
        case toon.ErrCodeEncodeFailed:
            // Encoding failed
        case toon.ErrCodeDecodeFailed:
            // Decoding failed
        }
        // Access underlying error
        fmt.Println(toonErr.Cause)
    }
}
```

## Environment Variables

- `TOON_TRU_BIN` - Path or command name for `tru` (highest priority)
- `TOON_BIN` - Alternate path/command name for `tru`

## TOON Format Example

JSON input:
```json
{
  "users": [
    {"name": "Alice", "age": 30},
    {"name": "Bob", "age": 25}
  ],
  "count": 2
}
```

TOON output:
```
users[2]{name,age}:
  Alice, 30
  Bob, 25
count: 2
```

This achieves ~35% token reduction for tabular data.

## Integration with Go Tools

Example integration pattern for CLI tools:

```go
type OutputFormat string

const (
    FormatJSON OutputFormat = "json"
    FormatTOON OutputFormat = "toon"
)

func OutputResults(data any, format OutputFormat) error {
    switch format {
    case FormatTOON:
        if !toon.Available() {
            // Graceful fallback
            return OutputResults(data, FormatJSON)
        }
        result, err := toon.Encode(data)
        if err != nil {
            return err
        }
        fmt.Println(result)
    case FormatJSON:
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(data)
    }
    return nil
}
```

## Performance

The library wraps the `tru` CLI binary via subprocess. Typical overhead is 5-10ms per operation. For high-frequency operations, consider:

1. Batching multiple items into a single encode/decode call
2. Using JSON for internal operations, TOON only for final output
3. Caching encoded results when data is stable

## License

MIT License - see [LICENSE](LICENSE)
