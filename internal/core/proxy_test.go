package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuildProxyCommandUsesRuntimeEnvAndLiteLLMBinary(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	if err := os.WriteFile(filepath.Join(runtime.Path, "config.yaml"), []byte("model_list: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime.Path, ".env"), []byte("LITELLM_MASTER_KEY=\"sk-test\"\nCUSTOM_FLAG=\"enabled\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(runtime.Path, ".venv", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	litellm := filepath.Join(binDir, "litellm")
	if err := os.WriteFile(litellm, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	spec, err := BuildProxyCommand(runtime)
	if err != nil {
		t.Fatalf("BuildProxyCommand returned error: %v", err)
	}

	if spec.Dir != runtime.Path {
		t.Fatalf("Dir = %q, want %q", spec.Dir, runtime.Path)
	}
	if spec.Path != litellm {
		t.Fatalf("Path = %q, want %q", spec.Path, litellm)
	}
	wantArgs := []string{"--config", filepath.Join(runtime.Path, "config.yaml"), "--host", "127.0.0.1", "--port", "4400"}
	if !reflect.DeepEqual(spec.Args, wantArgs) {
		t.Fatalf("Args = %#v, want %#v", spec.Args, wantArgs)
	}
	for _, want := range []string{"LITELLM_MASTER_KEY=sk-test", "CUSTOM_FLAG=enabled", "EZYL3_USAGE_DB=" + filepath.Join(runtime.Path, UsageDBFileName)} {
		if !containsString(spec.Env, want) {
			t.Fatalf("Env missing %q in %#v", want, spec.Env)
		}
	}
}

func TestBuildProxyCommandReportsMissingLiteLLMBinary(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	if err := os.WriteFile(filepath.Join(runtime.Path, "config.yaml"), []byte("model_list: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime.Path, ".env"), []byte("LITELLM_MASTER_KEY=\"sk-test\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := BuildProxyCommand(runtime)
	if err == nil {
		t.Fatalf("BuildProxyCommand returned nil error for missing litellm binary")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
