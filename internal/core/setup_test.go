package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeInstaller struct {
	paths []ProfilePaths
}

func (f *fakeInstaller) InstallPythonDeps(paths ProfilePaths) error {
	f.paths = append(f.paths, paths)
	return nil
}

type fakeLaunchAgentWriter struct {
	calls []launchAgentCall
}

type launchAgentCall struct {
	domain         string
	port           int
	executablePath string
}

func (f *fakeLaunchAgentWriter) WriteLaunchAgents(paths ProfilePaths, domain string, port int, executablePath string) error {
	f.calls = append(f.calls, launchAgentCall{domain: domain, port: port, executablePath: executablePath})
	return nil
}

func TestRunSetupCreatesLocalOnlyProfileAndRedactsSecrets(t *testing.T) {
	paths := setupTestPaths(t, "default")
	installer := &fakeInstaller{}
	writer := &fakeLaunchAgentWriter{}

	result, err := RunSetup(SetupOptions{
		Paths:          paths,
		ExecutablePath: "/usr/local/bin/ezyl3",
		Secrets: Secrets{
			HFToken:      "hf_secret",
			HFBillTo:     "billing-org",
			OllamaAPIKey: "ollama_secret",
		},
	}, SetupDependencies{Installer: installer, LaunchAgentWriter: writer})
	if err != nil {
		t.Fatalf("RunSetup returned error: %v", err)
	}

	if result.Profile.Mode != ProfileModeManaged {
		t.Fatalf("profile mode = %q", result.Profile.Mode)
	}
	if result.Domain != "" {
		t.Fatalf("domain = %q, want local-only", result.Domain)
	}
	if len(writer.calls) != 1 || writer.calls[0].domain != "" || writer.calls[0].port != 4400 || writer.calls[0].executablePath != "/usr/local/bin/ezyl3" {
		t.Fatalf("launch writer calls = %#v", writer.calls)
	}
	if len(installer.paths) != 1 || installer.paths[0] != paths {
		t.Fatalf("installer paths = %#v", installer.paths)
	}
	for _, path := range []string{"config.yaml", ".env", "profile.json", UsageDBFileName, "ezyl3_usage_callback.py"} {
		if _, err := os.Stat(filepath.Join(paths.ProfileDir, path)); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	cfg, err := LoadLiteLLMConfig(filepath.Join(paths.ProfileDir, "config.yaml"))
	if err != nil {
		t.Fatalf("LoadLiteLLMConfig returned error: %v", err)
	}
	if got := fmt.Sprint(cfg.LiteLLMSettings.Extra["callbacks"]); !strings.Contains(got, "ezyl3_usage_callback.proxy_handler_instance") {
		t.Fatalf("config callbacks = %v, want ezyl3 usage callback", got)
	}
	callback, err := os.ReadFile(filepath.Join(paths.ProfileDir, "ezyl3_usage_callback.py"))
	if err != nil {
		t.Fatalf("expected callback file to be readable: %v", err)
	}
	callbackText := string(callback)
	for _, want := range []string{"CustomLogger", "async_log_success_event", "sqlite3", "EZYL3_USAGE_DB"} {
		if !strings.Contains(callbackText, want) {
			t.Fatalf("callback file missing %q:\n%s", want, callbackText)
		}
	}
	for _, forbidden := range []string{"api_key", "completion_response", "messages"} {
		if strings.Contains(callbackText, forbidden) {
			t.Fatalf("callback file should not reference sensitive payload field %q:\n%s", forbidden, callbackText)
		}
	}
	if _, err := os.Stat(filepath.Join(paths.ProfileDir, "run-proxy.sh")); !os.IsNotExist(err) {
		t.Fatalf("run-proxy.sh should not be generated: %v", err)
	}
	secrets, err := ReadSecrets(filepath.Join(paths.ProfileDir, ".env"))
	if err != nil {
		t.Fatalf("ReadSecrets returned error: %v", err)
	}
	if !strings.HasPrefix(secrets.LiteLLMMasterKey, "sk-cursor-") {
		t.Fatalf("generated master key = %q", secrets.LiteLLMMasterKey)
	}
	summary := result.Summary()
	for _, leaked := range []string{"hf_secret", "ollama_secret", secrets.LiteLLMMasterKey} {
		if strings.Contains(summary, leaked) {
			t.Fatalf("summary leaked %q:\n%s", leaked, summary)
		}
	}
	for _, want := range []string{paths.ProfileDir, paths.LogsDir, "Base URL: http://127.0.0.1:4400/v1", "LiteLLM master key: set"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q:\n%s", want, summary)
		}
	}
	if !result.MasterKeyGenerated {
		t.Fatalf("expected MasterKeyGenerated to be true when no key supplied")
	}
	if !strings.Contains(summary, "new LiteLLM master key was generated") || !strings.Contains(summary, "--reveal-key") {
		t.Fatalf("summary should warn about regenerated key and point to reveal:\n%s", summary)
	}
}

