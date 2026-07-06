package datasource

// Real Dolt sql-server regression test for bt-xcvxv.
//
// GetLastModified() used to run:
//
//	SELECT GREATEST(
//	    COALESCE((SELECT MAX(updated_at) FROM issues), '1970-01-01'),
//	    COALESCE((SELECT MAX(created_at) FROM comments), '1970-01-01')
//	)
//
// The '1970-01-01' string literals inside COALESCE force MySQL/Dolt to type
// the GREATEST() result as VARCHAR, so go-sql-driver/mysql returns []uint8
// even with parseTime=true (parseTime only auto-parses columns declared
// DATE/DATETIME/TIMESTAMP, not a computed VARCHAR expression). sql.NullTime.Scan
// cannot store a []uint8 into a time.Time, so every freshness poll failed -
// driving server-mode auto-refresh (doltPollOnce) into its exponential backoff
// ceiling. A mock database can't reproduce this: the whole bug is real driver
// type coercion against a real server, which is why this test spawns one.
//
// Gated behind BT_DOLT_SERVER_INTEGRATION=1 because it spawns bd and a real
// `dolt sql-server`. Run with:
//
//	BT_DOLT_SERVER_INTEGRATION=1 go test ./internal/datasource/ -run DoltServer -v

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func requireDoltServerIntegration(t *testing.T) string {
	t.Helper()
	if os.Getenv("BT_DOLT_SERVER_INTEGRATION") != "1" {
		t.Skip("set BT_DOLT_SERVER_INTEGRATION=1 to run bd/dolt-server-spawning integration tests")
	}
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("bd CLI not on PATH")
	}
	return bdPath
}

// TestGetLastModified_RealServer_ReturnsScannableTime stands up a real
// `dolt sql-server` via bd, opens a DoltReader against it exactly as bt does
// at runtime, and asserts GetLastModified() scans without error into a sane
// time.Time. Before the fix this failed on every call with "unsupported Scan,
// storing driver.Value type []uint8 into type *time.Time".
func TestGetLastModified_RealServer_ReturnsScannableTime(t *testing.T) {
	bdPath := requireDoltServerIntegration(t)

	root := t.TempDir()
	if out, err := runBdAllow(bdPath, root, 30*time.Second, "init", "--server"); err != nil {
		t.Skipf("bd init --server unavailable in this environment: %v\n%s", err, out)
	}
	beadsDir := filepath.Join(root, ".beads")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := exec.CommandContext(ctx, bdPath, "dolt", "start")
	start.Dir = root
	if out, err := start.CombinedOutput(); err != nil {
		t.Skipf("bd dolt start unavailable in this environment: %v\n%s", err, out)
	}
	defer func() {
		stop := exec.Command(bdPath, "dolt", "stop")
		stop.Dir = root
		_ = stop.Run()
	}()

	runBd(t, bdPath, root, 15*time.Second, "create", "seed issue", "-t", "task", "-p", "1")

	src, err := DiscoverSource(DiscoveryOptions{BeadsDir: beadsDir})
	if err != nil {
		t.Fatalf("DiscoverSource: %v", err)
	}
	if src.Type != SourceTypeDolt {
		t.Fatalf("DiscoverSource type = %q, want server-mode Dolt", src.Type)
	}

	reader, err := NewDoltReader(src)
	if err != nil {
		t.Fatalf("NewDoltReader: %v", err)
	}
	defer reader.Close()

	modTime, err := reader.GetLastModified()
	if err != nil {
		t.Fatalf("GetLastModified against a real Dolt server: %v (this is the bt-xcvxv VARCHAR-scan regression if the error mentions []uint8)", err)
	}
	if modTime.IsZero() {
		t.Fatalf("GetLastModified returned zero time after a real `bd create` - freshness probe did not see the issues table")
	}
	if time.Since(modTime) > time.Hour {
		t.Errorf("GetLastModified = %s, expected recent (within the last hour)", modTime)
	}

	// Comments don't bump issues.updated_at, so this exercises the
	// comment-aware half of the freshness probe (bt-ju7o): a comment-only
	// change must not regress (or fail to scan) the freshness value.
	out := runBd(t, bdPath, root, 15*time.Second, "list")
	ids := parseIDs(out)
	if len(ids) == 0 {
		t.Fatalf("no issue IDs parsed from `bd list` output:\n%s", out)
	}
	runBd(t, bdPath, root, 15*time.Second, "comments", "add", ids[0], "a comment")

	modTime2, err := reader.GetLastModified()
	if err != nil {
		t.Fatalf("GetLastModified after adding a comment: %v", err)
	}
	if modTime2.Before(modTime) {
		t.Errorf("GetLastModified after comment = %s, want >= %s (comment-only change regressed freshness)", modTime2, modTime)
	}

	// GlobalDoltReader confirmation: buildLastModifiedQuery's single-database
	// case is structurally the exact query this test just proved works - a
	// UNION ALL of MAX(updated_at)/MAX(created_at) subqueries re-aggregated
	// with an outer MAX, no string-literal sentinels. Build and run that exact
	// query (not a paraphrase) against this same real server/database to
	// confirm the global/shared reader path is NOT affected by the VARCHAR
	// defect, per the dispatcher's request to verify rather than assume.
	cfg, ok := ReadDoltConfig(beadsDir)
	if !ok {
		t.Fatalf("ReadDoltConfig: could not read metadata.json for %s", beadsDir)
	}
	globalShapeQuery, err := buildLastModifiedQuery([]string{cfg.Database})
	if err != nil {
		t.Fatalf("buildLastModifiedQuery: %v", err)
	}
	var globalModTime sql.NullTime
	if err := reader.db.QueryRow(globalShapeQuery).Scan(&globalModTime); err != nil {
		t.Fatalf("GlobalDoltReader-shaped query failed to scan: %v\nquery:\n%s", err, globalShapeQuery)
	}
	if !globalModTime.Valid {
		t.Fatalf("GlobalDoltReader-shaped query returned NULL despite committed issues/comments")
	}
	if !globalModTime.Time.Equal(modTime2) {
		t.Errorf("GlobalDoltReader-shaped query time = %s, want %s (same underlying data, same query shape)", globalModTime.Time, modTime2)
	}
}

// runBdAllow runs a bd subcommand without failing the test on error, so the
// caller can decide to skip (used for optional server-mode setup, mirroring
// pkg/ui/claim_integration_test.go's runBdUIAllow).
func runBdAllow(bdPath, dir string, timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bdPath, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BD_NON_INTERACTIVE=1")
	return cmd.CombinedOutput()
}
