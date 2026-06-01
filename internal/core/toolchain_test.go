package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestUVArtifactForDarwinArchitectures(t *testing.T) {
	tests := []struct {
		goarch   string
		name     string
		checksum string
	}{
		{
			goarch:   "arm64",
			name:     "uv-aarch64-apple-darwin.tar.gz",
			checksum: "2b25be1af546be330b340b0a76b99f989daa6d92678fdffb87438e661e9d88fb",
		},
		{
			goarch:   "amd64",
			name:     "uv-x86_64-apple-darwin.tar.gz",
			checksum: "6b91ae3de155f51bd1f5b74814821c79f016a176561f252cd9ddfb976939af2e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.goarch, func(t *testing.T) {
			artifact, err := uvArtifactFor("darwin", tt.goarch)
			if err != nil {
				t.Fatalf("uvArtifactFor returned error: %v", err)
			}
			if artifact.Name != tt.name {
				t.Fatalf("Name = %q, want %q", artifact.Name, tt.name)
			}
			if artifact.SHA256 != tt.checksum {
				t.Fatalf("SHA256 = %q, want %q", artifact.SHA256, tt.checksum)
			}
			if !strings.Contains(artifact.URL, UVBootstrapVersion) || !strings.HasSuffix(artifact.URL, tt.name) {
				t.Fatalf("URL = %q, want pinned release URL ending in artifact name", artifact.URL)
			}
		})
	}
}

func TestUVArtifactForUnsupportedPlatform(t *testing.T) {
	_, err := uvArtifactFor("linux", "arm64")
	if err == nil {
		t.Fatal("uvArtifactFor returned nil error for unsupported platform")
	}
	if !strings.Contains(err.Error(), "unsupported uv bootstrap platform linux/arm64") {
		t.Fatalf("error = %q, want unsupported platform detail", err)
	}
}

func TestVerifySHA256RejectsMismatch(t *testing.T) {
	data := []byte("archive bytes")
	actual := fmt.Sprintf("%x", sha256.Sum256(data))

	if err := verifySHA256(data, actual); err != nil {
		t.Fatalf("verifySHA256 returned error for matching checksum: %v", err)
	}
	if err := verifySHA256(data, strings.Repeat("0", 64)); err == nil {
		t.Fatal("verifySHA256 returned nil error for mismatched checksum")
	}
}

func TestUVToolchainUsesPathUVAndBuildsManagedCommands(t *testing.T) {
	paths := setupTestPaths(t, "default")
	runner := &recordingCommandRunner{}
	var out, errBuf bytes.Buffer
	toolchain := UVToolchain{
		Stdout: &out,
		Stderr: &errBuf,
		Deps: UVToolchainDeps{
			LookPath: func(name string) (string, error) {
				if name != "uv" {
					return "", errors.New("unexpected lookup")
				}
				return "/usr/local/bin/uv", nil
			},
			Run: runner.Run,
		},
	}

	if err := toolchain.InstallPythonDeps(paths); err != nil {
		t.Fatalf("InstallPythonDeps returned error: %v", err)
	}

	venv := filepath.Join(paths.ProfileDir, ".venv")
	python := filepath.Join(venv, "bin", "python")
	want := []commandSpec{
		{
			Dir:    paths.ProfileDir,
			Path:   "/usr/local/bin/uv",
			Args:   []string{"--no-config", "python", "install", ManagedPythonVersion, "--managed-python"},
			Env:    uvManagedEnv(paths.CacheDir),
			Stdout: &out,
			Stderr: &errBuf,
		},
		{
			Dir:    paths.ProfileDir,
			Path:   "/usr/local/bin/uv",
			Args:   []string{"--no-config", "venv", "--python", ManagedPythonVersion, "--managed-python", venv},
			Env:    uvManagedEnv(paths.CacheDir),
			Stdout: &out,
			Stderr: &errBuf,
		},
		{
			Dir:    paths.ProfileDir,
			Path:   "/usr/local/bin/uv",
			Args:   []string{"--no-config", "pip", "install", "--python", python, "litellm[proxy]"},
			Env:    uvManagedEnv(paths.CacheDir),
			Stdout: &out,
			Stderr: &errBuf,
		},
	}
	if !reflect.DeepEqual(runner.commands, want) {
		t.Fatalf("commands = %#v, want %#v", runner.commands, want)
	}
}

