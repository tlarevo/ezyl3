package core

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type ProxyCommandSpec struct {
	Dir  string
	Path string
	Args []string
	Env  []string
}

type ProxyStdio struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func BuildProxyCommand(runtime Runtime) (ProxyCommandSpec, error) {
	runtimePath := strings.TrimSpace(runtime.Path)
	if runtimePath == "" {
		return ProxyCommandSpec{}, fmt.Errorf("runtime path is required")
	}
	port := runtime.Port
	if port == 0 {
		port = 4400
	}
	litellm := filepath.Join(runtimePath, ".venv", "bin", "litellm")
	if _, err := os.Stat(litellm); err != nil {
		return ProxyCommandSpec{}, fmt.Errorf("LiteLLM binary missing at %s; run ezyl3 setup without --skip-python-deps or install litellm[proxy]", litellm)
	}
	env, err := ReadEnvFile(filepath.Join(runtimePath, ".env"))
	if err != nil {
		return ProxyCommandSpec{}, err
	}
	env = append(env, "EZYL3_USAGE_DB="+UsageDBPath(runtime))
	return ProxyCommandSpec{
		Dir:  runtimePath,
		Path: litellm,
		Args: []string{
			"--config", filepath.Join(runtimePath, "config.yaml"),
			"--host", "127.0.0.1",
			"--port", strconv.Itoa(port),
		},
		Env: env,
	}, nil
}

func RunProxy(runtime Runtime, stdio ProxyStdio) error {
	spec, err := BuildProxyCommand(runtime)
	if err != nil {
		return err
	}
	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.Stdin = stdio.Stdin
	cmd.Stdout = stdio.Stdout
	cmd.Stderr = stdio.Stderr
	return cmd.Run()
}
