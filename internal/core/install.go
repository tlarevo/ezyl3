package core

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RenderLiteLLMConfig builds the managed LiteLLM config.yaml. It defines the four
// Hugging Face tiers, their Ollama fallback tiers, and the litellm-auto complexity
// router that selects a tier per request. The X-HF-Bill-To org-billing header is
// added to Hugging Face entries only when a bill-to org is supplied; when it is
// blank the header is omitted entirely rather than emitted empty.
func RenderLiteLLMConfig(secrets Secrets) string {
	billTo := strings.TrimSpace(secrets.HFBillTo)
	hf := func(name, model string) string {
		entry := fmt.Sprintf("  - model_name: %s\n    litellm_params:\n      model: %s\n      api_key: os.environ/HF_TOKEN\n", name, model)
		if billTo != "" {
			entry += fmt.Sprintf("      extra_headers:\n        X-HF-Bill-To: %q\n", billTo)
		}
		return entry
	}
	ollama := func(name, model string) string {
		return fmt.Sprintf("  - model_name: %s\n    litellm_params:\n      model: %s\n      api_base: https://ollama.com\n      api_key: os.environ/OLLAMA_API_KEY\n", name, model)
	}

	var b strings.Builder
	b.WriteString("model_list:\n")
	b.WriteString(hf("litellm-simple", "huggingface/deepseek-ai/DeepSeek-V4-Flash"))
	b.WriteString(hf("litellm-medium", "huggingface/deepseek-ai/DeepSeek-V4-Flash"))
	b.WriteString(hf("litellm-complex", "huggingface/deepseek-ai/DeepSeek-V4-Pro"))
	b.WriteString(hf("litellm-reasoning", "huggingface/deepseek-ai/DeepSeek-V4-Pro"))
	b.WriteString(ollama("litellm-simple-fb", "ollama_chat/gpt-oss:20b"))
	b.WriteString(ollama("litellm-medium-fb", "ollama_chat/qwen3-coder-next"))
	b.WriteString(ollama("litellm-complex-fb", "ollama_chat/deepseek-v3.2"))
	b.WriteString(ollama("litellm-reasoning-fb", "ollama_chat/deepseek-v4-pro"))
	b.WriteString(`  - model_name: litellm-auto
    litellm_params:
      model: auto_router/complexity_router
      complexity_router_config:
        tiers:
          SIMPLE: litellm-simple
          MEDIUM: litellm-medium
          COMPLEX: litellm-complex
          REASONING: litellm-reasoning
        tier_boundaries:
          simple_medium: 0.15
          medium_complex: 0.35
          complex_reasoning: 0.60
        default_model: litellm-medium
litellm_settings:
  callbacks: ezyl3_usage_callback.proxy_handler_instance
  drop_params: true
  num_retries: 2
  request_timeout: 600
  set_verbose: false
  turn_off_message_logging: true
  fallbacks:
    - litellm-auto: ["litellm-simple-fb", "litellm-medium-fb", "litellm-complex-fb", "litellm-reasoning-fb"]
    - litellm-simple: ["litellm-simple-fb"]
    - litellm-medium: ["litellm-medium-fb"]
    - litellm-complex: ["litellm-complex-fb"]
    - litellm-reasoning: ["litellm-reasoning-fb"]
    - litellm-medium-fb: ["litellm-simple-fb"]
    - litellm-complex-fb: ["litellm-simple-fb"]
    - litellm-reasoning-fb: ["litellm-simple-fb"]
general_settings:
  master_key: os.environ/LITELLM_MASTER_KEY
`)
	return b.String()
}

const UsageCallbackFileName = "ezyl3_usage_callback.py"

func CreateManagedProfile(paths ProfilePaths, secrets Secrets, domain string) (Profile, error) {
	if err := os.MkdirAll(paths.ProfileDir, 0o755); err != nil {
		return Profile{}, err
	}
	if err := os.MkdirAll(paths.LogsDir, 0o755); err != nil {
		return Profile{}, err
	}
	if err := os.WriteFile(filepath.Join(paths.ProfileDir, "config.yaml"), []byte(RenderLiteLLMConfig(secrets)), 0o644); err != nil {
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
	profile := Profile{Name: paths.Profile, Mode: ProfileModeManaged, RuntimeDir: paths.ProfileDir, LogsDir: paths.LogsDir, Port: 4400, TunnelProvider: "ngrok", Domain: domain, Paths: paths}
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

func InstallPythonDeps(paths ProfilePaths) error {
	return (UVToolchain{}).InstallPythonDeps(paths)
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
