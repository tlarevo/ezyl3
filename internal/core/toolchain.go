package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	UVBootstrapVersion   = "0.11.16"
	ManagedPythonVersion = "3.13.13"
	uvDownloadTimeout    = 5 * time.Minute
	uvCommandTimeout     = 30 * time.Minute
)

type UVToolchain struct {
	GOOS   string
	GOARCH string
	Deps   UVToolchainDeps
}

type UVToolchainDeps struct {
	LookPath func(string) (string, error)
	Artifact func(goos, goarch string) (uvArtifact, error)
	Download func(string) ([]byte, error)
	Run      func(commandSpec) error
}

type uvArtifact struct {
	Name   string
	URL    string
	SHA256 string
}

type commandSpec struct {
	Context context.Context
	Dir     string
	Path    string
	Args    []string
	Env     []string
}

func (t UVToolchain) InstallPythonDeps(paths ProfilePaths) error {
	if strings.TrimSpace(paths.ProfileDir) == "" {
		return fmt.Errorf("profile runtime path is required")
	}
	if strings.TrimSpace(paths.CacheDir) == "" {
		return fmt.Errorf("cache path is required")
	}
	uv, err := t.ensureUV(paths.CacheDir)
	if err != nil {
		return err
	}
	env := uvManagedEnv(paths.CacheDir)
	venv := filepath.Join(paths.ProfileDir, ".venv")
	commands := []commandSpec{
		{
			Dir:  paths.ProfileDir,
			Path: uv,
			Args: []string{"--no-config", "python", "install", ManagedPythonVersion, "--managed-python"},
			Env:  env,
		},
		{
			Dir:  paths.ProfileDir,
			Path: uv,
			Args: []string{"--no-config", "venv", "--python", ManagedPythonVersion, "--managed-python", venv},
			Env:  env,
		},
		{
			Dir:  paths.ProfileDir,
			Path: uv,
			Args: []string{"--no-config", "pip", "install", "--python", filepath.Join(venv, "bin", "python"), "litellm[proxy]"},
			Env:  env,
		},
	}
	run := t.Deps.Run
	if run == nil {
		run = runToolchainCommand
	}
	for _, command := range commands {
		if err := run(command); err != nil {
			return err
		}
	}
	return nil
}

func (t UVToolchain) ensureUV(cacheDir string) (string, error) {
	lookPath := t.Deps.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if path, err := lookPath("uv"); err == nil {
		return path, nil
	}

	goos := t.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := t.GOARCH
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	artifactFor := t.Deps.Artifact
	if artifactFor == nil {
		artifactFor = uvArtifactFor
	}
	artifact, err := artifactFor(goos, goarch)
	if err != nil {
		return "", err
	}
	uvPath := uvBinaryPath(cacheDir, goos, goarch)
	if info, err := os.Stat(uvPath); err == nil && !info.IsDir() {
		return uvPath, nil
	}
	download := t.Deps.Download
	if download == nil {
		download = downloadBytes
	}
	fmt.Fprintf(os.Stderr, "Downloading uv %s to %s\n", UVBootstrapVersion, filepath.Dir(uvPath))
	archive, err := download(artifact.URL)
	if err != nil {
		return "", fmt.Errorf("download uv %s: %w", UVBootstrapVersion, err)
	}
	if err := verifySHA256(archive, artifact.SHA256); err != nil {
		return "", err
	}
	if err := extractUVBinary(archive, uvPath); err != nil {
		return "", err
	}
	return uvPath, nil
}

func uvArtifactFor(goos, goarch string) (uvArtifact, error) {
	var name, checksum string
	switch goos + "/" + goarch {
	case "darwin/arm64":
		name = "uv-aarch64-apple-darwin.tar.gz"
		checksum = "2b25be1af546be330b340b0a76b99f989daa6d92678fdffb87438e661e9d88fb"
	case "darwin/amd64":
		name = "uv-x86_64-apple-darwin.tar.gz"
		checksum = "6b91ae3de155f51bd1f5b74814821c79f016a176561f252cd9ddfb976939af2e"
	default:
		return uvArtifact{}, fmt.Errorf("unsupported uv bootstrap platform %s/%s; install uv manually or use macOS arm64/amd64", goos, goarch)
	}
	return uvArtifact{
		Name:   name,
		URL:    fmt.Sprintf("https://github.com/astral-sh/uv/releases/download/%s/%s", UVBootstrapVersion, name),
		SHA256: checksum,
	}, nil
}

func uvBinaryPath(cacheDir, goos, goarch string) string {
	return filepath.Join(cacheDir, "tools", "uv", UVBootstrapVersion, goos+"-"+goarch, "uv")
}

func uvManagedEnv(cacheDir string) []string {
	return []string{
		"UV_CACHE_DIR=" + filepath.Join(cacheDir, "uv", "cache"),
		"UV_PYTHON_INSTALL_DIR=" + filepath.Join(cacheDir, "python"),
	}
}

func verifySHA256(data []byte, expected string) error {
	actual := fmt.Sprintf("%x", sha256.Sum256(data))
	if !strings.EqualFold(actual, strings.TrimSpace(expected)) {
		return fmt.Errorf("checksum mismatch for uv archive: got %s, want %s", actual, expected)
	}
	return nil
}

func extractUVBinary(archive []byte, target string) error {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("read uv archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read uv archive entry: %w", err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != "uv" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	return fmt.Errorf("uv binary not found in archive")
}

func downloadBytes(url string) ([]byte, error) {
	return downloadBytesWithTimeout(url, uvDownloadTimeout)
}

func downloadBytesWithTimeout(url string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func runToolchainCommand(spec commandSpec) error {
	ctx := spec.Context
	cancel := func() {}
	if ctx == nil {
		ctx, cancel = context.WithTimeout(context.Background(), uvCommandTimeout)
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("run %s: %w", spec.Path, ctxErr)
		}
		return err
	}
	return nil
}
