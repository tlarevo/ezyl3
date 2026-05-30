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
  callbacks: ezyl3_usage_callback.proxy_handler_instance
  drop_params: true
  num_retries: 2
  request_timeout: 600
  set_verbose: false
  turn_off_message_logging: true
  fallbacks:
    - litellm-simple: ["litellm-simple-fb"]
    - litellm-medium: ["litellm-medium-fb"]
    - litellm-complex: ["litellm-complex-fb"]
    - litellm-reasoning: ["litellm-reasoning-fb"]
general_settings:
  master_key: os.environ/LITELLM_MASTER_KEY
`

const UsageCallbackFileName = "ezyl3_usage_callback.py"

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
	store, err := OpenUsageStore(filepath.Join(paths.ProfileDir, UsageDBFileName))
	if err != nil {
		return Profile{}, err
	}
	if err := store.Close(); err != nil {
		return Profile{}, err
	}
	if err := os.WriteFile(filepath.Join(paths.ProfileDir, UsageCallbackFileName), []byte(UsageCallbackPython), 0o644); err != nil {
		return Profile{}, err
	}
	profile := Profile{Name: paths.Profile, Mode: ProfileModeManaged, RuntimeDir: paths.ProfileDir, Port: 4400, TunnelProvider: "ngrok", Domain: domain, Paths: paths}
	if err := WriteProfileFile(paths.ProfileDir, profile); err != nil {
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

func WriteLaunchAgents(paths ProfilePaths, domain string, port int, executablePath string) error {
	if err := os.MkdirAll(filepath.Dir(paths.LaunchAgentPath("litellm")), 0o755); err != nil {
		return err
	}
	litellm, err := RenderLiteLLMPlist(paths, port, executablePath)
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

const UsageCallbackPython = `import os
import sqlite3
from datetime import datetime, timezone
from litellm.integrations.custom_logger import CustomLogger


class Ezyl3UsageHandler(CustomLogger):
    async def async_log_success_event(self, kwargs, response_obj, start_time, end_time):
        self._record("success", kwargs, response_obj, start_time, end_time, "")

    async def async_log_failure_event(self, kwargs, response_obj, start_time, end_time):
        self._record("failure", kwargs, response_obj, start_time, end_time, type(response_obj).__name__[:200])

    def _record(self, status, kwargs, response_obj, start_time, end_time, error):
        db_path = os.environ.get("EZYL3_USAGE_DB")
        if not db_path:
            return
        try:
            usage = self._value(response_obj, "usage", {}) or {}
            model = str(kwargs.get("model") or self._value(response_obj, "model", "") or "")
            provider = model.split("/", 1)[0] if "/" in model else ""
            duration_ms = int(max((end_time - start_time).total_seconds() * 1000, 0))
            with sqlite3.connect(db_path, timeout=5) as db:
                self._ensure_schema(db)
                db.execute(
                    """INSERT INTO usage_events (
                        created_at, model, provider, prompt_tokens, completion_tokens,
                        total_tokens, cost_usd, duration_ms, status, error
                    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
                    (
                        datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
                        model,
                        provider,
                        int(self._value(usage, "prompt_tokens", 0) or 0),
                        int(self._value(usage, "completion_tokens", 0) or 0),
                        int(self._value(usage, "total_tokens", 0) or 0),
                        float(kwargs.get("response_cost") or 0),
                        duration_ms,
                        status,
                        error,
                    ),
                )
        except Exception:
            return

    def _value(self, obj, name, default=None):
        if isinstance(obj, dict):
            return obj.get(name, default)
        return getattr(obj, name, default)

    def _ensure_schema(self, db):
        db.execute(
            """CREATE TABLE IF NOT EXISTS usage_events (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                created_at TEXT NOT NULL,
                model TEXT NOT NULL DEFAULT '',
                provider TEXT NOT NULL DEFAULT '',
                prompt_tokens INTEGER NOT NULL DEFAULT 0,
                completion_tokens INTEGER NOT NULL DEFAULT 0,
                total_tokens INTEGER NOT NULL DEFAULT 0,
                cost_usd REAL NOT NULL DEFAULT 0,
                duration_ms INTEGER NOT NULL DEFAULT 0,
                status TEXT NOT NULL DEFAULT '',
                error TEXT NOT NULL DEFAULT ''
            )"""
        )


proxy_handler_instance = Ezyl3UsageHandler()
`
