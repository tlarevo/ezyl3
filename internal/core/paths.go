package core

import (
	"errors"
	"path/filepath"
	"strings"
)

type ProfilePaths struct {
	Profile    string
	AppConfig  string
	ProfileDir string
	LogsDir    string
	CacheDir   string
	HomeDir    string
}

func ResolvePaths(env map[string]string, profile string) (ProfilePaths, error) {
	home := envOrDefault(env, "HOME", "")
	if home == "" {
		return ProfilePaths{}, errors.New("HOME is required")
	}
	profile = strings.TrimSpace(profile)
	if profile == "" {
		profile = "default"
	}
	configHome := envOrDefault(env, "XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	dataHome := envOrDefault(env, "XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	stateHome := envOrDefault(env, "XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	cacheHome := envOrDefault(env, "XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	return ProfilePaths{
		Profile:    profile,
		HomeDir:    home,
		AppConfig:  filepath.Join(configHome, "ezyl3", "config.yaml"),
		ProfileDir: filepath.Join(dataHome, "ezyl3", "profiles", profile),
		LogsDir:    filepath.Join(stateHome, "ezyl3", "profiles", profile, "logs"),
		CacheDir:   filepath.Join(cacheHome, "ezyl3"),
	}, nil
}

func (p ProfilePaths) LaunchAgentPath(kind string) string {
	return filepath.Join(p.HomeDir, "Library", "LaunchAgents", p.LaunchAgentLabel(kind)+".plist")
}

func (p ProfilePaths) LaunchAgentLabel(kind string) string {
	return "com.ezyl3." + p.Profile + "." + kind
}

func envOrDefault(env map[string]string, key, fallback string) string {
	if v, ok := env[key]; ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}
