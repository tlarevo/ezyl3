package core

import (
	"fmt"
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

var RequiredModelNames = []string{
	"litellm-simple",
	"litellm-medium",
	"litellm-complex",
	"litellm-reasoning",
	"litellm-simple-fb",
	"litellm-medium-fb",
	"litellm-complex-fb",
	"litellm-reasoning-fb",
}

type LiteLLMConfig struct {
	ModelList       []ModelEntry    `yaml:"model_list"`
	LiteLLMSettings LiteLLMSettings `yaml:"litellm_settings,omitempty"`
	GeneralSettings map[string]any  `yaml:"general_settings,omitempty"`
	Extra           map[string]any  `yaml:",inline"`
}

type ModelEntry struct {
	ModelName     string         `yaml:"model_name"`
	LiteLLMParams map[string]any `yaml:"litellm_params"`
}

type LiteLLMSettings struct {
	Fallbacks []map[string][]string `yaml:"fallbacks,omitempty"`
	Extra     map[string]any        `yaml:",inline"`
}

func LoadLiteLLMConfig(path string) (*LiteLLMConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg LiteLLMConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *LiteLLMConfig) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c *LiteLLMConfig) SetModel(modelName, providerModel string) error {
	for i := range c.ModelList {
		if c.ModelList[i].ModelName == modelName {
			if c.ModelList[i].LiteLLMParams == nil {
				c.ModelList[i].LiteLLMParams = map[string]any{}
			}
			c.ModelList[i].LiteLLMParams["model"] = providerModel
			return nil
		}
	}
	return fmt.Errorf("model tier %q not found", modelName)
}

func (c *LiteLLMConfig) Validate() error {
	names := map[string]bool{}
	for _, entry := range c.ModelList {
		names[entry.ModelName] = true
	}
	for _, required := range RequiredModelNames {
		if !names[required] {
			return fmt.Errorf("required model %q is missing", required)
		}
	}
	for _, fallback := range c.LiteLLMSettings.Fallbacks {
		for source, targets := range fallback {
			if !names[source] {
				return fmt.Errorf("fallback source %q is not defined", source)
			}
			for _, target := range targets {
				if !names[target] {
					return fmt.Errorf("fallback target %q is not defined", target)
				}
			}
		}
	}
	return nil
}

func (c *LiteLLMConfig) Models() []ModelEntry {
	out := slices.Clone(c.ModelList)
	return out
}
