package agents

import (
	"os"
	"testing"
)

// TestMain redirects the agent-prompts preferences directory to an isolated
// temp dir for the entire test run. Without this, every test that exercises
// SaveAgentPromptPreference / RecordAccept / RecordDecline writes a leaked
// .json file under os.UserConfigDir() (e.g. %APPDATA%\bt\agent-prompts\ on
// Windows) and never cleans it up — see bt-zgnoa for the 1,475-file leak
// that surfaced this. Each test's t.TempDir() workDir still produces a
// unique hash filename, so the shared prefs dir doesn't cause collisions.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "bt-agent-prompts-test-*")
	if err != nil {
		panic("failed to create isolated prefs dir for tests: " + err.Error())
	}
	os.Setenv("BT_AGENT_PROMPTS_DIR", tmp)
	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}
