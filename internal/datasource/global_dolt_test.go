package datasource

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
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
			// Pretend every column is available on every database so the
			// query shape matches the legacy assertions below.
			columnsByDB := make(map[string]map[string]bool, len(tt.databases))
			for _, d := range tt.databases {
				avail := map[string]bool{}
				for _, c := range IssuesColumnList {
					avail[c] = true
				}
				columnsByDB[d] = avail
			}
			query, err := buildIssuesQuery(tt.databases, columnsByDB)
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
	_, err := buildIssuesQuery(nil, nil)
	if err == nil {
		t.Fatal("expected error for empty database list")
	}
	_, err = buildIssuesQuery([]string{}, nil)
	if err == nil {
		t.Fatal("expected error for empty database list")
	}
}

// TestBuildIssuesQuery_MixedSchema verifies that databases with different
// schemas (some on the fork's column set, some on stock bd's older set)
// produce a UNION ALL where each segment SELECTs the same column count via
// "NULL AS <col>" substitution for missing columns. Regression for bt-ebzy.
func TestBuildIssuesQuery_MixedSchema(t *testing.T) {
	// modern db has every column; legacy db is missing the three session
	// columns added by the local fork's bd-34v Phase 1a/1b.
	modernAvail := map[string]bool{}
	for _, c := range IssuesColumnList {
		modernAvail[c] = true
	}
	legacyAvail := map[string]bool{}
	for _, c := range IssuesColumnList {
		legacyAvail[c] = true
	}
	delete(legacyAvail, "created_by_session")
	delete(legacyAvail, "claimed_by_session")
	delete(legacyAvail, "closed_by_session")

	databases := []string{"modern", "legacy"}
	columnsByDB := map[string]map[string]bool{
		"modern": modernAvail,
		"legacy": legacyAvail,
	}

	query, err := buildIssuesQuery(databases, columnsByDB)
	if err != nil {
		t.Fatalf("buildIssuesQuery() error = %v", err)
	}

	parts := strings.Split(query, " UNION ALL ")
	if len(parts) != 2 {
		t.Fatalf("expected 2 UNION ALL parts, got %d", len(parts))
	}

	modernPart := parts[0]
	legacyPart := parts[1]

	// Modern segment should reference the column directly (not as NULL).
	if !strings.Contains(modernPart, ", created_by_session,") && !strings.Contains(modernPart, ", created_by_session ") {
		t.Errorf("modern segment should select created_by_session directly, got: %s", modernPart)
	}
	if strings.Contains(modernPart, "NULL AS created_by_session") {
		t.Errorf("modern segment should NOT have NULL substitution, got: %s", modernPart)
	}

	// Legacy segment should substitute NULL for each missing column.
	for _, missing := range []string{"created_by_session", "claimed_by_session", "closed_by_session"} {
		needle := "NULL AS " + missing
		if !strings.Contains(legacyPart, needle) {
			t.Errorf("legacy segment should contain %q, got: %s", needle, legacyPart)
		}
	}

	// Both segments must produce the same number of selected expressions
	// before the trailing "FROM" so the UNION column counts match.
	if countSelectExprs(modernPart) != countSelectExprs(legacyPart) {
		t.Errorf("UNION segments have mismatched column counts: modern=%d legacy=%d",
			countSelectExprs(modernPart), countSelectExprs(legacyPart))
	}
}

// countSelectExprs counts comma-separated expressions between SELECT and FROM
// in a single-table query segment. Crude but adequate for the mixed-schema
// regression test above.
func countSelectExprs(segment string) int {
	upper := strings.ToUpper(segment)
	selectIdx := strings.Index(upper, "SELECT ")
	fromIdx := strings.Index(upper, " FROM ")
	if selectIdx < 0 || fromIdx < 0 || fromIdx <= selectIdx {
		return -1
	}
	body := segment[selectIdx+len("SELECT ") : fromIdx]
	return strings.Count(body, ",") + 1
}

