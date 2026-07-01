package ui

import (
	"errors"
	"testing"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/correlation"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// TestLoadHistoryCmd_EmbeddedDegradesWithMessage asserts the History view
// degrades with a clear "not available in embedded mode yet" message rather
// than a silent-empty report when the active source is embedded (bt-ij71a).
func TestLoadHistoryCmd_EmbeddedDegradesWithMessage(t *testing.T) {
	ds := &datasource.DataSource{Type: datasource.SourceTypeEmbeddedDolt, Path: "/some/repo"}
	issues := []model.Issue{{ID: "x-1", Title: "T", Status: model.StatusOpen}}

	cmd := LoadHistoryCmd("/some/repo", "", issues, ds, "")
	if cmd == nil {
		t.Fatal("LoadHistoryCmd returned nil command")
	}
	msg := cmd()
	loaded, ok := msg.(HistoryLoadedMsg)
	if !ok {
		t.Fatalf("expected HistoryLoadedMsg, got %T", msg)
	}
	if loaded.Report != nil {
		t.Error("embedded history should not produce a report (silent-empty), want degradation error")
	}
	if !errors.Is(loaded.Error, correlation.ErrEmbeddedModeUnavailable) {
		t.Errorf("Error = %v, want ErrEmbeddedModeUnavailable", loaded.Error)
	}
}
