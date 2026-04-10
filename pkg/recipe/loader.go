package recipe

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

//go:embed defaults/recipes.yaml
var defaultRecipesFS embed.FS

// RecipeFile represents the structure of a recipes YAML file
type RecipeFile struct {
	Recipes map[string]*Recipe `yaml:"recipes"`
}

// RecipeSummary is a lightweight representation for discovery
type RecipeSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"` // "builtin", "user", "project"
}

// Loader handles loading and merging recipes from multiple sources
type Loader struct {
	recipes    map[string]Recipe
	sources    map[string]string // recipe name -> source
	userPath   string
	projectDir string
	warnings   []string
}

// LoaderOption configures the loader
type LoaderOption func(*Loader)

// WithUserPath sets a custom user config path (default: ~/.config/bv/recipes.yaml)
func WithUserPath(path string) LoaderOption {
	return func(l *Loader) {
		l.userPath = path
	}
}

// WithProjectDir sets the project directory (default: current directory)
func WithProjectDir(dir string) LoaderOption {
	return func(l *Loader) {
		l.projectDir = dir
	}
}

// NewLoader creates a new recipe loader with options
func NewLoader(opts ...LoaderOption) *Loader {
	l := &Loader{
		recipes: make(map[string]Recipe),
		sources: make(map[string]string),
	}

	for _, opt := range opts {
		opt(l)
	}

	// Set defaults
	if l.userPath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			l.userPath = filepath.Join(home, ".config", "bv", "recipes.yaml")
		}
	}

	if l.projectDir == "" {
		l.projectDir, _ = os.Getwd()
	}

	return l
}

// Load loads recipes from all sources in order: builtin < user < project
func (l *Loader) Load() error {
	// 1. Load embedded defaults
	if err := l.loadBuiltin(); err != nil {
		return fmt.Errorf("loading builtin recipes: %w", err)
	}

	// 2. Load user config (optional, no error if missing)
	if l.userPath != "" {
		if err := l.loadFromFile(l.userPath, "user"); err != nil {
			// Only add warning, don't fail
			if !os.IsNotExist(err) {
				l.warnings = append(l.warnings, fmt.Sprintf("user config: %v", err))
			}
		}
	}

	// 3. Load project config (optional, no error if missing)
	if l.projectDir != "" {
		projectPath := filepath.Join(l.projectDir, ".bv", "recipes.yaml")
		if err := l.loadFromFile(projectPath, "project"); err != nil {
			if !os.IsNotExist(err) {
				l.warnings = append(l.warnings, fmt.Sprintf("project config: %v", err))
			}
		}
	}

	return nil
}

// loadBuiltin loads the embedded default recipes
func (l *Loader) loadBuiltin() error {
	data, err := defaultRecipesFS.ReadFile("defaults/recipes.yaml")
	if err != nil {
		return err
	}

	var file RecipeFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parsing embedded defaults: %w", err)
	}

	for name, recipe := range file.Recipes {
		if recipe == nil {
			continue
		}
		recipe.Name = name
		l.recipes[name] = *recipe
		l.sources[name] = "builtin"
	}

	return nil
}

// loadFromFile loads recipes from a YAML file and merges them
func (l *Loader) loadFromFile(path, source string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var file RecipeFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	for name, recipe := range file.Recipes {
		if recipe == nil {
			// Explicit null means "disable this recipe"
			delete(l.recipes, name)
			delete(l.sources, name)
			continue
		}
		recipe.Name = name
		l.recipes[name] = *recipe
		l.sources[name] = source
	}

	return nil
}

// Get returns a recipe by name, or nil if not found
func (l *Loader) Get(name string) *Recipe {
	if recipe, ok := l.recipes[name]; ok {
		return &recipe
	}
	return nil
}

// List returns all available recipes sorted by name
func (l *Loader) List() []Recipe {
	var names []string
	for name := range l.recipes {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]Recipe, 0, len(names))
	for _, name := range names {
		result = append(result, l.recipes[name])
	}
	return result
}

// ListSummaries returns lightweight recipe summaries for discovery, sorted by name
func (l *Loader) ListSummaries() []RecipeSummary {
	var names []string
	for name := range l.recipes {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]RecipeSummary, 0, len(names))
	for _, name := range names {
		result = append(result, RecipeSummary{
			Name:        name,
			Description: l.recipes[name].Description,
			Source:      l.sources[name],
		})
	}
	return result
}

// Names returns all recipe names sorted alphabetically
func (l *Loader) Names() []string {
	names := make([]string, 0, len(l.recipes))
	for name := range l.recipes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Warnings returns any warnings from loading
func (l *Loader) Warnings() []string {
	return l.warnings
}

// Source returns the source of a recipe ("builtin", "user", "project")
func (l *Loader) Source(name string) string {
	return l.sources[name]
}

// LoadDefault creates a loader and loads with default settings
func LoadDefault() (*Loader, error) {
	loader := NewLoader()
	if err := loader.Load(); err != nil {
		return nil, err
	}
	return loader, nil
}
