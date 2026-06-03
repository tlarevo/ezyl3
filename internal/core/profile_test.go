package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportExternalProfileWritesProfileJSONWithoutMutatingSource(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(home, "litellm-cursor")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	sourceConfig := filepath.Join(source, "config.yaml")
	if err := os.WriteFile(sourceConfig, []byte("model_list: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pm := NewProfileManager(map[string]string{"HOME": home})
	profile, err := pm.ImportExternal("current", source)
	if err != nil {
		t.Fatalf("ImportExternal returned error: %v", err)
	}

	if profile.Mode != ProfileModeExternal {
		t.Fatalf("Mode = %q", profile.Mode)
	}
	if profile.RuntimeDir != source {
		t.Fatalf("RuntimeDir = %q", profile.RuntimeDir)
	}
	if _, err := os.Stat(filepath.Join(source, "profile.json")); !os.IsNotExist(err) {
		t.Fatalf("source was mutated; profile.json exists or stat errored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(profile.Paths.ProfileDir, "profile.json")); err != nil {
		t.Fatalf("profile.json was not written in managed profile dir: %v", err)
	}
}

func TestLoadProfileFromRuntimeFallsBackToLegacyMetadata(t *testing.T) {
	runtime := t.TempDir()
	legacy := Profile{
		Name:           "legacy",
		Mode:           ProfileModeManaged,
		RuntimeDir:     runtime,
		Port:           4400,
		TunnelProvider: "ngrok",
		Domain:         "legacy.ngrok-free.dev",
	}
	if err := writeJSON(filepath.Join(runtime, "metadata.json"), legacy, 0o644); err != nil {
		t.Fatal(err)
	}

	profile, err := LoadProfileFromRuntime(runtime)
	if err != nil {
		t.Fatalf("LoadProfileFromRuntime returned error: %v", err)
	}
	if profile.Name != "legacy" || profile.Domain != "legacy.ngrok-free.dev" {
		t.Fatalf("profile = %#v", profile)
	}
}

func TestProfileExposureStoredAndInferred(t *testing.T) {
	// Explicit mode wins.
	if got := (Profile{ExposureMode: ExposureDirect}).Exposure(); got != ExposureDirect {
		t.Fatalf("explicit exposure = %q, want direct", got)
	}
	// Legacy profile with a domain but no exposure mode infers tunnel.
	if got := (Profile{Domain: "demo.ngrok-free.dev"}).Exposure(); got != ExposureTunnel {
		t.Fatalf("legacy domain exposure = %q, want tunnel", got)
	}
	// Legacy profile with neither infers local.
	if got := (Profile{}).Exposure(); got != ExposureLocal {
		t.Fatalf("empty exposure = %q, want local", got)
	}
}
