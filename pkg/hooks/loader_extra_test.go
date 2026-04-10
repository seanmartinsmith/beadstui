package hooks

import "testing"

func TestLoaderConfigAndWarnings(t *testing.T) {
	loader := NewLoader()
	cfg := loader.Config()
	if cfg == nil {
		t.Fatalf("expected non-nil config even before load")
	}
	if loader.HasHooks() {
		t.Fatalf("no hooks should be present before load")
	}
	if len(loader.GetHooks(PreExport)) != 0 {
		t.Fatalf("expected no pre-export hooks")
	}
	if len(loader.Warnings()) != 0 {
		t.Fatalf("expected no warnings before load")
	}
}

func TestTruncateHelper(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz"
	short := truncate(long, 10)
	if len(short) != 10 {
		t.Fatalf("truncate should limit length to 10, got %d", len(short))
	}
}
