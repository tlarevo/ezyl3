package setupwizard

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"ezyl3/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWizardInitialViewShowsGuidedSetupFrame(t *testing.T) {
	m := newModel(testOptions(t), nil)

	view := m.View()
	for _, want := range []string{"ezyl3 setup", "Profile", "Tunnel", "Secrets", "Review", "enter next"} {
		if !strings.Contains(view, want) {
			t.Fatalf("initial view missing %q:\n%s", want, view)
		}
	}
}

func TestWizardCapturesTunnelDomainInput(t *testing.T) {
	m := newModel(testOptions(t), nil)
	m.step = stepTunnel

	updated, _ := m.Update(keyRunes("demo.ngrok-free.dev"))
	m = updated.(model)

	if got := m.domainInput.Value(); got != "demo.ngrok-free.dev" {
		t.Fatalf("domain input = %q", got)
	}
	if !strings.Contains(m.View(), "demo.ngrok-free.dev") {
		t.Fatalf("domain was not rendered:\n%s", m.View())
	}
}

func TestWizardShowsLocalOnlyCopyWhenDomainBlank(t *testing.T) {
	m := newModel(testOptions(t), nil)
	m.step = stepTunnel

	view := m.View()
	if !strings.Contains(view, "local-only") {
		t.Fatalf("blank domain should explain local-only setup:\n%s", view)
	}
}

func TestWizardMasksSecretInputs(t *testing.T) {
	m := newModel(testOptions(t), nil)
	m.step = stepSecrets

	updated, _ := m.Update(keyRunes("hf_secret"))
	m = updated.(model)

	view := m.View()
	if strings.Contains(view, "hf_secret") {
		t.Fatalf("secret leaked in wizard view:\n%s", view)
	}
	if !strings.Contains(view, "*********") {
		t.Fatalf("masked secret not rendered:\n%s", view)
	}
}

func TestWizardNavigationPreservesInputValues(t *testing.T) {
	m := newModel(testOptions(t), nil)
	m.step = stepTunnel
	updated, _ := m.Update(keyRunes("demo.ngrok-free.dev"))
	m = updated.(model)

	updated, _ = m.Update(keyString("enter"))
	m = updated.(model)
	updated, _ = m.Update(keyString("b"))
	m = updated.(model)

	if m.step != stepTunnel {
		t.Fatalf("step = %v, want tunnel", m.step)
	}
	if got := m.domainInput.Value(); got != "demo.ngrok-free.dev" {
		t.Fatalf("domain input after navigation = %q", got)
	}
}

func TestWizardReviewRunsSetupWithCapturedOptions(t *testing.T) {
	opts := testOptions(t)
	opts.Force = true
	m := newModel(opts, func(got core.SetupOptions, deps core.SetupDependencies) (core.SetupResult, error) {
		if got.Domain != "demo.ngrok-free.dev" {
			t.Fatalf("Domain = %q", got.Domain)
		}
		if got.Secrets.HFToken != "hf_secret" || got.Secrets.OllamaAPIKey != "ollama_secret" || got.Secrets.LiteLLMMasterKey != "master_secret" {
			t.Fatalf("Secrets = %#v", got.Secrets)
		}
		if !got.Force || !got.SkipPythonDeps {
			t.Fatalf("Force/SkipPythonDeps = %v/%v", got.Force, got.SkipPythonDeps)
		}
		if got.ExecutablePath != "/usr/local/bin/ezyl3" {
			t.Fatalf("ExecutablePath = %q", got.ExecutablePath)
		}
		return testSetupResult(opts), nil
	})
	m.domainInput.SetValue("demo.ngrok-free.dev")
	m.secretInputs[0].SetValue("hf_secret")
	m.secretInputs[1].SetValue("ollama_secret")
	m.secretInputs[3].SetValue("master_secret")
	m.step = stepReview

	updated, cmd := m.Update(keyString("enter"))
	m = updated.(model)
	if m.step != stepRunning {
		t.Fatalf("step after review enter = %v, want running", m.step)
	}
	if !strings.Contains(m.View(), "Creating managed profile") {
		t.Fatalf("running view missing spinner copy:\n%s", m.View())
	}
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(model)

	if m.step != stepDone || m.err != nil {
		t.Fatalf("final state = %v err=%v", m.step, m.err)
	}
	view := m.View()
	if !strings.Contains(view, "Created managed profile default") || strings.Contains(view, "hf_secret") {
		t.Fatalf("success view missing summary or leaked secret:\n%s", view)
	}
}

func TestWizardSetupErrorDoesNotLeakSecrets(t *testing.T) {
	opts := testOptions(t)
	m := newModel(opts, func(core.SetupOptions, core.SetupDependencies) (core.SetupResult, error) {
		return core.SetupResult{}, errors.New("boom")
	})
	m.secretInputs[0].SetValue("hf_secret")
	m.step = stepReview

	updated, cmd := m.Update(keyString("enter"))
	m = updated.(model)
	updated, _ = m.Update(cmd())
	m = updated.(model)

	view := m.View()
	if !strings.Contains(view, "Setup failed: boom") {
		t.Fatalf("error view missing failure:\n%s", view)
	}
	if strings.Contains(view, "hf_secret") {
		t.Fatalf("error view leaked secret:\n%s", view)
	}
}

func testOptions(t *testing.T) Options {
	t.Helper()
	root := t.TempDir()
	return Options{
		Paths: core.ProfilePaths{
			Profile:    "default",
			ProfileDir: filepath.Join(root, "profile"),
			LogsDir:    filepath.Join(root, "logs"),
			HomeDir:    root,
		},
		ExecutablePath: "/usr/local/bin/ezyl3",
		SkipPythonDeps: true,
	}
}

func testSetupResult(opts Options) core.SetupResult {
	secrets := core.Secrets{HFToken: "hf_secret", OllamaAPIKey: "ollama_secret", LiteLLMMasterKey: "master_secret"}
	return core.SetupResult{
		Profile: core.Profile{
			Name:       opts.Paths.Profile,
			RuntimeDir: opts.Paths.ProfileDir,
			Paths:      opts.Paths,
		},
		BaseURL:         "https://demo.ngrok-free.dev/v1",
		LaunchAgentDir:  filepath.Join(opts.Paths.HomeDir, "Library", "LaunchAgents"),
		PythonInstalled: false,
		Secrets:         secrets,
	}
}

func keyString(value string) tea.KeyMsg {
	switch value {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "b":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func keyRunes(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