func TestRunSetupKeepsSuppliedMasterKeyAndOmitsRegenWarning(t *testing.T) {
	paths := setupTestPaths(t, "default")
	result, err := RunSetup(SetupOptions{
		Paths:          paths,
		ExecutablePath: "/usr/local/bin/ezyl3",
		Secrets:        Secrets{LiteLLMMasterKey: "sk-cursor-supplied"},
		SkipPythonDeps: true,
	}, SetupDependencies{Installer: &fakeInstaller{}, LaunchAgentWriter: &fakeLaunchAgentWriter{}})
	if err != nil {
		t.Fatalf("RunSetup returned error: %v", err)
	}
	if result.MasterKeyGenerated {
		t.Fatalf("MasterKeyGenerated should be false when a key is supplied")
	}
	if strings.Contains(result.Summary(), "new LiteLLM master key was generated") {
		t.Fatalf("summary should not warn when key was supplied:\n%s", result.Summary())
	}
}

func TestRunSetupRefusesOverwriteWithoutForce(t *testing.T) {
	paths := setupTestPaths(t, "default")
	if err := os.MkdirAll(paths.ProfileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.ProfileDir, "config.yaml"), []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := RunSetup(SetupOptions{Paths: paths, SkipPythonDeps: true}, SetupDependencies{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("RunSetup error = %v, want already exists", err)
	}
}

func TestRunSetupNormalizesNgrokDomainBeforeWriting(t *testing.T) {
	paths := setupTestPaths(t, "default")
	writer := &fakeLaunchAgentWriter{}

	result, err := RunSetup(SetupOptions{
		Paths:          paths,
		Domain:         "https://Example.ngrok-free.dev/",
		ExecutablePath: "/usr/local/bin/ezyl3",
		SkipPythonDeps: true,
	}, SetupDependencies{LaunchAgentWriter: writer, NgrokChecker: &fakeNgrokChecker{}})
	if err != nil {
		t.Fatalf("RunSetup returned error: %v", err)
	}

	if result.Domain != "example.ngrok-free.dev" {
		t.Fatalf("domain = %q", result.Domain)
	}
	if len(writer.calls) != 1 || writer.calls[0].domain != "example.ngrok-free.dev" {
		t.Fatalf("launch writer calls = %#v", writer.calls)
	}
}

type fakeNgrokChecker struct {
	err    error
	called bool
}

func (f *fakeNgrokChecker) CheckNgrokReady() error { f.called = true; return f.err }

func TestRunSetupPublicURLCreatesDirectProfileWithoutNgrok(t *testing.T) {
	paths := setupTestPaths(t, "default")
	writer := &fakeLaunchAgentWriter{}
	checker := &fakeNgrokChecker{err: errors.New("ngrok should not be checked for direct")}

	result, err := RunSetup(SetupOptions{
		Paths:          paths,
		PublicURL:      "https://llm.example.com",
		ExecutablePath: "/usr/local/bin/ezyl3",
		SkipPythonDeps: true,
	}, SetupDependencies{Installer: &fakeInstaller{}, LaunchAgentWriter: writer, NgrokChecker: checker})
	if err != nil {
		t.Fatalf("RunSetup returned error: %v", err)
	}
	if checker.called {
		t.Fatalf("ngrok checker must not run for a direct (--public-url) profile")
	}
	if result.Profile.Exposure() != ExposureDirect || result.Profile.PublicURL != "https://llm.example.com" {
		t.Fatalf("profile = %#v, want direct with public URL", result.Profile)
	}
	if result.BaseURL != "https://llm.example.com/v1" {
		t.Fatalf("BaseURL = %q, want direct public URL", result.BaseURL)
	}
	// Direct profiles do not write an ngrok LaunchAgent.
	if len(writer.calls) != 1 || writer.calls[0].domain != "" {
		t.Fatalf("launch writer calls = %#v, want one call with empty domain", writer.calls)
	}
}

func TestRunSetupRejectsBothDomainAndPublicURL(t *testing.T) {
	paths := setupTestPaths(t, "default")
	_, err := RunSetup(SetupOptions{
		Paths:          paths,
		Domain:         "example.ngrok-free.dev",
		PublicURL:      "https://llm.example.com",
		ExecutablePath: "/usr/local/bin/ezyl3",
		SkipPythonDeps: true,
	}, SetupDependencies{Installer: &fakeInstaller{}, LaunchAgentWriter: &fakeLaunchAgentWriter{}, NgrokChecker: &fakeNgrokChecker{}})
	if err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("RunSetup error = %v, want rejection of domain+public-url", err)
	}
	if _, statErr := os.Stat(paths.ProfileDir); !os.IsNotExist(statErr) {
		t.Fatalf("profile dir created despite conflicting options: %v", statErr)
	}
}

