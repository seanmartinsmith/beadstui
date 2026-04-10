package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestTutorialProgressPath(t *testing.T) {
	path := TutorialProgressPath()
	if path == "" {
		t.Skip("Could not determine home directory")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("Expected absolute path, got %q", path)
	}
	if filepath.Base(path) != "tutorial-progress.json" {
		t.Errorf("Expected filename tutorial-progress.json, got %q", filepath.Base(path))
	}
}

func TestTutorialProgressManager_Basic(t *testing.T) {
	// Create a fresh manager for testing (bypass singleton)
	pm := &tutorialProgressManager{
		progress: &TutorialProgress{
			ViewedPages: make(map[string]bool),
		},
	}

	// Test initial state
	if pm.GetViewedCount() != 0 {
		t.Errorf("Expected 0 viewed pages, got %d", pm.GetViewedCount())
	}
	if pm.IsPageViewed("intro") {
		t.Error("Expected intro page to not be viewed initially")
	}
	if pm.HasCompletedOnce() {
		t.Error("Expected not completed initially")
	}

	// Mark page viewed
	pm.MarkPageViewed("intro")
	if !pm.IsPageViewed("intro") {
		t.Error("Expected intro page to be viewed after marking")
	}
	if pm.GetViewedCount() != 1 {
		t.Errorf("Expected 1 viewed page, got %d", pm.GetViewedCount())
	}
	if pm.GetLastPageID() != "intro" {
		t.Errorf("Expected last page to be 'intro', got %q", pm.GetLastPageID())
	}

	// Mark same page again (idempotent)
	pm.MarkPageViewed("intro")
	if pm.GetViewedCount() != 1 {
		t.Errorf("Expected still 1 viewed page, got %d", pm.GetViewedCount())
	}

	// Mark another page
	pm.MarkPageViewed("concepts")
	if pm.GetViewedCount() != 2 {
		t.Errorf("Expected 2 viewed pages, got %d", pm.GetViewedCount())
	}
	if pm.GetLastPageID() != "concepts" {
		t.Errorf("Expected last page to be 'concepts', got %q", pm.GetLastPageID())
	}

	// Test completed flag
	pm.SetCompletedOnce()
	if !pm.HasCompletedOnce() {
		t.Error("Expected completed after setting")
	}
}

func TestTutorialProgressManager_SaveLoad(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create a manager
	pm := &tutorialProgressManager{
		progress: &TutorialProgress{
			ViewedPages: make(map[string]bool),
		},
	}

	// Mark some pages viewed
	pm.MarkPageViewed("intro")
	pm.MarkPageViewed("concepts")
	pm.SetCompletedOnce()

	// Save
	if err := pm.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, ".config", "bv", "tutorial-progress.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Create new manager and load
	pm2 := &tutorialProgressManager{
		progress: &TutorialProgress{
			ViewedPages: make(map[string]bool),
		},
	}
	if err := pm2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify loaded data
	if !pm2.IsPageViewed("intro") {
		t.Error("Expected intro page to be viewed after load")
	}
	if !pm2.IsPageViewed("concepts") {
		t.Error("Expected concepts page to be viewed after load")
	}
	if !pm2.HasCompletedOnce() {
		t.Error("Expected completed after load")
	}
	if pm2.GetViewedCount() != 2 {
		t.Errorf("Expected 2 viewed pages after load, got %d", pm2.GetViewedCount())
	}
}

func TestTutorialProgressManager_LoadNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pm := &tutorialProgressManager{
		progress: &TutorialProgress{
			ViewedPages: make(map[string]bool),
		},
	}

	// Load should succeed with empty progress when file doesn't exist
	if err := pm.Load(); err != nil {
		t.Fatalf("Load should succeed for nonexistent file: %v", err)
	}

	if pm.GetViewedCount() != 0 {
		t.Errorf("Expected 0 viewed pages for fresh load, got %d", pm.GetViewedCount())
	}
}

func TestTutorialProgressManager_LoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create invalid JSON file
	configPath := filepath.Join(tmpDir, ".config", "bv", "tutorial-progress.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("{invalid"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	pm := &tutorialProgressManager{
		progress: &TutorialProgress{
			ViewedPages: make(map[string]bool),
		},
	}

	// Load should return error but initialize with empty progress
	err := pm.Load()
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}

	// Should still be usable with empty progress
	if pm.GetViewedCount() != 0 {
		t.Errorf("Expected 0 viewed pages after invalid JSON load, got %d", pm.GetViewedCount())
	}
}

func TestTutorialProgressManager_Reset(t *testing.T) {
	pm := &tutorialProgressManager{
		progress: &TutorialProgress{
			ViewedPages: make(map[string]bool),
		},
	}

	pm.MarkPageViewed("intro")
	pm.SetCompletedOnce()

	if pm.GetViewedCount() != 1 {
		t.Errorf("Expected 1 viewed page before reset, got %d", pm.GetViewedCount())
	}

	pm.Reset()

	if pm.GetViewedCount() != 0 {
		t.Errorf("Expected 0 viewed pages after reset, got %d", pm.GetViewedCount())
	}
	if pm.HasCompletedOnce() {
		t.Error("Expected not completed after reset")
	}
	if !pm.IsDirty() {
		t.Error("Expected dirty after reset")
	}
}

func TestTutorialProgressManager_GetProgress(t *testing.T) {
	pm := &tutorialProgressManager{
		progress: &TutorialProgress{
			ViewedPages: make(map[string]bool),
		},
	}

	pm.MarkPageViewed("intro")
	pm.MarkPageViewed("concepts")

	// Get copy
	progress := pm.GetProgress()

	// Verify copy
	if len(progress.ViewedPages) != 2 {
		t.Errorf("Expected 2 viewed pages in copy, got %d", len(progress.ViewedPages))
	}
	if !progress.ViewedPages["intro"] {
		t.Error("Expected intro in copy")
	}

	// Modify copy should not affect original
	progress.ViewedPages["new"] = true
	if pm.IsPageViewed("new") {
		t.Error("Modifying copy should not affect original")
	}
}

