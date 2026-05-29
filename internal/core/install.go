package core

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const DefaultLiteLLMConfig = `model_list:
  - model_name: litellm-simple
    litellm_params:
      model: huggingface/deepseek-ai/DeepSeek-V4-Flash
      api_key: os.environ/HF_TOKEN
  - model_name: litellm-medium
    litellm_params:
      model: huggingface/deepseek-ai/DeepSeek-V4-Flash
      api_key: os.environ/HF_TOKEN
  - model_name: litellm-complex
    litellm_params:
      model: huggingface/deepseek-ai/DeepSeek-V4-Pro
      api_key: os.environ/HF_TOKEN
  - model_name: litellm-reasoning
    litellm_params:
      model: huggingface/deepseek-ai/DeepSeek-V4-Pro
      api_key: os.environ/HF_TOKEN
  - model_name: litellm-simple-fb
    litellm_params:
      model: ollama_chat/gpt-oss:20b
      api_base: https://ollama.com
      api_key: os.environ/OLLAMA_API_KEY
  - model_name: litellm-medium-fb
    litellm_params:
      model: ollama_chat/qwen3-coder-next
      api_base: https://ollama.com
      api_key: os.environ/OLLAMA_API_KEY
  - model_name: litellm-complex-fb
    litellm_params:
      model: ollama_chat/deepseek-v3.2
      api_base: https://ollama.com
      api_key: os.environ/OLLAMA_API_KEY
  - model_name: litellm-reasoning-fb
    litellm_params:
      model: ollama_chat/deepseek-v4-pro
      api_base: https://ollama.com
      api_key: os.environ/OLLAMA_API_KEY
litellm_settings:
  drop_params: true
  num_retries: 2
  request_timeout: 600
  set_verbose: false
  fallbacks:
    - litellm-simple: ["litellm-simple-fb"]
    - litellm-medium: ["litellm-medium-fb"]
    - litellm-complex: ["litellm-complex-fb"]
    - litellm-reasoning: ["litellm-reasoning-fb"]
general_settings:
  master_key: os.environ/LITELLM_MASTER_KEY
`

func CreateManagedProfile(paths ProfilePaths, secrets Secrets, domain string) (Profile, error) {
	if err := os.MkdirAll(paths.ProfileDir, 0o755); err != nil {
		return Profile{}, err
	}
	if err := os.MkdirAll(paths.LogsDir, 0o755); err != nil {
		return Profile{}, err
	}
	if err := os.WriteFile(filepath.Join(paths.ProfileDir, "config.yaml"), []byte(DefaultLiteLLMConfig), 0o644); err != nil {
		return Profile{}, err
	}
	if err := WriteSecrets(filepath.Join(paths.ProfileDir, ".env"), secrets); err != nil {
		return Profile{}, err
	}
	if err := os.WriteFile(filepath.Join(paths.ProfileDir, "run-proxy.sh"), []byte(runProxyScript(paths.ProfileDir)), 0o755); err != nil {
		return Profile{}, err
	}
	profile := Profile{Name: paths.Profile, Mode: ProfileModeManaged, RuntimeDir: paths.ProfileDir, Port: 4400, TunnelProvider: "ngrok", Domain: domain, Paths: paths}
	if err := writeJSON(filepath.Join(paths.ProfileDir, "metadata.json"), profile, 0o644); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func GenerateMasterKey() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "sk-cursor-" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func InstallPythonDeps(runtimeDir string) error {
	python, err := exec.LookPath("python3.12")
	if err != nil {
		python, err = exec.LookPath("python3")
		if err != nil {
			return fmt.Errorf("python3.12 or python3 is required: %w", err)
		}
	}
	venv := filepath.Join(runtimeDir, ".venv")
	if err := runCommand(runtimeDir, python, "-m", "venv", venv); err != nil {
		return err
	}
	pip := filepath.Join(venv, "bin", "pip")
	if err := runCommand(runtimeDir, pip, "install", "--upgrade", "pip"); err != nil {
		return err
	}
	return runCommand(runtimeDir, pip, "install", "litellm[proxy]")
}

func WriteLaunchAgents(paths ProfilePaths, domain string, port int) error {
	if err := os.MkdirAll(filepath.Dir(paths.LaunchAgentPath("litellm")), 0o755); err != nil {
		return err
	}
	litellm, err := RenderLiteLLMPlist(paths, port)
	if err != nil {
		return err
	}
	if err := os.WriteFile(paths.LaunchAgentPath("litellm"), []byte(litellm), 0o600); err != nil {
		return err
	}
	if domain == "" {
		return nil
	}
	ngrok, err := RenderNgrokPlist(paths, domain, port)
	if err != nil {
		return err
	}
	return os.WriteFile(paths.LaunchAgentPath("ngrok"), []byte(ngrok), 0o600)
}

func runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runProxyScript(runtimeDir string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
set -a
source %q
set +a
exec %q --config %q --host 127.0.0.1 --port 4400
`, filepath.Join(runtimeDir, ".env"), filepath.Join(runtimeDir, ".venv", "bin", "litellm"), filepath.Join(runtimeDir, "config.yaml"))
}
