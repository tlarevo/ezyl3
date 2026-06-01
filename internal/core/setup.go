package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type SetupOptions struct {
	Paths          ProfilePaths
	Domain         string
	ExecutablePath string
	Secrets        Secrets
	Force          bool
	SkipPythonDeps bool
}

type SetupDependencies struct {
	Installer         PythonInstaller
	LaunchAgentWriter LaunchAgentWriter
}

type PythonInstaller interface {
	InstallPythonDeps(paths ProfilePaths) error
}

type LaunchAgentWriter interface {
	WriteLaunchAgents(paths ProfilePaths, domain string, port int, executablePath string) error
}

type SetupResult struct {
	Profile            Profile
	Domain             string
	BaseURL            string
	LaunchAgentDir     string
	PythonInstalled    bool
	MasterKeyGenerated bool
	Secrets            Secrets
}

type DefaultPythonInstaller struct{}

func (DefaultPythonInstaller) InstallPythonDeps(paths ProfilePaths) error {
	return InstallPythonDeps(paths)
}

type DefaultLaunchAgentWriter struct{}

func (DefaultLaunchAgentWriter) WriteLaunchAgents(paths ProfilePaths, domain string, port int, executablePath string) error {
	return WriteLaunchAgents(paths, domain, port, executablePath)
}

func RunSetup(opts SetupOptions, deps SetupDependencies) (SetupResult, error) {
	if strings.TrimSpace(opts.Paths.ProfileDir) == "" {
		return SetupResult{}, fmt.Errorf("profile paths are required")
	}
	domain := ""
	var err error
	if strings.TrimSpace(opts.Domain) != "" {
		domain, err = NormalizeNgrokDomain(opts.Domain)
		if err != nil {
			return SetupResult{}, err
		}
	}
	if _, err := os.Stat(opts.Paths.ProfileDir); err == nil && !opts.Force {
		return SetupResult{}, fmt.Errorf("managed profile %q already exists; rerun with --force to overwrite", opts.Paths.Profile)
	} else if err != nil && !os.IsNotExist(err) {
		return SetupResult{}, err
	}
	if opts.Force {
		if err := os.RemoveAll(opts.Paths.ProfileDir); err != nil {
			return SetupResult{}, err
		}
	}
	secrets := opts.Secrets
	masterKeyGenerated := false
	if strings.TrimSpace(secrets.LiteLLMMasterKey) == "" {
		secrets.LiteLLMMasterKey, err = GenerateMasterKey()
		if err != nil {
			return SetupResult{}, err
		}
		masterKeyGenerated = true
	}

	profile, err := CreateManagedProfile(opts.Paths, secrets, domain)
	if err != nil {
		return SetupResult{}, err
	}
	executablePath := strings.TrimSpace(opts.ExecutablePath)
	if executablePath == "" {
		executablePath, err = os.Executable()
		if err != nil {
			return SetupResult{}, fmt.Errorf("resolve ezyl3 executable path: %w", err)
		}
	}
	writer := deps.LaunchAgentWriter
	if writer == nil {
		writer = DefaultLaunchAgentWriter{}
	}
	if err := writer.WriteLaunchAgents(opts.Paths, domain, profile.Port, executablePath); err != nil {
		return SetupResult{}, err
	}
	installed := false
	if !opts.SkipPythonDeps {
		installer := deps.Installer
		if installer == nil {
			installer = DefaultPythonInstaller{}
		}
		if err := installer.InstallPythonDeps(profile.Paths); err != nil {
			return SetupResult{}, err
		}
		installed = true
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d/v1", profile.Port)
	if domain != "" {
		baseURL = "https://" + domain + "/v1"
	}
	return SetupResult{
		Profile:            profile,
		Domain:             domain,
		BaseURL:            baseURL,
		LaunchAgentDir:     filepath.Dir(opts.Paths.LaunchAgentPath("litellm")),
		PythonInstalled:    installed,
		MasterKeyGenerated: masterKeyGenerated,
		Secrets:            secrets,
	}, nil
}

func (r SetupResult) Summary() string {
	tunnel := "local-only"
	if r.Domain != "" {
		tunnel = "ngrok: " + r.Domain
	}
	python := "skipped"
	if r.PythonInstalled {
		python = "installed"
	}
	summary := fmt.Sprintf(`Created managed profile %s
Runtime: %s
Logs: %s
LaunchAgents: %s
Tunnel: %s
Python dependencies: %s
Base URL: %s
Models: litellm-auto, litellm-simple, litellm-medium, litellm-complex, litellm-reasoning
LiteLLM master key: %s
HF token: %s
Ollama API key: %s
`, r.Profile.Name, r.Profile.RuntimeDir, r.Profile.Paths.LogsDir, r.LaunchAgentDir, tunnel, python, r.BaseURL, presence(r.Secrets.LiteLLMMasterKey), presence(r.Secrets.HFToken), presence(r.Secrets.OllamaAPIKey))
	if r.MasterKeyGenerated {
		summary += "\nNOTE: A new LiteLLM master key was generated. If Cursor was already\n" +
			"configured, update its API key with:\n" +
			"  ezyl3 cursor settings --reveal-key\n"
	}
	return summary
}
