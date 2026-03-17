package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/loader"
)

func parseCommandLine(input string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	var args []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	flush := func() {
		if current.Len() == 0 {
			return
		}
		args = append(args, current.String())
		current.Reset()
	}

	for i := 0; i < len(input); {
		ch := input[i]
		if inSingle {
			if ch == '\'' {
				inSingle = false
				i++
				continue
			}
			current.WriteByte(ch)
			i++
			continue
		}
		if inDouble {
			switch ch {
			case '"':
				inDouble = false
				i++
				continue
			case '\\':
				if i+1 >= len(input) {
					return nil, fmt.Errorf("unterminated escape")
				}
				next := input[i+1]
				// In double quotes, only treat \" and \\ as escapes; otherwise preserve backslash.
				if next == '"' || next == '\\' {
					current.WriteByte(next)
					i += 2
					continue
				}
				current.WriteByte('\\')
				i++
				continue
			default:
				current.WriteByte(ch)
				i++
				continue
			}
		}

		switch ch {
		case ' ', '\t', '\n', '\r':
			flush()
			i++
		case '\'':
			inSingle = true
			i++
		case '"':
			inDouble = true
			i++
		case '\\':
			if i+1 >= len(input) {
				return nil, fmt.Errorf("unterminated escape")
			}
			next := input[i+1]
			if next == ' ' || next == '\t' || next == '\n' || next == '\r' || next == '\\' || next == '"' || next == '\'' {
				current.WriteByte(next)
				i += 2
				continue
			}
			current.WriteByte('\\')
			i++
		default:
			current.WriteByte(ch)
			i++
		}
	}

	if inSingle {
		return nil, fmt.Errorf("unterminated single quote")
	}
	if inDouble {
		return nil, fmt.Errorf("unterminated double quote")
	}
	flush()
	return args, nil
}

type editorCommandKind int

const (
	editorCommandOK editorCommandKind = iota
	editorCommandEmpty
	editorCommandTerminal
	editorCommandForbidden
)

type allowlistedGUIEditorKind int

const (
	allowlistedGUIEditorUnknown allowlistedGUIEditorKind = iota
	allowlistedGUIEditorOpenText
	allowlistedGUIEditorXdgOpen
	allowlistedGUIEditorCode
	allowlistedGUIEditorCodeInsiders
	allowlistedGUIEditorCursor
	allowlistedGUIEditorGedit
	allowlistedGUIEditorKate
	allowlistedGUIEditorXed
	allowlistedGUIEditorNotepad
)

var terminalEditorExecutables = map[string]bool{
	"vim":   true,
	"vi":    true,
	"nvim":  true,
	"nano":  true,
	"emacs": true,
	"pico":  true,
	"joe":   true,
	"ne":    true,
}

var forbiddenEditorExecutables = map[string]bool{
	// Shells and command interpreters.
	"sh":         true,
	"bash":       true,
	"zsh":        true,
	"fish":       true,
	"cmd":        true,
	"powershell": true,
	"pwsh":       true,
}

