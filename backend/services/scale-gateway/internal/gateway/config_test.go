package gateway

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoadConfigValid(t *testing.T) {
	path := writeTempConfig(t, `{
		"listen_address": ":8080",
		"scales": [
			{"id": "scale-1", "kind": "dialog_raw_tcp", "dialog_variant": "02", "address": "192.168.1.50:9999"},
			{"id": "scale-2", "kind": "dialog_raw_tcp", "dialog_variant": "04", "address": "192.168.1.51:9999"}
		]
	}`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.ListenAddress != ":8080" {
		t.Fatalf("ListenAddress = %q, want %q", cfg.ListenAddress, ":8080")
	}
	if len(cfg.Scales) != 2 {
		t.Fatalf("got %d scales, want 2", len(cfg.Scales))
	}
}

func TestLoadConfigMissingListenAddress(t *testing.T) {
	path := writeTempConfig(t, `{"scales": []}`)
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected error for missing listen_address, got nil")
	}
}

func TestLoadConfigDuplicateScaleID(t *testing.T) {
	path := writeTempConfig(t, `{
		"listen_address": ":8080",
		"scales": [
			{"id": "scale-1", "kind": "dialog_raw_tcp", "dialog_variant": "02", "address": "a:1"},
			{"id": "scale-1", "kind": "dialog_raw_tcp", "dialog_variant": "02", "address": "b:2"}
		]
	}`)
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected error for duplicate scale id, got nil")
	}
}

func TestLoadConfigMissingAddressForDialogRawTCP(t *testing.T) {
	path := writeTempConfig(t, `{
		"listen_address": ":8080",
		"scales": [{"id": "scale-1", "kind": "dialog_raw_tcp", "dialog_variant": "02"}]
	}`)
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected error for missing address, got nil")
	}
}

func TestLoadConfigInvalidDialogVariant(t *testing.T) {
	path := writeTempConfig(t, `{
		"listen_address": ":8080",
		"scales": [{"id": "scale-1", "kind": "dialog_raw_tcp", "dialog_variant": "99", "address": "a:1"}]
	}`)
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected error for invalid dialog_variant, got nil")
	}
}

func TestLoadConfigUnknownKind(t *testing.T) {
	path := writeTempConfig(t, `{
		"listen_address": ":8080",
		"scales": [{"id": "scale-1", "kind": "carrier-pigeon", "address": "a:1"}]
	}`)
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected error for unknown kind, got nil")
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadConfigMalformedJSON(t *testing.T) {
	path := writeTempConfig(t, `{ this is not json`)
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestBuildEntries(t *testing.T) {
	cfg := Config{
		ListenAddress: ":8080",
		Scales: []ScaleConfig{
			{ID: "scale-1", Kind: "dialog_raw_tcp", DialogVariant: "02", Address: "127.0.0.1:0"},
		},
	}
	entries, err := BuildEntries(cfg)
	if err != nil {
		t.Fatalf("BuildEntries returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "scale-1" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}
