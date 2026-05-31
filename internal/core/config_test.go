package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleLiteLLMConfig = `
model_list:
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
  - model_name: litellm-medium-fb
    litellm_params:
      model: ollama_chat/qwen3-coder-next
  - model_name: litellm-complex-fb
    litellm_params:
      model: ollama_chat/deepseek-v3.2
  - model_name: litellm-reasoning-fb
    litellm_params:
      model: ollama_chat/deepseek-v4-pro
  - model_name: litellm-auto
    litellm_params:
      model: auto_router/complexity_router
litellm_settings:
  fallbacks:
    - litellm-medium: ["litellm-medium-fb"]
`

func TestConfigManagerUpdatesOnlySelectedTier(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(sampleLiteLLMConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadLiteLLMConfig(path)
	if err != nil {
		t.Fatalf("LoadLiteLLMConfig returned error: %v", err)
	}
	if err := cfg.SetModel("litellm-medium", "huggingface/example/new-model"); err != nil {
		t.Fatalf("SetModel returned error: %v", err)
	}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "model: huggingface/example/new-model") {
		t.Fatalf("updated model not found:\n%s", text)
	}
	if strings.Count(text, "huggingface/example/new-model") != 1 {
		t.Fatalf("model update touched more than one entry:\n%s", text)
	}
	if !strings.Contains(text, "model: huggingface/deepseek-ai/DeepSeek-V4-Pro") {
		t.Fatalf("other tiers were unexpectedly changed:\n%s", text)
	}
}

func TestConfigValidationRejectsFallbackToUnknownModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	bad := strings.Replace(sampleLiteLLMConfig, `["litellm-medium-fb"]`, `["missing-fb"]`, 1)
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadLiteLLMConfig(path)
	if err != nil {
		t.Fatalf("LoadLiteLLMConfig returned error: %v", err)
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "missing-fb") {
		t.Fatalf("Validate error = %v, want missing fallback", err)
	}
}

func TestConfigValidationIdentifiesTierWithMissingProviderModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	bad := strings.Replace(sampleLiteLLMConfig, "      model: huggingface/deepseek-ai/DeepSeek-V4-Flash\n", "", 1)
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadLiteLLMConfig(path)
	if err != nil {
		t.Fatalf("LoadLiteLLMConfig returned error: %v", err)
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "litellm-simple") || !strings.Contains(err.Error(), "model") {
		t.Fatalf("Validate error = %v, want tier and missing model", err)
	}
}
