# Golang Best Practices

This guide outlines the best practices for Go development within this project, modeled after high-quality TUI applications.

## 1. Project Structure
- **cmd/**: Contains the main applications. Each subdirectory should be a main package (e.g., `cmd/bv/main.go`).
- **pkg/**: Library code organized by domain. This project uses `pkg/` for all packages:
  - **pkg/ui/**: User Interface components (Bubble Tea models, views).
  - **pkg/model/**: Domain models and data structures.
  - **pkg/analysis/**: Graph analysis and metrics computation.
  - **pkg/loader/**: Beads file discovery and parsing.
  - **pkg/export/**: Export functionality (Markdown, HTML, SQLite).
- **tests/**: End-to-end and integration tests.

## 2. Code Style
- **Formatting**: Always use `gofmt` (or `goimports`).
- **Naming**:
  - Use `CamelCase` for exported identifiers.
  - Use `camelCase` for unexported identifiers.
  - Keep variable names short but descriptive (e.g., `i` for index, `ctx` for context).
  - Package names should be short, lowercase, and singular (e.g., `model`, `ui`, `auth`).
- **Error Handling**:
  - Return errors as the last return value.
  - Check errors immediately.
  - Use `fmt.Errorf` with `%w` to wrap errors for context.
  - Don't panic unless it's a truly unrecoverable initialization error.

## 3. TUI Development (Charmbracelet Stack)
- **Architecture**: Follow The Elm Architecture (Model, View, Update) via `bubbletea`.
- **Components**: Break down complex UIs into smaller, reusable `tea.Model` components (e.g., `ListView`, `DetailView`, `FilterBar`).
- **Styling**: Use `lipgloss` for all styling. Define a central `styles.go` to maintain consistency (colors, margins, padding).
- **State**: Keep the main model clean. Delegate update logic to sub-models.

## 4. Configuration & Data
- **Config**: Use struct-based configuration. Load from environment variables or config files (YAML/JSON).
- **Data Access**: separate data loading (Loader/Repository pattern) from the UI logic. The UI should receive data, not fetch it directly if possible.

## 5. Testing
- Write unit tests for logic-heavy packages.
- Use table-driven tests for parser/validator logic.
- Run tests with `go test ./...`.

## 6. Dependencies
- Use `go mod` for dependency management.
- specific versions should be pinned in `go.mod`.
- Vendor dependencies if necessary for offline builds, but standard `go mod` is usually sufficient.

## 7. Documentation
- Add comments to exported functions and types (`// TypeName represents...`).
- Maintain a `README.md` with installation and usage instructions.

## 8. Concurrency
- Use channels for communication between goroutines.
- Use `sync.Mutex` for protecting shared state if not using channels.
- Avoid global state where possible.