func TestRunSetupRejectsInvalidPublicURL(t *testing.T) {
	paths := setupTestPaths(t, "default")
	_, err := RunSetup(SetupOptions{
		Paths:          paths,
		PublicURL:      "http://insecure.example.com",
		ExecutablePath: "/usr/local/bin/ezyl3",
		SkipPythonDeps: true,
	}, SetupDependencies{Installer: &fakeInstaller{}, LaunchAgentWriter: &fakeLaunchAgentWriter{}, NgrokChecker: &fakeNgrokChecker{}})
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("RunSetup error = %v, want https validation failure", err)
	}
}

func TestRunSetupFailsFastWhenNgrokNotReadyForTunnel(t *testing.T) {
	paths := setupTestPaths(t, "default")
	writer := &fakeLaunchAgentWriter{}

	_, err := RunSetup(SetupOptions{
		Paths:          paths,
		Domain:         "example.ngrok-free.dev",
		ExecutablePath: "/usr/local/bin/ezyl3",
		SkipPythonDeps: true,
	}, SetupDependencies{
		LaunchAgentWriter: writer,
		NgrokChecker:      &fakeNgrokChecker{err: errors.New("ngrok binary not found on PATH")},
	})
	if err == nil || !strings.Contains(err.Error(), "ngrok binary not found") {
		t.Fatalf("RunSetup error = %v, want ngrok-not-ready", err)
	}
	// Nothing should be written when the preflight fails.
	if len(writer.calls) != 0 {
		t.Fatalf("launch writer called despite ngrok preflight failure: %#v", writer.calls)
	}
	if _, statErr := os.Stat(paths.ProfileDir); !os.IsNotExist(statErr) {
		t.Fatalf("profile dir created despite ngrok preflight failure: %v", statErr)
	}
}

func TestRunSetupSkipsNgrokPreflightForLocalOnlyProfile(t *testing.T) {
	paths := setupTestPaths(t, "default")
	writer := &fakeLaunchAgentWriter{}

	// A failing checker must NOT block a local-only (no domain) setup, because
	// local-only profiles never use ngrok.
	_, err := RunSetup(SetupOptions{
		Paths:          paths,
		ExecutablePath: "/usr/local/bin/ezyl3",
		SkipPythonDeps: true,
	}, SetupDependencies{
		LaunchAgentWriter: writer,
		NgrokChecker:      &fakeNgrokChecker{err: errors.New("ngrok not installed")},
	})
	if err != nil {
		t.Fatalf("local-only setup should not run ngrok preflight: %v", err)
	}
}

func TestRunSetupRejectsInvalidNgrokDomainBeforeWriting(t *testing.T) {
	paths := setupTestPaths(t, "default")
	writer := &fakeLaunchAgentWriter{}

	_, err := RunSetup(SetupOptions{
		Paths:          paths,
		Domain:         "https://example.com/",
		ExecutablePath: "/usr/local/bin/ezyl3",
		SkipPythonDeps: true,
	}, SetupDependencies{LaunchAgentWriter: writer})
	if err == nil || !strings.Contains(err.Error(), "invalid ngrok") {
		t.Fatalf("RunSetup error = %v, want invalid ngrok", err)
	}
	if len(writer.calls) != 0 {
		t.Fatalf("launch writer was called before validation: %#v", writer.calls)
	}
	if _, statErr := os.Stat(paths.ProfileDir); !os.IsNotExist(statErr) {
		t.Fatalf("profile dir exists after validation failure: %v", statErr)
	}
}

func setupTestPaths(t *testing.T, profile string) ProfilePaths {
	t.Helper()
	home := t.TempDir()
	paths, err := ResolvePaths(map[string]string{"HOME": home}, profile)
	if err != nil {
		t.Fatalf("ResolvePaths returned error: %v", err)
	}
	return paths
}
