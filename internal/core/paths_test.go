package core

import (
	"path/filepath"
	"testing"
)

func TestResolvePathsUsesXDGDirectories(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{
		"HOME":            home,
		"XDG_CONFIG_HOME": filepath.Join(home, "cfg"),
		"XDG_DATA_HOME":   filepath.Join(home, "data"),
		"XDG_STATE_HOME":  filepath.Join(home, "state"),
		"XDG_CACHE_HOME":  filepath.Join(home, "cache"),
	}

	paths, err := ResolvePaths(env, "default")
	if err != nil {
		t.Fatalf("ResolvePaths returned error: %v", err)
	}

	wantProfile := filepath.Join(home, "data", "ezyl3", "profiles", "default")
	if paths.AppConfig != filepath.Join(home, "cfg", "ezyl3", "config.yaml") {
		t.Fatalf("AppConfig = %q", paths.AppConfig)
	}
	if paths.ProfileDir != wantProfile {
		t.Fatalf("ProfileDir = %q", paths.ProfileDir)
	}
	if paths.LogsDir != filepath.Join(home, "state", "ezyl3", "profiles", "default", "logs") {
		t.Fatalf("LogsDir = %q", paths.LogsDir)
	}
	if paths.CacheDir != filepath.Join(home, "cache", "ezyl3") {
		t.Fatalf("CacheDir = %q", paths.CacheDir)
	}
	if paths.LaunchAgentPath("litellm") != filepath.Join(home, "Library", "LaunchAgents", "com.ezyl3.default.litellm.plist") {
		t.Fatalf("unexpected litellm LaunchAgent path: %q", paths.LaunchAgentPath("litellm"))
	}
}

func TestResolvePathsUsesDefaultsWhenXDGUnset(t *testing.T) {
	home := t.TempDir()

	paths, err := ResolvePaths(map[string]string{"HOME": home}, "current")
	if err != nil {
		t.Fatalf("ResolvePaths returned error: %v", err)
	}

	if paths.ProfileDir != filepath.Join(home, ".local", "share", "ezyl3", "profiles", "current") {
		t.Fatalf("ProfileDir = %q", paths.ProfileDir)
	}
	if paths.LogsDir != filepath.Join(home, ".local", "state", "ezyl3", "profiles", "current", "logs") {
		t.Fatalf("LogsDir = %q", paths.LogsDir)
	}
}
