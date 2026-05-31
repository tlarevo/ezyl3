package core

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRenderLiteLLMConfigDefinesAutoRouterAndValidates(t *testing.T) {
	cfg, err := unmarshalConfig(RenderLiteLLMConfig(Secrets{HFBillTo: "billing-org"}))
	if err != nil {
		t.Fatalf("rendered config did not parse: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("rendered config failed validation: %v", err)
	}

	auto, ok := findModel(cfg, "litellm-auto")
	if !ok {
		t.Fatalf("rendered config is missing litellm-auto:\n%s", RenderLiteLLMConfig(Secrets{}))
	}
	if got := toStr(auto.LiteLLMParams["model"]); got != "auto_router/complexity_router" {
		t.Fatalf("litellm-auto model = %q, want auto_router/complexity_router", got)
	}
	routerCfg, ok := auto.LiteLLMParams["complexity_router_config"].(map[string]any)
	if !ok {
		t.Fatalf("litellm-auto is missing complexity_router_config: %#v", auto.LiteLLMParams)
	}
	if got := toStr(routerCfg["default_model"]); got != "litellm-medium" {
		t.Fatalf("router default_model = %q, want litellm-medium", got)
	}
	tiers, ok := routerCfg["tiers"].(map[string]any)
	if !ok {
		t.Fatalf("router config is missing tiers: %#v", routerCfg)
	}
	for tier, want := range map[string]string{
		"SIMPLE":    "litellm-simple",
		"MEDIUM":    "litellm-medium",
		"COMPLEX":   "litellm-complex",
		"REASONING": "litellm-reasoning",
	} {
		if got := toStr(tiers[tier]); got != want {
			t.Fatalf("router tier %s = %q, want %q", tier, got, want)
		}
	}
}

func TestRenderLiteLLMConfigIncludesAutoAndDowngradeFallbacks(t *testing.T) {
	cfg, err := unmarshalConfig(RenderLiteLLMConfig(Secrets{}))
	if err != nil {
		t.Fatalf("rendered config did not parse: %v", err)
	}
	wantAuto := []string{"litellm-simple-fb", "litellm-medium-fb", "litellm-complex-fb", "litellm-reasoning-fb"}
	if got := fallbackTargets(cfg, "litellm-auto"); !equalSlice(got, wantAuto) {
		t.Fatalf("litellm-auto fallbacks = %v, want %v", got, wantAuto)
	}
	for _, source := range []string{"litellm-medium-fb", "litellm-complex-fb", "litellm-reasoning-fb"} {
		if got := fallbackTargets(cfg, source); !equalSlice(got, []string{"litellm-simple-fb"}) {
			t.Fatalf("%s fallbacks = %v, want [litellm-simple-fb]", source, got)
		}
	}
}

func TestRenderLiteLLMConfigBillToHeaderIsOptional(t *testing.T) {
	withBill := RenderLiteLLMConfig(Secrets{HFBillTo: "eventinc-gmbh"})
	if !strings.Contains(withBill, `X-HF-Bill-To: "eventinc-gmbh"`) {
		t.Fatalf("expected quoted X-HF-Bill-To header when bill-to is set:\n%s", withBill)
	}

	withoutBill := RenderLiteLLMConfig(Secrets{})
	if strings.Contains(withoutBill, "X-HF-Bill-To") || strings.Contains(withoutBill, "extra_headers") {
		t.Fatalf("did not expect any bill-to header when bill-to is empty:\n%s", withoutBill)
	}
	cfg, err := unmarshalConfig(withoutBill)
	if err != nil {
		t.Fatalf("rendered config without bill-to did not parse: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("rendered config without bill-to failed validation: %v", err)
	}
}

func TestRenderLiteLLMConfigBillToAppliesToEveryHFEntry(t *testing.T) {
	cfg, err := unmarshalConfig(RenderLiteLLMConfig(Secrets{HFBillTo: "billing-org"}))
	if err != nil {
		t.Fatalf("rendered config did not parse: %v", err)
	}
	for _, name := range []string{"litellm-simple", "litellm-medium", "litellm-complex", "litellm-reasoning"} {
		entry, ok := findModel(cfg, name)
		if !ok {
			t.Fatalf("missing HF model %s", name)
		}
		headers, ok := entry.LiteLLMParams["extra_headers"].(map[string]any)
		if !ok {
			t.Fatalf("%s missing extra_headers: %#v", name, entry.LiteLLMParams)
		}
		if got := toStr(headers["X-HF-Bill-To"]); got != "billing-org" {
			t.Fatalf("%s X-HF-Bill-To = %q, want billing-org", name, got)
		}
	}
	// Ollama fallback entries should never carry the HF billing header.
	fb, _ := findModel(cfg, "litellm-simple-fb")
	if _, ok := fb.LiteLLMParams["extra_headers"]; ok {
		t.Fatalf("ollama fallback should not carry extra_headers: %#v", fb.LiteLLMParams)
	}
}

func unmarshalConfig(text string) (*LiteLLMConfig, error) {
	var cfg LiteLLMConfig
	if err := yaml.Unmarshal([]byte(text), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func findModel(cfg *LiteLLMConfig, name string) (ModelEntry, bool) {
	for _, entry := range cfg.ModelList {
		if entry.ModelName == name {
			return entry, true
		}
	}
	return ModelEntry{}, false
}

func fallbackTargets(cfg *LiteLLMConfig, source string) []string {
	for _, fallback := range cfg.LiteLLMSettings.Fallbacks {
		if targets, ok := fallback[source]; ok {
			return targets
		}
	}
	return nil
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func toStr(v any) string {
	s, _ := v.(string)
	return s
}
