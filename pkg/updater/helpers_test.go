package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRelease_FindPlatformAsset(t *testing.T) {
	rel := &Release{TagName: "v1.2.3"}
	target := getAssetName(rel.TagName)
	rel.Assets = []Asset{
		{Name: "other.tar.gz"},
		{Name: target, BrowserDownloadURL: "http://example.com/bv.tgz"},
	}

	asset := rel.FindPlatformAsset()
	if asset == nil {
		t.Fatalf("expected platform asset %q", target)
	}
	if asset.Name != target {
		t.Fatalf("expected %q, got %q", target, asset.Name)
	}
}

func TestRelease_FindChecksumAsset(t *testing.T) {
	rel := &Release{
		Assets: []Asset{
			{Name: "bv_v1.0.0_darwin_arm64.tar.gz"},
			{Name: "checksums.txt", BrowserDownloadURL: "http://example.com/checksums"},
		},
	}
	asset := rel.FindChecksumAsset()
	if asset == nil || asset.Name != "checksums.txt" {
		t.Fatalf("expected checksums.txt asset, got %#v", asset)
	}
}

func TestGetAssetName_UsesRuntimeAndTrimsV(t *testing.T) {
	name := getAssetName("v9.8.7")
	want := "bv_9.8.7_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	if name != want {
		t.Fatalf("getAssetName mismatch: got %q want %q", name, want)
	}
}

func TestParseChecksums(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "checksums.txt")

	content := "" +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  bv_1.0.0_darwin_arm64.tar.gz\n" +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb  checksums.txt\n" +
		"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}

	m, err := parseChecksums(path)
	if err != nil {
		t.Fatalf("parseChecksums failed: %v", err)
	}
	if got := m["bv_1.0.0_darwin_arm64.tar.gz"]; got != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("unexpected checksum for archive: %q", got)
	}
	if got := m["checksums.txt"]; got != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("unexpected checksum for checksums.txt: %q", got)
	}
}

func TestParseChecksums_FilenamesWithSpaces(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "checksums.txt")

	content := "" +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  bv 1.0.0 windows amd64.tar.gz\n" +
		"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}

	m, err := parseChecksums(path)
	if err != nil {
		t.Fatalf("parseChecksums failed: %v", err)
	}
	if got := m["bv 1.0.0 windows amd64.tar.gz"]; got != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("unexpected checksum for spaced filename: %q", got)
	}
}

func TestVerifyChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "file.bin")
	data := []byte("hello updater")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	sum := sha256.Sum256(data)
	okHash := hex.EncodeToString(sum[:])

	if err := verifyChecksum(path, okHash); err != nil {
		t.Fatalf("verifyChecksum expected ok, got %v", err)
	}
	if err := verifyChecksum(path, "deadbeef"); err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
}
