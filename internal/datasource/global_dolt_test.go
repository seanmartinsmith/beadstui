package datasource

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildIssuesQuery(t *testing.T) {
	tests := []struct {
		name      string
		databases []string
		wantParts int // number of UNION ALL parts
		wantErr   bool
	}{
		{
			name:      "single database",
			databases: []string{"myproject"},
			wantParts: 1,
		},
		{
			name:      "three databases",
			databases: []string{"alpha", "beta", "gamma"},
			wantParts: 3,
		},
		{
			name:      "ten databases",
			databases: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
			wantParts: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := buildIssuesQuery(tt.databases)
			if (err != nil) != tt.wantErr {
				t.Fatalf("buildIssuesQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			// Should contain UNION ALL between parts
			if tt.wantParts > 1 {
				unions := strings.Count(query, "UNION ALL")
				if unions != tt.wantParts-1 {
					t.Errorf("expected %d UNION ALL, got %d", tt.wantParts-1, unions)
				}
			}

			// Should end with ORDER BY
			if !strings.HasSuffix(query, "ORDER BY updated_at DESC") {
				t.Error("query should end with ORDER BY updated_at DESC")
			}

			// Each database should be backtick-quoted
			for _, db := range tt.databases {
				if !strings.Contains(query, "`"+db+"`") {
					t.Errorf("database %q not backtick-quoted in query", db)
				}
			}

			// Should contain _global_source column
			if !strings.Contains(query, "_global_source") {
				t.Error("query should contain _global_source column")
			}

			// Should contain IssuesColumns
			if !strings.Contains(query, "id, title, description") {
				t.Error("query should contain columns from IssuesColumns")
			}

			// Should filter tombstone
			if !strings.Contains(query, "status != 'tombstone'") {
				t.Error("query should filter tombstone issues")
			}
		})
	}
}

func TestBuildIssuesQuery_Empty(t *testing.T) {
	_, err := buildIssuesQuery(nil)
	if err == nil {
		t.Fatal("expected error for empty database list")
	}
	_, err = buildIssuesQuery([]string{})
	if err == nil {
		t.Fatal("expected error for empty database list")
	}
}

func TestBuildLabelsQuery(t *testing.T) {
	query, err := buildLabelsQuery([]string{"proj_a", "proj-b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(query, "`proj_a`.labels") {
		t.Error("should reference proj_a.labels")
	}
	if !strings.Contains(query, "`proj-b`.labels") {
		t.Error("should reference proj-b.labels with backtick quoting")
	}
	if !strings.Contains(query, "_db") {
		t.Error("should include _db column")
	}
	if strings.Count(query, "UNION ALL") != 1 {
		t.Error("two databases should produce one UNION ALL")
	}
}

func TestBuildDependenciesQuery(t *testing.T) {
	query, err := buildDependenciesQuery([]string{"alpha", "beta"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must use `type`, not `dependency_type` (Dolt column name)
	if !strings.Contains(query, ", type,") {
		t.Error("should use 'type' column, not 'dependency_type'")
	}
	if !strings.Contains(query, "depends_on_id") {
		t.Error("should include depends_on_id column")
	}
}

func TestBuildCommentsQuery(t *testing.T) {
	query, err := buildCommentsQuery([]string{"one", "two", "three"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasSuffix(query, "ORDER BY created_at") {
		t.Error("comments query should end with ORDER BY created_at")
	}
	if strings.Count(query, "UNION ALL") != 2 {
		t.Error("three databases should produce two UNION ALL")
	}
}

func TestBuildLastModifiedQuery(t *testing.T) {
	query, err := buildLastModifiedQuery([]string{"db1", "db2", "db3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(query, "SELECT MAX(m) FROM (") {
		t.Error("should wrap in outer MAX aggregation")
	}
	if !strings.HasSuffix(query, ") t") {
		t.Error("should end with ) t alias")
	}
	for _, db := range []string{"db1", "db2", "db3"} {
		if !strings.Contains(query, "`"+db+"`.issues") {
			t.Errorf("should reference %s.issues", db)
		}
	}
}

func TestFilterSystemDatabases(t *testing.T) {
	input := []string{
		"myproject",
		"information_schema",
		"mysql",
		"dolt",
		"another_project",
		"dolt_procedures",
		"sys",
		"third_project",
	}

	got := FilterSystemDatabases(input)
	want := []string{"myproject", "another_project", "third_project"}

	if len(got) != len(want) {
		t.Fatalf("got %d databases, want %d: %v", len(got), len(want), got)
	}
	for i, name := range got {
		if name != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, name, want[i])
		}
	}
}

func TestBacktickQuoting(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "`simple`"},
		{"my-project", "`my-project`"},
		{"with_underscore", "`with_underscore`"},
		{"has123numbers", "`has123numbers`"},
		{"back`tick", "`back``tick`"}, // backtick in name gets escaped
	}

	for _, tt := range tests {
		got := backtickQuote(tt.input)
		if got != tt.want {
			t.Errorf("backtickQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDiscoverSharedServer_EnvOverride(t *testing.T) {
	t.Setenv("BT_GLOBAL_DOLT_PORT", "9999")

	host, port, err := DiscoverSharedServer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "127.0.0.1" {
		t.Errorf("host = %q, want 127.0.0.1", host)
	}
	if port != 9999 {
		t.Errorf("port = %d, want 9999", port)
	}
}

func TestDiscoverSharedServer_PortFile(t *testing.T) {
	// Create a temp directory structure mimicking ~/.beads/shared-server/
	tmpHome := t.TempDir()
	portDir := filepath.Join(tmpHome, ".beads", "shared-server")
	if err := os.MkdirAll(portDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(portDir, "dolt-server.port"), []byte("4321\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override HOME/USERPROFILE to use our temp dir
	t.Setenv("BT_GLOBAL_DOLT_PORT", "") // clear env override
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	host, port, err := DiscoverSharedServer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "127.0.0.1" {
		t.Errorf("host = %q, want 127.0.0.1", host)
	}
	if port != 4321 {
		t.Errorf("port = %d, want 4321", port)
	}
}

func TestDiscoverSharedServer_Missing(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("BT_GLOBAL_DOLT_PORT", "")
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	_, _, err := DiscoverSharedServer()
	if err == nil {
		t.Fatal("expected error when port file doesn't exist")
	}
	if !strings.Contains(err.Error(), "shared Dolt server not running") {
		t.Errorf("error should mention shared server not running, got: %v", err)
	}
}

func TestEscapeSQLString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"it's", "it''s"},
		{"a'b'c", "a''b''c"},
	}
	for _, tt := range tests {
		got := escapeSQLString(tt.input)
		if got != tt.want {
			t.Errorf("escapeSQLString(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNewGlobalDataSource(t *testing.T) {
	ds := NewGlobalDataSource("127.0.0.1", 3307)
	if ds.Type != SourceTypeDoltGlobal {
		t.Errorf("type = %q, want %q", ds.Type, SourceTypeDoltGlobal)
	}
	if !strings.Contains(ds.Path, "127.0.0.1:3307") {
		t.Errorf("path should contain host:port, got %q", ds.Path)
	}
	if !strings.Contains(ds.Path, "parseTime=true") {
		t.Errorf("path should contain parseTime param, got %q", ds.Path)
	}
}