func normalizeExecutableBase(executable string) string {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return ""
	}
	base := executable
	if idx := strings.LastIndexAny(base, `/\`); idx >= 0 {
		base = base[idx+1:]
	}
	base = strings.ToLower(base)
	return strings.TrimSuffix(base, ".exe")
}

func classifyEditorCommand(editorArgs []string) (string, editorCommandKind) {
	if len(editorArgs) == 0 {
		return "", editorCommandEmpty
	}
	base := normalizeExecutableBase(editorArgs[0])
	if base == "" {
		return "", editorCommandEmpty
	}
	if terminalEditorExecutables[base] {
		return base, editorCommandTerminal
	}
	if forbiddenEditorExecutables[base] {
		return base, editorCommandForbidden
	}
	return base, editorCommandOK
}

func allowlistedGUIEditorKindForBase(base string) allowlistedGUIEditorKind {
	switch base {
	case "open":
		return allowlistedGUIEditorOpenText
	case "xdg-open":
		return allowlistedGUIEditorXdgOpen
	case "code":
		return allowlistedGUIEditorCode
	case "code-insiders":
		return allowlistedGUIEditorCodeInsiders
	case "cursor":
		return allowlistedGUIEditorCursor
	case "gedit":
		return allowlistedGUIEditorGedit
	case "kate":
		return allowlistedGUIEditorKate
	case "xed":
		return allowlistedGUIEditorXed
	case "notepad":
		return allowlistedGUIEditorNotepad
	default:
		return allowlistedGUIEditorUnknown
	}
}

func allowlistedGUIEditorDisplayName(kind allowlistedGUIEditorKind) string {
	switch kind {
	case allowlistedGUIEditorOpenText:
		return "default text editor"
	case allowlistedGUIEditorXdgOpen:
		return "default app"
	case allowlistedGUIEditorCode:
		return "code"
	case allowlistedGUIEditorCodeInsiders:
		return "code-insiders"
	case allowlistedGUIEditorCursor:
		return "cursor"
	case allowlistedGUIEditorGedit:
		return "gedit"
	case allowlistedGUIEditorKate:
		return "kate"
	case allowlistedGUIEditorXed:
		return "xed"
	case allowlistedGUIEditorNotepad:
		return "notepad"
	default:
		return "editor"
	}
}

func startAllowlistedGUIEditor(kind allowlistedGUIEditorKind, targetFile string) (allowlistedGUIEditorKind, error) {
	switch kind {
	case allowlistedGUIEditorOpenText:
		return kind, exec.Command("open", "-t", targetFile).Start()
	case allowlistedGUIEditorXdgOpen:
		return kind, exec.Command("xdg-open", targetFile).Start()
	case allowlistedGUIEditorCode:
		if runtime.GOOS == "darwin" {
			// Prefer launching the app directly so we don't depend on the `code` CLI being installed in PATH.
			if err := exec.Command("open", "-a", "Visual Studio Code", targetFile).Start(); err == nil {
				return kind, nil
			}
		}
		if _, err := exec.LookPath("code"); err == nil {
			return kind, exec.Command("code", targetFile).Start()
		}
		if runtime.GOOS == "linux" {
			if _, err := exec.LookPath("xdg-open"); err == nil {
				return allowlistedGUIEditorXdgOpen, exec.Command("xdg-open", targetFile).Start()
			}
		}
		return kind, fmt.Errorf("code not found in PATH")
	case allowlistedGUIEditorCodeInsiders:
		if runtime.GOOS == "darwin" {
			// Prefer launching the app directly so we don't depend on the `code-insiders` CLI being installed in PATH.
			if err := exec.Command("open", "-a", "Visual Studio Code - Insiders", targetFile).Start(); err == nil {
				return kind, nil
			}
		}
		if _, err := exec.LookPath("code-insiders"); err == nil {
			return kind, exec.Command("code-insiders", targetFile).Start()
		}
		if runtime.GOOS == "linux" {
			if _, err := exec.LookPath("xdg-open"); err == nil {
				return allowlistedGUIEditorXdgOpen, exec.Command("xdg-open", targetFile).Start()
			}
		}
		return kind, fmt.Errorf("code-insiders not found in PATH")
	case allowlistedGUIEditorCursor:
		if runtime.GOOS == "darwin" {
			// Prefer launching the app directly so we don't depend on the `cursor` CLI being installed in PATH.
			if err := exec.Command("open", "-a", "Cursor", targetFile).Start(); err == nil {
				return kind, nil
			}
		}
		if _, err := exec.LookPath("cursor"); err == nil {
			return kind, exec.Command("cursor", targetFile).Start()
		}
		if runtime.GOOS == "linux" {
			if _, err := exec.LookPath("xdg-open"); err == nil {
				return allowlistedGUIEditorXdgOpen, exec.Command("xdg-open", targetFile).Start()
			}
		}
		return kind, fmt.Errorf("cursor not found in PATH")
	case allowlistedGUIEditorGedit:
		return kind, exec.Command("gedit", targetFile).Start()
	case allowlistedGUIEditorKate:
		return kind, exec.Command("kate", targetFile).Start()
	case allowlistedGUIEditorXed:
		return kind, exec.Command("xed", targetFile).Start()
	case allowlistedGUIEditorNotepad:
		return kind, exec.Command("notepad", targetFile).Start()
	default:
		return kind, fmt.Errorf("unsupported editor")
	}
}

// openInEditor opens the beads file in the user's preferred editor
// Uses m.beadsPath which respects issues.jsonl (canonical per beads upstream)
func (m *Model) openInEditor() {
	// Use the configured beadsPath instead of hardcoded path
	beadsFile := m.beadsPath
	if beadsFile == "" {
		cwd, _ := os.Getwd()
		if found, err := loader.FindJSONLPath(filepath.Join(cwd, ".beads")); err == nil {
			beadsFile = found
		}
	}
	if beadsFile == "" {
		m.statusMsg = "❌ No .beads directory or beads.jsonl found"
		m.statusIsError = true
		return
	}
	if _, err := os.Stat(beadsFile); os.IsNotExist(err) {
		m.statusMsg = fmt.Sprintf("❌ Beads file not found: %s", beadsFile)
		m.statusIsError = true
		return
	}

	// Determine editor - prefer GUI editors that work in background
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}

	ignoredEditorBase := ""
	var requestedEditorKind allowlistedGUIEditorKind
	if editor != "" {
		editorArgs, err := parseCommandLine(editor)
		if err != nil {
			m.statusMsg = fmt.Sprintf("❌ Invalid $EDITOR/$VISUAL: %v", err)
			m.statusIsError = true
			return
		}

		editorBase, kind := classifyEditorCommand(editorArgs)
		switch kind {
		case editorCommandTerminal:
			m.statusMsg = fmt.Sprintf("⚠️ %s is a terminal editor - set $EDITOR to a GUI editor or quit first", editorBase)
			m.statusIsError = true
			return
		case editorCommandForbidden:
			m.statusMsg = fmt.Sprintf("❌ Refusing to run %s as editor (shell/interpreter). Set $EDITOR to a GUI editor", editorBase)
			m.statusIsError = true
			return
		case editorCommandEmpty:
			m.statusMsg = "❌ Invalid $EDITOR/$VISUAL: empty command"
			m.statusIsError = true
			return
		default:
			requestedEditorKind = allowlistedGUIEditorKindForBase(editorBase)
			if requestedEditorKind == allowlistedGUIEditorUnknown {
				ignoredEditorBase = editorBase
				editor = ""
			}
		}
	}

	// If no editor set, try platform-specific GUI options
	if editor == "" && requestedEditorKind == allowlistedGUIEditorUnknown {
		switch runtime.GOOS {
		case "darwin":
			requestedEditorKind = allowlistedGUIEditorOpenText
		case "windows":
			requestedEditorKind = allowlistedGUIEditorNotepad
		case "linux":
			// Try xdg-open first, then common GUI editors
			for _, tryEditor := range []string{"xdg-open", "code", "code-insiders", "cursor", "gedit", "kate", "xed"} {
				if _, err := exec.LookPath(tryEditor); err == nil {
					requestedEditorKind = allowlistedGUIEditorKindForBase(tryEditor)
					break
				}
			}
		}
	}

	if requestedEditorKind == allowlistedGUIEditorUnknown {
		m.statusMsg = "❌ No GUI editor found. Set $EDITOR to a GUI editor"
		m.statusIsError = true
		return
	}

	actualKind, err := startAllowlistedGUIEditor(requestedEditorKind, beadsFile)
	if err != nil {
		m.statusMsg = fmt.Sprintf("❌ Failed to open editor: %v", err)
		m.statusIsError = true
		return
	}
	requestedEditorKind = actualKind

	if ignoredEditorBase != "" {
		m.statusMsg = fmt.Sprintf("📝 Opened in %s (ignored $EDITOR=%s)", allowlistedGUIEditorDisplayName(requestedEditorKind), ignoredEditorBase)
	} else {
		m.statusMsg = fmt.Sprintf("📝 Opened in %s", allowlistedGUIEditorDisplayName(requestedEditorKind))
	}
	m.statusIsError = false
}
