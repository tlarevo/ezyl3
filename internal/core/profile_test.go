package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportExternalProfileWritesMetadataWithoutMutatingSource(t *testing.T) {
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
	if _, err := os.Stat(filepath.Join(source, "metadata.json")); !os.IsNotExist(err) {
		t.Fatalf("source was mutated; metadata exists or stat errored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(profile.Paths.ProfileDir, "metadata.json")); err != nil {
		t.Fatalf("metadata was not written in managed profile dir: %v", err)
	}
}
