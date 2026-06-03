package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	ProfileModeManaged  = "managed"
	ProfileModeExternal = "external"
	ProfileFileName     = "profile.json"
	LegacyMetadataFile  = "metadata.json"

	// Exposure describes how the proxy is reached by a client (e.g. Cursor).
	ExposureLocal  = "local"  // 127.0.0.1 only; not usable with Cursor
	ExposureTunnel = "tunnel" // ezyl3 runs a tunnel (ngrok) to a public URL
	ExposureDirect = "direct" // user-owned public HTTPS endpoint; ezyl3 only records it
)

type Profile struct {
	Name           string       `json:"name"`
	Mode           string       `json:"mode"`
	RuntimeDir     string       `json:"runtime_dir"`
	LogsDir        string       `json:"logs_dir,omitempty"`
	Port           int          `json:"port"`
	TunnelProvider string       `json:"tunnel_provider"`
	Domain         string       `json:"domain,omitempty"`
	ExposureMode   string       `json:"exposure_mode,omitempty"`
	PublicURL      string       `json:"public_url,omitempty"`
	Paths          ProfilePaths `json:"-"`
}

// Exposure returns the profile's exposure mode, inferring it for legacy profiles
// written before the field existed: a recorded ngrok domain implies tunnel,
// otherwise local.
func (p Profile) Exposure() string {
	if p.ExposureMode != "" {
		return p.ExposureMode
	}
	if p.PublicURL != "" {
		return ExposureDirect
	}
	if p.Domain != "" {
		return ExposureTunnel
	}
	return ExposureLocal
}

type ProfileManager struct {
	env map[string]string
}

func NewProfileManager(env map[string]string) ProfileManager {
	return ProfileManager{env: env}
}

func (pm ProfileManager) ImportExternal(name, source string) (Profile, error) {
	if source == "" {
		return Profile{}, errors.New("source path is required")
	}
	abs, err := filepath.Abs(source)
	if err != nil {
		return Profile{}, err
	}
	if _, err := os.Stat(filepath.Join(abs, "config.yaml")); err != nil {
		return Profile{}, err
	}
	paths, err := ResolvePaths(pm.env, name)
	if err != nil {
		return Profile{}, err
	}
	if err := os.MkdirAll(paths.ProfileDir, 0o755); err != nil {
		return Profile{}, err
	}
	profile := Profile{
		Name:           paths.Profile,
		Mode:           ProfileModeExternal,
		RuntimeDir:     abs,
		Port:           4400,
		TunnelProvider: "ngrok",
		Paths:          paths,
	}
	if err := WriteProfileFile(paths.ProfileDir, profile); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func WriteProfileFile(runtimeDir string, profile Profile) error {
	return writeJSON(filepath.Join(runtimeDir, ProfileFileName), profile, 0o644)
}

func LoadProfileFromRuntime(runtimeDir string) (Profile, error) {
	data, err := os.ReadFile(filepath.Join(runtimeDir, ProfileFileName))
	if err != nil {
		if !os.IsNotExist(err) {
			return Profile{}, err
		}
		data, err = os.ReadFile(filepath.Join(runtimeDir, LegacyMetadataFile))
		if err != nil {
			return Profile{}, err
		}
	}
	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func writeJSON(path string, value any, mode os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, mode)
}
