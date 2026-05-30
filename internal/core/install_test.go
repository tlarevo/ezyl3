package core

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSelectPythonForLiteLLMPrefersSupportedMinorVersion(t *testing.T) {
	paths := map[string]string{
		"python3.13": "/opt/homebrew/bin/python3.13",
		"python3.12": "/opt/homebrew/bin/python3.12",
		"python3":    "/usr/local/bin/python3",
	}
	versions := map[string]string{
		"/opt/homebrew/bin/python3.13": "Python 3.13.9",
		"/opt/homebrew/bin/python3.12": "Python 3.12.12",
		"/usr/local/bin/python3":       "Python 3.14.5",
	}

	got, err := selectPythonForLiteLLM(fakeLookPath(paths), fakePythonVersion(versions))
	if err != nil {
		t.Fatalf("selectPythonForLiteLLM returned error: %v", err)
	}

	if got != "/opt/homebrew/bin/python3.13" {
		t.Fatalf("python = %q, want python3.13 path", got)
	}
}

func TestSelectPythonForLiteLLMRejectsUnsupportedPython3Fallback(t *testing.T) {
	paths := map[string]string{
		"python3": "/usr/local/bin/python3",
	}
	versions := map[string]string{
		"/usr/local/bin/python3": "Python 3.14.5",
	}

	_, err := selectPythonForLiteLLM(fakeLookPath(paths), fakePythonVersion(versions))
	if err == nil {
		t.Fatal("selectPythonForLiteLLM returned nil error, want unsupported version")
	}
	if !strings.Contains(err.Error(), "python 3.12 or 3.13 is required") {
		t.Fatalf("error = %q, want supported version guidance", err)
	}
	if !strings.Contains(err.Error(), "found Python 3.14.5") {
		t.Fatalf("error = %q, want detected version", err)
	}
}

func TestPythonVersionTimesOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip Unix shell-script timeout helper on Windows")
	}
	script := filepath.Join(t.TempDir(), "slow-python")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 1\necho Python 3.13.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := pythonVersionWithTimeout(script, 10*time.Millisecond)
	if err == nil {
		t.Fatal("pythonVersionWithTimeout returned nil error, want timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %q, want timeout detail", err)
	}
}

func fakeLookPath(paths map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		path, ok := paths[name]
		if !ok {
			return "", errors.New("not found")
		}
		return path, nil
	}
}

func fakePythonVersion(versions map[string]string) func(string) (string, error) {
	return func(path string) (string, error) {
		version, ok := versions[path]
		if !ok {
			return "", errors.New("version failed")
		}
		return version, nil
	}
}