func TestSelectColumnExprs(t *testing.T) {
	cols := []string{"id", "title", "created_by_session"}
	available := map[string]bool{"id": true, "title": true} // missing created_by_session
	got := selectColumnExprs(cols, available)
	want := "id, title, NULL AS created_by_session"
	if got != want {
		t.Errorf("selectColumnExprs() = %q, want %q", got, want)
	}

	// All present.
	available["created_by_session"] = true
	got = selectColumnExprs(cols, available)
	want = "id, title, created_by_session"
	if got != want {
		t.Errorf("selectColumnExprs() all-present = %q, want %q", got, want)
	}

	// All missing.
	got = selectColumnExprs(cols, map[string]bool{})
	want = "NULL AS id, NULL AS title, NULL AS created_by_session"
	if got != want {
		t.Errorf("selectColumnExprs() all-missing = %q, want %q", got, want)
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
		if !strings.Contains(query, "`"+db+"`.comments") {
			t.Errorf("should reference %s.comments for comment change detection (bt-ju7o)", db)
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
	// Spin up a TCP listener on an OS-assigned port so the liveness check
	// (bt-mxz9) sees something on the recorded port. Without a real listener
	// DiscoverSharedServer rightly refuses to return success.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not start fake server: %v", err)
	}
	defer listener.Close()
	wantPort := listener.Addr().(*net.TCPAddr).Port

	// Create a temp directory structure mimicking ~/.beads/shared-server/
	tmpHome := t.TempDir()
	portDir := filepath.Join(tmpHome, ".beads", "shared-server")
	if err := os.MkdirAll(portDir, 0o755); err != nil {
		t.Fatal(err)
	}
	portContent := strconv.Itoa(wantPort) + "\n"
	if err := os.WriteFile(filepath.Join(portDir, "dolt-server.port"), []byte(portContent), 0o644); err != nil {
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
	if port != wantPort {
		t.Errorf("port = %d, want %d", port, wantPort)
	}
}

// TestDiscoverSharedServer_StalePortFile verifies bt-mxz9 Phase 1: a port
// file recording a port that nothing's listening on is rejected with a
// "no listener" error rather than blindly trusted. Without this guard, a
// dead-server-but-leftover-file state routes bt into the start-and-retry
// path, which prints misleading "Starting shared Dolt server..." noise
// even when the real failure is upstream.
func TestDiscoverSharedServer_StalePortFile(t *testing.T) {
	// Reserve a port by listening on it, capture the number, then close so
	// nothing is actually accepting. There's a small TOCTOU window — another
	// process could grab the port between Close and DialTimeout — but for a
	// unit test on localhost it's tight enough to be reliable.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not reserve port: %v", err)
	}
	stalePort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	tmpHome := t.TempDir()
	portDir := filepath.Join(tmpHome, ".beads", "shared-server")
	if err := os.MkdirAll(portDir, 0o755); err != nil {
		t.Fatal(err)
	}
	portContent := strconv.Itoa(stalePort) + "\n"
	if err := os.WriteFile(filepath.Join(portDir, "dolt-server.port"), []byte(portContent), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("BT_GLOBAL_DOLT_PORT", "")
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	_, _, err = DiscoverSharedServer()
	if err == nil {
		t.Fatal("expected error for stale port file, got nil")
	}
	if !strings.Contains(err.Error(), "no listener responded") {
		t.Errorf("error should mention no listener, got: %v", err)
	}
}

// TestDiscoverSharedServer_EnvOverrideSkipsLiveness verifies that explicit
// BT_GLOBAL_DOLT_PORT bypasses the liveness check. The user is asserting
// "this port is live" and may be pointing at a forwarded/remote port that
// won't respond to a local TCP probe (or will respond differently than a
// raw Dolt server). Trust the override.
func TestDiscoverSharedServer_EnvOverrideSkipsLiveness(t *testing.T) {
	t.Setenv("BT_GLOBAL_DOLT_PORT", "59999") // arbitrary, almost certainly nothing listening

	host, port, err := DiscoverSharedServer()
	if err != nil {
		t.Fatalf("env override should bypass liveness check, got error: %v", err)
	}
	if host != "127.0.0.1" {
		t.Errorf("host = %q, want 127.0.0.1", host)
	}
	if port != 59999 {
		t.Errorf("port = %d, want 59999", port)
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

func TestBuildIssuesQueryAsOf(t *testing.T) {
	tests := []struct {
		name   string
		dbName string
		tsStr  string
	}{
		{
			name:   "simple database",
			dbName: "myproject",
			tsStr:  "2026-01-15T00:00:00",
		},
		{
			name:   "database with hyphen",
			dbName: "my-project",
			tsStr:  "2025-06-30T12:30:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			avail := map[string]bool{}
			for _, c := range IssuesColumnList {
				avail[c] = true
			}
			query := buildIssuesQueryAsOf(tt.dbName, tt.tsStr, avail)

			// Should contain AS OF clause with timestamp
			if !strings.Contains(query, "AS OF '"+tt.tsStr+"'") {
				t.Errorf("query should contain AS OF clause, got: %s", query)
			}

			// Should reference backtick-quoted database
			if !strings.Contains(query, "`"+tt.dbName+"`") {
				t.Errorf("query should contain backtick-quoted database %q", tt.dbName)
			}

			// Should contain _global_source
			if !strings.Contains(query, "_global_source") {
				t.Error("query should contain _global_source column")
			}

			// Should use IssuesColumns
			if !strings.Contains(query, "id, title, description") {
				t.Error("query should contain columns from IssuesColumns")
			}

			// Should filter tombstone
			if !strings.Contains(query, "status != 'tombstone'") {
				t.Error("query should filter tombstone issues")
			}

			// Should NOT contain UNION ALL (single database)
			if strings.Contains(query, "UNION ALL") {
				t.Error("single-database AS OF query should not contain UNION ALL")
			}

			// Should NOT contain ORDER BY (ordering is done by caller)
			if strings.Contains(query, "ORDER BY") {
				t.Error("per-database AS OF query should not contain ORDER BY")
			}
		})
	}
}

func TestBuildIssuesQueryAsOf_QuoteEscaping(t *testing.T) {
	// Single quotes in timestamp string should be escaped to ''
	// so the payload stays inside the SQL string literal
	avail := map[string]bool{}
	for _, c := range IssuesColumnList {
		avail[c] = true
	}
	query := buildIssuesQueryAsOf("normal_db", "2026-01-01'; DROP TABLE issues; --", avail)

	// The single quote should be doubled (escaped)
	if !strings.Contains(query, "2026-01-01''; DROP TABLE issues; --") {
		t.Error("single quote in timestamp should be escaped to '' in query")
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
