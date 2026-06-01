package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorMissingRuntimeUsesActionableDetailsWithoutPathNoise(t *testing.T) {
	runtime := NewRuntime(filepath.Join(t.TempDir(), "missing-runtime"))

	report := Doctor(runtime)

	for _, check := range report.Checks {
		if check.Name == "config.yaml" {
			if strings.Contains(check.Detail, runtime.Path) {
				t.Fatalf("doctor detail includes runtime path noise: %q", check.Detail)
			}
			if !strings.Contains(check.Detail, "run ezyl3 setup") {
				t.Fatalf("doctor detail is not actionable: %q", check.Detail)
			}
			return
		}
	}
	t.Fatalf("config.yaml check not found: %#v", report.Checks)
}

func TestDoctorChecksLiteLLMBinaryInsteadOfLegacyRunProxyScript(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	if err := os.WriteFile(filepath.Join(runtime.Path, "config.yaml"), []byte("model_list: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime.Path, ".env"), []byte("LITELLM_MASTER_KEY=\"sk-secret\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(runtime.Path, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(runtime.Path, ".venv", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "litellm"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	report := Doctor(runtime)

	var sawLiteLLM bool
	for _, check := range report.Checks {
		if check.Name == "run-proxy.sh" {
			t.Fatalf("doctor should not require legacy run-proxy.sh: %#v", report.Checks)
		}
		if check.Name == "litellm binary" {
			sawLiteLLM = true
			if !check.OK {
				t.Fatalf("litellm binary check failed: %#v", check)
			}
		}
	}
	if !sawLiteLLM {
		t.Fatalf("doctor did not report litellm binary check: %#v", report.Checks)
	}
}

func TestDoctorUsesManagedProfileLogsDir(t *testing.T) {
	paths := setupTestPaths(t, "default")
	if err := os.MkdirAll(paths.ProfileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.LogsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := Profile{Name: "default", Mode: ProfileModeManaged, RuntimeDir: paths.ProfileDir, LogsDir: paths.LogsDir, Port: 4400, Paths: paths}
	if err := WriteProfileFile(paths.ProfileDir, profile); err != nil {
		t.Fatal(err)
	}

	report := Doctor(NewRuntime(paths.ProfileDir))

	for _, check := range report.Checks {
		if check.Name != "logs" {
			continue
		}
		if !check.OK {
			t.Fatalf("logs check = %#v, want managed logs dir to pass", check)
		}
		return
	}
	t.Fatalf("logs check not found: %#v", report.Checks)
}

func TestDoctorKeepsLogsCheckNameForCustomLogsDirBasename(t *testing.T) {
	paths := setupTestPaths(t, "default")
	if err := os.MkdirAll(paths.ProfileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logsDir := filepath.Join(t.TempDir(), "custom-log-output")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := Profile{Name: "default", Mode: ProfileModeManaged, RuntimeDir: paths.ProfileDir, LogsDir: logsDir, Port: 4400}
	if err := WriteProfileFile(paths.ProfileDir, profile); err != nil {
		t.Fatal(err)
	}

	report := Doctor(NewRuntime(paths.ProfileDir))

	for _, check := range report.Checks {
		if check.Name == "custom-log-output" {
			t.Fatalf("logs check should not be named from path basename: %#v", check)
		}
		if check.Name == "logs" {
			if !check.OK {
				t.Fatalf("logs check = %#v, want custom logs dir to pass", check)
			}
			return
		}
	}
	t.Fatalf("logs check not found: %#v", report.Checks)
}

func TestDoctorMissingLiteLLMBinaryPointsToManagedSetup(t *testing.T) {
	runtime := NewRuntime(t.TempDir())

	report := Doctor(runtime)

	for _, check := range report.Checks {
		if check.Name != "litellm binary" {
			continue
		}
		if !strings.Contains(check.Detail, "rerun ezyl3 setup") {
			t.Fatalf("litellm detail = %q, want setup guidance", check.Detail)
		}
		if strings.Contains(check.Detail, "install litellm[proxy]") {
			t.Fatalf("litellm detail should not suggest manual pip install: %q", check.Detail)
		}
		return
	}
	t.Fatalf("litellm binary check not found: %#v", report.Checks)
}

func TestCursorSettingsFallsBackToLocalBaseURLWithoutNgrok(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	if err := os.WriteFile(filepath.Join(runtime.Path, ".env"), []byte("LITELLM_MASTER_KEY=\"sk-secret\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	settings, err := CursorSettings(runtime, false)
	if err != nil {
		t.Fatalf("CursorSettings returned error: %v", err)
	}
	if !strings.Contains(settings, "Base URL: http://127.0.0.1:4400/v1") {
		t.Fatalf("settings missing local base URL:\n%s", settings)
	}
	if strings.Contains(settings, "sk-secret") {
		t.Fatalf("settings leaked secret:\n%s", settings)
	}
	if !strings.Contains(settings, "WARNING") || !strings.Contains(settings, "Cursor cannot use it") {
		t.Fatalf("local-only settings should warn that Cursor cannot use the URL:\n%s", settings)
	}
}

func TestCursorSettingsOmitsLocalOnlyWarningWithNgrokDomain(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	if err := os.WriteFile(filepath.Join(runtime.Path, ".env"), []byte("LITELLM_MASTER_KEY=\"sk-secret\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteProfileFile(runtime.Path, Profile{Name: "default", Mode: ProfileModeManaged, Domain: "demo.ngrok-free.dev"}); err != nil {
		t.Fatal(err)
	}

	settings, err := CursorSettings(runtime, false)
	if err != nil {
		t.Fatalf("CursorSettings returned error: %v", err)
	}
	if !strings.Contains(settings, "https://demo.ngrok-free.dev/v1") {
		t.Fatalf("settings missing tunnel base URL:\n%s", settings)
	}
	if strings.Contains(settings, "WARNING") {
		t.Fatalf("tunnel profile should not show local-only warning:\n%s", settings)
	}
}

func TestCursorSettingsRevealKeyPrintsUnquotedMasterKey(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	if err := os.WriteFile(filepath.Join(runtime.Path, ".env"), []byte("LITELLM_MASTER_KEY=\"sk-cursor-abc123\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	settings, err := CursorSettings(runtime, true)
	if err != nil {
		t.Fatalf("CursorSettings returned error: %v", err)
	}
	if !strings.Contains(settings, "API key: sk-cursor-abc123\n") {
		t.Fatalf("reveal should print the clean key:\n%s", settings)
	}
	// The revealed key must never carry the .env surrounding quotes, which is
	// exactly the trap that breaks pasting into Cursor.
	if strings.Contains(settings, "\"sk-cursor-abc123\"") {
		t.Fatalf("revealed key must not be quoted:\n%s", settings)
	}
}

func TestDetectNgrokDomainReadsProfileJSON(t *testing.T) {
	runtime := t.TempDir()
	profile := Profile{
		Name:           "default",
		Mode:           ProfileModeManaged,
		RuntimeDir:     runtime,
		Port:           4400,
		TunnelProvider: "ngrok",
		Domain:         "profile.ngrok-free.dev",
	}
	if err := writeJSON(filepath.Join(runtime, "profile.json"), profile, 0o644); err != nil {
		t.Fatal(err)
	}

	domain, err := DetectNgrokDomain(runtime)
	if err != nil {
		t.Fatalf("DetectNgrokDomain returned error: %v", err)
	}
	if domain != "profile.ngrok-free.dev" {
		t.Fatalf("domain = %q", domain)
	}
}