func TestTutorialProgressManager_Concurrent(t *testing.T) {
	pm := &tutorialProgressManager{
		progress: &TutorialProgress{
			ViewedPages: make(map[string]bool),
		},
	}

	var wg sync.WaitGroup
	pages := []string{"p1", "p2", "p3", "p4", "p5"}

	// Concurrent marks
	for _, p := range pages {
		wg.Add(1)
		go func(pageID string) {
			defer wg.Done()
			pm.MarkPageViewed(pageID)
		}(p)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pm.GetViewedCount()
			pm.GetLastPageID()
		}()
	}

	wg.Wait()

	// All pages should be marked
	if pm.GetViewedCount() != len(pages) {
		t.Errorf("Expected %d viewed pages, got %d", len(pages), pm.GetViewedCount())
	}
}

func TestTutorialProgressManager_IsDirty(t *testing.T) {
	pm := &tutorialProgressManager{
		progress: &TutorialProgress{
			ViewedPages: make(map[string]bool),
		},
	}

	if pm.IsDirty() {
		t.Error("Expected not dirty initially")
	}

	pm.MarkPageViewed("intro")
	if !pm.IsDirty() {
		t.Error("Expected dirty after marking page")
	}
}

func TestTutorialProgress_JSONSerialization(t *testing.T) {
	progress := TutorialProgress{
		ViewedPages: map[string]bool{
			"intro":    true,
			"concepts": true,
		},
		LastPageID:    "concepts",
		CompletedOnce: true,
	}

	data, err := json.Marshal(progress)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var loaded TutorialProgress
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(loaded.ViewedPages) != 2 {
		t.Errorf("Expected 2 viewed pages, got %d", len(loaded.ViewedPages))
	}
	if loaded.LastPageID != "concepts" {
		t.Errorf("Expected last page 'concepts', got %q", loaded.LastPageID)
	}
	if !loaded.CompletedOnce {
		t.Error("Expected completed once")
	}
}

// Tests for TutorialModel integration methods (bv-j4og)

func TestTutorialModel_SaveProgress(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Reset singleton for test isolation
	progressManager = nil
	progressManagerOnce = sync.Once{}

	// Create tutorial model
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	m := NewTutorialModel(theme)

	// Navigate to a page (page 1)
	m.NextPage()

	// Save progress
	if err := m.SaveProgress(); err != nil {
		t.Fatalf("SaveProgress failed: %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tmpDir, ".config", "bv", "tutorial-progress.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Progress file was not created")
	}

	// Verify persisted data
	pm := GetTutorialProgressManager()
	if !pm.IsPageViewed(m.pages[1].ID) {
		t.Error("Expected current page to be marked as viewed")
	}
}

func TestTutorialModel_LoadProgress(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Reset singleton
	progressManager = nil
	progressManagerOnce = sync.Once{}

	// Pre-populate progress file
	pm := GetTutorialProgressManager()
	pm.MarkPageViewed("intro-welcome")
	pm.MarkPageViewed("intro-philosophy")
	if err := pm.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reset singleton to simulate fresh start
	progressManager = nil
	progressManagerOnce = sync.Once{}

	// Create new tutorial model and load progress
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	m := NewTutorialModel(theme)
	m.LoadProgress()

	// Verify progress was loaded
	if !m.progress["intro-welcome"] {
		t.Error("Expected intro-welcome to be loaded into model progress")
	}
	if !m.progress["intro-philosophy"] {
		t.Error("Expected intro-philosophy to be loaded into model progress")
	}
}

func TestTutorialModel_HasViewedPage(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Reset singleton
	progressManager = nil
	progressManagerOnce = sync.Once{}

	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	m := NewTutorialModel(theme)

	// Not viewed initially
	if m.HasViewedPage("intro-welcome") {
		t.Error("Expected intro-welcome to not be viewed initially")
	}

	// Mark as viewed in local progress
	m.progress["intro-welcome"] = true
	if !m.HasViewedPage("intro-welcome") {
		t.Error("Expected intro-welcome to be viewed after local mark")
	}

	// Check persisted state
	pm := GetTutorialProgressManager()
	pm.MarkPageViewed("intro-philosophy")

	if !m.HasViewedPage("intro-philosophy") {
		t.Error("Expected intro-philosophy to be viewed from persisted state")
	}
}

func TestGetTutorialProgressManager_Singleton(t *testing.T) {
	// Reset singleton
	progressManager = nil
	progressManagerOnce = sync.Once{}

	pm1 := GetTutorialProgressManager()
	pm2 := GetTutorialProgressManager()

	if pm1 != pm2 {
		t.Error("GetTutorialProgressManager should return the same instance")
	}
}

func TestTutorialModel_SaveProgress_AllViewed(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Reset singleton
	progressManager = nil
	progressManagerOnce = sync.Once{}

	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	m := NewTutorialModel(theme)

	// Mark all pages as viewed
	pm := GetTutorialProgressManager()
	for _, page := range m.pages {
		pm.MarkPageViewed(page.ID)
	}

	// Save progress - should set completed
	if err := m.SaveProgress(); err != nil {
		t.Fatalf("SaveProgress failed: %v", err)
	}

	if !pm.HasCompletedOnce() {
		t.Error("Expected tutorial to be marked as completed when all pages viewed")
	}
}
