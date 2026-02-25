package export

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Prevent any test from accidentally opening a browser
	os.Setenv("BT_NO_BROWSER", "1")
	os.Setenv("BT_TEST_MODE", "1")

	os.Exit(m.Run())
}
