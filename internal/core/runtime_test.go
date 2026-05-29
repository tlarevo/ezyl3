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

func TestCursorSettingsFallsBackToLocalBaseURLWithoutNgrok(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	if err := os.WriteFile(filepath.Join(runtime.Path, ".env"), []byte("LITELLM_MASTER_KEY=\"sk-secret\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	settings, err := CursorSettings(runtime)
	if err != nil {
		t.Fatalf("CursorSettings returned error: %v", err)
	}
	if !strings.Contains(settings, "Base URL: http://127.0.0.1:4400/v1") {
		t.Fatalf("settings missing local base URL:\n%s", settings)
	}
	if strings.Contains(settings, "sk-secret") {
		t.Fatalf("settings leaked secret:\n%s", settings)
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
