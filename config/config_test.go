package config

import "testing"

func TestGetAssetVersionFallsBackToVersion(t *testing.T) {
	old := AssetVersion
	AssetVersion = ""
	defer func() {
		AssetVersion = old
	}()

	if got, want := GetAssetVersion(), GetVersion(); got != want {
		t.Fatalf("GetAssetVersion() = %q, want %q", got, want)
	}
}

func TestGetAssetVersionUsesOverride(t *testing.T) {
	old := AssetVersion
	AssetVersion = "test-build-123"
	defer func() {
		AssetVersion = old
	}()

	if got, want := GetAssetVersion(), "test-build-123"; got != want {
		t.Fatalf("GetAssetVersion() = %q, want %q", got, want)
	}
}
