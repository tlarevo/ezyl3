package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeInstaller struct {
	dirs []string
}

func (f *fakeInstaller) InstallPythonDeps(runtimeDir string) error {
	f.dirs = append(f.dirs, runtimeDir)
	return nil
}

type fakeLaunchAgentWriter struct {
	calls []launchAgentCall
}

type launchAgentCall struct {
	domain string
	port   int
}

func (f *fakeLaunchAgentWriter) WriteLaunchAgents(paths ProfilePaths, domain string, port int) error {
	f.calls = append(f.calls, launchAgentCall{domain: domain, port: port})
	return nil
}

func TestRunSetupCreatesLocalOnlyProfileAndRedactsSecrets(t *testing.T) {
	paths := setupTestPaths(t, "default")
	installer := &fakeInstaller{}
	writer := &fakeLaunchAgentWriter{}

	result, err := RunSetup(SetupOptions{
		Paths: paths,
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
	if len(writer.calls) != 1 || writer.calls[0].domain != "" || writer.calls[0].port != 4400 {
		t.Fatalf("launch writer calls = %#v", writer.calls)
	}
	if len(installer.dirs) != 1 || installer.dirs[0] != paths.ProfileDir {
		t.Fatalf("installer dirs = %#v", installer.dirs)
	}
	for _, path := range []string{"config.yaml", ".env", "run-proxy.sh", "profile.json"} {
		if _, err := os.Stat(filepath.Join(paths.ProfileDir, path)); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
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
		SkipPythonDeps: true,
	}, SetupDependencies{LaunchAgentWriter: writer})
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

func TestRunSetupRejectsInvalidNgrokDomainBeforeWriting(t *testing.T) {
	paths := setupTestPaths(t, "default")
	writer := &fakeLaunchAgentWriter{}

	_, err := RunSetup(SetupOptions{
		Paths:          paths,
		Domain:         "https://example.com/",
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