func TestUVToolchainDefaultsCommandWritersToInjectedSinks(t *testing.T) {
	// When writers are injected (as the wizard does to avoid corrupting the
	// alt-screen), every command spec must carry them rather than os.Stdout.
	paths := setupTestPaths(t, "default")
	runner := &recordingCommandRunner{}
	var sink bytes.Buffer
	toolchain := UVToolchain{
		Stdout: &sink,
		Stderr: &sink,
		Deps: UVToolchainDeps{
			LookPath: func(string) (string, error) { return "/usr/local/bin/uv", nil },
			Run:      runner.Run,
		},
	}
	if err := toolchain.InstallPythonDeps(paths); err != nil {
		t.Fatalf("InstallPythonDeps returned error: %v", err)
	}
	if len(runner.commands) == 0 {
		t.Fatal("no commands recorded")
	}
	for i, c := range runner.commands {
		if c.Stdout != &sink || c.Stderr != &sink {
			t.Fatalf("command %d writers = (%v,%v), want injected sink", i, c.Stdout, c.Stderr)
		}
	}
}

func TestUVToolchainDownloadsVerifiedUVWhenPathMissing(t *testing.T) {
	paths := setupTestPaths(t, "default")
	archive := testTarGz(t, "uv-aarch64-apple-darwin/uv", "#!/bin/sh\n")
	artifact, err := uvArtifactFor("darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	artifact.SHA256 = fmt.Sprintf("%x", sha256.Sum256(archive))

	runner := &recordingCommandRunner{}
	var downloadedURL string
	toolchain := UVToolchain{
		GOOS:   "darwin",
		GOARCH: "arm64",
		Deps: UVToolchainDeps{
			LookPath: func(string) (string, error) { return "", errors.New("not found") },
			Artifact: func(string, string) (uvArtifact, error) {
				return artifact, nil
			},
			Download: func(url string) ([]byte, error) {
				downloadedURL = url
				return archive, nil
			},
			Run: runner.Run,
		},
	}

	if err := toolchain.InstallPythonDeps(paths); err != nil {
		t.Fatalf("InstallPythonDeps returned error: %v", err)
	}

	wantUV := filepath.Join(paths.CacheDir, "tools", "uv", UVBootstrapVersion, "darwin-arm64", "uv")
	if downloadedURL != artifact.URL {
		t.Fatalf("downloaded URL = %q, want %q", downloadedURL, artifact.URL)
	}
	if _, err := os.Stat(wantUV); err != nil {
		t.Fatalf("expected uv binary to be installed at %s: %v", wantUV, err)
	}
	if len(runner.commands) != 3 || runner.commands[0].Path != wantUV {
		t.Fatalf("runner commands = %#v, want downloaded uv path", runner.commands)
	}
}

func TestUVToolchainRejectsBadChecksumBeforeRunningCommands(t *testing.T) {
	paths := setupTestPaths(t, "default")
	runner := &recordingCommandRunner{}
	toolchain := UVToolchain{
		GOOS:   "darwin",
		GOARCH: "arm64",
		Deps: UVToolchainDeps{
			LookPath: func(string) (string, error) { return "", errors.New("not found") },
			Artifact: func(string, string) (uvArtifact, error) {
				return uvArtifact{Name: "uv-aarch64-apple-darwin.tar.gz", URL: "https://example.invalid/uv.tar.gz", SHA256: strings.Repeat("0", 64)}, nil
			},
			Download: func(string) ([]byte, error) { return []byte("not the archive"), nil },
			Run:      runner.Run,
		},
	}

	err := toolchain.InstallPythonDeps(paths)
	if err == nil {
		t.Fatal("InstallPythonDeps returned nil error for bad checksum")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error = %q, want checksum mismatch", err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("commands ran before checksum failure: %#v", runner.commands)
	}
}

func TestDownloadBytesWithTimeoutCancelsHungRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	_, err := downloadBytesWithTimeout(server.URL, 10*time.Millisecond)
	if err == nil {
		t.Fatal("downloadBytesWithTimeout returned nil error for timed-out request")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}

func TestRunToolchainCommandHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runToolchainCommand(commandSpec{
		Context: ctx,
		Path:    testCommandPath(t),
		Args:    []string{"-c", "exit 0"},
	})
	if err == nil {
		t.Fatal("runToolchainCommand returned nil error for canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

type recordingCommandRunner struct {
	commands []commandSpec
}

func (r *recordingCommandRunner) Run(cmd commandSpec) error {
	r.commands = append(r.commands, cmd)
	return nil
}

func testTarGz(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testCommandPath(t *testing.T) string {
	t.Helper()
	for _, candidate := range []string{"sh", "true"} {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path
		}
	}
	t.Fatal("no test command found")
	return ""
}
