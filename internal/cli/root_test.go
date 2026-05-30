package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ezyl3/internal/core"
)

const cliSampleConfig = `
model_list:
  - model_name: litellm-simple
    litellm_params: {model: huggingface/simple}
  - model_name: litellm-medium
    litellm_params: {model: huggingface/medium}
  - model_name: litellm-complex
    litellm_params: {model: huggingface/complex}
  - model_name: litellm-reasoning
    litellm_params: {model: huggingface/reasoning}
  - model_name: litellm-simple-fb
    litellm_params: {model: ollama_chat/simple}
  - model_name: litellm-medium-fb
    litellm_params: {model: ollama_chat/medium}
  - model_name: litellm-complex-fb
    litellm_params: {model: ollama_chat/complex}
  - model_name: litellm-reasoning-fb
    litellm_params: {model: ollama_chat/reasoning}
litellm_settings:
  fallbacks:
    - litellm-medium: ["litellm-medium-fb"]
general_settings:
  master_key: os.environ/LITELLM_MASTER_KEY
`

func TestDoctorPathJSONDoesNotLeakSecrets(t *testing.T) {
	runtime := writeRuntimeFixture(t)

	out, err := execute("doctor", "--path", runtime, "--json")
	if err != nil {
		t.Fatalf("doctor returned error: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"runtime_path"`) || !strings.Contains(out, `"config.yaml"`) {
		t.Fatalf("doctor json missing expected fields: %s", out)
	}
	if strings.Contains(out, "sk-cursor-secret") || strings.Contains(out, "ollama.secret") {
		t.Fatalf("doctor leaked a secret: %s", out)
	}
}

func TestCursorSettingsPrintsURLAndRedactsKey(t *testing.T) {
	runtime := writeRuntimeFixture(t)

	out, err := execute("cursor", "settings", "--path", runtime)
	if err != nil {
		t.Fatalf("cursor settings returned error: %v\n%s", err, out)
	}
	for _, want := range []string{"https://example.ngrok-free.dev/v1", "litellm-auto", "API key: set"} {
		if !strings.Contains(out, want) {
			t.Fatalf("cursor settings missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "sk-cursor-secret") {
		t.Fatalf("cursor settings leaked master key: %s", out)
	}
}

func TestModelsSetUpdatesOnlySelectedTier(t *testing.T) {
	runtime := writeRuntimeFixture(t)

	out, err := execute("models", "set", "litellm-medium", "huggingface/example/new", "--path", runtime)
	if err != nil {
		t.Fatalf("models set returned error: %v\n%s", err, out)
	}
	data, err := os.ReadFile(filepath.Join(runtime, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Count(text, "huggingface/example/new") != 1 {
		t.Fatalf("expected exactly one update:\n%s", text)
	}
	if !strings.Contains(out, "updated litellm-medium") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestSetupCreatesLocalOnlyProfileWithoutLeakingSecrets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	out, err := execute(
		"setup",
		"--skip-python-deps",
		"--hf-token", "hf_secret",
		"--ollama-api-key", "ollama_secret",
	)
	if err != nil {
		t.Fatalf("setup returned error: %v\n%s", err, out)
	}

	runtime := filepath.Join(home, ".local", "share", "ezyl3", "profiles", "default")
	for _, path := range []string{"config.yaml", ".env", "profile.json"} {
		if _, err := os.Stat(filepath.Join(runtime, path)); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(runtime, "run-proxy.sh")); !os.IsNotExist(err) {
		t.Fatalf("run-proxy.sh should not be generated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "Library", "LaunchAgents", "com.ezyl3.default.litellm.plist")); err != nil {
		t.Fatalf("expected litellm LaunchAgent: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "Library", "LaunchAgents", "com.ezyl3.default.ngrok.plist")); !os.IsNotExist(err) {
		t.Fatalf("ngrok LaunchAgent should not exist for local-only setup: %v", err)
	}
	for _, leaked := range []string{"hf_secret", "ollama_secret", "sk-cursor-"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("setup leaked %q:\n%s", leaked, out)
		}
	}
	for _, want := range []string{"Base URL: http://127.0.0.1:4400/v1", "LiteLLM master key: set", runtime} {
		if !strings.Contains(out, want) {
			t.Fatalf("setup output missing %q:\n%s", want, out)
		}
	}
}

func TestSetupRequiresForceToOverwriteManagedProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if out, err := execute("setup", "--skip-python-deps"); err != nil {
		t.Fatalf("initial setup returned error: %v\n%s", err, out)
	}
	out, err := execute("setup", "--skip-python-deps")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second setup error = %v, output = %s", err, out)
	}
	if out, err := execute("setup", "--skip-python-deps", "--force"); err != nil {
		t.Fatalf("forced setup returned error: %v\n%s", err, out)
	}
}

func TestProxyCommandAppearsInHelp(t *testing.T) {
	out, err := execute("--help")
	if err != nil {
		t.Fatalf("help returned error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "proxy") {
		t.Fatalf("help does not mention proxy command:\n%s", out)
	}
}

func TestProxyRunUsesResolvedRuntime(t *testing.T) {
	runtime := writeRuntimeFixture(t)
	oldRunner := proxyRunner
	t.Cleanup(func() { proxyRunner = oldRunner })

	var got core.Runtime
	proxyRunner = func(runtime core.Runtime, stdio core.ProxyStdio) error {
		got = runtime
		_, _ = stdio.Stdout.Write([]byte("proxy delegated\n"))
		return nil
	}

	out, err := execute("proxy", "run", "--path", runtime)
	if err != nil {
		t.Fatalf("proxy run returned error: %v\n%s", err, out)
	}
	if got.Path != runtime || got.Port != 4400 {
		t.Fatalf("runtime = %#v, want path %q port 4400", got, runtime)
	}
	if !strings.Contains(out, "proxy delegated") {
		t.Fatalf("proxy output missing delegation marker:\n%s", out)
	}
}

func execute(args ...string) (string, error) {
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func writeRuntimeFixture(t *testing.T) string {
	t.Helper()
	runtime := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtime, "config.yaml"), []byte(cliSampleConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime, ".env"), []byte("OLLAMA_API_KEY=\"ollama.secret\"\nLITELLM_MASTER_KEY=\"sk-cursor-secret\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(runtime, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	logLine := `t=2026-05-26 lvl=info msg="started tunnel" url=https://example.ngrok-free.dev` + "\n"
	if err := os.WriteFile(filepath.Join(runtime, "logs", "ngrok.out.log"), []byte(logLine), 0o644); err != nil {
		t.Fatal(err)
	}
	return runtime
}
